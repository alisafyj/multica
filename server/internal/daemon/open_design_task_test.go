package daemon

import (
	"archive/zip"
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"reflect"
	"sort"
	"strings"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/multica-ai/multica/server/internal/daemon/execenv"
	"github.com/multica-ai/multica/server/internal/opendesign"
)

func TestPrepareOpenDesignScratchPersistsGCMetadataBeforeReturn(t *testing.T) {
	t.Parallel()

	const (
		taskID      = "11111111-1111-4111-8111-111111111111"
		workspaceID = "workspace-1"
	)
	d := &Daemon{
		cfg:    Config{WorkspacesRoot: t.TempDir()},
		logger: slog.New(slog.NewTextHandler(io.Discard, nil)),
	}
	task := Task{
		ID:                         taskID,
		WorkspaceID:                workspaceID,
		ProjectDesignSystemContext: json.RawMessage(`{"type":"project_design_system_task","operation":"generate"}`),
	}

	env, err := d.prepareOpenDesignScratch(context.Background(), task, "opencode")
	if err != nil {
		t.Fatalf("prepare Open Design scratch: %v", err)
	}
	meta, err := execenv.ReadGCMeta(env.RootDir)
	if err != nil {
		t.Fatalf("read Open Design GC metadata before worker import: %v", err)
	}
	if meta.Kind != execenv.GCKindQuickCreate || meta.TaskID != taskID || meta.WorkspaceID != workspaceID || meta.CompletedAt.IsZero() {
		t.Fatalf("Open Design GC metadata = %+v", meta)
	}
}

func TestPrepareOpenDesignScratchRestoresPinnedBaseArchive(t *testing.T) {
	t.Parallel()

	const (
		taskID       = "11111111-1111-4111-8111-111111111111"
		sourceTaskID = "22222222-2222-4222-8222-222222222222"
		workerRunID  = "33333333-3333-4333-8333-333333333333"
	)
	packageFiles := map[string]string{
		"DESIGN.md":              "# Existing CRM Design System\n",
		"colors_and_type.css":    ":root { --color-primary: #00ab84; }",
		"ui_kits/app/index.html": "<!doctype html><main>Existing CRM</main>",
	}
	archive := openDesignTaskTestArchive(t, packageFiles)
	manifest := openDesignTaskTestManifest(t, "project-1", packageFiles)
	collected, err := opendesign.CollectWorkerRunResult(
		json.RawMessage(`{"schema":"open-design.run-result-package.v1","run":{"id":"`+workerRunID+`"}}`),
		manifest,
		archive,
		workerRunID,
		"project-1",
	)
	if err != nil {
		t.Fatalf("CollectWorkerRunResult: %v", err)
	}
	reference := opendesign.BasePackageReference{
		Schema:        opendesign.BasePackageReferenceSchema,
		Slot:          "saved",
		ContentDigest: collected.ContentDigest,
		SourceTaskID:  sourceTaskID,
	}
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodGet || r.URL.Path != "/api/daemon/tasks/"+taskID+"/open-design/base-archive" {
			t.Errorf("base archive request = %s %s", r.Method, r.URL.Path)
			w.WriteHeader(http.StatusNotFound)
			return
		}
		w.Header().Set("Content-Type", opendesign.RunArchiveContentType)
		w.Header().Set(opendesign.RunArchiveContentDigestHeader, reference.ContentDigest)
		w.Header().Set(opendesign.BasePackageSlotHeader, reference.Slot)
		w.Header().Set(opendesign.BasePackageSourceTaskIDHeader, reference.SourceTaskID)
		_, _ = w.Write(archive)
	}))
	t.Cleanup(server.Close)

	client := NewClient(server.URL)
	d := &Daemon{
		cfg:    Config{WorkspacesRoot: t.TempDir()},
		client: client,
		logger: slog.New(slog.NewTextHandler(io.Discard, nil)),
	}
	contextJSON, err := json.Marshal(map[string]any{
		"type":         "project_design_system_task",
		"operation":    "adjust",
		"base_package": reference,
	})
	if err != nil {
		t.Fatalf("marshal task context: %v", err)
	}
	env, err := d.prepareOpenDesignScratch(context.Background(), Task{
		ID:                         taskID,
		WorkspaceID:                "workspace-1",
		ProjectDesignSystemContext: contextJSON,
	}, "opencode")
	if err != nil {
		t.Fatalf("prepare Open Design adjustment scratch: %v", err)
	}
	for path, want := range packageFiles {
		got, err := os.ReadFile(filepath.Join(env.WorkDir, filepath.FromSlash(path)))
		if err != nil {
			t.Fatalf("read restored %s: %v", path, err)
		}
		if string(got) != want {
			t.Fatalf("restored %s = %q, want %q", path, got, want)
		}
	}
	taskJSON, err := os.ReadFile(filepath.Join(env.WorkDir, ".agent_context", "project_design_system", "task.json"))
	if err != nil {
		t.Fatalf("read restored task context: %v", err)
	}
	if bytes.Contains(taskJSON, []byte("archive_object_key")) || !bytes.Contains(taskJSON, []byte(opendesign.BasePackageReferenceSchema)) {
		t.Fatalf("restored task context = %s", taskJSON)
	}
}

