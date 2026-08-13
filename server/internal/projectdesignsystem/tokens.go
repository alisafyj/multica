package projectdesignsystem

import (
	"io"
	"net/url"
	"sort"
	"strings"

	parse "github.com/tdewolff/parse/v2"
	"github.com/tdewolff/parse/v2/css"
)

type tokenParseResult struct {
	groups      []TokenGroup
	declared    map[string]struct{}
	references  map[string]struct{}
	diagnostics []Diagnostic
}

func parseTokens(source string, allowedHosts map[string]struct{}) tokenParseResult {
	result := tokenParseResult{
		declared:   make(map[string]struct{}),
		references: make(map[string]struct{}),
	}
	if strings.Contains(strings.ToLower(source), "</style") {
		result.diagnostics = append(result.diagnostics, errorDiagnostic(
			"tokens_css_unsafe_text",
			"tokens.css",
			"tokens.css cannot close the trusted preview style element",
		))
		return result
	}

	parser := css.NewParser(parse.NewInputString(source), false)
	groupOrder := make([]string, 0)
	groups := make(map[string][]TokenValue)
	openBlocks := 0
	invalidStructure := false
	for {
		grammar, _, data := parser.Next()
		if grammar == css.ErrorGrammar {
			if err := parser.Err(); err != nil && err != io.EOF {
				result.diagnostics = append(result.diagnostics, errorDiagnostic(
					"tokens_css_invalid",
					"tokens.css",
					"tokens.css is not valid CSS: "+err.Error(),
				))
			}
			if openBlocks != 0 && !invalidStructure {
				result.diagnostics = append(result.diagnostics, errorDiagnostic(
					"tokens_css_invalid",
					"tokens.css",
					"tokens.css contains an unclosed rule block",
				))
			}
			break
		}
		switch grammar {
		case css.BeginAtRuleGrammar, css.BeginRulesetGrammar:
			openBlocks++
		case css.EndAtRuleGrammar, css.EndRulesetGrammar:
			if strings.TrimSpace(string(data)) != "}" {
				if !invalidStructure {
					result.diagnostics = append(result.diagnostics, errorDiagnostic(
						"tokens_css_invalid",
						"tokens.css",
						"tokens.css contains an unclosed rule block",
					))
				}
				invalidStructure = true
			}
			if openBlocks > 0 {
				openBlocks--
			}
		}

		if grammar == css.AtRuleGrammar || grammar == css.BeginAtRuleGrammar {
			name := strings.TrimPrefix(strings.ToLower(strings.TrimSpace(string(data))), "@")
			if name == "import" {
				result.diagnostics = append(result.diagnostics, errorDiagnostic(
					"tokens_css_import_forbidden",
					"tokens.css",
					"CSS imports are not allowed",
				))
			}
		}

		values := parser.Values()
		inspectCSSTokens(values, allowedHosts, "tokens.css", &result)
		if grammar != css.CustomPropertyGrammar {
			continue
		}

		name := strings.TrimSpace(string(data))
		if _, exists := result.declared[name]; exists {
			result.diagnostics = append(result.diagnostics, errorDiagnostic(
				"token_duplicate",
				"tokens.css",
				"Token "+name+" is declared more than once",
			))
			continue
		}
		result.declared[name] = struct{}{}
		groupID := tokenGroupID(name)
		if _, exists := groups[groupID]; !exists {
			groupOrder = append(groupOrder, groupID)
		}
		groups[groupID] = append(groups[groupID], TokenValue{
			Name:  name,
			Value: cssTokenText(values),
		})
	}

	if len(result.declared) == 0 {
		result.diagnostics = append(result.diagnostics, errorDiagnostic(
			"token_declaration_missing",
			"tokens.css",
			"tokens.css must declare at least one custom property",
		))
	}
	for reference := range result.references {
		if _, exists := result.declared[reference]; !exists {
			result.diagnostics = append(result.diagnostics, errorDiagnostic(
				"token_reference_unknown",
				"tokens.css",
				"Token reference "+reference+" is not declared",
			))
		}
	}
	if len(result.references) == 0 {
		result.diagnostics = append(result.diagnostics, errorDiagnostic(
			"token_usage_missing",
			"tokens.css",
			"The UI Kit stylesheet must use at least one declared Token",
		))
	}

	for _, groupID := range groupOrder {
		result.groups = append(result.groups, TokenGroup{
			ID:     groupID,
			Label:  tokenGroupLabel(groupID),
			Tokens: groups[groupID],
		})
	}
	return result
}

