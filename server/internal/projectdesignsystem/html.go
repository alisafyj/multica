package projectdesignsystem

import (
	"bytes"
	"io"
	"strings"

	parse "github.com/tdewolff/parse/v2"
	"github.com/tdewolff/parse/v2/css"
	"golang.org/x/net/html"
	"golang.org/x/net/html/atom"
)

var forbiddenHTMLElements = map[string]struct{}{
	"base": {}, "embed": {}, "form": {}, "frame": {}, "frameset": {},
	"iframe": {}, "link": {}, "meta": {}, "object": {}, "script": {},
}

type htmlParseResult struct {
	locators    []Locator
	normalized  string
	diagnostics []Diagnostic
}

func parseComponentsHTML(source string, declaredTokens map[string]struct{}, allowedHosts map[string]struct{}) htmlParseResult {
	result := htmlParseResult{}
	context := &html.Node{Type: html.ElementNode, DataAtom: atom.Div, Data: "div"}
	nodes, err := html.ParseFragment(strings.NewReader(source), context)
	if err != nil {
		result.diagnostics = append(result.diagnostics, errorDiagnostic(
			"components_html_invalid",
			"components.html",
			"components.html is not valid HTML: "+err.Error(),
		))
		return result
	}

	seen := make(map[string]struct{})
	visibleLocator := false
	for _, node := range nodes {
		walkHTML(node, false, declaredTokens, allowedHosts, seen, &visibleLocator, &result)
	}
	if len(result.locators) == 0 {
		result.diagnostics = append(result.diagnostics, errorDiagnostic(
			"locator_missing",
			"components.html",
			"components.html must contain at least one selectable component or block",
		))
	}
	if !visibleLocator {
		result.diagnostics = append(result.diagnostics, errorDiagnostic(
			"ui_kit_not_visible",
			"components.html",
			"components.html must contain visible content inside a selectable component or block",
		))
	}

	var rendered bytes.Buffer
	for _, node := range nodes {
		if err := html.Render(&rendered, node); err != nil {
			result.diagnostics = append(result.diagnostics, errorDiagnostic(
				"components_html_invalid",
				"components.html",
				"components.html could not be normalized: "+err.Error(),
			))
			return result
		}
	}
	result.normalized = rendered.String()
	return result
}

func walkHTML(
	node *html.Node,
	hidden bool,
	declaredTokens map[string]struct{},
	allowedHosts map[string]struct{},
	seen map[string]struct{},
	visibleLocator *bool,
	result *htmlParseResult,
) {
	if node.Type == html.ElementNode {
		tag := strings.ToLower(node.Data)
		if _, forbidden := forbiddenHTMLElements[tag]; forbidden {
			result.diagnostics = append(result.diagnostics, errorDiagnostic(
				"html_forbidden_element",
				"components.html",
				"Element <"+tag+"> is not allowed in the UI Kit",
			))
		}
		hidden = hidden || elementIsHidden(node)
		validateHTMLAttributes(node, tag, declaredTokens, allowedHosts, result)
		if locator, present, complete := locatorFromNode(node); present {
			if !complete {
				result.diagnostics = append(result.diagnostics, errorDiagnostic(
					"locator_incomplete",
					"components.html",
					"A locator must include id, kind, and label",
				))
			} else {
				validateLocator(locator, node, hidden, seen, visibleLocator, result)
			}
		}
		if tag == "style" {
			validateEmbeddedStyles(node, declaredTokens, allowedHosts, result)
		}
	}

	for child := node.FirstChild; child != nil; child = child.NextSibling {
		walkHTML(child, hidden, declaredTokens, allowedHosts, seen, visibleLocator, result)
	}
}

