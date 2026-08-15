-- Design Document persistence (P-011 / DC-042).
--
-- Two invariants shape every query here:
--   1. Revisions are immutable. There is no UPDATE on
--      design_document_revision — an adjustment inserts a new row pointing at
--      its base.
--   2. draft and saved are pointers, and moving one is an atomic pointer
--      move, never a content copy. Saving cannot half-apply.

-- name: CreateDesignDocument :one
INSERT INTO design_document (
    workspace_id,
    project_id,
    project_resource_id,
    issue_id,
    title,
    platform,
    recipe,
    current_agent_id,
    active_task_id,
    active_operation,
    input_snapshot,
    created_by
)
SELECT
    sqlc.arg('workspace_id'),
    sqlc.arg('project_id'),
    sqlc.narg('project_resource_id'),
    sqlc.narg('issue_id'),
    sqlc.arg('title'),
    sqlc.arg('platform'),
    sqlc.arg('recipe'),
    sqlc.narg('current_agent_id'),
    sqlc.narg('active_task_id'),
    sqlc.narg('active_operation'),
    sqlc.arg('input_snapshot'),
    sqlc.narg('created_by')
FROM project
WHERE project.id = sqlc.arg('project_id')
  AND project.workspace_id = sqlc.arg('workspace_id')
RETURNING *;

-- name: GetDesignDocumentInWorkspace :one
SELECT * FROM design_document
WHERE id = sqlc.arg('id')
  AND workspace_id = sqlc.arg('workspace_id');

-- name: GetDesignDocumentInWorkspaceForUpdate :one
SELECT * FROM design_document
WHERE id = sqlc.arg('id')
  AND workspace_id = sqlc.arg('workspace_id')
FOR UPDATE;

-- A project holds many documents (DC-042). Most recently touched first,
-- which is the order the project tab lists them in.
-- name: ListDesignDocumentsByProject :many
SELECT * FROM design_document
WHERE workspace_id = sqlc.arg('workspace_id')
  AND project_id = sqlc.arg('project_id')
ORDER BY updated_at DESC;

-- name: GetDesignDocumentByActiveTask :one
SELECT * FROM design_document
WHERE workspace_id = sqlc.arg('workspace_id')
  AND active_task_id = sqlc.arg('active_task_id');

-- name: UpdateDesignDocumentActiveTask :one
UPDATE design_document SET
    current_agent_id = sqlc.narg('current_agent_id'),
    active_task_id = sqlc.narg('active_task_id'),
    active_operation = sqlc.narg('active_operation'),
    input_snapshot = sqlc.arg('input_snapshot'),
    last_error = NULL,
    updated_at = now()
WHERE id = sqlc.arg('id')
  AND workspace_id = sqlc.arg('workspace_id')
RETURNING *;

-- A failed task must leave the previous draft and saved pointers untouched:
-- a failure never degrades what the user already has (DC-034).
-- name: SetDesignDocumentFailure :one
UPDATE design_document SET
    active_task_id = NULL,
    active_operation = NULL,
    last_error = sqlc.arg('last_error'),
    updated_at = now()
WHERE id = sqlc.arg('id')
  AND workspace_id = sqlc.arg('workspace_id')
RETURNING *;

-- name: ClearDesignDocumentActiveTask :one
UPDATE design_document SET
    active_task_id = NULL,
    active_operation = NULL,
    updated_at = now()
WHERE id = sqlc.arg('id')
  AND workspace_id = sqlc.arg('workspace_id')
RETURNING *;

-- Revision numbers are per document and gapless from 1. Callers hold the
-- document row lock (GetDesignDocumentInWorkspaceForUpdate) so two concurrent
-- tasks cannot claim the same number.
-- name: GetNextDesignDocumentRevisionNumber :one
SELECT COALESCE(MAX(revision_number), 0) + 1 AS next_revision_number
FROM design_document_revision
WHERE design_document_id = sqlc.arg('design_document_id');

