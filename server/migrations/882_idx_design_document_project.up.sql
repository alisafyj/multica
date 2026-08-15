CREATE INDEX CONCURRENTLY IF NOT EXISTS idx_design_document_project ON design_document (workspace_id, project_id, updated_at DESC);
