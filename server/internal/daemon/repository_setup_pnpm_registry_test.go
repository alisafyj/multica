//go:build !windows

package daemon

import (
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestCopyRepositorySetupTreeOmitsPNPMProjectRegistry(t *testing.T) {
	source, destination := t.TempDir(), t.TempDir()
	registry := filepath.Join(source, "pnpm-store", "v10", "projects")
	if err := os.MkdirAll(registry, 0o700); err != nil {
		t.Fatal(err)
	}
	project := t.TempDir()
	if err := os.Symlink(project, filepath.Join(registry, "project-hash")); err != nil {
		t.Fatal(err)
	}
	for _, rel := range []string{"pnpm-store/v10/files/content", "go-mod/projects/source.go"} {
		file := filepath.Join(source, filepath.FromSlash(rel))
		if err := os.MkdirAll(filepath.Dir(file), 0o700); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(file, []byte("dependency"), 0o600); err != nil {
			t.Fatal(err)
		}
	}
	if err := copyRepositorySetupTree(context.Background(), source, destination); err != nil {
		t.Fatal(err)
	}
	if _, err := os.Lstat(filepath.Join(destination, "pnpm-store", "v10", "projects")); !os.IsNotExist(err) {
		t.Fatalf("task-local project registry copied to artifact: %v", err)
	}
	for _, rel := range []string{"pnpm-store/v10/files/content", "go-mod/projects/source.go"} {
		content, err := os.ReadFile(filepath.Join(destination, filepath.FromSlash(rel)))
		if err != nil || string(content) != "dependency" {
			t.Fatalf("dependency %s was not copied: %v", rel, err)
		}
	}
	if target, err := os.Readlink(filepath.Join(registry, "project-hash")); err != nil || target != project {
		t.Fatalf("source registry changed: %v", err)
	}
}

func TestCopyRepositorySetupTreeRegistryExceptionDoesNotAllowSymlinks(t *testing.T) {
	for _, rel := range []string{"pnpm-store/v10/projects", "pnpm-store/v10/files/link", "go-mod/projects/link"} {
		t.Run(rel, func(t *testing.T) {
			source, destination := t.TempDir(), t.TempDir()
			link := filepath.Join(source, filepath.FromSlash(rel))
			if err := os.MkdirAll(filepath.Dir(link), 0o700); err != nil {
				t.Fatal(err)
			}
			if err := os.Symlink(t.TempDir(), link); err != nil {
				t.Fatal(err)
			}
			if err := copyRepositorySetupTree(context.Background(), source, destination); err == nil || !strings.Contains(err.Error(), "symlink") {
				t.Fatalf("unsafe symlink accepted: %v", err)
			}
		})
	}
}
