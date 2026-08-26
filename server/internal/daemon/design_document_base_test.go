package daemon

import (
	"context"
	"encoding/json"
	"io"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/multica-ai/multica/server/internal/daemon/execenv"
	"github.com/multica-ai/multica/server/internal/designdocument"
	"github.com/multica-ai/multica/server/internal/service"
)

const (
	designDocumentAdjustTaskID     = "99999999-9999-4999-8999-999999999999"
	designDocumentBaseRevisionID   = "55555555-5555-4555-8555-555555555555"
	designDocumentAdjustWorkspace  = "33333333-3333-3333-3333-333333333333"
	designDocumentAdjustDocumentID = "11111111-1111-1111-1111-111111111111"
	designDocumentAdjustAgentID    = "44444444-4444-4444-4444-444444444444"
)

// TestDesignDocumentAdjustmentMaterializesTheBaseFromTheServersOwnContext is
// the crossing test for the adjust chain: it drives the REAL producer's output
// through the REAL consumers and proves base/ actually ends up on disk.
//
// Producer: service.DesignDocumentTaskContext, the exact struct the adjust
// handler marshals into agent_task_queue.context.
//
// Consumers, both real and both reading the envelope by JSON field name:
// execenv.Prepare (which reserves .agent_context/design_document/base/) and
// restoreDesignDocumentBaseArchive (which downloads, re-validates and extracts
// into it). If a producer field tag and a consumer's field name ever diverge,
// this test fails with an empty or missing base directory instead of shipping a
// task that dies writing its own workspace.
//
// The archive served is a REAL multica.design-document/v1 package built by the
// production collector, so validation runs against genuine bytes rather than a
// stub that would pass anything.
func TestDesignDocumentAdjustmentMaterializesTheBaseFromTheServersOwnContext(t *testing.T) {
	t.Parallel()

	base := buildDesignDocumentBaseArchive(t)
	requested := make(chan string, 4)
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		requested <- r.Method + " " + r.URL.Path
		serveDesignDocumentBaseArchive(w, base.Archive, base.Manifest.ContentDigest, designDocumentBaseRevisionID)
	}))
	t.Cleanup(server.Close)

	d := newDesignDocumentAdjustDaemon(t, server.URL)
	task := designDocumentAdjustTask(t, base.Manifest.ContentDigest, nil)
	env := prepareDesignDocumentAdjustEnv(t, d, task)

	if err := d.restoreDesignDocumentBaseArchive(context.Background(), task, env.RootDir, env.WorkDir); err != nil {
		t.Fatalf("restore design document base archive: %v", err)
	}
	if got := <-requested; got != "GET /api/daemon/tasks/"+designDocumentAdjustTaskID+"/design-document/base-archive" {
		t.Fatalf("base archive request = %q", got)
	}

	baseDir := filepath.Join(env.WorkDir, ".agent_context", "design_document", "base")
	// Every artifact the package declares must be present with identical bytes:
	// the agent is told base/ IS the revision, so a partial restore would make
	// it "adjust" something that never existed.
	for _, entry := range base.Manifest.Files {
		restored, err := os.ReadFile(filepath.Join(baseDir, filepath.FromSlash(entry.Path)))
		if err != nil {
			t.Fatalf("base entry %s did not materialize: %v", entry.Path, err)
		}
		if int64(len(restored)) != entry.SizeBytes {
			t.Fatalf("base entry %s = %d bytes, want %d", entry.Path, len(restored), entry.SizeBytes)
		}
		info, err := os.Stat(filepath.Join(baseDir, filepath.FromSlash(entry.Path)))
		if err != nil {
			t.Fatalf("stat base entry %s: %v", entry.Path, err)
		}
		if info.Mode().Perm()&0o222 != 0 {
			t.Fatalf("base entry %s is writable; the base revision is immutable", entry.Path)
		}
	}
	// manifest.json is generated, not a package artifact. Restoring it would put
	// a file in base/ that no prototype may reference and that the collector
	// rejects as an undeclared path if the agent copies it forward.
	if _, err := os.Stat(filepath.Join(baseDir, "manifest.json")); !os.IsNotExist(err) {
		t.Fatalf("manifest.json was restored into base/ (err=%v)", err)
	}
	if info, err := os.Stat(baseDir); err != nil || info.Mode().Perm()&0o222 != 0 {
		t.Fatalf("base/ was not stamped read-only after the restore (info=%v err=%v)", info, err)
	}
}

