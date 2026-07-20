# Design MCP Restore Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Build the v1 Multica Design MCP path: a local CLI stdio MCP adapter that connects to cloud Multica APIs and fetches Restore Packs for frame, Figma group, selected layers, and selection bounds.

**Architecture:** The backend owns Restore Pack generation because it already has native JSON parsing, revision access, frame grouping, and selection context code. The CLI owns MCP stdio JSON-RPC, auth validation, token renewal, and cloud API forwarding. Design Center copy-scope UI is a later slice after the API and MCP adapter are usable.

**Tech Stack:** Go backend handlers, sqlc-generated models, Cobra CLI, stdio JSON-RPC, existing `server/internal/cli.APIClient`, existing design native JSON helpers.

---

## File Map

- Modify: `server/internal/handler/design_file.go`
  - Add Restore Pack request/response structs.
  - Add `CreateDesignRestorePack` handler.
  - Add helper functions that expand `frame`, `figma_group`, `selected_layers`, and `selection_bounds` scopes using existing native JSON helpers.

- Modify: `server/internal/handler/design_file_test.go`
  - Add backend tests for frame, group, selected layers, selection bounds, hidden layer exclusion, asset preservation, and interaction hints.

- Modify: `server/cmd/server/router.go`
  - Register `POST /api/design-files/{id}/restore-pack`.

- Create: `server/cmd/multica/cmd_mcp.go`
  - Add `multica mcp setup design`, `multica mcp status design`, and `multica mcp serve design`.

- Create: `server/cmd/multica/mcp_stdio.go`
  - Implement the small JSON-RPC stdio loop for MCP initialize, tools/list, and tools/call.

- Create: `server/cmd/multica/mcp_design.go`
  - Implement design MCP tool descriptors and handlers that call cloud Multica APIs.

- Create: `server/cmd/multica/cmd_mcp_test.go`
  - Add CLI setup/status tests and JSON-RPC stdio tests.

- Modify: `server/cmd/multica/main.go`
  - Register `mcpCmd`.

## Task 1: Backend Restore Pack API

**Files:**
- Modify: `server/internal/handler/design_file.go`
- Modify: `server/internal/handler/design_file_test.go`
- Modify: `server/cmd/server/router.go`

- [ ] **Step 1: Run impact analysis**

Run:

```bash
GITNEXUS_INVOCATION=npx NPM_CONFIG_CACHE=/tmp/multica-gitnexus-impact-help node .gitnexus/run.cjs impact --target designFrameContextFromNativeJSON --direction upstream
GITNEXUS_INVOCATION=npx NPM_CONFIG_CACHE=/tmp/multica-gitnexus-impact-help node .gitnexus/run.cjs impact --target designSelectionContextFromNativeJSON --direction upstream
GITNEXUS_INVOCATION=npx NPM_CONFIG_CACHE=/tmp/multica-gitnexus-impact-help node .gitnexus/run.cjs impact --target requestedDesignRevision --direction upstream
```

Expected: no HIGH or CRITICAL risk. If HIGH or CRITICAL appears, stop and report the blast radius before editing.

- [ ] **Step 2: Write failing frame/group Restore Pack tests**

Add tests to `server/internal/handler/design_file_test.go`:

```go
func TestCreateDesignRestorePackFrameScope(t *testing.T) {
	created := createDesignFileForRestoreTest(t)
	body := map[string]any{
		"scope": map[string]any{
			"version":      "1.0",
			"kind":         "frame",
			"designFileId": created.File.ID,
			"revisionId":   created.CurrentRevision.ID,
			"frameId":      "frame-main",
		},
	}
	w := httptest.NewRecorder()
	req := withURLParam(newRequest("POST", "/api/design-files/"+created.File.ID+"/restore-pack?workspace_id="+testWorkspaceID, body), "id", created.File.ID)
	testHandler.CreateDesignRestorePack(w, req)
	if w.Code != http.StatusOK {
		t.Fatalf("status = %d body=%s", w.Code, w.Body.String())
	}
	var resp map[string]any
	if err := json.Unmarshal(w.Body.Bytes(), &resp); err != nil {
		t.Fatal(err)
	}
	if resp["version"] != "1.0" {
		t.Fatalf("version = %#v", resp["version"])
	}
	if scope, _ := resp["scope"].(map[string]any); scope["kind"] != "frame" {
		t.Fatalf("scope = %#v", resp["scope"])
	}
	frames, _ := resp["frames"].([]any)
	if len(frames) != 1 {
		t.Fatalf("frames = %#v, want one frame", frames)
	}
}

func TestCreateDesignRestorePackFigmaGroupScope(t *testing.T) {
	created := createGroupedDesignFileForRestorePackTest(t)
	body := map[string]any{
		"scope": map[string]any{
			"version":      "1.0",
			"kind":         "figma_group",
			"designFileId": created.File.ID,
			"revisionId":   created.CurrentRevision.ID,
			"groupId":      "group-wallet",
		},
	}
	w := httptest.NewRecorder()
	req := withURLParam(newRequest("POST", "/api/design-files/"+created.File.ID+"/restore-pack?workspace_id="+testWorkspaceID, body), "id", created.File.ID)
	testHandler.CreateDesignRestorePack(w, req)
	if w.Code != http.StatusOK {
		t.Fatalf("status = %d body=%s", w.Code, w.Body.String())
	}
	var resp map[string]any
	if err := json.Unmarshal(w.Body.Bytes(), &resp); err != nil {
		t.Fatal(err)
	}
	frames, _ := resp["frames"].([]any)
	if len(frames) != 2 {
		t.Fatalf("frames = %#v, want grouped frames", frames)
	}
	structure, _ := resp["designStructure"].(map[string]any)
	if structure["mode"] != "figma_group" {
		t.Fatalf("designStructure = %#v", structure)
	}
}
```

- [ ] **Step 3: Run tests and verify RED**

Run:

```bash
cd server && go test ./internal/handler -run 'TestCreateDesignRestorePack(FrameScope|FigmaGroupScope)' -count=1
```

Expected: compile failure because `CreateDesignRestorePack` does not exist.

- [ ] **Step 4: Implement minimal frame/group Restore Pack**

In `server/internal/handler/design_file.go`, add request structs:

```go
type DesignRestorePackRequest struct {
	Scope       DesignRestoreScopeV1 `json:"scope"`
	DetailLevel string               `json:"detailLevel"`
}

type DesignRestoreScopeV1 struct {
	Version                    string                        `json:"version"`
	Kind                       string                        `json:"kind"`
	DesignFileID               string                        `json:"designFileId"`
	RevisionID                 string                        `json:"revisionId"`
	FrameID                    string                        `json:"frameId"`
	GroupID                    string                        `json:"groupId"`
	GroupName                  string                        `json:"groupName"`
	GroupPath                  []string                      `json:"groupPath"`
	FrameIDs                   []string                      `json:"frameIds"`
	LayerIDs                   []string                      `json:"layerIds"`
	SelectionBounds            *DesignSelectionBoundsRequest `json:"selectionBounds"`
	IncludeIntersectingLayers  *bool                         `json:"includeIntersectingLayers"`
	Label                      string                        `json:"label"`
	SourcePageURL              string                        `json:"sourcePageUrl"`
}
```

Add `CreateDesignRestorePack` that validates file/revision with `parseDesignFileAndWorkspaceIDs` and `requestedDesignRevision`, then calls `buildDesignRestorePackFromNativeJSON(file, revision, req.Scope, req.DetailLevel)`.

Add `buildDesignRestorePackFromNativeJSON` that returns:

- `version`
- `designFile`
- `revision`
- `scope`
- `frames`
- `designStructure`
- `assets`
- `texts`
- `colors`
- `implementationHints`
- `warnings`
- `provenance`

- [ ] **Step 5: Register route**

In `server/cmd/server/router.go`, inside `/api/design-files/{id}` routes, add:

```go
r.Post("/restore-pack", h.CreateDesignRestorePack)
```

- [ ] **Step 6: Run tests and verify GREEN**

Run:

```bash
cd server && go test ./internal/handler -run 'TestCreateDesignRestorePack(FrameScope|FigmaGroupScope)' -count=1
```

Expected: PASS.

## Task 2: Selection, Hidden Layer, Asset, and Interaction Hints

