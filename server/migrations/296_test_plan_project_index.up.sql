-- This is intentionally a single statement: concurrent index creation cannot
-- run in a transaction or a multi-command migration.
CREATE INDEX CONCURRENTLY IF NOT EXISTS test_plan_project_idx
    ON test_plan (workspace_id, project_id, status);
