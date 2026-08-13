package handler

import (
	"encoding/json"
	"errors"
	"io"
	"mime"
	"net/http"
	"path"
	"reflect"
	"strings"

	"github.com/go-chi/chi/v5"
	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgtype"
	"github.com/multica-ai/multica/server/internal/opendesign"
	"github.com/multica-ai/multica/server/internal/service"
	db "github.com/multica-ai/multica/server/pkg/db/generated"
)

const (
	openDesignRunCallbackMaxBytes     int64 = 64 << 10
	openDesignEventCallbackMaxBytes   int64 = 1 << 20
	openDesignResultCallbackMaxBytes  int64 = 2 << 20
	openDesignAuditCallbackMaxBytes   int64 = 2 << 20
	openDesignPreviewCallbackMaxBytes int64 = 2 << 20
	openDesignEventNameMaxBytes             = 128
	openDesignFailureCodeMaxBytes           = 128
	openDesignFailureMessageMaxBytes        = 4 << 10
)

func (h *Handler) StartOpenDesignRun(w http.ResponseWriter, r *http.Request) {
	task, _, ok := h.loadOpenDesignRunForDaemonCallback(w, r)
	if !ok {
		return
	}
	var req opendesign.RunStartRequest
	if !decodeOpenDesignCallback(w, r, openDesignRunCallbackMaxBytes, &req, "open_design_start_invalid") {
		return
	}
	runID, ok := validateOpenDesignRunID(req.OpenDesignRunID)
	if !ok {
		writeProjectDesignSystemError(w, http.StatusBadRequest, "open_design_start_invalid", "open_design_run_id must be a canonical UUID")
		return
	}

	updated, err := h.Queries.StartOpenDesignRun(r.Context(), db.StartOpenDesignRunParams{
		OpenDesignRunID: pgtype.Text{String: runID, Valid: true},
		TaskID:          task.ID,
	})
	if errors.Is(err, pgx.ErrNoRows) {
		current, loadErr := h.Queries.GetOpenDesignRunByTask(r.Context(), task.ID)
		if loadErr == nil && openDesignRunIDMatches(current, runID) && current.StartedAt.Valid {
			writeOpenDesignRunCallbackResponse(w, current)
			return
		}
		writeProjectDesignSystemError(w, http.StatusConflict, "open_design_start_conflict", "Open Design run cannot be started from its current state")
		return
	}
	if err != nil {
		writeProjectDesignSystemError(w, http.StatusInternalServerError, "open_design_start_persist_failed", "failed to persist Open Design run start")
		return
	}
	writeOpenDesignRunCallbackResponse(w, updated)
}

func (h *Handler) RecordOpenDesignRunEvent(w http.ResponseWriter, r *http.Request) {
	task, _, ok := h.loadOpenDesignRunForDaemonCallback(w, r)
	if !ok {
		return
	}
	var req opendesign.RunEventRequest
	if !decodeOpenDesignCallback(w, r, openDesignEventCallbackMaxBytes, &req, "open_design_event_invalid") {
		return
	}
	runID, validRunID := validateOpenDesignRunID(req.OpenDesignRunID)
	req.Event.Event = strings.TrimSpace(req.Event.Event)
	if !validRunID || req.Event.ID <= 0 || req.Event.Event == "" || len(req.Event.Event) > openDesignEventNameMaxBytes || len(req.Event.Data) == 0 || !json.Valid(req.Event.Data) {
		writeProjectDesignSystemError(w, http.StatusBadRequest, "open_design_event_invalid", "invalid Open Design run event")
		return
	}
	eventJSON, err := json.Marshal(req.Event)
	if err != nil {
		writeProjectDesignSystemError(w, http.StatusBadRequest, "open_design_event_invalid", "invalid Open Design run event")
		return
	}

	updated, err := h.Queries.AppendOpenDesignRunEvent(r.Context(), db.AppendOpenDesignRunEventParams{
		EventID:         req.Event.ID,
		Event:           eventJSON,
		TaskID:          task.ID,
		OpenDesignRunID: pgtype.Text{String: runID, Valid: true},
	})
	if errors.Is(err, pgx.ErrNoRows) {
		current, loadErr := h.Queries.GetOpenDesignRunByTask(r.Context(), task.ID)
		if loadErr == nil && openDesignRunIDMatches(current, runID) && persistedOpenDesignEventMatches(current.Events, req.Event.ID, eventJSON) {
			writeOpenDesignRunCallbackResponse(w, current)
			return
		}
		writeProjectDesignSystemError(w, http.StatusConflict, "open_design_event_conflict", "Open Design run event conflicts with persisted lifecycle evidence")
		return
	}
	if err != nil {
		writeProjectDesignSystemError(w, http.StatusInternalServerError, "open_design_event_persist_failed", "failed to persist Open Design run event")
		return
	}
	writeOpenDesignRunCallbackResponse(w, updated)
}

