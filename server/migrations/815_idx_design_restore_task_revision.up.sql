CREATE INDEX CONCURRENTLY idx_design_restore_task_revision ON design_restore_task(revision_id, created_at DESC);
