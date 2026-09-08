package daemon

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"fmt"
	"io"
	"log/slog"
	"net/http"
	"net/url"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"sync/atomic"
	"time"

	"github.com/multica-ai/multica/server/pkg/agent"
)

const (
	taskRunEvidenceSchemaVersion              = "task_run_evidence/v1"
	taskRunEvidenceUploadTimeout              = time.Second
	taskRunEvidenceTerminalFlushTimeout       = 1250 * time.Millisecond
	maxRuntimeIdentityBytes             int64 = 512 << 20
)

var taskRunEvidenceRetrySchedule = []time.Duration{100 * time.Millisecond, 300 * time.Millisecond}

var runtimeIdentityCache sync.Map

type TaskRunEvidenceTiming struct {
	Known      bool   `json:"known"`
	DurationMS *int64 `json:"duration_ms"`
}

type TaskRunEvidenceTimings struct {
	Queue        TaskRunEvidenceTiming `json:"queue"`
	Preparation  TaskRunEvidenceTiming `json:"preparation"`
	FirstTool    TaskRunEvidenceTiming `json:"first_tool"`
	Execution    TaskRunEvidenceTiming `json:"execution"`
	Finalization TaskRunEvidenceTiming `json:"finalization"`
}

type TaskRunEvidenceModelConfig struct {
	Model  *string `json:"model"`
	Effort *string `json:"effort"`
}

type TaskRunEvidenceProviderModel struct {
	Model  *string `json:"model"`
	Source string  `json:"source"`
}

type TaskRunEvidenceRuntime struct {
	Version       *string `json:"version"`
	ContentSHA256 *string `json:"content_sha256"`
}

type TaskRunEvidenceUsage struct {
	InputUncachedTokens   *int64 `json:"input_uncached_tokens"`
	InputCacheReadTokens  *int64 `json:"input_cache_read_tokens"`
	InputCacheWriteTokens *int64 `json:"input_cache_write_tokens"`
	OutputTokens          *int64 `json:"output_tokens"`
	Complete              bool   `json:"complete"`
	Source                string `json:"source"`
}

type TaskRunEvidenceProviderCost struct {
	AmountUSDTicks *int64 `json:"amount_usd_ticks"`
	Complete       bool   `json:"complete"`
	Authority      string `json:"authority"`
	Basis          string `json:"basis"`
	Source         string `json:"source"`
}

type TaskRunEvidenceRequest struct {
	SchemaVersion    string                              `json:"schema_version"`
	Attempt          int32                               `json:"attempt"`
	ClaimGeneration  int64                               `json:"claim_generation"`
	Revision         int64                               `json:"revision"`
	Timings          TaskRunEvidenceTimings              `json:"timings"`
	Requested        TaskRunEvidenceModelConfig          `json:"requested"`
	ClientEffective  TaskRunEvidenceModelConfig          `json:"client_effective"`
	ProviderReported TaskRunEvidenceProviderModel        `json:"provider_reported"`
	Runtime          TaskRunEvidenceRuntime              `json:"runtime"`
	Usage            TaskRunEvidenceUsage                `json:"usage"`
	ProviderCost     TaskRunEvidenceProviderCost         `json:"provider_cost"`
	ModelUsage       *TaskRunEvidenceModelUsageInventory `json:"model_usage,omitempty"`
}

type taskRunEvidenceClient interface {
	ReportTaskRunEvidence(context.Context, string, string, TaskRunEvidenceRequest) error
}

func (c *Client) ReportTaskRunEvidence(ctx context.Context, runtimeID, taskID string, evidence TaskRunEvidenceRequest) error {
	path := fmt.Sprintf("/api/daemon/runtimes/%s/tasks/%s/run-evidence", url.PathEscape(runtimeID), url.PathEscape(taskID))
	return c.postJSONWithRetry(ctx, path, evidence, nil, taskRunEvidenceRetrySchedule)
}

type taskRunEvidenceContextKey struct{}

