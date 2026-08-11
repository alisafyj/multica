ALTER TABLE pmo_sync_config
    ADD CONSTRAINT pmo_sync_config_pkey PRIMARY KEY USING INDEX pmo_sync_config_id_idx;

ALTER TABLE pmo_sync_run
    ADD CONSTRAINT pmo_sync_run_pkey PRIMARY KEY USING INDEX pmo_sync_run_id_idx;

ALTER TABLE pmo_sync_link
    ADD CONSTRAINT pmo_sync_link_pkey PRIMARY KEY USING INDEX pmo_sync_link_id_idx;
