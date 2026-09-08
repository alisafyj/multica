package agent

import (
	"fmt"
	"log/slog"
	"reflect"
	"runtime"
	"strings"
	"testing"
	"time"
)

func TestCodexToolWatchdogReviewNotificationBurstDoesNotBlockTurnStartResponse(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("fake app-server fixture uses a POSIX shell")
	}

	fakePath := writeFakeCodexAppServer(t, ""+
		`read line`+"\n"+
		`echo '{"jsonrpc":"2.0","id":1,"result":{}}'`+"\n"+
		`read line`+"\n"+
		`read line`+"\n"+
		`echo '{"jsonrpc":"2.0","id":2,"result":{"thread":{"id":"thr-burst"}}}'`+"\n"+
		`read line`+"\n"+
		`i=0`+"\n"+
		`while [ $i -lt 257 ]; do`+"\n"+
		`  printf '{"jsonrpc":"2.0","method":"item/started","params":{"threadId":"thr-burst","turnId":"turn-burst","item":{"type":"commandExecution","id":"cmd-%s","command":"true"}}}\n' "$i"`+"\n"+
		`  printf '{"jsonrpc":"2.0","method":"item/completed","params":{"threadId":"thr-burst","turnId":"turn-burst","item":{"type":"commandExecution","id":"cmd-%s","aggregatedOutput":"ok"}}}\n' "$i"`+"\n"+
		`  i=$((i+1))`+"\n"+
		`done`+"\n"+
		`echo '{"jsonrpc":"2.0","id":3,"result":{}}'`+"\n"+
		`echo '{"jsonrpc":"2.0","method":"item/completed","params":{"threadId":"thr-burst","turnId":"turn-burst","item":{"type":"agentMessage","id":"msg-final","phase":"final_answer","text":"Done"}}}'`+"\n"+
		`echo '{"jsonrpc":"2.0","method":"turn/completed","params":{"threadId":"thr-burst","turn":{"id":"turn-burst","status":"completed"}}}'`+"\n")

	result := executeFakeCodex(t, fakePath, ExecOptions{
		Timeout:          3 * time.Second,
		HandshakeTimeout: time.Second,
	})
	if result.Status != "completed" || result.Output != "Done" {
		t.Fatalf("notification burst blocked turn/start response: status=%q output=%q error=%q", result.Status, result.Output, result.Error)
	}
}

func TestCodexToolWatchdogReviewToolStartSurvivesDroppedSemanticNotification(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("fake app-server fixture uses a POSIX shell")
	}

	fakePath := writeFakeCodexAppServer(t, ""+
		`read line`+"\n"+
		`echo '{"jsonrpc":"2.0","id":1,"result":{}}'`+"\n"+
		`read line`+"\n"+
		`read line`+"\n"+
		`echo '{"jsonrpc":"2.0","id":2,"result":{"thread":{"id":"thr-first-progress"}}}'`+"\n"+
		`read line`+"\n"+
		`echo '{"jsonrpc":"2.0","method":"turn/started","params":{"threadId":"thr-first-progress","turn":{"id":"turn-first-progress"}}}'`+"\n"+
		`i=0`+"\n"+
		`while [ $i -lt 300 ]; do`+"\n"+
		`  echo '{"jsonrpc":"2.0","method":"error","params":{"error":{"message":"retry fixture"},"willRetry":true}}'`+"\n"+
		`  i=$((i+1))`+"\n"+
		`done`+"\n"+
		`echo '{"jsonrpc":"2.0","method":"item/started","params":{"threadId":"thr-first-progress","turnId":"turn-first-progress","item":{"type":"commandExecution","id":"cmd-silent","command":"sleep"}}}'`+"\n"+
		`echo '{"jsonrpc":"2.0","id":3,"result":{}}'`+"\n"+
		`sleep 0.25`+"\n"+
		`echo '{"jsonrpc":"2.0","method":"item/completed","params":{"threadId":"thr-first-progress","turnId":"turn-first-progress","item":{"type":"commandExecution","id":"cmd-silent","aggregatedOutput":"ok"}}}'`+"\n"+
		`echo '{"jsonrpc":"2.0","method":"item/completed","params":{"threadId":"thr-first-progress","turnId":"turn-first-progress","item":{"type":"agentMessage","id":"msg-final","phase":"final_answer","text":"Done"}}}'`+"\n"+
		`echo '{"jsonrpc":"2.0","method":"turn/completed","params":{"threadId":"thr-first-progress","turn":{"id":"turn-first-progress","status":"completed"}}}'`+"\n")

	result := executeFakeCodex(t, fakePath, ExecOptions{
		Timeout:                    3 * time.Second,
		HandshakeTimeout:           time.Second,
		SemanticInactivityTimeout:  time.Second,
		FirstTurnNoProgressTimeout: 100 * time.Millisecond,
	})
	if result.Status != "completed" || result.Output != "Done" {
		t.Fatalf("observed tool start did not count as first-turn progress: status=%q output=%q error=%q", result.Status, result.Output, result.Error)
	}
}

