package daemon

import (
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"
)

func TestRepositorySetupWritableRoots(t *testing.T) {
	for _, variant := range []string{"valid", "keyed-valid", "duplicate-step", "bad-cache-key", "store-alias-mismatch", "outside", "other-task", "symlink-parent", "symlink-leaf", "missing-env", "failed-step", "unknown-step"} {
		t.Run(variant, func(t *testing.T) {
			root, err := filepath.EvalSymlinks(t.TempDir())
			if err != nil {
				t.Fatal(err)
			}
			task := Task{ID: "test-task"}
			key := "private"
			if variant == "keyed-valid" {
				key = strings.Repeat("a", 64)
			}
			mutable := filepath.Join(root, ".multica", "setup-cache", "tasks", repositorySetupPathSegment(task.ID), key)
			if err := os.MkdirAll(mutable, 0o700); err != nil {
				t.Fatal(err)
			}
			cache, store := filepath.Join(mutable, "go-build"), filepath.Join(mutable, "pnpm-store")
			setup := repositorySetupResult{Env: map[string]string{"GOCACHE": cache, "GOMODCACHE": filepath.Join(mutable, "go-mod"), "PNPM_STORE_DIR": store, "NPM_CONFIG_STORE_DIR": store}, Steps: []repositorySetupStepResult{{Name: "go_mod_download", Status: "completed"}, {Name: "pnpm_install", Status: "completed"}}}
			switch variant {
			case "keyed-valid":
				setup.CacheKey = key
			case "duplicate-step":
				setup.Steps = append(setup.Steps, setup.Steps[0])
			case "bad-cache-key":
				setup.CacheKey = "../escape"
			case "store-alias-mismatch":
				setup.Env["NPM_CONFIG_STORE_DIR"] = root
			case "outside":
				setup.Env["GOCACHE"] = root
			case "other-task":
				setup.Env["GOCACHE"] = filepath.Join(root, ".multica", "setup-cache", "tasks", "other-task", "private", "go-build")
			case "symlink-parent":
				if err := os.Remove(mutable); err != nil {
					t.Fatal(err)
				}
				if err := os.Symlink(root, mutable); err != nil {
					t.Fatal(err)
				}
			case "symlink-leaf":
				if err := os.Symlink(root, cache); err != nil {
					t.Fatal(err)
				}
			case "missing-env":
				delete(setup.Env, "GOCACHE")
			case "failed-step":
				setup.Steps[0].Status = "failed"
			case "unknown-step":
				setup.Steps[0].Name = "custom-shell"
			}
			for _, provider := range []string{"codex", "claude"} {
				got, err := repositorySetupWritableRoots(provider, task, root, setup)
				if variant != "valid" && variant != "keyed-valid" && variant != "duplicate-step" {
					if err == nil {
						t.Fatalf("%s accepted unsafe or incomplete setup", provider)
					}
					continue
				}
				if err != nil || !reflect.DeepEqual(got, []string{cache, store}) {
					t.Fatalf("%s roots=%v error=%v", provider, got, err)
				}
				for _, path := range got {
					if info, err := os.Lstat(path); err != nil || !info.IsDir() {
						t.Fatalf("%s cache leaf must exist before native sandbox setup", provider)
					}
				}
			}
			if _, err := os.Stat(setup.Env["GOMODCACHE"]); !os.IsNotExist(err) {
				t.Fatal("module cache must not receive a write grant")
			}
		})
	}
	for _, provider := range []string{"claude", "codex"} {
		got, err := repositorySetupWritableRoots(provider, Task{}, "", repositorySetupResult{})
		if err != nil || got != nil {
			t.Fatal("no setup should be a no-op")
		}
	}
	got, err := repositorySetupWritableRoots("cursor", Task{}, "", repositorySetupResult{Steps: []repositorySetupStepResult{{Name: "unknown"}}})
	if err != nil || got != nil {
		t.Fatal("other provider should be unchanged")
	}
}
