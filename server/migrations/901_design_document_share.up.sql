-- Durable share links for a design document revision (DC-062 item 5).
--
-- A share hands out the saved revision's prototype to someone without a
-- workspace account. The link itself never expires; only revocation kills
-- it. The archive bytes behind it are served through the short-lived preview
-- capability the public exchange endpoint re-issues per visit, so a
-- long-lived token never carries file access with it.
--
-- One live link per revision: the create endpoint is create-or-return, and
-- the partial unique index in 903 is the backstop behind that idempotence.
--
-- No foreign keys per repository policy: document, revision and creator are
-- resolved in application code, and cleanup rides the document delete path.
CREATE TABLE design_document_share (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    workspace_id UUID NOT NULL,
    design_document_id UUID NOT NULL,
    revision_id UUID NOT NULL,
    -- Raw random token (`mds_` + hex). Stored unhashed on purpose: the
    -- create endpoint is idempotent, so re-copying the link must show the
    -- same value a caller already holds.
    token TEXT NOT NULL,
    created_by UUID NOT NULL,
    created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    revoked_at TIMESTAMPTZ
);