-- name: CreateDesignDocumentRevision :one
INSERT INTO design_document_revision (
    workspace_id,
    design_document_id,
    revision_number,
    package_schema,
    content_digest,
    archive_object_key,
    artifact_index,
    manifest,
    brief,
    coverage,
    audit,
    preview,
    input_snapshot_sha256,
    base_revision_id,
    design_system_digest,
    source_task_id,
    agent_id,
    instruction,
    scope
) VALUES (
    sqlc.arg('workspace_id'),
    sqlc.arg('design_document_id'),
    sqlc.arg('revision_number'),
    sqlc.arg('package_schema'),
    sqlc.arg('content_digest'),
    sqlc.arg('archive_object_key'),
    sqlc.arg('artifact_index'),
    sqlc.arg('manifest'),
    sqlc.arg('brief'),
    sqlc.arg('coverage'),
    sqlc.arg('audit'),
    sqlc.narg('preview'),
    sqlc.arg('input_snapshot_sha256'),
    sqlc.narg('base_revision_id'),
    sqlc.narg('design_system_digest'),
    sqlc.narg('source_task_id'),
    sqlc.narg('agent_id'),
    sqlc.narg('instruction'),
    sqlc.narg('scope')
)
RETURNING *;

-- name: GetDesignDocumentRevisionInWorkspace :one
SELECT * FROM design_document_revision
WHERE id = sqlc.arg('id')
  AND workspace_id = sqlc.arg('workspace_id');

-- name: ListDesignDocumentRevisions :many
SELECT * FROM design_document_revision
WHERE workspace_id = sqlc.arg('workspace_id')
  AND design_document_id = sqlc.arg('design_document_id')
ORDER BY revision_number DESC;

-- Move the draft pointer onto a freshly created revision. The revision id is
-- checked against the document so a revision from another document can never
-- become this one's draft.
-- name: SetDesignDocumentDraftRevision :one
UPDATE design_document SET
    draft_revision_id = sqlc.arg('draft_revision_id'),
    active_task_id = NULL,
    active_operation = NULL,
    last_error = NULL,
    updated_at = now()
WHERE design_document.id = sqlc.arg('id')
  AND design_document.workspace_id = sqlc.arg('workspace_id')
  AND EXISTS (
      SELECT 1 FROM design_document_revision
      WHERE design_document_revision.id = sqlc.arg('draft_revision_id')
        AND design_document_revision.design_document_id = sqlc.arg('id')
        AND design_document_revision.workspace_id = sqlc.arg('workspace_id')
  )
RETURNING *;

-- Saving is a pointer move: saved follows draft, atomically. Nothing is
-- copied, so the saved content cannot end up half-written. Guarded on the
-- caller's expected draft so a save cannot land on a draft that changed
-- underneath the user.
-- name: SaveDesignDocumentDraft :one
UPDATE design_document SET
    saved_revision_id = draft_revision_id,
    saved_at = now(),
    updated_at = now()
WHERE id = sqlc.arg('id')
  AND workspace_id = sqlc.arg('workspace_id')
  AND draft_revision_id IS NOT NULL
  AND draft_revision_id = sqlc.arg('expected_draft_revision_id')
RETURNING *;

-- Discarding a draft drops the pointer only. The revision row stays: it is
-- immutable history and may be another revision's base.
-- name: DiscardDesignDocumentDraft :one
UPDATE design_document SET
    draft_revision_id = NULL,
    last_error = NULL,
    updated_at = now()
WHERE id = sqlc.arg('id')
  AND workspace_id = sqlc.arg('workspace_id')
RETURNING *;

-- name: DeleteDesignDocument :exec
WITH deleted_revisions AS (
    DELETE FROM design_document_revision
    WHERE design_document_revision.workspace_id = sqlc.arg('workspace_id')
      AND design_document_revision.design_document_id = sqlc.arg('id')
    RETURNING design_document_revision.id
)
DELETE FROM design_document
WHERE design_document.id = sqlc.arg('id')
  AND design_document.workspace_id = sqlc.arg('workspace_id')
  AND (SELECT count(*) FROM deleted_revisions) >= 0;
