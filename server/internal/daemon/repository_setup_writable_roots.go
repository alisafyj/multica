package daemon

import (
	"fmt"
	"path/filepath"
	"slices"
	"strings"
)

func repositorySetupWritableRoots(provider string, task Task, envRoot string, setup repositorySetupResult) ([]string, error) {
	if (provider != "codex" && provider != "claude") || len(setup.Steps) == 0 {
		return nil, nil
	}
	if strings.TrimSpace(task.ID) == "" || !filepath.IsAbs(envRoot) {
		return nil, fmt.Errorf("repository setup grants require a task environment")
	}
	root, err := filepath.EvalSymlinks(envRoot)
	if err != nil {
		return nil, err
	}
	key := setup.CacheKey
	if key == "" {
		key = "private"
	}
	if !filepath.IsLocal(key) || filepath.Base(key) != key {
		return nil, fmt.Errorf("invalid repository setup cache key")
	}
	mutable := filepath.Join(root, ".multica", "setup-cache", "tasks", repositorySetupPathSegment(task.ID), key)
	var roots []string
	for _, step := range setup.Steps {
		if step.Status != "completed" {
			return nil, fmt.Errorf("repository setup cache grants require completed setup")
		}
		var name, leaf string
		switch step.Name {
		case "go_mod_download":
			name, leaf = "GOCACHE", "go-build"
		case "pnpm_install":
			name, leaf = "PNPM_STORE_DIR", "pnpm-store"
		default:
			return nil, fmt.Errorf("unsupported repository setup cache grant")
		}
		expected := filepath.Join(mutable, leaf)
		if setup.Env[name] != expected || (name == "PNPM_STORE_DIR" && setup.Env["NPM_CONFIG_STORE_DIR"] != expected) {
			return nil, fmt.Errorf("repository setup cache grant escaped its task-private leaf")
		}
		relative, err := filepath.Rel(root, expected)
		if err != nil || !filepath.IsLocal(relative) {
			return nil, fmt.Errorf("invalid repository setup cache grant")
		}
		// Setup may not create an empty build cache; validate every parent while
		// creating that exact leaf before the native sandbox binds it.
		if err := ensureRepositorySetupDirectories(root, relative); err != nil {
			return nil, err
		}
		if !slices.Contains(roots, expected) {
			roots = append(roots, expected)
		}
	}
	return roots, nil
}