func TestBuildOpenDesignPromptKeepsWorkInPrimaryAgentAndBoundsAuditRepair(t *testing.T) {
	t.Parallel()

	prompt := buildOpenDesignPrompt()
	for _, requirement := range []string{
		"complete the work yourself as the primary agent",
		"Do not delegate",
		"For adjust operations, the verified base archive is already restored into the scratch root",
		"Modify that package in place",
		"Preserve files and behavior outside the requested adjustment",
		"at most 3 Package Audit executions in total",
		"exactly the same complete set of error and warning codes and paths",
		"Do not stop merely because one finding persists while other findings improve",
	} {
		if !strings.Contains(prompt, requirement) {
			t.Fatalf("Open Design prompt is missing %q: %q", requirement, prompt)
		}
	}
	if strings.Contains(prompt, "If the same audit error or warning remains after one repair attempt") {
		t.Fatalf("Open Design prompt still contains the premature per-finding stop rule: %q", prompt)
	}
}

func TestBuildOpenDesignPromptRequiresRenderedSemanticVerificationForAdjustments(t *testing.T) {
	t.Parallel()

	prompt := buildOpenDesignPrompt()
	for _, requirement := range []string{
		"enumerate every distinct rendered component variant affected by the request",
		"final computed styles",
		"CSS cascade and selector specificity",
		"Do not treat changed tokens or source declarations as proof",
		"repair every mismatched representative before completion",
	} {
		if !strings.Contains(prompt, requirement) {
			t.Fatalf("Open Design adjustment verification contract is missing %q: %q", requirement, prompt)
		}
	}
}

func TestBuildOpenDesignPromptUsesPinnedPackageAuditContract(t *testing.T) {
	t.Parallel()

	prompt := buildOpenDesignPrompt()
	for _, requirement := range []string{
		`"$OD_NODE_BIN" "$OD_BIN" tools connectors design-system-package-audit --path . --fail-on-warnings`,
		"Do not fetch package guidance from upstream `main`",
		"Do not read, glob, grep, or inspect the pinned checkout outside the current scratch workspace",
		"non-empty embedded `<style>` block",
		"linked shared stylesheets alone do not satisfy",
		"Product Overview or Product Context",
		"concrete reuse or review workflow",
		"ui_kits/app/README.md",
	} {
		if !strings.Contains(prompt, requirement) {
			t.Fatalf("Open Design pinned package contract is missing %q: %q", requirement, prompt)
		}
	}
	for _, forbidden := range []string{
		`"$OD_NODE_BIN" "$OD_BIN" tools package-audit`,
		`"$OD_NODE_BIN" "$OD_BIN" design-systems audit`,
		`"$OD_NODE_BIN" "$OD_BIN" tools audit`,
	} {
		if strings.Contains(prompt, forbidden) {
			t.Fatalf("Open Design prompt contains unsupported Package Audit command %q: %q", forbidden, prompt)
		}
	}
	if strings.Contains(prompt, "Use only the pinned engine checkout") {
		t.Fatalf("Open Design prompt must not direct the sandboxed agent to inspect the external checkout: %q", prompt)
	}
}

