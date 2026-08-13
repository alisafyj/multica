ALTER TABLE project_design_system
    DROP CONSTRAINT project_design_system_active_operation_check;

ALTER TABLE project_design_system
    ADD CONSTRAINT project_design_system_active_operation_check CHECK (
        active_operation IS NULL
        OR active_operation IN ('generate', 'adjust', 'regenerate', 'repository_analysis')
    );
