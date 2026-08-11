CREATE INDEX CONCURRENTLY idx_design_draft_catalog_template ON design_draft(catalog_template_id) WHERE catalog_template_id IS NOT NULL;
