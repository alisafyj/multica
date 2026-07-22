-- Gallery Native design files and revisions

-- name: ListDesignFolders :many
SELECT * FROM design_folder
WHERE workspace_id = $1
  AND project_id = $2
ORDER BY parent_id NULLS FIRST, position ASC, name ASC;

-- name: ListDesignFoldersInWorkspace :many
SELECT * FROM design_folder
WHERE workspace_id = $1
ORDER BY project_id, parent_id NULLS FIRST, position ASC, name ASC;

-- name: GetDesignFolderInProject :one
SELECT * FROM design_folder
WHERE id = $1 AND workspace_id = $2 AND project_id = $3;

-- name: CreateDesignFolder :one
INSERT INTO design_folder (workspace_id, project_id, parent_id, name, position, created_by)
VALUES ($1, $2, $3, $4, $5, $6)
RETURNING *;

-- name: ListDesignFiles :many
SELECT df.* FROM design_file df
WHERE df.workspace_id = $1
  AND COALESCE(df.source_ref->>'asset_type', '') NOT IN ('template', 'design_system')
  AND NOT EXISTS (
    SELECT 1
    FROM design_system_profile dsp
    WHERE dsp.source_file_id = df.id
      AND dsp.status <> 'archived'
  )
  AND NOT EXISTS (
    SELECT 1
    FROM design_template_revision dtr
    JOIN design_revision dr ON dr.id = dtr.design_revision_id
    WHERE dr.file_id = df.id
  )
ORDER BY updated_at DESC, created_at DESC;

-- name: ListDesignFilesByProject :many
SELECT df.* FROM design_file df
WHERE df.workspace_id = $1
  AND df.project_id = $2
  AND (sqlc.narg('folder_id')::uuid IS NULL OR df.folder_id = sqlc.narg('folder_id'))
  AND COALESCE(df.source_ref->>'asset_type', '') NOT IN ('template', 'design_system')
  AND NOT EXISTS (
    SELECT 1
    FROM design_system_profile dsp
    WHERE dsp.source_file_id = df.id
      AND dsp.status <> 'archived'
  )
  AND NOT EXISTS (
    SELECT 1
    FROM design_template_revision dtr
    JOIN design_revision dr ON dr.id = dtr.design_revision_id
    WHERE dr.file_id = df.id
  )
ORDER BY updated_at DESC, created_at DESC;

-- name: GetDesignFile :one
SELECT * FROM design_file
WHERE id = $1;

-- name: GetDesignFileInWorkspace :one
SELECT * FROM design_file
WHERE id = $1 AND workspace_id = $2;

-- name: GetDesignFileBySourceKeyForUpdate :one
SELECT * FROM design_file
WHERE workspace_id = $1
  AND project_id = $2
  AND folder_id IS NOT DISTINCT FROM sqlc.narg('folder_id')::uuid
  AND source_type = $3
  AND source_ref->>'source_key' = sqlc.arg('source_key')::text
FOR UPDATE;

-- name: CreateDesignFile :one
INSERT INTO design_file (workspace_id, project_id, folder_id, title, description, source_type, source_ref, created_by)
VALUES ($1, $2, $3, $4, $5, $6, $7, $8)
RETURNING *;

-- name: UpdateDesignFile :one
UPDATE design_file SET
    title = COALESCE(sqlc.narg('title'), title),
    description = sqlc.narg('description'),
    project_id = COALESCE(sqlc.narg('project_id'), project_id),
    folder_id = sqlc.narg('folder_id'),
    source_ref = COALESCE(sqlc.narg('source_ref'), source_ref),
    current_revision_id = sqlc.narg('current_revision_id'),
    updated_at = now()
WHERE id = $1 AND workspace_id = $2
RETURNING *;

-- name: DeleteDesignFile :exec
DELETE FROM design_file WHERE id = $1 AND workspace_id = $2;

-- name: ListDesignRevisions :many
SELECT id, file_id, workspace_id, revision_number, status, validation_errors, created_by, created_at FROM design_revision
WHERE file_id = $1
ORDER BY revision_number DESC;

-- name: ListDesignRevisionsWithNativeJSON :many
SELECT * FROM design_revision
WHERE file_id = $1
ORDER BY revision_number DESC;

