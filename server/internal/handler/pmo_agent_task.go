package handler

import (
	"context"
	"errors"
	"log/slog"
	"unicode/utf8"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgtype"
	"github.com/multica-ai/multica/server/internal/service"
	"github.com/multica-ai/multica/server/internal/util"
	db "github.com/multica-ai/multica/server/pkg/db/generated"
)

// maxPMORunErrorBytes bounds the error text persisted on a failed
// pmo_sync_run row. Genuine failure reasons are short; an unbounded agent
// payload must never reach this column (see boundPMORunError callers).
const maxPMORunErrorBytes = 200

// boundPMORunError truncates a failure/error message to the run row's byte
// bound, trimming any partial trailing rune the cut leaves behind. Every
// caller passes ONLY validation or transport error text — never agent output,
// snapshot content, or external identities — because the stored value is
// surfaced to workspace members.
func boundPMORunError(msg string) string {
	if len(msg) > maxPMORunErrorBytes {
		msg = msg[:maxPMORunErrorBytes]
		for len(msg) > 0 && !utf8.ValidString(msg) {
			msg = msg[:len(msg)-1]
		}
	}
	return msg
}

// failPMOSyncRunForTask marks the run owning a failed PMO sync agent task as
// failed with the given error code. Redacted + bounded: errorMsg must be
// validation / transport error text only, never the agent payload. A run
// already in a terminal state is a no-op (ErrNoRows from the guarded UPDATE).
func (h *Handler) failPMOSyncRunForTask(ctx context.Context, pmoCtx service.PMOSyncContext, errorCode, errorMsg string) {
	runID, err := util.ParseUUID(pmoCtx.RunID)
	if err != nil {
		slog.Warn("pmo sync failure: invalid run id", "error", err)
		return
	}
	workspaceID, err := util.ParseUUID(pmoCtx.WorkspaceID)
	if err != nil {
		slog.Warn("pmo sync failure: invalid workspace id", "error", err)
		return
	}
	if _, err := h.Queries.FailPMOSyncRun(ctx, db.FailPMOSyncRunParams{
		ID:           runID,
		WorkspaceID:  workspaceID,
		ErrorCode:    pgtype.Text{String: errorCode, Valid: true},
		ErrorMessage: pgtype.Text{String: boundPMORunError(errorMsg), Valid: true},
	}); err != nil && !errors.Is(err, pgx.ErrNoRows) {
		slog.Warn("pmo sync failure: mark run failed",
			"run_id", pmoCtx.RunID, "error_code", errorCode, "error", err)
	}
}

// markPMOSyncRunRunning flips the run owning this agent task queued →
// running when the daemon reports task start. Non-PMO contexts and runs that
// are not queued are no-ops.
func (h *Handler) markPMOSyncRunRunning(ctx context.Context, task db.AgentTaskQueue) {
	pmoCtx, ok := service.ParsePMOSyncContext(task.Context)
	if !ok {
		return
	}
	runID, err := util.ParseUUID(pmoCtx.RunID)
	if err != nil {
		return
	}
	workspaceID, err := util.ParseUUID(pmoCtx.WorkspaceID)
	if err != nil {
		return
	}
	if _, err := h.Queries.MarkPMOSyncRunRunning(ctx, db.MarkPMOSyncRunRunningParams{
		ID: runID, WorkspaceID: workspaceID,
	}); err != nil && !errors.Is(err, pgx.ErrNoRows) {
		slog.Warn("pmo sync start: mark run running failed",
			"task_id", uuidToString(task.ID), "run_id", pmoCtx.RunID, "error", err)
	}
}

// storePMOSyncRunPreview persists a validated snapshot as the run's preview:
// normalized source snapshot + diff + summary, status → preview_ready,
// completed_at stamped. The diff runs against empty local state: Task 5
// stores the preview only and never touches projects or issues — the
// three-way apply against canonical entities lands in Task 6.
func (h *Handler) storePMOSyncRunPreview(ctx context.Context, qtx *db.Queries, pmoCtx service.PMOSyncContext, snapshot service.PMOSnapshot) error {
	sourceSnapshot, diffJSON, summaryJSON, err := service.PreparePMOSyncRunPreview(snapshot)
	if err != nil {
		return err
	}
	runID, err := util.ParseUUID(pmoCtx.RunID)
	if err != nil {
		return err
	}
	workspaceID, err := util.ParseUUID(pmoCtx.WorkspaceID)
	if err != nil {
		return err
	}
	if _, err := qtx.StorePMOSyncRunPreview(ctx, db.StorePMOSyncRunPreviewParams{
		ID:             runID,
		WorkspaceID:    workspaceID,
		SourceSnapshot: sourceSnapshot,
		Diff:           diffJSON,
		Summary:        summaryJSON,
	}); err != nil {
		// A repeated completion (daemon retry after a committed store) finds
		// the run already preview_ready/applied; the guarded UPDATE returns
		// ErrNoRows. Treat it as idempotent success — the preview is already
		// stored and the task transaction must not roll back.
		if errors.Is(err, pgx.ErrNoRows) {
			return nil
		}
		return err
	}
	return nil
}
