package projectdesignsystem

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestValidateAcceptsCoherentPackage(t *testing.T) {
	input := validArtifactInput(t)

	pkg, err := Validate(input, []string{"static.soyoung.com"})
	if err != nil {
		t.Fatalf("Validate() error = %v", err)
	}
	if !pkg.Validation.Passed {
		t.Fatalf("Validation.Passed = false, diagnostics = %#v", pkg.Validation.Diagnostics)
	}
	if pkg.Manifest.SchemaVersion != SchemaVersion {
		t.Fatalf("SchemaVersion = %q, want %q", pkg.Manifest.SchemaVersion, SchemaVersion)
	}
	if len(pkg.Manifest.Digest) != 64 {
		t.Fatalf("Digest length = %d, want 64", len(pkg.Manifest.Digest))
	}
	if len(pkg.Manifest.Sections) != 3 {
		t.Fatalf("Sections = %#v, want 3 dynamic sections", pkg.Manifest.Sections)
	}
	if len(pkg.Manifest.TokenGroups) < 2 {
		t.Fatalf("TokenGroups = %#v, want at least color and radius", pkg.Manifest.TokenGroups)
	}
	if len(pkg.Manifest.Locators) != 2 {
		t.Fatalf("Locators = %#v, want 2", pkg.Manifest.Locators)
	}
	for _, name := range []string{"DESIGN.md", "tokens.css", "components.html"} {
		if pkg.Manifest.Files[name].SHA256 == "" {
			t.Fatalf("manifest for %s has no digest", name)
		}
	}
}

func TestValidateRejectsMissingAndOversizedFiles(t *testing.T) {
	tests := []struct {
		name   string
		mutate func(*ArtifactInput)
		code   string
	}{
		{
			name: "missing design markdown",
			mutate: func(input *ArtifactInput) {
				input.DesignMD = ""
			},
			code: "artifact_missing",
		},
		{
			name: "oversized token stylesheet",
			mutate: func(input *ArtifactInput) {
				input.TokensCSS = strings.Repeat("x", MaxTokensCSSBytes+1)
			},
			code: "artifact_too_large",
		},
		{
			name: "oversized aggregate",
			mutate: func(input *ArtifactInput) {
				input.DesignMD = strings.Repeat("#", MaxDesignMDBytes)
				input.TokensCSS = strings.Repeat("x", MaxTokensCSSBytes)
				input.ComponentsHTML = strings.Repeat("y", MaxAggregateBytes-MaxDesignMDBytes-MaxTokensCSSBytes+1)
			},
			code: "package_too_large",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			input := validArtifactInput(t)
			tt.mutate(&input)

			pkg, err := Validate(input, nil)
			if err == nil {
				t.Fatal("Validate() error = nil, want rejection")
			}
			assertDiagnosticCode(t, pkg.Validation, tt.code)
		})
	}
}

func TestValidateExtractsDynamicMarkdownSections(t *testing.T) {
	input := validArtifactInput(t)
	input.DesignMD = "# Atlas\n\nIntro.\n\n## Editorial rhythm\n\nQuiet density.\n\n## Empty states\n\nBe direct.\n"

	pkg, err := Validate(input, nil)
	if err != nil {
		t.Fatalf("Validate() error = %v", err)
	}
	if got := sectionTitles(pkg.Manifest.Sections); strings.Join(got, "|") != "Atlas|Editorial rhythm|Empty states" {
		t.Fatalf("section titles = %#v", got)
	}
	if !strings.Contains(pkg.Manifest.Sections[1].Markdown, "Quiet density") {
		t.Fatalf("section markdown = %q, want source content", pkg.Manifest.Sections[1].Markdown)
	}
}

func TestValidateRejectsMalformedCSSAndUnknownVariables(t *testing.T) {
	tests := []struct {
		name string
		css  string
		code string
	}{
		{
			name: "malformed",
			css:  ":root { --color-action: #2463eb;",
			code: "tokens_css_invalid",
		},
		{
			name: "unknown variable",
			css:  ":root { --color-action: #2463eb; } .primary { color: var(--color-missing); }",
			code: "token_reference_unknown",
		},
		{
			name: "unknown variable in custom property",
			css:  ":root { --color-action: #2463eb; --color-hover: var(--color-missing); } .primary { color: var(--color-action); }",
			code: "token_reference_unknown",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			input := validArtifactInput(t)
			input.TokensCSS = tt.css

			pkg, err := Validate(input, nil)
			if err == nil {
				t.Fatal("Validate() error = nil, want rejection")
			}
			assertDiagnosticCode(t, pkg.Validation, tt.code)
		})
	}
}

func TestValidateGroupsOnlyDeclaredCustomProperties(t *testing.T) {
	input := validArtifactInput(t)
	input.TokensCSS = `:root {
  --color-action: #2463eb;
  --space-control: 0.75rem;
}
.primary { color: var(--color-action); padding: var(--space-control); }
`

	pkg, err := Validate(input, nil)
	if err != nil {
		t.Fatalf("Validate() error = %v", err)
	}
	var names []string
	for _, group := range pkg.Manifest.TokenGroups {
		for _, token := range group.Tokens {
			names = append(names, token.Name)
		}
	}
	if got := strings.Join(names, "|"); got != "--color-action|--space-control" {
		t.Fatalf("token names = %q, want declared properties only", got)
	}
}

