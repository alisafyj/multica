-- Sweeping a deleted case's links is workspace-scoped, and so is every read
-- that starts from a case. The primary key leads with test_case_id but carries
-- no workspace column, so a tenant-scoped delete would otherwise scan.
--
-- This is intentionally a single statement: concurrent index creation cannot
-- run in a transaction or a multi-command migration.
CREATE INDEX CONCURRENTLY IF NOT EXISTS test_case_issue_workspace_case_idx
    ON test_case_issue (workspace_id, test_case_id);
