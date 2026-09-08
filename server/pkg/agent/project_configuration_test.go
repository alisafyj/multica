package agent

import (
	"context"
	"log/slog"
	"slices"
	"testing"
)

func TestClaudeProjectConfigurationSources(t *testing.T) {
	for _, tc := range []struct{ policy, want string }{
		{"restricted", "user"}, {"trusted", "user,project,local"},
	} {
		t.Run(tc.policy, func(t *testing.T) {
			args := buildClaudeArgs(ExecOptions{
				ProjectConfigurationPolicy: tc.policy,
				ClaudeSettingsPath:         "/private/managed-settings.json",
				ExtraArgs:                  []string{"--setting-sources=project", "--settings", "/other/settings.json"},
				CustomArgs:                 []string{"--setting-sources", "local", "--model", "test-model"},
			}, slog.Default())
			count := 0
			for i, arg := range args {
				if arg == "--setting-sources" {
					count++
					if i+1 >= len(args) || args[i+1] != tc.want {
						t.Fatalf("args = %v", args)
					}
				}
			}
			if count != 1 {
				t.Fatalf("expected one managed setting-sources flag, got %v", args)
			}
			if slices.Contains(args, "--setting-sources=project") || slices.Contains(args, "/other/settings.json") {
				t.Fatalf("competing policy survived: %v", args)
			}
			if !slices.Contains(args, "/private/managed-settings.json") {
				t.Fatalf("managed settings lost: %v", args)
			}
		})
	}
}

func TestClaudeManagedSettingsFilterRuntimePrefix(t *testing.T) {
	opts := ExecOptions{ProjectConfigurationPolicy: "restricted"}
	cmd := Command{Prefix: []string{"wrapper", "run", "--setting-sources=project", "--settings", "/other/settings.json", "--model", "test"}}
	got := cmd.withFilteredPrefix(func(args []string) []string {
		return filterCustomArgs(args, claudeManagedSettingsArgs(opts), slog.Default())
	})
	if !slices.Equal(got.Prefix, []string{"wrapper", "run", "--model", "test"}) {
		t.Fatalf("prefix = %v", got.Prefix)
	}
}

func TestClaudeInvalidConfigurationPolicyFailsBeforeLaunch(t *testing.T) {
	backend := &claudeBackend{}
	session, err := backend.Execute(context.Background(), "unused", ExecOptions{ProjectConfigurationPolicy: "typo"})
	if err == nil || session != nil {
		t.Fatalf("session=%v error=%v", session, err)
	}
}

func TestClaudeLegacyProjectConfigurationUnchanged(t *testing.T) {
	args := buildClaudeArgs(ExecOptions{CustomArgs: []string{"--setting-sources", "project"}}, slog.Default())
	i := slices.Index(args, "--setting-sources")
	if i < 0 || args[i+1] != "project" {
		t.Fatalf("legacy args = %v", args)
	}
}
