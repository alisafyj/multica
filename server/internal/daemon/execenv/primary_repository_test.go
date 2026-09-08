package execenv

import (
	"os"
	"path/filepath"
	"runtime"
	"testing"
)

func TestPrimaryRepositoryContextRejectsSymlinkWriteRoots(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("symlink creation requires privileges on Windows")
	}
	for _, name := range []string{".git", ".agent_context", ".multica", ".claude", ".claude/skills"} {
		t.Run(name, func(t *testing.T) {
			workDir, external := t.TempDir(), t.TempDir()
			path := filepath.Join(workDir, name)
			if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
				t.Fatal(err)
			}
			if err := os.Symlink(external, path); err != nil {
				t.Fatal(err)
			}
			if err := ValidatePrimaryRepositoryContext(workDir, "claude"); err == nil {
				t.Fatal("sidecar write could follow repository symlink")
			}
			entries, err := os.ReadDir(external)
			if err != nil || len(entries) != 0 {
				t.Fatalf("preflight modified external directory: %v %v", entries, err)
			}
		})
	}
}

func TestPrimaryRepositoryContextAllowsNativeConfiguration(t *testing.T) {
	workDir := t.TempDir()
	if err := os.MkdirAll(filepath.Join(workDir, ".claude", "skills", "project"), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(workDir, ".claude", "settings.json"), []byte(`{}`), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := ValidatePrimaryRepositoryContext(workDir, "claude"); err != nil {
		t.Fatal(err)
	}
}
