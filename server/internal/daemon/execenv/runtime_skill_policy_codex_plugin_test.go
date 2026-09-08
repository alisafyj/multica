package execenv

import (
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"

	"github.com/multica-ai/multica/server/pkg/agent"
	"github.com/pelletier/go-toml/v2"
)

func codexPluginPolicyFixture(t *testing.T) (task, shared, install string) {
	t.Helper()
	root := t.TempDir()
	task, shared = filepath.Join(root, "task"), filepath.Join(root, "shared")
	t.Setenv("HOME", filepath.Join(root, "user"))
	t.Setenv("CODEX_HOME", shared)
	install = filepath.Join(shared, "plugins", "cache", "local", "review", "1.0.0")
	other := filepath.Join(shared, "plugins", "cache", "other", "review", "1.0.0")
	for _, dir := range []string{task, filepath.Join(task, "plugins"), filepath.Join(install, ".codex-plugin"), filepath.Join(install, "skills", "folder"), filepath.Join(other, ".codex-plugin"), filepath.Join(other, "extra", "folder")} {
		if err := os.MkdirAll(dir, 0o700); err != nil {
			t.Fatal(err)
		}
	}
	for name, content := range map[string]string{
		filepath.Join(install, ".codex-plugin", "plugin.json"): `{"name":"review","version":"1.0.0","skills":"./skills"}`,
		filepath.Join(other, ".codex-plugin", "plugin.json"):   `{"name":"review","version":"1.0.0","skills":"./extra"}`,
		filepath.Join(install, "skills", "folder", "SKILL.md"): "---\nname: native-name\ndescription: fixture\n---\nfixture\n",
		filepath.Join(other, "extra", "folder", "SKILL.md"):    "---\nname: second-name\ndescription: fixture\n---\nfixture\n",
		filepath.Join(task, "config.toml"):                     "model = 'unchanged'\n[features]\nplugins = true\n[plugins.'review@local']\nenabled = true\n[plugins.'review@other']\nenabled = true\n",
		filepath.Join(shared, "config.toml"):                   "# shared configuration must stay unchanged\n",
	} {
		if err := os.WriteFile(name, []byte(content), 0o600); err != nil {
			t.Fatal(err)
		}
	}
	if err := os.Symlink(filepath.Join(shared, "plugins", "cache"), filepath.Join(task, "plugins", "cache")); err != nil {
		t.Fatal(err)
	}
	return task, shared, install
}

func TestCodexPluginSkillPolicyWritesCanonicalPaths(t *testing.T) {
	task, shared, install := codexPluginPolicyFixture(t)
	refs := []RuntimeSkillRefForEnv{
		{Root: "plugin", Plugin: "review@local", Key: "review@local:skills/folder", Name: "../../not-path-authority"},
		{Root: "plugin", Plugin: "review@other", Key: "review@other:extra/folder", Name: "review:native-name"},
	}
	refs = append(refs, refs[0])
	if _, err := ensureCodexDisabledSkillsConfig(filepath.Join(task, "config.toml"), task, refs, []SkillContextForEnv{{Name: "review:native-name"}}); err != nil {
		t.Fatal(err)
	}
	data, err := os.ReadFile(filepath.Join(task, "config.toml"))
	if err != nil {
		t.Fatal(err)
	}
	var config struct {
		Model  string
		Skills struct {
			Config []struct {
				Path    string
				Enabled bool
			}
		}
	}
	if err := toml.Unmarshal(data, &config); err != nil {
		t.Fatal(err)
	}
	if config.Model != "unchanged" || len(config.Skills.Config) != 2 {
		t.Fatalf("unexpected materialized config: model=%q entries=%d", config.Model, len(config.Skills.Config))
	}
	for i, file := range []string{filepath.Join(install, "skills", "folder", "SKILL.md"), filepath.Join(shared, "plugins", "cache", "other", "review", "1.0.0", "extra", "folder", "SKILL.md")} {
		want, err := filepath.EvalSymlinks(file)
		if err != nil || config.Skills.Config[i].Path != filepath.ToSlash(want) || config.Skills.Config[i].Enabled {
			t.Fatalf("entry %d does not disable the actual cached skill: %v", i, err)
		}
	}
	if data, err := os.ReadFile(filepath.Join(shared, "config.toml")); err != nil || string(data) != "# shared configuration must stay unchanged\n" {
		t.Fatal("shared config changed")
	}
}

