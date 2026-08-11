-- name: ListTestCapabilities :many
SELECT * FROM test_capability
WHERE workspace_id = $1
  AND (sqlc.narg('kind')::text IS NULL OR kind = sqlc.narg('kind'))
  AND (sqlc.narg('status')::text IS NULL OR status = sqlc.narg('status'))
  AND (sqlc.narg('daemon_id')::text IS NULL OR daemon_id = sqlc.narg('daemon_id'))
ORDER BY kind ASC, capability_key ASC;

-- name: ListAvailableTestCapabilities :many
-- Resolution input: only rows a run could actually be bound to right now.
SELECT * FROM test_capability
WHERE workspace_id = $1 AND status = 'available'
ORDER BY daemon_id ASC, kind ASC, capability_key ASC;

-- name: GetTestCapabilityInWorkspace :one
SELECT * FROM test_capability WHERE id = $1 AND workspace_id = $2;

-- name: UpsertTestCapability :one
-- Daemons re-report their inventory on every probe, so this is idempotent on
-- (workspace, daemon, key).
INSERT INTO test_capability (
    workspace_id, daemon_id, runtime_id, kind, capability_key, target,
    status, probe, last_probe_at
) VALUES ($1, $2, $3, $4, $5, $6, $7, $8, now())
ON CONFLICT (workspace_id, daemon_id, capability_key) DO UPDATE SET
    runtime_id    = EXCLUDED.runtime_id,
    kind          = EXCLUDED.kind,
    target        = EXCLUDED.target,
    status        = EXCLUDED.status,
    probe         = EXCLUDED.probe,
    last_probe_at = now(),
    updated_at    = now()
RETURNING *;

-- name: MarkTestCapabilitiesOfflineForDaemon :exec
-- A daemon that stops reporting a capability must not leave it looking
-- available: a run would resolve onto a device that is no longer attached.
UPDATE test_capability SET status = 'offline', updated_at = now()
WHERE workspace_id = $1 AND daemon_id = $2
  AND NOT (capability_key = ANY(sqlc.arg('present_keys')::text[]));

-- name: DeleteTestCapabilitiesForDaemon :exec
DELETE FROM test_capability WHERE workspace_id = $1 AND daemon_id = $2;
