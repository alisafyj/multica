package handler

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/go-chi/chi/v5"
	"github.com/multica-ai/multica/server/internal/projectdesignsystem"
	"github.com/multica-ai/multica/server/internal/service"
	db "github.com/multica-ai/multica/server/pkg/db/generated"
)

func TestNativePackagePreviewCSPTrustsOnlyTheBridge(t *testing.T) {
	csp := nativePackagePreviewCSP("resource-capability")
	if !strings.Contains(csp, "script-src 'sha256-") {
		t.Fatalf("CSP does not pin the preview bridge: %q", csp)
	}
	for _, forbidden := range []string{"'unsafe-inline'", "connect-src 'self'", "object-src 'self'"} {
		if strings.Contains(csp, forbidden) {
			t.Fatalf("CSP includes forbidden directive %q: %q", forbidden, csp)
		}
	}
}

func TestInjectNativePackagePreviewBridgeKeepsTrustedAssetsInDocument(t *testing.T) {
	html := injectNativePackagePreviewBridge([]byte("<html><head></head><body><main>UI Kit</main></body></html>"), "resource-capability")
	value := string(html)
	bridge := nativePackagePreviewBridgeScript("resource-capability")
	if !strings.Contains(value, `<link rel="stylesheet" href="tokens.css">`) || !strings.Contains(value, bridge) {
		t.Fatalf("injected preview = %q", value)
	}
	if !strings.Contains(value, "resource-capability") {
		t.Fatalf("injected preview did not bind the resource capability: %q", value)
	}
	if strings.Index(value, "tokens.css") > strings.Index(value, "</head>") || strings.Index(value, bridge) > strings.Index(value, "</body>") {
		t.Fatalf("trusted preview assets were injected outside their document locations: %q", value)
	}
}

func TestNativePackageRetrievalRejectsForeignSelfConsistentArchiveBinding(t *testing.T) {
	fixture := newNativeV2CompletionFixture(t, service.ProjectDesignSystemGenerate)
	if response := fixture.completeTask(t, fixture.buildPackagePayload(t, nil)); response.Code != http.StatusOK {
		t.Fatalf("complete native package: status = %d, body = %s", response.Code, response.Body.String())
	}

	foreignBinding := fixture.Binding
	foreignBinding.TaskID = "00000000-0000-4000-8000-000000000099"
	foreign := collectNativePackageArchive(t, foreignBinding)
	if _, err := fixture.Storage.Upload(context.Background(), fixture.archiveObjectKey(), foreign.Archive, nativePackageArchiveContentType, "foreign.zip"); err != nil {
		t.Fatalf("replace native archive: %v", err)
	}
	manifest, err := json.Marshal(foreign.Manifest)
	if err != nil {
		t.Fatalf("marshal foreign manifest: %v", err)
	}
	index, err := json.Marshal(foreign.Manifest.Files)
	if err != nil {
		t.Fatalf("marshal foreign index: %v", err)
	}
	if _, err := testPool.Exec(context.Background(), `
		UPDATE project_design_system_package
		SET manifest = $1, artifact_index = $2, integrity_sha256 = $3
		WHERE design_system_id = $4 AND slot = 'draft'
	`, manifest, index, strings.TrimPrefix(foreign.Manifest.ContentDigest, "sha256:"), fixture.Completion.System.ID); err != nil {
		t.Fatalf("replace persisted native metadata: %v", err)
	}

	systemID := uuidToString(fixture.Completion.System.ID)
	preview := performProjectDesignSystemIDRequest(t, testHandler.GetProjectDesignSystemPackagePreview, http.MethodGet, "/api/project-design-systems/"+systemID+"/package-preview", systemID, nil)
	if preview.Code != http.StatusConflict || strings.Contains(preview.Body.String(), "UI Kit") {
		t.Fatalf("preview exposed foreign package: status = %d, body = %s", preview.Code, preview.Body.String())
	}

	response := performProjectDesignSystemIDRequest(t, testHandler.GetProjectDesignSystem, http.MethodGet, "/api/project-design-systems/"+systemID, systemID, nil)
	if response.Code != http.StatusOK || strings.Contains(response.Body.String(), "UI Kit") || !strings.Contains(response.Body.String(), "native_package_invalid") {
		t.Fatalf("response exposed foreign package: status = %d, body = %s", response.Code, response.Body.String())
	}

	accessToken, _ := issueOpenDesignArchivePreviewAccessToken(testWorkspaceID, systemID, foreign.Manifest.ContentDigest)
	file := httptest.NewRecorder()
	request := newRequest(http.MethodGet, "/api/project-design-system-previews/"+testWorkspaceID+"/"+systemID+"/files", nil)
	route := chi.NewRouteContext()
	route.URLParams.Add("workspaceId", testWorkspaceID)
	route.URLParams.Add("systemId", systemID)
	route.URLParams.Add("digest", strings.TrimPrefix(foreign.Manifest.ContentDigest, "sha256:"))
	route.URLParams.Add("accessToken", accessToken)
	route.URLParams.Add("*", foreign.Manifest.PreviewTargets[0].Path)
	request = request.WithContext(context.WithValue(request.Context(), chi.RouteCtxKey, route))
	testHandler.GetProjectDesignSystemPackagePreviewFile(file, request)
	if file.Code != http.StatusConflict || strings.Contains(file.Body.String(), "UI Kit") {
		t.Fatalf("file exposed foreign package: status = %d, body = %s", file.Code, file.Body.String())
	}
}

