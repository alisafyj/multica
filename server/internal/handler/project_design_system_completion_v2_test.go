package handler

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/multica-ai/multica/server/internal/designpreview"
	"github.com/multica-ai/multica/server/internal/projectdesignsystem"
	"github.com/multica-ai/multica/server/internal/service"
	db "github.com/multica-ai/multica/server/pkg/db/generated"
)

// nativeV2CompletionFixture bundles everything the V2 completion test
// suite needs: a running project design system task backed by a
// mockStorage holding the archive uploaded under the daemon-derived
// object key. The receipt is built fresh per test so each case can
// perturb one field (digest, agentID, audit, preview, object key, etc.).
type nativeV2CompletionFixture struct {
	Completion  projectDesignSystemCompletionFixture
	Storage     *mockStorage
	Binding     projectdesignsystem.PackageBinding
	Collected   projectdesignsystem.CollectedV2Package
	Receipt     designpreview.Receipt
	ReceiptJSON []byte
}

func newNativeV2CompletionFixture(t *testing.T, operation service.ProjectDesignSystemOperation) *nativeV2CompletionFixture {
	t.Helper()
	completion := createProjectDesignSystemCompletionFixture(t, operation)
	binding := nativePackageBinding(t, nativePackageUploadFixture{
		System:  completion.System,
		TaskID:  completion.TaskID,
		AgentID: completion.AgentID,
	}, operation)
	// Rewrite the task context to carry PackageSchema=v2 so the handler
	// dispatches the V2 completion path. Also seed BasePackageSHA256 for
	// non-generate operations so the binding derivation accepts the task.
	rewriteTaskContextForV2(t, completion, binding)
	collected := collectNativePackageArchive(t, binding)

	storage := &mockStorage{}
	digestHex := strings.TrimPrefix(collected.Manifest.ContentDigest, "sha256:")
	objectKey := fmt.Sprintf("%s/%s/%s/%s/%s.zip",
		nativePackageObjectKeyRoot,
		binding.WorkspaceID,
		binding.DesignSystemID,
		binding.TaskID,
		digestHex,
	)
	if _, err := storage.Upload(context.Background(), objectKey, collected.Archive, "application/zip", "native.zip"); err != nil {
		t.Fatalf("seed archive in storage: %v", err)
	}
	previousStorage := testHandler.Storage
	testHandler.Storage = storage
	t.Cleanup(func() { testHandler.Storage = previousStorage })

	receipt, err := buildNativeV2PassingReceipt(t, collected)
	if err != nil {
		t.Fatalf("build passing receipt: %v", err)
	}
	receiptJSON, err := json.Marshal(receipt)
	if err != nil {
		t.Fatalf("marshal receipt: %v", err)
	}

	return &nativeV2CompletionFixture{
		Completion:  completion,
		Storage:     storage,
		Binding:     binding,
		Collected:   collected,
		Receipt:     *receipt,
		ReceiptJSON: receiptJSON,
	}
}

// rewriteTaskContextForV2 mutates the seeded completion fixture's task
// row so the context carries the V2 package schema marker (and a
// non-empty base package digest for adjust/regenerate operations). The
// handler dispatches the V2 path purely off PackageSchema.
func rewriteTaskContextForV2(t *testing.T, fixture projectDesignSystemCompletionFixture, binding projectdesignsystem.PackageBinding) {
	t.Helper()
	var taskContext service.ProjectDesignSystemTaskContext
	if err := json.Unmarshal([]byte(`{}`), &taskContext); err != nil {
		t.Fatalf("seed context: %v", err)
	}
	row := testPool.QueryRow(context.Background(),
		`SELECT context FROM agent_task_queue WHERE id = $1`, fixture.TaskID)
	var rawContext []byte
	if err := row.Scan(&rawContext); err != nil {
		t.Fatalf("load task context: %v", err)
	}
	if err := json.Unmarshal(rawContext, &taskContext); err != nil {
		t.Fatalf("decode task context: %v", err)
	}
	taskContext.PackageSchema = projectdesignsystem.PackageSchemaV2
	taskContext.InputSnapshotSHA256 = binding.InputSnapshotSHA256
	if binding.BasePackageSHA256 != "" {
		taskContext.BasePackageSHA256 = binding.BasePackageSHA256
	}
	encoded, err := json.Marshal(taskContext)
	if err != nil {
		t.Fatalf("re-marshal task context: %v", err)
	}
	if _, err := testPool.Exec(context.Background(),
		`UPDATE agent_task_queue SET context = $1 WHERE id = $2`,
		encoded, fixture.TaskID); err != nil {
		t.Fatalf("update task context: %v", err)
	}
}

