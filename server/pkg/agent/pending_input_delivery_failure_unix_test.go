//go:build !windows

package agent

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"log/slog"
	"os"
	"path/filepath"
	"strings"
	"sync/atomic"
	"testing"
	"time"
)

func TestPendingInputDeliveryFailureProviderResults(t *testing.T) {
	for _, provider := range []string{"claude", "codex"} {
		for _, tc := range []struct {
			name       string
			terminal   string
			ackFails   bool
			wantStatus string
			wantError  string
		}{
			{name: "late_receipt_failure", terminal: "completed", ackFails: true, wantStatus: "failed", wantError: "failed to acknowledge delivered user input"},
			{name: "normal_delivery", terminal: "completed", wantStatus: "completed"},
			{name: "provider_failure_preserved", terminal: "failed", ackFails: true, wantStatus: "failed", wantError: "original provider failure"},
			{name: "cancellation_preserved", terminal: "cancel", ackFails: true, wantStatus: "aborted", wantError: "execution cancelled"},
			{name: "native_abort_preserved", terminal: "aborted", ackFails: true, wantStatus: "aborted", wantError: "turn was aborted"},
			{name: "timeout_preserved", terminal: "timeout", ackFails: true, wantStatus: "timeout", wantError: "timed out"},
		} {
			if tc.terminal == "aborted" && provider != "codex" {
				continue
			}
			t.Run(provider+"/"+tc.name, func(t *testing.T) {
				const secret = "synthetic-delivery-receipt-secret"
				answerPath := filepath.Join(t.TempDir(), "native-answer.json")
				script := pendingInputDeliveryFailureScript(provider, tc.terminal, answerPath)
				fake := filepath.Join(t.TempDir(), provider)
				writeTestExecutable(t, fake, []byte(script))
				var logs bytes.Buffer
				logWriter := &lockedWriter{writer: &logs}
				cfg := Config{ExecutablePath: fake, Logger: slog.New(slog.NewTextHandler(logWriter, nil))}
				ctx, cancel := context.WithTimeout(t.Context(), 8*time.Second)
				defer cancel()
				var acknowledgements atomic.Int32
				opts := ExecOptions{
					Cwd: t.TempDir(), Timeout: time.Second,
					RequestUserInput: func(requestCtx context.Context, req PendingInputRequest) (PendingInputAnswer, error) {
						return PendingInputAnswer{
							Answers: map[string][]string{req.Questions[0].ID: {"Continue"}},
							OnDelivered: func(deliveryCtx context.Context) error {
								acknowledgements.Add(1)
								// Force the receipt to finish after the provider's terminal
								// result has been assembled and execution cleanup cancels.
								select {
								case <-requestCtx.Done():
								case <-deliveryCtx.Done():
									return errors.New("receipt outlived execution cleanup: " + secret)
								}
								if tc.ackFails {
									return errors.New("ack endpoint failed: " + secret)
								}
								return nil
							},
						}, nil
					},
				}
				var session *Session
				var err error
				if provider == "claude" {
					session, err = (&claudeBackend{cfg: cfg}).Execute(ctx, "Ask then finish", opts)
				} else {
					session, err = (&codexBackend{cfg: cfg}).executeOnce(ctx, "Ask then finish", opts, 1)
				}
				if err != nil {
					t.Fatal(err)
				}
				var nativeAnswer []byte
				deadline := time.Now().Add(3 * time.Second)
				for time.Now().Before(deadline) {
					nativeAnswer, err = os.ReadFile(answerPath)
					if err == nil && json.Valid(nativeAnswer) {
						break
					}
					time.Sleep(5 * time.Millisecond)
				}
				if !json.Valid(nativeAnswer) {
					t.Fatal("fake provider never read the native answer")
				}
				assertPendingInputDeliveryFailureNativeAnswer(t, provider, nativeAnswer)
				if tc.terminal == "cancel" {
					cancel()
				}
				select {
				case result := <-session.Result:
					if result.Status != tc.wantStatus || !strings.Contains(result.Error, tc.wantError) {
						t.Errorf("result status=%q error=%q, want status=%q error containing %q", result.Status, result.Error, tc.wantStatus, tc.wantError)
					}
					if result.Status == "completed" && result.Error != "" {
						t.Errorf("normal delivery returned an error: %q", result.Error)
					}
					if strings.Contains(result.Error, secret) {
						t.Error("result exposed the raw acknowledgement error")
					}
				case <-time.After(6 * time.Second):
					t.Fatal("terminal result was not published after receipt failure")
				}
				for range session.Messages {
				}
				if got := acknowledgements.Load(); got != 1 {
					t.Errorf("acknowledgement calls=%d, want 1 after the real native write", got)
				}
				logWriter.mu.Lock()
				logText := logs.String()
				logWriter.mu.Unlock()
				if strings.Contains(logText, secret) {
					t.Error("logs exposed the raw acknowledgement error")
				}
				if tc.ackFails && !strings.Contains(logText, "failed to acknowledge delivered user input") {
					t.Error("delivery failure was not logged")
				}
			})
		}
	}
}

