-- name: UpdateTaskExecutionMetrics :execrows
-- The handler validates monotonic snapshots. Compare-and-swap the exact JSONB
-- it read so concurrent reports cannot replace a newer snapshot. No timestamp
-- casts: NULL, absent fields, and malformed historical timestamps are safe.
UPDATE agent_task_queue
SET execution_metrics = sqlc.arg(execution_metrics)::jsonb
WHERE id = sqlc.arg(task_id)
  AND execution_metrics IS NOT DISTINCT FROM sqlc.narg(previous_metrics)::jsonb;

