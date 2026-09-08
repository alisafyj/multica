package execenv

import (
	"strings"
	"testing"
)

func TestRenderRuntimeBriefAllowsOnlyValidatedIssueBodySnapshotToReplaceIssueGet(t *testing.T) {
	brief := RenderRuntimeBrief("claude", TaskContextForEnv{IssueID: "issue-1"})

	for _, want := range []string{
		"unless the per-turn message contains a validated `## Authoritative Issue Body Snapshot`",
		"this is mandatory, not optional",
		"A validated resumed turn",
		"satisfies this step with its bounded delta or thread read",
		"scan every thread cheaply (`--roots-only --summary --compact`)",
		"Earlier comments often carry context the issue body lacks",
	} {
		if !strings.Contains(brief, want) {
			t.Fatalf("runtime brief missing snapshot/read contract %q:\n%s", want, brief)
		}
	}
	if strings.Contains(brief, "snapshot satisfies comment") {
		t.Fatalf("runtime brief allowed issue body snapshot to replace comment catch-up:\n%s", brief)
	}
}
