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

// A list row shows the palette at a glance: a handful of concrete colours in
// declaration order, never aliases, never more than the cap.
func TestEntriesCarryAFewConcreteSwatches(t *testing.T) {
	entries, err := List()
	if err != nil {
		t.Fatalf("List: %v", err)
	}
	withSwatches := 0
	for _, entry := range entries {
		if entry.Swatches == nil {
			t.Fatalf("%s swatches are nil rather than empty", entry.Slug)
		}
		if len(entry.Swatches) > maxSwatches {
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
	if withSwatches < 100 {
		t.Fatalf("only %d packages carry swatches", withSwatches)
	}
}
