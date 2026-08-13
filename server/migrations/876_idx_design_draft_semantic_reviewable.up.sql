CREATE INDEX CONCURRENTLY idx_design_draft_semantic_reviewable
    ON design_draft(workspace_id, issue_id, status, version DESC)
    WHERE generation_mode = 'semantic_pagespec'
      AND status IN ('generated', 'generated_with_warnings');
