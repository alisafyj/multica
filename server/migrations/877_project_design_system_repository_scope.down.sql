ALTER TABLE project_design_system
    DROP COLUMN IF EXISTS project_resource_id;

ALTER TABLE project_design_system
    ADD CONSTRAINT project_design_system_workspace_id_project_id_key
        UNIQUE (workspace_id, project_id);
