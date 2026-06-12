package handler

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/go-chi/chi/v5"
	"github.com/multica-ai/multica/server/internal/auth"
	"github.com/multica-ai/multica/server/internal/service"
)

func createProjectForDesignTest(t *testing.T, title string) string {
	t.Helper()
	var id string
	if err := testPool.QueryRow(context.Background(), `
		INSERT INTO project (workspace_id, title, description, status, priority)
		VALUES ($1, $2, '', 'planned', 'medium')
		RETURNING id
	`, testWorkspaceID, title).Scan(&id); err != nil {
		t.Fatalf("insert project: %v", err)
	}
	t.Cleanup(func() { _, _ = testPool.Exec(context.Background(), `DELETE FROM project WHERE id = $1`, id) })
	return id
}

func createDesignFolderForTest(t *testing.T, projectID string, name string) string {
	t.Helper()
	var id string
	if err := testPool.QueryRow(context.Background(), `
		INSERT INTO design_folder (workspace_id, project_id, name)
		VALUES ($1, $2, $3)
		RETURNING id
	`, testWorkspaceID, projectID, name).Scan(&id); err != nil {
		t.Fatalf("insert design folder: %v", err)
	}
	return id
}

func minimalDesignNativeJSON(title string) map[string]any {
	return map[string]any{
		"version": "1.0",
		"file": map[string]any{
			"title":      title,
			"sourceType": "upload",
		},
		"frames": []map[string]any{{
			"id":          "frame-1",
			"name":        title,
			"rootLayerId": "layer-1",
			"width":       1440,
			"height":      900,
		}},
		"layers": map[string]any{
			"layer-1": map[string]any{
				"id":      "layer-1",
				"frameId": "frame-1",
				"name":    "Page",
				"type":    "frame",
				"visible": true,
				"x":       0,
				"y":       0,
				"width":   1440,
				"height":  900,
			},
		},
		"assets": map[string]any{},
	}
}

func figmaDesignNativeJSONWithSourceNodes(title string, sourceNodeIDs ...string) map[string]any {
	frames := make([]map[string]any, 0, len(sourceNodeIDs))
	layers := make(map[string]any, len(sourceNodeIDs))
	for i, sourceNodeID := range sourceNodeIDs {
		frameID := fmt.Sprintf("figma-frame-%d", i+1)
		layerID := fmt.Sprintf("figma-layer-%d", i+1)
		frames = append(frames, map[string]any{
			"id":           frameID,
			"sourceNodeId": sourceNodeID,
			"name":         fmt.Sprintf("Frame %d", i+1),
			"rootLayerId":  layerID,
			"width":        1440,
			"height":       900,
		})
		layers[layerID] = map[string]any{
			"id":           layerID,
			"sourceNodeId": sourceNodeID,
			"frameId":      frameID,
			"name":         fmt.Sprintf("Frame %d Root", i+1),
			"type":         "frame",
			"visible":      true,
			"x":            0,
			"y":            0,
			"width":        1440,
			"height":       900,
		}
	}
	return map[string]any{
		"version": "1.0",
		"file": map[string]any{
			"title":      title,
			"sourceType": "import",
		},
		"frames": frames,
		"layers": layers,
		"assets": map[string]any{},
		"source": map[string]any{"provider": "figma"},
	}
}

func frameCountFromNativeJSONForTest(t *testing.T, raw json.RawMessage) int {
	t.Helper()
	var doc map[string]any
	if err := json.Unmarshal(raw, &doc); err != nil {
		t.Fatalf("decode native json: %v", err)
	}
	frames, ok := doc["frames"].([]any)
	if !ok {
		t.Fatalf("native_json frames type = %T", doc["frames"])
	}
	return len(frames)
}

func contextDesignNativeJSON(title string) map[string]any {
	return map[string]any{
		"version": "1.0",
		"file": map[string]any{
			"title":      title,
			"sourceType": "upload",
		},
		"frames": []map[string]any{
			{
				"id":               "frame-main",
				"name":             "Main Screen",
				"rootLayerId":      "main-root",
				"width":            800,
				"height":           600,
				"previewAssetId":   "asset-preview-main",
				"thumbnailAssetId": "asset-thumb-main",
			},
			{
				"id":          "frame-secondary",
				"name":        "Secondary Screen",
				"rootLayerId": "secondary-root",
				"width":       400,
				"height":      300,
			},
		},
		"layers": map[string]any{
			"main-root": map[string]any{
				"id":      "main-root",
				"frameId": "frame-main",
				"name":    "Main Root",
				"type":    "frame",
				"visible": true,
				"x":       0,
				"y":       0,
				"width":   800,
				"height":  600,
			},
			"main-title": map[string]any{
				"id":      "main-title",
				"frameId": "frame-main",
				"name":    "Title",
				"type":    "text",
				"visible": true,
				"x":       40,
				"y":       40,
				"width":   200,
				"height":  50,
				"text": map[string]any{
					"text":       "Welcome",
					"fontFamily": "Inter",
					"fontSize":   24,
					"fontWeight": 700,
					"color":      map[string]any{"r": 0, "g": 0, "b": 0, "a": 1},
				},
			},
			"main-image": map[string]any{
				"id":      "main-image",
				"frameId": "frame-main",
				"name":    "Hero Image",
				"type":    "image",
				"visible": true,
				"x":       300,
				"y":       80,
				"width":   120,
				"height":  120,
				"image":   map[string]any{"assetId": "asset-hero"},
				"exportable": []map[string]any{{
					"assetId": "asset-export-main",
					"format":  "png",
					"url":     "https://example.test/export-main.png",
				}},
			},
			"main-offscreen": map[string]any{
				"id":      "main-offscreen",
				"frameId": "frame-main",
				"name":    "Offscreen",
				"type":    "rectangle",
				"visible": true,
				"x":       650,
				"y":       450,
				"width":   80,
				"height":  80,
			},
			"secondary-root": map[string]any{
				"id":      "secondary-root",
				"frameId": "frame-secondary",
				"name":    "Secondary Root",
				"type":    "frame",
				"visible": true,
				"x":       0,
				"y":       0,
				"width":   400,
				"height":  300,
			},
			"secondary-title": map[string]any{
				"id":      "secondary-title",
				"frameId": "frame-secondary",
				"name":    "Secondary Title",
				"type":    "text",
				"visible": true,
				"x":       20,
				"y":       20,
				"width":   160,
				"height":  40,
				"text":    map[string]any{"text": "Other", "fontFamily": "Inter", "fontSize": 18},
			},
		},
		"assets": map[string]any{
			"asset-preview-main": map[string]any{"url": "https://example.test/preview-main.png"},
			"asset-thumb-main":   map[string]any{"url": "https://example.test/thumb-main.png"},
			"asset-hero":         map[string]any{"url": "https://example.test/hero.png"},
			"asset-export-main":  map[string]any{"url": "https://example.test/export-main.png"},
			"asset-secondary":    map[string]any{"url": "https://example.test/secondary.png"},
		},
		"annotations": []map[string]any{{"frameId": "frame-main", "layerId": "main-title", "text": "Check copy"}},
	}
}

func createDesignFileForTest(t *testing.T, title string) DesignFileDetailResponse {
	t.Helper()

	req := newRequest("POST", "/api/design-files?workspace_id="+testWorkspaceID, map[string]any{
		"title":       title,
		"source_type": "upload",
		"native_json": minimalDesignNativeJSON(title),
	})
	w := httptest.NewRecorder()
	testHandler.CreateDesignFile(w, req)
	if w.Code != http.StatusCreated {
		t.Fatalf("CreateDesignFile: expected 201, got %d: %s", w.Code, w.Body.String())
	}

	var resp DesignFileDetailResponse
	if err := json.NewDecoder(w.Body).Decode(&resp); err != nil {
		t.Fatalf("decode CreateDesignFile response: %v", err)
	}
	t.Cleanup(func() {
		_, _ = testPool.Exec(context.Background(), `DELETE FROM design_file WHERE id = $1`, resp.File.ID)
	})
	return resp
}

func updateDesignRevisionNativeJSONForTest(t *testing.T, revisionID string, nativeJSON map[string]any) {
	t.Helper()
	raw, err := json.Marshal(nativeJSON)
	if err != nil {
		t.Fatalf("marshal native json: %v", err)
	}
	if _, err := testPool.Exec(context.Background(), `UPDATE design_revision SET native_json = $1 WHERE id = $2`, raw, revisionID); err != nil {
		t.Fatalf("update design revision native json: %v", err)
	}
}

func withDesignURLParams(req *http.Request, kv ...string) *http.Request {
	rctx := chi.NewRouteContext()
	for i := 0; i+1 < len(kv); i += 2 {
		rctx.URLParams.Add(kv[i], kv[i+1])
	}
	return req.WithContext(context.WithValue(req.Context(), chi.RouteCtxKey, rctx))
}

func TestCreateDesignFileCreatesCurrentRevision(t *testing.T) {
	resp := createDesignFileForTest(t, "Handler Test Design")
	if resp.File.ID == "" {
		t.Fatal("expected file id")
	}
	if resp.File.CurrentRevisionID == nil || *resp.File.CurrentRevisionID == "" {
		t.Fatal("expected current revision id")
	}
	if resp.CurrentRevision == nil {
		t.Fatal("expected current revision")
	}
	if resp.CurrentRevision.RevisionNumber != 1 {
		t.Fatalf("revision number = %d, want 1", resp.CurrentRevision.RevisionNumber)
	}
	if resp.CurrentRevision.Status != "valid" {
		t.Fatalf("revision status = %q, want valid", resp.CurrentRevision.Status)
	}
}

func TestPublishDesignRevisionAsTemplateAndListGet(t *testing.T) {
	design := createDesignFileForTest(t, "Template Source Design")
	if design.CurrentRevision == nil {
		t.Fatal("expected current revision")
	}
	libraryKey := fmt.Sprintf("test-library-%d", time.Now().UnixNano())
	templateKey := fmt.Sprintf("test-template-%d", time.Now().UnixNano())
	t.Cleanup(func() {
		_, _ = testPool.Exec(context.Background(), `DELETE FROM design_template_library WHERE workspace_id = $1 AND key = $2`, testWorkspaceID, libraryKey)
	})

	req := newRequest("POST", "/api/design-revisions/"+design.CurrentRevision.ID+"/publish-template?workspace_id="+testWorkspaceID, map[string]any{
		"library_key":  libraryKey,
		"library_name": "Test Template Library",
		"template_key": templateKey,
		"name":         "Checkout Template",
		"description":  "Reusable checkout screen",
		"category":     "checkout",
		"slot_schema":  map[string]any{"title": map[string]any{"type": "text"}},
		"metadata":     map[string]any{"source": "handler-test"},
	})
	req = withDesignURLParams(req, "revisionId", design.CurrentRevision.ID)
	w := httptest.NewRecorder()
	testHandler.PublishDesignRevisionAsTemplate(w, req)
	if w.Code != http.StatusCreated {
		t.Fatalf("PublishDesignRevisionAsTemplate: expected 201, got %d: %s", w.Code, w.Body.String())
	}
	var published DesignCatalogTemplateResponse
	if err := json.NewDecoder(w.Body).Decode(&published); err != nil {
		t.Fatalf("decode publish response: %v", err)
	}
	if published.Key != templateKey || published.Name != "Checkout Template" || published.Category != "checkout" {
		t.Fatalf("unexpected published template: %+v", published)
	}
	if published.DesignRevisionID == nil || *published.DesignRevisionID != design.CurrentRevision.ID {
		t.Fatalf("design_revision_id = %v, want %s", published.DesignRevisionID, design.CurrentRevision.ID)
	}
	if published.TemplateRevisionNumber == nil || *published.TemplateRevisionNumber != 1 {
		t.Fatalf("template_revision_number = %v, want 1", published.TemplateRevisionNumber)
	}

	listReq := newRequest("GET", "/api/design-templates?workspace_id="+testWorkspaceID+"&category=checkout", nil)
	listW := httptest.NewRecorder()
	testHandler.ListDesignCatalogTemplates(listW, listReq)
	if listW.Code != http.StatusOK {
		t.Fatalf("ListDesignCatalogTemplates: expected 200, got %d: %s", listW.Code, listW.Body.String())
	}
	var listResp struct {
		Templates []DesignCatalogTemplateResponse `json:"templates"`
		Total     int                             `json:"total"`
	}
	if err := json.NewDecoder(listW.Body).Decode(&listResp); err != nil {
		t.Fatalf("decode list response: %v", err)
	}
	found := false
	for _, item := range listResp.Templates {
		if item.ID == published.ID {
			found = true
			break
		}
	}
	if !found {
		t.Fatalf("published template %s not found in list: %+v", published.ID, listResp.Templates)
	}

	getReq := newRequest("GET", "/api/design-templates/"+published.ID+"?workspace_id="+testWorkspaceID, nil)
	getReq = withDesignURLParams(getReq, "id", published.ID)
	getW := httptest.NewRecorder()
	testHandler.GetDesignCatalogTemplate(getW, getReq)
	if getW.Code != http.StatusOK {
		t.Fatalf("GetDesignCatalogTemplate: expected 200, got %d: %s", getW.Code, getW.Body.String())
	}
	var got DesignCatalogTemplateResponse
	if err := json.NewDecoder(getW.Body).Decode(&got); err != nil {
		t.Fatalf("decode get response: %v", err)
	}
	if got.ID != published.ID || got.DesignRevisionID == nil || *got.DesignRevisionID != design.CurrentRevision.ID {
		t.Fatalf("unexpected get response: %+v", got)
	}
}

