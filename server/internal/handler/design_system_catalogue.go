package handler

import (
	"net/http"
	"net/url"
	"strconv"
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
	// ShowcaseURL is the API path of the package's light showcase document,
	// digest-versioned so it caches immutably; "" when the package ships none.
	// The dark variant lives at the same path with a trailing /dark instead
	// of /light. Prefix with the API base URL.
	ShowcaseURL string `json:"showcase_url,omitempty"`
	// Swatches are the package's first concrete colour values, for list rows.
	Swatches []string `json:"swatches"`
}

func builtinDesignSystemResponse(entry designsystemcatalogue.Entry) BuiltinDesignSystemResponse {
	return BuiltinDesignSystemResponse{
		Slug:        entry.Slug,
		Name:        entry.Name,
		Category:    entry.Category,
		Description: entry.Description,
		ShowcaseURL: builtinDesignSystemShowcasePath(entry, "light"),
		Swatches:    entry.Swatches,
	}
}

// builtinDesignSystemShowcasePath composes the showcase URL next to the route
// that serves it so the two cannot drift; "" when the package has no showcase.
func builtinDesignSystemShowcasePath(entry designsystemcatalogue.Entry, variant string) string {
	if entry.ShowcaseDigest == "" {
		return ""
	}
	return "/api/design-systems/builtin/" + url.PathEscape(entry.Slug) + "/showcase/" + entry.ShowcaseDigest + "/" + variant
}

type BuiltinDesignSystemTokenResponse struct {
	Name  string `json:"name"`
	Value string `json:"value"`
	Type  string `json:"type"`
}

// BuiltinDesignSystemArtifactResponse is one 设计系统素材 card: the derived
// page, its caption, and the digest-versioned URL that frames it.
type BuiltinDesignSystemArtifactResponse struct {
	ID    string `json:"id"`
	Label string `json:"label"`
	URL   string `json:"url"`
}

type BuiltinDesignSystemDetailResponse struct {
	BuiltinDesignSystemResponse
	// Title is DESIGN.md's own H1 — the kit view's page heading and the
	// typography sample line.
	Title    string `json:"title"`
	Identity string `json:"identity"`
	// Palette / Typography / LayoutGuidelines / TokenContract / Artifacts are
	// the kit-view modules, parsed server-side from the package's own files.
	Palette          []designsystemcatalogue.PaletteEntry       `json:"palette"`
	Typography       designsystemcatalogue.Typography           `json:"typography"`
	LayoutGuidelines []string                                   `json:"layout_guidelines"`
	TokenContract    []designsystemcatalogue.TokenContractEntry `json:"token_contract"`
	Artifacts        []BuiltinDesignSystemArtifactResponse      `json:"artifacts"`
	Tokens           []BuiltinDesignSystemTokenResponse         `json:"tokens"`
	TokensCSS        string                                     `json:"tokens_css"`
	DesignMarkdown   string                                     `json:"design_markdown"`
}

func (h *Handler) ListBuiltinDesignSystems(w http.ResponseWriter, r *http.Request) {
	entries, err := designsystemcatalogue.List()
	if err != nil {
		writeProjectDesignSystemError(w, http.StatusInternalServerError, "lookup_failed", "failed to load built-in design systems")
		return
	}
	systems := make([]BuiltinDesignSystemResponse, 0, len(entries))
	for _, entry := range entries {
		systems = append(systems, builtinDesignSystemResponse(entry))
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
	tokens := make([]BuiltinDesignSystemTokenResponse, 0, len(detail.Tokens))
	for _, token := range detail.Tokens {
		tokens = append(tokens, BuiltinDesignSystemTokenResponse{
			Name: token.Name, Value: token.Value, Type: token.Type,
		})
	}
	artifacts := make([]BuiltinDesignSystemArtifactResponse, 0, len(detail.Artifacts))
	for _, artifact := range detail.Artifacts {
		artifacts = append(artifacts, BuiltinDesignSystemArtifactResponse{
			ID:    artifact.ID,
			Label: artifact.Label,
			URL:   builtinDesignSystemShowcasePath(detail.Entry, "artifact-"+artifact.ID),
		})
	}
	writeJSON(w, http.StatusOK, BuiltinDesignSystemDetailResponse{
		BuiltinDesignSystemResponse: builtinDesignSystemResponse(detail.Entry),
		Title:                       detail.Title,
		Identity:                    detail.Identity,
		Palette:                     detail.Palette,
		Typography:                  detail.Typography,
		LayoutGuidelines:            detail.LayoutGuidelines,
		TokenContract:               detail.TokenContract,
		Artifacts:                   artifacts,
		Tokens:                      tokens,
		TokensCSS:                   detail.TokensCSS,
		DesignMarkdown:              detail.DesignMarkdown,
	})
}

// GetBuiltinDesignSystemShowcase serves a package's showcase document to the
// library's cover frame. It is mounted outside the auth group because a frame
// cannot carry the Bearer header; the document is bundled reference content
// with no workspace data in it, so what fences it is its own CSP: no script,
// no network, styles inline only, framed by whichever app origin embeds it
// (the same open frame-ancestors the recipe covers argue for). The URL carries
// the bundle digest, so a matching response caches immutably and any other
// digest is a miss the browser must not remember.
func (h *Handler) GetBuiltinDesignSystemShowcase(w http.ResponseWriter, r *http.Request) {
	slug := strings.TrimSpace(chi.URLParam(r, "slug"))
	variant := strings.TrimSpace(chi.URLParam(r, "variant"))
	digest := chi.URLParam(r, "digest")
	entries, err := designsystemcatalogue.List()
	if err != nil {
		writeProjectDesignSystemError(w, http.StatusInternalServerError, "lookup_failed", "failed to load built-in design systems")
		return
	}
	var expected string
	for _, entry := range entries {
		if entry.Slug == slug {
			expected = entry.ShowcaseDigest
			break
		}
	}
	body, ok := designsystemcatalogue.Showcase(slug, variant)
	if !ok || expected == "" || digest != expected {
		w.Header().Set("Cache-Control", "no-store")
		writeProjectDesignSystemError(w, http.StatusNotFound, "showcase_not_found", "built-in design system showcase is unavailable")
		return
	}
	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	w.Header().Set("Content-Length", strconv.Itoa(len(body)))
	w.Header().Set("Cache-Control", "public, max-age=31536000, immutable")
	w.Header().Set("X-Content-Type-Options", "nosniff")
	w.Header().Set("Referrer-Policy", "no-referrer")
	w.Header().Set("Content-Security-Policy",
		"sandbox; default-src 'none'; style-src 'unsafe-inline'; img-src data:; font-src data:; "+
			"connect-src 'none'; frame-ancestors *; base-uri 'none'; form-action 'none'")
	w.WriteHeader(http.StatusOK)
	_, _ = w.Write(body)
}
