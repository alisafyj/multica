package handler

import (
	"net/http"
	"strings"

	"github.com/go-chi/chi/v5"
	"github.com/multica-ai/multica/server/internal/designrecipepreview"
	"github.com/multica-ai/multica/server/internal/realtime"
)

// GetDesignRecipePreview serves the cover for a built-in recipe card.
//
// Registered outside the authenticated router, like the design-system package
// preview: an <iframe> or <img> cannot attach the Bearer header the API
// otherwise requires, and the covers are bundled product content — identical
// for every workspace, holding no user data — so there is nothing a login
// would protect. What the route does protect is the app: see below.
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
		// frame-ancestors names the app origins: the cover is served from the
		// API origin and framed by the web app, so 'self' alone would refuse
		// the one embedder that is supposed to work.
		ancestors := "'self'"
		for _, origin := range realtime.AllowedOrigins() {
			// CORS accepts the literal "null" for opaque origins (the Figma
			// plugin panel); frame-ancestors has no such source, so only real
			// scheme://host entries are copied across.
			if strings.Contains(origin, "://") {
				ancestors += " " + origin
			}
		}
		w.Header().Set("Content-Security-Policy",
			"default-src 'none'; style-src 'unsafe-inline'; script-src 'unsafe-inline'; "+
				"img-src data: blob:; font-src data:; media-src data: blob:; "+
				"connect-src 'none'; frame-ancestors "+ancestors+"; base-uri 'none'; form-action 'none'")
	}
	w.Header().Set("Content-Type", preview.ContentType)
	w.WriteHeader(http.StatusOK)
	_, _ = w.Write(preview.Body)
}
