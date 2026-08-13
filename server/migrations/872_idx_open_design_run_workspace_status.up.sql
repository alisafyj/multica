CREATE INDEX CONCURRENTLY idx_open_design_run_workspace_status
    ON open_design_run(workspace_id, status);
