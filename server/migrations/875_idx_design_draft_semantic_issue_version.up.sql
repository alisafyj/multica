CREATE INDEX CONCURRENTLY idx_design_draft_semantic_issue_version
    ON design_draft(workspace_id, issue_id, version DESC)
    WHERE generation_mode = 'semantic_pagespec';