func TestCodexPluginSkillPolicyRejectsInvalidSelectionWithoutWriting(t *testing.T) {
	for _, tc := range []struct{ name, plugin, key string }{
		{"missing-plugin", "", "review@local:skills/folder"},
		{"other-marketplace", "review@missing", "review@missing:skills/folder"},
		{"wrong-prefix", "review@local", "review:skills/folder"},
		{"wrong-root", "review@local", "review@local:unknown/folder"},
		{"missing-skill", "review@local", "review@local:skills/missing"},
		{"parent-traversal", "review@local", "review@local:skills/../skills/folder"},
		{"dot-segment", "review@local", "review@local:skills/./folder"},
		{"backslash", "review@local", `review@local:skills\folder`},
		{"control", "review@local", "review@local:skills/fol\nder"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			task, _, _ := codexPluginPolicyFixture(t)
			configPath := filepath.Join(task, "config.toml")
			before, err := os.ReadFile(configPath)
			if err != nil {
				t.Fatal(err)
			}
			refs := []RuntimeSkillRefForEnv{{Root: "provider", Key: "valid"}, {Root: "plugin", Plugin: tc.plugin, Key: tc.key}}
			if _, err := ensureCodexDisabledSkillsConfig(configPath, task, refs, nil); err == nil {
				t.Fatal("invalid selected plugin was silently ignored")
			}
			if after, err := os.ReadFile(configPath); err != nil || !reflect.DeepEqual(before, after) {
				t.Fatal("invalid selection changed config before all paths were validated")
			}
		})
	}
}

func TestCodexPluginSkillPolicyRejectsUnavailableOrUnsafeFiles(t *testing.T) {
	for _, scenario := range []string{"missing-config", "symlink-skill", "symlink-directory", "symlink-config", "shared-home"} {
		t.Run(scenario, func(t *testing.T) {
			task, shared, install := codexPluginPolicyFixture(t)
			configPath := filepath.Join(task, "config.toml")
			var err error
			switch scenario {
			case "missing-config":
				err = os.Remove(configPath)
			case "symlink-skill", "symlink-directory", "symlink-config":
				target := filepath.Join(install, "skills", "folder", "SKILL.md")
				if scenario == "symlink-directory" {
					target = filepath.Dir(target)
				} else if scenario == "symlink-config" {
					target = configPath
				}
				moved := target + ".original"
				if err := os.Rename(target, moved); err != nil {
					t.Fatal(err)
				}
				err = os.Symlink(moved, target)
			case "shared-home":
				data, readErr := os.ReadFile(configPath)
				if readErr != nil {
					t.Fatal(readErr)
				}
				task, configPath = shared, filepath.Join(shared, "config.toml")
				err = os.WriteFile(configPath, data, 0o600)
			}
			if err != nil {
				t.Fatal(err)
			}
			before, _ := os.ReadFile(configPath)
			ref := RuntimeSkillRefForEnv{Root: "plugin", Plugin: "review@local", Key: "review@local:skills/folder"}
			if _, err := ensureCodexDisabledSkillsConfig(configPath, task, []RuntimeSkillRefForEnv{ref}, nil); err == nil {
				t.Fatal("unavailable or unsafe selected plugin accepted")
			}
			after, _ := os.ReadFile(configPath)
			if !reflect.DeepEqual(before, after) {
				t.Fatal("rejected configuration changed")
			}
		})
	}
}

