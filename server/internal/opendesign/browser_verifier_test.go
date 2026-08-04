package opendesign

import (
	"bytes"
	"fmt"
	"image"
	"image/color"
	"image/png"
	"net/http"
	"net/http/httptest"
	"os"
	"os/exec"
	"runtime"
	"sync/atomic"
	"testing"
)

func TestAnalyzePreviewScreenshotMeasuresVisualVariation(t *testing.T) {
	t.Parallel()

	imageData := image.NewNRGBA(image.Rect(0, 0, 64, 64))
	for y := range 64 {
		for x := range 64 {
			imageData.SetNRGBA(x, y, color.NRGBA{
				R: uint8(x * 4),
				G: uint8(y * 4),
				B: uint8((x + y) * 2),
				A: 255,
			})
		}
	}
	var encoded bytes.Buffer
	if err := png.Encode(&encoded, imageData); err != nil {
		t.Fatalf("encode screenshot: %v", err)
	}

	metrics, err := analyzePreviewScreenshot(encoded.Bytes())
	if err != nil {
		t.Fatalf("analyzePreviewScreenshot: %v", err)
	}
	if metrics.Width != 64 || metrics.Height != 64 || metrics.Bytes != encoded.Len() ||
		metrics.SHA256 == "" || metrics.Entropy <= 0.1 || metrics.MaxChannelStddev <= 1 {
		t.Fatalf("screenshot metrics = %+v", metrics)
	}
}

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
}

func installedTestBrowser(t *testing.T) string {
	t.Helper()
	if configured := os.Getenv("MULTICA_OPEN_DESIGN_BROWSER_PATH"); configured != "" {
		return configured
	}
	candidates := []string{
		"/Applications/Google Chrome.app/Contents/MacOS/Google Chrome",
		"/usr/bin/google-chrome",
		"/usr/bin/google-chrome-stable",
		"/usr/bin/chromium",
		"/usr/bin/chromium-browser",
	}
	for _, candidate := range candidates {
		if info, err := os.Stat(candidate); err == nil && !info.IsDir() {
			return candidate
		}
	}
	for _, name := range []string{"google-chrome", "chromium", "chromium-browser"} {
		if candidate, err := exec.LookPath(name); err == nil {
			return candidate
		}
	}
	t.Skipf("no Chromium browser is installed on %s", runtime.GOOS)
	return ""
}
