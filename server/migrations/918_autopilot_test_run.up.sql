-- Autopilot execution mode "test_run": a fired autopilot creates a test round
-- from a plan and dispatches it to its agent (testing-center M6, TS-033).
-- The plan and the round's parallelism cap live on the autopilot; the run
-- row records which round it launched so the round's convergence can close
-- the run. No foreign keys (fork rule): the plan may be deleted later, and
-- dispatch then skips the run with a reason instead of failing a constraint.
ALTER TABLE autopilot DROP CONSTRAINT IF EXISTS autopilot_execution_mode_check;
ALTER TABLE autopilot ADD CONSTRAINT autopilot_execution_mode_check
    CHECK (execution_mode IN ('create_issue', 'run_only', 'test_run'));
ALTER TABLE autopilot ADD COLUMN IF NOT EXISTS test_plan_id UUID;
ALTER TABLE autopilot ADD COLUMN IF NOT EXISTS test_run_parallelism INT;
ALTER TABLE autopilot_run ADD COLUMN IF NOT EXISTS test_run_id UUID;
