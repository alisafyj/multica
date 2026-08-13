package handler

import (
	"context"
	"net/http"
	"net/http/httptest"
	"reflect"
	"testing"
)

type projectDesignCleanupFixture struct {
	workspaceID string
	projectID   string
	designID    string
}

func TestDeleteProjectCleansProjectDesignData(t *testing.T) {
	if testHandler == nil || testPool == nil {
		t.Skip("database not available")
	}

	target := seedProjectDesignCleanupFixture(t, "project-design-cleanup-target", true)
	unrelated := seedProjectDesignCleanupFixture(t, "project-design-cleanup-unrelated", false)
	unrelatedBefore := projectDesignCleanupCounts(t, unrelated)

	w := httptest.NewRecorder()
	req := newRequest(http.MethodDelete, "/api/projects/"+target.projectID, nil)
	req.Header.Set("X-Workspace-ID", target.workspaceID)
	req = withURLParam(req, "id", target.projectID)
	testHandler.DeleteProject(w, req)
	if w.Code != http.StatusNoContent {
		t.Fatalf("DeleteProject: expected 204, got %d: %s", w.Code, w.Body.String())
	}

	assertProjectDesignRowCount(t, "project", `id = $1 AND workspace_id = $2`, target.projectID, target.workspaceID, 0)
	assertProjectDesignRowCount(t, "open_design_run", `project_id = $1 AND workspace_id = $2`, target.projectID, target.workspaceID, 0)
	assertProjectDesignRowCount(t, "project_design_system_package", `design_system_id = $1`, target.designID, "", 0)
	assertProjectDesignRowCount(t, "project_design_system", `project_id = $1 AND workspace_id = $2`, target.projectID, target.workspaceID, 0)
	assertProjectDesignRowCount(t, "design_repo_analysis", `project_id = $1 AND workspace_id = $2`, target.projectID, target.workspaceID, 0)
	assertProjectDesignRowCount(t, "design_folder", `project_id = $1 AND workspace_id = $2`, target.projectID, target.workspaceID, 0)

	assertProjectDesignRowCount(t, "design_file", `project_id = $1 AND workspace_id = $2`, target.projectID, target.workspaceID, 0)
	assertProjectDesignRowCount(t, "design_file", `project_id IS NULL AND folder_id IS NULL AND workspace_id = $1`, target.workspaceID, "", 1)
	assertProjectDesignRowCount(t, "design_delivery", `project_id = $1 AND workspace_id = $2`, target.projectID, target.workspaceID, 0)
	assertProjectDesignRowCount(t, "design_delivery", `project_id IS NULL AND workspace_id = $1`, target.workspaceID, "", 1)
	assertProjectDesignRowCount(t, "design_system_profile", `project_id = $1 AND workspace_id = $2`, target.projectID, target.workspaceID, 0)
	assertProjectDesignRowCount(t, "design_system_profile", `project_id IS NULL AND workspace_id = $1`, target.workspaceID, "", 1)

	assertProjectDesignRowCount(t, "workspace", `id = $1`, target.workspaceID, "", 1)
	assertProjectDesignRowCount(t, "workspace", `id = $1`, unrelated.workspaceID, "", 1)
	assertProjectDesignRowCount(t, "project_design_system", `id = $1 AND workspace_id = $2`, unrelated.designID, unrelated.workspaceID, 1)
	if unrelatedAfter := projectDesignCleanupCounts(t, unrelated); !reflect.DeepEqual(unrelatedAfter, unrelatedBefore) {
		t.Fatalf("unrelated workspace design rows changed: before=%v after=%v", unrelatedBefore, unrelatedAfter)
	}
}

