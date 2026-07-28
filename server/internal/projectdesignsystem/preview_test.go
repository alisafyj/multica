package projectdesignsystem

import (
	"crypto/sha256"
	"encoding/base64"
	"strings"
	"testing"

	nethtml "golang.org/x/net/html"
)

func TestBuildPreviewHTMLAllowsOnlyTrustedSelectionBridge(t *testing.T) {
	pkg, err := Validate(validArtifactInput(t), []string{"static.soyoung.com"})
	if err != nil {
		t.Fatalf("Validate() error = %v", err)
	}

	preview := BuildPreviewHTML(pkg, []string{"static.soyoung.com"})
	if preview == "" {
		t.Fatal("BuildPreviewHTML() returned an empty preview")
	}
	csp := previewCSP(t, preview)
	if !strings.Contains(csp, "default-src 'none'") {
		t.Fatalf("preview CSP does not default-deny: %s", preview)
	}
	if !strings.Contains(csp, "object-src 'none'") || !strings.Contains(csp, "form-action 'none'") {
		t.Fatalf("preview CSP does not block objects and forms: %s", preview)
	}
	if !strings.Contains(csp, "https://static.soyoung.com") {
		t.Fatalf("preview CSP does not include the approved host: %s", preview)
	}
	if strings.Contains(preview, "allow-same-origin") || strings.Contains(preview, "allow-forms") {
		t.Fatalf("preview includes iframe sandbox capabilities: %s", preview)
	}

	sum := sha256.Sum256([]byte(selectionBridgeScript))
	wantHash := "'sha256-" + base64.StdEncoding.EncodeToString(sum[:]) + "'"
	if !strings.Contains(csp, "script-src "+wantHash) {
		t.Fatalf("preview CSP does not pin the trusted bridge hash %s", wantHash)
	}
	if strings.Count(preview, "<script") != 1 || !strings.Contains(preview, selectionBridgeScript) {
		t.Fatalf("preview must contain exactly the trusted bridge: %s", preview)
	}
	if !strings.Contains(preview, `parent.postMessage({type:"multica:project-design-system-select",id:node.dataset.designNodeId},"*")`) {
		t.Fatalf("selection bridge does not post the stable locator ID: %s", preview)
	}
	if strings.Contains(preview, "alert(") || strings.Contains(preview, "onclick=") {
		t.Fatalf("preview contains Agent-authored executable content: %s", preview)
	}
}

func TestBuildPreviewHTMLRevalidatesArtifacts(t *testing.T) {
	pkg, err := Validate(validArtifactInput(t), nil)
	if err != nil {
		t.Fatalf("Validate() error = %v", err)
	}
	pkg.Artifacts.TokensCSS += "</style><script>alert(1)</script>"

	if preview := BuildPreviewHTML(pkg, nil); preview != "" {
		t.Fatalf("BuildPreviewHTML() rendered mutated unsafe artifacts: %s", preview)
	}
}

func previewCSP(t *testing.T, source string) string {
	t.Helper()
	document, err := nethtml.Parse(strings.NewReader(source))
	if err != nil {
		t.Fatalf("parse preview: %v", err)
	}
	var content string
	var walk func(*nethtml.Node)
	walk = func(node *nethtml.Node) {
		if node.Type == nethtml.ElementNode && node.Data == "meta" {
			var httpEquiv string
			var candidate string
			for _, attribute := range node.Attr {
				switch attribute.Key {
				case "http-equiv":
					httpEquiv = attribute.Val
				case "content":
					candidate = attribute.Val
				}
			}
			if strings.EqualFold(httpEquiv, "Content-Security-Policy") {
				content = candidate
			}
		}
		for child := node.FirstChild; child != nil; child = child.NextSibling {
			walk(child)
		}
	}
	walk(document)
	if content == "" {
		t.Fatal("preview has no Content-Security-Policy meta element")
	}
	return content
}
