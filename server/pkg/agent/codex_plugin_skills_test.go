//go:build darwin || linux

package agent

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"
)

func pluginSkillFixture(t *testing.T) (string, string) {
	t.Helper()
	home, root := cachePluginFixture(t)
	cachePluginWrite(t, filepath.Join(home, "config.toml"), "[plugins.'probe@local']\nenabled = true\n")
	if err := os.MkdirAll(filepath.Join(root, "skills", "custom"), 0o700); err != nil {
		t.Fatal(err)
	}
	return home, root
}

func TestCodexPluginSkillRootsDefaultAndManifest(t *testing.T) {
	for _, tc := range []struct {
		name, manifest, relative string
	}{
		{"default", `{"name":"probe"}`, "skills"},
		{"explicit default", `{"name":"probe","skills":"./skills/"}`, "skills"},
		{"custom replaces default", `{"name":"probe","skills":"./skills/custom"}`, "skills/custom"},
		{"MCP independent", `{"name":"probe","skills":"./skills/custom","mcpServers":{"inline":{"command":"unused"}}}`, "skills/custom"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			home, root := pluginSkillFixture(t)
			manifest := filepath.Join(root, ".codex-plugin", "plugin.json")
			cachePluginWrite(t, manifest, tc.manifest)
			cachePluginWrite(t, filepath.Join(root, ".mcp.json"), "not JSON; unused by skill discovery")
			roots, err := CodexPluginSkillRoots(home)
			resolved, resolveErr := ResolveCodexPluginSkillRoot(home, "probe@local")
			if resolveErr != nil || resolved.InstallationDigest == "" {
				t.Fatal("installed root digest is unavailable")
			}
			want := []CodexPluginSkillRoot{{PluginID: "probe@local", Name: "probe", Path: filepath.Join(root, filepath.FromSlash(tc.relative)), RelativePath: tc.relative, InstallationDigest: resolved.InstallationDigest}}
			if err != nil || !reflect.DeepEqual(roots, want) {
				t.Fatalf("roots=%+v err=%v, want %+v", roots, err, want)
			}
			data, err := os.ReadFile(manifest)
			if err != nil || string(data) != tc.manifest {
				t.Fatal("skill discovery modified the manifest")
			}
			assertNoCachePluginMetadata(t, home)
		})
	}
}

func TestCodexPluginSkillRootsConfig(t *testing.T) {
	for _, tc := range []struct {
		name, config string
		count        int
		invalid      bool
	}{
		{"empty", "", 0, false},
		{"no plugins", "model = 'unused'\n", 0, false},
		{"enabled", "[plugins.'probe@local']\nenabled = true\n", 1, false},
		{"disabled plugin", "[plugins.'probe@local']\nenabled = false\n", 0, false},
		{"enabled feature", "[features]\nplugins = true\n[plugins.'probe@local']\nenabled = true\n", 1, false},
		{"disabled feature", "[features]\nplugins = false\n[plugins.'probe@local']\nenabled = true\n", 0, false},
		{"missing enabled", "[plugins.'probe@local']\n", 0, true},
		{"invalid enabled", "[plugins.'probe@local']\nenabled = 'true'\n", 0, true},
		{"numeric enabled", "[plugins.'probe@local']\nenabled = 1\n", 0, true},
		{"invalid entry", "[plugins]\n'probe@local' = true\n", 0, true},
		{"invalid table", "plugins = []\n", 0, true},
		{"invalid feature", "[features]\nplugins = 'false'\n", 0, true},
		{"invalid features table", "features = false\n", 0, true},
		{"malformed", "[plugins\nsynthetic-private-value", 0, true},
		{"duplicate entry", "[plugins.'probe@local']\nenabled = true\nenabled = false\n", 0, true},
	} {
		t.Run(tc.name, func(t *testing.T) {
			home, _ := pluginSkillFixture(t)
			cachePluginWrite(t, filepath.Join(home, "config.toml"), tc.config)
			roots, err := CodexPluginSkillRoots(home)
			if len(roots) != tc.count || (err != nil) != tc.invalid {
				t.Fatalf("roots=%+v err=%v", roots, err)
			}
			if err != nil && (strings.Contains(err.Error(), home) || strings.Contains(err.Error(), "synthetic-private-value") || len(err.Error()) > 128) {
				t.Fatal("discovery exposed unbounded/private error details")
			}
		})
	}
}

