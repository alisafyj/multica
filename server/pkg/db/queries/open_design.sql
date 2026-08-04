-- name: CreateOpenDesignRun :one
INSERT INTO open_design_run (
    id,
    workspace_id,
    project_id,
    design_system_id,
    task_id,
    operation,
    status,
    engine_release,
    engine_commit,
    engine_lockfile_sha256,
    engine_dist_sha256,
    agent_id,
    agent_snapshot,
    adapter_id,
    model,
    input_snapshot,
    workspace_provenance
) VALUES (
    sqlc.arg('id'),
    sqlc.arg('workspace_id'),
    sqlc.arg('project_id'),
    sqlc.arg('design_system_id'),
    sqlc.arg('task_id'),
    sqlc.arg('operation'),
    'preflight_pending',
    sqlc.arg('engine_release'),
    sqlc.arg('engine_commit'),
    sqlc.arg('engine_lockfile_sha256'),
    sqlc.arg('engine_dist_sha256'),
    sqlc.arg('agent_id'),
    sqlc.arg('agent_snapshot'),
    sqlc.arg('adapter_id'),
    sqlc.narg('model'),
    sqlc.arg('input_snapshot'),
    sqlc.arg('workspace_provenance')
)
RETURNING *;

-- name: StartOpenDesignRun :one
UPDATE open_design_run SET
    status = 'running',
    open_design_run_id = sqlc.arg('open_design_run_id'),
    started_at = now(),
    failure = '{}'::jsonb,
    updated_at = now()
WHERE task_id = sqlc.arg('task_id')
  AND status = 'ready'
  AND open_design_run_id IS NULL
RETURNING *;

-- name: AppendOpenDesignRunEvent :one
UPDATE open_design_run SET
    events = CASE
        WHEN EXISTS (
            SELECT 1
            FROM jsonb_array_elements(events) AS existing(value)
            WHERE existing.value ->> 'id' = (sqlc.arg('event_id')::bigint)::text
        ) THEN events
        ELSE events || jsonb_build_array(sqlc.arg('event')::jsonb)
    END,
    updated_at = now()
WHERE task_id = sqlc.arg('task_id')
  AND status = 'running'
  AND open_design_run_id = sqlc.arg('open_design_run_id')
  AND sqlc.arg('event_id')::bigint > 0
  AND jsonb_typeof(sqlc.arg('event')::jsonb) = 'object'
  AND sqlc.arg('event')::jsonb ->> 'id' = (sqlc.arg('event_id')::bigint)::text
  AND (
      EXISTS (
          SELECT 1
          FROM jsonb_array_elements(events) AS existing(value)
          WHERE existing.value ->> 'id' = (sqlc.arg('event_id')::bigint)::text
            AND existing.value = sqlc.arg('event')::jsonb
      )
      OR (
          NOT EXISTS (
              SELECT 1
              FROM jsonb_array_elements(events) AS existing(value)
              WHERE existing.value ->> 'id' = (sqlc.arg('event_id')::bigint)::text
          )
          AND NOT EXISTS (
              SELECT 1
              FROM jsonb_array_elements(events) AS existing(value)
              WHERE (existing.value ->> 'id')::bigint >= sqlc.arg('event_id')::bigint
          )
      )
  )
RETURNING *;

-- name: MarkOpenDesignRunSucceeded :one
UPDATE open_design_run SET
    status = 'run_succeeded',
    result_package = sqlc.arg('result_package'),
    artifact_index = sqlc.arg('artifact_index'),
    updated_at = now()
WHERE task_id = sqlc.arg('task_id')
  AND status = 'running'
  AND open_design_run_id = sqlc.arg('open_design_run_id')
  AND archive_object_key = sqlc.arg('archive_object_key')
  AND content_digest = sqlc.arg('content_digest')
  AND jsonb_typeof(sqlc.arg('result_package')::jsonb) = 'object'
  AND sqlc.arg('result_package')::jsonb ->> 'schema' = 'open-design.run-result-package.v1'
  AND jsonb_typeof(sqlc.arg('artifact_index')::jsonb) = 'array'
  AND jsonb_array_length(sqlc.arg('artifact_index')::jsonb) > 0
  AND sqlc.arg('content_digest')::text ~ '^sha256:[a-f0-9]{64}$'