-- name: GetDesignRevision :one
SELECT * FROM design_revision
WHERE id = $1;

-- name: GetDesignRevisionInWorkspace :one
SELECT * FROM design_revision
WHERE id = $1 AND workspace_id = $2;

-- name: CreateDesignRevision :one
INSERT INTO design_revision (
    file_id, workspace_id, revision_number, status, native_json, validation_errors, created_by
) VALUES (
    $1, $2, $3, $4, $5, $6, $7
)
RETURNING *;

-- name: GetNextDesignRevisionNumber :one
SELECT COALESCE(MAX(revision_number), 0)::int + 1 AS next_revision_number
FROM design_revision
WHERE file_id = $1;

-- name: SetDesignFileCurrentRevision :one
UPDATE design_file SET
    current_revision_id = $3,
    updated_at = now()
WHERE id = $1 AND workspace_id = $2
RETURNING *;

-- name: CreateDesignImportCode :one
INSERT INTO design_import_code (workspace_id, user_id, provider, code_hash, expires_at)
VALUES ($1, $2, $3, $4, $5)
RETURNING *;

-- name: GetValidDesignImportCodeByHashForUpdate :one
SELECT * FROM design_import_code
WHERE code_hash = $1
  AND provider = $2
  AND consumed_at IS NULL
  AND expires_at > now()
FOR UPDATE;

-- name: ConsumeDesignImportCode :exec
UPDATE design_import_code
SET consumed_at = now()
WHERE id = $1;

-- name: MarkDesignImportCodeFailed :exec
UPDATE design_import_code
SET failed_attempts = failed_attempts + 1,
    last_failed_at = now()
WHERE code_hash = $1;

-- name: ListDesignAssets :many
SELECT * FROM design_asset
WHERE file_id = $1
ORDER BY created_at ASC;

-- name: UpsertDesignAsset :one
INSERT INTO design_asset (
    file_id, revision_id, workspace_id, asset_key, kind, url, content_type, size_bytes, metadata, created_by
) VALUES (
    $1, $2, $3, $4, $5, $6, $7, $8, $9, $10
)
ON CONFLICT (file_id, asset_key) DO UPDATE SET
    revision_id = EXCLUDED.revision_id,
    kind = EXCLUDED.kind,
    url = EXCLUDED.url,
    content_type = EXCLUDED.content_type,
    size_bytes = EXCLUDED.size_bytes,
    metadata = EXCLUDED.metadata
RETURNING *;

-- Gallery Native templates and slots

-- name: ListDesignTemplates :many
SELECT * FROM design_template
WHERE workspace_id = $1 OR (workspace_id IS NULL AND is_system = TRUE)
ORDER BY is_system DESC, category ASC, name ASC;

-- name: GetDesignTemplate :one
SELECT * FROM design_template
WHERE id = $1;

-- name: GetDesignTemplateByKey :one
SELECT * FROM design_template
WHERE (workspace_id = $1 OR (workspace_id IS NULL AND is_system = TRUE))
  AND key = $2
ORDER BY workspace_id NULLS LAST
LIMIT 1;

-- name: CreateDesignTemplate :one
INSERT INTO design_template (
    workspace_id, key, name, description, category, native_json, slot_schema, metadata, is_system, created_by
) VALUES (
    $1, $2, $3, $4, $5, $6, $7, $8, $9, $10
)
RETURNING *;

-- name: ListDesignTemplateSlots :many
SELECT * FROM design_template_slot
WHERE template_id = $1
ORDER BY position ASC, slot_key ASC;

-- name: UpsertDesignTemplateSlot :one
INSERT INTO design_template_slot (
    template_id, slot_key, label, slot_type, required, default_value, constraints, description, position
) VALUES (
    $1, $2, $3, $4, $5, $6, $7, $8, $9
)
ON CONFLICT (template_id, slot_key) DO UPDATE SET
    label = EXCLUDED.label,
    slot_type = EXCLUDED.slot_type,
    required = EXCLUDED.required,
    default_value = EXCLUDED.default_value,
    constraints = EXCLUDED.constraints,
    description = EXCLUDED.description,
    position = EXCLUDED.position
