package daemon

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func manualEditTaskContext(t *testing.T, edits string) json.RawMessage {
	t.Helper()
	return json.RawMessage(`{"type":"design_document_task","operation":"manual_edit","execution_ready":true,"manual_edits":` + edits + `}`)
}

func seedBasePackage(t *testing.T) (workDir string, outputDir string) {
	t.Helper()
	root := t.TempDir()
	workDir = filepath.Join(root, "work")
	outputDir = filepath.Join(root, "output")
	baseDir := filepath.Join(workDir, ".agent_context", "design_document", "base")
	for path, content := range map[string]string{
		"prototype/index.html": "<!doctype html><html><head></head><body><button id=\"go\">x</button></body></html>",
		"prototype/styles.css": "body{margin:0}",
		"brief.json":           `{"schema_version":"multica.design-document-brief/v1"}`,
	} {
		target := filepath.Join(baseDir, filepath.FromSlash(path))
		if err := os.MkdirAll(filepath.Dir(target), 0o755); err != nil {
			t.Fatal(err)
		}
		// 0o444: the real base is stamped read-only, and the applier must be
		// able to read it back anyway.
		if err := os.WriteFile(target, []byte(content), 0o444); err != nil {
			t.Fatal(err)
		}
	}
	if err := os.MkdirAll(outputDir, 0o755); err != nil {
		t.Fatal(err)
	}
	return workDir, outputDir
}

// The whole package must reach the output directory, not only the files the
// edits touched: the finalize pass collects what is there and audits it as a
// complete package.
func TestApplyDesignDocumentManualEditsWritesTheWholePackage(t *testing.T) {
	workDir, outputDir := seedBasePackage(t)
	task := Task{DesignDocumentContext: manualEditTaskContext(t,
		`[{"page":"prototype/index.html","selector":"#go","declarations":{"color":"#ff5701"}}]`)}

	if err := applyDesignDocumentManualEdits(task, workDir, outputDir); err != nil {
		t.Fatalf("apply: %v", err)
	}

	for _, path := range []string{"prototype/index.html", "prototype/styles.css", "brief.json", "prototype/manual-edits/index.html.css"} {
		if _, err := os.Stat(filepath.Join(outputDir, filepath.FromSlash(path))); err != nil {
			t.Fatalf("%s missing from the output package: %v", path, err)
		}
	}
	css, err := os.ReadFile(filepath.Join(outputDir, "prototype", "manual-edits", "index.html.css"))
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(css), "color: #ff5701 !important;") {
		t.Fatalf("override stylesheet = %s", css)
	}
	page, err := os.ReadFile(filepath.Join(outputDir, "prototype", "index.html"))
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(page), `data-multica-manual-edits="true"`) {
		t.Fatalf("page does not link its overrides: %s", page)
	}
	// The output must be writable: the finalize pass and cleanup both touch it,
	// and the read-only base's permissions must not have come along.
	info, err := os.Stat(filepath.Join(outputDir, "prototype", "index.html"))
	if err != nil {
		t.Fatal(err)
	}
	if info.Mode().Perm()&0o200 == 0 {
		t.Fatalf("output file is not writable: %v", info.Mode())
	}
}

// A malformed or empty edit set must fail the run rather than quietly produce
// a package identical to its base and call it a new revision.
func TestApplyDesignDocumentManualEditsRefusesAnUnusableEditSet(t *testing.T) {
	workDir, outputDir := seedBasePackage(t)
	for name, edits := range map[string]string{
		"empty":               `[]`,
		"page not in package": `[{"page":"prototype/gone.html","selector":"#go","declarations":{"color":"#fff"}}]`,
		"property off panel":  `[{"page":"prototype/index.html","selector":"#go","declarations":{"behavior":"url(x)"}}]`,
	} {
		t.Run(name, func(t *testing.T) {
			task := Task{DesignDocumentContext: manualEditTaskContext(t, edits)}
			if err := applyDesignDocumentManualEdits(task, workDir, outputDir); err == nil {
				t.Fatalf("edit set %q was applied", edits)
			}
		})
	}
}

// Only a manual edit skips the agent. Getting this predicate wrong would mean
// an adjustment silently producing its base unchanged.
func TestOnlyManualEditSkipsTheAgent(t *testing.T) {
	if !isDesignDocumentManualEdit(manualEditTaskContext(t, `[]`)) {
		t.Fatal("a manual edit was not recognised")
	}
	for _, raw := range []string{
		`{"type":"design_document_task","operation":"adjust","execution_ready":true}`,
		`{"type":"design_document_task","operation":"generate","execution_ready":true}`,
		`{"type":"design_document_task","operation":"regenerate","execution_ready":true}`,
		// Not execution-ready: the claim would have refused it anyway, and a
		// half-built context must never reach the applier.
		`{"type":"design_document_task","operation":"manual_edit"}`,
		`{"type":"project_design_system_task","operation":"manual_edit","execution_ready":true}`,
		`{`,
	} {
		if isDesignDocumentManualEdit(json.RawMessage(raw)) {
			t.Fatalf("%s was treated as a manual edit", raw)
		}
	}
	if isDesignDocumentManualEdit(nil) {
		t.Fatal("an absent context was treated as a manual edit")
	}
}

// A manual edit starts from the immutable base like an adjustment does, so
// every base-bound guard has to admit it — a missed one would hand the applier
// an empty base directory.
func TestManualEditCountsAsABaseBoundOperation(t *testing.T) {
	for _, operation := range []string{"adjust", "regenerate", "manual_edit"} {
		if !designDocumentOperationUsesBase(operation) {
			t.Fatalf("%s does not use a base revision", operation)
		}
	}
	if designDocumentOperationUsesBase("generate") {
		t.Fatal("a first generation must not expect a base revision")
	}
}