// A tampered archive must be refused before anything is written. The daemon
// runs on an employee machine and the base is the only thing standing between
// the user's saved work and whatever the next revision claims to be.
func TestDesignDocumentAdjustmentRefusesATamperedBaseArchive(t *testing.T) {
	t.Parallel()

	base := buildDesignDocumentBaseArchive(t)
	// One flipped byte inside the ZIP: enough to change the entry's recomputed
	// digest, which is what ValidateArchive checks the manifest index against.
	tampered := append([]byte(nil), base.Archive...)
	tampered[len(tampered)/2] ^= 0xff

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// The pinned digest is still advertised, so the header cross-check
		// passes and the archive's own bytes have to be what refuses it.
		serveDesignDocumentBaseArchive(w, tampered, base.Manifest.ContentDigest, designDocumentBaseRevisionID)
	}))
	t.Cleanup(server.Close)

	d := newDesignDocumentAdjustDaemon(t, server.URL)
	task := designDocumentAdjustTask(t, base.Manifest.ContentDigest, nil)
	env := prepareDesignDocumentAdjustEnv(t, d, task)

	err := d.restoreDesignDocumentBaseArchive(context.Background(), task, env.RootDir, env.WorkDir)
	if err == nil {
		t.Fatal("a tampered base archive was extracted")
	}
	if !strings.Contains(err.Error(), "validate design document base archive") {
		t.Fatalf("unexpected error: %v", err)
	}
	entries, err := os.ReadDir(filepath.Join(env.WorkDir, ".agent_context", "design_document", "base"))
	if err != nil {
		t.Fatalf("read base/: %v", err)
	}
	if len(entries) != 0 {
		t.Fatalf("base/ = %d entries after a refused archive, want none", len(entries))
	}
}

// A genuine, internally consistent package that is simply NOT the revision this
// task was pinned to must be refused too. Digest equality is the only thing
// that ties the restored bytes to the revision the server recorded as this
// adjustment's base.
func TestDesignDocumentAdjustmentRefusesASubstitutedBaseArchive(t *testing.T) {
	t.Parallel()

	base := buildDesignDocumentBaseArchive(t)
	substitute := buildDesignDocumentBaseArchiveVariant(t, "\n")
	if substitute.Manifest.ContentDigest == base.Manifest.ContentDigest {
		t.Fatal("the substitute package has the same digest; it cannot prove anything")
	}

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		serveDesignDocumentBaseArchive(w, substitute.Archive, base.Manifest.ContentDigest, designDocumentBaseRevisionID)
	}))
	t.Cleanup(server.Close)

	d := newDesignDocumentAdjustDaemon(t, server.URL)
	task := designDocumentAdjustTask(t, base.Manifest.ContentDigest, nil)
	env := prepareDesignDocumentAdjustEnv(t, d, task)

	err := d.restoreDesignDocumentBaseArchive(context.Background(), task, env.RootDir, env.WorkDir)
	if err == nil {
		t.Fatal("a substituted base archive was extracted")
	}
	if !strings.Contains(err.Error(), "does not match the pinned digest") {
		t.Fatalf("unexpected error: %v", err)
	}
}

// Generation has no base, so the restore must not reach for one. Without this
// every first-generation page-design task would fail its prepare phase on a
// download that could never succeed.
func TestDesignDocumentRestoreIsANoOpForGeneration(t *testing.T) {
	t.Parallel()

	d := newDesignDocumentAdjustDaemon(t, "http://127.0.0.1:1")
	task := designDocumentTask(designDocumentAdjustTaskID, stageDesignDocumentTaskContext(t))
	if err := d.restoreDesignDocumentBaseArchive(context.Background(), task, t.TempDir(), t.TempDir()); err != nil {
		t.Fatalf("generation must not download a base archive: %v", err)
	}
	if err := d.restoreDesignDocumentBaseArchive(context.Background(), Task{ID: designDocumentAdjustTaskID}, t.TempDir(), t.TempDir()); err != nil {
		t.Fatalf("a task with no design document context must be left alone: %v", err)
	}
}

// An adjustment whose pinned reference is unusable must fail loudly at prepare
// time rather than reaching the network.
func TestDesignDocumentRestoreRejectsAnUnusableReference(t *testing.T) {
	t.Parallel()

	d := newDesignDocumentAdjustDaemon(t, "http://127.0.0.1:1")
	for name, mutate := range map[string]func(*service.DesignDocumentTaskContext){
		"no revision": func(c *service.DesignDocumentTaskContext) { c.BaseRevisionID = "" },
		"no digest":   func(c *service.DesignDocumentTaskContext) { c.BaseContentDigest = "" },
		"bad digest":  func(c *service.DesignDocumentTaskContext) { c.BaseContentDigest = "sha256:nope" },
		"bad revision": func(c *service.DesignDocumentTaskContext) {
			c.BaseRevisionID = "not-a-uuid"
		},
	} {
		t.Run(name, func(t *testing.T) {
			task := designDocumentAdjustTask(t, "sha256:"+strings.Repeat("c", 64), mutate)
			err := d.restoreDesignDocumentBaseArchive(context.Background(), task, t.TempDir(), t.TempDir())
			if err == nil {
				t.Fatal("an unusable base reference was accepted")
			}
			if !strings.Contains(err.Error(), "validate design document base package reference") {
				t.Fatalf("unexpected error: %v", err)
			}
		})
	}
}

