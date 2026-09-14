package daemon

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"io/fs"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
	"testing"
	"time"
)

func TestPrepareRepositorySetupRealGoColdWarm(t *testing.T) {
	if os.Getenv("MULTICA_TEST_REAL_REPOSITORY_SETUP") != "1" {
		t.Skip("set MULTICA_TEST_REAL_REPOSITORY_SETUP=1 to run the real Go setup smoke test")
	}
	gitPath, err := exec.LookPath("git")
	if err != nil {
		t.Fatal("real repository setup requires git on PATH")
	}
	if _, err := exec.LookPath("go"); err != nil {
		t.Fatal("real repository setup requires go on PATH")
	}

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Minute)
	defer cancel()
	t.Setenv("HOME", t.TempDir())
	workspacesRoot := t.TempDir()
	registerRepositorySetupSharedCleanup(t, workspacesRoot)
	repositoryURL := "http://127.0.0.1:18883/T2.git"
	cloneEnv := []string{
		"GIT_CONFIG_NOSYSTEM=1",
		"GIT_CONFIG_GLOBAL=" + os.DevNull,
		"GIT_TERMINAL_PROMPT=0",
		"NO_PROXY=127.0.0.1,localhost",
	}
	cloneFixture := func(name string) (string, string) {
		t.Helper()
		envRoot := filepath.Join(workspacesRoot, name)
		workDir := filepath.Join(envRoot, "workdir")
		if err := os.MkdirAll(envRoot, 0o700); err != nil {
			t.Fatal(err)
		}
		clone := exec.CommandContext(ctx, gitPath,
			"-c", "credential.helper=",
			"-c", "core.askPass=",
			"clone", "--branch", "main", "--single-branch", repositoryURL, workDir,
		)
		clone.Env = cloneEnv
		if output, err := clone.CombinedOutput(); err != nil {
			t.Fatalf("clone frozen setup fixture: %v: %s", err, output)
		}
		return envRoot, workDir
	}
	coldEnvRoot, coldWorkDir := cloneFixture("cold-env")
	warmEnvRoot, warmWorkDir := cloneFixture("warm-env")

	gitOutput := func(workDir string, args ...string) string {
		t.Helper()
		command := exec.CommandContext(ctx, gitPath, append([]string{"-C", workDir}, args...)...)
		command.Env = cloneEnv
		output, err := command.CombinedOutput()
		if err != nil {
			t.Fatalf("git %s: %v: %s", strings.Join(args, " "), err, output)
		}
		return strings.TrimSpace(string(output))
	}
	beforeHead := gitOutput(coldWorkDir, "rev-parse", "HEAD")
	warmBeforeHead := gitOutput(warmWorkDir, "rev-parse", "HEAD")
	beforeStatus := gitOutput(coldWorkDir, "status", "--porcelain=v1", "--untracked-files=all")
	warmBeforeStatus := gitOutput(warmWorkDir, "status", "--porcelain=v1", "--untracked-files=all")
	if beforeStatus != "" || warmBeforeStatus != "" {
		t.Fatalf("frozen setup fixtures are not clean before setup: cold=%q warm=%q", beforeStatus, warmBeforeStatus)
	}
	beforeGoMod, err := os.ReadFile(filepath.Join(coldWorkDir, "go.mod"))
	if err != nil {
		t.Fatal(err)
	}
	beforeGoSum, err := os.ReadFile(filepath.Join(coldWorkDir, "go.sum"))
	if err != nil {
		t.Fatal(err)
	}

	resourceRef, err := json.Marshal(map[string]any{
		"url":                  repositoryURL,
		"ref":                  "main",
		"configuration_policy": "trusted",
		"setup": map[string]any{
			"steps":           []string{"go_mod_download"},
			"timeout_seconds": 300,
		},
	})
	if err != nil {
		t.Fatal(err)
	}
	explicitSetupEnv := make(map[string]string)
	for _, key := range []string{"HTTP_PROXY", "HTTPS_PROXY", "ALL_PROXY", "NO_PROXY", "GOPROXY", "GONOPROXY", "GOSUMDB"} {
		if value := os.Getenv(key); value != "" {
			explicitSetupEnv[key] = value
		}
	}
	explicitSetupEnv["GOCACHE"] = filepath.Join(workspacesRoot, "stale-explicit-go-build")
	explicitSetupEnv["GOMODCACHE"] = filepath.Join(workspacesRoot, "stale-explicit-go-mod")
	newTask := func(id string) Task {
		return Task{
			ID: id, AgentID: "real-setup-agent", RuntimeID: "real-setup-runtime", WorkspaceID: "real-setup-workspace", IssueID: "real-setup-issue",
			Agent:            &AgentData{CustomEnv: explicitSetupEnv},
			Repos:            []RepoData{{URL: repositoryURL, Ref: "main"}},
			ProjectResources: []ProjectResourceData{{ID: "real-setup-resource", ResourceType: "github_repo", ResourceRef: resourceRef}},
		}
	}

	coldStarted := time.Now()
	cold, err := prepareRepositorySetupWithSharedCache(ctx, newTask("real-setup-cold"), "claude", coldWorkDir, coldEnvRoot, workspacesRoot)
	coldDuration := time.Since(coldStarted)
	if err != nil {
		t.Fatal(err)
	}
	warmStarted := time.Now()
	warm, err := prepareRepositorySetupWithSharedCache(ctx, newTask("real-setup-warm"), "claude", warmWorkDir, warmEnvRoot, workspacesRoot)
	warmDuration := time.Since(warmStarted)
	if err != nil {
		t.Fatal(err)
	}
	t.Logf("real repository setup cold=%s warm=%s", coldDuration, warmDuration)

	if cold.CacheHit || !warm.CacheHit || cold.CacheKey == "" || warm.CacheKey != cold.CacheKey {
		t.Fatalf("unexpected cold/warm cache results: cold=%#v warm=%#v", cold, warm)
	}
	canonicalWorkspacesRoot, err := filepath.EvalSymlinks(workspacesRoot)
	if err != nil {
		t.Fatal(err)
	}
	cacheRoot := filepath.Join(canonicalWorkspacesRoot, ".setup-cache")
	for name, result := range map[string]repositorySetupResult{"cold": cold, "warm": warm} {
		for key, path := range result.Env {
			if rel, relErr := filepath.Rel(coldWorkDir, path); relErr == nil && filepath.IsLocal(rel) {
				t.Fatalf("%s %s cache is inside source checkout: %q", name, key, path)
			}
		}
	}
	if !strings.HasPrefix(filepath.Join(cacheRoot, "artifacts", "v2", cold.CacheKey), cacheRoot+string(os.PathSeparator)) {
		t.Fatal("shared artifact escaped workspace cache root")
	}
	if cold.Env["GOMODCACHE"] == warm.Env["GOMODCACHE"] ||
		!strings.Contains(cold.Env["GOMODCACHE"], "real-setup-cold") ||
		!strings.Contains(warm.Env["GOMODCACHE"], "real-setup-warm") {
		t.Fatalf("task-private Go module caches are not isolated: cold=%q warm=%q", cold.Env["GOMODCACHE"], warm.Env["GOMODCACHE"])
	}
	if afterHead := gitOutput(coldWorkDir, "rev-parse", "HEAD"); afterHead != beforeHead {
		t.Fatalf("repository HEAD changed during setup: before=%s after=%s", beforeHead, afterHead)
	}
	if afterStatus := gitOutput(coldWorkDir, "status", "--porcelain=v1", "--untracked-files=all"); afterStatus != beforeStatus {
		t.Fatalf("repository worktree changed during setup: before=%q after=%q", beforeStatus, afterStatus)
	}
	if afterHead := gitOutput(warmWorkDir, "rev-parse", "HEAD"); afterHead != warmBeforeHead {
		t.Fatalf("warm repository HEAD changed during setup: before=%s after=%s", warmBeforeHead, afterHead)
	}
	if afterStatus := gitOutput(warmWorkDir, "status", "--porcelain=v1", "--untracked-files=all"); afterStatus != warmBeforeStatus {
		t.Fatalf("warm repository worktree changed during setup: before=%q after=%q", warmBeforeStatus, afterStatus)
	}
	afterGoMod, err := os.ReadFile(filepath.Join(coldWorkDir, "go.mod"))
	if err != nil {
		t.Fatal(err)
	}
	afterGoSum, err := os.ReadFile(filepath.Join(coldWorkDir, "go.sum"))
	if err != nil {
		t.Fatal(err)
	}
	if !bytes.Equal(afterGoMod, beforeGoMod) || !bytes.Equal(afterGoSum, beforeGoSum) {
		t.Fatal("repository Go module files changed during setup")
	}
	artifactGoMod := filepath.Join(cacheRoot, "artifacts", "v2", cold.CacheKey, "go-mod")
	var artifactFile string
	if err := filepath.WalkDir(artifactGoMod, func(path string, entry os.DirEntry, walkErr error) error {
		if walkErr != nil || artifactFile != "" || entry.IsDir() || entry.Type()&os.ModeSymlink != 0 {
			return walkErr
		}
		info, err := entry.Info()
		if err == nil && info.Mode().IsRegular() && info.Size() > 0 && info.Size() <= 1<<20 {
			artifactFile = path
		}
		return nil
	}); err != nil {
		t.Fatal(err)
	}
	if artifactFile == "" {
		t.Fatal("real shared Go artifact has no bounded regular file for mutation-isolation proof")
	}
	rel, err := filepath.Rel(artifactGoMod, artifactFile)
	if err != nil {
		t.Fatal(err)
	}
	coldFile := filepath.Join(cold.Env["GOMODCACHE"], rel)
	warmFile := filepath.Join(warm.Env["GOMODCACHE"], rel)
	artifactBefore, err := os.ReadFile(artifactFile)
	if err != nil {
		t.Fatal(err)
	}
	coldBefore, err := os.ReadFile(coldFile)
	if err != nil {
		t.Fatal(err)
	}
	if err := os.Chmod(warmFile, 0o600); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(warmFile, []byte("task-private mutation"), 0o600); err != nil {
		t.Fatal(err)
	}
	artifactAfter, err := os.ReadFile(artifactFile)
	if err != nil {
		t.Fatal(err)
	}
	coldAfter, err := os.ReadFile(coldFile)
	if err != nil {
		t.Fatal(err)
	}
	if !bytes.Equal(artifactBefore, artifactAfter) || !bytes.Equal(coldBefore, coldAfter) {
		t.Fatal("mutating the real warm task cache changed the shared artifact or cold task cache")
	}
}

