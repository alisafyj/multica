package daemon

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func projectPolicyTask(t *testing.T, policy string, names ...string) Task {
	t.Helper()
	ref := map[string]any{
		"url":                  "https://github.com/acme/app.git",
		"ref":                  "release",
		"configuration_policy": policy,
		"mcp_servers":          names,
	}
	raw, err := json.Marshal(ref)
	if err != nil {
		t.Fatal(err)
	}
	return Task{
		IssueID: "issue-1",
		Repos:   []RepoData{{URL: "https://github.com/acme/app.git", Ref: "release"}},
		ProjectResources: []ProjectResourceData{{
			ID: "resource-1", ResourceType: "github_repo", ResourceRef: raw,
		}},
	}
}

func TestDeriveProjectRuntimePolicyDefaultsRestrictedAndRequiresAuthorizedMatch(t *testing.T) {
	root := t.TempDir()
	if err := os.WriteFile(filepath.Join(root, ".mcp.json"), []byte(`{"mcpServers":{"docs":{"command":"docs"}}}`), 0o600); err != nil {
		t.Fatal(err)
	}
	tasks := []Task{projectPolicyTask(t, "", "docs")}
	unauthorized := projectPolicyTask(t, "trusted", "docs")
	unauthorized.Repos[0].URL = "https://github.com/acme/other.git"
	tasks = append(tasks, unauthorized)
	for i, task := range tasks {
		got, err := deriveProjectRuntimePolicy(task, "claude", root)
		if i == 0 && err != nil {
			t.Fatal(err)
		}
		if i == 1 && err == nil {
			t.Fatal("unauthorized project resource did not fail closed")
		}
		if got.ConfigurationPolicy != projectConfigurationRestricted || len(got.MCPConfig) != 0 {
			t.Fatalf("policy = %+v, want restricted without MCP config", got)
		}
	}
}

func TestDeriveProjectRuntimePolicyTreatsEveryPreparedPrimaryAsRestricted(t *testing.T) {
	root := t.TempDir()
	legacy, err := deriveProjectRuntimePolicy(Task{}, "claude", root)
	if err != nil || legacy.ConfigurationPolicy != "" {
		t.Fatalf("no primary repository should remain legacy: %+v err=%v", legacy, err)
	}
	managed, err := deriveProjectRuntimePolicy(Task{
		IssueID: "issue-1",
		Repos:   []RepoData{{URL: "https://github.com/acme/app.git"}},
	}, "claude", root)
	if err != nil {
		t.Fatal(err)
	}
	if managed.ConfigurationPolicy != projectConfigurationRestricted {
		t.Fatalf("Task.Repos primary policy = %q, want restricted", managed.ConfigurationPolicy)
	}
}

