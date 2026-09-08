package daemon

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestRuntimeLocalSkillsCodexPluginIdentity(t *testing.T) {
	home := t.TempDir()
	codexHome := filepath.Join(home, ".codex")
	t.Setenv("HOME", home)
	t.Setenv("CODEX_HOME", codexHome)
	for _, marketplace := range []string{"first", "second"} {
		root := filepath.Join(codexHome, "plugins", "cache", marketplace, "review", "1.0.0")
		writeTestLocalSkill(t, root, ".codex-plugin", map[string]string{
			"plugin.json": `{"name":"review","version":"1.0.0","skills":"./skills/"}`,
		})
		writeTestLocalSkill(t, root, "skills/check", map[string]string{
			"SKILL.md": "---\nname: native-check\ndescription: Synthetic review skill\n---\n" + marketplace,
		})
	}
	config := "[features]\nplugins = true\n[plugins.'review@first']\nenabled = true\n[plugins.'review@second']\nenabled = true\n"
	if err := os.WriteFile(filepath.Join(codexHome, "config.toml"), []byte(config), 0o600); err != nil {
		t.Fatal(err)
	}

	skills, supported, err := listRuntimeLocalSkills("codex")
	if err != nil || !supported || len(skills) != 2 {
		t.Fatalf("skills=%+v supported=%v err=%v", skills, supported, err)
	}
	for i, marketplace := range []string{"first", "second"} {
		got := skills[i]
		id := "review@" + marketplace
		if got.Key != id+":skills/check" || got.Name != "review:native-check" || got.Plugin != id || got.Root != localSkillRootPlugin {
			t.Fatalf("plugin identity=%+v", got)
		}
		bundle, supported, err := loadRuntimeLocalSkillBundle("codex", got.Key)
		if err != nil || !supported || bundle == nil || bundle.Name != got.Name || !strings.HasSuffix(bundle.Content, marketplace) {
			t.Fatalf("bundle=%+v supported=%v err=%v", bundle, supported, err)
		}
	}
}

func TestRuntimeLocalSkillsCodexDisabledPlugin(t *testing.T) {
	for _, config := range []string{
		"[features]\nplugins = false\n[plugins.'review@local']\nenabled = true\n",
		"[features]\nplugins = true\n[plugins.'review@local']\nenabled = false\n",
	} {
		home := t.TempDir()
		codexHome := filepath.Join(home, ".codex")
		t.Setenv("HOME", home)
		t.Setenv("CODEX_HOME", codexHome)
		root := filepath.Join(codexHome, "plugins", "cache", "local", "review", "1.0.0")
		writeTestLocalSkill(t, root, ".codex-plugin", map[string]string{
			"plugin.json": `{"name":"review","version":"1.0.0","skills":"./skills/"}`,
		})
		writeTestLocalSkill(t, root, "skills/check", map[string]string{"SKILL.md": "---\nname: check\n---\n"})
		writeTestLocalSkill(t, codexHome, "skills/personal", map[string]string{"SKILL.md": "---\nname: personal\n---\n"})
		if err := os.WriteFile(filepath.Join(codexHome, "config.toml"), []byte(config), 0o600); err != nil {
			t.Fatal(err)
		}
		skills, supported, err := listRuntimeLocalSkills("codex")
		if err != nil || !supported || len(skills) != 1 || skills[0].Key != "personal" {
			t.Fatalf("skills=%+v supported=%v err=%v", skills, supported, err)
		}
	}
}

func TestRuntimeLocalSkillsCodexPluginDirectRoot(t *testing.T) {
	for _, skillsRoot := range []string{"./skills/", "./skills/check"} {
		for _, oversized := range []bool{false, true} {
			home := t.TempDir()
			codexHome := filepath.Join(home, ".codex")
			t.Setenv("HOME", home)
			t.Setenv("CODEX_HOME", codexHome)
			root := filepath.Join(codexHome, "plugins", "cache", "local", "review", "1.0.0")
			manifest, err := json.Marshal(map[string]string{"name": "review", "version": "1.0.0", "skills": skillsRoot})
			if err != nil {
				t.Fatal(err)
			}
			writeTestLocalSkill(t, root, ".codex-plugin", map[string]string{
				"plugin.json": string(manifest),
			})
			content := "---\nname: native-check\ndescription: Synthetic direct-root skill\n---\nSelected root"
			if oversized {
				content = strings.Repeat("x", int(maxLocalSkillFileSize)+1)
			}
			writeTestLocalSkill(t, root, "skills/check", map[string]string{"SKILL.md": content})
			writeTestLocalSkill(t, root, "skills/check/nested", map[string]string{"SKILL.md": "---\nname: nested\ndescription: Nested skill\n---\n"})
			writeTestLocalSkill(t, root, "other-skills/unselected", map[string]string{"SKILL.md": "---\nname: unselected\n---\n"})
			if err := os.WriteFile(filepath.Join(codexHome, "config.toml"), []byte("[plugins.'review@local']\nenabled = true\n"), 0o600); err != nil {
				t.Fatal(err)
			}
			skills, supported, err := listRuntimeLocalSkills("codex")
			wantCount := 2
			if oversized {
				wantCount = 1
			}
			if err != nil || !supported || len(skills) != wantCount {
				t.Fatalf("skills=%+v supported=%v err=%v", skills, supported, err)
			}
			nested := skills[len(skills)-1]
			if nested.Key != "review@local:skills/check/nested" || nested.Name != "review:nested" {
				t.Fatalf("nested identity=%+v", nested)
			}
			if oversized {
				continue
			}
			got := skills[0]
			if got.Key != "review@local:skills/check" || got.Name != "review:native-check" || !got.CanDisable || got.Plugin != "review@local" || got.Root != localSkillRootPlugin {
				t.Fatalf("direct-root identity=%+v", got)
			}
			bundle, supported, err := loadRuntimeLocalSkillBundle("codex", got.Key)
			if err != nil || !supported || bundle == nil || bundle.Name != got.Name || bundle.Content != content {
				t.Fatalf("bundle=%+v supported=%v err=%v", bundle, supported, err)
			}
		}
	}
}
