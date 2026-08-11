CREATE INDEX CONCURRENTLY idx_open_design_run_design_system
    ON open_design_run(design_system_id, created_at DESC);
