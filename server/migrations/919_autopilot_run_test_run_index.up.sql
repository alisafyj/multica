CREATE INDEX CONCURRENTLY IF NOT EXISTS idx_autopilot_run_test_run ON autopilot_run(test_run_id) WHERE test_run_id IS NOT NULL;
