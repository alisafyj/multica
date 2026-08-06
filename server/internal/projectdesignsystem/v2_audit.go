package projectdesignsystem

import (
	"bytes"
	"encoding/json"
	"encoding/xml"
	"io"
	"io/fs"
	"net/url"
	"path"
	"strings"

	parse "github.com/tdewolff/parse/v2"
	"github.com/tdewolff/parse/v2/css"
	"golang.org/x/net/html"
)

type v2CSSAudit struct {
	references     map[string]struct{}
	diagnostics    []Diagnostic
	hidden         bool
	documentHidden bool
}

type v2HTMLAudit struct {
	locators       []Locator
	diagnostics    []Diagnostic
	tokenUsed      bool
	visible        bool
	documentHidden bool
}

func auditV2Package(
	files map[string][]byte,
	index []ArtifactIndexEntry,
	binding PackageBinding,
	contentDigest string,
	previewTargets []PreviewTarget,
) v2AuditResult {
	result := v2AuditResult{}
	diagnostics := make([]Diagnostic, 0)
	for _, required := range []string{"DESIGN.md", "tokens.css", "source/index.json"} {
		if len(bytes.TrimSpace(files[required])) == 0 {
			diagnostics = append(diagnostics, errorDiagnostic("artifact_missing", required, required+" must be present and non-empty"))
		}
	}
	if len(diagnostics) == 0 {
		sections, sectionDiagnostics := parseMarkdownSections(string(files["DESIGN.md"]))
		result.sections = sections
		diagnostics = append(diagnostics, sectionDiagnostics...)
	}

	tokens := parseTokens(string(files["tokens.css"]), nil)
	result.tokenGroups = tokens.groups
	for _, diagnostic := range tokens.diagnostics {
		if diagnostic.Code != "token_usage_missing" && diagnostic.Code != "tokens_css_url_unsafe" {
			diagnostics = append(diagnostics, diagnostic)
		}
	}
	artifactByPath := make(map[string]ArtifactIndexEntry, len(index))
	for _, entry := range index {
		artifactByPath[entry.Path] = entry
	}
	tokenCSS := inspectV2CSS(string(files["tokens.css"]), false, "tokens.css", files, artifactByPath, tokens.declared, false)
	diagnostics = append(diagnostics, tokenCSS.diagnostics...)

	sourceDiagnostics := auditV2SourceIndex(files["source/index.json"], binding.InputSnapshotSHA256)
	diagnostics = append(diagnostics, sourceDiagnostics...)
	for _, optionalJSON := range []string{"design-tokens.json", "components.manifest.json"} {
		if raw, exists := files[optionalJSON]; exists {
			var value any
			decoder := json.NewDecoder(bytes.NewReader(raw))
			decoder.UseNumber()
			if err := decoder.Decode(&value); err != nil || requireJSONEOF(decoder) != nil {
				diagnostics = append(diagnostics, errorDiagnostic("artifact_json_invalid", optionalJSON, optionalJSON+" must contain one valid JSON value"))
			}
		}
	}
	for _, entry := range index {
		if entry.MediaType == "image/svg+xml" {
			diagnostics = append(diagnostics, auditV2SVG(entry.Path, files[entry.Path], files, artifactByPath, tokens.declared)...)
		}
	}

	seenLocators := make(map[string]struct{})
	for _, target := range previewTargets {
		htmlAudit := auditV2HTML(target.Path, files[target.Path], files, artifactByPath, tokens.declared, seenLocators)
		result.locators = append(result.locators, htmlAudit.locators...)
		diagnostics = append(diagnostics, htmlAudit.diagnostics...)
		if !htmlAudit.visible {
			diagnostics = append(diagnostics, errorDiagnostic("ui_kit_not_visible", target.Path, "Preview target must contain visible content in a stable locator"))
		}
		if !htmlAudit.tokenUsed {
			diagnostics = append(diagnostics, errorDiagnostic("token_usage_missing", target.Path, "Preview target must use at least one Token declared in tokens.css"))
		}
	}
	result.report = AuditReport{
		SchemaVersion: AuditSchemaV1,
		Passed:        !hasErrors(diagnostics),
		ContentDigest: contentDigest,
		Diagnostics:   nonNilDiagnostics(diagnostics),
	}
	return result
}

