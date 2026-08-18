ALTER TABLE project
    ADD CONSTRAINT project_created_by_required
    CHECK (created_by IS NOT NULL)
    NOT VALID;