RETURNING *;

-- name: RecordOpenDesignRunArchive :one
UPDATE open_design_run SET
    archive_object_key = COALESCE(archive_object_key, sqlc.arg('archive_object_key')),
    content_digest = COALESCE(content_digest, sqlc.arg('content_digest')),
    updated_at = now()
WHERE task_id = sqlc.arg('task_id')
  AND status = 'running'
  AND open_design_run_id = sqlc.arg('open_design_run_id')
  AND sqlc.arg('archive_object_key')::text <> ''
  AND sqlc.arg('content_digest')::text ~ '^sha256:[a-f0-9]{64}$'
  AND (archive_object_key IS NULL OR archive_object_key = sqlc.arg('archive_object_key'))
  AND (content_digest IS NULL OR content_digest = sqlc.arg('content_digest'))
RETURNING *;

-- name: RecordOpenDesignRunAudit :one
UPDATE open_design_run SET
    audit_report = sqlc.arg('audit_report'),
    status = CASE
        WHEN (sqlc.arg('audit_report')::jsonb -> 'audit' ->> 'ok')::boolean THEN status
        ELSE 'audit_failed'
    END,
    failure = CASE
        WHEN (sqlc.arg('audit_report')::jsonb -> 'audit' ->> 'ok')::boolean THEN failure
        ELSE sqlc.arg('failure')
    END,
    finished_at = CASE
        WHEN (sqlc.arg('audit_report')::jsonb -> 'audit' ->> 'ok')::boolean THEN finished_at
        ELSE now()
    END,
    updated_at = now()
WHERE task_id = sqlc.arg('task_id')
  AND status = 'run_succeeded'
  AND open_design_run_id = sqlc.arg('open_design_run_id')
  AND content_digest = sqlc.arg('content_digest')
  AND audit_report IS NULL
  AND jsonb_typeof(sqlc.arg('audit_report')::jsonb) = 'object'
  AND sqlc.arg('audit_report')::jsonb ->> 'schema' = 'multica.open-design-package-audit/v1'
  AND sqlc.arg('audit_report')::jsonb ->> 'content_digest' = sqlc.arg('content_digest')::text
  AND jsonb_typeof(sqlc.arg('audit_report')::jsonb -> 'audit') = 'object'
  AND jsonb_typeof(sqlc.arg('audit_report')::jsonb -> 'audit' -> 'ok') = 'boolean'
  AND (
      (sqlc.arg('audit_report')::jsonb -> 'audit' ->> 'ok')::boolean
      OR (
          jsonb_typeof(sqlc.arg('failure')::jsonb) = 'object'
          AND sqlc.arg('failure')::jsonb <> '{}'::jsonb
      )
  )
RETURNING *;

-- name: RecordOpenDesignRunPreview :one
UPDATE open_design_run SET
    preview_receipt = sqlc.arg('preview_receipt'),
    status = CASE
        WHEN (sqlc.arg('preview_receipt')::jsonb -> 'verification' ->> 'passed')::boolean THEN 'succeeded'
        ELSE 'preview_failed'
    END,
    failure = CASE
        WHEN (sqlc.arg('preview_receipt')::jsonb -> 'verification' ->> 'passed')::boolean THEN failure
        ELSE sqlc.arg('failure')
    END,
    finished_at = now(),
    updated_at = now()
