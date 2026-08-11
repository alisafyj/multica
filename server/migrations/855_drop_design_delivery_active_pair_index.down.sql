CREATE UNIQUE INDEX CONCURRENTLY idx_design_delivery_active_pair
    ON design_delivery(workspace_id, source_issue_id, target_issue_id)
    WHERE status = 'active';
