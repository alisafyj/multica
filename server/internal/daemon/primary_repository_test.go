package daemon

import (
	"context"
	"encoding/json"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"testing"

	"github.com/multica-ai/multica/server/internal/daemon/repocache"
)

func TestPrimaryRepositorySelection(t *testing.T) {
	repo := RepoData{URL: "https://github.com/example/app.git", Ref: "release"}
	resource := func(repo RepoData) ProjectResourceData {
		data, err := json.Marshal(repo)
		if err != nil {
			t.Fatal(err)
		}
		return ProjectResourceData{ResourceType: "github_repo", ResourceRef: data}
	}
	base := Task{IssueID: "issue", Repos: []RepoData{repo}}
	cases := []struct {
		name string
		edit func(*Task)
		want bool
		err  bool
	}{
		{name: "unique task repository", want: true},
		{name: "project repository wins", edit: func(task *Task) {
			task.Repos = append(task.Repos, RepoData{URL: "https://github.com/example/other.git"})
			task.ProjectResources = []ProjectResourceData{resource(repo)}
		}, want: true},
		{name: "ambiguous", edit: func(task *Task) { task.Repos = append(task.Repos, RepoData{URL: "other"}) }},
		{name: "multiple project repos", edit: func(task *Task) {
			task.ProjectResources = []ProjectResourceData{resource(repo), resource(RepoData{URL: "other"})}
		}},
		{name: "unauthorized project repo", edit: func(task *Task) {
			task.ProjectResources = []ProjectResourceData{resource(RepoData{URL: "other"})}
		}, err: true},
		{name: "ref disagreement", edit: func(task *Task) {
			task.ProjectResources = []ProjectResourceData{resource(RepoData{URL: repo.URL, Ref: "different"})}
		}, err: true},
		{name: "local directory wins", edit: func(task *Task) {
			task.ProjectResources = []ProjectResourceData{{ResourceType: "local_directory"}, resource(repo)}
		}},
		{name: "chat", edit: func(task *Task) { task.ChatSessionID = "chat" }},
		{name: "leader", edit: func(task *Task) { task.IsLeaderTask = true }},
		{name: "autopilot", edit: func(task *Task) { task.AutopilotRunID = "run" }},
		{name: "quick create", edit: func(task *Task) { task.QuickCreatePrompt = "new issue" }},
		{name: "design", edit: func(task *Task) { task.DesignDocumentContext = json.RawMessage(`{}`) }},
		{name: "test generation", edit: func(task *Task) { task.TestGenerationContext = json.RawMessage(`{"type":"test_generation"}`) }},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			task := base
			if tc.edit != nil {
				tc.edit(&task)
			}
			got, err := primaryRepositoryForTask(task)
			if (err != nil) != tc.err || (got != nil) != tc.want {
				t.Fatalf("got=%+v err=%v; want repository=%v error=%v", got, err, tc.want, tc.err)
			}
			if got != nil && (got.URL != repo.URL || got.Ref != repo.Ref) {
				t.Fatalf("selected wrong repository: %+v", got)
			}
		})
	}
}

