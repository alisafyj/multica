package handler

import (
	"archive/zip"
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/go-chi/chi/v5"
	"github.com/multica-ai/multica/server/internal/opendesign"
	db "github.com/multica-ai/multica/server/pkg/db/generated"
)

func TestDownloadOpenDesignRunEvidenceReturnsDeterministicArchive(t *testing.T) {
	fixture := prepareCompletedOpenDesignEvidenceRun(t, "Open Design evidence download")

	first := performOpenDesignEvidenceRequest(t, fixture.SystemID, fixture.RunID)
	if first.Code != http.StatusOK {
		t.Fatalf("DownloadOpenDesignRunEvidence: status = %d, body = %s", first.Code, first.Body.String())
	}
	second := performOpenDesignEvidenceRequest(t, fixture.SystemID, fixture.RunID)
	if second.Code != http.StatusOK {
		t.Fatalf("DownloadOpenDesignRunEvidence second pass: status = %d, body = %s", second.Code, second.Body.String())
	}
	if !bytes.Equal(first.Body.Bytes(), second.Body.Bytes()) {
		t.Fatal("evidence downloads are not deterministic")
	}
	digest := first.Header().Get(openDesignEvidenceDigestHeader)
	if err := opendesign.ValidateContentDigest(digest); err != nil {
		t.Fatalf("evidence digest header = %q: %v", digest, err)
	}
	if got := first.Header().Get("Content-Type"); got != opendesign.RunArchiveContentType {
		t.Fatalf("Content-Type = %q", got)
	}
	if got := first.Header().Get("Content-Disposition"); got != fmt.Sprintf(`attachment; filename="open-design-evidence-%s.zip"`, fixture.RunID) {
		t.Fatalf("Content-Disposition = %q", got)
	}
	if first.Header().Get("Cache-Control") != "no-store" || first.Header().Get("X-Content-Type-Options") != "nosniff" {
		t.Fatalf("security headers = %#v", first.Header())
	}

	reader, err := zip.NewReader(bytes.NewReader(first.Body.Bytes()), int64(first.Body.Len()))
	if err != nil {
		t.Fatalf("open evidence ZIP: %v", err)
	}
	files := make(map[string]*zip.File, len(reader.File))
	for _, file := range reader.File {
		files[file.Name] = file
	}
	for _, path := range []string{"manifest.json", "project/archive.zip", "run/events.json", "run/result-package.json", "run/audit.json", "run/preview.json"} {
		if files[path] == nil {
			t.Fatalf("evidence ZIP is missing %q", path)
		}
	}
	manifestReader, err := files["manifest.json"].Open()
	if err != nil {
		t.Fatalf("open evidence manifest: %v", err)
	}
	defer manifestReader.Close()
	var manifest opendesign.RunEvidenceManifest
	if err := json.NewDecoder(manifestReader).Decode(&manifest); err != nil {
		t.Fatalf("decode evidence manifest: %v", err)
	}
	if manifest.Run.SupervisorRunID != fixture.RunID || manifest.Run.Status != opendesign.RunStatusSucceeded || !manifest.Archive.Included {
		t.Fatalf("evidence manifest = %+v", manifest)
	}
}

func TestDownloadOpenDesignRunEvidenceRejectsNonTerminalRun(t *testing.T) {
	taskID, _, _ := prepareOpenDesignRunForAuditTest(t, "Open Design non-terminal evidence")
	fixture := loadOpenDesignEvidenceFixture(t, taskID)

	response := performOpenDesignEvidenceRequest(t, fixture.SystemID, fixture.RunID)
	assertProjectDesignSystemErrorCode(t, response, http.StatusConflict, "open_design_evidence_not_terminal")
}

func TestDownloadOpenDesignRunEvidenceRejectsForeignDesignSystem(t *testing.T) {
	fixture := prepareCompletedOpenDesignEvidenceRun(t, "Open Design foreign evidence")
	foreignWorkspaceID := createProjectDesignSystemWorkspace(t)
	foreignProjectID := createProjectDesignSystemProject(t, uuidToString(foreignWorkspaceID), "Foreign evidence project")
	foreignSystem := createProjectDesignSystemForTest(t, db.New(testPool), foreignWorkspaceID, foreignProjectID, "Foreign evidence system")

	response := performOpenDesignEvidenceRequest(t, uuidToString(foreignSystem.ID), fixture.RunID)
	assertProjectDesignSystemErrorCode(t, response, http.StatusNotFound, "project_design_system_not_found")
}

func TestDownloadOpenDesignRunEvidenceFailsWhenProjectArchiveIsMissing(t *testing.T) {
	fixture := prepareCompletedOpenDesignEvidenceRun(t, "Open Design missing archive evidence")
	store, ok := testHandler.Storage.(*mockStorage)
	if !ok {
		t.Fatalf("Open Design test storage = %T, want *mockStorage", testHandler.Storage)
	}
	store.Delete(context.Background(), fixture.ArchiveObjectKey)

	response := performOpenDesignEvidenceRequest(t, fixture.SystemID, fixture.RunID)
	assertProjectDesignSystemErrorCode(t, response, http.StatusBadGateway, "open_design_evidence_archive_unavailable")
}

type openDesignEvidenceFixture struct {
	RunID            string
	SystemID         string
	ArchiveObjectKey string
}

func prepareCompletedOpenDesignEvidenceRun(t *testing.T, name string) openDesignEvidenceFixture {
	t.Helper()
	taskID, _, contentDigest := prepareOpenDesignRunForAuditTest(t, name)
	audit := fmt.Sprintf(`{"schema":"%s","content_digest":%q,"audit":{"ok":true}}`, opendesign.PackageAuditReceiptSchema, contentDigest)
	preview := fmt.Sprintf(`{"schema":"%s","content_digest":%q,"verification":{"passed":true}}`, opendesign.PreviewVerificationReceiptSchema, contentDigest)
	if _, err := testPool.Exec(context.Background(), `
		UPDATE open_design_run
		SET status = 'succeeded',
			audit_report = $2::jsonb,
			preview_receipt = $3::jsonb,
			failure = '{}'::jsonb,
			finished_at = now(),
			updated_at = now()
		WHERE task_id = $1
	`, taskID, audit, preview); err != nil {
		t.Fatalf("complete Open Design evidence run: %v", err)
	}
	return loadOpenDesignEvidenceFixture(t, taskID)
}

func loadOpenDesignEvidenceFixture(t *testing.T, taskID string) openDesignEvidenceFixture {
	t.Helper()
	var fixture openDesignEvidenceFixture
	if err := testPool.QueryRow(context.Background(), `
		SELECT id::text, design_system_id::text, COALESCE(archive_object_key, '')
		FROM open_design_run
		WHERE task_id = $1
	`, taskID).Scan(&fixture.RunID, &fixture.SystemID, &fixture.ArchiveObjectKey); err != nil {
		t.Fatalf("load Open Design evidence fixture: %v", err)
	}
	return fixture
}

func performOpenDesignEvidenceRequest(t *testing.T, systemID, runID string) *httptest.ResponseRecorder {
	t.Helper()
	recorder := httptest.NewRecorder()
	request := newRequest(http.MethodGet, "/api/project-design-systems/"+systemID+"/open-design-runs/"+runID+"/evidence", nil)
	routeContext := chi.NewRouteContext()
	routeContext.URLParams.Add("id", systemID)
	routeContext.URLParams.Add("runId", runID)
	request = request.WithContext(context.WithValue(request.Context(), chi.RouteCtxKey, routeContext))
	testHandler.DownloadOpenDesignRunEvidence(recorder, request)
	return recorder
}
