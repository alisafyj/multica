package handler

import (
	"context"
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"strings"

	"github.com/jackc/pgx/v5/pgtype"
	"github.com/multica-ai/multica/server/internal/opendesign"
	"github.com/multica-ai/multica/server/internal/service"
	db "github.com/multica-ai/multica/server/pkg/db/generated"
)

type preparedOpenDesignDraft struct {
	Artifacts     opendesign.DraftCompatibilityArtifacts
	Manifest      []byte
	Validation    []byte
	ArtifactIndex []byte
	AuditReport   []byte
	Instruction   pgtype.Text
	Scope         json.RawMessage
}

func (h *Handler) prepareOpenDesignDraft(
	ctx context.Context,
	task db.AgentTaskQueue,
	run db.OpenDesignRun,
	workerRunID string,
	preview opendesign.PreviewVerificationReceipt,
) (preparedOpenDesignDraft, error) {
	var taskContext service.ProjectDesignSystemTaskContext
	if err := json.Unmarshal(task.Context, &taskContext); err != nil || !openDesignDraftContextMatches(taskContext, task, run) {
		return preparedOpenDesignDraft{}, openDesignDraftConflict()
	}
	var artifactIndex []opendesign.ArtifactIndexEntry
	if err := json.Unmarshal(run.ArtifactIndex, &artifactIndex); err != nil {
		return preparedOpenDesignDraft{}, openDesignDraftConflict()
	}
	resultRequest := opendesign.RunResultRequest{
		OpenDesignRunID:  workerRunID,
		ResultPackage:    run.ResultPackage,
		ArtifactIndex:    artifactIndex,
		ArchiveObjectKey: run.ArchiveObjectKey.String,
		ContentDigest:    run.ContentDigest.String,
	}
	if !run.ArchiveObjectKey.Valid || !run.ContentDigest.Valid || opendesign.ValidateRunResultRequest(resultRequest, workerRunID) != nil {
		return preparedOpenDesignDraft{}, openDesignDraftConflict()
	}
	var audit opendesign.PackageAuditReceipt
	if err := json.Unmarshal(run.AuditReport, &audit); err != nil ||
		opendesign.ValidatePackageAuditReceipt(audit) != nil ||
		!audit.Audit.OK ||
		audit.ContentDigest != run.ContentDigest.String ||
		!openDesignAuditEngineMatches(run, audit.Engine) {
		return preparedOpenDesignDraft{}, &projectDesignSystemRequestError{
			status: http.StatusConflict, code: "open_design_preview_conflict", message: "Open Design Preview verification requires a passing package audit",
		}
	}
	if !preview.Verification.Passed || preview.ContentDigest != run.ContentDigest.String || !openDesignAuditEngineMatches(run, preview.Engine) {
		return preparedOpenDesignDraft{}, openDesignDraftConflict()
	}

	archive, err := h.readOpenDesignDraftArchive(ctx, run.ArchiveObjectKey.String)
	if err != nil {
		return preparedOpenDesignDraft{}, err
	}
	artifacts, err := opendesign.ExtractDraftCompatibilityArtifacts(archive, artifactIndex, run.ContentDigest.String)
	if err != nil {
		return preparedOpenDesignDraft{}, openDesignDraftConflict()
	}
	manifest, err := json.Marshal(opendesign.DraftPackageManifest{
		Schema: opendesign.DraftPackageManifestSchema,
		Format: opendesign.DraftPackageFormat,
		Engine: opendesign.EngineIdentity{
			Schema:         opendesign.EngineIdentitySchema,
			Release:        run.EngineRelease,
			Commit:         run.EngineCommit,
			LockfileSHA256: run.EngineLockfileSha256,
			DistSHA256:     run.EngineDistSha256,
		},
		Run: opendesign.DraftRunReference{
			SupervisorRunID: uuidToString(run.ID),
			WorkerRunID:     workerRunID,
			TaskID:          uuidToString(run.TaskID),
			DesignSystemID:  uuidToString(run.DesignSystemID),
			Operation:       run.Operation,
		},
		Archive: opendesign.DraftArchiveEvidence{
			ObjectKey:     run.ArchiveObjectKey.String,
			ContentDigest: run.ContentDigest.String,
			ArtifactIndex: artifactIndex,
		},
		ResultPackage:      run.ResultPackage,
		CompatibilityFiles: artifacts.Sources,
	})
	if err != nil {
		return preparedOpenDesignDraft{}, projectDesignSystemInternalError("open_design_draft_manifest_failed", "failed to encode Open Design draft evidence")
	}
	validation, err := json.Marshal(opendesign.DraftPackageValidation{
		Schema:  opendesign.DraftPackageValidationSchema,
		Passed:  true,
		Audit:   audit,
		Preview: preview,
	})
	if err != nil {
		return preparedOpenDesignDraft{}, projectDesignSystemInternalError("open_design_draft_validation_failed", "failed to encode Open Design draft validation")
	}
	instruction := strings.TrimSpace(taskContext.Instruction)
	return preparedOpenDesignDraft{
		Artifacts:     artifacts,
		Manifest:      manifest,
		Validation:    validation,
		ArtifactIndex: run.ArtifactIndex,
		AuditReport:   run.AuditReport,
		Instruction:   pgtype.Text{String: instruction, Valid: instruction != ""},
		Scope:         validJSONOr(taskContext.Scope, nil),
	}, nil
}

