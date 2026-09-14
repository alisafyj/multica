package daemon

import (
	"os"
	"path/filepath"
	"reflect"
	"testing"
)

func TestRuntimeLocalSkillsCodexNestedParentAndChild(t *testing.T) {
	for _, source := range []string{"provider", "universal"} {
		t.Run(source, func(t *testing.T) {
			home := t.TempDir()
			t.Setenv("HOME", home)
			t.Setenv("CODEX_HOME", filepath.Join(home, ".codex"))
			root := filepath.Join(home, ".codex", "skills")
			if source == "universal" {
				root = filepath.Join(home, ".agents", "skills")
			}
			const childKey = "slides/plugins/slides/skills/slides"
			parent := "---\nname: slides\ndescription: Parent workflow\n---\nParent body\n"
			child := "---\nname: slides\ndescription: Nested workflow\n---\nChild body\n"
			writeTestLocalSkill(t, root, "slides", map[string]string{"SKILL.md": parent, "guide.md": "Parent guide\n"})
			writeTestLocalSkill(t, root, childKey, map[string]string{"SKILL.md": child})
			writeTestLocalSkill(t, root, "unrelated", map[string]string{"SKILL.md": "---\nname: unrelated\ndescription: Other workflow\n---\n"})

			skills, supported, err := listRuntimeLocalSkills("codex")
			if err != nil || !supported {
				t.Fatalf("list: supported=%v err=%v", supported, err)
			}
			var keys []string
			for _, item := range skills {
				keys = append(keys, item.Key)
				if item.Root != source || !item.CanDisable || item.Provider != "codex" {
					t.Fatalf("selection identity changed: %+v", item)
				}
			}
			if want := []string{"slides", childKey, "unrelated"}; !reflect.DeepEqual(keys, want) {
				t.Fatalf("keys = %v, want %v", keys, want)
			}
			for key, content := range map[string]string{"slides": parent, childKey: child} {
				bundle, supported, err := loadRuntimeLocalSkillBundle("codex", key)
				if err != nil || !supported || bundle == nil || bundle.Content != content || bundle.Name != "slides" {
					t.Fatalf("list/load mismatch for %q: supported=%v err=%v", key, supported, err)
				}
				if key == "slides" && (len(bundle.Files) != 1 || bundle.Files[0].Path != "guide.md" || bundle.Files[0].Content != "Parent guide\n") {
					t.Fatalf("parent supporting-file contract changed: %+v", bundle.Files)
				}
			}
		})
	}
}

func TestRuntimeLocalSkillsCodexNestedTraversalGuards(t *testing.T) {
	home := t.TempDir()
	t.Setenv("HOME", home)
	t.Setenv("CODEX_HOME", filepath.Join(home, ".codex"))
	root := filepath.Join(home, ".codex", "skills")
	content := "---\nname: example\ndescription: Example workflow\n---\n"
	for _, key := range []string{"parent", "parent/child", "parent/.hidden/child", "parent/a/b/c/d/e/too-deep"} {
		writeTestLocalSkill(t, root, key, map[string]string{"SKILL.md": content})
	}
	if err := os.Symlink(filepath.Join(root, "parent"), filepath.Join(root, "parent", "cycle")); err != nil {
		t.Skipf("symlinks unavailable: %v", err)
	}
	skills, supported, err := listRuntimeLocalSkills("codex")
	if err != nil || !supported {
		t.Fatalf("list: supported=%v err=%v", supported, err)
	}
	var keys []string
	for _, item := range skills {
		keys = append(keys, item.Key)
	}
	if want := []string{"parent", "parent/child", "parent/cycle"}; !reflect.DeepEqual(keys, want) {
		t.Fatalf("bounded traversal keys = %v, want %v", keys, want)
	}
}

func TestRuntimeLocalSkillsCodexNestedCandidatesRemainImportable(t *testing.T) {
	home := t.TempDir()
	t.Setenv("HOME", home)
	t.Setenv("CODEX_HOME", filepath.Join(home, ".codex"))
	root := filepath.Join(home, ".codex", "skills")
	const invalid = "Template without native frontmatter\n"
	writeTestLocalSkill(t, root, "parent", map[string]string{"SKILL.md": invalid})
	writeTestLocalSkill(t, root, "parent/template", map[string]string{"SKILL.md": invalid})
	writeTestLocalSkill(t, root, "parent/template/valid", map[string]string{"SKILL.md": "---\ndescription: Native default name is allowed\n---\n"})
	skills, supported, err := listRuntimeLocalSkills("codex")
	if err != nil || !supported {
		t.Fatalf("list: supported=%v err=%v", supported, err)
	}
	var keys []string
	for _, item := range skills {
		keys = append(keys, item.Key)
		bundle, supported, err := loadRuntimeLocalSkillBundle("codex", item.Key)
		if err != nil || !supported || bundle == nil || bundle.Name != item.Name {
			t.Fatalf("candidate list/load mismatch for %q: supported=%v err=%v", item.Key, supported, err)
		}
	}
	if want := []string{"parent", "parent/template", "parent/template/valid"}; !reflect.DeepEqual(keys, want) {
		t.Fatalf("candidate inventory keys = %v, want %v", keys, want)
	}
}
