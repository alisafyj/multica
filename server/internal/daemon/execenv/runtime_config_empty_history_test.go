package execenv

import (
	"strings"
	"testing"
)

func TestIssueWorkflowRecognizesOnlyVerifiedEmptyHistory(t *testing.T) {
	for _, provider := range []string{"claude", "codex"} {
		brief := buildMetaSkillContent(provider, TaskContextForEnv{IssueID: "issue-1"})
		for _, required := range []string{
			"## Verified Empty Comment History", "at task start", "later comments",
			"mandatory, not optional", "--roots-only --summary --compact",
		} {
			if !strings.Contains(brief, required) {
				t.Fatalf("%s brief missing %q", provider, required)
			}
		}
	}
}