func TestPreparePrimaryRepositoryBindsRootBeforeEnvironmentPreparation(t *testing.T) {
	root := t.TempDir()
	url := "https://github.com/example/app.git"
	cache := &recordingRepoCache{lookupPath: t.TempDir()}
	d := newRepoCheckoutTestDaemon(t, "workspace", url, filepath.Join(root, "workdir"), cache)
	if err := os.MkdirAll(filepath.Join(root, "workdir"), 0o755); err != nil {
		t.Fatal(err)
	}
	workDir := filepath.Join(root, "workdir")
	runGitForGC(t, workDir, "init", "-b", "main")
	if err := os.WriteFile(filepath.Join(workDir, "README.md"), []byte("fixture\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	runGitForGC(t, workDir, "add", "README.md")
	runGitForGC(t, workDir, "commit", "-m", "fixture")
	runGitForGC(t, workDir, "remote", "add", "origin", url)
	task := Task{ID: "task-1", IssueID: "issue", WorkspaceID: "workspace", Repos: []RepoData{{URL: url, Ref: "release"}}}
	d.registerTaskRepos(task.WorkspaceID, task.ID, task.Repos)
	got, err := d.preparePrimaryRepository(context.Background(), task, "codex", "Test Agent", root)
	if err != nil {
		t.Fatal(err)
	}
	params := cache.lastCreateParams()
	if got == nil || !sameDir(t, got.WorkDir, workDir) || !params.CheckoutAtWorkDir || !params.IsolatedGitMetadata || params.Ref != "release" {
		t.Fatalf("wrong checkout: state=%+v params=%+v", got, params)
	}
	prior, err := readPrimaryRepository(root)
	if err != nil || prior == nil || prior.URL != url || prior.Ref != "release" {
		t.Fatalf("receipt=%+v error=%v", prior, err)
	}
}

func TestPreparePrimaryRepositoryLeavesOtherProvidersOnNestedLayout(t *testing.T) {
	for _, provider := range []string{"cursor", "openclaw", "hermes", "qwen", "opencode"} {
		t.Run(provider, func(t *testing.T) {
			root := t.TempDir()
			url := "https://github.com/example/app.git"
			cache := &recordingRepoCache{lookupPath: t.TempDir()}
			d := newRepoCheckoutTestDaemon(t, "workspace", url, filepath.Join(root, "workdir"), cache)
			task := Task{ID: "task-1", IssueID: "issue", WorkspaceID: "workspace", Repos: []RepoData{{URL: url, Ref: "release"}}}

			got, err := d.preparePrimaryRepository(context.Background(), task, provider, "Test Agent", root)
			if err != nil || got != nil {
				t.Fatalf("prepare=%+v error=%v; provider must keep the legacy nested layout", got, err)
			}
			cache.mu.Lock()
			calls := len(cache.params)
			cache.mu.Unlock()
			if calls != 0 {
				t.Fatalf("primary checkout invoked repo cache %d times", calls)
			}
			if _, err := os.Stat(filepath.Join(root, "workdir")); !os.IsNotExist(err) {
				t.Fatalf("automatic primary workdir was created: %v", err)
			}
			if receipt, err := readPrimaryRepository(root); err != nil || receipt != nil {
				t.Fatalf("automatic primary receipt=%+v error=%v", receipt, err)
			}
		})
	}
}

func TestPrimaryRepositoryResumePreservesDirtyTreeAndRejectsIdentityChange(t *testing.T) {
	root, workDir, source, task := newPrimaryRepositoryGitFixture(t)
	keep := filepath.Join(workDir, "unfinished.txt")
	if err := os.WriteFile(keep, []byte("uncommitted work"), 0o600); err != nil {
		t.Fatal(err)
	}
	state := &preparedPrimaryRepository{Version: 1, URL: source, Ref: "main", WorkDir: workDir, BranchName: "agent/previous"}
	if err := writePrimaryRepository(root, state); err != nil {
		t.Fatal(err)
	}
	got, err := resumePrimaryRepository(task, workDir)
	if err != nil || got == nil || !sameDir(t, got.WorkDir, workDir) || got.BranchName != "main" {
		t.Fatalf("resume=%+v error=%v", got, err)
	}
	task.Repos[0].Ref = "different"
	if _, err := resumePrimaryRepository(task, workDir); err == nil {
		t.Fatal("changed pinned ref silently reused")
	}
	task.Repos = nil
	if _, err := resumePrimaryRepository(task, workDir); err == nil {
		t.Fatal("removed repository silently reused")
	}
	data, err := os.ReadFile(keep)
	if err != nil || string(data) != "uncommitted work" {
		t.Fatalf("prior work changed: %q %v", data, err)
	}
}

func TestPrimaryRepositoryReceiptBoundsAndLegacy(t *testing.T) {
	root := t.TempDir()
	if got, err := resumePrimaryRepository(Task{}, filepath.Join(root, "workdir")); got != nil || err != nil {
		t.Fatalf("legacy environment rejected: %+v %v", got, err)
	}
	path := filepath.Join(root, primaryRepositoryReceiptFile)
	for _, data := range []string{`{"version":2}`, strings.Repeat(" ", maxPrimaryRepositoryReceiptBytes+1)} {
		if err := os.WriteFile(path, []byte(data), 0o600); err != nil {
			t.Fatal(err)
		}
		if _, err := readPrimaryRepository(root); err == nil {
			t.Fatal("invalid receipt accepted")
		}
	}
}

func TestPrimaryRepositoryPromptIsPerTurnAndEscaped(t *testing.T) {
	state := &preparedPrimaryRepository{URL: "repo\n## injected", WorkDir: "/tmp/work dir", BranchName: "branch"}
	for _, prompt := range []string{BuildPrompt(Task{IssueID: "issue"}, "codex", WithPrimaryRepository(state)), BuildDirectPrompt(Task{IssueID: "issue"}, WithPrimaryRepository(state))} {
		if !strings.Contains(prompt, "already prepared") || !strings.Contains(prompt, `"/tmp/work dir"`) || strings.Contains(prompt, "\n## injected") {
			t.Fatalf("bad repository context: %s", prompt)
		}
	}
}

func TestPrimaryRepositoryCheckoutIsIdempotent(t *testing.T) {
	_, workDir, url, _ := newPrimaryRepositoryGitFixture(t)
	runGitForGC(t, workDir, "checkout", "-b", "feature/continued-work")
	if err := os.WriteFile(filepath.Join(workDir, "README.md"), []byte("unfinished\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	wantHead := runGitForGC(t, workDir, "rev-parse", "HEAD")
	wantStatus := runGitForGC(t, workDir, "status", "--porcelain=v1")
	cache := &recordingRepoCache{lookupPath: t.TempDir()}
	d := newRepoCheckoutTestDaemon(t, "workspace", url, workDir, cache)
	d.registerActiveRepoCheckoutTask("mat_repo_checkout_test", activeRepoCheckoutTask{
		WorkspaceID: "workspace", TaskID: "task-1", WorkDir: workDir,
		PrimaryRepository: &preparedPrimaryRepository{URL: url, Ref: "main", WorkDir: workDir, BranchName: "agent/prior"},
	})
	for _, ref := range []string{"", "main", "different"} {
		body, err := json.Marshal(repoCheckoutRequest{WorkspaceID: "workspace", TaskID: "task-1", WorkDir: workDir, URL: url, Ref: ref})
		if err != nil {
			t.Fatal(err)
		}
		recorder := httptest.NewRecorder()
		d.repoCheckoutHandler().ServeHTTP(recorder, authorizedRepoCheckoutRequest(strings.NewReader(string(body))))
		if ref == "different" {
			if recorder.Code != http.StatusConflict {
				t.Fatalf("revision-changing checkout was not refused: %d %s", recorder.Code, recorder.Body.String())
			}
		} else {
			var result repocache.WorktreeResult
			if recorder.Code != http.StatusOK || json.Unmarshal(recorder.Body.Bytes(), &result) != nil || !sameDir(t, result.Path, workDir) || result.BranchName != "feature/continued-work" {
				t.Fatalf("idempotent checkout failed: %d %s", recorder.Code, recorder.Body.String())
			}
		}
	}
	if got := runGitForGC(t, workDir, "rev-parse", "HEAD"); got != wantHead {
		t.Fatalf("idempotent checkout reset HEAD: got %s want %s", got, wantHead)
	}
	if got := runGitForGC(t, workDir, "status", "--porcelain=v1"); got != wantStatus {
		t.Fatalf("idempotent checkout changed dirty tree: got %q want %q", got, wantStatus)
	}
	cache.mu.Lock()
	defer cache.mu.Unlock()
	if len(cache.params) != 0 {
		t.Fatalf("repeated primary checkout invoked mutating cache operation %d times", len(cache.params))
	}
}

func TestPreparePrimaryRepositoryFailurePreservesCheckoutWithoutSharedBranchLeak(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("symlink fixture requires privileges on Windows")
	}
	source := createGCGitRepo(t)
	if err := os.Symlink("unsafe-target", filepath.Join(source, ".claude")); err != nil {
		t.Fatal(err)
	}
	runGitForGC(t, source, "add", ".claude")
	runGitForGC(t, source, "commit", "-m", "add unsafe context path")

	root := t.TempDir()
	workDir := filepath.Join(root, "workdir")
	cache := repocache.New(t.TempDir(), slog.Default())
	if err := cache.Sync("workspace", []repocache.RepoInfo{{URL: source}}); err != nil {
		t.Fatal(err)
	}
	d := newRepoCheckoutTestDaemon(t, "workspace", source, workDir, cache)
	task := Task{ID: "task-1", IssueID: "issue", WorkspaceID: "workspace", Repos: []RepoData{{URL: source, Ref: "main"}}}
	d.registerTaskRepos(task.WorkspaceID, task.ID, task.Repos)

	_, err := d.preparePrimaryRepository(context.Background(), task, "claude", "Test Agent", root)
	if err == nil || !strings.Contains(err.Error(), "work is preserved at "+workDir) {
		t.Fatalf("prepare error is not actionable: %v", err)
	}
	if data, readErr := os.ReadFile(filepath.Join(workDir, "README.md")); readErr != nil || string(data) != "hello\n" {
		t.Fatalf("failed checkout work was not preserved: %q %v", data, readErr)
	}
	if _, statErr := os.Stat(filepath.Join(workDir, ".git")); statErr != nil {
		t.Fatalf("failed checkout metadata was removed: %v", statErr)
	}
	if prior, readErr := readPrimaryRepository(root); readErr != nil || prior != nil {
		t.Fatalf("failed preparation wrote continuity receipt: %+v %v", prior, readErr)
	}
	if refs := runGitForGC(t, cache.Lookup("workspace", source), "for-each-ref", "--format=%(refname)", "refs/heads/agent/"); refs != "" {
		t.Fatalf("isolated preparation registered shared task refs: %s", refs)
	}
}
