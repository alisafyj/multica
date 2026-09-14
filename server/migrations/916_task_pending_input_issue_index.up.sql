CREATE INDEX CONCURRENTLY idx_task_pending_input_issue ON task_pending_input (workspace_id, issue_id, created_at DESC);
