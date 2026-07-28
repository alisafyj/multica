package handler

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"strings"

	"github.com/jackc/pgx/v5/pgtype"
	"github.com/multica-ai/multica/server/internal/projectdesignsystem"
	"github.com/multica-ai/multica/server/internal/service"
	"github.com/multica-ai/multica/server/internal/util"
	db "github.com/multica-ai/multica/server/pkg/db/generated"
)

type ProjectDesignSystemArtifacts struct {
	DesignMD       string `json:"design_md"`
	TokensCSS      string `json:"tokens_css"`
	ComponentsHTML string `json:"components_html"`
}

type preparedProjectDesignSystemCompletion struct {
	TaskContext service.ProjectDesignSystemTaskContext
	WorkspaceID pgtype.UUID
	ProjectID   pgtype.UUID
	SystemID    pgtype.UUID
	AgentID     pgtype.UUID
	Package     projectdesignsystem.ValidatedPackage
}

func isProjectDesignSystemTaskContext(task db.AgentTaskQueue) bool {
	if task.IssueID.Valid || task.ChatSessionID.Valid || task.AutopilotRunID.Valid || len(task.Context) == 0 {
		return false
	}
	var taskContext service.ProjectDesignSystemTaskContext
	return json.Unmarshal(task.Context, &taskContext) == nil && taskContext.Type == service.ProjectDesignSystemTaskContextType
}

func (h *Handler) prepareProjectDesignSystemCompletion(
	ctx context.Context,
	task db.AgentTaskQueue,
	resolvedWorkspaceID string,
	artifacts *ProjectDesignSystemArtifacts,
) (preparedProjectDesignSystemCompletion, error) {
	if artifacts == nil {
		return preparedProjectDesignSystemCompletion{}, errors.New("project design system artifacts are required")
	}
	validated, err := projectdesignsystem.Validate(projectdesignsystem.ArtifactInput{
		DesignMD:       artifacts.DesignMD,
		TokensCSS:      artifacts.TokensCSS,
		ComponentsHTML: artifacts.ComponentsHTML,
	}, h.projectDesignSystemAllowedHosts())
	if err != nil {
		return preparedProjectDesignSystemCompletion{}, fmt.Errorf("invalid project design system artifacts: %w", err)
	}

	var taskContext service.ProjectDesignSystemTaskContext
	if err := json.Unmarshal(task.Context, &taskContext); err != nil || taskContext.Type != service.ProjectDesignSystemTaskContextType {
		return preparedProjectDesignSystemCompletion{}, errors.New("invalid project design system task context")
	}
	workspaceID, err := util.ParseUUID(taskContext.WorkspaceID)
	if err != nil || taskContext.WorkspaceID != resolvedWorkspaceID {
		return preparedProjectDesignSystemCompletion{}, errors.New("project design system workspace does not match task")
	}
	projectID, err := util.ParseUUID(taskContext.ProjectID)
	if err != nil {
		return preparedProjectDesignSystemCompletion{}, errors.New("invalid project design system project id")
	}
	systemID, err := util.ParseUUID(taskContext.ProjectDesignSystemID)
	if err != nil {
		return preparedProjectDesignSystemCompletion{}, errors.New("invalid project design system id")
	}
	agentID, err := util.ParseUUID(taskContext.AgentID)
	if err != nil || uuidToString(agentID) != uuidToString(task.AgentID) {
		return preparedProjectDesignSystemCompletion{}, errors.New("project design system agent does not match task")
	}

	system, err := h.Queries.GetProjectDesignSystemInWorkspace(ctx, db.GetProjectDesignSystemInWorkspaceParams{
		ID:          systemID,
		WorkspaceID: workspaceID,
	})
	if err != nil {
		return preparedProjectDesignSystemCompletion{}, errors.New("project design system not found")
	}
	if uuidToString(system.ProjectID) != uuidToString(projectID) ||
		!system.CurrentAgentID.Valid || uuidToString(system.CurrentAgentID) != uuidToString(agentID) ||
		!system.ActiveTaskID.Valid || uuidToString(system.ActiveTaskID) != uuidToString(task.ID) {
		return preparedProjectDesignSystemCompletion{}, errors.New("project design system active task does not match completion")
	}

	return preparedProjectDesignSystemCompletion{
		TaskContext: taskContext,
		WorkspaceID: workspaceID,
		ProjectID:   projectID,
		SystemID:    systemID,
		AgentID:     agentID,
		Package:     validated,
	}, nil
}