type taskRunEvidenceUpload struct {
	request  TaskRunEvidenceRequest
	terminal bool
}

type taskRunEvidenceRecorder struct {
	mu                  sync.Mutex
	client              taskRunEvidenceClient
	logger              *slog.Logger
	taskID              string
	runtimeID           string
	attempt             int32
	revision            int64
	disabled            atomic.Bool
	terminalQueued      bool
	request             TaskRunEvidenceRequest
	preparationStarted  time.Time
	executionStarted    time.Time
	finalizationStarted time.Time
	pending             *taskRunEvidenceUpload
	uploadCancel        context.CancelFunc
	wake                chan struct{}
	stop                chan struct{}
	done                chan struct{}
	stopOnce            sync.Once
}

func newTaskRunEvidenceRecorder(client taskRunEvidenceClient, task Task, logger *slog.Logger) *taskRunEvidenceRecorder {
	recorder := &taskRunEvidenceRecorder{client: client, logger: logger, taskID: task.ID, runtimeID: task.RuntimeID, attempt: int32(task.ClaimAttempt), request: TaskRunEvidenceRequest{
		SchemaVersion:    taskRunEvidenceSchemaVersion,
		ClaimGeneration:  task.ClaimGeneration,
		ProviderReported: TaskRunEvidenceProviderModel{Source: agent.EvidenceSourceMissing},
		Usage:            TaskRunEvidenceUsage{Source: agent.EvidenceSourceMissing},
		ProviderCost:     TaskRunEvidenceProviderCost{Authority: agent.CostAuthorityMissing, Basis: agent.CostBasisMissing, Source: agent.EvidenceSourceMissing},
	}, wake: make(chan struct{}, 1), stop: make(chan struct{}), done: make(chan struct{})}
	// Old servers reject unknown JSON fields, so only a negotiated claim enables this extension.
	if task.TaskRunEvidenceModelUsageV1 {
		recorder.request.ModelUsage = modelUsageFromAgent(agent.ModelUsageInventoryEvidence{})
	}
	if recorder.attempt <= 0 || recorder.request.ClaimGeneration <= 0 || recorder.client == nil {
		close(recorder.done)
		return recorder
	}
	go recorder.uploadWorker()
	return recorder
}

func withTaskRunEvidenceRecorder(ctx context.Context, recorder *taskRunEvidenceRecorder) context.Context {
	return context.WithValue(ctx, taskRunEvidenceContextKey{}, recorder)
}

func taskRunEvidenceRecorderFromContext(ctx context.Context) *taskRunEvidenceRecorder {
	recorder, _ := ctx.Value(taskRunEvidenceContextKey{}).(*taskRunEvidenceRecorder)
	return recorder
}

func (r *taskRunEvidenceRecorder) beginPreparation(now time.Time) {
	if r == nil || r.attempt <= 0 {
		return
	}
	r.mu.Lock()
	if r.preparationStarted.IsZero() {
		r.preparationStarted = now
		r.queueLatestLocked(false)
	}
	r.mu.Unlock()
}

func (r *taskRunEvidenceRecorder) beginExecution(now time.Time) {
	if r == nil || r.attempt <= 0 {
		return
	}
	r.mu.Lock()
	if r.executionStarted.IsZero() {
		r.executionStarted = now
		r.request.Timings.Preparation = knownEvidenceTiming(now.Sub(r.preparationStarted))
		r.queueLatestLocked(false)
	}
	r.mu.Unlock()
}

func (r *taskRunEvidenceRecorder) setExecutionIdentity(requestedModel, requestedEffort, effectiveModel, effectiveEffort, version, executablePath string) {
	if r == nil || r.attempt <= 0 {
		return
	}
	sanitizedVersion, digest, verified := verifiedRuntimeIdentity(version, executablePath)
	r.mu.Lock()
	defer r.mu.Unlock()
	r.request.Requested = TaskRunEvidenceModelConfig{Model: nonEmptyEvidenceString(requestedModel), Effort: nonEmptyEvidenceString(requestedEffort)}
	r.request.ClientEffective = TaskRunEvidenceModelConfig{Model: nonEmptyEvidenceString(effectiveModel), Effort: nonEmptyEvidenceString(effectiveEffort)}
	if verified {
		r.request.Runtime = TaskRunEvidenceRuntime{Version: &sanitizedVersion, ContentSHA256: &digest}
	}
}

