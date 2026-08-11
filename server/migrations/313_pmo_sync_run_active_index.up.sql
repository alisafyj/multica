CREATE UNIQUE INDEX CONCURRENTLY pmo_sync_run_active_idx ON pmo_sync_run (workspace_id, config_id) WHERE status IN ('queued', 'running');