func createCatalogTemplateForDraftTest(t *testing.T) DesignCatalogTemplateResponse {
	t.Helper()
	design := createDesignFileForTest(t, "Draft Template Source")
	if design.CurrentRevision == nil {
		t.Fatal("expected current revision")
	}
	libraryKey := fmt.Sprintf("draft-library-%d", time.Now().UnixNano())
	templateKey := fmt.Sprintf("draft-template-%d", time.Now().UnixNano())
	t.Cleanup(func() {
		_, _ = testPool.Exec(context.Background(), `DELETE FROM design_template_library WHERE workspace_id = $1 AND key = $2`, testWorkspaceID, libraryKey)
	})
	req := newRequest("POST", "/api/design-revisions/"+design.CurrentRevision.ID+"/publish-template?workspace_id="+testWorkspaceID, map[string]any{
		"library_key":  libraryKey,
		"template_key": templateKey,
		"name":         "Draftable Template",
	})
	req = withDesignURLParams(req, "revisionId", design.CurrentRevision.ID)
	w := httptest.NewRecorder()
	testHandler.PublishDesignRevisionAsTemplate(w, req)
	if w.Code != http.StatusCreated {
		t.Fatalf("PublishDesignRevisionAsTemplate: expected 201, got %d: %s", w.Code, w.Body.String())
	}
	var published DesignCatalogTemplateResponse
	if err := json.NewDecoder(w.Body).Decode(&published); err != nil {
		t.Fatalf("decode publish response: %v", err)
	}
	return published
}

func createCatalogTemplateWithTextSlotForDraftTest(t *testing.T) DesignCatalogTemplateResponse {
	t.Helper()
	design := createDesignFileForTest(t, "Draft Template With Slot")
	if design.CurrentRevision == nil {
		t.Fatal("expected current revision")
	}
	nativeJSON := minimalDesignNativeJSON("Draft Template With Slot")
	layers := nativeJSON["layers"].(map[string]any)
	layers["title-layer"] = map[string]any{
		"id":      "title-layer",
		"frameId": "frame-1",
		"name":    "Title",
		"type":    "text",
		"visible": true,
		"x":       40,
		"y":       40,
		"width":   320,
		"height":  48,
		"text": map[string]any{
			"characters": "Original title",
			"text":       "Original title",
			"fontSize":   24,
		},
	}
	nativeJSON["slots"] = map[string]any{"title": map[string]any{"slotKey": "title", "layerIds": []any{"title-layer"}}}
	updateDesignRevisionNativeJSONForTest(t, design.CurrentRevision.ID, nativeJSON)
	libraryKey := fmt.Sprintf("slot-draft-library-%d", time.Now().UnixNano())
	templateKey := fmt.Sprintf("slot-draft-template-%d", time.Now().UnixNano())
	t.Cleanup(func() {
		_, _ = testPool.Exec(context.Background(), `DELETE FROM design_template_library WHERE workspace_id = $1 AND key = $2`, testWorkspaceID, libraryKey)
	})
	req := newRequest("POST", "/api/design-revisions/"+design.CurrentRevision.ID+"/publish-template?workspace_id="+testWorkspaceID, map[string]any{
		"library_key":  libraryKey,
		"template_key": templateKey,
		"name":         "Draftable Slot Template",
		"slot_schema":  map[string]any{"title": map[string]any{"type": "text"}},
	})
	req = withDesignURLParams(req, "revisionId", design.CurrentRevision.ID)
	w := httptest.NewRecorder()
	testHandler.PublishDesignRevisionAsTemplate(w, req)
	if w.Code != http.StatusCreated {
		t.Fatalf("PublishDesignRevisionAsTemplate: expected 201, got %d: %s", w.Code, w.Body.String())
	}
	var published DesignCatalogTemplateResponse
	if err := json.NewDecoder(w.Body).Decode(&published); err != nil {
		t.Fatalf("decode publish response: %v", err)
	}
	return published
}

func handlerTestAgentID(t *testing.T) string {
	t.Helper()
	var id string
	if err := testPool.QueryRow(context.Background(), `SELECT id FROM agent WHERE workspace_id = $1 AND name = 'Handler Test Agent' LIMIT 1`, testWorkspaceID).Scan(&id); err != nil {
		t.Fatalf("get handler test agent: %v", err)
	}
	return id
}

func TestCreateDesignDraftFromCatalogTemplate(t *testing.T) {
	template := createCatalogTemplateForDraftTest(t)
	req := newRequest("POST", "/api/design-drafts?workspace_id="+testWorkspaceID, map[string]any{
		"catalog_template_id": template.ID,
		"title":               "Generated Checkout Draft",
		"requirement_core":    map[string]any{"version": "1.0", "title": "Checkout"},
		"slot_values":         map[string]any{"title": "Pay now"},
		"patch":               []map[string]any{{"op": "replace", "path": "/layers/main-title/text/text", "value": "Pay now"}},
	})
	w := httptest.NewRecorder()
	testHandler.CreateDesignDraft(w, req)
	if w.Code != http.StatusCreated {
		t.Fatalf("CreateDesignDraft: expected 201, got %d: %s", w.Code, w.Body.String())
	}
	var draft DesignDraftResponse
	if err := json.NewDecoder(w.Body).Decode(&draft); err != nil {
		t.Fatalf("decode draft response: %v", err)
	}
	t.Cleanup(func() { _, _ = testPool.Exec(context.Background(), `DELETE FROM design_draft WHERE id = $1`, draft.ID) })
	if draft.CatalogTemplateID == nil || *draft.CatalogTemplateID != template.ID {
		t.Fatalf("catalog_template_id = %v, want %s", draft.CatalogTemplateID, template.ID)
	}
	if draft.TemplateRevisionID == nil || template.CurrentRevisionID == nil || *draft.TemplateRevisionID != *template.CurrentRevisionID {
		t.Fatalf("template_revision_id = %v, want %v", draft.TemplateRevisionID, template.CurrentRevisionID)
	}
	if draft.Status != "generated" {
		t.Fatalf("status = %q, want generated", draft.Status)
	}

	listReq := newRequest("GET", "/api/design-drafts?workspace_id="+testWorkspaceID, nil)
	listW := httptest.NewRecorder()
	testHandler.ListDesignDrafts(listW, listReq)
	if listW.Code != http.StatusOK {
		t.Fatalf("ListDesignDrafts: expected 200, got %d: %s", listW.Code, listW.Body.String())
	}

	getReq := newRequest("GET", "/api/design-drafts/"+draft.ID+"?workspace_id="+testWorkspaceID, nil)
	getReq = withDesignURLParams(getReq, "id", draft.ID)
	getW := httptest.NewRecorder()
	testHandler.GetDesignDraft(getW, getReq)
	if getW.Code != http.StatusOK {
		t.Fatalf("GetDesignDraft: expected 200, got %d: %s", getW.Code, getW.Body.String())
	}
}

func TestCreateDesignDraftAgentTaskEnqueuesTaskContext(t *testing.T) {
	template := createCatalogTemplateForDraftTest(t)
	agentID := handlerTestAgentID(t)
	req := newRequest("POST", "/api/design-drafts/agent-tasks?workspace_id="+testWorkspaceID, map[string]any{
		"agent_id":            agentID,
		"catalog_template_id": template.ID,
		"title":               "Agent Draft",
		"prompt":              "Generate a draft",
		"requirement_core":    map[string]any{"version": "1.0", "title": "Agent Draft"},
	})
	w := httptest.NewRecorder()
	testHandler.CreateDesignDraftAgentTask(w, req)
	if w.Code != http.StatusAccepted {
		t.Fatalf("CreateDesignDraftAgentTask: expected 202, got %d: %s", w.Code, w.Body.String())
	}
	var resp CreateDesignDraftAgentTaskResponse
	if err := json.NewDecoder(w.Body).Decode(&resp); err != nil {
		t.Fatalf("decode task response: %v", err)
	}
	t.Cleanup(func() {
		_, _ = testPool.Exec(context.Background(), `DELETE FROM agent_task_queue WHERE id = $1`, resp.TaskID)
	})
	var contextRaw []byte
	if err := testPool.QueryRow(context.Background(), `SELECT context FROM agent_task_queue WHERE id = $1`, resp.TaskID).Scan(&contextRaw); err != nil {
		t.Fatalf("get task context: %v", err)
	}
	var payload map[string]any
	if err := json.Unmarshal(contextRaw, &payload); err != nil {
		t.Fatalf("decode task context: %v", err)
	}
	if payload["type"] != "ui_agent_draft_create" {
		t.Fatalf("context type = %v, want ui_agent_draft_create", payload["type"])
	}
	if payload["catalog_template_id"] != template.ID {
		t.Fatalf("catalog_template_id = %v, want %s", payload["catalog_template_id"], template.ID)
	}
	if _, ok := payload["output_policy"].(map[string]any); !ok {
		t.Fatalf("expected output_policy in context: %+v", payload)
	}
}

func TestClaimUIDraftCreateTaskReturnsContext(t *testing.T) {
	template := createCatalogTemplateForDraftTest(t)
	agentID := handlerTestAgentID(t)
	req := newRequest("POST", "/api/design-drafts/agent-tasks?workspace_id="+testWorkspaceID, map[string]any{
		"agent_id":            agentID,
		"catalog_template_id": template.ID,
		"title":               "Claimed Agent Draft",
		"prompt":              "Generate draft JSON",
	})
	w := httptest.NewRecorder()
	testHandler.CreateDesignDraftAgentTask(w, req)
	if w.Code != http.StatusAccepted {
		t.Fatalf("CreateDesignDraftAgentTask: expected 202, got %d: %s", w.Code, w.Body.String())
	}
	var created CreateDesignDraftAgentTaskResponse
	if err := json.NewDecoder(w.Body).Decode(&created); err != nil {
		t.Fatalf("decode task response: %v", err)
	}
	t.Cleanup(func() {
		_, _ = testPool.Exec(context.Background(), `DELETE FROM agent_task_queue WHERE id = $1`, created.TaskID)
	})

	claimReq := newDaemonTokenRequest("POST", "/api/daemon/runtimes/"+testRuntimeID+"/tasks/claim", nil, testWorkspaceID, "ui-draft-claim")
	claimReq = withURLParam(claimReq, "runtimeId", testRuntimeID)
	claimW := httptest.NewRecorder()
	testHandler.ClaimTaskByRuntime(claimW, claimReq)
	if claimW.Code != http.StatusOK {
		t.Fatalf("ClaimTaskByRuntime: expected 200, got %d: %s", claimW.Code, claimW.Body.String())
	}
	var claimResp struct {
		Task *struct {
			ID                   string          `json:"id"`
			WorkspaceID          string          `json:"workspace_id"`
			UIDraftCreateContext json.RawMessage `json:"ui_draft_create_context"`
		} `json:"task"`
	}
	if err := json.Unmarshal(claimW.Body.Bytes(), &claimResp); err != nil {
		t.Fatalf("decode claim response: %v", err)
	}
	if claimResp.Task == nil || claimResp.Task.ID != created.TaskID {
		t.Fatalf("claimed task = %+v, want %s", claimResp.Task, created.TaskID)
	}
	if claimResp.Task.WorkspaceID != testWorkspaceID {
		t.Fatalf("workspace_id = %q, want %q", claimResp.Task.WorkspaceID, testWorkspaceID)
	}
	var ctxPayload map[string]any
	if err := json.Unmarshal(claimResp.Task.UIDraftCreateContext, &ctxPayload); err != nil {
		t.Fatalf("decode ui draft context: %v", err)
	}
	if ctxPayload["type"] != "ui_agent_draft_create" {
		t.Fatalf("context type = %v", ctxPayload["type"])
	}
}

func TestCompleteUIDraftCreateTaskCreatesDraft(t *testing.T) {
	template := createCatalogTemplateForDraftTest(t)
	agentID := handlerTestAgentID(t)
	req := newRequest("POST", "/api/design-drafts/agent-tasks?workspace_id="+testWorkspaceID, map[string]any{
		"agent_id":            agentID,
		"catalog_template_id": template.ID,
		"title":               "Task Draft Title",
	})
	w := httptest.NewRecorder()
	testHandler.CreateDesignDraftAgentTask(w, req)
	if w.Code != http.StatusAccepted {
		t.Fatalf("CreateDesignDraftAgentTask: expected 202, got %d: %s", w.Code, w.Body.String())
	}
	var created CreateDesignDraftAgentTaskResponse
	if err := json.NewDecoder(w.Body).Decode(&created); err != nil {
		t.Fatalf("decode task response: %v", err)
	}
	t.Cleanup(func() {
		_, _ = testPool.Exec(context.Background(), `DELETE FROM agent_task_queue WHERE id = $1`, created.TaskID)
	})
	if _, err := testPool.Exec(context.Background(), `UPDATE agent_task_queue SET status = 'running', started_at = now() WHERE id = $1`, created.TaskID); err != nil {
		t.Fatalf("mark task running: %v", err)
	}
	output := map[string]any{
		"title":            "Agent Generated Draft",
		"requirement_core": map[string]any{"version": "1.0", "title": "Agent Generated Draft"},
		"slot_values":      map[string]any{"title": "Pay now"},
		"patch":            []any{},
	}
	outputJSON, _ := json.Marshal(output)
	completeReq := newDaemonTokenRequest("POST", "/api/daemon/tasks/"+created.TaskID+"/complete", map[string]any{"output": string(outputJSON)}, testWorkspaceID, "ui-draft-complete")
	completeReq = withURLParam(completeReq, "taskId", created.TaskID)
	completeW := httptest.NewRecorder()
	testHandler.CompleteTask(completeW, completeReq)
	if completeW.Code != http.StatusOK {
		t.Fatalf("CompleteTask: expected 200, got %d: %s", completeW.Code, completeW.Body.String())
	}
	var draftID string
	if err := testPool.QueryRow(context.Background(), `SELECT id FROM design_draft WHERE workspace_id = $1 AND title = 'Agent Generated Draft' ORDER BY created_at DESC LIMIT 1`, testWorkspaceID).Scan(&draftID); err != nil {
		t.Fatalf("expected created design draft: %v", err)
	}
	t.Cleanup(func() { _, _ = testPool.Exec(context.Background(), `DELETE FROM design_draft WHERE id = $1`, draftID) })
}

