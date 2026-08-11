CREATE INDEX CONCURRENTLY idx_design_repo_analysis_project ON design_repo_analysis(workspace_id, project_id, updated_at DESC);
