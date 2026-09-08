package agent

import (
	"bytes"
	"context"
	"encoding/json"
	"log/slog"
	"strings"
	"testing"
)

func TestClaudePermissionDenialUsesNativeContract(t *testing.T) {
	t.Parallel()
	for _, command := range []string{"cat ~/.zshrc", "cat /etc/passwd"} {
		t.Run(command, func(t *testing.T) {
			var written bytes.Buffer
			b := &claudeBackend{cfg: Config{Logger: slog.Default(), WorkDir: "/tmp/repo"}}
			b.handleControlRequest(claudeSDKMessage{
				RequestID: "denied-request",
				Request: mustMarshal(t, map[string]any{
					"subtype": "can_use_tool", "tool_name": "Bash",
					"input": map[string]any{"command": command, "description": "synthetic-private-input"},
				}),
			}, &written)
			var response struct {
				Response struct {
					RequestID string         `json:"request_id"`
					Response  map[string]any `json:"response"`
				} `json:"response"`
			}
			if err := json.Unmarshal(written.Bytes(), &response); err != nil {
				t.Fatal(err)
			}
			payload := response.Response.Response
			if response.Response.RequestID != "denied-request" || payload["behavior"] != "deny" {
				t.Fatalf("unexpected denial: %s", written.String())
			}
			if message, ok := payload["message"].(string); !ok || strings.TrimSpace(message) == "" {
				t.Error("native deny requires a nonempty message")
			}
			if _, ok := payload["updatedInput"]; ok {
				t.Error("deny must not echo rejected tool input")
			}
			if strings.Contains(written.String(), command) || strings.Contains(written.String(), "synthetic-private-input") {
				t.Error("denial response leaked rejected input")
			}
		})
	}
}

func TestClaudeMalformedInteractiveControlNeverAutoAllows(t *testing.T) {
	t.Parallel()
	for _, subtype := range []string{"request_user_dialog", "can_use_tool"} {
		for _, field := range []string{"dialog_kind", "payload", "isSecret", "blocking", "requires_user_interaction"} {
			t.Run(subtype+"/"+field, func(t *testing.T) {
				request := map[string]any{
					"subtype": subtype, "tool_name": "AskUserQuestion", "tool_use_id": "question-tool",
					"requires_user_interaction": true, "dialog_kind": claudeAskUserQuestionDialogKind,
					"payload": validClaudeQuestionInput(), "input": validClaudeQuestionInput(),
				}
				request[field] = 42
				var written bytes.Buffer
				b := &claudeBackend{cfg: Config{Logger: slog.Default()}}
				msg := claudeSDKMessage{RequestID: "malformed-question", Request: mustMarshal(t, request)}
				if !b.handleUserInputControlRequest(context.Background(), msg, &written, ExecOptions{}, nil) {
					b.handleControlRequest(msg, &written)
				}
				if !strings.Contains(written.String(), `"subtype":"error"`) || !strings.Contains(written.String(), `"request_id":"malformed-question"`) {
					t.Fatalf("malformed question must receive a correlated error: %s", written.String())
				}
				if strings.Contains(written.String(), `"behavior":"allow"`) {
					t.Fatal("malformed question was automatically approved")
				}
			})
		}
	}
}
