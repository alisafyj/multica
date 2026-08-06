package opendesign

import (
	"archive/zip"
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"io/fs"
	"path"
	"sort"
	"strings"

	"github.com/multica-ai/multica/server/internal/designpreview"
)

const (
	PreviewVerificationReceiptSchema = "multica.open-design-preview-verification/v1"
	PreviewTargetKindPreview         = "preview"
	PreviewTargetKindUIKit           = "ui_kit"

	PreviewFailureDocumentNotLoaded      = "document_not_loaded"
	PreviewFailureDOMEmpty               = "dom_empty"
	PreviewFailureComputedHidden         = "computed_visibility_hidden"
	PreviewFailureRenderedContentMissing = "rendered_content_not_visible"
	PreviewFailurePageDimensions         = "page_dimensions_exceeded"
	PreviewFailureResourceLoad           = "resource_load_failed"
	PreviewFailureOutboundRequest        = "outbound_request_blocked"
	PreviewFailureConsoleError           = "console_error"
	PreviewFailureScreenshotMissing      = "screenshot_missing"
	PreviewFailureScreenshotUniform      = "screenshot_uniform"
)

const (
	previewManifestPath     = "preview/manifest.json"
	previewUIKitPath        = "ui_kits/app/index.html"
	previewManifestMaxBytes = 256 << 10
	previewTargetMaxCount   = 64
	previewTargetIDMaxBytes = 128
	previewPathMaxBytes     = 4 << 10
)

type PreviewTarget struct {
	Kind string `json:"kind"`
	ID   string `json:"id"`
	Path string `json:"path"`
}

type PreviewURL struct {
	Target PreviewTarget
	URL    string
}

type PreviewVerifier interface {
	Verify(context.Context, []PreviewURL) (PreviewVerification, error)
}

type PreviewBrowserIdentity struct {
	Name    string `json:"name"`
	Version string `json:"version"`
}

type PreviewVerificationPolicy struct {
	ViewportWidth         int     `json:"viewport_width"`
	ViewportHeight        int     `json:"viewport_height"`
	MinEntropy            float64 `json:"min_entropy"`
	MinMaxChannelStddev   float64 `json:"min_max_channel_stddev"`
	RequireSameOrigin     bool    `json:"require_same_origin"`
	RequireConsoleClean   bool    `json:"require_console_clean"`
	RequireResourcesClean bool    `json:"require_resources_clean"`
}

type PreviewScreenshot struct {
	SHA256           string  `json:"sha256,omitempty"`
	Bytes            int     `json:"bytes"`
	Width            int     `json:"width"`
	Height           int     `json:"height"`
	Entropy          float64 `json:"entropy"`
	MaxChannelStddev float64 `json:"max_channel_stddev"`
}

type PreviewCapture struct {
	Target                    PreviewTarget
	DocumentLoaded            bool
	DOMPresent                bool
	ComputedVisibilityVisible bool
	RenderedElementCount      int
	VisibleTextLength         int
	BodyWidth                 int
	BodyHeight                int
	ImageCount                int
	FailedImageCount          int
	FailedResourceCount       int
	ConsoleErrorCount         int
	OutboundRequestCount      int
	Screenshot                PreviewScreenshot
}

type PreviewTargetVerification struct {
	Target                    PreviewTarget     `json:"target"`
	Passed                    bool              `json:"passed"`
	FailureCode               string            `json:"failure_code,omitempty"`
	DocumentLoaded            bool              `json:"document_loaded"`
	DOMPresent                bool              `json:"dom_present"`
	ComputedVisibilityVisible bool              `json:"computed_visibility_visible"`
	RenderedElementCount      int               `json:"rendered_element_count"`
	VisibleTextLength         int               `json:"visible_text_length"`
	BodyWidth                 int               `json:"body_width"`
	BodyHeight                int               `json:"body_height"`
	ImageCount                int               `json:"image_count"`
	FailedImageCount          int               `json:"failed_image_count"`
	FailedResourceCount       int               `json:"failed_resource_count"`
	ConsoleErrorCount         int               `json:"console_error_count"`
	OutboundRequestCount      int               `json:"outbound_request_count"`
	Screenshot                PreviewScreenshot `json:"screenshot"`
}

