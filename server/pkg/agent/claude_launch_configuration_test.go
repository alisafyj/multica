package agent

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

const claudeLaunchConfigurationSecret = "R17_PRIVATE_SECRET_MUST_NOT_APPEAR"

func TestClaudeLaunchConfigurationProjectionVectors(t *testing.T) {
	t.Parallel()
	workDir := t.TempDir()
	settingsPath := filepath.Join(workDir, "private-settings-"+claudeLaunchConfigurationSecret+".json")
	if err := os.WriteFile(settingsPath, []byte(`{"permissions":{"allow":["Read"]}}`), 0o600); err != nil {
		t.Fatal(err)
	}
	mcpPath := filepath.Join(workDir, "private-mcp-"+claudeLaunchConfigurationSecret+".json")
	if err := os.WriteFile(mcpPath, []byte(`{"mcpServers":{}}`), 0o600); err != nil {
		t.Fatal(err)
	}

	validArgv := []string{
		"/private/bin/claude", "--no-chrome", "-p", "--output-format", "stream-json",
		"--input-format=stream-json", "--verbose", "--permission-mode", "manual",
		"--permission-prompt-tool=stdio", "--permission-prompts", "host", "--strict-mcp-config",
		"--no-session-persistence", "--model", "claude-requested", "--effort=high",
		"--max-budget-usd", "1.25", "--max-turns=7", "--resume", "private-session",
		"--allowedTools", claudeLaunchConfigurationSecret, "--disallowed-tools=AskUserQuestion",
		"--append-system-prompt", claudeLaunchConfigurationSecret,
		"--setting-sources", "user,project,local", "--settings", settingsPath,
		"--mcp-config=" + mcpPath,
	}
	validEnv := []string{
		"ANTHROPIC_MODEL=old-model",
		"ANTHROPIC_MODEL=claude-requested",
		"ANTHROPIC_DEFAULT_FABLE_MODEL=claude-fable",
		"ANTHROPIC_DEFAULT_OPUS_MODEL=claude-opus",
		"ANTHROPIC_DEFAULT_SONNET_MODEL=claude-sonnet",
		"ANTHROPIC_DEFAULT_HAIKU_MODEL=claude-haiku",
		"CLAUDE_CODE_SUBAGENT_MODEL=claude-subagent",
		"CLAUDE_CODE_SUBAGENT_MODEL_FORCE=1",
		"CLAUDE_CODE_DISABLE_NONESSENTIAL_TRAFFIC=true",
		"CLAUDE_CODE_DISABLE_AUTO_MEMORY=1",
		"CLAUDE_CODE_SUBPROCESS_ENV_SCRUB=false",
		"DISABLE_TELEMETRY=1",
		"DISABLE_ERROR_REPORTING=true",
		"DISABLE_AUTOUPDATER=0",
		"ENABLE_TOOL_SEARCH=false",
		"ANTHROPIC_BASE_URL=https://" + claudeLaunchConfigurationSecret + ".invalid",
		"CLAUDE_CONFIG_DIR=/private/" + claudeLaunchConfigurationSecret,
		"ANTHROPIC_API_KEY=old-secret",
		"ANTHROPIC_API_KEY=" + claudeLaunchConfigurationSecret,
		"CLAUDE_CODE_OAUTH_TOKEN=" + claudeLaunchConfigurationSecret,
	}

	tests := []struct {
		name       string
		argv       []string
		env        []string
		prompt     string
		wantStatus string
		wantErrors []string
		check      func(*testing.T, claudeLaunchConfiguration)
	}{
		{
			name: "valid", argv: validArgv, env: validEnv, prompt: "private prompt " + claudeLaunchConfigurationSecret,
			wantStatus: "observed",
			check: func(t *testing.T, got claudeLaunchConfiguration) {
				if got.Arguments.PermissionPrompts == nil || *got.Arguments.PermissionPrompts != "host" {
					t.Fatalf("permission prompt target = %v", got.Arguments.PermissionPrompts)
				}
				if got.Arguments.UnknownOptionCount != 0 || got.Arguments.Model == nil || *got.Arguments.Model != "claude-requested" ||
					got.Arguments.MaxBudgetUSD == nil || *got.Arguments.MaxBudgetUSD != 1.25 || got.Arguments.MaxTurns == nil || *got.Arguments.MaxTurns != 7 ||
					!got.Arguments.Print || !got.Arguments.Verbose || !got.Arguments.StrictMCPConfig ||
					!got.Arguments.NoSessionPersistence || !got.Arguments.NoChrome || !got.Arguments.ResumePresent || !got.Arguments.SystemPromptPresent {
					t.Fatalf("arguments = %#v", got.Arguments)
				}
				if got.Environment.ModelPolicy["ANTHROPIC_MODEL"] == nil || *got.Environment.ModelPolicy["ANTHROPIC_MODEL"] != "claude-requested" {
					t.Fatalf("final duplicate environment value not selected: %#v", got.Environment.ModelPolicy)
				}
				if strings.Join(got.Environment.CredentialVariables, ",") != "ANTHROPIC_API_KEY,CLAUDE_CODE_OAUTH_TOKEN" {
					t.Fatalf("credential inventory = %#v", got.Environment.CredentialVariables)
				}
				if got.Settings.Source != "file" || got.Settings.Status != "observed" || got.Settings.SHA256 == nil ||
					got.MCP.Source != "file" || got.MCP.Status != "observed" || got.MCP.SHA256 == nil || got.MCP.ServerCount == nil || *got.MCP.ServerCount != 0 {
					t.Fatalf("settings/mcp = %#v / %#v", got.Settings, got.MCP)
				}
			},
		},
		{
			name: "missing", argv: []string{"claude", "-p"}, env: nil, prompt: "ok", wantStatus: "observed",
			check: func(t *testing.T, got claudeLaunchConfiguration) {
				if got.Settings != (claudeLaunchConfigurationSource{Source: "absent", Status: "missing"}) {
					t.Fatalf("settings = %#v", got.Settings)
				}
				if got.MCP.Source != "absent" || got.MCP.Status != "missing" || got.MCP.SHA256 != nil || got.MCP.ServerCount != nil {
					t.Fatalf("mcp = %#v", got.MCP)
				}
			},
		},
		{
			name: "malformed", argv: []string{"claude", "--model", "bad model", "--max-turns", "0", "--settings", `[]`, "--mcp-config", `{}`},
			env: []string{"BROKEN", "DISABLE_TELEMETRY=yes"}, prompt: string([]byte{0xff}), wantStatus: "failed",
			wantErrors: []string{"ARGUMENT_INVALID", "ENVIRONMENT_INVALID", "MCP_INVALID", "PROMPT_INVALID", "SETTINGS_INVALID"},
		},
		{
			name: "prefix and duplicate", argv: []string{"claude", "wrapper-prefix", "--model", "one", "--model=two", "--mystery=" + claudeLaunchConfigurationSecret},
			env: nil, prompt: "ok", wantStatus: "failed", wantErrors: []string{"ARGUMENT_DUPLICATE"},
			check: func(t *testing.T, got claudeLaunchConfiguration) {
				if got.Arguments.UnknownOptionCount != 2 {
					t.Fatalf("unknown option count = %d, want prefix positional plus unknown flag", got.Arguments.UnknownOptionCount)
				}
			},
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			got := projectClaudeLaunchConfiguration(tc.argv, tc.env, workDir, tc.prompt)
			if got.Schema != "claude_launch_configuration/v1" || got.Source != "final_command_inputs" || got.Completeness != "controlled_inputs_only" || got.Status != tc.wantStatus {
				t.Fatalf("projection identity/status = %#v", got)
			}
			if strings.Join(got.Errors, ",") != strings.Join(tc.wantErrors, ",") {
				t.Fatalf("errors = %#v, want %#v", got.Errors, tc.wantErrors)
			}
			if tc.check != nil {
				tc.check(t, got)
			}
			encoded, err := json.Marshal(got)
			if err != nil {
				t.Fatal(err)
			}
			if strings.Contains(string(encoded), claudeLaunchConfigurationSecret) || strings.Contains(string(encoded), settingsPath) || strings.Contains(string(encoded), mcpPath) {
				t.Fatalf("projection leaked secret or path: %s", encoded)
			}
			t.Logf("CLAUDE_LAUNCH_CONFIGURATION_VECTOR %s", mustJSON(t, map[string]any{"case": tc.name, "configuration": got}))
		})
	}
}

