package daemon

import (
	"fmt"
	"strings"
	"testing"
)

func TestIssueCodeVerificationInOrdinaryPrompts(t *testing.T) {
	builders := []struct {
		name  string
		build func(Task) string
	}{
		{"claude", func(task Task) string { return BuildPrompt(task, "claude") }},
		{"codex", func(task Task) string { return BuildPrompt(task, "codex") }},
		{"direct", func(task Task) string { return BuildDirectPrompt(task) }},
	}
	for _, builder := range builders {
		for _, version := range []int{0, 1} {
			for _, trigger := range []string{"", "comment-1"} {
				task := Task{IssueID: "issue-1", IssueCompletionContractVersion: version,
					TriggerCommentID: trigger, TriggerCommentContent: "Keep this exact request."}
				t.Run(fmt.Sprintf("%s/trigger=%q/contract=%d", builder.name, trigger, version), func(t *testing.T) {
					prompt := builder.build(task)
					if strings.Count(prompt, "## Code Verification\n\n") != 1 {
						t.Fatal("ordinary issue prompt must include one code verification contract")
					}
					if trigger != "" && !strings.Contains(prompt, task.TriggerCommentContent) {
						t.Fatal("verification contract lost trigger input")
					}
					if version == 1 && !strings.Contains(prompt, "Multica posts that exact final response") {
						t.Fatal("verification contract lost platform delivery")
					}
				})
			}
		}
	}
}

func TestIssueCodeVerificationIsBoundedAndBehavioral(t *testing.T) {
	section := issueCodeVerificationPrompt
	prompt := BuildPrompt(Task{IssueID: "issue-1", IssueCompletionContractVersion: 1}, "claude")
	if !strings.Contains(prompt, section) {
		t.Fatal("missing complete code verification contract")
	}
	parts := strings.Split(strings.TrimSpace(section), "\n\n")
	if len(parts) != 2 || parts[0] != "## Code Verification" || strings.TrimSpace(parts[1]) == "" {
		t.Fatal("verification contract must remain a single bounded paragraph")
	}
	if len(section) > 900 {
		t.Fatalf("verification contract is %d bytes, exceeds 900", len(section))
	}
	for _, r := range section {
		if r > 127 {
			t.Fatal("verification contract must be ASCII")
		}
	}
	for _, want := range []string{"When changing code", "fails before the fix", "representative nonempty data", "real component", "state transitions", "allowed and rejected", "smallest relevant tests", "actually run", "verification gaps"} {
		if !strings.Contains(section, want) {
			t.Errorf("missing behavior verification requirement %q", want)
		}
	}
	for _, unwanted := range []string{"StockFlow", "T4", "viewer", "Playwright", "always run the full", "subagent", "--model"} {
		if strings.Contains(section, unwanted) {
			t.Errorf("verification guidance must not prescribe %q", unwanted)
		}
	}
}

func TestIssueCodeVerificationExcludesOtherWorkflows(t *testing.T) {
	for name, task := range map[string]Task{
		"chat":                {IssueID: "issue-1", ChatSessionID: "chat-1", ChatMessage: "Status only"},
		"quick-create":        {QuickCreatePrompt: "Create a task"},
		"autopilot":           {AutopilotRunID: "run-1"},
		"ui-draft":            {UIDraftCreateContext: []byte(`{"id":"draft-1"}`)},
		"design-restore":      {DesignRestoreContext: []byte(`{"id":"restore-1"}`)},
		"test-generation":     {TestGenerationContext: []byte(`{"id":"generation-1"}`)},
		"test-run":            {TestRunContext: []byte(`{"id":"test-1"}`)},
		"design-analysis":     {DesignSystemProfileAnalyzeContext: []byte(`{"id":"analysis-1"}`)},
		"template-analysis":   {TemplateBlueprintAnalyzeContext: []byte(`{"id":"template-1"}`)},
		"project-design":      {ProjectDesignSystemContext: []byte(`{"id":"project-1"}`)},
		"repository-analysis": {ProjectDesignSystemContext: []byte(`{"type":"project_design_system_task","operation":"repository_analysis"}`)},
		"design-document":     {DesignDocumentContext: []byte(`{"id":"document-1"}`)},
		"pmo":                 {PMOSyncContext: []byte(`{"id":"pmo-1"}`)},
	} {
		t.Run(name, func(t *testing.T) {
			for _, prompt := range []string{BuildPrompt(task, "claude"), BuildDirectPrompt(task)} {
				if strings.Contains(prompt, "## Code Verification") {
					t.Fatal("ordinary code verification contract leaked into another workflow")
				}
			}
		})
	}
	t.Run("direct-design-delivery", func(t *testing.T) {
		prompt := BuildDirectPrompt(Task{DesignDeliveryContext: []byte(`{"id":"delivery-1"}`)})
		if strings.Contains(prompt, "## Code Verification") {
			t.Fatal("ordinary code verification contract leaked into direct design delivery")
		}
	})
}