func TestBuildOpenDesignPromptDefinesPinnedPackageShapeBeforeFirstAudit(t *testing.T) {
	t.Parallel()

	prompt := buildOpenDesignPrompt()
	for _, requirement := range []string{
		"Before the first Package Audit",
		"root-level `DESIGN.md`, `README.md`, `SKILL.md`, `colors_and_type.css`, and `tokens.css`",
		"context/product, color/palette, typography/type, spacing/layout, components, motion/interaction, voice/brand, and anti-patterns",
		"at least 12 CSS custom properties",
		"at least 4 concrete color values",
		"font, radius, and spacing or gap tokens",
		"at least 6 focused, complete HTML cards",
		"`preview/colors-*.html`",
		"`preview/typography-specimens.html`",
		"`preview/spacing-*.html`",
		"`preview/components-*.html`",
		"YAML frontmatter with `name`, `description`, and `user-invocable`",
		"What is inside, Source Context, When to use, How to use, and Design System Highlights",
		"Package Contents, Source Context or Source Evidence, Review or Reuse Workflow, and Preview Manifest",
		"capability verb such as supports, provides, includes, enables, or offers",
		"Use the exact heading `## Reuse Workflow`",
		"`ui_kits/app/index.html` must load `../../colors_and_type.css`",
		"at least 3 modular files under `ui_kits/app/components/`",
		"source-derived component names",
		"at least 3 actual browser-ready `components/*.js` paths",
		"must render with all outbound network access blocked",
		"Do not use CDN scripts",
		"do not load raw `.jsx` or `.tsx` files in the browser",
		"ignore any direct-JSX CDN skeleton",
	} {
		if !strings.Contains(prompt, requirement) {
			t.Fatalf("Open Design prompt is missing pinned package shape requirement %q: %q", requirement, prompt)
		}
	}
	for _, forbidden := range []string{
		"include React, ReactDOM, and Babel",
		"actual `components/*.jsx` paths",
	} {
		if strings.Contains(prompt, forbidden) {
			t.Fatalf("Open Design prompt contains network-dependent UI-kit guidance %q: %q", forbidden, prompt)
		}
	}
}

func TestBuildOpenDesignPromptDefinesOfflineUIKitAuditShape(t *testing.T) {
	t.Parallel()

	prompt := buildOpenDesignPrompt()
	for _, requirement := range []string{
		"direct executable runtime bootstrap statement",
		`document.getElementById("app").replaceChildren(rootElement)`,
		"Calls that exist only inside imported component files or helper functions do not satisfy",
		"at least 3 truthful `components/<source-derived-name>.jsx` paths",
		"compiled browser-ready `.js` counterparts",
		"exact headings `## Structure`, `## Usage`, and `## Design Notes`",
		"Usage section must contain a concrete copy, compose, import, use, build, or create workflow",
		"Design Notes must describe the source basis plus layout, colors, typography, or tokens",
	} {
		if !strings.Contains(prompt, requirement) {
			t.Fatalf("Open Design prompt is missing offline UI-kit Audit requirement %q: %q", requirement, prompt)
		}
	}
}

