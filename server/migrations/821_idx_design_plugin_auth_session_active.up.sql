CREATE INDEX CONCURRENTLY IF NOT EXISTS idx_design_plugin_auth_session_active
    ON design_plugin_auth_session (provider, expires_at)
    WHERE consumed_at IS NULL;
