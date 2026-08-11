-- Gallery Native design asset system. These tables are workspace-scoped and
-- intentionally independent from issues/projects; issue metadata may point to
-- design_draft rows later, but never stores native JSON payloads inline.

CREATE TABLE design_file (
    id                  UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    workspace_id        UUID NOT NULL,
    project_id          UUID,
    folder_id           UUID,
    title               TEXT NOT NULL,
    description         TEXT,
    source_type         TEXT NOT NULL CHECK (source_type IN ('upload', 'ai_generated', 'template', 'import')),
    source_ref          JSONB NOT NULL DEFAULT '{}',
    current_revision_id UUID,
    created_by          UUID,
    created_at          TIMESTAMPTZ NOT NULL DEFAULT now(),
    updated_at          TIMESTAMPTZ NOT NULL DEFAULT now()
);

CREATE TABLE design_folder (
    id           UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    workspace_id UUID NOT NULL,
    project_id   UUID NOT NULL,
    parent_id    UUID,
    name         TEXT NOT NULL,
    position     INT NOT NULL DEFAULT 0,
    created_by   UUID,
    created_at   TIMESTAMPTZ NOT NULL DEFAULT now(),
    updated_at   TIMESTAMPTZ NOT NULL DEFAULT now(),
    UNIQUE (workspace_id, project_id, parent_id, name)
);

CREATE TABLE design_revision (
    id                UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    file_id           UUID NOT NULL,
    workspace_id      UUID NOT NULL,
    revision_number   INT NOT NULL,
    status            TEXT NOT NULL DEFAULT 'draft' CHECK (status IN ('draft', 'valid', 'invalid')),
    native_json       JSONB NOT NULL,
    validation_errors JSONB NOT NULL DEFAULT '[]',
    created_by        UUID,
    created_at        TIMESTAMPTZ NOT NULL DEFAULT now(),
    UNIQUE (file_id, revision_number)
);

CREATE TABLE design_asset (
    id            UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    file_id       UUID NOT NULL,
    revision_id   UUID,
    workspace_id  UUID NOT NULL,
    asset_key     TEXT NOT NULL,
    kind          TEXT NOT NULL CHECK (kind IN ('image', 'slice', 'thumbnail', 'source', 'other')),
    url           TEXT NOT NULL,
    content_type  TEXT,
    size_bytes    BIGINT,
    metadata      JSONB NOT NULL DEFAULT '{}',
    created_by    UUID,
    created_at    TIMESTAMPTZ NOT NULL DEFAULT now(),
    UNIQUE (file_id, asset_key)
);

CREATE TABLE design_template (
    id              UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    workspace_id    UUID,
    key             TEXT NOT NULL,
    name            TEXT NOT NULL,
    description     TEXT,
    category        TEXT NOT NULL DEFAULT 'custom',
    native_json     JSONB NOT NULL,
    slot_schema     JSONB NOT NULL DEFAULT '{}',
    metadata        JSONB NOT NULL DEFAULT '{}',
    is_system       BOOLEAN NOT NULL DEFAULT FALSE,
    created_by      UUID,
    created_at      TIMESTAMPTZ NOT NULL DEFAULT now(),
    updated_at      TIMESTAMPTZ NOT NULL DEFAULT now(),
    UNIQUE (workspace_id, key)
);

CREATE TABLE design_template_slot (
    id              UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    template_id     UUID NOT NULL,
    slot_key        TEXT NOT NULL,
    label           TEXT NOT NULL,
    slot_type       TEXT NOT NULL CHECK (slot_type IN ('text', 'number', 'boolean', 'image', 'color', 'enum', 'list', 'object')),
    required        BOOLEAN NOT NULL DEFAULT FALSE,
    default_value   JSONB,
    constraints     JSONB NOT NULL DEFAULT '{}',
    description     TEXT,
    position        INT NOT NULL DEFAULT 0,
    created_at      TIMESTAMPTZ NOT NULL DEFAULT now(),
    UNIQUE (template_id, slot_key)
);

CREATE TABLE design_draft (
    id                   UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    workspace_id         UUID NOT NULL,
    template_id          UUID,
    file_id              UUID,
    revision_id          UUID,
    issue_id             UUID,
    title                TEXT NOT NULL,
    requirement_core     JSONB NOT NULL DEFAULT '{}',
    slot_values          JSONB NOT NULL DEFAULT '{}',
    patch                JSONB NOT NULL DEFAULT '[]',
    status               TEXT NOT NULL DEFAULT 'draft' CHECK (status IN ('draft', 'generated', 'validated', 'failed', 'archived')),
    validation_errors    JSONB NOT NULL DEFAULT '[]',
    created_by           UUID,
    created_at           TIMESTAMPTZ NOT NULL DEFAULT now(),
    updated_at           TIMESTAMPTZ NOT NULL DEFAULT now()
);

CREATE TABLE design_restore_task (
    id              UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    workspace_id    UUID NOT NULL,
    file_id         UUID NOT NULL,
    revision_id     UUID NOT NULL,
    issue_id        UUID,
    agent_task_id   UUID,
    status          TEXT NOT NULL DEFAULT 'queued' CHECK (status IN ('queued', 'running', 'completed', 'failed', 'cancelled')),
    input           JSONB NOT NULL DEFAULT '{}',
    result          JSONB NOT NULL DEFAULT '{}',
    error           TEXT,
    created_by      UUID,
    created_at      TIMESTAMPTZ NOT NULL DEFAULT now(),
    updated_at      TIMESTAMPTZ NOT NULL DEFAULT now()
);

CREATE TABLE design_restore_mapping (
    id              UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    restore_task_id UUID NOT NULL,
    workspace_id    UUID NOT NULL,
    layer_id        TEXT NOT NULL,
    target_path     TEXT NOT NULL,
    target_kind     TEXT NOT NULL CHECK (target_kind IN ('component', 'file', 'symbol', 'route', 'unknown')),
    confidence      REAL NOT NULL DEFAULT 0 CHECK (confidence >= 0 AND confidence <= 1),
    metadata        JSONB NOT NULL DEFAULT '{}',
    created_at      TIMESTAMPTZ NOT NULL DEFAULT now()
);
