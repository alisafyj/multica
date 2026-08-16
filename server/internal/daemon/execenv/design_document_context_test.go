package execenv

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func designDocumentTaskContext(t *testing.T, overrides map[string]any) string {
	t.Helper()
	envelope := map[string]any{
		"type":               "design_document_task",
		"operation":          "generate",
		"workspace_id":       "33333333-3333-3333-3333-333333333333",
		"project_id":         "22222222-2222-2222-2222-222222222222",
		"design_document_id": "11111111-1111-1111-1111-111111111111",
		"agent_id":           "44444444-4444-4444-4444-444444444444",
		"platform":           "web",
		"recipe":             "ui-mockup",
		"brief":              "An order review page.",
		"package_schema":     "multica.design-document/v1",
	}
	for key, value := range overrides {
		envelope[key] = value
	}
	raw, err := json.Marshal(envelope)
	if err != nil {
		t.Fatalf("marshal design document context: %v", err)
	}
	return string(raw)
}

// designDocumentAdjustTaskContext is the generate envelope plus the two fields
// that make a run an adjustment: the revision it starts from and that
// revision's digest. They are what the daemon turns into a base archive
// download, so a test that omits them is testing a task nobody enqueues.
func designDocumentAdjustTaskContext(t *testing.T, overrides map[string]any) string {
	t.Helper()
	envelope := map[string]any{
		"operation":           "adjust",
		"base_revision_id":    "55555555-5555-5555-5555-555555555555",
		"base_content_digest": "sha256:" + strings.Repeat("b", 64),
		"instruction":         "Make the primary action clearer on the order review page.",
	}
	for key, value := range overrides {
		envelope[key] = value
	}
	return designDocumentTaskContext(t, envelope)
}

func writeDesignDocumentContextForTest(t *testing.T, context string) string {
	t.Helper()
	workDir := t.TempDir()
	// The sidecar is stamped read-only on purpose, which also blocks TempDir
	// cleanup; the daemon uses the same helper when reclaiming a task dir.
	t.Cleanup(func() { _ = RestoreV2SidecarWritability(workDir) })
	manifest := &sidecarManifest{}
	if err := writeDesignDocumentContext(workDir, TaskContextForEnv{DesignDocumentContext: context}, manifest); err != nil {
		t.Fatalf("writeDesignDocumentContext: %v", err)
	}
	return filepath.Join(workDir, ".agent_context", "design_document")
}

// The task envelope is what a revision's input_snapshot_sha256 is computed
// over. An agent that could edit its own brief could make any package look
// like it matched the request, so every input is stamped read-only.
func TestDesignDocumentContextInputsAreReadOnly(t *testing.T) {
	root := writeDesignDocumentContextForTest(t, designDocumentTaskContext(t, map[string]any{
		"design_context":       map[string]any{"source": "cloud_saved_project_design_system"},
		"repository_grounding": map[string]any{"commit_sha": "abc123"},
	}))

	for _, relative := range []string{
		"context/task.json",
		"context/design-system.json",
		"context/repository.json",
	} {
		info, err := os.Stat(filepath.Join(root, filepath.FromSlash(relative)))
		if err != nil {
			t.Fatalf("stat %s: %v", relative, err)
		}
		if info.Mode().Perm()&0o222 != 0 {
			t.Fatalf("%s is writable (mode %v); task inputs must be immutable", relative, info.Mode().Perm())
		}
	}
}

// Optional blocks are genuinely optional: a task with no repository and no
// saved design system still materializes a usable workspace.
func TestDesignDocumentContextOmitsAbsentOptionalBlocks(t *testing.T) {
	root := writeDesignDocumentContextForTest(t, designDocumentTaskContext(t, nil))

	if _, err := os.Stat(filepath.Join(root, "context", "task.json")); err != nil {
		t.Fatalf("task.json missing: %v", err)
	}
	for _, absent := range []string{"design-system.json", "repository.json"} {
		if _, err := os.Stat(filepath.Join(root, "context", absent)); !os.IsNotExist(err) {
			t.Fatalf("%s should not exist when the task carries no such block (err=%v)", absent, err)
		}
	}
	// base/ only exists for an adjustment; a first generation has nothing to
	// adjust and an empty base directory would be misleading.
	if _, err := os.Stat(filepath.Join(root, "base")); !os.IsNotExist(err) {
		t.Fatalf("base/ should not exist on first generation (err=%v)", err)
	}
}