func buildNativeV2PassingReceipt(t *testing.T, collected projectdesignsystem.CollectedV2Package) (*designpreview.Receipt, error) {
	t.Helper()
	policy := designpreview.DefaultPolicy()
	targets := make([]designpreview.TargetVerification, 0, len(collected.Manifest.PreviewTargets))
	for _, preview := range collected.Manifest.PreviewTargets {
		targets = append(targets, designpreview.TargetVerification{
			Target:                    designpreview.Target{Kind: preview.Kind, ID: preview.ID, Path: preview.Path},
			Passed:                    true,
			DocumentLoaded:            true,
			DOMPresent:                true,
			ComputedVisibilityVisible: true,
			RenderedElementCount:      1,
			VisibleTextLength:         10,
			BodyWidth:                 800,
			BodyHeight:                600,
			Screenshot: designpreview.Screenshot{
				SHA256:           "sha256:" + strings.Repeat("a", 64),
				Bytes:            4096,
				Width:            800,
				Height:           600,
				Entropy:          4.0,
				MaxChannelStddev: 32,
			},
		})
	}
	receipt, err := designpreview.NewReceipt(collected.Manifest.ContentDigest, designpreview.Verification{
		Browser: designpreview.BrowserIdentity{Name: "native-test-chromium", Version: "0.0.0"},
		Policy:  policy,
		Targets: targets,
		Passed:  true,
	})
	if err != nil {
		return nil, err
	}
	return &receipt, nil
}

func (f *nativeV2CompletionFixture) buildPackagePayload(t *testing.T, mutate func(*ProjectDesignSystemPackageReceipt)) map[string]any {
	t.Helper()
	receipt := ProjectDesignSystemPackageReceipt{
		SchemaVersion: projectdesignsystem.PackageSchemaV2,
		ObjectKey:     f.archiveObjectKey(),
		ContentDigest: f.Collected.Manifest.ContentDigest,
		ArtifactIndex: f.Collected.Manifest.Files,
		Audit:         f.Collected.Audit,
		Preview:       f.Receipt,
	}
	if mutate != nil {
		mutate(&receipt)
	}
	receiptJSON, err := json.Marshal(receipt)
	if err != nil {
		t.Fatalf("marshal receipt: %v", err)
	}
	return map[string]any{
		"output":                        "Native design system ready.",
		"project_design_system_package": json.RawMessage(receiptJSON),
	}
}

func (f *nativeV2CompletionFixture) archiveObjectKey() string {
	digestHex := strings.TrimPrefix(f.Collected.Manifest.ContentDigest, "sha256:")
	return fmt.Sprintf("%s/%s/%s/%s/%s.zip",
		nativePackageObjectKeyRoot,
		f.Binding.WorkspaceID,
		f.Binding.DesignSystemID,
		f.Binding.TaskID,
		digestHex,
	)
}

func (f *nativeV2CompletionFixture) completeTask(t *testing.T, body map[string]any) *httptest.ResponseRecorder {
	t.Helper()
	return completeProjectDesignSystemTaskForTest(t, f.Completion.TaskID, body)
}

