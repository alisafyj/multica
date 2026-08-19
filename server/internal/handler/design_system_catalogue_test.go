package handler

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/multica-ai/multica/server/internal/designsystemcatalogue"
)

// designsystemcatalogueDigestForTest reads the digest the catalogue computed
// for a slug, so the test asserts the URL the handler composes without
// re-deriving the hash.
func designsystemcatalogueDigestForTest(t *testing.T, slug string) string {
	t.Helper()
	entries, err := designsystemcatalogue.List()
	if err != nil {
		t.Fatal(err)
	}
	for _, entry := range entries {
		if entry.Slug == slug {
			return entry.ShowcaseDigest
		}
	}
	t.Fatalf("slug %q not in catalogue", slug)
	return ""
}

// The catalogue is embedded, so these exercise the real bundled packages
// rather than a fixture — the response the 官方 scope renders is what a
// reader sees here.
func TestListBuiltinDesignSystemsReturnsTheBundledCatalogue(t *testing.T) {
	w := httptest.NewRecorder()
	testHandler.ListBuiltinDesignSystems(w, httptest.NewRequest(http.MethodGet, "/api/design-systems/builtin", nil))
	if w.Code != http.StatusOK {
		t.Fatalf("status = %d, body = %s", w.Code, w.Body.String())
	}
	var body struct {
		DesignSystems []BuiltinDesignSystemResponse `json:"design_systems"`
	}
	if err := json.Unmarshal(w.Body.Bytes(), &body); err != nil {
		t.Fatal(err)
	}
	if len(body.DesignSystems) < 100 {
		t.Fatalf("catalogue returned %d systems", len(body.DesignSystems))
	}
	for _, system := range body.DesignSystems {
		if system.Slug == "" || system.Name == "" {
			t.Fatalf("system without identity: %+v", system)
		}
	}
}

func TestGetBuiltinDesignSystemReturnsContentAndRejectsUnknown(t *testing.T) {
	list := httptest.NewRecorder()
	testHandler.ListBuiltinDesignSystems(list, httptest.NewRequest(http.MethodGet, "/api/design-systems/builtin", nil))
	var body struct {
		DesignSystems []BuiltinDesignSystemResponse `json:"design_systems"`
	}
	if err := json.Unmarshal(list.Body.Bytes(), &body); err != nil || len(body.DesignSystems) == 0 {
		t.Fatalf("list precondition failed: %v", err)
	}
	slug := body.DesignSystems[0].Slug

	w := httptest.NewRecorder()
	req := withURLParam(httptest.NewRequest(http.MethodGet, "/api/design-systems/builtin/"+slug, nil), "slug", slug)
	testHandler.GetBuiltinDesignSystem(w, req)
	if w.Code != http.StatusOK {
		t.Fatalf("status = %d, body = %s", w.Code, w.Body.String())
	}
	var detail BuiltinDesignSystemDetailResponse
	if err := json.Unmarshal(w.Body.Bytes(), &detail); err != nil {
		t.Fatal(err)
	}
	if detail.Slug != slug || detail.TokensCSS == "" || detail.DesignMarkdown == "" {
		t.Fatalf("detail is incomplete: slug=%q tokens=%d design=%d",
			detail.Slug, len(detail.TokensCSS), len(detail.DesignMarkdown))
	}

	missing := httptest.NewRecorder()
	testHandler.GetBuiltinDesignSystem(missing,
		withURLParam(httptest.NewRequest(http.MethodGet, "/api/design-systems/builtin/nope", nil), "slug", "nope"))
	if missing.Code != http.StatusNotFound {
		t.Fatalf("unknown slug status = %d, want 404", missing.Code)
	}
}

// The list hands the client a digest-versioned showcase path, and the
// unauthenticated showcase route serves that document immutably, fenced by its
// own CSP: no scripts, no network, framed by any app origin. Any other digest
// or variant is a miss the browser must not remember.
func TestBuiltinDesignSystemShowcaseIsServedByDigest(t *testing.T) {
	list := httptest.NewRecorder()
	testHandler.ListBuiltinDesignSystems(list, httptest.NewRequest(http.MethodGet, "/api/design-systems/builtin", nil))
	var body struct {
		DesignSystems []BuiltinDesignSystemResponse `json:"design_systems"`
	}
	if err := json.Unmarshal(list.Body.Bytes(), &body); err != nil {
		t.Fatal(err)
	}
	var withShowcase *BuiltinDesignSystemResponse
	for i := range body.DesignSystems {
		if body.DesignSystems[i].ShowcaseURL != "" {
			withShowcase = &body.DesignSystems[i]
			break
		}
	}
	if withShowcase == nil {
		t.Fatal("no built-in system exposes a showcase")
	}
	digest := designsystemcatalogueDigestForTest(t, withShowcase.Slug)
	wantURL := "/api/design-systems/builtin/" + withShowcase.Slug + "/showcase/" + digest + "/light"
	if withShowcase.ShowcaseURL != wantURL {
		t.Fatalf("showcase_url = %q, want %q", withShowcase.ShowcaseURL, wantURL)
	}

	serve := func(slug, digest, variant string) *httptest.ResponseRecorder {
		recorder := httptest.NewRecorder()
		request := withURLParams(httptest.NewRequest(http.MethodGet, "/api/design-systems/builtin/"+slug+"/showcase/"+digest+"/"+variant, nil),
			"slug", slug, "digest", digest, "variant", variant)
		testHandler.GetBuiltinDesignSystemShowcase(recorder, request)
		return recorder
	}
	for _, variant := range []string{"light", "dark"} {
		recorder := serve(withShowcase.Slug, digest, variant)
		if recorder.Code != http.StatusOK {
			t.Fatalf("%s: status = %d, body = %s", variant, recorder.Code, recorder.Body.String())
		}
		if got := recorder.Header().Get("Content-Type"); !strings.HasPrefix(got, "text/html") {
			t.Fatalf("%s: Content-Type = %q", variant, got)
		}
		if got := recorder.Header().Get("Cache-Control"); !strings.Contains(got, "immutable") {
			t.Fatalf("%s: Cache-Control = %q, want immutable", variant, got)
		}
		csp := recorder.Header().Get("Content-Security-Policy")
		for _, directive := range []string{"default-src 'none'", "style-src 'unsafe-inline'", "frame-ancestors *", "sandbox"} {
			if !strings.Contains(csp, directive) {
				t.Fatalf("%s: CSP %q lacks %q", variant, csp, directive)
			}
		}
		if strings.Contains(csp, "script-src") {
			t.Fatalf("%s: showcase CSP admits scripts: %q", variant, csp)
		}
	}
	for _, tt := range []struct{ name, slug, digest, variant string }{
		{name: "stale digest", slug: withShowcase.Slug, digest: "000000000000", variant: "light"},
		{name: "unknown variant", slug: withShowcase.Slug, digest: digest, variant: "poster"},
		{name: "unknown slug", slug: "nope", digest: digest, variant: "light"},
	} {
		recorder := serve(tt.slug, tt.digest, tt.variant)
		if recorder.Code != http.StatusNotFound {
			t.Fatalf("%s: status = %d, want 404", tt.name, recorder.Code)
		}
		if got := recorder.Header().Get("Cache-Control"); got != "no-store" {
			t.Fatalf("%s: Cache-Control = %q, want no-store", tt.name, got)
		}
	}
}
