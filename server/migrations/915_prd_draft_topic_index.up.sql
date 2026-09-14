CREATE UNIQUE INDEX CONCURRENTLY IF NOT EXISTS chat_prd_draft_topic_idx ON chat_prd_draft (workspace_id, installation_id, channel_chat_id, channel_thread_id);
