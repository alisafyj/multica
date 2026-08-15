package daemon

import (
	"encoding/json"
	"net/url"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/multica-ai/multica/server/internal/projectdesignsystem"
)

// The V2 chain has two sides that were specified independently: the prompt
// tells the agent which files to write, and CollectV2Directory +
// auditV2Package decide which files the platform accepts. Nothing used to
// cross that boundary — the prompt tests asserted prompt text, the audit
// tests used a hand-written fixture — so the two drifted apart and every
// native generate/adjust/regenerate task failed at the collector with
// `archive_path_undeclared` before the audit ever ran.
//
// These tests close the gap: they build a package from what the prompt
// actually says and push it through the real collector and audit.

// promptContractPackage writes the file set the prompt declares as required
// into envRoot's output directory. inputSnapshotSHA must match the binding
// collectV2ForTest uses, mirroring the agent copying the digest out of
// context/task.json.
func promptContractPackage(t *testing.T, envRoot, inputSnapshotSHA string) string {
	t.Helper()
	outputDir := filepath.Join(envRoot, "output", "project-design-system")
	for _, dir := range []string{"source", "ui-kit"} {
		if err := os.MkdirAll(filepath.Join(outputDir, dir), 0o755); err != nil {
			t.Fatalf("mkdir %s: %v", dir, err)
		}
	}

	write := func(relative, body string) {
		t.Helper()
		if err := os.WriteFile(filepath.Join(outputDir, filepath.FromSlash(relative)), []byte(body), 0o644); err != nil {
			t.Fatalf("write %s: %v", relative, err)
		}
	}

	write("DESIGN.md", "# Acme Design System\n\n## Identity\n\nAcme runs a CRM for clinics.\n\n## Principles\n\nDense data first, calm chrome.\n")
	write("tokens.css", ":root {\n  --color-action: #1677ff;\n  --color-surface: #ffffff;\n  --space-control: 12px;\n}\n")

	sourceIndex := map[string]any{
		"schema_version":        projectdesignsystem.SourceIndexSchemaV1,
		"input_snapshot_sha256": inputSnapshotSHA,
		"evidence": []map[string]any{{
			"id":         "crm-orders-page",
			"kind":       "repository_fact",
			"summary":    "The order workspace pairs a dense table with compact repeated actions.",
			"references": []string{"apps/crm/orders/page.tsx"},
		}},
		"conflicts": []map[string]any{},
		"fallbacks": []map[string]any{{
			"id":      "no-brand-type",
			"summary": "No brand typeface was supplied; falling back to a system stack.",
		}},
	}
	sourceJSON, err := json.Marshal(sourceIndex)
	if err != nil {
		t.Fatalf("marshal source index: %v", err)
	}
	write("source/index.json", string(sourceJSON))

	write("ui-kit/index.html", `<!doctype html>
<html lang="en">
<head>
  <meta charset="utf-8">
  <title>Acme UI Kit</title>
  <style>
    body { margin: 0; background: var(--color-surface); }
    .primary { background: var(--color-action); padding: var(--space-control); }
  </style>
</head>
<body>
  <main data-design-node-id="orders-workspace" data-design-node-kind="block" data-design-node-label="Orders workspace">
    <button class="primary" type="button" data-design-node-id="create-order" data-design-node-kind="component" data-design-node-label="Create order">Create order</button>
  </main>
</body>
</html>
`)
	return outputDir
}

// TestProjectDesignSystemPromptContractPassesRealAudit is the regression that
// would have caught the components.html drift: a package built from the
// prompt's own required file list must survive the real collector and audit.
func TestProjectDesignSystemPromptContractPassesRealAudit(t *testing.T) {
	envRoot := t.TempDir()
	promptContractPackage(t, envRoot, "sha256:"+strings.Repeat("a", 64))

	collected, err := collectV2ForTest(t, envRoot)
	if err != nil {
		t.Fatalf("collector rejected the package the prompt asks for: %v", err)
	}
	if !collected.Audit.Passed {
		t.Fatalf("audit rejected the package the prompt asks for: %+v", collected.Audit.Diagnostics)
	}
	if len(collected.Manifest.PreviewTargets) == 0 {
		t.Fatal("prompt contract produced no preview target")
	}
}

