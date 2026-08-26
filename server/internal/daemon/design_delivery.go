package daemon

import (
	"context"
	"errors"
	"fmt"

	"github.com/multica-ai/multica/server/internal/daemon/execenv"
	"github.com/multica-ai/multica/server/internal/designdocument"
)

// restoreDesignDeliveryPackage fills the package directory execenv reserved for
// an implementation task whose issue has a delivered design (DC-062).
//
// Same shape as restoreDesignDocumentBaseArchive — download, verify, extract —
// and the same reason for the read-only landing spot: this package is an INPUT
// the agent must build from, not something it may edit. Full re-validation
// runs before a byte reaches the filesystem, so a package that no longer
// matches the digest the delivery pinned fails the task rather than quietly
// becoming the design the agent implements.
//
// A no-op when nothing was delivered, which is the common case.
func (d *Daemon) restoreDesignDeliveryPackage(ctx context.Context, task Task, envRoot, workDir string) error {
	if len(task.DesignDeliveryContext) == 0 {
		return nil
	}
	var envelope struct {
		SchemaVersion string `json:"schema_version"`
		RevisionID    string `json:"revision_id"`
		ContentDigest string `json:"content_digest"`
	}
	if err := jsonUnmarshal(task.DesignDeliveryContext, &envelope); err != nil {
		return fmt.Errorf("decode design delivery context: %w", err)
	}
	if envelope.SchemaVersion != designDeliverySchema {
		// A delivery envelope this daemon does not understand is not something
		// to guess at: refuse rather than hand the agent a partial input.
		return fmt.Errorf("unsupported design delivery schema %q", envelope.SchemaVersion)
	}
	reference := designdocument.BasePackageReference{
		RevisionID:    envelope.RevisionID,
		ContentDigest: envelope.ContentDigest,
	}
	if err := designdocument.ValidateBasePackageReference(reference); err != nil {
		return fmt.Errorf("validate delivered design reference: %w", err)
	}
	if d.client == nil {
		return errors.New("design delivery client is unavailable")
	}
	archive, err := d.client.DownloadDesignDeliveryArchive(ctx, task.ID, reference.ContentDigest)
	if err != nil {
		return fmt.Errorf("download delivered design package: %w", err)
	}
	_, files, err := designdocument.ReadBaseArchive(archive, reference.ContentDigest)
	if err != nil {
		return fmt.Errorf("validate delivered design package: %w", err)
	}
	if err := execenv.ExtractDesignDeliveryPackage(envRoot, workDir, files); err != nil {
		return fmt.Errorf("extract delivered design package: %w", err)
	}
	return nil
}

// designDeliverySchema mirrors service.DesignDeliverySchema. The daemon is a
// separate binary that speaks to the server over HTTP and must not import its
// internals, so the wire constant is restated here — a mismatch is caught by
// the cross-boundary test rather than at compile time.
const designDeliverySchema = "multica.design-delivery/v1"
