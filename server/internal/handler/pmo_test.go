package handler

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"reflect"
	"strings"
	"testing"
	"time"

	"github.com/jackc/pgx/v5/pgtype"
	"github.com/multica-ai/multica/server/internal/runtimeapps"
	db "github.com/multica-ai/multica/server/pkg/db/generated"
)

func TestCreatePMOConfigRejectsNonInvokableAgent(t *testing.T) {
	ctx := context.Background()
	suffix := time.Now().UnixNano()
	email := fmt.Sprintf("pmo-private-%d@example.test", suffix)
	var ownerID string
	if err := testPool.QueryRow(ctx, `INSERT INTO "user" (name, email) VALUES ('PMO Agent Owner', $1) RETURNING id`, email).Scan(&ownerID); err != nil {
		t.Fatal(err)
	}
	var agentID string
	if err := testPool.QueryRow(ctx, `
		INSERT INTO agent (
			workspace_id, name, description, runtime_mode, runtime_config,
			runtime_id, visibility, permission_mode, max_concurrent_tasks, owner_id
		) VALUES ($1, $2, '', 'cloud', '{}'::jsonb, $3, 'private', 'private', 1, $4)
		RETURNING id
	`, testWorkspaceID, fmt.Sprintf("PMO Private Agent %d", suffix), testRuntimeID, ownerID).Scan(&agentID); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() {
		_, _ = testPool.Exec(context.Background(), `DELETE FROM agent WHERE id = $1`, agentID)
		_, _ = testPool.Exec(context.Background(), `DELETE FROM "user" WHERE id = $1`, ownerID)
	})

	req := newRequest(http.MethodPost, "/api/pmo/configs", map[string]any{
		"name": "Example import", "agent_id": agentID, "root_external_key": "EXT-P-001",
	})
	w := httptest.NewRecorder()
	testHandler.CreatePMOConfig(w, req)
	if w.Code != http.StatusForbidden {
		t.Fatalf("got %d: %s", w.Code, w.Body.String())
	}
}

func TestUpdatePMOConfigRejectsScheduleBeforeFirstApply(t *testing.T) {
	config := createPMOConfigForTest(t)
	req := withURLParam(newRequest(http.MethodPut, "/api/pmo/configs/"+config.ID, map[string]any{
		"name": config.Name, "agent_id": config.AgentID,
		"root_external_key": config.RootExternalKey, "schedule_enabled": true,
	}), "id", config.ID)
	w := httptest.NewRecorder()
	testHandler.UpdatePMOConfig(w, req)
	if w.Code != http.StatusBadRequest {
		t.Fatalf("got %d: %s", w.Code, w.Body.String())
	}
}