func (h *Handler) readOpenDesignDraftArchive(ctx context.Context, objectKey string) ([]byte, error) {
	if h.Storage == nil {
		return nil, &projectDesignSystemRequestError{
			status: http.StatusServiceUnavailable, code: "open_design_draft_archive_unavailable", message: "Open Design draft archive storage is unavailable",
		}
	}
	reader, err := h.Storage.GetReader(ctx, objectKey)
	if err != nil {
		return nil, &projectDesignSystemRequestError{
			status: http.StatusServiceUnavailable, code: "open_design_draft_archive_unavailable", message: "Open Design draft archive is unavailable",
		}
	}
	archive, readErr := io.ReadAll(io.LimitReader(reader, opendesign.RunArchiveMaxBytes+1))
	closeErr := reader.Close()
	if readErr != nil || closeErr != nil {
		return nil, &projectDesignSystemRequestError{
			status: http.StatusServiceUnavailable, code: "open_design_draft_archive_unavailable", message: "Open Design draft archive could not be read",
		}
	}
	if int64(len(archive)) > opendesign.RunArchiveMaxBytes {
		return nil, openDesignDraftConflict()
	}
	return archive, nil
}

func openDesignDraftContextMatches(taskContext service.ProjectDesignSystemTaskContext, task db.AgentTaskQueue, run db.OpenDesignRun) bool {
	return taskContext.Type == service.ProjectDesignSystemTaskContextType &&
		string(taskContext.Operation) == run.Operation &&
		taskContext.WorkspaceID == uuidToString(run.WorkspaceID) &&
		taskContext.ProjectID == uuidToString(run.ProjectID) &&
		taskContext.ProjectDesignSystemID == uuidToString(run.DesignSystemID) &&
		taskContext.AgentID == uuidToString(task.AgentID) &&
		run.AgentID.Valid && uuidToString(run.AgentID) == uuidToString(task.AgentID)
}

func openDesignDraftConflict() error {
	return &projectDesignSystemRequestError{
		status: http.StatusConflict, code: "open_design_draft_evidence_conflict", message: "Open Design draft evidence is incomplete or conflicting",
	}
}

func writeOpenDesignDraftPreparationError(w http.ResponseWriter, err error) {
	var requestErr *projectDesignSystemRequestError
	if errors.As(err, &requestErr) {
		writeProjectDesignSystemError(w, requestErr.status, requestErr.code, requestErr.message)
		return
	}
	writeProjectDesignSystemError(w, http.StatusInternalServerError, "open_design_draft_prepare_failed", "failed to prepare Open Design draft")
}
