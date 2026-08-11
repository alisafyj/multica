CREATE UNIQUE INDEX CONCURRENTLY pmo_sync_link_identity_idx ON pmo_sync_link (workspace_id, config_id, external_type, external_key);
