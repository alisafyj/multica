// @canonical the dead-run guard matrix for design documents.
//
// Every write path on a design document refuses while a run is in flight. The
// question each guard has to answer is not "is a pointer set" but "can that
// task still finish" — a run that died holding active_task_id leaves the
// pointer set forever, and a guard reading the pointer alone turns exactly the
// documents a user most needs to recover into ones that can never be saved,
// discarded, restored, edited or deleted.
package handler

import (
	"context"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	db "github.com/multica-ai/multica/server/pkg/db/generated"
)

// armDesignDocumentRun points the fixture's document at a real task row in the
// given status and returns that task's id.
//
// The task has to be a real row reachable through the workspace tenant guard:
// a synthetic UUID exercises the vanished-task branch instead, and a document
// left with a NULL pointer exercises no guard at all.
func armDesignDocumentRun(t *testing.T, fixture designDocumentRevisionFixture, status string) string {
	t.Helper()
	agentID, runtimeID := createProjectDesignSystemAgent(t, "online")

	var taskID string
	if err := testPool.QueryRow(context.Background(), `
		INSERT INTO agent_task_queue (agent_id, runtime_id, status, priority, context)
		VALUES ($1, $2, $3, 0, '{}'::jsonb)
		RETURNING id
	`, agentID, runtimeID, status).Scan(&taskID); err != nil {
		t.Fatalf("enqueue a %s task: %v", status, err)
	}
	if _, err := db.New(testPool).UpdateDesignDocumentActiveTask(context.Background(), db.UpdateDesignDocumentActiveTaskParams{
		ID:            fixture.Document.ID,
		WorkspaceID:   fixture.Document.WorkspaceID,
		ActiveTaskID:  parseUUID(taskID),
		InputSnapshot: fixture.Document.InputSnapshot,
	}); err != nil {
		t.Fatalf("point the document at the %s task: %v", status, err)
	}

	// The pointer is the whole subject of these tests, so prove it landed
	// rather than trusting the write.
	var active *string
	if err := testPool.QueryRow(context.Background(),
		`SELECT active_task_id FROM design_document WHERE id = $1`, fixture.Document.ID).Scan(&active); err != nil {
		t.Fatalf("read back the pointer: %v", err)
	}
	if active == nil {
		t.Fatal("document was left with a NULL active_task_id; the guard under test would never run")
	}
	return taskID
}

func saveDesignDocumentRequest(t *testing.T, fixture designDocumentRevisionFixture) *httptest.ResponseRecorder {
	t.Helper()
	documentID := uuidToString(fixture.Document.ID)
	// The save handler parses its body before it reaches the guard, so an
	// unparseable body would 400 short of the branch under test.
	body := map[string]string{"draft_revision_id": uuidToString(fixture.Revision.ID)}
	recorder := httptest.NewRecorder()
	testHandler.SaveDesignDocument(recorder, withURLParam(
		newRequest(http.MethodPost, "/api/design-documents/"+documentID+"/save", body),
		"id", documentID))
	return recorder
}

func discardDesignDocumentRequest(t *testing.T, fixture designDocumentRevisionFixture) *httptest.ResponseRecorder {
	t.Helper()
	documentID := uuidToString(fixture.Document.ID)
	recorder := httptest.NewRecorder()
	testHandler.DiscardDesignDocument(recorder, withURLParam(
		newRequest(http.MethodPost, "/api/design-documents/"+documentID+"/discard", nil), "id", documentID))
	return recorder
}

func restoreDesignDocumentRequest(t *testing.T, fixture designDocumentRevisionFixture) *httptest.ResponseRecorder {
	t.Helper()
	documentID := uuidToString(fixture.Document.ID)
	revisionID := uuidToString(fixture.Revision.ID)
	recorder := httptest.NewRecorder()
	testHandler.RestoreDesignDocumentRevision(recorder, withURLParams(
		newRequest(http.MethodPost, "/api/design-documents/"+documentID+"/revisions/"+revisionID+"/restore", nil),
		"id", documentID, "revisionId", revisionID))
	return recorder
}

// A run that already reached a terminal state must not hold the document
// hostage. This is the regression behind "生成中 forever": the agent died, the
// task went failed, nothing released active_task_id, and every one of these
// paths answered 409 for the rest of the document's life.
func TestDesignDocumentWritePathsAcceptADocumentWhoseRunAlreadyEnded(t *testing.T) {
	paths := map[string]func(*testing.T, designDocumentRevisionFixture) *httptest.ResponseRecorder{
		"save":    saveDesignDocumentRequest,
		"discard": discardDesignDocumentRequest,
		"restore": restoreDesignDocumentRequest,
	}
	for _, status := range []string{"failed", "cancelled", "completed"} {
		for name, perform := range paths {
			t.Run(status+"/"+name, func(t *testing.T) {
				fixture := createDesignDocumentRevisionFixture(t)
				armDesignDocumentRun(t, fixture, status)

				recorder := perform(t, fixture)
				if recorder.Code == http.StatusConflict &&
					strings.Contains(recorder.Body.String(), "operation_in_progress") {
					t.Fatalf("%s refused a document whose %s run already ended: %s",
						name, status, recorder.Body.String())
				}
			})
		}
	}
}

// The same paths must still refuse while the run can genuinely still land.
// Losing this is the opposite failure: a write racing a live agent.
func TestDesignDocumentWritePathsStillRefuseALiveRun(t *testing.T) {
	paths := map[string]func(*testing.T, designDocumentRevisionFixture) *httptest.ResponseRecorder{
		"save":    saveDesignDocumentRequest,
		"discard": discardDesignDocumentRequest,
		"restore": restoreDesignDocumentRequest,
	}
	for _, status := range []string{"queued", "dispatched", "running"} {
		for name, perform := range paths {
			t.Run(status+"/"+name, func(t *testing.T) {
				fixture := createDesignDocumentRevisionFixture(t)
				armDesignDocumentRun(t, fixture, status)

				recorder := perform(t, fixture)
				if recorder.Code != http.StatusConflict ||
					!strings.Contains(recorder.Body.String(), "operation_in_progress") {
					t.Fatalf("%s allowed a write during a %s run: status = %d, body = %s",
						name, status, recorder.Code, recorder.Body.String())
				}
			})
		}
	}
}
