package execenv

import (
	"fmt"
	"strings"
	"testing"
)

func TestRuntimeBriefCompletionContractRemovesConflictingFinalDelivery(t *testing.T) {
	for _, provider := range []string{"codex", "claude"} {
		for _, leader := range []bool{false, true} {
			t.Run(fmt.Sprintf("%s/leader=%t", provider, leader), func(t *testing.T) {
				ctx := TaskContextForEnv{IssueID: "issue-1", IsSquadLeader: leader}
				legacy := RenderRuntimeBrief(provider, ctx)
				ctx.IssueCompletionContractVersion = 1
				brief := RenderRuntimeBrief(provider, ctx)
				for _, forbidden := range []string{"Final results MUST be delivered", "**Post your final results as a comment", "**Post exactly ONE comment per run", "text in your terminal or run logs is NOT delivered"} {
					if strings.Contains(brief, forbidden) {
						t.Errorf("completion-capable brief retains conflicting instruction %q", forbidden)
					}
				}
				requiredContracts := []string{"## Final Issue Delivery", "Agent Identity", "Runtime-local paths are never deliverables", "--attachment <path>"}
				if leader {
					requiredContracts = append(requiredContracts, "## Instruction Precedence", "## Authoritative Issue Body Snapshot", "this is mandatory, not optional", "skip any status call your Agent Identity forbids")
				} else {
					// Ordinary issues take the concrete read decision from the current turn.
					requiredContracts = append(requiredContracts, "Agent Identity wins", "THIS turn's issue-body snapshot/read", "mandatory bounded comment catch-up", "Agent Identity forbids the write")
				}
				for _, required := range requiredContracts {
					if !strings.Contains(brief, required) {
						t.Errorf("completion-capable brief lost required contract %q", required)
					}
				}
				if len(brief) >= len(legacy)-500 {
					t.Errorf("duplicate delivery reduction too small: legacy=%d capable=%d", len(legacy), len(brief))
				}
				if !strings.Contains(legacy, "Final results MUST be delivered") {
					t.Error("legacy CLI delivery changed")
				}
				resumed := ctx
				resumed.TriggerCommentID = "comment-2"
				resumed.PriorSessionResumed = true
				if got := RenderRuntimeBrief(provider, resumed); got != brief {
					t.Error("same completion capability changed stable brief on resume")
				}
			})
		}
	}
}

func TestRuntimeBriefCompletionContractRequiresKnownIssueCapability(t *testing.T) {
	for _, ctx := range []TaskContextForEnv{
		{}, {IssueID: "issue-1"}, {IssueID: "issue-1", ChatSessionID: "chat-1"},
		{IssueID: "issue-1", QuickCreatePrompt: "new issue"}, {IssueID: "issue-1", AutopilotRunID: "run-1"},
	} {
		baseline := RenderRuntimeBrief("codex", ctx)
		for _, version := range []int{-1, 0, 2} {
			ctx.IssueCompletionContractVersion = version
			if got := RenderRuntimeBrief("codex", ctx); got != baseline {
				t.Errorf("unrecognized completion version %d changed brief", version)
			}
		}
		if ctx.IssueID == "" || ctx.ChatSessionID != "" || ctx.QuickCreatePrompt != "" || ctx.AutopilotRunID != "" {
			ctx.IssueCompletionContractVersion = 1
			if got := RenderRuntimeBrief("codex", ctx); got != baseline {
				t.Error("completion capability changed a non-issue surface")
			}
		}
	}
}
