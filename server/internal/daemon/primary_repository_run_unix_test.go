//go:build !windows

package daemon

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/multica-ai/multica/server/internal/daemon/repocache"
)

func TestRunTaskPrimaryRepositoryPreservesRulesAndResumesCode(t *testing.T) {
	source := createGCGitRepo(t)
	rules := "Repository-specific rules.\n"
	for name, data := range map[string]string{
		"CLAUDE.md": rules, "AGENTS.md": rules,
		"go.mod": "module fixture.invalid/primary\n\ngo 1.24\n", "go.sum": "fixture input\n",
		".agent_context/issue_context.md": rules,
		".claude/settings.json":           `{}`, ".claude/skills/project/SKILL.md": "---\nname: project\ndescription: project fixture\n---\nProject rules.\n",
	} {
		path := filepath.Join(source, name)
		if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(path, []byte(data), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	runGitForGC(t, source, "add", ".")
	runGitForGC(t, source, "commit", "-m", "repository context")
	d, argsFile, cleanup := newLeaderReuseTestDaemon(t)
	defer cleanup()
	task := leaderReuseTestTask("task-primary-first")
	task.IsLeaderTask = false
	task.Repos = []RepoData{{URL: source, Ref: "main"}}
	capture := filepath.Join(t.TempDir(), "provider-cwd")
	task.Agent.CustomEnv = map[string]string{"CAPTURE_FILE": capture}
	providerPath := d.cfg.Agents["claude"].Path
	script, err := os.ReadFile(providerPath)
	if err != nil {
		t.Fatal(err)
	}
	probe := "pwd > \"$CAPTURE_FILE\"\ntest -f CLAUDE.md && test -f AGENTS.md && test -f .claude/settings.json && test -f .claude/skills/project/SKILL.md || exit 17\nif [ \"$CHECK_SETUP\" = 1 ]; then test -f \"$GOMODCACHE/setup-ready\" || exit 18; fi\n"
	if err := os.WriteFile(providerPath, []byte(strings.Replace(string(script), "#!/bin/sh\n", "#!/bin/sh\n"+probe, 1)), 0o755); err != nil {
		t.Fatal(err)
	}
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if strings.HasSuffix(r.URL.Path, "/repos") {
			if err := json.NewEncoder(w).Encode(WorkspaceReposResponse{WorkspaceID: task.WorkspaceID, Repos: task.Repos}); err != nil {
				t.Error(err)
			}
			return
		}
		w.WriteHeader(http.StatusOK)
	}))
	defer srv.Close()
	d.client = NewClient(srv.URL)
	d.cfg.ServerBaseURL = srv.URL
	d.workspaces[task.WorkspaceID] = newWorkspaceState(task.WorkspaceID, nil, "", task.Repos, nil)
	cache := repocache.New(t.TempDir(), d.logger)
	if err := cache.Sync(task.WorkspaceID, []repocache.RepoInfo{{URL: source}}); err != nil {
		t.Fatal(err)
	}
	d.repoCache = cache
	first, err := d.runTask(context.Background(), task, "claude", 0, d.logger)
	if err != nil || first.Status != "completed" {
		t.Fatalf("first run=%+v error=%v", first, err)
	}
	actualCwd, err := os.ReadFile(capture)
	if err != nil || !sameDir(t, strings.TrimSpace(string(actualCwd)), first.WorkDir) {
		t.Fatalf("provider did not launch in the primary checkout: %q %v", actualCwd, err)
	}
	firstArgs, err := os.ReadFile(argsFile)
	if err != nil || !strings.Contains(string(firstArgs), "--setting-sources\nuser\n") {
		t.Fatalf("new managed primary did not default to restricted settings: %s %v", firstArgs, err)
	}
	for _, name := range []string{"CLAUDE.md", "AGENTS.md", ".agent_context/issue_context.md"} {
		data, err := os.ReadFile(filepath.Join(first.WorkDir, name))
		if err != nil || string(data) != rules {
			t.Fatalf("tracked repository instructions %s were modified: %q %v", name, data, err)
		}
	}
	for _, name := range []string{".claude/settings.json", ".claude/skills/project/SKILL.md"} {
		if _, err := os.Stat(filepath.Join(first.WorkDir, name)); err != nil {
			t.Fatalf("repository context missing at startup cwd: %s: %v", name, err)
		}
	}
	if changes := runGitForGC(t, first.WorkDir, "status", "--porcelain"); changes != "" {
		t.Fatalf("platform context polluted the primary checkout: %s", changes)
	}
	newRule := filepath.Join(first.WorkDir, ".claude", "new-project-rule.md")
	if err := os.WriteFile(newRule, []byte("User-owned new rule"), 0o644); err != nil {
		t.Fatal(err)
	}
	if changes := runGitForGC(t, first.WorkDir, "status", "--porcelain"); !strings.Contains(changes, ".claude/new-project-rule.md") {
		t.Fatalf("new repository-owned configuration was ignored: %s", changes)
	}
	if head := runGitForGC(t, first.WorkDir, "rev-parse", "HEAD"); head != runGitForGC(t, source, "rev-parse", "HEAD") {
		t.Fatalf("primary checkout revision changed: %s", head)
	}
	readme := filepath.Join(first.WorkDir, "README.md")
	if err := os.WriteFile(readme, []byte("unfinished changes"), 0o644); err != nil {
		t.Fatal(err)
	}
	task.ID = "task-primary-next"
	task.PriorSessionID, task.PriorWorkDir = first.SessionID, first.WorkDir
	toolDir := t.TempDir()
	fakeGo := "#!/bin/sh\nif [ \"$1\" = version ]; then printf 'go version go1.fixture\\n'; exit 0; fi\n[ \"$1\" = mod ] && [ \"$2\" = download ] || exit 19\nmkdir -p \"$GOMODCACHE\"\nprintf ready > \"$GOMODCACHE/setup-ready\"\n"
	if err := os.WriteFile(filepath.Join(toolDir, "go"), []byte(fakeGo), 0o755); err != nil {
		t.Fatal(err)
	}
	t.Setenv("PATH", toolDir+string(os.PathListSeparator)+os.Getenv("PATH"))
	task.Agent.CustomEnv["CHECK_SETUP"] = "1"
	policyRef, err := json.Marshal(map[string]any{"url": source, "ref": "main", "configuration_policy": "trusted",
		"setup": map[string]any{"steps": []string{"go_mod_download"}, "timeout_seconds": 10}})
	if err != nil {
		t.Fatal(err)
	}
	task.ProjectResources = []ProjectResourceData{{ResourceType: "github_repo", ResourceRef: policyRef}}
	second, err := d.runTask(context.Background(), task, "claude", 0, d.logger)
	if err != nil || second.Status != "completed" || !sameDir(t, first.WorkDir, second.WorkDir) {
		t.Fatalf("continuation=%+v error=%v", second, err)
	}
	if data, err := os.ReadFile(readme); err != nil || string(data) != "unfinished changes" {
		t.Fatalf("resumed code changed: %q %v", data, err)
	}
	if data, err := os.ReadFile(filepath.Join(second.WorkDir, ".agent_context", "issue_context.md")); err != nil || string(data) != rules {
		t.Fatalf("resumed task overwrote repository-owned context: %q %v", data, err)
	}
	args, err := os.ReadFile(argsFile)
	if err != nil || !strings.Contains(string(args), "--resume\nsession-leader-reuse\n") {
		t.Fatalf("provider session did not resume: %s %v", args, err)
	}
	if !strings.Contains(string(args), "--setting-sources\nuser,project,local\n") {
		t.Fatalf("explicit project trust did not reach resumed provider: %s", args)
	}
	if changes := runGitForGC(t, source, "status", "--porcelain"); changes != "" {
		t.Fatalf("source repository was modified: %s", changes)
	}
}
