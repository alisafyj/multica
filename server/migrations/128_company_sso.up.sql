ALTER TABLE "user"
    ADD COLUMN account_kind TEXT NOT NULL DEFAULT 'human'
    CHECK (account_kind IN ('human', 'service'));

CREATE TABLE sso_authorization_code (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    code_hash BYTEA UNIQUE NOT NULL,
    user_id UUID NOT NULL REFERENCES "user"(id) ON DELETE CASCADE,
    client_id TEXT NOT NULL CHECK (client_id IN ('cli', 'desktop', 'mobile')),
    redirect_uri TEXT NOT NULL,
    code_challenge TEXT NOT NULL,
    session_expires_at TIMESTAMPTZ NOT NULL,
    expires_at TIMESTAMPTZ NOT NULL,
    consumed_at TIMESTAMPTZ,
    created_at TIMESTAMPTZ NOT NULL DEFAULT now()
);

CREATE INDEX sso_authorization_code_expires_at_idx
    ON sso_authorization_code (expires_at);

CREATE TABLE service_account_token (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    user_id UUID NOT NULL REFERENCES "user"(id) ON DELETE CASCADE,
    workspace_id UUID NOT NULL REFERENCES workspace(id) ON DELETE CASCADE,
    token_hash TEXT UNIQUE NOT NULL,
    expires_at TIMESTAMPTZ NOT NULL,
    revoked_at TIMESTAMPTZ,
    last_used_at TIMESTAMPTZ,
    created_by UUID NOT NULL REFERENCES "user"(id),
    created_at TIMESTAMPTZ NOT NULL DEFAULT now()
);

CREATE UNIQUE INDEX service_account_token_one_active_idx
    ON service_account_token (user_id)
    WHERE revoked_at IS NULL;
