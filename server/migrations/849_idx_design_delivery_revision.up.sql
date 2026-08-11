CREATE INDEX CONCURRENTLY idx_design_delivery_revision
    ON design_delivery(revision_id, delivered_at DESC);
