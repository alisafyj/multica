// Package designsystemcatalogue serves the built-in design systems the design
// centre's 官方 scope lists.
//
// These are read-only reference packages adapted from the Open Design
// catalogue, not workspace data: nobody edits them, they are identical for
// every workspace, and they carry no ownership. That is why they live in the
// binary rather than in project_design_system, which exists to hold a
// project's own generated-and-saved system (DC-052). Keeping the two apart
// means a built-in can never be mistaken for a saved system, and shipping a
// new one is a code change with a diff rather than a data migration.
//
// Each package carries the three files the catalogue reads: manifest.json for
// identity, tokens.css for the visual values the library renders, and
// DESIGN.md for the design language itself. Most also carry Open Design's
// token-driven showcase (system/kit.html and its dark variant), which the
// library frames as the system's cover the way Open Design's own tab does.
package designsystemcatalogue

import (
	"crypto/sha256"
	"embed"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io"
	"io/fs"
	"path"
	"sort"
	"strings"
	"sync"
)

//go:embed all:data
var packages embed.FS

// Entry is one built-in design system as the catalogue list presents it.
// Deliberately without token or design content: the list renders 150+ of
// these, and shipping every package's full text would make the response
// megabytes for a screen that shows names.
type Entry struct {
	Slug string `json:"slug"`
	Name string `json:"name"`
	// Category is Open Design's own zh label where it has one, falling back to
	// the English facet. The catalogue is a Chinese product surface, so the
	// English key stays an internal grouping value rather than something a
	// reader sees.
	Category    string `json:"category"`
	Description string `json:"description"`
	// ShowcaseDigest identifies the package's showcase bundle (the kit
	// documents, by path and content); "" when the package ships none. The
	// showcase URL carries it so a cover caches immutably and a new build is a
	// new URL — the same rule the recipe covers follow.
	ShowcaseDigest string `json:"showcase_digest"`
	// Swatches are the first few concrete colour values the package declares,
	// in declaration order, so a list row can show the palette at a glance
	// without fetching the detail. Empty for packages without design-tokens.json.
	Swatches []string `json:"swatches"`
}

const maxSwatches = 6

// showcaseFiles maps a showcase variant onto the package file that renders it.
var showcaseFiles = map[string]string{
	"light": "system/kit.html",
	"dark":  "system/kit.dark.html",
}

// Token is one declared design token, typed at the source.
//
// Read from the package's design-tokens.json rather than scraped out of the
// stylesheet: the file already states which tokens are colours and which are
// type, so the detail view groups them by fact instead of by a regex guess.
type Token struct {
	Name  string `json:"name"`
	Value string `json:"value"`
	Type  string `json:"type"`
}

// Detail adds the content one system's page needs.
type Detail struct {
	Entry
	// Tokens is empty for the few packages that ship no design-tokens.json;
	// TokensCSS is always present, so the view can still show the raw sheet.
	Tokens []Token `json:"tokens"`
	// TokensCSS is the package's own custom-property sheet.
	TokensCSS string `json:"tokens_css"`
	// DesignMarkdown is the design language in prose, translated where the
	// package ships a zh version.
	DesignMarkdown string `json:"design_markdown"`
}

type manifest struct {
	ID            string `json:"id"`
	Name          string `json:"name"`
	Category      string `json:"category"`
	CategoryZH    string `json:"category_zh"`
	Description   string `json:"description"`
	DescriptionZH string `json:"description_zh"`
}

// designTokens is the shape of a package's design-tokens.json.
type designTokens struct {
	Tokens []Token `json:"tokens"`
}

// preferZH returns the translated string when the package carries one. Open
// Design translated most but not every package, and an English fallback reads
// better than a blank field.
func preferZH(zh, fallback string) string {
	if zh != "" {
		return zh
	}
	return fallback
}

var (
	once    sync.Once
	entries []Entry
	bySlug  map[string]Entry
	loadErr error
)

func load() {
	dirs, err := fs.ReadDir(packages, "data")
	if err != nil {
		loadErr = fmt.Errorf("read design system catalogue: %w", err)
		return
	}
	bySlug = make(map[string]Entry, len(dirs))
	for _, dir := range dirs {
		if !dir.IsDir() {
			continue
		}
		raw, err := packages.ReadFile(path.Join("data", dir.Name(), "manifest.json"))
		if err != nil {
			loadErr = fmt.Errorf("read %s manifest: %w", dir.Name(), err)
			return
		}
		var m manifest
		if err := json.Unmarshal(raw, &m); err != nil {
			loadErr = fmt.Errorf("decode %s manifest: %w", dir.Name(), err)
			return
		}
		// The directory name is the addressable identity, so a manifest whose
		// id disagrees with it would make the detail route unreachable.
		if m.ID != dir.Name() {
			loadErr = fmt.Errorf("design system %s declares id %q", dir.Name(), m.ID)
			return
		}
		entry := Entry{
			Slug:        m.ID,
			Name:        m.Name,
			Category:    preferZH(m.CategoryZH, m.Category),
			Description: preferZH(m.DescriptionZH, m.Description),
		}
		entry.Swatches = swatchesFromTokens(dir.Name())
		showcase := map[string][]byte{}
		for _, file := range showcaseFiles {
			if body, err := packages.ReadFile(path.Join("data", dir.Name(), file)); err == nil {
				showcase[file] = body
			}
		}
		if len(showcase) > 0 {
			entry.ShowcaseDigest = bundleDigest(showcase)
		}
		entries = append(entries, entry)
		bySlug[entry.Slug] = entry
	}
	sort.Slice(entries, func(i, j int) bool {
		if entries[i].Category != entries[j].Category {
			return entries[i].Category < entries[j].Category
		}
		return entries[i].Name < entries[j].Name
	})
}