// A design document package is an archive, not three text files, so the task
// context carries only the pinned revision and the daemon downloads it. This
// step therefore RESERVES base/ and leaves it writable; stamping it read-only
// here would make the daemon fail extracting into the workspace it just built.
func TestDesignDocumentContextReservesWritableBaseForAdjust(t *testing.T) {
	root := writeDesignDocumentContextForTest(t, designDocumentAdjustTaskContext(t, nil))

	info, err := os.Stat(filepath.Join(root, "base"))
	if err != nil {
		t.Fatalf("base/ missing for an adjustment: %v", err)
	}
	if info.Mode().Perm()&0o200 == 0 {
		t.Fatalf("base/ is read-only (mode %v); the daemon still has to extract the base archive into it", info.Mode().Perm())
	}
	entries, err := os.ReadDir(filepath.Join(root, "base"))
	if err != nil {
		t.Fatalf("read base/: %v", err)
	}
	if len(entries) != 0 {
		t.Fatalf("base/ = %d entries, want it empty until the daemon restores the archive", len(entries))
	}
}

// An adjustment with no pinned revision cannot be satisfied. Reserving an empty
// base/ anyway would hand the agent a directory it reads as "nothing to adjust"
// and let it emit an unrelated fresh design.
func TestDesignDocumentContextRejectsAdjustWithoutAPinnedBase(t *testing.T) {
	for name, overrides := range map[string]map[string]any{
		"no base at all":     {},
		"revision only":      {"base_revision_id": "55555555-5555-5555-5555-555555555555"},
		"digest only":        {"base_content_digest": "sha256:" + strings.Repeat("b", 64)},
		"empty revision id":  {"base_revision_id": "", "base_content_digest": "sha256:" + strings.Repeat("b", 64)},
		"empty base digest":  {"base_revision_id": "55555555-5555-5555-5555-555555555555", "base_content_digest": ""},
		"null base revision": {"base_revision_id": nil, "base_content_digest": nil},
	} {
		t.Run(name, func(t *testing.T) {
			envelope := map[string]any{"operation": "adjust"}
			for key, value := range overrides {
				envelope[key] = value
			}
			workDir := t.TempDir()
			t.Cleanup(func() { _ = RestoreV2SidecarWritability(workDir) })
			err := writeDesignDocumentContext(workDir, TaskContextForEnv{
				DesignDocumentContext: designDocumentTaskContext(t, envelope),
			}, &sidecarManifest{})
			if err == nil {
				t.Fatal("an adjustment with no pinned base revision was accepted")
			}
			if !strings.Contains(err.Error(), "no base revision to adjust") {
				t.Fatalf("unexpected error: %v", err)
			}
		})
	}
}

// Inline base contents are the shape the design SYSTEM chain uses. Accepting
// them here silently would leave base/ empty while the producer believed it had
// shipped the whole package.
func TestDesignDocumentContextRejectsInlineBasePackage(t *testing.T) {
	workDir := t.TempDir()
	t.Cleanup(func() { _ = RestoreV2SidecarWritability(workDir) })
	err := writeDesignDocumentContext(workDir, TaskContextForEnv{
		DesignDocumentContext: designDocumentAdjustTaskContext(t, map[string]any{
			"base_package": map[string]string{"brief.json": `{"pages":[]}`},
		}),
	}, &sidecarManifest{})
	if err == nil {
		t.Fatal("an inline base package was accepted; base/ would be empty and the agent would adjust nothing")
	}
	if !strings.Contains(err.Error(), "must be a reference") {
		t.Fatalf("unexpected error: %v", err)
	}
}

