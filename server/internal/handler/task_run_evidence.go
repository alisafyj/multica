package handler

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"regexp"
	"strings"
	"unicode"
	"unicode/utf8"

	"github.com/go-chi/chi/v5"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgtype"
	"github.com/multica-ai/multica/server/internal/middleware"
	"github.com/multica-ai/multica/server/internal/service"
	db "github.com/multica-ai/multica/server/pkg/db/generated"
)

const (
	taskRunEvidenceSchemaVersion        = "task_run_evidence/v1"
	maxTaskRunEvidenceBodyBytes         = 32 << 10
	maxTaskRunEvidenceModelUsageEntries = 12
)

var taskRunEvidenceSHA256Pattern = regexp.MustCompile(`^sha256:[0-9a-f]{64}$`)

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

// TaskRunEvidenceProviderCost is provider-reported evidence, not billing data.
type TaskRunEvidenceProviderCost struct {
	AmountUSDTicks *int64 `json:"amount_usd_ticks"`
	Complete       bool   `json:"complete"`
	Authority      string `json:"authority"`
	Basis          string `json:"basis"`
	Source         string `json:"source"`
}

type TaskRunEvidenceModelUsage struct {
	Model        string                      `json:"model"`
	Usage        TaskRunEvidenceUsage        `json:"usage"`
	ProviderCost TaskRunEvidenceProviderCost `json:"provider_cost"`
}

