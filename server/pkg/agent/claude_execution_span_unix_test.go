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
	"syscall"
	"testing"
	"time"
)

const (
	claudeExecutionSpanPromptSecret = "R15_PROMPT_SECRET_MUST_NOT_APPEAR"
	claudeExecutionSpanTaskID       = "22222222-2222-4222-8222-222222222222"
	claudeExecutionSpanRuntimeID    = "33333333-3333-4333-8333-333333333333"
)

func TestClaudeExecutionSpanSuccess(t *testing.T) {
	result, observation := runClaudeExecutionSpanFixture(t, `
IFS= read -r _
printf '{"type":"system","session_id":"claude-span-success"}\n'
printf '{"type":"result","subtype":"success","is_error":false,"session_id":"claude-span-success","result":"done"}\n'
`, nil)
	if result.Status != "completed" {
		t.Fatalf("result = %+v", result)
	}
	assertClaudeExecutionSpan(t, observation, true)
	if len(observation.logs) > 16*1024 {
		t.Fatalf("fixture log receipt is not bounded: %d bytes", len(observation.logs))
	}
	t.Logf("claude execution span fixture receipt:\n%s", observation.logs)
}

func TestClaudeExecutionSpanFailure(t *testing.T) {
	result, observation := runClaudeExecutionSpanFixture(t, `
IFS= read -r _
exit 2
`, nil)
	if result.Status != "failed" {
		t.Fatalf("result = %+v", result)
	}
	assertClaudeExecutionSpan(t, observation, true)
}

func TestClaudeExecutionSpanCancellation(t *testing.T) {
	result, observation := runClaudeExecutionSpanFixture(t, `
trap 'exit 0' TERM
IFS= read -r _
printf '{"type":"system","session_id":"claude-span-cancel"}\n'
while :; do sleep 1; done
`, func(cancel context.CancelFunc, _ string) {
		cancel()
	})
	if result.Status != "aborted" {
		t.Fatalf("result = %+v", result)
	}
	assertClaudeExecutionSpan(t, observation, true)
}

func TestClaudeExecutionSpanReportsLiveDescendantAsUnconfirmed(t *testing.T) {
	result, observation := runClaudeExecutionSpanFixture(t, `
IFS= read -r _
( trap '' TERM; sleep 300 ) </dev/null >/dev/null 2>&1 &
printf '%s\n' "$!" > "$CLAUDE_DESCENDANT_PID_FILE"
printf '{"type":"system","session_id":"claude-span-descendant"}\n'
printf '{"type":"result","subtype":"success","is_error":false,"session_id":"claude-span-descendant","result":"done"}\n'
`, func(_ context.CancelFunc, descendantPIDFile string) {
		pid := waitForSinglePID(t, descendantPIDFile)
		t.Cleanup(func() {
			_ = syscall.Kill(pid, syscall.SIGKILL)
			waitProcessGone(t, pid)
		})
	})
	if result.Status != "completed" {
		t.Fatalf("result = %+v", result)
	}
	assertClaudeExecutionSpan(t, observation, false)
}

func TestClaudeExecutionSpanSpawnFailureEmitsNothing(t *testing.T) {
	var logs bytes.Buffer
	backend := &claudeBackend{cfg: Config{
		ExecutablePath: filepath.Join(t.TempDir(), "missing-claude"),
		Logger:         slog.New(slog.NewJSONHandler(&logs, nil)).With("component", "daemon"),
		TaskID:         claudeExecutionSpanTaskID,
		RuntimeID:      claudeExecutionSpanRuntimeID,
	}}
	if _, err := backend.Execute(context.Background(), claudeExecutionSpanPromptSecret, ExecOptions{Cwd: t.TempDir()}); err == nil {
		t.Fatal("Execute succeeded with a missing executable")
	}
	for _, entry := range parseJSONLogEntries(t, logs.String()) {
		if entry["component"] != "daemon" {
			t.Fatalf("spawn failure log component = %v, want daemon", entry["component"])
		}
		if entry["msg"] == "claude execution span observed" {
			t.Fatalf("spawn failure emitted execution span: %v", entry)
		}
	}
	if strings.Contains(logs.String(), claudeExecutionSpanPromptSecret) {
		t.Fatalf("spawn failure logs leaked prompt secret: %s", logs.String())
	}
}

type claudeExecutionSpanObservation struct {
	logs      string
	launch    map[string]any
	span      map[string]any
	processID int
	workDir   string
}

