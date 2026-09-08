package projectdesignsystem

import (
	"context"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"
)

func TestGenerateProgrammaticFirstPackagePassesV2AuditAndIsDeterministic(t *testing.T) {
	repository := t.TempDir()
	writeProgrammaticFixture(t, repository, "styles/tokens.css", `:root {
  --brand-primary: #7c3aed;
  --page-background: #fafafa;
  --text-primary: #171717;
  --border-color: #e5e7eb;
  --radius-card: 12px;
  font-family: "Inter", sans-serif;
}
.card { color: #171717; border-radius: 12px; padding: 16px; gap: 8px; }
`)
	writeProgrammaticFixture(t, repository, "components/Button.tsx", `export function Button(){ return <button className="primary">Create</button> }`)
	writeProgrammaticFixture(t, repository, "components/ClinicCard.tsx", `export function ClinicCard(){ return <article>Clinic</article> }`)
	writeProgrammaticFixture(t, repository, "pages/home/index.tsx", `export default function Home(){ return <main>Home</main> }`)
	writeProgrammaticFixture(t, repository, "node_modules/ignored/styles.css", `:root { --brand-primary: #ff0000; }`)

	input := ProgrammaticInput{
		ProjectName:         "Clinic",
		RepositoryName:      "clinic-web",
		RepositoryURL:       "https://example.test/clinic-web.git",
		CommitSHA:           strings.Repeat("a", 40),
		Platform:            "web",
		Brief:               "Create a calm clinic workspace.",
		InputSnapshotSHA256: "sha256:" + strings.Repeat("b", 64),
	}
	binding := PackageBinding{
		WorkspaceID:         "workspace-1",
		ProjectID:           "project-1",
		DesignSystemID:      "design-system-1",
		TaskID:              "task-1",
		AgentID:             "agent-1",
		Operation:           "generate",
		InputSnapshotSHA256: input.InputSnapshotSHA256,
	}

	var progress []ProgrammaticProgress
	firstRoot := t.TempDir()
	first, err := GenerateProgrammaticFirstPackage(context.Background(), repository, firstRoot, input, func(item ProgrammaticProgress) {
		progress = append(progress, item)
	})
	if err != nil {
		t.Fatalf("GenerateProgrammaticFirstPackage() error = %v", err)
	}
	if len(first.SourceFiles) == 0 || first.ColorCount == 0 || first.ComponentCount < 2 || first.PagePatternCount == 0 {
		t.Fatalf("unexpected extraction result: %+v", first)
	}
	if len(progress) != 3 || progress[0].Stage != "inventory" || progress[1].Stage != "extraction" || progress[2].Stage != "package" {
		t.Fatalf("progress = %+v", progress)
	}
	collected, err := CollectV2Directory(firstRoot, binding)
	if err != nil {
		t.Fatalf("CollectV2Directory() rejected programmatic package: %v", err)
	}
	if !collected.Audit.Passed || len(collected.Manifest.PreviewTargets) != 2 {
		t.Fatalf("collected package = %+v", collected.Manifest)
	}
	if strings.Contains(string(mustReadProgrammaticFile(t, firstRoot, "tokens.css")), "#ff0000") {
		t.Fatal("ignored node_modules source affected generated tokens")
	}
	uiKit := string(mustReadProgrammaticFile(t, firstRoot, "ui-kit/index.html"))
	for _, expected := range []string{"按钮与标签状态", "服务者信息卡", "商品与内容卡", "导航与固定操作", "反馈与异常状态", "仓库来源映射"} {
		if !strings.Contains(uiKit, expected) {
			t.Fatalf("UI Kit is missing visual component section %q", expected)
		}
	}
	if strings.Contains(uiKit, "来源仓库的共享组件模式") {
		t.Fatal("UI Kit regressed to the generic component-name list")
	}
	patterns := string(mustReadProgrammaticFile(t, firstRoot, "preview/page-patterns.html"))
	for _, expected := range []string{"服务者详情页", "内容列表页", "加载空失败状态页"} {
		if !strings.Contains(patterns, expected) {
			t.Fatalf("page preview is missing %q", expected)
		}
	}

	secondRoot := t.TempDir()
	second, err := GenerateProgrammaticFirstPackage(context.Background(), repository, secondRoot, input, nil)
	if err != nil {
		t.Fatalf("second GenerateProgrammaticFirstPackage() error = %v", err)
	}
	secondCollected, err := CollectV2Directory(secondRoot, binding)
	if err != nil {
		t.Fatalf("second CollectV2Directory() error = %v", err)
	}
	if !reflect.DeepEqual(first, second) {
		t.Fatalf("results differ:\nfirst=%+v\nsecond=%+v", first, second)
	}
	if !reflect.DeepEqual(collected.Archive, secondCollected.Archive) {
		t.Fatal("same fixed input did not produce deterministic package bytes")
	}
}

func TestGenerateProgrammaticFirstPackageCanUseFrozenAnalysisWithoutCheckout(t *testing.T) {
	output := t.TempDir()
	input := ProgrammaticInput{
		ProjectName:         "Clinic",
		RepositoryName:      "clinic-web",
		CommitSHA:           strings.Repeat("c", 40),
		Platform:            "web",
		Brief:               "Use the frozen repository evidence.",
		InputSnapshotSHA256: "sha256:" + strings.Repeat("d", 64),
		RepositoryAnalysis: &RepositoryDesignContext{
			SchemaVersion: RepositoryDesignContextSchemaVersion,
			Summary:       "A mobile-first clinic product.",
			Facts: []RepositoryDesignFact{{
				Kind: "layout", Label: "Card layout", Value: "Cards use compact vertical rhythm.",
				SourcePaths: []string{"pages/home.tsx"}, Confidence: 0.9,
			}},
			SourceFiles: []RepositoryDesignSourceFile{{Path: "pages/home.tsx", Kind: "page"}},
			RepresentativeWorkflows: []RepositoryDesignWorkflow{{
				Name: "Clinic home", Purpose: "Entry", SourcePaths: []string{"pages/home.tsx"}, Confidence: 0.9,
				Regions: []RepositoryDesignRegion{{Name: "Actions", Controls: []string{"Appointment card"}}},
			}},
			Conflicts: []RepositoryDesignConflict{}, Confidence: 0.9,
		},
	}
	if _, err := GenerateProgrammaticFirstPackage(context.Background(), "", output, input, nil); err != nil {
		t.Fatalf("GenerateProgrammaticFirstPackage() error = %v", err)
	}
	if body := string(mustReadProgrammaticFile(t, output, "DESIGN.md")); !strings.Contains(body, "Clinic home") || !strings.Contains(body, "仓库证据：已复用") || strings.Contains(body, "Cards use compact vertical rhythm") {
		t.Fatalf("DESIGN.md did not keep raw analysis out of the user-facing view:\n%s", body)
	}
}

func writeProgrammaticFixture(t *testing.T, root, relative, body string) {
	t.Helper()
	path := filepath.Join(root, filepath.FromSlash(relative))
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, []byte(body), 0o644); err != nil {
		t.Fatal(err)
	}
}

func mustReadProgrammaticFile(t *testing.T, root, relative string) []byte {
	t.Helper()
	contents, err := os.ReadFile(filepath.Join(root, filepath.FromSlash(relative)))
	if err != nil {
		t.Fatal(err)
	}
	return contents
}
