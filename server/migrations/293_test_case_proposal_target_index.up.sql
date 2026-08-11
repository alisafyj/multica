-- This is intentionally a single statement: concurrent index creation cannot
-- run in a transaction or a multi-command migration.
CREATE INDEX CONCURRENTLY IF NOT EXISTS test_case_proposal_target_idx
    ON test_case_proposal (workspace_id, target_case_id, status);
