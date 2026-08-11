CREATE UNIQUE INDEX CONCURRENTLY idx_design_restore_plan_active_task
    ON design_restore_plan(restore_task_id)
    WHERE status IN ('draft', 'approved', 'dispatched');
