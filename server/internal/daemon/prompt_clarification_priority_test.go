package daemon

import (
	"strings"
	"testing"
)

func TestIssueClarificationPrecedesFinalBlockedOutcome(t *testing.T) {
	snapshot := freshIssueSnapshot("issue-1")
	snapshot.Status = "in_progress"
	snapshot.Revision = 2
	snapshot.ETag = `W/"issue:issue-1:2"`
	for _, direct := range []bool{false, true} {
		for _, comment := range []bool{false, true} {
			task := Task{
				IssueID: "issue-1", ClaimGeneration: 7, IssueCompletionContractVersion: 1,
				IssueStartContractVersion: 1, IssueSnapshot: snapshot,
				RemoteMCPDaemonToken: "mdt_do_not_render",
			}
			if comment {
				task.TriggerCommentID = "comment-1"
				task.TriggerCommentContent = "Ask before choosing the threshold rule."
			}
			prompt := BuildPrompt(task, "codex")
			if direct {
				prompt = BuildDirectPrompt(task)
			}
			for _, required := range []string{
				"request_user_input", "AskUserQuestion", "wait for its answer",
				"same task and session", "Do not write a blocked outcome while waiting",
				"unavailable or fails",
			} {
				if !strings.Contains(prompt, required) {
					t.Errorf("direct=%v comment=%v: missing %q", direct, comment, required)
				}
			}
			if strings.Contains(prompt, "If required input is missing or verification cannot be completed") {
				t.Errorf("direct=%v comment=%v: unconditional final-blocked instruction conflicts with native clarification", direct, comment)
			}
			if strings.Contains(prompt, task.RemoteMCPDaemonToken) {
				t.Fatal("daemon credential rendered into the task prompt")
			}
		}
	}
}
