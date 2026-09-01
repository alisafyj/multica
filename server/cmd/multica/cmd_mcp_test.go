package main

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"

	"github.com/multica-ai/multica/server/internal/cli"
	"github.com/spf13/cobra"
)

func testRootCommandWithMCP() *cobra.Command {
	cmd := &cobra.Command{Use: "multica", SilenceUsage: true, SilenceErrors: true}
	cmd.PersistentFlags().String("server-url", "", "")
	cmd.PersistentFlags().String("workspace-id", "", "")
	cmd.PersistentFlags().String("profile", "", "")
	cmd.AddCommand(newMCPCommand())
	return cmd
}

func writeMCPTestJSON(w http.ResponseWriter, value any) {
	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(value)
}

func TestMCPSetupDesignPrintsSnippetWithoutToken(t *testing.T) {
	t.Setenv("MULTICA_TOKEN", "mul_secret")
	var sawAuth string
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		sawAuth = r.Header.Get("Authorization")
		switch r.URL.Path {
		case "/api/me":
			writeMCPTestJSON(w, map[string]any{"name": "A", "email": "a@example.com"})
		case "/api/workspaces":
			writeMCPTestJSON(w, []map[string]any{{"id": "ws-1", "name": "AMC"}})
		default:
			t.Fatalf("unexpected path %s", r.URL.Path)
		}
	}))
	defer srv.Close()

	cmd := testRootCommandWithMCP()
	cmd.SetArgs([]string{"--server-url", srv.URL, "--workspace-id", "ws-1", "mcp", "setup", "design"})
	out := &bytes.Buffer{}
	cmd.SetOut(out)
	if err := cmd.Execute(); err != nil {
		t.Fatal(err)
	}
	if sawAuth != "Bearer mul_secret" {
		t.Fatalf("Authorization = %q, want Bearer token", sawAuth)
	}
	if strings.Contains(out.String(), "mul_secret") {
		t.Fatalf("setup output leaked token: %s", out.String())
	}
	if !strings.Contains(out.String(), "multica") || !strings.Contains(out.String(), "mcp") || !strings.Contains(out.String(), "serve") {
		t.Fatalf("setup output missing command snippet: %s", out.String())
	}
}

func TestMCPServeDesignRenewsLegacyPAT(t *testing.T) {
	t.Setenv("MULTICA_TOKEN", "mul_secret")
	renewed := false
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/api/tokens/current/renew":
			renewed = true
			writeMCPTestJSON(w, map[string]any{})
		case "/api/me":
			writeMCPTestJSON(w, map[string]any{"name": "A", "email": "a@example.com"})
		case "/api/workspaces":
			writeMCPTestJSON(w, []map[string]any{{"id": "ws-1", "name": "AMC"}})
		default:
			t.Fatalf("unexpected path %s", r.URL.Path)
		}
	}))
	defer srv.Close()

	cmd := testRootCommandWithMCP()
	cmd.SetIn(strings.NewReader(""))
	cmd.SetArgs([]string{"--server-url", srv.URL, "--workspace-id", "ws-1", "mcp", "serve", "design"})
	if err := cmd.Execute(); err != nil {
		t.Fatal(err)
	}
	if !renewed {
		t.Fatal("legacy PAT was not renewed before MCP serve")
	}
}

func TestDesignMCPStdioListsTools(t *testing.T) {
	server := newDesignMCPServer(nil)
	input := strings.NewReader(`{"jsonrpc":"2.0","id":1,"method":"initialize","params":{"protocolVersion":"2025-06-18","capabilities":{},"clientInfo":{"name":"test","version":"1"}}}` + "\n" +
		`{"jsonrpc":"2.0","method":"notifications/initialized","params":{}}` + "\n" +
		`{"jsonrpc":"2.0","id":2,"method":"tools/list","params":{}}` + "\n")
	output := &bytes.Buffer{}
	if err := server.serve(input, output); err != nil && err != io.EOF {
		t.Fatal(err)
	}
	got := output.String()
	if !strings.Contains(got, "multica_design_get_restore_pack") {
		t.Fatalf("tools/list output = %s", got)
	}
	if !strings.Contains(got, "multica_design_get_implementation_context") {
		t.Fatalf("tools/list output missing unified implementation context: %s", got)
	}
}

