ALTER TABLE pmo_sync_link DROP CONSTRAINT IF EXISTS pmo_sync_link_local_type_check;
ALTER TABLE pmo_sync_link ADD CONSTRAINT pmo_sync_link_local_type_check
    CHECK (local_type IN ('project', 'issue', 'member', 'agent'));
