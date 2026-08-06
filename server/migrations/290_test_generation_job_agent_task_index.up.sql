-- This is intentionally a single statement: concurrent index creation cannot
-- run in a transaction or a multi-command migration.
CREATE INDEX CONCURRENTLY IF NOT EXISTS test_generation_job_agent_task_idx
    ON test_generation_job (agent_task_id);
