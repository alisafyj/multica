package handler

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"
)

type workspaceDesignFixture struct {
	workspaceID string
	templateID  string
	designID    string
}

func seedWorkspaceDesignFixture(t *testing.T, slug string, owner bool) workspaceDesignFixture {
	t.Helper()

	ctx := context.Background()
	var staleWorkspaceID string
	if err := testPool.QueryRow(ctx, `SELECT id FROM workspace WHERE slug = $1`, slug).Scan(&staleWorkspaceID); err == nil {
		cleanupWorkspaceDesignFixture(ctx, staleWorkspaceID)
	}

	var fixture workspaceDesignFixture
	err := testPool.QueryRow(ctx, `
WITH
ws AS (
    INSERT INTO workspace (name, slug)
    VALUES ('Workspace design cleanup', $1)
    RETURNING id
),
project_row AS (
    INSERT INTO project (workspace_id, title)
    SELECT id, 'Workspace design cleanup project' FROM ws
    RETURNING id, workspace_id
),
resource_row AS (
    INSERT INTO project_resource (project_id, workspace_id, resource_type, resource_ref)
    SELECT id, workspace_id, 'github_repo', '{"url":"https://example.com/design-cleanup"}'::jsonb
    FROM project_row
    RETURNING id, project_id, workspace_id
),
issue_row AS (
    INSERT INTO issue (workspace_id, title, creator_type, creator_id)
    SELECT id, 'Workspace design cleanup issue', 'member', $2 FROM ws
    RETURNING id, workspace_id
),
folder_row AS (
    INSERT INTO design_folder (workspace_id, project_id, name)
    SELECT workspace_id, id, 'Design cleanup root folder' FROM project_row
    RETURNING id, workspace_id, project_id
),
folder_child AS (
    INSERT INTO design_folder (workspace_id, project_id, parent_id, name)
    SELECT workspace_id, project_id, id, 'Design cleanup child folder' FROM folder_row
    RETURNING id, workspace_id, project_id
),
file_row AS (
    INSERT INTO design_file (workspace_id, project_id, folder_id, title, source_type)
    SELECT workspace_id, project_id, id, 'Design cleanup file', 'upload' FROM folder_child
    RETURNING id, workspace_id
),
revision_row AS (
    INSERT INTO design_revision (file_id, workspace_id, revision_number, native_json)
    SELECT id, workspace_id, 1, '{}'::jsonb FROM file_row
    RETURNING id, file_id, workspace_id
),
file_current AS (
    UPDATE design_file
    SET current_revision_id = revision_row.id
    FROM revision_row
    WHERE design_file.id = revision_row.file_id
),
asset_row AS (
    INSERT INTO design_asset (file_id, revision_id, workspace_id, asset_key, kind, url)
    SELECT file_id, id, workspace_id, 'source', 'source', 's3://design-cleanup/source' FROM revision_row
),
template_row AS (
    INSERT INTO design_template (workspace_id, key, name, native_json)
    SELECT id, 'design-cleanup-template', 'Design cleanup template', '{}'::jsonb FROM ws
    RETURNING id, workspace_id
),
slot_row AS (
    INSERT INTO design_template_slot (template_id, slot_key, label, slot_type)
    SELECT id, 'title', 'Title', 'text' FROM template_row
),
library_row AS (
    INSERT INTO design_template_library (workspace_id, key, name)
    SELECT id, 'design-cleanup-library', 'Design cleanup library' FROM ws
    RETURNING id, workspace_id
),
catalog_row AS (
    INSERT INTO design_catalog_template (workspace_id, library_id, key, name)
    SELECT workspace_id, id, 'design-cleanup-catalog', 'Design cleanup catalog template'
    FROM library_row
    RETURNING id, workspace_id
),
template_revision_row AS (
    INSERT INTO design_template_revision (
        workspace_id, template_id, design_revision_id, revision_number
    )
    SELECT catalog_row.workspace_id, catalog_row.id, revision_row.id, 1
    FROM catalog_row CROSS JOIN revision_row
    RETURNING id, template_id, design_revision_id, workspace_id
),
catalog_current AS (
    UPDATE design_catalog_template
    SET current_revision_id = template_revision_row.id
    FROM template_revision_row
    WHERE design_catalog_template.id = template_revision_row.template_id
),
blueprint_row AS (
    INSERT INTO design_template_blueprint (
        workspace_id, template_id, template_revision_id, source_revision_id,
        analysis_version, schema_version, status, structure_json, blueprint_json
    )
    SELECT workspace_id, template_id, id, design_revision_id,
           1, '1.0', 'valid', '{}'::jsonb, '{}'::jsonb
    FROM template_revision_row
    RETURNING id, workspace_id
),
profile_row AS (
    INSERT INTO design_system_profile (
        workspace_id, project_id, source_file_id, source_revision_id, name
    )
    SELECT file_row.workspace_id, project_row.id, file_row.id, revision_row.id,
           'Design cleanup profile'
    FROM file_row CROSS JOIN revision_row CROSS JOIN project_row
    RETURNING id, workspace_id, source_revision_id
),
recipe_row AS (
    INSERT INTO design_component_recipe_set (
        workspace_id, design_system_profile_id, source_revision_id,
        analysis_version, schema_version, status, recipes_json
    )
    SELECT workspace_id, id, source_revision_id, 1, '1.0', 'valid', '{}'::jsonb
    FROM profile_row
    RETURNING id, workspace_id
),
draft_parent AS (
    INSERT INTO design_draft (
        workspace_id, template_id, file_id, revision_id, issue_id, title,
        catalog_template_id, template_revision_id, generated_file_id,
        generated_revision_id, materialized_at
    )
    SELECT ws.id, template_row.id, file_row.id, revision_row.id, issue_row.id,
           'Design cleanup draft', catalog_row.id, template_revision_row.id,
           file_row.id, revision_row.id, now()
    FROM ws CROSS JOIN template_row CROSS JOIN file_row CROSS JOIN revision_row
         CROSS JOIN issue_row CROSS JOIN catalog_row CROSS JOIN template_revision_row
         CROSS JOIN blueprint_row CROSS JOIN recipe_row
    RETURNING id, workspace_id
),
draft_child AS (
    INSERT INTO design_draft (workspace_id, title, parent_draft_id, version)
    SELECT workspace_id, 'Design cleanup child draft', id, 2 FROM draft_parent
),
delivery_row AS (
    INSERT INTO design_delivery (
        workspace_id, project_id, source_issue_id, target_issue_id, file_id, revision_id
    )
    SELECT ws.id, project_row.id, issue_row.id, issue_row.id, file_row.id, revision_row.id
    FROM ws CROSS JOIN project_row CROSS JOIN issue_row CROSS JOIN file_row CROSS JOIN revision_row
    RETURNING id, workspace_id, file_id, revision_id
),
restore_task_row AS (
    INSERT INTO design_restore_task (workspace_id, file_id, revision_id, delivery_id)
    SELECT workspace_id, file_id, revision_id, id FROM delivery_row
    RETURNING id, workspace_id
),
restore_mapping_row AS (
    INSERT INTO design_restore_mapping (
        restore_task_id, workspace_id, layer_id, target_path, target_kind
    )
    SELECT id, workspace_id, 'layer-1', 'components/example.tsx', 'component'
    FROM restore_task_row
),
restore_plan_row AS (
    INSERT INTO design_restore_plan (workspace_id, restore_task_id)
    SELECT workspace_id, id FROM restore_task_row
),
repo_analysis_row AS (
    INSERT INTO design_repo_analysis (workspace_id, project_id, project_resource_id)
    SELECT workspace_id, project_id, id FROM resource_row
),
import_code_row AS (
    INSERT INTO design_import_code (
        workspace_id, user_id, provider, code_hash, expires_at
    )
    SELECT id, $2, 'figma', encode(digest(id::text || '-import', 'sha256'), 'hex'), now() + interval '1 hour'
    FROM ws
),
auth_session_row AS (
    INSERT INTO design_plugin_auth_session (
        provider, user_code, user_id, workspace_id, expires_at
    )
    SELECT 'figma', substr(replace(id::text, '-', ''), 1, 8), $2, id, now() + interval '1 hour'
    FROM ws
),
plugin_token_row AS (
    INSERT INTO design_plugin_token (
        provider, token_hash, token_prefix, user_id, workspace_id
    )
    SELECT 'figma', encode(digest(id::text || '-token', 'sha256'), 'hex'), 'fig_test', $2, id
    FROM ws
),
design_system_row AS (
    INSERT INTO project_design_system (
        workspace_id, project_id, name, platform, current_agent_id,
        active_task_id, active_operation, input_snapshot
    )
    SELECT workspace_id, id, 'Design cleanup system', 'web', gen_random_uuid(),
           gen_random_uuid(), 'generate', '{}'::jsonb
    FROM project_row
    RETURNING id, workspace_id, project_id
),
package_row AS (
    INSERT INTO project_design_system_package (
        design_system_id, slot, design_md, tokens_css, components_html,
        manifest, validation, integrity_sha256
    )
    SELECT id, 'draft', '# Design', ':root {}', '<main></main>',
           '{}'::jsonb, '{}'::jsonb, repeat('a', 64)
    FROM design_system_row
),
open_run_row AS (
    INSERT INTO open_design_run (
        id, workspace_id, project_id, design_system_id, task_id, operation,
        status, engine_release, engine_commit, engine_lockfile_sha256,
        engine_dist_sha256, agent_snapshot, adapter_id, input_snapshot,
        workspace_provenance
    )
    SELECT gen_random_uuid(), workspace_id, project_id, id, gen_random_uuid(),
           'generate', 'preflight_pending', 'test', repeat('b', 40),
           repeat('c', 64), repeat('d', 64), '{}'::jsonb, 'test', '{}'::jsonb, '{}'::jsonb
    FROM design_system_row
)
SELECT ws.id, template_row.id, design_system_row.id
FROM ws CROSS JOIN template_row CROSS JOIN design_system_row
`, slug, testUserID).Scan(&fixture.workspaceID, &fixture.templateID, &fixture.designID)
	if err != nil {
		t.Fatalf("seed workspace design fixture %q: %v", slug, err)
	}

	if owner {
		if _, err := testPool.Exec(ctx, `
INSERT INTO member (workspace_id, user_id, role) VALUES ($1, $2, 'owner')
`, fixture.workspaceID, testUserID); err != nil {
			t.Fatalf("seed workspace owner %q: %v", slug, err)
		}
	}

	t.Cleanup(func() {
		cleanupWorkspaceDesignFixture(context.Background(), fixture.workspaceID)
	})
	return fixture
}

