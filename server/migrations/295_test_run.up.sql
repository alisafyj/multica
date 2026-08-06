-- Test plans and execution runs. No FOREIGN KEY / cascade by repository rule:
-- project_id, plan_id, test_case_id, executor_id, agent_task_id, source_run_id
-- and defect_issue_id are validated in application code and swept in explicit
-- transactions.
CREATE TABLE test_plan (
    id           UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    workspace_id UUID NOT NULL,
    project_id   UUID NOT NULL,
    title        TEXT NOT NULL,
    description  TEXT NOT NULL DEFAULT '',
    status       TEXT NOT NULL DEFAULT 'draft'
                 CHECK (status IN ('draft', 'active', 'archived')),
    created_by   UUID,
    created_at   TIMESTAMPTZ NOT NULL DEFAULT now(),
    updated_at   TIMESTAMPTZ NOT NULL DEFAULT now()
);

CREATE TABLE test_plan_case (
    plan_id      UUID NOT NULL,
    workspace_id UUID NOT NULL,
    test_case_id UUID NOT NULL,
    position     INT  NOT NULL DEFAULT 0,
    created_at   TIMESTAMPTZ NOT NULL DEFAULT now(),
    PRIMARY KEY (plan_id, test_case_id)
);

-- One execution round. A retry is a NEW run pointing at the previous one
-- through source_run_id; results are never reset in place, because the point
-- of the record is that it cannot be rewritten.
CREATE TABLE test_run (
    id                 UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    workspace_id       UUID NOT NULL,
    project_id         UUID NOT NULL,
    plan_id            UUID,
    title              TEXT NOT NULL,
    executor_type      TEXT NOT NULL CHECK (executor_type IN ('member', 'agent')),
    executor_id        UUID NOT NULL,
    agent_task_id      UUID,
    environment        TEXT NOT NULL DEFAULT '',
    build_ref          TEXT NOT NULL DEFAULT '',
    capability_binding JSONB NOT NULL DEFAULT '{}',
    status             TEXT NOT NULL DEFAULT 'pending'
                       CHECK (status IN ('pending', 'running', 'completed', 'aborted', 'blocked')),
    source_run_id      UUID,
    retry_scope        TEXT CHECK (retry_scope IN ('all', 'failed_only', 'selected')),
    error              TEXT,
    started_at         TIMESTAMPTZ,
    completed_at       TIMESTAMPTZ,
    created_by         UUID,
    created_at         TIMESTAMPTZ NOT NULL DEFAULT now(),
    updated_at         TIMESTAMPTZ NOT NULL DEFAULT now()
);

-- case_snapshot freezes the case as it was when the run started. Editing a
-- case afterwards must not rewrite what a past round actually executed, or the
-- regression history stops meaning anything.
CREATE TABLE test_run_case (
    id               UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    workspace_id     UUID NOT NULL,
    run_id           UUID NOT NULL,
    test_case_id     UUID NOT NULL,
    case_snapshot    JSONB NOT NULL,
    position         INT  NOT NULL DEFAULT 0,
    result           TEXT NOT NULL DEFAULT 'pending'
                     CHECK (result IN ('pending', 'running', 'passed', 'failed', 'blocked', 'skipped')),
    notes            TEXT NOT NULL DEFAULT '',
    evidence         JSONB NOT NULL DEFAULT '[]',
    step_results     JSONB NOT NULL DEFAULT '[]',
    duration_ms      INT,
    executed_by_type TEXT CHECK (executed_by_type IN ('member', 'agent')),
    executed_by_id   UUID,
    executed_at      TIMESTAMPTZ,
    defect_issue_id  UUID,
    created_at       TIMESTAMPTZ NOT NULL DEFAULT now(),
    updated_at       TIMESTAMPTZ NOT NULL DEFAULT now()
);

-- Evidence reuses the existing attachment system rather than adding storage.
ALTER TABLE attachment ADD COLUMN test_run_case_id UUID;

-- Physical execution capabilities reported by a daemon: which phone, browser
-- or desktop a run can actually drive. target is JSONB for the same reason
-- project_resource.resource_ref is — a new kind costs one validation branch,
-- not a migration.
CREATE TABLE test_capability (
    id             UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    workspace_id   UUID NOT NULL,
    daemon_id      TEXT NOT NULL,
    runtime_id     UUID,
    kind           TEXT NOT NULL
                   CHECK (kind IN ('android_device', 'ios_device', 'computer_use', 'browser')),
    capability_key TEXT NOT NULL,
    target         JSONB NOT NULL DEFAULT '{}',
    status         TEXT NOT NULL DEFAULT 'unknown'
                   CHECK (status IN ('available', 'busy', 'offline', 'unknown')),
    probe          JSONB NOT NULL DEFAULT '{}',
    last_probe_at  TIMESTAMPTZ,
    created_at     TIMESTAMPTZ NOT NULL DEFAULT now(),
    updated_at     TIMESTAMPTZ NOT NULL DEFAULT now()
);
