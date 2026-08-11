CREATE INDEX CONCURRENTLY idx_design_revision_workspace ON design_revision(workspace_id, created_at DESC);
