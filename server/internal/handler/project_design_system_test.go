package handler

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgtype"
	"github.com/multica-ai/multica/server/internal/projectdesignsystem"
	db "github.com/multica-ai/multica/server/pkg/db/generated"
)

func TestCreateProjectDesignSystemRequiresExplicitReadyAgent(t *testing.T) {
	projectID := createProjectForDesignTest(t, "Explicit agent project")

	missingAgent := performProjectDesignSystemRequest(t, testHandler.CreateProjectDesignSystem, http.MethodPost, "/api/project-design-systems", map[string]any{
		"project_id": projectID,
		"platform":   "web",
		"brief":      "A focused operational product.",
	})
	assertProjectDesignSystemErrorCode(t, missingAgent, http.StatusBadRequest, "agent_id_required")

	offlineAgentID, _ := createProjectDesignSystemAgent(t, "offline")
	offline := performProjectDesignSystemRequest(t, testHandler.CreateProjectDesignSystem, http.MethodPost, "/api/project-design-systems", map[string]any{
		"project_id": projectID,
		"agent_id":   offlineAgentID,
		"platform":   "web",
		"brief":      "A focused operational product.",
	})
	assertProjectDesignSystemErrorCode(t, offline, http.StatusConflict, "agent_unavailable")

	var count int
	if err := testPool.QueryRow(context.Background(), `SELECT count(*) FROM project_design_system WHERE project_id = $1`, projectID).Scan(&count); err != nil {
		t.Fatalf("count project design systems: %v", err)
	}
	if count != 0 {
		t.Fatalf("project design system count = %d, want 0 after rejected dispatches", count)
	}
}

func TestCreateProjectDesignSystemRequiresPlatformAndBrief(t *testing.T) {
	projectID := createProjectForDesignTest(t, "Required input project")
	agentID, _ := createProjectDesignSystemAgent(t, "online")

	tests := []struct {
		name string
		body map[string]any
		code string
	}{
		{
			name: "platform",
			body: map[string]any{"project_id": projectID, "agent_id": agentID, "brief": "Operational product."},
			code: "platform_required",
		},
		{
			name: "brief",
			body: map[string]any{"project_id": projectID, "agent_id": agentID, "platform": "web", "brief": "  "},
			code: "brief_required",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			response := performProjectDesignSystemRequest(t, testHandler.CreateProjectDesignSystem, http.MethodPost, "/api/project-design-systems", tt.body)
			assertProjectDesignSystemErrorCode(t, response, http.StatusBadRequest, tt.code)
		})
	}
}

