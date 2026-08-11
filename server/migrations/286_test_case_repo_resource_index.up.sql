-- This is intentionally a single statement: concurrent index creation cannot
-- run in a transaction or a multi-command migration.
CREATE INDEX CONCURRENTLY IF NOT EXISTS test_case_repo_resource_idx
    ON test_case_repo (workspace_id, project_resource_id);
