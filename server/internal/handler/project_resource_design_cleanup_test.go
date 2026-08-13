package handler

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/google/uuid"
)

func TestDeleteProjectResourceCleansDesignRepoAnalysisInWorkspace(t *testing.T) {
	ctx := context.Background()
	foreignWorkspaceID := uuid.NewString()
	foreignProjectID := uuid.NewString()
	var projectID string
	if err := testPool.QueryRow(ctx, `
		INSERT INTO project (workspace_id, title)
		VALUES ($1, $2)
		RETURNING id
	`, testWorkspaceID, "project-resource-cleanup-"+uuid.NewString()).Scan(&projectID); err != nil {
		t.Fatalf("insert project: %v", err)
	}
	if _, err := testPool.Exec(ctx, `
		INSERT INTO workspace (id, name, slug, issue_prefix)
		VALUES ($1, $2, $3, $4)
	`, foreignWorkspaceID, "Foreign resource cleanup", "foreign-resource-cleanup-"+uuid.NewString(), "FRC"); err != nil {
		t.Fatalf("insert foreign workspace: %v", err)
	}
	if _, err := testPool.Exec(ctx, `
		INSERT INTO project (id, workspace_id, title)
		VALUES ($1, $2, $3)
	`, foreignProjectID, foreignWorkspaceID, "Foreign resource cleanup project"); err != nil {
		t.Fatalf("insert foreign project: %v", err)
	}

	var targetResourceID, unrelatedResourceID, foreignResourceID string
	if err := testPool.QueryRow(ctx, `
		INSERT INTO project_resource (project_id, workspace_id, resource_type, resource_ref, created_by)
		VALUES ($1, $2, 'github_repo', $3::jsonb, $4)
		RETURNING id
	`, projectID, testWorkspaceID, `{"url":"https://example.test/target.git"}`, testUserID).Scan(&targetResourceID); err != nil {
		t.Fatalf("insert target project resource: %v", err)
	}
	if err := testPool.QueryRow(ctx, `
		INSERT INTO project_resource (project_id, workspace_id, resource_type, resource_ref, created_by)
		VALUES ($1, $2, 'github_repo', $3::jsonb, $4)
		RETURNING id
	`, projectID, testWorkspaceID, `{"url":"https://example.test/unrelated.git"}`, testUserID).Scan(&unrelatedResourceID); err != nil {
		t.Fatalf("insert unrelated project resource: %v", err)
	}
	if err := testPool.QueryRow(ctx, `
		INSERT INTO project_resource (project_id, workspace_id, resource_type, resource_ref, created_by)
		VALUES ($1, $2, 'github_repo', $3::jsonb, $4)
		RETURNING id
	`, foreignProjectID, foreignWorkspaceID, `{"url":"https://example.test/foreign.git"}`, testUserID).Scan(&foreignResourceID); err != nil {
		t.Fatalf("insert foreign project resource: %v", err)
	}

	var targetAnalysisID, unrelatedAnalysisID, foreignAnalysisID string
	if err := testPool.QueryRow(ctx, `
		INSERT INTO design_repo_analysis (workspace_id, project_id, project_resource_id)
		VALUES ($1, $2, $3)
		RETURNING id
	`, testWorkspaceID, projectID, targetResourceID).Scan(&targetAnalysisID); err != nil {
		t.Fatalf("insert target analysis: %v", err)
	}
	if err := testPool.QueryRow(ctx, `
		INSERT INTO design_repo_analysis (workspace_id, project_id, project_resource_id)
		VALUES ($1, $2, $3)
		RETURNING id
	`, testWorkspaceID, projectID, unrelatedResourceID).Scan(&unrelatedAnalysisID); err != nil {
		t.Fatalf("insert unrelated analysis: %v", err)
	}
	if err := testPool.QueryRow(ctx, `
		INSERT INTO design_repo_analysis (workspace_id, project_id, project_resource_id)
		VALUES ($1, $2, $3)
		RETURNING id
	`, foreignWorkspaceID, foreignProjectID, foreignResourceID).Scan(&foreignAnalysisID); err != nil {
		t.Fatalf("insert foreign analysis: %v", err)
	}

	t.Cleanup(func() {
		_, _ = testPool.Exec(context.Background(), `DELETE FROM design_repo_analysis WHERE id IN ($1, $2, $3)`, targetAnalysisID, unrelatedAnalysisID, foreignAnalysisID)
		_, _ = testPool.Exec(context.Background(), `DELETE FROM project_resource WHERE id IN ($1, $2, $3)`, targetResourceID, unrelatedResourceID, foreignResourceID)
		_, _ = testPool.Exec(context.Background(), `DELETE FROM project WHERE id IN ($1, $2)`, projectID, foreignProjectID)
		_, _ = testPool.Exec(context.Background(), `DELETE FROM workspace WHERE id = $1`, foreignWorkspaceID)
	})

	w := httptest.NewRecorder()
	req := newRequest("DELETE", "/api/projects/"+projectID+"/resources/"+targetResourceID, nil)
	req = withURLParams(req, "id", projectID, "resourceId", targetResourceID)
	testHandler.DeleteProjectResource(w, req)
	if w.Code != http.StatusNoContent {
		t.Fatalf("DeleteProjectResource: expected 204, got %d: %s", w.Code, w.Body.String())
	}

	requireProjectResourceCleanupRowCount(t, "project_resource", targetResourceID, 0)
	requireProjectResourceCleanupRowCount(t, "design_repo_analysis", targetAnalysisID, 0)
	requireProjectResourceCleanupRowCount(t, "design_repo_analysis", unrelatedAnalysisID, 1)
	requireProjectResourceCleanupRowCount(t, "design_repo_analysis", foreignAnalysisID, 1)
}

func requireProjectResourceCleanupRowCount(t *testing.T, table, id string, want int) {
	t.Helper()
	var got int
	query := "SELECT count(*) FROM " + table + " WHERE id = $1"
	if err := testPool.QueryRow(context.Background(), query, id).Scan(&got); err != nil {
		t.Fatalf("count %s: %v", table, err)
	}
	if got != want {
		t.Fatalf("%s row count = %d, want %d", table, got, want)
	}
}