func TestBuildOpenDesignPromptUsesAuditCompatiblePreservedArtifactNamespaces(t *testing.T) {
	t.Parallel()

	prompt := buildOpenDesignPrompt()
	for _, requirement := range []string{
		"`assets/source-backed/`",
		"`fonts/source-backed/`",
		"`build/runtime/`",
		"do not invent or copy placeholder files",
	} {
		if !strings.Contains(prompt, requirement) {
			t.Fatalf("Open Design preserved-artifact contract is missing %q: %q", requirement, prompt)
		}
	}
}

func TestHandleTaskRoutesOpenDesignRunThroughSupervisorWithoutLegacyCompletion(t *testing.T) {
	t.Parallel()

	const (
		taskID  = "11111111-1111-4111-8111-111111111111"
		runID   = "22222222-2222-4222-8222-222222222222"
		agentID = "33333333-3333-4333-8333-333333333333"
	)
	var (
		workerMu       sync.Mutex
		workerPaths    []string
		importedRoot   string
		workerPrompt   string
		serverMu       sync.Mutex
		serverPaths    []string
		legacyComplete atomic.Int32
		legacyFail     atomic.Int32
		legacyRunner   atomic.Int32
		lifecycleMu    sync.Mutex
		lifecycle      []string
	)
	recordLifecycle := func(step string) {
		lifecycleMu.Lock()
		lifecycle = append(lifecycle, step)
		lifecycleMu.Unlock()
	}
	packageFiles := map[string]string{
		"DESIGN.md":             "# CRM",
		"preview/manifest.json": `{"version":1,"previews":[{"id":"colors","path":"colors.html"}]}`,
		"preview/colors.html":   "<main>Colors</main>",
	}
	packageManifest := openDesignTaskTestManifest(t, "project-1", packageFiles)
	packageArchive := openDesignTaskTestArchive(t, packageFiles)

	workerServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		workerMu.Lock()
		workerPaths = append(workerPaths, r.Method+" "+r.URL.Path)
		workerMu.Unlock()
		w.Header().Set("Content-Type", "application/json")
		switch r.Method + " " + r.URL.Path {
		case "GET /api/agents":
			recordLifecycle("probe")
			fmt.Fprint(w, `{"agents":[{"id":"opencode","available":true,"authStatus":"ok","version":"1.0.0","modelsSource":"live","models":[{"id":"anthropic/claude-sonnet-4-5","enabled":true}]}]}`)
		case "POST /api/import/folder":
			recordLifecycle("import")
			var body workerImportRequestForTest
			if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
				t.Errorf("decode worker import: %v", err)
				w.WriteHeader(http.StatusBadRequest)
				return
			}
			importedRoot = body.BaseDir
			if _, err := os.Stat(filepath.Join(importedRoot, ".agent_context", "project_design_system", "task.json")); err != nil {
				t.Errorf("prepared task context is missing: %v", err)
			}
			fmt.Fprint(w, `{"project":{"id":"project-1"},"conversationId":"conversation-1"}`)
		case "PATCH /api/projects/project-1":
			fmt.Fprint(w, `{"project":{"id":"project-1"}}`)
		case "POST /api/runs":
			recordLifecycle("run")
			var body map[string]json.RawMessage
			if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
				t.Errorf("decode worker run: %v", err)
				w.WriteHeader(http.StatusBadRequest)
				return
			}
			if err := json.Unmarshal(body["message"], &workerPrompt); err != nil {
				t.Errorf("decode worker prompt: %v", err)
			}
			fmt.Fprintf(w, `{"runId":%q}`, runID)
		case "GET /api/runs/" + runID + "/events":
			w.Header().Set("Content-Type", "text/event-stream")
			fmt.Fprint(w, "id: 1\nevent: start\ndata: {\"status\":\"running\"}\n\nid: 2\nevent: end\ndata: {\"status\":\"succeeded\"}\n\n")
		case "GET /api/runs/" + runID:
			fmt.Fprintf(w, `{"id":%q,"status":"succeeded","exitCode":0}`, runID)
		case "GET /api/runs/" + runID + "/result-package":
			fmt.Fprintf(w, `{"schema":%q,"run":{"id":%q,"status":"succeeded"}}`, opendesign.RunResultPackageSchema, runID)
		case "GET /api/projects/project-1/export/manifest":
			_, _ = w.Write(packageManifest)
		case "GET /api/projects/project-1/archive":
			w.Header().Set("Content-Type", "application/zip")
			_, _ = w.Write(packageArchive)
		case "GET /api/projects/project-1/design-system-package-audit":
			fmt.Fprint(w, `{"audit":{"ok":true,"projectPath":"/private/tmp/open-design/project-1","filesInspected":3,"errors":[],"warnings":[]}}`)
		case "GET /api/projects/project-1/preview-url":
			fmt.Fprint(w, `{"url":"/api/projects/project-1/preview/preview_scope_123/preview/colors.html","file":"preview/colors.html","csp":"connect-src 'none'; sandbox allow-scripts allow-forms","iframeSandbox":"allow-scripts allow-forms","opaqueOrigin":true}`)
		default:
			http.NotFound(w, r)
		}
	}))
	t.Cleanup(workerServer.Close)

	multicaServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		serverMu.Lock()
		serverPaths = append(serverPaths, r.Method+" "+r.URL.Path)
		serverMu.Unlock()
		if strings.HasSuffix(r.URL.Path, "/complete") {
			legacyComplete.Add(1)
		}
		if strings.HasSuffix(r.URL.Path, "/fail") {
			legacyFail.Add(1)
		}
		if strings.HasSuffix(r.URL.Path, "/open-design/preflight") {
			recordLifecycle("persist-ready")
		}
		w.Header().Set("Content-Type", "application/json")
		if strings.HasSuffix(r.URL.Path, "/status") {
			fmt.Fprint(w, `{"status":"running"}`)
			return
		}
		if strings.HasSuffix(r.URL.Path, "/open-design/archive") {
			fmt.Fprint(w, `{"archive_object_key":"workspaces/workspace-1/design-systems/design-system-1/open-design-runs/task-1/archive.zip"}`)
			return
		}
		fmt.Fprint(w, `{}`)
	}))
	t.Cleanup(multicaServer.Close)

	d := &Daemon{
		cfg: Config{
			ServerBaseURL:  multicaServer.URL,
			WorkspacesRoot: t.TempDir(),
		},
		client:             NewClient(multicaServer.URL),
		logger:             slog.New(slog.NewTextHandler(io.Discard, nil)),
		workspaces:         make(map[string]*workspaceState),
		activeEnvRoots:     make(map[string]int),
		runtimeIndex:       map[string]Runtime{"runtime-1": {ID: "runtime-1", Provider: "opencode"}},
		cancelPollInterval: time.Hour,
	}
	d.runner = taskRunnerFunc(func(context.Context, Task, string, int, *slog.Logger) (TaskResult, error) {
		legacyRunner.Add(1)
		return TaskResult{Status: "completed"}, nil
	})
	d.openDesignSupervisorFactory = func(prepare openDesignScratchPreparer) (openDesignSupervisor, error) {
		workerClient, err := opendesign.NewWorkerClient(workerServer.URL, "", workerServer.Client())
		if err != nil {
			return nil, err
		}
		worker := &openDesignPreparedWorker{
			WorkerAPI: workerClient,
			prepare: func(ctx context.Context) (*execenv.Environment, error) {
				recordLifecycle("scratch")
				return prepare(ctx)
			},
		}
		probe, err := opendesign.NewProbeClient(workerServer.URL, "", workerServer.Client())
		if err != nil {
			return nil, err
		}
		return opendesign.NewSupervisor(opendesign.SupervisorConfig{
			ArtifactRoot: "/pinned/open-design",
			Worker:       worker,
			Probe:        probe,
			Callbacks:    d.client,
			Preview: openDesignPreviewVerifierFunc(func(_ context.Context, targets []opendesign.PreviewURL) (opendesign.PreviewVerification, error) {
				return successfulOpenDesignTaskPreview(targets), nil
			}),
			VerifyArtifact: func(string, opendesign.EngineIdentity) (opendesign.ArtifactVerification, error) {
				return opendesign.ArtifactVerification{}, nil
			},
		})
	}

	runContext := opendesign.TaskRunContext{
		Schema: opendesign.RunSchema,
		RunID:  "44444444-4444-4444-8444-444444444444",
		Engine: opendesign.PinnedEngineIdentity(),
		Agent: opendesign.AgentIdentity{
			MulticaAgentID: agentID,
			AdapterID:      "opencode",
			Model:          "anthropic/claude-sonnet-4-5",
		},
	}
	projectContext, err := json.Marshal(map[string]any{
		"type":                     "project_design_system_task",
		"operation":                "generate",
		"project_id":               "project-1",
		"project_design_system_id": "design-system-1",
		"brief":                    "Create a source-grounded CRM design system.",
		"repository_analysis": map[string]any{
			"commit_sha": "abc123",
		},
		"open_design_run": runContext,
	})
	if err != nil {
		t.Fatalf("marshal project context: %v", err)
	}
	task := Task{
		ID:                         taskID,
		AgentID:                    agentID,
		RuntimeID:                  "runtime-1",
		WorkspaceID:                "workspace-1",
		ProjectID:                  "project-1",
		ProjectTitle:               "CRM",
		ProjectDesignSystemContext: projectContext,
		Agent: &AgentData{
			ID:   agentID,
			Name: "Local UI Designer",
		},
	}

	d.handleTask(context.Background(), task, 0)

	workerMu.Lock()
	gotWorkerPaths := append([]string(nil), workerPaths...)
	workerMu.Unlock()
	wantWorkerPaths := []string{
		"GET /api/agents",
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
	if !reflect.DeepEqual(gotWorkerPaths, wantWorkerPaths) {
		t.Fatalf("worker paths = %#v, want %#v", gotWorkerPaths, wantWorkerPaths)
	}
	if importedRoot == "" || !strings.Contains(workerPrompt, ".agent_context/project_design_system/task.json") {
		t.Fatalf("imported root = %q, worker prompt = %q", importedRoot, workerPrompt)
	}
	if strings.Contains(workerPrompt, "MULTICA_OUTPUT_DIR") {
		t.Fatalf("Open Design prompt reused the legacy three-file collector: %q", workerPrompt)
	}
	if !strings.Contains(workerPrompt, "non-interactive") || !strings.Contains(workerPrompt, "Do not ask") {
		t.Fatalf("Open Design prompt allowed an interactive confirmation instead of executing the confirmed task: %q", workerPrompt)
	}
	if legacyRunner.Load() != 0 || legacyComplete.Load() != 0 || legacyFail.Load() != 0 {
		t.Fatalf("legacy path used: runner=%d complete=%d fail=%d", legacyRunner.Load(), legacyComplete.Load(), legacyFail.Load())
	}
	lifecycleMu.Lock()
	gotLifecycle := append([]string(nil), lifecycle...)
	lifecycleMu.Unlock()
	wantLifecycle := []string{"probe", "persist-ready", "scratch", "import", "run"}
	if !reflect.DeepEqual(gotLifecycle, wantLifecycle) {
		t.Fatalf("Open Design lifecycle = %#v, want %#v", gotLifecycle, wantLifecycle)
	}

	serverMu.Lock()
	gotServerPaths := append([]string(nil), serverPaths...)
	serverMu.Unlock()
	for _, suffix := range []string{
		"/start",
		"/open-design/preflight",
		"/open-design/start",
		"/open-design/events",
		"/open-design/archive",
		"/open-design/result",
		"/open-design/audit",
		"/open-design/preview",
	} {
		if !containsPathSuffix(gotServerPaths, suffix) {
			t.Fatalf("Multica callbacks = %#v, missing %q", gotServerPaths, suffix)
		}
	}
}