func TestUpdatePMOConfigRejectsTrailingJSONBody(t *testing.T) {
	config := createPMOConfigForTest(t)
	body := fmt.Sprintf(`{"name":%q,"agent_id":%q,"root_external_key":%q,"schedule_enabled":false} {"extra":1}`,
		config.Name, config.AgentID, config.RootExternalKey)
	req := httptest.NewRequest(http.MethodPut, "/api/pmo/configs/"+config.ID, strings.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("X-User-ID", testUserID)
	req.Header.Set("X-Workspace-ID", testWorkspaceID)
	req = withURLParam(req, "id", config.ID)
	w := httptest.NewRecorder()
	testHandler.UpdatePMOConfig(w, req)
	if w.Code != http.StatusBadRequest {
		t.Fatalf("got %d: %s", w.Code, w.Body.String())
	}
}

func TestUpdatePMOConfigRejectsRootExternalKeyChangeAfterFirstApply(t *testing.T) {
	config := createPMOConfigForTest(t)
	if _, err := testPool.Exec(context.Background(),
		`UPDATE pmo_sync_config SET last_applied_at = now() WHERE id = $1 AND workspace_id = $2`,
		config.ID, testWorkspaceID); err != nil {
		t.Fatal(err)
	}
	req := withURLParam(newRequest(http.MethodPut, "/api/pmo/configs/"+config.ID, map[string]any{
		"name": config.Name, "agent_id": config.AgentID,
		"root_external_key": config.RootExternalKey + "-moved", "schedule_enabled": false,
	}), "id", config.ID)
	w := httptest.NewRecorder()
	testHandler.UpdatePMOConfig(w, req)
	if w.Code != http.StatusBadRequest {
		t.Fatalf("got %d: %s", w.Code, w.Body.String())
	}
}

func TestStartPMORunStampsRuntimeMCPOverlay(t *testing.T) {
	config := createPMOConfigForTest(t)
	withComposioMCPAppsFlag(t, testHandler, true)
	const overlayJSON = `{"mcpServers":{"pmo-test":{"url":"https://mcp.example.test/session"}}}`
	origBuilder := testHandler.TaskService.Composio
	testHandler.TaskService.Composio = pmoOverlayBuilder{overlay: overlayJSON}
	t.Cleanup(func() { testHandler.TaskService.Composio = origBuilder })

	req := withURLParam(newRequest(http.MethodPost, "/api/pmo/configs/"+config.ID+"/runs", nil), "id", config.ID)
	w := httptest.NewRecorder()
	testHandler.StartPMORun(w, req)
	if w.Code != http.StatusCreated {
		t.Fatalf("start run got %d: %s", w.Code, w.Body.String())
	}
	var run PMORunResponse
	if err := json.NewDecoder(w.Body).Decode(&run); err != nil {
		t.Fatal(err)
	}
	if run.AgentTaskID == nil {
		t.Fatal("run has no agent task")
	}
	var storedOverlay []byte
	if err := testPool.QueryRow(context.Background(),
		`SELECT runtime_mcp_overlay FROM agent_task_queue WHERE id = $1`, *run.AgentTaskID).Scan(&storedOverlay); err != nil {
		t.Fatal(err)
	}
	var got, want any
	if err := json.Unmarshal(storedOverlay, &got); err != nil {
		t.Fatalf("stored overlay is not JSON: %v", err)
	}
	if err := json.Unmarshal([]byte(overlayJSON), &want); err != nil {
		t.Fatal(err)
	}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("overlay = %s, want %s", storedOverlay, overlayJSON)
	}
}

// pmoOverlayBuilder is a deterministic fake for the Composio overlay seam so the
// PMO enqueue path can be checked without any real integration.
type pmoOverlayBuilder struct {
	overlay string
}

func (b pmoOverlayBuilder) BuildTaskOverlay(_ context.Context, _ pgtype.UUID, _ db.Agent) (runtimeapps.MCPOverlayResult, error) {
	return runtimeapps.MCPOverlayResult{MCPOverlay: json.RawMessage(b.overlay)}, nil
}

func TestStartPMORunRejectsSecondActiveRun(t *testing.T) {
	config := createPMOConfigForTest(t)
	start := func() *httptest.ResponseRecorder {
		req := withURLParam(newRequest(http.MethodPost, "/api/pmo/configs/"+config.ID+"/runs", nil), "id", config.ID)
		w := httptest.NewRecorder()
		testHandler.StartPMORun(w, req)
		return w
	}
	first := start()
	if first.Code != http.StatusCreated {
		t.Fatalf("first run got %d: %s", first.Code, first.Body.String())
	}
	second := start()
	if second.Code != http.StatusConflict {
		t.Fatalf("second run got %d: %s", second.Code, second.Body.String())
	}
}

func createPMOConfigForTest(t *testing.T) PMOConfigResponse {
	t.Helper()
	agentID := handlerTestAgentID(t)
	rootKey := fmt.Sprintf("EXT-P-%d", time.Now().UnixNano())
	req := newRequest(http.MethodPost, "/api/pmo/configs", map[string]any{
		"name": "Example import", "agent_id": agentID, "root_external_key": rootKey,
	})
	w := httptest.NewRecorder()
	testHandler.CreatePMOConfig(w, req)
	if w.Code != http.StatusCreated {
		t.Fatalf("create config got %d: %s", w.Code, w.Body.String())
	}
	var config PMOConfigResponse
	if err := json.NewDecoder(w.Body).Decode(&config); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() {
		_, _ = testPool.Exec(context.Background(), `DELETE FROM agent_task_queue WHERE id IN (SELECT agent_task_id FROM pmo_sync_run WHERE config_id = $1)`, config.ID)
		_, _ = testPool.Exec(context.Background(), `DELETE FROM pmo_sync_link WHERE config_id = $1`, config.ID)
		_, _ = testPool.Exec(context.Background(), `DELETE FROM pmo_sync_run WHERE config_id = $1`, config.ID)
		_, _ = testPool.Exec(context.Background(), `DELETE FROM pmo_sync_config WHERE id = $1`, config.ID)
	})
	return config
}
