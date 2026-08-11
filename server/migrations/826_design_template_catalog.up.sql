CREATE TABLE design_template_library (
    id            UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    workspace_id  UUID NOT NULL,
    key           TEXT NOT NULL,
    name          TEXT NOT NULL,
    description   TEXT,
    metadata      JSONB NOT NULL DEFAULT '{}',
    created_by    UUID,
    created_at    TIMESTAMPTZ NOT NULL DEFAULT now(),
    updated_at    TIMESTAMPTZ NOT NULL DEFAULT now(),
    UNIQUE (workspace_id, key)
);

CREATE TABLE design_catalog_template (
    id                  UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    workspace_id        UUID NOT NULL,
    library_id          UUID NOT NULL,
    key                 TEXT NOT NULL,
    name                TEXT NOT NULL,
    description         TEXT,
    category            TEXT NOT NULL DEFAULT 'custom',
    current_revision_id UUID,
    metadata            JSONB NOT NULL DEFAULT '{}',
    created_by          UUID,
    created_at          TIMESTAMPTZ NOT NULL DEFAULT now(),
    updated_at          TIMESTAMPTZ NOT NULL DEFAULT now(),
    UNIQUE (workspace_id, library_id, key)
);

CREATE TABLE design_template_revision (
    id                 UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    workspace_id       UUID NOT NULL,
    template_id        UUID NOT NULL,
    design_revision_id UUID NOT NULL,
    revision_number    INT NOT NULL,
    status             TEXT NOT NULL DEFAULT 'published' CHECK (status IN ('draft', 'published', 'archived')),
    slot_schema        JSONB NOT NULL DEFAULT '{}',
    metadata           JSONB NOT NULL DEFAULT '{}',
    created_by         UUID,
    created_at         TIMESTAMPTZ NOT NULL DEFAULT now(),
    UNIQUE (template_id, revision_number),
    UNIQUE (template_id, design_revision_id)
);
