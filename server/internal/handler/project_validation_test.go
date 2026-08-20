package handler

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/multica-ai/multica/server/internal/events"
	"github.com/multica-ai/multica/server/pkg/protocol"
)

// An unknown project status must fail fast with a 400 and the valid list, not
// surface the DB CHECK violation as a 500 (#3925: `--status active`).
func TestCreateProjectInvalidStatusReturns400(t *testing.T) {
	w := httptest.NewRecorder()
	req := newRequest("POST", "/api/projects?workspace_id="+testWorkspaceID, map[string]any{
		"title":  "invalid status project",
		"status": "active",
	})
	testHandler.CreateProject(w, req)
	if w.Code != http.StatusBadRequest {
		t.Fatalf("expected 400 for invalid status, got %d: %s", w.Code, w.Body.String())
	}
	if body := w.Body.String(); !strings.Contains(body, "planned") {
		t.Errorf("expected error to list valid statuses, got: %s", body)
	}
}

func TestCreateProjectInvalidPriorityReturns400(t *testing.T) {
	w := httptest.NewRecorder()
	req := newRequest("POST", "/api/projects?workspace_id="+testWorkspaceID, map[string]any{
		"title":    "invalid priority project",
		"priority": "critical",
	})
	testHandler.CreateProject(w, req)
	if w.Code != http.StatusBadRequest {
		t.Fatalf("expected 400 for invalid priority, got %d: %s", w.Code, w.Body.String())
	}
}

// A valid status still creates the project (the validation does not over-reject).
func TestCreateProjectValidStatusReturns201(t *testing.T) {
	w := httptest.NewRecorder()
	req := newRequest("POST", "/api/projects?workspace_id="+testWorkspaceID, map[string]any{
		"title":  "valid status project",
		"status": "in_progress",
	})
	testHandler.CreateProject(w, req)
	if w.Code != http.StatusCreated {
		t.Fatalf("expected 201 for valid status, got %d: %s", w.Code, w.Body.String())
	}
	var project ProjectResponse
	if err := json.NewDecoder(w.Body).Decode(&project); err != nil {
		t.Fatalf("decode CreateProject: %v", err)
	}
	t.Cleanup(func() {
		req := newRequest("DELETE", "/api/projects/"+project.ID, nil)
		req = withURLParam(req, "id", project.ID)
		testHandler.DeleteProject(httptest.NewRecorder(), req)
	})
	if project.Status != "in_progress" {
		t.Errorf("expected status in_progress, got %q", project.Status)
	}
}

// Updating to an unknown status is a 400, not a 500.
func TestUpdateProjectInvalidStatusReturns400(t *testing.T) {
	// Seed a project to update.
	w := httptest.NewRecorder()
	req := newRequest("POST", "/api/projects?workspace_id="+testWorkspaceID, map[string]any{
		"title": "update validation project",
	})
	testHandler.CreateProject(w, req)
	if w.Code != http.StatusCreated {
		t.Fatalf("seed CreateProject: %d %s", w.Code, w.Body.String())
	}
	var project ProjectResponse
	if err := json.NewDecoder(w.Body).Decode(&project); err != nil {
		t.Fatalf("decode CreateProject: %v", err)
	}
	t.Cleanup(func() {
		req := newRequest("DELETE", "/api/projects/"+project.ID, nil)
		req = withURLParam(req, "id", project.ID)
		testHandler.DeleteProject(httptest.NewRecorder(), req)
	})

	w = httptest.NewRecorder()
	req = newRequest("PUT", "/api/projects/"+project.ID, map[string]any{"status": "active"})
	req = withURLParam(req, "id", project.ID)
	testHandler.UpdateProject(w, req)
	if w.Code != http.StatusBadRequest {
		t.Fatalf("expected 400 for invalid update status, got %d: %s", w.Code, w.Body.String())
	}
}

