CREATE INDEX CONCURRENTLY idx_design_template_system ON design_template(is_system, category, key) WHERE is_system = TRUE;
