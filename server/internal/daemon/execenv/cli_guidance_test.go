package execenv

import (
	"runtime"
	"strings"
	"testing"
)

func TestRuntimeHeaderUsesDaemonCLIOnPOSIX(t *testing.T) {
	t.Parallel()
	if runtime.GOOS == "windows" {
		t.Skip("POSIX-only runtime header guidance")
	}

	out := buildMetaSkillContent("claude", TaskContextForEnv{IssueID: "issue-1"})
	for _, want := range []string{
		`"$MULTICA_CLI" ...`,
		"daemon's exact binary",
		"missing or empty",
		"stop running Multica platform commands",
		"runtime configuration is missing",
		"do not search for credentials or modify PATH",
	} {
		if !strings.Contains(out, want) {
			t.Errorf("runtime header missing POSIX CLI guidance %q", want)
		}
	}
	if strings.Contains(out, `${MULTICA_CLI:-multica}`) || strings.Contains(out, "fallback `multica`") {
		t.Fatalf("runtime header must not allow PATH fallback:\n%s", out)
	}
}

func TestMulticaCLIInvocationGuidanceOSBoundary(t *testing.T) {
	t.Parallel()

	posix := BuildMulticaCLIInvocationGuidance("linux")
	if !strings.Contains(posix, `"$MULTICA_CLI" ...`) || strings.Contains(posix, "$env:") {
		t.Fatalf("POSIX guidance is not exact or introduced PowerShell env access: %q", posix)
	}
	for _, want := range []string{"missing or empty", "stop running Multica platform commands", "runtime configuration is missing"} {
		if !strings.Contains(posix, want) {
			t.Errorf("POSIX guidance missing fail-closed instruction %q: %q", want, posix)
		}
	}
	if strings.Contains(posix, `${MULTICA_CLI:-multica}`) || strings.Contains(posix, "fallback `multica`") {
		t.Fatalf("POSIX guidance must not allow PATH fallback: %q", posix)
	}
	if got := BuildMulticaCLIInvocationGuidance("windows"); got != "" {
		t.Fatalf("Windows guidance = %q, want empty to preserve the privacy boundary", got)
	}
}