// swatchesFromTokens reads a package's design-tokens.json and keeps the first
// concrete colour values it declares (aliases such as var(--x) are skipped:
// they render as nothing in a swatch). Never nil, so the JSON is [] not null.
func swatchesFromTokens(slug string) []string {
	swatches := []string{}
	raw, err := packages.ReadFile(path.Join("data", slug, "design-tokens.json"))
	if err != nil {
		return swatches
	}
	var parsed designTokens
	if json.Unmarshal(raw, &parsed) != nil {
		return swatches
	}
	seen := map[string]bool{}
	for _, token := range parsed.Tokens {
		value := strings.TrimSpace(token.Value)
		if token.Type != "color" || value == "" || strings.Contains(value, "var(") || seen[value] {
			continue
		}
		seen[value] = true
		swatches = append(swatches, value)
		if len(swatches) == maxSwatches {
			break
		}
	}
	return swatches
}

// bundleDigest hashes a set of files by path and content, in path order.
// Twelve hex chars is plenty for a cache key that only has to differ between
// builds.
func bundleDigest(files map[string][]byte) string {
	names := make([]string, 0, len(files))
	for name := range files {
		names = append(names, name)
	}
	sort.Strings(names)
	h := sha256.New()
	for _, name := range names {
		_, _ = io.WriteString(h, name)
		_, _ = h.Write([]byte{0})
		_, _ = h.Write(files[name])
		_, _ = h.Write([]byte{0})
	}
	return hex.EncodeToString(h.Sum(nil))[:12]
}

// Showcase returns the showcase document of one variant ("light" or "dark").
// Reports ok=false for an unknown slug, a package without a showcase, or a
// variant that is not one of the two, so the handler answers 404.
func Showcase(slug, variant string) ([]byte, bool) {
	once.Do(load)
	if loadErr != nil {
		return nil, false
	}
	entry, ok := bySlug[strings.TrimSpace(slug)]
	if !ok || entry.ShowcaseDigest == "" {
		return nil, false
	}
	file, ok := showcaseFiles[variant]
	if !ok {
		return nil, false
	}
	// The slug indexes an in-memory map built from directory names and the
	// variant indexes a fixed table, so neither can reach ReadFile as a path.
	body, err := packages.ReadFile(path.Join("data", entry.Slug, file))
	if err != nil {
		return nil, false
	}
	return body, true
}

// List returns every built-in system, grouped by category then name so the
// library's default order is stable rather than filesystem order.
func List() ([]Entry, error) {
	once.Do(load)
	if loadErr != nil {
		return nil, loadErr
	}
	out := make([]Entry, len(entries))
	copy(out, entries)
	return out, nil
}

// Get returns one system with its content. Reports ok=false for an unknown
// slug so the caller answers 404 rather than an error.
func Get(slug string) (Detail, bool, error) {
	once.Do(load)
	if loadErr != nil {
		return Detail{}, false, loadErr
	}
	// The slug indexes an in-memory map built from directory names, so a
	// hostile value can never reach ReadFile as a path.
	entry, ok := bySlug[strings.TrimSpace(slug)]
	if !ok {
		return Detail{}, false, nil
	}
	tokens, err := packages.ReadFile(path.Join("data", entry.Slug, "tokens.css"))
	if err != nil {
		return Detail{}, false, fmt.Errorf("read %s tokens: %w", entry.Slug, err)
	}
	design, err := packages.ReadFile(path.Join("data", entry.Slug, "DESIGN.md"))
	if err != nil {
		return Detail{}, false, fmt.Errorf("read %s design: %w", entry.Slug, err)
	}
	detail := Detail{Entry: entry, Tokens: []Token{}, TokensCSS: string(tokens), DesignMarkdown: string(design)}
	// Not every package ships one; a missing token file leaves the typed list
	// empty rather than failing the read.
	if raw, err := packages.ReadFile(path.Join("data", entry.Slug, "design-tokens.json")); err == nil {
		var parsed designTokens
		if json.Unmarshal(raw, &parsed) == nil && len(parsed.Tokens) > 0 {
			detail.Tokens = parsed.Tokens
		}
	}
	return detail, true, nil
}
