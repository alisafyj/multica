-- This is intentionally a single statement: concurrent index creation cannot
-- run in a transaction or a multi-command migration.
CREATE UNIQUE INDEX CONCURRENTLY IF NOT EXISTS test_capability_key_idx
    ON test_capability (workspace_id, daemon_id, capability_key);
