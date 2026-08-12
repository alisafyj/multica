package designpreview

import (
	"context"
	"fmt"
	"net/http"
	"net/http/httptest"
	"os/exec"
	"strings"
	"testing"
	"time"

	"github.com/chromedp/chromedp"
)

// Temporary CI-only diagnostic: reproduces the "second target hangs" pattern
// from the backend job and reports per-stage timings plus browser identity.
func TestDiagCIChromedpTwoTargets(t *testing.T) {
	browserPath, err := ResolveBrowserPath("")
	if err != nil {
		t.Skipf("no browser: %v", err)
	}
	verOut, _ := exec.Command(browserPath, "--version").CombinedOutput()
	t.Logf("browser=%s version=%q", browserPath, strings.TrimSpace(string(verOut)))

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "text/html")
		fmt.Fprint(w, `<!doctype html><html><style>body{margin:0;background:#fff;color:#111}main{padding:48px;background:#eee}</style><body><main>hello</main></body></html>`)
	}))
	defer server.Close()

	runTarget := func(t *testing.T, name string, opts []chromedp.ExecAllocatorOption) {
		t.Run(name, func(t *testing.T) {
			allocCtx, cancelAlloc := chromedp.NewExecAllocator(context.Background(), opts...)
			defer cancelAlloc()
			browserCtx, cancelBrowser := chromedp.NewContext(allocCtx)
			defer cancelBrowser()

			start := time.Now()
			if err := chromedp.Run(browserCtx); err != nil {
				t.Fatalf("start browser: %v", err)
			}
			t.Logf("start browser ok after %v", time.Since(start))

			for i := 1; i <= 2; i++ {
				targetCtx, cancelTarget := chromedp.NewContext(browserCtx)
				targetCtx, cancelTimeout := context.WithTimeout(targetCtx, 20*time.Second)
				stage := func(label string, fn func() error) {
					s := time.Now()
					if err := fn(); err != nil {
						t.Fatalf("target%d %s: %v (after %v)", i, label, err, time.Since(s))
					}
					t.Logf("target%d %s ok after %v", i, label, time.Since(s))
				}
				stage("navigate", func() error { return chromedp.Run(targetCtx, chromedp.Navigate(server.URL)) })
				stage("sleep", func() error { return chromedp.Run(targetCtx, chromedp.Sleep(750*time.Millisecond)) })
				var dom map[string]any
				var shot []byte
				stage("evaluate+screenshot", func() error {
					return chromedp.Run(targetCtx,
						chromedp.Evaluate(`(() => { const m = document.querySelector('main'); return {text: m ? m.textContent : '', count: document.querySelectorAll('*').length}; })()`, &dom),
						chromedp.CaptureScreenshot(&shot),
					)
				})
				t.Logf("target%d evaluate=%v screenshot=%d bytes", i, dom, len(shot))
				cancelTimeout()
				cancelTarget()
			}
		})
	}

	// Baseline: chromedp default options (same as production code path).
	runTarget(t, "default", append(chromedp.DefaultExecAllocatorOptions[:],
		chromedp.ExecPath(browserPath),
	))

	// Variant A: add --no-sandbox (root-equivalent CI sandbox workaround).
	runTarget(t, "no-sandbox", append(chromedp.DefaultExecAllocatorOptions[:],
		chromedp.ExecPath(browserPath),
		chromedp.Flag("no-sandbox", true),
	))
}
