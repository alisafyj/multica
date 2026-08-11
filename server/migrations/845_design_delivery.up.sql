CREATE TABLE design_delivery (
    id              UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    workspace_id    UUID NOT NULL,
    project_id      UUID,
    source_issue_id UUID NOT NULL,
    target_issue_id UUID NOT NULL,
    file_id         UUID NOT NULL,
    revision_id     UUID NOT NULL,
    scope           JSONB NOT NULL DEFAULT '{}',
    status          TEXT NOT NULL DEFAULT 'active' CHECK (status IN ('active', 'superseded', 'cancelled')),
    delivered_by    UUID,
    delivered_at    TIMESTAMPTZ NOT NULL DEFAULT now(),
    created_at      TIMESTAMPTZ NOT NULL DEFAULT now(),
    updated_at      TIMESTAMPTZ NOT NULL DEFAULT now()
);
