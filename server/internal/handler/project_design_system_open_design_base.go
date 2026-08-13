package handler

import (
	"encoding/json"
	"errors"
	"net/http"
	"strconv"

	"github.com/go-chi/chi/v5"
	"github.com/jackc/pgx/v5"
	"github.com/multica-ai/multica/server/internal/opendesign"
	"github.com/multica-ai/multica/server/internal/service"
	"github.com/multica-ai/multica/server/internal/util"
	db "github.com/multica-ai/multica/server/pkg/db/generated"
)

func (h *Handler) DownloadOpenDesignBaseArchive(w http.ResponseWriter, r *http.Request) {
	task, workspaceID, ok := h.requireDaemonTaskAccessWithWorkspace(w, r, chi.URLParam(r, "taskId"))
	if !ok {
		return
	}
	var taskContext service.ProjectDesignSystemTaskContext
	if json.Unmarshal(task.Context, &taskContext) != nil ||
		taskContext.Type != service.ProjectDesignSystemTaskContextType ||
		(taskContext.Operation != service.ProjectDesignSystemAdjust && taskContext.Operation != service.ProjectDesignSystemRegenerate) ||
		taskContext.WorkspaceID != workspaceID ||
		taskContext.AgentID != uuidToString(task.AgentID) {
		writeOpenDesignBaseArchiveUnavailable(w)
		return
	}
	var reference opendesign.BasePackageReference
	if json.Unmarshal(taskContext.BasePackage, &reference) != nil || opendesign.ValidateBasePackageReference(reference) != nil {
		writeOpenDesignBaseArchiveUnavailable(w)
		return
	}
	systemID, err := util.ParseUUID(taskContext.ProjectDesignSystemID)
	if err != nil {
		writeOpenDesignBaseArchiveUnavailable(w)
		return
	}
	workspaceUUID, err := util.ParseUUID(workspaceID)
	if err != nil {
		writeOpenDesignBaseArchiveUnavailable(w)
		return
	}
	system, err := h.Queries.GetProjectDesignSystemInWorkspace(r.Context(), db.GetProjectDesignSystemInWorkspaceParams{
		ID: systemID, WorkspaceID: workspaceUUID,
	})
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			writeOpenDesignBaseArchiveUnavailable(w)
			return
		}
		writeProjectDesignSystemError(w, http.StatusInternalServerError, "open_design_base_archive_lookup_failed", "failed to load Open Design base archive")
		return
	}
	if taskContext.ProjectID != uuidToString(system.ProjectID) {
		writeOpenDesignBaseArchiveUnavailable(w)
		return
	}
	selected, err := h.Queries.GetProjectDesignSystemPackageBySlot(r.Context(), db.GetProjectDesignSystemPackageBySlotParams{
		DesignSystemID: system.ID,
		Slot:           reference.Slot,
		WorkspaceID:    system.WorkspaceID,
	})
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			writeOpenDesignBaseArchiveUnavailable(w)
			return
		}
		writeProjectDesignSystemError(w, http.StatusInternalServerError, "open_design_base_archive_lookup_failed", "failed to load Open Design base archive")
		return
	}
	if !selected.SourceTaskID.Valid || uuidToString(selected.SourceTaskID) != reference.SourceTaskID {
		writeOpenDesignBaseArchiveUnavailable(w)
		return
	}
	loaded, err := h.loadOpenDesignArchivePreviewPackage(r.Context(), h.Queries, system, selected)
	if err != nil {
		writeProjectDesignSystemRequestError(w, err)
		return
	}
	if loaded.Slot != reference.Slot || loaded.ContentDigest != reference.ContentDigest ||
		opendesign.ValidateProjectArchiveContentDigest(loaded.Archive, reference.ContentDigest) != nil {
		writeOpenDesignBaseArchiveUnavailable(w)
		return
	}

	w.Header().Set("Content-Type", opendesign.RunArchiveContentType)
	w.Header().Set("Content-Length", strconv.Itoa(len(loaded.Archive)))
	w.Header().Set("Cache-Control", "no-store")
	w.Header().Set("X-Content-Type-Options", "nosniff")
	w.Header().Set(opendesign.RunArchiveContentDigestHeader, reference.ContentDigest)
	w.Header().Set(opendesign.BasePackageSlotHeader, reference.Slot)
	w.Header().Set(opendesign.BasePackageSourceTaskIDHeader, reference.SourceTaskID)
	w.WriteHeader(http.StatusOK)
	_, _ = w.Write(loaded.Archive)
}

func writeOpenDesignBaseArchiveUnavailable(w http.ResponseWriter) {
	writeProjectDesignSystemError(w, http.StatusConflict, "open_design_base_archive_unavailable", "Open Design base archive is unavailable")
}
