package daemon

import (
	"context"
	"errors"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"testing"
	"time"
)

func newPrimaryRepositoryGitFixture(t *testing.T) (envRoot, workDir, source string, task Task) {
	t.Helper()
	envRoot = t.TempDir()
	workDir = filepath.Join(envRoot, "workdir")
	source = createGCGitRepo(t)
	runGitForGC(t, "", "clone", source, workDir)
	task = Task{IssueID: "issue", Repos: []RepoData{{URL: source, Ref: "main"}}}
	return envRoot, workDir, source, task
}

func writePrimaryRepositoryFixture(t *testing.T, envRoot, workDir, source string) {
	t.Helper()
	if err := writePrimaryRepository(envRoot, &preparedPrimaryRepository{
		Version: 1, URL: source, Ref: "main", WorkDir: workDir, BranchName: "stale/initial-branch",
	}); err != nil {
		t.Fatal(err)
	}
}

func TestResumePrimaryRepositoryVerifiesGitIdentity(t *testing.T) {
	t.Run("mismatched origin", func(t *testing.T) {
		envRoot, workDir, source, task := newPrimaryRepositoryGitFixture(t)
		writePrimaryRepositoryFixture(t, envRoot, workDir, source)
		const credential = "resume-secret-value"
		runGitForGC(t, workDir, "remote", "set-url", "origin", "https://user:"+credential+"@example.invalid/other.git")
		if _, err := resumePrimaryRepository(task, workDir); err == nil {
			t.Fatal("checkout with mismatched origin was resumed")
		} else if strings.Contains(err.Error(), credential) {
			t.Fatalf("origin credential leaked in error: %v", err)
		}
	})

	t.Run("missing git metadata", func(t *testing.T) {
		envRoot, workDir, source, task := newPrimaryRepositoryGitFixture(t)
		writePrimaryRepositoryFixture(t, envRoot, workDir, source)
		if err := os.RemoveAll(filepath.Join(workDir, ".git")); err != nil {
			t.Fatal(err)
		}
		if _, err := resumePrimaryRepository(task, workDir); err == nil {
			t.Fatal("checkout without Git metadata was resumed")
		}
	})

	t.Run("parent repository", func(t *testing.T) {
		envRoot := t.TempDir()
		runGitForGC(t, envRoot, "init", "-b", "main")
		workDir := filepath.Join(envRoot, "workdir")
		if err := os.Mkdir(workDir, 0o755); err != nil {
			t.Fatal(err)
		}
		source := createGCGitRepo(t)
		writePrimaryRepositoryFixture(t, envRoot, workDir, source)
		task := Task{IssueID: "issue", Repos: []RepoData{{URL: source, Ref: "main"}}}
		if _, err := resumePrimaryRepository(task, workDir); err == nil {
			t.Fatal("nested directory inherited parent repository identity")
		}
	})

	t.Run("outside git directory symlink", func(t *testing.T) {
		if runtime.GOOS == "windows" {
			t.Skip("symlink fixture requires privileges on Windows")
		}
		envRoot := t.TempDir()
		workDir := filepath.Join(envRoot, "workdir")
		if err := os.Mkdir(workDir, 0o755); err != nil {
			t.Fatal(err)
		}
		source := createGCGitRepo(t)
		if err := os.Symlink(filepath.Join(source, ".git"), filepath.Join(workDir, ".git")); err != nil {
			t.Fatal(err)
		}
		writePrimaryRepositoryFixture(t, envRoot, workDir, source)
		task := Task{IssueID: "issue", Repos: []RepoData{{URL: source, Ref: "main"}}}
		if _, err := resumePrimaryRepository(task, workDir); err == nil {
			t.Fatal("checkout with external Git metadata was resumed")
		}
	})
}

func TestResumePrimaryRepositoryAllowsDirtyBranchChangesAndDetachedHead(t *testing.T) {
	envRoot, workDir, source, task := newPrimaryRepositoryGitFixture(t)
	writePrimaryRepositoryFixture(t, envRoot, workDir, source)
	runGitForGC(t, workDir, "checkout", "-b", "feature/user-work")
	if err := os.WriteFile(filepath.Join(workDir, "README.md"), []byte("dirty work\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	wantHead := runGitForGC(t, workDir, "rev-parse", "HEAD")
	wantStatus := runGitForGC(t, workDir, "status", "--porcelain=v1")

	state, err := resumePrimaryRepository(task, workDir)
	if err != nil || state == nil || state.BranchName != "feature/user-work" {
		t.Fatalf("dirty branch resume=%+v error=%v", state, err)
	}
	canonicalWorkDir, err := filepath.EvalSymlinks(workDir)
	if err != nil {
		t.Fatal(err)
	}
	if state.WorkDir != canonicalWorkDir {
		t.Fatalf("resume workdir = %q, want canonical path %q", state.WorkDir, canonicalWorkDir)
	}
	if got := runGitForGC(t, workDir, "rev-parse", "HEAD"); got != wantHead {
		t.Fatalf("resume changed HEAD: got %s want %s", got, wantHead)
	}
	if got := runGitForGC(t, workDir, "status", "--porcelain=v1"); got != wantStatus {
		t.Fatalf("resume changed dirty tree: got %q want %q", got, wantStatus)
	}

	runGitForGC(t, workDir, "checkout", "--detach", "HEAD")
	state, err = resumePrimaryRepository(task, workDir)
	if err != nil || state == nil || state.BranchName != "" {
		t.Fatalf("detached HEAD resume=%+v error=%v", state, err)
	}
}

func TestResumePrimaryRepositoryContextReturnsPromptlyWhenCanceled(t *testing.T) {
	envRoot, workDir, source, task := newPrimaryRepositoryGitFixture(t)
	writePrimaryRepositoryFixture(t, envRoot, workDir, source)
	ctx, cancel := context.WithCancel(context.Background())
	cancel()

	started := time.Now()
	_, err := resumePrimaryRepositoryContext(ctx, task, workDir)
	if !errors.Is(err, context.Canceled) {
		t.Fatalf("resume error = %v, want context.Canceled", err)
	}
	if elapsed := time.Since(started); elapsed > time.Second {
		t.Fatalf("canceled resume took %s; Git timeout was not interrupted", elapsed)
	}
}