type PreviewVerification struct {
	Browser PreviewBrowserIdentity      `json:"browser"`
	Policy  PreviewVerificationPolicy   `json:"policy"`
	Targets []PreviewTargetVerification `json:"targets"`
	Passed  bool                        `json:"passed"`
}

type PreviewVerificationReceipt struct {
	Schema        string              `json:"schema"`
	Engine        EngineIdentity      `json:"engine"`
	ContentDigest string              `json:"content_digest"`
	Verification  PreviewVerification `json:"verification"`
}

type RunPreviewRequest struct {
	OpenDesignRunID string                     `json:"open_design_run_id"`
	PreviewReceipt  PreviewVerificationReceipt `json:"preview_receipt"`
}

func PinnedPreviewVerificationPolicy() PreviewVerificationPolicy {
	return PreviewVerificationPolicy{
		ViewportWidth:         1440,
		ViewportHeight:        1000,
		MinEntropy:            0.1,
		MinMaxChannelStddev:   1,
		RequireSameOrigin:     true,
		RequireConsoleClean:   true,
		RequireResourcesClean: true,
	}
}

func DiscoverPreviewTargets(archive []byte) ([]PreviewTarget, error) {
	if len(archive) == 0 {
		return nil, errors.New("Open Design archive is empty")
	}
	reader, err := zip.NewReader(bytes.NewReader(archive), int64(len(archive)))
	if err != nil {
		return nil, fmt.Errorf("open Open Design archive for Preview discovery: %w", err)
	}
	files := make(map[string]*zip.File, len(reader.File))
	for _, file := range reader.File {
		name := strings.TrimSuffix(file.Name, "/")
		if name == "" || file.FileInfo().IsDir() {
			continue
		}
		if _, err := validateArchivePath(name); err != nil {
			return nil, fmt.Errorf("invalid Open Design Preview archive path %q: %w", file.Name, err)
		}
		if file.Mode()&fs.ModeSymlink != 0 || !file.Mode().IsRegular() {
			return nil, fmt.Errorf("Open Design Preview archive entry %q is not a regular file", name)
		}
		if _, exists := files[name]; exists {
			return nil, fmt.Errorf("Open Design Preview archive contains duplicate file %q", name)
		}
		files[name] = file
	}

	targets := make([]PreviewTarget, 0)
	if manifestFile := files[previewManifestPath]; manifestFile != nil {
		manifest, err := readPreviewManifest(manifestFile)
		if err != nil {
			return nil, err
		}
		seenIDs := make(map[string]struct{}, len(manifest.Previews))
		seenPaths := make(map[string]struct{}, len(manifest.Previews))
		for _, declared := range manifest.Previews {
			id := strings.TrimSpace(declared.ID)
			if id == "" || id != declared.ID || len(id) > previewTargetIDMaxBytes {
				return nil, errors.New("Open Design preview manifest has an invalid preview id")
			}
			if _, exists := seenIDs[id]; exists {
				return nil, fmt.Errorf("Open Design preview manifest repeats preview id %q", id)
			}
			relativePath, err := validatePreviewHTMLPath(declared.Path)
			if err != nil {
				return nil, fmt.Errorf("Open Design preview %q: %w", id, err)
			}
			fullPath := path.Join("preview", relativePath)
			if _, exists := seenPaths[fullPath]; exists {
				return nil, fmt.Errorf("Open Design preview manifest repeats file %q", fullPath)
			}
			if files[fullPath] == nil {
				return nil, fmt.Errorf("Open Design preview %q is missing from the archive", fullPath)
			}
			seenIDs[id] = struct{}{}
			seenPaths[fullPath] = struct{}{}
			targets = append(targets, PreviewTarget{Kind: PreviewTargetKindPreview, ID: id, Path: fullPath})
		}
	} else {
		previewPaths := make([]string, 0)
		for filePath := range files {
			if strings.HasPrefix(filePath, "preview/") && strings.EqualFold(path.Ext(filePath), ".html") {
				previewPaths = append(previewPaths, filePath)
			}
		}
		sort.Strings(previewPaths)
		for _, fullPath := range previewPaths {
			relativePath, err := validatePreviewHTMLPath(strings.TrimPrefix(fullPath, "preview/"))
			if err != nil {
				return nil, fmt.Errorf("Open Design preview %q: %w", fullPath, err)
			}
			id := strings.TrimSuffix(relativePath, path.Ext(relativePath))
			if id == "" || len(id) > previewTargetIDMaxBytes {
				return nil, fmt.Errorf("Open Design preview %q has an invalid inferred id", fullPath)
			}
			targets = append(targets, PreviewTarget{Kind: PreviewTargetKindPreview, ID: id, Path: fullPath})
		}
	}
	if files[previewUIKitPath] != nil {
		targets = append(targets, PreviewTarget{Kind: PreviewTargetKindUIKit, ID: "app", Path: previewUIKitPath})
	}
	if len(targets) == 0 {
		return nil, errors.New("Open Design package has no browser-reviewable Preview or UI Kit")
	}
	if len(targets) > previewTargetMaxCount {
		return nil, fmt.Errorf("Open Design package declares too many Preview targets: %d", len(targets))
	}
	return targets, nil
}