func TestUpdateProjectCompletedMarksOpenIssuesDone(t *testing.T) {
	project := createProjectPermissionTestProject(t, "completed project cascades to issues")
	t.Cleanup(func() {
		_, _ = testPool.Exec(context.Background(), `DELETE FROM issue WHERE project_id = $1`, project.ID)
	})

	statuses := []string{"backlog", "todo", "in_progress", "in_review", "blocked", "cancelled", "done"}
	for _, status := range statuses {
		w := httptest.NewRecorder()
		req := newRequest("POST", "/api/issues?workspace_id="+testWorkspaceID, map[string]any{
			"title":      status,
			"status":     status,
			"project_id": project.ID,
		})
		testHandler.CreateIssue(w, req)
		if w.Code != http.StatusCreated {
			t.Fatalf("create %s issue: %d %s", status, w.Code, w.Body.String())
		}
	}

	updatedEvents := make(chan events.Event, len(statuses))
	testHandler.Bus.Subscribe(protocol.EventIssueUpdated, func(event events.Event) {
		payload, ok := event.Payload.(map[string]any)
		if !ok {
			return
		}
		issue, ok := payload["issue"].(IssueResponse)
		if !ok || issue.ProjectID == nil || *issue.ProjectID != project.ID {
			return
		}
		updatedEvents <- event
	})

	w := httptest.NewRecorder()
	req := newRequest("PUT", "/api/projects/"+project.ID, map[string]any{"status": "completed"})
	req = withURLParam(req, "id", project.ID)
	testHandler.UpdateProject(w, req)
	if w.Code != http.StatusOK {
		t.Fatalf("complete project: %d %s", w.Code, w.Body.String())
	}

	var updated ProjectResponse
	if err := json.NewDecoder(w.Body).Decode(&updated); err != nil {
		t.Fatalf("decode UpdateProject: %v", err)
	}
	if updated.Status != "completed" {
		t.Errorf("project status = %q, want completed", updated.Status)
	}
	if updated.IssueCount != int64(len(statuses)) || updated.DoneCount != int64(len(statuses)) {
		t.Errorf("project issue stats = %d/%d, want %d/%d", updated.DoneCount, updated.IssueCount, len(statuses), len(statuses))
	}

	if got := len(updatedEvents); got != 5 {
		t.Fatalf("issue:updated events = %d, want 5 changed issues", got)
	}
	seenPrevious := make(map[string]bool, 5)
	for len(updatedEvents) > 0 {
		event := <-updatedEvents
		payload := event.Payload.(map[string]any)
		issue := payload["issue"].(IssueResponse)
		if issue.Status != "done" || payload["status_changed"] != true {
			t.Errorf("issue event = status %q, status_changed %v", issue.Status, payload["status_changed"])
		}
		previous, _ := payload["prev_status"].(string)
		seenPrevious[previous] = true
	}
	for _, previous := range []string{"backlog", "todo", "in_progress", "in_review", "blocked"} {
		if !seenPrevious[previous] {
			t.Errorf("missing issue:updated event for previous status %q", previous)
		}
	}

	rows, err := testPool.Query(context.Background(), `SELECT title, status FROM issue WHERE project_id = $1`, project.ID)
	if err != nil {
		t.Fatalf("list project issues: %v", err)
	}
	defer rows.Close()
	for rows.Next() {
		var title, status string
		if err := rows.Scan(&title, &status); err != nil {
			t.Fatalf("scan project issue: %v", err)
		}
		want := "done"
		if title == "cancelled" {
			want = "cancelled"
		}
		if status != want {
			t.Errorf("%s issue status = %q, want %q", title, status, want)
		}
	}
	if err := rows.Err(); err != nil {
		t.Fatalf("iterate project issues: %v", err)
	}
}

