//go:build !windows

package agent

import (
	"context"
	"log/slog"
	"reflect"
	"strings"
	"testing"
	"time"
)

func TestCodexInFlightToolTimeoutRemainsBounded(t *testing.T) {
	codexInFlightToolTimeoutNanos.Store(0)
	t.Cleanup(func() { codexInFlightToolTimeoutNanos.Store(0) })

	if got := codexInFlightToolTimeout(ExecOptions{SemanticInactivityTimeout: time.Minute}); got != defaultCodexInFlightToolTimeout {
		t.Fatalf("default tool timeout = %s, want %s", got, defaultCodexInFlightToolTimeout)
	}
	if got := codexInFlightToolTimeout(ExecOptions{SemanticInactivityTimeout: 3 * time.Hour}); got != 3*time.Hour {
		t.Fatalf("explicit longer semantic ceiling = %s, want 3h", got)
	}
	if got := codexInFlightToolTimeout(ExecOptions{SemanticInactivityTimeout: time.Minute, InFlightToolTimeout: 6 * time.Hour}); got != 6*time.Hour {
		t.Fatalf("dedicated tool timeout = %s, want 6h", got)
	}
	if got := codexInFlightToolTimeout(ExecOptions{SemanticInactivityTimeout: 3 * time.Hour, InFlightToolTimeout: time.Hour}); got != 3*time.Hour {
		t.Fatalf("dedicated tool timeout must not narrow semantic ceiling: got %s, want 3h", got)
	}
	if got := codexInFlightToolTimeout(ExecOptions{InFlightToolTimeout: time.Minute}); got != defaultCodexSemanticInactivityTimeout {
		t.Fatalf("dedicated tool timeout must not narrow default semantic ceiling: got %s, want %s", got, defaultCodexSemanticInactivityTimeout)
	}

	codexInFlightToolTimeoutNanos.Store(int64(250 * time.Millisecond))
	if got := codexInFlightToolTimeout(ExecOptions{SemanticInactivityTimeout: 100 * time.Millisecond, InFlightToolTimeout: time.Hour}); got != 250*time.Millisecond {
		t.Fatalf("test override = %s, want 250ms", got)
	}
	if got := codexInFlightToolTimeout(ExecOptions{SemanticInactivityTimeout: 500 * time.Millisecond}); got != 500*time.Millisecond {
		t.Fatalf("tool timeout must not narrow the semantic ceiling: got %s, want 500ms", got)
	}
}

func TestCodexToolTransitionRequiresRealCallID(t *testing.T) {
	cases := []struct {
		name string
		msg  Message
		want codexToolTransition
		ok   bool
	}{
		{name: "tool start", msg: Message{Type: MessageToolUse, Tool: "exec_command", CallID: "cmd-1"}, want: codexToolTransition{activity: codexToolStarted, callID: "cmd-1"}, ok: true},
		{name: "tool result", msg: Message{Type: MessageToolResult, Tool: "exec_command", CallID: "cmd-1"}, want: codexToolTransition{activity: codexToolCompleted, callID: "cmd-1"}, ok: true},
		{name: "plan display", msg: Message{Type: MessageToolUse, Tool: "todo_write"}},
		{name: "ordinary message", msg: Message{Type: MessageText, CallID: "msg-1"}},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			got, ok := codexToolTransitionFromMessage(tc.msg)
			if ok != tc.ok || !reflect.DeepEqual(got, tc.want) {
				t.Fatalf("transition = %+v, %v; want %+v, %v", got, ok, tc.want, tc.ok)
			}
		})
	}
}

func TestCodexItemProgressActivityRejectsHeartbeats(t *testing.T) {
	cases := []struct {
		method string
		want   bool
	}{
		{method: "item/started", want: true},
		{method: "item/completed", want: true},
		{method: "item/commandExecution/outputDelta", want: true},
		{method: "item/mcpToolCall/progress", want: false},
	}
	for _, tc := range cases {
		if got := isCodexItemProgressActivity(tc.method); got != tc.want {
			t.Errorf("isCodexItemProgressActivity(%q) = %v, want %v", tc.method, got, tc.want)
		}
	}
}

