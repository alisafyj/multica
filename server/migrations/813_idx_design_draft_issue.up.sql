CREATE INDEX CONCURRENTLY idx_design_draft_issue ON design_draft(issue_id) WHERE issue_id IS NOT NULL;
