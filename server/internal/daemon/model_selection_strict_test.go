package daemon

import (
	"context"
	"encoding/json"
	"errors"
	"strings"
	"testing"

	"github.com/multica-ai/multica/server/pkg/agent"
	"github.com/multica-ai/multica/server/pkg/taskfailure"
)

func TestResolveTaskModelSelectionForTaskRejectsKnownInvalidOrdinaryIssueSelection(t *testing.T) {
	tests := []struct {
		name    string
		catalog agent.Catalog
		in      taskModelSelection
		want    string
	}{
		{
			name: "unavailable model",
			catalog: agent.Catalog{Unavailable: []agent.UnavailableModel{{
				ID: "gpt-retired", Reason: "upgrade with token secret-must-not-leak",
			}}},
			in:   taskModelSelection{Model: "gpt-retired"},
			want: "model \"gpt-retired\" is unavailable",
		},
		{
			name: "unsupported thinking level",
			catalog: agent.Catalog{Models: []agent.Model{{
				ID: "gpt-current", Thinking: &agent.ModelThinking{SupportedLevels: []agent.ThinkingLevel{{Value: "high"}}},
			}}},
			in:   taskModelSelection{Model: "gpt-current", ThinkingLevel: "ultra"},
			want: "thinking_level \"ultra\" is not supported",
		},
		{
			name: "unsupported service tier",
			catalog: agent.Catalog{Models: []agent.Model{{
				ID: "gpt-current", ServiceTiers: []agent.ModelServiceTier{{ID: "priority"}},
			}}},
			in:   taskModelSelection{Model: "gpt-current", ServiceTier: "future"},
			want: "service_tier \"future\" is not supported",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			reads := stubModelDiscovery(t, map[string]agent.Catalog{"codex": tt.catalog})
			got, err := resolveTaskModelSelectionForTask(context.Background(), Task{IssueID: "issue-1"}, "codex", agent.Command{}, tt.in, quietTaskLog())
			if err == nil || !strings.Contains(err.Error(), tt.want) {
				t.Fatalf("error = %v, want actionable error containing %q", err, tt.want)
			}
			if strings.Contains(err.Error(), "secret-must-not-leak") {
				t.Fatalf("error leaked runtime discovery detail: %v", err)
			}
			if got.Effective != nil {
				t.Fatalf("effective selection = %+v, want nil after rejection", *got.Effective)
			}
			if got.Requested != tt.in {
				t.Fatalf("requested selection = %+v, want %+v", got.Requested, tt.in)
			}
			if reason := taskfailure.Classify(err.Error()); reason != taskfailure.ReasonAgentModelNotFoundOrUnavailable {
				t.Fatalf("failure reason = %q, want nonretryable model selection reason", reason)
			}
			if reads() != 1 {
				t.Fatalf("catalog reads = %d, want 1", reads())
			}
		})
	}
}

func TestResolveTaskModelSelectionForTaskPreservesUncertainSelections(t *testing.T) {
	tests := []struct {
		name       string
		catalog    agent.Catalog
		catalogErr error
		in         taskModelSelection
	}{
		{
			name: "unknown manual model",
			catalog: agent.Catalog{Models: []agent.Model{{
				ID: "gpt-known", Thinking: &agent.ModelThinking{SupportedLevels: []agent.ThinkingLevel{{Value: "high"}}},
			}}},
			in: taskModelSelection{Model: "gateway/future-model", ThinkingLevel: "ultra"},
		},
		{
			name: "fallback catalog",
			catalog: agent.Catalog{Fallback: true, Models: []agent.Model{{
				ID: "gpt-current", Thinking: &agent.ModelThinking{SupportedLevels: []agent.ThinkingLevel{{Value: "high"}}},
			}}},
			in: taskModelSelection{Model: "gpt-current", ThinkingLevel: "ultra"},
		},
		{
			name:       "discovery error",
			catalogErr: errors.New("catalog unavailable"),
			in:         taskModelSelection{Model: "gpt-current", ThinkingLevel: "ultra"},
		},
		{
			name:    "known model with unknown thinking capabilities",
			catalog: agent.Catalog{Models: []agent.Model{{ID: "gpt-current"}}},
			in:      taskModelSelection{Model: "gpt-current", ThinkingLevel: "ultra"},
		},
		{
			name:    "known model with unknown service tiers",
			catalog: agent.Catalog{Models: []agent.Model{{ID: "gpt-current"}}},
			in:      taskModelSelection{Model: "gpt-current", ServiceTier: "priority"},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			orig := listModels
			reads := 0
			listModels = func(context.Context, string, agent.Command) (agent.Catalog, error) {
				reads++
				return tt.catalog, tt.catalogErr
			}
			t.Cleanup(func() { listModels = orig })

			got, err := resolveTaskModelSelectionForTask(context.Background(), Task{IssueID: "issue-1"}, "codex", agent.Command{}, tt.in, quietTaskLog())
			if err != nil {
				t.Fatalf("resolve uncertain selection: %v", err)
			}
			if got.Effective == nil || *got.Effective != tt.in {
				t.Fatalf("effective selection = %+v, want %+v unchanged", got.Effective, tt.in)
			}
			if reads != 1 {
				t.Fatalf("catalog reads = %d, want 1", reads)
			}
		})
	}
}

