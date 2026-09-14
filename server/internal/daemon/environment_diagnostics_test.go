package daemon

import (
	"encoding/json"
	"strings"
	"testing"

	"github.com/multica-ai/multica/server/pkg/taskfailure"
)

func TestAgentEnvironmentDiagnosticsPreserveExplicitAuthorization(t *testing.T) {
	inherited := []string{"HTTPS_PROXY=https://private-user:secret@proxy.invalid", "ANTHROPIC_BASE_URL=https://private.invalid", "GOPROXY=https://mirror.invalid", "PRIVATE_TOKEN=hidden", "SSL_CERT_FILE=/private/cert"}
	custom := map[string]string{"HTTPS_PROXY": "http://explicit:secret@proxy.invalid", "ANTHROPIC_BASE_URL": "", "GOPROXY": "https://approved.invalid"}
	diagnostics := agentEnvironmentDiagnostics("claude", inherited, custom)
	want := map[string]string{"HTTPS_PROXY": "explicit", "ANTHROPIC_BASE_URL": "explicit_empty", "GOPROXY": "explicit", "SSL_CERT_FILE": "inherited_allowed"}
	for _, diagnostic := range diagnostics {
		if diagnostic.Source != want[diagnostic.Key] {
			t.Fatalf("unexpected key/source: %+v", diagnostic)
		}
		delete(want, diagnostic.Key)
	}
	if len(want) != 0 {
		t.Fatalf("missing sources: %v", want)
	}
	raw, err := json.Marshal(diagnostics)
	if err != nil {
		t.Fatal(err)
	}
	for _, secret := range []string{"private-user", "secret", "hidden", "mirror.invalid", "approved.invalid", "/private", "PRIVATE_TOKEN"} {
		if strings.Contains(string(raw), secret) {
			t.Fatalf("diagnostic leaked %q", secret)
		}
	}
}

func TestAgentEnvironmentDiagnosticsReportDroppedNotMissing(t *testing.T) {
	diagnostics := agentEnvironmentDiagnostics("codex", []string{"OPENAI_BASE_URL=https://hidden.invalid", "https_proxy=http://hidden.invalid", "ANTHROPIC_MODEL=other-provider"}, nil)
	if len(diagnostics) != 2 {
		t.Fatalf("unexpected diagnostics: %+v", diagnostics)
	}
	for _, diagnostic := range diagnostics {
		if diagnostic.Source != "not_inherited" {
			t.Fatalf("unexpected source: %+v", diagnostic)
		}
	}
	if got := agentEnvironmentDiagnostics("claude", nil, nil); len(got) != 0 {
		t.Fatalf("absence must not invent missing native config: %+v", got)
	}
}

func TestEnvironmentFailureHintDoesNotChangeClassificationOrLeakValues(t *testing.T) {
	diagnostics := agentEnvironmentDiagnostics("codex", []string{"HTTPS_PROXY=https://secret.invalid", "OPENAI_BASE_URL=https://secret.invalid"}, nil)
	reason := taskfailure.ReasonAgentProviderNetwork.String()
	got := annotateAgentEnvironmentFailure("connection refused", reason, diagnostics)
	if !strings.Contains(got, "HTTPS_PROXY, OPENAI_BASE_URL") || !strings.Contains(got, "custom_env") || strings.Contains(got, "secret.invalid") {
		t.Fatalf("invalid safe diagnostic: %s", got)
	}
	if taskfailure.Classify(got) != taskfailure.ReasonAgentProviderNetwork {
		t.Fatalf("hint changed classification: %s", got)
	}
	for _, unrelated := range []string{"timeout", taskfailure.ReasonAgentUnknown.String(), taskfailure.ReasonAgentModelNotFoundOrUnavailable.String()} {
		if got := annotateAgentEnvironmentFailure("original", unrelated, diagnostics); got != "original" {
			t.Fatalf("annotated unrelated failure %s", unrelated)
		}
	}
	if got := annotateAgentEnvironmentFailure("connection refused", reason, nil); got != "connection refused" {
		t.Fatalf("unobserved config must not be blamed: %s", got)
	}
	explicit := agentEnvironmentDiagnostics("codex", nil, map[string]string{"HTTPS_PROXY": "http://private.invalid"})
	if got := annotateAgentEnvironmentFailure("connection refused", reason, explicit); got != "connection refused" {
		t.Fatalf("explicit config was reported missing: %s", got)
	}
}