RETURNING *;

-- Gallery Native drafts and restore tasks

-- name: ListDesignDrafts :many
SELECT * FROM design_draft
WHERE workspace_id = $1
ORDER BY updated_at DESC, created_at DESC;

-- name: GetDesignDraftInWorkspace :one
SELECT * FROM design_draft
WHERE id = $1 AND workspace_id = $2;

-- name: CreateDesignDraft :one
INSERT INTO design_draft (
    workspace_id, template_id, catalog_template_id, template_revision_id, file_id, revision_id, issue_id, title,
    requirement_core, slot_values, patch, status, validation_errors, created_by
) VALUES (
    $1, $2, $3, $4, $5, $6, $7, $8, $9, $10, $11, $12, $13, $14
)
RETURNING *;

-- name: UpdateDesignDraft :one
UPDATE design_draft SET
    template_id = sqlc.narg('template_id'),
    catalog_template_id = sqlc.narg('catalog_template_id'),
    template_revision_id = sqlc.narg('template_revision_id'),
    file_id = sqlc.narg('file_id'),
    revision_id = sqlc.narg('revision_id'),
    generated_file_id = sqlc.narg('generated_file_id'),
    generated_revision_id = sqlc.narg('generated_revision_id'),
    issue_id = sqlc.narg('issue_id'),
    title = COALESCE(sqlc.narg('title'), title),
    requirement_core = COALESCE(sqlc.narg('requirement_core'), requirement_core),
    slot_values = COALESCE(sqlc.narg('slot_values'), slot_values),
    patch = COALESCE(sqlc.narg('patch'), patch),
    status = COALESCE(sqlc.narg('status'), status),
    validation_errors = COALESCE(sqlc.narg('validation_errors'), validation_errors),
    materialized_at = COALESCE(sqlc.narg('materialized_at'), materialized_at),
    updated_at = now()
WHERE id = $1 AND workspace_id = $2
RETURNING *;

-- Gallery Native design systems

-- name: ListDesignSystemProfiles :many
SELECT * FROM design_system_profile
WHERE workspace_id = $1
  AND (
    sqlc.narg('project_id')::uuid IS NULL
    OR project_id = sqlc.narg('project_id')::uuid
  )
  AND status <> 'archived'
ORDER BY is_default DESC, updated_at DESC, created_at DESC;

-- name: GetDesignSystemProfileInWorkspace :one
SELECT * FROM design_system_profile
WHERE id = $1
  AND workspace_id = $2
  AND status <> 'archived';

-- name: GetDefaultDesignSystemProfileForProject :one
SELECT * FROM design_system_profile
WHERE workspace_id = $1
  AND project_id = $2
  AND is_default = true
  AND status = 'analyzed'
ORDER BY updated_at DESC
LIMIT 1;

-- name: CreateDesignSystemProfile :one
INSERT INTO design_system_profile (
    workspace_id, project_id, source_file_id, source_revision_id, name,
    description, status, is_default, profile_json, analysis_errors, created_by
) VALUES (
    $1, $2, $3, $4, $5,
    $6, $7, $8, $9, $10, $11
)
RETURNING *;

-- name: ClearDefaultDesignSystemProfilesForProject :exec
UPDATE design_system_profile
SET is_default = false,
    updated_at = now()
WHERE workspace_id = $1
  AND project_id = $2
  AND is_default = true;

-- name: SetDesignSystemProfileDefault :one
UPDATE design_system_profile
SET is_default = true,
    updated_at = now()
WHERE id = $1
  AND workspace_id = $2
  AND project_id = $3
  AND status = 'analyzed'
RETURNING *;

-- name: UpdateDesignSystemProfileAnalysis :one
UPDATE design_system_profile
SET status = $3,
    profile_json = $4,
    analysis_errors = $5,
    updated_at = now()
WHERE id = $1
  AND workspace_id = $2
RETURNING *;

-- name: CreateDesignRestoreTask :one
INSERT INTO design_restore_task (
    workspace_id, file_id, revision_id, issue_id, delivery_id, agent_task_id, status, input, result, error, created_by
) VALUES (
    $1, $2, $3, $4, $5, $6, $7, $8, $9, $10, $11
)
RETURNING *;

