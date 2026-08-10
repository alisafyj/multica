package handler

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/multica-ai/multica/server/internal/service"
)

// seedPreviewReadyPMORunForTest drives the full pipeline through the task
// handlers (config → run → completion with a valid snapshot) so the returned
// run is preview_ready and owned by testWorkspaceID.
func seedPreviewReadyPMORunForTest(t *testing.T) (PMOConfigResponse, PMORunResponse) {
	t.Helper()
	config := createPMOConfigForTest(t)
	run := startPMORunForTest(t, config.ID)
	markAgentTaskRunningForTest(t, *run.AgentTaskID)
	w := pmoCompleteTaskForTest(t, *run.AgentTaskID, validPMOSnapshotForTest(t))
	if w.Code != http.StatusOK {
		t.Fatalf("complete task: %d %s", w.Code, w.Body.String())
	}
	return config, run
}

func applyPMORunForTest(t *testing.T, runID string, body any) *httptest.ResponseRecorder {
	t.Helper()
	req := newRequest(http.MethodPost, "/api/pmo/runs/"+runID+"/apply", body)
	req = withURLParam(req, "id", runID)
	w := httptest.NewRecorder()
	testHandler.ApplyPMORun(w, req)
	return w
}

func TestApplyPMORunEndpointHappyPath(t *testing.T) {
	if testHandler == nil {
		t.Skip("database not available")
	}
	config, run := seedPreviewReadyPMORunForTest(t)

	w := applyPMORunForTest(t, run.ID, nil)
	if w.Code != http.StatusOK {
		t.Fatalf("apply: expected 200, got %d: %s", w.Code, w.Body.String())
	}
	var applied PMORunResponse
	if err := json.NewDecoder(w.Body).Decode(&applied); err != nil {
		t.Fatalf("decode apply response: %v", err)
	}
	if applied.Status != "applied" {
		t.Fatalf("status = %q, want applied", applied.Status)
	}
	if applied.ConfigID != config.ID {
		t.Fatalf("config_id = %q, want %q", applied.ConfigID, config.ID)
	}

	// Second apply: run is no longer preview_ready → 409.
	w = applyPMORunForTest(t, run.ID, nil)
	if w.Code != http.StatusConflict {
		t.Fatalf("second apply: expected 409, got %d: %s", w.Code, w.Body.String())
	}
}

func TestApplyPMORunEndpointRejectsInvalidResolutions(t *testing.T) {
	if testHandler == nil {
		t.Skip("database not available")
	}
	_, run := seedPreviewReadyPMORunForTest(t)

	w := applyPMORunForTest(t, run.ID, map[string]any{
		"conflict_resolutions": []map[string]any{
			{"external_type": "requirement", "external_key": "EXT-I-001", "field": "title", "choice": "sideways"},
		},
	})
	if w.Code != http.StatusBadRequest {
		t.Fatalf("bad choice: expected 400, got %d: %s", w.Code, w.Body.String())
	}

	// Run stays preview_ready after the rejected input.
	var status string
	if err := testPool.QueryRow(context.Background(), `SELECT status FROM pmo_sync_run WHERE id = $1`, run.ID).Scan(&status); err != nil {
		t.Fatalf("read run: %v", err)
	}
	if status != "preview_ready" {
		t.Fatalf("run status = %q, want preview_ready", status)
	}
}

func TestApplyPMORunEndpointUnknownRun(t *testing.T) {
	if testHandler == nil {
		t.Skip("database not available")
	}
	w := applyPMORunForTest(t, "0f2b6f6e-0000-4000-8000-000000000001", nil)
	if w.Code != http.StatusNotFound {
		t.Fatalf("unknown run: expected 404, got %d: %s", w.Code, w.Body.String())
	}
}

func TestApplyPMORunRejectsCrossWorkspaceMember(t *testing.T) {
	if testHandler == nil {
		t.Skip("database not available")
	}
	_, run := seedPreviewReadyPMORunForTest(t)

	req := newRequest(http.MethodPost, "/api/pmo/runs/"+run.ID+"/apply", nil)
	req = withURLParam(req, "id", run.ID)
	req.Header.Set("X-Workspace-ID", "0f2b6f6e-0000-4000-8000-000000000002")
	w := httptest.NewRecorder()
	testHandler.ApplyPMORun(w, req)
	if w.Code == http.StatusOK {
		t.Fatalf("cross-workspace apply must not succeed: %s", w.Body.String())
	}
}