type openDesignPreviewVerifierFunc func(context.Context, []opendesign.PreviewURL) (opendesign.PreviewVerification, error)

func (f openDesignPreviewVerifierFunc) Verify(ctx context.Context, targets []opendesign.PreviewURL) (opendesign.PreviewVerification, error) {
	return f(ctx, targets)
}

func successfulOpenDesignTaskPreview(targets []opendesign.PreviewURL) opendesign.PreviewVerification {
	verification := opendesign.PreviewVerification{
		Browser: opendesign.PreviewBrowserIdentity{Name: "Chrome", Version: "150.0.0.0"},
		Policy:  opendesign.PinnedPreviewVerificationPolicy(),
		Targets: make([]opendesign.PreviewTargetVerification, 0, len(targets)),
		Passed:  true,
	}
	for _, target := range targets {
		verification.Targets = append(verification.Targets, opendesign.PreviewTargetVerification{
			Target:                    target.Target,
			Passed:                    true,
			DocumentLoaded:            true,
			DOMPresent:                true,
			ComputedVisibilityVisible: true,
			RenderedElementCount:      3,
			VisibleTextLength:         12,
			BodyWidth:                 1440,
			BodyHeight:                1000,
			Screenshot: opendesign.PreviewScreenshot{
				SHA256:           "sha256:aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa",
				Bytes:            1024,
				Width:            1440,
				Height:           1000,
				Entropy:          1.5,
				MaxChannelStddev: 24,
			},
		})
	}
	return verification
}

