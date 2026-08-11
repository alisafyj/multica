CREATE INDEX CONCURRENTLY idx_design_system_profile_workspace_project
    ON design_system_profile (workspace_id, project_id, updated_at DESC);
