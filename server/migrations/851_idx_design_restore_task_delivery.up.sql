CREATE INDEX CONCURRENTLY idx_design_restore_task_delivery
    ON design_restore_task(delivery_id, created_at DESC);
