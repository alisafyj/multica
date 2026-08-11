-- This is intentionally a single statement: concurrent index creation cannot
-- run in a transaction or a multi-command migration.
CREATE INDEX CONCURRENTLY IF NOT EXISTS test_generation_job_project_status_idx
    ON test_generation_job (workspace_id, project_id, status);