func cleanupWorkspaceDesignFixture(ctx context.Context, workspaceID string) {
	statements := []string{
		`DELETE FROM project_design_system_package WHERE design_system_id IN (SELECT id FROM project_design_system WHERE workspace_id = $1)`,
		`DELETE FROM design_template_slot WHERE template_id IN (SELECT id FROM design_template WHERE workspace_id = $1)`,
		`DELETE FROM design_restore_mapping WHERE workspace_id = $1`,
		`DELETE FROM design_restore_plan WHERE workspace_id = $1`,
		`DELETE FROM design_restore_task WHERE workspace_id = $1`,
		`DELETE FROM design_draft WHERE workspace_id = $1`,
		`DELETE FROM open_design_run WHERE workspace_id = $1`,
		`DELETE FROM design_template_blueprint WHERE workspace_id = $1`,
		`DELETE FROM design_component_recipe_set WHERE workspace_id = $1`,
		`DELETE FROM design_template_revision WHERE workspace_id = $1`,
		`DELETE FROM design_catalog_template WHERE workspace_id = $1`,
		`DELETE FROM design_template_library WHERE workspace_id = $1`,
		`DELETE FROM design_template WHERE workspace_id = $1`,
		`DELETE FROM design_delivery WHERE workspace_id = $1`,
		`DELETE FROM design_system_profile WHERE workspace_id = $1`,
		`DELETE FROM design_asset WHERE workspace_id = $1`,
		`DELETE FROM design_revision WHERE workspace_id = $1`,
		`DELETE FROM design_repo_analysis WHERE workspace_id = $1`,
		`DELETE FROM design_import_code WHERE workspace_id = $1`,
		`DELETE FROM design_plugin_auth_session WHERE workspace_id = $1`,
		`DELETE FROM design_plugin_token WHERE workspace_id = $1`,
		`DELETE FROM project_design_system WHERE workspace_id = $1`,
		`DELETE FROM design_file WHERE workspace_id = $1`,
		`DELETE FROM design_folder WHERE workspace_id = $1`,
		`DELETE FROM workspace WHERE id = $1`,
	}
	for _, statement := range statements {
		_, _ = testPool.Exec(ctx, statement, workspaceID)
	}
}

