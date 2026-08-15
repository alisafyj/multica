CREATE UNIQUE INDEX CONCURRENTLY IF NOT EXISTS idx_project_design_system_project_scope ON project_design_system (workspace_id, project_id) WHERE project_resource_id IS NULL;