func (h *Handler) UploadOpenDesignRunArchive(w http.ResponseWriter, r *http.Request) {
	task, run, ok := h.loadOpenDesignRunForDaemonCallback(w, r)
	if !ok {
		return
	}
	rawRunID := r.Header.Get(opendesign.RunArchiveRunIDHeader)
	runID, validRunID := validateOpenDesignRunID(rawRunID)
	rawContentDigest := r.Header.Get(opendesign.RunArchiveContentDigestHeader)
	contentDigest := strings.TrimSpace(rawContentDigest)
	if !validRunID || rawRunID != runID || rawContentDigest != contentDigest || opendesign.ValidateContentDigest(contentDigest) != nil {
		writeProjectDesignSystemError(w, http.StatusBadRequest, "open_design_archive_invalid", "invalid Open Design archive metadata")
		return
	}
	if !openDesignRunIDMatches(run, runID) {
		writeProjectDesignSystemError(w, http.StatusConflict, "open_design_archive_conflict", "Open Design archive does not match the active run")
		return
	}
	archiveObjectKey := openDesignArchiveObjectKey(run, contentDigest)
	if run.ArchiveObjectKey.Valid {
		if run.ArchiveObjectKey.String == archiveObjectKey && run.ContentDigest.Valid && run.ContentDigest.String == contentDigest {
			writeJSON(w, http.StatusOK, opendesign.RunArchiveResponse{ArchiveObjectKey: archiveObjectKey})
			return
		}
		writeProjectDesignSystemError(w, http.StatusConflict, "open_design_archive_conflict", "Open Design run already references a different archive")
		return
	}
	if run.Status != string(opendesign.RunStatusRunning) {
		writeProjectDesignSystemError(w, http.StatusConflict, "open_design_archive_conflict", "Open Design archive cannot be uploaded from the current run state")
		return
	}
	mediaType, _, err := mime.ParseMediaType(r.Header.Get("Content-Type"))
	if err != nil || mediaType != opendesign.RunArchiveContentType {
		writeProjectDesignSystemError(w, http.StatusUnsupportedMediaType, "open_design_archive_invalid", "Open Design archive must be an application/zip payload")
		return
	}
	r.Body = http.MaxBytesReader(w, r.Body, opendesign.RunArchiveMaxBytes)
	archive, err := io.ReadAll(r.Body)
	if err != nil {
		var maxBytesError *http.MaxBytesError
		if errors.As(err, &maxBytesError) {
			writeProjectDesignSystemError(w, http.StatusRequestEntityTooLarge, "open_design_archive_too_large", "Open Design archive exceeds the upload limit")
			return
		}
		writeProjectDesignSystemError(w, http.StatusBadRequest, "open_design_archive_invalid", "failed to read Open Design archive")
		return
	}
	if err := opendesign.ValidateProjectArchiveContentDigest(archive, contentDigest); err != nil {
		writeProjectDesignSystemError(w, http.StatusBadRequest, "open_design_archive_invalid", "Open Design archive does not match its content digest")
		return
	}
	if h.Storage == nil {
		writeProjectDesignSystemError(w, http.StatusServiceUnavailable, "open_design_archive_storage_unavailable", "Open Design archive storage is unavailable")
		return
	}
	digestHex := strings.TrimPrefix(contentDigest, "sha256:")
	filename := "open-design-package-" + digestHex[:12] + ".zip"
	if _, err := h.Storage.Upload(r.Context(), archiveObjectKey, archive, opendesign.RunArchiveContentType, filename); err != nil {
		writeProjectDesignSystemError(w, http.StatusInternalServerError, "open_design_archive_upload_failed", "failed to upload Open Design archive")
		return
	}

	updated, err := h.Queries.RecordOpenDesignRunArchive(r.Context(), db.RecordOpenDesignRunArchiveParams{
		ArchiveObjectKey: pgtype.Text{String: archiveObjectKey, Valid: true},
		ContentDigest:    pgtype.Text{String: contentDigest, Valid: true},
		TaskID:           task.ID,
		OpenDesignRunID:  pgtype.Text{String: runID, Valid: true},
	})
	if errors.Is(err, pgx.ErrNoRows) {
		current, loadErr := h.Queries.GetOpenDesignRunByTask(r.Context(), task.ID)
		if loadErr == nil && openDesignRunIDMatches(current, runID) && current.ArchiveObjectKey.Valid && current.ArchiveObjectKey.String == archiveObjectKey && current.ContentDigest.Valid && current.ContentDigest.String == contentDigest {
			writeJSON(w, http.StatusOK, opendesign.RunArchiveResponse{ArchiveObjectKey: archiveObjectKey})
			return
		}
		if loadErr == nil && current.ArchiveObjectKey.Valid && current.ArchiveObjectKey.String != archiveObjectKey {
			h.Storage.Delete(r.Context(), archiveObjectKey)
		}
		writeProjectDesignSystemError(w, http.StatusConflict, "open_design_archive_conflict", "Open Design archive conflicts with persisted lifecycle evidence")
		return
	}
	if err != nil {
		writeProjectDesignSystemError(w, http.StatusInternalServerError, "open_design_archive_persist_failed", "failed to persist Open Design archive")
		return
	}
	writeJSON(w, http.StatusOK, opendesign.RunArchiveResponse{ArchiveObjectKey: updated.ArchiveObjectKey.String})
}