func TestCreateDesignDraftRejectsLayoutPatch(t *testing.T) {
	template := createCatalogTemplateForDraftTest(t)
	req := newRequest("POST", "/api/design-drafts?workspace_id="+testWorkspaceID, map[string]any{
		"catalog_template_id": template.ID,
		"patch":               []map[string]any{{"op": "replace", "path": "/layers/main-title/x", "value": 10}},
	})
	w := httptest.NewRecorder()
	testHandler.CreateDesignDraft(w, req)
	if w.Code != http.StatusBadRequest {
		t.Fatalf("CreateDesignDraft layout patch: expected 400, got %d: %s", w.Code, w.Body.String())
	}
}

func TestCreateDesignDraftValidatesTemplateSlotSchema(t *testing.T) {
	design := createDesignFileForTest(t, "Slot Schema Template Source")
	if design.CurrentRevision == nil {
		t.Fatal("expected current revision")
	}
	libraryKey := fmt.Sprintf("slot-schema-library-%d", time.Now().UnixNano())
	t.Cleanup(func() {
		_, _ = testPool.Exec(context.Background(), `DELETE FROM design_template_library WHERE workspace_id = $1 AND key = $2`, testWorkspaceID, libraryKey)
	})
	publishReq := newRequest("POST", "/api/design-revisions/"+design.CurrentRevision.ID+"/publish-template?workspace_id="+testWorkspaceID, map[string]any{
		"library_key":  libraryKey,
		"template_key": fmt.Sprintf("slot-schema-template-%d", time.Now().UnixNano()),
		"name":         "Slot Schema Template",
		"slot_schema": map[string]any{
			"title": map[string]any{"type": "text", "required": true},
			"count": map[string]any{"type": "number"},
		},
	})
	publishReq = withDesignURLParams(publishReq, "revisionId", design.CurrentRevision.ID)
	publishW := httptest.NewRecorder()
	testHandler.PublishDesignRevisionAsTemplate(publishW, publishReq)
	if publishW.Code != http.StatusCreated {
		t.Fatalf("PublishDesignRevisionAsTemplate: expected 201, got %d: %s", publishW.Code, publishW.Body.String())
	}
	var template DesignCatalogTemplateResponse
	if err := json.NewDecoder(publishW.Body).Decode(&template); err != nil {
		t.Fatalf("decode publish response: %v", err)
	}
	if len(template.SlotSchema) == 0 || string(template.SlotSchema) == "null" {
		t.Fatal("expected published template response to include slot_schema")
	}

	missingReq := newRequest("POST", "/api/design-drafts?workspace_id="+testWorkspaceID, map[string]any{
		"catalog_template_id": template.ID,
		"slot_values":         map[string]any{"count": 1},
	})
	missingW := httptest.NewRecorder()
	testHandler.CreateDesignDraft(missingW, missingReq)
	if missingW.Code != http.StatusBadRequest {
		t.Fatalf("missing required slot: expected 400, got %d: %s", missingW.Code, missingW.Body.String())
	}

	typeReq := newRequest("POST", "/api/design-drafts?workspace_id="+testWorkspaceID, map[string]any{
		"catalog_template_id": template.ID,
		"slot_values":         map[string]any{"title": "Hello", "count": "not-number"},
	})
	typeW := httptest.NewRecorder()
	testHandler.CreateDesignDraft(typeW, typeReq)
	if typeW.Code != http.StatusBadRequest {
		t.Fatalf("wrong slot type: expected 400, got %d: %s", typeW.Code, typeW.Body.String())
	}

	validReq := newRequest("POST", "/api/design-drafts?workspace_id="+testWorkspaceID, map[string]any{
		"catalog_template_id": template.ID,
		"slot_values":         map[string]any{"title": "Hello", "count": 2},
	})
	validW := httptest.NewRecorder()
	testHandler.CreateDesignDraft(validW, validReq)
	if validW.Code != http.StatusCreated {
		t.Fatalf("valid slots: expected 201, got %d: %s", validW.Code, validW.Body.String())
	}
	var draft DesignDraftResponse
	if err := json.NewDecoder(validW.Body).Decode(&draft); err != nil {
		t.Fatalf("decode draft response: %v", err)
	}
	t.Cleanup(func() { _, _ = testPool.Exec(context.Background(), `DELETE FROM design_draft WHERE id = $1`, draft.ID) })
}

func TestMaterializeDesignDraftCreatesGeneratedDesign(t *testing.T) {
	template := createCatalogTemplateWithTextSlotForDraftTest(t)
	createReq := newRequest("POST", "/api/design-drafts?workspace_id="+testWorkspaceID, map[string]any{
		"catalog_template_id": template.ID,
		"title":               "Materialized Draft",
		"slot_values":         map[string]any{"title": "Slot title"},
		"patch":               []map[string]any{{"op": "replace", "path": "/layers/layer-1/name", "value": "Generated Page"}},
	})
	createW := httptest.NewRecorder()
	testHandler.CreateDesignDraft(createW, createReq)
	if createW.Code != http.StatusCreated {
		t.Fatalf("CreateDesignDraft: expected 201, got %d: %s", createW.Code, createW.Body.String())
	}
	var draft DesignDraftResponse
	if err := json.NewDecoder(createW.Body).Decode(&draft); err != nil {
		t.Fatalf("decode draft response: %v", err)
	}
	t.Cleanup(func() { _, _ = testPool.Exec(context.Background(), `DELETE FROM design_draft WHERE id = $1`, draft.ID) })

	matReq := newRequest("POST", "/api/design-drafts/"+draft.ID+"/materialize?workspace_id="+testWorkspaceID, nil)
	matReq = withDesignURLParams(matReq, "id", draft.ID)
	matW := httptest.NewRecorder()
	testHandler.MaterializeDesignDraft(matW, matReq)
	if matW.Code != http.StatusCreated {
		t.Fatalf("MaterializeDesignDraft: expected 201, got %d: %s", matW.Code, matW.Body.String())
	}
	var resp DesignDraftMaterializeResponse
	if err := json.NewDecoder(matW.Body).Decode(&resp); err != nil {
		t.Fatalf("decode materialize response: %v", err)
	}
	t.Cleanup(func() {
		_, _ = testPool.Exec(context.Background(), `DELETE FROM design_file WHERE id = $1`, resp.DesignFile.File.ID)
	})
	if resp.DesignFile.File.SourceType != "ai_generated" {
		t.Fatalf("source_type = %q, want ai_generated", resp.DesignFile.File.SourceType)
	}
	if resp.Draft.Status != "validated" {
		t.Fatalf("draft status = %q, want validated", resp.Draft.Status)
	}
	if resp.Draft.GeneratedFileID == nil || *resp.Draft.GeneratedFileID != resp.DesignFile.File.ID {
		t.Fatalf("generated_file_id = %v, want %s", resp.Draft.GeneratedFileID, resp.DesignFile.File.ID)
	}
	if resp.Draft.MaterializedAt == nil || *resp.Draft.MaterializedAt == "" {
		t.Fatal("expected materialized_at")
	}
	if resp.DesignFile.CurrentRevision == nil {
		t.Fatal("expected generated current revision")
	}
	var native map[string]any
	if err := json.Unmarshal(resp.DesignFile.CurrentRevision.NativeJSON, &native); err != nil {
		t.Fatalf("decode generated native json: %v", err)
	}
	layers := native["layers"].(map[string]any)
	layer := layers["layer-1"].(map[string]any)
	if layer["name"] != "Generated Page" {
		t.Fatalf("materialized layer name = %q, want Generated Page", layer["name"])
	}
	titleLayer := layers["title-layer"].(map[string]any)
	text := titleLayer["text"].(map[string]any)
	if text["characters"] != "Slot title" || text["text"] != "Slot title" {
		t.Fatalf("materialized slot text = %#v, want Slot title", text)
	}
}

func TestCreateDesignFileWithProjectAndFolder(t *testing.T) {
	projectID := createProjectForDesignTest(t, "Design Project")
	folderID := createDesignFolderForTest(t, projectID, "App Screens")
	req := newRequest("POST", "/api/design-files?workspace_id="+testWorkspaceID, map[string]any{
		"title":       "Project Design",
		"project_id":  projectID,
		"folder_id":   folderID,
		"source_type": "upload",
		"native_json": minimalDesignNativeJSON("Project Design"),
	})
	w := httptest.NewRecorder()
	testHandler.CreateDesignFile(w, req)
	if w.Code != http.StatusCreated {
		t.Fatalf("CreateDesignFile with project/folder: expected 201, got %d: %s", w.Code, w.Body.String())
	}
	var resp DesignFileDetailResponse
	if err := json.NewDecoder(w.Body).Decode(&resp); err != nil {
		t.Fatalf("decode response: %v", err)
	}
	if resp.File.ProjectID == nil || *resp.File.ProjectID != projectID {
		t.Fatalf("project_id = %v, want %s", resp.File.ProjectID, projectID)
	}
	if resp.File.FolderID == nil || *resp.File.FolderID != folderID {
		t.Fatalf("folder_id = %v, want %s", resp.File.FolderID, folderID)
	}
	t.Cleanup(func() {
		_, _ = testPool.Exec(context.Background(), `DELETE FROM design_file WHERE id = $1`, resp.File.ID)
	})
}

func TestCreateDesignFileRejectsFolderFromAnotherProject(t *testing.T) {
	projectID := createProjectForDesignTest(t, "Design Project A")
	otherProjectID := createProjectForDesignTest(t, "Design Project B")
	folderID := createDesignFolderForTest(t, otherProjectID, "Other Project Folder")
	req := newRequest("POST", "/api/design-files?workspace_id="+testWorkspaceID, map[string]any{
		"title":       "Wrong Folder Design",
		"project_id":  projectID,
		"folder_id":   folderID,
		"source_type": "upload",
		"native_json": minimalDesignNativeJSON("Wrong Folder Design"),
	})
	w := httptest.NewRecorder()
	testHandler.CreateDesignFile(w, req)
	if w.Code != http.StatusBadRequest {
		t.Fatalf("CreateDesignFile wrong folder: expected 400, got %d: %s", w.Code, w.Body.String())
	}
}

func TestCreateDesignFileRejectsInvalidNativeJSON(t *testing.T) {
	req := newRequest("POST", "/api/design-files?workspace_id="+testWorkspaceID, map[string]any{
		"title":       "Broken Design",
		"source_type": "upload",
		"native_json": map[string]any{
			"version": "1.0",
			"file":    map[string]any{"title": "Broken", "sourceType": "upload"},
			"frames":  []map[string]any{{"id": "frame-1", "name": "Broken", "rootLayerId": "missing", "width": 100, "height": 100}},
			"layers":  map[string]any{},
			"assets":  map[string]any{},
		},
	})
	w := httptest.NewRecorder()
	testHandler.CreateDesignFile(w, req)
	if w.Code != http.StatusBadRequest {
		t.Fatalf("CreateDesignFile invalid JSON: expected 400, got %d: %s", w.Code, w.Body.String())
	}
}

func TestListAndGetDesignFiles(t *testing.T) {
	created := createDesignFileForTest(t, "Listable Design")

	listReq := newRequest("GET", "/api/design-files?workspace_id="+testWorkspaceID, nil)
	listW := httptest.NewRecorder()
	testHandler.ListDesignFiles(listW, listReq)
	if listW.Code != http.StatusOK {
		t.Fatalf("ListDesignFiles: expected 200, got %d: %s", listW.Code, listW.Body.String())
	}
	var listResp struct {
		DesignFiles []DesignFileResponse `json:"design_files"`
		Total       int                  `json:"total"`
	}
	if err := json.NewDecoder(listW.Body).Decode(&listResp); err != nil {
		t.Fatalf("decode ListDesignFiles: %v", err)
	}
	found := false
	for _, file := range listResp.DesignFiles {
		if file.ID == created.File.ID {
			found = true
			break
		}
	}
	if !found {
		t.Fatalf("created design file %s not found in list", created.File.ID)
	}

	getReq := withURLParam(newRequest("GET", "/api/design-files/"+created.File.ID+"?workspace_id="+testWorkspaceID, nil), "id", created.File.ID)
	getW := httptest.NewRecorder()
	testHandler.GetDesignFile(getW, getReq)
	if getW.Code != http.StatusOK {
		t.Fatalf("GetDesignFile: expected 200, got %d: %s", getW.Code, getW.Body.String())
	}
}