type TaskRunEvidenceModelUsageInventory struct {
	Entries   []TaskRunEvidenceModelUsage `json:"entries"`
	Complete  bool                        `json:"complete"`
	Truncated bool                        `json:"truncated"`
	Source    string                      `json:"source"`
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

type TaskRunEvidenceResponse struct {
	SchemaVersion       string                              `json:"schema_version"`
	TaskID              string                              `json:"task_id"`
	Attempt             int32                               `json:"attempt"`
	EvidenceID          *string                             `json:"evidence_id"`
	ClaimIdentitySource string                              `json:"claim_identity_source"`
	Revision            int64                               `json:"revision"`
	Timings             TaskRunEvidenceTimings              `json:"timings"`
	Requested           TaskRunEvidenceModelConfig          `json:"requested"`
	ClientEffective     TaskRunEvidenceModelConfig          `json:"client_effective"`
	ProviderReported    TaskRunEvidenceProviderModel        `json:"provider_reported"`
	Runtime             TaskRunEvidenceRuntime              `json:"runtime"`
	Usage               TaskRunEvidenceUsage                `json:"usage"`
	ProviderCost        TaskRunEvidenceProviderCost         `json:"provider_cost"`
	ModelUsage          *TaskRunEvidenceModelUsageInventory `json:"model_usage,omitempty"`
	CreatedAt           string                              `json:"created_at"`
	UpdatedAt           string                              `json:"updated_at"`
}

type TaskRunEvidenceListResponse struct {
	SchemaVersion string                    `json:"schema_version"`
	TaskID        string                    `json:"task_id"`
	Attempts      []TaskRunEvidenceResponse `json:"attempts"`
}

func decodeTaskRunEvidenceRequest(w http.ResponseWriter, r *http.Request) (TaskRunEvidenceRequest, bool) {
	var req TaskRunEvidenceRequest
	r.Body = http.MaxBytesReader(w, r.Body, maxTaskRunEvidenceBodyBytes)
	decoder := json.NewDecoder(r.Body)
	decoder.DisallowUnknownFields()
	if err := decoder.Decode(&req); err != nil {
		writeError(w, http.StatusBadRequest, "invalid task run evidence")
		return TaskRunEvidenceRequest{}, false
	}
	if err := decoder.Decode(&struct{}{}); !errors.Is(err, io.EOF) {
		writeError(w, http.StatusBadRequest, "invalid task run evidence")
		return TaskRunEvidenceRequest{}, false
	}
	if err := validateTaskRunEvidenceRequest(req); err != nil {
		writeError(w, http.StatusBadRequest, err.Error())
		return TaskRunEvidenceRequest{}, false
	}
	return req, true
}

func validateTaskRunEvidenceRequest(req TaskRunEvidenceRequest) error {
	if req.SchemaVersion != taskRunEvidenceSchemaVersion {
		return fmt.Errorf("unsupported schema_version")
	}
	if req.Attempt <= 0 {
		return fmt.Errorf("attempt must be positive")
	}
	if req.ClaimGeneration <= 0 {
		return fmt.Errorf("claim_generation must be positive")
	}
	if req.Revision <= 0 {
		return fmt.Errorf("revision must be positive")
	}
	for name, timing := range map[string]TaskRunEvidenceTiming{
		"queue": req.Timings.Queue, "preparation": req.Timings.Preparation,
		"first_tool": req.Timings.FirstTool, "execution": req.Timings.Execution,
		"finalization": req.Timings.Finalization,
	} {
		if timing.Known != (timing.DurationMS != nil) || (timing.DurationMS != nil && *timing.DurationMS < 0) {
			return fmt.Errorf("invalid %s timing", name)
		}
	}
	for name, value := range map[string]*string{
		"requested.model": req.Requested.Model, "requested.effort": req.Requested.Effort,
		"client_effective.model": req.ClientEffective.Model, "client_effective.effort": req.ClientEffective.Effort,
		"provider_reported.model": req.ProviderReported.Model,
	} {
		limit := 255
		if strings.HasSuffix(name, ".effort") {
			limit = 64
		}
		if err := validateBoundedText(name, value, limit); err != nil {
			return err
		}
	}
	if !oneOf(req.ProviderReported.Source, "provider_event", "provider_summary", "model_usage", "missing") {
		return fmt.Errorf("invalid provider_reported.source")
	}
	if (req.ProviderReported.Source == "missing") != (req.ProviderReported.Model == nil) {
		return fmt.Errorf("provider_reported model and source disagree")
	}
	if (req.Runtime.Version == nil) != (req.Runtime.ContentSHA256 == nil) {
		return fmt.Errorf("runtime version and content_sha256 must both be known or both be null")
	}
	if req.Runtime.Version != nil {
		if err := validateBoundedText("runtime.version", req.Runtime.Version, 128); err != nil {
			return err
		}
		if strings.ContainsAny(*req.Runtime.Version, `/\`) {
			return fmt.Errorf("runtime.version must not contain a path")
		}
	}
	if req.Runtime.ContentSHA256 != nil && !taskRunEvidenceSHA256Pattern.MatchString(*req.Runtime.ContentSHA256) {
		return fmt.Errorf("invalid runtime.content_sha256")
	}
	if err := validateTaskRunEvidenceUsage(req.Usage); err != nil {
		return err
	}
	if err := validateTaskRunEvidenceProviderCost(req.ProviderCost); err != nil {
		return err
	}
	if req.ModelUsage != nil {
		return validateTaskRunEvidenceModelUsageInventory(*req.ModelUsage)
	}
	return nil
}

func validateTaskRunEvidenceModelUsageInventory(inventory TaskRunEvidenceModelUsageInventory) error {
	if inventory.Entries == nil {
		return fmt.Errorf("model_usage.entries must be an array")
	}
	if len(inventory.Entries) > maxTaskRunEvidenceModelUsageEntries {
		return fmt.Errorf("model_usage.entries exceeds maximum")
	}
	if !oneOf(inventory.Source, "model_usage", "missing") {
		return fmt.Errorf("invalid model_usage.source")
	}
	if inventory.Source == "missing" {
		if len(inventory.Entries) != 0 || inventory.Complete || inventory.Truncated {
			return fmt.Errorf("missing model_usage must be empty and incomplete")
		}
		return nil
	}
	if inventory.Complete && (inventory.Truncated || len(inventory.Entries) == 0) {
		return fmt.Errorf("complete model_usage requires retained untruncated entries")
	}
	for i, entry := range inventory.Entries {
		if !utf8.ValidString(entry.Model) || len(entry.Model) == 0 || len(entry.Model) > 255 {
			return fmt.Errorf("invalid model_usage.entries.model")
		}
		for _, r := range entry.Model {
			if unicode.IsControl(r) {
				return fmt.Errorf("invalid model_usage.entries.model")
			}
		}
		if i > 0 && inventory.Entries[i-1].Model >= entry.Model {
			return fmt.Errorf("model_usage.entries must be sorted and unique")
		}
		if err := validateTaskRunEvidenceUsage(entry.Usage); err != nil {
			return fmt.Errorf("invalid model_usage entry usage: %w", err)
		}
		if err := validateTaskRunEvidenceProviderCost(entry.ProviderCost); err != nil {
			return fmt.Errorf("invalid model_usage entry provider_cost: %w", err)
		}
		if inventory.Complete && !entry.Usage.Complete {
			return fmt.Errorf("complete model_usage requires complete usage for every entry")
		}
	}
	return nil
}

func validateTaskRunEvidenceUsage(usage TaskRunEvidenceUsage) error {
	if !oneOf(usage.Source, "provider_event", "provider_summary", "model_usage", "missing") {
		return fmt.Errorf("invalid usage.source")
	}
	values := []*int64{usage.InputUncachedTokens, usage.InputCacheReadTokens, usage.InputCacheWriteTokens, usage.OutputTokens}
	known := 0
	for _, value := range values {
		if value != nil {
			known++
			if *value < 0 {
				return fmt.Errorf("usage buckets must be non-negative")
			}
		}
	}
	if usage.Complete && known != len(values) {
		return fmt.Errorf("complete usage requires all four buckets")
	}
	if usage.Source == "missing" && (known != 0 || usage.Complete) {
		return fmt.Errorf("missing usage must keep all buckets null")
	}
	if usage.Source != "missing" && known == 0 {
		return fmt.Errorf("observed usage requires at least one bucket")
	}
	return nil
}

func validateTaskRunEvidenceProviderCost(cost TaskRunEvidenceProviderCost) error {
	if !oneOf(cost.Authority, "provider_reported", "missing") ||
		!oneOf(cost.Basis, "provider_reported", "unknown", "missing") ||
		!oneOf(cost.Source, "provider_event", "provider_summary", "model_usage", "task_usage_api", "missing") {
		return fmt.Errorf("invalid provider_cost provenance")
	}
	if cost.Authority == "missing" {
		if cost.AmountUSDTicks != nil || cost.Complete || cost.Basis != "missing" || cost.Source != "missing" {
			return fmt.Errorf("missing provider_cost must have null amount and missing provenance")
		}
		return nil
	}
	if cost.AmountUSDTicks == nil || *cost.AmountUSDTicks < 0 || cost.Source == "missing" || cost.Basis == "missing" {
		return fmt.Errorf("invalid provider-reported cost")
	}
	if cost.Basis == "unknown" && cost.Complete {
		return fmt.Errorf("unknown provider_cost basis cannot be complete")
	}
	return nil
}

func validateBoundedText(name string, value *string, max int) error {
	if value == nil {
		return nil
	}
	if len(*value) == 0 || len(*value) > max {
		return fmt.Errorf("invalid %s", name)
	}
	for _, r := range *value {
		if unicode.IsControl(r) {
			return fmt.Errorf("invalid %s", name)
		}
	}
	return nil
}

func oneOf(value string, allowed ...string) bool {
	for _, candidate := range allowed {
		if value == candidate {
			return true
		}
	}
	return false
}

// ReportTaskRunEvidence accepts evidence only from the daemon currently bound
// to the task's assigned runtime and current claim generation.
func (h *Handler) ReportTaskRunEvidence(w http.ResponseWriter, r *http.Request) {
	runtimeID := chi.URLParam(r, "runtimeId")
	taskID := chi.URLParam(r, "taskId")
	daemonID := middleware.DaemonIDFromContext(r.Context())
	if daemonID == "" {
		writeError(w, http.StatusForbidden, "daemon attestation required")
		return
	}
	workspaceID := middleware.DaemonWorkspaceIDFromContext(r.Context())
	workspaceUUID, err := parseUUIDFromContext(workspaceID)
	if err != nil {
		writeError(w, http.StatusNotFound, "workspace not found")
		return
	}
	runtimeUUID, ok := parseUUIDOrBadRequest(w, runtimeID, "runtime_id")
	if !ok {
		return
	}
	taskUUID, ok := parseUUIDOrBadRequest(w, taskID, "task_id")
	if !ok {
		return
	}

	req, ok := decodeTaskRunEvidenceRequest(w, r)
	if !ok {
		return
	}

	tx, err := h.TxStarter.Begin(r.Context())
	if err != nil {
		writeError(w, http.StatusInternalServerError, "failed to record task run evidence")
		return
	}
	defer tx.Rollback(r.Context())
	queries := h.Queries.WithTx(tx)
	if _, err := queries.LockTaskRunEvidenceWorkspace(r.Context(), workspaceUUID); err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			writeError(w, http.StatusNotFound, "workspace not found")
		} else {
			writeError(w, http.StatusInternalServerError, "failed to record task run evidence")
		}
		return
	}
	runtime, err := queries.LockTaskRunEvidenceRuntime(r.Context(), db.LockTaskRunEvidenceRuntimeParams{
		RuntimeID: runtimeUUID, WorkspaceID: workspaceUUID, DaemonID: pgtype.Text{String: daemonID, Valid: true},
	})
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			writeError(w, http.StatusNotFound, "runtime not found")
		} else {
			writeError(w, http.StatusInternalServerError, "failed to record task run evidence")
		}
		return
	}
	task, err := queries.LockTaskRunEvidenceTask(r.Context(), db.LockTaskRunEvidenceTaskParams{
		TaskID: taskUUID, RuntimeID: runtime.ID,
	})
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			writeError(w, http.StatusNotFound, "task not found")
		} else {
			writeError(w, http.StatusInternalServerError, "failed to record task run evidence")
		}
		return
	}
	taskService := &service.TaskService{Queries: queries}
	if taskService.ResolveTaskWorkspaceID(r.Context(), task) != workspaceID {
		writeError(w, http.StatusNotFound, "task not found")
		return
	}
	if req.Attempt != task.Attempt || !task.DispatchedAt.Valid || req.ClaimGeneration != task.DispatchedAt.Time.UnixMicro() {
		writeError(w, http.StatusConflict, "task claim does not match current dispatch")
		return
	}
	req.Timings.Queue = taskRunEvidenceCanonicalQueueTiming(task)
	payload, err := json.Marshal(req)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "failed to encode task run evidence")
		return
	}
	sum := sha256.Sum256(payload)
	payloadHash := "sha256:" + hex.EncodeToString(sum[:])

	row, err := queries.UpsertTaskRunEvidence(r.Context(), taskRunEvidenceParams(req, task.ID, runtime.WorkspaceID, runtime.ID, daemonID, payloadHash))
	if err != nil {
		if !errors.Is(err, pgx.ErrNoRows) {
			writeError(w, http.StatusInternalServerError, "failed to record task run evidence")
			return
		}
		existing, getErr := queries.GetTaskRunEvidence(r.Context(), db.GetTaskRunEvidenceParams{
			TaskID: task.ID, Attempt: req.Attempt, ClaimGeneration: taskRunEvidenceInt8(&req.ClaimGeneration),
		})
		if getErr != nil {
			writeError(w, http.StatusConflict, "task run evidence revision conflict")
			return
		}
		if existing.Revision == req.Revision && existing.PayloadSha256 == payloadHash &&
			existing.WorkspaceID.Bytes == runtime.WorkspaceID.Bytes && existing.RuntimeID.Bytes == runtime.ID.Bytes && existing.DaemonID == daemonID {
			if err := tx.Commit(r.Context()); err != nil {
				writeError(w, http.StatusInternalServerError, "failed to record task run evidence")
				return
			}
			writeJSON(w, http.StatusOK, taskRunEvidenceResponse(existing))
			return
		}
		writeError(w, http.StatusConflict, "task run evidence revision conflict")
		return
	}
	if err := tx.Commit(r.Context()); err != nil {
		writeError(w, http.StatusInternalServerError, "failed to record task run evidence")
		return
	}
	writeJSON(w, http.StatusOK, taskRunEvidenceResponse(row))
}

func taskRunEvidenceCanonicalQueueTiming(task db.AgentTaskQueue) TaskRunEvidenceTiming {
	if !task.QueueStartedAt.Valid || task.QueueStartedAt.InfinityModifier != pgtype.Finite || task.QueueStartedAt.Time.IsZero() ||
		!task.DispatchedAt.Valid || task.DispatchedAt.InfinityModifier != pgtype.Finite || task.DispatchedAt.Time.IsZero() ||
		task.DispatchedAt.Time.Before(task.QueueStartedAt.Time) {
		return TaskRunEvidenceTiming{}
	}
	durationMS := task.DispatchedAt.Time.Sub(task.QueueStartedAt.Time).Milliseconds()
	return TaskRunEvidenceTiming{Known: true, DurationMS: &durationMS}
}

func (h *Handler) ListTaskRunEvidenceByUser(w http.ResponseWriter, r *http.Request) {
	taskID := chi.URLParam(r, "taskId")
	taskUUID, ok := parseUUIDOrBadRequest(w, taskID, "task_id")
	if !ok {
		return
	}
	workspaceUUID, err := parseUUIDFromContext(middleware.WorkspaceIDFromContext(r.Context()))
	if err != nil {
		writeError(w, http.StatusNotFound, "task not found")
		return
	}
	if _, err := h.Queries.GetAgentTaskInWorkspace(r.Context(), db.GetAgentTaskInWorkspaceParams{ID: taskUUID, WorkspaceID: workspaceUUID}); err != nil {
		writeError(w, http.StatusNotFound, "task not found")
		return
	}
	rows, err := h.Queries.ListTaskRunEvidence(r.Context(), taskUUID)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "failed to list task run evidence")
		return
	}
	attempts := make([]TaskRunEvidenceResponse, len(rows))
	for i, row := range rows {
		attempts[i] = taskRunEvidenceResponse(row)
	}
	writeJSON(w, http.StatusOK, TaskRunEvidenceListResponse{SchemaVersion: taskRunEvidenceSchemaVersion, TaskID: taskID, Attempts: attempts})
}

func parseUUIDFromContext(value string) (pgtype.UUID, error) {
	if value == "" {
		return pgtype.UUID{}, errors.New("missing UUID")
	}
	var id pgtype.UUID
	if err := id.Scan(value); err != nil {
		return pgtype.UUID{}, err
	}
	return id, nil
}

func taskRunEvidenceInt8(value *int64) pgtype.Int8 {
	if value == nil {
		return pgtype.Int8{}
	}
	return pgtype.Int8{Int64: *value, Valid: true}
}

func taskRunEvidenceParams(req TaskRunEvidenceRequest, taskID, workspaceID, runtimeID pgtype.UUID, daemonID, payloadHash string) db.UpsertTaskRunEvidenceParams {
	return db.UpsertTaskRunEvidenceParams{
		TaskID: taskID, WorkspaceID: workspaceID, RuntimeID: runtimeID, DaemonID: daemonID,
		Attempt: req.Attempt, ClaimGeneration: taskRunEvidenceInt8(&req.ClaimGeneration), Revision: req.Revision, PayloadSha256: payloadHash,
		QueueKnown: req.Timings.Queue.Known, QueueDurationMs: taskRunEvidenceInt8(req.Timings.Queue.DurationMS),
		PreparationKnown: req.Timings.Preparation.Known, PreparationDurationMs: taskRunEvidenceInt8(req.Timings.Preparation.DurationMS),
		FirstToolKnown: req.Timings.FirstTool.Known, FirstToolDurationMs: taskRunEvidenceInt8(req.Timings.FirstTool.DurationMS),
		ExecutionKnown: req.Timings.Execution.Known, ExecutionDurationMs: taskRunEvidenceInt8(req.Timings.Execution.DurationMS),
		FinalizationKnown: req.Timings.Finalization.Known, FinalizationDurationMs: taskRunEvidenceInt8(req.Timings.Finalization.DurationMS),
		RequestedModel: ptrToText(req.Requested.Model), RequestedEffort: ptrToText(req.Requested.Effort),
		ClientEffectiveModel: ptrToText(req.ClientEffective.Model), ClientEffectiveEffort: ptrToText(req.ClientEffective.Effort),
		ProviderReportedModel: ptrToText(req.ProviderReported.Model), ProviderModelSource: req.ProviderReported.Source,
		RuntimeVersion: ptrToText(req.Runtime.Version), RuntimeContentSha256: ptrToText(req.Runtime.ContentSHA256),
		InputUncachedTokens: taskRunEvidenceInt8(req.Usage.InputUncachedTokens), InputCacheReadTokens: taskRunEvidenceInt8(req.Usage.InputCacheReadTokens),
		InputCacheWriteTokens: taskRunEvidenceInt8(req.Usage.InputCacheWriteTokens), OutputTokens: taskRunEvidenceInt8(req.Usage.OutputTokens),
		UsageComplete: req.Usage.Complete, UsageSource: req.Usage.Source,
		ProviderCostUsdTicks: taskRunEvidenceInt8(req.ProviderCost.AmountUSDTicks), ProviderCostComplete: req.ProviderCost.Complete,
		ProviderCostAuthority: req.ProviderCost.Authority, ProviderCostBasis: req.ProviderCost.Basis, ProviderCostSource: req.ProviderCost.Source,
		ModelUsage: taskRunEvidenceModelUsageJSON(req.ModelUsage),
	}
}

func taskRunEvidenceModelUsageJSON(value *TaskRunEvidenceModelUsageInventory) []byte {
	if value == nil {
		return nil
	}
	payload, err := json.Marshal(value)
	if err != nil {
		panic(fmt.Sprintf("marshal validated task run evidence model usage: %v", err))
	}
	return payload
}

func taskRunEvidenceResponse(row db.TaskRunEvidence) TaskRunEvidenceResponse {
	evidenceID, claimIdentitySource := taskRunEvidenceClaimIdentity(row)
	var modelUsage *TaskRunEvidenceModelUsageInventory
	if len(row.ModelUsage) > 0 {
		var value TaskRunEvidenceModelUsageInventory
		if err := json.Unmarshal(row.ModelUsage, &value); err == nil {
			modelUsage = &value
		}
	}
	return TaskRunEvidenceResponse{
		SchemaVersion: taskRunEvidenceSchemaVersion, TaskID: uuidToString(row.TaskID), Attempt: row.Attempt,
		EvidenceID: evidenceID, ClaimIdentitySource: claimIdentitySource, Revision: row.Revision,
		Timings: TaskRunEvidenceTimings{
			Queue:        TaskRunEvidenceTiming{Known: row.QueueKnown, DurationMS: int8ToPtr(row.QueueDurationMs)},
			Preparation:  TaskRunEvidenceTiming{Known: row.PreparationKnown, DurationMS: int8ToPtr(row.PreparationDurationMs)},
			FirstTool:    TaskRunEvidenceTiming{Known: row.FirstToolKnown, DurationMS: int8ToPtr(row.FirstToolDurationMs)},
			Execution:    TaskRunEvidenceTiming{Known: row.ExecutionKnown, DurationMS: int8ToPtr(row.ExecutionDurationMs)},
			Finalization: TaskRunEvidenceTiming{Known: row.FinalizationKnown, DurationMS: int8ToPtr(row.FinalizationDurationMs)},
		},
		Requested:        TaskRunEvidenceModelConfig{Model: textToPtr(row.RequestedModel), Effort: textToPtr(row.RequestedEffort)},
		ClientEffective:  TaskRunEvidenceModelConfig{Model: textToPtr(row.ClientEffectiveModel), Effort: textToPtr(row.ClientEffectiveEffort)},
		ProviderReported: TaskRunEvidenceProviderModel{Model: textToPtr(row.ProviderReportedModel), Source: row.ProviderModelSource},
		Runtime:          TaskRunEvidenceRuntime{Version: textToPtr(row.RuntimeVersion), ContentSHA256: textToPtr(row.RuntimeContentSha256)},
		Usage: TaskRunEvidenceUsage{
			InputUncachedTokens: int8ToPtr(row.InputUncachedTokens), InputCacheReadTokens: int8ToPtr(row.InputCacheReadTokens),
			InputCacheWriteTokens: int8ToPtr(row.InputCacheWriteTokens), OutputTokens: int8ToPtr(row.OutputTokens),
			Complete: row.UsageComplete, Source: row.UsageSource,
		},
		ProviderCost: TaskRunEvidenceProviderCost{
			AmountUSDTicks: int8ToPtr(row.ProviderCostUsdTicks), Complete: row.ProviderCostComplete,
			Authority: row.ProviderCostAuthority, Basis: row.ProviderCostBasis, Source: row.ProviderCostSource,
		},
		ModelUsage: modelUsage,
		CreatedAt:  timestampToString(row.CreatedAt), UpdatedAt: timestampToString(row.UpdatedAt),
	}
}

func taskRunEvidenceClaimIdentity(row db.TaskRunEvidence) (*string, string) {
	if !row.ClaimGeneration.Valid {
		return nil, "missing"
	}
	sum := sha256.Sum256([]byte(fmt.Sprintf("%s:%d:%d", uuidToString(row.TaskID), row.Attempt, row.ClaimGeneration.Int64)))
	id := "sha256:" + hex.EncodeToString(sum[:])
	return &id, "claim_generation"
}
