package daemon

import (
	"context"
	"errors"
	"fmt"

	"github.com/multica-ai/multica/server/internal/daemon/execenv"
	"github.com/multica-ai/multica/server/internal/designdocument"
)

// restoreDesignDocumentBaseArchive fills the base directory execenv reserved
// for a page-design adjustment.
//
// It mirrors restoreOpenDesignBaseArchive: download, verify, extract. The one
// structural difference is where the package lands. An Open Design adjustment
// edits its base in place, so the archive is restored into the scratch root; a
// design document adjustment must emit a whole new package to
// $MULTICA_OUTPUT_DIR, so the base stays a read-only INPUT under
// .agent_context/design_document/base/ alongside the task envelope.
//
// A no-op for anything that is not an adjustment or regeneration — those are
// the only operations execenv reserves a base directory for.
//
// envRoot is the environment's scratch root, carried through so the extracted
// package joins the sidecar manifest: this restore runs after execenv.Prepare
// persisted its list, and a base that is not on it survives cleanup in the
// user's own directory.
func (d *Daemon) restoreDesignDocumentBaseArchive(ctx context.Context, task Task, envRoot, workDir string) error {
	if len(task.DesignDocumentContext) == 0 {
		return nil
	}
	var taskContext struct {
		Type              string `json:"type"`
		Operation         string `json:"operation"`
		BaseRevisionID    string `json:"base_revision_id"`
		BaseContentDigest string `json:"base_content_digest"`
	}
	if err := jsonUnmarshal(task.DesignDocumentContext, &taskContext); err != nil {
		return fmt.Errorf("decode design document task context: %w", err)
	}
	if taskContext.Type != "design_document_task" {
		return nil
	}
	if !designDocumentOperationUsesBase(taskContext.Operation) {
		return nil
	}
	reference := designdocument.BasePackageReference{
		RevisionID:    taskContext.BaseRevisionID,
		ContentDigest: taskContext.BaseContentDigest,
	}
	if err := designdocument.ValidateBasePackageReference(reference); err != nil {
		return fmt.Errorf("validate design document base package reference: %w", err)
	}
	if d.client == nil {
		return errors.New("design document base archive client is unavailable")
	}
	archive, err := d.client.DownloadDesignDocumentBaseArchive(ctx, task.ID, reference)
	if err != nil {
		return fmt.Errorf("download design document base archive: %w", err)
	}
	// Full re-validation before a single byte reaches the agent's filesystem:
	// a package that no longer audits, or that is not the digest this task was
	// pinned to, must fail the task rather than become the thing the agent
	// "adjusts".
	_, files, err := designdocument.ReadBaseArchive(archive, reference.ContentDigest)
	if err != nil {
		return fmt.Errorf("validate design document base archive: %w", err)
	}
	if err := execenv.ExtractDesignDocumentBase(envRoot, workDir, files); err != nil {
		return fmt.Errorf("extract design document base archive: %w", err)
	}
	return nil
}

// designDocumentOperationUsesBase reports whether a run starts from an
// immutable base revision rather than from nothing.
//
// All three of these restore base/, ground against a pinned receipt instead of
// re-reading the repository, and emit a whole new package. They differ only in
// who produces that package: an agent for adjust and regenerate, the daemon
// itself for a manual edit (DC-062).
func designDocumentOperationUsesBase(operation string) bool {
	switch operation {
	case "adjust", "regenerate", "manual_edit":
		return true
	default:
		return false
	}
}
