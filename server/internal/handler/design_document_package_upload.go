package handler

import (
	"encoding/json"
	"errors"
	"io"
	"mime"
	"net/http"
	"strings"

	"github.com/go-chi/chi/v5"

	"github.com/multica-ai/multica/server/internal/designdocument"
	"github.com/multica-ai/multica/server/internal/service"
)

const designDocumentUploadFilenameRoot = "design-document-package-"

// UploadDesignDocumentPackage stores a collected page-design package. It keeps
// its own object key namespace so a package of one design kind can never land
// under the other's prefix, and it validates the archive against the task
// binding before storing anything — an archive that does not belong to this
// task must not reach object storage at all.
func (h *Handler) UploadDesignDocumentPackage(w http.ResponseWriter, r *http.Request) {
	task, workspaceID, ok := h.requireDaemonTaskAccessWithWorkspace(w, r, chi.URLParam(r, "taskId"))
	if !ok {
		return
	}
	if task.Status != "running" {
		writeProjectDesignSystemError(w, http.StatusConflict, "design_document_task_not_running", "design document package upload requires a running task")
		return
	}

	var taskContext service.DesignDocumentTaskContext
	if err := json.Unmarshal(task.Context, &taskContext); err != nil ||
		taskContext.Type != service.DesignDocumentTaskContextType {
		writeProjectDesignSystemError(w, http.StatusConflict, "design_document_task_invalid", "task is not a design document task")
		return
	}
	if taskContext.WorkspaceID != workspaceID || taskContext.AgentID != uuidToString(task.AgentID) {
		writeProjectDesignSystemError(w, http.StatusConflict, "design_document_task_invalid", "task context does not match daemon task ownership")
		return
	}

	rawDigest := r.Header.Get(nativePackageDigestHeader)
	contentDigest := strings.TrimSpace(rawDigest)
	if rawDigest != contentDigest || !validNativePackageDigest(contentDigest) {
		writeProjectDesignSystemError(w, http.StatusBadRequest, "design_document_digest_invalid", "invalid design document package digest")
		return
	}
	mediaType, _, err := mime.ParseMediaType(r.Header.Get("Content-Type"))
	if err != nil || mediaType != nativePackageArchiveContentType {
		writeProjectDesignSystemError(w, http.StatusUnsupportedMediaType, "design_document_media_type_invalid", "design document package must be an application/zip payload")
		return
	}
	if r.ContentLength > nativePackageArchiveMaxBytes {
		writeProjectDesignSystemError(w, http.StatusRequestEntityTooLarge, "design_document_too_large", "design document package exceeds the upload limit")
		return
	}
	r.Body = http.MaxBytesReader(w, r.Body, nativePackageArchiveMaxBytes)
	archive, err := io.ReadAll(r.Body)
	if err != nil {
		var maxBytesError *http.MaxBytesError
		if errors.As(err, &maxBytesError) {
			writeProjectDesignSystemError(w, http.StatusRequestEntityTooLarge, "design_document_too_large", "design document package exceeds the upload limit")
			return
		}
		writeProjectDesignSystemError(w, http.StatusBadRequest, "design_document_read_failed", "failed to read design document package")
		return
	}

	binding := designDocumentBindingFromContext(taskContext, task)
	validated, err := designdocument.ValidateArchive(archive, binding)
	if err != nil || validated.Manifest.ContentDigest != contentDigest {
		writeProjectDesignSystemError(w, http.StatusUnprocessableEntity, "design_document_invalid", "design document package does not match its task binding or digest")
		return
	}
	if h.Storage == nil {
		writeProjectDesignSystemError(w, http.StatusServiceUnavailable, "design_document_storage_unavailable", "design document package storage is unavailable")
		return
	}

	objectKey := designDocumentObjectKey(binding, contentDigest)
	digestHex := strings.TrimPrefix(contentDigest, "sha256:")
	filename := designDocumentUploadFilenameRoot + digestHex[:12] + ".zip"
	if _, err := h.Storage.Upload(r.Context(), objectKey, archive, nativePackageArchiveContentType, filename); err != nil {
		writeProjectDesignSystemError(w, http.StatusInternalServerError, "design_document_upload_failed", "failed to upload design document package")
		return
	}
	writeJSON(w, http.StatusOK, projectDesignSystemPackageUploadResponse{
		ObjectKey:     objectKey,
		ContentDigest: contentDigest,
	})
}
