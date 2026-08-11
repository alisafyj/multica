ALTER TABLE project_design_system_package
    ADD COLUMN package_schema TEXT NOT NULL DEFAULT 'legacy',
    ADD COLUMN archive_object_key TEXT,
    ADD COLUMN artifact_index JSONB NOT NULL DEFAULT '[]'::jsonb,
    ADD COLUMN input_snapshot_sha256 TEXT,
    ADD COLUMN base_package_sha256 TEXT;

UPDATE project_design_system_package
SET package_schema = CASE
    WHEN manifest->>'schema_version' = 'multica.project-design-system/v1'
        THEN 'multica.project-design-system/v1'
    WHEN manifest->>'schema' = 'multica.open-design-draft-package/v1'
        THEN 'multica.open-design-draft-package/v1'
    ELSE 'legacy'
END;

ALTER TABLE project_design_system_package
    ADD CONSTRAINT project_design_system_package_schema_check
        CHECK (package_schema IN (
            'legacy',
            'multica.project-design-system/v1',
            'multica.open-design-draft-package/v1',
            'multica.project-design-system/v2'
        ));