func TestCreateProjectDesignSystemSnapshotsExactInputs(t *testing.T) {
	projectID := createProjectForDesignTest(t, "Snapshot project")
	if _, err := testPool.Exec(context.Background(), `UPDATE project SET description = $2 WHERE id = $1`, projectID, "Current CRM for service teams"); err != nil {
		t.Fatalf("update project description: %v", err)
	}
	agentID, _ := createProjectDesignSystemAgent(t, "online")
	attachmentID, designFileID, profileID := createProjectDesignSystemReferencesForTest(t, projectID)

	response := performProjectDesignSystemRequest(t, testHandler.CreateProjectDesignSystem, http.MethodPost, "/api/project-design-systems", map[string]any{
		"project_id": projectID,
		"agent_id":   agentID,
		"platform":   "web",
		"brief":      "  Calm CRM for repeated customer operations.  ",
		"references": []map[string]any{
			{"kind": "brand_color", "value": "#abc", "label": "Primary"},
			{"kind": "link", "value": "https://example.com/brand", "label": "Brand guide"},
			{"kind": "attachment", "attachment_id": attachmentID, "label": "Logo"},
			{"kind": "design_file", "design_file_id": designFileID, "label": "Current dashboard"},
			{"kind": "design_system_profile", "design_system_profile_id": profileID, "label": "Figma UI specification"},
		},
	})
	if response.Code != http.StatusAccepted {
		t.Fatalf("CreateProjectDesignSystem: status = %d, body = %s", response.Code, response.Body.String())
	}

	var created ProjectDesignSystemResponse
	if err := json.NewDecoder(response.Body).Decode(&created); err != nil {
		t.Fatalf("decode create response: %v", err)
	}
	if created.ID == "" || created.ActiveTask == nil || created.ActiveTask.ID == "" {
		t.Fatalf("create response missing system/task identity: %+v", created)
	}

	var inputJSON, taskContextJSON []byte
	if err := testPool.QueryRow(context.Background(), `
		SELECT pds.input_snapshot, task.context
		FROM project_design_system pds, agent_task_queue task
		WHERE pds.id = $1 AND task.id = pds.active_task_id
	`, created.ID).Scan(&inputJSON, &taskContextJSON); err != nil {
		t.Fatalf("load frozen input/task context: %v", err)
	}

	var input map[string]any
	if err := json.Unmarshal(inputJSON, &input); err != nil {
		t.Fatalf("decode input snapshot: %v", err)
	}
	if input["agent_id"] != agentID || input["platform"] != "web" || input["brief"] != "Calm CRM for repeated customer operations." {
		t.Fatalf("input snapshot lost exact selected values: %#v", input)
	}
	references, ok := input["references"].([]any)
	if !ok || len(references) != 5 {
		t.Fatalf("input references = %#v, want 5 frozen references", input["references"])
	}
	color := references[0].(map[string]any)
	if color["kind"] != "brand_color" || color["value"] != "#AABBCC" || color["label"] != "Primary" {
		t.Fatalf("brand color snapshot = %#v", color)
	}
	link := references[1].(map[string]any)
	if link["kind"] != "link" || link["url"] != "https://example.com/brand" || link["label"] != "Brand guide" {
		t.Fatalf("link snapshot = %#v", link)
	}
	attachment := references[2].(map[string]any)
	if attachment["attachment_id"] != attachmentID || attachment["filename"] != "atlas-logo.png" || attachment["content_type"] != "image/png" || attachment["url"] != "https://static.soyoung.com/atlas-logo.png" {
		t.Fatalf("attachment snapshot = %#v", attachment)
	}
	designFile := references[3].(map[string]any)
	if designFile["design_file_id"] != designFileID || designFile["title"] != "Atlas dashboard" || designFile["thumbnail_url"] != "https://static.soyoung.com/atlas-dashboard.png" {
		t.Fatalf("design file snapshot = %#v", designFile)
	}
	frames := designFile["frames"].([]any)
	if len(frames) != 1 || frames[0].(map[string]any)["name"] != "Dashboard" || frames[0].(map[string]any)["preview_url"] != "https://static.soyoung.com/atlas-dashboard.png" {
		t.Fatalf("design file frame snapshot = %#v", frames)
	}
	profile := references[4].(map[string]any)
	if profile["design_system_profile_id"] != profileID || profile["title"] != "Atlas Figma UI specification" {
		t.Fatalf("UI specification snapshot = %#v", profile)
	}
	profileJSON := profile["profile"].(map[string]any)
	if profileJSON["density"] != "compact" {
		t.Fatalf("UI specification profile snapshot = %#v", profileJSON)
	}

	var taskContext map[string]any
	if err := json.Unmarshal(taskContextJSON, &taskContext); err != nil {
		t.Fatalf("decode task context: %v", err)
	}
	if taskContext["type"] != "project_design_system_task" || taskContext["operation"] != "generate" {
		t.Fatalf("task discriminator = %#v", taskContext)
	}
	if taskContext["agent_id"] != agentID || taskContext["project_id"] != projectID || taskContext["project_design_system_id"] != created.ID {
		t.Fatalf("task identity snapshot = %#v", taskContext)
	}
	project := taskContext["project"].(map[string]any)
	if project["name"] != "Snapshot project" || project["description"] != "Current CRM for service teams" {
		t.Fatalf("task project snapshot = %#v", project)
	}
}

func TestCreateProjectDesignSystemRejectsSecondSystem(t *testing.T) {
	projectID := createProjectForDesignTest(t, "Single system project")
	agentID, _ := createProjectDesignSystemAgent(t, "online")
	body := map[string]any{
		"project_id": projectID,
		"agent_id":   agentID,
		"platform":   "mobile",
		"brief":      "A concise field-service app.",
	}

	first := performProjectDesignSystemRequest(t, testHandler.CreateProjectDesignSystem, http.MethodPost, "/api/project-design-systems", body)
	if first.Code != http.StatusAccepted {
		t.Fatalf("first create status = %d, body = %s", first.Code, first.Body.String())
	}
	second := performProjectDesignSystemRequest(t, testHandler.CreateProjectDesignSystem, http.MethodPost, "/api/project-design-systems", body)
	assertProjectDesignSystemErrorCode(t, second, http.StatusConflict, "project_design_system_exists")

	var count int
	if err := testPool.QueryRow(context.Background(), `SELECT count(*) FROM project_design_system WHERE project_id = $1`, projectID).Scan(&count); err != nil {
		t.Fatalf("count project design systems: %v", err)
	}
	if count != 1 {
		t.Fatalf("project design system count = %d, want 1", count)
	}
}

