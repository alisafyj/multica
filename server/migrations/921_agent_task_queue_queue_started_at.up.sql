ALTER TABLE agent_task_queue
ADD COLUMN queue_started_at TIMESTAMPTZ,
ALTER COLUMN queue_started_at SET DEFAULT now();
