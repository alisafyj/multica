package daemon

import (
	"context"
	"encoding/json"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync"
	"testing"

	"github.com/multica-ai/multica/server/internal/daemon/execenv"
)

func TestCompleteTaskCarriesTypedIssueCompletion(t *testing.T) {
	var body map[string]any
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
			t.Fatalf("decode request: %v", err)
		}
		w.WriteHeader(http.StatusOK)
	}))
	defer srv.Close()

	report := IssueCompletionReport{
		ClaimGeneration: 123,
		Intent: &IssueCompletionIntent{
			Version:      1,
			Outcome:      IssueCompletionOutcomeReviewReady,
			Comment:      "Ready for review.",
			BaseRevision: 7,
			BaseETag:     `W/"issue:issue-1:7"`,
			BaseStatus:   "in_progress",
		},
	}
	if err := NewClient(srv.URL).CompleteTask(context.Background(), "task-1", "Ready for review.", "", "", "", false, "", "", report); err != nil {
		t.Fatalf("CompleteTask: %v", err)
	}
	if got := int64(body["claim_generation"].(float64)); got != 123 {
		t.Fatalf("claim_generation = %d, want 123", got)
	}
	completion, ok := body["issue_completion"].(map[string]any)
	if !ok {
		t.Fatalf("issue_completion = %#v", body["issue_completion"])
	}
	if completion["outcome"] != string(IssueCompletionOutcomeReviewReady) || completion["base_status"] != "in_progress" {
		t.Fatalf("issue_completion = %#v", completion)
	}
}

func TestCapableOrdinaryPromptUsesPlatformFinalDelivery(t *testing.T) {
	for _, build := range []struct {
		name string
		fn   func(Task) string
	}{
		{name: "standard", fn: func(task Task) string { return BuildPrompt(task, "claude") }},
		{name: "direct", fn: func(task Task) string { return BuildDirectPrompt(task) }},
	} {
		t.Run(build.name, func(t *testing.T) {
			prompt := build.fn(Task{IssueID: "issue-1", IssueCompletionContractVersion: 1})
			if !strings.Contains(prompt, "Multica posts that exact final response") {
				t.Fatalf("capable prompt missing platform delivery contract:\n%s", prompt)
			}
			if strings.Contains(prompt, "multica issue comment add issue-1") {
				t.Fatalf("capable prompt still requires CLI final delivery:\n%s", prompt)
			}
			for _, want := range []string{"do not post routine progress updates or plans", "streamed run progress is already visible", "all other comment guardrails remain in force"} {
				if !strings.Contains(prompt, want) {
					t.Fatalf("capable prompt missing comment guardrail %q:\n%s", want, prompt)
				}
			}
			if strings.Contains(prompt, "progress comments remain allowed") {
				t.Fatalf("capable prompt re-enables routine progress comments:\n%s", prompt)
			}
		})
	}
	legacy := BuildDirectPrompt(Task{IssueID: "issue-1"})
	if !strings.Contains(legacy, "multica issue comment add issue-1") {
		t.Fatalf("legacy prompt lost CLI final delivery:\n%s", legacy)
	}
}

func TestCompletionCapablePromptExplicitlyOverridesLegacyBriefDelivery(t *testing.T) {
	brief := execenv.RenderRuntimeBrief("claude", execenv.TaskContextForEnv{IssueID: "issue-1"})
	for _, legacyRule := range []string{
		"Final results MUST be delivered via `multica issue comment add`",
		"Post your final results as a comment — this step is mandatory",
		"Post exactly ONE comment per run",
		"Do NOT post progress updates",
	} {
		if !strings.Contains(brief, legacyRule) {
			t.Fatalf("legacy runtime brief missing delivery rule %q", legacyRule)
		}
	}

	capable := BuildPrompt(Task{IssueID: "issue-1", IssueCompletionContractVersion: 1}, "claude")
	for _, override := range []string{"Multica posts that exact final response", "supersedes generic runtime instructions", "do not post routine progress updates or plans"} {
		if !strings.Contains(capable, override) {
			t.Fatalf("capable turn missing explicit override %q", override)
		}
	}
	legacy := BuildPrompt(Task{IssueID: "issue-1"}, "claude")
	if !strings.Contains(legacy, "multica issue comment add issue-1") {
		t.Fatalf("legacy turn missing CLI delivery contract")
	}
}

func TestCapableSquadNoActionRemainsSilent(t *testing.T) {
	prompt := BuildPrompt(Task{
		IssueID:                        "issue-1",
		TriggerCommentID:               "comment-1",
		IsLeaderTask:                   true,
		LeaderRoleResolved:             true,
		IssueCompletionContractVersion: 1,
	}, "claude")
	for _, want := range []string{"return no user-facing final body", "automatic delivery remains silent", "return exactly one short failure result"} {
		if !strings.Contains(prompt, want) {
			t.Fatalf("capable squad prompt missing %q:\n%s", want, prompt)
		}
	}
}

func TestReportTaskResultSynthesizesDeliveredFromPlainFinal(t *testing.T) {
	rec := &issueCompletionRecorder{}
	srv := httptest.NewServer(rec.handler(t))
	defer srv.Close()

	d := &Daemon{client: NewClient(srv.URL)}
	d.reportTaskResult(context.Background(), "task-1", TaskResult{
		Status:                         "completed",
		Comment:                        "plain final response",
		ClaimGeneration:                456,
		IssueCompletionContractVersion: 1,
	}, slog.Default())

	rec.mu.Lock()
	defer rec.mu.Unlock()
	completion, ok := rec.payload["issue_completion"].(map[string]any)
	if !ok {
		t.Fatalf("issue_completion = %#v", rec.payload["issue_completion"])
	}
	if completion["outcome"] != string(IssueCompletionOutcomeDelivered) || completion["comment"] != "plain final response" {
		t.Fatalf("issue_completion = %#v", completion)
	}
	if got := int64(rec.payload["claim_generation"].(float64)); got != 456 {
		t.Fatalf("claim_generation = %d, want 456", got)
	}
}

func TestReportTaskResultBlankCapableCompletionStaysLegacy(t *testing.T) {
	rec := &issueCompletionRecorder{}
	srv := httptest.NewServer(rec.handler(t))
	defer srv.Close()

	d := &Daemon{client: NewClient(srv.URL)}
	d.reportTaskResult(context.Background(), "task-blank", TaskResult{
		Status:                         "completed",
		Comment:                        "   ",
		ClaimGeneration:                789,
		IssueCompletionContractVersion: 1,
	}, slog.Default())

	rec.mu.Lock()
	defer rec.mu.Unlock()
	if _, present := rec.payload["issue_completion"]; present {
		t.Fatalf("blank completion emitted invalid issue_completion: %#v", rec.payload)
	}
	if _, present := rec.payload["claim_generation"]; present {
		t.Fatalf("blank legacy completion emitted claim_generation: %#v", rec.payload)
	}
}

type issueCompletionRecorder struct {
	mu      sync.Mutex
	payload map[string]any
}

func (r *issueCompletionRecorder) handler(t *testing.T) http.HandlerFunc {
	t.Helper()
	return func(w http.ResponseWriter, req *http.Request) {
		var payload map[string]any
		if err := json.NewDecoder(req.Body).Decode(&payload); err != nil {
			t.Errorf("decode request: %v", err)
			http.Error(w, err.Error(), http.StatusBadRequest)
			return
		}
		r.mu.Lock()
		r.payload = payload
		r.mu.Unlock()
		w.WriteHeader(http.StatusOK)
	}
}
