package handler

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/google/uuid"

	db "github.com/multica-ai/multica/server/pkg/db/generated"
)

func TestMatchPathGlob(t *testing.T) {
	cases := []struct {
		glob, path string
		want       bool
	}{
		{"src/order/**", "src/order/checkout.ts", true},
		{"src/order/**", "src/order/a/b/c.ts", true},
		{"src/order/**", "src/orders/x.ts", false},
		{"**/*.go", "server/internal/handler/test_case.go", true},
		{"**/*.go", "main.go", true},
		{"*.go", "server/main.go", true},
		{"*.go", "server/main.ts", false},
		{"apps/mobile/", "apps/mobile/app/index.tsx", true},
		{"apps/mobile", "apps/mobile/app/index.tsx", true},
		{"apps/mobile", "apps/mobile-web/app.tsx", false},
		{"apps/*/package.json", "apps/web/package.json", true},
		{"apps/*/package.json", "apps/web/src/package.json", false},
		{"packages/{core,views}/**", "packages/views/testing/x.tsx", true},
		{"packages/{core,views}/**", "packages/ui/x.tsx", false},
		{"src/?.ts", "src/a.ts", true},
		{"src/?.ts", "src/ab.ts", false},
		{"src/[abc].ts", "src/b.ts", true},
		{"src/[!abc].ts", "src/b.ts", false},
		{"/server/**", "server/x.go", true},
		{"./docs/**", "docs/readme.md", true},
		{"", "anything", false},
		{"src/**/test/*.ts", "src/test/a.ts", true},
		{"src/**/test/*.ts", "src/x/y/test/a.ts", true},
		{"src/**/test/*.ts", "src/x/test/deep/a.ts", false},
	}
	for _, c := range cases {
		if got := matchPathGlob(c.glob, c.path); got != c.want {
			t.Errorf("matchPathGlob(%q, %q) = %v, want %v", c.glob, c.path, got, c.want)
		}
	}
}

func TestRecommendTestCasesRanksByClaimedPaths(t *testing.T) {
	caseA := parseUUID(uuid.NewString())
	caseB := parseUUID(uuid.NewString())
	bindings := []db.ListTestCaseReposForProjectRow{
		{TestCaseID: caseA, Alias: "web", Role: "under_test", PathGlobs: []byte(`["src/order/**","src/cart/**"]`)},
		{TestCaseID: caseB, Alias: "web", Role: "under_test", PathGlobs: []byte(`["src/order/checkout.ts"]`)},
		{TestCaseID: caseB, Alias: "api", Role: "verifier", PathGlobs: []byte(`["server/**"]`)},
		{TestCaseID: parseUUID(uuid.NewString()), Alias: "docs", Role: "fixture", PathGlobs: nil},
	}
	paths := []string{"src/order/checkout.ts", "src/cart/total.ts", "README.md"}

	claims := recommendTestCases(bindings, paths, "")
	if len(claims.byCase) != 2 {
		t.Fatalf("claimed cases = %d, want 2", len(claims.byCase))
	}
	if got := claims.byCase[uuidToString(caseA)]; len(got.paths) != 2 || len(got.matches) != 2 {
		t.Errorf("case A claim = %+v, want two globs over two paths", got)
	}
	if got := claims.byCase[uuidToString(caseB)]; len(got.paths) != 1 || got.matches[0].Glob != "src/order/checkout.ts" {
		t.Errorf("case B claim = %+v", got)
	}
	if len(claims.unmatched) != 1 || claims.unmatched[0] != "README.md" {
		t.Errorf("unmatched = %v, want README.md only", claims.unmatched)
	}

	// The repo filter drops claims made through other bindings.
	filtered := recommendTestCases(bindings, []string{"server/main.go", "src/order/checkout.ts"}, "api")
	if len(filtered.byCase) != 1 || filtered.byCase[uuidToString(caseB)] == nil {
		t.Errorf("api-only claims = %+v, want case B through the api binding only", filtered.byCase)
	}
}

func TestRecommendTestCasesHandlerReturnsClaimingCasesOnly(t *testing.T) {
	projectID := newTestCaseProject(t)
	webID := newTestCaseRepoResource(t, projectID, "https://github.com/acme/web")
	claiming := createTestCaseForTest(t, map[string]any{
		"project_id": projectID,
		"title":      "Checkout total",
		"steps":      []map[string]any{{"action": "open cart", "expected": "total shown"}},
		"repos": []map[string]any{
			{"project_resource_id": webID, "alias": "web", "role": "under_test", "path_globs": []string{"src/cart/**"}},
		},
	})
	createTestCaseForTest(t, map[string]any{
		"project_id": projectID,
		"title":      "Login",
		"steps":      []map[string]any{{"action": "log in", "expected": "home"}},
		"repos": []map[string]any{
			{"project_resource_id": webID, "alias": "web", "role": "under_test", "path_globs": []string{"src/auth/**"}},
		},
	})

	w := httptest.NewRecorder()
	testHandler.RecommendTestCases(w, newRequest("POST", "/api/test-cases/recommend?workspace_id="+testWorkspaceID, map[string]any{
		"project_id": projectID,
		"paths":      []string{"./src/cart/total.ts", "src/cart/total.ts", "docs/readme.md"},
	}))
	if w.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200: %s", w.Code, w.Body.String())
	}
	var resp RecommendTestCasesResponse
	if err := json.NewDecoder(w.Body).Decode(&resp); err != nil {
		t.Fatal(err)
	}
	if resp.Total != 1 || len(resp.Cases) != 1 || resp.Cases[0].TestCase.ID != claiming.ID {
		t.Fatalf("cases = %+v, want only %s", resp.Cases, claiming.Key)
	}
	if resp.Cases[0].PathCount != 1 || len(resp.Cases[0].Matches) != 1 || resp.Cases[0].Matches[0].Glob != "src/cart/**" {
		t.Errorf("match = %+v", resp.Cases[0].Matches)
	}
	if len(resp.Cases[0].TestCase.Repos) != 1 || resp.Cases[0].TestCase.Repos[0].Alias != "web" {
		t.Errorf("the recommended case carries its repo bindings, got %+v", resp.Cases[0].TestCase.Repos)
	}
	if len(resp.UnmatchedPaths) != 1 || resp.UnmatchedPaths[0] != "docs/readme.md" {
		t.Errorf("unmatched = %v", resp.UnmatchedPaths)
	}

	// No paths is a bad request, not an empty recommendation.
	w = httptest.NewRecorder()
	testHandler.RecommendTestCases(w, newRequest("POST", "/api/test-cases/recommend?workspace_id="+testWorkspaceID, map[string]any{"project_id": projectID, "paths": []string{" ", ""}}))
	if w.Code != http.StatusBadRequest {
		t.Errorf("empty paths status = %d, want 400", w.Code)
	}
}
