-- Which requirements a test case covers.
--
-- Until now the only structured path between the testing surface and an issue
-- was `test_run_case.defect_issue_id`, which records a failure that came OUT of
-- one execution. Nothing recorded what a case was written FOR, so neither
-- direction could be asked: "which cases cover MUL-123" had no answer, and an
-- issue could not show whether it was tested at all.
--
-- No FOREIGN KEY / cascade by repository rule: test_case_id and issue_id are
-- validated in application code, and both deletes sweep this table inside their
-- own transaction.
CREATE TABLE test_case_issue (
    test_case_id UUID NOT NULL,
    issue_id     UUID NOT NULL,
    workspace_id UUID NOT NULL,
    -- Who drew the link. 'ai' means a generation job proposed the case under a
    -- plan scoped to this issue; 'human' means someone linked it by hand. The
    -- distinction matters at review time: an AI-asserted coverage claim is
    -- exactly the kind of thing a reviewer needs to see flagged.
    origin       TEXT NOT NULL DEFAULT 'human'
                 CHECK (origin IN ('ai', 'human')),
    created_by   UUID,
    created_at   TIMESTAMPTZ NOT NULL DEFAULT now(),
    PRIMARY KEY (test_case_id, issue_id)
);
