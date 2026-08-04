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
	"math"
	"path"
	"sort"
	"strings"
)

const (
	PreviewVerificationReceiptSchema = "multica.open-design-preview-verification/v1"
	PreviewTargetKindPreview         = "preview"
	PreviewTargetKindUIKit           = "ui_kit"

	PreviewFailureDocumentNotLoaded      = "document_not_loaded"
	PreviewFailureDOMEmpty               = "dom_empty"
	PreviewFailureComputedHidden         = "computed_visibility_hidden"
	PreviewFailureRenderedContentMissing = "rendered_content_not_visible"
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
	previewMetricMaxCount   = 1_000_000
	previewDimensionMax     = 100_000
	previewScreenshotMax    = 32 << 20
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
	verification := PreviewTargetVerification{
		Target:                    capture.Target,
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
		Screenshot:                capture.Screenshot,
	}
	switch {
	case !capture.DocumentLoaded:
		verification.FailureCode = PreviewFailureDocumentNotLoaded
	case !capture.DOMPresent || capture.BodyWidth <= 0 || capture.BodyHeight <= 0:
		verification.FailureCode = PreviewFailureDOMEmpty
	case !capture.ComputedVisibilityVisible:
		verification.FailureCode = PreviewFailureComputedHidden
	case capture.RenderedElementCount <= 0:
		verification.FailureCode = PreviewFailureRenderedContentMissing
	case policy.RequireSameOrigin && capture.OutboundRequestCount > 0:
		verification.FailureCode = PreviewFailureOutboundRequest
	case policy.RequireResourcesClean && (capture.FailedImageCount > 0 || capture.FailedResourceCount > 0):
		verification.FailureCode = PreviewFailureResourceLoad
	case policy.RequireConsoleClean && capture.ConsoleErrorCount > 0:
		verification.FailureCode = PreviewFailureConsoleError
	case capture.Screenshot.SHA256 == "" || capture.Screenshot.Bytes <= 0 || capture.Screenshot.Width <= 0 || capture.Screenshot.Height <= 0:
		verification.FailureCode = PreviewFailureScreenshotMissing
	case capture.Screenshot.Entropy < policy.MinEntropy || capture.Screenshot.MaxChannelStddev < policy.MinMaxChannelStddev:
		verification.FailureCode = PreviewFailureScreenshotUniform
	default:
		verification.Passed = true
	}
	return verification
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
	verification := receipt.Verification
	if strings.TrimSpace(verification.Browser.Name) == "" || strings.TrimSpace(verification.Browser.Version) == "" ||
		len(verification.Browser.Name) > 128 || len(verification.Browser.Version) > 256 {
		return errors.New("Open Design Preview browser identity is invalid")
	}
	if verification.Policy != PinnedPreviewVerificationPolicy() {
		return errors.New("Open Design Preview verification policy is not pinned")
	}
	if len(verification.Targets) == 0 || len(verification.Targets) > previewTargetMaxCount {
		return errors.New("Open Design Preview verification has an invalid target count")
	}
	seen := make(map[string]struct{}, len(verification.Targets))
	allPassed := true
	for _, target := range verification.Targets {
		if err := validatePreviewTargetVerification(target, verification.Policy); err != nil {
			return err
		}
		key := target.Target.Kind + "\x00" + target.Target.ID + "\x00" + target.Target.Path
		if _, exists := seen[key]; exists {
			return errors.New("Open Design Preview verification repeats a target")
		}
		seen[key] = struct{}{}
		allPassed = allPassed && target.Passed
	}
	if verification.Passed != allPassed {
		return errors.New("Open Design Preview overall result does not match its targets")
	}
	return nil
}

func ValidatePreviewVerificationTargetSet(verification PreviewVerification, expected []PreviewTarget) error {
	if len(verification.Targets) != len(expected) {
		return errors.New("Open Design Preview verification target count does not match the declared package targets")
	}
	for index, target := range verification.Targets {
		if target.Target != expected[index] {
			return fmt.Errorf("Open Design Preview verification target %d does not match the declared package target", index)
		}
	}
	return nil
}

