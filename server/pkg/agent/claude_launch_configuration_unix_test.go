//go:build unix

package agent

import (
	"bytes"
	"context"
	"encoding/json"
	"log/slog"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

func TestClaudeLaunchConfigurationExecuteReceipt(t *testing.T) {
	root := t.TempDir()
	workDir := filepath.Join(root, "work")
	if err := os.Mkdir(workDir, 0o700); err != nil {
		t.Fatal(err)
	}
	settingsPath := filepath.Join(workDir, "settings-"+claudeLaunchConfigurationSecret+".json")
	if err := os.WriteFile(settingsPath, []byte(`{"permissions":{"allow":[]}}`), 0o600); err != nil {
		t.Fatal(err)
	}
	fakePath := filepath.Join(root, "claude")
	writeTestExecutable(t, fakePath, []byte(`#!/bin/sh
IFS= read -r _
printf '%s\n' '{"type":"system","subtype":"init","session_id":"private-session","model":"claude-requested","permissionMode":"default","mcp_servers":[]}'
printf '%s\n' '{"type":"assistant","session_id":"private-session","message":{"model":"claude-requested","content":[]}}'
printf '%s\n' '{"type":"result","subtype":"success","is_error":false,"session_id":"private-session","result":"done","modelUsage":{"claude-requested":{"inputTokens":1,"cacheReadInputTokens":0,"cacheCreationInputTokens":0,"outputTokens":1}}}'
`))

	var logs bytes.Buffer
	backend := &claudeBackend{cfg: Config{
		ExecutablePath: fakePath,
		LaunchPrefix:   []string{"--no-chrome", "--no-session-persistence"},
		Logger:         slog.New(slog.NewJSONHandler(&logs, nil)).With("component", "daemon"),
		TaskID:         "66666666-6666-4666-8666-666666666666",
		RuntimeID:      "77777777-7777-4777-8777-777777777777",
		Env: map[string]string{
			"ANTHROPIC_MODEL":                          "claude-requested",
			"ANTHROPIC_DEFAULT_FABLE_MODEL":            "claude-fable",
			"ANTHROPIC_DEFAULT_OPUS_MODEL":             "claude-opus",
			"ANTHROPIC_DEFAULT_SONNET_MODEL":           "claude-sonnet",
			"ANTHROPIC_DEFAULT_HAIKU_MODEL":            "claude-haiku",
			"CLAUDE_CODE_SUBAGENT_MODEL":               "claude-subagent",
			"CLAUDE_CODE_SUBAGENT_MODEL_FORCE":         "1",
			"CLAUDE_CODE_DISABLE_NONESSENTIAL_TRAFFIC": "1",
			"CLAUDE_CODE_DISABLE_AUTO_MEMORY":          "1",
			"CLAUDE_CODE_SUBPROCESS_ENV_SCRUB":         "1",
			"DISABLE_TELEMETRY":                        "1",
			"DISABLE_ERROR_REPORTING":                  "1",
			"DISABLE_AUTOUPDATER":                      "1",
			"ENABLE_TOOL_SEARCH":                       "false",
			"ANTHROPIC_BASE_URL":                       "https://vibe.soyoung.com",
			"CLAUDE_CONFIG_DIR":                        "/private/" + claudeLaunchConfigurationSecret,
			"ANTHROPIC_AUTH_TOKEN":                     claudeLaunchConfigurationSecret,
		},
	}}
	session, err := backend.Execute(context.Background(), "prompt "+claudeLaunchConfigurationSecret, ExecOptions{
		Cwd: workDir, Timeout: 5 * time.Second, Model: "claude-requested", ThinkingLevel: "high",
		MaxTurns: 3, CustomArgs: []string{"--max-budget-usd", "1.25"}, ClaudeSettingsPath: settingsPath, McpConfig: json.RawMessage(`{"mcpServers":{}}`),
		RequestUserInput: func(context.Context, PendingInputRequest) (PendingInputAnswer, error) {
			t.Error("unexpected user input request in launch receipt fixture")
			return PendingInputAnswer{}, nil
		},
	})
	if err != nil {
		t.Fatal(err)
	}
	go func() {
		for range session.Messages {
		}
	}()
	result := <-session.Result
	if result.Status != "completed" {
		t.Fatalf("result = %+v", result)
	}

	entries := parseJSONLogEntries(t, logs.String())
	started := onlyLogMessage(t, entries, "claude started")
	configuration := onlyLogMessage(t, entries, "claude launch configuration observed")
	span := onlyLogMessage(t, entries, "claude execution span observed")
	stream := onlyLogMessage(t, entries, "claude stream metadata observed")
	for _, key := range []string{"task_id", "runtime_id", "attempt"} {
		if configuration[key] != started[key] || configuration[key] != span[key] || configuration[key] != stream[key] {
			t.Fatalf("identity %s differs across logs", key)
		}
	}
	if configuration["process_id"] != started["pid"] || configuration["process_id"] != span["process_id"] || configuration["process_id"] != stream["process_id"] ||
		configuration["work_dir"] != started["cwd"] || configuration["work_dir"] != span["work_dir"] || configuration["work_dir"] != stream["work_dir"] {
		t.Fatalf("process/work identity differs: %#v", configuration)
	}
	payload, ok := configuration["configuration"].(map[string]any)
	if !ok || payload["status"] != "observed" {
		t.Fatalf("configuration payload = %#v", configuration["configuration"])
	}
	args := payload["arguments"].(map[string]any)
	if args["permission_mode"] != "manual" || args["permission_prompt_tool"] != "stdio" || args["permission_prompts"] != nil {
		t.Fatalf("manual transport not projected: %#v", args)
	}
	if args["no_chrome"] != true || args["no_session_persistence"] != true || args["unknown_option_count"] != float64(0) {
		t.Fatalf("runtime prefix not projected: %#v", args)
	}
	if strings.Contains(logs.String(), claudeLaunchConfigurationSecret) || strings.Contains(logs.String(), settingsPath) || strings.Contains(logs.String(), "private-session") {
		t.Fatalf("logs leaked secret, private path, or session: %s", logs.String())
	}
	receipt := []map[string]any{started, configuration, span, stream}
	t.Logf("CLAUDE_LAUNCH_CONFIGURATION_EXECUTE_RECEIPT %s", mustJSON(t, receipt))
}

func TestClaudeLaunchConfigurationSpawnFailureEmitsNothing(t *testing.T) {
	var logs bytes.Buffer
	backend := &claudeBackend{cfg: Config{
		ExecutablePath: filepath.Join(t.TempDir(), "missing-claude"),
		Logger:         slog.New(slog.NewJSONHandler(&logs, nil)).With("component", "daemon"),
		TaskID:         "task",
		RuntimeID:      "runtime",
	}}
	if _, err := backend.Execute(context.Background(), claudeLaunchConfigurationSecret, ExecOptions{Cwd: t.TempDir()}); err == nil {
		t.Fatal("Execute succeeded with missing executable")
	}
	for _, entry := range parseJSONLogEntries(t, logs.String()) {
		if entry["msg"] == "claude launch configuration observed" {
			t.Fatalf("spawn failure emitted launch configuration: %#v", entry)
		}
	}
}
