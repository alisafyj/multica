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
	// The brief requires this named test to demonstrate that each
	// binding control in the V2 completion path rejects its own kind
	// of perturbation:
	//   * outer prepare (project_design_system_completion.go:94-97)
	//     — taskContext.AgentID must match task.AgentID;
	//   * nativePackageBindingForTaskContext (:278-280)
	//     — pinnedInputDigest must match the snapshot derived from the
	//     system row's InputSnapshot column;
	//   * nativePackageBindingForTaskContext (:281-283) +
	//     ValidateV2Archive — for non-generate operations, the
	//     BasePackageSHA256 must round-trip into the V2 binding and
	//     match what the daemon actually collected against.
	//
	// Each sub-case mutates exactly one control, posts a well-formed
	// receipt+archive, and asserts both the rejection AND that the
	// pre-seeded draft/saved packages survive byte-distinct.
	queries := db.New(testPool)

	t.Run("mutated task input snapshot digest", func(t *testing.T) {
		fixture := newNativeV2CompletionFixture(t, service.ProjectDesignSystemGenerate)
		draftDigest := strings.Repeat("d", 64)
		savedDigest := strings.Repeat("s", 64)
		upsertProjectDesignSystemPackageForTest(t, queries, fixture.Completion.System.ID, "draft", "draft-before-input-mismatch", draftDigest)
		upsertProjectDesignSystemPackageForTest(t, queries, fixture.Completion.System.ID, "saved", "saved-before-input-mismatch", savedDigest)

		rewriteTaskContextFields(t, fixture.Completion.TaskID, func(ctx *service.ProjectDesignSystemTaskContext) {
			// Pin a snapshot digest that is NOT what the system row's
			// InputSnapshot columns digest to. The
			// nativePackageBindingForTaskContext check at
			// project_design_system_completion.go:278-280 must reject.
			ctx.InputSnapshotSHA256 = "sha256:" + strings.Repeat("9", 64)
		})

		w := fixture.completeTask(t, fixture.buildPackagePayload(t, nil))
		if w.Code != http.StatusBadRequest {
			t.Fatalf("CompleteTask status = %d, body = %s", w.Code, w.Body.String())
		}
		assertProjectDesignSystemTaskFailed(t, fixture.Completion.TaskID, "project_design_system_invalid_artifacts")
		assertProjectDesignSystemFailureState(t, fixture.Completion.System.ID, fixture.Completion.TaskID, "project_design_system_invalid_artifacts")
		assertProjectDesignSystemPackageDigest(t, queries, fixture.Completion.System.ID, "draft", draftDigest)
		assertProjectDesignSystemPackageDigest(t, queries, fixture.Completion.System.ID, "saved", savedDigest)
	})

	t.Run("mutated task AgentID does not match task row", func(t *testing.T) {
		fixture := newNativeV2CompletionFixture(t, service.ProjectDesignSystemGenerate)
		draftDigest := strings.Repeat("d", 64)
		savedDigest := strings.Repeat("s", 64)
		upsertProjectDesignSystemPackageForTest(t, queries, fixture.Completion.System.ID, "draft", "draft-before-agent-mismatch", draftDigest)
		upsertProjectDesignSystemPackageForTest(t, queries, fixture.Completion.System.ID, "saved", "saved-before-agent-mismatch", savedDigest)

		rewriteTaskContextFields(t, fixture.Completion.TaskID, func(ctx *service.ProjectDesignSystemTaskContext) {
			// The outer prepare at :94-97 reads taskContext.AgentID and
			// compares to task.AgentID (the task row's column). Use a
			// well-formed UUID that is NOT the row's agent to trip the
			// "project design system agent does not match task" guard
			// before the V2 path is even entered.
			ctx.AgentID = "00000000-0000-0000-0000-000000000001"
		})

		w := fixture.completeTask(t, fixture.buildPackagePayload(t, nil))
		if w.Code != http.StatusBadRequest {
			t.Fatalf("CompleteTask status = %d, body = %s", w.Code, w.Body.String())
		}
		// The agent mismatch is detected in two layers — once in
		// prepareProjectDesignSystemCompletion (rejects with 400), and
		// again by the task-service's own agent guard when the
		// completion handler tries to mark the task failed. The second
		// guard refuses to transition a task whose stored agent no
		// longer matches the system; the task stays running but the
		// completion still failed and no draft was persisted. The
		// critical invariant for this sub-case is: the response is
		// 400 AND the seeded draft + saved packages are byte-distinct.
		task, err := queries.GetAgentTask(context.Background(), parseUUID(fixture.Completion.TaskID))
		if err != nil {
			t.Fatalf("get task: %v", err)
		}
		if task.Status == "completed" {
			t.Fatalf("task was marked completed despite agent binding rejection")
		}
		assertProjectDesignSystemPackageDigest(t, queries, fixture.Completion.System.ID, "draft", draftDigest)
		assertProjectDesignSystemPackageDigest(t, queries, fixture.Completion.System.ID, "saved", savedDigest)
	})

	t.Run("mutated base package digest on adjust", func(t *testing.T) {
		// Adjust requires a base package digest in the binding (see
		// validateV2Binding at projectdesignsystem/v2_archive.go:540
		// — adjust/regenerate bindings REQUIRE a valid base package
		// digest). Build a base-digest-bearing fixture: seed a "saved"
		// package on the system with a valid integrity_sha256, derive
		// the binding from that digest, then perturb the task context's
		// BasePackageSHA256 so the validator's recomputed binding
		// mismatches the manifest's binding.
		completion := createProjectDesignSystemCompletionFixture(t, service.ProjectDesignSystemAdjust)
		baseDigest := strings.Repeat("a", 64)
		upsertProjectDesignSystemPackageForTest(t, queries, completion.System.ID, "saved", "saved-base-for-adjust", baseDigest)

		binding := nativePackageBindingWithBase(t, nativePackageUploadFixture{
			System:  completion.System,
			TaskID:  completion.TaskID,
			AgentID: completion.AgentID,
		}, service.ProjectDesignSystemAdjust, "sha256:"+baseDigest)
		rewriteTaskContextForV2(t, completion, binding)
		collected := collectNativePackageArchive(t, binding)

		fixture := &nativeV2CompletionFixture{
			Completion: completion,
			Binding:    binding,
			Collected:  collected,
		}
		fixture.installStorage(t)

		draftDigest := strings.Repeat("d", 64)
		upsertProjectDesignSystemPackageForTest(t, queries, completion.System.ID, "draft", "draft-before-base-mismatch", draftDigest)

		rewriteTaskContextFields(t, completion.TaskID, func(ctx *service.ProjectDesignSystemTaskContext) {
			// A different but well-formed base digest. The manifest
			// embedded in the archive was collected against
			// "sha256:<baseDigest>"; pinning a different digest on the
			// context means the binding the V2 validator builds will
			// not equal the manifest's binding.
			ctx.BasePackageSHA256 = "sha256:" + strings.Repeat("b", 64)
		})

		w := fixture.completeTask(t, fixture.buildPackagePayload(t, nil))
		if w.Code != http.StatusBadRequest {
			t.Fatalf("CompleteTask status = %d, body = %s", w.Code, w.Body.String())
		}
		assertProjectDesignSystemTaskFailed(t, completion.TaskID, "project_design_system_invalid_artifacts")
		assertProjectDesignSystemFailureState(t, completion.System.ID, completion.TaskID, "project_design_system_invalid_artifacts")
		assertProjectDesignSystemPackageDigest(t, queries, completion.System.ID, "draft", draftDigest)
		assertProjectDesignSystemPackageDigest(t, queries, completion.System.ID, "saved", baseDigest)
	})
}

