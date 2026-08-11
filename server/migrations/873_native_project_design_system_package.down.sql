ALTER TABLE project_design_system_package
    DROP CONSTRAINT project_design_system_package_schema_check,
    DROP COLUMN package_schema,
    DROP COLUMN archive_object_key,
    DROP COLUMN artifact_index,
    DROP COLUMN input_snapshot_sha256,
    DROP COLUMN base_package_sha256;
