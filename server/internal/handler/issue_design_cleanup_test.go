package handler

import (
	"context"
	"fmt"
	"net/http"
	"net/http/httptest"
	"reflect"
	"testing"
)

type issueDesignCleanupFixture struct {
	workspaceID      string
	targetIssueID    string
	unrelatedIssueID string
	fileID           string
	revisionID       string
	targetDraftID    string
	targetRestoreID  string
	deliveryID       string
	draftID          string
	restoreTaskID    string
}

type issueDesignUnrelatedSnapshot struct {
	deliveryWorkspaceID string
	deliverySourceID    string
	deliveryTargetID    string
	draftWorkspaceID    string
	draftIssueID        string
	restoreWorkspaceID  string
	restoreIssueID      string
}

func TestDeleteIssueCleansDesignReferences(t *testing.T) {
	if testHandler == nil || testPool == nil {
		t.Skip("database not available")
	}

	target := seedIssueDesignCleanupFixture(t, "issue-design-cleanup-target", true)
	foreign := seedIssueDesignCleanupFixture(t, "issue-design-cleanup-foreign", false)
	targetUnrelatedBefore := issueDesignUnrelatedRows(t, target)
	foreignBefore := issueDesignUnrelatedRows(t, foreign)

	w := httptest.NewRecorder()
	req := newRequest(http.MethodDelete, "/api/issues/"+target.targetIssueID, nil)
	req.Header.Set("X-Workspace-ID", target.workspaceID)
	req = withURLParam(req, "id", target.targetIssueID)
	testHandler.DeleteIssue(w, req)
	if w.Code != http.StatusNoContent {
		t.Fatalf("DeleteIssue: expected 204, got %d: %s", w.Code, w.Body.String())
	}

	if got := issueDesignUnrelatedRows(t, target); !reflect.DeepEqual(got, targetUnrelatedBefore) {
		t.Fatalf("unrelated issue design rows changed: before=%+v after=%+v", targetUnrelatedBefore, got)
	}
	if got := issueDesignUnrelatedRows(t, foreign); !reflect.DeepEqual(got, foreignBefore) {
		t.Fatalf("foreign workspace design rows changed: before=%+v after=%+v", foreignBefore, got)
	}

	var issueCount, deliveryCount, clearedDraftCount, clearedRestoreCount int
	ctx := context.Background()
	if err := testPool.QueryRow(ctx, `SELECT count(*) FROM issue WHERE id = $1 AND workspace_id = $2`, target.targetIssueID, target.workspaceID).Scan(&issueCount); err != nil {
		t.Fatalf("count deleted issue: %v", err)
	}
	if err := testPool.QueryRow(ctx, `
		SELECT count(*) FROM design_delivery
		WHERE workspace_id = $1 AND (source_issue_id = $2 OR target_issue_id = $2)
	`, target.workspaceID, target.targetIssueID).Scan(&deliveryCount); err != nil {
		t.Fatalf("count target design deliveries: %v", err)
	}
	if err := testPool.QueryRow(ctx, `
		SELECT count(*) FROM design_draft
		WHERE id = $1 AND workspace_id = $2 AND issue_id IS NULL
	`, target.targetDraftID, target.workspaceID).Scan(&clearedDraftCount); err != nil {
		t.Fatalf("count cleared target design draft: %v", err)
	}
	if err := testPool.QueryRow(ctx, `
		SELECT count(*) FROM design_restore_task
		WHERE id = $1 AND workspace_id = $2 AND issue_id IS NULL
	`, target.targetRestoreID, target.workspaceID).Scan(&clearedRestoreCount); err != nil {
		t.Fatalf("count cleared target design restore task: %v", err)
	}

	if issueCount != 0 || deliveryCount != 0 || clearedDraftCount != 1 || clearedRestoreCount != 1 {
		t.Fatalf(
			"unexpected design state after issue delete: issue=%d delivery=%d cleared_draft=%d cleared_restore=%d; want 0, 0, 1, 1",
			issueCount, deliveryCount, clearedDraftCount, clearedRestoreCount,
		)
	}
}