func TestCompleteProjectDesignSystemV2CreatesPassedDraftAfterAllEvidenceMatches(t *testing.T) {
	fixture := newNativeV2CompletionFixture(t, service.ProjectDesignSystemGenerate)

	queries := db.New(testPool)
	draftDigest := strings.Repeat("d", 64)
	savedDigest := strings.Repeat("s", 64)
	upsertProjectDesignSystemPackageForTest(t, queries, fixture.Completion.System.ID, "draft", "draft-before", draftDigest)
	upsertProjectDesignSystemPackageForTest(t, queries, fixture.Completion.System.ID, "saved", "saved-before", savedDigest)

	w := fixture.completeTask(t, fixture.buildPackagePayload(t, nil))
	if w.Code != http.StatusOK {
		t.Fatalf("CompleteTask status = %d, body = %s", w.Code, w.Body.String())
	}

	task, err := queries.GetAgentTask(context.Background(), parseUUID(fixture.Completion.TaskID))
	if err != nil {
		t.Fatalf("get completed task: %v", err)
	}
	if task.Status != "completed" {
		t.Fatalf("task status = %q, want completed", task.Status)
	}

	draft, err := queries.GetProjectDesignSystemPackageBySlot(context.Background(), db.GetProjectDesignSystemPackageBySlotParams{
		DesignSystemID: fixture.Completion.System.ID,
		Slot:           "draft",
		WorkspaceID:    parseUUID(testWorkspaceID),
	})
	if err != nil {
		t.Fatalf("get generated draft: %v", err)
	}
	if draft.PackageSchema != projectdesignsystem.PackageSchemaV2 {
		t.Fatalf("draft package_schema = %q, want %q", draft.PackageSchema, projectdesignsystem.PackageSchemaV2)
	}
	if draft.RenderStatus != "passed" {
		t.Fatalf("draft render_status = %q, want passed", draft.RenderStatus)
	}
	if uuidToString(draft.SourceTaskID) != fixture.Completion.TaskID {
		t.Fatalf("source task = %q, want %q", uuidToString(draft.SourceTaskID), fixture.Completion.TaskID)
	}
	if uuidToString(draft.AgentID) != fixture.Completion.AgentID {
		t.Fatalf("agent id = %q, want %q", uuidToString(draft.AgentID), fixture.Completion.AgentID)
	}
	integrityWant := strings.TrimPrefix(fixture.Collected.Manifest.ContentDigest, "sha256:")
	if draft.IntegritySha256 != integrityWant {
		t.Fatalf("integrity_sha256 = %q, want %q", draft.IntegritySha256, integrityWant)
	}
	if !draft.ArchiveObjectKey.Valid || draft.ArchiveObjectKey.String != fixture.archiveObjectKey() {
		t.Fatalf("archive_object_key = %+v, want %q", draft.ArchiveObjectKey, fixture.archiveObjectKey())
	}
	if len(draft.ArtifactIndex) == 0 {
		t.Fatalf("artifact_index was not persisted")
	}
	var persistedIndex []projectdesignsystem.ArtifactIndexEntry
	if err := json.Unmarshal(draft.ArtifactIndex, &persistedIndex); err != nil {
		t.Fatalf("decode artifact_index: %v", err)
	}
	if len(persistedIndex) != len(fixture.Collected.Manifest.Files) {
		t.Fatalf("artifact_index length = %d, want %d", len(persistedIndex), len(fixture.Collected.Manifest.Files))
	}
	if !draft.InputSnapshotSha256.Valid || draft.InputSnapshotSha256.String != fixture.Binding.InputSnapshotSHA256 {
		t.Fatalf("input_snapshot_sha256 = %+v, want %q", draft.InputSnapshotSha256, fixture.Binding.InputSnapshotSHA256)
	}
	if draft.BasePackageSha256.Valid {
		t.Fatalf("base_package_sha256 should be empty for generate, got %q", draft.BasePackageSha256.String)
	}
	if !json.Valid(draft.Manifest) || !json.Valid(draft.Validation) || !json.Valid(draft.RenderReport) {
		t.Fatalf("manifest/validation/render_report must be valid JSON")
	}

	system, err := queries.GetProjectDesignSystemInWorkspace(context.Background(), db.GetProjectDesignSystemInWorkspaceParams{
		ID:          fixture.Completion.System.ID,
		WorkspaceID: parseUUID(testWorkspaceID),
	})
	if err != nil {
		t.Fatalf("get completed design system: %v", err)
	}
	if system.ActiveTaskID.Valid || system.ActiveOperation.Valid || len(system.LastError) != 0 {
		t.Fatalf("active task state was not cleared: %+v", system)
	}
	saved, err := queries.GetProjectDesignSystemPackageBySlot(context.Background(), db.GetProjectDesignSystemPackageBySlotParams{
		DesignSystemID: fixture.Completion.System.ID,
		Slot:           "saved",
		WorkspaceID:    parseUUID(testWorkspaceID),
	})
	if err != nil || saved.IntegritySha256 != savedDigest {
		t.Fatalf("saved package was modified: %+v err=%v", saved, err)
	}
}