func seedProjectDesignCleanupFixture(t *testing.T, slug string, owner bool) projectDesignCleanupFixture {
	t.Helper()

	ctx := context.Background()
	var staleWorkspaceID string
	if err := testPool.QueryRow(ctx, `SELECT id FROM workspace WHERE slug = $1`, slug).Scan(&staleWorkspaceID); err == nil {
		cleanupProjectDesignCleanupFixture(ctx, staleWorkspaceID)
	}

	var fixture projectDesignCleanupFixture
	if err := testPool.QueryRow(ctx, `
WITH
ws AS (
    INSERT INTO workspace (name, slug)
    VALUES ('Project design cleanup', $1)
    RETURNING id
),
project_row AS (
    INSERT INTO project (workspace_id, title)
    SELECT id, 'Project design cleanup project' FROM ws
    RETURNING id, workspace_id
),
folder_row AS (
    INSERT INTO design_folder (workspace_id, project_id, name)
    SELECT workspace_id, id, 'Project design cleanup folder' FROM project_row
    RETURNING id, workspace_id, project_id
),
file_row AS (
    INSERT INTO design_file (workspace_id, project_id, folder_id, title, source_type)
    SELECT workspace_id, project_id, id, 'Project design cleanup file', 'upload' FROM folder_row
    RETURNING id, workspace_id, project_id
),
revision_row AS (
    INSERT INTO design_revision (file_id, workspace_id, revision_number, status, native_json, validation_errors)
    SELECT file_row.id, file_row.workspace_id, 1, 'valid', '{}'::jsonb, '[]'::jsonb
    FROM file_row
    RETURNING id, workspace_id
),
source_issue_row AS (
    INSERT INTO issue (workspace_id, title, status, priority, creator_type, creator_id, project_id, number)
    SELECT ws.id, 'Project design cleanup source issue', 'todo', 'medium', 'member', $2, project_row.id, 1
    FROM ws CROSS JOIN project_row
    RETURNING id
),
target_issue_row AS (
    INSERT INTO issue (workspace_id, title, status, priority, creator_type, creator_id, project_id, number)
    SELECT ws.id, 'Project design cleanup target issue', 'todo', 'medium', 'member', $2, project_row.id, 2
    FROM ws CROSS JOIN project_row
    RETURNING id
),
resource_row AS (
    INSERT INTO project_resource (project_id, workspace_id, resource_type, resource_ref, label, created_by)
    SELECT project_row.id, project_row.workspace_id, 'local_directory', '{"localPath":"/tmp/project-design-cleanup"}'::jsonb, 'Repository root', $2
    FROM project_row
    RETURNING id
),
delivery_row AS (
    INSERT INTO design_delivery (
        workspace_id, project_id, source_issue_id, target_issue_id, file_id, revision_id
    )
    SELECT file_row.workspace_id, file_row.project_id, source_issue_row.id, target_issue_row.id, file_row.id, revision_row.id
    FROM file_row CROSS JOIN revision_row CROSS JOIN source_issue_row CROSS JOIN target_issue_row
),
profile_row AS (
    INSERT INTO design_system_profile (
        workspace_id, project_id, source_file_id, source_revision_id, name
    )
    SELECT file_row.workspace_id, file_row.project_id, file_row.id, revision_row.id, 'Project design cleanup profile'
    FROM file_row CROSS JOIN revision_row
),
repo_analysis_row AS (
    INSERT INTO design_repo_analysis (workspace_id, project_id, project_resource_id)
    SELECT project_row.workspace_id, project_row.id, resource_row.id
    FROM project_row CROSS JOIN resource_row
),
design_system_row AS (
    INSERT INTO project_design_system (workspace_id, project_id, name, platform)
    SELECT workspace_id, id, 'Project design cleanup system', 'web' FROM project_row
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
           repeat('c', 64), repeat('d', 64), '{}'::jsonb, 'test', '{}'::jsonb,
           '{}'::jsonb
    FROM design_system_row
)
SELECT ws.id, project_row.id, design_system_row.id
FROM ws CROSS JOIN project_row CROSS JOIN design_system_row
`, slug, testUserID).Scan(&fixture.workspaceID, &fixture.projectID, &fixture.designID); err != nil {
		t.Fatalf("seed project design cleanup fixture %q: %v", slug, err)
	}

	if owner {
		if _, err := testPool.Exec(ctx, `
INSERT INTO member (workspace_id, user_id, role) VALUES ($1, $2, 'owner')
`, fixture.workspaceID, testUserID); err != nil {
			t.Fatalf("seed project design cleanup owner %q: %v", slug, err)
		}
	}

	t.Cleanup(func() { cleanupProjectDesignCleanupFixture(context.Background(), fixture.workspaceID) })
	return fixture
}

