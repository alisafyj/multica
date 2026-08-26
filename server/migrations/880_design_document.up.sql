-- Design Document: the versioned page-design artifact produced from the
-- design centre home composer (P-011 / DC-042).
--
-- This is a NEW entity beside the historical design_draft table, not a
-- replacement of it. The old PageSpec path keeps its data and its consumers
-- until a later slice proves full replacement (spec §17.4).
--
-- Shape:
--   design_document          stable identity + draft/saved pointers
--   design_document_revision immutable content, one row per generation or
--                            adjustment
--
-- No foreign keys per repository policy: the pointers below are resolved and
-- cleaned up in application code, inside a transaction where atomicity
-- matters.

-- Before any of that: fork main shipped a DIFFERENT design_document under the
-- stem 878_design_document (released in v0.4.23-sso.7), developed in parallel
-- with this one. The runner keys schema_migrations on the full stem, so on a
-- database that ran fork main both stems are independent and this file still
-- executes — straight into "relation design_document already exists", which
-- stops the whole deployment.
--
-- Two of this branch's own indexes make it worse quietly: 882 and 883 create
-- idx_design_document_project and idx_design_document_issue, names fork main's
-- 879 and 880 already took on the old tables. Both use IF NOT EXISTS, so they
-- would report success and leave the new design_document with no index behind
-- the project list or the issue lookup.
--
-- The predecessor is not data this build can read: its Go code, its queries and
-- its design_document_input_snapshot table are gone from this branch, and the
-- redesign is not expressible as ALTERs (the snapshot table folds into a JSONB
-- column). So it is removed, and only when the old shape is what is actually
-- there — the branch's own design_document has a platform column and the
-- predecessor's does not. On a fresh database, and on one that already ran this
-- file, the block is a no-op.
DO $$
BEGIN
    IF to_regclass('public.design_document') IS NOT NULL
        AND NOT EXISTS (
            SELECT 1
            FROM information_schema.columns
            WHERE table_schema = 'public'
              AND table_name = 'design_document'
              AND column_name = 'platform'
        )
    THEN
        DROP TABLE IF EXISTS design_document_input_snapshot;
        DROP TABLE IF EXISTS design_document_revision;
        DROP TABLE IF EXISTS design_document;
    END IF;
END
$$;

CREATE TABLE design_document (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    workspace_id UUID NOT NULL,
    project_id UUID NOT NULL,
    -- Optional repository the design targets (DC-053). NULL means the task
    -- ran without a repository and did no grounding.
    project_resource_id UUID,
    -- Optional traceable link. Saving a document never changes issue state
    -- (DC-045).
    issue_id UUID,
    title TEXT NOT NULL,
    platform TEXT NOT NULL CHECK (platform IN ('web', 'mobile', 'cross_platform')),
    -- The recipe the home composer dispatched (DC-049). Kept as free text so
    -- the template slice can widen it to template ids without a migration.
    recipe TEXT NOT NULL DEFAULT 'default',
    -- Pointers into design_document_revision. draft is what the user is
    -- iterating on; saved is what downstream agents, MCP and delivery read.
    -- Both may be NULL: a document whose first generation failed has neither.
    draft_revision_id UUID,
    saved_revision_id UUID,
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
    saved_at TIMESTAMPTZ
);

-- Revisions are immutable. Nothing updates a row here; an adjustment writes a
-- new revision that points at the one it was based on. That is what makes a
-- draft safe to throw away without touching what was saved.
CREATE TABLE design_document_revision (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    workspace_id UUID NOT NULL,
    design_document_id UUID NOT NULL,
    revision_number INT NOT NULL CHECK (revision_number > 0),
    package_schema TEXT NOT NULL,
    -- Digest of the package contents. Audit and preview receipts bind to this
    -- value, so a changed package automatically invalidates old receipts.
    content_digest TEXT NOT NULL,
    archive_object_key TEXT NOT NULL,
    artifact_index JSONB NOT NULL DEFAULT '[]'::jsonb,
    manifest JSONB NOT NULL,
    brief JSONB NOT NULL,
    coverage JSONB NOT NULL,
    audit JSONB NOT NULL,
    preview JSONB,
    -- Fixed inputs for this revision: what the user asked for, which revision
    -- it was based on, and which saved design system constrained it.
    input_snapshot_sha256 TEXT NOT NULL,
    base_revision_id UUID,
    design_system_digest TEXT,
    source_task_id UUID,
    agent_id UUID,
    -- Set for adjust/regenerate: the instruction and the document, page,
    -- state or named block it was scoped to.
    instruction TEXT,
    scope JSONB,
    created_at TIMESTAMPTZ NOT NULL DEFAULT now()
);