// Keep the task brief's named lifecycle test visible in this focused package
// suite while the shared completion fixture proves persisted archive metadata.
func TestProjectDesignSystemResponseRendersV2ContentWithoutLegacyValidate(t *testing.T) {
	fixture := newNativeV2CompletionFixture(t, service.ProjectDesignSystemGenerate)
	if response := fixture.completeTask(t, fixture.buildPackagePayload(t, nil)); response.Code != http.StatusOK {
		t.Fatalf("complete native package: status = %d, body = %s", response.Code, response.Body.String())
	}
	selected, err := testHandler.Queries.GetProjectDesignSystemPackageBySlot(context.Background(), db.GetProjectDesignSystemPackageBySlotParams{DesignSystemID: fixture.Completion.System.ID, Slot: "draft", WorkspaceID: parseUUID(testWorkspaceID)})
	if err != nil {
		t.Fatalf("load native draft: %v", err)
	}
	if _, err := testHandler.expectedNativeProjectDesignSystemPackageBinding(context.Background(), fixture.Completion.System, selected); err != nil {
		t.Fatalf("expected native binding: %v", err)
	}
	response, err := testHandler.projectDesignSystemResponse(context.Background(), fixture.Completion.System)
	if err != nil {
		t.Fatalf("projectDesignSystemResponse: %v", err)
	}
	if response.Content.PackageSchema != projectdesignsystem.PackageSchemaV2 || len(response.Content.PreviewTargets) == 0 || response.Content.PreviewHTML != "" {
		t.Fatalf("native response content = %+v, last_error = %s", response.Content, response.LastError)
	}
}

