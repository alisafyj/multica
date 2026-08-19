package handler

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/multica-ai/multica/server/internal/designrecipepreview"
	"github.com/multica-ai/multica/server/internal/realtime"
)

func firstPreviewSlug(t *testing.T, kind designrecipepreview.Kind) string {
	t.Helper()
	// Any seeded slug of the wanted kind; the bundled set is fixed, so probing
	// a few well-known ones is enough to find one of each.
	for _, slug := range []string{"blog-post", "doc-kami-parchment", "social-spotify-card", "frame-data-rollup", "frame-takram-organic"} {
		if designrecipepreview.KindFor(slug) == kind {
			return slug
		}
	}
	t.Skipf("no bundled preview of kind %q among probes", kind)
	return ""
}

// The HTML cover is served as a document into a sandboxed frame, so it must
// leave with a CSP that denies it the network — that header is the difference
// between a cover and an exfiltration path.
func TestGetDesignRecipePreviewServesHTMLWithANetworklessCSP(t *testing.T) {
	realtime.SetAllowedOrigins([]string{"https://app.example.test"})
	t.Cleanup(func() { realtime.SetAllowedOrigins(nil) })
	slug := firstPreviewSlug(t, designrecipepreview.KindHTML)
	w := httptest.NewRecorder()
	testHandler.GetDesignRecipePreview(w,
		withURLParam(httptest.NewRequest(http.MethodGet, "/api/design-recipes/"+slug+"/preview", nil), "slug", slug))
	if w.Code != http.StatusOK {
		t.Fatalf("status = %d", w.Code)
	}
	if ct := w.Header().Get("Content-Type"); !strings.HasPrefix(ct, "text/html") {
		t.Fatalf("content-type = %q", ct)
	}
	csp := w.Header().Get("Content-Security-Policy")
	// The cover is framed by the web app on another origin, so 'self' alone
	// would refuse the only embedder that matters.
	for _, must := range []string{"default-src 'none'", "connect-src 'none'", "frame-ancestors 'self' https://app.example.test"} {
		if !strings.Contains(csp, must) {
			t.Fatalf("CSP %q lacks %q", csp, must)
		}
	}
	if w.Header().Get("X-Content-Type-Options") != "nosniff" {
		t.Fatal("missing nosniff")
	}
	if !strings.Contains(strings.ToLower(w.Body.String()), "<html") {
		t.Fatal("body is not an HTML document")
	}
}

func TestGetDesignRecipePreviewRejectsUnknownAndTraversal(t *testing.T) {
	for _, slug := range []string{"nope", "../../go.mod", "blog-post/../blog-post"} {
		w := httptest.NewRecorder()
		testHandler.GetDesignRecipePreview(w,
			withURLParam(httptest.NewRequest(http.MethodGet, "/api/design-recipes/x/preview", nil), "slug", slug))
		if w.Code != http.StatusNotFound {
			t.Fatalf("slug %q → %d, want 404", slug, w.Code)
		}
	}
}

// The list stamps each built-in with the cover kind it actually has, so the
// gallery decides frame-vs-image from a fact rather than a fetch.
func TestListDesignScenarioRecipesStampsPreviewKind(t *testing.T) {
	if testPool == nil {
		t.Skip("database not available")
	}
	w := httptest.NewRecorder()
	req := newRequest(http.MethodGet, "/api/design-recipes?workspace_id="+testWorkspaceID, nil)
	testHandler.ListDesignScenarioRecipes(w, req)
	if w.Code != http.StatusOK {
		t.Fatalf("status = %d body = %s", w.Code, w.Body.String())
	}
	body := w.Body.String()
	if !strings.Contains(body, `"preview_kind":"html"`) {
		t.Fatal("no built-in recipe reported an html cover")
	}
}
