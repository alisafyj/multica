package service

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgtype"
	"github.com/multica-ai/multica/server/internal/projectdesignsystem"
	"github.com/multica-ai/multica/server/internal/util"
	db "github.com/multica-ai/multica/server/pkg/db/generated"
)

const DesignContextVersion = "multica.design-context/v1"

type DesignContextSource string

const (
	DesignContextSourceNone              DesignContextSource = "none"
	DesignContextSourceCloudSaved        DesignContextSource = "cloud_saved_project_design_system"
	DesignContextSourceLocalDesignMD     DesignContextSource = "local_design_md"
	DesignContextSourceRepositoryReality DesignContextSource = "repository_reality"
)

var ErrSavedDesignContextInvalid = errors.New("saved project design system context is invalid")

type ProjectDesignContextStore interface {
	GetProjectDesignSystemByProject(context.Context, db.GetProjectDesignSystemByProjectParams) (db.ProjectDesignSystem, error)
	GetProjectDesignSystemPackageBySlot(context.Context, db.GetProjectDesignSystemPackageBySlotParams) (db.ProjectDesignSystemPackage, error)
}

var _ ProjectDesignContextStore = (*db.Queries)(nil)

type ProjectDesignContextResolver struct {
	Store        ProjectDesignContextStore
	AllowedHosts []string
}

type ResolveProjectDesignContextParams struct {
	WorkspaceID pgtype.UUID
	ProjectID   pgtype.UUID
}

type ResolvedDesignContext struct {
	Version   string                     `json:"version"`
	ProjectID string                     `json:"project_id"`
	Source    DesignContextSource        `json:"source"`
	Priority  []DesignContextSource      `json:"priority"`
	Digest    string                     `json:"digest,omitempty"`
	Package   *SavedProjectDesignContext `json:"package,omitempty"`
}

type SavedProjectDesignContext struct {
	DesignSystemID string                            `json:"design_system_id"`
	Name           string                            `json:"name"`
	Platform       string                            `json:"platform"`
	SourceTaskID   string                            `json:"source_task_id,omitempty"`
	SavedAt        string                            `json:"saved_at"`
	Manifest       projectdesignsystem.Manifest      `json:"manifest"`
	Artifacts      projectdesignsystem.ArtifactInput `json:"artifacts"`
}

func (r ProjectDesignContextResolver) Resolve(
	ctx context.Context,
	params ResolveProjectDesignContextParams,
) (ResolvedDesignContext, error) {
	resolved := emptyProjectDesignContext(params.ProjectID)
	if r.Store == nil {
		return resolved, errors.New("project design context store is required")
	}
	if !params.WorkspaceID.Valid || !params.ProjectID.Valid {
		return resolved, errors.New("workspace_id and project_id are required")
	}

	system, err := r.Store.GetProjectDesignSystemByProject(ctx, db.GetProjectDesignSystemByProjectParams{
		WorkspaceID: params.WorkspaceID,
		ProjectID:   params.ProjectID,
	})
	if errors.Is(err, pgx.ErrNoRows) {
		return resolved, nil
	}
	if err != nil {
		return resolved, fmt.Errorf("load project design system: %w", err)
	}

	saved, err := r.Store.GetProjectDesignSystemPackageBySlot(ctx, db.GetProjectDesignSystemPackageBySlotParams{
		DesignSystemID: system.ID,
		Slot:           "saved",
		WorkspaceID:    params.WorkspaceID,
	})
	if errors.Is(err, pgx.ErrNoRows) {
		return resolved, nil
	}
	if err != nil {
		return resolved, fmt.Errorf("load saved project design system package: %w", err)
	}

	validated, err := validateSavedProjectDesignContext(system, saved, params, r.AllowedHosts)
	if err != nil {
		return resolved, err
	}

	resolved.Source = DesignContextSourceCloudSaved
	resolved.Digest = validated.Manifest.Digest
	resolved.Package = &SavedProjectDesignContext{
		DesignSystemID: util.UUIDToString(system.ID),
		Name:           system.Name,
		Platform:       system.Platform,
		SourceTaskID:   util.UUIDToString(saved.SourceTaskID),
		SavedAt:        system.SavedAt.Time.UTC().Format(time.RFC3339Nano),
		Manifest:       validated.Manifest,
		Artifacts:      validated.Artifacts,
	}
	return resolved, nil
}

func emptyProjectDesignContext(projectID pgtype.UUID) ResolvedDesignContext {
	return ResolvedDesignContext{
		Version:   DesignContextVersion,
		ProjectID: util.UUIDToString(projectID),
		Source:    DesignContextSourceNone,
		Priority: []DesignContextSource{
			DesignContextSourceCloudSaved,
			DesignContextSourceLocalDesignMD,
			DesignContextSourceRepositoryReality,
		},
	}
}

func validateSavedProjectDesignContext(
	system db.ProjectDesignSystem,
	saved db.ProjectDesignSystemPackage,
	params ResolveProjectDesignContextParams,
	allowedHosts []string,
) (projectdesignsystem.ValidatedPackage, error) {
	invalid := func(reason string) (projectdesignsystem.ValidatedPackage, error) {
		return projectdesignsystem.ValidatedPackage{}, fmt.Errorf("%w: %s", ErrSavedDesignContextInvalid, reason)
	}
	if system.WorkspaceID != params.WorkspaceID || system.ProjectID != params.ProjectID || !system.SavedAt.Valid {
		return invalid("project design system identity or saved state does not match")
	}
	if saved.DesignSystemID != system.ID || saved.Slot != "saved" || saved.RenderStatus != "passed" {
		return invalid("saved package identity or render status does not match")
	}
	var storedValidation projectdesignsystem.ValidationReport
	if len(saved.Validation) == 0 || json.Unmarshal(saved.Validation, &storedValidation) != nil || !storedValidation.Passed {
		return invalid("stored package validation did not pass")
	}

	validated, err := projectdesignsystem.Validate(projectdesignsystem.ArtifactInput{
		DesignMD:       saved.DesignMd,
		TokensCSS:      saved.TokensCss,
		ComponentsHTML: saved.ComponentsHtml,
	}, allowedHosts)
	if err != nil || !validated.Validation.Passed {
		return invalid("saved package artifacts failed validation")
	}
	if validated.Manifest.Digest != saved.IntegritySha256 {
		return invalid("saved package digest does not match its artifacts")
	}
	return validated, nil
}
