CREATE UNIQUE INDEX CONCURRENTLY idx_task_run_evidence_task_attempt ON task_run_evidence (task_id, attempt);