func repositorySetupTask(t *testing.T, policy string, timeout int, steps ...string) Task {
	t.Helper()
	raw, err := json.Marshal(map[string]any{
		"url":                  "https://github.com/acme/app.git",
		"ref":                  "release",
		"configuration_policy": policy,
		"setup": map[string]any{
			"steps":           steps,
			"timeout_seconds": timeout,
		},
	})
	if err != nil {
		t.Fatal(err)
	}
	return Task{
		ID: "task-1", AgentID: "agent-1", RuntimeID: "runtime-1", WorkspaceID: "workspace-1", IssueID: "issue-1",
		Repos:            []RepoData{{URL: "https://github.com/acme/app.git", Ref: "release"}},
		ProjectResources: []ProjectResourceData{{ID: "resource-1", ResourceType: "github_repo", ResourceRef: raw}},
	}
}

func writeRepositorySetupFiles(t *testing.T, workDir string) {
	t.Helper()
	for name, content := range map[string]string{
		"go.mod": "module example.com/app\n\ngo 1.26\n", "go.sum": "example.com/mod v1.0.0 h1:test\n",
		"package.json": `{"name":"app","packageManager":"pnpm@10.0.0"}`, "pnpm-lock.yaml": "lockfileVersion: '9.0'\n",
	} {
		if err := os.WriteFile(filepath.Join(workDir, name), []byte(content), 0o600); err != nil {
			t.Fatal(err)
		}
	}
}

func registerRepositorySetupSharedCleanup(t *testing.T, workspacesRoot string) {
	t.Helper()
	t.Cleanup(func() {
		ctx, cancel := context.WithTimeout(context.Background(), time.Second)
		defer cancel()
		_ = makeRepositorySetupMutableCacheRemovable(ctx, filepath.Join(workspacesRoot, ".setup-cache"))
	})
}

type fakeRepositorySetupTools struct {
	bin     string
	logPath string
}