// TestProjectDesignSystemPromptNamesOnlyAcceptedPaths pushes every concrete
// file path the prompt mentions through the collector. A path the prompt
// names but the contract rejects fails here rather than in production.
func TestProjectDesignSystemPromptNamesOnlyAcceptedPaths(t *testing.T) {
	prompt := BuildPrompt(projectDesignSystemPromptTask(t, "generate"), "opencode")

	// Concrete optional paths the prompt offers beyond the required set.
	// `<name>` / `<file>` placeholders are instantiated with real names.
	optional := map[string]string{
		"USAGE.md":                 "# Usage\n\n## Applying tokens\n\nImport tokens.css first.\n",
		"design-tokens.json":       `{"color":{"action":"#1677ff"}}`,
		"components.manifest.json": `{"components":[{"id":"create-order"}]}`,
		"preview/dashboard.html": `<!doctype html><html><head><style>.c{color:var(--color-action)}</style></head>` +
			`<body><section class="c" data-design-node-id="dash" data-design-node-kind="block" data-design-node-label="Dashboard">Dashboard</section></body></html>`,
		"assets/mark.svg": `<svg xmlns="http://www.w3.org/2000/svg" viewBox="0 0 8 8"><rect width="8" height="8" fill="#1677ff"/></svg>`,
	}

	for relative, body := range optional {
		t.Run(relative, func(t *testing.T) {
			// The prompt must actually offer this path, otherwise the test
			// is asserting something the agent was never told about.
			stem := strings.SplitN(relative, "/", 2)[0]
			if !strings.Contains(prompt, relative) && !strings.Contains(prompt, stem+"/") {
				t.Fatalf("prompt does not mention %q", relative)
			}

			envRoot := t.TempDir()
			outputDir := promptContractPackage(t, envRoot, "sha256:"+strings.Repeat("a", 64))
			full := filepath.Join(outputDir, filepath.FromSlash(relative))
			if err := os.MkdirAll(filepath.Dir(full), 0o755); err != nil {
				t.Fatalf("mkdir: %v", err)
			}
			if err := os.WriteFile(full, []byte(body), 0o644); err != nil {
				t.Fatalf("write %s: %v", relative, err)
			}

			collected, err := collectV2ForTest(t, envRoot)
			if err != nil {
				t.Fatalf("collector rejected optional path %q offered by the prompt: %v", relative, err)
			}
			if !collected.Audit.Passed {
				t.Fatalf("audit rejected optional path %q offered by the prompt: %+v", relative, collected.Audit.Diagnostics)
			}
		})
	}
}

// TestProjectDesignSystemPromptDropsRetiredThreeFileContract pins the specific
// drift that broke the chain, so a future edit cannot reintroduce the V1
// artifact under the V2 schema.
func TestProjectDesignSystemPromptDropsRetiredThreeFileContract(t *testing.T) {
	for _, operation := range []string{"generate", "adjust", "regenerate"} {
		prompt := BuildPrompt(projectDesignSystemPromptTask(t, operation), "opencode")
		if strings.Contains(prompt, "components.html") {
			t.Fatalf("%s prompt still asks for components.html, which classifyV2Artifact rejects", operation)
		}
		for _, required := range []string{"DESIGN.md", "tokens.css", "source/index.json", "ui-kit/index.html"} {
			if !strings.Contains(prompt, required) {
				t.Fatalf("%s prompt does not declare required artifact %q", operation, required)
			}
		}
	}
}

// TestPreviewTokensStylesheetResolvesFromEveryTarget covers the second half of
// the same defect class. classifyV2Artifact only admits preview targets one
// directory down (`ui-kit/index.html`, `preview/*.html`), so a relative
// `tokens.css` href resolves to `<dir>/tokens.css` and 404s — the verifier
// then screenshots an untokenized page while audit and preview both pass.
func TestPreviewTokensStylesheetResolvesFromEveryTarget(t *testing.T) {
	envRoot := t.TempDir()
	stageProjectDesignSystemV2Package(t, envRoot)

	collected, err := collectV2ForTest(t, envRoot)
	if err != nil {
		t.Fatalf("collect V2: %v", err)
	}
	baseURL, prefix, cleanup := startLoopbackPreviewServerForTest(t, collected)
	defer cleanup()

	if len(collected.Manifest.PreviewTargets) == 0 {
		t.Fatal("fixture produced no preview target")
	}
	for _, target := range collected.Manifest.PreviewTargets {
		t.Run(target.Path, func(t *testing.T) {
			pageURL := baseURL + "/" + prefix + "/" + target.Path
			body, _, _, status := fetchLoopbackURL(t, pageURL)
			if status != 200 {
				t.Fatalf("preview target status = %d, want 200", status)
			}

			href := stylesheetHref(t, body)
			base, parseErr := url.Parse(pageURL)
			if parseErr != nil {
				t.Fatalf("parse page URL: %v", parseErr)
			}
			reference, parseErr := url.Parse(href)
			if parseErr != nil {
				t.Fatalf("parse stylesheet href %q: %v", href, parseErr)
			}
			resolved := base.ResolveReference(reference)

			_, _, contentType, tokensStatus := fetchLoopbackURL(t, resolved.String())
			if tokensStatus != 200 {
				t.Fatalf("tokens stylesheet %q resolved to %q and returned %d; the target renders without design tokens",
					href, resolved.Path, tokensStatus)
			}
			if !strings.HasPrefix(contentType, "text/css") {
				t.Fatalf("tokens stylesheet content type = %q, want text/css", contentType)
			}
		})
	}
}

func stylesheetHref(t *testing.T, body string) string {
	t.Helper()
	const marker = `<link rel="stylesheet" href="`
	start := strings.Index(body, marker)
	if start < 0 {
		t.Fatalf("preview target carries no injected stylesheet link: %q", body)
	}
	rest := body[start+len(marker):]
	end := strings.IndexByte(rest, '"')
	if end < 0 {
		t.Fatalf("malformed stylesheet link in %q", body)
	}
	return rest[:end]
}
