CREATE INDEX CONCURRENTLY IF NOT EXISTS idx_design_document_active_task ON design_document (active_task_id) WHERE active_task_id IS NOT NULL;
