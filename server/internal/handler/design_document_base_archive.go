package handler

import (
	"encoding/json"
	"errors"
	"net/http"
	"strconv"

	"github.com/go-chi/chi/v5"
	"github.com/jackc/pgx/v5"

	"github.com/multica-ai/multica/server/internal/designdocument"
	"github.com/multica-ai/multica/server/internal/service"
	"github.com/multica-ai/multica/server/internal/util"
	db "github.com/multica-ai/multica/server/pkg/db/generated"
)

// DownloadDesignDocumentBaseArchive serves the prototype package a page-design
// adjustment starts from, the design-document sibling of
// DownloadOpenDesignBaseArchive.
//
// The revision is taken from the TASK CONTEXT, never from the request: a
// daemon holding a token for one task must not be able to name another
// document's revision and pull its archive. The revision is then re-checked
// against the same document AND the same workspace as the task context before
// a single byte is read from storage.
func (h *Handler) DownloadDesignDocumentBaseArchive(w http.ResponseWriter, r *http.Request) {
	task, workspaceID, ok := h.requireDaemonTaskAccessWithWorkspace(w, r, chi.URLParam(r, "taskId"))
	if !ok {
		return
	}
	var taskContext service.DesignDocumentTaskContext
	if json.Unmarshal(task.Context, &taskContext) != nil ||
		taskContext.Type != service.DesignDocumentTaskContextType ||
		(taskContext.Operation != service.DesignDocumentAdjust && taskContext.Operation != service.DesignDocumentRegenerate) ||
		taskContext.WorkspaceID != workspaceID ||
		taskContext.AgentID != uuidToString(task.AgentID) {
		writeDesignDocumentBaseArchiveUnavailable(w)
		return
	}
	reference := designdocument.BasePackageReference{
		RevisionID:    taskContext.BaseRevisionID,
		ContentDigest: taskContext.BaseContentDigest,
	}
	if designdocument.ValidateBasePackageReference(reference) != nil {
		writeDesignDocumentBaseArchiveUnavailable(w)
		return
	}
	// Both ids come from the task context the server itself wrote, so they are
	// trusted round-trips rather than request input — but they are still parsed
	// safely, because a context row could have been written by an older shape.
	revisionUUID, err := util.ParseUUID(reference.RevisionID)
	if err != nil {
		writeDesignDocumentBaseArchiveUnavailable(w)
		return
	}
	workspaceUUID, err := util.ParseUUID(workspaceID)
	if err != nil {
		writeDesignDocumentBaseArchiveUnavailable(w)
		return
	}
	revision, err := h.Queries.GetDesignDocumentRevisionInWorkspace(r.Context(), db.GetDesignDocumentRevisionInWorkspaceParams{
		ID: revisionUUID, WorkspaceID: workspaceUUID,
	})
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			writeDesignDocumentBaseArchiveUnavailable(w)
			return
		}
		writeProjectDesignSystemError(w, http.StatusInternalServerError, "design_document_base_archive_lookup_failed", "failed to load design document base archive")
		return
	}
	// The workspace filter above already scopes the row; this is the tenancy
	// check that matters within a workspace — one document's task must never be
	// able to read another document's prototype.
	if uuidToString(revision.DesignDocumentID) != taskContext.DesignDocumentID {
		writeDesignDocumentBaseArchiveUnavailable(w)
		return
	}
	if revision.ContentDigest != reference.ContentDigest || revision.PackageSchema != designdocument.PackageSchemaV1 {
		writeDesignDocumentBaseArchiveUnavailable(w)
		return
	}
	if h.Storage == nil {
		writeProjectDesignSystemError(w, http.StatusServiceUnavailable, "design_document_base_archive_storage_unavailable", "design document package storage is unavailable")
		return
	}
	archive, err := readNativeArchiveFromStorage(r.Context(), h.Storage, revision.ArchiveObjectKey)
	if err != nil {
		writeProjectDesignSystemError(w, http.StatusInternalServerError, "design_document_base_archive_read_failed", "failed to read design document base archive")
		return
	}

	w.Header().Set("Content-Type", designdocument.BaseArchiveContentType)
	w.Header().Set("Content-Length", strconv.Itoa(len(archive)))
	w.Header().Set("Cache-Control", "no-store")
	w.Header().Set("X-Content-Type-Options", "nosniff")
	w.Header().Set(designdocument.BaseArchiveContentDigestHeader, revision.ContentDigest)
	w.Header().Set(designdocument.BaseArchiveRevisionIDHeader, uuidToString(revision.ID))
	w.WriteHeader(http.StatusOK)
	_, _ = w.Write(archive)
}

func writeDesignDocumentBaseArchiveUnavailable(w http.ResponseWriter) {
	writeProjectDesignSystemError(w, http.StatusConflict, "design_document_base_archive_unavailable", "design document base archive is unavailable")
}
