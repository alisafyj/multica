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
		PMOSyncContext: json.RawMessage(raw),
	}

	out := BuildPrompt(task, "claude")

	for _, want := range []string{
		"You are running as a PMO requirement sync agent",
		"Return JSON only",
		"EXT-P-001",
		"owner.external_id",
		"@soyoung.com",
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

// TestBuildPromptPMOSyncOpenClawInvokesDataQuerySkill locks the provider-specific
// instruction required for OpenClaw to load the installed PMO acquisition skill.
func TestBuildPromptPMOSyncOpenClawInvokesDataQuerySkill(t *testing.T) {
	ctx := service.PMOSyncContext{
		Type:            service.PMOSyncContextType,
		WorkspaceID:     "0f2b6f6e-0000-4000-8000-000000000001",
		RunID:           "0f2b6f6e-0000-4000-8000-000000000002",
		RootExternalKey: "SY-P-20260452",
		Prompt:          service.BuildPMOSyncPrompt("SY-P-20260452"),
	}
	raw, err := json.Marshal(ctx)
	if err != nil {
		t.Fatalf("marshal pmo context: %v", err)
	}

	out := BuildPrompt(Task{PMOSyncContext: json.RawMessage(raw)}, "openclaw")
	want := `/skill:sy-pmo-data-query snapshot "SY-P-20260452"`
	if strings.TrimSpace(out) != want {
		t.Fatalf("OpenClaw PMO prompt = %q, want %q", out, want)
	}
	if strings.Contains(out, "$sy-pmo-data-query") {
		t.Fatalf("OpenClaw PMO prompt used Codex skill syntax instead of an OpenClaw slash skill:\n%s", out)
	}
}

func TestBuildPromptPMOSyncOpenClawEscapesRootKey(t *testing.T) {
	ctx := service.PMOSyncContext{
		Type:            service.PMOSyncContextType,
		RootExternalKey: "EXT-\\\"-LINE\n2",
		Prompt:          service.BuildPMOSyncPrompt("EXT-unsafe"),
	}
	raw, err := json.Marshal(ctx)
	if err != nil {
		t.Fatalf("marshal pmo context: %v", err)
	}

	out := BuildPrompt(Task{PMOSyncContext: json.RawMessage(raw)}, "openclaw")
	wantArg, err := json.Marshal(ctx.RootExternalKey)
	if err != nil {
		t.Fatalf("marshal root key: %v", err)
	}
	want := "/skill:sy-pmo-data-query snapshot " + string(wantArg)
	if strings.TrimSpace(out) != want {
		t.Fatalf("OpenClaw PMO prompt = %q, want %q", out, want)
	}
	if strings.Count(out, "\n") != 1 {
		t.Fatalf("OpenClaw PMO prompt contains an injected newline: %q", out)
	}
}

func TestBuildPromptPMOSyncOpenClawLegacyContextFallsBack(t *testing.T) {
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

	out := BuildPrompt(Task{PMOSyncContext: json.RawMessage(raw)}, "openclaw")
	if strings.Contains(out, "/skill:sy-pmo-data-query") {
		t.Fatalf("legacy PMO context without root_external_key must not synthesize a skill argument: %s", out)
	}
	if !strings.Contains(out, "Return JSON only") || !strings.Contains(out, "EXT-P-001") {
		t.Fatalf("legacy PMO context lost its strict fallback prompt: %s", out)
	}
}

func TestBuildPromptPMOSyncOpenClawProviderIsolation(t *testing.T) {
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

	for _, provider := range []string{"claude", "opencode"} {
		out := BuildPrompt(Task{PMOSyncContext: json.RawMessage(raw)}, provider)
		if strings.Contains(out, "/skill:sy-pmo-data-query") {
			t.Fatalf("%s PMO prompt leaked OpenClaw skill instruction:\n%s", provider, out)
		}
	}
}

// TestBuildPromptPMOSyncFallback guards the degenerate case: a PMO task whose
// context cannot be parsed still renders a strict JSON-only instruction
// instead of falling through to the issue prompt.
func TestBuildPromptPMOSyncFallback(t *testing.T) {
	out := buildPromptBody(Task{PMOSyncContext: json.RawMessage("not-json")}, "claude", "")
	if !strings.Contains(out, "PMO requirement sync agent") {
		t.Fatalf("fallback prompt missing framing:\n%s", out)
	}
	if !strings.Contains(out, "JSON object only") {
		t.Fatalf("fallback prompt missing strict JSON contract:\n%s", out)
	}
	if strings.Contains(out, "assigned issue ID") {
		t.Fatalf("PMO fallback fell through to issue prompt:\n%s", out)
	}

	openClawOut := buildPromptBody(Task{PMOSyncContext: json.RawMessage("not-json")}, "openclaw", "")
	if strings.Contains(openClawOut, "/skill:sy-pmo-data-query") {
		t.Fatalf("malformed OpenClaw PMO context must not invoke the data skill:\n%s", openClawOut)
	}
}

// TestTaskPMOSyncContextJSONObjectDecode locks the regression where the
// daemon decoded the server's claim response (pmo_sync_context is a JSON
// object) into a string field and silently dropped it, so the PMO builder
// never ran and the agent got the generic issue prompt.
func TestTaskPMOSyncContextJSONObjectDecode(t *testing.T) {
	ctx := service.PMOSyncContext{
		Type:        service.PMOSyncContextType,
		WorkspaceID: "0f2b6f6e-0000-4000-8000-000000000001",
		RunID:       "0f2b6f6e-0000-4000-8000-000000000002",
		Prompt:      service.BuildPMOSyncPrompt("EXT-P-001"),
	}
	rawCtx, err := json.Marshal(ctx)
	if err != nil {
		t.Fatalf("marshal pmo context: %v", err)
	}
	// Shape mirrors server/internal/handler/daemon.go: resp.PMOSyncContext =
	// json.RawMessage(task.Context) inside the claim response.
	claim := map[string]any{
		"id":               "pmo-task-1",
		"workspace_id":     "pmo-workspace-1",
		"pmo_sync_context": json.RawMessage(rawCtx),
	}
	claimJSON, err := json.Marshal(claim)
	if err != nil {
		t.Fatalf("marshal claim: %v", err)
	}

	var task Task
	if err := json.Unmarshal(claimJSON, &task); err != nil {
		t.Fatalf("unmarshal claim into task: %v", err)
	}
	if len(task.PMOSyncContext) == 0 {
		t.Fatal("PMOSyncContext dropped during claim decode; PMO prompt branch can never run")
	}
	out := BuildPrompt(task, "claude")
	if !strings.Contains(out, "PMO requirement sync agent") || !strings.Contains(out, "EXT-P-001") {
		t.Fatalf("PMO prompt not rendered after decode:\n%s", out)
	}
}