func TestSetPMOAssigneeMappingEndpoint(t *testing.T) {
	if testHandler == nil {
		t.Skip("database not available")
	}
	config, _ := seedPreviewReadyPMORunForTest(t)

	req := newRequest(http.MethodPut, "/api/pmo/configs/"+config.ID+"/assignees/EXT-U-001", map[string]any{
		"member_id": testUserID,
	})
	req = withURLParams(req, "id", config.ID, "externalKey", "EXT-U-001")
	w := httptest.NewRecorder()
	testHandler.SetPMOAssigneeMapping(w, req)
	if w.Code != http.StatusOK {
		t.Fatalf("map member: expected 200, got %d: %s", w.Code, w.Body.String())
	}

	// Unknown member → 404.
	req = newRequest(http.MethodPut, "/api/pmo/configs/"+config.ID+"/assignees/EXT-U-001", map[string]any{
		"member_id": "0f2b6f6e-0000-4000-8000-000000000003",
	})
	req = withURLParams(req, "id", config.ID, "externalKey", "EXT-U-001")
	w = httptest.NewRecorder()
	testHandler.SetPMOAssigneeMapping(w, req)
	if w.Code != http.StatusNotFound {
		t.Fatalf("unknown member: expected 404, got %d: %s", w.Code, w.Body.String())
	}

	// Unknown config → 404.
	req = newRequest(http.MethodPut, "/api/pmo/configs/0f2b6f6e-0000-4000-8000-000000000001/assignees/EXT-U-001", map[string]any{
		"member_id": testUserID,
	})
	req = withURLParams(req, "id", "0f2b6f6e-0000-4000-8000-000000000001", "externalKey", "EXT-U-001")
	w = httptest.NewRecorder()
	testHandler.SetPMOAssigneeMapping(w, req)
	if w.Code != http.StatusNotFound {
		t.Fatalf("unknown config: expected 404, got %d: %s", w.Code, w.Body.String())
	}
}

// TestApplyPMORunUsesMappedAssigneeViaEndpoint exercises the endpoint +
// service pairing: an owner with an unmapped external owner first applies
// with the assignee left unassigned, then maps the member and re-applies.
func TestApplyPMORunUsesMappedAssigneeViaEndpoint(t *testing.T) {
	if testHandler == nil {
		t.Skip("database not available")
	}
	config := createPMOConfigForTest(t)
	run := startPMORunForTest(t, config.ID)
	markAgentTaskRunningForTest(t, *run.AgentTaskID)

	output := validPMOSnapshotForTest(t) // parent EXT-P-001, child EXT-I-001
	// Add an unmapped external owner to the parent requirement.
	var snapshot map[string]any
	if err := json.Unmarshal([]byte(output), &snapshot); err != nil {
		t.Fatalf("decode snapshot: %v", err)
	}
	parent := snapshot["parent_requirement"].(map[string]any)
	parent["owner"] = map[string]any{"external_id": "EXT-U-001", "display_name": "Fictional PMO Lead"}
	raw, _ := json.Marshal(snapshot)

	w := pmoCompleteTaskForTest(t, *run.AgentTaskID, string(raw))
	if w.Code != http.StatusOK {
		t.Fatalf("complete: %d %s", w.Code, w.Body.String())
	}

	// Map the external owner by member ID (never display name).
	mapReq := newRequest(http.MethodPut, "/api/pmo/configs/"+config.ID+"/assignees/EXT-U-001", map[string]any{
		"member_id": testUserID,
	})
	mapReq = withURLParams(mapReq, "id", config.ID, "externalKey", "EXT-U-001")
	mapW := httptest.NewRecorder()
	testHandler.SetPMOAssigneeMapping(mapW, mapReq)
	if mapW.Code != http.StatusOK {
		t.Fatalf("map assignee: %d %s", mapW.Code, mapW.Body.String())
	}

	applyW := applyPMORunForTest(t, run.ID, nil)
	if applyW.Code != http.StatusOK {
		t.Fatalf("apply: %d %s", applyW.Code, applyW.Body.String())
	}

	// The project lead is the mapped member — verified via project.lead_id.
	var projectID string
	if err := testPool.QueryRow(context.Background(), `
		SELECT l.local_id FROM pmo_sync_link l
		WHERE l.config_id = $1 AND l.external_type = 'requirement' AND l.external_key = 'EXT-P-001'
	`, config.ID).Scan(&projectID); err != nil {
		t.Fatalf("read project link: %v", err)
	}
	var leadID *string
	if err := testPool.QueryRow(context.Background(), `SELECT lead_id::text FROM project WHERE id = $1`, projectID).Scan(&leadID); err != nil {
		t.Fatalf("read project: %v", err)
	}
	if leadID == nil || *leadID != testUserID {
		t.Fatalf("project lead = %v, want mapped member %s", leadID, testUserID)
	}
}

var _ = service.PMOConflictResolution{}
