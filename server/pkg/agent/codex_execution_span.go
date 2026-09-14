package agent

import (
	"log/slog"
	"time"
)

const codexExecutionSpanSchema = "codex_execution_span/v1"

type codexExecutionSpan struct {
	Schema           string `json:"schema"`
	Scope            string `json:"scope"`
	StartedAt        string `json:"started_at"`
	EndedAt          string `json:"ended_at"`
	DurationMS       int64  `json:"duration_ms"`
	ClockSource      string `json:"clock_source"`
	CleanupConfirmed bool   `json:"cleanup_confirmed"`
}

func logCodexExecutionSpan(logger *slog.Logger, cfg Config, processID, attempt int, workDir string, startedAt, endedAt time.Time, cleanupConfirmed bool) {
	if attempt == 0 {
		return
	}
	logger.Info("codex execution span observed",
		"task_id", cfg.TaskID,
		"runtime_id", cfg.RuntimeID,
		"process_id", processID,
		"attempt", attempt,
		"work_dir", workDir,
		"execution_span", codexExecutionSpan{
			Schema:           codexExecutionSpanSchema,
			Scope:            "post_spawn_through_cleanup",
			StartedAt:        startedAt.Format(time.RFC3339Nano),
			EndedAt:          endedAt.Format(time.RFC3339Nano),
			DurationMS:       endedAt.Sub(startedAt).Milliseconds(),
			ClockSource:      "monotonic",
			CleanupConfirmed: cleanupConfirmed,
		},
	)
}
