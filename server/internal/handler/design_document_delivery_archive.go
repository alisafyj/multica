package handler

import (
	"errors"
	"net/http"
	"strconv"

	"github.com/go-chi/chi/v5"
	"github.com/jackc/pgx/v5"

	"github.com/multica-ai/multica/server/internal/designdocument"
	db "github.com/multica-ai/multica/server/pkg/db/generated"
)

// DownloadDesignDeliveryArchive serves the delivered design package to the
// daemon running an implementation task (DC-062).
//
// The revision is resolved the same way the claim resolved it: from the issue
// this task belongs to, through the document's own saved pointer. Nothing in
// the request names a document or a revision, so a daemon holding a token for
// one task cannot reach another issue's design — and because the pointer is
// re-read here rather than trusted from the claim, a design that was detached
// or unsaved in between stops being downloadable immediately.
func (h *Handler) DownloadDesignDeliveryArchive(w http.ResponseWriter, r *http.Request) {
	task, workspaceID, ok := h.requireDaemonTaskAccessWithWorkspace(w, r, chi.URLParam(r, "taskId"))
	if !ok {
		return
	}
	if !task.IssueID.Valid {
		writeDesignDeliveryArchiveUnavailable(w)
		return
	}
	issue, err := h.Queries.GetIssue(r.Context(), task.IssueID)
	if err != nil || uuidToString(issue.WorkspaceID) != workspaceID {
		writeDesignDeliveryArchiveUnavailable(w)
		return
	}
	delivery := h.designDeliveryContextForIssue(r.Context(), issue.WorkspaceID, issue.ID)
	if delivery == nil {
		writeDesignDeliveryArchiveUnavailable(w)
		return
	}
	revisionUUID, ok := parseUUIDOrBadRequest(w, delivery.RevisionID, "revision_id")
	if !ok {
		return
	}
	revision, err := h.Queries.GetDesignDocumentRevisionInWorkspace(r.Context(), db.GetDesignDocumentRevisionInWorkspaceParams{
		ID: revisionUUID, WorkspaceID: issue.WorkspaceID,
	})
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			writeDesignDeliveryArchiveUnavailable(w)
			return
		}
		writeProjectDesignSystemError(w, http.StatusInternalServerError, "design_delivery_archive_lookup_failed", "failed to load the delivered design package")
		return
	}
	if revision.ContentDigest != delivery.ContentDigest || revision.PackageSchema != designdocument.PackageSchemaV1 {
		writeDesignDeliveryArchiveUnavailable(w)
		return
	}
	if h.Storage == nil {
		writeProjectDesignSystemError(w, http.StatusServiceUnavailable, "design_delivery_archive_storage_unavailable", "design document package storage is unavailable")
		return
	}
	archive, err := readNativeArchiveFromStorage(r.Context(), h.Storage, revision.ArchiveObjectKey)
	if err != nil {
		writeProjectDesignSystemError(w, http.StatusInternalServerError, "design_delivery_archive_read_failed", "failed to read the delivered design package")
		return
	}

	w.Header().Set("Content-Type", designdocument.BaseArchiveContentType)
	w.Header().Set("Content-Length", strconv.Itoa(len(archive)))
	w.Header().Set("Cache-Control", "no-store")
	w.Header().Set("X-Content-Type-Options", "nosniff")
	w.Header().Set(designdocument.DeliveryArchiveDigestHeader, revision.ContentDigest)
	w.WriteHeader(http.StatusOK)
	_, _ = w.Write(archive)
}

func writeDesignDeliveryArchiveUnavailable(w http.ResponseWriter) {
	writeProjectDesignSystemError(w, http.StatusConflict, "design_delivery_archive_unavailable", "no delivered design package is available for this task")
}