WHERE task_id = sqlc.arg('task_id')
  AND status = 'run_succeeded'
  AND open_design_run_id = sqlc.arg('open_design_run_id')
  AND content_digest = sqlc.arg('content_digest')
  AND preview_receipt IS NULL
  AND audit_report ->> 'schema' = 'multica.open-design-package-audit/v1'
  AND jsonb_typeof(audit_report -> 'audit' -> 'ok') = 'boolean'
  AND (audit_report -> 'audit' ->> 'ok')::boolean
  AND jsonb_typeof(sqlc.arg('preview_receipt')::jsonb) = 'object'
  AND sqlc.arg('preview_receipt')::jsonb ->> 'schema' = 'multica.open-design-preview-verification/v1'
  AND sqlc.arg('preview_receipt')::jsonb ->> 'content_digest' = sqlc.arg('content_digest')::text
  AND sqlc.arg('preview_receipt')::jsonb -> 'engine' ->> 'schema' = 'multica.open-design-engine-identity/v1'
  AND sqlc.arg('preview_receipt')::jsonb -> 'engine' ->> 'release' = engine_release
  AND sqlc.arg('preview_receipt')::jsonb -> 'engine' ->> 'commit' = engine_commit
  AND sqlc.arg('preview_receipt')::jsonb -> 'engine' ->> 'lockfile_sha256' = engine_lockfile_sha256
  AND sqlc.arg('preview_receipt')::jsonb -> 'engine' ->> 'dist_sha256' = engine_dist_sha256
  AND jsonb_typeof(sqlc.arg('preview_receipt')::jsonb -> 'verification') = 'object'
  AND jsonb_typeof(sqlc.arg('preview_receipt')::jsonb -> 'verification' -> 'passed') = 'boolean'
  AND (
      (sqlc.arg('preview_receipt')::jsonb -> 'verification' ->> 'passed')::boolean
      OR (
          jsonb_typeof(sqlc.arg('failure')::jsonb) = 'object'
          AND sqlc.arg('failure')::jsonb <> '{}'::jsonb
      )
  )
RETURNING *;

-- name: PersistOpenDesignRunDraft :one
WITH eligible_run AS (
    SELECT run.*
    FROM open_design_run run
    WHERE run.task_id = sqlc.arg('task_id')
      AND run.status = 'run_succeeded'
      AND run.open_design_run_id = sqlc.arg('open_design_run_id')
      AND run.result_package = sqlc.arg('result_package')::jsonb
      AND run.result_package ->> 'schema' = 'open-design.run-result-package.v1'
      AND run.result_package -> 'run' ->> 'id' = sqlc.arg('open_design_run_id')::text
      AND run.artifact_index = sqlc.arg('artifact_index')::jsonb
      AND jsonb_typeof(run.artifact_index) = 'array'
      AND jsonb_array_length(run.artifact_index) > 0
      AND run.archive_object_key = sqlc.arg('archive_object_key')
      AND run.content_digest = sqlc.arg('content_digest')
      AND run.audit_report = sqlc.arg('audit_report')::jsonb
      AND run.audit_report ->> 'schema' = 'multica.open-design-package-audit/v1'
      AND run.audit_report ->> 'content_digest' = sqlc.arg('content_digest')::text
      AND (run.audit_report -> 'audit' ->> 'ok')::boolean
      AND run.audit_report -> 'engine' ->> 'release' = run.engine_release
      AND run.audit_report -> 'engine' ->> 'commit' = run.engine_commit
      AND run.audit_report -> 'engine' ->> 'lockfile_sha256' = run.engine_lockfile_sha256
      AND run.audit_report -> 'engine' ->> 'dist_sha256' = run.engine_dist_sha256
      AND run.preview_receipt IS NULL
      AND sqlc.arg('preview_receipt')::jsonb ->> 'schema' = 'multica.open-design-preview-verification/v1'
      AND sqlc.arg('preview_receipt')::jsonb ->> 'content_digest' = sqlc.arg('content_digest')::text
      AND (sqlc.arg('preview_receipt')::jsonb -> 'verification' ->> 'passed')::boolean
      AND sqlc.arg('preview_receipt')::jsonb -> 'engine' ->> 'release' = run.engine_release
      AND sqlc.arg('preview_receipt')::jsonb -> 'engine' ->> 'commit' = run.engine_commit
      AND sqlc.arg('preview_receipt')::jsonb -> 'engine' ->> 'lockfile_sha256' = run.engine_lockfile_sha256
      AND sqlc.arg('preview_receipt')::jsonb -> 'engine' ->> 'dist_sha256' = run.engine_dist_sha256
      AND EXISTS (
          SELECT 1
          FROM project_design_system system
          WHERE system.id = run.design_system_id
            AND system.workspace_id = run.workspace_id
            AND system.project_id = run.project_id
            AND system.active_task_id = run.task_id
            AND system.active_operation = run.operation
      )
      AND EXISTS (
          SELECT 1
          FROM agent_task_queue task
          WHERE task.id = run.task_id
            AND task.status = 'completed'
            AND task.agent_id = run.agent_id
      )
    FOR UPDATE
), updated_run AS (
    UPDATE open_design_run run SET
        preview_receipt = sqlc.arg('preview_receipt'),
        status = 'succeeded',
        failure = '{}'::jsonb,
        finished_at = now(),
        updated_at = now()
    FROM eligible_run eligible
    WHERE run.id = eligible.id
    RETURNING run.*
), upserted_draft AS (
    INSERT INTO project_design_system_package (
        design_system_id,
        slot,
        design_md,
        tokens_css,
        components_html,
        manifest,
        validation,
        integrity_sha256,
        source_task_id,
        agent_id,
        instruction,
        scope,
        render_status,
        render_report,
        rendered_at
    )
    SELECT
        run.design_system_id,
        'draft',
        sqlc.arg('design_md'),
        sqlc.arg('tokens_css'),
        sqlc.arg('components_html'),
        sqlc.arg('manifest'),
        sqlc.arg('validation'),
        sqlc.arg('integrity_sha256'),
        run.task_id,
        run.agent_id,
        sqlc.narg('instruction'),
        sqlc.narg('scope'),
        'passed',
        sqlc.arg('preview_receipt'),
        now()
    FROM updated_run run
    ON CONFLICT (design_system_id, slot) DO UPDATE SET
        design_md = EXCLUDED.design_md,
        tokens_css = EXCLUDED.tokens_css,
        components_html = EXCLUDED.components_html,
        manifest = EXCLUDED.manifest,
        validation = EXCLUDED.validation,
        integrity_sha256 = EXCLUDED.integrity_sha256,
        source_task_id = EXCLUDED.source_task_id,
        agent_id = EXCLUDED.agent_id,
        instruction = EXCLUDED.instruction,
        scope = EXCLUDED.scope,
        render_status = EXCLUDED.render_status,
        render_report = EXCLUDED.render_report,
        rendered_at = EXCLUDED.rendered_at,
        updated_at = now()
    RETURNING design_system_id
), cleared_system AS (
    UPDATE project_design_system system SET
        active_task_id = NULL,
        active_operation = NULL,
        last_error = NULL,
        updated_at = now()
    FROM upserted_draft draft
    WHERE system.id = draft.design_system_id
      AND system.active_task_id = sqlc.arg('task_id')
    RETURNING system.id
)
SELECT updated_run.*
FROM updated_run
WHERE EXISTS (SELECT 1 FROM cleared_system);

