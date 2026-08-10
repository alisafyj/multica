package handler

import (
	"strings"
	"testing"
)

func TestNativePackagePreviewCSPTrustsOnlyTheBridge(t *testing.T) {
	csp := nativePackagePreviewCSP()
	if !strings.Contains(csp, "script-src 'sha256-") {
		t.Fatalf("CSP does not pin the preview bridge: %q", csp)
	}
	for _, forbidden := range []string{"'unsafe-inline'", "connect-src 'self'", "object-src 'self'"} {
		if strings.Contains(csp, forbidden) {
			t.Fatalf("CSP includes forbidden directive %q: %q", forbidden, csp)
		}
	}
}

func TestInjectNativePackagePreviewBridgeKeepsTrustedAssetsInDocument(t *testing.T) {
	html := injectNativePackagePreviewBridge([]byte("<html><head></head><body><main>UI Kit</main></body></html>"))
	value := string(html)
	if !strings.Contains(value, `<link rel="stylesheet" href="tokens.css">`) || !strings.Contains(value, nativePackagePreviewBridgeScript) {
		t.Fatalf("injected preview = %q", value)
	}
	if strings.Index(value, "tokens.css") > strings.Index(value, "</head>") || strings.Index(value, nativePackagePreviewBridgeScript) > strings.Index(value, "</body>") {
		t.Fatalf("trusted preview assets were injected outside their document locations: %q", value)
	}
}
