package daemon

import (
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"sort"

	"github.com/multica-ai/multica/server/internal/designdocument"
)

// Running a designer's own edits, with no agent in the loop (DC-062).
//
// This is the one design-document operation the daemon performs itself. The
// designer already saw the result on the canvas, so re-deriving it through a
// model would be slower and could come back different; what runs instead is a
// pure transformation of the base package.
//
// Everything after this point is unchanged. The edited package lands in
// $MULTICA_OUTPUT_DIR exactly where an agent would have written it, and the
// same finalize pass collects it, runs the static Audit, drives the browser
// preview gate and uploads it. A manual edit is faster than an adjustment,
// never less checked than one — a bad override that blanks the page fails the
// gate like any other bad package would.

func isDesignDocumentManualEdit(raw json.RawMessage) bool {
	if len(raw) == 0 {
		return false
	}
	var contextValue struct {
		Type           string `json:"type"`
		Operation      string `json:"operation"`
		ExecutionReady bool   `json:"execution_ready"`
	}
	return json.Unmarshal(raw, &contextValue) == nil &&
		contextValue.Type == "design_document_task" &&
		contextValue.Operation == "manual_edit" &&
		contextValue.ExecutionReady
}

// applyDesignDocumentManualEdits reads the restored base, applies the
// overrides and writes the whole package to the output directory.
//
// The base is read back off disk rather than kept from the download, because
// the extraction is what the run is actually built on: if the two ever
// disagreed, the package that reached Audit would not be the one this function
// thought it produced.
func applyDesignDocumentManualEdits(task Task, workDir, outputDir string) error {
	var envelope struct {
		ManualEdits []designdocument.ManualEdit `json:"manual_edits"`
	}
	if err := jsonUnmarshal(task.DesignDocumentContext, &envelope); err != nil {
		return fmt.Errorf("decode manual edits: %w", err)
	}
	if len(envelope.ManualEdits) == 0 {
		return errors.New("manual edit task carries no edits")
	}
	if outputDir == "" {
		return errors.New("manual edit has no output directory")
	}
	baseDir := filepath.Join(workDir, ".agent_context", "design_document", "base")
	files, err := readPackageTree(baseDir)
	if err != nil {
		return fmt.Errorf("read the base package: %w", err)
	}
	edited, err := designdocument.ApplyManualEdits(files, envelope.ManualEdits)
	if err != nil {
		return fmt.Errorf("apply manual edits: %w", err)
	}
	return writePackageTree(outputDir, edited)
}

// readPackageTree loads a package directory into memory, keyed by the
// slash-separated path the package contract uses.
func readPackageTree(root string) (map[string][]byte, error) {
	files := map[string][]byte{}
	err := filepath.WalkDir(root, func(path string, entry os.DirEntry, walkErr error) error {
		if walkErr != nil {
			return walkErr
		}
		if entry.IsDir() {
			return nil
		}
		info, err := entry.Info()
		if err != nil {
			return err
		}
		// A package holds regular files only; anything else is not something
		// to copy blindly into the run's output.
		if !info.Mode().IsRegular() {
			return fmt.Errorf("package entry %q is not a regular file", path)
		}
		relative, err := filepath.Rel(root, path)
		if err != nil {
			return err
		}
		content, err := os.ReadFile(path)
		if err != nil {
			return err
		}
		files[filepath.ToSlash(relative)] = content
		return nil
	})
	if err != nil {
		return nil, err
	}
	if len(files) == 0 {
		return nil, errors.New("package directory is empty")
	}
	return files, nil
}

// writePackageTree writes the package out in a stable order, so a failure part
// way through leaves the same partial state every time rather than a different
// one per run.
func writePackageTree(root string, files map[string][]byte) error {
	names := make([]string, 0, len(files))
	for name := range files {
		names = append(names, name)
	}
	sort.Strings(names)
	for _, name := range names {
		target := filepath.Join(root, filepath.FromSlash(name))
		if err := os.MkdirAll(filepath.Dir(target), 0o755); err != nil {
			return fmt.Errorf("create %q: %w", name, err)
		}
		if err := os.WriteFile(target, files[name], 0o644); err != nil {
			return fmt.Errorf("write %q: %w", name, err)
		}
	}
	return nil
}