func (f fakeRepositorySetupTools) setMode(t *testing.T, mode string) {
	t.Helper()
	if err := os.WriteFile(filepath.Join(f.bin, "mode"), []byte(mode), 0o600); err != nil {
		t.Fatal(err)
	}
}

func (f fakeRepositorySetupTools) setVersion(t *testing.T, version string) {
	t.Helper()
	if err := os.WriteFile(filepath.Join(f.bin, "version"), []byte(version), 0o600); err != nil {
		t.Fatal(err)
	}
}

func installFakeRepositorySetupTools(t *testing.T) fakeRepositorySetupTools {
	t.Helper()
	if runtime.GOOS == "windows" {
		t.Skip("fake shell tools are covered on unix")
	}
	bin := t.TempDir()
	logPath := filepath.Join(t.TempDir(), "commands.log")
	if err := os.WriteFile(filepath.Join(bin, "log-path"), []byte(logPath), 0o600); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(bin, "version"), []byte("1.0"), 0o600); err != nil {
		t.Fatal(err)
	}
	originalPath := os.Getenv("PATH")
	script := `#!/bin/sh
bin_dir=$(dirname "$0")
log_path=$(cat "$bin_dir/log-path")
version=$(cat "$bin_dir/version")
mode=$(cat "$bin_dir/mode" 2>/dev/null || true)
case "$1" in
  version|--version) echo "fake-$0-$version"; exit 0 ;;
esac
printf '%s %s\n' "$(basename "$0")" "$*" >> "$log_path"
printf 'env %s %s %s %s\n' "$HTTP_PROXY" "$NPM_CONFIG_REGISTRY" "$UNSAFE_INHERITED" "$GOTOOLCHAIN" >> "$log_path"
case "$mode" in
  fail) exit 17 ;;
  wait) exec sleep 5 ;;
  symlink)
    mkdir -p "${GOMODCACHE:-$PNPM_STORE_DIR}/payload"
    ln -s /tmp "${GOMODCACHE:-$PNPM_STORE_DIR}/payload/escape"
    exit 0
    ;;
esac
mkdir -p "${GOMODCACHE:-$PNPM_STORE_DIR}/payload"
printf '%s' "$0" > "${GOMODCACHE:-$PNPM_STORE_DIR}/payload/tool"
if [ "$mode" = "readonly" ]; then
  chmod 500 "${GOMODCACHE:-$PNPM_STORE_DIR}/payload"
fi
`
	for _, name := range []string{"go", "pnpm"} {
		if err := os.WriteFile(filepath.Join(bin, name), []byte(script), 0o700); err != nil {
			t.Fatal(err)
		}
	}
	t.Setenv("PATH", bin+string(os.PathListSeparator)+originalPath)
	return fakeRepositorySetupTools{bin: bin, logPath: logPath}
}

func TestPrepareRepositorySetupLegacyRestrictedAndProviderGates(t *testing.T) {
	root := t.TempDir()
	workDir := filepath.Join(root, "workdir")
	if err := os.Mkdir(workDir, 0o700); err != nil {
		t.Fatal(err)
	}

	legacy, err := prepareRepositorySetup(context.Background(), Task{}, "claude", workDir, root)
	if err != nil || len(legacy.Env) != 0 {
		t.Fatalf("legacy = %#v, err=%v", legacy, err)
	}
	if _, err := prepareRepositorySetup(context.Background(), repositorySetupTask(t, "restricted", 60, "go_mod_download"), "claude", workDir, root); err == nil || !strings.Contains(err.Error(), "trusted") {
		t.Fatalf("restricted setup error = %v", err)
	}
	if _, err := prepareRepositorySetup(context.Background(), repositorySetupTask(t, "trusted", 60, "go_mod_download"), "other", workDir, root); err == nil || !strings.Contains(err.Error(), "provider") {
		t.Fatalf("unsupported provider error = %v", err)
	}
}

