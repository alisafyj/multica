-- name: ListTestCases :many
SELECT * FROM test_case
WHERE workspace_id = $1
  AND (sqlc.narg('project_id')::uuid IS NULL OR project_id = sqlc.narg('project_id'))
  AND (sqlc.narg('status')::text IS NULL OR status = sqlc.narg('status'))
  AND (sqlc.narg('module')::text IS NULL OR module = sqlc.narg('module'))
  AND (sqlc.narg('priority')::text IS NULL OR priority = sqlc.narg('priority'))
  AND (sqlc.narg('case_type')::text IS NULL OR case_type = sqlc.narg('case_type'))
  AND (sqlc.narg('origin')::text IS NULL OR origin = sqlc.narg('origin'))
ORDER BY case_number DESC;

-- name: GetTestCaseInWorkspace :one
SELECT * FROM test_case
WHERE id = $1 AND workspace_id = $2;

-- name: GetTestCaseByNumber :one
SELECT * FROM test_case
WHERE workspace_id = $1 AND case_number = $2;

-- name: ListTestCaseModules :many
SELECT module, count(*)::bigint AS case_count
FROM test_case
WHERE workspace_id = $1 AND project_id = $2
GROUP BY module
ORDER BY module ASC;

-- name: CreateTestCase :one
INSERT INTO test_case (
    workspace_id, project_id, case_number, title, module, preconditions,
    steps, expected_result, test_data, priority, case_type, scope,
    execution_mode, required_capabilities, business_rules_ref, status,
    origin, source_refs, generation_job_id, created_by, updated_by
) VALUES (
    $1, $2, $3, $4, $5, $6, $7, $8, $9, $10, $11, $12,
    $13, $14, $15, $16, $17, $18, $19, $20, $21
) RETURNING *;

-- name: UpdateTestCase :one
-- Every mutable field uses COALESCE(narg, col) so a partial update cannot
-- silently blank a column the caller did not mention. version is bumped by the
-- statement itself, so the caller never has to read-modify-write it.
UPDATE test_case SET
    title                 = COALESCE(sqlc.narg('title'), title),
    module                = COALESCE(sqlc.narg('module'), module),
    preconditions         = COALESCE(sqlc.narg('preconditions'), preconditions),
    steps                 = COALESCE(sqlc.narg('steps'), steps),
    expected_result       = COALESCE(sqlc.narg('expected_result'), expected_result),
    test_data             = COALESCE(sqlc.narg('test_data'), test_data),
    priority              = COALESCE(sqlc.narg('priority'), priority),
    case_type             = COALESCE(sqlc.narg('case_type'), case_type),
    scope                 = COALESCE(sqlc.narg('scope'), scope),
    execution_mode        = COALESCE(sqlc.narg('execution_mode'), execution_mode),
    required_capabilities = COALESCE(sqlc.narg('required_capabilities'), required_capabilities),
    business_rules_ref    = COALESCE(sqlc.narg('business_rules_ref'), business_rules_ref),
    status                = COALESCE(sqlc.narg('status'), status),
    reviewed_by           = COALESCE(sqlc.narg('reviewed_by'), reviewed_by),
    reviewed_at           = COALESCE(sqlc.narg('reviewed_at'), reviewed_at),
    updated_by            = COALESCE(sqlc.narg('updated_by'), updated_by),
    version               = version + 1,
    updated_at            = now()
WHERE id = $1 AND workspace_id = $2
RETURNING *;

-- name: DeleteTestCase :exec
-- Defense-in-depth: workspace_id is a SQL-layer tenant guard. See DeleteProject.
DELETE FROM test_case WHERE id = $1 AND workspace_id = $2;

-- name: ListTestCaseRepos :many
SELECT * FROM test_case_repo
WHERE test_case_id = $1
ORDER BY alias ASC, role ASC;

-- name: ListTestCaseReposForCases :many
SELECT * FROM test_case_repo
WHERE test_case_id = ANY(sqlc.arg('case_ids')::uuid[])
ORDER BY test_case_id, alias ASC, role ASC;

-- name: CreateTestCaseRepo :one
INSERT INTO test_case_repo (
    test_case_id, workspace_id, project_resource_id, alias, role, path_globs
) VALUES ($1, $2, $3, $4, $5, $6)
RETURNING *;

-- name: DeleteTestCaseRepos :exec
DELETE FROM test_case_repo WHERE test_case_id = $1 AND workspace_id = $2;

-- name: CreateTestCaseRevision :one
INSERT INTO test_case_revision (
    workspace_id, test_case_id, version, snapshot, change_kind,
    changed_by, changed_by_type, note
) VALUES ($1, $2, $3, $4, $5, $6, $7, $8)
RETURNING *;

-- name: ListTestCaseRevisions :many
SELECT * FROM test_case_revision
WHERE test_case_id = $1 AND workspace_id = $2
ORDER BY version DESC
LIMIT $3;

-- name: DeleteTestCaseRevisions :exec
DELETE FROM test_case_revision WHERE test_case_id = $1 AND workspace_id = $2;
