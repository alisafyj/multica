CREATE INDEX CONCURRENTLY IF NOT EXISTS idx_design_file_project_folder ON design_file(workspace_id, project_id, folder_id, updated_at DESC);
