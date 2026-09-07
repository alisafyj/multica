package handler

import (
	"context"
	"encoding/json"
	"errors"
	"io"
	"log/slog"
	"net/http"
	"strings"

	"github.com/go-chi/chi/v5"
	"github.com/jackc/pgx/v5/pgtype"
	"github.com/multica-ai/multica/server/internal/middleware"
	db "github.com/multica-ai/multica/server/pkg/db/generated"
	"github.com/multica-ai/multica/server/pkg/taskexecution"
)

// ReportTaskExecution accepts daemon observations, never agent-authored task
// metadata. Identity comes from the authenticated task/runtime, not the body.
func (h *Handler) ReportTaskExecution(w http.ResponseWriter, r *http.Request) {
	// DaemonAuth does not accept mat_ tokens. Keep the boundary fail-closed
	// even if this handler is accidentally mounted under ordinary Auth later.
	if r.Header.Get("X-Actor-Source") == "task_token" || strings.HasPrefix(strings.TrimPrefix(r.Header.Get("Authorization"), "Bearer "), "mat_") {
		writeError(w, http.StatusForbidden, "daemon credentials required")
		return
	}
	task, workspaceID, ok := h.requireDaemonTaskAccessWithWorkspace(w, r, chi.URLParam(r, "taskId"))
	if !ok {
		return
	}
	runtime, ok := h.requireDaemonRuntimeAccess(w, r, uuidToString(task.RuntimeID))
	if !ok {
		return
	}
	if uuidToString(runtime.WorkspaceID) != workspaceID {
		writeError(w, http.StatusNotFound, "task not found")
		return
	}
	if daemonID := middleware.DaemonIDFromContext(r.Context()); daemonID != "" {
		if !runtime.DaemonID.Valid || runtime.DaemonID.String != daemonID {
			writeError(w, http.StatusForbidden, "task runtime belongs to another daemon")
			return
		}
	} else if userID := requestUserID(r); userID == "" || !runtime.OwnerID.Valid || uuidToString(runtime.OwnerID) != userID {
		writeError(w, http.StatusForbidden, "task runtime belongs to another user")
		return
	}

	body, err := io.ReadAll(http.MaxBytesReader(w, r.Body, taskexecution.MaxPayloadBytes))
	if err != nil {
		var tooLarge *http.MaxBytesError
		if errors.As(err, &tooLarge) {
			writeError(w, http.StatusRequestEntityTooLarge, "execution snapshot is too large")
		} else {
			writeError(w, http.StatusBadRequest, "invalid execution snapshot")
		}
		return
	}
	snapshot, err := taskexecution.Decode(body)
	if err != nil {
		writeError(w, http.StatusBadRequest, "invalid execution snapshot: "+err.Error())
		return
	}
	if snapshot.ConciseMode != task.ConciseMode || snapshot.Provider != runtime.Provider {
		writeError(w, http.StatusBadRequest, "execution metadata does not match task")
		return
	}
	if previous, err := taskexecution.Decode(task.ExecutionMetrics); err == nil && !snapshot.CanReplace(*previous) {
		// A late retry is harmless and must not overwrite newer observations.
		writeJSON(w, http.StatusOK, map[string]string{"status": "ignored_stale"})
		return
	}
	canonical, err := json.Marshal(snapshot)
	if err != nil {
		writeError(w, http.StatusBadRequest, "invalid execution snapshot")
		return
	}
	updated, err := h.Queries.UpdateTaskExecutionMetrics(r.Context(), db.UpdateTaskExecutionMetricsParams{
		TaskID:           task.ID,
		ExecutionMetrics: canonical,
		PreviousMetrics:  task.ExecutionMetrics,
	})
	if err != nil {
		slog.Warn("update task execution metrics failed", "task_id", uuidToString(task.ID), "error", err)
		writeError(w, http.StatusInternalServerError, "failed to save execution snapshot")
		return
	}
	if updated == 0 {
		writeError(w, http.StatusConflict, "execution snapshot changed concurrently")
		return
	}
	writeJSON(w, http.StatusOK, map[string]string{"status": "ok"})
}

func executionMetricsFromJSON(data []byte) *taskexecution.Snapshot {
	snapshot, err := taskexecution.Decode(data)
	if err != nil {
		return nil
	}
	return snapshot
}

// Agent history uses the same per-task/provider/model grain as issue history.
// No row means unknown usage, not zero; one query avoids per-task lookups.
func (h *Handler) hydrateAgentTaskUsage(ctx context.Context, agentID pgtype.UUID, resp []AgentTaskResponse) {
	if len(resp) == 0 {
		return
	}
	rows, err := h.Queries.ListAgentTaskUsage(ctx, agentID)
	if err != nil {
		slog.Warn("list agent task usage failed", "agent_id", uuidToString(agentID), "error", err)
		return
	}
	byTask := make(map[string][]TaskUsageData, len(resp))
	for _, row := range rows {
		var cost *int64
		if row.CostUsdTicks.Valid {
			value := row.CostUsdTicks.Int64
			cost = &value
		}
		taskID := uuidToString(row.TaskID)
		byTask[taskID] = append(byTask[taskID], TaskUsageData{
			Provider:         row.Provider,
			Model:            row.Model,
			InputTokens:      row.InputTokens,
			OutputTokens:     row.OutputTokens,
			CacheReadTokens:  row.CacheReadTokens,
			CacheWriteTokens: row.CacheWriteTokens,
			CostUsdTicks:     cost,
		})
	}
	for i := range resp {
		resp[i].Usage = byTask[resp[i].ID]
	}
}
