package handler

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	db "github.com/multica-ai/multica/server/pkg/db/generated"
)

// A rerun is only for the dead end: no revision and no live task. Anything
// else must be refused — a running task would race the pointer move, and a
// document with a revision is adjusted, not silently regenerated over.
func TestDesignDocumentRegenerateBlockedMatrix(t *testing.T) {
	task := designDocumentRevisionUUID(t, 0x33)
	draft := designDocumentRevisionUUID(t, 0x11)
	saved := designDocumentRevisionUUID(t, 0x22)

	if code, _, blocked := designDocumentRegenerateBlocked(db.DesignDocument{ActiveTaskID: task}); !blocked || code != "operation_in_progress" {
		t.Fatalf("running document: code=%q blocked=%v", code, blocked)
	}
	if code, _, blocked := designDocumentRegenerateBlocked(db.DesignDocument{DraftRevisionID: draft}); !blocked || code != "revision_exists" {
		t.Fatalf("draft document: code=%q blocked=%v", code, blocked)
	}
	if code, _, blocked := designDocumentRegenerateBlocked(db.DesignDocument{SavedRevisionID: saved}); !blocked || code != "revision_exists" {
		t.Fatalf("saved document: code=%q blocked=%v", code, blocked)
	}
	// The dead end itself: failed or stopped before any revision landed.
	if code, _, blocked := designDocumentRegenerateBlocked(db.DesignDocument{}); blocked {
		t.Fatalf("empty document blocked with %q", code)
	}
}

// The rerun runs from the frozen snapshot verbatim; an agent override rewrites
// the snapshot so it keeps describing the run it produces.
func TestDesignDocumentRegenerateInputKeepsTheFrozenSnapshot(t *testing.T) {
	snapshot := []byte(`{
		"agent_id": "agent-frozen",
		"platform": "mobile",
		"recipe": "wireframe",
		"brief": "司机端接单页",
		"attachments": [{"attachment_id": "attachment-1", "filename": "ref.png", "content_type": "image/png", "size_bytes": 10, "sha256": "abc"}]
	}`)

	input, inputJSON, attachments, err := designDocumentRegenerateInput(snapshot, "")
	if err != nil {
		t.Fatal(err)
	}
	if input.AgentID != "agent-frozen" || input.Platform != "mobile" || input.Recipe != "wireframe" || input.Brief != "司机端接单页" {
		t.Fatalf("frozen input = %+v", input)
	}
	if len(attachments) != 1 || attachments[0].AttachmentID != "attachment-1" || attachments[0].SHA256 != "abc" {
		t.Fatalf("attachments = %+v", attachments)
	}
	var roundTrip designDocumentInputSnapshot
	if err := json.Unmarshal(inputJSON, &roundTrip); err != nil || roundTrip.AgentID != "agent-frozen" {
		t.Fatalf("input JSON does not round-trip: %s (%v)", inputJSON, err)
	}
}

func TestDesignDocumentRegenerateInputAppliesTheAgentOverride(t *testing.T) {
	snapshot := []byte(`{"agent_id": "agent-frozen", "platform": "web", "recipe": "default", "brief": "CRM"}`)

	input, inputJSON, _, err := designDocumentRegenerateInput(snapshot, "agent-next")
	if err != nil {
		t.Fatal(err)
	}
	if input.AgentID != "agent-next" {
		t.Fatalf("override not applied: %+v", input)
	}
	var reread designDocumentInputSnapshot
	if err := json.Unmarshal(inputJSON, &reread); err != nil || reread.AgentID != "agent-next" {
		t.Fatalf("snapshot JSON kept the old agent: %s", inputJSON)
	}
}

// A snapshot that cannot be read back cannot be rerun honestly.
func TestDesignDocumentRegenerateInputRefusesAnUnreadableSnapshot(t *testing.T) {
	for _, snapshot := range [][]byte{nil, {}, []byte("{"), []byte(`{"attachments": 7}`)} {
		if _, _, _, err := designDocumentRegenerateInput(snapshot, ""); err == nil {
			t.Fatalf("snapshot %q was accepted", snapshot)
		}
	}
}

func performDesignDocumentRegenerate(t *testing.T, documentID string, body any) *httptest.ResponseRecorder {
	t.Helper()
	recorder := httptest.NewRecorder()
	request := withURLParam(newRequest(http.MethodPost, "/api/design-documents/"+documentID+"/regenerate", body), "id", documentID)
	testHandler.RegenerateDesignDocument(recorder, request)
	return recorder
}