func TestGetDesignFileContextReturnsSummaryWithoutNativeJSON(t *testing.T) {
	created := createDesignFileForTest(t, "Context Design")
	req := withURLParam(newRequest("GET", "/api/design-files/"+created.File.ID+"/context?workspace_id="+testWorkspaceID, nil), "id", created.File.ID)
	w := httptest.NewRecorder()
	testHandler.GetDesignFileContext(w, req)
	if w.Code != http.StatusOK {
		t.Fatalf("GetDesignFileContext: expected 200, got %d: %s", w.Code, w.Body.String())
	}

	var resp map[string]any
	if err := json.NewDecoder(w.Body).Decode(&resp); err != nil {
		t.Fatalf("decode context response: %v", err)
	}
	if _, ok := resp["native_json"]; ok {
		t.Fatal("context response should not include native_json")
	}
	if _, ok := resp["nativeJson"]; ok {
		t.Fatal("context response should not include nativeJson")
	}
	frames, ok := resp["frames"].([]any)
	if !ok || len(frames) != 1 {
		t.Fatalf("frames = %T len %d, want one frame summary", resp["frames"], len(frames))
	}
	frame, ok := frames[0].(map[string]any)
	if !ok {
		t.Fatalf("frame summary type = %T", frames[0])
	}
	if frame["id"] != "frame-1" || frame["name"] != "Context Design" || frame["layerCount"] != float64(1) {
		t.Fatalf("unexpected frame summary: %+v", frame)
	}
	if _, ok := frame["layers"]; ok {
		t.Fatal("frame summary should not include full layers")
	}
}

func TestGetDesignFrameContextReturnsOnlyRequestedFrameDetails(t *testing.T) {
	created := createDesignFileForTest(t, "Frame Context Design")
	updateDesignRevisionNativeJSONForTest(t, created.CurrentRevision.ID, contextDesignNativeJSON("Frame Context Design"))

	req := withDesignURLParams(newRequest("GET", "/api/design-files/"+created.File.ID+"/frames/frame-main/context?workspace_id="+testWorkspaceID, nil), "id", created.File.ID, "frameId", "frame-main")
	w := httptest.NewRecorder()
	testHandler.GetDesignFrameContext(w, req)
	if w.Code != http.StatusOK {
		t.Fatalf("GetDesignFrameContext: expected 200, got %d: %s", w.Code, w.Body.String())
	}

	var resp map[string]any
	if err := json.NewDecoder(w.Body).Decode(&resp); err != nil {
		t.Fatalf("decode frame context response: %v", err)
	}
	frame := resp["frame"].(map[string]any)
	if frame["id"] != "frame-main" {
		t.Fatalf("frame id = %v, want frame-main", frame["id"])
	}
	layers := resp["layers"].(map[string]any)
	for _, id := range []string{"main-root", "main-title", "main-image", "main-offscreen"} {
		if _, ok := layers[id]; !ok {
			t.Fatalf("expected layer %s in frame context", id)
		}
	}
	if _, ok := layers["secondary-title"]; ok {
		t.Fatal("frame context included layer from another frame")
	}
	assets := resp["assets"].(map[string]any)
	for _, id := range []string{"asset-preview-main", "asset-thumb-main", "asset-hero", "asset-export-main"} {
		if _, ok := assets[id]; !ok {
			t.Fatalf("expected asset %s in frame context", id)
		}
	}
	if _, ok := assets["asset-secondary"]; ok {
		t.Fatal("frame context included unrelated asset")
	}
	if exportables := resp["exportables"].([]any); len(exportables) != 1 {
		t.Fatalf("exportables len = %d, want 1", len(exportables))
	}
	text := resp["text"].([]any)
	if len(text) != 1 || text[0].(map[string]any)["layerId"] != "main-title" {
		t.Fatalf("unexpected text context: %+v", text)
	}
}

func TestDesignContextSanitizesHistoricalEmbeddedBinary(t *testing.T) {
	created := createDesignFileForTest(t, "Historical Binary Context Design")
	nativeJSON := contextDesignNativeJSON("Historical Binary Context Design")
	nativeJSON["frames"].([]map[string]any)[0]["thumbnailDataUrl"] = "data:image/png;base64,AAAA"
	assets := nativeJSON["assets"].(map[string]any)
	assets["asset-hero"].(map[string]any)["bytes"] = []int{1, 2, 3}
	assets["asset-hero"].(map[string]any)["url"] = "data:image/png;base64,BBBB"
	layers := nativeJSON["layers"].(map[string]any)
	layers["main-image"].(map[string]any)["buffer"] = []int{4, 5, 6}
	updateDesignRevisionNativeJSONForTest(t, created.CurrentRevision.ID, nativeJSON)

	req := withDesignURLParams(newRequest("GET", "/api/design-files/"+created.File.ID+"/frames/frame-main/context?workspace_id="+testWorkspaceID, nil), "id", created.File.ID, "frameId", "frame-main")
	w := httptest.NewRecorder()
	testHandler.GetDesignFrameContext(w, req)
	if w.Code != http.StatusOK {
		t.Fatalf("GetDesignFrameContext: expected 200, got %d: %s", w.Code, w.Body.String())
	}
	var resp map[string]any
	if err := json.NewDecoder(w.Body).Decode(&resp); err != nil {
		t.Fatalf("decode frame context response: %v", err)
	}
	frame := resp["frame"].(map[string]any)
	if _, ok := frame["thumbnailDataUrl"]; ok {
		t.Fatal("frame context leaked thumbnailDataUrl data URL")
	}
	assetsResp := resp["assets"].(map[string]any)
	hero := assetsResp["asset-hero"].(map[string]any)
	if _, ok := hero["bytes"]; ok {
		t.Fatal("frame context leaked asset bytes")
	}
	if _, ok := hero["url"]; ok {
		t.Fatal("frame context leaked data:image asset URL")
	}
	layersResp := resp["layers"].(map[string]any)
	imageLayer := layersResp["main-image"].(map[string]any)
	if _, ok := imageLayer["buffer"]; ok {
		t.Fatal("frame context leaked layer buffer")
	}
}

func TestGetDesignSelectionContextWithBoundsReturnsIntersectingLayers(t *testing.T) {
	created := createDesignFileForTest(t, "Selection Context Design")
	updateDesignRevisionNativeJSONForTest(t, created.CurrentRevision.ID, contextDesignNativeJSON("Selection Context Design"))

	req := withDesignURLParams(newRequest("POST", "/api/design-files/"+created.File.ID+"/frames/frame-main/selection-context?workspace_id="+testWorkspaceID, map[string]any{
		"selectionBounds": map[string]any{"x": 35, "y": 35, "width": 230, "height": 80},
	}), "id", created.File.ID, "frameId", "frame-main")
	w := httptest.NewRecorder()
	testHandler.GetDesignSelectionContext(w, req)
	if w.Code != http.StatusOK {
		t.Fatalf("GetDesignSelectionContext: expected 200, got %d: %s", w.Code, w.Body.String())
	}

	var resp map[string]any
	if err := json.NewDecoder(w.Body).Decode(&resp); err != nil {
		t.Fatalf("decode selection context response: %v", err)
	}
	layers := resp["layers"].(map[string]any)
	if _, ok := layers["main-title"]; !ok {
		t.Fatal("selection context should include intersecting main-title layer")
	}
	if _, ok := layers["main-root"]; !ok {
		t.Fatal("selection context should include intersecting main-root layer")
	}
	for _, id := range []string{"main-image", "main-offscreen", "secondary-title"} {
		if _, ok := layers[id]; ok {
			t.Fatalf("selection context should not include non-intersecting layer %s", id)
		}
	}
	resolved := resp["resolvedLayerIds"].([]any)
	if len(resolved) != 2 || resolved[0] != "main-root" || resolved[1] != "main-title" {
		t.Fatalf("resolvedLayerIds = %+v, want [main-root main-title]", resolved)
	}
	text := resp["text"].([]any)
	if len(text) != 1 || text[0].(map[string]any)["layerId"] != "main-title" {
		t.Fatalf("unexpected selection text context: %+v", text)
	}
}

func TestGetDesignRevision(t *testing.T) {
	created := createDesignFileForTest(t, "Revision Design")
	if created.CurrentRevision == nil {
		t.Fatal("expected current revision")
	}

	listReq := withURLParam(newRequest("GET", "/api/design-files/"+created.File.ID+"/revisions?workspace_id="+testWorkspaceID, nil), "id", created.File.ID)
	listW := httptest.NewRecorder()
	testHandler.ListDesignRevisions(listW, listReq)
	if listW.Code != http.StatusOK {
		t.Fatalf("ListDesignRevisions: expected 200, got %d: %s", listW.Code, listW.Body.String())
	}
	var listResp struct {
		Revisions []map[string]any `json:"revisions"`
		Total     int              `json:"total"`
	}
	if err := json.NewDecoder(listW.Body).Decode(&listResp); err != nil {
		t.Fatalf("decode ListDesignRevisions: %v", err)
	}
	if listResp.Total == 0 || len(listResp.Revisions) == 0 {
		t.Fatal("expected revisions in list response")
	}
	if _, ok := listResp.Revisions[0]["native_json"]; ok {
		t.Fatal("ListDesignRevisions response should not include native_json")
	}

	revisionReq := withURLParam(newRequest("GET", "/api/design-revisions/"+created.CurrentRevision.ID+"?workspace_id="+testWorkspaceID, nil), "revisionId", created.CurrentRevision.ID)
	revisionW := httptest.NewRecorder()
	testHandler.GetDesignRevision(revisionW, revisionReq)
	if revisionW.Code != http.StatusOK {
		t.Fatalf("GetDesignRevision: expected 200, got %d: %s", revisionW.Code, revisionW.Body.String())
	}
	var revisionResp DesignRevisionResponse
	if err := json.NewDecoder(revisionW.Body).Decode(&revisionResp); err != nil {
		t.Fatalf("decode GetDesignRevision: %v", err)
	}
	if len(revisionResp.NativeJSON) == 0 {
		t.Fatal("GetDesignRevision response should include native_json")
	}
}

func postDesignLayerLightweightEditForTest(t *testing.T, fileID string, layerID string, body map[string]any) *httptest.ResponseRecorder {
	t.Helper()
	req := withDesignURLParams(newRequest("POST", "/api/design-files/"+fileID+"/layers/"+layerID+"/lightweight-edit?workspace_id="+testWorkspaceID, body), "id", fileID, "layerId", layerID)
	w := httptest.NewRecorder()
	testHandler.UpdateDesignLayerLightweight(w, req)
	return w
}

func decodeDesignRevisionNativeJSONForTest(t *testing.T, raw json.RawMessage) map[string]any {
	t.Helper()
	var doc map[string]any
	if err := json.Unmarshal(raw, &doc); err != nil {
		t.Fatalf("decode revision native_json: %v", err)
	}
	return doc
}

func layerFromNativeJSONForTest(t *testing.T, doc map[string]any, layerID string) map[string]any {
	t.Helper()
	layers, ok := doc["layers"].(map[string]any)
	if !ok {
		t.Fatalf("native_json layers type = %T", doc["layers"])
	}
	layer, ok := layers[layerID].(map[string]any)
	if !ok {
		t.Fatalf("native_json layer %s type = %T", layerID, layers[layerID])
	}
	return layer
}

func lastLightweightEditFromNativeJSONForTest(t *testing.T, doc map[string]any) map[string]any {
	t.Helper()
	source, ok := doc["source"].(map[string]any)
	if !ok {
		t.Fatalf("native_json source type = %T", doc["source"])
	}
	lastEdit, ok := source["lastLightweightEdit"].(map[string]any)
	if !ok {
		t.Fatalf("native_json source.lastLightweightEdit type = %T", source["lastLightweightEdit"])
	}
	return lastEdit
}

func assertLightweightEditChangedFieldsForTest(t *testing.T, lastEdit map[string]any, want []string) {
	t.Helper()
	changedFields, ok := lastEdit["changedFields"].([]any)
	if !ok {
		t.Fatalf("lastLightweightEdit.changedFields type = %T", lastEdit["changedFields"])
	}
	if len(changedFields) != len(want) {
		t.Fatalf("lastLightweightEdit.changedFields = %+v, want %+v", changedFields, want)
	}
	for i, field := range want {
		if changedFields[i] != field {
			t.Fatalf("lastLightweightEdit.changedFields[%d] = %v, want %s (changedFields=%+v)", i, changedFields[i], field, changedFields)
		}
	}
}