func TestCreateProjectDesignSystemRejectsUnsafeOrForeignReferences(t *testing.T) {
	projectID := createProjectForDesignTest(t, "Reference boundary project")
	foreignProjectID := createProjectForDesignTest(t, "Foreign reference project")
	_, foreignDesignFileID, _ := createProjectDesignSystemReferencesForTest(t, foreignProjectID)
	agentID, _ := createProjectDesignSystemAgent(t, "online")
	base := map[string]any{
		"project_id": projectID,
		"agent_id":   agentID,
		"platform":   "web",
		"brief":      "Reference validation.",
	}

	tests := []struct {
		name       string
		references any
		status     int
		code       string
	}{
		{name: "unknown kind", references: []map[string]any{{"kind": "moodboard"}}, status: http.StatusBadRequest, code: "reference_kind_invalid"},
		{name: "non HTTPS link", references: []map[string]any{{"kind": "link", "value": "http://example.com"}}, status: http.StatusBadRequest, code: "reference_invalid"},
		{name: "foreign design file", references: []map[string]any{{"kind": "design_file", "design_file_id": foreignDesignFileID}}, status: http.StatusNotFound, code: "reference_not_found"},
		{name: "oversized snapshot", references: []map[string]any{{"kind": "brand_color", "value": "#2463eb", "label": strings.Repeat("x", maxProjectDesignSystemSnapshotBytes)}}, status: http.StatusRequestEntityTooLarge, code: "input_snapshot_too_large"},
	}
	many := make([]map[string]any, maxProjectDesignSystemReferences+1)
	for index := range many {
		many[index] = map[string]any{"kind": "brand_color", "value": "#2463eb"}
	}
	tests = append(tests, struct {
		name       string
		references any
		status     int
		code       string
	}{name: "too many references", references: many, status: http.StatusBadRequest, code: "too_many_references"})

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			body := make(map[string]any, len(base)+1)
			for key, value := range base {
				body[key] = value
			}
			body["references"] = tt.references
			response := performProjectDesignSystemRequest(t, testHandler.CreateProjectDesignSystem, http.MethodPost, "/api/project-design-systems", body)
			assertProjectDesignSystemErrorCode(t, response, tt.status, tt.code)
		})
	}
}

func TestGetProjectDesignSystemReturnsUnestablishedAfterFailedFirstRun(t *testing.T) {
	projectID := createProjectForDesignTest(t, "Failed first run project")
	agentID, _ := createProjectDesignSystemAgent(t, "online")
	input := projectDesignSystemInputSnapshot{
		AgentID:    agentID,
		Platform:   "web",
		Brief:      "Keep this input after failure.",
		References: []projectDesignSystemReferenceSnapshot{},
	}
	system := createProjectDesignSystemIdentityForTest(t, projectID, agentID, input)
	if _, err := testPool.Exec(context.Background(), `
		UPDATE project_design_system
		SET active_task_id = NULL,
		    active_operation = NULL,
		    last_error = '{"code":"agent_failed","message":"generation failed"}'::jsonb
		WHERE id = $1
	`, uuidToString(system.ID)); err != nil {
		t.Fatalf("record failed first run: %v", err)
	}

	response := performProjectDesignSystemRequest(t, testHandler.GetProjectDesignSystemByProject, http.MethodGet, "/api/project-design-systems?project_id="+projectID, nil)
	if response.Code != http.StatusOK {
		t.Fatalf("GetProjectDesignSystemByProject: status = %d, body = %s", response.Code, response.Body.String())
	}
	var got ProjectDesignSystemResponse
	if err := json.NewDecoder(response.Body).Decode(&got); err != nil {
		t.Fatalf("decode response: %v", err)
	}
	if got.Status != "unestablished" || got.ActiveTask != nil || got.HasUnsavedChanges {
		t.Fatalf("failed first run response = %+v", got)
	}
	var snapshot map[string]any
	if err := json.Unmarshal(got.InputSnapshot, &snapshot); err != nil || snapshot["brief"] != input.Brief {
		t.Fatalf("input snapshot after failure = %#v, err = %v", snapshot, err)
	}
	var lastError map[string]any
	if err := json.Unmarshal(got.LastError, &lastError); err != nil || lastError["code"] != "agent_failed" {
		t.Fatalf("last error = %#v, err = %v", lastError, err)
	}
}

