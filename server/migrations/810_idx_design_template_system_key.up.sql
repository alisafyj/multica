CREATE UNIQUE INDEX CONCURRENTLY idx_design_template_system_key ON design_template(key) WHERE workspace_id IS NULL AND is_system = TRUE;
