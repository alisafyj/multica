CREATE INDEX CONCURRENTLY idx_project_design_system_active_task
    ON project_design_system(active_task_id)
    WHERE active_task_id IS NOT NULL;