func TestPrepareRepositorySetupColdWarmAndCacheIsolation(t *testing.T) {
	tools := installFakeRepositorySetupTools(t)
	root := t.TempDir()
	workDir := filepath.Join(root, "workdir")
	if err := os.Mkdir(workDir, 0o700); err != nil {
		t.Fatal(err)
	}
	writeRepositorySetupFiles(t, workDir)
	task := repositorySetupTask(t, "trusted", 60, "go_mod_download", "pnpm_install")
	task.Agent = &AgentData{CustomEnv: map[string]string{
		"HTTP_PROXY": "http://proxy.test", "NPM_CONFIG_REGISTRY": "https://registry.test", "PATH": "/unsafe",
		"GOCACHE": "/untrusted/go-build", "GOMODCACHE": "/untrusted/go-mod", "NPM_CONFIG_STORE_DIR": "/untrusted/pnpm",
	}}
	t.Setenv("UNSAFE_INHERITED", "must-not-pass")

	cold, err := prepareRepositorySetup(context.Background(), task, "claude", workDir, root)
	if err != nil {
		t.Fatal(err)
	}
	if cold.CacheHit || len(cold.Env) != 4 || len(cold.SidecarExcludes) != 1 {
		t.Fatalf("cold result = %#v", cold)
	}
	canonicalRoot, err := filepath.EvalSymlinks(root)
	if err != nil {
		t.Fatal(err)
	}
	if !strings.HasPrefix(cold.Env["GOCACHE"], canonicalRoot+string(os.PathSeparator)) || !strings.Contains(cold.Env["PNPM_STORE_DIR"], task.ID) {
		t.Fatalf("private env = %#v", cold.Env)
	}
	if cold.Env["GOCACHE"] == task.Agent.CustomEnv["GOCACHE"] || cold.Env["GOMODCACHE"] == task.Agent.CustomEnv["GOMODCACHE"] || cold.Env["NPM_CONFIG_STORE_DIR"] == task.Agent.CustomEnv["NPM_CONFIG_STORE_DIR"] {
		t.Fatalf("custom env overrode fixed setup cache paths: %#v", cold.Env)
	}
	if cold.SidecarExcludes[0] != filepath.Join(canonicalRoot, ".multica", "setup-cache") {
		t.Fatalf("sidecar excludes = %v", cold.SidecarExcludes)
	}
	if rel, err := filepath.Rel(workDir, cold.SidecarExcludes[0]); err != nil || filepath.IsLocal(rel) {
		t.Fatalf("setup cache must stay outside workdir: rel=%q err=%v", rel, err)
	}
	if _, err := os.Stat(filepath.Join(workDir, ".gitignore")); !errors.Is(err, os.ErrNotExist) {
		t.Fatalf("repository setup must not create or edit .gitignore: %v", err)
	}
	warm, err := prepareRepositorySetup(context.Background(), task, "codex", workDir, root)
	if err != nil {
		t.Fatal(err)
	}
	if !warm.CacheHit || warm.CacheKey != cold.CacheKey {
		t.Fatalf("warm = %#v, cold = %#v", warm, cold)
	}
	artifactFile := filepath.Join(root, ".multica", "setup-cache", "artifacts", cold.CacheKey, "go-mod", "payload", "tool")
	mutableFile := filepath.Join(warm.Env["GOMODCACHE"], "payload", "tool")
	artifactInfo, err := os.Stat(artifactFile)
	if err != nil {
		t.Fatal(err)
	}
	mutableInfo, err := os.Stat(mutableFile)
	if err != nil {
		t.Fatal(err)
	}
	if os.SameFile(artifactInfo, mutableInfo) {
		t.Fatal("warm cache restore used a hardlink")
	}
	commands, err := os.ReadFile(tools.logPath)
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(commands), "go mod download") || !strings.Contains(string(commands), "pnpm install --frozen-lockfile --ignore-scripts --ignore-pnpmfile") {
		t.Fatalf("commands = %s", commands)
	}
	if !strings.Contains(string(commands), "env http://proxy.test https://registry.test  local") || strings.Contains(string(commands), "must-not-pass") {
		t.Fatalf("sanitized setup environment = %s", commands)
	}

	for _, mutate := range []func(*Task){
		func(changed *Task) { changed.WorkspaceID = "workspace-2" },
		func(changed *Task) { changed.AgentID = "agent-2" },
		func(changed *Task) { changed.RuntimeID = "runtime-2" },
	} {
		changed := task
		mutate(&changed)
		isolated, err := prepareRepositorySetup(context.Background(), changed, "claude", workDir, root)
		if err != nil {
			t.Fatal(err)
		}
		if isolated.CacheKey == cold.CacheKey || isolated.Env["GOCACHE"] == cold.Env["GOCACHE"] {
			t.Fatal("tenant/runtime isolation did not change cache identity")
		}
	}
	differentRepo := repositorySetupTask(t, "trusted", 60, "go_mod_download", "pnpm_install")
	differentRepo.Agent = task.Agent
	differentRepo.Repos[0].URL = "https://github.com/acme/other.git"
	var differentRef map[string]any
	if err := json.Unmarshal(differentRepo.ProjectResources[0].ResourceRef, &differentRef); err != nil {
		t.Fatal(err)
	}
	differentRef["url"] = differentRepo.Repos[0].URL
	differentRepo.ProjectResources[0].ResourceRef, err = json.Marshal(differentRef)
	if err != nil {
		t.Fatal(err)
	}
	repoChanged, err := prepareRepositorySetup(context.Background(), differentRepo, "claude", workDir, root)
	if err != nil {
		t.Fatal(err)
	}
	if repoChanged.CacheKey == cold.CacheKey {
		t.Fatal("repository identity did not change cache key")
	}

	goPath, err := exec.LookPath("go")
	if err != nil {
		t.Fatal(err)
	}
	toolBytes, err := os.ReadFile(goPath)
	if err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(goPath, append(toolBytes, []byte("\n# changed binary bytes\n")...), 0o700); err != nil {
		t.Fatal(err)
	}
	binaryChanged, err := prepareRepositorySetup(context.Background(), task, "claude", workDir, root)
	if err != nil {
		t.Fatal(err)
	}
	if binaryChanged.CacheKey == cold.CacheKey {
		t.Fatal("toolchain executable content did not change cache key")
	}
	if err := os.WriteFile(filepath.Join(workDir, "go.sum"), []byte("changed\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	lockChanged, err := prepareRepositorySetup(context.Background(), task, "claude", workDir, root)
	if err != nil {
		t.Fatal(err)
	}
	if lockChanged.CacheKey == binaryChanged.CacheKey {
		t.Fatal("lockfile change did not change cache key")
	}
	tools.setVersion(t, "2.0")
	toolChanged, err := prepareRepositorySetup(context.Background(), task, "claude", workDir, root)
	if err != nil {
		t.Fatal(err)
	}
	if toolChanged.CacheKey == lockChanged.CacheKey {
		t.Fatal("toolchain version change did not change cache key")
	}
}

func TestRepositorySetupArtifactLockRespectsContext(t *testing.T) {
	release, err := lockRepositorySetupArtifact(context.Background(), "blocked-artifact")
	if err != nil {
		t.Fatal(err)
	}
	defer release()
	ctx, cancel := context.WithTimeout(context.Background(), 50*time.Millisecond)
	defer cancel()
	started := time.Now()
	if _, err := lockRepositorySetupArtifact(ctx, "blocked-artifact"); !errors.Is(err, context.DeadlineExceeded) {
		t.Fatalf("lock error = %v", err)
	}
	if time.Since(started) > time.Second {
		t.Fatal("cache lock ignored its context deadline")
	}
}

func TestCopyRepositorySetupTreeRespectsCancellation(t *testing.T) {
	source := filepath.Join(t.TempDir(), "source")
	destination := filepath.Join(t.TempDir(), "destination")
	if err := os.Mkdir(source, 0o700); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(source, "cache.bin"), []byte("cache"), 0o600); err != nil {
		t.Fatal(err)
	}
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	if err := copyRepositorySetupTree(ctx, source, destination); !errors.Is(err, context.Canceled) {
		t.Fatalf("copy error = %v", err)
	}
	if _, err := os.Stat(filepath.Join(destination, "cache.bin")); !errors.Is(err, os.ErrNotExist) {
		t.Fatalf("cancelled copy left a published file: %v", err)
	}
}

