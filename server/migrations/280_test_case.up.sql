-- Test case library. No FOREIGN KEY / cascade by repository rule: relationships
-- (project_id, generation_job_id, project_resource_id) are validated in
-- application code and cleaned up in explicit transactions.
CREATE TABLE test_case (
    id                    UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    workspace_id          UUID NOT NULL,
    project_id            UUID NOT NULL,
    case_number           INT  NOT NULL,
    title                 TEXT NOT NULL,
    module                TEXT NOT NULL DEFAULT '',
    preconditions         TEXT NOT NULL DEFAULT '',
    steps                 JSONB NOT NULL DEFAULT '[]',
    expected_result       TEXT NOT NULL DEFAULT '',
    test_data             JSONB NOT NULL DEFAULT '{}',
    priority              TEXT NOT NULL DEFAULT 'p2'
                          CHECK (priority IN ('p0', 'p1', 'p2', 'p3')),
    case_type             TEXT NOT NULL DEFAULT 'functional'
                          CHECK (case_type IN ('functional', 'business_flow', 'api', 'ui', 'e2e',
                                               'regression', 'boundary', 'exception', 'permission',
                                               'data_consistency', 'compatibility', 'performance', 'security')),
    scope                 TEXT NOT NULL DEFAULT 'single_repo'
                          CHECK (scope IN ('single_repo', 'cross_repo', 'no_repo')),
    execution_mode        TEXT NOT NULL DEFAULT 'manual'
                          CHECK (execution_mode IN ('manual', 'agent', 'both')),
    required_capabilities JSONB NOT NULL DEFAULT '[]',
    business_rules_ref    JSONB NOT NULL DEFAULT '[]',
    status                TEXT NOT NULL DEFAULT 'draft'
                          CHECK (status IN ('draft', 'active', 'deprecated')),
    origin                TEXT NOT NULL DEFAULT 'human'
                          CHECK (origin IN ('ai', 'human')),
    source_refs           JSONB NOT NULL DEFAULT '{}',
    generation_job_id     UUID,
    version               INT  NOT NULL DEFAULT 1,
    created_by            UUID,
    updated_by            UUID,
    reviewed_by           UUID,
    reviewed_at           TIMESTAMPTZ,
    created_at            TIMESTAMPTZ NOT NULL DEFAULT now(),
    updated_at            TIMESTAMPTZ NOT NULL DEFAULT now()
);

-- Snapshot of a case as it was BEFORE each change, so review is reversible.
CREATE TABLE test_case_revision (
    id              UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    workspace_id    UUID NOT NULL,
    test_case_id    UUID NOT NULL,
    version         INT  NOT NULL,
    snapshot        JSONB NOT NULL,
    change_kind     TEXT NOT NULL
                    CHECK (change_kind IN ('human_edit', 'proposal_accepted', 'status_change', 'restore')),
    changed_by      UUID,
    changed_by_type TEXT NOT NULL DEFAULT 'member'
                    CHECK (changed_by_type IN ('member', 'agent')),
    note            TEXT NOT NULL DEFAULT '',
    created_at      TIMESTAMPTZ NOT NULL DEFAULT now()
);

-- Which repositories of the project a case touches. Bound to
-- project_resource.id (stable within a workspace) rather than a repo URL
-- (mutable). role lets a cross-repo case say which system each step drives.
CREATE TABLE test_case_repo (
    test_case_id        UUID NOT NULL,
    workspace_id        UUID NOT NULL,
    project_resource_id UUID NOT NULL,
    alias               TEXT NOT NULL,
    role                TEXT NOT NULL DEFAULT 'under_test'
                        CHECK (role IN ('under_test', 'driver', 'verifier', 'fixture')),
    path_globs          JSONB NOT NULL DEFAULT '[]',
    created_at          TIMESTAMPTZ NOT NULL DEFAULT now(),
    PRIMARY KEY (test_case_id, project_resource_id, role)
);

-- Human-readable case key TC-<n>, allocated per workspace exactly like
-- workspace.issue_counter (migration 020).
ALTER TABLE workspace ADD COLUMN test_case_counter INT NOT NULL DEFAULT 0;
