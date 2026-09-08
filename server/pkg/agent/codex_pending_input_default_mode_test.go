package agent

import (
	"bytes"
	"context"
	"encoding/json"
	"log/slog"
	"testing"
	"time"
)

func TestCodexDefaultModeUserInputWaitsForActualAnswer(t *testing.T) {
	var written bytes.Buffer
	started := make(chan PendingInputRequest, 1)
	release := make(chan struct{})
	delivered := make(chan struct{})
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	c := &codexClient{
		cfg: Config{Logger: slog.Default()}, stdin: &written, requestContext: ctx,
		requestUserInput: func(ctx context.Context, req PendingInputRequest) (PendingInputAnswer, error) {
			started <- req
			select {
			case <-release:
			case <-ctx.Done():
				return PendingInputAnswer{}, ctx.Err()
			}
			return PendingInputAnswer{
				Answers:     map[string][]string{"boundary": {"Inclusive"}},
				OnDelivered: func(context.Context) error { close(delivered); return nil },
			}, nil
		},
	}
	c.handleLine(`{"id":"default-question","method":"item/tool/requestUserInput","params":{"threadId":"thread-1","turnId":"turn-1","itemId":"item-1","isBlocking":false,"questions":[{"id":"boundary","header":"Low stock","question":"Include equality?","options":[{"label":"Inclusive","description":"Include equality"},{"label":"Exclusive","description":"Below only"}]}]}}`)
	select {
	case req := <-started:
		if !req.Blocking {
			t.Fatal("managed issue question must wait for an actual answer")
		}
	case <-time.After(time.Second):
		t.Fatal("default-mode native question was rejected before the broker")
	}
	if written.Len() != 0 {
		t.Fatal("native response was written before the owner answered")
	}
	close(release)
	select {
	case <-delivered:
	case <-time.After(time.Second):
		t.Fatal("answer was not delivered")
	}
	var response struct {
		ID     string `json:"id"`
		Result struct {
			Answers map[string]struct {
				Answers []string `json:"answers"`
			} `json:"answers"`
		} `json:"result"`
	}
	if err := json.Unmarshal(bytes.TrimSpace(written.Bytes()), &response); err != nil {
		t.Fatal(err)
	}
	if response.ID != "default-question" || len(response.Result.Answers["boundary"].Answers) != 1 || response.Result.Answers["boundary"].Answers[0] != "Inclusive" {
		t.Fatalf("unexpected native answer: %+v", response)
	}
}

func TestCodexUserInputRejectsMissingProtocolFields(t *testing.T) {
	for _, input := range []string{
		`{"threadId":"t","turnId":"u","itemId":"i"}`,
		`{"threadId":"t","turnId":"u","itemId":"i","isBlocking":null}`,
		`{"threadId":"t","turnId":"u","itemId":"i","isBlocking":"false"}`,
		`{"threadId":"","turnId":"u","itemId":"i","isBlocking":false}`,
	} {
		var params map[string]any
		if err := json.Unmarshal([]byte(input), &params); err != nil {
			t.Fatal(err)
		}
		params["questions"] = []any{map[string]any{"id": "q", "question": "Choose?", "header": "Choice", "options": nil}}
		encoded, err := json.Marshal(params)
		if err != nil {
			t.Fatal(err)
		}
		var written bytes.Buffer
		c := &codexClient{cfg: Config{Logger: slog.Default()}, stdin: &written,
			requestUserInput: func(context.Context, PendingInputRequest) (PendingInputAnswer, error) {
				t.Error("malformed protocol input reached broker")
				return PendingInputAnswer{}, nil
			},
		}
		c.handleRequestUserInput(json.RawMessage(`1`), encoded)
		if !bytes.Contains(written.Bytes(), []byte(`"error"`)) {
			t.Errorf("missing protocol error for %s", input)
		}
	}
}
