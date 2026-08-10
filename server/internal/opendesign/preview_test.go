package opendesign

import (
	"encoding/json"
	"strings"
	"testing"
)

func TestDiscoverPreviewTargetsUsesDeclaredPreviewsAndUIKit(t *testing.T) {
	t.Parallel()

	previewManifest := `{
		"version":1,
		"designSystemId":"user:crm",
		"previews":[
			{"id":"colors","title":"Colors","path":"colors-primary.html"},
			{"id":"components","title":"Components","path":"components.html"}
		]
	}`
	archive := testProjectArchive(t, []testArchiveFile{
		{Path: "DESIGN.md", Body: "# CRM"},
		{Path: "preview/manifest.json", Body: previewManifest},
		{Path: "preview/colors-primary.html", Body: "<main>Colors</main>"},
		{Path: "preview/components.html", Body: "<main>Components</main>"},
		{Path: "ui_kits/app/index.html", Body: "<main>UI Kit</main>"},
	})

	targets, err := DiscoverPreviewTargets(archive)
	if err != nil {
		t.Fatalf("DiscoverPreviewTargets: %v", err)
	}
	want := []PreviewTarget{
		{Kind: PreviewTargetKindPreview, ID: "colors", Path: "preview/colors-primary.html"},
		{Kind: PreviewTargetKindPreview, ID: "components", Path: "preview/components.html"},
		{Kind: PreviewTargetKindUIKit, ID: "app", Path: "ui_kits/app/index.html"},
	}
	if encoded, _ := json.Marshal(targets); string(encoded) != mustJSON(t, want) {
		t.Fatalf("targets = %s, want %s", encoded, mustJSON(t, want))
	}
}

func TestDiscoverPreviewTargetsUsesNativePreviewHTMLFilesWithoutJSONManifest(t *testing.T) {
	t.Parallel()

	archive := testProjectArchive(t, []testArchiveFile{
		{Path: "DESIGN.md", Body: "# CRM"},
		{Path: "preview/typography-specimens.html", Body: "<main>Typography</main>"},
		{Path: "preview/colors-primary.html", Body: "<main>Colors</main>"},
		{Path: "preview/notes.txt", Body: "not a preview target"},
		{Path: "ui_kits/app/index.html", Body: "<main>UI Kit</main>"},
	})

	targets, err := DiscoverPreviewTargets(archive)
	if err != nil {
		t.Fatalf("DiscoverPreviewTargets: %v", err)
	}
	want := []PreviewTarget{
		{Kind: PreviewTargetKindPreview, ID: "colors-primary", Path: "preview/colors-primary.html"},
		{Kind: PreviewTargetKindPreview, ID: "typography-specimens", Path: "preview/typography-specimens.html"},
		{Kind: PreviewTargetKindUIKit, ID: "app", Path: "ui_kits/app/index.html"},
	}
	if encoded, _ := json.Marshal(targets); string(encoded) != mustJSON(t, want) {
		t.Fatalf("targets = %s, want %s", encoded, mustJSON(t, want))
	}
}

func TestDiscoverPreviewTargetsRejectsMissingAndEscapingDeclaredFiles(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name     string
		manifest string
		files    []testArchiveFile
		want     string
	}{
		{
			name:     "no preview surface",
			manifest: `{"version":1,"previews":[]}`,
			want:     "no browser-reviewable Preview or UI Kit",
		},
		{
			name:     "escaping preview path",
			manifest: `{"version":1,"previews":[{"id":"bad","path":"../index.html"}]}`,
			files:    []testArchiveFile{{Path: "index.html", Body: "<main>bad</main>"}},
			want:     "normalized relative HTML path",
		},
		{
			name:     "missing declared preview",
			manifest: `{"version":1,"previews":[{"id":"colors","path":"colors.html"}]}`,
			want:     "is missing from the archive",
		},
	}
	for _, tt := range tests {
		testCase := tt
		t.Run(testCase.name, func(t *testing.T) {
			t.Parallel()
			files := append([]testArchiveFile{{Path: "preview/manifest.json", Body: testCase.manifest}}, testCase.files...)
			_, err := DiscoverPreviewTargets(testProjectArchive(t, files))
			if err == nil || !strings.Contains(err.Error(), testCase.want) {
				t.Fatalf("DiscoverPreviewTargets error = %v, want %q", err, testCase.want)
			}
		})
	}
}

