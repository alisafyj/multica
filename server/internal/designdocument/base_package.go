package designdocument

import (
	"errors"
	"fmt"

	"github.com/google/uuid"
)

// The base package of a page-design adjustment travels as a REFERENCE, never
// as contents. A design system base package is three text files and can be
// inlined into a task context; a design document package is an archive of
// HTML, CSS, JavaScript and assets that runs to tens of megabytes, so inlining
// it would push the whole prototype through the task-context JSONB column and
// through every task list that reads it.
//
// The reference is not a second on-the-wire shape either: it is built from the
// base_revision_id and base_content_digest the task context already pins, so
// there is exactly one place a base can be described and nothing for a
// producer and a consumer to disagree about.
const (
	// BaseArchiveContentType is the media type the base archive endpoint serves.
	BaseArchiveContentType = "application/zip"
	// BaseArchiveContentDigestHeader carries the served archive's content
	// digest so the daemon can reject a substituted body before reading it.
	BaseArchiveContentDigestHeader = "X-Multica-Design-Document-Base-Digest"
	// DeliveryArchiveDigestHeader carries the digest of a package delivered to
	// an implementation task, so the daemon can refuse a mismatch before a byte
	// reaches the agent's filesystem (DC-062).
	DeliveryArchiveDigestHeader = "X-Multica-Design-Delivery-Digest"
	// BaseArchiveRevisionIDHeader names the revision the served archive is.
	BaseArchiveRevisionIDHeader = "X-Multica-Design-Document-Base-Revision"
	// BaseArchiveMaxBytes bounds a base archive download. It is the same
	// ceiling ValidateArchive enforces, so a body this endpoint would accept
	// but validation would reject can never be read into memory.
	BaseArchiveMaxBytes int64 = maxArchiveBytes
)

// BasePackageReference pins the immutable revision an adjustment starts from.
type BasePackageReference struct {
	RevisionID    string `json:"revision_id"`
	ContentDigest string `json:"content_digest"`
}

// ValidateBasePackageReference rejects a reference that could not name a real
// revision. Both fields are checked because either one alone is forgeable: the
// revision id decides which archive the server loads, and the digest decides
// whether the bytes that come back are the ones the task was pinned to.
func ValidateBasePackageReference(reference BasePackageReference) error {
	parsed, err := uuid.Parse(reference.RevisionID)
	if err != nil || parsed.String() != reference.RevisionID {
		return errors.New("design document base package reference revision is invalid")
	}
	if !validSHA256Reference(reference.ContentDigest) {
		return errors.New("design document base package reference digest is invalid")
	}
	return nil
}

// ReadBaseArchive re-validates a base archive and returns every package file
// keyed by its package path (manifest.json excluded — it is generated, not a
// package artifact, and nothing in base/ may reference it).
//
// The archive is validated against the binding written into its OWN manifest,
// because a base revision was produced by an earlier task and its identity can
// never match the run that is adjusting it. That is not a weaker check:
// ValidateArchive recomputes every per-file digest from the archive bytes,
// requires the manifest index to equal the recomputed one, re-derives the
// content digest from that index, and re-runs the whole audit. Tampering with
// any file changes the recomputed index; rewriting the manifest to match
// changes the content digest — and the digest the caller pinned is what closes
// that door, which is why it is required rather than optional.
func ReadBaseArchive(archive []byte, contentDigest string) (ValidatedPackage, map[string][]byte, error) {
	if !validSHA256Reference(contentDigest) {
		return ValidatedPackage{}, nil, errors.New("design document base archive digest is invalid")
	}
	files, _, manifestJSON, err := readAndIndexArchive(archive)
	if err != nil {
		return ValidatedPackage{}, nil, err
	}
	var manifest Manifest
	if err := decodeStrictJSON(manifestJSON, &manifest); err != nil {
		return ValidatedPackage{}, nil, fmt.Errorf("decode design document base manifest: %w", err)
	}
	validated, err := ValidateArchive(archive, manifest.Binding)
	if err != nil {
		return ValidatedPackage{}, nil, err
	}
	if validated.Manifest.ContentDigest != contentDigest {
		return ValidatedPackage{}, nil, fmt.Errorf(
			"design document base archive digest %q does not match the pinned digest %q",
			validated.Manifest.ContentDigest, contentDigest)
	}
	return validated, files, nil
}