func designFixtureCounts(t *testing.T, fixture workspaceDesignFixture) map[string]int {
	t.Helper()

	queries := map[string]string{
		"design_restore_mapping": `SELECT count(*) FROM design_restore_mapping WHERE workspace_id = $1`,
		"design_restore_plan":    `SELECT count(*) FROM design_restore_plan WHERE workspace_id = $1`,
		"design_restore_task":    `SELECT count(*) FROM design_restore_task WHERE workspace_id = $1`,
		"design_draft":           `SELECT count(*) FROM design_draft WHERE workspace_id = $1`,
		"design_template_slot": `SELECT count(*)
FROM design_template_slot AS slot
JOIN design_template AS template ON template.id = slot.template_id
WHERE template.workspace_id = $1`,
		"design_template_blueprint":   `SELECT count(*) FROM design_template_blueprint WHERE workspace_id = $1`,
		"design_component_recipe_set": `SELECT count(*) FROM design_component_recipe_set WHERE workspace_id = $1`,
		"design_template_revision":    `SELECT count(*) FROM design_template_revision WHERE workspace_id = $1`,
		"design_catalog_template":     `SELECT count(*) FROM design_catalog_template WHERE workspace_id = $1`,
		"design_template_library":     `SELECT count(*) FROM design_template_library WHERE workspace_id = $1`,
		"design_template":             `SELECT count(*) FROM design_template WHERE workspace_id = $1`,
		"design_delivery":             `SELECT count(*) FROM design_delivery WHERE workspace_id = $1`,
		"design_system_profile":       `SELECT count(*) FROM design_system_profile WHERE workspace_id = $1`,
		"design_asset":                `SELECT count(*) FROM design_asset WHERE workspace_id = $1`,
		"design_revision":             `SELECT count(*) FROM design_revision WHERE workspace_id = $1`,
		"design_repo_analysis":        `SELECT count(*) FROM design_repo_analysis WHERE workspace_id = $1`,
		"design_import_code":          `SELECT count(*) FROM design_import_code WHERE workspace_id = $1`,
		"design_plugin_auth_session":  `SELECT count(*) FROM design_plugin_auth_session WHERE workspace_id = $1`,
		"design_plugin_token":         `SELECT count(*) FROM design_plugin_token WHERE workspace_id = $1`,
		"open_design_run":             `SELECT count(*) FROM open_design_run WHERE workspace_id = $1`,
		"project_design_system_package": `SELECT count(*)
FROM project_design_system_package AS package
JOIN project_design_system AS system ON system.id = package.design_system_id
WHERE system.workspace_id = $1`,
		"project_design_system": `SELECT count(*) FROM project_design_system WHERE workspace_id = $1`,
		"design_file":           `SELECT count(*) FROM design_file WHERE workspace_id = $1`,
		"design_folder":         `SELECT count(*) FROM design_folder WHERE workspace_id = $1`,
	}

	counts := make(map[string]int, len(queries))
	for table, query := range queries {
		var count int
		if err := testPool.QueryRow(context.Background(), query, fixture.workspaceID).Scan(&count); err != nil {
			t.Fatalf("count %s rows: %v", table, err)
		}
		counts[table] = count
	}
	return counts
}

