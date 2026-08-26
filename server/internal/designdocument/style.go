package designdocument

import (
	"bytes"
	"io"
	"io/fs"
	"net/url"
	"path"
	"strings"

	parse "github.com/tdewolff/parse/v2"
	"github.com/tdewolff/parse/v2/css"
)

// blockAtRules is the allow list of block at-rules a prototype stylesheet may
// open. An unknown block at-rule is rejected rather than skipped so a future
// browser feature cannot silently carry behaviour past this audit.
var blockAtRules = map[string]struct{}{
	"container": {}, "font-face": {}, "keyframes": {}, "layer": {}, "media": {},
	"page": {}, "property": {}, "scope": {}, "starting-style": {}, "supports": {},
}

// auditStyle parses prototype CSS with a real CSS parser and rejects anything
// that would make the prototype reach outside the package: @import, remote
// url() references and remote URL strings. Everything a prototype legitimately
// needs, including nesting and modern at-rules, stays available.
func auditStyle(source, basePath string, inline bool, artifacts map[string]ArtifactIndexEntry) []Diagnostic {
	diagnostics := make([]Diagnostic, 0)
	parser := css.NewParser(parse.NewInputString(source), inline)
	for {
		grammar, _, data := parser.Next()
		if grammar == css.ErrorGrammar {
			if err := parser.Err(); err != nil && err != io.EOF {
				diagnostics = append(diagnostics, errorDiagnostic("prototype_css_invalid", basePath, "prototype CSS is invalid"))
			}
			break
		}
		values := parser.Values()
		if grammar == css.AtRuleGrammar || grammar == css.BeginAtRuleGrammar {
			diagnostics = append(diagnostics, rejectEscapedCSSStructure(data, basePath)...)
			name := strings.TrimPrefix(strings.ToLower(strings.TrimSpace(string(data))), "@")
			if name == "import" {
				diagnostics = append(diagnostics, errorDiagnostic("prototype_css_import_forbidden", basePath, "CSS @import is not allowed"))
			}
			if grammar == css.BeginAtRuleGrammar {
				if _, ok := blockAtRules[name]; !ok {
					diagnostics = append(diagnostics, errorDiagnostic("prototype_css_at_rule_unsupported", basePath, "CSS block at-rule @"+name+" is not supported"))
				}
			}
		}
		if grammar == css.DeclarationGrammar || grammar == css.CustomPropertyGrammar {
			diagnostics = append(diagnostics, rejectEscapedCSSStructure(data, basePath)...)
		}
		diagnostics = append(diagnostics, inspectCSSTokens(values, basePath, artifacts)...)
	}
	return diagnostics
}

func inspectCSSTokens(tokens []css.Token, basePath string, artifacts map[string]ArtifactIndexEntry) []Diagnostic {
	diagnostics := make([]Diagnostic, 0)
	for _, token := range tokens {
		switch token.TokenType {
		case css.FunctionToken, css.IdentToken, css.AtKeywordToken, css.HashToken, css.CustomPropertyNameToken:
			diagnostics = append(diagnostics, rejectEscapedCSSStructure(token.Data, basePath)...)
		case css.URLToken:
			diagnostics = append(diagnostics, rejectEscapedCSSStructure(token.Data, basePath)...)
			if !validLocalStyleResource(cssURLValue(string(token.Data)), basePath, artifacts) {
				diagnostics = append(diagnostics, errorDiagnostic("prototype_css_url_unsafe", basePath, "CSS resources must be package-local assets or fonts"))
			}
		case css.BadURLToken:
			diagnostics = append(diagnostics, errorDiagnostic("prototype_css_url_unsafe", basePath, "CSS contains a malformed URL"))
		case css.StringToken:
			if cssStringIsRemoteURL(token.Data) {
				diagnostics = append(diagnostics, errorDiagnostic("prototype_css_url_unsafe", basePath, "CSS cannot contain remote URL strings"))
			}
		case css.CustomPropertyValueToken:
			diagnostics = append(diagnostics, inspectCSSValue(token.Data, basePath, artifacts)...)
		}
	}
	return diagnostics
}

func inspectCSSValue(value []byte, basePath string, artifacts map[string]ArtifactIndexEntry) []Diagnostic {
	diagnostics := make([]Diagnostic, 0)
	lexer := css.NewLexer(parse.NewInputBytes(value))
	for {
		tokenType, data := lexer.Next()
		if tokenType == css.ErrorToken {
			return diagnostics
		}
		switch tokenType {
		case css.FunctionToken, css.IdentToken, css.CustomPropertyNameToken:
			diagnostics = append(diagnostics, rejectEscapedCSSStructure(data, basePath)...)
		case css.URLToken:
			diagnostics = append(diagnostics, rejectEscapedCSSStructure(data, basePath)...)
			if !validLocalStyleResource(cssURLValue(string(data)), basePath, artifacts) {
				diagnostics = append(diagnostics, errorDiagnostic("prototype_css_url_unsafe", basePath, "CSS resources must be package-local assets or fonts"))
			}
		case css.BadURLToken:
			diagnostics = append(diagnostics, errorDiagnostic("prototype_css_url_unsafe", basePath, "CSS contains a malformed URL"))
		case css.StringToken:
			if cssStringIsRemoteURL(data) {
				diagnostics = append(diagnostics, errorDiagnostic("prototype_css_url_unsafe", basePath, "CSS cannot contain remote URL strings"))
			}
		}
	}
}

