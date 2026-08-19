package handler

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/multica-ai/multica/server/internal/designrecipepreview"
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

// The HTML cover is served as a document into a frame, so it must leave with
// a CSP that sandboxes it into an opaque origin and denies it the network —
// that header is the difference between a cover and an exfiltration path.
func TestGetDesignRecipePreviewServesHTMLWithANetworklessCSP(t *testing.T) {
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
	// Sandboxed by the response, not by a frame attribute (some embedders
	// refuse to fetch client-sandboxed frames); framed by web, desktop and
	// the Figma panel, none of which the server can enumerate, and holding
	// nothing framing could take — so ancestors are open.
	for _, must := range []string{"sandbox allow-scripts;", "default-src 'none'", "connect-src 'none'", "frame-ancestors *;"} {
		if !strings.Contains(csp, must) {
			t.Fatalf("CSP %q lacks %q", csp, must)
		}
	}
	if strings.Contains(csp, "allow-same-origin") || strings.Contains(csp, "allow-top-navigation") || strings.Contains(csp, "allow-forms") {
		t.Fatalf("CSP %q loosens the sandbox", csp)
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

// A deck example loads its runtime from `assets/deck-stage.js` beside it. The
// gallery frames the document at `/preview/`, so that relative path lands on
// the wildcard route, which must hand back the bundled file with the type
// the browser needs to run it — and nothing outside the slug's own bundle.
func TestGetDesignRecipePreviewServesBundledSiblingFiles(t *testing.T) {
	const slug = "html-ppt-zhangzara-pin-and-paper"
	if designrecipepreview.KindFor(slug) != designrecipepreview.KindHTML {
		t.Skip("deck example not bundled")
	}
	get := func(rel string) *httptest.ResponseRecorder {
		w := httptest.NewRecorder()
		req := httptest.NewRequest(http.MethodGet, "/api/design-recipes/"+slug+"/preview/"+rel, nil)
		testHandler.GetDesignRecipePreview(w, withURLParams(req, "slug", slug, "*", rel))
		return w
	}
	for rel, wantType := range map[string]string{
		"assets/deck-stage.js": "text/javascript; charset=utf-8",
		"assets/styles.css":    "text/css; charset=utf-8",
	} {
		w := get(rel)
		if w.Code != http.StatusOK {
			t.Fatalf("%s → %d", rel, w.Code)
		}
		if got := w.Header().Get("Content-Type"); got != wantType {
			t.Fatalf("%s content-type = %q, want %q", rel, got, wantType)
		}
		if w.Header().Get("X-Content-Type-Options") != "nosniff" {
			t.Fatalf("%s missing nosniff", rel)
		}
		if w.Body.Len() == 0 {
			t.Fatalf("%s is empty", rel)
		}
	}
	// An empty wildcard is the document itself — the trailing-slash URL the
	// gallery frames.
	if w := get(""); w.Code != http.StatusOK || !strings.HasPrefix(w.Header().Get("Content-Type"), "text/html") {
		t.Fatalf("empty wildcard → %d %q, want the HTML document", w.Code, w.Header().Get("Content-Type"))
	}
	for _, rel := range []string{
		"assets/missing.js",
		"../blog-post/example.html",
		"assets/../../blog-post/example.html",
		"/assets/deck-stage.js",
		"assets//deck-stage.js",
		"example.html", // the document has its own URL; not addressable as an asset
	} {
		if w := get(rel); w.Code != http.StatusNotFound {
			t.Fatalf("%q → %d, want 404", rel, w.Code)
		}
	}
}

// The document's CSP must admit the bundled siblings ('self') and still deny
// every network source: that is what lets a deck runtime load without giving
// an example a way out.
func TestGetDesignRecipePreviewCSPAdmitsOnlyBundledAndInlineSources(t *testing.T) {
	slug := firstPreviewSlug(t, designrecipepreview.KindHTML)
	w := httptest.NewRecorder()
	testHandler.GetDesignRecipePreview(w,
		withURLParam(httptest.NewRequest(http.MethodGet, "/api/design-recipes/"+slug+"/preview", nil), "slug", slug))
	csp := w.Header().Get("Content-Security-Policy")
	for _, must := range []string{
		"script-src 'self' 'unsafe-inline';",
		"style-src 'self' 'unsafe-inline';",
		"img-src 'self' data: blob:;",
		"worker-src blob:;",
		"connect-src 'none';",
	} {
		if !strings.Contains(csp, must) {
			t.Fatalf("CSP %q lacks %q", csp, must)
		}
	}
	if strings.Contains(csp, "https:") || strings.Contains(csp, "http:") {
		t.Fatalf("CSP %q admits a network scheme", csp)
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
