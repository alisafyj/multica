-- This is intentionally a single statement: concurrent index creation cannot
-- run in a transaction or a multi-command migration.
CREATE UNIQUE INDEX CONCURRENTLY IF NOT EXISTS test_generation_plan_live_idx
    ON test_generation_plan (job_id)
    WHERE status IN ('draft', 'approved', 'dispatched');
