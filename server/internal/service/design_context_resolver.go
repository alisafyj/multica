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
	DesignContextSourceNone DesignContextSource = "none"
	// A design system owned by one repository (DC-052). Tried first when the
	// caller names a repository, so an admin console does not inherit the
	// consumer site's visual language just because they share a project.
	DesignContextSourceCloudSavedRepository DesignContextSource = "cloud_saved_repository_design_system"
	// The project-level system: used across repositories, and the one a
	// design task runs against when no repository was picked (DC-053).
	DesignContextSourceCloudSaved        DesignContextSource = "cloud_saved_project_design_system"
	DesignContextSourceLocalDesignMD     DesignContextSource = "local_design_md"
	DesignContextSourceRepositoryReality DesignContextSource = "repository_reality"
)

// DesignContextScope tells the agent whether the system it received was
// written for the repository it is designing against or is the project's
// shared one. Same package shape either way; the distinction changes how
// much the agent should treat the system as authoritative for this surface.
type DesignContextScope string

const (
	DesignContextScopeRepository DesignContextScope = "repository"
	DesignContextScopeProject    DesignContextScope = "project"
)

var ErrSavedDesignContextInvalid = errors.New("saved project design system context is invalid")

type ProjectDesignContextStore interface {
	GetProjectDesignSystemByProject(context.Context, db.GetProjectDesignSystemByProjectParams) (db.ProjectDesignSystem, error)
	GetProjectDesignSystemByResource(context.Context, db.GetProjectDesignSystemByResourceParams) (db.ProjectDesignSystem, error)
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
	// Optional. When the caller names a repository, its own design system is
	// preferred and the project-level one is the fallback. An unset value
	// resolves straight to the project-level system (DC-053).
	ProjectResourceID pgtype.UUID
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
	Scope             DesignContextScope                `json:"scope"`
	ProjectResourceID string                            `json:"project_resource_id,omitempty"`
	DesignSystemID    string                            `json:"design_system_id"`
	Name              string                            `json:"name"`
	Platform          string                            `json:"platform"`
	SourceTaskID      string                            `json:"source_task_id,omitempty"`
	SavedAt           string                            `json:"saved_at"`
	Manifest          projectdesignsystem.Manifest      `json:"manifest"`
	Artifacts         projectdesignsystem.ArtifactInput `json:"artifacts"`
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

	// Repository scope first when the caller named one. A repository whose
	// own system exists but has no saved package yet still falls through to
	// the project-level system: an in-progress draft must never become the
	// constraint (DC-034), and leaving the agent with nothing would be worse
	// than the project's shared system.
	if params.ProjectResourceID.Valid {
		found, err := r.resolveScope(ctx, params, DesignContextScopeRepository)
		if err != nil {
			return resolved, err
		}
		if found != nil {
			return *found, nil
		}
	}

	found, err := r.resolveScope(ctx, params, DesignContextScopeProject)
	if err != nil {
		return resolved, err
	}
	if found != nil {
		return *found, nil
	}
	return resolved, nil
}

// resolveScope loads one scope's saved package. A nil result means "this
// scope has nothing usable" — either no system or no saved package — and the
// caller should try the next scope. A non-nil error is a real failure and
// must not be swallowed into a fallback: an invalid saved package is a
// problem the user has to see, not a reason to silently design against
// something else.
func (r ProjectDesignContextResolver) resolveScope(
	ctx context.Context,
	params ResolveProjectDesignContextParams,
	scope DesignContextScope,
) (*ResolvedDesignContext, error) {
	var system db.ProjectDesignSystem
	var err error
	if scope == DesignContextScopeRepository {
		system, err = r.Store.GetProjectDesignSystemByResource(ctx, db.GetProjectDesignSystemByResourceParams{
			WorkspaceID:       params.WorkspaceID,
			ProjectID:         params.ProjectID,
			ProjectResourceID: params.ProjectResourceID,
		})
	} else {
		system, err = r.Store.GetProjectDesignSystemByProject(ctx, db.GetProjectDesignSystemByProjectParams{
			WorkspaceID: params.WorkspaceID,
			ProjectID:   params.ProjectID,
		})
	}
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, nil
	}
	if err != nil {
		return nil, fmt.Errorf("load %s design system: %w", scope, err)
	}

	saved, err := r.Store.GetProjectDesignSystemPackageBySlot(ctx, db.GetProjectDesignSystemPackageBySlotParams{
		DesignSystemID: system.ID,
		Slot:           "saved",
		WorkspaceID:    params.WorkspaceID,
	})
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, nil
	}
	if err != nil {
		return nil, fmt.Errorf("load saved %s design system package: %w", scope, err)
	}

	validated, err := validateSavedProjectDesignContext(system, saved, params, r.AllowedHosts)
	if err != nil {
		return nil, err
	}

	resolved := emptyProjectDesignContext(params.ProjectID)
	resolved.Source = DesignContextSourceCloudSaved
	if scope == DesignContextScopeRepository {
		resolved.Source = DesignContextSourceCloudSavedRepository
	}
	resolved.Digest = validated.Manifest.Digest
	resolved.Package = &SavedProjectDesignContext{
		Scope:             scope,
		ProjectResourceID: util.UUIDToString(system.ProjectResourceID),
		DesignSystemID:    util.UUIDToString(system.ID),
		Name:              system.Name,
		Platform:          system.Platform,
		SourceTaskID:      util.UUIDToString(saved.SourceTaskID),
		SavedAt:           system.SavedAt.Time.UTC().Format(time.RFC3339Nano),
		Manifest:          validated.Manifest,
		Artifacts:         validated.Artifacts,
	}
	return &resolved, nil
}

func emptyProjectDesignContext(projectID pgtype.UUID) ResolvedDesignContext {
	return ResolvedDesignContext{
		Version:   DesignContextVersion,
		ProjectID: util.UUIDToString(projectID),
		Source:    DesignContextSourceNone,
		Priority: []DesignContextSource{
			DesignContextSourceCloudSavedRepository,
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
