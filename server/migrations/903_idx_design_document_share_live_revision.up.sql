CREATE UNIQUE INDEX CONCURRENTLY IF NOT EXISTS idx_design_document_share_live_revision ON design_document_share (design_document_id, revision_id) WHERE revoked_at IS NULL;
