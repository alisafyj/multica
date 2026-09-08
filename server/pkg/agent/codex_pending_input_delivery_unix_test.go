//go:build !windows

package agent

import (
	"context"
	"io"
	"log/slog"
	"os"
	"path/filepath"
	"testing"
	"time"
)

func TestCodexResultWaitsForPendingInputDelivery(t *testing.T) {
	fake := filepath.Join(t.TempDir(), "codex")
	script := `#!/bin/sh
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
echo '{"jsonrpc":"2.0","id":70,"method":"item/tool/requestUserInput","params":{"threadId":"thread-delivery","turnId":"turn-delivery","itemId":"question-delivery","isBlocking":false,"questions":[{"id":"boundary","header":"Boundary","question":"Include equality?","options":[{"label":"Inclusive","description":"Include equality"},{"label":"Strict","description":"Exclude equality"}],"isOther":false,"isSecret":false}]}}'
read line
echo '{"jsonrpc":"2.0","method":"turn/completed","params":{"threadId":"thread-delivery","turn":{"id":"turn-delivery","status":"completed"}}}'
`
	if err := os.WriteFile(fake, []byte(script), 0o755); err != nil {
		t.Fatal(err)
	}
	started := make(chan struct{})
	release := make(chan struct{})
	done := make(chan struct{})
	ctx, cancel := context.WithTimeout(t.Context(), 10*time.Second)
	defer cancel()
	b := &codexBackend{cfg: Config{ExecutablePath: fake, Logger: slog.New(slog.NewTextHandler(io.Discard, nil))}}
	session, err := b.executeOnce(ctx, "Clarify then finish", ExecOptions{Cwd: t.TempDir(), Timeout: 5 * time.Second,
		RequestUserInput: func(context.Context, PendingInputRequest) (PendingInputAnswer, error) {
			return PendingInputAnswer{Answers: map[string][]string{"boundary": {"Inclusive"}}, OnDelivered: func(ctx context.Context) error {
				close(started)
				defer close(done)
				select {
				case <-release:
					return nil
				case <-ctx.Done():
					return ctx.Err()
				}
			}}, nil
		}}, 1)
	if err != nil {
		t.Fatal(err)
	}
	select {
	case <-started:
	case result := <-session.Result:
		t.Fatalf("result arrived without a question delivery: %+v", result)
	case <-ctx.Done():
		t.Fatal("question delivery did not start")
	}
	var early *Result
	select {
	case result := <-session.Result:
		early = &result
		t.Error("terminal result became visible before answer delivery receipt completed")
	case <-done:
		t.Error("normal completion cancelled the delivery callback")
	case <-time.After(150 * time.Millisecond):
	}
	close(release)
	if early == nil {
		select {
		case result := <-session.Result:
			if result.Status != "completed" {
				t.Fatalf("result after acknowledgement: %+v", result)
			}
		case <-ctx.Done():
			t.Fatal("result did not follow acknowledgement")
		}
	}
	for range session.Messages {
	}
	<-done
}
