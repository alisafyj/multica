package handler

import (
	"net/http"
	"testing"

	"github.com/multica-ai/multica/server/internal/testutil"
)

func TestListDesignRepositoriesCatalogue(t *testing.T) {
	projectA := dbfx.Project(t, "Alpha project")
	projectB := dbfx.Project(t, "Beta project")
	resourceA2 := finderRepository(t, projectB, "zeta", `{"url":"https://github.com/example/zeta.git","default_branch_hint":"develop"}`)
	resourceA1 := finderRepository(t, projectA, "web", `{"url":"https://github.com/example/web.git","default_branch_hint":"main"}`)
	finderRepository(t, projectA, "ignored", `{}`)

	req := testutil.JSONRequest(http.MethodGet, "/api/design-repositories", nil)
	req.Header.Set("X-User-ID", testUserID)
	req.Header.Set("X-Workspace-ID", testWorkspaceID)
	resp := testutil.Call(t, testHandler.ListDesignRepositories, req)
	resp.Want(http.StatusOK)
	rows := resp.Map()["repositories"].([]any)
	if len(rows) != 2 {
		t.Fatalf("repository rows = %#v", rows)
	}
	first := rows[0].(map[string]any)
	second := rows[1].(map[string]any)
	if first["id"] != resourceA1 || second["id"] != resourceA2 {
		t.Fatalf("catalogue order = %#v", rows)
	}
	if first["project_title"] != "Alpha project" || first["label"] != "web" || first["repository_url"] != "https://github.com/example/web.git" || first["default_branch_hint"] != "main" {
		t.Fatalf("first catalogue row = %#v", first)
	}
	if second["project_id"] != projectB {
		t.Fatalf("second project = %#v", second)
	}
}

func TestListDesignRepositoriesHidesMalformedRepositories(t *testing.T) {
	project := dbfx.Project(t, "Malformed repository project")
	valid := finderRepository(t, project, "valid", `{"url":"https://github.com/example/valid","default_branch_hint":"main"}`)
	finderRepository(t, project, "malformed", `{"url":7}`)
	otherWorkspace, otherUser := createFinderWorkspace(t, "finder-isolation")
	otherProject := dbfx.Insert(t, "project", testutil.Cols{
		"workspace_id": otherWorkspace, "title": "Other workspace", "created_by": otherUser,
	})
	finderRepositoryForWorkspace(t, otherWorkspace, otherUser, otherProject, "cross", `{}`)

	req := testutil.JSONRequest(http.MethodGet, "/api/design-repositories", nil)
	req.Header.Set("X-User-ID", testUserID)
	req.Header.Set("X-Workspace-ID", testWorkspaceID)
	resp := testutil.Call(t, testHandler.ListDesignRepositories, req)
	resp.Want(http.StatusOK)
	rows := resp.Map()["repositories"].([]any)
	if len(rows) != 1 || rows[0].(map[string]any)["id"] != valid {
		t.Fatalf("catalogue = %#v", rows)
	}
}

func finderRepository(t *testing.T, projectID, label, ref string) string {
	t.Helper()
	return finderRepositoryForWorkspace(t, testWorkspaceID, testUserID, projectID, label, ref)
}

func finderRepositoryForWorkspace(t *testing.T, workspaceID, userID, projectID, label, ref string) string {
	t.Helper()
	return dbfx.Insert(t, "project_resource", testutil.Cols{
		"project_id": projectID, "workspace_id": workspaceID, "resource_type": "github_repo",
		"resource_ref": testutil.Raw(`'` + ref + `'::jsonb`), "label": label, "created_by": userID,
	})
}

func createFinderWorkspace(t *testing.T, slug string) (workspaceID string, userID string) {
	t.Helper()
	userID = dbfx.User(t, "Finder user", slug+"@example.test")
	workspaceID = dbfx.Workspace(t, "Finder workspace", slug, testutil.Cols{
		"issue_prefix": "FDR",
	})
	dbfx.Member(t, workspaceID, userID, "owner")
	return workspaceID, userID
}