func TestPrepareRepositorySetupConcurrentCallsHaveSinglePublisher(t *testing.T) {
	installFakeRepositorySetupTools(t)
	root := t.TempDir()
	workDir := filepath.Join(root, "workdir")
	if err := os.Mkdir(workDir, 0o700); err != nil {
		t.Fatal(err)
	}
	writeRepositorySetupFiles(t, workDir)
	task := repositorySetupTask(t, "trusted", 60, "go_mod_download")

	type outcome struct {
		result repositorySetupResult
		err    error
	}
	start := make(chan struct{})
	outcomes := make(chan outcome, 2)
	for range 2 {
		go func() {
			<-start
			result, err := prepareRepositorySetup(context.Background(), task, "claude", workDir, root)
			outcomes <- outcome{result: result, err: err}
		}()
	}
	close(start)
	hits := 0
	for range 2 {
		outcome := <-outcomes
		if outcome.err != nil {
			t.Fatal(outcome.err)
		}
		if outcome.result.CacheHit {
			hits++
		}
	}
	if hits != 1 {
		t.Fatalf("cache hits = %d, want exactly one publisher and one restore", hits)
	}
	entries, err := os.ReadDir(filepath.Join(root, ".multica", "setup-cache", "artifacts"))
	if err != nil {
		t.Fatal(err)
	}
	if len(entries) != 1 {
		t.Fatalf("published artifacts = %d, want 1", len(entries))
	}
}

func TestPrepareRepositorySetupLeavesPrivateCacheRemovable(t *testing.T) {
	tools := installFakeRepositorySetupTools(t)
	tools.setMode(t, "readonly")
	root := t.TempDir()
	workDir := filepath.Join(root, "workdir")
	if err := os.Mkdir(workDir, 0o700); err != nil {
		t.Fatal(err)
	}
	writeRepositorySetupFiles(t, workDir)
	task := repositorySetupTask(t, "trusted", 60, "go_mod_download")
	if _, err := prepareRepositorySetup(context.Background(), task, "claude", workDir, root); err != nil {
		t.Fatal(err)
	}
	if err := os.RemoveAll(root); err != nil {
		t.Fatalf("remove environment root after setup: %v", err)
	}
}

func TestPrepareRepositorySetupMissingFilesFailureCancellationAndTimeoutDoNotPublish(t *testing.T) {
	tools := installFakeRepositorySetupTools(t)
	for _, tc := range []struct {
		name    string
		mode    string
		timeout int
		cancel  bool
	}{
		{name: "failed", mode: "fail", timeout: 60},
		{name: "cancelled", mode: "wait", timeout: 60, cancel: true},
		{name: "timeout", mode: "wait", timeout: 1},
	} {
		t.Run(tc.name, func(t *testing.T) {
			tools.setMode(t, tc.mode)
			root := t.TempDir()
			workDir := filepath.Join(root, "workdir")
			if err := os.Mkdir(workDir, 0o700); err != nil {
				t.Fatal(err)
			}
			writeRepositorySetupFiles(t, workDir)
			ctx := context.Background()
			if tc.cancel {
				var cancel context.CancelFunc
				ctx, cancel = context.WithCancel(ctx)
				time.AfterFunc(50*time.Millisecond, cancel)
			}
			task := repositorySetupTask(t, "trusted", tc.timeout, "go_mod_download")
			_, err := prepareRepositorySetup(ctx, task, "claude", workDir, root)
			if err == nil {
				t.Fatal("setup unexpectedly succeeded")
			}
			artifacts, readErr := os.ReadDir(filepath.Join(root, ".multica", "setup-cache", "artifacts"))
			if readErr == nil && len(artifacts) != 0 {
				t.Fatalf("failed setup published %d artifacts", len(artifacts))
			}
		})
	}

	root := t.TempDir()
	workDir := filepath.Join(root, "workdir")
	if err := os.Mkdir(workDir, 0o700); err != nil {
		t.Fatal(err)
	}
	if _, err := prepareRepositorySetup(context.Background(), repositorySetupTask(t, "trusted", 60, "pnpm_install"), "claude", workDir, root); err == nil || !strings.Contains(err.Error(), "package.json") {
		t.Fatalf("missing config error = %v", err)
	}
}

func TestPrepareRepositorySetupRejectsSymlinkedInputs(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("symlink fixture requires unix permissions")
	}
	installFakeRepositorySetupTools(t)
	root := t.TempDir()
	workDir := filepath.Join(root, "workdir")
	if err := os.Mkdir(workDir, 0o700); err != nil {
		t.Fatal(err)
	}
	writeRepositorySetupFiles(t, workDir)
	if err := os.Remove(filepath.Join(workDir, "go.sum")); err != nil {
		t.Fatal(err)
	}
	if err := os.Symlink(filepath.Join(workDir, "go.mod"), filepath.Join(workDir, "go.sum")); err != nil {
		t.Fatal(err)
	}
	_, err := prepareRepositorySetup(context.Background(), repositorySetupTask(t, "trusted", 60, "go_mod_download"), "claude", workDir, root)
	if err == nil || !strings.Contains(err.Error(), "regular file") {
		t.Fatalf("symlink error = %v", err)
	}
	if errors.Is(err, context.Canceled) {
		t.Fatalf("wrong error = %v", err)
	}
}

func TestPrepareRepositorySetupRejectsSymlinkedCacheRoot(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("symlink fixture requires unix permissions")
	}
	installFakeRepositorySetupTools(t)
	root := t.TempDir()
	workDir := filepath.Join(root, "workdir")
	if err := os.Mkdir(workDir, 0o700); err != nil {
		t.Fatal(err)
	}
	writeRepositorySetupFiles(t, workDir)
	outside := t.TempDir()
	if err := os.Symlink(outside, filepath.Join(root, ".multica")); err != nil {
		t.Fatal(err)
	}
	_, err := prepareRepositorySetup(context.Background(), repositorySetupTask(t, "trusted", 60, "go_mod_download"), "claude", workDir, root)
	if err == nil || !strings.Contains(err.Error(), "safe directory") {
		t.Fatalf("cache symlink error = %v", err)
	}
	entries, readErr := os.ReadDir(outside)
	if readErr != nil {
		t.Fatal(readErr)
	}
	if len(entries) != 0 {
		t.Fatal("setup wrote through a symlinked cache root")
	}
}