func TestAdjustProjectDesignSystemValidatesScopeAgainstManifest(t *testing.T) {
	projectID := createProjectForDesignTest(t, "Scoped adjustment project")
	agentID, _ := createProjectDesignSystemAgent(t, "online")
	input := projectDesignSystemInputSnapshot{
		AgentID:    agentID,
		Platform:   "web",
		Brief:      "A calm CRM.",
		References: []projectDesignSystemReferenceSnapshot{},
	}
	system := createProjectDesignSystemIdentityForTest(t, projectID, agentID, input)
	pkg := validProjectDesignSystemPackageForTest(t)
	upsertValidatedProjectDesignSystemPackageForTest(t, system.ID, "draft", pkg)

	invalid := performProjectDesignSystemIDRequest(t, testHandler.AdjustProjectDesignSystem, http.MethodPost, "/api/project-design-systems/"+uuidToString(system.ID)+"/adjust", uuidToString(system.ID), map[string]any{
		"agent_id":    agentID,
		"instruction": "Make this section denser.",
		"scope":       map[string]any{"kind": "section", "id": "missing-section"},
	})
	assertProjectDesignSystemErrorCode(t, invalid, http.StatusBadRequest, "scope_not_found")

	valid := performProjectDesignSystemIDRequest(t, testHandler.AdjustProjectDesignSystem, http.MethodPost, "/api/project-design-systems/"+uuidToString(system.ID)+"/adjust", uuidToString(system.ID), map[string]any{
		"agent_id":    agentID,
		"instruction": "Make the primary button more decisive.",
		"scope":       map[string]any{"kind": "component", "id": "button-primary"},
	})
	if valid.Code != http.StatusAccepted {
		t.Fatalf("AdjustProjectDesignSystem: status = %d, body = %s", valid.Code, valid.Body.String())
	}
	var response ProjectDesignSystemResponse
	if err := json.NewDecoder(valid.Body).Decode(&response); err != nil {
		t.Fatalf("decode adjustment response: %v", err)
	}
	if response.ActiveTask == nil || response.Status != "generating" {
		t.Fatalf("adjustment response = %+v", response)
	}

	var contextJSON []byte
	if err := testPool.QueryRow(context.Background(), `SELECT context FROM agent_task_queue WHERE id = $1`, response.ActiveTask.ID).Scan(&contextJSON); err != nil {
		t.Fatalf("load adjustment context: %v", err)
	}
	var taskContext map[string]any
	if err := json.Unmarshal(contextJSON, &taskContext); err != nil {
		t.Fatalf("decode adjustment context: %v", err)
	}
	if taskContext["operation"] != "adjust" || taskContext["instruction"] != "Make the primary button more decisive." {
		t.Fatalf("adjustment task context = %#v", taskContext)
	}
	scope := taskContext["scope"].(map[string]any)
	if scope["kind"] != "component" || scope["id"] != "button-primary" {
		t.Fatalf("adjustment scope = %#v", scope)
	}
	base := taskContext["base_package"].(map[string]any)
	if !strings.Contains(base["design_md"].(string), "Atlas CRM") {
		t.Fatalf("base package was not frozen into task context: %#v", base)
	}
}

