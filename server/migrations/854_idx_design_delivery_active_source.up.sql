CREATE UNIQUE INDEX CONCURRENTLY idx_design_delivery_active_source
    ON design_delivery(workspace_id, source_issue_id)
    WHERE status = 'active';