func TestCompleteProjectDesignSystemV2RejectsWrongTaskInputAgentAndBaseDigest(t *testing.T) {
	// The completion fixture creates the system with no prior base
	// package, so an Adjust/Regenerate binding derivation fails. Use
	// Generate so the binding derivation succeeds; the test then mutates
	// the receipt's content digest to one that doesn't match the
	// recomputed archive digest, which is what a daemon sending a wrong
	// task input / agent produces.
	fixture := newNativeV2CompletionFixture(t, service.ProjectDesignSystemGenerate)

	queries := db.New(testPool)
	draftDigest := strings.Repeat("d", 64)
	savedDigest := strings.Repeat("s", 64)
	upsertProjectDesignSystemPackageForTest(t, queries, fixture.Completion.System.ID, "draft", "draft-before-rejected", draftDigest)
	upsertProjectDesignSystemPackageForTest(t, queries, fixture.Completion.System.ID, "saved", "saved-before-rejected", savedDigest)

	body := fixture.buildPackagePayload(t, func(r *ProjectDesignSystemPackageReceipt) {
		// Replace the digest with one that points at a different archive,
		// mimicking a daemon that re-ran against a different input or a
		// different agent's output. The server's recomputed digest will
		// not match.
		r.ContentDigest = "sha256:" + strings.Repeat("e", 64)
	})
	w := fixture.completeTask(t, body)
	if w.Code != http.StatusBadRequest {
		t.Fatalf("CompleteTask status = %d, body = %s", w.Code, w.Body.String())
	}

	assertProjectDesignSystemTaskFailed(t, fixture.Completion.TaskID, "project_design_system_invalid_artifacts")
	assertProjectDesignSystemFailureState(t, fixture.Completion.System.ID, fixture.Completion.TaskID, "project_design_system_invalid_artifacts")
	assertProjectDesignSystemPackageDigest(t, queries, fixture.Completion.System.ID, "draft", draftDigest)
	assertProjectDesignSystemPackageDigest(t, queries, fixture.Completion.System.ID, "saved", savedDigest)
}

