package opendesign

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"net/http/httptest"
	"path/filepath"
	"reflect"
	"strings"
	"testing"
)

type workerHTTPStatusCoder interface {
	HTTPStatusCode() int
}

func TestWorkerClientPreparesScratchAndRunsWithoutScenarioPlugin(t *testing.T) {
	t.Parallel()

	const (
		token = "worker-token"
		runID = "11111111-1111-4111-8111-111111111111"
	)
	var paths []string
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		paths = append(paths, r.Method+" "+r.URL.Path)
		if got := r.Header.Get("Authorization"); got != "Bearer "+token {
			http.Error(w, "unauthorized", http.StatusUnauthorized)
			return
		}
		w.Header().Set("Content-Type", "application/json")
		switch r.Method + " " + r.URL.Path {
		case "POST /api/import/folder":
			var body workerImportRequest
			if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
				t.Fatalf("decode import request: %v", err)
			}
			if !filepath.IsAbs(body.BaseDir) || body.OrchestratorWorkspace.Kind != "scratch" || body.OrchestratorWorkspace.Writeback != "external" {
				t.Fatalf("import request = %+v", body)
			}
			fmt.Fprint(w, `{"project":{"id":"project-1"},"conversationId":"conversation-1"}`)
		case "PATCH /api/projects/project-1":
			var body struct {
				Metadata map[string]any `json:"metadata"`
			}
			if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
				t.Fatalf("decode project patch: %v", err)
			}
			if len(body.Metadata) != 0 {
				t.Fatalf("project metadata patch = %#v, want empty metadata to disable scenario fallback", body.Metadata)
			}
			fmt.Fprint(w, `{"project":{"id":"project-1"}}`)
		case "POST /api/runs":
			var body workerRunRequest
			if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
				t.Fatalf("decode run request: %v", err)
			}
			if body.ProjectID != "project-1" || body.ConversationID != "conversation-1" || body.AgentID != "opencode" || body.Message != "Build the design system" {
				t.Fatalf("run request = %+v", body)
			}
			fmt.Fprintf(w, `{"runId":%q}`, runID)
		case "GET /api/runs/" + runID + "/events":
			w.Header().Set("Content-Type", "text/event-stream")
			fmt.Fprint(w, "id: 1\nevent: start\ndata: {\"status\":\"running\"}\n\nid: 2\nevent: end\ndata: {\"status\":\"succeeded\"}\n\n")
		case "GET /api/runs/" + runID:
			fmt.Fprintf(w, `{"id":%q,"status":"succeeded","exitCode":0}`, runID)
		case "GET /api/runs/" + runID + "/result-package":
			fmt.Fprintf(w, `{"schema":%q,"run":{"id":%q,"status":"succeeded"}}`, RunResultPackageSchema, runID)
		case "GET /api/projects/project-1/export/manifest":
			_, _ = w.Write(testProjectExportManifest(map[string]testManifestFile{
				"index.html": {MIME: "text/html", Role: "entry", Body: "<main></main>"},
			}))
		case "GET /api/projects/project-1/archive":
			w.Header().Set("Content-Type", "application/zip")
			_, _ = w.Write(testProjectArchive(t, []testArchiveFile{{Path: "index.html", Body: "<main></main>"}}))
		case "GET /api/projects/project-1/design-system-package-audit":
			fmt.Fprint(w, `{"audit":{"ok":true,"projectPath":"/private/tmp/open-design/project-1","filesInspected":1,"errors":[],"warnings":[]}}`)
		case "GET /api/projects/project-1/preview-url":
			if got := r.URL.Query().Get("file"); got != "preview/colors.html" {
				t.Fatalf("preview file query = %q", got)
			}
			fmt.Fprint(w, `{"url":"/api/projects/project-1/preview/preview_scope_123/preview/colors.html","file":"preview/colors.html","csp":"default-src 'self'; connect-src 'none'; sandbox allow-scripts allow-forms","iframeSandbox":"allow-scripts allow-forms","opaqueOrigin":true}`)
		default:
			http.NotFound(w, r)
		}
	}))
	defer server.Close()

	client, err := NewWorkerClient(server.URL, token, server.Client())
	if err != nil {
		t.Fatalf("NewWorkerClient: %v", err)
	}
	workspace, err := client.PrepareWorkspace(context.Background(), WorkerWorkspaceRequest{
		ScratchRoot: t.TempDir(),
		Name:        "CRM design system",
		Provenance: WorkerWorkspaceProvenance{
			SourceLabel:  "multica-project:crm",
			SourceRef:    "main@abc123",
			BaseRevision: "abc123",
		},
	})
	if err != nil {
		t.Fatalf("PrepareWorkspace: %v", err)
	}
	startedRunID, err := client.StartRun(context.Background(), WorkerStartRunRequest{
		Workspace: workspace,
		Agent:     AgentIdentity{AdapterID: "opencode"},
		Prompt:    "Build the design system",
	})
	if err != nil {
		t.Fatalf("StartRun: %v", err)
	}
	var events []RunEvent
	if err := client.StreamRunEvents(context.Background(), startedRunID, 0, func(event RunEvent) error {
		events = append(events, event)
		return nil
	}); err != nil {
		t.Fatalf("StreamRunEvents: %v", err)
	}
	status, err := client.GetRun(context.Background(), startedRunID)
	if err != nil {
		t.Fatalf("GetRun: %v", err)
	}
	resultPackage, err := client.GetResultPackage(context.Background(), startedRunID)
	if err != nil {
		t.Fatalf("GetResultPackage: %v", err)
	}
	manifest, err := client.GetProjectExportManifest(context.Background(), workspace.ProjectID)
	if err != nil {
		t.Fatalf("GetProjectExportManifest: %v", err)
	}
	archive, err := client.GetProjectArchive(context.Background(), workspace.ProjectID)
	if err != nil {
		t.Fatalf("GetProjectArchive: %v", err)
	}
	audit, err := client.GetProjectPackageAudit(context.Background(), workspace.ProjectID)
	if err != nil {
		t.Fatalf("GetProjectPackageAudit: %v", err)
	}
	previewURL, err := client.GetProjectPreviewURL(context.Background(), workspace.ProjectID, PreviewTarget{
		Kind: PreviewTargetKindPreview,
		ID:   "colors",
		Path: "preview/colors.html",
	})
	if err != nil {
		t.Fatalf("GetProjectPreviewURL: %v", err)
	}

	if status.Status != "succeeded" || len(events) != 2 || events[0].ID != 1 || events[1].ID != 2 {
		t.Fatalf("status = %+v, events = %+v", status, events)
	}
	if !json.Valid(resultPackage) {
		t.Fatalf("result package is invalid JSON: %s", resultPackage)
	}
	if !json.Valid(manifest) || len(archive) == 0 {
		t.Fatalf("project export evidence = manifest:%s archive_bytes:%d", manifest, len(archive))
	}
	if !audit.OK || audit.FilesInspected != 1 || len(audit.Errors) != 0 || len(audit.Warnings) != 0 {
		t.Fatalf("project package audit = %+v", audit)
	}
	if previewURL.Target.Path != "preview/colors.html" || previewURL.URL != server.URL+"/api/projects/project-1/preview/preview_scope_123/preview/colors.html" {
		t.Fatalf("project Preview URL = %+v", previewURL)
	}
	wantPaths := []string{
		"POST /api/import/folder",
		"PATCH /api/projects/project-1",
		"POST /api/runs",
		"GET /api/runs/" + runID + "/events",
		"GET /api/runs/" + runID,
		"GET /api/runs/" + runID + "/result-package",
		"GET /api/projects/project-1/export/manifest",
		"GET /api/projects/project-1/archive",
		"GET /api/projects/project-1/design-system-package-audit",
		"GET /api/projects/project-1/preview-url",
	}
	if !reflect.DeepEqual(paths, wantPaths) {
		t.Fatalf("request paths = %#v, want %#v", paths, wantPaths)
	}
}