func TestUpdateProjectUsesStatusReadUnderLockBeforeCompletingIssues(t *testing.T) {
	project := createProjectPermissionTestProject(t, "locked project status")
	t.Cleanup(func() {
		_, _ = testPool.Exec(context.Background(), `DELETE FROM issue WHERE project_id = $1`, project.ID)
	})

	w := httptest.NewRecorder()
	req := newRequest("PUT", "/api/projects/"+project.ID, map[string]any{"status": "completed"})
	req = withURLParam(req, "id", project.ID)
	testHandler.UpdateProject(w, req)
	if w.Code != http.StatusOK {
		t.Fatalf("complete project: %d %s", w.Code, w.Body.String())
	}

	w = httptest.NewRecorder()
	req = newRequest("POST", "/api/issues?workspace_id="+testWorkspaceID, map[string]any{
		"title":      "must stay open after concurrent reopen",
		"status":     "todo",
		"project_id": project.ID,
	})
	testHandler.CreateIssue(w, req)
	if w.Code != http.StatusCreated {
		t.Fatalf("create issue: %d %s", w.Code, w.Body.String())
	}
	var issue IssueResponse
	if err := json.NewDecoder(w.Body).Decode(&issue); err != nil {
		t.Fatalf("decode issue: %v", err)
	}
	if _, err := testPool.Exec(context.Background(), `UPDATE issue SET status = 'todo' WHERE id = $1`, issue.ID); err != nil {
		t.Fatalf("restore open issue fixture: %v", err)
	}

	tx, err := testPool.Begin(context.Background())
	if err != nil {
		t.Fatalf("begin concurrent reopen: %v", err)
	}
	defer tx.Rollback(context.Background())
	if _, err := tx.Exec(context.Background(), `UPDATE project SET status = 'in_progress' WHERE id = $1`, project.ID); err != nil {
		t.Fatalf("stage concurrent reopen: %v", err)
	}

	response := make(chan *httptest.ResponseRecorder, 1)
	go func() {
		w := httptest.NewRecorder()
		req := newRequest("PUT", "/api/projects/"+project.ID, map[string]any{"title": "renamed after reopen"})
		req = withURLParam(req, "id", project.ID)
		testHandler.UpdateProject(w, req)
		response <- w
	}()
	select {
	case early := <-response:
		t.Fatalf("project update did not wait for row lock: %d %s", early.Code, early.Body.String())
	case <-time.After(100 * time.Millisecond):
	}
	if err := tx.Commit(context.Background()); err != nil {
		t.Fatalf("commit concurrent reopen: %v", err)
	}
	select {
	case w = <-response:
	case <-time.After(3 * time.Second):
		t.Fatal("project update remained blocked after lock release")
	}
	if w.Code != http.StatusOK {
		t.Fatalf("rename project: %d %s", w.Code, w.Body.String())
	}

	var projectStatus, issueStatus string
	if err := testPool.QueryRow(context.Background(), `SELECT status FROM project WHERE id = $1`, project.ID).Scan(&projectStatus); err != nil {
		t.Fatalf("read project status: %v", err)
	}
	if err := testPool.QueryRow(context.Background(), `SELECT status FROM issue WHERE id = $1`, issue.ID).Scan(&issueStatus); err != nil {
		t.Fatalf("read issue status: %v", err)
	}
	if projectStatus != "in_progress" || issueStatus != "todo" {
		t.Fatalf("statuses after concurrent reopen = project %q, issue %q; want in_progress/todo", projectStatus, issueStatus)
	}
}