func TestAdjustProjectDesignSystemUsesImmutableV2BaseReference(t *testing.T) {
	fixture := newNativeV2CompletionFixture(t, service.ProjectDesignSystemGenerate)
	if response := fixture.completeTask(t, fixture.buildPackagePayload(t, nil)); response.Code != http.StatusOK {
		t.Fatalf("complete native package: status = %d, body = %s", response.Code, response.Body.String())
	}
	if _, err := testPool.Exec(context.Background(), `
		UPDATE project_design_system
		SET input_snapshot = $1::jsonb
		WHERE id = $2
	`, `{"agent_id":"`+fixture.Completion.AgentID+`","platform":"web","brief":"Native base adjustment.","references":[]}`, fixture.Completion.System.ID); err != nil {
		t.Fatalf("seed readable input snapshot: %v", err)
	}
	systemID := uuidToString(fixture.Completion.System.ID)
	response := performProjectDesignSystemIDRequest(t, testHandler.AdjustProjectDesignSystem, http.MethodPost, "/api/project-design-systems/"+systemID+"/adjust", systemID, map[string]any{
		"agent_id": fixture.Completion.AgentID, "instruction": "Tighten the primary action.", "scope": map[string]any{"kind": "all"},
	})
	if response.Code != http.StatusAccepted {
		t.Fatalf("AdjustProjectDesignSystem: status = %d, body = %s", response.Code, response.Body.String())
	}
	var adjusted ProjectDesignSystemResponse
	if err := json.NewDecoder(response.Body).Decode(&adjusted); err != nil || adjusted.ActiveTask == nil {
		t.Fatalf("decode adjustment response: err = %v, response = %+v", err, adjusted)
	}
	var taskContext service.ProjectDesignSystemTaskContext
	var rawContext []byte
	if err := testPool.QueryRow(context.Background(), `SELECT context FROM agent_task_queue WHERE id = $1`, adjusted.ActiveTask.ID).Scan(&rawContext); err != nil || json.Unmarshal(rawContext, &taskContext) != nil {
		t.Fatalf("load adjustment context: %v", err)
	}
	var base nativeBasePackageReference
	if err := json.Unmarshal(taskContext.BasePackage, &base); err != nil {
		t.Fatalf("decode native base reference: %v", err)
	}
	if base.Schema != projectdesignsystem.PackageSchemaV2 || base.Slot != "draft" || base.SourceTaskID != fixture.Completion.TaskID || base.IntegritySHA256 != strings.TrimPrefix(fixture.Collected.Manifest.ContentDigest, "sha256:") || taskContext.BasePackageSHA256 != fixture.Collected.Manifest.ContentDigest {
		t.Fatalf("immutable V2 base context = %+v, task = %+v", base, taskContext)
	}

	foreignBinding := fixture.Binding
	foreignBinding.TaskID = "00000000-0000-4000-8000-000000000088"
	foreign := collectNativePackageArchive(t, foreignBinding)
	if _, err := fixture.Storage.Upload(context.Background(), fixture.archiveObjectKey(), foreign.Archive, nativePackageArchiveContentType, "foreign-base.zip"); err != nil {
		t.Fatalf("replace native base archive: %v", err)
	}
	foreignManifest, err := json.Marshal(foreign.Manifest)
	if err != nil {
		t.Fatalf("marshal foreign base manifest: %v", err)
	}
	foreignIndex, err := json.Marshal(foreign.Manifest.Files)
	if err != nil {
		t.Fatalf("marshal foreign base index: %v", err)
	}
	if _, err := testPool.Exec(context.Background(), `
		UPDATE project_design_system_package
		SET manifest = $1, artifact_index = $2, integrity_sha256 = $3
		WHERE design_system_id = $4 AND slot = 'draft'
	`, foreignManifest, foreignIndex, strings.TrimPrefix(foreign.Manifest.ContentDigest, "sha256:"), fixture.Completion.System.ID); err != nil {
		t.Fatalf("replace persisted native base metadata: %v", err)
	}
	download := httptest.NewRecorder()
	downloadRequest := newDaemonTokenRequest(http.MethodGet, "/api/daemon/tasks/"+adjusted.ActiveTask.ID+"/project-design-system/base-package", nil, testWorkspaceID, "")
	downloadRequest = withURLParam(downloadRequest, "taskId", adjusted.ActiveTask.ID)
	testHandler.DownloadProjectDesignSystemBasePackage(download, downloadRequest)
	if download.Code != http.StatusConflict || download.Header().Get(nativePackageDigestHeader) != "" {
		t.Fatalf("base endpoint exposed foreign package: status = %d, headers = %#v, body = %s", download.Code, download.Header(), download.Body.String())
	}
}