// The base entry names come from a platform-built package, but this writes to
// the agent's filesystem, so it validates rather than trusts.
func TestExtractDesignDocumentBaseRejectsEscapingEntries(t *testing.T) {
	for _, name := range []string{
		"../escape.json",
		"prototype/../../escape.json",
		"/absolute.json",
		"nested/../../escape.json",
		"./dot.json",
	} {
		t.Run(name, func(t *testing.T) {
			workDir := t.TempDir()
			t.Cleanup(func() { _ = RestoreV2SidecarWritability(workDir) })
			if err := writeDesignDocumentContext(workDir, TaskContextForEnv{
				DesignDocumentContext: designDocumentAdjustTaskContext(t, nil),
			}, &sidecarManifest{}); err != nil {
				t.Fatalf("writeDesignDocumentContext: %v", err)
			}
			err := ExtractDesignDocumentBase(t.TempDir(), workDir, map[string][]byte{
				"brief.json": []byte(`{"pages":[]}`),
				name:         []byte("x"),
			})
			if err == nil {
				t.Fatalf("base entry %q was accepted; it can escape the base directory", name)
			}
			if !strings.Contains(err.Error(), "unsafe design document base entry") {
				t.Fatalf("unexpected error for %q: %v", name, err)
			}
			// Nothing may be on disk: the guard runs over the whole entry set
			// before the first write, so one bad name cannot leave a partial
			// base the agent would treat as the revision.
			entries, readErr := os.ReadDir(filepath.Join(workDir, ".agent_context", "design_document", "base"))
			if readErr != nil {
				t.Fatalf("read base/: %v", readErr)
			}
			if len(entries) != 0 {
				t.Fatalf("base/ = %d entries after a rejected package, want none", len(entries))
			}
		})
	}
}

// ExtractDesignDocumentBase owns the read-only stamp, because it is the step
// that knows the base is complete.
func TestExtractDesignDocumentBaseStampsTheRestoredPackageReadOnly(t *testing.T) {
	workDir := t.TempDir()
	t.Cleanup(func() { _ = RestoreV2SidecarWritability(workDir) })
	if err := writeDesignDocumentContext(workDir, TaskContextForEnv{
		DesignDocumentContext: designDocumentAdjustTaskContext(t, nil),
	}, &sidecarManifest{}); err != nil {
		t.Fatalf("writeDesignDocumentContext: %v", err)
	}
	files := map[string][]byte{
		"brief.json":           []byte(`{"pages":[]}`),
		"coverage.json":        []byte(`{"requirements":[]}`),
		"prototype/index.html": []byte("<!doctype html><html></html>"),
	}
	if err := ExtractDesignDocumentBase(t.TempDir(), workDir, files); err != nil {
		t.Fatalf("ExtractDesignDocumentBase: %v", err)
	}

	baseDir := filepath.Join(workDir, ".agent_context", "design_document", "base")
	for name, want := range files {
		path := filepath.Join(baseDir, filepath.FromSlash(name))
		got, err := os.ReadFile(path)
		if err != nil {
			t.Fatalf("read restored %s: %v", name, err)
		}
		if string(got) != string(want) {
			t.Fatalf("restored %s = %q, want %q", name, got, want)
		}
		info, err := os.Stat(path)
		if err != nil {
			t.Fatalf("stat %s: %v", name, err)
		}
		if info.Mode().Perm()&0o222 != 0 {
			t.Fatalf("base entry %s is writable; the base revision is immutable", name)
		}
	}
	info, err := os.Stat(baseDir)
	if err != nil {
		t.Fatalf("stat base/: %v", err)
	}
	if info.Mode().Perm()&0o222 != 0 {
		t.Fatalf("base/ is still writable (mode %v) after the archive was restored", info.Mode().Perm())
	}
}

// ExtractDesignDocumentBase must not invent the directory. A base landing
// somewhere writeDesignDocumentContext did not reserve would sit outside the
// tree the agent is told to read and outside the tree cleanup can reclaim.
func TestExtractDesignDocumentBaseRequiresTheReservedDirectory(t *testing.T) {
	workDir := t.TempDir()
	t.Cleanup(func() { _ = RestoreV2SidecarWritability(workDir) })
	err := ExtractDesignDocumentBase(t.TempDir(), workDir, map[string][]byte{"brief.json": []byte("{}")})
	if err == nil {
		t.Fatal("extraction into an unreserved workspace was accepted")
	}
	if !strings.Contains(err.Error(), "not reserved") {
		t.Fatalf("unexpected error: %v", err)
	}
}

