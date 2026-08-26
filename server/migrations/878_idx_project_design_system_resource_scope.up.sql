CREATE UNIQUE INDEX CONCURRENTLY IF NOT EXISTS idx_project_design_system_resource_scope ON project_design_system (workspace_id, project_id, project_resource_id) WHERE project_resource_id IS NOT NULL;