func TestRegenerateProjectDesignSystemPreservesCurrentPackage(t *testing.T) {
	projectID := createProjectForDesignTest(t, "Regeneration project")
	agentID, _ := createProjectDesignSystemAgent(t, "online")
	input := projectDesignSystemInputSnapshot{
		AgentID:  agentID,
		Platform: "web",
		Brief:    "Original direction.",
		References: []projectDesignSystemReferenceSnapshot{
			{Kind: "brand_color", Label: "Primary", Value: "#2463EB"},
		},
	}
	system := createProjectDesignSystemIdentityForTest(t, projectID, agentID, input)
	pkg := validProjectDesignSystemPackageForTest(t)
	upsertValidatedProjectDesignSystemPackageForTest(t, system.ID, "saved", pkg)

	response := performProjectDesignSystemIDRequest(t, testHandler.RegenerateProjectDesignSystem, http.MethodPost, "/api/project-design-systems/"+uuidToString(system.ID)+"/regenerate", uuidToString(system.ID), map[string]any{
		"agent_id": agentID,
		"platform": "mobile",
		"brief":    "A touch-first field operations system.",
	})
	if response.Code != http.StatusAccepted {
		t.Fatalf("RegenerateProjectDesignSystem: status = %d, body = %s", response.Code, response.Body.String())
	}

	queries := db.New(testPool)
	saved, err := queries.GetProjectDesignSystemPackageBySlot(context.Background(), db.GetProjectDesignSystemPackageBySlotParams{
		DesignSystemID: system.ID,
		Slot:           "saved",
		WorkspaceID:    parseUUID(testWorkspaceID),
	})
	if err != nil {
		t.Fatalf("load saved package after regenerate: %v", err)
	}
	if saved.IntegritySha256 != pkg.Manifest.Digest || saved.DesignMd != pkg.Artifacts.DesignMD {
		t.Fatalf("regenerate changed saved package: %+v", saved)
	}

	var got ProjectDesignSystemResponse
	if err := json.NewDecoder(response.Body).Decode(&got); err != nil {
		t.Fatalf("decode regenerate response: %v", err)
	}
	var contextJSON []byte
	if err := testPool.QueryRow(context.Background(), `SELECT context FROM agent_task_queue WHERE id = $1`, got.ActiveTask.ID).Scan(&contextJSON); err != nil {
		t.Fatalf("load regenerate context: %v", err)
	}
	var taskContext map[string]any
	if err := json.Unmarshal(contextJSON, &taskContext); err != nil {
		t.Fatalf("decode regenerate context: %v", err)
	}
	if taskContext["operation"] != "regenerate" || taskContext["platform"] != "mobile" || taskContext["brief"] != "A touch-first field operations system." {
		t.Fatalf("regenerate task context = %#v", taskContext)
	}
	references := taskContext["references"].([]any)
	if len(references) != 1 || references[0].(map[string]any)["value"] != "#2463EB" {
		t.Fatalf("regenerate did not preserve omitted references: %#v", references)
	}
}

func TestSaveProjectDesignSystemRequiresValidatedDraft(t *testing.T) {
	projectID := createProjectForDesignTest(t, "Save validation project")
	agentID, _ := createProjectDesignSystemAgent(t, "online")
	input := projectDesignSystemInputSnapshot{AgentID: agentID, Platform: "web", Brief: "Save only valid work.", References: []projectDesignSystemReferenceSnapshot{}}
	system := createProjectDesignSystemIdentityForTest(t, projectID, agentID, input)
	systemID := uuidToString(system.ID)

	missing := performProjectDesignSystemIDRequest(t, testHandler.SaveProjectDesignSystem, http.MethodPost, "/api/project-design-systems/"+systemID+"/save", systemID, nil)
	assertProjectDesignSystemErrorCode(t, missing, http.StatusConflict, "draft_required")

	queries := db.New(testPool)
	if _, err := queries.UpsertProjectDesignSystemPackage(context.Background(), db.UpsertProjectDesignSystemPackageParams{
		DesignSystemID:  system.ID,
		Slot:            "draft",
		DesignMd:        "invalid",
		TokensCss:       "invalid",
		ComponentsHtml:  "invalid",
		Manifest:        []byte(`{}`),
		Validation:      []byte(`{"passed":false}`),
		IntegritySha256: strings.Repeat("f", 64),
		WorkspaceID:     parseUUID(testWorkspaceID),
	}); err != nil {
		t.Fatalf("insert invalid draft: %v", err)
	}
	invalid := performProjectDesignSystemIDRequest(t, testHandler.SaveProjectDesignSystem, http.MethodPost, "/api/project-design-systems/"+systemID+"/save", systemID, nil)
	assertProjectDesignSystemErrorCode(t, invalid, http.StatusUnprocessableEntity, "draft_invalid")

	pkg := validProjectDesignSystemPackageForTest(t)
	upsertValidatedProjectDesignSystemPackageForTest(t, system.ID, "draft", pkg)
	savedResponse := performProjectDesignSystemIDRequest(t, testHandler.SaveProjectDesignSystem, http.MethodPost, "/api/project-design-systems/"+systemID+"/save", systemID, nil)
	if savedResponse.Code != http.StatusOK {
		t.Fatalf("SaveProjectDesignSystem: status = %d, body = %s", savedResponse.Code, savedResponse.Body.String())
	}
	var response ProjectDesignSystemResponse
	if err := json.NewDecoder(savedResponse.Body).Decode(&response); err != nil {
		t.Fatalf("decode saved response: %v", err)
	}
	if response.Status != "saved" || response.HasUnsavedChanges {
		t.Fatalf("saved response = %+v", response)
	}
	if _, err := queries.GetProjectDesignSystemPackageBySlot(context.Background(), db.GetProjectDesignSystemPackageBySlotParams{DesignSystemID: system.ID, Slot: "draft", WorkspaceID: parseUUID(testWorkspaceID)}); !errors.Is(err, pgx.ErrNoRows) {
		t.Fatalf("draft lookup error = %v, want pgx.ErrNoRows", err)
	}
	saved, err := queries.GetProjectDesignSystemPackageBySlot(context.Background(), db.GetProjectDesignSystemPackageBySlotParams{DesignSystemID: system.ID, Slot: "saved", WorkspaceID: parseUUID(testWorkspaceID)})
	if err != nil || saved.IntegritySha256 != pkg.Manifest.Digest {
		t.Fatalf("saved package = %+v, err = %v", saved, err)
	}
}

