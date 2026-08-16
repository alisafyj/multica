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
func (d *Daemon) restoreDesignDocumentBaseArchive(ctx context.Context, task Task, workDir string) error {
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
	if taskContext.Operation != "adjust" && taskContext.Operation != "regenerate" {
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
	if err := execenv.ExtractDesignDocumentBase(workDir, files); err != nil {
		return fmt.Errorf("extract design document base archive: %w", err)
	}
	return nil
}