func readPreviewManifest(file *zip.File) (struct {
	Previews []struct {
		ID   string `json:"id"`
		Path string `json:"path"`
	} `json:"previews"`
}, error) {
	var manifest struct {
		Previews []struct {
			ID   string `json:"id"`
			Path string `json:"path"`
		} `json:"previews"`
	}
	if file.UncompressedSize64 > previewManifestMaxBytes {
		return manifest, errors.New("Open Design preview manifest exceeds the size limit")
	}
	opened, err := file.Open()
	if err != nil {
		return manifest, fmt.Errorf("open Open Design preview manifest: %w", err)
	}
	defer opened.Close()
	data, err := io.ReadAll(io.LimitReader(opened, previewManifestMaxBytes+1))
	if err != nil {
		return manifest, fmt.Errorf("read Open Design preview manifest: %w", err)
	}
	if len(data) > previewManifestMaxBytes || json.Unmarshal(data, &manifest) != nil || manifest.Previews == nil {
		return manifest, errors.New("Open Design preview manifest is invalid")
	}
	return manifest, nil
}

func validatePreviewHTMLPath(value string) (string, error) {
	if len(value) > previewPathMaxBytes {
		return "", errors.New("preview path exceeds the size limit")
	}
	validated, err := validateArchivePath(value)
	if err != nil || strings.ToLower(path.Ext(value)) != ".html" {
		return "", errors.New("preview path must be a normalized relative HTML path")
	}
	return validated, nil
}

func EvaluatePreviewCapture(capture PreviewCapture, policy PreviewVerificationPolicy) PreviewTargetVerification {
	return fromDesignPreviewTargetVerification(designpreview.EvaluateCapture(
		toDesignPreviewCapture(capture),
		toDesignPreviewPolicy(policy),
	))
}

func NewPreviewVerificationReceipt(engine EngineIdentity, contentDigest string, verification PreviewVerification) (PreviewVerificationReceipt, error) {
	receipt := PreviewVerificationReceipt{
		Schema:        PreviewVerificationReceiptSchema,
		Engine:        engine,
		ContentDigest: contentDigest,
		Verification:  verification,
	}
	if err := ValidatePreviewVerificationReceipt(receipt); err != nil {
		return PreviewVerificationReceipt{}, err
	}
	return receipt, nil
}

func ValidatePreviewVerificationReceipt(receipt PreviewVerificationReceipt) error {
	if receipt.Schema != PreviewVerificationReceiptSchema {
		return fmt.Errorf("Open Design Preview receipt schema %q does not match %q", receipt.Schema, PreviewVerificationReceiptSchema)
	}
	if err := receipt.Engine.Validate(); err != nil {
		return fmt.Errorf("invalid Open Design Preview engine: %w", err)
	}
	if err := ValidateContentDigest(receipt.ContentDigest); err != nil {
		return err
	}
	for _, target := range receipt.Verification.Targets {
		if err := validatePreviewTarget(target.Target); err != nil {
			return err
		}
	}
	return designpreview.ValidateVerification(
		toDesignPreviewVerification(receipt.Verification),
		toDesignPreviewPolicy(PinnedPreviewVerificationPolicy()),
	)
}

