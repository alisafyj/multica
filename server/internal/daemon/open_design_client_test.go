package daemon

import (
	"bytes"
	"context"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"reflect"
	"strings"
	"testing"

	"github.com/multica-ai/multica/server/internal/opendesign"
)

func TestClientReportsOpenDesignLifecycleCallbacks(t *testing.T) {
	type capturedRequest struct {
		Path        string
		Body        map[string]json.RawMessage
		RawBody     []byte
		ContentType string
		RunID       string
		Digest      string
	}
	requests := make([]capturedRequest, 0, 8)
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if strings.HasSuffix(r.URL.Path, "/open-design/archive") {
			body, err := io.ReadAll(r.Body)
			if err != nil {
				t.Errorf("read %s request: %v", r.URL.Path, err)
				w.WriteHeader(http.StatusBadRequest)
				return
			}
			requests = append(requests, capturedRequest{
				Path:        r.URL.Path,
				RawBody:     body,
				ContentType: r.Header.Get("Content-Type"),
				RunID:       r.Header.Get("X-Open-Design-Run-ID"),
				Digest:      r.Header.Get("X-Open-Design-Content-Digest"),
			})
			w.Header().Set("Content-Type", "application/json")
			_, _ = io.WriteString(w, `{"archive_object_key":"workspaces/workspace-1/design-systems/design-system-1/open-design-runs/task-1/archive.zip"}`)
			return
		}
		var body map[string]json.RawMessage
		if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
			t.Errorf("decode %s request: %v", r.URL.Path, err)
			w.WriteHeader(http.StatusBadRequest)
			return
		}
		requests = append(requests, capturedRequest{Path: r.URL.Path, Body: body})
		w.WriteHeader(http.StatusOK)
	}))
	t.Cleanup(server.Close)

	client := NewClient(server.URL)
	runID := "11111111-1111-4111-8111-111111111111"
	preflight := opendesign.PreflightReport{Schema: opendesign.PreflightSchema}
	event := opendesign.RunEvent{ID: 1, Event: "start", Data: json.RawMessage(`{"adapter_id":"opencode"}`)}
	resultPackage := json.RawMessage(`{"schema":"open-design.run-result-package.v1","run":{"id":"11111111-1111-4111-8111-111111111111"}}`)
	collectedResult := opendesign.CollectedRunResult{
		ResultPackage: resultPackage,
		ArtifactIndex: []opendesign.ArtifactIndexEntry{{
			Path: "index.html", Role: "entry", MIME: "text/html", Size: 13,
			SHA256: "aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa",
		}},
		ContentDigest: "sha256:bbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbb",
	}
	auditReceipt := opendesign.PackageAuditReceipt{
		Schema:        opendesign.PackageAuditReceiptSchema,
		Engine:        opendesign.PinnedEngineIdentity(),
		ContentDigest: collectedResult.ContentDigest,
		Audit: opendesign.PackageAudit{
			OK:             true,
			FilesInspected: 1,
			Errors:         []opendesign.PackageAuditIssue{},
			Warnings:       []opendesign.PackageAuditIssue{},
		},
	}
	previewReceipt := opendesign.PreviewVerificationReceipt{
		Schema:        opendesign.PreviewVerificationReceiptSchema,
		Engine:        opendesign.PinnedEngineIdentity(),
		ContentDigest: collectedResult.ContentDigest,
		Verification: opendesign.PreviewVerification{
			Browser: opendesign.PreviewBrowserIdentity{Name: "Chrome", Version: "150.0.0.0"},
			Policy:  opendesign.PinnedPreviewVerificationPolicy(),
			Passed:  true,
		},
	}
	archive := []byte("PK\x03\x04test-archive")
	failure := json.RawMessage(`{"code":"open_design_audit_failed","message":"audit rejected candidate"}`)

	if err := client.ReportOpenDesignPreflight(context.Background(), "task-1", preflight); err != nil {
		t.Fatalf("ReportOpenDesignPreflight: %v", err)
	}
	if err := client.StartOpenDesignRun(context.Background(), "task-1", runID); err != nil {
		t.Fatalf("StartOpenDesignRun: %v", err)
	}
	if err := client.ReportOpenDesignRunEvent(context.Background(), "task-1", runID, event); err != nil {
		t.Fatalf("ReportOpenDesignRunEvent: %v", err)
	}
	archiveObjectKey, err := client.UploadOpenDesignRunArchive(context.Background(), "task-1", runID, collectedResult.ContentDigest, archive)
	if err != nil {
		t.Fatalf("UploadOpenDesignRunArchive: %v", err)
	}
	collectedResult.ArchiveObjectKey = archiveObjectKey
	if err := client.ReportOpenDesignRunResult(context.Background(), "task-1", runID, collectedResult); err != nil {
		t.Fatalf("ReportOpenDesignRunResult: %v", err)
	}
	if err := client.ReportOpenDesignRunAudit(context.Background(), "task-1", runID, auditReceipt); err != nil {
		t.Fatalf("ReportOpenDesignRunAudit: %v", err)
	}
	if err := client.ReportOpenDesignRunPreview(context.Background(), "task-1", runID, previewReceipt); err != nil {
		t.Fatalf("ReportOpenDesignRunPreview: %v", err)
	}
	if err := client.FinalizeOpenDesignRun(context.Background(), "task-1", runID, opendesign.RunStatusAuditFailed, failure); err != nil {
		t.Fatalf("FinalizeOpenDesignRun: %v", err)
	}

	wantPaths := []string{
		"/api/daemon/tasks/task-1/open-design/preflight",
		"/api/daemon/tasks/task-1/open-design/start",
		"/api/daemon/tasks/task-1/open-design/events",
		"/api/daemon/tasks/task-1/open-design/archive",
		"/api/daemon/tasks/task-1/open-design/result",
		"/api/daemon/tasks/task-1/open-design/audit",
		"/api/daemon/tasks/task-1/open-design/preview",
		"/api/daemon/tasks/task-1/open-design/terminal",
	}
	gotPaths := make([]string, 0, len(requests))
	for _, request := range requests {
		gotPaths = append(gotPaths, request.Path)
	}
	if !reflect.DeepEqual(gotPaths, wantPaths) {
		t.Fatalf("callback paths = %#v, want %#v", gotPaths, wantPaths)
	}
	assertOpenDesignClientJSONField(t, requests[1].Body, "open_design_run_id", runID)
	assertOpenDesignClientJSONField(t, requests[2].Body, "open_design_run_id", runID)
	if !bytes.Equal(requests[3].RawBody, archive) || requests[3].ContentType != "application/zip" || requests[3].RunID != runID || requests[3].Digest != collectedResult.ContentDigest {
		t.Fatalf("archive request = %+v", requests[3])
	}
	assertOpenDesignClientJSONField(t, requests[7].Body, "status", string(opendesign.RunStatusAuditFailed))
	if _, ok := requests[4].Body["result_package"]; !ok {
		t.Fatal("result callback omitted result_package")
	}
	if _, ok := requests[4].Body["artifact_index"]; !ok {
		t.Fatal("result callback omitted artifact_index")
	}
	assertOpenDesignClientJSONField(t, requests[4].Body, "content_digest", collectedResult.ContentDigest)
	assertOpenDesignClientJSONField(t, requests[4].Body, "archive_object_key", archiveObjectKey)
	assertOpenDesignClientJSONField(t, requests[5].Body, "open_design_run_id", runID)
	if _, ok := requests[5].Body["audit_report"]; !ok {
		t.Fatal("audit callback omitted audit_report")
	}
	assertOpenDesignClientJSONField(t, requests[6].Body, "open_design_run_id", runID)
	if _, ok := requests[6].Body["preview_receipt"]; !ok {
		t.Fatal("Preview callback omitted preview_receipt")
	}
	if _, ok := requests[7].Body["failure"]; !ok {
		t.Fatal("terminal callback omitted failure")
	}
}

