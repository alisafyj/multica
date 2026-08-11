package handler

import (
	"context"
	"encoding/json"
	"strings"

	"github.com/jackc/pgx/v5/pgtype"
	"github.com/multica-ai/multica/server/internal/service"
	"github.com/multica-ai/multica/server/internal/util"
	db "github.com/multica-ai/multica/server/pkg/db/generated"
)

// testGenerationResultMarker prefixes the agent's closing summary block. Only
// the summary travels this way: the generated cases themselves are written
// through the authenticated propose endpoint, so a malformed or missing marker
// costs a status line, never data.
const testGenerationResultMarker = "TEST_GENERATION_RESULT_JSON:"

// testGenerationResultSummary is the agent's declared outcome.
type testGenerationResultSummary struct {
	Status   string         `json:"status"`
	Summary  string         `json:"summary"`
	Stats    map[string]int `json:"stats"`
	Blockers []string       `json:"blockers"`
}

// parseTestGenerationResultSummary scrapes the last marker block out of the
// agent's final message, tolerating a fenced code block around it.
func parseTestGenerationResultSummary(output string) (testGenerationResultSummary, bool) {
	idx := strings.LastIndex(output, testGenerationResultMarker)
	if idx < 0 {
		return testGenerationResultSummary{}, false
	}
	tail := strings.TrimSpace(output[idx+len(testGenerationResultMarker):])
	tail = strings.TrimPrefix(tail, "```json")
	tail = strings.TrimPrefix(tail, "```")
	if fence := strings.Index(tail, "```"); fence >= 0 {
		tail = tail[:fence]
	}
	tail = strings.TrimSpace(tail)

	var summary testGenerationResultSummary
	if err := json.Unmarshal([]byte(tail), &summary); err != nil {
		return testGenerationResultSummary{}, false
	}
	return summary, true
}

// testGenerationJobForTask resolves the domain row behind an agent task, or
// reports that this task is not a generation run.
func (h *Handler) testGenerationJobForTask(ctx context.Context, task db.AgentTaskQueue) (db.TestGenerationJob, bool, error) {
	var genCtx service.TestGenerationContext
	if err := json.Unmarshal(task.Context, &genCtx); err != nil || genCtx.Type != service.TestGenerationContextType {
		return db.TestGenerationJob{}, false, nil
	}
	wsUUID, err := util.ParseUUID(genCtx.WorkspaceID)
	if err != nil {
		return db.TestGenerationJob{}, false, err
	}
	job, err := h.Queries.GetTestGenerationJobByAgentTask(ctx, db.GetTestGenerationJobByAgentTaskParams{
		AgentTaskID: task.ID,
		WorkspaceID: wsUUID,
	})
	if err != nil {
		return db.TestGenerationJob{}, false, err
	}
	return job, true, nil
}

func (h *Handler) markTestGenerationJobRunning(ctx context.Context, task db.AgentTaskQueue) error {
	job, isGeneration, err := h.testGenerationJobForTask(ctx, task)
	if !isGeneration {
		return err
	}
	_, err = h.Queries.UpdateTestGenerationJob(ctx, db.UpdateTestGenerationJobParams{
		ID:          job.ID,
		WorkspaceID: job.WorkspaceID,
		Status:      pgtype.Text{String: "running", Valid: true},
	})
	return err
}

func (h *Handler) updateTestGenerationJobFromAgentCompletion(ctx context.Context, task db.AgentTaskQueue, req TaskCompleteRequest) error {
	job, isGeneration, err := h.testGenerationJobForTask(ctx, task)
	if !isGeneration {
		return err
	}

	summary, hasSummary := parseTestGenerationResultSummary(req.Output)
	// Only the agent's declared status decides the outcome. design_restore also
	// scans the raw output for the substring "blocked", which fails a run whose
	// narration merely says "nothing was blocked".
	status := "completed"
	switch summary.Status {
	case "blocked", "failed":
		status = "failed"
	}

	result := unmarshalJSONObject(job.Result)
	result["output"] = req.Output
	result["session_id"] = req.SessionID
	result["work_dir"] = req.WorkDir
	if hasSummary {
		result["summary"] = summary.Summary
		result["blockers"] = summary.Blockers
		if len(summary.Stats) > 0 {
			result["declared_stats"] = summary.Stats
		}
	} else {
		// Cases may still have landed through propose; say so rather than
		// implying the run produced nothing.
		result["warning"] = "missing_test_generation_result_json"
	}

	_, err = h.Queries.UpdateTestGenerationJob(ctx, db.UpdateTestGenerationJobParams{
		ID:          job.ID,
		WorkspaceID: job.WorkspaceID,
		Status:      pgtype.Text{String: status, Valid: true},
		Result:      marshalJSONColumn(result, "{}"),
	})
	return err
}

func (h *Handler) updateTestGenerationJobFromAgentFailure(ctx context.Context, task db.AgentTaskQueue, req TaskFailRequest) error {
	job, isGeneration, err := h.testGenerationJobForTask(ctx, task)
	if !isGeneration {
		return err
	}
	result := unmarshalJSONObject(job.Result)
	result["session_id"] = req.SessionID
	result["work_dir"] = req.WorkDir

	_, err = h.Queries.UpdateTestGenerationJob(ctx, db.UpdateTestGenerationJobParams{
		ID:          job.ID,
		WorkspaceID: job.WorkspaceID,
		Status:      pgtype.Text{String: "failed", Valid: true},
		Result:      marshalJSONColumn(result, "{}"),
		Error:       pgtype.Text{String: req.Error, Valid: req.Error != ""},
	})
	return err
}
