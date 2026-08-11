CREATE INDEX CONCURRENTLY IF NOT EXISTS idx_design_folder_project ON design_folder(workspace_id, project_id, parent_id, position, name);
