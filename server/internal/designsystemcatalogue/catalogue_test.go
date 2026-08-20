// @vitest-environment n/a — Go test, no DB required: the catalogue is embedded.
package designsystemcatalogue

import (
	"strings"
	"testing"
)

func TestListReturnsEveryBundledPackage(t *testing.T) {
	entries, err := List()
	if err != nil {
		t.Fatalf("List: %v", err)
	}
	if len(entries) < 100 {
		t.Fatalf("catalogue has %d entries, expected the full bundled set", len(entries))
	}
	seen := make(map[string]bool, len(entries))
	for _, e := range entries {
		if e.Slug == "" || e.Name == "" {
			t.Fatalf("entry with empty identity: %+v", e)
		}
		if seen[e.Slug] {
			t.Fatalf("duplicate slug %q", e.Slug)
		}
		seen[e.Slug] = true
	}
}

// The list is what the library renders in order, so it must not depend on
// however the filesystem happened to enumerate the directories.
func TestListIsOrderedByCategoryThenName(t *testing.T) {
	entries, err := List()
	if err != nil {
		t.Fatalf("List: %v", err)
	}
	for i := 1; i < len(entries); i++ {
		prev, cur := entries[i-1], entries[i]
		if prev.Category > cur.Category {
			t.Fatalf("category order broken at %d: %q then %q", i, prev.Category, cur.Category)
		}
		if prev.Category == cur.Category && prev.Name > cur.Name {
			t.Fatalf("name order broken in %q: %q then %q", cur.Category, prev.Name, cur.Name)
		}
	}
}

// A mutable copy would let one caller reorder every later caller's list.
func TestListDoesNotExposeTheSharedSlice(t *testing.T) {
	first, err := List()
	if err != nil {
		t.Fatalf("List: %v", err)
	}
	if len(first) == 0 {
		t.Fatal("empty catalogue")
	}
	original := first[0]
	first[0] = Entry{Slug: "mutated"}
	second, err := List()
	if err != nil {
		t.Fatalf("List: %v", err)
	}
	if second[0].Slug != original.Slug || second[0].Name != original.Name {
		t.Fatalf("caller mutation leaked into the catalogue: %+v", second[0])
	}
}

func TestGetReturnsContentForAKnownSlug(t *testing.T) {
	entries, err := List()
	if err != nil {
		t.Fatalf("List: %v", err)
	}
	detail, ok, err := Get(entries[0].Slug)
	if err != nil || !ok {
		t.Fatalf("Get(%q) = ok:%v err:%v", entries[0].Slug, ok, err)
	}
	if strings.TrimSpace(detail.TokensCSS) == "" {
		t.Fatal("tokens.css is empty")
	}
	if strings.TrimSpace(detail.DesignMarkdown) == "" {
		t.Fatal("DESIGN.md is empty")
	}
}

// An unknown slug is a 404, not a failure — and a traversal attempt is just
// another unknown slug, because Get indexes a map rather than building a path.
func TestGetRejectsUnknownAndTraversalSlugs(t *testing.T) {
	for _, slug := range []string{"", "nope", "../data", "../../go.mod", "apple/../apple"} {
		detail, ok, err := Get(slug)
		if err != nil {
			t.Fatalf("Get(%q) errored: %v", slug, err)
		}
		if ok {
			t.Fatalf("Get(%q) resolved to %+v", slug, detail.Entry)
		}
	}
}

