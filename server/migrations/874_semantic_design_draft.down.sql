ALTER TABLE design_draft
    DROP COLUMN IF EXISTS version,
    DROP COLUMN IF EXISTS parent_draft_id,
    DROP COLUMN IF EXISTS recipe_set_id,
    DROP COLUMN IF EXISTS blueprint_id,
    DROP COLUMN IF EXISTS quality_report,
    DROP COLUMN IF EXISTS compiled_native_json,
    DROP COLUMN IF EXISTS page_spec,
    DROP COLUMN IF EXISTS generation_mode;

ALTER TABLE design_draft
    DROP CONSTRAINT IF EXISTS design_draft_status_check;

ALTER TABLE design_draft
    ADD CONSTRAINT design_draft_status_check
    CHECK (status IN ('draft', 'generated', 'validated', 'failed', 'archived'));
