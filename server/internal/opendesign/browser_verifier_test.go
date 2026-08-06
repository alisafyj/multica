package opendesign

import (
	"fmt"
	"net/http"
	"net/http/httptest"
	"os"
	"strings"
	"sync/atomic"
	"testing"

	"github.com/multica-ai/multica/server/internal/designpreview"
)

func TestChromiumPreviewVerifierCapturesVisiblePageAndBlocksOutboundRequests(t *testing.T) {
	browserPath := installedTestBrowser(t)

	var outboundRequests atomic.Int64
	external := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		outboundRequests.Add(1)
		w.Header().Set("Content-Type", "image/png")
		w.WriteHeader(http.StatusNoContent)
	}))
	defer external.Close()

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "text/html; charset=utf-8")
		switch r.URL.Path {
		case "/good":
			fmt.Fprint(w, `<!doctype html><html><style>body{margin:0;background:#f7f8fa;color:#16181d;font:16px sans-serif}main{display:grid;grid-template-columns:repeat(3,1fr);gap:24px;padding:48px}.card{min-height:180px;padding:24px;background:#fff;border:1px solid #d6dae1}.accent{background:#1769e0;color:#fff}</style><body><main><section class="card accent">Primary</section><section class="card">Typography</section><section class="card">Components</section></main></body></html>`)
		case "/outbound":
			fmt.Fprintf(w, `<!doctype html><html><style>body{margin:0;background:#fff;color:#111;font:16px sans-serif}main{padding:48px;background:#e8f0fe}img{width:120px;height:120px}</style><body><main>Outbound asset<img src=%q></main></body></html>`, external.URL+"/asset.png")
		default:
			http.NotFound(w, r)
		}
	}))
	defer server.Close()

	verifier, err := NewChromiumPreviewVerifier(browserPath)
	if err != nil {
		t.Fatalf("NewChromiumPreviewVerifier: %v", err)
	}
	verification, err := verifier.Verify(t.Context(), []PreviewURL{
		{
			Target: PreviewTarget{Kind: PreviewTargetKindPreview, ID: "good", Path: "preview/good.html"},
			URL:    server.URL + "/good",
		},
		{
			Target: PreviewTarget{Kind: PreviewTargetKindPreview, ID: "outbound", Path: "preview/outbound.html"},
			URL:    server.URL + "/outbound",
		},
	})
	if err != nil {
		t.Fatalf("Verify: %v", err)
	}
	if verification.Passed || len(verification.Targets) != 2 || !verification.Targets[0].Passed || verification.Targets[1].Passed {
		t.Fatalf("verification = %+v", verification)
	}
	if verification.Targets[0].RenderedElementCount == 0 || verification.Targets[0].Screenshot.SHA256 == "" {
		t.Fatalf("visible target evidence = %+v", verification.Targets[0])
	}
	if verification.Targets[1].OutboundRequestCount == 0 {
		t.Fatalf("outbound target evidence = %+v", verification.Targets[1])
	}
	if verification.Targets[1].FailureCode != PreviewFailureOutboundRequest {
		t.Fatalf("outbound failure code = %q", verification.Targets[1].FailureCode)
	}
	if outboundRequests.Load() != 0 {
		t.Fatalf("blocked external server received %d request(s)", outboundRequests.Load())
	}
	_, err = verifier.Verify(t.Context(), []PreviewURL{{
		Target: PreviewTarget{Kind: PreviewTargetKindUIKit, ID: "ui-kit", Path: "ui-kit/index.html"},
		URL:    server.URL + "/good",
	}})
	if err == nil || !strings.Contains(err.Error(), "UI Kit target path") {
		t.Fatalf("native UI Kit target error = %v", err)
	}
}

func installedTestBrowser(t *testing.T) string {
	t.Helper()
	browserPath, err := designpreview.ResolveBrowserPath(os.Getenv("MULTICA_OPEN_DESIGN_BROWSER_PATH"))
	if err != nil {
		t.Skipf("no Chromium browser is installed: %v", err)
	}
	return browserPath
}
