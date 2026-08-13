package handler

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"strings"

	"github.com/go-chi/chi/v5"
	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgtype"
	"github.com/multica-ai/multica/server/internal/opendesign"
	"github.com/multica-ai/multica/server/internal/service"
	"github.com/multica-ai/multica/server/internal/util"
	db "github.com/multica-ai/multica/server/pkg/db/generated"
)

const openDesignPreflightRequestMaxBytes int64 = 64 << 10

type preparedOpenDesignRun struct {
	ID        pgtype.UUID
	Identity  opendesign.EngineIdentity
	Agent     opendesign.AgentIdentity
	Context   json.RawMessage
	Operation service.ProjectDesignSystemOperation
}

func (h *Handler) prepareOpenDesignRun(
	ctx context.Context,
	queries *db.Queries,
	agent db.Agent,
	operation service.ProjectDesignSystemOperation,
) (*preparedOpenDesignRun, error) {
	if !h.cfg.OpenDesignEnabled || operation == service.ProjectDesignSystemRepositoryAnalysis {
		return nil, nil
	}
	runtime, err := queries.GetAgentRuntime(ctx, agent.RuntimeID)
	if err != nil {
		return nil, projectDesignSystemInternalError("agent_runtime_lookup_failed", "failed to load agent runtime")
	}
	adapterID, ok := opendesign.ResolveAdapter(runtime.Provider)
	if !ok {
		return nil, &projectDesignSystemRequestError{
			status:  http.StatusConflict,
			code:    "open_design_agent_unsupported",
			message: fmt.Sprintf("agent runtime %q is not supported by the pinned Open Design worker", runtime.Provider),
		}
	}
	identity := opendesign.PinnedEngineIdentity()
	if err := identity.Validate(); err != nil {
		return nil, projectDesignSystemInternalError("open_design_engine_invalid", "pinned Open Design engine identity is invalid")
	}
	runUUID, err := uuid.NewV7()
	if err != nil {
		return nil, projectDesignSystemInternalError("open_design_run_id_failed", "failed to allocate Open Design run")
	}
	runID, err := util.ParseUUID(runUUID.String())
	if err != nil {
		return nil, projectDesignSystemInternalError("open_design_run_id_failed", "failed to allocate Open Design run")
	}
	agentIdentity := opendesign.AgentIdentity{
		MulticaAgentID: util.UUIDToString(agent.ID),
		AdapterID:      adapterID,
	}
	if agent.Model.Valid {
		agentIdentity.Model = strings.TrimSpace(agent.Model.String)
	}
	contextJSON, err := json.Marshal(opendesign.TaskRunContext{
		Schema: opendesign.RunSchema,
		RunID:  runUUID.String(),
		Engine: identity,
		Agent:  agentIdentity,
	})
	if err != nil {
		return nil, projectDesignSystemInternalError("open_design_context_failed", "failed to build Open Design run context")
	}
	return &preparedOpenDesignRun{
		ID:        runID,
		Identity:  identity,
		Agent:     agentIdentity,
		Context:   contextJSON,
		Operation: operation,
	}, nil
}

func persistOpenDesignRun(
	ctx context.Context,
	queries *db.Queries,
	prepared *preparedOpenDesignRun,
	system db.ProjectDesignSystem,
	task db.AgentTaskQueue,
	inputSnapshot json.RawMessage,
) error {
	if prepared == nil {
		return nil
	}
	agentSnapshot, err := json.Marshal(prepared.Agent)
	if err != nil {
		return fmt.Errorf("marshal Open Design agent snapshot: %w", err)
	}
	provenance, err := json.Marshal(map[string]any{
		"kind":      "orchestrator-scratch",
		"writeback": "external",
	})
	if err != nil {
		return fmt.Errorf("marshal Open Design workspace provenance: %w", err)
	}
	model := pgtype.Text{}
	if prepared.Agent.Model != "" {
		model = pgtype.Text{String: prepared.Agent.Model, Valid: true}
	}
	_, err = queries.CreateOpenDesignRun(ctx, db.CreateOpenDesignRunParams{
		ID:                   prepared.ID,
		WorkspaceID:          system.WorkspaceID,
		ProjectID:            system.ProjectID,
		DesignSystemID:       system.ID,
		TaskID:               task.ID,
		Operation:            string(prepared.Operation),
		EngineRelease:        prepared.Identity.Release,
		EngineCommit:         prepared.Identity.Commit,
		EngineLockfileSha256: prepared.Identity.LockfileSHA256,
		EngineDistSha256:     prepared.Identity.DistSHA256,
		AgentID:              task.AgentID,
		AgentSnapshot:        agentSnapshot,
		AdapterID:            prepared.Agent.AdapterID,
		Model:                model,
		InputSnapshot:        inputSnapshot,
		WorkspaceProvenance:  provenance,
	})
	return err
}