func TestDesignMCPGetRestorePackCallsCloudAPI(t *testing.T) {
	var gotPath string
	var gotAuth string
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotPath = r.URL.Path
		gotAuth = r.Header.Get("Authorization")
		if r.URL.Path != "/api/design-files/file-1/restore-pack" {
			t.Fatalf("unexpected path %s", r.URL.Path)
		}
		writeMCPTestJSON(w, map[string]any{"version": "1.0", "scope": map[string]any{"kind": "frame"}})
	}))
	defer srv.Close()

	adapter := &designMCPAdapter{
		client: cli.NewAPIClient(srv.URL, "ws-1", "mul_secret"),
	}
	server := newDesignMCPServer(adapter)
	input := strings.NewReader(`{"jsonrpc":"2.0","id":1,"method":"tools/call","params":{"name":"multica_design_get_restore_pack","arguments":{"scope":{"version":"1.0","kind":"frame","designFileId":"file-1","revisionId":"rev-1","frameId":"frame-1"}}}}` + "\n")
	output := &bytes.Buffer{}
	if err := server.serve(input, output); err != nil && err != io.EOF {
		t.Fatal(err)
	}
	if gotPath != "/api/design-files/file-1/restore-pack" {
		t.Fatalf("path = %q", gotPath)
	}
	if gotAuth != "Bearer mul_secret" {
		t.Fatalf("Authorization = %q", gotAuth)
	}
	var resp map[string]any
	if err := json.Unmarshal(output.Bytes(), &resp); err != nil {
		t.Fatalf("decode JSON-RPC output %q: %v", output.String(), err)
	}
	result := resp["result"].(map[string]any)
	content := result["content"].([]any)
	text := content[0].(map[string]any)["text"].(string)
	if !strings.Contains(text, `"version":"1.0"`) {
		t.Fatalf("output text = %s", text)
	}
}

func TestDesignMCPGetImplementationContextMaterializesBoundedRelativeFiles(t *testing.T) {
	var gotBody map[string]any
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost || r.URL.Path != "/api/design-assets/design_v1_example/implementation-context" {
			t.Fatalf("unexpected request %s %s", r.Method, r.URL.Path)
		}
		if err := json.NewDecoder(r.Body).Decode(&gotBody); err != nil {
			t.Fatal(err)
		}
		writeMCPTestJSON(w, map[string]any{
			"schema_version": "multica.design-implementation-context/v1",
			"design_ref":     "design_v1_example", "revision_id": "revision-1", "content_digest": "sha256:" + strings.Repeat("a", 64),
			"frame_refs": []string{"frame_v1_example"}, "project_id": "project-1", "issue_id": "issue-1",
			"project_resource_id": "repository-1", "design_title": "Customers",
			"allowed_write_paths": []string{"."}, "verification_requirements": []string{"pnpm test"},
			"source_capabilities": map[string]any{"has_layers": true, "has_prototype": false, "has_assets": true, "has_interactions": true},
			"paths": map[string]any{
				"context_path":            ".agent_context/design_implementation/context.json",
				"design_manifest_path":    ".agent_context/design_implementation/design/manifest.json",
				"design_package_path":     ".agent_context/design_implementation/design/package",
				"scope_path":              ".agent_context/design_implementation/design/scope.json",
				"repository_context_path": ".agent_context/design_implementation/repository",
				"result_path":             ".agent_context/design_implementation/result/implementation-result.json",
			},
		})
	}))
	defer srv.Close()

	root := t.TempDir()
	adapter := &designMCPAdapter{client: cli.NewAPIClient(srv.URL, "ws-1", "mul_secret"), rootDir: root}
	result, err := adapter.getImplementationContext(context.Background(), map[string]any{
		"designRef": "design_v1_example", "revisionId": "revision-1", "frameRefs": []any{"frame_v1_example"},
		"targetRepositoryId": "repository-1", "issueId": "issue-1",
	})
	if err != nil {
		t.Fatal(err)
	}
	if gotBody["revision_id"] != "revision-1" || gotBody["issue_id"] != "issue-1" {
		t.Fatalf("API body = %+v", gotBody)
	}
	projected := result.(map[string]any)
	if strings.Contains(fmt.Sprint(projected), root) {
		t.Fatalf("MCP result leaked absolute root: %+v", projected)
	}
	for _, relative := range []string{
		".agent_context/design_implementation/context.json",
		".agent_context/design_implementation/design/manifest.json",
		".agent_context/design_implementation/design/scope.json",
	} {
		raw, err := os.ReadFile(filepath.Join(root, filepath.FromSlash(relative)))
		if err != nil || !json.Valid(raw) {
			t.Fatalf("materialized %s = %q, %v", relative, raw, err)
		}
	}
}