func (r *taskRunEvidenceRecorder) firstTool(now time.Time) {
	if r == nil || r.attempt <= 0 {
		return
	}
	r.mu.Lock()
	if r.request.Timings.FirstTool.Known || r.executionStarted.IsZero() {
		r.mu.Unlock()
		return
	}
	r.request.Timings.FirstTool = knownEvidenceTiming(now.Sub(r.executionStarted))
	r.queueLatestLocked(false)
	r.mu.Unlock()
}

func (r *taskRunEvidenceRecorder) finishExecution(now time.Time, evidence *agent.ExecutionEvidence) {
	if r == nil || r.attempt <= 0 {
		return
	}
	r.mu.Lock()
	if !r.executionStarted.IsZero() && !r.request.Timings.Execution.Known {
		r.request.Timings.Execution = knownEvidenceTiming(now.Sub(r.executionStarted))
		r.finalizationStarted = now
		r.applyAgentEvidence(evidence)
		r.queueLatestLocked(false)
	}
	r.mu.Unlock()
}

func (r *taskRunEvidenceRecorder) finishRunTask(now time.Time) {
	if r == nil || r.attempt <= 0 {
		return
	}
	r.mu.Lock()
	if r.finalizationStarted.IsZero() {
		if !r.executionStarted.IsZero() && !r.request.Timings.Execution.Known {
			r.request.Timings.Execution = knownEvidenceTiming(now.Sub(r.executionStarted))
		} else if !r.preparationStarted.IsZero() && !r.request.Timings.Preparation.Known {
			r.request.Timings.Preparation = knownEvidenceTiming(now.Sub(r.preparationStarted))
		}
		r.finalizationStarted = now
		r.queueLatestLocked(false)
	}
	r.mu.Unlock()
}

func (r *taskRunEvidenceRecorder) finishFinalization(now time.Time) {
	if r == nil || r.attempt <= 0 {
		return
	}
	r.mu.Lock()
	if r.finalizationStarted.IsZero() || r.request.Timings.Finalization.Known {
		r.mu.Unlock()
		return
	}
	r.request.Timings.Finalization = knownEvidenceTiming(now.Sub(r.finalizationStarted))
	r.queueLatestLocked(true)
	r.mu.Unlock()
	r.waitForWorker(taskRunEvidenceTerminalFlushTimeout)
}

func knownEvidenceTiming(duration time.Duration) TaskRunEvidenceTiming {
	ms := duration.Milliseconds()
	if ms < 0 {
		ms = 0
	}
	return TaskRunEvidenceTiming{Known: true, DurationMS: &ms}
}

func (r *taskRunEvidenceRecorder) applyAgentEvidence(value *agent.ExecutionEvidence) {
	if value == nil {
		return
	}
	r.request.Usage = TaskRunEvidenceUsage{InputUncachedTokens: value.Usage.InputUncachedTokens, InputCacheReadTokens: value.Usage.InputCacheReadTokens, InputCacheWriteTokens: value.Usage.InputCacheWriteTokens, OutputTokens: value.Usage.OutputTokens, Complete: value.Usage.Complete, Source: value.Usage.Source}
	r.request.ProviderReported = TaskRunEvidenceProviderModel{Model: value.ProviderModel.Model, Source: value.ProviderModel.Source}
	r.request.ProviderCost = TaskRunEvidenceProviderCost{AmountUSDTicks: value.ProviderCost.AmountUSDTicks, Complete: value.ProviderCost.Complete, Authority: value.ProviderCost.Authority, Basis: value.ProviderCost.Basis, Source: value.ProviderCost.Source}
	if r.request.ModelUsage != nil {
		r.request.ModelUsage = modelUsageFromAgent(value.ModelUsage)
	}
}