// designDocumentAdjustTask marshals the real server-side task context. Nothing
// here is a hand-written JSON literal: the field names the daemon reads have to
// come from service.DesignDocumentTaskContext's own tags or the test proves
// nothing about the chain it is guarding.
func designDocumentAdjustTask(t *testing.T, baseContentDigest string, mutate func(*service.DesignDocumentTaskContext)) Task {
	t.Helper()
	taskContext := service.DesignDocumentTaskContext{
		Type:                service.DesignDocumentTaskContextType,
		Operation:           service.DesignDocumentAdjust,
		RequesterID:         "66666666-6666-6666-6666-666666666666",
		WorkspaceID:         designDocumentAdjustWorkspace,
		ProjectID:           "22222222-2222-2222-2222-222222222222",
		DesignDocumentID:    designDocumentAdjustDocumentID,
		AgentID:             designDocumentAdjustAgentID,
		Platform:            "web",
		Recipe:              "ui-mockup",
		Brief:               "An order review page.",
		BaseRevisionID:      designDocumentBaseRevisionID,
		BaseContentDigest:   baseContentDigest,
		Instruction:         "Make the primary action clearer.",
		Scope:               json.RawMessage(`{"page_id":"orders"}`),
		PackageSchema:       designdocument.PackageSchemaV1,
		InputSnapshotSHA256: designDocumentInputDigest,
		DesignSystemDigest:  designDocumentDesignSystemDigest,
	}
	if mutate != nil {
		mutate(&taskContext)
	}
	raw, err := json.Marshal(taskContext)
	if err != nil {
		t.Fatalf("marshal design document adjust context: %v", err)
	}
	task := designDocumentTask(designDocumentAdjustTaskID, raw)
	task.WorkspaceID = designDocumentAdjustWorkspace
	return task
}

// prepareDesignDocumentAdjustEnv builds the agent workspace through the real
// execenv path, which is what reserves base/ for the restore to fill.
func prepareDesignDocumentAdjustEnv(t *testing.T, d *Daemon, task Task) *execenv.Environment {
	t.Helper()
	env, err := execenv.Prepare(execenv.PrepareParams{
		WorkspacesRoot: d.cfg.WorkspacesRoot,
		WorkspaceID:    task.WorkspaceID,
		TaskID:         task.ID,
		AgentName:      "agent",
		Provider:       "claude",
		// The one field that matters here, carried exactly as the run path
		// carries it (see the TaskContextForEnv literal in runTask).
		Task: execenv.TaskContextForEnv{
			DesignDocumentContext: strings.TrimSpace(string(task.DesignDocumentContext)),
		},
	}, d.logger)
	if err != nil {
		t.Fatalf("prepare design document workspace: %v", err)
	}
	// base/ and context/ are stamped read-only, which also blocks TempDir
	// cleanup; production reclaims the directory through the same helper.
	t.Cleanup(func() { _ = execenv.RestoreV2SidecarWritability(env.WorkDir) })
	return env
}

func newDesignDocumentAdjustDaemon(t *testing.T, serverURL string) *Daemon {
	t.Helper()
	return &Daemon{
		cfg:    Config{WorkspacesRoot: t.TempDir()},
		client: NewClient(serverURL),
		logger: slog.New(slog.NewTextHandler(io.Discard, nil)),
	}
}

func serveDesignDocumentBaseArchive(w http.ResponseWriter, archive []byte, contentDigest, revisionID string) {
	w.Header().Set("Content-Type", designdocument.BaseArchiveContentType)
	w.Header().Set(designdocument.BaseArchiveContentDigestHeader, contentDigest)
	w.Header().Set(designdocument.BaseArchiveRevisionIDHeader, revisionID)
	_, _ = w.Write(archive)
}

// buildDesignDocumentBaseArchive produces a real package through the production
// collector, standing in for the archive the server stored for the base
// revision. Its binding names the EARLIER task that produced it, which is the
// whole reason the restore validates against the archive's own manifest binding
// rather than the adjusting run's.
func buildDesignDocumentBaseArchive(t *testing.T) designdocument.CollectedPackage {
	t.Helper()
	return buildDesignDocumentBaseArchiveVariant(t, "")
}

// buildDesignDocumentBaseArchiveVariant appends harmless whitespace to the
// prototype stylesheet so a second package is genuinely different bytes — and
// therefore a different content digest — while still passing the real audit.
func buildDesignDocumentBaseArchiveVariant(t *testing.T, styleSuffix string) designdocument.CollectedPackage {
	t.Helper()
	envRoot := t.TempDir()
	stageDesignDocumentPackage(t, envRoot)
	if styleSuffix != "" {
		writeDesignDocumentFile(t, filepath.Join(envRoot, "output", "design-document"),
			"prototype/styles.css", designDocumentStylesCSS+styleSuffix)
	}

	sourceTask := designDocumentTask("00000000-0000-4000-8000-000000000001", stageDesignDocumentTaskContext(t))
	binding, err := DecodeDesignDocumentTaskBinding(sourceTask)
	if err != nil {
		t.Fatalf("decode base package binding: %v", err)
	}
	collected, err := designdocument.CollectDirectory(filepath.Join(envRoot, "output", "design-document"), binding)
	if err != nil {
		t.Fatalf("collect base design document package: %v (audit=%+v)", err, collected.Audit)
	}
	return collected
}
