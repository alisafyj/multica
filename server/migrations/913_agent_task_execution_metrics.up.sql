ALTER TABLE agent_task_queue ADD COLUMN IF NOT EXISTS execution_metrics JSONB;
