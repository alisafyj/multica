-- The reverse lookup: every case covering one issue, which is what the issue
-- detail's coverage block asks for. The primary key already serves the
-- case -> issues direction.
--
-- This is intentionally a single statement: concurrent index creation cannot
-- run in a transaction or a multi-command migration.
CREATE INDEX CONCURRENTLY IF NOT EXISTS test_case_issue_issue_idx
    ON test_case_issue (workspace_id, issue_id);