**Files:**
- Modify: `server/internal/handler/design_file.go`
- Modify: `server/internal/handler/design_file_test.go`

- [ ] **Step 1: Write failing selection/noise tests**

Add tests:

```go
func TestCreateDesignRestorePackSelectionBoundsScope(t *testing.T) {
	created := createDesignFileForRestoreTest(t)
	body := map[string]any{
		"scope": map[string]any{
			"version":      "1.0",
			"kind":         "selection_bounds",
			"designFileId": created.File.ID,
			"revisionId":   created.CurrentRevision.ID,
			"frameId":      "frame-main",
			"selectionBounds": map[string]any{"x": 0, "y": 0, "width": 300, "height": 200},
		},
	}
	w := httptest.NewRecorder()
	req := withURLParam(newRequest("POST", "/api/design-files/"+created.File.ID+"/restore-pack?workspace_id="+testWorkspaceID, body), "id", created.File.ID)
	testHandler.CreateDesignRestorePack(w, req)
	if w.Code != http.StatusOK {
		t.Fatalf("status = %d body=%s", w.Code, w.Body.String())
	}
	var resp map[string]any
	if err := json.Unmarshal(w.Body.Bytes(), &resp); err != nil {
		t.Fatal(err)
	}
	if scope, _ := resp["scope"].(map[string]any); scope["kind"] != "selection_bounds" {
		t.Fatalf("scope = %#v", scope)
	}
	if hints, _ := resp["implementationHints"].(map[string]any); hints["interactionCueCount"].(float64) < 1 {
		t.Fatalf("implementationHints = %#v, want interaction cue", hints)
	}
}
```

- [ ] **Step 2: Verify RED**

Run:

```bash
cd server && go test ./internal/handler -run 'TestCreateDesignRestorePackSelectionBoundsScope' -count=1
```

Expected: FAIL because selection Restore Pack support and interaction hints are not complete.

- [ ] **Step 3: Implement selection and conservative noise handling**

Update `buildDesignRestorePackFromNativeJSON` so:

- `selected_layers` and `selection_bounds` use `designSelectionContextFromNativeJSON`.
- hidden layers are excluded from Restore Pack layer summaries.
- visible layers that reference assets remain in `assets` and `implementationHints.assetLayerIds`.
- text containing `请选择` creates a select cue.
- text containing `请输入` creates an input cue.

- [ ] **Step 4: Verify GREEN**

Run:

```bash
cd server && go test ./internal/handler -run 'TestCreateDesignRestorePack(SelectionBoundsScope|FrameScope|FigmaGroupScope)' -count=1
```

Expected: PASS.

## Task 3: CLI MCP Commands and Auth Validation

**Files:**
- Create: `server/cmd/multica/cmd_mcp.go`
- Modify: `server/cmd/multica/main.go`
- Create: `server/cmd/multica/cmd_mcp_test.go`

- [ ] **Step 1: Run impact analysis**

Run:

```bash
GITNEXUS_INVOCATION=npx NPM_CONFIG_CACHE=/tmp/multica-gitnexus-impact-help node .gitnexus/run.cjs impact --target rootCmd --direction upstream
GITNEXUS_INVOCATION=npx NPM_CONFIG_CACHE=/tmp/multica-gitnexus-impact-help node .gitnexus/run.cjs impact --target newAPIClient --direction upstream
```

Expected: no HIGH or CRITICAL risk. If HIGH or CRITICAL appears, stop and report.

- [ ] **Step 2: Write failing setup/status tests**

Add tests in `server/cmd/multica/cmd_mcp_test.go`:

