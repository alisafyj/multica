package handler

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/multica-ai/multica/server/internal/service"
)

func performDesignDocumentManualEdit(t *testing.T, documentID string, body any) *httptest.ResponseRecorder {
	t.Helper()
	recorder := httptest.NewRecorder()
	request := withURLParam(newRequest(http.MethodPost, "/api/design-documents/"+documentID+"/manual-edit", body), "id", documentID)
	testHandler.ManualEditDesignDocument(recorder, request)
	return recorder
}

// A manual edit is the one design-document run with no agent in it. What it
// must still carry is everything that makes a revision trustworthy: the
// immutable base it started from, and a context the daemon will accept.
func TestManualEditEnqueuesADeterministicRunAgainstTheCurrentBase(t *testing.T) {
	ctx := context.Background()
	fixture := createDesignDocumentRevisionFixture(t)
	agentID, _ := createProjectDesignSystemAgent(t, "online")

	response := performDesignDocumentManualEdit(t, uuidToString(fixture.Document.ID), map[string]any{
		"agent_id": agentID,
		"edits": []map[string]any{{
			"page":         "prototype/index.html",
			"selector":     "#open-filters",
			"declarations": map[string]string{"color": "#ff5701", "font-size": "14px"},
		}},
	})
	if response.Code != http.StatusAccepted {
		t.Fatalf("status = %d, body = %s", response.Code, response.Body.String())
	}
	var updated DesignDocumentResponse
	if err := json.NewDecoder(response.Body).Decode(&updated); err != nil {
		t.Fatal(err)
	}
	if updated.ActiveTask == nil || updated.ActiveTask.Operation != string(service.DesignDocumentManualEdit) {
		t.Fatalf("active task = %+v", updated.ActiveTask)
	}

	var taskContextJSON []byte
	if err := testPool.QueryRow(ctx, `
		SELECT task.context FROM design_document d, agent_task_queue task
		WHERE d.id = $1 AND task.id = d.active_task_id
	`, fixture.Document.ID).Scan(&taskContextJSON); err != nil {
		t.Fatalf("load task context: %v", err)
	}
	var taskContext struct {
		Operation      string `json:"operation"`
		ExecutionReady bool   `json:"execution_ready"`
		BaseRevisionID string `json:"base_revision_id"`
		Instruction    string `json:"instruction"`
		Input          struct {
			RepositoryGrounding string `json:"repository_grounding"`
		} `json:"input"`
		ManualEdits []struct {
			Page         string            `json:"page"`
			Selector     string            `json:"selector"`
			Declarations map[string]string `json:"declarations"`
		} `json:"manual_edits"`
	}
	if err := json.Unmarshal(taskContextJSON, &taskContext); err != nil {
		t.Fatal(err)
	}
	if taskContext.Operation != "manual_edit" || !taskContext.ExecutionReady {
		t.Fatalf("task context = %+v", taskContext)
	}
	// Pinned to the revision the designer was editing, so the run cannot land
	// on content they never saw.
	if taskContext.BaseRevisionID != uuidToString(fixture.Revision.ID) {
		t.Fatalf("base revision = %q, want %q", taskContext.BaseRevisionID, uuidToString(fixture.Revision.ID))
	}
	// Pinned grounding: a manual edit re-reads no repository, like an
	// adjustment (DC-059's envelope contract).
	if taskContext.Input.RepositoryGrounding != "pinned" {
		t.Fatalf("grounding = %q, want pinned", taskContext.Input.RepositoryGrounding)
	}
	if len(taskContext.ManualEdits) != 1 || taskContext.ManualEdits[0].Declarations["color"] != "#ff5701" {
		t.Fatalf("manual edits did not survive into the task: %+v", taskContext.ManualEdits)
	}
	// The timeline must say a person made this change, not the agent whose
	// runtime happened to run the gate.
	if !strings.Contains(taskContext.Instruction, "手动") {
		t.Fatalf("instruction does not read as manual: %q", taskContext.Instruction)
	}
}

// The whitelist is the security boundary; the endpoint has to enforce it
// before anything is enqueued, so a bad edit fails where the user can see it.
func TestManualEditRefusesEditsThatCouldEscapeTheirRule(t *testing.T) {
	fixture := createDesignDocumentRevisionFixture(t)
	agentID, _ := createProjectDesignSystemAgent(t, "online")

	for name, edits := range map[string][]map[string]any{
		"property off the panel": {{"page": "prototype/index.html", "selector": "#a", "declarations": map[string]string{"behavior": "url(x)"}}},
		"value closes the rule":  {{"page": "prototype/index.html", "selector": "#a", "declarations": map[string]string{"color": "red} body{display:none"}}},
		"page outside prototype": {{"page": "assets/logo.png", "selector": "#a", "declarations": map[string]string{"color": "#fff"}}},
		"nothing at all":         {},
	} {
		t.Run(name, func(t *testing.T) {
			response := performDesignDocumentManualEdit(t, uuidToString(fixture.Document.ID), map[string]any{
				"agent_id": agentID, "edits": edits,
			})
			if response.Code != http.StatusBadRequest || !strings.Contains(response.Body.String(), "manual_edits_invalid") {
				t.Fatalf("status = %d, body = %s", response.Code, response.Body.String())
			}
		})
	}
}

// Editing a revision that moved underneath the designer must be a detectable
// conflict, not a silent overwrite of someone else's work.
func TestManualEditRefusesAStaleBase(t *testing.T) {
	fixture := createDesignDocumentRevisionFixture(t)
	agentID, _ := createProjectDesignSystemAgent(t, "online")

	response := performDesignDocumentManualEdit(t, uuidToString(fixture.Document.ID), map[string]any{
		"agent_id":         agentID,
		"base_revision_id": "11111111-1111-4111-8111-111111111111",
		"edits": []map[string]any{{
			"page": "prototype/index.html", "selector": "#a", "declarations": map[string]string{"color": "#fff"},
		}},
	})
	if response.Code != http.StatusConflict || !strings.Contains(response.Body.String(), "base_revision_changed") {
		t.Fatalf("status = %d, body = %s", response.Code, response.Body.String())
	}
}