// rewriteTaskContextFields loads the task row's stored context JSON,
// applies mutator, and writes it back. Used to exercise the binding
// controls that read from the DB-seeded task context (AgentID,
// input_snapshot_sha256, base_package_sha256) — see
// project_design_system_completion.go:94-97 and
// nativePackageBindingForTaskContext at :278-283.
func rewriteTaskContextFields(t *testing.T, taskID string, mutator func(*service.ProjectDesignSystemTaskContext)) {
	t.Helper()
	var rawContext []byte
	if err := testPool.QueryRow(context.Background(),
		`SELECT context FROM agent_task_queue WHERE id = $1`, taskID,
	).Scan(&rawContext); err != nil {
		t.Fatalf("load task context: %v", err)
	}
	var ctx service.ProjectDesignSystemTaskContext
	if err := json.Unmarshal(rawContext, &ctx); err != nil {
		t.Fatalf("decode task context: %v", err)
	}
	mutator(&ctx)
	encoded, err := json.Marshal(ctx)
	if err != nil {
		t.Fatalf("re-marshal task context: %v", err)
	}
	if _, err := testPool.Exec(context.Background(),
		`UPDATE agent_task_queue SET context = $1 WHERE id = $2`,
		encoded, taskID); err != nil {
		t.Fatalf("update task context: %v", err)
	}
}

// nativePackageBindingWithBase is a sibling of the existing
// nativePackageBinding helper that stamps the package binding with
// a caller-supplied base package digest. Adjust / Regenerate flows
// require a non-empty base digest in the binding.
func nativePackageBindingWithBase(t *testing.T, fixture nativePackageUploadFixture, operation service.ProjectDesignSystemOperation, baseDigest string) projectdesignsystem.PackageBinding {
	t.Helper()
	inputDigest, err := projectdesignsystem.SnapshotDigest(fixture.System.InputSnapshot)
	if err != nil {
		t.Fatalf("digest input snapshot: %v", err)
	}
	return projectdesignsystem.PackageBinding{
		WorkspaceID:         testWorkspaceID,
		ProjectID:           uuidToString(fixture.System.ProjectID),
		DesignSystemID:      uuidToString(fixture.System.ID),
		TaskID:              fixture.TaskID,
		AgentID:             fixture.AgentID,
		Operation:           string(operation),
		InputSnapshotSHA256: inputDigest,
		BasePackageSHA256:   baseDigest,
	}
}

// installStorage uploads the fixture's archive into a fresh mockStorage
// under the daemon-derived object key and swaps testHandler.Storage to
// it for the duration of the test. Mirrors the storage install path in
// newNativeV2CompletionFixture for sub-cases that build their own
// fixture inline.
func (f *nativeV2CompletionFixture) installStorage(t *testing.T) {
	t.Helper()
	storage := &mockStorage{}
	digestHex := strings.TrimPrefix(f.Collected.Manifest.ContentDigest, "sha256:")
	objectKey := fmt.Sprintf("%s/%s/%s/%s/%s.zip",
		nativePackageObjectKeyRoot,
		f.Binding.WorkspaceID,
		f.Binding.DesignSystemID,
		f.Binding.TaskID,
		digestHex,
	)
	if _, err := storage.Upload(context.Background(), objectKey, f.Collected.Archive, "application/zip", "native.zip"); err != nil {
		t.Fatalf("seed archive in storage: %v", err)
	}
	previousStorage := testHandler.Storage
	testHandler.Storage = storage
	t.Cleanup(func() { testHandler.Storage = previousStorage })
	f.Storage = storage
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