func validatePreviewTargetVerification(target PreviewTargetVerification, policy PreviewVerificationPolicy) error {
	if err := validatePreviewTarget(target.Target); err != nil {
		return err
	}
	for _, value := range []int{
		target.RenderedElementCount, target.VisibleTextLength, target.ImageCount,
		target.FailedImageCount, target.FailedResourceCount, target.ConsoleErrorCount,
		target.OutboundRequestCount,
	} {
		if value < 0 || value > previewMetricMaxCount {
			return errors.New("Open Design Preview target has an invalid count")
		}
	}
	if target.FailedImageCount > target.ImageCount || target.BodyWidth < 0 || target.BodyWidth > previewDimensionMax ||
		target.BodyHeight < 0 || target.BodyHeight > previewDimensionMax {
		return errors.New("Open Design Preview target has invalid dimensions or image counts")
	}
	if err := validatePreviewScreenshot(target.Screenshot, target.Passed); err != nil {
		return err
	}
	capture := PreviewCapture{
		Target:                    target.Target,
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
		Screenshot:                target.Screenshot,
	}
	evaluated := EvaluatePreviewCapture(capture, policy)
	if evaluated.Passed != target.Passed || evaluated.FailureCode != target.FailureCode {
		return errors.New("Open Design Preview passed target does not match its visual signals")
	}
	return nil
}

func validatePreviewTarget(target PreviewTarget) error {
	if target.Kind != PreviewTargetKindPreview && target.Kind != PreviewTargetKindUIKit {
		return errors.New("Open Design Preview target kind is invalid")
	}
	if target.ID == "" || strings.TrimSpace(target.ID) != target.ID || len(target.ID) > previewTargetIDMaxBytes {
		return errors.New("Open Design Preview target id is invalid")
	}
	if len(target.Path) > previewPathMaxBytes {
		return errors.New("Open Design Preview target path exceeds the size limit")
	}
	if _, err := validateArchivePath(target.Path); err != nil || strings.ToLower(path.Ext(target.Path)) != ".html" {
		return errors.New("Open Design Preview target path is invalid")
	}
	if target.Kind == PreviewTargetKindPreview && !strings.HasPrefix(target.Path, "preview/") {
		return errors.New("Open Design Preview target is outside preview/")
	}
	if target.Kind == PreviewTargetKindUIKit && target.Path != previewUIKitPath {
		return errors.New("Open Design UI Kit target path is invalid")
	}
	return nil
}

func validatePreviewScreenshot(screenshot PreviewScreenshot, required bool) error {
	empty := screenshot.SHA256 == "" && screenshot.Bytes == 0 && screenshot.Width == 0 && screenshot.Height == 0 &&
		screenshot.Entropy == 0 && screenshot.MaxChannelStddev == 0
	if empty && !required {
		return nil
	}
	if err := ValidateContentDigest(screenshot.SHA256); err != nil {
		return errors.New("Open Design Preview screenshot digest is invalid")
	}
	if screenshot.Bytes <= 0 || screenshot.Bytes > previewScreenshotMax ||
		screenshot.Width <= 0 || screenshot.Width > previewDimensionMax ||
		screenshot.Height <= 0 || screenshot.Height > previewDimensionMax ||
		math.IsNaN(screenshot.Entropy) || math.IsInf(screenshot.Entropy, 0) || screenshot.Entropy < 0 || screenshot.Entropy > 8 ||
		math.IsNaN(screenshot.MaxChannelStddev) || math.IsInf(screenshot.MaxChannelStddev, 0) || screenshot.MaxChannelStddev < 0 || screenshot.MaxChannelStddev > 128 {
		return errors.New("Open Design Preview screenshot metrics are invalid")
	}
	return nil
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
