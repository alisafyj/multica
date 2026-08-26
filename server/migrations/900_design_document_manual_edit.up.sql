-- A designer's own edits are a fourth kind of run against a design document
-- (DC-062). They produce a revision like the others and pass the same Audit
-- and browser gate; what they skip is the agent, not the checks.
ALTER TABLE design_document
    DROP CONSTRAINT IF EXISTS design_document_active_operation_check;

ALTER TABLE design_document
    ADD CONSTRAINT design_document_active_operation_check CHECK (
        active_operation IS NULL
        OR active_operation IN ('generate', 'adjust', 'regenerate', 'manual_edit')
    );