func TestValidateRejectsScriptEventsImportsAndUnsafeURLs(t *testing.T) {
	tests := []struct {
		name string
		html string
		css  string
		code string
	}{
		{name: "script", html: `<script>alert(1)</script>`, code: "html_forbidden_element"},
		{name: "event handler", html: `<button data-design-node-id="button" data-design-node-kind="component" data-design-node-label="Button" onclick="alert(1)">Create</button>`, code: "html_event_handler"},
		{name: "iframe", html: `<iframe src="https://static.soyoung.com/example"></iframe>`, code: "html_forbidden_element"},
		{name: "form", html: `<form><button>Submit</button></form>`, code: "html_forbidden_element"},
		{name: "srcdoc", html: `<div data-design-node-id="block" data-design-node-kind="block" data-design-node-label="Block" srcdoc="<p>unsafe</p>">Visible</div>`, code: "html_forbidden_attribute"},
		{name: "javascript url", html: `<a data-design-node-id="link" data-design-node-kind="component" data-design-node-label="Link" href="javascript:alert(1)">Open</a>`, code: "html_url_unsafe"},
		{name: "non fragment anchor", html: `<a data-design-node-id="link" data-design-node-kind="component" data-design-node-label="Link" href="/elsewhere">Open</a>`, code: "html_url_unsafe"},
		{name: "unapproved image host", html: `<main data-design-node-id="block" data-design-node-kind="block" data-design-node-label="Block"><img src="https://example.com/a.png" alt="Example">Visible</main>`, code: "html_url_unsafe"},
		{name: "css import", css: `@import url("https://static.soyoung.com/theme.css"); :root { --color-action: #2463eb; } .primary { color: var(--color-action); }`, code: "tokens_css_import_forbidden"},
		{name: "css unapproved host", css: `:root { --color-action: #2463eb; } .primary { color: var(--color-action); background: url("https://example.com/a.png"); }`, code: "tokens_css_url_unsafe"},
		{name: "custom property unapproved host", css: `:root { --color-action: #2463eb; --image-hero: url("https://example.com/a.png"); } .primary { color: var(--color-action); }`, code: "tokens_css_url_unsafe"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			input := validArtifactInput(t)
			if tt.html != "" {
				input.ComponentsHTML = tt.html
			}
			if tt.css != "" {
				input.TokensCSS = tt.css
			}

			pkg, err := Validate(input, []string{"static.soyoung.com"})
			if err == nil {
				t.Fatal("Validate() error = nil, want rejection")
			}
			assertDiagnosticCode(t, pkg.Validation, tt.code)
		})
	}
}

func TestValidateRequiresUniqueStableLocators(t *testing.T) {
	tests := []struct {
		name string
		html string
		code string
	}{
		{
			name: "duplicate",
			html: `<main data-design-node-id="same" data-design-node-kind="block" data-design-node-label="One">One</main><button data-design-node-id="same" data-design-node-kind="component" data-design-node-label="Two">Two</button>`,
			code: "locator_id_duplicate",
		},
		{
			name: "unstable syntax",
			html: `<main data-design-node-id="Has Spaces" data-design-node-kind="block" data-design-node-label="One">One</main>`,
			code: "locator_id_invalid",
		},
		{
			name: "incomplete",
			html: `<main data-design-node-id="overview" data-design-node-kind="block">One</main>`,
			code: "locator_incomplete",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			input := validArtifactInput(t)
			input.ComponentsHTML = tt.html

			pkg, err := Validate(input, nil)
			if err == nil {
				t.Fatal("Validate() error = nil, want rejection")
			}
			assertDiagnosticCode(t, pkg.Validation, tt.code)
		})
	}
}

func TestValidateRequiresVisibleUIKitContentAndTokenUsage(t *testing.T) {
	tests := []struct {
		name string
		html string
		css  string
		code string
	}{
		{
			name: "blank locator",
			html: `<main data-design-node-id="overview" data-design-node-kind="block" data-design-node-label="Overview"></main>`,
			code: "ui_kit_not_visible",
		},
		{
			name: "no locator",
			html: `<main>Visible but not selectable</main>`,
			code: "locator_missing",
		},
		{
			name: "no token reference",
			css:  `:root { --color-action: #2463eb; } .primary { color: #2463eb; }`,
			code: "token_usage_missing",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			input := validArtifactInput(t)
			if tt.html != "" {
				input.ComponentsHTML = tt.html
			}
			if tt.css != "" {
				input.TokensCSS = tt.css
			}

			pkg, err := Validate(input, nil)
			if err == nil {
				t.Fatal("Validate() error = nil, want rejection")
			}
			assertDiagnosticCode(t, pkg.Validation, tt.code)
		})
	}
}

func validArtifactInput(t *testing.T) ArtifactInput {
	t.Helper()
	read := func(name string) string {
		t.Helper()
		contents, err := os.ReadFile(filepath.Join("testdata", "valid", name))
		if err != nil {
			t.Fatalf("read %s: %v", name, err)
		}
		return string(contents)
	}
	return ArtifactInput{
		DesignMD:       read("DESIGN.md"),
		TokensCSS:      read("tokens.css"),
		ComponentsHTML: read("components.html"),
	}
}

func assertDiagnosticCode(t *testing.T, report ValidationReport, code string) {
	t.Helper()
	for _, diagnostic := range report.Diagnostics {
		if diagnostic.Code == code {
			return
		}
	}
	t.Fatalf("diagnostics = %#v, want code %q", report.Diagnostics, code)
}

func sectionTitles(sections []Section) []string {
	titles := make([]string, 0, len(sections))
	for _, section := range sections {
		titles = append(titles, section.Title)
	}
	return titles
}
