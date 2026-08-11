-- name: CreateTestGenerationJob :one
INSERT INTO test_generation_job (
    workspace_id, project_id, agent_id, status, input, created_by
) VALUES ($1, $2, $3, $4, $5, $6)
RETURNING *;

-- name: UpdateTestGenerationJob :one
-- Every optional field uses COALESCE so a partial update cannot blank a column
-- the caller did not mention. design_restore's equivalent omits COALESCE on
-- error and nulls it on every status write; do not repeat that here.
UPDATE test_generation_job SET
    status        = COALESCE(sqlc.narg('status'), status),
    agent_id      = COALESCE(sqlc.narg('agent_id'), agent_id),
    agent_task_id = COALESCE(sqlc.narg('agent_task_id'), agent_task_id),
    input         = COALESCE(sqlc.narg('input'), input),
    result        = COALESCE(sqlc.narg('result'), result),
    error         = COALESCE(sqlc.narg('error'), error),
    updated_at    = now()
WHERE id = $1 AND workspace_id = $2
RETURNING *;

-- name: ClearTestGenerationJobError :exec
-- Explicit error reset, so UpdateTestGenerationJob can keep COALESCE semantics.
UPDATE test_generation_job SET error = NULL, updated_at = now()
WHERE id = $1 AND workspace_id = $2;

-- name: GetTestGenerationJobInWorkspace :one
SELECT * FROM test_generation_job
WHERE id = $1 AND workspace_id = $2;

-- name: GetTestGenerationJobByAgentTask :one
-- workspace_id is required here even though agent_task_id is a trusted UUID
-- round-trip: every query in this repository filters by workspace.
SELECT * FROM test_generation_job
WHERE agent_task_id = $1 AND workspace_id = $2;

-- name: GetReusableTestGenerationJob :one
-- Idempotent create: an in-flight job for the same project is returned instead
-- of minting a second one.
SELECT * FROM test_generation_job
WHERE workspace_id = $1 AND project_id = $2 AND status IN ('queued', 'running')
ORDER BY created_at DESC
LIMIT 1;

-- name: ListTestGenerationJobs :many
SELECT * FROM test_generation_job
WHERE workspace_id = $1
  AND (sqlc.narg('project_id')::uuid IS NULL OR project_id = sqlc.narg('project_id'))
  AND (sqlc.narg('status')::text IS NULL OR status = sqlc.narg('status'))
ORDER BY created_at DESC
LIMIT $2;

-- name: CreateTestGenerationPlan :one
INSERT INTO test_generation_plan (
    workspace_id, job_id, status, plan, review_notes, created_by
) VALUES ($1, $2, $3, $4, $5, $6)
RETURNING *;

-- name: GetTestGenerationPlanByJob :one
SELECT * FROM test_generation_plan
WHERE job_id = $1 AND workspace_id = $2
  AND status IN ('draft', 'approved', 'dispatched')
ORDER BY updated_at DESC
LIMIT 1;

-- name: UpdateTestGenerationPlan :one
UPDATE test_generation_plan SET
    plan         = COALESCE(sqlc.narg('plan'), plan),
    review_notes = COALESCE(sqlc.narg('review_notes'), review_notes),
    status       = COALESCE(sqlc.narg('status'), status),
    approved_by  = COALESCE(sqlc.narg('approved_by'), approved_by),
    approved_at  = COALESCE(sqlc.narg('approved_at'), approved_at),
    updated_at   = now()
WHERE id = $1 AND workspace_id = $2
RETURNING *;

-- name: MarkTestGenerationPlanDispatched :one
UPDATE test_generation_plan SET status = 'dispatched', updated_at = now()
WHERE id = $1 AND workspace_id = $2 AND status = 'approved'
RETURNING *;

-- name: ArchiveTestGenerationPlansForJob :exec
UPDATE test_generation_plan SET status = 'archived', updated_at = now()
WHERE job_id = $1 AND workspace_id = $2 AND status IN ('draft', 'approved', 'dispatched');

-- name: CreateTestCaseProposal :one
INSERT INTO test_case_proposal (
    workspace_id, job_id, target_case_id, kind, payload, rationale
) VALUES ($1, $2, $3, $4, $5, $6)
RETURNING *;

-- name: GetTestCaseProposalInWorkspace :one
SELECT * FROM test_case_proposal
WHERE id = $1 AND workspace_id = $2;

-- name: ListTestCaseProposalsForCase :many
SELECT * FROM test_case_proposal
WHERE workspace_id = $1 AND target_case_id = $2
  AND (sqlc.narg('status')::text IS NULL OR status = sqlc.narg('status'))
ORDER BY created_at DESC;

-- name: ListTestCaseProposalsForJob :many
SELECT * FROM test_case_proposal
WHERE job_id = $1 AND workspace_id = $2
ORDER BY created_at DESC;

-- name: CountPendingTestCaseProposals :one
-- Powers the "needs review" badge. Columns are alias-qualified because the
-- correlated subquery brings a second workspace_id into scope.
SELECT count(*) FROM test_case_proposal p
WHERE p.workspace_id = $1 AND p.status = 'pending'
  AND (sqlc.narg('project_scope')::uuid IS NULL OR p.target_case_id IN (
      SELECT c.id FROM test_case c
      WHERE c.workspace_id = p.workspace_id
        AND c.project_id = sqlc.narg('project_scope')
  ));

-- name: UpdateTestCaseProposalStatus :one
UPDATE test_case_proposal SET
    status      = $3,
    reviewed_by = $4,
    reviewed_at = now()
WHERE id = $1 AND workspace_id = $2 AND status = 'pending'
RETURNING *;

-- name: DeleteTestCaseProposalsForCase :exec
-- Called from the DeleteTestCase transaction. There is no foreign key, so a
-- deleted case would otherwise leave dangling proposals behind.
DELETE FROM test_case_proposal WHERE target_case_id = $1 AND workspace_id = $2;
