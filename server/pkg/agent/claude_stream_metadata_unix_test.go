//go:build unix

package agent

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"log/slog"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

const (
	claudeStreamMetadataTaskID    = "44444444-4444-4444-8444-444444444444"
	claudeStreamMetadataRuntimeID = "55555555-5555-4555-8555-555555555555"
	claudeStreamMetadataSecret    = "R16_PRIVATE_PROMPT_AND_RESPONSE"
)

func TestClaudeStreamMetadataScannerFailureAfterTerminal(t *testing.T) {
	body := `
IFS= read -r _
printf '%s\n' '{"type":"system","subtype":"init","session_id":"s","model":"m","permissionMode":"default","mcp_servers":[]}'
printf '%s\n' '{"type":"assistant","session_id":"s","message":{"model":"m","content":[]}}'
printf '%s\n' '{"type":"result","subtype":"success","is_error":false,"session_id":"s","modelUsage":{"m":{"inputTokens":1,"cacheReadInputTokens":0,"cacheCreationInputTokens":0,"outputTokens":1}}}'
` + fmt.Sprintf(`dd if=/dev/zero bs=1048576 count=%d 2>/dev/null | tr '\000' x; printf '\n'`, agentStreamMaxLineBytes/(1024*1024)+1)
	result, entries, _ := runClaudeStreamMetadataExecuteFixture(t, body)
	if result.Status != "failed" {
		t.Fatalf("scanner failure status = %q, want failed", result.Status)
	}
	payload := onlyClaudeStreamMetadataLog(t, entries)["stream_metadata"].(map[string]any)
	if payload["status"] != "failed" {
		t.Fatalf("scanner failure metadata status = %v, want failed", payload["status"])
	}
}

func TestClaudeStreamMetadataExecuteReceipt(t *testing.T) {
	result, entries, logs := runClaudeStreamMetadataExecuteFixture(t, `
IFS= read -r _
printf '%s\n' '{"type":"system","subtype":"init","session_id":"private-session","model":"configured-alias","permissionMode":"bypassPermissions","mcp_servers":[]}'
printf '%s\n' '{"type":"assistant","session_id":"private-session","parent_tool_use_id":null,"message":{"model":"main-model","content":[{"type":"text","text":"R16_PRIVATE_PROMPT_AND_RESPONSE"}]}}'
printf '%s\n' '{"type":"assistant","session_id":"private-session","parent_tool_use_id":"private-tool-id","message":{"model":"child-model","content":[]}}'
printf '%s\n' '{"type":"result","subtype":"success","is_error":false,"session_id":"private-session","result":"R16_PRIVATE_PROMPT_AND_RESPONSE","modelUsage":{"main-model":{"inputTokens":3,"cacheReadInputTokens":4,"cacheCreationInputTokens":5,"outputTokens":6},"child-model":{"inputTokens":7,"cacheReadInputTokens":8,"cacheCreationInputTokens":9,"outputTokens":10}}}'
`)
	if result.Status != "completed" || result.Output != claudeStreamMetadataSecret {
		t.Fatalf("result = %+v", result)
	}
	metadata := onlyClaudeStreamMetadataLog(t, entries)
	span := onlyLogMessage(t, entries, "claude execution span observed")
	started := onlyLogMessage(t, entries, "claude started")
	for _, key := range []string{"task_id", "runtime_id", "attempt"} {
		if metadata[key] != span[key] || metadata[key] != started[key] {
			t.Fatalf("identity %s differs: metadata=%v span=%v started=%v", key, metadata[key], span[key], started[key])
		}
	}
	if metadata["process_id"] != span["process_id"] || metadata["process_id"] != started["pid"] ||
		metadata["work_dir"] != span["work_dir"] || metadata["work_dir"] != started["cwd"] {
		t.Fatalf("process/work identity differs: metadata=%v span=%v started=%v", metadata, span, started)
	}
	payload, ok := metadata["stream_metadata"].(map[string]any)
	if !ok || len(payload) != 18 || payload["status"] != "observed" || payload["mcp_server_count"] != float64(0) || payload["configured_model"] != "configured-alias" || payload["main_model"] != "main-model" {
		t.Fatalf("stream metadata payload = %#v", metadata["stream_metadata"])
	}
	if strings.Contains(logs, claudeStreamMetadataSecret) || strings.Contains(logs, "private-session") || strings.Contains(logs, "private-tool-id") {
		t.Fatalf("logs leaked private stream data: %s", logs)
	}
	receipt, err := json.Marshal([]map[string]any{started, span, metadata})
	if err != nil {
		t.Fatal(err)
	}
	t.Logf("CLAUDE_STREAM_METADATA_EXECUTE_RECEIPT %s", receipt)
	t.Logf("claude stream metadata Execute logs:\n%s", logs)
}

