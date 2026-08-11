CREATE INDEX CONCURRENTLY IF NOT EXISTS idx_design_import_code_active
    ON design_import_code (workspace_id, expires_at)
    WHERE consumed_at IS NULL;
