package designdocument

import (
	"strings"
	"testing"
)

func edit(overrides func(*ManualEdit)) ManualEdit {
	value := ManualEdit{
		Page:         "prototype/index.html",
		Selector:     "#open-filters",
		Declarations: map[string]string{"color": "#ffffff"},
	}
	if overrides != nil {
		overrides(&value)
	}
	return value
}

func TestValidateManualEditsAcceptsWhatThePanelProduces(t *testing.T) {
	err := ValidateManualEdits([]ManualEdit{
		edit(nil),
		edit(func(e *ManualEdit) {
			e.Selector = `[data-block="block.orders.toolbar"] > button:nth-of-type(2)`
			e.Declarations = map[string]string{
				"font-family":      `"Noto Sans", system-ui, sans-serif`,
				"font-size":        "14px",
				"padding":          "8px 12px",
				"background-color": "rgba(0, 0, 0, 0.04)",
				"display":          "flex",
				"gap":              "0.5rem",
			}
		}),
	})
	if err != nil {
		t.Fatalf("a normal edit set was refused: %v", err)
	}
}

// The whitelist is the security boundary: this CSS is written into a package
// that Chrome then renders in the preview gate.
func TestValidateManualEditsRefusesAnythingThatCouldEscapeItsRule(t *testing.T) {
	cases := []struct {
		name string
		edit ManualEdit
	}{
		{"property not on the panel", edit(func(e *ManualEdit) { e.Declarations = map[string]string{"behavior": "url(x.htc)"} })},
		{"value fetches a resource", edit(func(e *ManualEdit) { e.Declarations = map[string]string{"background-color": "url(https://evil/x.png)"} })},
		{"value closes the rule", edit(func(e *ManualEdit) { e.Declarations = map[string]string{"color": "red} body {display:none"} })},
		{"value opens an at-rule", edit(func(e *ManualEdit) { e.Declarations = map[string]string{"color": "red; @import 'x'"} })},
		{"selector closes the rule", edit(func(e *ManualEdit) { e.Selector = "a { } body" })},
		{"selector opens a comment", edit(func(e *ManualEdit) { e.Selector = "a /* x" })},
		{"page outside the prototype", edit(func(e *ManualEdit) { e.Page = "assets/logo.png" })},
		{"page escapes the package", edit(func(e *ManualEdit) { e.Page = "prototype/../../etc/passwd.html" })},
		{"nothing changed", edit(func(e *ManualEdit) { e.Declarations = map[string]string{} })},
	}
	for _, testCase := range cases {
		t.Run(testCase.name, func(t *testing.T) {
			if err := ValidateManualEdits([]ManualEdit{testCase.edit}); err == nil {
				t.Fatalf("%+v was accepted", testCase.edit)
			}
		})
	}
	if err := ValidateManualEdits(nil); err == nil {
		t.Fatal("an empty edit set was accepted")
	}
}

func fixturePackage() map[string][]byte {
	return map[string][]byte{
		"prototype/index.html":  []byte("<!doctype html><html><head><title>a</title></head><body><button id=\"open-filters\">x</button></body></html>"),
		"prototype/orders.html": []byte("<!doctype html><html><head></head><body></body></html>"),
		"prototype/styles.css":  []byte("body{margin:0}"),
	}
}

func TestApplyManualEditsWritesOneStylesheetPerEditedPage(t *testing.T) {
	applied, err := ApplyManualEdits(fixturePackage(), []ManualEdit{
		edit(nil),
		edit(func(e *ManualEdit) { e.Page = "prototype/orders.html"; e.Selector = "body"; e.Declarations = map[string]string{"gap": "8px"} }),
	})
	if err != nil {
		t.Fatal(err)
	}

	indexCSS, ok := applied["prototype/manual-edits/index.html.css"]
	if !ok {
		t.Fatalf("no stylesheet for the index page: %v", keysOf(applied))
	}
	if !strings.Contains(string(indexCSS), "#open-filters {") || !strings.Contains(string(indexCSS), "color: #ffffff !important;") {
		t.Fatalf("index stylesheet = %s", indexCSS)
	}
	// Scoped per page: the orders page must not carry the index's overrides.
	ordersCSS := string(applied["prototype/manual-edits/orders.html.css"])
	if strings.Contains(ordersCSS, "#open-filters") || !strings.Contains(ordersCSS, "gap: 8px !important;") {
		t.Fatalf("orders stylesheet = %s", ordersCSS)
	}
	// Each page links only its own.
	if !strings.Contains(string(applied["prototype/index.html"]), `href="manual-edits/index.html.css"`) {
		t.Fatalf("index page = %s", applied["prototype/index.html"])
	}
	if !strings.Contains(string(applied["prototype/index.html"]), "</head>") {
		t.Fatal("the link was not placed inside the head")
	}
	// The rest of the package is untouched.
	if string(applied["prototype/styles.css"]) != "body{margin:0}" {
		t.Fatal("an unedited file changed")
	}
}