-- name: UpdateDesignRestoreTask :one
UPDATE design_restore_task SET
    status = COALESCE(sqlc.narg('status'), status),
    issue_id = COALESCE(sqlc.narg('issue_id'), issue_id),
    agent_task_id = COALESCE(sqlc.narg('agent_task_id'), agent_task_id),
    result = COALESCE(sqlc.narg('result'), result),
    error = sqlc.narg('error'),
    updated_at = now()
WHERE id = $1 AND workspace_id = $2
RETURNING *;

-- name: GetDesignRestoreTaskInWorkspace :one
SELECT * FROM design_restore_task
WHERE id = $1 AND workspace_id = $2;

-- name: GetDesignRestoreTaskByAgentTask :one
SELECT * FROM design_restore_task
WHERE agent_task_id = $1;

-- name: GetReusableDesignRestoreTaskByIssue :one
SELECT * FROM design_restore_task
WHERE workspace_id = $1
  AND issue_id = $2
  AND file_id = $3
  AND revision_id = $4
  AND delivery_id IS NULL
  AND status IN ('queued', 'running')
ORDER BY
  CASE
    WHEN agent_task_id IS NOT NULL THEN 0
    WHEN status = 'running' THEN 1
    ELSE 2
  END,
  created_at DESC
LIMIT 1;

-- name: GetReusableDesignRestoreTaskByDelivery :one
SELECT * FROM design_restore_task
WHERE workspace_id = $1
  AND delivery_id = $2
  AND status IN ('queued', 'running')
ORDER BY
  CASE
    WHEN agent_task_id IS NOT NULL THEN 0
    WHEN status = 'running' THEN 1
    ELSE 2
  END,
  created_at DESC
LIMIT 1;

-- name: ListDesignRestoreTasks :many
SELECT * FROM design_restore_task
WHERE workspace_id = $1
ORDER BY created_at DESC
LIMIT 50;

-- name: ListDesignRestoreMappings :many
SELECT * FROM design_restore_mapping
WHERE restore_task_id = $1
ORDER BY created_at ASC;

-- name: DeleteDesignRestoreMappingsByTask :exec
DELETE FROM design_restore_mapping
WHERE restore_task_id = $1 AND workspace_id = $2;

-- name: CreateDesignRestorePlan :one
INSERT INTO design_restore_plan (
    workspace_id, restore_task_id, status, plan, review_notes, created_by
) VALUES (
    $1, $2, $3, $4, $5, $6
)
RETURNING *;

-- name: GetDesignRestorePlanByTask :one
SELECT * FROM design_restore_plan
WHERE restore_task_id = $1 AND workspace_id = $2
  AND status IN ('draft', 'approved', 'dispatched')
ORDER BY updated_at DESC
LIMIT 1;

-- name: UpdateDesignRestorePlan :one
UPDATE design_restore_plan SET
    status = COALESCE(sqlc.narg('status'), status),
    plan = COALESCE(sqlc.narg('plan'), plan),
    review_notes = sqlc.narg('review_notes'),
    approved_by = COALESCE(sqlc.narg('approved_by'), approved_by),
    approved_at = COALESCE(sqlc.narg('approved_at'), approved_at),
    updated_at = now()
WHERE id = $1 AND workspace_id = $2
RETURNING *;

-- name: MarkDesignRestorePlanDispatched :one
UPDATE design_restore_plan SET
    status = 'dispatched',
    updated_at = now()
WHERE id = $1 AND workspace_id = $2 AND status = 'approved'
RETURNING *;

-- name: CreateDesignRepoAnalysis :one
INSERT INTO design_repo_analysis (
    workspace_id, project_id, project_resource_id, status, schema_version, source_fingerprint,
    framework, language, package_manager, app_type, routing, styling, directories,
    commands, boundaries, target_candidates, confidence, summary, raw_result, error, analyzed_at
) VALUES (
    $1, $2, $3, $4, $5, $6,
    $7, $8, $9, $10, $11, $12, $13,
    $14, $15, $16, $17, $18, $19, $20, $21
)
RETURNING *;