func auditV2SourceIndex(raw []byte, expectedSnapshot string) []Diagnostic {
	var source SourceIndex
	if err := decodeStrictJSON(raw, &source); err != nil {
		return []Diagnostic{errorDiagnostic("source_index_invalid", "source/index.json", "source index is invalid: "+err.Error())}
	}
	diagnostics := make([]Diagnostic, 0)
	if source.SchemaVersion != SourceIndexSchemaV1 {
		diagnostics = append(diagnostics, errorDiagnostic("source_schema_invalid", "source/index.json", "source index schema is invalid"))
	}
	if source.InputSnapshotSHA256 != expectedSnapshot {
		diagnostics = append(diagnostics, errorDiagnostic("source_snapshot_mismatch", "source/index.json", "source index does not match the task input snapshot"))
	}
	if source.Evidence == nil || source.Conflicts == nil || source.Fallbacks == nil {
		diagnostics = append(diagnostics, errorDiagnostic("source_index_shape_invalid", "source/index.json", "source index arrays must be present"))
	}
	seen := make(map[string]struct{})
	for _, evidence := range source.Evidence {
		diagnostics = append(diagnostics, auditV2SourceRecord(evidence.ID, evidence.Kind, evidence.Summary, evidence.References, true, seen)...)
	}
	for _, conflict := range source.Conflicts {
		diagnostics = append(diagnostics, auditV2SourceRecord(conflict.ID, "conflict", conflict.Summary, conflict.References, true, seen)...)
	}
	for _, fallback := range source.Fallbacks {
		diagnostics = append(diagnostics, auditV2SourceRecord(fallback.ID, "fallback", fallback.Summary, fallback.References, false, seen)...)
	}
	return diagnostics
}

func auditV2SourceRecord(id, kind, summary string, references []string, referencesRequired bool, seen map[string]struct{}) []Diagnostic {
	diagnostics := make([]Diagnostic, 0)
	if !validSourceID(id) || strings.TrimSpace(kind) == "" || strings.TrimSpace(summary) == "" {
		diagnostics = append(diagnostics, errorDiagnostic("source_record_invalid", "source/index.json", "source records require stable IDs, kinds, and summaries"))
	}
	if _, exists := seen[id]; exists {
		diagnostics = append(diagnostics, errorDiagnostic("source_id_duplicate", "source/index.json", "source record IDs must be unique"))
	} else if id != "" {
		seen[id] = struct{}{}
	}
	if referencesRequired && len(references) == 0 {
		diagnostics = append(diagnostics, errorDiagnostic("source_reference_missing", "source/index.json", "source evidence and conflicts require references"))
	}
	for _, reference := range references {
		if !validV2SourceReference(reference) {
			diagnostics = append(diagnostics, errorDiagnostic("source_reference_invalid", "source/index.json", "source references must be snapshot IDs, credential-free HTTPS URLs, or safe repository-relative paths"))
		}
	}
	return diagnostics
}

func validV2SourceReference(reference string) bool {
	if reference == "" || reference != strings.TrimSpace(reference) || len(reference) > 2048 || strings.Contains(reference, "\\") ||
		strings.IndexFunc(reference, func(character rune) bool { return character < 0x20 || character == 0x7f }) >= 0 {
		return false
	}
	if strings.HasPrefix(reference, "~") && strings.IndexByte(reference, '/') > 0 {
		return false
	}
	parsed, err := url.Parse(reference)
	if err == nil && parsed.Scheme != "" {
		return parsed.Scheme == "https" && parsed.Host != "" && parsed.User == nil && parsed.RawQuery == "" &&
			!strings.Contains(parsed.Host, "@") && !v2URLFragmentContainsCredential(parsed.Fragment)
	}
	if strings.Contains(reference, "/") {
		return !strings.Contains(reference, ":") && !strings.HasPrefix(reference, "/") && fs.ValidPath(reference) && path.Clean(reference) == reference
	}
	return validSourceID(reference)
}

func v2URLFragmentContainsCredential(fragment string) bool {
	values, err := url.ParseQuery(fragment)
	if err != nil {
		return false
	}
	for key, candidates := range values {
		normalized := strings.ReplaceAll(strings.ToLower(strings.TrimSpace(key)), "-", "_")
		switch normalized {
		case "access_token", "api_key", "apikey", "authorization", "client_secret", "credential", "credentials", "password", "secret", "token":
			for _, candidate := range candidates {
				if strings.TrimSpace(candidate) != "" {
					return true
				}
			}
		}
	}
	return false
}