func (r *taskRunEvidenceRecorder) queueLatestLocked(terminal bool) {
	if r.terminalQueued && !terminal {
		return
	}
	r.revision++
	r.request.Attempt, r.request.Revision = r.attempt, r.revision
	if terminal {
		r.terminalQueued = true
	}
	if r.disabled.Load() || r.client == nil {
		return
	}
	upload := taskRunEvidenceUpload{request: cloneTaskRunEvidenceRequest(r.request), terminal: terminal}
	r.pending = &upload
	if r.uploadCancel != nil {
		r.uploadCancel()
	}
	select {
	case r.wake <- struct{}{}:
	default:
	}
}

func (r *taskRunEvidenceRecorder) uploadWorker() {
	defer close(r.done)
	for {
		select {
		case <-r.stop:
			return
		case <-r.wake:
			r.mu.Lock()
			upload := r.pending
			if upload == nil {
				r.mu.Unlock()
				continue
			}
			// Register cancellation before a newer snapshot can supersede this
			// upload, so terminal flush never waits behind an uncancellable one.
			ctx, cancel := context.WithTimeout(context.Background(), taskRunEvidenceUploadTimeout)
			r.pending = nil
			r.uploadCancel = cancel
			r.mu.Unlock()
			select {
			case <-r.stop:
				cancel()
				return
			default:
			}
			err := r.client.ReportTaskRunEvidence(ctx, r.runtimeID, r.taskID, upload.request)
			cancel()
			r.mu.Lock()
			r.uploadCancel = nil
			r.mu.Unlock()
			if err != nil && !errors.Is(err, context.Canceled) {
				var requestErr *requestError
				if errors.As(err, &requestErr) && requestErr.StatusCode == http.StatusNotFound {
					r.disabled.Store(true)
				}
				if r.logger != nil {
					r.logger.Debug("task run evidence upload failed", "status_code", requestErrorStatus(err))
				}
			}
			if r.disabled.Load() || upload.terminal {
				return
			}
		}
	}
}

func (r *taskRunEvidenceRecorder) waitForWorker(timeout time.Duration) {
	if r == nil {
		return
	}
	timer := time.NewTimer(timeout)
	defer timer.Stop()
	select {
	case <-r.done:
	case <-timer.C:
		r.stopWorker()
	}
}

func (r *taskRunEvidenceRecorder) close() {
	if r == nil {
		return
	}
	r.stopWorker()
	r.waitForWorker(taskRunEvidenceUploadTimeout + 250*time.Millisecond)
}

func (r *taskRunEvidenceRecorder) stopWorker() {
	if r == nil {
		return
	}
	r.stopOnce.Do(func() {
		select {
		case <-r.done:
		default:
			close(r.stop)
		}
		r.mu.Lock()
		cancel := r.uploadCancel
		r.mu.Unlock()
		if cancel != nil {
			cancel()
		}
	})
}

