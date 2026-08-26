CREATE UNIQUE INDEX CONCURRENTLY IF NOT EXISTS idx_design_document_revision_number ON design_document_revision (design_document_id, revision_number);
