package daemon

import (
	"strings"
	"testing"
)

func TestBuildDirectPromptKeepsOnlyTaskInput(t *testing.T) {
	tests := []struct {
		name string
		task Task
		want string
	}{
		{
			name: "comment",
			task: Task{IssueID: "issue-1", TriggerCommentID: "comment-1", TriggerCommentContent: "Please inspect the timeout."},
			want: "Please inspect the timeout.",
		},
		{
			name: "chat",
			task: Task{ChatSessionID: "chat-1", ChatMessage: "How does this parser work?"},
			want: "How does this parser work?",
		},
		{
			name: "issue fallback",
			task: Task{IssueID: "issue-1"},
			want: "Work on issue issue-1.",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := BuildDirectPrompt(tt.task); got != tt.want {
				t.Fatalf("BuildDirectPrompt() = %q, want %q", got, tt.want)
			}
		})
	}
}

func TestBuildDirectPromptDoesNotCarryMulticaWorkflow(t *testing.T) {
	prompt := BuildDirectPrompt(Task{
		IssueID:               "issue-1",
		TriggerCommentID:      "comment-1",
		TriggerCommentContent: "Please inspect the timeout.",
	})

	for _, banned := range []string{
		"multica issue get",
		"multica issue comment add",
		"Read the discussion",
		"You are running as a local coding agent",
	} {
		if strings.Contains(prompt, banned) {
			t.Errorf("direct prompt contains workflow text %q: %q", banned, prompt)
		}
	}
}
