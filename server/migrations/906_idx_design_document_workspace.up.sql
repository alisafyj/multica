-- The design picker in the create-project modal lists every design document
-- in the workspace (ListDesignDocumentsInWorkspace), not just one project's.
-- Without this index the workspace-wide list is a sequential scan per open of
-- the picker; the per-project lists already have their own indexes (882/883).
CREATE INDEX CONCURRENTLY IF NOT EXISTS idx_design_document_workspace
ON design_document (workspace_id, updated_at DESC);
