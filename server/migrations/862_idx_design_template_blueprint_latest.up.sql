CREATE INDEX CONCURRENTLY idx_design_template_blueprint_latest
    ON design_template_blueprint (workspace_id, template_revision_id, analysis_version DESC)
    WHERE status = 'valid';