func persistProjectDesignSystemCompletion(
	ctx context.Context,
	queries *db.Queries,
	completedTask db.AgentTaskQueue,
	prepared preparedProjectDesignSystemCompletion,
) (db.ProjectDesignSystem, error) {
	if _, err := queries.LockProjectInWorkspaceForUpdate(ctx, db.LockProjectInWorkspaceForUpdateParams{
		ID:          prepared.ProjectID,
		WorkspaceID: prepared.WorkspaceID,
	}); err != nil {
		return db.ProjectDesignSystem{}, err
	}
	system, err := queries.GetProjectDesignSystemInWorkspace(ctx, db.GetProjectDesignSystemInWorkspaceParams{
		ID:          prepared.SystemID,
		WorkspaceID: prepared.WorkspaceID,
	})
	if err != nil {
		return db.ProjectDesignSystem{}, err
	}
	if uuidToString(system.ProjectID) != uuidToString(prepared.ProjectID) ||
		!system.CurrentAgentID.Valid || uuidToString(system.CurrentAgentID) != uuidToString(prepared.AgentID) ||
		!system.ActiveTaskID.Valid || uuidToString(system.ActiveTaskID) != uuidToString(completedTask.ID) {
		return db.ProjectDesignSystem{}, errors.New("project design system active task changed before completion")
	}

	manifestJSON, err := json.Marshal(prepared.Package.Manifest)
	if err != nil {
		return db.ProjectDesignSystem{}, fmt.Errorf("marshal project design system manifest: %w", err)
	}
	validationJSON, err := json.Marshal(prepared.Package.Validation)
	if err != nil {
		return db.ProjectDesignSystem{}, fmt.Errorf("marshal project design system validation: %w", err)
	}
	if _, err := queries.UpsertProjectDesignSystemPackage(ctx, db.UpsertProjectDesignSystemPackageParams{
		DesignSystemID:  system.ID,
		Slot:            "draft",
		DesignMd:        prepared.Package.Artifacts.DesignMD,
		TokensCss:       prepared.Package.Artifacts.TokensCSS,
		ComponentsHtml:  prepared.Package.Artifacts.ComponentsHTML,
		Manifest:        manifestJSON,
		Validation:      validationJSON,
		IntegritySha256: prepared.Package.Manifest.Digest,
		SourceTaskID:    completedTask.ID,
		AgentID:         prepared.AgentID,
		Instruction:     pgtype.Text{String: prepared.TaskContext.Instruction, Valid: strings.TrimSpace(prepared.TaskContext.Instruction) != ""},
		Scope:           prepared.TaskContext.Scope,
		WorkspaceID:     prepared.WorkspaceID,
	}); err != nil {
		return db.ProjectDesignSystem{}, err
	}

	return queries.ClearProjectDesignSystemActiveTask(ctx, db.ClearProjectDesignSystemActiveTaskParams{
		ID:           system.ID,
		WorkspaceID:  prepared.WorkspaceID,
		ActiveTaskID: completedTask.ID,
	})
}

func (h *Handler) failInvalidProjectDesignSystemCompletion(ctx context.Context, task db.AgentTaskQueue, req TaskCompleteRequest, cause error) {
	failedTask, err := h.TaskService.FailTask(ctx, task.ID, cause.Error(), req.SessionID, req.WorkDir, "project_design_system_invalid_artifacts")
	if err != nil || failedTask == nil {
		return
	}
	_ = h.Queries.DeleteTaskTokensByTask(ctx, failedTask.ID)
}