func TestClaudeLaunchConfigurationArgumentRejections(t *testing.T) {
	t.Parallel()
	for _, tc := range []struct {
		name string
		argv []string
		want string
	}{
		{"invalid utf8", []string{"claude", string([]byte{0xff})}, "ARGUMENT_INVALID"},
		{"unsafe integer", []string{"claude", "--max-turns=9007199254740992"}, "ARGUMENT_INVALID"},
		{"print aliases", []string{"claude", "-p", "--print"}, "ARGUMENT_DUPLICATE"},
		{"bypass before mode", []string{"claude", "--dangerously-skip-permissions", "--permission-mode=manual"}, "ARGUMENT_DUPLICATE"},
		{"bypass after mode", []string{"claude", "--permission-mode=manual", "--dangerously-skip-permissions"}, "ARGUMENT_DUPLICATE"},
		{"missing prompt target", []string{"claude", "--permission-prompts"}, "ARGUMENT_INVALID"},
		{"invalid prompt target", []string{"claude", "--permission-prompts=true"}, "ARGUMENT_INVALID"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			got := projectClaudeLaunchConfiguration(tc.argv, nil, t.TempDir(), "ok")
			if got.Status != "failed" || !containsString(got.Errors, tc.want) {
				t.Fatalf("projection = %#v, want %s", got, tc.want)
			}
			if got.Arguments.UnknownOptionCount > claudeLaunchArgumentLimit {
				t.Fatalf("unbounded unknown count: %d", got.Arguments.UnknownOptionCount)
			}
		})
	}
}