func TestDesignMCPGetImplementationContextRejectsSymlinkParentEscape(t *testing.T) {
	outside := t.TempDir()
	root := t.TempDir()
	if err := os.Symlink(outside, filepath.Join(root, ".agent_context")); err != nil {
		t.Fatal(err)
	}
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		writeMCPTestJSON(w, map[string]any{
			"schema_version": "multica.design-implementation-context/v1", "design_ref": "design_v1_example",
			"revision_id": "revision-1", "content_digest": "sha256:" + strings.Repeat("a", 64),
			"frame_refs": []string{"frame_v1_example"}, "project_id": "project-1", "issue_id": "issue-1", "project_resource_id": "repository-1",
		})
	}))
	defer srv.Close()
	adapter := &designMCPAdapter{client: cli.NewAPIClient(srv.URL, "ws-1", "mul_secret"), rootDir: root}
	_, err := adapter.getImplementationContext(context.Background(), map[string]any{
		"designRef": "design_v1_example", "revisionId": "revision-1", "frameRefs": []any{"frame_v1_example"},
		"targetRepositoryId": "repository-1", "issueId": "issue-1",
	})
	if err == nil {
		t.Fatal("symlink parent escape was accepted")
	}
	if _, statErr := os.Stat(filepath.Join(outside, "design_implementation", "context.json")); !os.IsNotExist(statErr) {
		t.Fatalf("context escaped root: %v", statErr)
	}
}

