package agent

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"log/slog"
	"strings"
	"testing"
)

func TestClaudeCanUseToolAskUserQuestionWritesUpdatedInputBeforeAck(t *testing.T) {
	t.Parallel()

	var written bytes.Buffer
	ackedAfterWrite := false
	callbackCalls := 0
	var received PendingInputRequest
	statuses := make(chan Message, 2)
	coordinator := newPendingInputCoordinator()
	b := &claudeBackend{cfg: Config{Logger: slog.Default()}}
	msg := claudeAskUserQuestionControlRequest(t, map[string]any{
		"questions": []map[string]any{{
			"question": "Which paths?", "header": "Paths", "multiSelect": true,
			"options": []map[string]string{{"label": "A", "description": "First"}, {"label": "B", "description": "Second"}},
		}},
		"probe_marker": "preserve-me",
	})

	handled := b.handleUserInputControlRequest(context.Background(), msg, &written, ExecOptions{
		RequestUserInput: func(_ context.Context, req PendingInputRequest) (PendingInputAnswer, error) {
			callbackCalls++
			received = req
			return PendingInputAnswer{
				Answers: map[string][]string{req.Questions[0].ID: {"A", "B"}},
				OnDelivered: func(context.Context) error {
					ackedAfterWrite = written.Len() > 0
					return nil
				},
			}, nil
		},
	}, statuses, coordinator)
	if !handled {
		t.Fatal("AskUserQuestion can_use_tool request was not handled")
	}
	if !b.handleUserInputControlRequest(context.Background(), msg, &written, ExecOptions{
		RequestUserInput: func(context.Context, PendingInputRequest) (PendingInputAnswer, error) {
			t.Fatal("replayed AskUserQuestion invoked a second callback")
			return PendingInputAnswer{}, nil
		},
	}, statuses, coordinator) {
		t.Fatal("replayed AskUserQuestion can_use_tool request was not handled")
	}
	coordinator.wait()

	if callbackCalls != 1 {
		t.Fatalf("callback calls = %d, want 1", callbackCalls)
	}
	if !received.Blocking || len(received.Questions) != 1 || !received.Questions[0].MultiSelect {
		t.Fatalf("unexpected normalized request: %+v", received)
	}
	if !ackedAfterWrite {
		t.Fatal("delivery was acknowledged before the native response write")
	}
	for _, want := range []string{"waiting_for_input", "running"} {
		if got := (<-statuses).Status; got != want {
			t.Fatalf("status = %q, want %q", got, want)
		}
	}

	response := decodeClaudeControlResponse(t, written.Bytes())
	if response.Behavior != "allow" {
		t.Fatalf("behavior = %q, want allow; response=%s", response.Behavior, written.String())
	}
	if response.UpdatedInput["probe_marker"] != "preserve-me" {
		t.Fatalf("updatedInput did not preserve original input: %+v", response.UpdatedInput)
	}
	answers, ok := response.UpdatedInput["answers"].(map[string]any)
	if !ok || answers["Which paths?"] != "A, B" {
		t.Fatalf("updatedInput answers = %#v, want question-text keyed answer", response.UpdatedInput["answers"])
	}
	if _, ok := response.UpdatedInput["questions"].([]any); !ok {
		t.Fatalf("updatedInput lost original questions: %+v", response.UpdatedInput)
	}
	if got := strings.Count(written.String(), "\n"); got != 1 {
		t.Fatalf("native response writes = %d, want 1", got)
	}
}

func TestClaudeCanUseToolAskUserQuestionNeverFallsThroughToAutoAllow(t *testing.T) {
	t.Parallel()

	for _, tc := range []struct {
		name    string
		ctx     context.Context
		input   map[string]any
		options ExecOptions
	}{
		{
			name:  "no callback",
			ctx:   context.Background(),
			input: validClaudeQuestionInput(),
		},
		{
			name: "secret",
			ctx:  context.Background(),
			input: map[string]any{
				"isSecret":  true,
				"questions": []map[string]any{{"question": "Password?", "header": "Secret"}},
			},
			options: claudePendingInputOptions(func(context.Context, PendingInputRequest) (PendingInputAnswer, error) {
				t.Fatal("secret question reached callback")
				return PendingInputAnswer{}, nil
			}),
		},
		{
			name:  "invalid shape",
			ctx:   context.Background(),
			input: map[string]any{"questions": "not-an-array"},
			options: claudePendingInputOptions(func(context.Context, PendingInputRequest) (PendingInputAnswer, error) {
				t.Fatal("invalid question reached callback")
				return PendingInputAnswer{}, nil
			}),
		},
		{
			name:  "empty answer",
			ctx:   context.Background(),
			input: validClaudeQuestionInput(),
			options: claudePendingInputOptions(func(context.Context, PendingInputRequest) (PendingInputAnswer, error) {
				return PendingInputAnswer{}, nil
			}),
		},
		{
			name: "cancelled",
			ctx: func() context.Context {
				ctx, cancel := context.WithCancel(context.Background())
				cancel()
				return ctx
			}(),
			input: validClaudeQuestionInput(),
			options: claudePendingInputOptions(func(ctx context.Context, _ PendingInputRequest) (PendingInputAnswer, error) {
				return PendingInputAnswer{}, ctx.Err()
			}),
		},
	} {
		t.Run(tc.name, func(t *testing.T) {
			var written bytes.Buffer
			coordinator := newPendingInputCoordinator()
			b := &claudeBackend{cfg: Config{Logger: slog.Default()}}
			handled := b.handleUserInputControlRequest(tc.ctx, claudeAskUserQuestionControlRequest(t, tc.input), &written, tc.options, nil, coordinator)
			if !handled {
				t.Fatal("AskUserQuestion can_use_tool request fell through to automatic permission handling")
			}
			coordinator.wait()
			if !strings.Contains(written.String(), `"subtype":"error"`) {
				t.Fatalf("request did not receive a native error response: %s", written.String())
			}
			if strings.Contains(written.String(), `"behavior":"allow"`) {
				t.Fatalf("request was automatically allowed: %s", written.String())
			}
		})
	}
}

