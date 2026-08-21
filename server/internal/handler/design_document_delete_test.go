package handler

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"

	db "github.com/multica-ai/multica/server/pkg/db/generated"
)

func deleteDesignDocumentRequest(t *testing.T, documentID string) *httptest.ResponseRecorder {
	t.Helper()
	recorder := httptest.NewRecorder()
	testHandler.DeleteDesignDocument(
		recorder,
		withURLParam(newRequest(http.MethodDelete, "/api/design-documents/"+documentID, nil), "id", documentID),
	)
	return recorder
}

// Deleting takes the document's revisions with it in one statement. A revision
// left behind would be unreachable — its document is the only way in — and
// would still count against the workspace's storage.
func TestDeleteDesignDocumentRemovesItsRevisions(t *testing.T) {
	fixture := createDesignDocumentRevisionFixture(t)
	documentID := uuidToString(fixture.Document.ID)

	if recorder := deleteDesignDocumentRequest(t, documentID); recorder.Code != http.StatusNoContent {
		t.Fatalf("status = %d, body = %s", recorder.Code, recorder.Body.String())
	}

	queries := db.New(testPool)
	if _, err := queries.GetDesignDocumentInWorkspace(context.Background(), db.GetDesignDocumentInWorkspaceParams{
		ID: fixture.Document.ID, WorkspaceID: fixture.Document.WorkspaceID,
	}); err == nil {
		t.Fatal("document still readable after delete")
	}
	var revisions int
	if err := testPool.QueryRow(context.Background(),
		`SELECT count(*) FROM design_document_revision WHERE design_document_id = $1`,
		fixture.Document.ID,
	).Scan(&revisions); err != nil {
		t.Fatalf("count revisions: %v", err)
	}
	if revisions != 0 {
		t.Fatalf("revisions left behind = %d, want 0", revisions)
	}

	// Deleting what is already gone reads as a missing document, not a
	// success — the id no longer resolves.
	if recorder := deleteDesignDocumentRequest(t, documentID); recorder.Code == http.StatusNoContent {
		t.Fatal("second delete reported success for a document that no longer exists")
	}
}

// A document whose run already failed or was cancelled still holds
// active_task_id. Guarding delete on the pointer rather than the task's status
// made exactly those documents — the dead ends a user most wants to clear —
// the only ones that could never be deleted.
func TestDeleteDesignDocumentClearsADocumentWhoseRunAlreadyEnded(t *testing.T) {
	for _, status := range []string{"failed", "cancelled", "completed"} {
		t.Run(status, func(t *testing.T) {
			fixture := createDesignDocumentRevisionFixture(t)
			queries := db.New(testPool)
			if _, err := testPool.Exec(context.Background(),
				`UPDATE agent_task_queue SET status = $2 WHERE id = $1`,
				fixture.Document.ActiveTaskID, status,
			); err != nil {
				t.Fatalf("age the fixture task: %v", err)
			}
			if _, err := queries.UpdateDesignDocumentActiveTask(context.Background(), db.UpdateDesignDocumentActiveTaskParams{
				ID:            fixture.Document.ID,
				WorkspaceID:   fixture.Document.WorkspaceID,
				ActiveTaskID:  fixture.Document.ActiveTaskID,
				InputSnapshot: fixture.Document.InputSnapshot,
			}); err != nil {
				t.Fatalf("re-arm the dangling pointer: %v", err)
			}

			recorder := deleteDesignDocumentRequest(t, uuidToString(fixture.Document.ID))
			if recorder.Code != http.StatusNoContent {
				t.Fatalf("task %q: status = %d, want 204; body = %s", status, recorder.Code, recorder.Body.String())
			}
		})
	}
}

// A task row that no longer exists cannot complete into anything either, so a
// pointer at a vanished task must not lock the document out.
func TestDeleteDesignDocumentClearsADocumentPointingAtAVanishedTask(t *testing.T) {
	fixture := createDesignDocumentRevisionFixture(t)
	queries := db.New(testPool)
	if _, err := queries.UpdateDesignDocumentActiveTask(context.Background(), db.UpdateDesignDocumentActiveTaskParams{
		ID:            fixture.Document.ID,
		WorkspaceID:   fixture.Document.WorkspaceID,
		ActiveTaskID:  parseUUID("0c0c0c0c-0c0c-4c0c-8c0c-0c0c0c0c0c0c"),
		InputSnapshot: fixture.Document.InputSnapshot,
	}); err != nil {
		t.Fatalf("point at a task that does not exist: %v", err)
	}
	if recorder := deleteDesignDocumentRequest(t, uuidToString(fixture.Document.ID)); recorder.Code != http.StatusNoContent {
		t.Fatalf("status = %d, want 204; body = %s", recorder.Code, recorder.Body.String())
	}
}

// The agent task outlives the row it was enqueued for, so a mid-run delete
// would leave a task completing into a document that no longer exists. The
// task has to be reachable through the tenant guard for this to bite, which
// is what makes it the live case rather than the leftover-pointer one above.
func TestDeleteDesignDocumentRefusesARunningDocument(t *testing.T) {
	fixture := createDesignDocumentRevisionFixture(t)
	queries := db.New(testPool)
	agentID, runtimeID := createProjectDesignSystemAgent(t, "online")

	var taskID string
	if err := testPool.QueryRow(context.Background(), `
		INSERT INTO agent_task_queue (agent_id, runtime_id, status, priority, context)
		VALUES ($1, $2, 'running', 0, '{}'::jsonb)
		RETURNING id
	`, agentID, runtimeID).Scan(&taskID); err != nil {
		t.Fatalf("enqueue a live task: %v", err)
	}
	if _, err := queries.UpdateDesignDocumentActiveTask(context.Background(), db.UpdateDesignDocumentActiveTaskParams{
		ID:            fixture.Document.ID,
		WorkspaceID:   fixture.Document.WorkspaceID,
		ActiveTaskID:  parseUUID(taskID),
		InputSnapshot: fixture.Document.InputSnapshot,
	}); err != nil {
		t.Fatalf("point the document at the live task: %v", err)
	}

	recorder := deleteDesignDocumentRequest(t, uuidToString(fixture.Document.ID))
	if recorder.Code != http.StatusConflict {
		t.Fatalf("status = %d, want 409; body = %s", recorder.Code, recorder.Body.String())
	}
	if _, err := queries.GetDesignDocumentInWorkspace(context.Background(), db.GetDesignDocumentInWorkspaceParams{
		ID: fixture.Document.ID, WorkspaceID: fixture.Document.WorkspaceID,
	}); err != nil {
		t.Fatalf("refused delete still removed the document: %v", err)
	}
}
