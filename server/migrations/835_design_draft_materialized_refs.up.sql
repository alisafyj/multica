ALTER TABLE design_draft
    ADD COLUMN generated_file_id UUID,
    ADD COLUMN generated_revision_id UUID,
    ADD COLUMN materialized_at TIMESTAMPTZ;