func validSourceID(value string) bool {
	if value == "" || len(value) > 256 {
		return false
	}
	for _, character := range value {
		if (character >= 'a' && character <= 'z') || (character >= 'A' && character <= 'Z') ||
			(character >= '0' && character <= '9') || strings.ContainsRune("._:-", character) {
			continue
		}
		return false
	}
	return true
}

func auditV2HTML(
	name string,
	raw []byte,
	files map[string][]byte,
	artifacts map[string]ArtifactIndexEntry,
	declaredTokens map[string]struct{},
	seenLocators map[string]struct{},
) v2HTMLAudit {
	result := v2HTMLAudit{}
	document, err := html.Parse(bytes.NewReader(raw))
	if err != nil {
		result.diagnostics = append(result.diagnostics, errorDiagnostic("html_invalid", name, "Preview target is not valid HTML"))
		return result
	}
	walkV2HTML(document, false, name, files, artifacts, declaredTokens, seenLocators, &result)
	if result.documentHidden {
		result.visible = false
	}
	if len(result.locators) == 0 {
		result.diagnostics = append(result.diagnostics, errorDiagnostic("locator_missing", name, "Preview target requires at least one stable locator"))
	}
	return result
}

func walkV2HTML(
	node *html.Node,
	hidden bool,
	name string,
	files map[string][]byte,
	artifacts map[string]ArtifactIndexEntry,
	declaredTokens map[string]struct{},
	seenLocators map[string]struct{},
	result *v2HTMLAudit,
) {
	if node.Type == html.ElementNode {
		tag := strings.ToLower(node.Data)
		if forbiddenV2HTMLElement(tag) {
			result.diagnostics = append(result.diagnostics, errorDiagnostic("html_forbidden_element", name, "Element <"+tag+"> is not allowed"))
		}
		hidden = hidden || v2ElementHidden(node)
		for _, attribute := range node.Attr {
			key := strings.ToLower(attribute.Key)
			value := strings.TrimSpace(attribute.Val)
			switch {
			case strings.HasPrefix(key, "on"):
				result.diagnostics = append(result.diagnostics, errorDiagnostic("html_event_handler", name, "HTML event attributes are not allowed"))
			case key == "srcdoc" || key == "srcset":
				result.diagnostics = append(result.diagnostics, errorDiagnostic("html_forbidden_attribute", name, "Attribute "+key+" is not allowed"))
			case key == "href" || key == "xlink:href":
				if value != "" && !strings.HasPrefix(value, "#") {
					result.diagnostics = append(result.diagnostics, errorDiagnostic("html_url_unsafe", name, "links must use fragment-only targets"))
				}
			case key == "src":
				if tag != "img" || !validV2LocalResource(value, name, "asset", artifacts) {
					result.diagnostics = append(result.diagnostics, errorDiagnostic("html_url_unsafe", name, "HTML resources must be package-local assets"))
				}
			case key == "action" || key == "formaction" || key == "poster" || key == "data" || key == "background" || key == "ping" || key == "cite":
				if value != "" {
					result.diagnostics = append(result.diagnostics, errorDiagnostic("html_url_unsafe", name, "navigation and embedded resource attributes are not allowed"))
				}
			case key == "http-equiv":
				result.diagnostics = append(result.diagnostics, errorDiagnostic("html_forbidden_attribute", name, "http-equiv is not allowed"))
			case key == "style":
				style := inspectV2CSS(value, true, name, files, artifacts, declaredTokens, true)
				result.diagnostics = append(result.diagnostics, style.diagnostics...)
				if style.hidden {
					hidden = true
				}
				if containsDeclaredV2Token(style.references, declaredTokens) {
					result.tokenUsed = true
				}
			}
		}
		if tag == "style" {
			var source strings.Builder
			for child := node.FirstChild; child != nil; child = child.NextSibling {
				if child.Type == html.TextNode {
					source.WriteString(child.Data)
				}
			}
			style := inspectV2CSS(source.String(), false, name, files, artifacts, declaredTokens, true)
			result.diagnostics = append(result.diagnostics, style.diagnostics...)
			if style.documentHidden {
				result.documentHidden = true
			}
			if containsDeclaredV2Token(style.references, declaredTokens) {
				result.tokenUsed = true
			}
		}
		locator, present, complete := locatorFromNode(node)
		if present {
			_, duplicate := seenLocators[locator.ID]
			switch {
			case !complete:
				result.diagnostics = append(result.diagnostics, errorDiagnostic("locator_incomplete", name, "locators require id, kind, and label"))
			case !validLocatorID(locator.ID):
				result.diagnostics = append(result.diagnostics, errorDiagnostic("locator_id_invalid", name, "locator ID is not stable"))
			case locator.Kind != "component" && locator.Kind != "block":
				result.diagnostics = append(result.diagnostics, errorDiagnostic("locator_kind_invalid", name, "locator kind must be component or block"))
			case duplicate:
				result.diagnostics = append(result.diagnostics, errorDiagnostic("locator_id_duplicate", name, "locator IDs must be package-unique"))
			default:
				seenLocators[locator.ID] = struct{}{}
				result.locators = append(result.locators, locator)
				if !hidden && v2NodeHasVisibleContent(node, false) {
					result.visible = true
				}
			}
		}
	}
	for child := node.FirstChild; child != nil; child = child.NextSibling {
		walkV2HTML(child, hidden, name, files, artifacts, declaredTokens, seenLocators, result)
	}
}

