-- This is intentionally a single statement: concurrent index creation cannot
-- run in a transaction or a multi-command migration.
CREATE INDEX CONCURRENTLY IF NOT EXISTS attachment_test_run_case_idx
    ON attachment (test_run_case_id) WHERE test_run_case_id IS NOT NULL;