func pendingInputDeliveryFailureScript(provider, terminal, answerPath string) string {
	var script string
	if provider == "codex" {
		script = `#!/bin/sh
if [ "$1" = "features" ]; then
  echo 'default_mode_request_user_input under development false'
  exit 0
fi
read line
echo '{"jsonrpc":"2.0","id":1,"result":{}}'
read line
read line
echo '{"jsonrpc":"2.0","id":2,"result":{"config":{"features":{"default_mode_request_user_input":true}}}}'
read line
echo '{"jsonrpc":"2.0","id":3,"result":{"thread":{"id":"thread-delivery"}}}'
read line
echo '{"jsonrpc":"2.0","id":4,"result":{}}'
echo '{"jsonrpc":"2.0","method":"turn/started","params":{"threadId":"thread-delivery","turn":{"id":"turn-delivery"}}}'
echo '{"jsonrpc":"2.0","id":70,"method":"item/tool/requestUserInput","params":{"threadId":"thread-delivery","turnId":"turn-delivery","itemId":"question-delivery","isBlocking":false,"questions":[{"id":"decision","header":"Decision","question":"Continue?","options":[{"label":"Continue","description":"Proceed"},{"label":"Stop","description":"End"}],"isOther":false,"isSecret":false}]}}'
`
	} else {
		script = `#!/bin/sh
IFS= read -r prompt || exit 11
echo '{"type":"system","session_id":"session-delivery"}'
echo '{"type":"control_request","request_id":"request-delivery","session_id":"session-delivery","request":{"subtype":"request_user_dialog","dialog_kind":"permission_ask_user_question","tool_use_id":"tool-delivery","payload":{"questions":[{"question":"Continue?","header":"Decision","options":[{"label":"Continue","description":"Proceed"},{"label":"Stop","description":"End"}],"multiSelect":false}]}}}'
`
	}
	script += fmt.Sprintf("IFS= read -r answer || exit 12\nprintf '%%s\\n' \"$answer\" > %q\n", answerPath)
	if terminal == "cancel" || terminal == "timeout" {
		return script + "while IFS= read -r line; do :; done\n"
	}
	if provider == "codex" {
		if terminal == "aborted" {
			return script + `echo '{"jsonrpc":"2.0","method":"turn/completed","params":{"threadId":"thread-delivery","turn":{"id":"turn-delivery","status":"cancelled"}}}'` + "\n"
		}
		if terminal == "failed" {
			return script + `echo '{"jsonrpc":"2.0","method":"turn/completed","params":{"threadId":"thread-delivery","turn":{"id":"turn-delivery","status":"failed","error":{"message":"original provider failure"}}}}'` + "\n"
		}
		return script + `echo '{"jsonrpc":"2.0","method":"turn/completed","params":{"threadId":"thread-delivery","turn":{"id":"turn-delivery","status":"completed"}}}'` + "\n"
	}
	if terminal == "failed" {
		return script + `echo '{"type":"result","subtype":"error_during_execution","is_error":true,"session_id":"session-delivery","result":"original provider failure"}'` + "\n"
	}
	return script + `echo '{"type":"result","subtype":"success","is_error":false,"session_id":"session-delivery","result":"done"}'` + "\n"
}

func assertPendingInputDeliveryFailureNativeAnswer(t *testing.T, provider string, data []byte) {
	t.Helper()
	if provider == "codex" {
		var response struct {
			ID     int `json:"id"`
			Result struct {
				Answers map[string]struct {
					Answers []string `json:"answers"`
				} `json:"answers"`
			} `json:"result"`
		}
		if err := json.Unmarshal(data, &response); err != nil {
			t.Fatal(err)
		}
		answers := response.Result.Answers["decision"].Answers
		if response.ID != 70 || len(answers) != 1 || answers[0] != "Continue" {
			t.Fatalf("native Codex did not receive the selected answer: %s", data)
		}
		return
	}
	var response struct {
		Response struct {
			Subtype   string `json:"subtype"`
			RequestID string `json:"request_id"`
			Response  struct {
				Answers map[string]string `json:"answers"`
			} `json:"response"`
		} `json:"response"`
	}
	if err := json.Unmarshal(data, &response); err != nil {
		t.Fatal(err)
	}
	if response.Response.Subtype != "success" || response.Response.RequestID != "request-delivery" || response.Response.Response.Answers["Continue?"] != "Continue" {
		t.Fatalf("native Claude did not receive the selected answer: %s", data)
	}
}
