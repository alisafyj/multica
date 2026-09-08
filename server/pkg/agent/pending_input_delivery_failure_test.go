package agent

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"log/slog"
	"strings"
	"testing"
)

func TestPendingInputDeliveryFailureUnknownClaudeDialogWithoutCallbackFailsClosed(t *testing.T) {
	for _, kind := range []string{"unknown_dialog", "secret_prompt", ""} {
		t.Run(kind, func(t *testing.T) {
			var written bytes.Buffer
			backend := &claudeBackend{cfg: Config{Logger: slog.Default()}}
			msg := claudeSDKMessage{
				RequestID: "unknown-dialog-request",
				Request: mustMarshal(t, map[string]any{
					"subtype": "request_user_dialog", "dialog_kind": kind,
				}),
			}
			handled := backend.handleUserInputControlRequest(context.Background(), msg, &written, ExecOptions{}, nil)
			if !handled {
				t.Fatal("unknown dialog without a callback fell through to generic permission allow")
			}
			var response struct {
				Response struct {
					Subtype   string `json:"subtype"`
					RequestID string `json:"request_id"`
					Error     string `json:"error"`
				} `json:"response"`
			}
			if err := json.Unmarshal(written.Bytes(), &response); err != nil {
				t.Fatal(err)
			}
			if response.Response.Subtype != "error" || response.Response.RequestID != msg.RequestID || response.Response.Error == "" {
				t.Fatalf("expected correlated native error, got %s", written.String())
			}
			if strings.Contains(written.String(), `"behavior":"allow"`) {
				t.Fatal("unknown dialog was automatically allowed")
			}
		})
	}
}

func TestPendingInputDeliveryFailureRecordedWithoutLogger(t *testing.T) {
	for _, provider := range []string{"claude", "codex"} {
		t.Run(provider, func(t *testing.T) {
			const secret = "synthetic-receipt-error-without-logger"
			var written bytes.Buffer
			coordinator := newPendingInputCoordinator()
			wroteBeforeAck := false
			callback := func(_ context.Context, request PendingInputRequest) (PendingInputAnswer, error) {
				return PendingInputAnswer{
					Answers: map[string][]string{request.Questions[0].ID: {"Continue"}},
					OnDelivered: func(context.Context) error {
						wroteBeforeAck = written.Len() > 0
						return errors.New(secret)
					},
				}, nil
			}
			if provider == "claude" {
				backend := &claudeBackend{}
				if !backend.handleUserInputControlRequest(context.Background(), claudeAskUserQuestionControlRequest(t, validClaudeQuestionInput()), &written,
					ExecOptions{RequestUserInput: callback}, nil, coordinator) {
					t.Fatal("question was not handled")
				}
			} else {
				request := PendingInputRequest{
					Version: PendingInputVersion1, RequestKey: NewPendingInputRequestKey("test-delivery"), Blocking: true,
					Questions: []PendingInputQuestion{{ID: "q1", Question: "Continue?", AllowOther: true}},
				}
				if coordinator.begin("70", request.RequestKey, pendingInputPayloadHash(request)) != pendingInputAccepted {
					t.Fatal("question was not accepted")
				}
				client := &codexClient{stdin: &written, pendingInputs: coordinator, requestUserInput: callback}
				client.fulfillRequestUserInput("70", json.RawMessage(`70`), request)
			}
			err := coordinator.wait()
			if !errors.Is(err, errPendingInputDeliveryFailed) || strings.Contains(err.Error(), secret) {
				t.Fatalf("delivery failure must be recorded as a safe error without a logger: %v", err)
			}
			if !wroteBeforeAck || strings.Contains(written.String(), secret) {
				t.Fatal("native answer was missing before acknowledgement or exposed its error")
			}
		})
	}
}

func TestPendingInputDeliveryFailureCoordinatorJoinsConcurrentFailures(t *testing.T) {
	coordinator := newPendingInputCoordinator()
	for index := 0; index < pendingInputMaxActiveRequests; index++ {
		id := fmt.Sprintf("request-%d", index)
		if coordinator.begin(id, id, [32]byte{}) != pendingInputAccepted {
			t.Fatal("question was not accepted")
		}
	}
	for index := 0; index < pendingInputMaxActiveRequests; index++ {
		go func() {
			defer coordinator.finish(fmt.Sprintf("request-%d", index))
			if index != 1 {
				coordinator.recordDeliveryFailure()
			}
		}()
	}
	if err := coordinator.wait(); !errors.Is(err, errPendingInputDeliveryFailed) {
		t.Fatalf("concurrent failures were not retained: %v", err)
	}
	if coordinator.begin("later-success", "later-success", [32]byte{}) != pendingInputAccepted {
		t.Fatal("later question was not accepted")
	}
	coordinator.finish("later-success")
	if err := coordinator.wait(); !errors.Is(err, errPendingInputDeliveryFailed) {
		t.Fatalf("later success cleared the delivery failure: %v", err)
	}
}
