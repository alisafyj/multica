package projectdesignsystem

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestAuditV2RejectsScriptsNetworkFormsAndUnsafeCSS(t *testing.T) {
	tests := []struct {
		name   string
		html   string
		tokens string
		code   string
	}{
		{name: "script", html: `<main data-design-node-id="orders" data-design-node-kind="block" data-design-node-label="Orders" style="color:var(--color-action)">Orders<script>alert(1)</script></main>`, code: "html_forbidden_element"},
		{name: "event", html: `<button data-design-node-id="save" data-design-node-kind="component" data-design-node-label="Save" style="color:var(--color-action)" onclick="save()">Save</button>`, code: "html_event_handler"},
		{name: "form", html: `<form><button data-design-node-id="save" data-design-node-kind="component" data-design-node-label="Save" style="color:var(--color-action)">Save</button></form>`, code: "html_forbidden_element"},
		{name: "iframe", html: `<main data-design-node-id="orders" data-design-node-kind="block" data-design-node-label="Orders" style="color:var(--color-action)">Orders<iframe src="../assets/crm-mark.svg"></iframe></main>`, code: "html_forbidden_element"},
		{name: "javascript url", html: `<a data-design-node-id="orders" data-design-node-kind="component" data-design-node-label="Orders" style="color:var(--color-action)" href="javascript:alert(1)">Orders</a>`, code: "html_url_unsafe"},
		{name: "https url", html: `<main data-design-node-id="orders" data-design-node-kind="block" data-design-node-label="Orders" style="color:var(--color-action)"><img src="https://example.com/logo.png" alt="Orders"></main>`, code: "html_url_unsafe"},
		{name: "protocol relative url", html: `<main data-design-node-id="orders" data-design-node-kind="block" data-design-node-label="Orders" style="color:var(--color-action)"><img src="//example.com/logo.png" alt="Orders"></main>`, code: "html_url_unsafe"},
		{name: "css import", tokens: `@import url("../assets/theme.css"); :root { --color-action: #1677ff; }`, code: "tokens_css_import_forbidden"},
		{name: "css https url", tokens: `:root { --color-action: #1677ff; --image-logo: url("https://example.com/logo.png"); }`, code: "tokens_css_url_unsafe"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			root := copyV2Fixture(t)
			if tt.html != "" {
				if err := os.WriteFile(filepath.Join(root, "ui-kit", "index.html"), []byte(tt.html), 0o644); err != nil {
					t.Fatal(err)
				}
			}
			if tt.tokens != "" {
				if err := os.WriteFile(filepath.Join(root, "tokens.css"), []byte(tt.tokens), 0o644); err != nil {
					t.Fatal(err)
				}
			}
			collected, err := CollectV2Directory(root, validV2Binding())
			assertV2DiagnosticCode(t, collected.Audit, err, tt.code)
		})
	}
}

func TestAuditV2RequiresVisibleTokenBackedPreview(t *testing.T) {
	tests := []struct {
		name string
		html string
	}{
		{
			name: "hidden content",
			html: `<main hidden data-design-node-id="orders" data-design-node-kind="block" data-design-node-label="Orders" style="color:var(--color-action)">Orders</main>`,
		},
		{
			name: "no token reference",
			html: `<main data-design-node-id="orders" data-design-node-kind="block" data-design-node-label="Orders" style="color:#1677ff">Orders</main>`,
		},
		{
			name: "unknown token reference",
			html: `<main data-design-node-id="orders" data-design-node-kind="block" data-design-node-label="Orders" style="color:var(--color-missing)">Orders</main>`,
		},
		{
			name: "duplicate locator",
			html: `<main data-design-node-id="orders" data-design-node-kind="block" data-design-node-label="Orders" style="color:var(--color-action)">Orders</main><aside data-design-node-id="orders" data-design-node-kind="block" data-design-node-label="Filters">Filters</aside>`,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			root := copyV2Fixture(t)
			if err := os.WriteFile(filepath.Join(root, "ui-kit", "index.html"), []byte(tt.html), 0o644); err != nil {
				t.Fatal(err)
			}
			if _, err := CollectV2Directory(root, validV2Binding()); err == nil {
				t.Fatal("CollectV2Directory() accepted a non-visible or non-token-backed Preview")
			}
		})
	}

	collected := collectValidV2(t, validV2Binding())
	if !collected.Audit.Passed || collected.Audit.SchemaVersion != AuditSchemaV1 {
		t.Fatalf("valid fixture audit = %#v", collected.Audit)
	}
	if collected.Audit.ContentDigest != collected.Manifest.ContentDigest {
		t.Fatalf("audit digest = %q, manifest digest = %q", collected.Audit.ContentDigest, collected.Manifest.ContentDigest)
	}
}