func (h *Handler) RecordOpenDesignRunResult(w http.ResponseWriter, r *http.Request) {
	task, run, ok := h.loadOpenDesignRunForDaemonCallback(w, r)
	if !ok {
		return
	}
	var req opendesign.RunResultRequest
	if !decodeOpenDesignCallback(w, r, openDesignResultCallbackMaxBytes, &req, "open_design_result_invalid") {
		return
	}
	runID, validRunID := validateOpenDesignRunID(req.OpenDesignRunID)
	if !validRunID || opendesign.ValidateRunResultRequest(req, runID) != nil {
		writeProjectDesignSystemError(w, http.StatusBadRequest, "open_design_result_invalid", "invalid Open Design result package")
		return
	}
	if !run.ArchiveObjectKey.Valid || run.ArchiveObjectKey.String != req.ArchiveObjectKey || !run.ContentDigest.Valid || run.ContentDigest.String != req.ContentDigest {
		writeProjectDesignSystemError(w, http.StatusConflict, "open_design_result_conflict", "Open Design result does not match the persisted archive")
		return
	}
	artifactIndex, err := json.Marshal(req.ArtifactIndex)
	if err != nil {
		writeProjectDesignSystemError(w, http.StatusBadRequest, "open_design_result_invalid", "invalid Open Design artifact index")
		return
	}

	updated, err := h.Queries.MarkOpenDesignRunSucceeded(r.Context(), db.MarkOpenDesignRunSucceededParams{
		ResultPackage:    req.ResultPackage,
		ArtifactIndex:    artifactIndex,
		ArchiveObjectKey: pgtype.Text{String: req.ArchiveObjectKey, Valid: true},
		ContentDigest:    pgtype.Text{String: req.ContentDigest, Valid: true},
		TaskID:           task.ID,
		OpenDesignRunID:  pgtype.Text{String: runID, Valid: true},
	})
	if errors.Is(err, pgx.ErrNoRows) {
		current, loadErr := h.Queries.GetOpenDesignRunByTask(r.Context(), task.ID)
		if loadErr == nil && openDesignRunIDMatches(current, runID) &&
			jsonValuesEqual(current.ResultPackage, req.ResultPackage) &&
			jsonValuesEqual(current.ArtifactIndex, artifactIndex) &&
			current.ArchiveObjectKey.Valid && current.ArchiveObjectKey.String == req.ArchiveObjectKey &&
			current.ContentDigest.Valid && current.ContentDigest.String == req.ContentDigest &&
			current.Status != string(opendesign.RunStatusRunning) {
			writeOpenDesignRunCallbackResponse(w, current)
			return
		}
		writeProjectDesignSystemError(w, http.StatusConflict, "open_design_result_conflict", "Open Design result conflicts with persisted lifecycle evidence")
		return
	}
	if err != nil {
		writeProjectDesignSystemError(w, http.StatusInternalServerError, "open_design_result_persist_failed", "failed to persist Open Design result package")
		return
	}
	writeOpenDesignRunCallbackResponse(w, updated)
}

