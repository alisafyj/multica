package projectdesignsystem

import (
	"crypto/sha256"
	"encoding/base64"
	"html"
	"strings"
)

const selectionBridgeScript = "(()=>{document.addEventListener(\"click\",event=>{const target=event.target;const node=target instanceof Element?target.closest(\"[data-design-node-id]\"):null;if(!node)return;event.preventDefault();parent.postMessage({type:\"multica:project-design-system-select\",id:node.dataset.designNodeId},\"*\")})})();"

func BuildPreviewHTML(pkg ValidatedPackage, allowedHosts []string) string {
	if !pkg.Validation.Passed {
		return ""
	}
	validated, err := Validate(pkg.Artifacts, allowedHosts)
	if err != nil {
		return ""
	}
	pkg = validated
	hosts := normalizeAllowedHosts(allowedHosts)
	components := parseComponentsHTML(pkg.Artifacts.ComponentsHTML, declaredTokenNames(pkg.Manifest.TokenGroups), hosts)
	if hasErrors(components.diagnostics) {
		return ""
	}

	hash := sha256.Sum256([]byte(selectionBridgeScript))
	scriptSource := "'sha256-" + base64.StdEncoding.EncodeToString(hash[:]) + "'"
	hostSources := make([]string, 0, len(hosts))
	for _, host := range sortedAllowedHosts(hosts) {
		hostSources = append(hostSources, "https://"+host)
	}
	imgSources := append([]string{"data:"}, hostSources...)
	fontSources := hostSources
	if len(fontSources) == 0 {
		fontSources = []string{"'none'"}
	}
	csp := strings.Join([]string{
		"default-src 'none'",
		"script-src " + scriptSource,
		"style-src 'unsafe-inline'",
		"img-src " + strings.Join(imgSources, " "),
		"font-src " + strings.Join(fontSources, " "),
		"connect-src 'none'",
		"frame-src 'none'",
		"child-src 'none'",
		"object-src 'none'",
		"base-uri 'none'",
		"form-action 'none'",
		"navigate-to 'none'",
	}, "; ")

	var document strings.Builder
	document.WriteString("<!doctype html><html><head><meta charset=\"utf-8\">")
	document.WriteString("<meta http-equiv=\"Content-Security-Policy\" content=\"")
	document.WriteString(html.EscapeString(csp))
	document.WriteString("\"><meta name=\"viewport\" content=\"width=device-width,initial-scale=1\">")
	document.WriteString("<style>")
	document.WriteString(pkg.Artifacts.TokensCSS)
	document.WriteString("</style></head><body>")
	document.WriteString(components.normalized)
	document.WriteString("<script>")
	document.WriteString(selectionBridgeScript)
	document.WriteString("</script></body></html>")
	return document.String()
}

func declaredTokenNames(groups []TokenGroup) map[string]struct{} {
	declared := make(map[string]struct{})
	for _, group := range groups {
		for _, token := range group.Tokens {
			declared[token.Name] = struct{}{}
		}
	}
	return declared
}
