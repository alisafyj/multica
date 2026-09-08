//go:build !windows

package daemon

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"log/slog"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/multica-ai/multica/server/internal/agentconfig"
)

func TestRuntimeMCPSelectionRunTaskRejectsBeforeProviderLaunch(t *testing.T) {
	for _, provider := range []string{"claude", "codex"} {
		for _, name := range []string{"missing", "off", "screen-control", "invalid-policy"} {
			t.Run(provider+"/"+name, func(t *testing.T) {
				writeRuntimeMCPSelectionFixture(t, provider)
				d, argsFile, cleanup := newLeaderReuseTestDaemon(t)
				defer cleanup()
				d.cfg.Agents[provider] = d.cfg.Agents["claude"]
				var logs bytes.Buffer
				d.logger = slog.New(slog.NewTextHandler(&logs, nil))
				task := leaderReuseTestTask("task-runtime-mcp-selection")
				task.Agent.McpConfig = json.RawMessage(`{"_multica":{"runtimeMcp":{"mode":"allowlist","allow":["` + name + `"]}}}`)
				if name == "invalid-policy" {
					task.Agent.McpConfig = json.RawMessage(`{"_multica":{"runtimeMcp":{"mode":"deny_all","headers":"synthetic-r03-policy-secret"}}}`)
				}
				result, err := d.runTask(context.Background(), task, provider, 0, d.logger)
				var setupErr *environmentSetupError
				if !errors.Is(err, agentconfig.ErrRuntimeMCPSelection) || !errors.As(err, &setupErr) || result.Status == "completed" {
					t.Fatalf("required runtime selection did not fail preparation: status=%s error=%v", result.Status, err)
				}
				if err.Error() != "invalid runtime MCP selection: repair the agent selection or local runtime configuration before retrying" {
					t.Fatal("selection failure must return a fixed safe reason")
				}
				if _, statErr := os.Stat(argsFile); !os.IsNotExist(statErr) {
					t.Fatal("provider launched despite unsatisfied selection")
				}
				if strings.Contains(logs.String()+err.Error(), "local-selection-secret") || strings.Contains(logs.String()+err.Error(), "synthetic-r03-policy-secret") {
					t.Fatal("selection failure leaked configuration secrets")
				}
			})
		}
	}
}

func TestRuntimeMCPSelectionRunTaskPreservesLegacyInheritFailureBehavior(t *testing.T) {
	for _, explicit := range []bool{false, true} {
		t.Run(map[bool]string{false: "missing", true: "explicit"}[explicit], func(t *testing.T) {
			writeRuntimeMCPSelectionFixture(t, "claude")
			if err := os.WriteFile(filepath.Join(os.Getenv("HOME"), ".claude.json"), []byte("malformed legacy runtime config {"), 0o600); err != nil {
				t.Fatal(err)
			}
			d, argsFile, cleanup := newLeaderReuseTestDaemon(t)
			defer cleanup()
			task := leaderReuseTestTask("task-runtime-mcp-inherit")
			if explicit {
				task.Agent.McpConfig = json.RawMessage(`{"_multica":{"runtimeMcp":{"mode":"inherit"}}}`)
			}
			result, err := d.runTask(context.Background(), task, "claude", 0, d.logger)
			if err != nil || result.Status != "completed" {
				t.Fatalf("legacy ordinary inherit behavior changed: status=%s error=%v", result.Status, err)
			}
			args, _ := os.ReadFile(argsFile)
			if !strings.Contains(string(args), "--output-format") {
				t.Fatal("fake provider was not launched")
			}
		})
	}
}
