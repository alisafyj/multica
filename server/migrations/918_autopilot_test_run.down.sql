ALTER TABLE autopilot_run DROP COLUMN IF EXISTS test_run_id;
ALTER TABLE autopilot DROP COLUMN IF EXISTS test_run_parallelism;
ALTER TABLE autopilot DROP COLUMN IF EXISTS test_plan_id;
UPDATE autopilot SET execution_mode = 'run_only' WHERE execution_mode = 'test_run';
ALTER TABLE autopilot DROP CONSTRAINT IF EXISTS autopilot_execution_mode_check;
ALTER TABLE autopilot ADD CONSTRAINT autopilot_execution_mode_check
    CHECK (execution_mode IN ('create_issue', 'run_only'));