// Stopping the first generation must release the document — the cancel path
// used to skip design documents entirely, leaving active_task_id pointing at
// the cancelled task and every later adjust/save/discard/regenerate answering
// operation_in_progress forever — and the released dead end must be rerunnable
// from its frozen snapshot.
func TestStoppedFirstGenerationReleasesTheDocumentAndRegenerates(t *testing.T) {
	ctx := context.Background()
	projectID := createProjectDesignSystemProject(t, testWorkspaceID, "Regenerate release")
	agentID, _ := createProjectDesignSystemAgent(t, "online")
	overrideAgentID, _ := createProjectDesignSystemAgent(t, "online")

	// A real first generation, through the composer endpoint, so the frozen
	// snapshot and the task context are exactly what production writes.
	created := performProjectDesignSystemRequest(t, testHandler.CreateDesignDocument, http.MethodPost, "/api/design-documents", map[string]any{
		"project_id": projectID,
		"agent_id":   agentID,
		"platform":   "web",
		"brief":      "客户列表页，支持筛选与批量操作。",
	})
	if created.Code != http.StatusCreated {
		t.Fatalf("create: status = %d, body = %s", created.Code, created.Body.String())
	}
	var document DesignDocumentResponse
	if err := json.NewDecoder(created.Body).Decode(&document); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() {
		_, _ = testPool.Exec(context.Background(), `DELETE FROM design_document WHERE id = $1`, parseUUID(document.ID))
	})
	if document.ActiveTask == nil {
		t.Fatal("create left no active task")
	}

	// While the first run is live, a rerun must be refused.
	blocked := performDesignDocumentRegenerate(t, document.ID, nil)
	if blocked.Code != http.StatusConflict || !strings.Contains(blocked.Body.String(), "operation_in_progress") {
		t.Fatalf("regenerate while running: status = %d, body = %s", blocked.Code, blocked.Body.String())
	}

	// The user stops the run.
	if _, err := testHandler.TaskService.CancelTask(ctx, parseUUID(document.ActiveTask.ID)); err != nil {
		t.Fatalf("cancel first generation: %v", err)
	}
	queries := db.New(testPool)
	released, err := queries.GetDesignDocumentInWorkspace(ctx, db.GetDesignDocumentInWorkspaceParams{
		ID: parseUUID(document.ID), WorkspaceID: parseUUID(testWorkspaceID),
	})
	if err != nil {
		t.Fatal(err)
	}
	if released.ActiveTaskID.Valid {
		t.Fatal("cancel left active_task_id set; the document is wedged")
	}
	if !strings.Contains(string(released.LastError), "design_document_cancelled") {
		t.Fatalf("last_error after cancel = %s", released.LastError)
	}

	// Rerun under a different agent: accepted, running again, snapshot
	// rewritten to the override, the old failure no longer current.
	rerun := performDesignDocumentRegenerate(t, document.ID, map[string]any{"agent_id": overrideAgentID})
	if rerun.Code != http.StatusAccepted {
		t.Fatalf("regenerate: status = %d, body = %s", rerun.Code, rerun.Body.String())
	}
	var rerunning DesignDocumentResponse
	if err := json.NewDecoder(rerun.Body).Decode(&rerunning); err != nil {
		t.Fatal(err)
	}
	if rerunning.Status != "running" || rerunning.ActiveTask == nil || rerunning.ActiveTask.Operation != "generate" {
		t.Fatalf("rerun response = %+v", rerunning)
	}

	var inputJSON, lastError, taskContextJSON []byte
	if err := testPool.QueryRow(ctx, `
		SELECT d.input_snapshot, coalesce(d.last_error, 'null'::jsonb), task.context
		FROM design_document d, agent_task_queue task
		WHERE d.id = $1 AND task.id = d.active_task_id
	`, parseUUID(document.ID)).Scan(&inputJSON, &lastError, &taskContextJSON); err != nil {
		t.Fatalf("load rerun state: %v", err)
	}
	if string(lastError) != "null" {
		t.Fatalf("rerun kept last_error current: %s", lastError)
	}
	var input designDocumentInputSnapshot
	if err := json.Unmarshal(inputJSON, &input); err != nil || input.AgentID != overrideAgentID {
		t.Fatalf("snapshot agent after override = %q (%v), want %q", input.AgentID, err, overrideAgentID)
	}
	if input.Brief != "客户列表页，支持筛选与批量操作。" {
		t.Fatalf("rerun snapshot lost the frozen brief: %+v", input)
	}
	// The daemon-facing contract: a generate-shaped, execution-ready envelope
	// with the honest no-repository grounding mode (DC-059).
	var taskContext struct {
		Operation      string `json:"operation"`
		ExecutionReady bool   `json:"execution_ready"`
		Input          struct {
			RepositoryGrounding string `json:"repository_grounding"`
		} `json:"input"`
	}
	if err := json.Unmarshal(taskContextJSON, &taskContext); err != nil {
		t.Fatal(err)
	}
	if taskContext.Operation != "generate" || !taskContext.ExecutionReady || taskContext.Input.RepositoryGrounding != "unavailable" {
		t.Fatalf("rerun task context = %+v", taskContext)
	}
}

// A document that already has a revision is adjusted, never silently
// regenerated over.
func TestRegenerateRefusesADocumentWithARevision(t *testing.T) {
	fixture := createDesignDocumentRevisionFixture(t)
	response := performDesignDocumentRegenerate(t, uuidToString(fixture.Document.ID), nil)
	if response.Code != http.StatusConflict || !strings.Contains(response.Body.String(), "revision_exists") {
		t.Fatalf("status = %d, body = %s", response.Code, response.Body.String())
	}
}
