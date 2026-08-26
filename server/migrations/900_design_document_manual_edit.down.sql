-- Any document mid-manual-edit would violate the narrower constraint, so the
-- pointer is released first. Draft and saved are untouched: a rollback must
-- not degrade what the user already has (DC-034).
UPDATE design_document
SET active_task_id = NULL, active_operation = NULL
WHERE active_operation = 'manual_edit';

ALTER TABLE design_document
    DROP CONSTRAINT IF EXISTS design_document_active_operation_check;

ALTER TABLE design_document
    ADD CONSTRAINT design_document_active_operation_check CHECK (
        active_operation IS NULL
        OR active_operation IN ('generate', 'adjust', 'regenerate')
    );