func cloneTaskRunEvidenceRequest(request TaskRunEvidenceRequest) TaskRunEvidenceRequest {
	clone := request
	clone.Timings.Queue.DurationMS = cloneInt64Pointer(request.Timings.Queue.DurationMS)
	clone.Timings.Preparation.DurationMS = cloneInt64Pointer(request.Timings.Preparation.DurationMS)
	clone.Timings.FirstTool.DurationMS = cloneInt64Pointer(request.Timings.FirstTool.DurationMS)
	clone.Timings.Execution.DurationMS = cloneInt64Pointer(request.Timings.Execution.DurationMS)
	clone.Timings.Finalization.DurationMS = cloneInt64Pointer(request.Timings.Finalization.DurationMS)
	clone.Requested.Model = cloneStringPointer(request.Requested.Model)
	clone.Requested.Effort = cloneStringPointer(request.Requested.Effort)
	clone.ClientEffective.Model = cloneStringPointer(request.ClientEffective.Model)
	clone.ClientEffective.Effort = cloneStringPointer(request.ClientEffective.Effort)
	clone.ProviderReported.Model = cloneStringPointer(request.ProviderReported.Model)
	clone.Runtime.Version = cloneStringPointer(request.Runtime.Version)
	clone.Runtime.ContentSHA256 = cloneStringPointer(request.Runtime.ContentSHA256)
	clone.Usage.InputUncachedTokens = cloneInt64Pointer(request.Usage.InputUncachedTokens)
	clone.Usage.InputCacheReadTokens = cloneInt64Pointer(request.Usage.InputCacheReadTokens)
	clone.Usage.InputCacheWriteTokens = cloneInt64Pointer(request.Usage.InputCacheWriteTokens)
	clone.Usage.OutputTokens = cloneInt64Pointer(request.Usage.OutputTokens)
	clone.ProviderCost.AmountUSDTicks = cloneInt64Pointer(request.ProviderCost.AmountUSDTicks)
	clone.ModelUsage = cloneTaskRunEvidenceModelUsage(request.ModelUsage)
	return clone
}

func cloneInt64Pointer(value *int64) *int64 {
	if value == nil {
		return nil
	}
	clone := *value
	return &clone
}

func cloneStringPointer(value *string) *string {
	if value == nil {
		return nil
	}
	clone := *value
	return &clone
}

func requestErrorStatus(err error) int {
	var requestErr *requestError
	if errors.As(err, &requestErr) {
		return requestErr.StatusCode
	}
	return 0
}

func nonEmptyEvidenceString(value string) *string {
	value = strings.TrimSpace(value)
	if value == "" {
		return nil
	}
	return &value
}

func verifiedRuntimeIdentity(version, executablePath string) (string, string, bool) {
	version = strings.TrimSpace(version)
	if version == "" || strings.ContainsAny(version, `/\`) || !filepath.IsAbs(executablePath) {
		return "", "", false
	}
	realPath, err := filepath.EvalSymlinks(executablePath)
	if err != nil || !filepath.IsAbs(realPath) {
		return "", "", false
	}
	pathStat, err := os.Stat(realPath)
	if err != nil || !validRuntimeIdentityStat(pathStat) {
		return "", "", false
	}
	if cached, ok := runtimeIdentityCache.Load(realPath); ok {
		entry := cached.(runtimeIdentityCacheEntry)
		if sameRuntimeIdentityStat(entry.info, pathStat) {
			return version, entry.digest, true
		}
	}
	file, err := os.Open(realPath)
	if err != nil {
		return "", "", false
	}
	defer file.Close()
	stat, err := file.Stat()
	if err != nil || !validRuntimeIdentityStat(stat) || !sameRuntimeIdentityStat(pathStat, stat) {
		return "", "", false
	}
	hash := sha256.New()
	written, err := io.Copy(hash, io.LimitReader(file, maxRuntimeIdentityBytes+1))
	if err != nil || written != stat.Size() || written > maxRuntimeIdentityBytes {
		return "", "", false
	}
	postHashStat, err := file.Stat()
	if err != nil || !sameRuntimeIdentityStat(stat, postHashStat) {
		return "", "", false
	}
	digest := "sha256:" + hex.EncodeToString(hash.Sum(nil))
	runtimeIdentityCache.Store(realPath, runtimeIdentityCacheEntry{info: postHashStat, digest: digest})
	return version, digest, true
}

type runtimeIdentityCacheEntry struct {
	info   os.FileInfo
	digest string
}

func validRuntimeIdentityStat(info os.FileInfo) bool {
	return info.Mode().IsRegular() && info.Size() >= 0 && info.Size() <= maxRuntimeIdentityBytes
}

func sameRuntimeIdentityStat(left, right os.FileInfo) bool {
	return os.SameFile(left, right) && left.Mode() == right.Mode() && left.Size() == right.Size() && left.ModTime().Equal(right.ModTime())
}
