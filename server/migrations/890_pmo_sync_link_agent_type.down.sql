-- Agent mappings cannot be represented by the pre-890 member-only contract.
-- Keep the external link reviewable, but do not reinterpret an Agent UUID as a
-- member UUID before restoring the narrower constraint.
UPDATE pmo_sync_link
SET local_type = NULL,
    local_id = NULL,
    baseline_local = '{}'::jsonb
WHERE local_type = 'agent';

ALTER TABLE pmo_sync_link DROP CONSTRAINT IF EXISTS pmo_sync_link_local_type_check;
ALTER TABLE pmo_sync_link ADD CONSTRAINT pmo_sync_link_local_type_check
    CHECK (local_type IN ('project', 'issue', 'member'));