func TestCodexToolWatchdogReviewLegacyAndRawLifecyclesAreBalanced(t *testing.T) {
	cases := []struct {
		name  string
		lines []string
		want  []codexToolTransition
	}{
		{
			name: "legacy command",
			lines: []string{
				`{"jsonrpc":"2.0","method":"codex/event","params":{"msg":{"type":"exec_command_begin","call_id":"legacy-1","command":"true"}}}`,
				`{"jsonrpc":"2.0","method":"codex/event","params":{"msg":{"type":"exec_command_end","call_id":"legacy-1","output":"ok"}}}`,
			},
			want: []codexToolTransition{
				{activity: codexToolStarted, callID: "legacy-1"},
				{activity: codexToolCompleted, callID: "legacy-1"},
			},
		},
		{
			name: "raw failed MCP call",
			lines: []string{
				`{"jsonrpc":"2.0","method":"item/started","params":{"threadId":"raw-thread","turnId":"raw-turn","item":{"type":"mcpToolCall","id":"raw-1","server":"fixture","tool":"lookup","status":"inProgress"}}}`,
				`{"jsonrpc":"2.0","method":"item/completed","params":{"threadId":"raw-thread","turnId":"raw-turn","item":{"type":"mcpToolCall","id":"raw-1","server":"fixture","tool":"lookup","status":"failed","error":{"message":"fixture failure"}}}}`,
			},
			want: []codexToolTransition{
				{activity: codexToolStarted, callID: "raw-1"},
				{activity: codexToolCompleted, callID: "raw-1"},
			},
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			var got []codexToolTransition
			client := &codexClient{
				cfg:                  Config{Logger: slog.Default()},
				notificationProtocol: "unknown",
				onMessage: func(msg Message) {
					if transition, ok := codexToolTransitionFromMessage(msg); ok {
						got = append(got, transition)
					}
				},
			}
			if tc.name == "raw failed MCP call" {
				client.threadID = "raw-thread"
			}
			for _, line := range tc.lines {
				client.handleLine(line)
			}
			if !reflect.DeepEqual(got, tc.want) {
				t.Fatalf("tool transitions = %s, want %s", formatReviewTransitions(got), formatReviewTransitions(tc.want))
			}
		})
	}
}

func TestCodexToolWatchdogReviewTimeoutPreservesDaemonRoutingAndSpecificDiagnostic(t *testing.T) {
	errorText := buildCodexTimeoutDiagnosticError(codexTimeoutDiagnostic{
		Kind:         codexTimeoutInFlightTool,
		Timeout:      2 * time.Hour,
		LastActivity: "tool_in_flight:cmd-1",
	}, "")
	if !strings.Contains(errorText, CodexSemanticInactivityMarker) {
		t.Fatalf("in-flight timeout lost the marker used by daemon resume-safety routing: %q", errorText)
	}
	if !strings.Contains(errorText, "in-flight tool exceeded") {
		t.Fatalf("in-flight tool timeout lost its specific classification: %q", errorText)
	}
}

func TestCodexToolWatchdogReviewStateUsesObservedStartAndIgnoresDuplicates(t *testing.T) {
	outOfOrder := newCodexInFlightToolState()
	outOfOrder.observe(codexToolTransition{activity: codexToolStarted, callID: "before-turn"}, time.Unix(90, 0))
	outOfOrder.observeTurnStarted(time.Unix(91, 0))
	if snapshot := outOfOrder.snapshot(); !snapshot.firstProgressAt.IsZero() {
		t.Fatalf("tool start before status:running counted as first-turn progress at %s", snapshot.firstProgressAt)
	}

	state := newCodexInFlightToolState()
	startedAt := time.Unix(100, 0)
	duplicateAt := startedAt.Add(time.Hour)
	completedAt := duplicateAt.Add(time.Minute)
	start := codexToolTransition{activity: codexToolStarted, callID: "cmd-1"}

	state.observeTurnStarted(startedAt.Add(-time.Second))
	state.observe(start, startedAt)
	state.observe(start, duplicateAt)
	snapshot := state.snapshot()
	if snapshot.count != 1 || snapshot.oldestCallID != "cmd-1" || !snapshot.oldestStartedAt.Equal(startedAt) {
		t.Fatalf("duplicate start changed initial lifecycle state: %+v", snapshot)
	}
	if snapshot.version != 1 {
		t.Fatalf("duplicate start advanced state version to %d, want 1", snapshot.version)
	}
	if !snapshot.firstProgressAt.Equal(startedAt) {
		t.Fatalf("first progress timestamp = %s, want %s", snapshot.firstProgressAt, startedAt)
	}

	state.observe(codexToolTransition{activity: codexToolCompleted, callID: "cmd-1"}, completedAt)
	snapshot = state.snapshot()
	if snapshot.count != 0 || !snapshot.becameIdleAt.Equal(completedAt) || snapshot.version != 2 {
		t.Fatalf("completion did not close lifecycle at observation time: %+v", snapshot)
	}

	for i := 0; i < 1000; i++ {
		callID := fmt.Sprintf("burst-%d", i)
		state.observe(codexToolTransition{activity: codexToolStarted, callID: callID}, completedAt.Add(time.Duration(i+1)))
		state.observe(codexToolTransition{activity: codexToolCompleted, callID: callID}, completedAt.Add(time.Duration(i+2)))
	}
	snapshot = state.snapshot()
	if snapshot.count != 0 || snapshot.version != 2002 {
		t.Fatalf("coalesced burst lost lifecycle state: %+v", snapshot)
	}
	if got := len(state.wakeup); got != 1 {
		t.Fatalf("coalesced wakeup depth = %d, want 1", got)
	}
}

func formatReviewTransitions(transitions []codexToolTransition) string {
	parts := make([]string, 0, len(transitions))
	for _, transition := range transitions {
		parts = append(parts, fmt.Sprintf("%d:%s", transition.activity, transition.callID))
	}
	return strings.Join(parts, ",")
}
