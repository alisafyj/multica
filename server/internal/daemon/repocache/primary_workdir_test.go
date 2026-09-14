package repocache

import (
	"os"
	"path/filepath"
	"testing"
)

func TestPrimaryRepositoryCheckoutUsesWorkDir(t *testing.T) {
	for _, isolated := range []bool{false, true} {
		name := "linked"
		if isolated {
			name = "isolated"
		}
		t.Run(name, func(t *testing.T) {
			source := createTestRepo(t)
			if err := os.WriteFile(filepath.Join(source, "AGENTS.md"), []byte("Repository-specific instructions.\n"), 0o644); err != nil {
				t.Fatal(err)
			}
			runGitAuthored(t, source, "add", "AGENTS.md")
			runGitAuthored(t, source, "commit", "-m", "add repository rules")
			cache := New(t.TempDir(), testLogger())
			if err := cache.Sync("workspace", []RepoInfo{{URL: source}}); err != nil {
				t.Fatal(err)
			}
			workDir := filepath.Join(t.TempDir(), "workdir")
			result, err := cache.CreateWorktree(WorktreeParams{
				WorkspaceID: "workspace", RepoURL: source, WorkDir: workDir,
				AgentName: "primary", TaskID: "11111111-2222-3333-4444-555555555555",
				CheckoutAtWorkDir: true, IsolatedGitMetadata: isolated,
			})
			if err != nil {
				t.Fatal(err)
			}
			if result.Path != workDir {
				t.Fatalf("provider cwd %q does not contain primary checkout %q", workDir, result.Path)
			}
			if _, err := os.Stat(filepath.Join(workDir, "AGENTS.md")); err != nil {
				t.Fatalf("repository rules unavailable at provider startup: %v", err)
			}
			if got := gitHead(t, workDir); got != gitHead(t, source) {
				t.Fatalf("primary checkout HEAD = %s, want source revision", got)
			}
		})
	}
}

func TestPrimaryRepositoryCheckoutDoesNotReplaceOccupiedDirectory(t *testing.T) {
	source := createTestRepo(t)
	cache := New(t.TempDir(), testLogger())
	if err := cache.Sync("workspace", []RepoInfo{{URL: source}}); err != nil {
		t.Fatal(err)
	}
	workDir := t.TempDir()
	sentinel := filepath.Join(workDir, "user-work.txt")
	if err := os.WriteFile(sentinel, []byte("keep\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	_, err := cache.CreateWorktree(WorktreeParams{
		WorkspaceID: "workspace", RepoURL: source, WorkDir: workDir,
		AgentName: "primary", TaskID: "11111111-2222-3333-4444-555555555555",
		CheckoutAtWorkDir: true, IsolatedGitMetadata: true,
	})
	if err == nil {
		t.Fatal("primary checkout accepted an occupied non-repository directory")
	}
	if content, err := os.ReadFile(sentinel); err != nil || string(content) != "keep\n" {
		t.Fatalf("existing content changed: %q, %v", content, err)
	}
}