func forbiddenV2HTMLElement(tag string) bool {
	switch tag {
	case "animate", "animatemotion", "animatetransform", "base", "discard", "embed", "foreignobject",
		"form", "frame", "frameset", "iframe", "link", "object", "script", "set":
		return true
	default:
		return false
	}
}

func v2ElementHidden(node *html.Node) bool {
	for _, attribute := range node.Attr {
		key := strings.ToLower(attribute.Key)
		value := strings.ToLower(strings.TrimSpace(attribute.Val))
		if key == "hidden" || (key == "aria-hidden" && value == "true") {
			return true
		}
		if key == "style" && inspectV2CSS(value, true, "", nil, nil, nil, false).hidden {
			return true
		}
	}
	return false
}

func v2NodeHasVisibleContent(node *html.Node, hidden bool) bool {
	if node.Type == html.ElementNode {
		tag := strings.ToLower(node.Data)
		if tag == "head" || tag == "style" || tag == "template" || tag == "noscript" || tag == "script" {
			return false
		}
		hidden = hidden || v2ElementHidden(node)
		if hidden {
			return false
		}
		if tag == "img" {
			for _, attribute := range node.Attr {
				if strings.EqualFold(attribute.Key, "alt") && strings.TrimSpace(attribute.Val) != "" {
					return true
				}
			}
		}
	}
	if node.Type == html.TextNode && !hidden && strings.TrimSpace(node.Data) != "" {
		return true
	}
	for child := node.FirstChild; child != nil; child = child.NextSibling {
		if v2NodeHasVisibleContent(child, hidden) {
			return true
		}
	}
	return false
}

func inspectV2CSS(
	source string,
	inline bool,
	basePath string,
	files map[string][]byte,
	artifacts map[string]ArtifactIndexEntry,
	declaredTokens map[string]struct{},
	forbidTokenDeclarations bool,
) v2CSSAudit {
	result := v2CSSAudit{references: make(map[string]struct{})}
	parser := css.NewParser(parse.NewInputString(source), inline)
	atRuleDepth := 0
	documentRule := false
	for {
		grammar, _, data := parser.Next()
		if grammar == css.ErrorGrammar {
			if err := parser.Err(); err != nil && err != io.EOF {
				result.diagnostics = append(result.diagnostics, errorDiagnostic("css_invalid", basePath, "CSS is invalid"))
			}
			break
		}
		values := parser.Values()
		switch grammar {
		case css.BeginAtRuleGrammar:
			atRuleDepth++
		case css.EndAtRuleGrammar:
			if atRuleDepth > 0 {
				atRuleDepth--
			}
		case css.BeginRulesetGrammar:
			documentRule = atRuleDepth == 0 && v2SelectorTargetsDocumentRoot(values)
		case css.EndRulesetGrammar:
			documentRule = false
		}
		if grammar == css.AtRuleGrammar || grammar == css.BeginAtRuleGrammar {
			if strings.TrimPrefix(strings.ToLower(strings.TrimSpace(string(data))), "@") == "import" {
				result.diagnostics = append(result.diagnostics, errorDiagnostic("tokens_css_import_forbidden", basePath, "CSS @import is not allowed"))
			}
		}
		if grammar == css.CustomPropertyGrammar && forbidTokenDeclarations {
			result.diagnostics = append(result.diagnostics, errorDiagnostic("html_token_declaration_forbidden", basePath, "Preview HTML cannot declare a second Token source"))
		}
		if grammar == css.DeclarationGrammar {
			property := strings.ToLower(strings.TrimSpace(string(data)))
			value := strings.ToLower(cssTokenText(values))
			if (property == "display" && strings.TrimSpace(value) == "none") ||
				(property == "visibility" && strings.TrimSpace(value) == "hidden") {
				result.hidden = true
				if documentRule {
					result.documentHidden = true
				}
			}
		}
		inspectV2CSSTokens(values, basePath, files, artifacts, declaredTokens, &result)
	}
	return result
}