func TestCodexExecuteAllowsSilentLongToolPastSemanticInactivity(t *testing.T) {
	fakePath := writeFakeCodexAppServer(t, ""+
		`read line`+"\n"+
		`echo '{"jsonrpc":"2.0","id":1,"result":{}}'`+"\n"+
		`read line`+"\n"+
		`read line`+"\n"+
		`echo '{"jsonrpc":"2.0","id":2,"result":{"thread":{"id":"thr-long-tool"}}}'`+"\n"+
		`read line`+"\n"+
		`echo '{"jsonrpc":"2.0","id":3,"result":{}}'`+"\n"+
		`echo '{"jsonrpc":"2.0","method":"turn/started","params":{"threadId":"thr-long-tool","turn":{"id":"turn-long-tool"}}}'`+"\n"+
		`echo '{"jsonrpc":"2.0","method":"item/started","params":{"threadId":"thr-long-tool","turnId":"turn-long-tool","item":{"type":"commandExecution","id":"cmd-long","command":"go test ./..."}}}'`+"\n"+
		`sleep 0.25`+"\n"+
		`echo '{"jsonrpc":"2.0","method":"item/completed","params":{"threadId":"thr-long-tool","turnId":"turn-long-tool","item":{"type":"commandExecution","id":"cmd-long","aggregatedOutput":"ok"}}}'`+"\n"+
		`echo '{"jsonrpc":"2.0","method":"item/completed","params":{"threadId":"thr-long-tool","turnId":"turn-long-tool","item":{"type":"agentMessage","id":"msg-final","phase":"final_answer","text":"Done"}}}'`+"\n"+
		`echo '{"jsonrpc":"2.0","method":"turn/completed","params":{"threadId":"thr-long-tool","turn":{"id":"turn-long-tool","status":"completed"}}}'`+"\n")

	result := executeFakeCodex(t, fakePath, ExecOptions{
		Timeout:                   2 * time.Second,
		SemanticInactivityTimeout: 100 * time.Millisecond,
		InFlightToolTimeout:       500 * time.Millisecond,
	})
	if result.Status != "completed" {
		t.Fatalf("silent in-flight tool should use its own watchdog, got status=%q error=%q", result.Status, result.Error)
	}
	if result.Output != "Done" {
		t.Fatalf("output = %q, want Done", result.Output)
	}
}

func TestCodexExecuteInFlightToolWatchdogIgnoresHeartbeats(t *testing.T) {
	fakePath := writeFakeCodexAppServer(t, ""+
		`read line`+"\n"+
		`echo '{"jsonrpc":"2.0","id":1,"result":{}}'`+"\n"+
		`read line`+"\n"+
		`read line`+"\n"+
		`echo '{"jsonrpc":"2.0","id":2,"result":{"thread":{"id":"thr-tool-heartbeat"}}}'`+"\n"+
		`read line`+"\n"+
		`echo '{"jsonrpc":"2.0","id":3,"result":{}}'`+"\n"+
		`echo '{"jsonrpc":"2.0","method":"turn/started","params":{"threadId":"thr-tool-heartbeat","turn":{"id":"turn-tool-heartbeat"}}}'`+"\n"+
		`echo '{"jsonrpc":"2.0","method":"item/started","params":{"threadId":"thr-tool-heartbeat","turnId":"turn-tool-heartbeat","item":{"type":"mcpToolCall","id":"mcp-stuck","server":"fetch","tool":"crawl"}}}'`+"\n"+
		`i=0`+"\n"+
		`while [ $i -lt 8 ]; do`+"\n"+
		`  echo '{"jsonrpc":"2.0","method":"item/mcpToolCall/progress","params":{"threadId":"thr-tool-heartbeat","turnId":"turn-tool-heartbeat","item":{"type":"mcpToolCall","id":"mcp-stuck"},"progress":{"message":"still running"}}}'`+"\n"+
		`  echo '{"jsonrpc":"2.0","method":"error","params":{"error":{"message":"temporary reconnect"},"willRetry":true}}'`+"\n"+
		`  sleep 0.05`+"\n"+
		`  i=$((i+1))`+"\n"+
		`done`+"\n")

	result := executeFakeCodex(t, fakePath, ExecOptions{
		Timeout:                   2 * time.Second,
		SemanticInactivityTimeout: 80 * time.Millisecond,
		InFlightToolTimeout:       250 * time.Millisecond,
	})
	if result.Status != "timeout" {
		t.Fatalf("expected bounded in-flight tool timeout, got status=%q error=%q", result.Status, result.Error)
	}
	for _, want := range []string{CodexSemanticInactivityMarker, "in-flight tool exceeded 250ms", "tool_in_flight:mcp-stuck"} {
		if !strings.Contains(result.Error, want) {
			t.Fatalf("tool timeout missing %q: %q", want, result.Error)
		}
	}
}