func TestCompletedProjectRejectsOpenIssueUpdates(t *testing.T) {
	project := createProjectPermissionTestProject(t, "completed project update guard")
	createIssue := func(title string) IssueResponse {
		w := httptest.NewRecorder()
		req := newRequest("POST", "/api/issues?workspace_id="+testWorkspaceID, map[string]any{
			"title": title, "status": "todo", "project_id": project.ID,
		})
		testHandler.CreateIssue(w, req)
		if w.Code != http.StatusCreated {
			t.Fatalf("create issue: %d %s", w.Code, w.Body.String())
		}
		var issue IssueResponse
		if err := json.NewDecoder(w.Body).Decode(&issue); err != nil {
			t.Fatalf("decode issue: %v", err)
		}
		t.Cleanup(func() { _, _ = testPool.Exec(context.Background(), `DELETE FROM issue WHERE id = $1`, issue.ID) })
		return issue
	}

	w := httptest.NewRecorder()
	req := newRequest("PUT", "/api/projects/"+project.ID, map[string]any{"status": "completed"})
	req = withURLParam(req, "id", project.ID)
	testHandler.UpdateProject(w, req)
	if w.Code != http.StatusOK {
		t.Fatalf("complete project: %d %s", w.Code, w.Body.String())
	}

	single := createIssue("single update cannot reopen")
	w = httptest.NewRecorder()
	req = newRequest("PUT", "/api/issues/"+single.ID, map[string]any{"status": "todo"})
	req = withURLParam(req, "id", single.ID)
	testHandler.UpdateIssue(w, req)
	if w.Code != http.StatusOK {
		t.Fatalf("UpdateIssue: %d %s", w.Code, w.Body.String())
	}
	var updated IssueResponse
	if err := json.NewDecoder(w.Body).Decode(&updated); err != nil {
		t.Fatalf("decode UpdateIssue: %v", err)
	}
	if updated.Status != "done" {
		t.Fatalf("single update reopened completed-project issue to %q", updated.Status)
	}

	batch := createIssue("batch update cannot reopen")
	w = httptest.NewRecorder()
	req = newRequest("POST", "/api/issues/batch-update?workspace_id="+testWorkspaceID, map[string]any{
		"issue_ids": []string{batch.ID}, "updates": map[string]any{"status": "todo"},
	})
	testHandler.BatchUpdateIssues(w, req)
	if w.Code != http.StatusOK {
		t.Fatalf("BatchUpdateIssues: %d %s", w.Code, w.Body.String())
	}
	var batchStatus string
	if err := testPool.QueryRow(context.Background(), `SELECT status FROM issue WHERE id = $1`, batch.ID).Scan(&batchStatus); err != nil {
		t.Fatalf("load batch issue: %v", err)
	}
	if batchStatus != "done" {
		t.Fatalf("batch update reopened completed-project issue to %q", batchStatus)
	}
}

func TestUpdateIssueWaitsForProjectCompletionBeforeMovingIntoProject(t *testing.T) {
	project := createProjectPermissionTestProject(t, "concurrent move completion guard")
	w := httptest.NewRecorder()
	req := newRequest("POST", "/api/issues?workspace_id="+testWorkspaceID, map[string]any{
		"title": "move into completing project", "status": "todo",
	})
	testHandler.CreateIssue(w, req)
	if w.Code != http.StatusCreated {
		t.Fatalf("create issue: %d %s", w.Code, w.Body.String())
	}
	var issue IssueResponse
	if err := json.NewDecoder(w.Body).Decode(&issue); err != nil {
		t.Fatalf("decode issue: %v", err)
	}
	t.Cleanup(func() { _, _ = testPool.Exec(context.Background(), `DELETE FROM issue WHERE id = $1`, issue.ID) })

	completionTx, err := testPool.Begin(context.Background())
	if err != nil {
		t.Fatalf("begin completion: %v", err)
	}
	defer completionTx.Rollback(context.Background())
	if _, err := completionTx.Exec(context.Background(), `UPDATE project SET status = 'completed' WHERE id = $1`, project.ID); err != nil {
		t.Fatalf("stage project completion: %v", err)
	}

	response := make(chan *httptest.ResponseRecorder, 1)
	go func() {
		w := httptest.NewRecorder()
		req := newRequest("PUT", "/api/issues/"+issue.ID, map[string]any{
			"project_id": project.ID, "status": "todo",
		})
		req = withURLParam(req, "id", issue.ID)
		testHandler.UpdateIssue(w, req)
		response <- w
	}()
	select {
	case early := <-response:
		t.Fatalf("issue update did not wait for project lock: %d %s", early.Code, early.Body.String())
	case <-time.After(100 * time.Millisecond):
	}
	if err := completionTx.Commit(context.Background()); err != nil {
		t.Fatalf("commit project completion: %v", err)
	}
	select {
	case w = <-response:
	case <-time.After(3 * time.Second):
		t.Fatal("issue update remained blocked after project completion")
	}
	if w.Code != http.StatusOK {
		t.Fatalf("UpdateIssue: %d %s", w.Code, w.Body.String())
	}
	var moved IssueResponse
	if err := json.NewDecoder(w.Body).Decode(&moved); err != nil {
		t.Fatalf("decode moved issue: %v", err)
	}
	if moved.ProjectID == nil || *moved.ProjectID != project.ID || moved.Status != "done" {
		t.Fatalf("moved issue = project %v status %q, want %s/done", moved.ProjectID, moved.Status, project.ID)
	}
}

