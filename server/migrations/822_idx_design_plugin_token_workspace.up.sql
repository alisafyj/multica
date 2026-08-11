CREATE INDEX CONCURRENTLY IF NOT EXISTS idx_design_plugin_token_workspace
    ON design_plugin_token (workspace_id, user_id, provider)
    WHERE revoked_at IS NULL;
