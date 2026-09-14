//go:build darwin || linux

package agent

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestCodexPluginSkillPolicySelectionLimit128(t *testing.T) {
	cfg, opts := pluginMCPTestOptions(t)
	home, installed := pluginSkillFixture(t)
	cfg.Env["CODEX_HOME"] = home
	cfg.CLIVersion = "codex-cli 0.153.4"
	opts.RuntimeMCPSelection = nil
	var config strings.Builder
	config.WriteString("[plugins.'probe@local']\nenabled = true\n")
	for index := 0; index < 129; index++ {
		name := fmt.Sprintf("skill-%03d", index)
		path := filepath.Join(installed, "skills", name, "SKILL.md")
		if err := os.MkdirAll(filepath.Dir(path), 0o700); err != nil {
			t.Fatal(err)
		}
		cachePluginWrite(t, path, fmt.Sprintf("---\nname: %s\ndescription: synthetic policy fixture\n---\nLocal fixture only.\n", name))
		key := "probe@local:skills/" + name
		opts.CodexPluginSkillSelections = append(opts.CodexPluginSkillSelections, CodexPluginSkillSelection{PluginID: "probe@local", Key: key})
		opts.CodexPluginSkillBindings = append(opts.CodexPluginSkillBindings, CodexPluginSkillBinding{PluginID: "probe@local", Key: key, SkillPath: path})
		fmt.Fprintf(&config, "[[skills.config]]\npath = %q\nenabled = false\n", path)
	}
	cachePluginWrite(t, filepath.Join(home, "config.toml"), config.String())
	root, err := ResolveCodexPluginSkillRoot(home, "probe@local")
	if err != nil || root.InstallationDigest == "" {
		t.Fatalf("fixture installation was not verified: %v", err)
	}
	for index := range opts.CodexPluginSkillBindings {
		opts.CodexPluginSkillBindings[index].InstallationDigest = root.InstallationDigest
	}
	backend := &codexBackend{cfg: cfg}
	for _, tc := range []struct {
		name                 string
		selections, bindings int
		rejected             bool
	}{
		{"128 complete bindings accepted", 128, 128, false},
		{"129 complete bindings rejected", 129, 129, true},
		{"128 missing bindings rejected", 128, 0, true},
		{"128 truncated bindings rejected", 128, 127, true},
	} {
		t.Run(tc.name, func(t *testing.T) {
			input := opts
			input.CodexPluginSkillSelections = input.CodexPluginSkillSelections[:tc.selections]
			input.CodexPluginSkillBindings = input.CodexPluginSkillBindings[:tc.bindings]
			if tc.bindings == 0 {
				input.CodexPluginSkillBindings = nil
			}
			prepared, err := backend.prepareCodexPluginSkillPolicy(input)
			if tc.rejected {
				if err != errCodexPluginSkillPolicy {
					t.Fatalf("expected policy rejection, got %v", err)
				}
				return
			}
			if err != nil {
				t.Fatalf("complete 128-selection policy was rejected: %v", err)
			}
			if prepared.codexPluginSkills == nil || len(prepared.codexPluginSkills.bindings) != tc.bindings {
				t.Fatal("prepared policy did not retain all bindings")
			}
		})
	}
}

func TestCodexPluginSkillPolicySupported(t *testing.T) {
	for _, version := range []string{"0.153.4", "codex-cli 0.153.4", "  codex-cli 0.153.4  "} {
		if !CodexPluginSkillsSupported(version, true) || CodexPluginSkillsSupported(version, false) {
			t.Fatalf("incorrect built-in version gate for %q", version)
		}
	}
	for _, version := range []string{"", "0.153.3", "0.153.5", "v0.153.4", "codex-cli 0.153.4-custom", "codex-cli 0.153.4 extra"} {
		if CodexPluginSkillsSupported(version, true) {
			t.Fatalf("unsupported version accepted: %q", version)
		}
	}
}

func TestCodexPluginSkillPolicyResolveInactiveInstallation(t *testing.T) {
	home, installed := pluginSkillFixture(t)
	cachePluginWrite(t, filepath.Join(home, "config.toml"), "[features]\nplugins = false\n[plugins.'probe@local']\nenabled = false\n")
	root, err := ResolveCodexPluginSkillRoot(home, "probe@local")
	if err != nil || root.Path != filepath.Join(installed, "skills") || !strings.HasPrefix(root.InstallationDigest, "sha256:") || len(root.InstallationDigest) != 71 {
		t.Fatalf("installed inactive root was not bound: %+v %v", root, err)
	}
	again, err := ResolveCodexPluginSkillRoot(home, "probe@local")
	if err != nil || root != again {
		t.Fatal("stable installation produced unstable binding")
	}
	if roots, err := CodexPluginSkillRoots(home); err != nil || len(roots) != 0 {
		t.Fatal("active listing changed for disabled plugin")
	}
	if _, err := ResolveCodexPluginSkillRoot(home, "missing@local"); err == nil {
		t.Fatal("uninstalled selection accepted")
	}
	assertNoCachePluginMetadata(t, home)
}

func TestCodexPluginSkillPolicyInstallationDigestDrift(t *testing.T) {
	for _, variant := range []string{"manifest", "version", "root-directory", "relative-root"} {
		t.Run(variant, func(t *testing.T) {
			home, installed := pluginSkillFixture(t)
			before, err := ResolveCodexPluginSkillRoot(home, "probe@local")
			if err != nil {
				t.Fatal(err)
			}
			switch variant {
			case "manifest":
				cachePluginWrite(t, filepath.Join(installed, ".codex-plugin", "plugin.json"), `{"name":"probe","version":"1.0.0","description":"changed"}`)
			case "version":
				if err := os.Rename(installed, filepath.Join(filepath.Dir(installed), "2.0.0")); err != nil {
					t.Fatal(err)
				}
			case "root-directory":
				if err := os.Rename(filepath.Join(installed, "skills"), filepath.Join(installed, "old-skills")); err != nil {
					t.Fatal(err)
				}
				if err := os.Mkdir(filepath.Join(installed, "skills"), 0o700); err != nil {
					t.Fatal(err)
				}
			case "relative-root":
				cachePluginWrite(t, filepath.Join(installed, ".codex-plugin", "plugin.json"), `{"name":"probe","skills":"./skills/custom"}`)
			}
			after, err := ResolveCodexPluginSkillRoot(home, "probe@local")
			if err != nil || before.InstallationDigest == after.InstallationDigest {
				t.Fatalf("installation drift was not distinguished: %v", err)
			}
		})
	}
}
