package opendesign

import (
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"fmt"
	"image/color"
	"image/png"
	"math"
	"net"
	"net/url"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"time"

	"github.com/chromedp/cdproto/browser"
	"github.com/chromedp/cdproto/cdp"
	"github.com/chromedp/cdproto/fetch"
	"github.com/chromedp/cdproto/network"
	cdpruntime "github.com/chromedp/cdproto/runtime"
	"github.com/chromedp/chromedp"
)

const (
	previewBrowserTargetTimeout = 25 * time.Second
	previewBrowserSettleDelay   = 750 * time.Millisecond
	previewScreenshotMaxSamples = 1_000_000
)

const previewDOMMetricsExpression = `(() => {
  const body = document.body;
  const root = document.documentElement;
  if (!body || !root) {
    return { documentLoaded: false, domPresent: false, computedVisibilityVisible: false, renderedElementCount: 0, visibleTextLength: 0, bodyWidth: 0, bodyHeight: 0, imageCount: 0, failedImageCount: 0 };
  }
  const bodyStyle = getComputedStyle(body);
  const opacity = Number.parseFloat(bodyStyle.opacity || '1');
  const visible = bodyStyle.display !== 'none' && bodyStyle.visibility !== 'hidden' && opacity > 0;
  const elements = Array.from(body.querySelectorAll('*'));
  let renderedElementCount = 0;
  for (const element of elements) {
    const style = getComputedStyle(element);
    const rect = element.getBoundingClientRect();
    const elementOpacity = Number.parseFloat(style.opacity || '1');
    if (style.display !== 'none' && style.visibility !== 'hidden' && elementOpacity > 0 && rect.width > 0.5 && rect.height > 0.5) {
      renderedElementCount += 1;
    }
  }
  const images = Array.from(body.querySelectorAll('img'));
  return {
    documentLoaded: document.readyState === 'interactive' || document.readyState === 'complete',
    domPresent: body.childNodes.length > 0,
    computedVisibilityVisible: visible,
    renderedElementCount,
    visibleTextLength: (body.innerText || '').trim().length,
    bodyWidth: Math.ceil(Math.max(body.scrollWidth, body.offsetWidth, root.clientWidth, root.scrollWidth, root.offsetWidth)),
    bodyHeight: Math.ceil(Math.max(body.scrollHeight, body.offsetHeight, root.clientHeight, root.scrollHeight, root.offsetHeight)),
    imageCount: images.length,
    failedImageCount: images.filter((image) => !image.complete || image.naturalWidth <= 0 || image.naturalHeight <= 0).length,
  };
})()`

type ChromiumPreviewVerifier struct {
	browserPath string
}

type previewDOMMetrics struct {
	DocumentLoaded            bool `json:"documentLoaded"`
	DOMPresent                bool `json:"domPresent"`
	ComputedVisibilityVisible bool `json:"computedVisibilityVisible"`
	RenderedElementCount      int  `json:"renderedElementCount"`
	VisibleTextLength         int  `json:"visibleTextLength"`
	BodyWidth                 int  `json:"bodyWidth"`
	BodyHeight                int  `json:"bodyHeight"`
	ImageCount                int  `json:"imageCount"`
	FailedImageCount          int  `json:"failedImageCount"`
}

type previewNetworkEvidence struct {
	mu               sync.Mutex
	allowedOrigin    string
	failedRequests   map[network.RequestID]struct{}
	outboundRequests map[network.RequestID]struct{}
	requestURLs      map[network.RequestID]string
	consoleErrors    int
	interceptionErr  error
}

func NewChromiumPreviewVerifier(rawBrowserPath string) (*ChromiumPreviewVerifier, error) {
	browserPath, err := filepath.Abs(strings.TrimSpace(rawBrowserPath))
	if err != nil {
		return nil, fmt.Errorf("resolve Open Design Preview browser path: %w", err)
	}
	info, err := os.Lstat(browserPath)
	if err != nil {
		return nil, fmt.Errorf("inspect Open Design Preview browser: %w", err)
	}
	if info.Mode()&os.ModeSymlink != 0 || !info.Mode().IsRegular() || info.Mode().Perm()&0o111 == 0 {
		return nil, errors.New("Open Design Preview browser must be an executable regular file")
	}
	return &ChromiumPreviewVerifier{browserPath: browserPath}, nil
}