// Nearly every bundled package ships Open Design's token-driven showcase
// (system/kit.html plus a dark variant). The catalogue must expose it under a
// bundle digest so the library can frame it and cache it immutably, and must
// refuse anything but the two known variants.
func TestShowcaseIsServedByDigestAndVariant(t *testing.T) {
	entries, err := List()
	if err != nil {
		t.Fatalf("List: %v", err)
	}
	withShowcase := 0
	for _, entry := range entries {
		if entry.ShowcaseDigest == "" {
			continue
		}
		withShowcase++
		if len(entry.ShowcaseDigest) != 12 {
			t.Fatalf("%s showcase digest %q is not 12 hex chars", entry.Slug, entry.ShowcaseDigest)
		}
		light, ok := Showcase(entry.Slug, "light")
		if !ok || !strings.Contains(string(light), "<html") {
			t.Fatalf("%s light showcase missing", entry.Slug)
		}
		if _, ok := Showcase(entry.Slug, "poster"); ok {
			t.Fatalf("%s served an unknown showcase variant", entry.Slug)
		}
	}
	if withShowcase < 100 {
		t.Fatalf("only %d packages expose a showcase", withShowcase)
	}
	if _, ok := Showcase("nope", "light"); ok {
		t.Fatal("unknown slug served a showcase")
	}
}

// The digest is a cache key: it must change when the showcase changes and be
// identical for identical bundles, so it is derived from file contents.
func TestShowcaseDigestFollowsTheBundleContents(t *testing.T) {
	a := bundleDigest(map[string][]byte{"system/kit.html": []byte("<html>a</html>")})
	b := bundleDigest(map[string][]byte{"system/kit.html": []byte("<html>b</html>")})
	again := bundleDigest(map[string][]byte{"system/kit.html": []byte("<html>a</html>")})
	if a == b || a != again || len(a) != 12 {
		t.Fatalf("digests a=%q b=%q again=%q", a, b, again)
	}
}

// A list row's stripe tile follows Open Design's four-slot rule: background,
// support, foreground, accent, picked by name from DESIGN.md's own colours.
// The Agentic preset resolves to its documented row; nothing carries more
// than the four slots or a non-concrete value.
func TestEntriesCarryTheFourSlotSwatchRow(t *testing.T) {
	entries, err := List()
	if err != nil {
		t.Fatalf("List: %v", err)
	}
	withSwatches := 0
	for _, entry := range entries {
		if entry.Swatches == nil {
			t.Fatalf("%s swatches are nil rather than empty", entry.Slug)
		}
		if len(entry.Swatches) > 4 {
			t.Fatalf("%s carries %d swatches", entry.Slug, len(entry.Swatches))
		}
		for _, value := range entry.Swatches {
			if strings.Contains(value, "var(") || strings.TrimSpace(value) == "" {
				t.Fatalf("%s swatch %q is not a concrete colour", entry.Slug, value)
			}
		}
		if len(entry.Swatches) > 0 {
			withSwatches++
		}
	}
	if withSwatches < 140 {
		t.Fatalf("only %d packages carry swatches", withSwatches)
	}
	for _, entry := range entries {
		if entry.Slug != "agentic" {
			continue
		}
		want := []string{"#ffffff", "#f6f6f1", "#111827", "#ff5701"}
		if len(entry.Swatches) != 4 {
			t.Fatalf("agentic swatches = %v", entry.Swatches)
		}
		for i, value := range want {
			if entry.Swatches[i] != value {
				t.Fatalf("agentic swatches = %v, want %v", entry.Swatches, want)
			}
		}
	}
}