func TestPrepareRepositorySetupDoesNotPublishSymlinkedToolCache(t *testing.T) {
	tools := installFakeRepositorySetupTools(t)
	tools.setMode(t, "symlink")
	root := t.TempDir()
	workDir := filepath.Join(root, "workdir")
	if err := os.Mkdir(workDir, 0o700); err != nil {
		t.Fatal(err)
	}
	writeRepositorySetupFiles(t, workDir)
	task := repositorySetupTask(t, "trusted", 60, "go_mod_download")
	_, err := prepareRepositorySetup(context.Background(), task, "claude", workDir, root)
	if err == nil || !strings.Contains(err.Error(), "symlink") {
		t.Fatalf("cache publication error = %v", err)
	}
	entries, readErr := os.ReadDir(filepath.Join(root, ".multica", "setup-cache", "artifacts"))
	if readErr != nil {
		t.Fatal(readErr)
	}
	if len(entries) != 0 {
		t.Fatalf("unsafe setup published %d artifacts", len(entries))
	}
}

func TestPrepareRepositorySetupSharedCacheCrossEnvRootAndMutationIsolation(t *testing.T) {
	t.Setenv("HOME", t.TempDir())
	installFakeRepositorySetupTools(t)
	workspacesRoot := t.TempDir()
	registerRepositorySetupSharedCleanup(t, workspacesRoot)
	makeEnv := func(name string) (string, string) {
		t.Helper()
		envRoot := filepath.Join(workspacesRoot, name)
		workDir := filepath.Join(envRoot, "workdir")
		if err := os.MkdirAll(workDir, 0o700); err != nil {
			t.Fatal(err)
		}
		writeRepositorySetupFiles(t, workDir)
		return envRoot, workDir
	}
	coldRoot, coldWorkDir := makeEnv("task-cold")
	warmRoot, warmWorkDir := makeEnv("task-warm")
	task := repositorySetupTask(t, "trusted", 60, "go_mod_download")

	cold, err := prepareRepositorySetupWithSharedCache(context.Background(), task, "claude", coldWorkDir, coldRoot, workspacesRoot)
	if err != nil {
		t.Fatal(err)
	}
	task.ID = "task-2"
	warm, err := prepareRepositorySetupWithSharedCache(context.Background(), task, "claude", warmWorkDir, warmRoot, workspacesRoot)
	if err != nil {
		t.Fatal(err)
	}
	if cold.CacheHit || cold.SharedCacheStatus != repositorySetupSharedCachePublished {
		t.Fatalf("cold result = %#v", cold)
	}
	if !warm.CacheHit || warm.SharedCacheStatus != repositorySetupSharedCacheHit || warm.CacheKey != cold.CacheKey {
		t.Fatalf("warm result = %#v, cold = %#v", warm, cold)
	}
	if cold.Env["GOMODCACHE"] == warm.Env["GOMODCACHE"] {
		t.Fatal("cross-envRoot restore reused a writable task cache")
	}
	stages, err := os.ReadDir(filepath.Join(workspacesRoot, ".setup-cache", "staging", repositorySetupSharedCacheVersion))
	if err != nil {
		t.Fatal(err)
	}
	if len(stages) != 0 {
		t.Fatalf("successful publication left %d staging directories", len(stages))
	}

	artifactFile := filepath.Join(workspacesRoot, ".setup-cache", "artifacts", "v2", cold.CacheKey, "go-mod", "payload", "tool")
	coldFile := filepath.Join(cold.Env["GOMODCACHE"], "payload", "tool")
	warmFile := filepath.Join(warm.Env["GOMODCACHE"], "payload", "tool")
	artifactBefore, err := os.ReadFile(artifactFile)
	if err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(warmFile, []byte("mutated task copy"), 0o600); err != nil {
		t.Fatal(err)
	}
	artifactAfter, err := os.ReadFile(artifactFile)
	if err != nil {
		t.Fatal(err)
	}
	coldAfter, err := os.ReadFile(coldFile)
	if err != nil {
		t.Fatal(err)
	}
	if !bytes.Equal(artifactBefore, artifactAfter) || !bytes.Equal(artifactBefore, coldAfter) {
		t.Fatal("mutating a task-private restore changed the immutable artifact or another task cache")
	}
	for _, pair := range [][2]string{{artifactFile, coldFile}, {artifactFile, warmFile}, {coldFile, warmFile}} {
		a, err := os.Stat(pair[0])
		if err != nil {
			t.Fatal(err)
		}
		b, err := os.Stat(pair[1])
		if err != nil {
			t.Fatal(err)
		}
		if os.SameFile(a, b) {
			t.Fatalf("shared artifact/task copy used a hardlink: %v", pair)
		}
	}
	task.Agent = &AgentData{CustomEnv: map[string]string{
		"GOCACHE":     "/stale/go-build",
		"GOMODCACHE":  "/stale/go-mod",
		"GOTOOLCHAIN": "auto",
	}}
	managedOverrides, err := prepareRepositorySetupWithSharedCache(context.Background(), task, "claude", warmWorkDir, warmRoot, workspacesRoot)
	if err != nil {
		t.Fatal(err)
	}
	if !managedOverrides.CacheHit || managedOverrides.CacheKey != cold.CacheKey {
		t.Fatalf("managed cache overrides changed eligibility/key: %#v", managedOverrides)
	}
	task.Agent = nil

	providerChanged, err := prepareRepositorySetupWithSharedCache(context.Background(), task, "codex", warmWorkDir, warmRoot, workspacesRoot)
	if err != nil {
		t.Fatal(err)
	}
	if providerChanged.CacheKey == cold.CacheKey {
		t.Fatal("provider did not change shared artifact key")
	}
	task.Agent = &AgentData{CustomEnv: map[string]string{"GOPROXY": "https://proxy.example.test"}}
	configChanged, err := prepareRepositorySetupWithSharedCache(context.Background(), task, "claude", warmWorkDir, warmRoot, workspacesRoot)
	if err != nil {
		t.Fatal(err)
	}
	if configChanged.CacheKey == cold.CacheKey {
		t.Fatal("reviewed non-secret dependency configuration did not change shared artifact key")
	}
	task.Agent = &AgentData{CustomEnv: map[string]string{"GOOS": "linux"}}
	targetChanged, err := prepareRepositorySetupWithSharedCache(context.Background(), task, "claude", warmWorkDir, warmRoot, workspacesRoot)
	if err != nil {
		t.Fatal(err)
	}
	if targetChanged.CacheKey == cold.CacheKey {
		t.Fatal("explicit Go target configuration did not change shared artifact key")
	}
}