func TestDeriveProjectRuntimePolicyReportsMCPSelectionThatRequiresTrust(t *testing.T) {
	got, err := deriveProjectRuntimePolicy(projectPolicyTask(t, "restricted", "docs"), "claude", t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	want := []projectMCPDiagnostic{{Name: "docs", Status: "trust_required"}}
	if len(got.MCPDiagnostics) != 1 || got.MCPDiagnostics[0] != want[0] {
		t.Fatalf("diagnostics = %+v, want %+v", got.MCPDiagnostics, want)
	}
}

func TestDeriveProjectRuntimePolicyLoadsSelectedClaudeServersAndSanitizesDiagnostics(t *testing.T) {
	root := t.TempDir()
	config := `{"mcpServers":{"docs":{"command":"docs","args":["--token=secret"]},"off":{"command":"off","disabled":true},"screen-control":{"command":"screen"}}}`
	if err := os.WriteFile(filepath.Join(root, ".mcp.json"), []byte(config), 0o600); err != nil {
		t.Fatal(err)
	}
	got, err := deriveProjectRuntimePolicy(projectPolicyTask(t, "trusted", "docs", "off", "missing", "screen-control"), "claude", root)
	if err != nil {
		t.Fatal(err)
	}
	if got.ConfigurationPolicy != projectConfigurationTrusted {
		t.Fatalf("policy = %q", got.ConfigurationPolicy)
	}
	if string(got.MCPConfig) != `{"mcpServers":{"docs":{"args":["--token=secret"],"command":"docs"}}}` {
		t.Fatalf("MCP config = %s", got.MCPConfig)
	}
	want := []projectMCPDiagnostic{{Name: "docs", Status: "selected"}, {Name: "off", Status: "disabled"}, {Name: "missing", Status: "missing"}, {Name: "screen-control", Status: "blocked_privacy"}}
	if len(got.MCPDiagnostics) != len(want) {
		t.Fatalf("diagnostics = %+v", got.MCPDiagnostics)
	}
	for i := range want {
		if got.MCPDiagnostics[i] != want[i] {
			t.Fatalf("diagnostic %d = %+v, want %+v", i, got.MCPDiagnostics[i], want[i])
		}
		if strings.Contains(strings.ToLower(got.MCPDiagnostics[i].Name+got.MCPDiagnostics[i].Status), "secret") {
			t.Fatalf("diagnostic leaked configuration: %+v", got.MCPDiagnostics[i])
		}
	}
}

func TestDeriveProjectRuntimePolicyLoadsSelectedCodexServers(t *testing.T) {
	root := t.TempDir()
	if err := os.Mkdir(filepath.Join(root, ".codex"), 0o700); err != nil {
		t.Fatal(err)
	}
	config := "[mcp_servers.docs]\ncommand = \"docs\"\n[mcp_servers.off]\ncommand = \"off\"\nenabled = false\n"
	if err := os.WriteFile(filepath.Join(root, ".codex", "config.toml"), []byte(config), 0o600); err != nil {
		t.Fatal(err)
	}
	got, err := deriveProjectRuntimePolicy(projectPolicyTask(t, "trusted", "docs", "off"), "codex", root)
	if err != nil {
		t.Fatal(err)
	}
	if len(got.MCPConfig) == 0 || got.MCPDiagnostics[0].Status != "selected" || got.MCPDiagnostics[1].Status != "disabled" {
		t.Fatalf("policy = %+v", got)
	}
}

func TestDeriveProjectRuntimePolicyRejectsMalformedSelectedServer(t *testing.T) {
	root := t.TempDir()
	if err := os.WriteFile(filepath.Join(root, ".mcp.json"), []byte(`{"mcpServers":{"docs":"not-an-object"}}`), 0o600); err != nil {
		t.Fatal(err)
	}
	if _, err := deriveProjectRuntimePolicy(projectPolicyTask(t, "trusted", "docs"), "claude", root); err == nil {
		t.Fatal("malformed selected server accepted")
	}
}

func TestDeriveProjectRuntimePolicyRejectsUnsafeProjectConfigFiles(t *testing.T) {
	for _, tc := range []struct {
		name  string
		setup func(*testing.T, string)
	}{
		{name: "symlink", setup: func(t *testing.T, root string) {
			target := filepath.Join(t.TempDir(), "outside.json")
			if err := os.WriteFile(target, []byte(`{"mcpServers":{}}`), 0o600); err != nil {
				t.Fatal(err)
			}
			if err := os.Symlink(target, filepath.Join(root, ".mcp.json")); err != nil {
				t.Fatal(err)
			}
		}},
		{name: "oversized", setup: func(t *testing.T, root string) {
			if err := os.WriteFile(filepath.Join(root, ".mcp.json"), []byte(strings.Repeat(" ", maxProjectMCPConfigBytes+1)), 0o600); err != nil {
				t.Fatal(err)
			}
		}},
	} {
		t.Run(tc.name, func(t *testing.T) {
			root := t.TempDir()
			tc.setup(t, root)
			if _, err := deriveProjectRuntimePolicy(projectPolicyTask(t, "trusted", "docs"), "claude", root); err == nil {
				t.Fatal("unsafe project MCP config accepted")
			}
		})
	}

	t.Run("symlinked config directory", func(t *testing.T) {
		root := t.TempDir()
		outside := t.TempDir()
		if err := os.WriteFile(filepath.Join(outside, "config.toml"), []byte("[mcp_servers.docs]\ncommand = \"docs\"\n"), 0o600); err != nil {
			t.Fatal(err)
		}
		if err := os.Symlink(outside, filepath.Join(root, ".codex")); err != nil {
			t.Fatal(err)
		}
		if _, err := deriveProjectRuntimePolicy(projectPolicyTask(t, "trusted", "docs"), "codex", root); err == nil {
			t.Fatal("symlinked project config directory accepted")
		}
	})
}