func inspectCSSTokens(values []css.Token, allowedHosts map[string]struct{}, path string, result *tokenParseResult) {
	for index, token := range values {
		switch token.TokenType {
		case css.CustomPropertyValueToken:
			inspectCSSValue(token.Data, allowedHosts, path, result)
		case css.FunctionToken:
			if strings.EqualFold(strings.TrimSpace(string(token.Data)), "var(") {
				for next := index + 1; next < len(values); next++ {
					candidate := strings.TrimSpace(string(values[next].Data))
					if candidate == "" {
						continue
					}
					if strings.HasPrefix(candidate, "--") {
						result.references[candidate] = struct{}{}
					}
					break
				}
			}
		case css.URLToken:
			if !isAllowedResourceURL(cssURLValue(string(token.Data)), allowedHosts) {
				result.diagnostics = append(result.diagnostics, errorDiagnostic(
					"tokens_css_url_unsafe",
					path,
					"CSS contains an unapproved resource URL",
				))
			}
		case css.BadURLToken:
			result.diagnostics = append(result.diagnostics, errorDiagnostic(
				"tokens_css_url_unsafe",
				path,
				"CSS contains a malformed resource URL",
			))
		}
	}
}

func inspectCSSValue(value []byte, allowedHosts map[string]struct{}, path string, result *tokenParseResult) {
	lexer := css.NewLexer(parse.NewInputBytes(value))
	expectVariable := false
	for {
		tokenType, data := lexer.Next()
		if tokenType == css.ErrorToken {
			if err := lexer.Err(); err != nil && err != io.EOF {
				result.diagnostics = append(result.diagnostics, errorDiagnostic(
					"tokens_css_invalid",
					path,
					"CSS contains an invalid custom property value",
				))
			}
			return
		}
		switch tokenType {
		case css.FunctionToken:
			expectVariable = strings.EqualFold(strings.TrimSpace(string(data)), "var(")
		case css.IdentToken, css.CustomPropertyNameToken:
			if expectVariable {
				candidate := strings.TrimSpace(string(data))
				if strings.HasPrefix(candidate, "--") {
					result.references[candidate] = struct{}{}
				}
			}
			expectVariable = false
		case css.URLToken:
			if !isAllowedResourceURL(cssURLValue(string(data)), allowedHosts) {
				result.diagnostics = append(result.diagnostics, errorDiagnostic(
					"tokens_css_url_unsafe",
					path,
					"CSS contains an unapproved resource URL",
				))
			}
			expectVariable = false
		case css.BadURLToken:
			result.diagnostics = append(result.diagnostics, errorDiagnostic(
				"tokens_css_url_unsafe",
				path,
				"CSS contains a malformed resource URL",
			))
			expectVariable = false
		case css.WhitespaceToken:
		default:
			if tokenType != css.CommentToken {
				expectVariable = false
			}
		}
	}
}

func cssTokenText(values []css.Token) string {
	var builder strings.Builder
	for _, token := range values {
		builder.Write(token.Data)
	}
	return strings.TrimSpace(builder.String())
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

func isAllowedResourceURL(raw string, allowedHosts map[string]struct{}) bool {
	value := strings.TrimSpace(raw)
	if value == "" || strings.HasPrefix(value, "#") {
		return true
	}
	if strings.HasPrefix(strings.ToLower(value), "data:image/") {
		return true
	}
	parsed, err := url.Parse(value)
	if err != nil || parsed.Scheme != "https" || parsed.User != nil || parsed.Hostname() == "" {
		return false
	}
	_, ok := allowedHosts[strings.ToLower(parsed.Hostname())]
	return ok
}

func tokenGroupID(name string) string {
	name = strings.TrimPrefix(name, "--")
	if index := strings.IndexByte(name, '-'); index > 0 {
		return name[:index]
	}
	return name
}

func tokenGroupLabel(groupID string) string {
	labels := map[string]string{
		"color":      "Color",
		"font":       "Typography",
		"typography": "Typography",
		"space":      "Spacing",
		"spacing":    "Spacing",
		"radius":     "Radius",
		"shadow":     "Shadow",
		"motion":     "Motion",
	}
	if label, ok := labels[groupID]; ok {
		return label
	}
	parts := strings.FieldsFunc(groupID, func(r rune) bool { return r == '-' || r == '_' })
	for index := range parts {
		if parts[index] != "" {
			parts[index] = strings.ToUpper(parts[index][:1]) + parts[index][1:]
		}
	}
	return strings.Join(parts, " ")
}

func normalizeAllowedHosts(hosts []string) map[string]struct{} {
	normalized := make(map[string]struct{}, len(hosts))
	for _, raw := range hosts {
		host := strings.TrimSpace(strings.ToLower(raw))
		if parsed, err := url.Parse(host); err == nil && parsed.Hostname() != "" {
			host = strings.ToLower(parsed.Hostname())
		} else {
			host = strings.TrimSuffix(strings.TrimPrefix(host, "https://"), "/")
			if colon := strings.IndexByte(host, ':'); colon >= 0 {
				host = host[:colon]
			}
		}
		if host != "" {
			normalized[host] = struct{}{}
		}
	}
	return normalized
}

func sortedAllowedHosts(hosts map[string]struct{}) []string {
	values := make([]string, 0, len(hosts))
	for host := range hosts {
		values = append(values, host)
	}
	sort.Strings(values)
	return values
}