func TestRepositorySetupSharedCacheOrdinaryGitConfigDoesNotDisableCache(t *testing.T) {
	home := t.TempDir()
	t.Setenv("HOME", home)
	installFakeRepositorySetupTools(t)
	workspacesRoot := t.TempDir()
	registerRepositorySetupSharedCleanup(t, workspacesRoot)
	makeEnv := func(name string) (string, string) {
		t.Helper()
		envRoot := filepath.Join(workspacesRoot, name)
		workDir := filepath.Join(envRoot, "workdir")
		if err := os.MkdirAll(workDir, 0o700); err != nil {
			t.Fatal(err)
		}
		writeRepositorySetupFiles(t, workDir)
		return envRoot, workDir
	}

	if err := os.WriteFile(filepath.Join(home, ".gitconfig"), []byte("[user]\n\tname = Test User\n\temail = test@example.invalid\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	envRoot, workDir := makeEnv("ordinary-git-config")
	result, err := prepareRepositorySetupWithSharedCache(context.Background(), repositorySetupTask(t, "trusted", 60, "go_mod_download"), "claude", workDir, envRoot, workspacesRoot)
	if err != nil {
		t.Fatal(err)
	}
	if result.SharedCacheStatus != repositorySetupSharedCachePublished {
		t.Fatalf("ordinary user-only git config disabled shared cache: %#v", result)
	}
}

func TestRepositorySetupSharedCacheSkipsHiddenGoAndNPMConfig(t *testing.T) {
	installFakeRepositorySetupTools(t)
	for _, tc := range []struct {
		name string
		step string
		seed func(t *testing.T, home, workDir string)
	}{
		{name: "go env", step: "go_mod_download", seed: func(t *testing.T, _, _ string) {
			configDir, err := os.UserConfigDir()
			if err != nil {
				t.Fatal(err)
			}
			if err := os.MkdirAll(filepath.Join(configDir, "go"), 0o700); err != nil {
				t.Fatal(err)
			}
			if err := os.WriteFile(filepath.Join(configDir, "go", "env"), []byte("configured"), 0o600); err != nil {
				t.Fatal(err)
			}
		}},
		{name: "git credentials", step: "go_mod_download", seed: func(t *testing.T, home, _ string) {
			if err := os.WriteFile(filepath.Join(home, ".git-credentials"), []byte("configured"), 0o600); err != nil {
				t.Fatal(err)
			}
		}},
		{name: "repository npmrc", step: "pnpm_install", seed: func(t *testing.T, _, workDir string) {
			if err := os.WriteFile(filepath.Join(workDir, ".npmrc"), []byte("configured"), 0o600); err != nil {
				t.Fatal(err)
			}
		}},
	} {
		t.Run(tc.name, func(t *testing.T) {
			home := t.TempDir()
			t.Setenv("HOME", home)
			workspacesRoot := t.TempDir()
			registerRepositorySetupSharedCleanup(t, workspacesRoot)
			envRoot := filepath.Join(workspacesRoot, "task")
			workDir := filepath.Join(envRoot, "workdir")
			if err := os.MkdirAll(workDir, 0o700); err != nil {
				t.Fatal(err)
			}
			writeRepositorySetupFiles(t, workDir)
			tc.seed(t, home, workDir)
			result, err := prepareRepositorySetupWithSharedCache(context.Background(), repositorySetupTask(t, "trusted", 60, tc.step), "claude", workDir, envRoot, workspacesRoot)
			if err != nil {
				t.Fatal(err)
			}
			if result.SharedCacheStatus != repositorySetupSharedCacheSkippedCredentials {
				t.Fatalf("hidden config did not disable shared cache: %#v", result)
			}
		})
	}
}

func TestPrepareRepositorySetupSharedRootRefusesUnmarkedOrInvalidCache(t *testing.T) {
	for _, tc := range []struct {
		name    string
		prepare func(t *testing.T, root string)
	}{
		{name: "unmarked non-empty", prepare: func(t *testing.T, root string) {
			if err := os.MkdirAll(filepath.Join(root, ".setup-cache"), 0o700); err != nil {
				t.Fatal(err)
			}
			if err := os.WriteFile(filepath.Join(root, ".setup-cache", "user-file"), []byte("owned"), 0o600); err != nil {
				t.Fatal(err)
			}
		}},
		{name: "invalid marker", prepare: func(t *testing.T, root string) {
			if err := os.MkdirAll(filepath.Join(root, ".setup-cache"), 0o700); err != nil {
				t.Fatal(err)
			}
			if err := os.WriteFile(filepath.Join(root, ".setup-cache", ".multica-managed-v2"), []byte("other\n"), 0o600); err != nil {
				t.Fatal(err)
			}
		}},
	} {
		t.Run(tc.name, func(t *testing.T) {
			root := t.TempDir()
			tc.prepare(t, root)
			if _, err := prepareRepositorySetupSharedRoot(root); err == nil {
				t.Fatal("unsafe shared cache root was adopted")
			}
		})
	}
}

func TestPrepareRepositorySetupSharedRootConcurrentInitialization(t *testing.T) {
	root := t.TempDir()
	const callers = 32
	start := make(chan struct{})
	errs := make(chan error, callers)
	for range callers {
		go func() {
			<-start
			_, err := prepareRepositorySetupSharedRoot(root)
			errs <- err
		}()
	}
	close(start)
	for range callers {
		if err := <-errs; err != nil {
			t.Fatalf("concurrent shared cache initialization: %v", err)
		}
	}
	marker, err := os.ReadFile(filepath.Join(root, ".setup-cache", ".multica-managed-v2"))
	if err != nil {
		t.Fatal(err)
	}
	if string(marker) != repositorySetupSharedCacheMarkerContent {
		t.Fatalf("marker = %q", marker)
	}
}

func TestPrepareRepositorySetupSharedCacheSkipsAmbiguousCredentialConfiguration(t *testing.T) {
	t.Setenv("HOME", t.TempDir())
	installFakeRepositorySetupTools(t)
	for _, tc := range []struct {
		name string
		env  map[string]string
	}{
		{name: "credential", env: map[string]string{"GOAUTH": "credential helper"}},
		{name: "unknown tool configuration", env: map[string]string{"NODE_OPTIONS": "--require somewhere"}},
		{name: "credentialed proxy URL", env: map[string]string{"HTTPS_PROXY": "https://user:password@proxy.example.test"}},
	} {
		t.Run(tc.name, func(t *testing.T) {
			workspacesRoot := t.TempDir()
			registerRepositorySetupSharedCleanup(t, workspacesRoot)
			envRoot := filepath.Join(workspacesRoot, "task")
			workDir := filepath.Join(envRoot, "workdir")
			if err := os.MkdirAll(workDir, 0o700); err != nil {
				t.Fatal(err)
			}
			writeRepositorySetupFiles(t, workDir)
			task := repositorySetupTask(t, "trusted", 60, "go_mod_download")
			task.Agent = &AgentData{CustomEnv: tc.env}
			result, err := prepareRepositorySetupWithSharedCache(context.Background(), task, "claude", workDir, envRoot, workspacesRoot)
			if err != nil {
				t.Fatal(err)
			}
			if result.CacheHit || result.CacheKey != "" || result.SharedCacheStatus != repositorySetupSharedCacheSkippedCredentials {
				t.Fatalf("credential-ambiguous result = %#v", result)
			}
			entries, err := os.ReadDir(filepath.Join(workspacesRoot, ".setup-cache", "artifacts", "v2"))
			if err != nil {
				t.Fatal(err)
			}
			if len(entries) != 0 {
				t.Fatalf("credential-ambiguous setup published %d shared artifacts", len(entries))
			}
		})
	}
}

func TestPrepareRepositorySetupSharedCacheFailureAndCancellationDoNotPublish(t *testing.T) {
	t.Setenv("HOME", t.TempDir())
	for _, tc := range []struct {
		name   string
		mode   string
		cancel bool
	}{{name: "failure", mode: "fail"}, {name: "cancel", mode: "wait", cancel: true}} {
		t.Run(tc.name, func(t *testing.T) {
			tools := installFakeRepositorySetupTools(t)
			tools.setMode(t, tc.mode)
			workspacesRoot := t.TempDir()
			registerRepositorySetupSharedCleanup(t, workspacesRoot)
			envRoot := filepath.Join(workspacesRoot, "task")
			workDir := filepath.Join(envRoot, "workdir")
			if err := os.MkdirAll(workDir, 0o700); err != nil {
				t.Fatal(err)
			}
			writeRepositorySetupFiles(t, workDir)
			task := repositorySetupTask(t, "trusted", 60, "go_mod_download")
			ctx := context.Background()
			if tc.cancel {
				var cancel context.CancelFunc
				ctx, cancel = context.WithCancel(ctx)
				time.AfterFunc(50*time.Millisecond, cancel)
			}
			if _, err := prepareRepositorySetupWithSharedCache(ctx, task, "claude", workDir, envRoot, workspacesRoot); err == nil {
				t.Fatal("setup unexpectedly succeeded")
			}
			entries, err := os.ReadDir(filepath.Join(workspacesRoot, ".setup-cache", "artifacts", "v2"))
			if err != nil {
				t.Fatal(err)
			}
			if len(entries) != 0 {
				t.Fatalf("failed setup published %d shared artifacts", len(entries))
			}
		})
	}
}

func TestPruneRepositorySetupSharedArtifactsSkipsActiveAndRemovesStale(t *testing.T) {
	workspacesRoot := t.TempDir()
	sharedRoot, err := prepareRepositorySetupSharedRoot(workspacesRoot)
	if err != nil {
		t.Fatal(err)
	}
	active := filepath.Join(sharedRoot, "artifacts", "v2", strings.Repeat("a", 64))
	stale := filepath.Join(sharedRoot, "artifacts", "v2", strings.Repeat("b", 64))
	for _, path := range []string{active, stale} {
		if err := os.MkdirAll(path, 0o700); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(filepath.Join(path, "cache"), []byte("payload"), 0o600); err != nil {
			t.Fatal(err)
		}
		old := time.Now().Add(-repositorySetupSharedCacheTTL - time.Hour)
		if err := os.Chtimes(path, old, old); err != nil {
			t.Fatal(err)
		}
	}
	release, err := lockRepositorySetupSharedArtifact(context.Background(), sharedRoot, active)
	if err != nil {
		t.Fatal(err)
	}
	removed, err := pruneRepositorySetupSharedArtifacts(context.Background(), sharedRoot, time.Now(), repositorySetupMaxSharedArtifacts)
	if err != nil {
		release()
		t.Fatal(err)
	}
	if removed != 1 {
		release()
		t.Fatalf("removed = %d, want stale unlocked artifact only", removed)
	}
	if _, err := os.Stat(active); err != nil {
		release()
		t.Fatalf("active artifact was pruned: %v", err)
	}
	release()
	removed, err = pruneRepositorySetupSharedArtifacts(context.Background(), sharedRoot, time.Now(), repositorySetupMaxSharedArtifacts)
	if err != nil {
		t.Fatal(err)
	}
	if removed != 1 {
		t.Fatalf("removed after release = %d, want 1", removed)
	}
}

func TestPruneRepositorySetupSharedArtifactsRemovesOnlyStalePublicationStages(t *testing.T) {
	workspacesRoot := t.TempDir()
	sharedRoot, err := prepareRepositorySetupSharedRoot(workspacesRoot)
	if err != nil {
		t.Fatal(err)
	}
	stagingRoot := filepath.Join(sharedRoot, "staging", repositorySetupSharedCacheVersion)
	stale, err := os.MkdirTemp(stagingRoot, ".publish-")
	if err != nil {
		t.Fatal(err)
	}
	fresh, err := os.MkdirTemp(stagingRoot, ".publish-")
	if err != nil {
		t.Fatal(err)
	}
	for _, path := range []string{stale, fresh} {
		if err := os.WriteFile(filepath.Join(path, "cache"), []byte("payload"), 0o600); err != nil {
			t.Fatal(err)
		}
	}
	old := time.Now().Add(-repositorySetupSharedStagingTTL - time.Minute)
	if err := os.Chtimes(stale, old, old); err != nil {
		t.Fatal(err)
	}
	if _, err := pruneRepositorySetupSharedArtifacts(context.Background(), sharedRoot, time.Now(), repositorySetupMaxSharedArtifacts); err != nil {
		t.Fatal(err)
	}
	if _, err := os.Stat(stale); !errors.Is(err, fs.ErrNotExist) {
		t.Fatalf("stale publication stage was not removed: %v", err)
	}
	if _, err := os.Stat(fresh); err != nil {
		t.Fatalf("fresh publication stage was removed: %v", err)
	}
}