func TestClaudeStreamMetadataExecuteErrorAndMissingResult(t *testing.T) {
	tests := []struct {
		name       string
		body       string
		wantStatus string
		wantCode   string
	}{
		{name: "no auth result", body: `
IFS= read -r _
printf '%s\n' '{"type":"system","subtype":"init","session_id":"s","model":"configured","permissionMode":"bypassPermissions","mcp_servers":[]}'
printf '%s\n' '{"type":"result","subtype":"error_during_execution","is_error":true,"session_id":"s","result":"credential secret"}'
`, wantStatus: "failed", wantCode: "TERMINAL_FAILED"},
		{name: "missing result", body: `
IFS= read -r _
printf '%s\n' '{"type":"system","subtype":"init","session_id":"s","model":"configured","permissionMode":"bypassPermissions","mcp_servers":[]}'
printf '%s\n' '{"type":"assistant","session_id":"s","message":{"model":"main","content":[]}}'
`, wantStatus: "missing", wantCode: "TERMINAL_MISSING"},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			result, entries, logs := runClaudeStreamMetadataExecuteFixture(t, tc.body)
			if result.Status != "failed" {
				t.Fatalf("result = %+v", result)
			}
			metadata := onlyClaudeStreamMetadataLog(t, entries)
			payload := metadata["stream_metadata"].(map[string]any)
			if payload["status"] != tc.wantStatus {
				t.Fatalf("projection = %#v", payload)
			}
			encoded, _ := json.Marshal(payload["errors"])
			if !strings.Contains(string(encoded), tc.wantCode) {
				t.Fatalf("missing %s in %s", tc.wantCode, encoded)
			}
			if strings.Contains(logs, "credential secret") {
				t.Fatalf("logs leaked terminal error: %s", logs)
			}
		})
	}
}

func TestClaudeStreamMetadataSpawnFailureEmitsNothing(t *testing.T) {
	var logs bytes.Buffer
	backend := &claudeBackend{cfg: Config{
		ExecutablePath: filepath.Join(t.TempDir(), "missing-claude"),
		Logger:         slog.New(slog.NewJSONHandler(&logs, nil)).With("component", "daemon"),
		TaskID:         claudeStreamMetadataTaskID,
		RuntimeID:      claudeStreamMetadataRuntimeID,
	}}
	if _, err := backend.Execute(context.Background(), claudeStreamMetadataSecret, ExecOptions{Cwd: t.TempDir()}); err == nil {
		t.Fatal("Execute succeeded with missing executable")
	}
	for _, entry := range parseJSONLogEntries(t, logs.String()) {
		if entry["msg"] == "claude stream metadata observed" {
			t.Fatalf("spawn failure emitted metadata: %v", entry)
		}
	}
}

func runClaudeStreamMetadataExecuteFixture(t *testing.T, body string) (Result, []map[string]any, string) {
	t.Helper()
	root := t.TempDir()
	fakePath := filepath.Join(root, "claude")
	writeTestExecutable(t, fakePath, []byte("#!/bin/sh\n"+body))
	workDir := filepath.Join(root, "work")
	if err := os.Mkdir(workDir, 0o700); err != nil {
		t.Fatal(err)
	}
	var logs bytes.Buffer
	backend := &claudeBackend{cfg: Config{
		ExecutablePath: fakePath,
		Logger:         slog.New(slog.NewJSONHandler(&logs, nil)).With("component", "daemon"),
		TaskID:         claudeStreamMetadataTaskID,
		RuntimeID:      claudeStreamMetadataRuntimeID,
	}}
	session, err := backend.Execute(context.Background(), claudeStreamMetadataSecret, ExecOptions{Cwd: workDir, Timeout: 5 * time.Second})
	if err != nil {
		t.Fatal(err)
	}
	go func() {
		for range session.Messages {
		}
	}()
	result := <-session.Result
	return result, parseJSONLogEntries(t, logs.String()), logs.String()
}

func onlyClaudeStreamMetadataLog(t *testing.T, entries []map[string]any) map[string]any {
	t.Helper()
	return onlyLogMessage(t, entries, "claude stream metadata observed")
}

func onlyLogMessage(t *testing.T, entries []map[string]any, message string) map[string]any {
	t.Helper()
	var matches []map[string]any
	for _, entry := range entries {
		if entry["msg"] == message {
			matches = append(matches, entry)
		}
	}
	if len(matches) != 1 {
		t.Fatalf("%s logs = %d, want 1; entries=%v", message, len(matches), entries)
	}
	return matches[0]
}