func TestAuditV2ValidatesSourceIndexWithoutKeywordTaxonomy(t *testing.T) {
	root := copyV2Fixture(t)
	source := SourceIndex{
		SchemaVersion:       SourceIndexSchemaV1,
		InputSnapshotSHA256: validV2Binding().InputSnapshotSHA256,
		Evidence: []SourceEvidence{
			{
				ID:         "fact-dense-layout",
				Kind:       "unfamiliar_repository_observation",
				Summary:    "The existing workspace favors dense scanning and repeated operations.",
				References: []string{"apps/crm/orders/page.tsx", "snapshot-item-42", "https://docs.example.com/design/reference"},
			},
		},
		Conflicts: []SourceConflict{{
			ID:         "conflict-brand-blue",
			Summary:    "The brief and current interface use different action colors.",
			References: []string{"snapshot-brand-1", "packages/ui/theme.css"},
		}},
		Fallbacks: []SourceFallback{{
			ID:      "fallback-type-scale",
			Summary: "Use the product default type scale because no brand font was supplied.",
		}},
	}
	writeV2SourceIndex(t, root, source)
	if _, err := CollectV2Directory(root, validV2Binding()); err != nil {
		t.Fatalf("CollectV2Directory() imposed a source keyword taxonomy: %v", err)
	}

	tests := []struct {
		name   string
		mutate func(*SourceIndex)
	}{
		{name: "schema", mutate: func(index *SourceIndex) { index.SchemaVersion = "other" }},
		{name: "snapshot", mutate: func(index *SourceIndex) { index.InputSnapshotSHA256 = "sha256:" + strings.Repeat("b", 64) }},
		{name: "absolute path", mutate: func(index *SourceIndex) { index.Evidence[0].References = []string{"/Users/me/source.ts"} }},
		{name: "credential url", mutate: func(index *SourceIndex) {
			index.Evidence[0].References = []string{"https://user:secret@example.com/source"}
		}},
		{name: "credential query", mutate: func(index *SourceIndex) {
			index.Evidence[0].References = []string{"https://example.com/source?token=secret"}
		}},
		{name: "duplicate id", mutate: func(index *SourceIndex) {
			index.Fallbacks = append(index.Fallbacks, SourceFallback{ID: index.Evidence[0].ID, Summary: "Duplicate"})
		}},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			candidateRoot := copyV2Fixture(t)
			candidate := source
			candidate.Evidence = append([]SourceEvidence(nil), source.Evidence...)
			candidate.Conflicts = append([]SourceConflict(nil), source.Conflicts...)
			candidate.Fallbacks = append([]SourceFallback(nil), source.Fallbacks...)
			tt.mutate(&candidate)
			writeV2SourceIndex(t, candidateRoot, candidate)
			if _, err := CollectV2Directory(candidateRoot, validV2Binding()); err == nil {
				t.Fatal("CollectV2Directory() accepted invalid source index")
			}
		})
	}

	t.Run("unknown JSON field", func(t *testing.T) {
		candidateRoot := copyV2Fixture(t)
		raw, err := json.Marshal(source)
		if err != nil {
			t.Fatal(err)
		}
		var object map[string]any
		if err := json.Unmarshal(raw, &object); err != nil {
			t.Fatal(err)
		}
		object["component_taxonomy"] = []string{"table", "card"}
		encoded, _ := json.Marshal(object)
		if err := os.WriteFile(filepath.Join(candidateRoot, "source", "index.json"), encoded, 0o644); err != nil {
			t.Fatal(err)
		}
		if _, err := CollectV2Directory(candidateRoot, validV2Binding()); err == nil {
			t.Fatal("CollectV2Directory() accepted undeclared source index fields")
		}
	})
}

func TestAuditV2RejectsHomeRelativeSourceReference(t *testing.T) {
	root := copyV2Fixture(t)
	writeV2SourceIndex(t, root, SourceIndex{
		SchemaVersion:       SourceIndexSchemaV1,
		InputSnapshotSHA256: validV2Binding().InputSnapshotSHA256,
		Evidence: []SourceEvidence{{
			ID:         "local-source",
			Kind:       "repository_fact",
			Summary:    "A local source path must not escape the repository reference boundary.",
			References: []string{"~/source.ts"},
		}},
		Conflicts: []SourceConflict{},
		Fallbacks: []SourceFallback{},
	})
	if _, err := CollectV2Directory(root, validV2Binding()); err == nil {
		t.Fatal("CollectV2Directory() accepted a home-relative source reference")
	}
}

func TestAuditV2RejectsEmbeddedSourceTextReference(t *testing.T) {
	root := copyV2Fixture(t)
	writeV2SourceIndex(t, root, SourceIndex{
		SchemaVersion:       SourceIndexSchemaV1,
		InputSnapshotSHA256: validV2Binding().InputSnapshotSHA256,
		Evidence: []SourceEvidence{{
			ID:         "embedded-source",
			Kind:       "repository_fact",
			Summary:    "Source references must identify evidence without embedding its contents.",
			References: []string{"src/app.ts\nconst apiKey = \"secret\""},
		}},
		Conflicts: []SourceConflict{},
		Fallbacks: []SourceFallback{},
	})
	if _, err := CollectV2Directory(root, validV2Binding()); err == nil {
		t.Fatal("CollectV2Directory() accepted embedded source text as a source reference")
	}
}

func TestAuditV2RejectsActiveSVGNetworkReferences(t *testing.T) {
	t.Run("inline SVG", func(t *testing.T) {
		root := copyV2Fixture(t)
		html := `<main data-design-node-id="orders" data-design-node-kind="block" data-design-node-label="Orders" style="color:var(--color-action)"><svg><image id="logo" href="#local"></image><set href="#logo" attributeName="href" to="https://example.com/logo.svg"></set></svg>Orders</main>`
		if err := os.WriteFile(filepath.Join(root, "ui-kit", "index.html"), []byte(html), 0o644); err != nil {
			t.Fatal(err)
		}
		collected, err := CollectV2Directory(root, validV2Binding())
		assertV2DiagnosticCode(t, collected.Audit, err, "html_forbidden_element")
	})

	t.Run("SVG asset", func(t *testing.T) {
		root := copyV2Fixture(t)
		svg := `<svg xmlns="http://www.w3.org/2000/svg"><image id="logo" href="#local"/><set href="#logo" attributeName="href" to="https://example.com/logo.svg"/></svg>`
		if err := os.WriteFile(filepath.Join(root, "assets", "crm-mark.svg"), []byte(svg), 0o644); err != nil {
			t.Fatal(err)
		}
		collected, err := CollectV2Directory(root, validV2Binding())
		assertV2DiagnosticCode(t, collected.Audit, err, "svg_unsafe")
	})
}