func TestUpdateProjectCompletionScopesIssueSweepToWorkspace(t *testing.T) {
	ctx := context.Background()
	project := createProjectPermissionTestProject(t, "workspace-scoped completion")
	suffix := fmt.Sprintf("%d", time.Now().UnixNano())
	var foreignWorkspaceID, foreignIssueID string
	if err := testPool.QueryRow(ctx, `
		INSERT INTO workspace (name, slug, description, issue_prefix)
		VALUES ($1, $2, '', 'PCS') RETURNING id
	`, "Project completion foreign "+suffix, "project-completion-foreign-"+suffix).Scan(&foreignWorkspaceID); err != nil {
		t.Fatalf("insert foreign workspace: %v", err)
	}
	if err := testPool.QueryRow(ctx, `
		INSERT INTO issue (workspace_id, title, status, priority, creator_type, creator_id, number, project_id)
		VALUES ($1, 'foreign soft reference', 'todo', 'none', 'member', $2, 1, $3)
		RETURNING id
	`, foreignWorkspaceID, testUserID, project.ID).Scan(&foreignIssueID); err != nil {
		t.Fatalf("insert foreign issue: %v", err)
	}
	t.Cleanup(func() {
		_, _ = testPool.Exec(ctx, `DELETE FROM issue WHERE id = $1`, foreignIssueID)
		_, _ = testPool.Exec(ctx, `DELETE FROM workspace WHERE id = $1`, foreignWorkspaceID)
	})

	leakedEvents := make(chan events.Event, 1)
	testHandler.Bus.Subscribe(protocol.EventIssueUpdated, func(event events.Event) {
		payload, _ := event.Payload.(map[string]any)
		issue, _ := payload["issue"].(IssueResponse)
		if issue.ID == foreignIssueID {
			leakedEvents <- event
		}
	})
	w := httptest.NewRecorder()
	req := newRequest("PUT", "/api/projects/"+project.ID, map[string]any{"status": "completed"})
	req = withURLParam(req, "id", project.ID)
	testHandler.UpdateProject(w, req)
	if w.Code != http.StatusOK {
		t.Fatalf("complete project: %d %s", w.Code, w.Body.String())
	}
	var status string
	if err := testPool.QueryRow(ctx, `SELECT status FROM issue WHERE id = $1`, foreignIssueID).Scan(&status); err != nil {
		t.Fatalf("load foreign issue: %v", err)
	}
	if status != "todo" || len(leakedEvents) != 0 {
		t.Fatalf("foreign soft reference changed to %q with %d leaked events", status, len(leakedEvents))
	}
}

