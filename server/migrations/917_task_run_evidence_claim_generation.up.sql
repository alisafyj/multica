ALTER TABLE task_run_evidence
ADD COLUMN claim_generation BIGINT CHECK (claim_generation > 0);