func (v *ChromiumPreviewVerifier) Verify(ctx context.Context, targets []PreviewURL) (PreviewVerification, error) {
	allowedOrigin, err := validateBrowserPreviewTargets(targets)
	if err != nil {
		return PreviewVerification{}, err
	}
	profileDir, err := os.MkdirTemp("", "multica-open-design-preview-")
	if err != nil {
		return PreviewVerification{}, fmt.Errorf("create isolated Open Design Preview browser profile: %w", err)
	}
	defer os.RemoveAll(profileDir)

	policy := PinnedPreviewVerificationPolicy()
	opts := append(chromedp.DefaultExecAllocatorOptions[:],
		chromedp.ExecPath(v.browserPath),
		chromedp.UserDataDir(profileDir),
		chromedp.WindowSize(policy.ViewportWidth, policy.ViewportHeight),
	)
	allocatorCtx, cancelAllocator := chromedp.NewExecAllocator(ctx, opts...)
	defer cancelAllocator()
	browserCtx, cancelBrowser := chromedp.NewContext(allocatorCtx)
	defer cancelBrowser()
	if err := chromedp.Run(browserCtx); err != nil {
		return PreviewVerification{}, fmt.Errorf("start isolated Open Design Preview browser: %w", err)
	}
	browserIdentity, err := readPreviewBrowserIdentity(browserCtx)
	if err != nil {
		return PreviewVerification{}, err
	}

	verification := PreviewVerification{
		Browser: browserIdentity,
		Policy:  policy,
		Targets: make([]PreviewTargetVerification, 0, len(targets)),
		Passed:  true,
	}
	for _, target := range targets {
		capture, err := v.captureTarget(browserCtx, allowedOrigin, target, policy)
		if err != nil {
			return PreviewVerification{}, err
		}
		result := EvaluatePreviewCapture(capture, policy)
		verification.Targets = append(verification.Targets, result)
		verification.Passed = verification.Passed && result.Passed
	}
	return verification, nil
}

func (v *ChromiumPreviewVerifier) captureTarget(parent context.Context, allowedOrigin string, target PreviewURL, policy PreviewVerificationPolicy) (PreviewCapture, error) {
	targetCtx, cancelTarget := chromedp.NewContext(parent)
	defer cancelTarget()
	targetCtx, cancelTimeout := context.WithTimeout(targetCtx, previewBrowserTargetTimeout)
	defer cancelTimeout()

	evidence := &previewNetworkEvidence{
		allowedOrigin:    allowedOrigin,
		failedRequests:   make(map[network.RequestID]struct{}),
		outboundRequests: make(map[network.RequestID]struct{}),
		requestURLs:      make(map[network.RequestID]string),
	}
	evidence.listen(targetCtx)
	if err := chromedp.Run(targetCtx,
		network.Enable(),
		network.SetCacheDisabled(true),
		network.SetBypassServiceWorker(true),
		fetch.Enable(),
		chromedp.EmulateViewport(int64(policy.ViewportWidth), int64(policy.ViewportHeight)),
	); err != nil {
		return PreviewCapture{}, fmt.Errorf("prepare Open Design Preview browser target: %w", err)
	}

	capture := PreviewCapture{Target: target.Target}
	if err := chromedp.Run(targetCtx, chromedp.Navigate(target.URL)); err != nil {
		if interceptionErr := evidence.interceptionError(); interceptionErr != nil {
			return PreviewCapture{}, fmt.Errorf("intercept Open Design Preview requests for %q: %w", target.Target.Path, interceptionErr)
		}
		capture.FailedResourceCount, capture.ConsoleErrorCount, capture.OutboundRequestCount = evidence.counts()
		return capture, nil
	}
	if err := chromedp.Run(targetCtx, chromedp.Sleep(previewBrowserSettleDelay)); err != nil {
		capture.FailedResourceCount, capture.ConsoleErrorCount, capture.OutboundRequestCount = evidence.counts()
		return capture, nil
	}

	var dom previewDOMMetrics
	var screenshot []byte
	if err := chromedp.Run(targetCtx,
		chromedp.Evaluate(previewDOMMetricsExpression, &dom),
		chromedp.CaptureScreenshot(&screenshot),
	); err != nil {
		return PreviewCapture{}, fmt.Errorf("capture Open Design Preview evidence for %q: %w", target.Target.Path, err)
	}
	screenshotMetrics, err := analyzePreviewScreenshot(screenshot)
	if err != nil {
		return PreviewCapture{}, fmt.Errorf("analyze Open Design Preview screenshot for %q: %w", target.Target.Path, err)
	}
	failedResources, consoleErrors, outboundRequests := evidence.counts()
	return PreviewCapture{
		Target:                    target.Target,
		DocumentLoaded:            dom.DocumentLoaded,
		DOMPresent:                dom.DOMPresent,
		ComputedVisibilityVisible: dom.ComputedVisibilityVisible,
		RenderedElementCount:      dom.RenderedElementCount,
		VisibleTextLength:         dom.VisibleTextLength,
		BodyWidth:                 dom.BodyWidth,
		BodyHeight:                dom.BodyHeight,
		ImageCount:                dom.ImageCount,
		FailedImageCount:          dom.FailedImageCount,
		FailedResourceCount:       failedResources,
		ConsoleErrorCount:         consoleErrors,
		OutboundRequestCount:      outboundRequests,
		Screenshot:                screenshotMetrics,
	}, nil
}

