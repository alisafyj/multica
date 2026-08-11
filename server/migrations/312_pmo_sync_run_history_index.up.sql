CREATE INDEX CONCURRENTLY pmo_sync_run_history_idx ON pmo_sync_run (workspace_id, config_id, created_at DESC);