// The detail carries what Open Design's kit view renders, parsed from the
// package's own files: the style foundations from DESIGN.md's Color section
// (with OD's fixed role → token names, Neutral's derived entry excluded), the
// three type families with their weights, the layout guidelines, the token
// contract in file order, and the artifact pages in OD's display order.
func TestDetailCarriesTheKitViewContent(t *testing.T) {
	detail, ok, err := Get("agentic")
	if err != nil || !ok {
		t.Fatalf("Get(agentic) = ok:%v err:%v", ok, err)
	}
	if detail.Identity == "" || strings.Contains(detail.Identity, "Category:") {
		t.Fatalf("identity = %q", detail.Identity)
	}
	wantPalette := []PaletteEntry{
		{Name: "Primary", Role: "accent", Value: "#FF5701", Usage: "Token from style foundations."},
		{Name: "Secondary", Role: "accent-secondary", Value: "#F6F6F1", Usage: "Token from style foundations."},
		{Name: "Success", Role: "success", Value: "#16A34A", Usage: "Token from style foundations."},
		{Name: "Warning", Role: "warning", Value: "#D97706", Usage: "Token from style foundations."},
		{Name: "Danger", Role: "danger", Value: "#DC2626", Usage: "Token from style foundations."},
		{Name: "Surface", Role: "surface", Value: "#FFFFFF", Usage: "Token from style foundations."},
		{Name: "Text", Role: "foreground", Value: "#111827", Usage: "Token from style foundations."},
	}
	// The derived Neutral bullet and the prose bullets repeat hexes above, so
	// de-dup by hex must leave exactly these seven cards.
	if len(detail.Palette) != len(wantPalette) {
		t.Fatalf("palette = %+v", detail.Palette)
	}
	for i, want := range wantPalette {
		if detail.Palette[i] != want {
			t.Fatalf("palette[%d] = %+v, want %+v", i, detail.Palette[i], want)
		}
	}
	if detail.Typography.Display != "Playfair Display" || detail.Typography.Body != "Playfair Display" ||
		detail.Typography.Mono != "JetBrains Mono" {
		t.Fatalf("typography = %+v", detail.Typography)
	}
	if len(detail.Typography.Weights) != 9 || detail.Typography.Weights[0] != "100" || detail.Typography.Weights[8] != "900" {
		t.Fatalf("weights = %+v", detail.Typography.Weights)
	}
	if detail.Title != "Design System Inspired by Agentic" {
		t.Fatalf("title = %q", detail.Title)
	}
	if len(detail.LayoutGuidelines) == 0 || detail.LayoutGuidelines[0] != "Spacing scale: 8pt baseline grid" {
		t.Fatalf("layout guidelines = %+v", detail.LayoutGuidelines)
	}
	wantContract := []TokenContractEntry{
		{Name: "colorPrimary", Value: "#60a5fa"},
		{Name: "colorPrimaryBg", Value: "#182343"},
		{Name: "colorPrimaryHover", Value: "color-mix(in oklab, var(--accent), black 8%)"},
		{Name: "colorPrimaryActive", Value: "color-mix(in oklab, var(--accent), black 14%)"},
		{Name: "fontSize", Value: "15"},
		{Name: "borderRadius", Value: "12"},
	}
	if len(detail.TokenContract) != len(wantContract) {
		t.Fatalf("token contract = %+v", detail.TokenContract)
	}
	for i, want := range wantContract {
		if detail.TokenContract[i] != want {
			t.Fatalf("token contract[%d] = %+v, want %+v", i, detail.TokenContract[i], want)
		}
	}
	if len(detail.Artifacts) != 6 || detail.Artifacts[0] != (Artifact{ID: "landing", Label: "Landing page"}) ||
		detail.Artifacts[1].ID != "deck" || detail.Artifacts[5].ID != "form" {
		t.Fatalf("artifacts = %+v", detail.Artifacts)
	}
	// The artifact documents are served under the same digest as the kits.
	if body, ok := Showcase("agentic", "artifact-landing"); !ok || !strings.Contains(string(body), "Landing module") {
		t.Fatal("artifact-landing is not served")
	}
	if _, ok := Showcase("agentic", "artifact-nope"); ok {
		t.Fatal("unknown artifact variant was served")
	}
}

// Palette parsing holds across the whole bundled set: every package's colour
// section yields at least one card, and hand-authored formats (the HUD table,
// the Totality front matter) parse too.
func TestPaletteParsesAcrossTheBundle(t *testing.T) {
	entries, err := List()
	if err != nil {
		t.Fatalf("List: %v", err)
	}
	complete := 0
	for _, entry := range entries {
		detail, ok, err := Get(entry.Slug)
		if err != nil || !ok {
			t.Fatalf("Get(%s): ok:%v err:%v", entry.Slug, ok, err)
		}
		if len(detail.Palette) > 0 {
			complete++
		}
		if len(detail.Palette) > 12 {
			t.Fatalf("%s palette has %d entries", entry.Slug, len(detail.Palette))
		}
	}
	if complete < 140 {
		t.Fatalf("only %d packages parse all seven foundations", complete)
	}
}
