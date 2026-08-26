package designdocument

import (
	"errors"
	"fmt"
	"regexp"
	"sort"
	"strings"
)

// Applying a designer's direct edits to a prototype package, deterministically.
//
// Every other change to a design document goes through an agent. A manual edit
// does not: the designer already saw the result on the canvas, so re-deriving
// it through a model would be slower and could come back different. What runs
// instead is this — a pure transformation from (package, edit set) to a new
// package, with no judgement in it anywhere.
//
// Determinism is the whole contract. The same edits applied to the same
// package must produce byte-identical output, or the content digest the
// revision is keyed on would change for no reason. That is why the generated
// CSS is sorted and why nothing here consults a clock, a map order, or a
// random source.
//
// The edits land in their own stylesheet rather than being spliced into the
// agent's rules. Rewriting an existing rule in place means guessing which of
// several matching rules the designer meant; an override sheet loaded last
// says exactly what a person changed and leaves the design the agent wrote
// legible underneath it.

const (
	// ManualEditsDirectory holds one stylesheet per edited page. Scoping by
	// page keeps an edit to what the designer was looking at: the same
	// selector can match on another page, and silently restyling a page they
	// never opened is not what "I changed this button" means.
	ManualEditsDirectory = "prototype/manual-edits"

	maxManualEdits             = 200
	maxManualEditSelectorBytes = 512
	maxManualEditValueBytes    = 200
)

// ManualEdit is one element's overridden declarations.
type ManualEdit struct {
	// Package path of the page this edit was made on.
	Page string `json:"page"`
	// The selector the pick resolved to, in the page's own document.
	Selector string `json:"selector"`
	// Property -> value. An empty value removes the override.
	Declarations map[string]string `json:"declarations"`
}

// manualEditProperties is what the properties panel can set. A whitelist, not
// a filter: CSS that reaches a rendered package should only ever contain
// properties a control in the UI actually produced, so a value that arrives
// for anything else is a bug or an attack, never a feature.
var manualEditProperties = map[string]struct{}{
	// Typography
	"color": {}, "font-family": {}, "font-size": {}, "font-weight": {},
	"font-style": {}, "line-height": {}, "letter-spacing": {}, "text-align": {},
	"text-transform": {}, "text-decoration": {},
	// Fill and border
	"background-color": {}, "border-color": {}, "border-width": {},
	"border-style": {}, "border-radius": {}, "box-shadow": {}, "opacity": {},
	// Spacing
	"padding": {}, "padding-top": {}, "padding-right": {}, "padding-bottom": {}, "padding-left": {},
	"margin": {}, "margin-top": {}, "margin-right": {}, "margin-bottom": {}, "margin-left": {},
	"gap": {}, "row-gap": {}, "column-gap": {},
	// Layout
	"display": {}, "flex-direction": {}, "flex-wrap": {}, "justify-content": {},
	"align-items": {}, "align-self": {}, "width": {}, "height": {},
	"min-width": {}, "min-height": {}, "max-width": {}, "max-height": {},
}

// A value may contain letters, digits, spaces and the punctuation CSS lengths,
// colours, keywords and font stacks need. Everything that could end a
// declaration block, start an at-rule, open a comment or fetch a resource is
// absent by construction, so a value cannot escape the rule it sits in.
var manualEditValuePattern = regexp.MustCompile(`^[-#%.,()'"\sA-Za-z0-9/]+$`)

// A selector may contain the shapes elementSelector produces: ids, classes,
// attribute predicates, tag names, combinators and :nth-of-type. Braces,
// at-signs, comment markers and newlines are refused, so a selector cannot
// terminate its own rule and inject another.
var manualEditSelectorPattern = regexp.MustCompile(`^[-_#.\[\]="':()>+~\sA-Za-z0-9\x{4e00}-\x{9fff}]+$`)

// ValidateManualEdits refuses an edit set that could not be applied
// faithfully. It runs on the server, before a task is ever enqueued, so a
// malformed set fails where the user can see it rather than inside a run.
func ValidateManualEdits(edits []ManualEdit) error {
	if len(edits) == 0 {
		return errors.New("manual edit set is empty")
	}
	if len(edits) > maxManualEdits {
		return fmt.Errorf("manual edit set has %d entries, more than the %d allowed", len(edits), maxManualEdits)
	}
	for _, edit := range edits {
		if !isPrototypePage(edit.Page) {
			return fmt.Errorf("manual edit names %q, which is not a prototype page", edit.Page)
		}
		selector := strings.TrimSpace(edit.Selector)
		if selector == "" || len(selector) > maxManualEditSelectorBytes || !manualEditSelectorPattern.MatchString(selector) {
			return fmt.Errorf("manual edit selector %q is not usable", edit.Selector)
		}
		if len(edit.Declarations) == 0 {
			return fmt.Errorf("manual edit for %q changes nothing", selector)
		}
		for property, value := range edit.Declarations {
			property = strings.ToLower(strings.TrimSpace(property))
			if _, ok := manualEditProperties[property]; !ok {
				return fmt.Errorf("manual edit sets %q, which is not an editable property", property)
			}
			trimmed := strings.TrimSpace(value)
			if trimmed == "" {
				continue
			}
			if len(trimmed) > maxManualEditValueBytes || !manualEditValuePattern.MatchString(trimmed) {
				return fmt.Errorf("manual edit value for %q is not usable", property)
			}
		}
	}
	return nil
}