func TestNewPreviewVerificationReceiptBindsEngineDigestAndVisualSignals(t *testing.T) {
	t.Parallel()

	verification := successfulPreviewVerification()
	receipt, err := NewPreviewVerificationReceipt(
		PinnedEngineIdentity(),
		"sha256:aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa",
		verification,
	)
	if err != nil {
		t.Fatalf("NewPreviewVerificationReceipt: %v", err)
	}
	if receipt.Schema != PreviewVerificationReceiptSchema ||
		receipt.Engine != PinnedEngineIdentity() ||
		receipt.ContentDigest != "sha256:aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa" ||
		!receipt.Verification.Passed {
		t.Fatalf("receipt = %+v", receipt)
	}

	invalid := receipt
	invalid.Verification.Targets = append([]PreviewTargetVerification(nil), receipt.Verification.Targets...)
	invalid.Verification.Targets[0].Screenshot.Entropy = 0
	if err := ValidatePreviewVerificationReceipt(invalid); err == nil || !strings.Contains(err.Error(), "passed target") {
		t.Fatalf("ValidatePreviewVerificationReceipt error = %v, want inconsistent passed target", err)
	}

	nativeUIKit := receipt
	nativeUIKit.Verification.Targets = append([]PreviewTargetVerification(nil), receipt.Verification.Targets...)
	nativeUIKit.Verification.Targets[1].Target = PreviewTarget{Kind: PreviewTargetKindUIKit, ID: "ui-kit", Path: "ui-kit/index.html"}
	if err := ValidatePreviewVerificationReceipt(nativeUIKit); err == nil || !strings.Contains(err.Error(), "UI Kit target path") {
		t.Fatalf("native UI Kit target error = %v", err)
	}
}

func TestEvaluatePreviewCaptureRejectsUniformScreenshotEvenWhenPixelsAreNonWhite(t *testing.T) {
	t.Parallel()

	capture := PreviewCapture{
		Target:                    PreviewTarget{Kind: PreviewTargetKindPreview, ID: "colors", Path: "preview/colors.html"},
		DocumentLoaded:            true,
		DOMPresent:                true,
		ComputedVisibilityVisible: true,
		RenderedElementCount:      41,
		VisibleTextLength:         317,
		BodyWidth:                 1425,
		BodyHeight:                1064,
		Screenshot: PreviewScreenshot{
			SHA256:           "sha256:bbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbb",
			Bytes:            12_345,
			Width:            1440,
			Height:           1000,
			Entropy:          0,
			MaxChannelStddev: 0,
		},
	}

	verification := EvaluatePreviewCapture(capture, PinnedPreviewVerificationPolicy())
	if verification.Passed || verification.FailureCode != PreviewFailureScreenshotUniform {
		t.Fatalf("verification = %+v", verification)
	}
}

func successfulPreviewVerification() PreviewVerification {
	policy := PinnedPreviewVerificationPolicy()
	previewCapture := PreviewCapture{
		Target:                    PreviewTarget{Kind: PreviewTargetKindPreview, ID: "colors", Path: "preview/colors.html"},
		DocumentLoaded:            true,
		DOMPresent:                true,
		ComputedVisibilityVisible: true,
		RenderedElementCount:      41,
		VisibleTextLength:         317,
		BodyWidth:                 1425,
		BodyHeight:                1064,
		Screenshot: PreviewScreenshot{
			SHA256:           "sha256:bbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbb",
			Bytes:            12_345,
			Width:            policy.ViewportWidth,
			Height:           policy.ViewportHeight,
			Entropy:          3.4,
			MaxChannelStddev: 76.8,
		},
	}
	uiKitCapture := previewCapture
	uiKitCapture.Target = PreviewTarget{Kind: PreviewTargetKindUIKit, ID: "app", Path: previewUIKitPath}
	uiKitCapture.Screenshot.SHA256 = "sha256:cccccccccccccccccccccccccccccccccccccccccccccccccccccccccccccccc"
	return PreviewVerification{
		Browser: PreviewBrowserIdentity{Name: "chromium", Version: "138.0.0.0"},
		Policy:  policy,
		Targets: []PreviewTargetVerification{
			EvaluatePreviewCapture(previewCapture, policy),
			EvaluatePreviewCapture(uiKitCapture, policy),
		},
		Passed: true,
	}
}

func mustJSON(t *testing.T, value any) string {
	t.Helper()
	encoded, err := json.Marshal(value)
	if err != nil {
		t.Fatalf("marshal JSON: %v", err)
	}
	return string(encoded)
}
