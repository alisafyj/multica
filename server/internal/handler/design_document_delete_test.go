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

// The agent task outlives the row it was enqueued for, so a mid-run delete
// would leave a task completing into a document that no longer exists.
func TestDeleteDesignDocumentRefusesARunningDocument(t *testing.T) {
	fixture := createDesignDocumentRevisionFixture(t)
	queries := db.New(testPool)
	if _, err := queries.UpdateDesignDocumentActiveTask(context.Background(), db.UpdateDesignDocumentActiveTaskParams{
		ID:          fixture.Document.ID,
		WorkspaceID: fixture.Document.WorkspaceID,
		// The fixture's run has already completed, so re-arm the pointer with
		// a task id of our own (no foreign keys here by repo policy).
		ActiveTaskID:  parseUUID("0e0e0e0e-0e0e-4e0e-8e0e-0e0e0e0e0e0e"),
		InputSnapshot: fixture.Document.InputSnapshot,
	}); err != nil {
		t.Fatalf("re-arm the active task: %v", err)
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
