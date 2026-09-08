ALTER TABLE task_pending_input
    ADD COLUMN question_issue_revision BIGINT,
    ADD COLUMN answer_issue_revision BIGINT,
    ADD CONSTRAINT task_pending_input_question_revision_positive
        CHECK (question_issue_revision IS NULL OR question_issue_revision > 0),
    ADD CONSTRAINT task_pending_input_answer_revision_valid
        CHECK (
            answer_issue_revision IS NULL OR
            (question_issue_revision IS NOT NULL AND answer_issue_revision > question_issue_revision)
        );
