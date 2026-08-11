CREATE INDEX CONCURRENTLY IF NOT EXISTS idx_design_import_code_expires_at
    ON design_import_code (expires_at);