// The digest a revision is keyed on must not move because a map iterated in a
// different order.
func TestApplyManualEditsIsDeterministic(t *testing.T) {
	edits := []ManualEdit{
		edit(func(e *ManualEdit) {
			e.Declarations = map[string]string{"color": "#111", "background-color": "#eee", "padding": "4px", "gap": "2px", "width": "50%"}
		}),
		edit(func(e *ManualEdit) { e.Selector = "body"; e.Declarations = map[string]string{"margin": "0", "display": "flex"} }),
	}
	first, err := ApplyManualEdits(fixturePackage(), edits)
	if err != nil {
		t.Fatal(err)
	}
	for attempt := 0; attempt < 20; attempt++ {
		next, err := ApplyManualEdits(fixturePackage(), edits)
		if err != nil {
			t.Fatal(err)
		}
		for name, content := range first {
			if string(next[name]) != string(content) {
				t.Fatalf("%s differed between runs:\n%s\n---\n%s", name, content, next[name])
			}
		}
	}
}

// A second round of manual edits must replace the first link, not stack up.
func TestApplyManualEditsDoesNotAccumulateLinks(t *testing.T) {
	once, err := ApplyManualEdits(fixturePackage(), []ManualEdit{edit(nil)})
	if err != nil {
		t.Fatal(err)
	}
	twice, err := ApplyManualEdits(once, []ManualEdit{edit(func(e *ManualEdit) { e.Declarations = map[string]string{"color": "#000"} })})
	if err != nil {
		t.Fatal(err)
	}
	page := string(twice["prototype/index.html"])
	if strings.Count(page, `data-multica-manual-edits="true"`) != 1 {
		t.Fatalf("link count != 1:\n%s", page)
	}
	if !strings.Contains(string(twice["prototype/manual-edits/index.html.css"]), "color: #000 !important;") {
		t.Fatal("the second round did not win")
	}
}

// Client and package disagreeing about what is being edited must stop the run,
// not silently produce a design nobody asked for.
func TestApplyManualEditsRefusesAPageThePackageLacks(t *testing.T) {
	if _, err := ApplyManualEdits(fixturePackage(), []ManualEdit{
		edit(func(e *ManualEdit) { e.Page = "prototype/missing.html" }),
	}); err == nil {
		t.Fatal("an edit for a page the package lacks was applied")
	}
}

// An empty value clears an override rather than writing an empty declaration.
func TestApplyManualEditsClearsAnOverrideWithAnEmptyValue(t *testing.T) {
	applied, err := ApplyManualEdits(fixturePackage(), []ManualEdit{
		edit(func(e *ManualEdit) { e.Declarations = map[string]string{"color": "#fff", "gap": "4px"} }),
		edit(func(e *ManualEdit) { e.Declarations = map[string]string{"color": ""} }),
	})
	if err != nil {
		t.Fatal(err)
	}
	css := string(applied["prototype/manual-edits/index.html.css"])
	if strings.Contains(css, "color:") {
		t.Fatalf("the cleared override survived:\n%s", css)
	}
	if !strings.Contains(css, "gap: 4px !important;") {
		t.Fatalf("clearing one override dropped another:\n%s", css)
	}
}

// A page whose markup has no head still gets its overrides.
func TestApplyManualEditsFallsBackWhenThereIsNoHead(t *testing.T) {
	files := map[string][]byte{"prototype/index.html": []byte("<body><button id=\"open-filters\"></button></body>")}
	applied, err := ApplyManualEdits(files, []ManualEdit{edit(nil)})
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(applied["prototype/index.html"]), `data-multica-manual-edits="true"`) {
		t.Fatalf("page = %s", applied["prototype/index.html"])
	}
}

func keysOf(files map[string][]byte) []string {
	names := make([]string, 0, len(files))
	for name := range files {
		names = append(names, name)
	}
	return names
}