func (h *Handler) RecordOpenDesignRunAudit(w http.ResponseWriter, r *http.Request) {
	task, run, ok := h.loadOpenDesignRunForDaemonCallback(w, r)
	if !ok {
		return
	}
	var req opendesign.RunAuditRequest
	if !decodeOpenDesignCallback(w, r, openDesignAuditCallbackMaxBytes, &req, "open_design_audit_invalid") {
		return
	}
	runID, validRunID := validateOpenDesignRunID(req.OpenDesignRunID)
	if !validRunID || opendesign.ValidatePackageAuditReceipt(req.AuditReport) != nil {
		writeProjectDesignSystemError(w, http.StatusBadRequest, "open_design_audit_invalid", "invalid Open Design package audit report")
		return
	}
	if !openDesignRunIDMatches(run, runID) || !run.ContentDigest.Valid || run.ContentDigest.String != req.AuditReport.ContentDigest || !openDesignAuditEngineMatches(run, req.AuditReport.Engine) {
		writeProjectDesignSystemError(w, http.StatusConflict, "open_design_audit_conflict", "Open Design package audit does not match the persisted run result")
		return
	}
	auditReport, err := json.Marshal(req.AuditReport)
	if err != nil {
		writeProjectDesignSystemError(w, http.StatusBadRequest, "open_design_audit_invalid", "invalid Open Design package audit report")
		return
	}
	failure := opendesign.PackageAuditFailure(req.AuditReport.Audit)
	updated, err := h.Queries.RecordOpenDesignRunAudit(r.Context(), db.RecordOpenDesignRunAuditParams{
		AuditReport:     auditReport,
		Failure:         failure,
		ContentDigest:   pgtype.Text{String: req.AuditReport.ContentDigest, Valid: true},
		TaskID:          task.ID,
		OpenDesignRunID: pgtype.Text{String: runID, Valid: true},
	})
	if errors.Is(err, pgx.ErrNoRows) {
		current, loadErr := h.Queries.GetOpenDesignRunByTask(r.Context(), task.ID)
		expectedStatus := string(opendesign.RunStatusRunSucceeded)
		if !req.AuditReport.Audit.OK {
			expectedStatus = string(opendesign.RunStatusAuditFailed)
		}
		if loadErr == nil && openDesignRunIDMatches(current, runID) && current.Status == expectedStatus &&
			current.ContentDigest.Valid && current.ContentDigest.String == req.AuditReport.ContentDigest &&
			jsonValuesEqual(current.AuditReport, auditReport) && jsonValuesEqual(current.Failure, failure) {
			writeOpenDesignRunCallbackResponse(w, current)
			return
		}
		writeProjectDesignSystemError(w, http.StatusConflict, "open_design_audit_conflict", "Open Design package audit conflicts with persisted lifecycle evidence")
		return
	}
	if err != nil {
		writeProjectDesignSystemError(w, http.StatusInternalServerError, "open_design_audit_persist_failed", "failed to persist Open Design package audit")
		return
	}
	writeOpenDesignRunCallbackResponse(w, updated)
}

