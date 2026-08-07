-- This is intentionally a single statement: concurrent index creation cannot
-- run in a transaction or a multi-command migration.
CREATE INDEX CONCURRENTLY IF NOT EXISTS test_run_project_idx
    ON test_run (workspace_id, project_id, created_at DESC);
