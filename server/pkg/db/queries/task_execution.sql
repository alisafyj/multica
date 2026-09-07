-- name: UpdateTaskExecutionMetrics :execrows
-- The handler validates monotonic snapshots. Compare-and-swap the exact JSONB
-- it read so concurrent reports cannot replace a newer snapshot. No timestamp
-- casts: NULL, absent fields, and malformed historical timestamps are safe.
UPDATE agent_task_queue
SET execution_metrics = sqlc.arg(execution_metrics)::jsonb
WHERE id = sqlc.arg(task_id)
  AND execution_metrics IS NOT DISTINCT FROM sqlc.narg(previous_metrics)::jsonb;

-- name: ListAgentTaskUsage :many
-- The caller has resolved and authorized this agent in its workspace.
SELECT tu.*
FROM task_usage tu
JOIN agent_task_queue t ON t.id = tu.task_id
WHERE t.agent_id = sqlc.arg(agent_id)
ORDER BY tu.task_id, tu.provider, tu.model;