func (h *Handler) RecordOpenDesignRunPreview(w http.ResponseWriter, r *http.Request) {
	task, run, ok := h.loadOpenDesignRunForDaemonCallback(w, r)
	if !ok {
		return
	}
	var req opendesign.RunPreviewRequest
	if !decodeOpenDesignCallback(w, r, openDesignPreviewCallbackMaxBytes, &req, "open_design_preview_invalid") {
		return
	}
	runID, validRunID := validateOpenDesignRunID(req.OpenDesignRunID)
	if !validRunID || opendesign.ValidatePreviewVerificationReceipt(req.PreviewReceipt) != nil {
		writeProjectDesignSystemError(w, http.StatusBadRequest, "open_design_preview_invalid", "invalid Open Design Preview verification receipt")
		return
	}
	if !openDesignRunIDMatches(run, runID) || !run.ContentDigest.Valid || run.ContentDigest.String != req.PreviewReceipt.ContentDigest || !openDesignAuditEngineMatches(run, req.PreviewReceipt.Engine) {
		writeProjectDesignSystemError(w, http.StatusConflict, "open_design_preview_conflict", "Open Design Preview verification does not match the persisted run result")
		return
	}
	previewReceipt, err := json.Marshal(req.PreviewReceipt)
	if err != nil {
		writeProjectDesignSystemError(w, http.StatusBadRequest, "open_design_preview_invalid", "invalid Open Design Preview verification receipt")
		return
	}
	failure := opendesign.PreviewVerificationFailure(req.PreviewReceipt.Verification)
	expectedStatus := string(opendesign.RunStatusPreviewFailed)
	if req.PreviewReceipt.Verification.Passed {
		expectedStatus = string(opendesign.RunStatusSucceeded)
	}
	if len(run.PreviewReceipt) > 0 {
		if persistedOpenDesignPreviewMatches(run, runID, expectedStatus, previewReceipt, failure) {
			writeOpenDesignRunCallbackResponse(w, run)
			return
		}
		writeProjectDesignSystemError(w, http.StatusConflict, "open_design_preview_conflict", "Open Design Preview verification conflicts with persisted lifecycle evidence")
		return
	}

	var updated db.OpenDesignRun
	persist := func(qtx *db.Queries) error {
		var persistErr error
		updated, persistErr = qtx.RecordOpenDesignRunPreview(r.Context(), db.RecordOpenDesignRunPreviewParams{
			PreviewReceipt:  previewReceipt,
			Failure:         failure,
			TaskID:          task.ID,
			OpenDesignRunID: pgtype.Text{String: runID, Valid: true},
			ContentDigest:   pgtype.Text{String: req.PreviewReceipt.ContentDigest, Valid: true},
		})
		return persistErr
	}
	if req.PreviewReceipt.Verification.Passed {
		preparedDraft, prepareErr := h.prepareOpenDesignDraft(r.Context(), task, run, runID, req.PreviewReceipt)
		if prepareErr != nil {
			writeOpenDesignDraftPreparationError(w, prepareErr)
			return
		}
		result, marshalErr := json.Marshal(TaskCompleteRequest{Output: "Open Design package passed Preview verification"})
		if marshalErr != nil {
			writeProjectDesignSystemError(w, http.StatusInternalServerError, "open_design_preview_persist_failed", "failed to persist Open Design Preview verification")
			return
		}
		_, err = h.TaskService.CompleteTaskWithMutation(r.Context(), task.ID, result, "", "", func(qtx *db.Queries, _ db.AgentTaskQueue) error {
			_, persistErr := qtx.PersistOpenDesignRunDraft(r.Context(), db.PersistOpenDesignRunDraftParams{
				TaskID:           task.ID,
				OpenDesignRunID:  pgtype.Text{String: runID, Valid: true},
				ResultPackage:    run.ResultPackage,
				ArtifactIndex:    preparedDraft.ArtifactIndex,
				ArchiveObjectKey: run.ArchiveObjectKey,
				ContentDigest:    run.ContentDigest,
				AuditReport:      preparedDraft.AuditReport,
				PreviewReceipt:   previewReceipt,
				DesignMd:         preparedDraft.Artifacts.DesignMD,
				TokensCss:        preparedDraft.Artifacts.TokensCSS,
				ComponentsHtml:   preparedDraft.Artifacts.ComponentsHTML,
				Manifest:         preparedDraft.Manifest,
				Validation:       preparedDraft.Validation,
				IntegritySha256:  strings.TrimPrefix(run.ContentDigest.String, "sha256:"),
				Instruction:      preparedDraft.Instruction,
				Scope:            preparedDraft.Scope,
			})
			return persistErr
		})
		if err == nil {
			updated, err = h.Queries.GetOpenDesignRunByTask(r.Context(), task.ID)
		}
	} else {
		err = persist(h.Queries)
	}
	if err != nil || !updated.ID.Valid {
		current, loadErr := h.Queries.GetOpenDesignRunByTask(r.Context(), task.ID)
		if loadErr == nil && persistedOpenDesignPreviewMatches(current, runID, expectedStatus, previewReceipt, failure) {
			writeOpenDesignRunCallbackResponse(w, current)
			return
		}
		if errors.Is(err, pgx.ErrNoRows) || err == nil {
			writeProjectDesignSystemError(w, http.StatusConflict, "open_design_preview_conflict", "Open Design Preview verification conflicts with persisted lifecycle evidence")
			return
		}
		writeProjectDesignSystemError(w, http.StatusInternalServerError, "open_design_preview_persist_failed", "failed to persist Open Design Preview verification")
		return
	}
	writeOpenDesignRunCallbackResponse(w, updated)
}

