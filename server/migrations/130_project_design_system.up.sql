CREATE TABLE project_design_system (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    workspace_id UUID NOT NULL REFERENCES workspace(id) ON DELETE CASCADE,
    project_id UUID NOT NULL REFERENCES project(id) ON DELETE CASCADE,
    name TEXT NOT NULL,
    platform TEXT NOT NULL CHECK (platform IN ('web', 'mobile', 'cross_platform')),
    current_agent_id UUID REFERENCES agent(id) ON DELETE SET NULL,
    active_task_id UUID REFERENCES agent_task_queue(id) ON DELETE SET NULL,
    active_operation TEXT CHECK (
        active_operation IS NULL
        OR active_operation IN ('generate', 'adjust', 'regenerate')
    ),
    input_snapshot JSONB NOT NULL DEFAULT '{}'::jsonb,
    last_error JSONB,
    created_by UUID REFERENCES "user"(id) ON DELETE SET NULL,
    created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    saved_at TIMESTAMPTZ,
    UNIQUE (workspace_id, project_id)
);

CREATE INDEX idx_project_design_system_workspace
    ON project_design_system(workspace_id);

CREATE INDEX idx_project_design_system_active_task
    ON project_design_system(active_task_id)
    WHERE active_task_id IS NOT NULL;

CREATE TABLE project_design_system_package (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    design_system_id UUID NOT NULL REFERENCES project_design_system(id) ON DELETE CASCADE,
    slot TEXT NOT NULL CHECK (slot IN ('draft', 'saved')),
    design_md TEXT NOT NULL,
    tokens_css TEXT NOT NULL,
    components_html TEXT NOT NULL,
    manifest JSONB NOT NULL,
    validation JSONB NOT NULL,
    integrity_sha256 TEXT NOT NULL,
    source_task_id UUID REFERENCES agent_task_queue(id) ON DELETE SET NULL,
    agent_id UUID REFERENCES agent(id) ON DELETE SET NULL,
    instruction TEXT,
    scope JSONB,
    created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    UNIQUE (design_system_id, slot)
);

CREATE INDEX idx_project_design_system_package_system
    ON project_design_system_package(design_system_id);