```go
func TestMCPSetupDesignPrintsSnippetWithoutToken(t *testing.T) {
	var sawAuth string
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		sawAuth = r.Header.Get("Authorization")
		switch r.URL.Path {
		case "/api/me":
			writeTestJSON(w, map[string]any{"name": "A", "email": "a@example.com"})
		case "/api/workspaces":
			writeTestJSON(w, []map[string]any{{"id": "ws-1", "name": "AMC"}})
		default:
			t.Fatalf("unexpected path %s", r.URL.Path)
		}
	}))
	defer srv.Close()
	cmd := testRootCommandWithMCP(t)
	cmd.SetArgs([]string{"--server-url", srv.URL, "--workspace-id", "ws-1", "mcp", "setup", "design"})
	out := &bytes.Buffer{}
	cmd.SetOut(out)
	if err := cmd.Execute(); err != nil {
		t.Fatal(err)
	}
	if sawAuth != "Bearer mul_secret" {
		t.Fatalf("Authorization = %q", sawAuth)
	}
	if strings.Contains(out.String(), "mul_secret") {
		t.Fatalf("setup output leaked token: %s", out.String())
	}
	if !strings.Contains(out.String(), "multica") || !strings.Contains(out.String(), "mcp") {
		t.Fatalf("setup output missing command snippet: %s", out.String())
	}
}
```

- [ ] **Step 3: Verify RED**

Run:

```bash
cd server && go test ./cmd/multica -run 'TestMCPSetupDesignPrintsSnippetWithoutToken' -count=1
```

Expected: compile failure because `mcpCmd` and helpers do not exist.

- [ ] **Step 4: Implement commands**

Create `cmd_mcp.go` with:

- `mcpCmd`
- `mcpSetupCmd`
- `mcpStatusCmd`
- `mcpServeCmd`
- `runMCPSetupDesign`
- `runMCPStatusDesign`
- `runMCPServeDesign`

Register `mcpCmd` in `main.go`.

- [ ] **Step 5: Verify GREEN**

Run:

```bash
cd server && go test ./cmd/multica -run 'TestMCPSetupDesignPrintsSnippetWithoutToken' -count=1
```

Expected: PASS.

## Task 4: Stdio MCP JSON-RPC Server

**Files:**
- Create: `server/cmd/multica/mcp_stdio.go`
- Create: `server/cmd/multica/mcp_design.go`
- Modify: `server/cmd/multica/cmd_mcp_test.go`

- [ ] **Step 1: Write failing JSON-RPC tests**

Add tests:

```go
func TestDesignMCPStdioListsTools(t *testing.T) {
	server := newDesignMCPServer(nil)
	input := strings.NewReader(`{"jsonrpc":"2.0","id":1,"method":"initialize","params":{"protocolVersion":"2025-06-18","capabilities":{},"clientInfo":{"name":"test","version":"1"}}}` + "\n" +
		`{"jsonrpc":"2.0","method":"notifications/initialized","params":{}}` + "\n" +
		`{"jsonrpc":"2.0","id":2,"method":"tools/list","params":{}}` + "\n")
	output := &bytes.Buffer{}
	if err := server.serve(input, output); err != nil {
		t.Fatal(err)
	}
	got := output.String()
	if !strings.Contains(got, "multica_design_get_restore_pack") {
		t.Fatalf("tools/list output = %s", got)
	}
}
```

- [ ] **Step 2: Verify RED**

Run:

```bash
cd server && go test ./cmd/multica -run 'TestDesignMCPStdioListsTools' -count=1
```

Expected: compile failure because `newDesignMCPServer` does not exist.

- [ ] **Step 3: Implement minimal JSON-RPC server**

Implement:

- JSON line scanner over stdin.
- Requests with `id` get JSON-RPC responses.
- Notifications without `id` get no response.
- `initialize` returns protocol version, server info, and tools capability.
- `tools/list` returns tool descriptors.
- `tools/call` dispatches by tool name.
- Unknown methods return JSON-RPC error `-32601`.

- [ ] **Step 4: Verify GREEN**

Run:

```bash
cd server && go test ./cmd/multica -run 'TestDesignMCPStdioListsTools' -count=1
```

Expected: PASS.

## Task 5: MCP Tool Calls to Cloud Restore Pack API

**Files:**
- Modify: `server/cmd/multica/mcp_design.go`
- Modify: `server/cmd/multica/cmd_mcp_test.go`

- [ ] **Step 1: Write failing tools/call test**

Add test:

```go
func TestDesignMCPGetRestorePackCallsCloudAPI(t *testing.T) {
	var gotPath string
	var gotAuth string
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotPath = r.URL.Path
		gotAuth = r.Header.Get("Authorization")
		if r.URL.Path != "/api/design-files/file-1/restore-pack" {
			t.Fatalf("unexpected path %s", r.URL.Path)
		}
		writeTestJSON(w, map[string]any{"version": "1.0", "scope": map[string]any{"kind": "frame"}})
	}))
	defer srv.Close()
	adapter := &designMCPAdapter{
		client: cli.NewAPIClient(srv.URL, "ws-1", "mul_secret"),
	}
	server := newDesignMCPServer(adapter)
	input := strings.NewReader(`{"jsonrpc":"2.0","id":1,"method":"tools/call","params":{"name":"multica_design_get_restore_pack","arguments":{"scope":{"version":"1.0","kind":"frame","designFileId":"file-1","revisionId":"rev-1","frameId":"frame-1"}}}}` + "\n")
	output := &bytes.Buffer{}
	if err := server.serve(input, output); err != nil {
		t.Fatal(err)
	}
	if gotPath != "/api/design-files/file-1/restore-pack" {
		t.Fatalf("path = %q", gotPath)
	}
	if gotAuth != "Bearer mul_secret" {
		t.Fatalf("Authorization = %q", gotAuth)
	}
	if !strings.Contains(output.String(), `"version":"1.0"`) {
		t.Fatalf("output = %s", output.String())
	}
}
```

- [ ] **Step 2: Verify RED**

Run:

```bash
cd server && go test ./cmd/multica -run 'TestDesignMCPGetRestorePackCallsCloudAPI' -count=1
```

Expected: FAIL because tool call is not implemented.

- [ ] **Step 3: Implement REST forwarding**

Implement:

- `designMCPAdapter.callTool`
- `multica_design_get_restore_pack`
- `multica_design_list_files`
- `multica_design_list_frames`
- `multica_design_list_groups`
- `multica_design_get_selection_context`
- `multica_design_get_ui_restore_artifact`

For tools not fully backed by new APIs, return useful server data from existing endpoints or an actionable message with the path/instructions.

- [ ] **Step 4: Verify GREEN**

Run:

```bash
cd server && go test ./cmd/multica -run 'TestDesignMCP(GetRestorePackCallsCloudAPI|StdioListsTools|SetupDesignPrintsSnippetWithoutToken)' -count=1
```

Expected: PASS.

## Task 6: Final Verification and Commit

**Files:**
- All changed implementation files.

- [ ] **Step 1: Run focused backend and CLI tests**

Run:

```bash
cd server && go test ./internal/handler -run 'TestCreateDesignRestorePack' -count=1
cd server && go test ./cmd/multica -run 'Test(MCP|DesignMCP)' -count=1
```

Expected: PASS.

- [ ] **Step 2: Run broader package tests**

Run:

```bash
cd server && go test ./internal/handler ./cmd/multica -count=1
```

Expected: PASS or only documented unrelated failures. Any new failure from MCP work must be fixed before commit.

- [ ] **Step 3: Run formatting and diff checks**

Run:

```bash
gofmt -w server/internal/handler/design_file.go server/internal/handler/design_file_test.go server/cmd/server/router.go server/cmd/multica/cmd_mcp.go server/cmd/multica/mcp_stdio.go server/cmd/multica/mcp_design.go server/cmd/multica/cmd_mcp_test.go server/cmd/multica/main.go
git diff --check
```

Expected: no output from `git diff --check`.

- [ ] **Step 4: Run GitNexus detect changes**

Run:

```bash
GITNEXUS_INVOCATION=npx NPM_CONFIG_CACHE=/tmp/multica-gitnexus-impact-help node .gitnexus/run.cjs detect-changes
```

Expected: risk MEDIUM or lower. If HIGH or CRITICAL appears, report before committing.

- [ ] **Step 5: Commit**

Run:

```bash
git add server/internal/handler/design_file.go server/internal/handler/design_file_test.go server/cmd/server/router.go server/cmd/multica/cmd_mcp.go server/cmd/multica/mcp_stdio.go server/cmd/multica/mcp_design.go server/cmd/multica/cmd_mcp_test.go server/cmd/multica/main.go docs/superpowers/plans/2026-07-06-design-mcp-restore-implementation.md
git commit -m "feat: add design restore mcp adapter"
```

Expected: commit succeeds.
