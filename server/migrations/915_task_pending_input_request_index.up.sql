CREATE UNIQUE INDEX CONCURRENTLY idx_task_pending_input_request ON task_pending_input (task_id, claim_generation, request_key);
