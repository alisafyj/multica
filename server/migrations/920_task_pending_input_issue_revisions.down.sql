ALTER TABLE task_pending_input
    DROP CONSTRAINT IF EXISTS task_pending_input_answer_revision_valid,
    DROP CONSTRAINT IF EXISTS task_pending_input_question_revision_positive,
    DROP COLUMN IF EXISTS answer_issue_revision,
    DROP COLUMN IF EXISTS question_issue_revision;
