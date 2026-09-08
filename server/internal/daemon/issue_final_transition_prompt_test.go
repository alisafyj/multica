package daemon

import (
	"strings"
	"testing"
	"time"
)

func TestIssueFinalTransitionUsesOneChannel(t *testing.T) {
	for _, provider := range []string{"codex", "claude"} {
		for _, direct := range []bool{false, true} {
			for _, comment := range []bool{false, true} {
				task := Task{
					IssueID: "issue-1", ClaimGeneration: 7,
					IssueStartContractVersion: 1, IssueCompletionContractVersion: 1,
					IssueSnapshot: freshIssueSnapshot("issue-1"),
				}
				if comment {
					task.TriggerCommentID = "comment-1"
					task.TriggerThreadID = "thread-1"
					task.TriggerCommentContent = "Complete the requested change."
					task.IssueSnapshot.Trigger = &IssueTaskTriggerSnapshot{
						CommentID: task.TriggerCommentID, ThreadID: task.TriggerThreadID, ContentEmbedded: true,
					}
				}
				prompt := BuildPrompt(task, provider)
				if direct {
					prompt = BuildDirectPrompt(task)
				}
				for _, want := range []string{
					"Use this artifact instead of a separate status or review command for the same final transition",
					"Mid-turn status changes and explicitly requested CLI status or review commands remain available",
					"If you already applied the final transition through the CLI, omit the artifact",
					"required verification has passed", "exactly one JSON object", "does not infer",
				} {
					if !strings.Contains(prompt, want) {
						t.Errorf("provider=%s direct=%t comment=%t missing %q", provider, direct, comment, want)
					}
				}
				if strings.Count(prompt, "Use this artifact instead of") != 1 {
					t.Errorf("provider=%s direct=%t comment=%t final transition rule must occur once", provider, direct, comment)
				}
				for _, duplicate := range []string{"in_review --no-start", "blocked --no-start"} {
					if strings.Contains(prompt, duplicate) {
						t.Errorf("provider=%s direct=%t comment=%t duplicates final transition with %q", provider, direct, comment, duplicate)
					}
				}
			}
		}
	}
}

func TestIssueFinalTransitionDoesNotAdvertiseUnavailableArtifact(t *testing.T) {
	cases := map[string]func(*Task){
		"missing start":    func(task *Task) { task.IssueStartContractVersion = 0 },
		"unknown start":    func(task *Task) { task.IssueStartContractVersion = 2 },
		"missing claim":    func(task *Task) { task.ClaimGeneration = 0 },
		"missing snapshot": func(task *Task) { task.IssueSnapshot = nil },
		"incomplete":       func(task *Task) { task.IssueSnapshot.Complete = false },
		"expired": func(task *Task) {
			task.IssueSnapshot.ExpiresAt = time.Now().Add(-time.Second).UTC().Format(time.RFC3339Nano)
		},
		"rejected start": func(task *Task) { applyIssueStartState(task, &IssueStartState{BaselineAccepted: false}) },
	}
	for name, mutate := range cases {
		t.Run(name, func(t *testing.T) {
			task := Task{
				IssueID: "issue-1", ClaimGeneration: 7,
				IssueStartContractVersion: 1, IssueCompletionContractVersion: 1,
				IssueSnapshot: freshIssueSnapshot("issue-1"),
			}
			mutate(&task)
			prompt := buildIssueCompletionDeliveryPrompt(task)
			if strings.Contains(prompt, IssueOutcomeFileEnv) || strings.Contains(prompt, "Use this artifact instead of") {
				t.Fatal("unavailable artifact replaced explicit status commands")
			}
			if !strings.Contains(prompt, "Existing explicit status or review commands remain available") {
				t.Fatal("non-artifact final delivery lost explicit status commands")
			}
			direct := BuildDirectPrompt(task)
			for _, command := range []string{"in_review --no-start", "blocked --no-start"} {
				if !strings.Contains(direct, command) {
					t.Errorf("non-artifact direct prompt lost %q", command)
				}
			}
		})
	}
}

func TestIssueFinalTransitionRetainsCapturedCapabilityDecision(t *testing.T) {
	task := Task{
		IssueID: "issue-1", ClaimGeneration: 7,
		IssueStartContractVersion: 1, IssueCompletionContractVersion: 1,
		IssueSnapshot: freshIssueSnapshot("issue-1"),
	}
	available := taskSupportsIssueOutcomeArtifact(task)
	if !available {
		t.Fatal("fresh fixture did not authorize the outcome artifact")
	}
	task.IssueSnapshot.ExpiresAt = time.Now().Add(-time.Second).UTC().Format(time.RFC3339Nano)
	if taskSupportsIssueOutcomeArtifact(task) {
		t.Fatal("expired fixture unexpectedly authorized a new outcome decision")
	}
	for _, decision := range []bool{available, false} {
		prompt := buildIssueCompletionDeliveryPromptWithOutcome(task, decision)
		if strings.Contains(prompt, "Use this artifact instead of") != decision {
			t.Fatal("rendering changed the previously captured capability decision")
		}
		if strings.Contains(prompt, "Existing explicit status or review commands remain available") == decision {
			t.Fatal("rendering advertised both final transition channels")
		}
	}
}