func TestDeleteWorkspace_CleansDesignTablesAndPreservesOtherWorkspace(t *testing.T) {
	if testHandler == nil || testPool == nil {
		t.Skip("database not available")
	}

	target := seedWorkspaceDesignFixture(t, "handler-tests-design-cleanup-target", true)
	neighbor := seedWorkspaceDesignFixture(t, "handler-tests-design-cleanup-neighbor", false)

	for table, count := range designFixtureCounts(t, target) {
		if count == 0 {
			t.Fatalf("target fixture did not seed %s", table)
		}
	}
	neighborBefore := designFixtureCounts(t, neighbor)

	recorder := httptest.NewRecorder()
	request := newRequest(http.MethodDelete, "/api/workspaces/"+target.workspaceID, nil)
	request = withURLParam(request, "id", target.workspaceID)
	testHandler.DeleteWorkspace(recorder, request)
	if recorder.Code != http.StatusNoContent {
		t.Fatalf("DeleteWorkspace returned %d: %s", recorder.Code, recorder.Body.String())
	}

	for table, count := range designFixtureCounts(t, target) {
		if count != 0 {
			t.Errorf("target %s rows survived workspace delete: %d", table, count)
		}
	}
	neighborAfter := designFixtureCounts(t, neighbor)
	for table, before := range neighborBefore {
		if count := neighborAfter[table]; count != before {
			t.Errorf("neighbor %s rows changed: got %d, want %d", table, count, before)
		}
	}
}
