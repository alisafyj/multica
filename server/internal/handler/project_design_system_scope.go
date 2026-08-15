package handler

import (
	"context"
	"errors"
	"net/http"
	"strings"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgtype"
	db "github.com/multica-ai/multica/server/pkg/db/generated"
)

// projectDesignSystemScope names which design system a request is about
// (DC-052). A project can hold several repositories — a consumer H5 site, a
// mobile app, an admin console — and each keeps its own system, because the
// first and third are both platform='web' and nothing else in the model
// separates them.
//
// The zero value is the project-level scope: the system used across
// repositories, and the one a design task runs against when the user did not
// pick a repository (DC-053).
type projectDesignSystemScope struct {
	// Invalid means project-level.
	ProjectResourceID pgtype.UUID
}

func (s projectDesignSystemScope) isRepository() bool {
	return s.ProjectResourceID.Valid
}

// lookup loads the design system for this scope. Callers get pgx.ErrNoRows
// when the scope has no system yet, exactly as the single-scope code did.
func (s projectDesignSystemScope) lookup(
	ctx context.Context,
	queries projectDesignSystemQuerier,
	workspaceID pgtype.UUID,
	projectID pgtype.UUID,
) (db.ProjectDesignSystem, error) {
	if s.isRepository() {
		return queries.GetProjectDesignSystemByResource(ctx, db.GetProjectDesignSystemByResourceParams{
			WorkspaceID:       workspaceID,
			ProjectID:         projectID,
			ProjectResourceID: s.ProjectResourceID,
		})
	}
	return queries.GetProjectDesignSystemByProject(ctx, db.GetProjectDesignSystemByProjectParams{
		WorkspaceID: workspaceID,
		ProjectID:   projectID,
	})
}

// projectDesignSystemQuerier is the slice of the query surface the scope
// helpers need. Both *db.Queries and a transaction's *db.Queries satisfy it,
// which is why the create paths can pass their transactional querier.
type projectDesignSystemQuerier interface {
	GetProjectDesignSystemByProject(context.Context, db.GetProjectDesignSystemByProjectParams) (db.ProjectDesignSystem, error)
	GetProjectDesignSystemByResource(context.Context, db.GetProjectDesignSystemByResourceParams) (db.ProjectDesignSystem, error)
}

// resolveProjectDesignSystemScope reads the optional project_resource_id and
// proves it is a github_repo belonging to this project before any write uses
// it. The id arrives from the request boundary, so it goes through
// parseUUIDOrBadRequest and is then re-checked against the project — a valid
// UUID from another project must not be able to attach a design system here.
//
// An absent or empty parameter is the project-level scope, not an error.
func (h *Handler) resolveProjectDesignSystemScope(
	w http.ResponseWriter,
	r *http.Request,
	workspaceID pgtype.UUID,
	projectID pgtype.UUID,
) (projectDesignSystemScope, bool) {
	raw := strings.TrimSpace(r.URL.Query().Get("project_resource_id"))
	if raw == "" {
		return projectDesignSystemScope{}, true
	}
	resourceID, ok := parseUUIDOrBadRequest(w, raw, "project_resource_id")
	if !ok {
		return projectDesignSystemScope{}, false
	}
	if !h.projectResourceBelongsToProject(r.Context(), w, workspaceID, projectID, resourceID) {
		return projectDesignSystemScope{}, false
	}
	return projectDesignSystemScope{ProjectResourceID: resourceID}, true
}

// projectDesignSystemScopeFromBody validates an already-parsed resource id
// carried in a JSON request body. Same guarantee as the query-parameter
// variant: the resource must be a github_repo under this project.
func (h *Handler) projectDesignSystemScopeFromBody(
	ctx context.Context,
	w http.ResponseWriter,
	workspaceID pgtype.UUID,
	projectID pgtype.UUID,
	raw string,
) (projectDesignSystemScope, bool) {
	raw = strings.TrimSpace(raw)
	if raw == "" {
		return projectDesignSystemScope{}, true
	}
	resourceID, ok := parseUUIDOrBadRequest(w, raw, "project_resource_id")
	if !ok {
		return projectDesignSystemScope{}, false
	}
	if !h.projectResourceBelongsToProject(ctx, w, workspaceID, projectID, resourceID) {
		return projectDesignSystemScope{}, false
	}
	return projectDesignSystemScope{ProjectResourceID: resourceID}, true
}

func (h *Handler) projectResourceBelongsToProject(
	ctx context.Context,
	w http.ResponseWriter,
	workspaceID pgtype.UUID,
	projectID pgtype.UUID,
	resourceID pgtype.UUID,
) bool {
	resource, err := h.Queries.GetProjectResourceInWorkspace(ctx, db.GetProjectResourceInWorkspaceParams{
		ID:          resourceID,
		WorkspaceID: workspaceID,
	})
	if errors.Is(err, pgx.ErrNoRows) {
		writeProjectDesignSystemError(w, http.StatusNotFound, "project_resource_not_found", "repository not found")
		return false
	}
	if err != nil {
		writeProjectDesignSystemError(w, http.StatusInternalServerError, "project_resource_lookup_failed", "failed to load repository")
		return false
	}
	if resource.ProjectID != projectID {
		writeProjectDesignSystemError(w, http.StatusNotFound, "project_resource_not_found", "repository not found")
		return false
	}
	// Design systems describe a codebase's visual language, so only code
	// repositories can own one. Other resource types (docs, links) would
	// produce a system with nothing to ground it against.
	if resource.ResourceType != projectResourceTypeGitHubRepo {
		writeProjectDesignSystemError(w, http.StatusBadRequest, "project_resource_not_repository", "only repository resources can own a design system")
		return false
	}
	return true
}
