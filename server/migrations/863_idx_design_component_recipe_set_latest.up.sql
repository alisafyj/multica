CREATE INDEX CONCURRENTLY idx_design_component_recipe_set_latest
    ON design_component_recipe_set (workspace_id, design_system_profile_id, analysis_version DESC)
    WHERE status = 'valid';
