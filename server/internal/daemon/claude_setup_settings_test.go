package daemon

import (
	"encoding/json"
	"os"
	"path/filepath"
	"reflect"
	"runtime"
	"testing"
)

func TestMergeClaudeSetupWritableRootsPreservesRestrictiveSettings(t *testing.T) {
	root := t.TempDir()
	settingsPath := filepath.Join(root, "claude-settings.json")
	initial := map[string]any{
		"skillOverrides": map[string]any{"review": "off"},
		"permissions":    map[string]any{"deny": []any{"Skill(review)", "Bash(rm *)"}},
		"hooks":          map[string]any{"PreToolUse": []any{map[string]any{"matcher": "Bash", "hooks": []any{map[string]any{"type": "command", "command": "deny-hook"}}}}},
		"sandbox": map[string]any{
			"enabled": true,
			"filesystem": map[string]any{
				"denyWrite":  []any{"/"},
				"allowWrite": []any{filepath.Join(root, "existing")},
			},
		},
	}
	data, err := json.Marshal(initial)
	if err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(settingsPath, data, 0o600); err != nil {
		t.Fatal(err)
	}
	goCache := filepath.Join(root, "go-build")
	pnpmStore := filepath.Join(root, "pnpm-store")
	gotPath, err := prepareClaudeSetupSettings(root, settingsPath, []string{goCache, pnpmStore, goCache})
	if err != nil {
		t.Fatalf("prepareClaudeSetupSettings: %v", err)
	}
	if gotPath != settingsPath {
		t.Fatalf("settings path=%q want=%q", gotPath, settingsPath)
	}

	mergedData, err := os.ReadFile(settingsPath)
	if err != nil {
		t.Fatal(err)
	}
	var got map[string]any
	if err := json.Unmarshal(mergedData, &got); err != nil {
		t.Fatal(err)
	}
	for _, key := range []string{"skillOverrides", "permissions", "hooks"} {
		if !reflect.DeepEqual(got[key], initial[key]) {
			t.Fatalf("%s changed: got=%#v want=%#v", key, got[key], initial[key])
		}
	}
	sandbox := got["sandbox"].(map[string]any)
	if sandbox["enabled"] != true {
		t.Fatalf("sandbox policy changed: %#v", sandbox)
	}
	filesystem := sandbox["filesystem"].(map[string]any)
	if !reflect.DeepEqual(filesystem["denyWrite"], []any{"/"}) {
		t.Fatalf("denyWrite changed: %#v", filesystem["denyWrite"])
	}
	wantAllowWrite := []any{filepath.Join(root, "existing"), goCache, pnpmStore}
	if !reflect.DeepEqual(filesystem["allowWrite"], wantAllowWrite) {
		t.Fatalf("allowWrite=%#v want=%#v", filesystem["allowWrite"], wantAllowWrite)
	}
	info, err := os.Stat(settingsPath)
	if err != nil {
		t.Fatal(err)
	}
	if runtime.GOOS != "windows" && info.Mode().Perm() != 0o600 {
		t.Fatalf("settings mode=%v", info.Mode().Perm())
	}
}

func TestMergeClaudeSetupWritableRootsRejectsAmbiguousSettings(t *testing.T) {
	path := filepath.Join(t.TempDir(), "claude-settings.json")
	if err := os.WriteFile(path, []byte(`{"permissions":{"deny":["Bash(rm *)"]},"sandbox":"inherit"}`), 0o600); err != nil {
		t.Fatal(err)
	}
	before, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := prepareClaudeSetupSettings(filepath.Dir(path), path, []string{filepath.Join(t.TempDir(), "go-build")}); err == nil {
		t.Fatal("non-object sandbox accepted")
	}
	after, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	if !reflect.DeepEqual(after, before) {
		t.Fatal("rejected settings were modified")
	}
}

func TestPrepareClaudeSetupSettingsRejectsUntrustedExistingFile(t *testing.T) {
	for _, variant := range []string{"symlink-parent", "symlink-leaf", "loose-mode", "oversize"} {
		t.Run(variant, func(t *testing.T) {
			if runtime.GOOS == "windows" && variant != "oversize" {
				t.Skip("requires POSIX permissions or unprivileged symlinks")
			}
			root := t.TempDir()
			path := filepath.Join(root, "claude-settings.json")
			switch variant {
			case "symlink-parent":
				target := t.TempDir()
				if err := os.WriteFile(filepath.Join(target, "claude-settings.json"), []byte(`{}`), 0o600); err != nil {
					t.Fatal(err)
				}
				parent := filepath.Join(root, "linked")
				if err := os.Symlink(target, parent); err != nil {
					t.Fatal(err)
				}
				path = filepath.Join(parent, "claude-settings.json")
			case "symlink-leaf":
				target := filepath.Join(root, "target.json")
				if err := os.WriteFile(target, []byte(`{}`), 0o600); err != nil {
					t.Fatal(err)
				}
				if err := os.Symlink(target, path); err != nil {
					t.Fatal(err)
				}
			case "loose-mode":
				if err := os.WriteFile(path, []byte(`{}`), 0o644); err != nil {
					t.Fatal(err)
				}
			case "oversize":
				data := make([]byte, claudeSetupSettingsMaxBytes+1)
				if err := os.WriteFile(path, data, 0o600); err != nil {
					t.Fatal(err)
				}
			}
			if _, err := prepareClaudeSetupSettings(root, path, []string{filepath.Join(root, "go-build")}); err == nil {
				t.Fatalf("%s existing settings accepted", variant)
			}
		})
	}
}

func TestMergeClaudeSetupWritableRootsNoRootsIsNoOp(t *testing.T) {
	path, err := prepareClaudeSetupSettings("", "", nil)
	if err != nil {
		t.Fatalf("empty roots: %v", err)
	}
	if path != "" {
		t.Fatalf("empty roots settings path=%q", path)
	}
}

func TestPrepareClaudeSetupSettingsCreatesTaskLocalFile(t *testing.T) {
	root, err := filepath.EvalSymlinks(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	goCache := filepath.Join(root, ".multica", "setup-cache", "tasks", "task", "private", "go-build")
	settingsPath, err := prepareClaudeSetupSettings(root, "", []string{goCache})
	if err != nil {
		t.Fatalf("prepareClaudeSetupSettings: %v", err)
	}
	wantPath := filepath.Join(root, "claude-repository-setup-settings.json")
	if settingsPath != wantPath {
		t.Fatalf("settings path=%q want=%q", settingsPath, wantPath)
	}
	data, err := os.ReadFile(settingsPath)
	if err != nil {
		t.Fatal(err)
	}
	var settings struct {
		Sandbox struct {
			Filesystem struct {
				AllowWrite []string `json:"allowWrite"`
			} `json:"filesystem"`
		} `json:"sandbox"`
	}
	if err := json.Unmarshal(data, &settings); err != nil {
		t.Fatal(err)
	}
	if !reflect.DeepEqual(settings.Sandbox.Filesystem.AllowWrite, []string{goCache}) {
		t.Fatalf("allowWrite=%v", settings.Sandbox.Filesystem.AllowWrite)
	}
}