func cleanupProjectDesignCleanupFixture(ctx context.Context, workspaceID string) {
	statements := []string{
		`DELETE FROM open_design_run WHERE workspace_id = $1`,
		`DELETE FROM project_design_system_package WHERE design_system_id IN (SELECT id FROM project_design_system WHERE workspace_id = $1)`,
		`DELETE FROM project_design_system WHERE workspace_id = $1`,
		`DELETE FROM design_repo_analysis WHERE workspace_id = $1`,
		`DELETE FROM design_delivery WHERE workspace_id = $1`,
		`DELETE FROM design_system_profile WHERE workspace_id = $1`,
		`DELETE FROM design_file WHERE workspace_id = $1`,
		`DELETE FROM design_folder WHERE workspace_id = $1`,
		`DELETE FROM member WHERE workspace_id = $1`,
		`DELETE FROM project WHERE workspace_id = $1`,
		`DELETE FROM workspace WHERE id = $1`,
	}
	for _, statement := range statements {
		_, _ = testPool.Exec(ctx, statement, workspaceID)
	}
}

func projectDesignCleanupCounts(t *testing.T, fixture projectDesignCleanupFixture) map[string]int {
	t.Helper()

	queries := map[string]string{
		"workspace":                     `SELECT count(*) FROM workspace WHERE id = $1`,
		"project":                       `SELECT count(*) FROM project WHERE id = $1`,
		"open_design_run":               `SELECT count(*) FROM open_design_run WHERE workspace_id = $1`,
		"project_design_system_package": `SELECT count(*) FROM project_design_system_package WHERE design_system_id = $1`,
		"project_design_system":         `SELECT count(*) FROM project_design_system WHERE workspace_id = $1`,
		"design_repo_analysis":          `SELECT count(*) FROM design_repo_analysis WHERE workspace_id = $1`,
		"design_folder":                 `SELECT count(*) FROM design_folder WHERE workspace_id = $1`,
		"design_file":                   `SELECT count(*) FROM design_file WHERE workspace_id = $1`,
		"design_delivery":               `SELECT count(*) FROM design_delivery WHERE workspace_id = $1`,
		"design_system_profile":         `SELECT count(*) FROM design_system_profile WHERE workspace_id = $1`,
	}
	counts := make(map[string]int, len(queries))
	for table, query := range queries {
		arg := fixture.workspaceID
		if table == "project" {
			arg = fixture.projectID
		} else if table == "project_design_system_package" {
			arg = fixture.designID
		}
		var count int
		if err := testPool.QueryRow(context.Background(), query, arg).Scan(&count); err != nil {
			t.Fatalf("count unrelated %s rows: %v", table, err)
		}
		counts[table] = count
	}
	return counts
}

func assertProjectDesignRowCount(t *testing.T, table, predicate, firstArg, secondArg string, want int) {
	t.Helper()

	args := []any{firstArg}
	if secondArg != "" {
		args = append(args, secondArg)
	}
	var got int
	if err := testPool.QueryRow(context.Background(), "SELECT count(*) FROM "+table+" WHERE "+predicate, args...).Scan(&got); err != nil {
		t.Fatalf("count %s rows: %v", table, err)
	}
	if got != want {
		t.Fatalf("%s rows matching %q = %d, want %d", table, predicate, got, want)
	}
}