func TestUpdateDesignLayerLightweightTextMutatesCurrentRevisionAndPreservesStyle(t *testing.T) {
	created := createDesignFileForTest(t, "Lightweight Text Edit Design")
	if created.CurrentRevision == nil {
		t.Fatal("expected current revision")
	}
	nativeJSON := contextDesignNativeJSON("Lightweight Text Edit Design")
	nativeJSON["source"] = map[string]any{"fixtureKey": "preserve-me"}
	updateDesignRevisionNativeJSONForTest(t, created.CurrentRevision.ID, nativeJSON)

	w := postDesignLayerLightweightEditForTest(t, created.File.ID, "main-title", map[string]any{
		"revision_id": created.CurrentRevision.ID,
		"text":        "Start building",
	})
	if w.Code != http.StatusOK {
		t.Fatalf("UpdateDesignLayerLightweight text: expected 200, got %d: %s", w.Code, w.Body.String())
	}

	var resp DesignFileDetailResponse
	if err := json.NewDecoder(w.Body).Decode(&resp); err != nil {
		t.Fatalf("decode lightweight edit response: %v", err)
	}
	if resp.CurrentRevision == nil {
		t.Fatal("expected current revision response")
	}
	if resp.CurrentRevision.ID != created.CurrentRevision.ID {
		t.Fatalf("revision id = %s, want existing revision %s", resp.CurrentRevision.ID, created.CurrentRevision.ID)
	}
	if resp.CurrentRevision.RevisionNumber != created.CurrentRevision.RevisionNumber {
		t.Fatalf("revision number = %d, want existing revision number %d", resp.CurrentRevision.RevisionNumber, created.CurrentRevision.RevisionNumber)
	}
	if resp.File.CurrentRevisionID == nil || *resp.File.CurrentRevisionID != resp.CurrentRevision.ID {
		t.Fatalf("response current_revision_id = %v, want %s", resp.File.CurrentRevisionID, resp.CurrentRevision.ID)
	}

	var currentRevisionID string
	if err := testPool.QueryRow(context.Background(), `SELECT current_revision_id FROM design_file WHERE id = $1`, created.File.ID).Scan(&currentRevisionID); err != nil {
		t.Fatalf("query current_revision_id: %v", err)
	}
	if currentRevisionID != resp.CurrentRevision.ID {
		t.Fatalf("db current_revision_id = %s, want %s", currentRevisionID, resp.CurrentRevision.ID)
	}

	doc := decodeDesignRevisionNativeJSONForTest(t, resp.CurrentRevision.NativeJSON)
	textLayer := layerFromNativeJSONForTest(t, doc, "main-title")
	text, ok := textLayer["text"].(map[string]any)
	if !ok {
		t.Fatalf("text layer text type = %T", textLayer["text"])
	}
	if text["text"] != "Start building" {
		t.Fatalf("text.text = %v, want Start building", text["text"])
	}
	if text["characters"] != "Start building" {
		t.Fatalf("text.characters = %v, want Start building", text["characters"])
	}
	if text["fontFamily"] != "Inter" || text["fontSize"] != float64(24) || text["fontWeight"] != float64(700) {
		t.Fatalf("text style was not preserved: %+v", text)
	}
	color, ok := text["color"].(map[string]any)
	if !ok || color["a"] != float64(1) {
		t.Fatalf("text color style was not preserved: %+v", text["color"])
	}
	source, ok := doc["source"].(map[string]any)
	if !ok {
		t.Fatalf("native_json source type = %T", doc["source"])
	}
	if source["fixtureKey"] != "preserve-me" {
		t.Fatalf("source.fixtureKey = %v, want preserve-me (source=%+v)", source["fixtureKey"], source)
	}
	lastEdit := lastLightweightEditFromNativeJSONForTest(t, doc)
	if lastEdit["layerId"] != "main-title" {
		t.Fatalf("lastLightweightEdit.layerId = %v, want main-title", lastEdit["layerId"])
	}
	if lastEdit["layerName"] != "Title" {
		t.Fatalf("lastLightweightEdit.layerName = %v, want Title", lastEdit["layerName"])
	}
	if lastEdit["frameId"] != "frame-main" {
		t.Fatalf("lastLightweightEdit.frameId = %v, want frame-main", lastEdit["frameId"])
	}
	if lastEdit["summary"] != "Updated text for Title" {
		t.Fatalf("lastLightweightEdit.summary = %v, want Updated text for Title", lastEdit["summary"])
	}
	assertLightweightEditChangedFieldsForTest(t, lastEdit, []string{"text"})
}

func TestUpdateDesignLayerLightweightSemanticUpdatesKeys(t *testing.T) {
	created := createDesignFileForTest(t, "Lightweight Semantic Edit Design")
	if created.CurrentRevision == nil {
		t.Fatal("expected current revision")
	}
	updateDesignRevisionNativeJSONForTest(t, created.CurrentRevision.ID, contextDesignNativeJSON("Lightweight Semantic Edit Design"))

	w := postDesignLayerLightweightEditForTest(t, created.File.ID, "main-title", map[string]any{
		"revision_id": created.CurrentRevision.ID,
		"semantic": map[string]string{
			"role":      "headline",
			"moduleKey": "hero",
			"stateKey":  "default",
			"slotKey":   "title",
		},
	})
	if w.Code != http.StatusOK {
		t.Fatalf("UpdateDesignLayerLightweight semantic: expected 200, got %d: %s", w.Code, w.Body.String())
	}

	var resp DesignFileDetailResponse
	if err := json.NewDecoder(w.Body).Decode(&resp); err != nil {
		t.Fatalf("decode lightweight edit response: %v", err)
	}
	if resp.CurrentRevision == nil || resp.CurrentRevision.ID != created.CurrentRevision.ID {
		t.Fatalf("current revision = %+v, want existing revision %s", resp.CurrentRevision, created.CurrentRevision.ID)
	}
	doc := decodeDesignRevisionNativeJSONForTest(t, resp.CurrentRevision.NativeJSON)
	semantic, ok := layerFromNativeJSONForTest(t, doc, "main-title")["semantic"].(map[string]any)
	if !ok {
		t.Fatalf("semantic type = %T", layerFromNativeJSONForTest(t, doc, "main-title")["semantic"])
	}
	want := map[string]string{"role": "headline", "moduleKey": "hero", "stateKey": "default", "slotKey": "title"}
	for key, value := range want {
		if semantic[key] != value {
			t.Fatalf("semantic[%s] = %v, want %s (semantic=%+v)", key, semantic[key], value, semantic)
		}
	}
	lastEdit := lastLightweightEditFromNativeJSONForTest(t, doc)
	if lastEdit["layerId"] != "main-title" {
		t.Fatalf("lastLightweightEdit.layerId = %v, want main-title", lastEdit["layerId"])
	}
	if lastEdit["layerName"] != "Title" {
		t.Fatalf("lastLightweightEdit.layerName = %v, want Title", lastEdit["layerName"])
	}
	if lastEdit["frameId"] != "frame-main" {
		t.Fatalf("lastLightweightEdit.frameId = %v, want frame-main", lastEdit["frameId"])
	}
	if lastEdit["summary"] != "Updated semantic.role, semantic.moduleKey, semantic.stateKey, semantic.slotKey for Title" {
		t.Fatalf("lastLightweightEdit.summary = %v, want semantic summary", lastEdit["summary"])
	}
	assertLightweightEditChangedFieldsForTest(t, lastEdit, []string{"semantic.role", "semantic.moduleKey", "semantic.stateKey", "semantic.slotKey"})
}

func TestUpdateDesignLayerLightweightRejectsMismatchedRevision(t *testing.T) {
	created := createDesignFileForTest(t, "Lightweight Stale Revision Design")
	if created.CurrentRevision == nil {
		t.Fatal("expected current revision")
	}
	updateDesignRevisionNativeJSONForTest(t, created.CurrentRevision.ID, contextDesignNativeJSON("Lightweight Stale Revision Design"))

	stale := postDesignLayerLightweightEditForTest(t, created.File.ID, "main-title", map[string]any{
		"revision_id": "00000000-0000-0000-0000-000000000000",
		"text":        "Stale text",
	})
	if stale.Code != http.StatusConflict {
		t.Fatalf("stale lightweight edit: expected 409, got %d: %s", stale.Code, stale.Body.String())
	}
}

func TestUpdateDesignLayerLightweightRejectsTextEditOnNonTextLayer(t *testing.T) {
	created := createDesignFileForTest(t, "Lightweight Non Text Edit Design")
	if created.CurrentRevision == nil {
		t.Fatal("expected current revision")
	}
	updateDesignRevisionNativeJSONForTest(t, created.CurrentRevision.ID, contextDesignNativeJSON("Lightweight Non Text Edit Design"))

	w := postDesignLayerLightweightEditForTest(t, created.File.ID, "main-image", map[string]any{
		"revision_id": created.CurrentRevision.ID,
		"text":        "Not allowed",
	})
	if w.Code != http.StatusBadRequest {
		t.Fatalf("text edit on non-text layer: expected 400, got %d: %s", w.Code, w.Body.String())
	}
}

func TestCreateDesignRestoreTaskUsesCurrentRevisionAndStoresInput(t *testing.T) {
	created := createDesignFileForTest(t, "Restore Task Design")
	if created.CurrentRevision == nil {
		t.Fatal("expected current revision")
	}

	input := map[string]any{
		"prompt": "restore the hero section",
		"selection": map[string]any{
			"frameId":  "frame-1",
			"layerIds": []string{"layer-1"},
		},
	}
	req := newRequest("POST", "/api/design-restore-tasks?workspace_id="+testWorkspaceID, map[string]any{
		"file_id": created.File.ID,
		"input":   input,
	})
	w := httptest.NewRecorder()
	testHandler.CreateDesignRestoreTask(w, req)
	if w.Code != http.StatusCreated {
		t.Fatalf("CreateDesignRestoreTask: expected 201, got %d: %s", w.Code, w.Body.String())
	}

	var resp DesignRestoreTaskResponse
	if err := json.NewDecoder(w.Body).Decode(&resp); err != nil {
		t.Fatalf("decode CreateDesignRestoreTask: %v", err)
	}
	t.Cleanup(func() {
		_, _ = testPool.Exec(context.Background(), `DELETE FROM design_restore_task WHERE id = $1`, resp.ID)
	})

	if resp.ID == "" {
		t.Fatal("expected task id")
	}
	if resp.WorkspaceID != testWorkspaceID {
		t.Fatalf("workspace_id = %s, want %s", resp.WorkspaceID, testWorkspaceID)
	}
	if resp.FileID != created.File.ID {
		t.Fatalf("file_id = %s, want %s", resp.FileID, created.File.ID)
	}
	if resp.RevisionID != created.CurrentRevision.ID {
		t.Fatalf("revision_id = %s, want current revision %s", resp.RevisionID, created.CurrentRevision.ID)
	}
	if resp.Status != "queued" {
		t.Fatalf("status = %q, want queued", resp.Status)
	}
	if resp.CreatedBy == nil || *resp.CreatedBy != testUserID {
		t.Fatalf("created_by = %v, want %s", resp.CreatedBy, testUserID)
	}

	var gotInput map[string]any
	if err := json.Unmarshal(resp.Input, &gotInput); err != nil {
		t.Fatalf("unmarshal task input: %v", err)
	}
	if gotInput["prompt"] != "restore the hero section" {
		t.Fatalf("input prompt = %v, want restore prompt", gotInput["prompt"])
	}
	selection, ok := gotInput["selection"].(map[string]any)
	if !ok {
		t.Fatalf("input selection type = %T", gotInput["selection"])
	}
	if selection["frameId"] != "frame-1" {
		t.Fatalf("input selection frameId = %v, want frame-1", selection["frameId"])
	}
}

func TestGetDesignRestoreTaskReturnsTaskInWorkspace(t *testing.T) {
	created := createDesignFileForTest(t, "Get Restore Task Design")
	if created.CurrentRevision == nil {
		t.Fatal("expected current revision")
	}

	createReq := newRequest("POST", "/api/design-restore-tasks?workspace_id="+testWorkspaceID, map[string]any{
		"file_id": created.File.ID,
		"input": map[string]any{
			"prompt": "get this task",
		},
	})
	createW := httptest.NewRecorder()
	testHandler.CreateDesignRestoreTask(createW, createReq)
	if createW.Code != http.StatusCreated {
		t.Fatalf("CreateDesignRestoreTask: expected 201, got %d: %s", createW.Code, createW.Body.String())
	}
	var createdTask DesignRestoreTaskResponse
	if err := json.NewDecoder(createW.Body).Decode(&createdTask); err != nil {
		t.Fatalf("decode CreateDesignRestoreTask: %v", err)
	}
	t.Cleanup(func() {
		_, _ = testPool.Exec(context.Background(), `DELETE FROM design_restore_task WHERE id = $1`, createdTask.ID)
	})

	getReq := withURLParam(newRequest("GET", "/api/design-restore-tasks/"+createdTask.ID+"?workspace_id="+testWorkspaceID, nil), "id", createdTask.ID)
	getW := httptest.NewRecorder()
	testHandler.GetDesignRestoreTask(getW, getReq)
	if getW.Code != http.StatusOK {
		t.Fatalf("GetDesignRestoreTask: expected 200, got %d: %s", getW.Code, getW.Body.String())
	}
	var got DesignRestoreTaskResponse
	if err := json.NewDecoder(getW.Body).Decode(&got); err != nil {
		t.Fatalf("decode GetDesignRestoreTask: %v", err)
	}

	if got.ID != createdTask.ID {
		t.Fatalf("id = %s, want %s", got.ID, createdTask.ID)
	}
	if got.WorkspaceID != testWorkspaceID {
		t.Fatalf("workspace_id = %s, want %s", got.WorkspaceID, testWorkspaceID)
	}
	if got.FileID != createdTask.FileID {
		t.Fatalf("file_id = %s, want %s", got.FileID, createdTask.FileID)
	}
	if got.RevisionID != createdTask.RevisionID {
		t.Fatalf("revision_id = %s, want %s", got.RevisionID, createdTask.RevisionID)
	}
	if string(got.Input) != string(createdTask.Input) {
		t.Fatalf("input = %s, want %s", string(got.Input), string(createdTask.Input))
	}
}