// A page-design task materializes its sidecar under a root of its own, and
// stamps it read-only. Cleanup therefore has to know that root by name: a
// 0o555 context/ holding a 0o444 task.json cannot be unlinked until the
// directory is writable again, and the unlink fails with EACCES instead.
//
// The consequence is not a cosmetic leftover. In place, Multica's own files
// stay in the user's repository; in worktree mode the cleanup error makes the
// daemon abort the worktree, so the branch that was the whole deliverable is
// never committed.
func TestCleanupLocalDirectorySidecarsRemovesTheDesignDocumentSidecar(t *testing.T) {
	envRoot := t.TempDir()
	workDir := t.TempDir()
	t.Cleanup(func() { _ = RestoreV2SidecarWritability(workDir) })

	manifest := &sidecarManifest{}
	if err := writeDesignDocumentContext(workDir, TaskContextForEnv{
		DesignDocumentContext: designDocumentTaskContext(t, map[string]any{
			"design_context":       map[string]any{"source": "cloud_saved_project_design_system"},
			"repository_grounding": map[string]any{"repository_id": "66666666-6666-6666-6666-666666666666"},
		}),
	}, manifest); err != nil {
		t.Fatalf("writeDesignDocumentContext: %v", err)
	}
	if err := writeSidecarManifest(envRoot, manifest); err != nil {
		t.Fatalf("writeSidecarManifest: %v", err)
	}
	// Guard the guard: if the inputs were not actually stamped read-only the
	// cleanup below would pass without ever exercising the restore step.
	contextDir := filepath.Join(workDir, ".agent_context", "design_document", "context")
	info, err := os.Stat(contextDir)
	if err != nil {
		t.Fatalf("stat context/: %v", err)
	}
	if info.Mode().Perm()&0o222 != 0 {
		t.Fatalf("context/ mode = %o, want no write bits; the test cannot prove cleanup restores writability", info.Mode().Perm())
	}

	if err := CleanupLocalDirectorySidecars(envRoot, workDir); err != nil {
		t.Fatalf("CleanupLocalDirectorySidecars: %v", err)
	}
	if _, err := os.Stat(filepath.Join(workDir, ".agent_context")); !os.IsNotExist(err) {
		t.Fatalf("design document sidecar remains in the workdir after cleanup: %v", err)
	}
}

// The base package lands after execenv.Prepare has already persisted its
// manifest, so the extraction has to append itself to it. Unrecorded, base/ is
// indistinguishable from a directory the user filled — the case cleanup
// deliberately preserves — and a whole prototype package survives in the
// user's own repository even once the permissions are right.
func TestCleanupLocalDirectorySidecarsRemovesTheRestoredDesignDocumentBase(t *testing.T) {
	envRoot := t.TempDir()
	workDir := t.TempDir()
	t.Cleanup(func() { _ = RestoreV2SidecarWritability(workDir) })

	manifest := &sidecarManifest{}
	if err := writeDesignDocumentContext(workDir, TaskContextForEnv{
		DesignDocumentContext: designDocumentAdjustTaskContext(t, nil),
	}, manifest); err != nil {
		t.Fatalf("writeDesignDocumentContext: %v", err)
	}
	if err := writeSidecarManifest(envRoot, manifest); err != nil {
		t.Fatalf("writeSidecarManifest: %v", err)
	}
	// Nested entries, because a real prototype package carries directories and
	// those are created by the extraction rather than reserved by Prepare.
	if err := ExtractDesignDocumentBase(envRoot, workDir, map[string][]byte{
		"brief.json":               []byte(`{"pages":[]}`),
		"prototype/index.html":     []byte("<!doctype html><html></html>"),
		"prototype/assets/app.css": []byte("body{margin:0}"),
	}); err != nil {
		t.Fatalf("ExtractDesignDocumentBase: %v", err)
	}
	baseDir := filepath.Join(workDir, ".agent_context", "design_document", "base")
	if _, err := os.Stat(filepath.Join(baseDir, "prototype", "assets", "app.css")); err != nil {
		t.Fatalf("base package did not materialize: %v", err)
	}

	if err := CleanupLocalDirectorySidecars(envRoot, workDir); err != nil {
		t.Fatalf("CleanupLocalDirectorySidecars: %v", err)
	}
	if _, err := os.Stat(filepath.Join(workDir, ".agent_context")); !os.IsNotExist(err) {
		t.Fatalf("restored base package remains in the workdir after cleanup: %v", err)
	}
}

func TestDesignDocumentContextRejectsForeignTaskTypes(t *testing.T) {
	for _, context := range []string{
		designDocumentTaskContext(t, map[string]any{"type": "project_design_system_task"}),
		designDocumentTaskContext(t, map[string]any{"operation": "repository_analysis"}),
	} {
		workDir := t.TempDir()
		t.Cleanup(func() { _ = RestoreV2SidecarWritability(workDir) })
		if err := writeDesignDocumentContext(workDir, TaskContextForEnv{DesignDocumentContext: context}, &sidecarManifest{}); err == nil {
			t.Fatalf("accepted a context that is not a design document generate/adjust task: %s", context)
		}
	}
}
