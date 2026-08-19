package handler

import (
	"net/http"
	"strings"

	"github.com/go-chi/chi/v5"
	"github.com/multica-ai/multica/server/internal/designrecipepreview"
)

// GetDesignRecipePreview serves the cover for a built-in recipe card, and the
// files an HTML cover references beside itself.
//
// Registered outside the authenticated router, like the design-system package
// preview: an <iframe> or <img> cannot attach the Bearer header the API
// otherwise requires, and the covers are bundled product content — identical
// for every workspace, holding no user data — so there is nothing a login
// would protect. What the route does protect is the app: see below.
//
// The HTML variant is a template's own example output, authored upstream and
// rendered inside an iframe on the gallery card. It is served as a document
// on purpose — that is what an iframe needs — so the response itself fences
// it: a CSP `sandbox allow-scripts` gives the document an opaque origin (it
// cannot read this app's cookies or storage, submit forms, or navigate the
// top window), and the rest of the policy denies it the network. A cover that
// cannot phone home is still a cover; one that can is an exfiltration path.
//
// The sandbox is applied by the response rather than by a `sandbox`
// attribute on the frame for a practical reason: some embedding environments
// refuse to fetch a frame that is sandboxed client-side into an opaque origin
// (the request never leaves the browser), while a document that arrives and
// is then sandboxed by its own headers loads everywhere. The protection is
// the same either way; only the enforcer differs, and the server is ours.
//
// The route is mounted twice: at `/preview` and at `/preview/*`. The gallery
// frames the trailing-slash form so a relative `assets/deck-stage.js` in the
// example resolves under `/preview/`, where the wildcard serves it from the
// same bundle; an empty wildcard is the document itself.
func (h *Handler) GetDesignRecipePreview(w http.ResponseWriter, r *http.Request) {
	slug := strings.TrimSpace(chi.URLParam(r, "slug"))
	if rel := chi.URLParam(r, "*"); rel != "" {
		h.serveDesignRecipePreviewAsset(w, slug, rel)
		return
	}
	preview, ok, err := designrecipepreview.Get(slug)
	if err != nil {
		writeProjectDesignSystemError(w, http.StatusInternalServerError, "lookup_failed", "failed to load recipe preview")
		return
	}
	if !ok {
		writeProjectDesignSystemError(w, http.StatusNotFound, "preview_not_found", "recipe has no preview")
		return
	}
	writeDesignRecipePreviewHeaders(w, preview.ContentType, preview.Kind == designrecipepreview.KindHTML)
	w.WriteHeader(http.StatusOK)
	_, _ = w.Write(preview.Body)
}

func (h *Handler) serveDesignRecipePreviewAsset(w http.ResponseWriter, slug, rel string) {
	asset, ok, err := designrecipepreview.GetAsset(slug, rel)
	if err != nil {
		writeProjectDesignSystemError(w, http.StatusInternalServerError, "lookup_failed", "failed to load recipe preview asset")
		return
	}
	if !ok {
		writeProjectDesignSystemError(w, http.StatusNotFound, "preview_not_found", "recipe preview has no such file")
		return
	}
	// The CSP goes on every file, not only .html: a browser only honours it on
	// documents, and stamping it unconditionally means an SVG or a nested page
	// navigated to directly is fenced the same way as the cover.
	writeDesignRecipePreviewHeaders(w, asset.ContentType, true)
	w.WriteHeader(http.StatusOK)
	_, _ = w.Write(asset.Body)
}

func writeDesignRecipePreviewHeaders(w http.ResponseWriter, contentType string, document bool) {
	// Bundled with the binary, so it changes only with a deploy.
	w.Header().Set("Cache-Control", "public, max-age=86400")
	w.Header().Set("X-Content-Type-Options", "nosniff")
	if document {
		// Overrides the app-wide CSP for this document only. Inline styles
		// and scripts are what the examples are made of, 'self' admits the
		// files bundled beside them, and the network is what they must not
		// have: no connect-src, no third-party host anywhere. A blob: worker
		// is the one script source beyond inline that an example spawns.
		//
		// frame-ancestors is open. The cover is served from the API origin
		// and framed by the web app, the desktop app (a per-worktree dev
		// port, file: when installed) and the Figma plugin panel — embedders
		// this server cannot enumerate — and it holds nothing that framing
		// could take: no user data, no session, no action to hijack. The
		// route being unauthenticated already concedes the content is
		// public; naming ancestors here would only break the app.
		w.Header().Set("Content-Security-Policy",
			"sandbox allow-scripts; default-src 'none'; style-src 'self' 'unsafe-inline'; script-src 'self' 'unsafe-inline'; "+
				"worker-src blob:; img-src 'self' data: blob:; font-src 'self' data:; media-src 'self' data: blob:; "+
				"connect-src 'none'; frame-ancestors *; base-uri 'none'; form-action 'none'")
	}
	w.Header().Set("Content-Type", contentType)
}
