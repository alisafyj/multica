package daemon

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestStartTaskWithIssueStartCarriesGenerationAndReturnsBaseline(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		var body map[string]any
		if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
			t.Fatal(err)
		}
		if body["claim_generation"] != float64(77) {
			t.Fatalf("claim_generation = %#v", body["claim_generation"])
		}
		start, _ := body["issue_start"].(map[string]any)
		if start["version"] != float64(1) || start["base_revision"] != float64(9) || start["base_status"] != "todo" {
			t.Fatalf("issue_start = %#v", start)
		}
		_ = json.NewEncoder(w).Encode(map[string]any{"issue_start": map[string]any{
			"version": 1, "applied": true, "baseline_accepted": true,
			"status": "in_progress", "revision": 10, "etag": `W/"issue:issue-1:10"`,
		}})
	}))
	defer server.Close()

	state, err := NewClient(server.URL).StartTaskWithIssueStart(t.Context(), "task-1", IssueStartReport{
		ClaimGeneration: 77,
		Intent:          IssueStartIntent{Version: 1, BaseRevision: 9, BaseETag: `W/"issue:issue-1:9"`, BaseStatus: "todo"},
	})
	if err != nil {
		t.Fatal(err)
	}
	if state == nil || !state.Applied || !state.BaselineAccepted || state.Revision != 10 || state.Status != "in_progress" {
		t.Fatalf("state = %#v", state)
	}
}