func TestProjectDesignSystemRoutesRejectForeignWorkspace(t *testing.T) {
	foreignWorkspaceID := createProjectDesignSystemWorkspace(t)
	foreignProjectID := createProjectDesignSystemProject(t, uuidToString(foreignWorkspaceID), "Foreign design system project")
	system := createProjectDesignSystemForTest(t, db.New(testPool), foreignWorkspaceID, foreignProjectID, "Foreign design system")
	systemID := uuidToString(system.ID)

	lookup := performProjectDesignSystemRequest(t, testHandler.GetProjectDesignSystemByProject, http.MethodGet, "/api/project-design-systems?project_id="+uuidToString(foreignProjectID), nil)
	assertProjectDesignSystemErrorCode(t, lookup, http.StatusNotFound, "project_not_found")

	tests := []struct {
		name    string
		handler http.HandlerFunc
		path    string
		body    any
	}{
		{name: "detail", handler: testHandler.GetProjectDesignSystem, path: "/api/project-design-systems/" + systemID},
		{name: "adjust", handler: testHandler.AdjustProjectDesignSystem, path: "/api/project-design-systems/" + systemID + "/adjust", body: map[string]any{"agent_id": testUserID, "instruction": "change", "scope": map[string]any{"kind": "all"}}},
		{name: "regenerate", handler: testHandler.RegenerateProjectDesignSystem, path: "/api/project-design-systems/" + systemID + "/regenerate", body: map[string]any{"agent_id": testUserID}},
		{name: "save", handler: testHandler.SaveProjectDesignSystem, path: "/api/project-design-systems/" + systemID + "/save"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			response := performProjectDesignSystemIDRequest(t, tt.handler, http.MethodPost, tt.path, systemID, tt.body)
			assertProjectDesignSystemErrorCode(t, response, http.StatusNotFound, "project_design_system_not_found")
		})
	}
}