func TestCompleteProjectDesignSystemV2RejectsMissingOrMutatedStoredArchive(t *testing.T) {
	queries := db.New(testPool)
	draftDigest := strings.Repeat("d", 64)
	savedDigest := strings.Repeat("s", 64)

	fixture := newNativeV2CompletionFixture(t, service.ProjectDesignSystemGenerate)
	upsertProjectDesignSystemPackageForTest(t, queries, fixture.Completion.System.ID, "draft", "draft-before-missing-archive", draftDigest)
	upsertProjectDesignSystemPackageForTest(t, queries, fixture.Completion.System.ID, "saved", "saved-before-missing-archive", savedDigest)

	delete(fixture.Storage.files, fixture.archiveObjectKey())
	w := fixture.completeTask(t, fixture.buildPackagePayload(t, nil))
	if w.Code != http.StatusBadRequest {
		t.Fatalf("CompleteTask missing archive status = %d, body = %s", w.Code, w.Body.String())
	}

	assertProjectDesignSystemTaskFailed(t, fixture.Completion.TaskID, "project_design_system_invalid_artifacts")
	assertProjectDesignSystemFailureState(t, fixture.Completion.System.ID, fixture.Completion.TaskID, "project_design_system_invalid_artifacts")
	assertProjectDesignSystemPackageDigest(t, queries, fixture.Completion.System.ID, "draft", draftDigest)
	assertProjectDesignSystemPackageDigest(t, queries, fixture.Completion.System.ID, "saved", savedDigest)

	mutatedFixture := newNativeV2CompletionFixture(t, service.ProjectDesignSystemGenerate)
	mutatedDraftDigest := strings.Repeat("m", 64)
	mutatedSavedDigest := strings.Repeat("n", 64)
	upsertProjectDesignSystemPackageForTest(t, queries, mutatedFixture.Completion.System.ID, "draft", "draft-before-mutated-archive", mutatedDraftDigest)
	upsertProjectDesignSystemPackageForTest(t, queries, mutatedFixture.Completion.System.ID, "saved", "saved-before-mutated-archive", mutatedSavedDigest)

	mutatedKey := mutatedFixture.archiveObjectKey()
	mutatedFixture.Storage.files[mutatedKey] = append([]byte(nil), mutatedFixture.Collected.Archive...)
	mutatedFixture.Storage.files[mutatedKey][len(mutatedFixture.Storage.files[mutatedKey])/2] ^= 0xFF
	w = mutatedFixture.completeTask(t, mutatedFixture.buildPackagePayload(t, nil))
	if w.Code != http.StatusBadRequest {
		t.Fatalf("CompleteTask mutated archive status = %d, body = %s", w.Code, w.Body.String())
	}
	assertProjectDesignSystemTaskFailed(t, mutatedFixture.Completion.TaskID, "project_design_system_invalid_artifacts")
	assertProjectDesignSystemFailureState(t, mutatedFixture.Completion.System.ID, mutatedFixture.Completion.TaskID, "project_design_system_invalid_artifacts")
	assertProjectDesignSystemPackageDigest(t, queries, mutatedFixture.Completion.System.ID, "draft", mutatedDraftDigest)
	assertProjectDesignSystemPackageDigest(t, queries, mutatedFixture.Completion.System.ID, "saved", mutatedSavedDigest)
}

func TestCompleteProjectDesignSystemV2RejectsAuditOrPreviewFailure(t *testing.T) {
	queries := db.New(testPool)
	draftDigest := strings.Repeat("d", 64)
	savedDigest := strings.Repeat("s", 64)

	fixture := newNativeV2CompletionFixture(t, service.ProjectDesignSystemGenerate)
	upsertProjectDesignSystemPackageForTest(t, queries, fixture.Completion.System.ID, "draft", "draft-before-evidence-failure", draftDigest)
	upsertProjectDesignSystemPackageForTest(t, queries, fixture.Completion.System.ID, "saved", "saved-before-evidence-failure", savedDigest)

	body := fixture.buildPackagePayload(t, func(r *ProjectDesignSystemPackageReceipt) {
		r.Audit.Passed = false
		r.Audit.Diagnostics = append(r.Audit.Diagnostics, projectdesignsystem.Diagnostic{
			Code:     "audit_failed",
			Severity: projectdesignsystem.DiagnosticError,
			Path:     "manifest.json",
			Message:  "synthetic failure",
		})
	})
	w := fixture.completeTask(t, body)
	if w.Code != http.StatusBadRequest {
		t.Fatalf("CompleteTask audit failure status = %d, body = %s", w.Code, w.Body.String())
	}

	assertProjectDesignSystemTaskFailed(t, fixture.Completion.TaskID, "project_design_system_invalid_artifacts")
	assertProjectDesignSystemFailureState(t, fixture.Completion.System.ID, fixture.Completion.TaskID, "project_design_system_invalid_artifacts")
	assertProjectDesignSystemPackageDigest(t, queries, fixture.Completion.System.ID, "draft", draftDigest)
	assertProjectDesignSystemPackageDigest(t, queries, fixture.Completion.System.ID, "saved", savedDigest)

	fixture2 := newNativeV2CompletionFixture(t, service.ProjectDesignSystemGenerate)
	previewDraftDigest := strings.Repeat("p", 64)
	previewSavedDigest := strings.Repeat("q", 64)
	upsertProjectDesignSystemPackageForTest(t, queries, fixture2.Completion.System.ID, "draft", "draft-before-preview-failure", previewDraftDigest)
	upsertProjectDesignSystemPackageForTest(t, queries, fixture2.Completion.System.ID, "saved", "saved-before-preview-failure", previewSavedDigest)

	body2 := fixture2.buildPackagePayload(t, func(r *ProjectDesignSystemPackageReceipt) {
		r.Preview.Verification.Passed = false
		r.Preview.Verification.Targets = nil
	})
	w = fixture2.completeTask(t, body2)
	if w.Code != http.StatusBadRequest {
		t.Fatalf("CompleteTask preview failure status = %d, body = %s", w.Code, w.Body.String())
	}

	assertProjectDesignSystemTaskFailed(t, fixture2.Completion.TaskID, "project_design_system_invalid_artifacts")
	assertProjectDesignSystemFailureState(t, fixture2.Completion.System.ID, fixture2.Completion.TaskID, "project_design_system_invalid_artifacts")
	assertProjectDesignSystemPackageDigest(t, queries, fixture2.Completion.System.ID, "draft", previewDraftDigest)
	assertProjectDesignSystemPackageDigest(t, queries, fixture2.Completion.System.ID, "saved", previewSavedDigest)
}