func runClaudeExecutionSpanFixture(t *testing.T, body string, afterStart func(context.CancelFunc, string)) (Result, claudeExecutionSpanObservation) {
	t.Helper()
	root := t.TempDir()
	pidFile := filepath.Join(root, "leader.pid")
	descendantPIDFile := filepath.Join(root, "descendant.pid")
	fakePath := filepath.Join(root, "claude")
	writeTestExecutable(t, fakePath, []byte("#!/bin/sh\nprintf '%s\\n' \"$$\" > \"$CLAUDE_LEADER_PID_FILE\"\n"+body))
	workDir := filepath.Join(root, "work")
	if err := os.Mkdir(workDir, 0o700); err != nil {
		t.Fatal(err)
	}

	var logs bytes.Buffer
	backend := &claudeBackend{cfg: Config{
		ExecutablePath: fakePath,
		Logger:         slog.New(slog.NewJSONHandler(&logs, nil)).With("component", "daemon"),
		TaskID:         claudeExecutionSpanTaskID,
		RuntimeID:      claudeExecutionSpanRuntimeID,
		Env: map[string]string{
			"CLAUDE_LEADER_PID_FILE":     pidFile,
			"CLAUDE_DESCENDANT_PID_FILE": descendantPIDFile,
		},
	}}
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	session, err := backend.Execute(ctx, claudeExecutionSpanPromptSecret, ExecOptions{Cwd: workDir, Timeout: 5 * time.Second})
	if err != nil {
		t.Fatal(err)
	}
	processID := waitForSinglePID(t, pidFile)
	if afterStart != nil {
		afterStart(cancel, descendantPIDFile)
	}
	go func() {
		for range session.Messages {
		}
	}()
	result := <-session.Result

	entries := parseJSONLogEntries(t, logs.String())
	var launches []map[string]any
	var spans []map[string]any
	for _, entry := range entries {
		switch entry["msg"] {
		case "claude started":
			launches = append(launches, entry)
		case "claude execution span observed":
			spans = append(spans, entry)
		}
	}
	if len(launches) != 1 {
		t.Fatalf("claude started logs = %d, want 1; logs=%s", len(launches), logs.String())
	}
	if len(spans) != 1 {
		t.Fatalf("execution spans = %d, want 1; logs=%s", len(spans), logs.String())
	}
	if strings.Contains(logs.String(), claudeExecutionSpanPromptSecret) {
		t.Fatalf("logs leaked prompt secret: %s", logs.String())
	}
	return result, claudeExecutionSpanObservation{
		logs: logs.String(), launch: launches[0], span: spans[0], processID: processID, workDir: workDir,
	}
}

func assertClaudeExecutionSpan(t *testing.T, observation claudeExecutionSpanObservation, cleanupConfirmed bool) {
	t.Helper()
	if len(observation.launch) != 10 {
		t.Fatalf("launch root fields = %v, want time, level, msg, component and six launch fields", observation.launch)
	}
	if len(observation.span) != 10 {
		t.Fatalf("span root fields = %v, want time, level, msg, component and six fixed fields", observation.span)
	}
	for key, want := range map[string]any{
		"component": "daemon", "task_id": claudeExecutionSpanTaskID,
		"runtime_id": claudeExecutionSpanRuntimeID, "attempt": float64(1),
	} {
		if observation.launch[key] != want {
			t.Fatalf("launch %s = %v, want %v; launch=%v", key, observation.launch[key], want, observation.launch)
		}
		if observation.span[key] != want {
			t.Fatalf("span %s = %v, want %v; span=%v", key, observation.span[key], want, observation.span)
		}
	}
	if observation.launch["pid"] != float64(observation.processID) {
		t.Fatalf("launch pid = %v, spawned pid = %d", observation.launch["pid"], observation.processID)
	}
	if observation.launch["cwd"] != observation.workDir {
		t.Fatalf("launch cwd = %v, want %q", observation.launch["cwd"], observation.workDir)
	}
	if observation.span["process_id"] != float64(observation.processID) {
		t.Fatalf("span process_id = %v, spawned pid = %d", observation.span["process_id"], observation.processID)
	}
	if observation.span["work_dir"] != observation.workDir {
		t.Fatalf("work_dir = %v, want %q", observation.span["work_dir"], observation.workDir)
	}

	span, ok := observation.span["execution_span"].(map[string]any)
	if !ok || len(span) != 7 {
		t.Fatalf("execution_span = %#v", observation.span["execution_span"])
	}
	if span["schema"] != "claude_execution_span/v1" || span["scope"] != "post_spawn_through_cleanup_check" ||
		span["clock_source"] != "monotonic" || span["cleanup_confirmed"] != cleanupConfirmed {
		t.Fatalf("execution_span contract = %#v", span)
	}
	startedAt, startErr := time.Parse(time.RFC3339Nano, span["started_at"].(string))
	endedAt, endErr := time.Parse(time.RFC3339Nano, span["ended_at"].(string))
	if startErr != nil || endErr != nil || endedAt.Before(startedAt) {
		t.Fatalf("invalid span boundaries: start=%v (%v), end=%v (%v)", startedAt, startErr, endedAt, endErr)
	}
	duration, ok := span["duration_ms"].(float64)
	if !ok || duration < 0 || int64(duration) != endedAt.Sub(startedAt).Milliseconds() {
		t.Fatalf("duration_ms = %#v, wall boundaries = %s", span["duration_ms"], endedAt.Sub(startedAt))
	}
}

func waitForSinglePID(t *testing.T, path string) int {
	t.Helper()
	deadline := time.Now().Add(5 * time.Second)
	for {
		raw, err := os.ReadFile(path)
		if err == nil {
			pid, parseErr := strconv.Atoi(strings.TrimSpace(string(raw)))
			if parseErr != nil {
				t.Fatalf("parse pid %q: %v", raw, parseErr)
			}
			return pid
		}
		if !os.IsNotExist(err) {
			t.Fatal(err)
		}
		if time.Now().After(deadline) {
			t.Fatalf("timed out waiting for pid file %s", path)
		}
		time.Sleep(5 * time.Millisecond)
	}
}
