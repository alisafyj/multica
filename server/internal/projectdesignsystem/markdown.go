package projectdesignsystem

import (
	"crypto/sha256"
	"encoding/hex"
	"strconv"
	"strings"
	"unicode"

	"github.com/yuin/goldmark"
	"github.com/yuin/goldmark/ast"
	"github.com/yuin/goldmark/parser"
	"github.com/yuin/goldmark/text"
)

type markdownHeading struct {
	title string
	start int
	end   int
}

func parseMarkdownSections(source string) ([]Section, []Diagnostic) {
	raw := []byte(source)
	document := goldmark.DefaultParser().Parse(
		text.NewReader(raw),
		parser.WithContext(parser.NewContext()),
	)

	var headings []markdownHeading
	_ = ast.Walk(document, func(node ast.Node, entering bool) (ast.WalkStatus, error) {
		if !entering {
			return ast.WalkContinue, nil
		}
		heading, ok := node.(*ast.Heading)
		if !ok || (heading.Level != 1 && heading.Level != 2) || heading.Lines().Len() == 0 {
			return ast.WalkContinue, nil
		}
		segment := heading.Lines().At(0)
		headings = append(headings, markdownHeading{
			title: strings.TrimSpace(string(heading.Text(raw))),
			start: lineStart(raw, segment.Start),
			end:   lineEnd(raw, segment.Stop),
		})
		return ast.WalkContinue, nil
	})

	if len(headings) == 0 {
		return nil, []Diagnostic{errorDiagnostic(
			"design_markdown_structure_missing",
			"DESIGN.md",
			"DESIGN.md must contain a title or at least one level-two section",
		)}
	}

	seen := make(map[string]int, len(headings))
	sections := make([]Section, 0, len(headings))
	for index, heading := range headings {
		if heading.title == "" {
			return nil, []Diagnostic{errorDiagnostic(
				"design_markdown_heading_empty",
				"DESIGN.md",
				"DESIGN.md contains an empty section heading",
			)}
		}
		contentEnd := len(raw)
		if index+1 < len(headings) {
			contentEnd = headings[index+1].start
		}
		id := uniqueSectionID(sectionID(heading.title), seen)
		sections = append(sections, Section{
			ID:       id,
			Title:    heading.title,
			Markdown: strings.TrimSpace(string(raw[heading.end:contentEnd])),
		})
	}
	return sections, nil
}

func lineStart(source []byte, offset int) int {
	if offset > len(source) {
		offset = len(source)
	}
	for offset > 0 && source[offset-1] != '\n' {
		offset--
	}
	return offset
}

func lineEnd(source []byte, offset int) int {
	if offset > len(source) {
		offset = len(source)
	}
	for offset < len(source) && source[offset] != '\n' {
		offset++
	}
	if offset < len(source) {
		offset++
	}
	return offset
}

func sectionID(title string) string {
	var builder strings.Builder
	separator := false
	for _, r := range strings.ToLower(title) {
		switch {
		case unicode.IsLetter(r) || unicode.IsDigit(r):
			if separator && builder.Len() > 0 {
				builder.WriteByte('-')
			}
			separator = false
			builder.WriteRune(r)
		default:
			separator = true
		}
	}
	id := strings.Trim(builder.String(), "-")
	if id != "" {
		return id
	}
	sum := sha256.Sum256([]byte(title))
	return "section-" + hex.EncodeToString(sum[:6])
}

func uniqueSectionID(base string, seen map[string]int) string {
	seen[base]++
	if seen[base] == 1 {
		return base
	}
	return base + "-" + strconv.Itoa(seen[base])
}
