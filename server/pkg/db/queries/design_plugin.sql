-- name: CreateDesignPluginAuthSession :one
INSERT INTO design_plugin_auth_session (provider, user_code, expires_at)
VALUES ($1, $2, $3)
RETURNING *;

-- name: GetDesignPluginAuthSession :one
SELECT * FROM design_plugin_auth_session
WHERE id = $1 AND provider = $2;

-- name: ApproveDesignPluginAuthSession :one
UPDATE design_plugin_auth_session
SET user_id = $3,
    workspace_id = $4,
    approved_at = now()
WHERE id = $1
  AND provider = $2
  AND approved_at IS NULL
  AND consumed_at IS NULL
  AND expires_at > now()
RETURNING *;

-- name: GetConsumableDesignPluginAuthSessionForUpdate :one
SELECT * FROM design_plugin_auth_session
WHERE id = $1
  AND provider = $2
  AND approved_at IS NOT NULL
  AND consumed_at IS NULL
  AND expires_at > now()
FOR UPDATE;

-- name: ConsumeDesignPluginAuthSession :exec
UPDATE design_plugin_auth_session
SET consumed_at = now()
WHERE id = $1;

-- name: CreateDesignPluginToken :one
INSERT INTO design_plugin_token (provider, token_hash, token_prefix, user_id, workspace_id, scope, name, expires_at)
VALUES ($1, $2, $3, $4, $5, $6, $7, $8)
RETURNING *;

-- name: GetDesignPluginTokenByHash :one
SELECT * FROM design_plugin_token
WHERE token_hash = $1
  AND provider = $2
  AND revoked_at IS NULL
  AND (expires_at IS NULL OR expires_at > now());

-- name: UpdateDesignPluginTokenLastUsed :exec
UPDATE design_plugin_token
SET last_used_at = now()
WHERE id = $1;
