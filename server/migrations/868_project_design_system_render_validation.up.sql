ALTER TABLE project_design_system_package
    ADD COLUMN render_status TEXT NOT NULL DEFAULT 'pending',
    ADD COLUMN render_report JSONB NOT NULL DEFAULT '{}'::jsonb,
    ADD COLUMN rendered_at TIMESTAMPTZ,
    ADD CONSTRAINT project_design_system_package_render_status_check
        CHECK (render_status IN ('pending', 'passed', 'failed'));

UPDATE project_design_system_package
SET render_status = 'passed',
    render_report = '{"source":"pre_render_verification_saved_package"}'::jsonb,
    rendered_at = updated_at
WHERE slot = 'saved';