func seedIssueDesignCleanupFixture(t *testing.T, slug string, member bool) issueDesignCleanupFixture {
	t.Helper()

	ctx := context.Background()
	cleanupIssueDesignCleanupFixture(ctx, slug)

	var fixture issueDesignCleanupFixture
	if err := testPool.QueryRow(ctx, `
		INSERT INTO workspace (name, slug, issue_prefix)
		VALUES ('Issue design cleanup', $1, 'IDC')
		RETURNING id
	`, slug).Scan(&fixture.workspaceID); err != nil {
		t.Fatalf("seed issue design cleanup workspace %q: %v", slug, err)
	}
	if member {
		if _, err := testPool.Exec(ctx, `
			INSERT INTO member (workspace_id, user_id, role) VALUES ($1, $2, 'owner')
		`, fixture.workspaceID, testUserID); err != nil {
			t.Fatalf("seed issue design cleanup member %q: %v", slug, err)
		}
	}

	issueIDs := make([]string, 3)
	for index := range issueIDs {
		if err := testPool.QueryRow(ctx, `
			INSERT INTO issue (workspace_id, title, creator_type, creator_id, number)
			VALUES ($1, $2, 'member', $3, $4)
			RETURNING id
		`, fixture.workspaceID, fmt.Sprintf("Issue design cleanup %d", index+1), testUserID, index+1).Scan(&issueIDs[index]); err != nil {
			t.Fatalf("seed issue design cleanup issue %q: %v", slug, err)
		}
	}
	fixture.targetIssueID = issueIDs[0]
	fixture.unrelatedIssueID = issueIDs[2]
	if err := testPool.QueryRow(ctx, `
		WITH file_row AS (
			INSERT INTO design_file (workspace_id, title, source_type)
			VALUES ($1, 'Issue design cleanup file', 'upload')
			RETURNING id, workspace_id
		), revision_row AS (
			INSERT INTO design_revision (file_id, workspace_id, revision_number, native_json)
			SELECT id, workspace_id, 1, '{}'::jsonb FROM file_row
			RETURNING id, file_id
		)
		SELECT file_id, id FROM revision_row
	`, fixture.workspaceID).Scan(&fixture.fileID, &fixture.revisionID); err != nil {
		t.Fatalf("seed issue design cleanup file %q: %v", slug, err)
	}

	if _, err := testPool.Exec(ctx, `
		INSERT INTO design_delivery (workspace_id, source_issue_id, target_issue_id, file_id, revision_id)
		VALUES
			($1, $2, $3, $4, $5),
			($1, $3, $2, $4, $5)
	`, fixture.workspaceID, fixture.targetIssueID, issueIDs[1], fixture.fileID, fixture.revisionID); err != nil {
		t.Fatalf("seed target issue design deliveries %q: %v", slug, err)
	}
	if err := testPool.QueryRow(ctx, `
		INSERT INTO design_delivery (workspace_id, source_issue_id, target_issue_id, file_id, revision_id)
		VALUES ($1, $2, $3, $4, $5)
		RETURNING id
	`, fixture.workspaceID, fixture.unrelatedIssueID, issueIDs[1], fixture.fileID, fixture.revisionID).Scan(&fixture.deliveryID); err != nil {
		t.Fatalf("seed unrelated issue design delivery %q: %v", slug, err)
	}

	if err := testPool.QueryRow(ctx, `
		INSERT INTO design_draft (workspace_id, issue_id, title)
		VALUES ($1, $2, 'Target issue design draft')
		RETURNING id
	`, fixture.workspaceID, fixture.targetIssueID).Scan(&fixture.targetDraftID); err != nil {
		t.Fatalf("seed target issue design draft %q: %v", slug, err)
	}
	if err := testPool.QueryRow(ctx, `
		INSERT INTO design_draft (workspace_id, issue_id, title)
		VALUES ($1, $2, 'Unrelated issue design draft')
		RETURNING id
	`, fixture.workspaceID, fixture.unrelatedIssueID).Scan(&fixture.draftID); err != nil {
		t.Fatalf("seed unrelated issue design draft %q: %v", slug, err)
	}

	if err := testPool.QueryRow(ctx, `
		INSERT INTO design_restore_task (workspace_id, file_id, revision_id, issue_id)
		VALUES ($1, $2, $3, $4)
		RETURNING id
	`, fixture.workspaceID, fixture.fileID, fixture.revisionID, fixture.targetIssueID).Scan(&fixture.targetRestoreID); err != nil {
		t.Fatalf("seed target issue design restore task %q: %v", slug, err)
	}
	if err := testPool.QueryRow(ctx, `
		INSERT INTO design_restore_task (workspace_id, file_id, revision_id, issue_id)
		VALUES ($1, $2, $3, $4)
		RETURNING id
	`, fixture.workspaceID, fixture.fileID, fixture.revisionID, fixture.unrelatedIssueID).Scan(&fixture.restoreTaskID); err != nil {
		t.Fatalf("seed unrelated issue design restore task %q: %v", slug, err)
	}

	t.Cleanup(func() { cleanupIssueDesignCleanupFixture(context.Background(), slug) })
	return fixture
}

