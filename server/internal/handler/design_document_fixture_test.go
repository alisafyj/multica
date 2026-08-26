package handler

import (
	"os"
	"path/filepath"
	"testing"
)

// copyDesignDocumentFixture copies the designdocument package's valid fixture
// (brief, coverage, a two page prototype with a stylesheet, a script and one
// asset) into a fresh temp dir. Handler tests build real packages from it
// through CollectDirectory instead of hand-writing manifests, so a contract
// change in the package shows up here rather than in a stale fixture.
func copyDesignDocumentFixture(t *testing.T) string {
	t.Helper()
	source := filepath.Join("..", "designdocument", "testdata", "valid")
	destination := t.TempDir()
	err := filepath.WalkDir(source, func(current string, entry os.DirEntry, walkErr error) error {
		if walkErr != nil {
			return walkErr
		}
		relative, err := filepath.Rel(source, current)
		if err != nil || relative == "." {
			return err
		}
		target := filepath.Join(destination, relative)
		if entry.IsDir() {
			return os.MkdirAll(target, 0o755)
		}
		contents, err := os.ReadFile(current)
		if err != nil {
			return err
		}
		return os.WriteFile(target, contents, 0o644)
	})
	if err != nil {
		t.Fatalf("copy design document fixture: %v", err)
	}
	return destination
}