func TestWorkerClientPreservesStructuredHTTPFailure(t *testing.T) {
	t.Parallel()

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusNotFound)
		fmt.Fprint(w, `{"error":{"code":"NOT_FOUND","message":"run not found"}}`)
	}))
	defer server.Close()

	client, err := NewWorkerClient(server.URL, "", server.Client())
	if err != nil {
		t.Fatalf("NewWorkerClient: %v", err)
	}
	_, err = client.GetRun(context.Background(), "missing-run")
	var statusErr workerHTTPStatusCoder
	if !errors.As(err, &statusErr) {
		t.Fatalf("GetRun error = %v, want an HTTP status error", err)
	}
	if statusErr.HTTPStatusCode() != http.StatusNotFound || !strings.Contains(err.Error(), "NOT_FOUND") || !strings.Contains(err.Error(), "run not found") {
		t.Fatalf("GetRun error = %v", err)
	}
}

func TestWorkerClientRejectsUnscopedPreviewURLs(t *testing.T) {
	t.Parallel()

	target := PreviewTarget{Kind: PreviewTargetKindPreview, ID: "colors", Path: "preview/colors.html"}
	tests := []struct {
		name string
		body string
	}{
		{
			name: "cross origin",
			body: `{"url":"https://attacker.example/preview/colors.html","file":"preview/colors.html","csp":"connect-src 'none'; sandbox allow-scripts allow-forms","iframeSandbox":"allow-scripts allow-forms","opaqueOrigin":true}`,
		},
		{
			name: "unscoped path",
			body: `{"url":"/api/projects/project-1/raw/preview/colors.html","file":"preview/colors.html","csp":"connect-src 'none'; sandbox allow-scripts allow-forms","iframeSandbox":"allow-scripts allow-forms","opaqueOrigin":true}`,
		},
		{
			name: "mismatched file",
			body: `{"url":"/api/projects/project-1/preview/preview_scope_123/preview/colors.html","file":"preview/other.html","csp":"connect-src 'none'; sandbox allow-scripts allow-forms","iframeSandbox":"allow-scripts allow-forms","opaqueOrigin":true}`,
		},
		{
			name: "same origin escape",
			body: `{"url":"/api/projects/project-1/preview/preview_scope_123/preview/colors.html","file":"preview/colors.html","csp":"connect-src 'none'; sandbox allow-scripts allow-forms allow-same-origin","iframeSandbox":"allow-scripts allow-forms allow-same-origin","opaqueOrigin":true}`,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				w.Header().Set("Content-Type", "application/json")
				fmt.Fprint(w, tt.body)
			}))
			defer server.Close()

			client, err := NewWorkerClient(server.URL, "", server.Client())
			if err != nil {
				t.Fatalf("NewWorkerClient: %v", err)
			}
			if _, err := client.GetProjectPreviewURL(context.Background(), "project-1", target); err == nil {
				t.Fatal("GetProjectPreviewURL accepted an unsafe Preview URL")
			}
		})
	}
}

