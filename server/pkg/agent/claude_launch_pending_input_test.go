package agent

import (
	"context"
	"log/slog"
	"slices"
	"testing"
)

func TestBuildClaudeArgsEnablesStdioPermissionPromptWithoutRestrictingTools(t *testing.T) {
	t.Parallel()

	args := buildClaudeArgs(ExecOptions{
		RequestUserInput: func(context.Context, PendingInputRequest) (PendingInputAnswer, error) {
			return PendingInputAnswer{}, nil
		},
	}, slog.Default())

	if !containsAdjacent(args, "--permission-mode", "manual") {
		t.Fatalf("interactive Claude launch must use manual permission mode: %v", args)
	}
	if !containsAdjacent(args, "--permission-prompt-tool", "stdio") {
		t.Fatalf("interactive Claude launch must route permission prompts over stdio: %v", args)
	}
	if slices.Contains(args, "--tools") {
		t.Fatalf("interactive Claude launch must preserve the default coding tool catalog: %v", args)
	}
	if slices.Contains(args, "--disallowedTools") || slices.Contains(args, "AskUserQuestion") {
		t.Fatalf("interactive Claude launch must leave AskUserQuestion available: %v", args)
	}
}

func TestBuildClaudeArgsKeepsInteractivePermissionTransportDaemonOwned(t *testing.T) {
	t.Parallel()

	args := buildClaudeArgs(ExecOptions{
		RequestUserInput: func(context.Context, PendingInputRequest) (PendingInputAnswer, error) {
			return PendingInputAnswer{}, nil
		},
		CustomArgs: []string{
			"--permission-mode", "bypassPermissions",
			"--permission-prompt-tool", "other-tool",
		},
	}, slog.Default())

	if got := countAdjacent(args, "--permission-mode", "manual"); got != 1 {
		t.Fatalf("manual permission mode count = %d, args=%v", got, args)
	}
	if got := countAdjacent(args, "--permission-prompt-tool", "stdio"); got != 1 {
		t.Fatalf("stdio permission prompt transport count = %d, args=%v", got, args)
	}
	if slices.Contains(args, "other-tool") || slices.Contains(args, "bypassPermissions") {
		t.Fatalf("custom args overrode daemon-owned interactive protocol: %v", args)
	}
}

func countAdjacent(args []string, flag, value string) int {
	count := 0
	for i := 0; i+1 < len(args); i++ {
		if args[i] == flag && args[i+1] == value {
			count++
		}
	}
	return count
}