func TestListDesignRestoreTasksReturnsWorkspaceTasks(t *testing.T) {
	created := createDesignFileForTest(t, "List Restore Task Design")
	if created.CurrentRevision == nil {
		t.Fatal("expected current revision")
	}

	createReq := newRequest("POST", "/api/design-restore-tasks?workspace_id="+testWorkspaceID, map[string]any{
		"file_id": created.File.ID,
		"input": map[string]any{
			"version":   "1.0",
			"projectId": "project-list-test",
			"items":     []any{},
		},
	})
	createW := httptest.NewRecorder()
	testHandler.CreateDesignRestoreTask(createW, createReq)
	if createW.Code != http.StatusCreated {
		t.Fatalf("CreateDesignRestoreTask: expected 201, got %d: %s", createW.Code, createW.Body.String())
	}
	var createdTask DesignRestoreTaskResponse
	if err := json.NewDecoder(createW.Body).Decode(&createdTask); err != nil {
		t.Fatalf("decode CreateDesignRestoreTask: %v", err)
	}
	t.Cleanup(func() {
		_, _ = testPool.Exec(context.Background(), `DELETE FROM design_restore_task WHERE id = $1`, createdTask.ID)
	})

	listReq := newRequest("GET", "/api/design-restore-tasks?workspace_id="+testWorkspaceID, nil)
	listW := httptest.NewRecorder()
	testHandler.ListDesignRestoreTasks(listW, listReq)
	if listW.Code != http.StatusOK {
		t.Fatalf("ListDesignRestoreTasks: expected 200, got %d: %s", listW.Code, listW.Body.String())
	}
	var resp DesignRestoreTaskListResponse
	if err := json.NewDecoder(listW.Body).Decode(&resp); err != nil {
		t.Fatalf("decode ListDesignRestoreTasks: %v", err)
	}
	for _, task := range resp.Tasks {
		if task.ID == createdTask.ID {
			return
		}
	}
	t.Fatalf("expected listed restore tasks to include %s", createdTask.ID)
}

func TestDispatchDesignRestoreTaskCreatesAgentTask(t *testing.T) {
	created := createDesignFileForTest(t, "Dispatch Restore Task Design")
	if created.CurrentRevision == nil {
		t.Fatal("expected current revision")
	}
	updateDesignRevisionNativeJSONForTest(t, created.CurrentRevision.ID, contextDesignNativeJSON("Dispatch Restore Task Design"))
	agentID := createHandlerTestAgent(t, "Dispatch Restore Agent", []byte("[]"))

	createReq := newRequest("POST", "/api/design-restore-tasks?workspace_id="+testWorkspaceID, map[string]any{
		"file_id": created.File.ID,
		"input": map[string]any{
			"version": "1.0",
			"items": []map[string]any{{
				"itemId":       "dispatch-frame-1",
				"order":        1,
				"designFileId": created.File.ID,
				"revisionId":   created.CurrentRevision.ID,
				"frameId":      "frame-main",
				"source":       "frame",
			}},
		},
	})
	createW := httptest.NewRecorder()
	testHandler.CreateDesignRestoreTask(createW, createReq)
	if createW.Code != http.StatusCreated {
		t.Fatalf("CreateDesignRestoreTask: expected 201, got %d: %s", createW.Code, createW.Body.String())
	}
	var createdTask DesignRestoreTaskResponse
	if err := json.NewDecoder(createW.Body).Decode(&createdTask); err != nil {
		t.Fatalf("decode CreateDesignRestoreTask: %v", err)
	}
	t.Cleanup(func() {
		_, _ = testPool.Exec(context.Background(), `DELETE FROM design_restore_task WHERE id = $1`, createdTask.ID)
	})

	req := withURLParam(newRequest("POST", "/api/design-restore-tasks/"+createdTask.ID+"/dispatch?workspace_id="+testWorkspaceID, map[string]any{
		"agent_id": agentID,
		"prompt":   "restore this frame",
	}), "id", createdTask.ID)
	w := httptest.NewRecorder()
	testHandler.DispatchDesignRestoreTask(w, req)
	if w.Code != http.StatusCreated {
		t.Fatalf("DispatchDesignRestoreTask: expected 201, got %d: %s", w.Code, w.Body.String())
	}
	var resp DispatchDesignRestoreTaskResponse
	if err := json.NewDecoder(w.Body).Decode(&resp); err != nil {
		t.Fatalf("decode DispatchDesignRestoreTask: %v", err)
	}
	t.Cleanup(func() {
		_, _ = testPool.Exec(context.Background(), `DELETE FROM agent_task_queue WHERE id = $1`, resp.AgentTaskID)
	})
	if resp.AgentTaskID == "" {
		t.Fatal("expected agent task id")
	}
	if resp.Task.AgentTaskID == nil || *resp.Task.AgentTaskID != resp.AgentTaskID {
		t.Fatalf("restore task agent_task_id = %v, want %s", resp.Task.AgentTaskID, resp.AgentTaskID)
	}
	if resp.Task.Status != "running" {
		t.Fatalf("restore task status = %q, want running", resp.Task.Status)
	}
	var queuedContextRaw []byte
	if err := testPool.QueryRow(context.Background(), `SELECT context FROM agent_task_queue WHERE id = $1`, resp.AgentTaskID).Scan(&queuedContextRaw); err != nil {
		t.Fatalf("load queued task context: %v", err)
	}
	var queuedContext map[string]any
	if err := json.Unmarshal(queuedContextRaw, &queuedContext); err != nil {
		t.Fatalf("decode queued task context: %v", err)
	}
	if queuedContext["type"] != service.DesignRestoreTaskContextType {
		t.Fatalf("queued context type = %v", queuedContext["type"])
	}
	restorePolicy, ok := queuedContext["restore_policy"].(map[string]any)
	if !ok || restorePolicy["restoreMode"] != "strict-structure" || restorePolicy["allowFullFramePreview"] != false {
		t.Fatalf("queued restore_policy = %#v, want strict-structure with full frame preview disabled", queuedContext["restore_policy"])
	}
	itemContexts, ok := queuedContext["item_contexts"].([]any)
	if !ok || len(itemContexts) != 1 {
		t.Fatalf("queued item_contexts = %#v, want one item context", queuedContext["item_contexts"])
	}
	firstItemContext := itemContexts[0].(map[string]any)
	contextPayload := firstItemContext["context"].(map[string]any)
	frame := contextPayload["frame"].(map[string]any)
	if frame["id"] != "frame-main" {
		t.Fatalf("embedded frame context id = %v, want frame-main", frame["id"])
	}
}

func TestGetDesignRestoreTaskItemContextFrameReturnsFrameContext(t *testing.T) {
	created := createDesignFileForTest(t, "Restore Frame Context Design")
	if created.CurrentRevision == nil {
		t.Fatal("expected current revision")
	}
	updateDesignRevisionNativeJSONForTest(t, created.CurrentRevision.ID, contextDesignNativeJSON("Restore Frame Context Design"))

	createReq := newRequest("POST", "/api/design-restore-tasks?workspace_id="+testWorkspaceID, map[string]any{
		"file_id": created.File.ID,
		"input": map[string]any{
			"version": "1",
			"items": []map[string]any{{
				"itemId":       "item-frame-main",
				"order":        1,
				"designFileId": created.File.ID,
				"revisionId":   created.CurrentRevision.ID,
				"frameId":      "frame-main",
				"frameName":    "Main Screen",
				"source":       "frame",
				"moduleKey":    "module-a",
				"stateKey":     "state-a",
				"slotKey":      "slot-a",
			}},
		},
	})
	createW := httptest.NewRecorder()
	testHandler.CreateDesignRestoreTask(createW, createReq)
	if createW.Code != http.StatusCreated {
		t.Fatalf("CreateDesignRestoreTask: expected 201, got %d: %s", createW.Code, createW.Body.String())
	}
	var createdTask DesignRestoreTaskResponse
	if err := json.NewDecoder(createW.Body).Decode(&createdTask); err != nil {
		t.Fatalf("decode CreateDesignRestoreTask: %v", err)
	}
	t.Cleanup(func() {
		_, _ = testPool.Exec(context.Background(), `DELETE FROM design_restore_task WHERE id = $1`, createdTask.ID)
	})

	req := withDesignURLParams(newRequest("GET", "/api/design-restore-tasks/"+createdTask.ID+"/items/item-frame-main/context?workspace_id="+testWorkspaceID, nil), "id", createdTask.ID, "itemId", "item-frame-main")
	w := httptest.NewRecorder()
	testHandler.GetDesignRestoreTaskItemContext(w, req)
	if w.Code != http.StatusOK {
		t.Fatalf("GetDesignRestoreTaskItemContext: expected 200, got %d: %s", w.Code, w.Body.String())
	}

	var resp DesignRestoreTaskItemContextResponse
	if err := json.NewDecoder(w.Body).Decode(&resp); err != nil {
		t.Fatalf("decode item context response: %v", err)
	}
	if resp.Task.ID != createdTask.ID || resp.Task.FileID != created.File.ID || resp.Task.RevisionID != created.CurrentRevision.ID {
		t.Fatalf("unexpected task metadata: %+v", resp.Task)
	}
	if resp.Item.ItemID != "item-frame-main" || resp.Item.DesignFileID != created.File.ID || resp.Item.RevisionID != created.CurrentRevision.ID || resp.Item.FrameID != "frame-main" || resp.Item.Source != "frame" {
		t.Fatalf("unexpected item metadata: %+v", resp.Item)
	}
	frame := resp.Context["frame"].(map[string]any)
	if frame["id"] != "frame-main" || frame["name"] != "Main Screen" {
		t.Fatalf("unexpected frame context frame: %+v", frame)
	}
	layers := resp.Context["layers"].(map[string]any)
	for _, id := range []string{"main-root", "main-title", "main-image", "main-offscreen"} {
		if _, ok := layers[id]; !ok {
			t.Fatalf("expected layer %s in frame context", id)
		}
	}
	if _, ok := layers["secondary-title"]; ok {
		t.Fatal("frame context included layer from another frame")
	}
}

func TestGetDesignRestoreTaskItemContextSelectionBoundsReturnsSelectionContext(t *testing.T) {
	created := createDesignFileForTest(t, "Restore Selection Context Design")
	if created.CurrentRevision == nil {
		t.Fatal("expected current revision")
	}
	updateDesignRevisionNativeJSONForTest(t, created.CurrentRevision.ID, contextDesignNativeJSON("Restore Selection Context Design"))

	createReq := newRequest("POST", "/api/design-restore-tasks?workspace_id="+testWorkspaceID, map[string]any{
		"file_id": created.File.ID,
		"input": map[string]any{
			"version": "1",
			"items": []map[string]any{{
				"itemId":       "item-selection-main",
				"order":        1,
				"designFileId": created.File.ID,
				"revisionId":   created.CurrentRevision.ID,
				"frameId":      "frame-main",
				"frameName":    "Main Screen",
				"source":       "selection_bounds",
				"selectionBounds": map[string]any{
					"x": 35, "y": 35, "width": 230, "height": 80,
				},
			}},
		},
	})
	createW := httptest.NewRecorder()
	testHandler.CreateDesignRestoreTask(createW, createReq)
	if createW.Code != http.StatusCreated {
		t.Fatalf("CreateDesignRestoreTask: expected 201, got %d: %s", createW.Code, createW.Body.String())
	}
	var createdTask DesignRestoreTaskResponse
	if err := json.NewDecoder(createW.Body).Decode(&createdTask); err != nil {
		t.Fatalf("decode CreateDesignRestoreTask: %v", err)
	}
	t.Cleanup(func() {
		_, _ = testPool.Exec(context.Background(), `DELETE FROM design_restore_task WHERE id = $1`, createdTask.ID)
	})

	req := withDesignURLParams(newRequest("GET", "/api/design-restore-tasks/"+createdTask.ID+"/items/item-selection-main/context?workspace_id="+testWorkspaceID, nil), "id", createdTask.ID, "itemId", "item-selection-main")
	w := httptest.NewRecorder()
	testHandler.GetDesignRestoreTaskItemContext(w, req)
	if w.Code != http.StatusOK {
		t.Fatalf("GetDesignRestoreTaskItemContext: expected 200, got %d: %s", w.Code, w.Body.String())
	}

	var resp DesignRestoreTaskItemContextResponse
	if err := json.NewDecoder(w.Body).Decode(&resp); err != nil {
		t.Fatalf("decode item context response: %v", err)
	}
	if resp.Item.ItemID != "item-selection-main" || resp.Item.Source != "selection_bounds" {
		t.Fatalf("unexpected item metadata: %+v", resp.Item)
	}
	resolved := resp.Context["resolvedLayerIds"].([]any)
	if len(resolved) != 2 || resolved[0] != "main-root" || resolved[1] != "main-title" {
		t.Fatalf("resolvedLayerIds = %+v, want [main-root main-title]", resolved)
	}
	layers := resp.Context["layers"].(map[string]any)
	if _, ok := layers["main-title"]; !ok {
		t.Fatal("selection context should include intersecting main-title layer")
	}
	if _, ok := layers["main-image"]; ok {
		t.Fatal("selection context should not include non-intersecting main-image layer")
	}
}