func (h *Handler) RecordOpenDesignRunPreflight(w http.ResponseWriter, r *http.Request) {
	taskID := chi.URLParam(r, "taskId")
	task, _, ok := h.requireDaemonTaskAccessWithWorkspace(w, r, taskID)
	if !ok {
		return
	}
	var taskContext service.ProjectDesignSystemTaskContext
	if err := json.Unmarshal(task.Context, &taskContext); err != nil || len(taskContext.OpenDesignRun) == 0 {
		writeProjectDesignSystemError(w, http.StatusNotFound, "open_design_run_not_found", "Open Design run not found")
		return
	}
	var expectedContext opendesign.TaskRunContext
	if err := json.Unmarshal(taskContext.OpenDesignRun, &expectedContext); err != nil {
		writeProjectDesignSystemError(w, http.StatusConflict, "open_design_context_invalid", "Open Design run context is invalid")
		return
	}
	run, err := h.Queries.GetOpenDesignRunByTask(r.Context(), task.ID)
	if errors.Is(err, pgx.ErrNoRows) {
		writeProjectDesignSystemError(w, http.StatusNotFound, "open_design_run_not_found", "Open Design run not found")
		return
	}
	if err != nil {
		writeProjectDesignSystemError(w, http.StatusInternalServerError, "open_design_run_lookup_failed", "failed to load Open Design run")
		return
	}
	if expectedContext.RunID != util.UUIDToString(run.ID) {
		writeProjectDesignSystemError(w, http.StatusConflict, "open_design_run_mismatch", "Open Design run does not match task")
		return
	}

	r.Body = http.MaxBytesReader(w, r.Body, openDesignPreflightRequestMaxBytes)
	var report opendesign.PreflightReport
	if err := json.NewDecoder(r.Body).Decode(&report); err != nil {
		writeProjectDesignSystemError(w, http.StatusBadRequest, "open_design_preflight_invalid", "invalid Open Design preflight report")
		return
	}
	reportJSON, err := json.Marshal(report)
	if err != nil {
		writeProjectDesignSystemError(w, http.StatusBadRequest, "open_design_preflight_invalid", "invalid Open Design preflight report")
		return
	}
	expected := opendesign.ExpectedPreflight{
		Engine: opendesign.EngineIdentity{
			Schema:         opendesign.EngineIdentitySchema,
			Release:        run.EngineRelease,
			Commit:         run.EngineCommit,
			LockfileSHA256: run.EngineLockfileSha256,
			DistSHA256:     run.EngineDistSha256,
		},
		AdapterID: run.AdapterID,
	}
	if run.Model.Valid {
		expected.Model = run.Model.String
	}
	validationErr := opendesign.ValidatePreflight(expected, report)
	status := "ready"
	failureJSON := json.RawMessage(`{}`)
	if validationErr != nil {
		status = "preflight_failed"
		failureJSON, _ = json.Marshal(map[string]string{
			"code":    "open_design_preflight_failed",
			"message": validationErr.Error(),
		})
	}
	updated, err := h.Queries.RecordOpenDesignRunPreflight(r.Context(), db.RecordOpenDesignRunPreflightParams{
		Status:    status,
		Preflight: reportJSON,
		Failure:   failureJSON,
		TaskID:    task.ID,
	})
	if errors.Is(err, pgx.ErrNoRows) {
		if run.Status == "ready" && validationErr == nil && json.Valid(run.Preflight) {
			writeJSON(w, http.StatusOK, map[string]string{"id": util.UUIDToString(run.ID), "status": run.Status})
			return
		}
		writeProjectDesignSystemError(w, http.StatusConflict, "open_design_preflight_finalized", "Open Design preflight is already finalized")
		return
	}
	if err != nil {
		writeProjectDesignSystemError(w, http.StatusInternalServerError, "open_design_preflight_persist_failed", "failed to persist Open Design preflight")
		return
	}
	if validationErr != nil {
		writeProjectDesignSystemError(w, http.StatusUnprocessableEntity, "open_design_preflight_failed", validationErr.Error())
		return
	}
	writeJSON(w, http.StatusOK, map[string]string{"id": util.UUIDToString(updated.ID), "status": updated.Status})
}