func issueDesignUnrelatedRows(t *testing.T, fixture issueDesignCleanupFixture) issueDesignUnrelatedSnapshot {
	t.Helper()

	ctx := context.Background()
	var snapshot issueDesignUnrelatedSnapshot
	if err := testPool.QueryRow(ctx, `
		SELECT workspace_id, source_issue_id, target_issue_id
		FROM design_delivery WHERE id = $1
	`, fixture.deliveryID).Scan(&snapshot.deliveryWorkspaceID, &snapshot.deliverySourceID, &snapshot.deliveryTargetID); err != nil {
		t.Fatalf("load unrelated design delivery: %v", err)
	}
	if err := testPool.QueryRow(ctx, `
		SELECT workspace_id, issue_id FROM design_draft WHERE id = $1
	`, fixture.draftID).Scan(&snapshot.draftWorkspaceID, &snapshot.draftIssueID); err != nil {
		t.Fatalf("load unrelated design draft: %v", err)
	}
	if err := testPool.QueryRow(ctx, `
		SELECT workspace_id, issue_id FROM design_restore_task WHERE id = $1
	`, fixture.restoreTaskID).Scan(&snapshot.restoreWorkspaceID, &snapshot.restoreIssueID); err != nil {
		t.Fatalf("load unrelated design restore task: %v", err)
	}
	return snapshot
}

func cleanupIssueDesignCleanupFixture(ctx context.Context, slug string) {
	statements := []string{
		`DELETE FROM design_restore_task WHERE workspace_id IN (SELECT id FROM workspace WHERE slug = $1)`,
		`DELETE FROM design_draft WHERE workspace_id IN (SELECT id FROM workspace WHERE slug = $1)`,
		`DELETE FROM design_delivery WHERE workspace_id IN (SELECT id FROM workspace WHERE slug = $1)`,
		`DELETE FROM design_revision WHERE workspace_id IN (SELECT id FROM workspace WHERE slug = $1)`,
		`DELETE FROM design_file WHERE workspace_id IN (SELECT id FROM workspace WHERE slug = $1)`,
		`DELETE FROM member WHERE workspace_id IN (SELECT id FROM workspace WHERE slug = $1)`,
		`DELETE FROM issue WHERE workspace_id IN (SELECT id FROM workspace WHERE slug = $1)`,
		`DELETE FROM workspace WHERE slug = $1`,
	}
	for _, statement := range statements {
		_, _ = testPool.Exec(ctx, statement, slug)
	}
}