func persistedOpenDesignPreviewMatches(run db.OpenDesignRun, runID, expectedStatus string, previewReceipt, failure []byte) bool {
	return openDesignRunIDMatches(run, runID) &&
		run.Status == expectedStatus &&
		run.FinishedAt.Valid &&
		jsonValuesEqual(run.PreviewReceipt, previewReceipt) &&
		jsonValuesEqual(run.Failure, failure)
}

func openDesignAuditEngineMatches(run db.OpenDesignRun, engine opendesign.EngineIdentity) bool {
	return run.EngineRelease == engine.Release &&
		run.EngineCommit == engine.Commit &&
		run.EngineLockfileSha256 == engine.LockfileSHA256 &&
		run.EngineDistSha256 == engine.DistSHA256
}

func openDesignArchiveObjectKey(run db.OpenDesignRun, contentDigest string) string {
	return path.Join(
		"workspaces", uuidToString(run.WorkspaceID),
		"design-systems", uuidToString(run.DesignSystemID),
		"open-design-runs", uuidToString(run.TaskID),
		strings.TrimPrefix(contentDigest, "sha256:")+".zip",
	)
}

func (h *Handler) FinalizeOpenDesignRun(w http.ResponseWriter, r *http.Request) {
	task, run, ok := h.loadOpenDesignRunForDaemonCallback(w, r)
	if !ok {
		return
	}
	var req opendesign.RunTerminalRequest
	if !decodeOpenDesignCallback(w, r, openDesignRunCallbackMaxBytes, &req, "open_design_terminal_invalid") {
		return
	}
	req.OpenDesignRunID = strings.TrimSpace(req.OpenDesignRunID)
	if !opendesign.IsSupervisorTerminalRunStatus(req.Status) || !validOpenDesignFailure(req.Failure) {
		writeProjectDesignSystemError(w, http.StatusBadRequest, "open_design_terminal_invalid", "invalid Open Design terminal callback")
		return
	}
	if !validTerminalOpenDesignRunID(run, req.OpenDesignRunID, req.Status) {
		writeProjectDesignSystemError(w, http.StatusConflict, "open_design_terminal_conflict", "Open Design terminal callback does not match the active run")
		return
	}

	updated, err := h.Queries.FinalizeOpenDesignRun(r.Context(), db.FinalizeOpenDesignRunParams{
		Status:  string(req.Status),
		Failure: req.Failure,
		TaskID:  task.ID,
	})
	if errors.Is(err, pgx.ErrNoRows) {
		current, loadErr := h.Queries.GetOpenDesignRunByTask(r.Context(), task.ID)
		if loadErr == nil && current.Status == string(req.Status) && jsonValuesEqual(current.Failure, req.Failure) {
			writeOpenDesignRunCallbackResponse(w, current)
			return
		}
		writeProjectDesignSystemError(w, http.StatusConflict, "open_design_terminal_conflict", "Open Design run already has a different terminal state")
		return
	}
	if err != nil {
		writeProjectDesignSystemError(w, http.StatusInternalServerError, "open_design_terminal_persist_failed", "failed to persist Open Design terminal state")
		return
	}
	writeOpenDesignRunCallbackResponse(w, updated)
}

func (h *Handler) loadOpenDesignRunForDaemonCallback(w http.ResponseWriter, r *http.Request) (db.AgentTaskQueue, db.OpenDesignRun, bool) {
	taskID := chi.URLParam(r, "taskId")
	task, _, ok := h.requireDaemonTaskAccessWithWorkspace(w, r, taskID)
	if !ok {
		return db.AgentTaskQueue{}, db.OpenDesignRun{}, false
	}
	var taskContext service.ProjectDesignSystemTaskContext
	if err := json.Unmarshal(task.Context, &taskContext); err != nil || len(taskContext.OpenDesignRun) == 0 {
		writeProjectDesignSystemError(w, http.StatusNotFound, "open_design_run_not_found", "Open Design run not found")
		return db.AgentTaskQueue{}, db.OpenDesignRun{}, false
	}
	var expectedContext opendesign.TaskRunContext
	if err := json.Unmarshal(taskContext.OpenDesignRun, &expectedContext); err != nil {
		writeProjectDesignSystemError(w, http.StatusConflict, "open_design_context_invalid", "Open Design run context is invalid")
		return db.AgentTaskQueue{}, db.OpenDesignRun{}, false
	}
	run, err := h.Queries.GetOpenDesignRunByTask(r.Context(), task.ID)
	if errors.Is(err, pgx.ErrNoRows) {
		writeProjectDesignSystemError(w, http.StatusNotFound, "open_design_run_not_found", "Open Design run not found")
		return db.AgentTaskQueue{}, db.OpenDesignRun{}, false
	}
	if err != nil {
		writeProjectDesignSystemError(w, http.StatusInternalServerError, "open_design_run_lookup_failed", "failed to load Open Design run")
		return db.AgentTaskQueue{}, db.OpenDesignRun{}, false
	}
	if expectedContext.RunID != uuidToString(run.ID) {
		writeProjectDesignSystemError(w, http.StatusConflict, "open_design_run_mismatch", "Open Design run does not match task")
		return db.AgentTaskQueue{}, db.OpenDesignRun{}, false
	}
	return task, run, true
}