func ValidatePreviewVerificationTargetSet(verification PreviewVerification, expected []PreviewTarget) error {
	genericExpected := make([]designpreview.Target, 0, len(expected))
	for _, target := range expected {
		genericExpected = append(genericExpected, toDesignPreviewTarget(target))
	}
	return designpreview.ValidateTargetSet(toDesignPreviewVerification(verification), genericExpected)
}

func validatePreviewTarget(target PreviewTarget) error {
	if err := designpreview.ValidateTarget(toDesignPreviewTarget(target)); err != nil {
		return err
	}
	if target.Kind == PreviewTargetKindPreview && !strings.HasPrefix(target.Path, "preview/") {
		return errors.New("Open Design Preview target is outside preview/")
	}
	if target.Kind == PreviewTargetKindUIKit && target.Path != previewUIKitPath {
		return errors.New("Open Design UI Kit target path is invalid")
	}
	return nil
}

func toDesignPreviewTarget(target PreviewTarget) designpreview.Target {
	return designpreview.Target{Kind: target.Kind, ID: target.ID, Path: target.Path}
}

func fromDesignPreviewTarget(target designpreview.Target) PreviewTarget {
	return PreviewTarget{Kind: target.Kind, ID: target.ID, Path: target.Path}
}

func toDesignPreviewTargetURL(target PreviewURL) designpreview.TargetURL {
	return designpreview.TargetURL{Target: toDesignPreviewTarget(target.Target), URL: target.URL}
}

func toDesignPreviewPolicy(policy PreviewVerificationPolicy) designpreview.Policy {
	return designpreview.Policy{
		ViewportWidth:         policy.ViewportWidth,
		ViewportHeight:        policy.ViewportHeight,
		MinEntropy:            policy.MinEntropy,
		MinMaxChannelStddev:   policy.MinMaxChannelStddev,
		RequireSameOrigin:     policy.RequireSameOrigin,
		RequireConsoleClean:   policy.RequireConsoleClean,
		RequireResourcesClean: policy.RequireResourcesClean,
	}
}

func fromDesignPreviewPolicy(policy designpreview.Policy) PreviewVerificationPolicy {
	return PreviewVerificationPolicy{
		ViewportWidth:         policy.ViewportWidth,
		ViewportHeight:        policy.ViewportHeight,
		MinEntropy:            policy.MinEntropy,
		MinMaxChannelStddev:   policy.MinMaxChannelStddev,
		RequireSameOrigin:     policy.RequireSameOrigin,
		RequireConsoleClean:   policy.RequireConsoleClean,
		RequireResourcesClean: policy.RequireResourcesClean,
	}
}

func toDesignPreviewScreenshot(screenshot PreviewScreenshot) designpreview.Screenshot {
	return designpreview.Screenshot{
		SHA256:           screenshot.SHA256,
		Bytes:            screenshot.Bytes,
		Width:            screenshot.Width,
		Height:           screenshot.Height,
		Entropy:          screenshot.Entropy,
		MaxChannelStddev: screenshot.MaxChannelStddev,
	}
}

func fromDesignPreviewScreenshot(screenshot designpreview.Screenshot) PreviewScreenshot {
	return PreviewScreenshot{
		SHA256:           screenshot.SHA256,
		Bytes:            screenshot.Bytes,
		Width:            screenshot.Width,
		Height:           screenshot.Height,
		Entropy:          screenshot.Entropy,
		MaxChannelStddev: screenshot.MaxChannelStddev,
	}
}

func toDesignPreviewCapture(capture PreviewCapture) designpreview.Capture {
	return designpreview.Capture{
		Target:                    toDesignPreviewTarget(capture.Target),
		DocumentLoaded:            capture.DocumentLoaded,
		DOMPresent:                capture.DOMPresent,
		ComputedVisibilityVisible: capture.ComputedVisibilityVisible,
		RenderedElementCount:      capture.RenderedElementCount,
		VisibleTextLength:         capture.VisibleTextLength,
		BodyWidth:                 capture.BodyWidth,
		BodyHeight:                capture.BodyHeight,
		ImageCount:                capture.ImageCount,
		FailedImageCount:          capture.FailedImageCount,
		FailedResourceCount:       capture.FailedResourceCount,
		ConsoleErrorCount:         capture.ConsoleErrorCount,
		OutboundRequestCount:      capture.OutboundRequestCount,
		Screenshot:                toDesignPreviewScreenshot(capture.Screenshot),
	}
}