func TestCodexPluginSkillRootsMissingAndDisabledInputs(t *testing.T) {
	for _, tc := range []string{"config", "default skills", "disabled unavailable cache"} {
		t.Run(tc, func(t *testing.T) {
			home, root := pluginSkillFixture(t)
			switch tc {
			case "config":
				if err := os.Remove(filepath.Join(home, "config.toml")); err != nil {
					t.Fatal(err)
				}
			case "default skills":
				if err := os.RemoveAll(filepath.Join(root, "skills")); err != nil {
					t.Fatal(err)
				}
			case "disabled unavailable cache":
				cachePluginWrite(t, filepath.Join(home, "config.toml"), "[features]\nplugins = false\n[plugins.'probe@local']\nenabled = true\n")
				if err := os.RemoveAll(root); err != nil {
					t.Fatal(err)
				}
			}
			roots, err := CodexPluginSkillRoots(home)
			if err != nil || len(roots) != 0 {
				t.Fatalf("roots=%+v err=%v", roots, err)
			}
		})
	}
}

func TestCodexPluginSkillRootsRejectsUnverifiedManifestPaths(t *testing.T) {
	for _, skills := range []any{nil, false, 1, "", "skills/custom", "../outside", "./../outside", "./skills/../custom", "/tmp/outside", "./", "./skills\\custom", "./skills/\x00bad", "./missing", "./.mcp.json", []string{"./skills"}, []string{"./skills", "./custom"}, map[string]any{"path": "./skills"}} {
		raw, err := json.Marshal(map[string]any{"name": "probe", "skills": skills})
		if err != nil {
			t.Fatal(err)
		}
		t.Run(string(raw), func(t *testing.T) {
			home, root := pluginSkillFixture(t)
			cachePluginWrite(t, filepath.Join(root, ".codex-plugin", "plugin.json"), string(raw))
			roots, err := CodexPluginSkillRoots(home)
			if err == nil || len(roots) != 0 {
				t.Fatalf("unverified root accepted: %+v err=%v", roots, err)
			}
		})
	}
}

func TestCodexPluginSkillRootsRejectsUnsafeCache(t *testing.T) {
	for _, variant := range []string{"missing cache", "multiple versions", "skill symlink", "manifest symlink", "marketplace symlink", "name mismatch", "name traversal", "malformed manifest", "missing manifest", "ID traversal", "config symlink", "oversized config"} {
		t.Run(variant, func(t *testing.T) {
			home, root := pluginSkillFixture(t)
			manifest := filepath.Join(root, ".codex-plugin", "plugin.json")
			switch variant {
			case "missing cache":
				if err := os.RemoveAll(root); err != nil {
					t.Fatal(err)
				}
			case "multiple versions":
				if err := os.Mkdir(filepath.Join(filepath.Dir(root), "2.0.0"), 0o700); err != nil {
					t.Fatal(err)
				}
			case "skill symlink":
				if err := os.Symlink(t.TempDir(), filepath.Join(root, "skills", "linked")); err != nil {
					t.Fatal(err)
				}
			case "manifest symlink", "config symlink":
				file := manifest
				if variant == "config symlink" {
					file = filepath.Join(home, "config.toml")
				}
				if err := os.Rename(file, file+".original"); err != nil {
					t.Fatal(err)
				}
				if err := os.Symlink(file+".original", file); err != nil {
					t.Fatal(err)
				}
			case "marketplace symlink":
				market := filepath.Dir(filepath.Dir(root))
				if err := os.Rename(market, market+".original"); err != nil {
					t.Fatal(err)
				}
				if err := os.Symlink(market+".original", market); err != nil {
					t.Fatal(err)
				}
			case "name mismatch":
				cachePluginWrite(t, manifest, `{"name":"different"}`)
			case "name traversal":
				cachePluginWrite(t, manifest, `{"name":"../outside"}`)
			case "malformed manifest":
				cachePluginWrite(t, manifest, `{"name":"probe",`)
			case "missing manifest":
				if err := os.Remove(manifest); err != nil {
					t.Fatal(err)
				}
			case "ID traversal":
				cachePluginWrite(t, filepath.Join(home, "config.toml"), "[plugins.'../probe@local']\nenabled = true\n")
			case "oversized config":
				cachePluginWrite(t, filepath.Join(home, "config.toml"), strings.Repeat("#", maxCodexProjectPolicyConfigBytes+1))
			}
			roots, err := CodexPluginSkillRoots(home)
			if err == nil || len(roots) != 0 {
				t.Fatalf("unsafe cache accepted: %+v err=%v", roots, err)
			}
		})
	}
}