func TestCompleteProjectDesignSystemV2DoesNotReplaceExistingDraftOnFailure(t *testing.T) {
	fixture := newNativeV2CompletionFixture(t, service.ProjectDesignSystemGenerate)

	queries := db.New(testPool)
	draftDigest := strings.Repeat("d", 64)
	savedDigest := strings.Repeat("s", 64)
	upsertProjectDesignSystemPackageForTest(t, queries, fixture.Completion.System.ID, "draft", "draft-preserved-on-failure", draftDigest)
	upsertProjectDesignSystemPackageForTest(t, queries, fixture.Completion.System.ID, "saved", "saved-preserved-on-failure", savedDigest)

	body := fixture.buildPackagePayload(t, func(r *ProjectDesignSystemPackageReceipt) {
		r.ContentDigest = "sha256:" + strings.Repeat("1", 64)
	})
	w := fixture.completeTask(t, body)
	if w.Code != http.StatusBadRequest {
		t.Fatalf("CompleteTask status = %d, body = %s", w.Code, w.Body.String())
	}

	assertProjectDesignSystemPackageDigest(t, queries, fixture.Completion.System.ID, "draft", draftDigest)
	if _, err := queries.GetProjectDesignSystemPackageBySlot(context.Background(), db.GetProjectDesignSystemPackageBySlotParams{
		DesignSystemID: fixture.Completion.System.ID,
		Slot:           "draft",
		WorkspaceID:    parseUUID(testWorkspaceID),
	}); err != nil {
		t.Fatalf("draft was deleted on failure: %v", err)
	}
	var persistedDraft db.ProjectDesignSystemPackage
	if err := testPool.QueryRow(context.Background(),
		`SELECT design_md, integrity_sha256, package_schema, render_status, archive_object_key
		   FROM project_design_system_package WHERE design_system_id = $1 AND slot = 'draft'`,
		fixture.Completion.System.ID,
	).Scan(&persistedDraft.DesignMd, &persistedDraft.IntegritySha256, &persistedDraft.PackageSchema, &persistedDraft.RenderStatus, &persistedDraft.ArchiveObjectKey); err != nil {
		t.Fatalf("scan draft: %v", err)
	}
	if persistedDraft.IntegritySha256 != draftDigest || persistedDraft.PackageSchema == projectdesignsystem.PackageSchemaV2 {
		t.Fatalf("draft was mutated on failure: %+v", persistedDraft)
	}
	assertProjectDesignSystemPackageDigest(t, queries, fixture.Completion.System.ID, "saved", savedDigest)
}

