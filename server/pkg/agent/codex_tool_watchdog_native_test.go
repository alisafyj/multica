package agent

import (
	"encoding/json"
	"fmt"
	"log/slog"
	"runtime"
	"strings"
	"testing"
	"time"
)

const codexNativeWatchdogHandshake = `read line
echo '{"jsonrpc":"2.0","id":1,"result":{}}'
read line
read line
echo '{"jsonrpc":"2.0","id":2,"result":{"thread":{"id":"thr-native"}}}'
read line
echo '{"jsonrpc":"2.0","id":3,"result":{}}'
echo '{"jsonrpc":"2.0","method":"turn/started","params":{"threadId":"thr-native","turn":{"id":"turn-native"}}}'
`

func TestCodexToolWatchdogNativeItems(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("fake app-server fixture uses a POSIX shell")
	}
	codexInFlightToolTimeoutNanos.Store(int64(time.Second))
	t.Cleanup(func() { codexInFlightToolTimeoutNanos.Store(0) })

	// Required item fields follow Codex 0.153.4's generated
	// v2/ItemStartedNotification.json and ItemCompletedNotification.json.
	for _, tc := range []struct {
		name string
		item string
		work bool
	}{
		{"commandExecution", `{"type":"commandExecution","id":"work-1","command":"true","commandActions":[],"cwd":"/tmp","status":"inProgress"}`, true},
		{"fileChange", `{"type":"fileChange","id":"work-1","changes":[],"status":"inProgress"}`, true},
		{"mcpToolCall", `{"type":"mcpToolCall","id":"work-1","arguments":{},"server":"fixture","tool":"lookup","status":"inProgress"}`, true},
		{"collabAgentToolCall", `{"type":"collabAgentToolCall","id":"work-1","agentsStates":{},"receiverThreadIds":["child"],"senderThreadId":"thr-native","tool":"wait","status":"inProgress"}`, true},
		{"dynamicToolCall", `{"type":"dynamicToolCall","id":"work-1","arguments":{},"tool":"fixture","status":"inProgress"}`, true},
		{"webSearch", `{"type":"webSearch","id":"work-1","query":"fixture"}`, true},
		{"imageView", `{"type":"imageView","id":"work-1","path":"/tmp/fixture.png"}`, true},
		{"sleep", `{"type":"sleep","id":"work-1","durationMs":250}`, true},
		{"imageGeneration", `{"type":"imageGeneration","id":"work-1","result":"","status":"inProgress"}`, true},
		{"unknown", `{"type":"futureItem","id":"work-1"}`, false},
		{"displayOnly", `{"type":"subAgentActivity","id":"work-1","agentPath":"child","agentThreadId":"child","kind":"started"}`, false},
		{"missingID", `{"type":"collabAgentToolCall","tool":"wait"}`, false},
	} {
		t.Run(tc.name, func(t *testing.T) {
			completed := strings.ReplaceAll(tc.item, `"inProgress"`, `"completed"`)
			fakePath := writeFakeCodexAppServer(t, codexNativeWatchdogHandshake+fmt.Sprintf(`echo '{"jsonrpc":"2.0","method":"item/started","params":{"threadId":"thr-native","turnId":"turn-native","item":%s}}'
sleep 0.25
echo '{"jsonrpc":"2.0","method":"item/completed","params":{"threadId":"thr-native","turnId":"turn-native","item":%s}}'
echo '{"jsonrpc":"2.0","method":"item/completed","params":{"threadId":"thr-native","turnId":"turn-native","item":{"type":"agentMessage","id":"answer","phase":"final_answer","text":"Done"}}}'
echo '{"jsonrpc":"2.0","method":"turn/completed","params":{"threadId":"thr-native","turn":{"id":"turn-native","status":"completed"}}}'
`, tc.item, completed))
			result := executeFakeCodex(t, fakePath, ExecOptions{
				Timeout:                   3 * time.Second,
				SemanticInactivityTimeout: 100 * time.Millisecond,
			})
			if tc.work {
				if result.Status != "completed" || result.Output != "Done" {
					t.Fatalf("native work must survive semantic silence and complete: status=%q output=%q error=%q", result.Status, result.Output, result.Error)
				}
			} else if result.Status != "timeout" || !strings.Contains(result.Error, CodexSemanticInactivityMarker) || strings.Contains(result.Error, "in-flight tool exceeded") {
				t.Fatalf("non-work item must retain semantic timeout: status=%q error=%q", result.Status, result.Error)
			}
		})
	}
}

func TestCodexToolWatchdogNativeItemCompletionRestoresSemanticTimeout(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("fake app-server fixture uses a POSIX shell")
	}
	fakePath := writeFakeCodexAppServer(t, codexNativeWatchdogHandshake+`echo '{"jsonrpc":"2.0","method":"item/started","params":{"threadId":"thr-native","turnId":"turn-native","item":{"type":"collabAgentToolCall","id":"wait-1","tool":"wait"}}}'
sleep 0.25
echo '{"jsonrpc":"2.0","method":"item/completed","params":{"threadId":"thr-native","turnId":"turn-native","item":{"type":"collabAgentToolCall","id":"wait-1","status":"completed","tool":"wait"}}}'
echo '{"jsonrpc":"2.0","method":"item/completed","params":{"threadId":"thr-native","turnId":"turn-native","item":{"type":"agentMessage","id":"after-work","text":"wait completed"}}}'
sleep 2
`)
	result := executeFakeCodex(t, fakePath, ExecOptions{Timeout: 3 * time.Second, SemanticInactivityTimeout: 100 * time.Millisecond, InFlightToolTimeout: time.Second})
	if result.Status != "timeout" || result.Output != "wait completed" || !strings.Contains(result.Error, CodexSemanticInactivityMarker) || !strings.Contains(result.Error, "last activity: text") || strings.Contains(result.Error, "in-flight tool exceeded") {
		t.Fatalf("completed native item must release tool budget: status=%q output=%q error=%q", result.Status, result.Output, result.Error)
	}
}

