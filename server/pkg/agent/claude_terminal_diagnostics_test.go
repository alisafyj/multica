package agent

import (
	"fmt"
	"runtime"
	"strings"
	"testing"
)

func TestClaudeEmptyErrorResultPreservesSafeSubtype(t *testing.T) {
	t.Parallel()
	if runtime.GOOS == "windows" {
		t.Skip("shell-script fixture is POSIX-only")
	}
	for _, subtype := range []string{"success", "error_max_turns", "error_during_execution", "error_max_budget_usd", "error_max_structured_output_retries", "synthetic-private-provider-detail"} {
		t.Run(subtype, func(t *testing.T) {
			t.Parallel()
			frames := `{"type":"system","session_id":"terminal-diagnostic"}` + "\n" +
				fmt.Sprintf(`{"type":"result","session_id":"terminal-diagnostic","subtype":%q,"is_error":true,"num_turns":31,"result":""}`, subtype) + "\n"
			result := runClaudeFixture(t, frames, "")
			if result.Status != "failed" || result.Output != "" {
				t.Fatalf("error result became success: %+v", result)
			}
			want := subtype
			if strings.HasPrefix(subtype, "synthetic-private") {
				want = "unknown"
				if strings.Contains(result.Error, subtype) {
					t.Fatal("unknown provider subtype leaked into error")
				}
			}
			if !strings.Contains(result.Error, "subtype="+want) || !strings.Contains(result.Error, "turns=31") {
				t.Fatalf("missing safe terminal diagnostics: %q", result.Error)
			}
		})
	}
}
