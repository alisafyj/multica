package daemon

import (
	"fmt"
	"strings"
	"testing"

	"github.com/multica-ai/multica/server/internal/daemon/execenv"
)

func TestCompactIssueBriefAndTurnPromptKeepOneDeliveryAuthority(t *testing.T) {
	for _, provider := range []string{"codex", "claude"} {
		for _, artifact := range []bool{false, true} {
			t.Run(fmt.Sprintf("%s/artifact=%t", provider, artifact), func(t *testing.T) {
				task := Task{IssueID: "issue-1", ClaimGeneration: 7, IssueCompletionContractVersion: 1,
					IssueSnapshot: freshIssueSnapshot("issue-1"),
				}
				if artifact {
					task.IssueStartContractVersion = 1
				}
				brief := execenv.RenderRuntimeBrief(provider, execenv.TaskContextForEnv{
					IssueID: task.IssueID, IssueCompletionContractVersion: 1,
				})
				combined := brief + "\n" + BuildPrompt(task, provider)
				for _, required := range []string{
					"Discover command flags on demand",
					"Do not run `multica issue comment add` for this final response",
					"to every covered trigger thread", "The default `delivered` outcome does not move issue status",
					"use the native `request_user_input` tool in Codex or `AskUserQuestion` in Claude Code",
					"wait for its answer before continuing the same task and session",
					"Do not write a blocked outcome while waiting", "If the native question tool is unavailable or fails",
					"--attachment <path>", "--content-file <path>", "Runtime-local paths are never deliverables",
					"FIRST follow the clarification rules", "Successful process exit alone is not verified delivery",
				} {
					if !strings.Contains(combined, required) {
						t.Errorf("combined instructions lost %q", required)
					}
				}
				if count := strings.Count(combined, "\n## Final Issue Delivery\n"); count != 1 {
					t.Errorf("delivery definitions = %d, want 1", count)
				}
				if strings.Contains(brief, IssueOutcomeFileEnv) || strings.Contains(combined, IssueOutcomeFileEnv) != artifact {
					t.Error("outcome file availability does not follow the per-turn start contract")
				}
				for _, forbidden := range []string{"Final results MUST be delivered", "**Post your final results as a comment", "**Post exactly ONE comment per run"} {
					if strings.Contains(combined, forbidden) {
						t.Errorf("contradictory manual delivery rule: %q", forbidden)
					}
				}
			})
		}
	}
}