func TestCodexPluginSkillPolicyPluginRootRelativeKeys(t *testing.T) {
	_, _, install := codexPluginPolicyFixture(t)
	canonical, err := filepath.EvalSymlinks(install)
	if err != nil {
		t.Fatal(err)
	}
	roots := []agent.CodexPluginSkillRoot{{PluginID: "review@local", Name: "unused-display-name", Path: canonical, RelativePath: "."}}
	ref := RuntimeSkillRefForEnv{Root: "plugin", Plugin: "review@local", Key: "review@local:skills/folder", Name: "../../ignored"}
	got, err := disabledCodexPluginSkillPath(roots, ref)
	if err != nil || got != filepath.Join(canonical, "skills", "folder", "SKILL.md") {
		t.Fatalf("plugin-root-relative key did not resolve: %v", err)
	}
	ref.Key = "review@local:./skills/folder"
	if _, err := disabledCodexPluginSkillPath(roots, ref); err == nil {
		t.Fatal("dot-prefixed plugin key accepted")
	}
}

func TestCodexPluginSkillPolicyUsesVerifiedPartialRoots(t *testing.T) {
	for _, selectUnverified := range []bool{false, true} {
		task, shared, install := codexPluginPolicyFixture(t)
		manifest := filepath.Join(shared, "plugins", "cache", "other", "review", "1.0.0", ".codex-plugin", "plugin.json")
		if err := os.WriteFile(manifest, []byte(`{"name":"review","skills":["./extra"]}`), 0o600); err != nil {
			t.Fatal(err)
		}
		configPath := filepath.Join(task, "config.toml")
		before, err := os.ReadFile(configPath)
		if err != nil {
			t.Fatal(err)
		}
		refs := []RuntimeSkillRefForEnv{{Root: "plugin", Plugin: "review@local", Key: "review@local:skills/folder"}}
		if selectUnverified {
			refs = append(refs, RuntimeSkillRefForEnv{Root: "plugin", Plugin: "review@other", Key: "review@other:extra/folder"})
		}
		_, err = ensureCodexDisabledSkillsConfig(configPath, task, refs, nil)
		if (err != nil) != selectUnverified {
			t.Fatalf("selected unverified=%v, error=%v", selectUnverified, err)
		}
		after, err := os.ReadFile(configPath)
		if err != nil {
			t.Fatal(err)
		}
		if selectUnverified {
			if !reflect.DeepEqual(before, after) {
				t.Fatal("unverified selected plugin changed config")
			}
			continue
		}
		var parsed struct {
			Skills struct {
				Config []struct {
					Path    string
					Enabled *bool
				}
			}
		}
		canonical, err := filepath.EvalSymlinks(filepath.Join(install, "skills", "folder", "SKILL.md"))
		if err != nil || toml.Unmarshal(after, &parsed) != nil || len(parsed.Skills.Config) != 1 {
			t.Fatal("verified selection was not materialized")
		}
		entry := parsed.Skills.Config[0]
		if entry.Path != filepath.ToSlash(canonical) || entry.Enabled == nil || *entry.Enabled {
			t.Fatal("verified selection did not disable the exact skill")
		}
	}
}

func TestCodexPluginSkillPolicyRetainsInactiveInstallation(t *testing.T) {
	for _, disabledKey := range []string{"plugins = true", "enabled = true"} {
		task, _, install := codexPluginPolicyFixture(t)
		configPath := filepath.Join(task, "config.toml")
		before, err := os.ReadFile(configPath)
		if err != nil {
			t.Fatal(err)
		}
		before = []byte(strings.ReplaceAll(string(before), disabledKey, strings.ReplaceAll(disabledKey, "true", "false")))
		if err := os.WriteFile(configPath, before, 0o600); err != nil {
			t.Fatal(err)
		}
		bindings, err := ensureCodexDisabledSkillsConfig(configPath, task, []RuntimeSkillRefForEnv{
			{Root: "plugin", Plugin: "review@local", Key: "review@local:skills/folder"},
		}, nil)
		canonical, pathErr := filepath.EvalSymlinks(filepath.Join(install, "skills", "folder", "SKILL.md"))
		if err != nil || pathErr != nil || len(bindings) != 1 || bindings[0].SkillPath != canonical || bindings[0].InstallationDigest == "" {
			t.Fatalf("inactive installed selection was not bound: %v", err)
		}
		after, err := os.ReadFile(configPath)
		if err != nil || !strings.HasPrefix(string(after), string(before)) {
			t.Fatal("plugin enablement configuration was changed")
		}
	}
}