func TestWorkerClientPreservesRejectedPackageAuditFromHTTP200(t *testing.T) {
	t.Parallel()

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodGet || r.URL.Path != "/api/projects/project-1/design-system-package-audit" {
			http.NotFound(w, r)
			return
		}
		w.Header().Set("Content-Type", "application/json")
		fmt.Fprint(w, `{"audit":{"ok":false,"projectPath":"/private/tmp/open-design/project-1","filesInspected":796,"errors":[{"severity":"error","code":"insufficient_preview_cards","message":"Expected focused preview cards","path":"preview/"}],"warnings":[]}}`)
	}))
	defer server.Close()

	client, err := NewWorkerClient(server.URL, "", server.Client())
	if err != nil {
		t.Fatalf("NewWorkerClient: %v", err)
	}
	audit, err := client.GetProjectPackageAudit(context.Background(), "project-1")
	if err != nil {
		t.Fatalf("GetProjectPackageAudit: %v", err)
	}
	if audit.OK || audit.FilesInspected != 796 || len(audit.Errors) != 1 || audit.Errors[0].Code != "insufficient_preview_cards" || audit.Errors[0].Path != "preview/" {
		t.Fatalf("rejected package audit = %+v", audit)
	}
}

func TestWorkerClientAppliesFailOnWarningsToPackageAudit(t *testing.T) {
	t.Parallel()

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodGet || r.URL.Path != "/api/projects/project-1/design-system-package-audit" {
			http.NotFound(w, r)
			return
		}
		w.Header().Set("Content-Type", "application/json")
		fmt.Fprint(w, `{"audit":{"ok":true,"projectPath":"/private/tmp/open-design/project-1","filesInspected":39,"errors":[],"warnings":[{"severity":"warning","code":"readme_missing_product_overview","message":"README needs a product overview","path":"README.md"}]}}`)
	}))
	defer server.Close()

	client, err := NewWorkerClient(server.URL, "", server.Client())
	if err != nil {
		t.Fatalf("NewWorkerClient: %v", err)
	}
	audit, err := client.GetProjectPackageAudit(context.Background(), "project-1")
	if err != nil {
		t.Fatalf("GetProjectPackageAudit: %v", err)
	}
	if audit.OK || len(audit.Errors) != 0 || len(audit.Warnings) != 1 || audit.Warnings[0].Code != "readme_missing_product_overview" {
		t.Fatalf("strict package audit = %+v", audit)
	}
}
