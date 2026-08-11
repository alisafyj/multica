CREATE TABLE IF NOT EXISTS design_plugin_auth_session (
    id            UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    provider      TEXT NOT NULL CHECK (provider IN ('figma')),
    user_code     TEXT NOT NULL UNIQUE,
    user_id       UUID,
    workspace_id  UUID,
    approved_at   TIMESTAMPTZ,
    consumed_at   TIMESTAMPTZ,
    expires_at    TIMESTAMPTZ NOT NULL,
    created_at    TIMESTAMPTZ NOT NULL DEFAULT now()
);

CREATE TABLE IF NOT EXISTS design_plugin_token (
    id            UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    provider      TEXT NOT NULL CHECK (provider IN ('figma')),
    token_hash    TEXT NOT NULL UNIQUE,
    token_prefix  TEXT NOT NULL,
    user_id       UUID NOT NULL,
    workspace_id  UUID NOT NULL,
    scope         TEXT NOT NULL DEFAULT 'design_import',
    name          TEXT NOT NULL DEFAULT 'Figma Plugin',
    expires_at    TIMESTAMPTZ,
    revoked_at    TIMESTAMPTZ,
    last_used_at  TIMESTAMPTZ,
    created_at    TIMESTAMPTZ NOT NULL DEFAULT now()
);
