package opendesign

import (
	"context"

	"github.com/multica-ai/multica/server/internal/designpreview"
)

type ChromiumPreviewVerifier struct {
	verifier *designpreview.ChromiumVerifier
}

func NewChromiumPreviewVerifier(rawBrowserPath string) (*ChromiumPreviewVerifier, error) {
	verifier, err := designpreview.NewChromiumVerifierWithPolicy(
		rawBrowserPath,
		toDesignPreviewPolicy(PinnedPreviewVerificationPolicy()),
	)
	if err != nil {
		return nil, err
	}
	return &ChromiumPreviewVerifier{verifier: verifier}, nil
}

func (v *ChromiumPreviewVerifier) Verify(ctx context.Context, targets []PreviewURL) (PreviewVerification, error) {
	genericTargets := make([]designpreview.TargetURL, 0, len(targets))
	for _, target := range targets {
		if err := validatePreviewTarget(target.Target); err != nil {
			return PreviewVerification{}, err
		}
		genericTargets = append(genericTargets, toDesignPreviewTargetURL(target))
	}
	verification, err := v.verifier.Verify(ctx, genericTargets)
	if err != nil {
		return PreviewVerification{}, err
	}
	return fromDesignPreviewVerification(verification), nil
}
