CREATE UNIQUE INDEX CONCURRENTLY idx_task_run_evidence_task_attempt_generation ON task_run_evidence (task_id, attempt, claim_generation) NULLS NOT DISTINCT;
