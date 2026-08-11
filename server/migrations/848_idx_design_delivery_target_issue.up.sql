CREATE INDEX CONCURRENTLY idx_design_delivery_target_issue
    ON design_delivery(workspace_id, target_issue_id, delivered_at DESC);
