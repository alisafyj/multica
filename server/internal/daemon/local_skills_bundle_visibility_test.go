package daemon

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestRuntimeLocalSkillsRetainUnimportableCandidates(t *testing.T) {
	for _, provider := range []string{"codex", "claude", "opencode"} {
		for _, limit := range []string{"file_count", "bundle_bytes"} {
			t.Run(provider+"/"+limit, func(t *testing.T) {
				home := t.TempDir()
				t.Setenv("HOME", home)
				t.Setenv("CODEX_HOME", filepath.Join(home, ".codex"))
				roots, supported, err := localSkillRootsForProvider(provider)
				if err != nil || !supported || len(roots) == 0 {
					t.Fatalf("roots: supported=%v err=%v", supported, err)
				}
				files := map[string]string{"SKILL.md": "---\nname: large-skill\ndescription: Large native skill\n---\n"}
				count, content := maxLocalSkillFileCount+1, "support\n"
				if limit == "bundle_bytes" {
					count = int(maxLocalSkillBundleSize/maxLocalSkillFileSize) + 1
					content = strings.Repeat("x", int(maxLocalSkillFileSize))
				}
				for i := range count {
					files[fmt.Sprintf("support/%03d.md", i)] = content
				}
				writeTestLocalSkill(t, roots[0].path, "large-skill", files)
				writeTestLocalSkill(t, roots[0].path, "small-skill", map[string]string{"SKILL.md": "---\nname: small-skill\ndescription: Small native skill\n---\n"})
				skills, supported, err := listRuntimeLocalSkills(provider)
				if err != nil || !supported || len(skills) != 2 || skills[0].Key != "large-skill" {
					t.Fatalf("oversized candidate disappeared: skills=%+v supported=%v err=%v", skills, supported, err)
				}
				if skills[0].CanDisable != (provider == "codex" || provider == "claude") {
					t.Fatal("import failure changed native control capability")
				}
				if skills[0].CanImport == nil || *skills[0].CanImport || skills[0].FileCount != 0 {
					t.Fatal("unavailable bundle was reported as importable or completely counted")
				}
				if skills[1].CanImport == nil || !*skills[1].CanImport || skills[1].FileCount != 1 {
					t.Fatal("ordinary importable bundle contract changed")
				}
				if _, _, err := loadRuntimeLocalSkillBundle(provider, "large-skill"); err == nil {
					t.Fatal("oversized import bypassed the existing limit")
				}
			})
		}
	}
}

func TestLocalSkillBundleKnownHostDiagnostic(t *testing.T) {
	if os.Getenv("MULTICA_TEST_LOCAL_SKILL_BUNDLE_DIAGNOSTIC") != "1" {
		t.Skip("opt-in read-only diagnostic of the known missing host skill")
	}
	home, err := os.UserHomeDir()
	if err != nil {
		t.Fatal("user home unavailable")
	}
	dir := filepath.Join(home, ".codex", "skills", "ppt-master")
	main, err := readLocalSkillMainFile(dir)
	if err != nil {
		t.Fatal("known skill main file is not readable")
	}
	_, err = collectLocalSkillFiles(dir, false)
	switch {
	case err == nil:
		t.Fatal("bundle-limit hypothesis not reproduced")
	case err.Error() == fmt.Sprintf("local skill exceeds %d files", maxLocalSkillFileCount):
		t.Logf("known skill=ppt-master main_bytes=%d bundle_rejection=file_count_limit", len(main))
	case err.Error() == fmt.Sprintf("local skill exceeds %d bytes in total", maxLocalSkillBundleSize):
		t.Logf("known skill=ppt-master main_bytes=%d bundle_rejection=byte_limit", len(main))
	default:
		t.Fatal("bundle failed for an unclassified reason")
	}
}