func isPrototypePage(path string) bool {
	if path == "" || strings.Contains(path, "..") || strings.HasPrefix(path, "/") {
		return false
	}
	if !strings.HasPrefix(path, "prototype/") {
		return false
	}
	lower := strings.ToLower(path)
	return strings.HasSuffix(lower, ".html") || strings.HasSuffix(lower, ".htm")
}

// ManualEditsStylesheetPath is where a page's overrides live.
func ManualEditsStylesheetPath(page string) string {
	name := strings.TrimPrefix(page, "prototype/")
	name = strings.ReplaceAll(name, "/", "__")
	return ManualEditsDirectory + "/" + name + ".css"
}

// ApplyManualEdits returns a new package with the edits applied: one
// stylesheet per edited page, and a link to it in that page's markup.
//
// The input map is not modified. A page named by an edit that the package does
// not contain is an error rather than a silent skip — it means the client and
// the package disagree about what was being edited, and applying the rest
// would produce a design the user did not ask for.
func ApplyManualEdits(files map[string][]byte, edits []ManualEdit) (map[string][]byte, error) {
	if err := ValidateManualEdits(edits); err != nil {
		return nil, err
	}
	byPage := map[string][]ManualEdit{}
	pages := make([]string, 0, len(edits))
	for _, edit := range edits {
		if _, ok := files[edit.Page]; !ok {
			return nil, fmt.Errorf("manual edit names page %q, which this package does not contain", edit.Page)
		}
		if _, seen := byPage[edit.Page]; !seen {
			pages = append(pages, edit.Page)
		}
		byPage[edit.Page] = append(byPage[edit.Page], edit)
	}
	sort.Strings(pages)

	result := make(map[string][]byte, len(files)+len(pages))
	for name, content := range files {
		result[name] = content
	}
	for _, page := range pages {
		stylesheet := ManualEditsStylesheetPath(page)
		result[stylesheet] = []byte(renderManualEditStylesheet(byPage[page]))
		linked, err := linkStylesheet(string(files[page]), relativeStylesheetHref(page, stylesheet))
		if err != nil {
			return nil, fmt.Errorf("link manual edits into %q: %w", page, err)
		}
		result[page] = []byte(linked)
	}
	return result, nil
}

// renderManualEditStylesheet writes the override rules in a stable order.
// Later edits to the same selector win, which is what a designer who changed
// the same element twice expects.
func renderManualEditStylesheet(edits []ManualEdit) string {
	merged := map[string]map[string]string{}
	order := []string{}
	for _, edit := range edits {
		selector := strings.TrimSpace(edit.Selector)
		if _, seen := merged[selector]; !seen {
			merged[selector] = map[string]string{}
			order = append(order, selector)
		}
		for property, value := range edit.Declarations {
			property = strings.ToLower(strings.TrimSpace(property))
			trimmed := strings.TrimSpace(value)
			if trimmed == "" {
				delete(merged[selector], property)
				continue
			}
			merged[selector][property] = trimmed
		}
	}
	sort.Strings(order)

	var b strings.Builder
	b.WriteString("/* Manual edits made in the Multica design workbench.\n")
	b.WriteString("   Generated deterministically from the designer's own changes; the\n")
	b.WriteString("   design underneath is the agent's and is left untouched. */\n")
	for _, selector := range order {
		declarations := merged[selector]
		if len(declarations) == 0 {
			continue
		}
		properties := make([]string, 0, len(declarations))
		for property := range declarations {
			properties = append(properties, property)
		}
		sort.Strings(properties)
		b.WriteString("\n" + selector + " {\n")
		for _, property := range properties {
			// !important: these rules are the designer's explicit override and
			// must win over a more specific rule the agent happened to write.
			fmt.Fprintf(&b, "  %s: %s !important;\n", property, declarations[property])
		}
		b.WriteString("}\n")
	}
	return b.String()
}

// relativeStylesheetHref addresses the stylesheet from the page that links it.
// Both live under prototype/, so the href is relative to that page's directory.
func relativeStylesheetHref(page, stylesheet string) string {
	pageDepth := strings.Count(strings.TrimPrefix(page, "prototype/"), "/")
	return strings.Repeat("../", pageDepth) + strings.TrimPrefix(stylesheet, "prototype/")
}

var headClosePattern = regexp.MustCompile(`(?i)</head\s*>`)
var bodyClosePattern = regexp.MustCompile(`(?i)</body\s*>`)

// linkStylesheet inserts the override stylesheet last in the page, so it wins
// over everything the page already loads.
//
// A page that was already linked keeps exactly one link: applying edits twice
// (a second manual edit on top of the first) must not accumulate duplicates.
func linkStylesheet(html, href string) (string, error) {
	link := fmt.Sprintf(`<link rel="stylesheet" href="%s" data-multica-manual-edits="true">`, href)
	if strings.Contains(html, `data-multica-manual-edits="true"`) {
		// Replace the existing link rather than adding a second one.
		existing := regexp.MustCompile(`(?i)<link[^>]*data-multica-manual-edits="true"[^>]*>`)
		return existing.ReplaceAllLiteralString(html, link), nil
	}
	if location := headClosePattern.FindStringIndex(html); location != nil {
		return html[:location[0]] + "  " + link + "\n" + html[location[0]:], nil
	}
	// No head to close: a stylesheet at the end of the body still applies, and
	// refusing a page over its markup style would fail an edit the designer
	// already watched work on the canvas.
	if location := bodyClosePattern.FindStringIndex(html); location != nil {
		return html[:location[0]] + "  " + link + "\n" + html[location[0]:], nil
	}
	if strings.TrimSpace(html) == "" {
		return "", errors.New("page is empty")
	}
	return html + "\n" + link + "\n", nil
}