func decodeOpenDesignCallback(w http.ResponseWriter, r *http.Request, maxBytes int64, destination any, code string) bool {
	r.Body = http.MaxBytesReader(w, r.Body, maxBytes)
	decoder := json.NewDecoder(r.Body)
	decoder.DisallowUnknownFields()
	if err := decoder.Decode(destination); err != nil {
		var maxBytesError *http.MaxBytesError
		if errors.As(err, &maxBytesError) {
			writeProjectDesignSystemError(w, http.StatusRequestEntityTooLarge, code, "Open Design callback is too large")
			return false
		}
		writeProjectDesignSystemError(w, http.StatusBadRequest, code, "invalid Open Design callback")
		return false
	}
	if err := decoder.Decode(&struct{}{}); !errors.Is(err, io.EOF) {
		writeProjectDesignSystemError(w, http.StatusBadRequest, code, "invalid Open Design callback")
		return false
	}
	return true
}

func validateOpenDesignRunID(value string) (string, bool) {
	value = strings.TrimSpace(value)
	parsed, err := uuid.Parse(value)
	return value, err == nil && parsed.String() == value
}

func validOpenDesignFailure(raw json.RawMessage) bool {
	var failure struct {
		Code    string `json:"code"`
		Message string `json:"message"`
	}
	if len(raw) == 0 || json.Unmarshal(raw, &failure) != nil {
		return false
	}
	failure.Code = strings.TrimSpace(failure.Code)
	return failure.Code != "" && len(failure.Code) <= openDesignFailureCodeMaxBytes && len(failure.Message) <= openDesignFailureMessageMaxBytes
}

func validTerminalOpenDesignRunID(run db.OpenDesignRun, rawRunID string, status opendesign.RunStatus) bool {
	if run.OpenDesignRunID.Valid {
		runID, ok := validateOpenDesignRunID(rawRunID)
		return ok && openDesignRunIDMatches(run, runID)
	}
	return rawRunID == "" && (status == opendesign.RunStatusCanceled || status == opendesign.RunStatusAgentFailed)
}

func openDesignRunIDMatches(run db.OpenDesignRun, runID string) bool {
	return run.OpenDesignRunID.Valid && run.OpenDesignRunID.String == runID
}

func persistedOpenDesignEventMatches(eventsJSON []byte, eventID int64, expected []byte) bool {
	var events []struct {
		ID int64 `json:"id"`
	}
	var rawEvents []json.RawMessage
	if json.Unmarshal(eventsJSON, &rawEvents) != nil || json.Unmarshal(eventsJSON, &events) != nil {
		return false
	}
	for index, event := range events {
		if event.ID == eventID {
			return jsonValuesEqual(rawEvents[index], expected)
		}
	}
	return false
}

func jsonValuesEqual(left, right []byte) bool {
	var leftValue any
	var rightValue any
	return json.Unmarshal(left, &leftValue) == nil &&
		json.Unmarshal(right, &rightValue) == nil &&
		reflect.DeepEqual(leftValue, rightValue)
}

func writeOpenDesignRunCallbackResponse(w http.ResponseWriter, run db.OpenDesignRun) {
	response := map[string]any{
		"id":     uuidToString(run.ID),
		"status": run.Status,
	}
	if run.OpenDesignRunID.Valid {
		response["open_design_run_id"] = run.OpenDesignRunID.String
	}
	writeJSON(w, http.StatusOK, response)
}