func TestResolveTaskModelSelectionForTaskDefersEmptyCodexModelToStartup(t *testing.T) {
	in := taskModelSelection{ThinkingLevel: "ultra", ServiceTier: "priority"}
	reads := stubModelDiscovery(t, thinkingCatalogs())

	got, err := resolveTaskModelSelectionForTask(context.Background(), Task{IssueID: "issue-1"}, "codex", agent.Command{}, in, quietTaskLog())
	if err != nil {
		t.Fatalf("resolve empty-model Codex selection: %v", err)
	}
	if got.Launch != in {
		t.Fatalf("launch selection = %+v, want %+v unchanged", got.Launch, in)
	}
	if got.Effective != nil {
		t.Fatalf("effective selection = %+v before startup validation, want nil", *got.Effective)
	}
	if got.ValidateResolvedModelSelection == nil {
		t.Fatal("empty-model Codex override must be marked for validation after startup resolves the actual model")
	}
	if reads() != 1 {
		t.Fatalf("catalog reads = %d, want one catalog snapshot shared with startup preflight", reads())
	}
	if err := got.ValidateResolvedModelSelection("gpt-5.6-sol"); err == nil {
		t.Fatal("startup preflight accepted ultra for the actual gpt-5.6-sol catalog entry")
	}
}

func TestValidateResolvedCodexModelSelectionUsesActualModel(t *testing.T) {
	catalog := agent.Catalog{Models: []agent.Model{
		{ID: "catalog-display-default", Default: true, Thinking: &agent.ModelThinking{SupportedLevels: []agent.ThinkingLevel{{Value: "ultra"}}}},
		{ID: "gpt-high", Thinking: &agent.ModelThinking{SupportedLevels: []agent.ThinkingLevel{{Value: "high"}}}},
		{ID: "gpt-ultra", Thinking: &agent.ModelThinking{SupportedLevels: []agent.ThinkingLevel{{Value: "high"}, {Value: "ultra"}}}},
	}}
	requested := taskModelSelection{ThinkingLevel: "ultra"}

	if err := validateResolvedCodexModelSelection(catalog, "gpt-high", requested); err == nil {
		t.Fatal("actual model lacking ultra must reject the deferred override")
	}
	if err := validateResolvedCodexModelSelection(catalog, "gpt-ultra", requested); err != nil {
		t.Fatalf("actual model supporting ultra rejected: %v", err)
	}
	if err := validateResolvedCodexModelSelection(agent.Catalog{Fallback: true, Models: catalog.Models}, "gpt-high", requested); err != nil {
		t.Fatalf("fallback catalog must not validate the actual model: %v", err)
	}
}

func TestResolveTaskModelSelectionForTaskPreservesLegacyAndSpecializedBehavior(t *testing.T) {
	specialized := []Task{
		{},
		{IssueID: "issue-1", ChatSessionID: "chat-1"},
		{IssueID: "issue-1", AutopilotRunID: "auto-1"},
		{IssueID: "issue-1", IsLeaderTask: true, LeaderRoleResolved: true},
		{IssueID: "issue-1", QuickCreatePrompt: "create"},
		{IssueID: "issue-1", RegenerateQuickActionsFor: "task-1"},
		{IssueID: "issue-1", TestRunContext: json.RawMessage(`{"type":"test_run"}`)},
		{IssueID: "issue-1", DesignDocumentContext: []byte(`{}`)},
		{IssueID: "issue-1", PMOSyncContext: []byte(`{}`)},
	}

	for i, task := range specialized {
		t.Run(string(rune('a'+i)), func(t *testing.T) {
			reads := stubModelDiscovery(t, thinkingCatalogs())
			in := taskModelSelection{Model: "gpt-5.6-sol", ThinkingLevel: "ultra"}
			got, err := resolveTaskModelSelectionForTask(context.Background(), task, "codex", agent.Command{}, in, quietTaskLog())
			if err != nil {
				t.Fatalf("legacy/specialized selection rejected: %v", err)
			}
			want := taskModelSelection{Model: "gpt-5.6-sol"}
			if got.Effective == nil || *got.Effective != want {
				t.Fatalf("effective selection = %+v, want legacy result %+v", got.Effective, want)
			}
			if reads() != 1 {
				t.Fatalf("catalog reads = %d, want 1", reads())
			}
		})
	}
}
