CREATE UNIQUE INDEX CONCURRENTLY pmo_sync_run_agent_task_idx ON pmo_sync_run (agent_task_id) WHERE agent_task_id IS NOT NULL;