func rejectEscapedCSSStructure(data []byte, basePath string) []Diagnostic {
	if bytes.IndexByte(data, '\\') >= 0 {
		return []Diagnostic{errorDiagnostic("prototype_css_escape_unsupported", basePath, "CSS escapes are not supported in structural tokens")}
	}
	return nil
}

// validLocalStyleResource resolves a CSS url() against the stylesheet path and
// requires it to land on an asset or font declared by this package.
func validLocalStyleResource(raw, basePath string, artifacts map[string]ArtifactIndexEntry) bool {
	resolved, ok := resolveLocalResource(raw, basePath)
	if !ok {
		return false
	}
	if strings.HasPrefix(resolved, "#") {
		return true
	}
	entry, exists := artifacts[resolved]
	return exists && (entry.Role == "asset" || entry.Role == "font")
}

// resolveLocalResource turns a package-relative reference into a package path.
// Anything with a scheme, a host, credentials or a query, and anything that
// escapes the package root, fails.
func resolveLocalResource(raw, basePath string) (string, bool) {
	value := strings.TrimSpace(raw)
	if value == "" || strings.Contains(value, "\\") || strings.HasPrefix(value, "//") {
		return "", false
	}
	if strings.HasPrefix(value, "#") {
		return value, true
	}
	parsed, err := url.Parse(value)
	if err != nil || parsed.Scheme != "" || parsed.Host != "" || parsed.User != nil || parsed.RawQuery != "" {
		return "", false
	}
	decoded, err := url.PathUnescape(parsed.EscapedPath())
	if err != nil || decoded == "" || strings.HasPrefix(decoded, "/") || strings.Contains(decoded, "\\") {
		return "", false
	}
	resolved := path.Clean(path.Join(path.Dir(basePath), decoded))
	if resolved == "." || strings.HasPrefix(resolved, "../") || !fs.ValidPath(resolved) {
		return "", false
	}
	return resolved, true
}

func cssURLValue(raw string) string {
	value := strings.TrimSpace(raw)
	if len(value) >= 4 && strings.EqualFold(value[:4], "url(") {
		value = strings.TrimSpace(value[4:])
	}
	if strings.HasSuffix(value, ")") {
		value = strings.TrimSpace(value[:len(value)-1])
	}
	if len(value) >= 2 && ((value[0] == '"' && value[len(value)-1] == '"') || (value[0] == '\'' && value[len(value)-1] == '\'')) {
		value = value[1 : len(value)-1]
	}
	return strings.TrimSpace(value)
}

func cssStringIsRemoteURL(data []byte) bool {
	value := normalizeForURLPolicy(decodeCSSString(data))
	parsed, err := url.Parse(value)
	return strings.HasPrefix(value, "//") || (err == nil && parsed.Scheme != "")
}

// normalizeForURLPolicy strips the whitespace and control characters browsers
// ignore inside a URL, so a padded or line-broken URL cannot slip past.
func normalizeForURLPolicy(value string) string {
	var normalized strings.Builder
	normalized.Grow(len(value))
	for _, character := range value {
		if character != '\t' && character != '\n' && character != '\r' {
			normalized.WriteRune(character)
		}
	}
	return strings.TrimFunc(normalized.String(), func(character rune) bool {
		return character >= 0 && character <= 0x20
	})
}

// decodeCSSString unquotes and unescapes a CSS string so an escaped scheme
// cannot hide a remote URL from the policy check.
func decodeCSSString(data []byte) string {
	value := strings.TrimSpace(string(data))
	if len(value) >= 2 && ((value[0] == '"' && value[len(value)-1] == '"') || (value[0] == '\'' && value[len(value)-1] == '\'')) {
		value = value[1 : len(value)-1]
	}
	var decoded strings.Builder
	decoded.Grow(len(value))
	for index := 0; index < len(value); {
		if value[index] != '\\' {
			decoded.WriteByte(value[index])
			index++
			continue
		}
		index++
		if index == len(value) {
			break
		}
		if digit, ok := hexDigit(value[index]); ok {
			codePoint := uint32(digit)
			digits := 1
			index++
			for index < len(value) && digits < 6 {
				digit, ok = hexDigit(value[index])
				if !ok {
					break
				}
				codePoint = codePoint*16 + uint32(digit)
				digits++
				index++
			}
			if index < len(value) && cssWhitespace(value[index]) {
				if value[index] == '\r' && index+1 < len(value) && value[index+1] == '\n' {
					index++
				}
				index++
			}
			if codePoint == 0 || codePoint > 0x10ffff || (0xd800 <= codePoint && codePoint <= 0xdfff) {
				decoded.WriteRune('�')
			} else {
				decoded.WriteRune(rune(codePoint))
			}
			continue
		}
		switch value[index] {
		case '\n', '\f':
			index++
		case '\r':
			index++
			if index < len(value) && value[index] == '\n' {
				index++
			}
		default:
			decoded.WriteByte(value[index])
			index++
		}
	}
	return decoded.String()
}

func hexDigit(value byte) (byte, bool) {
	switch {
	case '0' <= value && value <= '9':
		return value - '0', true
	case 'a' <= value && value <= 'f':
		return value - 'a' + 10, true
	case 'A' <= value && value <= 'F':
		return value - 'A' + 10, true
	default:
		return 0, false
	}
}

func cssWhitespace(value byte) bool {
	return value == ' ' || value == '\t' || value == '\n' || value == '\r' || value == '\f'
}
