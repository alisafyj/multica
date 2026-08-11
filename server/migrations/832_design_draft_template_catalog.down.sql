ALTER TABLE design_draft
    DROP COLUMN IF EXISTS template_revision_id,
    DROP COLUMN IF EXISTS catalog_template_id;
