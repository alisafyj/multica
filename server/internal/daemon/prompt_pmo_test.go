package daemon

import (
	"encoding/json"
	"strings"
	"testing"

	"github.com/multica-ai/multica/server/internal/service"
)

// TestBuildPromptPMOSyncStrictAndClean locks the daemon-side rendering of the
// PMO acquisition prompt: the enqueue-time prompt (service.BuildPMOSyncPrompt)
// is authoritative and must reach the agent verbatim under the standard
// framing, and the rendered result must stay strict (JSON-only contract) and
// free of any URL / infrastructure reference.
func TestBuildPromptPMOSyncStrictAndClean(t *testing.T) {
	ctx := service.PMOSyncContext{
		Type:        service.PMOSyncContextType,
		WorkspaceID: "0f2b6f6e-0000-4000-8000-000000000001",
		RunID:       "0f2b6f6e-0000-4000-8000-000000000002",
		Prompt:      service.BuildPMOSyncPrompt("EXT-P-001"),
	}
	raw, err := json.Marshal(ctx)
	if err != nil {
		t.Fatalf("marshal pmo context: %v", err)
	}
	task := Task{
		ID:             "pmo-task-1",
		WorkspaceID:    "pmo-workspace-1",
		PMOSyncContext: string(raw),
	}

	out := BuildPrompt(task, "claude")

	for _, want := range []string{
		"You are running as a PMO requirement sync agent",
		"Return JSON only",
		"EXT-P-001",
	} {
		if !strings.Contains(out, want) {
			t.Fatalf("PMO prompt missing %q:\n%s", want, out)
		}
	}
	if strings.Contains(out, "://") {
		t.Fatalf("PMO prompt must not contain any URL fragment:\n%s", out)
	}
	// Branch selection: the PMO task must not fall through to the issue,
	// chat, autopilot, quick-create, or design-task prompt builders.
	for _, forbidden := range []string{
		"assigned issue ID",
		"quick-create modal",
		"design restore context JSON",
		"UI draft context JSON",
	} {
		if strings.Contains(out, forbidden) {
			t.Fatalf("PMO prompt leaked %q from another builder:\n%s", forbidden, out)
		}
	}
}

// TestBuildPromptPMOSyncFallback guards the degenerate case: a PMO task whose
// context cannot be parsed still renders a strict JSON-only instruction
// instead of falling through to the issue prompt.
func TestBuildPromptPMOSyncFallback(t *testing.T) {
	out := buildPromptBody(Task{PMOSyncContext: "not-json"}, "claude")
	if !strings.Contains(out, "PMO requirement sync agent") {
		t.Fatalf("fallback prompt missing framing:\n%s", out)
	}
	if !strings.Contains(out, "JSON object only") {
		t.Fatalf("fallback prompt missing strict JSON contract:\n%s", out)
	}
	if strings.Contains(out, "assigned issue ID") {
		t.Fatalf("PMO fallback fell through to issue prompt:\n%s", out)
	}
}
