package daemon

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"net/http"
	"regexp"
	"time"

	"github.com/multica-ai/multica/server/pkg/taskexecution"
)

// Only the documented fork release spelling establishes a community base.
// Development/commit-only builds must not guess a base from their version.
var executionSSORelease = regexp.MustCompile(`^(?:v)?(0\.[0-9]+\.[0-9]+)-sso\.[0-9]+$`)

func executionCommunityBase(version string) string {
	if match := executionSSORelease.FindStringSubmatch(version); match != nil {
		return "v" + match[1]
	}
	return ""
}

func (c *Client) ReportTaskExecution(ctx context.Context, taskID string, snapshot taskexecution.Snapshot) error {
	return c.postJSONWithRetry(ctx, fmt.Sprintf("/api/daemon/tasks/%s/execution", taskID), snapshot, nil, defaultTerminalRetrySchedule)
}

type taskExecutionContextKey struct{}

// A tracker is owned by handleTask/runTask's goroutine. Its bounded worker only
// receives immutable snapshots, preserving order without putting network time
// inside phase transitions. finish drains reports even after task cancellation.
// Open Design's separate supervisor does not use these provider-run boundaries.
type taskExecution struct {
	client     *Client
	ctx        context.Context
	taskID     string
	log        *slog.Logger
	snapshot   taskexecution.Snapshot
	phaseStart time.Time
	reports    chan taskexecution.Snapshot
	done       chan struct{}
}

func (d *Daemon) newTaskExecution(ctx context.Context, task Task, provider string, log *slog.Logger) *taskExecution {
	model := d.agents()[provider].Model
	if task.Agent != nil && task.Agent.Model != "" {
		model = task.Agent.Model
	}
	return &taskExecution{
		client: d.client, ctx: ctx, taskID: task.ID, log: log,
		snapshot: taskexecution.Snapshot{
			SchemaVersion:        1,
			Provider:             provider,
			RequestedModel:       model,
			DaemonVersion:        d.cfg.CLIVersion,
			DaemonCommit:         d.cfg.CLICommit,
			CommunityBaseVersion: executionCommunityBase(d.cfg.CLIVersion),
			// Keep the daemon-wide direct-mode dimension independent of concise.
			// taskUsesDirectAgentMode ORs them for execution, not for comparison.
			DirectAgentMode: d.cfg.DirectAgentMode,
			ConciseMode:     task.ConciseMode,
		},
	}
}

func taskExecutionFromContext(ctx context.Context) *taskExecution {
	execution, _ := ctx.Value(taskExecutionContextKey{}).(*taskExecution)
	return execution
}

func (e *taskExecution) start() {
	if e == nil || len(e.snapshot.Phases) != 0 {
		return
	}
	now := time.Now()
	e.phaseStart = now
	e.snapshot.StartedAt = now.UTC()
	e.snapshot.Phases = []taskexecution.Phase{{Name: taskexecution.Prepare, StartedAt: now.UTC(), Status: taskexecution.Running}}
	e.reports = make(chan taskexecution.Snapshot, 4)
	e.done = make(chan struct{})
	go e.reportSnapshots()
	e.publish()
}

func (e *taskExecution) beginExecute() {
	if e == nil || len(e.snapshot.Phases) == 0 {
		return
	}
	e.transition(taskexecution.Execute, taskexecution.Completed)
}

func (e *taskExecution) beginFinalize(status string) {
	if e == nil || len(e.snapshot.Phases) == 0 || e.snapshot.Phases[len(e.snapshot.Phases)-1].Name == taskexecution.Finalize {
		return
	}
	e.transition(taskexecution.Finalize, status)
}

func (e *taskExecution) transition(name, previousStatus string) {
	now := time.Now()
	e.closePhase(now, previousStatus)
	e.snapshot.Phases = append(e.snapshot.Phases, taskexecution.Phase{Name: name, StartedAt: now.UTC(), Status: taskexecution.Running})
	e.phaseStart = now
	e.publish()
}

func (e *taskExecution) closePhase(now time.Time, status string) {
	phase := &e.snapshot.Phases[len(e.snapshot.Phases)-1]
	phase.DurationMS = max(0, now.Sub(e.phaseStart).Milliseconds())
	phase.Status = status
}

func (e *taskExecution) finish(status string) {
	if e == nil || len(e.snapshot.Phases) == 0 {
		return // No actual preparation occurred; do not invent successful phases.
	}
	now := time.Now()
	e.closePhase(now, status)
	finishedAt := now.UTC()
	e.snapshot.FinishedAt = &finishedAt
	e.publish()
	close(e.reports)
	<-e.done
}

func (e *taskExecution) observeToolCalls(count int32) {
	if e != nil {
		observed := int64(count)
		e.snapshot.ToolCalls = &observed
	}
}

func (e *taskExecution) publish() {
	snapshot := e.snapshot
	snapshot.Phases = append([]taskexecution.Phase(nil), snapshot.Phases...)
	e.reports <- snapshot
}

func (e *taskExecution) reportSnapshots() {
	defer close(e.done)
	unsupported := false
	for snapshot := range e.reports {
		if unsupported {
			continue
		}
		// Task/poll cancellation must not discard the measurements it produced.
		ctx, cancel := context.WithTimeout(context.WithoutCancel(e.ctx), 5*time.Second)
		err := e.client.ReportTaskExecution(ctx, e.taskID, snapshot)
		cancel()
		var requestErr *requestError
		if errors.As(err, &requestErr) && requestErr.StatusCode == http.StatusNotFound {
			unsupported = true
			e.log.Debug("execution telemetry unavailable on server", "error", err)
		} else if err != nil {
			e.log.Warn("report task execution metrics failed", "error", err)
		}
	}
}

func taskExecutionOutcome(ctx context.Context, result TaskResult, err error) string {
	if errors.Is(context.Cause(ctx), errAuthenticationExpired) {
		return taskexecution.Failed
	}
	if errors.Is(ctx.Err(), context.Canceled) || errors.Is(err, context.Canceled) || result.Status == "cancelled" {
		return taskexecution.Cancelled
	}
	if err != nil || (result.Status != "completed" && result.Status != "") {
		return taskexecution.Failed
	}
	return taskexecution.Completed
}
