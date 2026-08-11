ALTER TABLE IF EXISTS design_file
    DROP COLUMN IF EXISTS folder_id,
    DROP COLUMN IF EXISTS project_id;

DROP TABLE IF EXISTS design_folder;