func TestMaterializeDesignImplementationContextRejectsInRepositoryFinalSymlink(t *testing.T) {
	root := t.TempDir()
	target := filepath.Join(root, "src", "app.ts")
	if err := os.MkdirAll(filepath.Dir(target), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(target, []byte("keep me"), 0o644); err != nil {
		t.Fatal(err)
	}
	implementationDir := filepath.Join(root, ".agent_context", "design_implementation")
	if err := os.MkdirAll(implementationDir, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.Symlink("../../src/app.ts", filepath.Join(implementationDir, "context.json")); err != nil {
		t.Fatal(err)
	}

	err := materializeDesignImplementationContext(root, designImplementationContextWire{SchemaVersion: "multica.design-implementation-context/v1"})
	if err == nil {
		t.Fatal("in-repository final symlink was accepted")
	}
	got, readErr := os.ReadFile(target)
	if readErr != nil {
		t.Fatal(readErr)
	}
	if string(got) != "keep me" {
		t.Fatalf("symlink target changed to %q", got)
	}
}

func TestMaterializeDesignImplementationContextRejectsInRepositoryAgentContextSymlink(t *testing.T) {
	root := t.TempDir()
	targetDir := filepath.Join(root, "src")
	if err := os.MkdirAll(filepath.Join(targetDir, "design_implementation"), 0o755); err != nil {
		t.Fatal(err)
	}
	target := filepath.Join(targetDir, "design_implementation", "context.json")
	if err := os.WriteFile(target, []byte("keep me"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.Symlink("src", filepath.Join(root, ".agent_context")); err != nil {
		t.Fatal(err)
	}

	err := materializeDesignImplementationContext(root, designImplementationContextWire{SchemaVersion: "multica.design-implementation-context/v1"})
	if err == nil {
		t.Fatal("in-repository .agent_context symlink was accepted")
	}
	got, readErr := os.ReadFile(target)
	if readErr != nil {
		t.Fatal(readErr)
	}
	if string(got) != "keep me" {
		t.Fatalf("symlink target changed to %q", got)
	}
}

func TestMaterializeDesignImplementationContextReplacesOwnedSubtreeAndPreservesSiblings(t *testing.T) {
	root := t.TempDir()
	for relative, content := range map[string]string{
		".agent_context/design_implementation/design/package/stale.bin":          "stale package",
		".agent_context/design_implementation/repository/stale.json":             "stale repository",
		".agent_context/design_implementation/result/implementation-result.json": "stale result",
		".agent_context/design_delivery/package/restore-pack.json":               "keep sibling",
	} {
		full := filepath.Join(root, filepath.FromSlash(relative))
		if err := os.MkdirAll(filepath.Dir(full), 0o755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(full, []byte(content), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	contextValue := designImplementationContextWire{
		SchemaVersion: "multica.design-implementation-context/v1", DesignRef: "design_v1_new",
		RevisionID: "revision-new", FrameRefs: []string{"frame_v1_new"},
	}
	if err := materializeDesignImplementationContext(root, contextValue); err != nil {
		t.Fatal(err)
	}
	for _, relative := range []string{
		".agent_context/design_implementation/design/package/stale.bin",
		".agent_context/design_implementation/repository/stale.json",
		".agent_context/design_implementation/result/implementation-result.json",
	} {
		if _, err := os.Stat(filepath.Join(root, filepath.FromSlash(relative))); !os.IsNotExist(err) {
			t.Fatalf("stale artifact %s remains: %v", relative, err)
		}
	}
	sibling, err := os.ReadFile(filepath.Join(root, ".agent_context", "design_delivery", "package", "restore-pack.json"))
	if err != nil {
		t.Fatal(err)
	}
	if string(sibling) != "keep sibling" {
		t.Fatalf("unrelated sibling changed to %q", sibling)
	}
	raw, err := os.ReadFile(filepath.Join(root, filepath.FromSlash(designImplementationContextPath)))
	if err != nil {
		t.Fatal(err)
	}
	var got designImplementationContextWire
	if err := json.Unmarshal(raw, &got); err != nil {
		t.Fatal(err)
	}
	if got.DesignRef != "design_v1_new" || got.RevisionID != "revision-new" {
		t.Fatalf("new identity was not materialized: %+v", got)
	}
}

func TestReplaceImplementationRootRestoresOldContextWhenBackupCleanupFails(t *testing.T) {
	root := t.TempDir()
	agentContext := filepath.Join(root, ".agent_context")
	for relative, content := range map[string]string{
		"design_implementation/context.json":      "old identity",
		".design_implementation-new/context.json": "new identity",
	} {
		full := filepath.Join(agentContext, filepath.FromSlash(relative))
		if err := os.MkdirAll(filepath.Dir(full), 0o755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(full, []byte(content), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	agentRoot, err := os.OpenRoot(agentContext)
	if err != nil {
		t.Fatal(err)
	}
	defer agentRoot.Close()
	cleanupFailure := errors.New("forced backup cleanup failure")
	cleanup := func(relative string) error {
		if strings.HasPrefix(relative, ".design_implementation-old-") {
			return cleanupFailure
		}
		return agentRoot.RemoveAll(relative)
	}

	err = replaceImplementationRoot(agentRoot, ".design_implementation-new", cleanup)
	if !errors.Is(err, cleanupFailure) {
		t.Fatalf("replacement error = %v, want cleanup failure", err)
	}
	got, readErr := os.ReadFile(filepath.Join(agentContext, "design_implementation", "context.json"))
	if readErr != nil {
		t.Fatal(readErr)
	}
	if string(got) != "old identity" {
		t.Fatalf("canonical context = %q, want complete old identity", got)
	}
	entries, readErr := os.ReadDir(agentContext)
	if readErr != nil {
		t.Fatal(readErr)
	}
	if len(entries) != 1 || entries[0].Name() != "design_implementation" {
		t.Fatalf("replacement left non-canonical trees: %+v", entries)
	}
}

func TestDesignMCPGetImplementationContextPreservesSavedMulticaIdentity(t *testing.T) {
	want := designImplementationContextWire{
		SchemaVersion: "multica.design-implementation-context/v1", DesignRef: "design_v1_multica",
		RevisionID: "saved-revision", ContentDigest: "sha256:" + strings.Repeat("b", 64),
		FrameRefs: []string{"frame_v1_page"}, ProjectID: "project-1", IssueID: "issue-1",
		ProjectResourceID: "repository-1", DesignTitle: "Saved document", DesignSystemDigest: "sha256:design-system",
		AllowedWritePaths: []string{"."}, Verification: []string{"pnpm test"},
		Capabilities: map[string]any{"has_layers": false, "has_prototype": true, "has_assets": true, "has_interactions": true},
	}
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		writeMCPTestJSON(w, want)
	}))
	defer srv.Close()

	root := t.TempDir()
	adapter := &designMCPAdapter{client: cli.NewAPIClient(srv.URL, "ws-1", "mul_secret"), rootDir: root}
	_, err := adapter.getImplementationContext(context.Background(), map[string]any{
		"designRef": want.DesignRef, "revisionId": want.RevisionID, "frameRefs": []any{want.FrameRefs[0]},
		"targetRepositoryId": want.ProjectResourceID, "issueId": want.IssueID,
	})
	if err != nil {
		t.Fatal(err)
	}
	raw, err := os.ReadFile(filepath.Join(root, filepath.FromSlash(designImplementationContextPath)))
	if err != nil {
		t.Fatal(err)
	}
	var got designImplementationContextWire
	if err := json.Unmarshal(raw, &got); err != nil {
		t.Fatal(err)
	}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("saved Multica context identity changed:\n got: %+v\nwant: %+v", got, want)
	}
}
