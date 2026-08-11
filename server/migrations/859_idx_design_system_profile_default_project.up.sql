CREATE UNIQUE INDEX CONCURRENTLY idx_design_system_profile_default_project
    ON design_system_profile (workspace_id, project_id)
    WHERE is_default = true
      AND project_id IS NOT NULL;