func v2SelectorTargetsDocumentRoot(tokens []css.Token) bool {
	start := 0
	for index, token := range tokens {
		if token.TokenType == css.CommaToken {
			if v2DocumentRootSelectorGroup(tokens[start:index]) {
				return true
			}
			start = index + 1
		}
	}
	return v2DocumentRootSelectorGroup(tokens[start:])
}

func v2DocumentRootSelectorGroup(tokens []css.Token) bool {
	filtered := make([]css.Token, 0, len(tokens))
	for _, token := range tokens {
		if token.TokenType != css.WhitespaceToken && token.TokenType != css.CommentToken {
			filtered = append(filtered, token)
		}
	}
	if len(filtered) == 1 {
		value := strings.ToLower(strings.TrimSpace(string(filtered[0].Data)))
		return (filtered[0].TokenType == css.IdentToken && (value == "html" || value == "body")) ||
			(filtered[0].TokenType == css.DelimToken && value == "*")
	}
	return len(filtered) == 2 && filtered[0].TokenType == css.ColonToken &&
		filtered[1].TokenType == css.IdentToken && strings.EqualFold(strings.TrimSpace(string(filtered[1].Data)), "root")
}

func inspectV2CSSTokens(
	tokens []css.Token,
	basePath string,
	files map[string][]byte,
	artifacts map[string]ArtifactIndexEntry,
	declaredTokens map[string]struct{},
	result *v2CSSAudit,
) {
	for index, token := range tokens {
		switch token.TokenType {
		case css.FunctionToken:
			if strings.EqualFold(strings.TrimSpace(string(token.Data)), "var(") {
				for next := index + 1; next < len(tokens); next++ {
					candidate := strings.TrimSpace(string(tokens[next].Data))
					if strings.HasPrefix(candidate, "--") {
						result.references[candidate] = struct{}{}
						if declaredTokens != nil {
							if _, exists := declaredTokens[candidate]; !exists {
								result.diagnostics = append(result.diagnostics, errorDiagnostic("token_reference_unknown", basePath, "CSS references an undeclared Token"))
							}
						}
						break
					}
				}
			}
		case css.URLToken:
			if !validV2LocalCSSResource(cssURLValue(string(token.Data)), basePath, artifacts) {
				result.diagnostics = append(result.diagnostics, errorDiagnostic("tokens_css_url_unsafe", basePath, "CSS resources must be package-local assets or fonts"))
			}
		case css.BadURLToken:
			result.diagnostics = append(result.diagnostics, errorDiagnostic("tokens_css_url_unsafe", basePath, "CSS contains a malformed URL"))
		case css.StringToken:
			value := strings.Trim(strings.TrimSpace(string(token.Data)), "\"'")
			parsed, err := url.Parse(value)
			if strings.HasPrefix(value, "//") || (err == nil && parsed.Scheme != "") {
				result.diagnostics = append(result.diagnostics, errorDiagnostic("tokens_css_url_unsafe", basePath, "CSS cannot contain remote URL strings"))
			}
		case css.CustomPropertyValueToken:
			inspectV2CSSValue(token.Data, basePath, artifacts, declaredTokens, result)
		}
	}
}

