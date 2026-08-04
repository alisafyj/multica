ALTER TABLE project_design_system_package
    DROP CONSTRAINT project_design_system_package_render_status_check,
    DROP COLUMN rendered_at,
    DROP COLUMN render_report,
    DROP COLUMN render_status;
