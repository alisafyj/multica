CREATE TABLE design_restore_plan (
    id              UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    workspace_id    UUID NOT NULL,
    restore_task_id UUID NOT NULL,
    status          TEXT NOT NULL DEFAULT 'draft' CHECK (status IN ('draft', 'approved', 'dispatched', 'archived')),
    plan            JSONB NOT NULL DEFAULT '{}',
    review_notes    TEXT,
    approved_by     UUID,
    approved_at     TIMESTAMPTZ,
    created_by      UUID,
    created_at      TIMESTAMPTZ NOT NULL DEFAULT now(),
    updated_at      TIMESTAMPTZ NOT NULL DEFAULT now()
);