func (e *previewNetworkEvidence) listen(ctx context.Context) {
	chromedp.ListenTarget(ctx, func(event any) {
		switch event := event.(type) {
		case *fetch.EventRequestPaused:
			allowed := samePreviewOrigin(event.Request.URL, e.allowedOrigin)
			requestID := event.RequestID
			go func() {
				chromedpContext := chromedp.FromContext(ctx)
				if chromedpContext == nil || chromedpContext.Target == nil {
					e.recordInterceptionError(errors.New("Preview target executor is unavailable"))
					return
				}
				executorCtx := cdp.WithExecutor(ctx, chromedpContext.Target)
				var err error
				if allowed {
					err = fetch.ContinueRequest(requestID).Do(executorCtx)
				} else {
					err = fetch.FailRequest(requestID, network.ErrorReasonBlockedByClient).Do(executorCtx)
				}
				if err != nil && !errors.Is(err, context.Canceled) {
					e.recordInterceptionError(err)
				}
			}()
		case *network.EventRequestWillBeSent:
			e.mu.Lock()
			e.requestURLs[event.RequestID] = event.Request.URL
			if !samePreviewOrigin(event.Request.URL, e.allowedOrigin) {
				e.outboundRequests[event.RequestID] = struct{}{}
			}
			e.mu.Unlock()
		case *network.EventResponseReceived:
			if event.Response.Status >= 400 && !ignorablePreviewBrowserResource(event.Response.URL) {
				e.recordFailedRequest(event.RequestID)
			}
		case *network.EventLoadingFailed:
			e.mu.Lock()
			requestURL := e.requestURLs[event.RequestID]
			if _, outbound := e.outboundRequests[event.RequestID]; !outbound && !ignorablePreviewBrowserResource(requestURL) {
				e.failedRequests[event.RequestID] = struct{}{}
			}
			e.mu.Unlock()
		case *cdpruntime.EventConsoleAPICalled:
			if event.Type == cdpruntime.APITypeError {
				e.mu.Lock()
				e.consoleErrors++
				e.mu.Unlock()
			}
		case *cdpruntime.EventExceptionThrown:
			e.mu.Lock()
			e.consoleErrors++
			e.mu.Unlock()
		}
	})
}

func (e *previewNetworkEvidence) recordInterceptionError(err error) {
	e.mu.Lock()
	defer e.mu.Unlock()
	if e.interceptionErr == nil {
		e.interceptionErr = err
	}
}

func (e *previewNetworkEvidence) interceptionError() error {
	e.mu.Lock()
	defer e.mu.Unlock()
	return e.interceptionErr
}

func (e *previewNetworkEvidence) recordFailedRequest(requestID network.RequestID) {
	e.mu.Lock()
	defer e.mu.Unlock()
	if _, outbound := e.outboundRequests[requestID]; !outbound {
		e.failedRequests[requestID] = struct{}{}
	}
}

func (e *previewNetworkEvidence) counts() (failedResources, consoleErrors, outboundRequests int) {
	e.mu.Lock()
	defer e.mu.Unlock()
	return len(e.failedRequests), e.consoleErrors, len(e.outboundRequests)
}

func validateBrowserPreviewTargets(targets []PreviewURL) (string, error) {
	if len(targets) == 0 || len(targets) > previewTargetMaxCount {
		return "", errors.New("Open Design Preview browser has an invalid target count")
	}
	allowedOrigin := ""
	for _, target := range targets {
		if err := validatePreviewTarget(target.Target); err != nil {
			return "", err
		}
		parsed, err := url.Parse(target.URL)
		if err != nil || (parsed.Scheme != "http" && parsed.Scheme != "https") || parsed.Host == "" || parsed.User != nil || parsed.Fragment != "" {
			return "", errors.New("Open Design Preview browser target URL is invalid")
		}
		hostname := parsed.Hostname()
		address := net.ParseIP(hostname)
		if hostname != "localhost" && (address == nil || !address.IsLoopback()) {
			return "", errors.New("Open Design Preview browser target must use a loopback host")
		}
		origin := parsed.Scheme + "://" + parsed.Host
		if allowedOrigin == "" {
			allowedOrigin = origin
		} else if origin != allowedOrigin {
			return "", errors.New("Open Design Preview browser targets must use one origin")
		}
	}
	return allowedOrigin, nil
}

