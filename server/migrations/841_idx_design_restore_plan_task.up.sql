CREATE INDEX CONCURRENTLY idx_design_restore_plan_task ON design_restore_plan(restore_task_id, updated_at DESC);