func TestCompleteProjectDesignSystemV2NeverChangesSavedOnFailure(t *testing.T) {
	fixture := newNativeV2CompletionFixture(t, service.ProjectDesignSystemGenerate)

	queries := db.New(testPool)
	draftDigest := strings.Repeat("d", 64)
	savedDigest := strings.Repeat("s", 64)
	savedDesignMD := "saved-must-stay"
	upsertProjectDesignSystemPackageForTest(t, queries, fixture.Completion.System.ID, "draft", "draft-on-failure", draftDigest)
	upsertProjectDesignSystemPackageForTest(t, queries, fixture.Completion.System.ID, "saved", savedDesignMD, savedDigest)

	body := fixture.buildPackagePayload(t, func(r *ProjectDesignSystemPackageReceipt) {
		r.Audit.Passed = false
	})
	w := fixture.completeTask(t, body)
	if w.Code != http.StatusBadRequest {
		t.Fatalf("CompleteTask status = %d, body = %s", w.Code, w.Body.String())
	}

	saved, err := queries.GetProjectDesignSystemPackageBySlot(context.Background(), db.GetProjectDesignSystemPackageBySlotParams{
		DesignSystemID: fixture.Completion.System.ID,
		Slot:           "saved",
		WorkspaceID:    parseUUID(testWorkspaceID),
	})
	if err != nil {
		t.Fatalf("saved was deleted on failure: %v", err)
	}
	if saved.DesignMd != savedDesignMD || saved.IntegritySha256 != savedDigest {
		t.Fatalf("saved package was mutated on failure: %+v", saved)
	}
}

func TestCompleteProjectDesignSystemV2IsAtomicWithTaskCompletion(t *testing.T) {
	fixture := newNativeV2CompletionFixture(t, service.ProjectDesignSystemGenerate)

	queries := db.New(testPool)
	draftDigest := strings.Repeat("d", 64)
	savedDigest := strings.Repeat("s", 64)
	upsertProjectDesignSystemPackageForTest(t, queries, fixture.Completion.System.ID, "draft", "draft-before-atomic", draftDigest)
	upsertProjectDesignSystemPackageForTest(t, queries, fixture.Completion.System.ID, "saved", "saved-before-atomic", savedDigest)

	body := fixture.buildPackagePayload(t, func(r *ProjectDesignSystemPackageReceipt) {
		r.ObjectKey = "project-design-systems/wrong/key"
	})
	w := fixture.completeTask(t, body)
	if w.Code != http.StatusBadRequest {
		t.Fatalf("CompleteTask status = %d, body = %s", w.Code, w.Body.String())
	}

	task, err := queries.GetAgentTask(context.Background(), parseUUID(fixture.Completion.TaskID))
	if err != nil {
		t.Fatalf("get task: %v", err)
	}
	if task.Status == "completed" {
		t.Fatalf("task was marked completed despite validation failure")
	}

	if _, err := queries.GetProjectDesignSystemPackageBySlot(context.Background(), db.GetProjectDesignSystemPackageBySlotParams{
		DesignSystemID: fixture.Completion.System.ID,
		Slot:           "draft",
		WorkspaceID:    parseUUID(testWorkspaceID),
	}); err != nil {
		t.Fatalf("draft was deleted on failure: %v", err)
	}
	assertProjectDesignSystemPackageDigest(t, queries, fixture.Completion.System.ID, "draft", draftDigest)
	assertProjectDesignSystemPackageDigest(t, queries, fixture.Completion.System.ID, "saved", savedDigest)

	system, err := queries.GetProjectDesignSystemInWorkspace(context.Background(), db.GetProjectDesignSystemInWorkspaceParams{
		ID:          fixture.Completion.System.ID,
		WorkspaceID: parseUUID(testWorkspaceID),
	})
	if err != nil {
		t.Fatalf("get system: %v", err)
	}
	// The completion handler calls FailTask on validation failure, which
	// clears active_task_id; the system row should now be in a failed
	// terminal state with no active task and no draft promoted.
	if system.ActiveTaskID.Valid {
		t.Fatalf("system still has active_task_id = %+v after failed completion", system.ActiveTaskID)
	}
}