func toDesignPreviewTargetVerification(target PreviewTargetVerification) designpreview.TargetVerification {
	return designpreview.TargetVerification{
		Target:                    toDesignPreviewTarget(target.Target),
		Passed:                    target.Passed,
		FailureCode:               target.FailureCode,
		DocumentLoaded:            target.DocumentLoaded,
		DOMPresent:                target.DOMPresent,
		ComputedVisibilityVisible: target.ComputedVisibilityVisible,
		RenderedElementCount:      target.RenderedElementCount,
		VisibleTextLength:         target.VisibleTextLength,
		BodyWidth:                 target.BodyWidth,
		BodyHeight:                target.BodyHeight,
		ImageCount:                target.ImageCount,
		FailedImageCount:          target.FailedImageCount,
		FailedResourceCount:       target.FailedResourceCount,
		ConsoleErrorCount:         target.ConsoleErrorCount,
		OutboundRequestCount:      target.OutboundRequestCount,
		Screenshot:                toDesignPreviewScreenshot(target.Screenshot),
	}
}

func fromDesignPreviewTargetVerification(target designpreview.TargetVerification) PreviewTargetVerification {
	return PreviewTargetVerification{
		Target:                    fromDesignPreviewTarget(target.Target),
		Passed:                    target.Passed,
		FailureCode:               target.FailureCode,
		DocumentLoaded:            target.DocumentLoaded,
		DOMPresent:                target.DOMPresent,
		ComputedVisibilityVisible: target.ComputedVisibilityVisible,
		RenderedElementCount:      target.RenderedElementCount,
		VisibleTextLength:         target.VisibleTextLength,
		BodyWidth:                 target.BodyWidth,
		BodyHeight:                target.BodyHeight,
		ImageCount:                target.ImageCount,
		FailedImageCount:          target.FailedImageCount,
		FailedResourceCount:       target.FailedResourceCount,
		ConsoleErrorCount:         target.ConsoleErrorCount,
		OutboundRequestCount:      target.OutboundRequestCount,
		Screenshot:                fromDesignPreviewScreenshot(target.Screenshot),
	}
}

func toDesignPreviewVerification(verification PreviewVerification) designpreview.Verification {
	targets := make([]designpreview.TargetVerification, 0, len(verification.Targets))
	for _, target := range verification.Targets {
		targets = append(targets, toDesignPreviewTargetVerification(target))
	}
	return designpreview.Verification{
		Browser: designpreview.BrowserIdentity{
			Name:    verification.Browser.Name,
			Version: verification.Browser.Version,
		},
		Policy:  toDesignPreviewPolicy(verification.Policy),
		Targets: targets,
		Passed:  verification.Passed,
	}
}

func fromDesignPreviewVerification(verification designpreview.Verification) PreviewVerification {
	targets := make([]PreviewTargetVerification, 0, len(verification.Targets))
	for _, target := range verification.Targets {
		targets = append(targets, fromDesignPreviewTargetVerification(target))
	}
	return PreviewVerification{
		Browser: PreviewBrowserIdentity{
			Name:    verification.Browser.Name,
			Version: verification.Browser.Version,
		},
		Policy:  fromDesignPreviewPolicy(verification.Policy),
		Targets: targets,
		Passed:  verification.Passed,
	}
}

func PreviewVerificationFailure(verification PreviewVerification) json.RawMessage {
	if verification.Passed {
		return json.RawMessage(`{}`)
	}
	code := "open_design_preview_failed"
	message := "Open Design Preview verification rejected the candidate"
	for _, target := range verification.Targets {
		if !target.Passed {
			message += ": " + target.FailureCode + " at " + target.Target.Path
			break
		}
	}
	payload, _ := json.Marshal(map[string]string{"code": code, "message": message})
	return payload
}