func inspectV2CSSValue(value []byte, basePath string, artifacts map[string]ArtifactIndexEntry, declaredTokens map[string]struct{}, result *v2CSSAudit) {
	lexer := css.NewLexer(parse.NewInputBytes(value))
	expectVariable := false
	for {
		tokenType, data := lexer.Next()
		if tokenType == css.ErrorToken {
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
					if _, exists := declaredTokens[candidate]; declaredTokens != nil && !exists {
						result.diagnostics = append(result.diagnostics, errorDiagnostic("token_reference_unknown", basePath, "CSS references an undeclared Token"))
					}
				}
			}
			expectVariable = false
		case css.URLToken:
			if !validV2LocalCSSResource(cssURLValue(string(data)), basePath, artifacts) {
				result.diagnostics = append(result.diagnostics, errorDiagnostic("tokens_css_url_unsafe", basePath, "CSS resources must be package-local assets or fonts"))
			}
			expectVariable = false
		case css.BadURLToken:
			result.diagnostics = append(result.diagnostics, errorDiagnostic("tokens_css_url_unsafe", basePath, "CSS contains a malformed URL"))
			expectVariable = false
		case css.StringToken:
			candidate := strings.Trim(strings.TrimSpace(string(data)), "\"'")
			parsed, err := url.Parse(candidate)
			if strings.HasPrefix(candidate, "//") || (err == nil && parsed.Scheme != "") {
				result.diagnostics = append(result.diagnostics, errorDiagnostic("tokens_css_url_unsafe", basePath, "CSS cannot contain remote URL strings"))
			}
			expectVariable = false
		case css.WhitespaceToken, css.CommentToken:
		default:
			expectVariable = false
		}
	}
}

func validV2LocalCSSResource(raw, basePath string, artifacts map[string]ArtifactIndexEntry) bool {
	resolved, ok := resolveV2LocalResource(raw, basePath)
	if !ok {
		return false
	}
	if strings.HasPrefix(resolved, "#") {
		return true
	}
	entry, exists := artifacts[resolved]
	return exists && (entry.Role == "asset" || entry.Role == "font")
}

func validV2LocalResource(raw, basePath, role string, artifacts map[string]ArtifactIndexEntry) bool {
	resolved, ok := resolveV2LocalResource(raw, basePath)
	if !ok || strings.HasPrefix(resolved, "#") {
		return false
	}
	entry, exists := artifacts[resolved]
	return exists && entry.Role == role
}

func resolveV2LocalResource(raw, basePath string) (string, bool) {
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

func containsDeclaredV2Token(references, declared map[string]struct{}) bool {
	for reference := range references {
		if _, exists := declared[reference]; exists {
			return true
		}
	}
	return false
}

func auditV2SVG(name string, raw []byte, files map[string][]byte, artifacts map[string]ArtifactIndexEntry, declaredTokens map[string]struct{}) []Diagnostic {
	decoder := xml.NewDecoder(bytes.NewReader(raw))
	diagnostics := make([]Diagnostic, 0)
	for {
		token, err := decoder.Token()
		if err == io.EOF {
			break
		}
		if err != nil {
			return append(diagnostics, errorDiagnostic("svg_invalid", name, "SVG asset is invalid XML"))
		}
		switch value := token.(type) {
		case xml.Directive, xml.ProcInst:
			diagnostics = append(diagnostics, errorDiagnostic("svg_unsafe", name, "SVG directives and processing instructions are not allowed"))
		case xml.StartElement:
			tag := strings.ToLower(value.Name.Local)
			switch tag {
			case "animate", "animatemotion", "animatetransform", "discard", "foreignobject", "iframe", "script", "set", "style":
				diagnostics = append(diagnostics, errorDiagnostic("svg_unsafe", name, "active SVG content is not allowed"))
			}
			for _, attribute := range value.Attr {
				key := strings.ToLower(attribute.Name.Local)
				if strings.HasPrefix(key, "on") {
					diagnostics = append(diagnostics, errorDiagnostic("svg_unsafe", name, "SVG event attributes are not allowed"))
				}
				if (key == "href" || key == "src") && !strings.HasPrefix(strings.TrimSpace(attribute.Value), "#") {
					diagnostics = append(diagnostics, errorDiagnostic("svg_unsafe", name, "SVG external resources are not allowed"))
				}
				if key == "style" {
					style := inspectV2CSS(attribute.Value, true, name, files, artifacts, declaredTokens, true)
					diagnostics = append(diagnostics, style.diagnostics...)
				} else if key != "href" && key != "src" {
					valueAudit := v2CSSAudit{references: make(map[string]struct{})}
					inspectV2CSSValue([]byte(attribute.Value), name, artifacts, declaredTokens, &valueAudit)
					diagnostics = append(diagnostics, valueAudit.diagnostics...)
				}
			}
		}
	}
	return diagnostics
}
