CREATE INDEX CONCURRENTLY idx_design_draft_generated_file ON design_draft(generated_file_id) WHERE generated_file_id IS NOT NULL;