func openDesignTaskTestManifest(t *testing.T, projectID string, files map[string]string) []byte {
	t.Helper()
	names := make([]string, 0, len(files))
	for name := range files {
		names = append(names, name)
	}
	sort.Strings(names)
	type manifestFile struct {
		Name     string `json:"name"`
		Size     int    `json:"size"`
		MIME     string `json:"mime"`
		Included bool   `json:"included"`
		Role     string `json:"role"`
	}
	manifestFiles := make([]manifestFile, 0, len(names))
	for _, name := range names {
		mime, role := "text/html", "artifact"
		if strings.HasSuffix(name, ".md") {
			mime, role = "text/markdown", "source"
		} else if strings.HasSuffix(name, ".json") {
			mime, role = "application/json", "supporting"
		}
		manifestFiles = append(manifestFiles, manifestFile{Name: name, Size: len(files[name]), MIME: mime, Included: true, Role: role})
	}
	encoded, err := json.Marshal(map[string]any{
		"schema":    opendesign.ProjectExportManifestSchema,
		"projectId": projectID,
		"files":     manifestFiles,
	})
	if err != nil {
		t.Fatalf("marshal Open Design task manifest: %v", err)
	}
	return encoded
}

func openDesignTaskTestArchive(t *testing.T, files map[string]string) []byte {
	t.Helper()
	var buffer bytes.Buffer
	writer := zip.NewWriter(&buffer)
	names := make([]string, 0, len(files))
	for name := range files {
		names = append(names, name)
	}
	sort.Strings(names)
	for _, name := range names {
		entry, err := writer.Create(name)
		if err != nil {
			t.Fatalf("create Open Design task ZIP entry: %v", err)
		}
		if _, err := entry.Write([]byte(files[name])); err != nil {
			t.Fatalf("write Open Design task ZIP entry: %v", err)
		}
	}
	if err := writer.Close(); err != nil {
		t.Fatalf("close Open Design task ZIP: %v", err)
	}
	return buffer.Bytes()
}

type workerImportRequestForTest struct {
	BaseDir string `json:"baseDir"`
}

func containsPathSuffix(paths []string, suffix string) bool {
	for _, path := range paths {
		if strings.HasSuffix(path, suffix) {
			return true
		}
	}
	return false
}
