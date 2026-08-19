package handler

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
)

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
