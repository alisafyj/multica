package handler

import (
	"encoding/json"
	"net/http"
)

type DesignRepositoryResponse struct {
	ID                string `json:"id"`
	ProjectID         string `json:"project_id"`
	ProjectTitle      string `json:"project_title"`
	Label             string `json:"label"`
	RepositoryURL     string `json:"repository_url"`
	DefaultBranchHint string `json:"default_branch_hint"`
}

// ListDesignRepositories returns the small workspace catalogue the MVP Finder
// uses to choose one GitHub repository without opening a project first.
func (h *Handler) ListDesignRepositories(w http.ResponseWriter, r *http.Request) {
	workspaceID, ok := parseUUIDOrBadRequest(w, h.resolveWorkspaceID(r), "workspace id")
	if !ok {
		return
	}
	rows, err := h.Queries.ListDesignRepositoriesInWorkspace(r.Context(), workspaceID)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "failed to list design repositories")
		return
	}
	repositories := make([]DesignRepositoryResponse, 0, len(rows))
	for _, row := range rows {
		var ref githubRepoRef
		if err := json.Unmarshal(row.ResourceRef, &ref); err != nil {
			continue
		}
		// Legacy rows were validated on write, but a malformed value must not
		// break the picker or expose its raw JSON parsing failure.
		if !isValidGitRepoURL(ref.URL) {
			continue
		}
		repositories = append(repositories, DesignRepositoryResponse{
			ID:                uuidToString(row.ID),
			ProjectID:         uuidToString(row.ProjectID),
			ProjectTitle:      row.ProjectTitle,
			Label:             textToString(row.Label),
			RepositoryURL:     ref.URL,
			DefaultBranchHint: ref.DefaultBranchHint,
		})
	}
	writeJSON(w, http.StatusOK, map[string]any{"repositories": repositories})
}