func TestDeleteProjectRequiresAdminOrOwner(t *testing.T) {
	memberUserID := createProjectPermissionTestMember(t, "member")
	project := createProjectPermissionTestProject(t, "delete permission denied project")

	w := httptest.NewRecorder()
	req := newRequestAs(memberUserID, "DELETE", "/api/projects/"+project.ID, nil)
	req = withURLParam(req, "id", project.ID)
	testHandler.DeleteProject(w, req)

	if w.Code != http.StatusForbidden {
		t.Fatalf("expected 403 for plain member project delete, got %d: %s", w.Code, w.Body.String())
	}

	var exists bool
	if err := testPool.QueryRow(context.Background(), `SELECT EXISTS (SELECT 1 FROM project WHERE id = $1)`, project.ID).Scan(&exists); err != nil {
		t.Fatalf("verify project exists: %v", err)
	}
	if !exists {
		t.Fatal("project was deleted despite plain member request")
	}
}

func TestDeleteProjectAllowsAdmin(t *testing.T) {
	adminUserID := createProjectPermissionTestMember(t, "admin")
	project := createProjectPermissionTestProject(t, "delete permission admin project")

	w := httptest.NewRecorder()
	req := newRequestAs(adminUserID, "DELETE", "/api/projects/"+project.ID, nil)
	req = withURLParam(req, "id", project.ID)
	testHandler.DeleteProject(w, req)

	if w.Code != http.StatusNoContent {
		t.Fatalf("expected 204 for admin project delete, got %d: %s", w.Code, w.Body.String())
	}

	var exists bool
	if err := testPool.QueryRow(context.Background(), `SELECT EXISTS (SELECT 1 FROM project WHERE id = $1)`, project.ID).Scan(&exists); err != nil {
		t.Fatalf("verify project deleted: %v", err)
	}
	if exists {
		t.Fatal("project still exists after admin delete")
	}
}

func createProjectPermissionTestMember(t *testing.T, role string) string {
	t.Helper()

	ctx := context.Background()
	email := "project-delete-" + role + "@multica.test"
	// The schema uses no foreign keys or cascades, so a leftover member from a
	// prior run won't disappear when its user is deleted. Drop the member first.
	_, _ = testPool.Exec(ctx, `DELETE FROM member WHERE user_id IN (SELECT id FROM "user" WHERE email = $1)`, email)
	_, _ = testPool.Exec(ctx, `DELETE FROM "user" WHERE email = $1`, email)

	var userID string
	if err := testPool.QueryRow(ctx, `
INSERT INTO "user" (name, email)
VALUES ($1, $2)
RETURNING id
`, "Project Delete "+role, email).Scan(&userID); err != nil {
		t.Fatalf("create %s user: %v", role, err)
	}
	t.Cleanup(func() {
		// No cascade in the schema: remove the member row before its user so the
		// shared test workspace isn't left with an orphaned member record.
		_, _ = testPool.Exec(context.Background(), `DELETE FROM member WHERE user_id = $1`, userID)
		_, _ = testPool.Exec(context.Background(), `DELETE FROM "user" WHERE id = $1`, userID)
	})

	if _, err := testPool.Exec(ctx, `
INSERT INTO member (workspace_id, user_id, role)
VALUES ($1, $2, $3)
`, testWorkspaceID, userID, role); err != nil {
		t.Fatalf("create %s member: %v", role, err)
	}

	return userID
}

func createProjectPermissionTestProject(t *testing.T, title string) ProjectResponse {
	t.Helper()

	w := httptest.NewRecorder()
	req := newRequest("POST", "/api/projects?workspace_id="+testWorkspaceID, map[string]any{
		"title": title,
	})
	testHandler.CreateProject(w, req)
	if w.Code != http.StatusCreated {
		t.Fatalf("CreateProject: expected 201, got %d: %s", w.Code, w.Body.String())
	}
	var project ProjectResponse
	if err := json.NewDecoder(w.Body).Decode(&project); err != nil {
		t.Fatalf("decode CreateProject: %v", err)
	}
	t.Cleanup(func() {
		_, _ = testPool.Exec(context.Background(), `DELETE FROM project WHERE id = $1`, project.ID)
	})
	return project
}