func createProjectDesignSystemAgent(t *testing.T, runtimeStatus string) (string, string) {
	t.Helper()
	suffix := time.Now().UnixNano()
	var runtimeID string
	if err := testPool.QueryRow(context.Background(), `
		INSERT INTO agent_runtime (
			workspace_id, daemon_id, name, runtime_mode, provider, status,
			device_info, metadata, last_seen_at, owner_id
		) VALUES ($1, NULL, $2, 'cloud', 'project_design_system_test', $3, '', '{}'::jsonb, now(), $4)
		RETURNING id
	`, testWorkspaceID, fmt.Sprintf("Project Design System Runtime %d", suffix), runtimeStatus, testUserID).Scan(&runtimeID); err != nil {
		t.Fatalf("create project design system runtime: %v", err)
	}

	var agentID string
	if err := testPool.QueryRow(context.Background(), `
		INSERT INTO agent (
			workspace_id, name, description, runtime_mode, runtime_config,
			runtime_id, visibility, max_concurrent_tasks, owner_id
		) VALUES ($1, $2, '', 'cloud', '{}'::jsonb, $3, 'workspace', 1, $4)
		RETURNING id
	`, testWorkspaceID, fmt.Sprintf("Project Design System Agent %d", suffix), runtimeID, testUserID).Scan(&agentID); err != nil {
		t.Fatalf("create project design system agent: %v", err)
	}

	t.Cleanup(func() {
		_, _ = testPool.Exec(context.Background(), `DELETE FROM agent_task_queue WHERE agent_id = $1`, agentID)
		_, _ = testPool.Exec(context.Background(), `DELETE FROM agent WHERE id = $1`, agentID)
		_, _ = testPool.Exec(context.Background(), `DELETE FROM agent_runtime WHERE id = $1`, runtimeID)
	})
	return agentID, runtimeID
}

func performProjectDesignSystemRequest(
	t *testing.T,
	handler http.HandlerFunc,
	method string,
	path string,
	body any,
) *httptest.ResponseRecorder {
	t.Helper()
	recorder := httptest.NewRecorder()
	handler(recorder, newRequest(method, path, body))
	return recorder
}

func performProjectDesignSystemIDRequest(
	t *testing.T,
	handler http.HandlerFunc,
	method string,
	path string,
	id string,
	body any,
) *httptest.ResponseRecorder {
	t.Helper()
	recorder := httptest.NewRecorder()
	request := withURLParam(newRequest(method, path, body), "id", id)
	handler(recorder, request)
	return recorder
}

func createProjectDesignSystemIdentityForTest(
	t *testing.T,
	projectID string,
	agentID string,
	input projectDesignSystemInputSnapshot,
) db.ProjectDesignSystem {
	t.Helper()
	inputJSON, err := json.Marshal(input)
	if err != nil {
		t.Fatalf("marshal project design system input: %v", err)
	}
	system, err := db.New(testPool).CreateProjectDesignSystem(context.Background(), db.CreateProjectDesignSystemParams{
		WorkspaceID:    parseUUID(testWorkspaceID),
		ProjectID:      parseUUID(projectID),
		Name:           "Test design system",
		Platform:       input.Platform,
		CurrentAgentID: parseUUID(agentID),
		InputSnapshot:  inputJSON,
		CreatedBy:      parseUUID(testUserID),
	})
	if err != nil {
		t.Fatalf("create project design system identity: %v", err)
	}
	return system
}

func validProjectDesignSystemPackageForTest(t *testing.T) projectdesignsystem.ValidatedPackage {
	t.Helper()
	read := func(name string) string {
		data, err := os.ReadFile(filepath.Join("..", "projectdesignsystem", "testdata", "valid", name))
		if err != nil {
			t.Fatalf("read %s: %v", name, err)
		}
		return string(data)
	}
	pkg, err := projectdesignsystem.Validate(projectdesignsystem.ArtifactInput{
		DesignMD:       read("DESIGN.md"),
		TokensCSS:      read("tokens.css"),
		ComponentsHTML: read("components.html"),
	}, nil)
	if err != nil {
		t.Fatalf("validate project design system fixture: %v", err)
	}
	return pkg
}

func upsertValidatedProjectDesignSystemPackageForTest(
	t *testing.T,
	designSystemID pgtype.UUID,
	slot string,
	pkg projectdesignsystem.ValidatedPackage,
) {
	t.Helper()
	manifestJSON, err := json.Marshal(pkg.Manifest)
	if err != nil {
		t.Fatalf("marshal package manifest: %v", err)
	}
	validationJSON, err := json.Marshal(pkg.Validation)
	if err != nil {
		t.Fatalf("marshal package validation: %v", err)
	}
	if _, err := db.New(testPool).UpsertProjectDesignSystemPackage(context.Background(), db.UpsertProjectDesignSystemPackageParams{
		DesignSystemID:  designSystemID,
		Slot:            slot,
		DesignMd:        pkg.Artifacts.DesignMD,
		TokensCss:       pkg.Artifacts.TokensCSS,
		ComponentsHtml:  pkg.Artifacts.ComponentsHTML,
		Manifest:        manifestJSON,
		Validation:      validationJSON,
		IntegritySha256: pkg.Manifest.Digest,
		WorkspaceID:     parseUUID(testWorkspaceID),
	}); err != nil {
		t.Fatalf("upsert validated %s package: %v", slot, err)
	}
}

