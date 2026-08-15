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

func TestDesignDocumentContextMaterializesBaseForAdjust(t *testing.T) {
	root := writeDesignDocumentContextForTest(t, designDocumentTaskContext(t, map[string]any{
		"operation": "adjust",
		"base_package": map[string]string{
			"brief.json":           `{"pages":[]}`,
			"prototype/index.html": "<!doctype html><html></html>",
			"coverage.json":        `{"requirements":[]}`,
		},
	}))

	for _, relative := range []string{"brief.json", "prototype/index.html", "coverage.json"} {
		path := filepath.Join(root, "base", filepath.FromSlash(relative))
		info, err := os.Stat(path)
		if err != nil {
			t.Fatalf("base entry %s missing: %v", relative, err)
		}
		if info.Mode().Perm()&0o222 != 0 {
			t.Fatalf("base entry %s is writable; the base revision is immutable", relative)
		}
	}
}

// The base entry names come from a platform-built package, but this writes to
// the agent's filesystem, so it validates rather than trusts.
func TestDesignDocumentContextRejectsEscapingBaseEntries(t *testing.T) {
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
			err := writeDesignDocumentContext(workDir, TaskContextForEnv{
				DesignDocumentContext: designDocumentTaskContext(t, map[string]any{
					"operation":    "adjust",
					"base_package": map[string]string{name: "x"},
				}),
			}, &sidecarManifest{})
			if err == nil {
				t.Fatalf("base entry %q was accepted; it can escape the base directory", name)
			}
			if !strings.Contains(err.Error(), "unsafe design document base entry") {
				t.Fatalf("unexpected error for %q: %v", name, err)
			}
		})
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
