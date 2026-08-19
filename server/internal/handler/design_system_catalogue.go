package handler

import (
	"net/http"
	"strings"

	"github.com/go-chi/chi/v5"
	"github.com/multica-ai/multica/server/internal/designsystemcatalogue"
)

// Built-in design systems (the 官方 scope of the design centre's library).
//
// These are bundled reference packages, identical for every workspace and
// owned by nobody, so both routes are workspace-agnostic reads. They stay
// behind the authenticated router because the catalogue is product content,
// not public documentation — but there is no per-workspace filtering to do.

type BuiltinDesignSystemResponse struct {
	Slug        string `json:"slug"`
	Name        string `json:"name"`
	Category    string `json:"category"`
	Description string `json:"description"`
}

type BuiltinDesignSystemDetailResponse struct {
	BuiltinDesignSystemResponse
	TokensCSS      string `json:"tokens_css"`
	DesignMarkdown string `json:"design_markdown"`
}

func (h *Handler) ListBuiltinDesignSystems(w http.ResponseWriter, r *http.Request) {
	entries, err := designsystemcatalogue.List()
	if err != nil {
		writeProjectDesignSystemError(w, http.StatusInternalServerError, "lookup_failed", "failed to load built-in design systems")
		return
	}
	systems := make([]BuiltinDesignSystemResponse, 0, len(entries))
	for _, entry := range entries {
		systems = append(systems, BuiltinDesignSystemResponse{
			Slug:        entry.Slug,
			Name:        entry.Name,
			Category:    entry.Category,
			Description: entry.Description,
		})
	}
	writeJSON(w, http.StatusOK, map[string]any{"design_systems": systems})
}

func (h *Handler) GetBuiltinDesignSystem(w http.ResponseWriter, r *http.Request) {
	slug := strings.TrimSpace(chi.URLParam(r, "slug"))
	detail, ok, err := designsystemcatalogue.Get(slug)
	if err != nil {
		writeProjectDesignSystemError(w, http.StatusInternalServerError, "lookup_failed", "failed to load built-in design system")
		return
	}
	if !ok {
		writeProjectDesignSystemError(w, http.StatusNotFound, "design_system_not_found", "built-in design system not found")
		return
	}
	writeJSON(w, http.StatusOK, BuiltinDesignSystemDetailResponse{
		BuiltinDesignSystemResponse: BuiltinDesignSystemResponse{
			Slug:        detail.Slug,
			Name:        detail.Name,
			Category:    detail.Category,
			Description: detail.Description,
		},
		TokensCSS:      detail.TokensCSS,
		DesignMarkdown: detail.DesignMarkdown,
	})
}
