//go:build !windows

package agent

import (
	"context"
	"log/slog"
	"os"
	"path/filepath"
	"testing"
	"time"
)

func TestClaudeExecuteWaitsForPendingInputDeliveryBeforePublishingTerminalResult(t *testing.T) {
	t.Parallel()

	fakeClaude := filepath.Join(t.TempDir(), "claude")
	script := `#!/bin/sh
IFS= read -r prompt || exit 11
printf '%s\n' '{"type":"system","session_id":"session-question"}'
printf '%s\n' '{"type":"control_request","request_id":"request-question","session_id":"session-question","request":{"subtype":"request_user_dialog","dialog_kind":"permission_ask_user_question","tool_use_id":"tool-question","payload":{"questions":[{"question":"Continue?","header":"Decision","options":[{"label":"Continue","description":"Proceed with the task."},{"label":"Stop","description":"End the task."}],"multiSelect":false}]}}}'
IFS= read -r answer || exit 12
printf '%s\n' '{"type":"assistant","session_id":"session-question","message":{"role":"assistant","content":[{"type":"text","text":"terminal-ready"}]}}'
printf '%s\n' '{"type":"result","subtype":"success","is_error":false,"session_id":"session-question","result":"done"}'
`
	if err := os.WriteFile(fakeClaude, []byte(script), 0o700); err != nil {
		t.Fatal(err)
	}

	backend, err := New("claude", Config{
		ExecutablePath: fakeClaude,
		Logger:         slog.Default(),
	})
	if err != nil {
		t.Fatal(err)
	}
	releaseAck := make(chan struct{})
	defer func() {
		select {
		case <-releaseAck:
		default:
			close(releaseAck)
		}
	}()
	ackContext := make(chan context.Context, 1)
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	session, err := backend.Execute(ctx, "ask a question", ExecOptions{
		Timeout: 4 * time.Second,
		RequestUserInput: func(_ context.Context, req PendingInputRequest) (PendingInputAnswer, error) {
			return PendingInputAnswer{
				Answers: map[string][]string{req.Questions[0].ID: {"Continue"}},
				OnDelivered: func(ctx context.Context) error {
					ackContext <- ctx
					<-releaseAck
					return ctx.Err()
				},
			}, nil
		},
	})
	if err != nil {
		t.Fatal(err)
	}

	var deliveryCtx context.Context
	select {
	case deliveryCtx = <-ackContext:
	case <-time.After(2 * time.Second):
		t.Fatal("delivery acknowledgement did not start")
	}
	for {
		select {
		case message, ok := <-session.Messages:
			if !ok {
				t.Fatal("message stream closed before terminal marker")
			}
			if message.Type == MessageText && message.Content == "terminal-ready" {
				goto terminalObserved
			}
		case <-time.After(2 * time.Second):
			t.Fatal("fake Claude terminal marker was not observed")
		}
	}

terminalObserved:
	select {
	case <-deliveryCtx.Done():
		t.Fatalf("delivery acknowledgement context was cancelled before its own bound: %v", deliveryCtx.Err())
	case result := <-session.Result:
		t.Fatalf("terminal result became visible before delivery acknowledgement completed: %+v", result)
	case <-time.After(150 * time.Millisecond):
	}

	close(releaseAck)
	select {
	case result := <-session.Result:
		if result.Status != "completed" || result.Output != "done" {
			t.Fatalf("unexpected terminal result after delivery acknowledgement: %+v", result)
		}
	case <-time.After(2 * time.Second):
		t.Fatal("terminal result did not become visible after delivery acknowledgement")
	}
}
