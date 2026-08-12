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

	"github.com/chromedp/cdproto/page"
	"github.com/chromedp/chromedp"
)

// Temporary CI-only diagnostic: verifies candidate fixes for the
// "second captureScreenshot hangs" bug (Chrome 150 headless waits on a frame
// that never arrives).
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

	eachTarget := func(t *testing.T, name string, opts []chromedp.ExecAllocatorOption, extraActions func() []chromedp.Action) {
		t.Run(name, func(t *testing.T) {
			allocCtx, cancelAlloc := chromedp.NewExecAllocator(context.Background(), opts...)
			defer cancelAlloc()
			browserCtx, cancelBrowser := chromedp.NewContext(allocCtx)
			defer cancelBrowser()
			if err := chromedp.Run(browserCtx); err != nil {
				t.Fatalf("start browser: %v", err)
			}
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
				stage("evaluate", func() error {
					return chromedp.Run(targetCtx, chromedp.Evaluate(`(() => { const m = document.querySelector('main'); return {text: m ? m.textContent : '', count: document.querySelectorAll('*').length}; })()`, &dom))
				})
				for _, act := range extraActions() {
					stage("extra", func() error { return chromedp.Run(targetCtx, act) })
				}
				var shot []byte
				stage("screenshot", func() error { return chromedp.Run(targetCtx, chromedp.CaptureScreenshot(&shot)) })
				t.Logf("target%d evaluate=%v screenshot=%d bytes", i, dom, len(shot))
				cancelTimeout()
				cancelTarget()
			}
		})
	}

	bringToFront := func() []chromedp.Action {
		return []chromedp.Action{chromedp.ActionFunc(func(ctx context.Context) error {
			return page.BringToFront().Do(ctx)
		})}
	}
	forcePaint := func() []chromedp.Action {
		// Force a new compositor frame before capture.
		return []chromedp.Action{chromedp.Evaluate(`document.body.style.background='#fefefe'; document.body.offsetHeight; document.body.style.background='#ffffff';`, nil)}
	}

	defaultOpts := append(chromedp.DefaultExecAllocatorOptions[:], chromedp.ExecPath(browserPath))
	eachTarget(t, "baseline", defaultOpts, func() []chromedp.Action { return nil })
	eachTarget(t, "bring-to-front", defaultOpts, bringToFront)
	eachTarget(t, "force-paint", defaultOpts, forcePaint)

	// Minimal options without chromedp's disable-features override.
	minimalOpts := []chromedp.ExecAllocatorOption{
		chromedp.ExecPath(browserPath),
		chromedp.Flag("headless", true),
		chromedp.Flag("disable-gpu", true),
		chromedp.Flag("no-first-run", true),
		chromedp.Flag("no-default-browser-check", true),
		chromedp.Flag("disable-dev-shm-usage", true),
	}
	eachTarget(t, "no-disable-features", minimalOpts, bringToFront)
}
