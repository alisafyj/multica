CREATE INDEX CONCURRENTLY idx_design_delivery_source_issue
    ON design_delivery(workspace_id, source_issue_id, delivered_at DESC);