func TestClientDownloadsPinnedOpenDesignBaseArchive(t *testing.T) {
	t.Parallel()

	reference := opendesign.BasePackageReference{
		Schema:        opendesign.BasePackageReferenceSchema,
		Slot:          "saved",
		ContentDigest: "sha256:" + strings.Repeat("a", 64),
		SourceTaskID:  "11111111-1111-4111-8111-111111111111",
	}
	archive := []byte("PK\x03\x04verified-base-archive")
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodGet || r.URL.Path != "/api/daemon/tasks/task-1/open-design/base-archive" {
			t.Errorf("base archive request = %s %s", r.Method, r.URL.Path)
			w.WriteHeader(http.StatusNotFound)
			return
		}
		if r.Header.Get("Authorization") != "Bearer daemon-token" {
			t.Errorf("authorization header = %q", r.Header.Get("Authorization"))
		}
		w.Header().Set("Content-Type", opendesign.RunArchiveContentType)
		w.Header().Set(opendesign.RunArchiveContentDigestHeader, reference.ContentDigest)
		w.Header().Set(opendesign.BasePackageSlotHeader, reference.Slot)
		w.Header().Set(opendesign.BasePackageSourceTaskIDHeader, reference.SourceTaskID)
		_, _ = w.Write(archive)
	}))
	t.Cleanup(server.Close)

	client := NewClient(server.URL)
	client.SetToken("daemon-token")
	got, err := client.DownloadOpenDesignBaseArchive(context.Background(), "task-1", reference)
	if err != nil {
		t.Fatalf("DownloadOpenDesignBaseArchive: %v", err)
	}
	if !bytes.Equal(got, archive) {
		t.Fatalf("downloaded archive = %q, want %q", got, archive)
	}
}

func TestClientRejectsMismatchedOpenDesignBaseArchiveHeaders(t *testing.T) {
	t.Parallel()

	reference := opendesign.BasePackageReference{
		Schema:        opendesign.BasePackageReferenceSchema,
		Slot:          "saved",
		ContentDigest: "sha256:" + strings.Repeat("a", 64),
		SourceTaskID:  "11111111-1111-4111-8111-111111111111",
	}
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Content-Type", opendesign.RunArchiveContentType)
		w.Header().Set(opendesign.RunArchiveContentDigestHeader, "sha256:"+strings.Repeat("b", 64))
		w.Header().Set(opendesign.BasePackageSlotHeader, reference.Slot)
		w.Header().Set(opendesign.BasePackageSourceTaskIDHeader, reference.SourceTaskID)
		_, _ = w.Write([]byte("PK\x03\x04wrong-base-archive"))
	}))
	t.Cleanup(server.Close)

	client := NewClient(server.URL)
	if _, err := client.DownloadOpenDesignBaseArchive(context.Background(), "task-1", reference); err == nil || !strings.Contains(err.Error(), "digest") {
		t.Fatalf("mismatched base archive error = %v", err)
	}
}

func assertOpenDesignClientJSONField(t *testing.T, body map[string]json.RawMessage, field string, want string) {
	t.Helper()
	var got string
	if err := json.Unmarshal(body[field], &got); err != nil {
		t.Fatalf("decode %s: %v", field, err)
	}
	if got != want {
		t.Fatalf("%s = %q, want %q", field, got, want)
	}
}