func TestCodexExecuteTotalDeadlineWinsDuringInFlightTool(t *testing.T) {
	codexInFlightToolTimeoutNanos.Store(int64(2 * time.Second))
	t.Cleanup(func() { codexInFlightToolTimeoutNanos.Store(0) })

	fakePath := writeFakeCodexAppServer(t, ""+
		`read line`+"\n"+
		`echo '{"jsonrpc":"2.0","id":1,"result":{}}'`+"\n"+
		`read line`+"\n"+
		`read line`+"\n"+
		`echo '{"jsonrpc":"2.0","id":2,"result":{"thread":{"id":"thr-tool-deadline"}}}'`+"\n"+
		`read line`+"\n"+
		`echo '{"jsonrpc":"2.0","id":3,"result":{}}'`+"\n"+
		`echo '{"jsonrpc":"2.0","method":"turn/started","params":{"threadId":"thr-tool-deadline","turn":{"id":"turn-tool-deadline"}}}'`+"\n"+
		`echo '{"jsonrpc":"2.0","method":"item/started","params":{"threadId":"thr-tool-deadline","turnId":"turn-tool-deadline","item":{"type":"commandExecution","id":"cmd-deadline","command":"sleep 30"}}}'`+"\n"+
		`sleep 3`+"\n")

	result := executeFakeCodex(t, fakePath, ExecOptions{
		Timeout:                   time.Second,
		SemanticInactivityTimeout: 80 * time.Millisecond,
	})
	if result.Status != "timeout" || !strings.Contains(result.Error, "codex timed out after 1s") {
		t.Fatalf("execution deadline must win, got status=%q error=%q", result.Status, result.Error)
	}
	if strings.Contains(result.Error, "in-flight tool exceeded") {
		t.Fatalf("tool watchdog must not replace the earlier execution deadline: %q", result.Error)
	}
}

func TestCodexExecuteCancellationWinsDuringInFlightTool(t *testing.T) {
	codexInFlightToolTimeoutNanos.Store(int64(2 * time.Second))
	t.Cleanup(func() { codexInFlightToolTimeoutNanos.Store(0) })

	fakePath := writeFakeCodexAppServer(t, ""+
		`read line`+"\n"+
		`echo '{"jsonrpc":"2.0","id":1,"result":{}}'`+"\n"+
		`read line`+"\n"+
		`read line`+"\n"+
		`echo '{"jsonrpc":"2.0","id":2,"result":{"thread":{"id":"thr-tool-cancel"}}}'`+"\n"+
		`read line`+"\n"+
		`echo '{"jsonrpc":"2.0","id":3,"result":{}}'`+"\n"+
		`echo '{"jsonrpc":"2.0","method":"turn/started","params":{"threadId":"thr-tool-cancel","turn":{"id":"turn-tool-cancel"}}}'`+"\n"+
		`echo '{"jsonrpc":"2.0","method":"item/started","params":{"threadId":"thr-tool-cancel","turnId":"turn-tool-cancel","item":{"type":"commandExecution","id":"cmd-cancel","command":"sleep 30"}}}'`+"\n"+
		`sleep 3`+"\n")

	backend, err := New("codex", Config{ExecutablePath: fakePath, Logger: slog.Default()})
	if err != nil {
		t.Fatalf("new codex backend: %v", err)
	}
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	session, err := backend.Execute(ctx, "prompt", ExecOptions{SemanticInactivityTimeout: 80 * time.Millisecond})
	if err != nil {
		t.Fatalf("execute: %v", err)
	}
	cancelled := false
	for {
		select {
		case msg, ok := <-session.Messages:
			if ok && msg.Type == MessageToolUse && msg.CallID == "cmd-cancel" && !cancelled {
				cancelled = true
				cancel()
			}
		case result := <-session.Result:
			if !cancelled {
				t.Fatal("run ended before the in-flight tool was observed")
			}
			if result.Status != "aborted" || !strings.Contains(result.Error, "execution cancelled") {
				t.Fatalf("cancellation must win, got status=%q error=%q", result.Status, result.Error)
			}
			return
		case <-time.After(5 * time.Second):
			t.Fatal("timeout waiting for cancelled Codex run")
		}
	}
}
