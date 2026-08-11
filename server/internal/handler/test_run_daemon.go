package handler

import (
	"context"
	"encoding/json"
	"strings"
	"time"

	"github.com/jackc/pgx/v5/pgtype"
	"github.com/multica-ai/multica/server/internal/service"
	"github.com/multica-ai/multica/server/internal/util"
	db "github.com/multica-ai/multica/server/pkg/db/generated"
)

// testRunResultMarker prefixes the agent's closing summary. Per-case results
// are written through the authenticated CLI, so a missing marker costs a status
// line and never a result.
const testRunResultMarker = "TEST_RUN_RESULT_JSON:"

type testRunResultSummary struct {
	Status   string   `json:"status"`
	Summary  string   `json:"summary"`
	Blockers []string `json:"blockers"`
}

func parseTestRunResultSummary(output string) (testRunResultSummary, bool) {
	idx := strings.LastIndex(output, testRunResultMarker)
	if idx < 0 {
		return testRunResultSummary{}, false
	}
	tail := strings.TrimSpace(output[idx+len(testRunResultMarker):])
	tail = strings.TrimPrefix(tail, "```json")
	tail = strings.TrimPrefix(tail, "```")
	if fence := strings.Index(tail, "```"); fence >= 0 {
		tail = tail[:fence]
	}
	var summary testRunResultSummary
	if err := json.Unmarshal([]byte(strings.TrimSpace(tail)), &summary); err != nil {
		return testRunResultSummary{}, false
	}
	return summary, true
}

func (h *Handler) testRunForTask(ctx context.Context, task db.AgentTaskQueue) (db.TestRun, bool, error) {
	var runCtx service.TestRunContext
	if err := json.Unmarshal(task.Context, &runCtx); err != nil || runCtx.Type != service.TestRunContextType {
		return db.TestRun{}, false, nil
	}
	wsUUID, err := util.ParseUUID(runCtx.WorkspaceID)
	if err != nil {
		return db.TestRun{}, false, err
	}
	run, err := h.Queries.GetTestRunByAgentTask(ctx, db.GetTestRunByAgentTaskParams{
		AgentTaskID: task.ID,
		WorkspaceID: wsUUID,
	})
	if err != nil {
		return db.TestRun{}, false, err
	}
	return run, true, nil
}

func (h *Handler) markTestRunRunning(ctx context.Context, task db.AgentTaskQueue) error {
	run, isRun, err := h.testRunForTask(ctx, task)
	if !isRun {
		return err
	}
	_, err = h.Queries.UpdateTestRun(ctx, db.UpdateTestRunParams{
		ID:          run.ID,
		WorkspaceID: run.WorkspaceID,
		Status:      pgtype.Text{String: "running", Valid: true},
		StartedAt:   pgtype.Timestamptz{Time: time.Now(), Valid: true},
	})
	return err
}

// updateTestRunFromAgentCompletion closes the round. The per-case results were
// already written through the CLI, so the run's own status only reflects
// whether the agent got through its work — a round where every case legitimately
// failed is still a completed round.
func (h *Handler) updateTestRunFromAgentCompletion(ctx context.Context, task db.AgentTaskQueue, req TaskCompleteRequest) error {
	run, isRun, err := h.testRunForTask(ctx, task)
	if !isRun {
		return err
	}
	summary, _ := parseTestRunResultSummary(req.Output)
	status := "completed"
	// Only the agent's declared status decides this; scanning the transcript for
	// the word "blocked" is the design_restore mistake.
	if summary.Status == "blocked" {
		status = "blocked"
	}
	params := db.UpdateTestRunParams{
		ID:          run.ID,
		WorkspaceID: run.WorkspaceID,
		Status:      pgtype.Text{String: status, Valid: true},
		CompletedAt: pgtype.Timestamptz{Time: time.Now(), Valid: true},
	}
	if len(summary.Blockers) > 0 {
		params.Error = pgtype.Text{String: strings.Join(summary.Blockers, "; "), Valid: true}
	}
	_, err = h.Queries.UpdateTestRun(ctx, params)
	return err
}

func (h *Handler) updateTestRunFromAgentFailure(ctx context.Context, task db.AgentTaskQueue, req TaskFailRequest) error {
	run, isRun, err := h.testRunForTask(ctx, task)
	if !isRun {
		return err
	}
	_, err = h.Queries.UpdateTestRun(ctx, db.UpdateTestRunParams{
		ID:          run.ID,
		WorkspaceID: run.WorkspaceID,
		Status:      pgtype.Text{String: "aborted", Valid: true},
		CompletedAt: pgtype.Timestamptz{Time: time.Now(), Valid: true},
		Error:       pgtype.Text{String: req.Error, Valid: req.Error != ""},
	})
	return err
}