func TestCodexPluginSkillRootsPartialAndStableOrder(t *testing.T) {
	home, local := pluginSkillFixture(t)
	other := filepath.Join(home, "plugins", "cache", "another", "probe", "local")
	if err := os.MkdirAll(filepath.Join(other, ".claude-plugin"), 0o700); err != nil {
		t.Fatal(err)
	}
	if err := os.MkdirAll(filepath.Join(other, "skills"), 0o700); err != nil {
		t.Fatal(err)
	}
	cachePluginWrite(t, filepath.Join(other, ".claude-plugin", "plugin.json"), `{"name":"probe"}`)
	cachePluginWrite(t, filepath.Join(home, "config.toml"), "[plugins.'probe@local']\nenabled = true\n[plugins.'probe@another']\nenabled = true\n[plugins.'unavailable@local']\nenabled = true\n[plugins.'unverified@local']\n[plugins.'ignored@local']\nenabled = false\n")
	want := []CodexPluginSkillRoot{
		{PluginID: "probe@another", Name: "probe", Path: filepath.Join(other, "skills"), RelativePath: "skills"},
		{PluginID: "probe@local", Name: "probe", Path: filepath.Join(local, "skills"), RelativePath: "skills"},
	}
	for index := range want {
		resolved, err := ResolveCodexPluginSkillRoot(home, want[index].PluginID)
		if err != nil || resolved.InstallationDigest == "" {
			t.Fatal("installed root digest is unavailable")
		}
		want[index].InstallationDigest = resolved.InstallationDigest
	}
	for range 5 {
		roots, err := CodexPluginSkillRoots(home)
		if err == nil || !reflect.DeepEqual(roots, want) {
			t.Fatalf("partial roots=%+v err=%v, want %+v", roots, err, want)
		}
	}
}

func TestCodexPluginSkillRootsSharedCacheLink(t *testing.T) {
	home, root := pluginSkillFixture(t)
	shared := filepath.Join(t.TempDir(), "shared-cache")
	cache := filepath.Join(home, "plugins", "cache")
	if err := os.Rename(cache, shared); err != nil {
		t.Fatal(err)
	}
	if err := os.Symlink(shared, cache); err != nil {
		t.Fatal(err)
	}
	roots, err := CodexPluginSkillRoots(home)
	expected, resolveErr := filepath.EvalSymlinks(filepath.Join(root, "skills"))
	if err != nil || resolveErr != nil || len(roots) != 1 || roots[0].Path != expected {
		t.Fatalf("shared root=%+v err=%v", roots, err)
	}
}

func TestCodexPluginSkillRootsManifestLocations(t *testing.T) {
	for _, directory := range []string{".claude-plugin", ".cursor-plugin"} {
		t.Run(directory, func(t *testing.T) {
			home, root := pluginSkillFixture(t)
			if err := os.Rename(filepath.Join(root, ".codex-plugin"), filepath.Join(root, directory)); err != nil {
				t.Fatal(err)
			}
			cachePluginWrite(t, filepath.Join(root, directory, "plugin.json"), `{"name":"probe","skills":"./skills/custom"}`)
			roots, err := CodexPluginSkillRoots(home)
			if err != nil || len(roots) != 1 || roots[0].RelativePath != "skills/custom" {
				t.Fatalf("legacy manifest roots=%+v err=%v", roots, err)
			}
		})
	}
}

func TestCodexPluginSkillRootsEntryLimit(t *testing.T) {
	home, _ := pluginSkillFixture(t)
	var config strings.Builder
	for index := range 128 {
		fmt.Fprintf(&config, "[plugins.'a%03d@local']\nenabled = false\n", index)
	}
	config.WriteString("[plugins.'probe@local']\nenabled = true\n")
	cachePluginWrite(t, filepath.Join(home, "config.toml"), config.String())
	roots, err := CodexPluginSkillRoots(home)
	if err == nil || len(roots) != 0 {
		t.Fatalf("entry budget exceeded without an error: %+v err=%v", roots, err)
	}
}

func TestCodexPluginSkillRootsHomeBoundary(t *testing.T) {
	for _, home := range []string{"", ".", "relative/codex"} {
		if roots, err := CodexPluginSkillRoots(home); err == nil || len(roots) != 0 {
			t.Fatalf("nonabsolute home accepted: %+v err=%v", roots, err)
		}
	}
	missing := filepath.Join(t.TempDir(), "not-created")
	if roots, err := CodexPluginSkillRoots(missing); err != nil || len(roots) != 0 {
		t.Fatalf("missing config is not an empty result: %+v err=%v", roots, err)
	}
}