func TestClaudeCanUseToolAskUserQuestionRequiresInteractiveRequest(t *testing.T) {
	t.Parallel()

	var written bytes.Buffer
	b := &claudeBackend{cfg: Config{Logger: slog.Default()}}
	handled := b.handleUserInputControlRequest(context.Background(), claudeSDKMessage{
		RequestID: "request-noninteractive-question",
		SessionID: "session-question",
		Request: mustMarshal(t, map[string]any{
			"subtype":                   "can_use_tool",
			"tool_name":                 "AskUserQuestion",
			"input":                     validClaudeQuestionInput(),
			"tool_use_id":               "tool-question",
			"requires_user_interaction": false,
		}),
	}, &written, claudePendingInputOptions(func(context.Context, PendingInputRequest) (PendingInputAnswer, error) {
		t.Fatal("noninteractive question reached callback")
		return PendingInputAnswer{}, nil
	}), nil)
	if !handled {
		t.Fatal("AskUserQuestion can_use_tool request fell through to automatic permission handling")
	}
	if !strings.Contains(written.String(), `"subtype":"error"`) || strings.Contains(written.String(), `"behavior":"allow"`) {
		t.Fatalf("noninteractive question response = %s", written.String())
	}
}

func TestClaudeCanUseToolAskUserQuestionWriteFailureDoesNotAckDelivery(t *testing.T) {
	t.Parallel()

	acked := false
	coordinator := newPendingInputCoordinator()
	b := &claudeBackend{cfg: Config{Logger: slog.Default()}}
	handled := b.handleUserInputControlRequest(context.Background(), claudeAskUserQuestionControlRequest(t, validClaudeQuestionInput()), failingClaudeWriter{}, claudePendingInputOptions(
		func(_ context.Context, req PendingInputRequest) (PendingInputAnswer, error) {
			return PendingInputAnswer{
				Answers: map[string][]string{req.Questions[0].ID: {"Continue"}},
				OnDelivered: func(context.Context) error {
					acked = true
					return nil
				},
			}, nil
		},
	), nil, coordinator)
	if !handled {
		t.Fatal("AskUserQuestion can_use_tool request was not handled")
	}
	coordinator.wait()
	if acked {
		t.Fatal("failed native write was acknowledged as delivered")
	}
}

func TestClaudeCanUseToolNonQuestionPermissionRemainsUnchanged(t *testing.T) {
	t.Parallel()

	b := &claudeBackend{cfg: Config{Logger: slog.Default()}}
	handled := b.handleUserInputControlRequest(context.Background(), claudeSDKMessage{
		RequestID: "request-bash",
		Request: mustMarshal(t, map[string]any{
			"subtype": "can_use_tool", "tool_name": "Bash", "input": map[string]any{"command": "pwd"},
		}),
	}, &bytes.Buffer{}, claudePendingInputOptions(func(context.Context, PendingInputRequest) (PendingInputAnswer, error) {
		t.Fatal("non-question permission reached pending input callback")
		return PendingInputAnswer{}, nil
	}), nil)
	if handled {
		t.Fatal("non-question can_use_tool permission must remain on the existing permission path")
	}
}

func claudeAskUserQuestionControlRequest(t *testing.T, input map[string]any) claudeSDKMessage {
	t.Helper()
	return claudeSDKMessage{
		RequestID: "request-question",
		SessionID: "session-question",
		Request: mustMarshal(t, map[string]any{
			"subtype":                   "can_use_tool",
			"tool_name":                 "AskUserQuestion",
			"display_name":              "AskUserQuestion",
			"input":                     input,
			"tool_use_id":               "tool-question",
			"requires_user_interaction": true,
		}),
	}
}

func validClaudeQuestionInput() map[string]any {
	return map[string]any{"questions": []map[string]any{{
		"question": "Continue?", "header": "Decision", "multiSelect": false,
		"options": []map[string]string{{"label": "Continue", "description": "Proceed with the task."}, {"label": "Stop", "description": "End the task."}},
	}}}
}

func claudePendingInputOptions(callback func(context.Context, PendingInputRequest) (PendingInputAnswer, error)) ExecOptions {
	return ExecOptions{RequestUserInput: callback}
}

type failingClaudeWriter struct{}

func (failingClaudeWriter) Write([]byte) (int, error) {
	return 0, errors.New("forced write failure")
}

type decodedClaudeControlResponse struct {
	Behavior     string
	UpdatedInput map[string]any
}

func decodeClaudeControlResponse(t *testing.T, data []byte) decodedClaudeControlResponse {
	t.Helper()
	var wire struct {
		Response struct {
			Response struct {
				Behavior     string         `json:"behavior"`
				UpdatedInput map[string]any `json:"updatedInput"`
			} `json:"response"`
		} `json:"response"`
	}
	if err := json.Unmarshal(bytes.TrimSpace(data), &wire); err != nil {
		t.Fatal(err)
	}
	return decodedClaudeControlResponse{
		Behavior:     wire.Response.Response.Behavior,
		UpdatedInput: wire.Response.Response.UpdatedInput,
	}
}