func TestCodexToolWatchdogNativeItemDuplicateStartsStayBounded(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("fake app-server fixture uses a POSIX shell")
	}
	codexInFlightToolTimeoutNanos.Store(int64(250 * time.Millisecond))
	t.Cleanup(func() { codexInFlightToolTimeoutNanos.Store(0) })
	fakePath := writeFakeCodexAppServer(t, codexNativeWatchdogHandshake+`i=0
while [ $i -lt 20 ]; do
  echo '{"jsonrpc":"2.0","method":"item/started","params":{"threadId":"thr-native","turnId":"turn-native","item":{"type":"collabAgentToolCall","id":"wait-stuck","tool":"wait"}}}'
  sleep 0.04
  i=$((i+1))
done
`)
	result := executeFakeCodex(t, fakePath, ExecOptions{Timeout: 2 * time.Second, SemanticInactivityTimeout: 100 * time.Millisecond})
	if result.Status != "timeout" || !strings.Contains(result.Error, "in-flight tool exceeded 250ms") || !strings.Contains(result.Error, "tool_in_flight:wait-stuck") {
		t.Fatalf("duplicate native starts must retain original tool deadline: status=%q error=%q", result.Status, result.Error)
	}
}

func TestCodexToolWatchdogNativeLifecycleIdentityAndGates(t *testing.T) {
	t.Parallel()
	state := newCodexInFlightToolState()
	gate := &codexTurnNotificationGate{}
	observedAt := time.Unix(100, 0)
	semantic := make(chan string, 1)
	semantic <- "already full"
	client := &codexClient{
		cfg:                  Config{Logger: slog.Default()},
		threadID:             "thr-native",
		notificationProtocol: "unknown",
		acceptNotification:   gate.accept,
		onToolTransition: func(transition codexToolTransition) {
			state.observe(transition, observedAt)
		},
		onMessage: func(msg Message) {
			if transition, ok := codexToolTransitionFromMessage(msg); ok {
				state.observe(transition, observedAt)
			}
		},
		onSemanticActivity: func(activity string) { trySendString(semantic, activity) },
	}
	emit := func(method, itemType, itemID, threadID, turnID string) {
		t.Helper()
		line, err := json.Marshal(map[string]any{
			"jsonrpc": "2.0", "method": method,
			"params": map[string]any{
				"threadId": threadID, "turnId": turnID,
				"item": map[string]any{"type": itemType, "id": itemID},
			},
		})
		if err != nil {
			t.Fatal(err)
		}
		client.handleLine(string(line))
	}
	emit("item/started", "collabAgentToolCall", "resume-replay", "thr-native", "old-turn")
	gate.arm()
	client.handleLine(`{"jsonrpc":"2.0","method":"turn/started","params":{"threadId":"thr-native","turn":{"id":"turn-native"}}}`)
	emit("item/started", "collabAgentToolCall", "old-turn", "thr-native", "old-turn")
	emit("item/started", "collabAgentToolCall", "child", "child", "turn-native")
	if got := state.snapshot(); got.count != 0 || got.version != 0 {
		t.Fatalf("filtered lifecycle mutated current work: %+v", got)
	}

	for _, kind := range []string{"commandExecution", "fileChange", "mcpToolCall", "collabAgentToolCall", "dynamicToolCall", "webSearch", "imageView", "sleep", "imageGeneration"} {
		before := state.snapshot()
		startedAt := observedAt
		emit("item/started", kind, kind, "thr-native", "turn-native")
		observedAt = observedAt.Add(time.Second)
		emit("item/started", kind, kind, "thr-native", "turn-native")
		emit("item/completed", kind, kind+" ", "thr-native", "turn-native")
		emit("item/completed", kind, kind, "child", "turn-native")
		emit("item/completed", kind, kind, "thr-native", "old-turn")
		emit("item/"+kind+"/progress", kind, kind, "thr-native", "turn-native")
		if got := state.snapshot(); got.count != 1 || got.version != before.version+1 || got.oldestCallID != kind || !got.oldestStartedAt.Equal(startedAt) {
			t.Fatalf("%s: duplicate, progress or foreign completion changed work identity: %+v", kind, got)
		}
		emit("item/completed", kind, kind, "thr-native", "turn-native")
		completedAt := observedAt
		observedAt = observedAt.Add(time.Second)
		for range 300 {
			emit("item/completed", kind, kind, "thr-native", "turn-native")
		}
		if got := state.snapshot(); got.count != 0 || got.version != before.version+2 || !got.becameIdleAt.Equal(completedAt) {
			t.Fatalf("%s: completion storm changed idle transition: %+v", kind, got)
		}
	}
	if len(state.wakeup) != 1 || len(semantic) != 1 {
		t.Fatal("saturated lifecycle/semantic wakeups must remain coalesced and nonblocking")
	}
}
