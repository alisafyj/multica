package agent

import (
	"log/slog"
	"time"
)

const claudeExecutionSpanSchema = "claude_execution_span/v1"

type claudeExecutionSpan struct {
	Schema           string `json:"schema"`
	Scope            string `json:"scope"`
	StartedAt        string `json:"started_at"`
	EndedAt          string `json:"ended_at"`
	DurationMS       int64  `json:"duration_ms"`
	ClockSource      string `json:"clock_source"`
	CleanupConfirmed bool   `json:"cleanup_confirmed"`
}

func logClaudeExecutionSpan(logger *slog.Logger, cfg Config, processID int, workDir string, startedAt, endedAt time.Time, cleanupConfirmed bool) {
	logger.Info("claude execution span observed",
		"task_id", cfg.TaskID,
		"runtime_id", cfg.RuntimeID,
		"process_id", processID,
		"attempt", 1,
		"work_dir", workDir,
		"execution_span", claudeExecutionSpan{
			Schema:           claudeExecutionSpanSchema,
			Scope:            "post_spawn_through_cleanup_check",
			StartedAt:        startedAt.Format(time.RFC3339Nano),
			EndedAt:          endedAt.Format(time.RFC3339Nano),
			DurationMS:       endedAt.Sub(startedAt).Milliseconds(),
			ClockSource:      "monotonic",
			CleanupConfirmed: cleanupConfirmed,
		},
	)
}
