CREATE TABLE pmo_sync_config (
    id UUID NOT NULL DEFAULT gen_random_uuid(),
    workspace_id UUID NOT NULL,
    name TEXT NOT NULL CHECK (btrim(name) <> ''),
    agent_id UUID NOT NULL,
    root_external_key TEXT NOT NULL CHECK (btrim(root_external_key) <> ''),
    workload_property_id UUID,
    schedule_enabled BOOLEAN NOT NULL DEFAULT false,
    next_run_at TIMESTAMPTZ,
    last_run_at TIMESTAMPTZ,
    last_applied_at TIMESTAMPTZ,
    created_by UUID NOT NULL,
    created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT now()
);

CREATE TABLE pmo_sync_run (
    id UUID NOT NULL DEFAULT gen_random_uuid(),
    workspace_id UUID NOT NULL,
    config_id UUID NOT NULL,
    agent_task_id UUID,
    trigger TEXT NOT NULL CHECK (trigger IN ('manual', 'scheduled')),
    status TEXT NOT NULL CHECK (status IN (
        'queued', 'running', 'preview_ready', 'applied',
        'applied_with_review', 'failed'
    )),
    source_snapshot JSONB NOT NULL DEFAULT '{}'::jsonb,
    diff JSONB NOT NULL DEFAULT '{}'::jsonb,
    summary JSONB NOT NULL DEFAULT '{}'::jsonb,
    error_code TEXT,
    error_message TEXT,
    requested_by UUID,
    created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    started_at TIMESTAMPTZ,
    completed_at TIMESTAMPTZ,
    applied_at TIMESTAMPTZ
);

CREATE TABLE pmo_sync_link (
    id UUID NOT NULL DEFAULT gen_random_uuid(),
    workspace_id UUID NOT NULL,
    config_id UUID NOT NULL,
    external_type TEXT NOT NULL CHECK (external_type IN ('requirement', 'task', 'assignee')),
    external_key TEXT NOT NULL,
    external_display_number TEXT,
    external_numeric_id BIGINT,
    external_task_id TEXT,
    parent_external_key TEXT,
    local_type TEXT CHECK (local_type IN ('project', 'issue', 'member')),
    local_id UUID,
    baseline_external JSONB NOT NULL DEFAULT '{}'::jsonb,
    baseline_local JSONB NOT NULL DEFAULT '{}'::jsonb,
    external_metadata JSONB NOT NULL DEFAULT '{}'::jsonb,
    externally_removed_at TIMESTAMPTZ,
    created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT now()
);