func createProjectDesignSystemReferencesForTest(t *testing.T, projectID string) (string, string, string) {
	t.Helper()
	ctx := context.Background()
	var attachmentID string
	if err := testPool.QueryRow(ctx, `
		INSERT INTO attachment (
			id, workspace_id, uploader_type, uploader_id, filename, url, content_type, size_bytes
		) VALUES (gen_random_uuid(), $1, 'member', $2, 'atlas-logo.png', 'https://static.soyoung.com/atlas-logo.png', 'image/png', 2048)
		RETURNING id
	`, testWorkspaceID, testUserID).Scan(&attachmentID); err != nil {
		t.Fatalf("create reference attachment: %v", err)
	}

	var designFileID string
	if err := testPool.QueryRow(ctx, `
		INSERT INTO design_file (workspace_id, project_id, title, description, source_type, source_ref, created_by)
		VALUES ($1, $2, 'Atlas dashboard', '', 'import', '{}'::jsonb, $3)
		RETURNING id
	`, testWorkspaceID, projectID, testUserID).Scan(&designFileID); err != nil {
		t.Fatalf("create reference design file: %v", err)
	}
	var revisionID string
	if err := testPool.QueryRow(ctx, `
		INSERT INTO design_revision (
			file_id, workspace_id, revision_number, status, native_json, validation_errors, created_by
		) VALUES (
			$1, $2, 1, 'valid',
			'{"frames":[{"id":"frame-1","name":"Dashboard","previewAssetId":"asset-preview"}],"assets":{"asset-preview":{"url":"https://static.soyoung.com/atlas-dashboard.png"}},"layers":{}}'::jsonb,
			'[]'::jsonb, $3
		)
		RETURNING id
	`, designFileID, testWorkspaceID, testUserID).Scan(&revisionID); err != nil {
		t.Fatalf("create reference design revision: %v", err)
	}
	if _, err := testPool.Exec(ctx, `UPDATE design_file SET current_revision_id = $2 WHERE id = $1`, designFileID, revisionID); err != nil {
		t.Fatalf("set current reference revision: %v", err)
	}

	var profileID string
	if err := testPool.QueryRow(ctx, `
		INSERT INTO design_system_profile (
			workspace_id, project_id, source_file_id, source_revision_id, name, description,
			status, is_default, profile_json, analysis_errors, created_by
		) VALUES (
			$1, $2, $3, $4, 'Atlas Figma UI specification', '',
			'analyzed', false, '{"density":"compact"}'::jsonb, '[]'::jsonb, $5
		)
		RETURNING id
	`, testWorkspaceID, projectID, designFileID, revisionID, testUserID).Scan(&profileID); err != nil {
		t.Fatalf("create reference UI specification: %v", err)
	}

	t.Cleanup(func() {
		_, _ = testPool.Exec(ctx, `DELETE FROM design_system_profile WHERE id = $1`, profileID)
		_, _ = testPool.Exec(ctx, `DELETE FROM design_revision WHERE id = $1`, revisionID)
		_, _ = testPool.Exec(ctx, `DELETE FROM design_file WHERE id = $1`, designFileID)
		_, _ = testPool.Exec(ctx, `DELETE FROM attachment WHERE id = $1`, attachmentID)
	})
	return attachmentID, designFileID, profileID
}

func assertProjectDesignSystemErrorCode(t *testing.T, response *httptest.ResponseRecorder, status int, code string) {
	t.Helper()
	if response.Code != status {
		t.Fatalf("status = %d, want %d: %s", response.Code, status, response.Body.String())
	}
	var payload map[string]any
	if err := json.NewDecoder(response.Body).Decode(&payload); err != nil {
		t.Fatalf("decode error response: %v", err)
	}
	if payload["code"] != code {
		t.Fatalf("error code = %#v, want %q: %#v", payload["code"], code, payload)
	}
}
