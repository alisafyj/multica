-- name: GetTaskRunEvidence :one
SELECT *
FROM task_run_evidence
WHERE task_id = $1 AND attempt = $2 AND claim_generation = $3;

-- name: ListTaskRunEvidence :many
SELECT *
FROM task_run_evidence
WHERE task_id = $1
ORDER BY attempt ASC, claim_generation ASC NULLS FIRST;

-- name: LockTaskRunEvidenceWorkspace :one
-- Evidence writes take the workspace lock first, matching workspace teardown.
-- FOR KEY SHARE permits concurrent evidence writers but conflicts with delete.
SELECT id
FROM workspace
WHERE id = $1
FOR KEY SHARE;

-- name: LockTaskRunEvidenceRuntime :one
-- Stabilizes mutable runtime workspace/daemon ownership through the evidence commit.
SELECT *
FROM agent_runtime
WHERE id = sqlc.arg(runtime_id)
  AND workspace_id = sqlc.arg(workspace_id)
  AND daemon_id = sqlc.arg(daemon_id)
FOR UPDATE;

-- name: LockTaskRunEvidenceTask :one
-- Serializes the attempt check and evidence upsert with task reclaim/delete.
SELECT *
FROM agent_task_queue
WHERE id = sqlc.arg(task_id)
  AND runtime_id = sqlc.arg(runtime_id)
FOR UPDATE;

-- name: UpsertTaskRunEvidence :one
INSERT INTO task_run_evidence (
    task_id, workspace_id, runtime_id, daemon_id, attempt, claim_generation, revision, payload_sha256,
    queue_known, queue_duration_ms, preparation_known, preparation_duration_ms,
    first_tool_known, first_tool_duration_ms, execution_known, execution_duration_ms,
    finalization_known, finalization_duration_ms,
    requested_model, requested_effort, client_effective_model, client_effective_effort,
    provider_reported_model, provider_model_source, runtime_version, runtime_content_sha256,
    input_uncached_tokens, input_cache_read_tokens, input_cache_write_tokens, output_tokens,
    usage_complete, usage_source,
    provider_cost_usd_ticks, provider_cost_complete, provider_cost_authority,
    provider_cost_basis, provider_cost_source, model_usage
) VALUES (
    $1, $2, $3, $4, $5, $6, $7, $8,
    $9, $10, $11, $12, $13, $14, $15, $16, $17, $18,
    $19, $20, $21, $22, $23, $24, $25, $26,
    $27, $28, $29, $30, $31, $32, $33, $34, $35, $36, $37, $38
)
ON CONFLICT (task_id, attempt, claim_generation) DO UPDATE SET
    workspace_id = EXCLUDED.workspace_id,
    runtime_id = EXCLUDED.runtime_id,
    daemon_id = EXCLUDED.daemon_id,
    revision = EXCLUDED.revision,
    payload_sha256 = EXCLUDED.payload_sha256,
    queue_known = EXCLUDED.queue_known,
    queue_duration_ms = EXCLUDED.queue_duration_ms,
    preparation_known = EXCLUDED.preparation_known,
    preparation_duration_ms = EXCLUDED.preparation_duration_ms,
    first_tool_known = EXCLUDED.first_tool_known,
    first_tool_duration_ms = EXCLUDED.first_tool_duration_ms,
    execution_known = EXCLUDED.execution_known,
    execution_duration_ms = EXCLUDED.execution_duration_ms,
    finalization_known = EXCLUDED.finalization_known,
    finalization_duration_ms = EXCLUDED.finalization_duration_ms,
    requested_model = EXCLUDED.requested_model,
    requested_effort = EXCLUDED.requested_effort,
    client_effective_model = EXCLUDED.client_effective_model,
    client_effective_effort = EXCLUDED.client_effective_effort,
    provider_reported_model = EXCLUDED.provider_reported_model,
    provider_model_source = EXCLUDED.provider_model_source,
    runtime_version = EXCLUDED.runtime_version,
    runtime_content_sha256 = EXCLUDED.runtime_content_sha256,
    input_uncached_tokens = EXCLUDED.input_uncached_tokens,
    input_cache_read_tokens = EXCLUDED.input_cache_read_tokens,
    input_cache_write_tokens = EXCLUDED.input_cache_write_tokens,
    output_tokens = EXCLUDED.output_tokens,
    usage_complete = EXCLUDED.usage_complete,
    usage_source = EXCLUDED.usage_source,
    provider_cost_usd_ticks = EXCLUDED.provider_cost_usd_ticks,
    provider_cost_complete = EXCLUDED.provider_cost_complete,
    provider_cost_authority = EXCLUDED.provider_cost_authority,
    provider_cost_basis = EXCLUDED.provider_cost_basis,
    provider_cost_source = EXCLUDED.provider_cost_source,
    model_usage = EXCLUDED.model_usage,
    updated_at = now()
WHERE task_run_evidence.workspace_id = EXCLUDED.workspace_id
  AND task_run_evidence.runtime_id = EXCLUDED.runtime_id
  AND task_run_evidence.daemon_id = EXCLUDED.daemon_id
  AND task_run_evidence.revision < EXCLUDED.revision
RETURNING *;

-- name: DeleteTaskRunEvidenceByTask :exec
DELETE FROM task_run_evidence WHERE task_id = $1;
