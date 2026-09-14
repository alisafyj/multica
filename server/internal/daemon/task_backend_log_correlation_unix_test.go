//go:build !windows

package daemon

import (
	"context"
	"encoding/json"
	"io"
	"log/slog"
	"strings"
	"testing"
)

func TestRunTaskPreservesTaskLoggerAtProviderBoundary(t *testing.T) {
	writeRuntimeMCPSelectionFixture(t, "claude")
	d, _, cleanup := newLeaderReuseTestDaemon(t)
	defer cleanup()
	logs := &lockedBuffer{}
	d.logger = slog.New(slog.NewJSONHandler(logs, nil)).With("component", "daemon")

	for _, taskID := range []string{collidingTaskIDA, collidingTaskIDB} {
		task := leaderReuseTestTask(taskID)
		task.AuthToken = "mat_synthetic_provider_log_secret"
		result, err := d.runTask(context.Background(), task, "claude", 0, d.logger.With("task", taskID))
		if err != nil || result.Status != "completed" {
			t.Fatalf("fixture provider did not complete: status=%s error=%v", result.Status, err)
		}
	}

	output := logs.String()
	if strings.Contains(output, "mat_synthetic_provider_log_secret") {
		t.Fatal("task credential appeared in provider logs")
	}
	decoder := json.NewDecoder(strings.NewReader(output))
	starts := make(map[string]int)
	finishes := make(map[string]int)
	for {
		var record map[string]any
		if err := decoder.Decode(&record); err == io.EOF {
			break
		} else if err != nil {
			t.Fatalf("decode structured provider log: %v", err)
		}
		if record["msg"] != "claude started" && record["msg"] != "claude finished" {
			continue
		}
		taskID, _ := record["task"].(string)
		if taskID != collidingTaskIDA && taskID != collidingTaskIDB {
			t.Errorf("provider lifecycle log is not bound to the full task ID: message=%v task=%q", record["msg"], taskID)
			continue
		}
		if pid, ok := record["pid"].(float64); !ok || pid <= 0 {
			t.Error("provider lifecycle log has no process ID")
		}
		if record["component"] != "daemon" {
			t.Error("provider lifecycle log lost the daemon component")
		}
		if record["msg"] == "claude started" {
			starts[taskID]++
		} else {
			finishes[taskID]++
		}
	}
	for _, taskID := range []string{collidingTaskIDA, collidingTaskIDB} {
		if starts[taskID] != 1 || finishes[taskID] != 1 {
			t.Errorf("task lifecycle evidence must contain one start and finish: starts=%d finishes=%d", starts[taskID], finishes[taskID])
		}
	}
}
