package handler

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"strings"

	"github.com/multica-ai/multica/server/internal/projectdesignsystem"
	"github.com/multica-ai/multica/server/internal/service"
	db "github.com/multica-ai/multica/server/pkg/db/generated"
)

const projectDesignSystemRepositoryAnalysisResultMarker = "REPOSITORY_DESIGN_CONTEXT_JSON:"

type preparedProjectDesignSystemRepositoryAnalysis struct {
	TaskContext service.ProjectDesignSystemTaskContext
	Value       projectdesignsystem.RepositoryDesignContext
}

func prepareProjectDesignSystemRepositoryAnalysisCompletion(task db.AgentTaskQueue, output string) (*preparedProjectDesignSystemRepositoryAnalysis, bool, error) {
	var taskContext service.ProjectDesignSystemTaskContext
	if json.Unmarshal(task.Context, &taskContext) != nil ||
		taskContext.Type != service.ProjectDesignSystemTaskContextType ||
		taskContext.Operation != service.ProjectDesignSystemRepositoryAnalysis {
		return nil, false, nil
	}

	value, err := parseProjectDesignSystemRepositoryAnalysisOutput(output)
	if err != nil {
		return nil, true, err
	}
	return &preparedProjectDesignSystemRepositoryAnalysis{TaskContext: taskContext, Value: value}, true, nil
}

func parseProjectDesignSystemRepositoryAnalysisOutput(output string) (projectdesignsystem.RepositoryDesignContext, error) {
	trimmed := strings.TrimSpace(output)
	if strings.Count(trimmed, projectDesignSystemRepositoryAnalysisResultMarker) != 1 ||
		!strings.HasPrefix(trimmed, projectDesignSystemRepositoryAnalysisResultMarker) {
		return projectdesignsystem.RepositoryDesignContext{}, errors.New("repository analysis output must contain exactly one final result marker")
	}
	payload := strings.TrimSpace(strings.TrimPrefix(trimmed, projectDesignSystemRepositoryAnalysisResultMarker))
	if payload == "" || len(payload) > projectdesignsystem.MaxRepositoryDesignContextBytes {
		return projectdesignsystem.RepositoryDesignContext{}, errors.New("repository analysis payload is empty or exceeds its size limit")
	}

	decoder := json.NewDecoder(strings.NewReader(payload))
	decoder.DisallowUnknownFields()
	var value projectdesignsystem.RepositoryDesignContext
	if err := decoder.Decode(&value); err != nil {
		return projectdesignsystem.RepositoryDesignContext{}, fmt.Errorf("decode repository analysis output: %w", err)
	}
	if err := decoder.Decode(&struct{}{}); !errors.Is(err, io.EOF) {
		return projectdesignsystem.RepositoryDesignContext{}, errors.New("repository analysis output must contain exactly one JSON object")
	}
	validated, err := projectdesignsystem.ValidateRepositoryDesignContext(value)
	if err != nil {
		return projectdesignsystem.RepositoryDesignContext{}, fmt.Errorf("validate repository analysis output: %w", err)
	}
	return validated, nil
}

func persistProjectDesignSystemRepositoryAnalysisCompletion(
	ctx context.Context,
	qtx *db.Queries,
	completedTask db.AgentTaskQueue,
	prepared preparedProjectDesignSystemRepositoryAnalysis,
) (db.ProjectDesignSystem, error) {
	workspaceID := parseUUID(prepared.TaskContext.WorkspaceID)
	systemID := parseUUID(prepared.TaskContext.ProjectDesignSystemID)
	system, err := qtx.GetProjectDesignSystemInWorkspaceForUpdate(ctx, db.GetProjectDesignSystemInWorkspaceForUpdateParams{
		ID: systemID, WorkspaceID: workspaceID,
	})
	if err != nil {
		return db.ProjectDesignSystem{}, fmt.Errorf("load repository analysis input snapshot: %w", err)
	}
	var input map[string]json.RawMessage
	if err := json.Unmarshal(system.InputSnapshot, &input); err != nil {
		return db.ProjectDesignSystem{}, fmt.Errorf("decode repository analysis input snapshot: %w", err)
	}
	if input == nil {
		return db.ProjectDesignSystem{}, errors.New("decode repository analysis input snapshot: expected JSON object")
	}
	repositoryAnalysisJSON, err := json.Marshal(prepared.Value)
	if err != nil {
		return db.ProjectDesignSystem{}, fmt.Errorf("encode repository analysis value: %w", err)
	}
	input["repository_analysis"] = repositoryAnalysisJSON
	inputJSON, err := json.Marshal(input)
	if err != nil || len(inputJSON) > maxProjectDesignSystemSnapshotBytes {
		return db.ProjectDesignSystem{}, errors.New("encode repository analysis input snapshot")
	}
	completed, err := qtx.CompleteProjectDesignSystemRepositoryAnalysis(ctx, db.CompleteProjectDesignSystemRepositoryAnalysisParams{
		InputSnapshot: inputJSON,
		ID:            systemID,
		WorkspaceID:   workspaceID,
		ActiveTaskID:  completedTask.ID,
	})
	if err != nil {
		return db.ProjectDesignSystem{}, fmt.Errorf("complete repository analysis: %w", err)
	}
	return completed, nil
}
