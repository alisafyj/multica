package daemon

import (
	"context"
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func setupTaskDirectories(t *testing.T, task Task, directories any) Task {
	t.Helper()
	var ref map[string]any
	if err := json.Unmarshal(task.ProjectResources[0].ResourceRef, &ref); err != nil {
		t.Fatal(err)
	}
	ref["setup"].(map[string]any)["step_directories"] = directories
	raw, err := json.Marshal(ref)
	if err != nil {
		t.Fatal(err)
	}
	task.ProjectResources = append([]ProjectResourceData(nil), task.ProjectResources...)
	task.ProjectResources[0].ResourceRef = raw
	return task
}

func setupDirectoryFixture(t *testing.T) (string, string) {
	t.Helper()
	root := t.TempDir()
	workDir := filepath.Join(root, "workdir")
	for _, dir := range []string{workDir, filepath.Join(workDir, "web"), filepath.Join(workDir, "other-web")} {
		if err := os.MkdirAll(dir, 0o700); err != nil {
			t.Fatal(err)
		}
		writeRepositorySetupFiles(t, dir)
	}
	for _, name := range []string{"package.json", "pnpm-lock.yaml"} {
		if err := os.Remove(filepath.Join(workDir, name)); err != nil {
			t.Fatal(err)
		}
	}
	return root, workDir
}

func TestRepositorySetupStepDirectoriesMonorepoAndCacheIdentity(t *testing.T) {
	tools := installFakeRepositorySetupTools(t)
	for _, name := range []string{"go", "pnpm"} {
		file := filepath.Join(tools.bin, name)
		body, err := os.ReadFile(file)
		if err != nil {
			t.Fatal(err)
		}
		_, bodyText, _ := strings.Cut(string(body), "\n")
		bodyText = "#!/bin/sh\ncase \"$1\" in version|--version) ;; *) pwd -P > \"setup-$(basename \"$0\").cwd\" ;; esac\n" + bodyText
		if err := os.WriteFile(file, []byte(bodyText), 0o700); err != nil {
			t.Fatal(err)
		}
	}
	root, workDir := setupDirectoryFixture(t)
	task := repositorySetupTask(t, "trusted", 60, "go_mod_download", "pnpm_install")
	task = setupTaskDirectories(t, task, map[string]string{"pnpm_install": "web"})
	cold, err := prepareRepositorySetup(context.Background(), task, "codex", workDir, root)
	if err != nil {
		t.Fatal(err)
	}
	encoded, err := json.Marshal(cold.Steps)
	if err != nil {
		t.Fatal(err)
	}
	var steps []map[string]any
	if err := json.Unmarshal(encoded, &steps); err != nil {
		t.Fatal(err)
	}
	if len(steps) != 2 || steps[0]["directory"] != nil || steps[1]["directory"] != "web" {
		t.Fatalf("setup evidence does not distinguish root and web steps: %s", encoded)
	}
	for name, dir := range map[string]string{"go": workDir, "pnpm": filepath.Join(workDir, "web")} {
		observed, err := os.ReadFile(filepath.Join(dir, "setup-"+name+".cwd"))
		canonical, resolveErr := filepath.EvalSymlinks(dir)
		if err != nil || resolveErr != nil || strings.TrimSpace(string(observed)) != canonical {
			t.Fatalf("%s did not execute in its configured directory: read=%v resolve=%v", name, err, resolveErr)
		}
	}
	warm, err := prepareRepositorySetup(context.Background(), task, "codex", workDir, root)
	if err != nil || !warm.CacheHit || warm.CacheKey != cold.CacheKey {
		t.Fatalf("same directory did not reuse cache: %v", err)
	}
	other := setupTaskDirectories(t, task, map[string]string{"pnpm_install": "other-web"})
	changed, err := prepareRepositorySetup(context.Background(), other, "codex", workDir, root)
	if err != nil || changed.CacheKey == cold.CacheKey {
		t.Fatalf("different directory with identical lockfile reused cache identity: %v", err)
	}
}

func TestRepositorySetupStepDirectoriesRejectUnsafePaths(t *testing.T) {
	check := func(t *testing.T, directories any) {
		t.Helper()
		task := setupTaskDirectories(t, repositorySetupTask(t, "trusted", 60, "pnpm_install"), directories)
		ref, err := repositorySetupRefForTask(task)
		if err == nil {
			err = validateRepositorySetupConfig(*ref.Setup)
		}
		if err == nil {
			t.Fatal("invalid directory map passed configuration validation")
		}
	}
	for _, directory := range []string{"", ".", "..", "../web", "/tmp/web", "web/../other-web", "web//nested", "web/", " web", "web\\nested", "web:stream", "web\nnext", ".git", ".GIT", "web/.multica", "web/ nested", "web/dir.", strings.Repeat("a", 513)} {
		t.Run(directory, func(t *testing.T) {
			check(t, map[string]string{"pnpm_install": directory})
		})
	}
	for _, directories := range []any{nil, map[string]string{}, map[string]any{"pnpm_install": nil}, map[string]string{"unknown": "web"}, map[string]string{"go_mod_download": "web"}} {
		check(t, directories)
	}
}

func TestRepositorySetupStepDirectoriesRejectSymlinkSegments(t *testing.T) {
	installFakeRepositorySetupTools(t)
	root, workDir := setupDirectoryFixture(t)
	outside := t.TempDir()
	writeRepositorySetupFiles(t, outside)
	for name, target := range map[string]string{"outside-link": outside, "inside-link": filepath.Join(workDir, "web")} {
		if err := os.Symlink(target, filepath.Join(workDir, name)); err != nil {
			t.Fatal(err)
		}
		task := setupTaskDirectories(t, repositorySetupTask(t, "trusted", 60, "pnpm_install"), map[string]string{"pnpm_install": name})
		if _, err := prepareRepositorySetup(context.Background(), task, "codex", workDir, root); err == nil {
			t.Fatal("symlink directory was accepted")
		}
	}
}

func TestRepositorySetupStepDirectoriesLegacySerialization(t *testing.T) {
	const legacy = `{"steps":["pnpm_install"],"timeout_seconds":60}`
	var config repositorySetupConfig
	if err := json.Unmarshal([]byte(legacy), &config); err != nil {
		t.Fatal(err)
	}
	if err := validateRepositorySetupConfig(config); err != nil {
		t.Fatal(err)
	}
	roundtrip, err := json.Marshal(config)
	if err != nil || string(roundtrip) != legacy {
		t.Fatalf("legacy setup changed: %s, %v", roundtrip, err)
	}
	config.StepDirectories = map[string]string{"pnpm_install": "packages/\u754c\u9762"}
	if err := validateRepositorySetupConfig(config); err != nil {
		t.Fatalf("valid Unicode directory rejected: %v", err)
	}
}

func TestRepositorySetupStepDirectoriesNestedNPMConfigSkipsSharedCache(t *testing.T) {
	installFakeRepositorySetupTools(t)
	t.Setenv("HOME", t.TempDir())
	workspacesRoot := t.TempDir()
	registerRepositorySetupSharedCleanup(t, workspacesRoot)
	root := filepath.Join(workspacesRoot, "task-env")
	workDir := filepath.Join(root, "workdir")
	web := filepath.Join(workDir, "packages", "web")
	if err := os.MkdirAll(web, 0o700); err != nil {
		t.Fatal(err)
	}
	writeRepositorySetupFiles(t, web)
	task := setupTaskDirectories(t, repositorySetupTask(t, "trusted", 60, "pnpm_install"), map[string]string{"pnpm_install": "packages/web"})
	for _, directory := range []string{workDir, filepath.Join(workDir, "packages"), web} {
		config := filepath.Join(directory, ".npmrc")
		if err := os.WriteFile(config, []byte("registry=https://example.invalid\n"), 0o600); err != nil {
			t.Fatal(err)
		}
		result, err := prepareRepositorySetupWithSharedCache(context.Background(), task, "codex", workDir, root, workspacesRoot)
		if err != nil || result.SharedCacheStatus != repositorySetupSharedCacheSkippedCredentials || result.CacheKey != "" {
			t.Fatalf("nested config did not keep dependency artifacts private: %v status=%s", err, result.SharedCacheStatus)
		}
		if err := os.Remove(config); err != nil {
			t.Fatal(err)
		}
	}
}