-- name: ListDesignRepoAnalysesByProject :many
SELECT * FROM design_repo_analysis
WHERE workspace_id = $1 AND project_id = $2
ORDER BY updated_at DESC
LIMIT 20;

-- name: GetDesignRepoAnalysisInWorkspace :one
SELECT * FROM design_repo_analysis
WHERE id = $1 AND workspace_id = $2;

-- name: GetLatestCompletedDesignRepoAnalysisForProject :one
SELECT * FROM design_repo_analysis
WHERE workspace_id = $1 AND project_id = $2 AND status = 'completed'
ORDER BY analyzed_at DESC NULLS LAST, updated_at DESC
LIMIT 1;

-- name: GetLatestCompletedDesignRepoAnalysisForResource :one
SELECT * FROM design_repo_analysis
WHERE workspace_id = $1 AND project_resource_id = $2 AND status = 'completed'
ORDER BY analyzed_at DESC NULLS LAST, updated_at DESC
LIMIT 1;

-- name: CreateDesignRestoreMapping :one
INSERT INTO design_restore_mapping (
    restore_task_id, workspace_id, layer_id, target_path, target_kind, confidence, metadata
) VALUES (
    $1, $2, $3, $4, $5, $6, $7
)
RETURNING *;

-- name: SupersedeActiveDesignDeliveries :exec
UPDATE design_delivery SET
    status = 'superseded',
    audit_metadata = audit_metadata || jsonb_strip_nulls(jsonb_build_object(
        'superseded_by_delivery_id', sqlc.arg('superseded_by_delivery_id')::uuid,
        'superseded_by_target_issue_id', sqlc.arg('superseded_by_target_issue_id')::uuid,
        'superseded_by_file_id', sqlc.arg('superseded_by_file_id')::uuid,
        'superseded_by_revision_id', sqlc.arg('superseded_by_revision_id')::uuid,
        'superseded_at', now()
    )),
    updated_at = now()
WHERE workspace_id = $1
  AND source_issue_id = $2
  AND status = 'active';

-- name: CreateDesignDelivery :one
INSERT INTO design_delivery (
    id, workspace_id, project_id, source_issue_id, target_issue_id, file_id, revision_id,
    scope, status, delivered_by
) VALUES (
    $1, $2, $3, $4, $5, $6, $7, $8, $9, $10
)
RETURNING *;

-- name: GetDesignDeliveryInWorkspace :one
SELECT * FROM design_delivery
WHERE id = $1 AND workspace_id = $2;

-- name: CancelDesignDelivery :one
UPDATE design_delivery SET
    status = 'cancelled',
    cancelled_by = sqlc.narg('cancelled_by'),
    cancelled_at = now(),
    cancel_reason = NULLIF(btrim(sqlc.narg('cancel_reason')::text), ''),
    audit_metadata = jsonb_strip_nulls(jsonb_build_object(
        'cancel_reason', NULLIF(btrim(sqlc.narg('cancel_reason')::text), ''),
        'cancelled_by', sqlc.narg('cancelled_by')::uuid,
        'cancelled_at', now()
    )),
    updated_at = now()
WHERE id = $1
  AND workspace_id = $2
  AND status = 'active'
RETURNING *;

-- name: ListDesignDeliveriesByIssue :many
SELECT * FROM design_delivery
WHERE workspace_id = $1
  AND (source_issue_id = $2 OR target_issue_id = $2)
ORDER BY
  CASE status
    WHEN 'active' THEN 0
    WHEN 'superseded' THEN 1
    ELSE 2
  END,
  delivered_at DESC;

-- name: GetLatestActiveDesignDeliveryBySourceIssue :one
SELECT * FROM design_delivery
WHERE workspace_id = $1
  AND source_issue_id = $2
  AND status = 'active'
ORDER BY delivered_at DESC
LIMIT 1;

-- name: EnsureDesignTemplateLibrary :one
INSERT INTO design_template_library (workspace_id, key, name, description, metadata, created_by)
VALUES ($1, $2, $3, $4, $5, $6)
ON CONFLICT (workspace_id, key) DO UPDATE SET
    name = EXCLUDED.name,
    description = COALESCE(design_template_library.description, EXCLUDED.description),
    updated_at = now()