func validateLocator(locator Locator, node *html.Node, hidden bool, seen map[string]struct{}, visibleLocator *bool, result *htmlParseResult) {
	switch {
	case !validLocatorID(locator.ID):
		result.diagnostics = append(result.diagnostics, errorDiagnostic(
			"locator_id_invalid",
			"components.html",
			"Locator ID "+locator.ID+" is not stable",
		))
	case locator.Kind != "component" && locator.Kind != "block":
		result.diagnostics = append(result.diagnostics, errorDiagnostic(
			"locator_kind_invalid",
			"components.html",
			"Locator kind must be component or block",
		))
	default:
		if _, exists := seen[locator.ID]; exists {
			result.diagnostics = append(result.diagnostics, errorDiagnostic(
				"locator_id_duplicate",
				"components.html",
				"Locator ID "+locator.ID+" is duplicated",
			))
			return
		}
		seen[locator.ID] = struct{}{}
		result.locators = append(result.locators, locator)
		if !hidden && nodeHasVisibleContent(node) {
			*visibleLocator = true
		}
	}
}

func validateHTMLAttributes(node *html.Node, tag string, declaredTokens map[string]struct{}, allowedHosts map[string]struct{}, result *htmlParseResult) {
	for _, attribute := range node.Attr {
		key := strings.ToLower(attribute.Key)
		value := strings.TrimSpace(attribute.Val)
		switch {
		case strings.HasPrefix(key, "on"):
			result.diagnostics = append(result.diagnostics, errorDiagnostic(
				"html_event_handler",
				"components.html",
				"Event handler attribute "+key+" is not allowed",
			))
		case key == "srcdoc" || key == "srcset":
			result.diagnostics = append(result.diagnostics, errorDiagnostic(
				"html_forbidden_attribute",
				"components.html",
				"Attribute "+key+" is not allowed",
			))
		case key == "href" || key == "xlink:href":
			if value != "" && !strings.HasPrefix(value, "#") {
				result.diagnostics = append(result.diagnostics, errorDiagnostic(
					"html_url_unsafe",
					"components.html",
					"Links in the UI Kit must use fragment-only targets",
				))
			}
		case key == "src":
			if tag != "img" || !isAllowedResourceURL(value, allowedHosts) {
				result.diagnostics = append(result.diagnostics, errorDiagnostic(
					"html_url_unsafe",
					"components.html",
					"Resource URL is not allowed",
				))
			}
		case key == "action" || key == "formaction" || key == "poster" || key == "data" || key == "background":
			if value != "" {
				result.diagnostics = append(result.diagnostics, errorDiagnostic(
					"html_url_unsafe",
					"components.html",
					"Navigation and embedded resource attributes are not allowed",
				))
			}
		case key == "style":
			validateInlineStyle(value, declaredTokens, allowedHosts, result)
		}
	}
}

func validateInlineStyle(source string, declaredTokens map[string]struct{}, allowedHosts map[string]struct{}, result *htmlParseResult) {
	parser := css.NewParser(parse.NewInputString(source), true)
	inspect := tokenParseResult{declared: declaredTokens, references: make(map[string]struct{})}
	for {
		grammar, _, _ := parser.Next()
		if grammar == css.ErrorGrammar {
			if err := parser.Err(); err != nil && err != io.EOF {
				result.diagnostics = append(result.diagnostics, errorDiagnostic(
					"html_style_invalid",
					"components.html",
					"Inline style is not valid CSS",
				))
			}
			break
		}
		inspectCSSTokens(parser.Values(), allowedHosts, "components.html", &inspect)
	}
	appendUnknownHTMLTokenReferences(inspect, declaredTokens, result)
	result.diagnostics = append(result.diagnostics, inspect.diagnostics...)
}

