package daemon

import (
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"testing"

	"github.com/multica-ai/multica/server/internal/projectdesignsystem"
)

func writeProjectDesignSystemArtifactFiles(t *testing.T, outputDir string) ProjectDesignSystemArtifacts {
	t.Helper()
	if err := os.MkdirAll(outputDir, 0o755); err != nil {
		t.Fatalf("create output dir: %v", err)
	}
	want := ProjectDesignSystemArtifacts{
		DesignMD:       "# CRM Design System\n\n## Principles\n\nClear and calm.",
		TokensCSS:      ":root { --color-primary: #1677ff; }",
		ComponentsHTML: `<main data-design-node-id="overview">CRM kit</main>`,
	}
	for name, contents := range map[string]string{
		"DESIGN.md":       want.DesignMD,
		"tokens.css":      want.TokensCSS,
		"components.html": want.ComponentsHTML,
	} {
		if err := os.WriteFile(filepath.Join(outputDir, name), []byte(contents), 0o644); err != nil {
			t.Fatalf("write %s: %v", name, err)
		}
	}
	return want
}

func TestReadProjectDesignSystemArtifactsAcceptsExactRegularFiles(t *testing.T) {
	outputDir := filepath.Join(t.TempDir(), "output", "project-design-system")
	want := writeProjectDesignSystemArtifactFiles(t, outputDir)

	got, err := readProjectDesignSystemArtifacts(outputDir)
	if err != nil {
		t.Fatalf("read artifacts: %v", err)
	}
	if got != want {
		t.Fatalf("artifacts = %#v, want %#v", got, want)
	}
}

func TestReadProjectDesignSystemArtifactsRejectsMissingFile(t *testing.T) {
	outputDir := filepath.Join(t.TempDir(), "output")
	writeProjectDesignSystemArtifactFiles(t, outputDir)
	if err := os.Remove(filepath.Join(outputDir, "tokens.css")); err != nil {
		t.Fatalf("remove tokens.css: %v", err)
	}

	if _, err := readProjectDesignSystemArtifacts(outputDir); err == nil {
		t.Fatal("missing artifact was accepted")
	}
}

func TestReadProjectDesignSystemArtifactsRejectsSymlink(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("symlink fixture requires developer mode on Windows")
	}
	root := t.TempDir()
	outputDir := filepath.Join(root, "output")
	writeProjectDesignSystemArtifactFiles(t, outputDir)
	target := filepath.Join(root, "outside.html")
	if err := os.WriteFile(target, []byte("outside"), 0o644); err != nil {
		t.Fatalf("write symlink target: %v", err)
	}
	path := filepath.Join(outputDir, "components.html")
	if err := os.Remove(path); err != nil {
		t.Fatalf("remove components.html: %v", err)
	}
	if err := os.Symlink(target, path); err != nil {
		t.Skipf("create symlink: %v", err)
	}

	if _, err := readProjectDesignSystemArtifacts(outputDir); err == nil {
		t.Fatal("symlink artifact was accepted")
	}
}

func TestReadProjectDesignSystemArtifactsRejectsNonRegularAndOversizedFile(t *testing.T) {
	t.Run("directory", func(t *testing.T) {
		outputDir := filepath.Join(t.TempDir(), "output")
		writeProjectDesignSystemArtifactFiles(t, outputDir)
		path := filepath.Join(outputDir, "DESIGN.md")
		if err := os.Remove(path); err != nil {
			t.Fatalf("remove DESIGN.md: %v", err)
		}
		if err := os.Mkdir(path, 0o755); err != nil {
			t.Fatalf("create directory artifact: %v", err)
		}
		if _, err := readProjectDesignSystemArtifacts(outputDir); err == nil {
			t.Fatal("non-regular artifact was accepted")
		}
	})

	t.Run("oversized", func(t *testing.T) {
		outputDir := filepath.Join(t.TempDir(), "output")
		writeProjectDesignSystemArtifactFiles(t, outputDir)
		path := filepath.Join(outputDir, "DESIGN.md")
		if err := os.WriteFile(path, []byte(strings.Repeat("x", projectdesignsystem.MaxDesignMDBytes+1)), 0o644); err != nil {
			t.Fatalf("write oversized DESIGN.md: %v", err)
		}
		if _, err := readProjectDesignSystemArtifacts(outputDir); err == nil {
			t.Fatal("oversized artifact was accepted")
		}
	})
}

func TestCompletedProjectDesignSystemWithoutArtifactsBecomesBlocked(t *testing.T) {
	result := attachProjectDesignSystemArtifacts(Task{
		ProjectDesignSystemContext: []byte(`{"type":"project_design_system_task"}`),
	}, TaskResult{
		Status:  "completed",
		Comment: "done",
		EnvRoot: t.TempDir(),
	})

	if result.Status != "blocked" {
		t.Fatalf("status = %q, want blocked", result.Status)
	}
	if result.FailureReason != "project_design_system_artifacts_invalid" {
		t.Fatalf("failure reason = %q", result.FailureReason)
	}
	if result.ProjectDesignSystemArtifacts != nil {
		t.Fatal("invalid artifact result retained a payload")
	}
}
