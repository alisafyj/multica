CREATE TABLE open_design_run (
    id UUID PRIMARY KEY,
    workspace_id UUID NOT NULL REFERENCES workspace(id) ON DELETE CASCADE,
    project_id UUID NOT NULL REFERENCES project(id) ON DELETE CASCADE,
    design_system_id UUID NOT NULL REFERENCES project_design_system(id) ON DELETE CASCADE,
    task_id UUID NOT NULL UNIQUE,
    operation TEXT NOT NULL CHECK (operation IN ('generate', 'adjust', 'regenerate')),
    status TEXT NOT NULL CHECK (
        status IN (
            'preflight_pending',
            'preflight_failed',
            'ready',
            'running',
            'run_succeeded',
            'canceled',
            'agent_failed',
            'audit_failed',
            'preview_failed',
            'succeeded'
        )
    ),
    engine_release TEXT NOT NULL,
    engine_commit TEXT NOT NULL,
    engine_lockfile_sha256 TEXT NOT NULL,
    engine_dist_sha256 TEXT NOT NULL,
    agent_id UUID REFERENCES agent(id) ON DELETE SET NULL,
    agent_snapshot JSONB NOT NULL,
    adapter_id TEXT NOT NULL,
    model TEXT,
    preflight JSONB NOT NULL DEFAULT '{}'::jsonb,
    input_snapshot JSONB NOT NULL,
    workspace_provenance JSONB NOT NULL,
    open_design_run_id TEXT,
    events JSONB NOT NULL DEFAULT '[]'::jsonb,
    result_package JSONB,
    artifact_index JSONB NOT NULL DEFAULT '[]'::jsonb,
    archive_object_key TEXT,
    content_digest TEXT,
    audit_report JSONB,
    preview_receipt JSONB,
    failure JSONB NOT NULL DEFAULT '{}'::jsonb,
    started_at TIMESTAMPTZ,
    finished_at TIMESTAMPTZ,
    created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT now()
);

CREATE INDEX idx_open_design_run_design_system
    ON open_design_run(design_system_id, created_at DESC);

CREATE INDEX idx_open_design_run_workspace_status
    ON open_design_run(workspace_id, status);
