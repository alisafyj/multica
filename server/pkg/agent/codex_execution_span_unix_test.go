//go:build unix

package agent

import (
	"bytes"
	"context"
	"log/slog"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"testing"
	"time"
)

func TestCodexExecutionSpanSuccess(t *testing.T) {
	result, entry := runCodexExecutionSpanFixture(t, 1, codexExecutionSpanSuccessScript, nil, 1)
	if result.Status != "completed" {
		t.Fatalf("result = %+v", result)
	}
	assertCodexExecutionSpan(t, entry, 1, true)
}

func TestCodexExecutionSpanAttemptZeroIsNotEmitted(t *testing.T) {
	result, entry := runCodexExecutionSpanFixture(t, 0, codexExecutionSpanSuccessScript, nil, 0)
	if result.Status != "completed" {
		t.Fatalf("result = %+v", result)
	}
	if entry != nil {
		t.Fatalf("attempt zero emitted formal execution span: %v", entry)
	}
}

func TestCodexExecutionSpanFailure(t *testing.T) {
	result, entry := runCodexExecutionSpanFixture(t, 2, "exit 2\n", nil, 1)
	if result.Status != "failed" {
		t.Fatalf("result = %+v", result)
	}
	assertCodexExecutionSpan(t, entry, 2, true)
}

func TestCodexExecutionSpanCancellation(t *testing.T) {
	result, entry := runCodexExecutionSpanFixture(t, 3, "read line\nsleep 30\n", func(cancel context.CancelFunc) { cancel() }, 1)
	if result.Status != "failed" {
		t.Fatalf("result = %+v", result)
	}
	assertCodexExecutionSpan(t, entry, 3, true)
}

func TestCodexExecutionSpanReportsUnconfirmedCleanupOnce(t *testing.T) {
	codexCleanupConfirmationOverride.Store(-1)
	t.Cleanup(func() { codexCleanupConfirmationOverride.Store(0) })

	result, entry := runCodexExecutionSpanFixture(t, 4, "exit 2\n", nil, 1)
	if result.Status != "failed" {
		t.Fatalf("result = %+v", result)
	}
	assertCodexExecutionSpan(t, entry, 4, false)
}

const codexExecutionSpanSuccessScript = `
read line
echo '{"jsonrpc":"2.0","id":1,"result":{}}'
read line
read line
echo '{"jsonrpc":"2.0","id":2,"result":{"thread":{"id":"thr-span"}}}'
read line
echo '{"jsonrpc":"2.0","id":3,"result":{}}'
echo '{"jsonrpc":"2.0","method":"turn/completed","params":{"threadId":"thr-span","turn":{"id":"turn-span","status":"completed"}}}'
`

func runCodexExecutionSpanFixture(t *testing.T, attempt int, body string, afterStart func(context.CancelFunc), wantSpans int) (Result, map[string]any) {
	t.Helper()
	codexGracefulShutdownTimeoutNanos.Store(int64(100 * time.Millisecond))
	t.Cleanup(func() { codexGracefulShutdownTimeoutNanos.Store(0) })

	root := t.TempDir()
	pidFile := filepath.Join(root, "pid")
	fakePath := writeFakeCodexAppServer(t, "echo $$ > \""+pidFile+"\"\n"+body)
	workDir := filepath.Join(root, "work")
	if err := os.Mkdir(workDir, 0o700); err != nil {
		t.Fatal(err)
	}
	var logs bytes.Buffer
	backend := &codexBackend{cfg: Config{
		ExecutablePath: fakePath,
		Logger:         slog.New(slog.NewJSONHandler(&logs, nil)),
		TaskID:         "task-span",
		RuntimeID:      "runtime-span",
	}}
	ctx := context.Background()
	cancel := func() {}
	if afterStart != nil {
		ctx, cancel = context.WithCancel(ctx)
		defer cancel()
	}
	session, err := backend.executeOnce(ctx, "prompt", ExecOptions{Cwd: workDir, Timeout: 3 * time.Second, HandshakeTimeout: time.Second}, attempt)
	if err != nil {
		t.Fatal(err)
	}
	if afterStart != nil {
		deadline := time.Now().Add(time.Second)
		for {
			if _, err := os.Stat(pidFile); err == nil {
				break
			} else if !os.IsNotExist(err) {
				t.Fatal(err)
			}
			if time.Now().After(deadline) {
				t.Fatal("spawned fixture did not publish its pid")
			}
			time.Sleep(5 * time.Millisecond)
		}
		afterStart(cancel)
	}
	go func() {
		for range session.Messages {
		}
	}()
	result := <-session.Result

	rawPID, err := os.ReadFile(pidFile)
	if err != nil {
		t.Fatal(err)
	}
	pid, err := strconv.Atoi(strings.TrimSpace(string(rawPID)))
	if err != nil {
		t.Fatal(err)
	}
	var spans []map[string]any
	entries := parseJSONLogEntries(t, logs.String())
	for _, entry := range entries {
		if entry["msg"] == "codex execution span observed" {
			spans = append(spans, entry)
		}
	}
	if len(spans) != wantSpans {
		t.Fatalf("execution spans = %d, want %d; logs=%s", len(spans), wantSpans, logs.String())
	}
	if wantSpans == 0 {
		phaseCounts := map[string]int{}
		for _, entry := range entries {
			if entry["msg"] == "codex lifecycle" {
				if phase, ok := entry["phase"].(string); ok {
					phaseCounts[phase]++
				}
			}
		}
		if phaseCounts["spawn"] != 1 || phaseCounts["cleanup"] != 1 {
			t.Fatalf("attempt zero legacy lifecycle logs changed: %v", phaseCounts)
		}
		return result, nil
	}
	if spans[0]["process_id"] != float64(pid) {
		t.Fatalf("process_id = %v, spawned pid = %d", spans[0]["process_id"], pid)
	}
	if spans[0]["work_dir"] != workDir {
		t.Fatalf("work_dir = %v, want %q", spans[0]["work_dir"], workDir)
	}
	return result, spans[0]
}

func assertCodexExecutionSpan(t *testing.T, entry map[string]any, attempt int, cleanupConfirmed bool) {
	t.Helper()
	if len(entry) != 9 {
		t.Fatalf("root fields = %v, want time, level, msg and six fixed fields", entry)
	}
	for key, want := range map[string]any{
		"task_id": "task-span", "runtime_id": "runtime-span", "attempt": float64(attempt),
	} {
		if entry[key] != want {
			t.Fatalf("%s = %v, want %v; entry=%v", key, entry[key], want, entry)
		}
	}
	span, ok := entry["execution_span"].(map[string]any)
	if !ok {
		t.Fatalf("execution_span = %#v", entry["execution_span"])
	}
	if len(span) != 7 {
		t.Fatalf("execution_span fields = %#v", span)
	}
	if span["schema"] != "codex_execution_span/v1" || span["scope"] != "post_spawn_through_cleanup" ||
		span["clock_source"] != "monotonic" || span["cleanup_confirmed"] != cleanupConfirmed {
		t.Fatalf("execution_span contract = %#v", span)
	}
	startedAt, startErr := time.Parse(time.RFC3339Nano, span["started_at"].(string))
	endedAt, endErr := time.Parse(time.RFC3339Nano, span["ended_at"].(string))
	if startErr != nil || endErr != nil || endedAt.Before(startedAt) {
		t.Fatalf("invalid span boundaries: start=%v (%v), end=%v (%v)", startedAt, startErr, endedAt, endErr)
	}
	if duration, ok := span["duration_ms"].(float64); !ok || duration < 0 {
		t.Fatalf("duration_ms = %#v", span["duration_ms"])
	}
}
