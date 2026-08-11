CREATE TABLE IF NOT EXISTS design_import_code (
    id                  UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    workspace_id        UUID NOT NULL,
    user_id             UUID NOT NULL,
    provider            TEXT NOT NULL CHECK (provider IN ('figma')),
    code_hash           TEXT NOT NULL UNIQUE,
    expires_at          TIMESTAMPTZ NOT NULL,
    consumed_at         TIMESTAMPTZ,
    failed_attempts     INTEGER NOT NULL DEFAULT 0,
    last_failed_at      TIMESTAMPTZ,
    created_at          TIMESTAMPTZ NOT NULL DEFAULT now()
);
