-- AI test case generation. No FOREIGN KEY / cascade by repository rule:
-- project_id, agent_id, agent_task_id, job_id and target_case_id are validated
-- in application code and cleaned up in explicit transactions.
CREATE TABLE test_generation_job (
    id            UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    workspace_id  UUID NOT NULL,
    project_id    UUID NOT NULL,
    agent_id      UUID,
    agent_task_id UUID,
    status        TEXT NOT NULL DEFAULT 'queued'
                  CHECK (status IN ('queued', 'running', 'completed', 'failed', 'cancelled')),
    input         JSONB NOT NULL DEFAULT '{}',
    result        JSONB NOT NULL DEFAULT '{}',
    error         TEXT,
    created_by    UUID,
    created_at    TIMESTAMPTZ NOT NULL DEFAULT now(),
    updated_at    TIMESTAMPTZ NOT NULL DEFAULT now()
);

-- The human-reviewed scope contract: which repositories, paths, modules and
-- business rules a run may cover. Approval gates dispatch, so a wrong scope is
-- caught before it burns a whole context window.
CREATE TABLE test_generation_plan (
    id           UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    workspace_id UUID NOT NULL,
    job_id       UUID NOT NULL,
    status       TEXT NOT NULL DEFAULT 'draft'
                 CHECK (status IN ('draft', 'approved', 'dispatched', 'archived')),
    plan         JSONB NOT NULL DEFAULT '{}',
    review_notes TEXT NOT NULL DEFAULT '',
    approved_by  UUID,
    approved_at  TIMESTAMPTZ,
    created_by   UUID,
    created_at   TIMESTAMPTZ NOT NULL DEFAULT now(),
    updated_at   TIMESTAMPTZ NOT NULL DEFAULT now()
);

-- AI suggestions that must NOT silently overwrite a human-approved case.
-- New cases land directly in test_case(status='draft'); only update and
-- obsolete come through here for side-by-side review.
CREATE TABLE test_case_proposal (
    id             UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    workspace_id   UUID NOT NULL,
    job_id         UUID NOT NULL,
    target_case_id UUID NOT NULL,
    kind           TEXT NOT NULL CHECK (kind IN ('update', 'obsolete')),
    payload        JSONB NOT NULL DEFAULT '{}',
    rationale      TEXT NOT NULL DEFAULT '',
    status         TEXT NOT NULL DEFAULT 'pending'
                   CHECK (status IN ('pending', 'accepted', 'rejected')),
    reviewed_by    UUID,
    reviewed_at    TIMESTAMPTZ,
    created_at     TIMESTAMPTZ NOT NULL DEFAULT now()
);
