ALTER TABLE design_draft
    DROP CONSTRAINT IF EXISTS design_draft_status_check;

ALTER TABLE design_draft
    ADD CONSTRAINT design_draft_status_check
    CHECK (status IN (
        'draft',
        'generated',
        'generated_with_warnings',
        'validated',
        'compile_failed',
        'failed',
        'rejected',
        'approved',
        'archived'
    ));

ALTER TABLE design_draft
    ADD COLUMN generation_mode TEXT NOT NULL DEFAULT 'legacy_patch' CHECK (generation_mode IN ('legacy_patch', 'semantic_pagespec')),
    ADD COLUMN page_spec JSONB,
    ADD COLUMN compiled_native_json JSONB,
    ADD COLUMN quality_report JSONB,
    ADD COLUMN blueprint_id UUID,
    ADD COLUMN recipe_set_id UUID,
    ADD COLUMN parent_draft_id UUID,
    ADD COLUMN version INT NOT NULL DEFAULT 1 CHECK (version > 0);
