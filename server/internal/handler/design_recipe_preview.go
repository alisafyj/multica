package handler

import (
	"net/http"
	"strings"

	"github.com/go-chi/chi/v5"
	"github.com/multica-ai/multica/server/internal/designrecipepreview"
)

// GetDesignRecipePreview serves the cover for a built-in recipe card.
//
// The HTML variant is a template's own example output, authored upstream and
// rendered inside a sandboxed iframe on the gallery card. It is served as a
// document on purpose — that is what an iframe needs — so two things keep it
// from becoming a foothold: the frame is created with `sandbox` and no
// `allow-same-origin` (so the document runs as an opaque origin and cannot
// read this app's cookies or storage), and the response carries a CSP that
// forbids scripts from reaching out. A cover that cannot phone home is still a
// cover; one that can is an exfiltration path.
func (h *Handler) GetDesignRecipePreview(w http.ResponseWriter, r *http.Request) {
	slug := strings.TrimSpace(chi.URLParam(r, "slug"))
	preview, ok, err := designrecipepreview.Get(slug)
	if err != nil {
		writeProjectDesignSystemError(w, http.StatusInternalServerError, "lookup_failed", "failed to load recipe preview")
		return
	}
	if !ok {
		writeProjectDesignSystemError(w, http.StatusNotFound, "preview_not_found", "recipe has no preview")
		return
	}
	// Bundled with the binary, so it changes only with a deploy.
	w.Header().Set("Cache-Control", "public, max-age=86400")
	w.Header().Set("X-Content-Type-Options", "nosniff")
	if preview.Kind == designrecipepreview.KindHTML {
		// Overrides the app-wide CSP for this document only. Inline styles
		// and scripts are what the examples are made of; the network is what
		// they must not have. `frame-ancestors 'self'` keeps the cover from
		// being embedded by another site.
		w.Header().Set("Content-Security-Policy",
			"default-src 'none'; style-src 'unsafe-inline'; script-src 'unsafe-inline'; "+
				"img-src data: blob:; font-src data:; media-src data: blob:; "+
				"connect-src 'none'; frame-ancestors 'self'; base-uri 'none'; form-action 'none'")
	}
	w.Header().Set("Content-Type", preview.ContentType)
	w.WriteHeader(http.StatusOK)
	_, _ = w.Write(preview.Body)
}