RETURNING *;

-- name: GetDesignTemplateLibraryByKey :one
SELECT * FROM design_template_library
WHERE workspace_id = $1 AND key = $2;

-- name: CreateDesignCatalogTemplate :one
INSERT INTO design_catalog_template (
    workspace_id, library_id, key, name, description, category, metadata, created_by
) VALUES (
    $1, $2, $3, $4, $5, $6, $7, $8
)
RETURNING *;

-- name: GetDesignCatalogTemplateByKey :one
SELECT * FROM design_catalog_template
WHERE workspace_id = $1 AND library_id = $2 AND key = $3;

-- name: CreateDesignTemplateRevision :one
INSERT INTO design_template_revision (
    workspace_id, template_id, design_revision_id, revision_number, status, slot_schema, metadata, created_by
) VALUES (
    $1, $2, $3, $4, $5, $6, $7, $8
)
RETURNING *;

-- name: GetDesignTemplateRevisionInWorkspace :one
SELECT * FROM design_template_revision
WHERE id = $1 AND workspace_id = $2;

-- name: GetNextDesignTemplateRevisionNumber :one
SELECT COALESCE(MAX(revision_number), 0)::int + 1 AS next_revision_number
FROM design_template_revision
WHERE template_id = $1;

-- name: UpdateDesignCatalogTemplateCurrentRevision :one
UPDATE design_catalog_template
SET current_revision_id = $3, updated_at = now()
WHERE id = $1 AND workspace_id = $2
RETURNING *;

-- name: ListDesignCatalogTemplates :many
SELECT
    t.id,
    t.workspace_id,
    t.library_id,
    t.key,
    t.name,
    t.description,
    t.category,
    t.current_revision_id,
    t.metadata,
    t.created_by,
    t.created_at,
    t.updated_at,
    tr.design_revision_id,
    tr.revision_number AS template_revision_number,
    tr.slot_schema AS slot_schema,
    dr.file_id AS design_file_id,
    df.title AS design_file_title
FROM design_catalog_template t
LEFT JOIN design_template_revision tr ON tr.id = t.current_revision_id
LEFT JOIN design_revision dr ON dr.id = tr.design_revision_id
LEFT JOIN design_file df ON df.id = dr.file_id
WHERE t.workspace_id = $1
  AND ($2::uuid IS NULL OR t.library_id = $2)
  AND ($3::text = '' OR t.category = $3)
ORDER BY t.updated_at DESC, t.created_at DESC;

-- name: GetDesignCatalogTemplate :one
SELECT
    t.id,
    t.workspace_id,
    t.library_id,
    t.key,
    t.name,
    t.description,
    t.category,
    t.current_revision_id,
    t.metadata,
    t.created_by,
    t.created_at,
    t.updated_at,
    tr.design_revision_id,
    tr.revision_number AS template_revision_number,
    tr.slot_schema AS slot_schema,
    dr.file_id AS design_file_id,
    df.title AS design_file_title
FROM design_catalog_template t
LEFT JOIN design_template_revision tr ON tr.id = t.current_revision_id
LEFT JOIN design_revision dr ON dr.id = tr.design_revision_id
LEFT JOIN design_file df ON df.id = dr.file_id
WHERE t.id = $1 AND t.workspace_id = $2;

-- Semantic design generation assets

-- name: CreateDesignTemplateBlueprint :one
INSERT INTO design_template_blueprint (
    workspace_id, template_id, template_revision_id, source_revision_id,
    analysis_version, schema_version, status, structure_json, blueprint_json,
	validation_errors, created_by
)
SELECT
	sqlc.arg('workspace_id'), sqlc.arg('template_id'), sqlc.arg('template_revision_id'), sqlc.arg('source_revision_id'),
	sqlc.arg('analysis_version'), sqlc.arg('schema_version'), sqlc.arg('status'), sqlc.arg('structure_json'), sqlc.arg('blueprint_json'),
	sqlc.arg('validation_errors'), sqlc.narg('created_by')