func TestClaudeLaunchConfigurationLimitsAndSymlinks(t *testing.T) {
	t.Parallel()
	workDir := t.TempDir()
	target := filepath.Join(workDir, "settings.json")
	if err := os.WriteFile(target, []byte(`{}`), 0o600); err != nil {
		t.Fatal(err)
	}
	link := filepath.Join(workDir, "settings-link.json")
	if err := os.Symlink(target, link); err != nil {
		t.Fatal(err)
	}

	tests := []struct {
		name string
		argv []string
		env  []string
		want string
	}{
		{name: "argument count", argv: append([]string{"claude"}, make([]string, 257)...), want: "ARGUMENT_LIMIT"},
		{name: "argument bytes", argv: []string{"claude", "--model", strings.Repeat("x", 64*1024)}, want: "ARGUMENT_LIMIT"},
		{name: "environment count", argv: []string{"claude"}, env: make([]string, 4097), want: "ENVIRONMENT_LIMIT"},
		{name: "environment bytes", argv: []string{"claude"}, env: []string{"X=" + strings.Repeat("x", 1024*1024)}, want: "ENVIRONMENT_LIMIT"},
		{name: "settings symlink", argv: []string{"claude", "--settings", link}, want: "SETTINGS_INVALID"},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			got := projectClaudeLaunchConfiguration(tc.argv, tc.env, workDir, "ok")
			if got.Status != "failed" || !containsString(got.Errors, tc.want) {
				t.Fatalf("projection = %#v, want %s", got, tc.want)
			}
		})
	}
}

func mustJSON(t *testing.T, value any) string {
	t.Helper()
	encoded, err := json.Marshal(value)
	if err != nil {
		t.Fatal(err)
	}
	return string(encoded)
}

func TestClaudeLaunchConfigurationEnvironmentCaseSemantics(t *testing.T) {
	t.Parallel()
	env := []string{
		"ANTHROPIC_MODEL=initial", "anthropic_model=effective",
		"ENABLE_TOOL_SEARCH=false", "Enable_Tool_Search=true",
		"ANTHROPIC_API_KEY=placeholder", "claude_code_oauth_token=placeholder",
		"ANTHROPIC_BASE_URL=https://initial.invalid", "Anthropic_Base_Url=https://effective.invalid",
		"CLAUDE_CONFIG_DIR=/initial", "Claude_Config_Dir=/effective",
	}
	for _, windows := range []bool{false, true} {
		got, errors := projectClaudeLaunchEnvironment(env, windows)
		if len(errors) != 0 {
			t.Fatalf("unexpected projection errors: %v", errors)
		}
		model, control, route, directory, credentials := "initial", "false", "https://initial.invalid", "/initial", "ANTHROPIC_API_KEY"
		if windows {
			model, control, route, directory, credentials = "effective", "true", "https://effective.invalid", "/effective", "ANTHROPIC_API_KEY,CLAUDE_CODE_OAUTH_TOKEN"
		}
		if got.ModelPolicy["ANTHROPIC_MODEL"] == nil || *got.ModelPolicy["ANTHROPIC_MODEL"] != model ||
			got.Controls["ENABLE_TOOL_SEARCH"] == nil || *got.Controls["ENABLE_TOOL_SEARCH"] != control ||
			got.BaseURLSHA256 == nil || *got.BaseURLSHA256 != *claudeLaunchStringDigest(route) ||
			got.ConfigDirectorySHA256 == nil || *got.ConfigDirectorySHA256 != *claudeLaunchStringDigest(directory) ||
			strings.Join(got.CredentialVariables, ",") != credentials {
			t.Fatalf("case-insensitive=%t: incorrect effective environment: %#v", windows, got)
		}
	}
	got, errors := projectClaudeLaunchEnvironment([]string{"claude_code_oauth_token=placeholder", "CLAUDE_CODE_OAUTH_TOKEN="}, true)
	if len(errors) != 0 || len(got.CredentialVariables) != 0 {
		t.Fatal("later empty Windows credential did not replace its differently-cased predecessor")
	}
}