func samePreviewOrigin(rawURL, allowedOrigin string) bool {
	parsed, err := url.Parse(rawURL)
	if err != nil {
		return false
	}
	switch parsed.Scheme {
	case "data", "blob", "about":
		return true
	case "http", "https":
		return parsed.Scheme+"://"+parsed.Host == allowedOrigin
	default:
		return false
	}
}

func ignorablePreviewBrowserResource(rawURL string) bool {
	parsed, err := url.Parse(rawURL)
	return err == nil && parsed.Path == "/favicon.ico"
}

func readPreviewBrowserIdentity(ctx context.Context) (PreviewBrowserIdentity, error) {
	chromedpContext := chromedp.FromContext(ctx)
	if chromedpContext == nil || chromedpContext.Browser == nil {
		return PreviewBrowserIdentity{}, errors.New("Open Design Preview browser context is unavailable")
	}
	_, product, _, _, _, err := browser.GetVersion().Do(cdp.WithExecutor(ctx, chromedpContext.Browser))
	if err != nil {
		return PreviewBrowserIdentity{}, fmt.Errorf("read Open Design Preview browser identity: %w", err)
	}
	name, version, found := strings.Cut(strings.TrimSpace(product), "/")
	if !found || name == "" || version == "" {
		return PreviewBrowserIdentity{}, errors.New("Open Design Preview browser returned an invalid identity")
	}
	return PreviewBrowserIdentity{Name: name, Version: version}, nil
}

func analyzePreviewScreenshot(data []byte) (PreviewScreenshot, error) {
	if len(data) == 0 || len(data) > previewScreenshotMax {
		return PreviewScreenshot{}, errors.New("Open Design Preview screenshot has an invalid size")
	}
	decoded, err := png.Decode(bytes.NewReader(data))
	if err != nil {
		return PreviewScreenshot{}, fmt.Errorf("decode Preview screenshot: %w", err)
	}
	bounds := decoded.Bounds()
	width, height := bounds.Dx(), bounds.Dy()
	if width <= 0 || height <= 0 || width > previewDimensionMax || height > previewDimensionMax {
		return PreviewScreenshot{}, errors.New("Open Design Preview screenshot has invalid dimensions")
	}

	pixelCount := width * height
	stride := 1
	if pixelCount > previewScreenshotMaxSamples {
		stride = int(math.Ceil(float64(pixelCount) / previewScreenshotMaxSamples))
	}
	var histogram [256]uint64
	var sums, squareSums [3]float64
	samples := 0
	index := 0
	for y := bounds.Min.Y; y < bounds.Max.Y; y++ {
		for x := bounds.Min.X; x < bounds.Max.X; x++ {
			if index%stride != 0 {
				index++
				continue
			}
			index++
			pixel := color.NRGBAModel.Convert(decoded.At(x, y)).(color.NRGBA)
			channels := [3]float64{float64(pixel.R), float64(pixel.G), float64(pixel.B)}
			for channel := range channels {
				sums[channel] += channels[channel]
				squareSums[channel] += channels[channel] * channels[channel]
			}
			gray := (299*uint64(pixel.R) + 587*uint64(pixel.G) + 114*uint64(pixel.B)) / 1000
			histogram[gray]++
			samples++
		}
	}
	if samples == 0 {
		return PreviewScreenshot{}, errors.New("Open Design Preview screenshot has no pixels")
	}

	entropy := 0.0
	for _, count := range histogram {
		if count == 0 {
			continue
		}
		probability := float64(count) / float64(samples)
		entropy -= probability * math.Log2(probability)
	}
	maxStddev := 0.0
	for channel := range sums {
		mean := sums[channel] / float64(samples)
		variance := squareSums[channel]/float64(samples) - mean*mean
		if variance < 0 {
			variance = 0
		}
		maxStddev = math.Max(maxStddev, math.Sqrt(variance))
	}
	digest := sha256.Sum256(data)
	return PreviewScreenshot{
		SHA256:           "sha256:" + hex.EncodeToString(digest[:]),
		Bytes:            len(data),
		Width:            width,
		Height:           height,
		Entropy:          entropy,
		MaxChannelStddev: maxStddev,
	}, nil
}
