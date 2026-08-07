-- This is intentionally a single statement: concurrent index creation cannot
-- run in a transaction or a multi-command migration.
CREATE UNIQUE INDEX CONCURRENTLY IF NOT EXISTS test_case_workspace_number_idx
    ON test_case (workspace_id, case_number);