func TestGetDesignRestoreTaskItemContextUnknownItemReturnsNotFound(t *testing.T) {
	created := createDesignFileForTest(t, "Restore Unknown Item Design")
	if created.CurrentRevision == nil {
		t.Fatal("expected current revision")
	}

	createReq := newRequest("POST", "/api/design-restore-tasks?workspace_id="+testWorkspaceID, map[string]any{
		"file_id": created.File.ID,
		"input": map[string]any{
			"version": "1",
			"items": []map[string]any{{
				"itemId":       "known-item",
				"designFileId": created.File.ID,
				"revisionId":   created.CurrentRevision.ID,
				"frameId":      "frame-1",
				"source":       "frame",
			}},
		},
	})
	createW := httptest.NewRecorder()
	testHandler.CreateDesignRestoreTask(createW, createReq)
	if createW.Code != http.StatusCreated {
		t.Fatalf("CreateDesignRestoreTask: expected 201, got %d: %s", createW.Code, createW.Body.String())
	}
	var createdTask DesignRestoreTaskResponse
	if err := json.NewDecoder(createW.Body).Decode(&createdTask); err != nil {
		t.Fatalf("decode CreateDesignRestoreTask: %v", err)
	}
	t.Cleanup(func() {
		_, _ = testPool.Exec(context.Background(), `DELETE FROM design_restore_task WHERE id = $1`, createdTask.ID)
	})

	req := withDesignURLParams(newRequest("GET", "/api/design-restore-tasks/"+createdTask.ID+"/items/missing-item/context?workspace_id="+testWorkspaceID, nil), "id", createdTask.ID, "itemId", "missing-item")
	w := httptest.NewRecorder()
	testHandler.GetDesignRestoreTaskItemContext(w, req)
	if w.Code != http.StatusNotFound {
		t.Fatalf("GetDesignRestoreTaskItemContext unknown item: expected 404, got %d: %s", w.Code, w.Body.String())
	}
}

func TestGetDesignFileRejectsInvalidID(t *testing.T) {
	req := withURLParam(newRequest("GET", "/api/design-files/not-a-uuid?workspace_id="+testWorkspaceID, nil), "id", "not-a-uuid")
	w := httptest.NewRecorder()
	testHandler.GetDesignFile(w, req)
	if w.Code != http.StatusBadRequest {
		t.Fatalf("GetDesignFile invalid ID: expected 400, got %d: %s", w.Code, w.Body.String())
	}
}

func TestDeleteDesignFile(t *testing.T) {
	created := createDesignFileForTest(t, "Delete Me Design")
	req := withURLParam(newRequest("DELETE", "/api/design-files/"+created.File.ID+"?workspace_id="+testWorkspaceID, nil), "id", created.File.ID)
	w := httptest.NewRecorder()
	testHandler.DeleteDesignFile(w, req)
	if w.Code != http.StatusNoContent {
		t.Fatalf("DeleteDesignFile: expected 204, got %d: %s", w.Code, w.Body.String())
	}

	getReq := withURLParam(newRequest("GET", "/api/design-files/"+created.File.ID+"?workspace_id="+testWorkspaceID, nil), "id", created.File.ID)
	getW := httptest.NewRecorder()
	testHandler.GetDesignFile(getW, getReq)
	if getW.Code != http.StatusNotFound {
		t.Fatalf("GetDesignFile after delete: expected 404, got %d: %s", getW.Code, getW.Body.String())
	}
}

func TestDeleteDesignFileRejectsInvalidID(t *testing.T) {
	req := withURLParam(newRequest("DELETE", "/api/design-files/not-a-uuid?workspace_id="+testWorkspaceID, nil), "id", "not-a-uuid")
	w := httptest.NewRecorder()
	testHandler.DeleteDesignFile(w, req)
	if w.Code != http.StatusBadRequest {
		t.Fatalf("DeleteDesignFile invalid ID: expected 400, got %d: %s", w.Code, w.Body.String())
	}
}

func createFigmaImportCodeForTest(t *testing.T) string {
	t.Helper()
	req := newRequest("POST", "/api/design-files/figma-connections?workspace_id="+testWorkspaceID, nil)
	w := httptest.NewRecorder()
	testHandler.CreateFigmaImportConnection(w, req)
	if w.Code != http.StatusCreated {
		t.Fatalf("CreateFigmaImportConnection: expected 201, got %d: %s", w.Code, w.Body.String())
	}
	var resp CreateFigmaImportConnectionResponse
	if err := json.NewDecoder(w.Body).Decode(&resp); err != nil {
		t.Fatalf("decode figma import connection: %v", err)
	}
	if resp.Code == "" || resp.ExpiresAt == "" {
		t.Fatalf("expected code and expires_at, got %+v", resp)
	}
	return resp.Code
}

func importFigmaDesignForTest(t *testing.T, code string, title string, nativeJSON map[string]any) *httptest.ResponseRecorder {
	t.Helper()
	req := newRequest("POST", "/api/design-files/imports/figma", map[string]any{
		"code":           code,
		"workspace_slug": handlerTestWorkspaceSlug,
		"title":          title,
		"description":    "Imported from Figma plugin",
		"source_ref":     map[string]any{"tool": "figma", "test": true},
		"native_json":    nativeJSON,
	})
	req.Header.Del("X-User-ID")
	req.Header.Del("X-Workspace-ID")
	w := httptest.NewRecorder()
	testHandler.ImportFigmaDesignFile(w, req)
	return w
}

func TestFigmaImportConnectionAndImport(t *testing.T) {
	code := createFigmaImportCodeForTest(t)
	w := importFigmaDesignForTest(t, code, "Figma Imported Design", minimalDesignNativeJSON("Figma Imported Design"))
	if w.Code != http.StatusCreated {
		t.Fatalf("ImportFigmaDesignFile: expected 201, got %d: %s", w.Code, w.Body.String())
	}
	var resp DesignFileDetailResponse
	if err := json.NewDecoder(w.Body).Decode(&resp); err != nil {
		t.Fatalf("decode import response: %v", err)
	}
	if resp.File.CreatedBy == nil || *resp.File.CreatedBy != testUserID {
		t.Fatalf("created_by = %v, want %s", resp.File.CreatedBy, testUserID)
	}
	t.Cleanup(func() {
		_, _ = testPool.Exec(context.Background(), `DELETE FROM design_file WHERE id = $1`, resp.File.ID)
	})

	reuse := importFigmaDesignForTest(t, code, "Reused Code Design", minimalDesignNativeJSON("Reused Code Design"))
	if reuse.Code != http.StatusUnauthorized {
		t.Fatalf("reused code: expected 401, got %d: %s", reuse.Code, reuse.Body.String())
	}
}

func TestFigmaImportRejectsWorkspaceMismatch(t *testing.T) {
	code := createFigmaImportCodeForTest(t)
	req := newRequest("POST", "/api/design-files/imports/figma", map[string]any{
		"code":           code,
		"workspace_slug": "wrong-workspace",
		"title":          "Wrong Workspace",
		"native_json":    minimalDesignNativeJSON("Wrong Workspace"),
	})
	req.Header.Del("X-User-ID")
	req.Header.Del("X-Workspace-ID")
	w := httptest.NewRecorder()
	testHandler.ImportFigmaDesignFile(w, req)
	if w.Code != http.StatusUnauthorized {
		t.Fatalf("workspace mismatch: expected 401, got %d: %s", w.Code, w.Body.String())
	}
}

func TestFigmaImportRejectsExpiredCode(t *testing.T) {
	code := "figma_expired_test_code"
	_, err := testPool.Exec(context.Background(), `
		INSERT INTO design_import_code (workspace_id, user_id, provider, code_hash, expires_at)
		VALUES ($1, $2, 'figma', $3, $4)
	`, testWorkspaceID, testUserID, auth.HashToken(code), time.Now().Add(-time.Minute))
	if err != nil {
		t.Fatalf("insert expired code: %v", err)
	}
	w := importFigmaDesignForTest(t, code, "Expired Code Design", minimalDesignNativeJSON("Expired Code Design"))
	if w.Code != http.StatusUnauthorized {
		t.Fatalf("expired code: expected 401, got %d: %s", w.Code, w.Body.String())
	}
}

func TestFigmaImportInvalidNativeJSONDoesNotConsumeCode(t *testing.T) {
	code := createFigmaImportCodeForTest(t)
	invalid := importFigmaDesignForTest(t, code, "Invalid Figma Design", map[string]any{
		"version": "1.0",
		"file":    map[string]any{"title": "Invalid", "sourceType": "import"},
		"frames":  []map[string]any{{"id": "frame-1", "name": "Invalid", "rootLayerId": "missing", "width": 100, "height": 100}},
		"layers":  map[string]any{},
		"assets":  map[string]any{},
	})
	if invalid.Code != http.StatusBadRequest {
		t.Fatalf("invalid native json: expected 400, got %d: %s", invalid.Code, invalid.Body.String())
	}

	valid := importFigmaDesignForTest(t, code, "Valid After Invalid", minimalDesignNativeJSON("Valid After Invalid"))
	if valid.Code != http.StatusCreated {
		t.Fatalf("valid after invalid: expected 201, got %d: %s", valid.Code, valid.Body.String())
	}
}

func createPluginTokenForTest(t *testing.T) string {
	t.Helper()
	token := fmt.Sprintf("mfp_test_%d", time.Now().UnixNano())
	_, err := testPool.Exec(context.Background(), `
		INSERT INTO design_plugin_token (provider, token_hash, token_prefix, user_id, workspace_id, scope, name)
		VALUES ('figma', $1, $2, $3, $4, 'design_import', 'Figma Plugin Test')
	`, auth.HashToken(token), token[:12], testUserID, testWorkspaceID)
	if err != nil {
		t.Fatalf("insert plugin token: %v", err)
	}
	return token
}

func TestFigmaPluginContextReturnsProjectFolders(t *testing.T) {
	projectID := createProjectForDesignTest(t, "Plugin Context Project")
	folderID := createDesignFolderForTest(t, projectID, "Plugin Folder")
	design := createDesignFileForTest(t, "Plugin Context Existing Design")
	if _, err := testPool.Exec(context.Background(), `UPDATE design_file SET project_id = $1, folder_id = $2 WHERE id = $3`, projectID, folderID, design.File.ID); err != nil {
		t.Fatalf("attach design file to project folder: %v", err)
	}
	token := createPluginTokenForTest(t)
	req := newRequest("GET", "/api/design-plugin/figma/context", nil)
	req.Header.Set("Authorization", "Bearer "+token)
	w := httptest.NewRecorder()
	testHandler.GetFigmaPluginContext(w, req)
	if w.Code != http.StatusOK {
		t.Fatalf("GetFigmaPluginContext: expected 200, got %d: %s", w.Code, w.Body.String())
	}
	var resp FigmaPluginContextResponse
	if err := json.NewDecoder(w.Body).Decode(&resp); err != nil {
		t.Fatalf("decode context: %v", err)
	}
	for _, project := range resp.Projects {
		if project.ID == projectID {
			foundFolder := false
			for _, folder := range project.Folders {
				if folder.ID == folderID {
					foundFolder = true
					break
				}
			}
			if !foundFolder {
				t.Fatalf("context did not include folder %s under project %s", folderID, projectID)
			}
			for _, file := range project.DesignFiles {
				if file.ID == design.File.ID {
					if file.FolderID == nil || *file.FolderID != folderID {
						t.Fatalf("design file folder_id = %v, want %s", file.FolderID, folderID)
					}
					return
				}
			}
			t.Fatalf("context did not include design file %s under project %s", design.File.ID, projectID)
		}
	}
	t.Fatalf("context did not include project %s", projectID)
}

func TestFigmaPluginImportRequiresProject(t *testing.T) {
	token := createPluginTokenForTest(t)
	req := newRequest("POST", "/api/design-plugin/figma/imports", map[string]any{
		"title":       "Plugin Import Without Project",
		"native_json": minimalDesignNativeJSON("Plugin Import Without Project"),
	})
	req.Header.Set("Authorization", "Bearer "+token)
	w := httptest.NewRecorder()
	testHandler.ImportFigmaDesignWithPluginToken(w, req)
	if w.Code != http.StatusBadRequest {
		t.Fatalf("plugin import without project: expected 400, got %d: %s", w.Code, w.Body.String())
	}
}

func TestCreateDesignFileRejectsEmbeddedBinaryNativeJSON(t *testing.T) {
	nativeJSON := minimalDesignNativeJSON("Embedded Binary Design")
	nativeJSON["assets"] = map[string]any{
		"asset-1": map[string]any{"id": "asset-1", "kind": "frame_preview", "url": "data:image/png;base64,AAAA"},
	}
	req := newRequest("POST", "/api/design-files?workspace_id="+testWorkspaceID, map[string]any{
		"title":       "Embedded Binary Design",
		"native_json": nativeJSON,
	})
	w := httptest.NewRecorder()
	testHandler.CreateDesignFile(w, req)
	if w.Code != http.StatusBadRequest {
		t.Fatalf("CreateDesignFile embedded binary: expected 400, got %d: %s", w.Code, w.Body.String())
	}
}