-- name: FinalizeOpenDesignRun :one
UPDATE open_design_run SET
    status = sqlc.arg('status'),
    failure = sqlc.arg('failure'),
    finished_at = now(),
    updated_at = now()
WHERE task_id = sqlc.arg('task_id')
  AND jsonb_typeof(sqlc.arg('failure')::jsonb) = 'object'
  AND sqlc.arg('failure')::jsonb <> '{}'::jsonb
  AND (
      (sqlc.arg('status') = 'canceled' AND status IN ('ready', 'running', 'run_succeeded'))
      OR (sqlc.arg('status') = 'agent_failed' AND status IN ('preflight_pending', 'ready', 'running'))
      OR (
          sqlc.arg('status') = 'audit_failed'
          AND status = 'run_succeeded'
          AND COALESCE(audit_report -> 'audit' ->> 'ok', 'false') <> 'true'
      )
      OR (sqlc.arg('status') = 'preview_failed' AND status = 'run_succeeded')
  )
RETURNING *;

-- name: GetOpenDesignRunByTask :one
SELECT * FROM open_design_run
WHERE task_id = sqlc.arg('task_id');

-- name: GetOpenDesignRunForEvidence :one
SELECT * FROM open_design_run
WHERE id = sqlc.arg('id')
  AND design_system_id = sqlc.arg('design_system_id')
  AND workspace_id = sqlc.arg('workspace_id');

-- name: RecordOpenDesignRunPreflight :one
UPDATE open_design_run SET
    status = sqlc.arg('status'),
    preflight = sqlc.arg('preflight'),
    failure = sqlc.arg('failure'),
    updated_at = now()
WHERE task_id = sqlc.arg('task_id')
  AND status = 'preflight_pending'
RETURNING *;
