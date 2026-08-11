-- This is intentionally a single statement: concurrent index creation cannot
-- run in a transaction or a multi-command migration.
CREATE INDEX CONCURRENTLY IF NOT EXISTS test_run_case_timeline_idx
    ON test_run_case (workspace_id, test_case_id, created_at DESC);