func TestFigmaPluginImportRejectsEmbeddedBinaryNativeJSON(t *testing.T) {
	projectID := createProjectForDesignTest(t, "Plugin Binary Guard Project")
	token := createPluginTokenForTest(t)
	nativeJSON := minimalDesignNativeJSON("Plugin Binary Guard Design")
	nativeJSON["assets"] = map[string]any{
		"asset-1": map[string]any{"id": "asset-1", "kind": "frame_preview", "url": "https://static.example/frame.png", "bytes": []int{1, 2, 3}},
	}
	req := newRequest("POST", "/api/design-plugin/figma/imports", map[string]any{
		"title":       "Plugin Binary Guard Design",
		"project_id":  projectID,
		"native_json": nativeJSON,
	})
	req.Header.Set("Authorization", "Bearer "+token)
	w := httptest.NewRecorder()
	testHandler.ImportFigmaDesignWithPluginToken(w, req)
	if w.Code != http.StatusBadRequest {
		t.Fatalf("plugin import embedded binary: expected 400, got %d: %s", w.Code, w.Body.String())
	}
}

func TestFigmaPluginImportWithProjectAndFolder(t *testing.T) {
	projectID := createProjectForDesignTest(t, "Plugin Import Project")
	folderID := createDesignFolderForTest(t, projectID, "Plugin Import Folder")
	token := createPluginTokenForTest(t)
	var beforeCount int
	if err := testPool.QueryRow(context.Background(), `SELECT count(*) FROM design_file WHERE workspace_id = $1 AND project_id = $2 AND folder_id = $3`, testWorkspaceID, projectID, folderID).Scan(&beforeCount); err != nil {
		t.Fatalf("count design files before import: %v", err)
	}
	req := newRequest("POST", "/api/design-plugin/figma/imports", map[string]any{
		"title":               "Plugin Import Request Title",
		"design_file_title":   "Plugin Import Design File Title",
		"project_id":          projectID,
		"folder_id":           folderID,
		"source_ref":          map[string]any{"provider": "figma"},
		"native_json":         minimalDesignNativeJSON("Plugin Import Design"),
		"publish_as_template": false,
	})
	req.Header.Set("Authorization", "Bearer "+token)
	w := httptest.NewRecorder()
	testHandler.ImportFigmaDesignWithPluginToken(w, req)
	if w.Code != http.StatusCreated {
		t.Fatalf("plugin import: expected 201, got %d: %s", w.Code, w.Body.String())
	}
	var resp DesignFileDetailResponse
	if err := json.NewDecoder(w.Body).Decode(&resp); err != nil {
		t.Fatalf("decode plugin import: %v", err)
	}
	if resp.File.ProjectID == nil || *resp.File.ProjectID != projectID {
		t.Fatalf("project_id = %v, want %s", resp.File.ProjectID, projectID)
	}
	if resp.File.FolderID == nil || *resp.File.FolderID != folderID {
		t.Fatalf("folder_id = %v, want %s", resp.File.FolderID, folderID)
	}
	if resp.File.Title != "Plugin Import Design File Title" {
		t.Fatalf("title = %q, want Plugin Import Design File Title", resp.File.Title)
	}
	var afterCount int
	if err := testPool.QueryRow(context.Background(), `SELECT count(*) FROM design_file WHERE workspace_id = $1 AND project_id = $2 AND folder_id = $3`, testWorkspaceID, projectID, folderID).Scan(&afterCount); err != nil {
		t.Fatalf("count design files after import: %v", err)
	}
	if afterCount != beforeCount+1 {
		t.Fatalf("file count after import = %d, want %d", afterCount, beforeCount+1)
	}
	t.Cleanup(func() {
		_, _ = testPool.Exec(context.Background(), `DELETE FROM design_file WHERE id = $1`, resp.File.ID)
	})
}

func TestFigmaPluginImportTargetDesignFileMergesNewSourceNode(t *testing.T) {
	projectID := createProjectForDesignTest(t, "Plugin Merge Project")
	folderID := createDesignFolderForTest(t, projectID, "Plugin Merge Folder")
	token := createPluginTokenForTest(t)

	initialReq := newRequest("POST", "/api/design-plugin/figma/imports", map[string]any{
		"title":               "Plugin Merge Design",
		"design_file_title":   "Plugin Merge Design",
		"project_id":          projectID,
		"folder_id":           folderID,
		"source_ref":          map[string]any{"provider": "figma", "source_key": "merge-source"},
		"native_json":         figmaDesignNativeJSONWithSourceNodes("Plugin Merge Design", "1:1", "1:2", "1:3", "1:4"),
		"publish_as_template": false,
	})
	initialReq.Header.Set("Authorization", "Bearer "+token)
	initialW := httptest.NewRecorder()
	testHandler.ImportFigmaDesignWithPluginToken(initialW, initialReq)
	if initialW.Code != http.StatusCreated {
		t.Fatalf("initial plugin import: expected 201, got %d: %s", initialW.Code, initialW.Body.String())
	}
	var initial DesignFileDetailResponse
	if err := json.NewDecoder(initialW.Body).Decode(&initial); err != nil {
		t.Fatalf("decode initial plugin import: %v", err)
	}
	t.Cleanup(func() {
		_, _ = testPool.Exec(context.Background(), `DELETE FROM design_file WHERE id = $1`, initial.File.ID)
	})
	if initial.CurrentRevision == nil || initial.CurrentRevision.RevisionNumber != 1 {
		t.Fatalf("initial revision = %+v, want number 1", initial.CurrentRevision)
	}
	if got := frameCountFromNativeJSONForTest(t, initial.CurrentRevision.NativeJSON); got != 4 {
		t.Fatalf("initial frame count = %d, want 4", got)
	}
	var beforeCount int
	if err := testPool.QueryRow(context.Background(), `SELECT count(*) FROM design_file WHERE workspace_id = $1 AND project_id = $2 AND folder_id = $3`, testWorkspaceID, projectID, folderID).Scan(&beforeCount); err != nil {
		t.Fatalf("count design files before merge: %v", err)
	}

	mergeReq := newRequest("POST", "/api/design-plugin/figma/imports", map[string]any{
		"title":                 "Plugin Merge Design New Frame",
		"project_id":            projectID,
		"folder_id":             folderID,
		"target_design_file_id": initial.File.ID,
		"source_ref":            map[string]any{"provider": "figma", "source_key": "merge-source"},
		"native_json":           figmaDesignNativeJSONWithSourceNodes("Plugin Merge Design New Frame", "1:5"),
		"publish_as_template":   false,
	})
	mergeReq.Header.Set("Authorization", "Bearer "+token)
	mergeW := httptest.NewRecorder()
	testHandler.ImportFigmaDesignWithPluginToken(mergeW, mergeReq)
	if mergeW.Code != http.StatusCreated {
		t.Fatalf("merge plugin import: expected 201, got %d: %s", mergeW.Code, mergeW.Body.String())
	}
	var merged DesignFileDetailResponse
	if err := json.NewDecoder(mergeW.Body).Decode(&merged); err != nil {
		t.Fatalf("decode merge plugin import: %v", err)
	}
	if merged.File.ID != initial.File.ID {
		t.Fatalf("merged file id = %s, want %s", merged.File.ID, initial.File.ID)
	}
	if merged.CurrentRevision == nil || merged.CurrentRevision.RevisionNumber != 2 {
		t.Fatalf("merged revision = %+v, want number 2", merged.CurrentRevision)
	}
	if got := frameCountFromNativeJSONForTest(t, merged.CurrentRevision.NativeJSON); got != 5 {
		t.Fatalf("merged frame count = %d, want 5", got)
	}
	var afterCount int
	if err := testPool.QueryRow(context.Background(), `SELECT count(*) FROM design_file WHERE workspace_id = $1 AND project_id = $2 AND folder_id = $3`, testWorkspaceID, projectID, folderID).Scan(&afterCount); err != nil {
		t.Fatalf("count design files after merge: %v", err)
	}
	if afterCount != beforeCount {
		t.Fatalf("file count after merge = %d, want %d", afterCount, beforeCount)
	}
}

func TestFigmaPluginImportCanPublishTemplate(t *testing.T) {
	projectID := createProjectForDesignTest(t, "Plugin Template Project")
	token := createPluginTokenForTest(t)
	templateKey := fmt.Sprintf("plugin-template-%d", time.Now().UnixNano())
	req := newRequest("POST", "/api/design-plugin/figma/imports", map[string]any{
		"title":                "Plugin Template Design",
		"project_id":           projectID,
		"source_ref":           map[string]any{"provider": "figma", "source_key": templateKey},
		"native_json":          minimalDesignNativeJSON("Plugin Template Design"),
		"publish_as_template":  true,
		"template_library_key": "figma",
		"template_key":         templateKey,
		"template_name":        "Plugin Published Template",
		"template_category":    "figma",
	})
	req.Header.Set("Authorization", "Bearer "+token)
	w := httptest.NewRecorder()
	testHandler.ImportFigmaDesignWithPluginToken(w, req)
	if w.Code != http.StatusCreated {
		t.Fatalf("plugin import template: expected 201, got %d: %s", w.Code, w.Body.String())
	}
	var resp struct {
		File     DesignFileResponse             `json:"file"`
		Template *DesignCatalogTemplateResponse `json:"template"`
	}
	if err := json.NewDecoder(w.Body).Decode(&resp); err != nil {
		t.Fatalf("decode plugin template import: %v", err)
	}
	if resp.Template == nil {
		t.Fatal("expected template response")
	}
	if resp.Template.Name != "Plugin Published Template" {
		t.Fatalf("template name = %q", resp.Template.Name)
	}
	t.Cleanup(func() {
		_, _ = testPool.Exec(context.Background(), `DELETE FROM design_file WHERE id = $1`, resp.File.ID)
		_, _ = testPool.Exec(context.Background(), `DELETE FROM design_template_library WHERE workspace_id = $1 AND key = 'figma'`, testWorkspaceID)
	})
}

func TestFigmaPluginImportTemplateRequiresProject(t *testing.T) {
	token := createPluginTokenForTest(t)
	templateKey := fmt.Sprintf("plugin-template-no-project-%d", time.Now().UnixNano())
	req := newRequest("POST", "/api/design-plugin/figma/imports", map[string]any{
		"title":                "Plugin Template Design Without Project",
		"source_ref":           map[string]any{"provider": "figma", "source_key": templateKey},
		"native_json":          minimalDesignNativeJSON("Plugin Template Design Without Project"),
		"publish_as_template":  true,
		"template_library_key": "figma",
		"template_key":         templateKey,
		"template_name":        "Plugin Published Template Without Project",
		"template_category":    "figma",
	})
	req.Header.Set("Authorization", "Bearer "+token)
	w := httptest.NewRecorder()
	testHandler.ImportFigmaDesignWithPluginToken(w, req)
	if w.Code != http.StatusBadRequest {
		t.Fatalf("plugin import template without project: expected 400, got %d: %s", w.Code, w.Body.String())
	}
}

func TestFigmaPluginRepeatedImportWithoutTargetCreatesNewFile(t *testing.T) {
	projectID := createProjectForDesignTest(t, "Plugin Version Project")
	folderID := createDesignFolderForTest(t, projectID, "Plugin Version Folder")
	token := createPluginTokenForTest(t)
	sourceRef := map[string]any{
		"tool":       "figma",
		"file_key":   "figma-file-1",
		"page_id":    "page-1",
		"scope":      "selected",
		"source_key": "figma:figma-file-1:page:page-1:scope:selected:nodes:1:2",
		"node_ids":   []string{"1:2"},
	}

	postImport := func(title string) DesignFileDetailResponse {
		req := newRequest("POST", "/api/design-plugin/figma/imports", map[string]any{
			"title":       title,
			"project_id":  projectID,
			"folder_id":   folderID,
			"source_ref":  sourceRef,
			"native_json": minimalDesignNativeJSON(title),
		})
		req.Header.Set("Authorization", "Bearer "+token)
		w := httptest.NewRecorder()
		testHandler.ImportFigmaDesignWithPluginToken(w, req)
		if w.Code != http.StatusCreated {
			t.Fatalf("plugin import %s: expected 201, got %d: %s", title, w.Code, w.Body.String())
		}
		var resp DesignFileDetailResponse
		if err := json.NewDecoder(w.Body).Decode(&resp); err != nil {
			t.Fatalf("decode plugin import %s: %v", title, err)
		}
		return resp
	}

	first := postImport("Plugin Version Design v1")
	t.Cleanup(func() {
		_, _ = testPool.Exec(context.Background(), `DELETE FROM design_file WHERE id = $1`, first.File.ID)
	})
	second := postImport("Plugin Version Design v2")
	t.Cleanup(func() {
		_, _ = testPool.Exec(context.Background(), `DELETE FROM design_file WHERE id = $1`, second.File.ID)
	})

	if second.File.ID == first.File.ID {
		t.Fatalf("second upload file id = %s, want a new file when no target_design_file_id is provided", second.File.ID)
	}
	if first.CurrentRevision == nil || first.CurrentRevision.RevisionNumber != 1 {
		t.Fatalf("first revision = %+v, want number 1", first.CurrentRevision)
	}
	if second.CurrentRevision == nil || second.CurrentRevision.RevisionNumber != 1 {
		t.Fatalf("second revision = %+v, want number 1", second.CurrentRevision)
	}
	var fileCount int
	if err := testPool.QueryRow(context.Background(), `
		SELECT count(*) FROM design_file
		WHERE workspace_id = $1 AND project_id = $2 AND folder_id = $3 AND source_ref->>'source_key' = $4
	`, testWorkspaceID, projectID, folderID, sourceRef["source_key"]).Scan(&fileCount); err != nil {
		t.Fatalf("count design files: %v", err)
	}
	if fileCount != 2 {
		t.Fatalf("file count = %d, want 2", fileCount)
	}
}