func validateEmbeddedStyles(node *html.Node, declaredTokens map[string]struct{}, allowedHosts map[string]struct{}, result *htmlParseResult) {
	var source strings.Builder
	for child := node.FirstChild; child != nil; child = child.NextSibling {
		if child.Type == html.TextNode {
			source.WriteString(child.Data)
		}
	}
	parser := css.NewParser(parse.NewInputString(source.String()), false)
	inspect := tokenParseResult{declared: declaredTokens, references: make(map[string]struct{})}
	for {
		grammar, _, data := parser.Next()
		if grammar == css.ErrorGrammar {
			if err := parser.Err(); err != nil && err != io.EOF {
				result.diagnostics = append(result.diagnostics, errorDiagnostic(
					"html_style_invalid",
					"components.html",
					"Embedded style is not valid CSS",
				))
			}
			break
		}
		if grammar == css.AtRuleGrammar || grammar == css.BeginAtRuleGrammar {
			name := strings.TrimPrefix(strings.ToLower(strings.TrimSpace(string(data))), "@")
			if name == "import" {
				result.diagnostics = append(result.diagnostics, errorDiagnostic(
					"tokens_css_import_forbidden",
					"components.html",
					"CSS imports are not allowed",
				))
			}
		}
		if grammar == css.CustomPropertyGrammar {
			result.diagnostics = append(result.diagnostics, errorDiagnostic(
				"html_token_declaration_forbidden",
				"components.html",
				"UI Kit HTML cannot declare a second Token source",
			))
		}
		inspectCSSTokens(parser.Values(), allowedHosts, "components.html", &inspect)
	}
	appendUnknownHTMLTokenReferences(inspect, declaredTokens, result)
	result.diagnostics = append(result.diagnostics, inspect.diagnostics...)
}

func appendUnknownHTMLTokenReferences(inspect tokenParseResult, declaredTokens map[string]struct{}, result *htmlParseResult) {
	for reference := range inspect.references {
		if _, exists := declaredTokens[reference]; !exists {
			result.diagnostics = append(result.diagnostics, errorDiagnostic(
				"token_reference_unknown",
				"components.html",
				"Token reference "+reference+" is not declared in tokens.css",
			))
		}
	}
}

func locatorFromNode(node *html.Node) (Locator, bool, bool) {
	var locator Locator
	fields := 0
	for _, attribute := range node.Attr {
		switch strings.ToLower(attribute.Key) {
		case "data-design-node-id":
			locator.ID = strings.TrimSpace(attribute.Val)
			fields++
		case "data-design-node-kind":
			locator.Kind = strings.TrimSpace(attribute.Val)
			fields++
		case "data-design-node-label":
			locator.Label = strings.TrimSpace(attribute.Val)
			fields++
		}
	}
	present := fields > 0
	complete := fields == 3 && locator.ID != "" && locator.Kind != "" && locator.Label != ""
	return locator, present, complete
}

func elementIsHidden(node *html.Node) bool {
	for _, attribute := range node.Attr {
		key := strings.ToLower(attribute.Key)
		value := strings.ToLower(strings.TrimSpace(attribute.Val))
		if key == "hidden" || (key == "aria-hidden" && value == "true") {
			return true
		}
		if key == "style" &&
			(strings.Contains(value, "display:none") ||
				strings.Contains(value, "display: none") ||
				strings.Contains(value, "visibility:hidden") ||
				strings.Contains(value, "visibility: hidden")) {
			return true
		}
	}
	return false
}

func nodeHasVisibleContent(node *html.Node) bool {
	if node.Type == html.ElementNode && strings.EqualFold(node.Data, "img") {
		for _, attribute := range node.Attr {
			if strings.EqualFold(attribute.Key, "alt") && strings.TrimSpace(attribute.Val) != "" {
				return true
			}
		}
	}
	for child := node.FirstChild; child != nil; child = child.NextSibling {
		if child.Type == html.TextNode && strings.TrimSpace(child.Data) != "" {
			return true
		}
		if child.Type == html.ElementNode {
			tag := strings.ToLower(child.Data)
			if tag != "style" && tag != "template" && tag != "noscript" && !elementIsHidden(child) && nodeHasVisibleContent(child) {
				return true
			}
		}
	}
	return false
}