WHERE EXISTS (
	SELECT 1 FROM project p
	WHERE p.id = sqlc.arg('target_project_id')
	  AND p.workspace_id = sqlc.arg('workspace_id')
)
AND EXISTS (
	SELECT 1 FROM design_template_revision dtr
	WHERE dtr.id = sqlc.arg('template_revision_id')
	  AND dtr.workspace_id = sqlc.arg('workspace_id')
	  AND dtr.template_id = sqlc.arg('template_id')
	  AND dtr.design_revision_id = sqlc.arg('source_revision_id')
)
AND EXISTS (
	SELECT 1 FROM design_catalog_template dct
	WHERE dct.id = sqlc.arg('template_id')
	  AND dct.workspace_id = sqlc.arg('workspace_id')
)
AND EXISTS (
	SELECT 1 FROM design_revision dr
	WHERE dr.id = sqlc.arg('source_revision_id')
	  AND dr.workspace_id = sqlc.arg('workspace_id')
	  AND EXISTS (
		SELECT 1 FROM design_file df
		WHERE df.id = dr.file_id
		  AND df.workspace_id = sqlc.arg('workspace_id')
		  AND df.project_id = sqlc.arg('target_project_id')
	  )
)
RETURNING *;

-- name: GetLatestValidDesignTemplateBlueprint :one
SELECT * FROM design_template_blueprint
WHERE design_template_blueprint.workspace_id = sqlc.arg('workspace_id')
  AND design_template_blueprint.template_revision_id = sqlc.arg('template_revision_id')
  AND design_template_blueprint.status = 'valid'
  AND EXISTS (
	SELECT 1 FROM design_revision dr
	WHERE dr.id = design_template_blueprint.source_revision_id
	  AND dr.workspace_id = sqlc.arg('workspace_id')
	  AND EXISTS (
		SELECT 1 FROM design_file df
		WHERE df.id = dr.file_id
		  AND df.workspace_id = sqlc.arg('workspace_id')
		  AND df.project_id = sqlc.arg('target_project_id')
	  )
  )
ORDER BY analysis_version DESC
LIMIT 1;

-- name: CreateDesignComponentRecipeSet :one
INSERT INTO design_component_recipe_set (
    workspace_id, design_system_profile_id, source_revision_id,
    analysis_version, schema_version, status, recipes_json,
	validation_errors, created_by
)
SELECT
	sqlc.arg('workspace_id'), sqlc.arg('design_system_profile_id'), sqlc.arg('source_revision_id'),
	sqlc.arg('analysis_version'), sqlc.arg('schema_version'), sqlc.arg('status'), sqlc.arg('recipes_json'),
	sqlc.arg('validation_errors'), sqlc.narg('created_by')
WHERE EXISTS (
	SELECT 1 FROM project p
	WHERE p.id = sqlc.arg('target_project_id')
	  AND p.workspace_id = sqlc.arg('workspace_id')
)
AND EXISTS (
	SELECT 1 FROM design_system_profile dsp
	WHERE dsp.id = sqlc.arg('design_system_profile_id')
	  AND dsp.workspace_id = sqlc.arg('workspace_id')
	  AND dsp.source_revision_id = sqlc.arg('source_revision_id')
	  AND (dsp.project_id IS NULL OR dsp.project_id = sqlc.arg('target_project_id'))
	  AND EXISTS (
		SELECT 1 FROM design_revision dr
		WHERE dr.id = sqlc.arg('source_revision_id')
		  AND dr.workspace_id = sqlc.arg('workspace_id')
		  AND dr.file_id = dsp.source_file_id
	  )
)
RETURNING *;

-- name: GetLatestValidDesignComponentRecipeSet :one
SELECT * FROM design_component_recipe_set
WHERE design_component_recipe_set.workspace_id = sqlc.arg('workspace_id')
  AND design_component_recipe_set.design_system_profile_id = sqlc.arg('design_system_profile_id')
  AND design_component_recipe_set.status = 'valid'
  AND EXISTS (
	SELECT 1 FROM design_system_profile dsp
	WHERE dsp.id = design_component_recipe_set.design_system_profile_id
	  AND dsp.workspace_id = sqlc.arg('workspace_id')
	  AND (dsp.project_id IS NULL OR dsp.project_id = sqlc.arg('target_project_id'))
  )
ORDER BY analysis_version DESC
LIMIT 1;
