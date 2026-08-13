package handler

import (
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"strconv"

	"github.com/go-chi/chi/v5"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgtype"
	"github.com/multica-ai/multica/server/internal/opendesign"
	db "github.com/multica-ai/multica/server/pkg/db/generated"
)

const openDesignEvidenceDigestHeader = "X-Open-Design-Evidence-Digest"

func (h *Handler) DownloadOpenDesignRunEvidence(w http.ResponseWriter, r *http.Request) {
	workspaceID, _, ok := h.projectDesignSystemRequestScope(w, r)
	if !ok {
		return
	}
	system, ok := h.loadProjectDesignSystemForRequest(w, r, workspaceID)
	if !ok {
		return
	}
	runID, ok := parseUUIDOrBadRequest(w, chi.URLParam(r, "runId"), "open_design_run_id")
	if !ok {
		return
	}
	run, err := h.Queries.GetOpenDesignRunForEvidence(r.Context(), db.GetOpenDesignRunForEvidenceParams{
		ID:             runID,
		DesignSystemID: system.ID,
		WorkspaceID:    workspaceID,
	})
	if errors.Is(err, pgx.ErrNoRows) {
		writeProjectDesignSystemError(w, http.StatusNotFound, "open_design_run_not_found", "Open Design run not found")
		return
	}
	if err != nil {
		writeProjectDesignSystemError(w, http.StatusInternalServerError, "open_design_evidence_load_failed", "failed to load Open Design run evidence")
		return
	}
	status := opendesign.RunStatus(run.Status)
	if !opendesign.IsRunEvidenceTerminalStatus(status) {
		writeProjectDesignSystemError(w, http.StatusConflict, "open_design_evidence_not_terminal", "Open Design run evidence is available only after the run reaches a terminal state")
		return
	}

	projectArchive, ok := h.readOpenDesignEvidenceProjectArchive(w, r, run)
	if !ok {
		return
	}
	artifactIndex := []opendesign.ArtifactIndexEntry{}
	if err := json.Unmarshal(run.ArtifactIndex, &artifactIndex); err != nil {
		writeProjectDesignSystemError(w, http.StatusInternalServerError, "open_design_evidence_invalid", "stored Open Design artifact evidence is invalid")
		return
	}
	archive, digest, err := opendesign.BuildRunEvidenceArchive(opendesign.RunEvidenceArchiveInput{
		Run: opendesign.RunEvidenceReference{
			SupervisorRunID: uuidToString(run.ID),
			WorkerRunID:     openDesignEvidenceText(run.OpenDesignRunID),
			TaskID:          uuidToString(run.TaskID),
			WorkspaceID:     uuidToString(run.WorkspaceID),
			ProjectID:       uuidToString(run.ProjectID),
			DesignSystemID:  uuidToString(run.DesignSystemID),
			Operation:       run.Operation,
			Status:          status,
			AdapterID:       run.AdapterID,
			Model:           openDesignEvidenceText(run.Model),
			CreatedAt:       timestampToString(run.CreatedAt),
			StartedAt:       timestampToString(run.StartedAt),
			FinishedAt:      timestampToString(run.FinishedAt),
			UpdatedAt:       timestampToString(run.UpdatedAt),
		},
		Engine: opendesign.EngineIdentity{
			Schema:         opendesign.EngineIdentitySchema,
			Release:        run.EngineRelease,
			Commit:         run.EngineCommit,
			LockfileSHA256: run.EngineLockfileSha256,
			DistSHA256:     run.EngineDistSha256,
		},
		AgentSnapshot:       run.AgentSnapshot,
		InputSnapshot:       run.InputSnapshot,
		WorkspaceProvenance: run.WorkspaceProvenance,
		Preflight:           run.Preflight,
		Events:              run.Events,
		ResultPackage:       run.ResultPackage,
		ArtifactIndex:       artifactIndex,
		ArchiveObjectKey:    openDesignEvidenceText(run.ArchiveObjectKey),
		ContentDigest:       openDesignEvidenceText(run.ContentDigest),
		AuditReport:         run.AuditReport,
		PreviewReceipt:      run.PreviewReceipt,
		Failure:             run.Failure,
		ProjectArchive:      projectArchive,
	})
	if err != nil {
		writeProjectDesignSystemError(w, http.StatusInternalServerError, "open_design_evidence_invalid", "stored Open Design run evidence is inconsistent")
		return
	}

	filename := fmt.Sprintf("open-design-evidence-%s.zip", uuidToString(run.ID))
	w.Header().Set("Content-Type", opendesign.RunArchiveContentType)
	w.Header().Set("Content-Disposition", fmt.Sprintf("attachment; filename=%q", filename))
	w.Header().Set("Content-Length", strconv.Itoa(len(archive)))
	w.Header().Set("Cache-Control", "no-store")
	w.Header().Set("X-Content-Type-Options", "nosniff")
	w.Header().Set(openDesignEvidenceDigestHeader, digest)
	w.WriteHeader(http.StatusOK)
	_, _ = w.Write(archive)
}

func (h *Handler) readOpenDesignEvidenceProjectArchive(w http.ResponseWriter, r *http.Request, run db.OpenDesignRun) ([]byte, bool) {
	hasObjectKey := run.ArchiveObjectKey.Valid && run.ArchiveObjectKey.String != ""
	hasContentDigest := run.ContentDigest.Valid && run.ContentDigest.String != ""
	if !hasObjectKey && !hasContentDigest {
		return nil, true
	}
	if !hasObjectKey || !hasContentDigest {
		writeProjectDesignSystemError(w, http.StatusInternalServerError, "open_design_evidence_invalid", "stored Open Design archive evidence is incomplete")
		return nil, false
	}
	if h.Storage == nil {
		writeProjectDesignSystemError(w, http.StatusServiceUnavailable, "open_design_evidence_storage_unavailable", "Open Design evidence storage is unavailable")
		return nil, false
	}
	reader, err := h.Storage.GetReader(r.Context(), run.ArchiveObjectKey.String)
	if err != nil {
		writeProjectDesignSystemError(w, http.StatusBadGateway, "open_design_evidence_archive_unavailable", "Open Design project archive is unavailable")
		return nil, false
	}
	archive, readErr := io.ReadAll(io.LimitReader(reader, opendesign.RunArchiveMaxBytes+1))
	closeErr := reader.Close()
	if readErr != nil || closeErr != nil || int64(len(archive)) > opendesign.RunArchiveMaxBytes {
		writeProjectDesignSystemError(w, http.StatusBadGateway, "open_design_evidence_archive_unavailable", "Open Design project archive is unavailable")
		return nil, false
	}
	return archive, true
}

func openDesignEvidenceText(value pgtype.Text) string {
	if !value.Valid {
		return ""
	}
	return value.String
}
