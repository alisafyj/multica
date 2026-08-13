CREATE TABLE project_design_system (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    workspace_id UUID NOT NULL,
    project_id UUID NOT NULL,
    name TEXT NOT NULL,
    platform TEXT NOT NULL CHECK (platform IN ('web', 'mobile', 'cross_platform')),
    current_agent_id UUID,
    active_task_id UUID,
    active_operation TEXT CHECK (
        active_operation IS NULL
        OR active_operation IN ('generate', 'adjust', 'regenerate')
    ),
    input_snapshot JSONB NOT NULL DEFAULT '{}'::jsonb,
    last_error JSONB,
    created_by UUID,
    created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    saved_at TIMESTAMPTZ,
    UNIQUE (workspace_id, project_id)
);

CREATE TABLE project_design_system_package (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    design_system_id UUID NOT NULL,
    slot TEXT NOT NULL CHECK (slot IN ('draft', 'saved')),
    design_md TEXT NOT NULL,
    tokens_css TEXT NOT NULL,
    components_html TEXT NOT NULL,
    manifest JSONB NOT NULL,
    validation JSONB NOT NULL,
    integrity_sha256 TEXT NOT NULL,
    source_task_id UUID,
    agent_id UUID,
    instruction TEXT,
    scope JSONB,
    created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    UNIQUE (design_system_id, slot)
);
