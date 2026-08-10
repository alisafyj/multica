package handler

import (
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"mime"
	"net/http"
	"strings"

	"github.com/go-chi/chi/v5"
	"github.com/jackc/pgx/v5"

	"github.com/multica-ai/multica/server/internal/projectdesignsystem"
	"github.com/multica-ai/multica/server/internal/service"
	"github.com/multica-ai/multica/server/internal/util"
	db "github.com/multica-ai/multica/server/pkg/db/generated"
)

const (
	nativePackageArchiveContentType = "application/zip"
	nativePackageDigestHeader       = "X-Multica-Design-Package-Digest"
	nativePackageArchiveMaxBytes    = 64 << 20
	nativePackageObjectKeyRoot      = "project-design-systems"
	nativePackageUploadFilenameRoot = "native-design-package-"
)

type projectDesignSystemPackageUploadResponse struct {
	ObjectKey     string `json:"object_key"`
	ContentDigest string `json:"content_digest"`
}

func (h *Handler) UploadProjectDesignSystemPackage(w http.ResponseWriter, r *http.Request) {
	task, workspaceID, ok := h.requireDaemonTaskAccessWithWorkspace(w, r, chi.URLParam(r, "taskId"))
	if !ok {
		return
	}
	if task.Status != "running" {
		writeProjectDesignSystemError(w, http.StatusConflict, "native_package_task_not_running", "design system package upload requires a running task")
		return
	}
	binding, err := h.nativePackageBindingForTask(r, task, workspaceID)
	if err != nil {
		writeProjectDesignSystemError(w, http.StatusConflict, "native_package_task_invalid", "native design package task binding is invalid")
		return
	}

	rawDigest := r.Header.Get(nativePackageDigestHeader)
	contentDigest := strings.TrimSpace(rawDigest)
	if rawDigest != contentDigest || !validNativePackageDigest(contentDigest) {
		writeProjectDesignSystemError(w, http.StatusBadRequest, "native_package_digest_invalid", "invalid native design package digest")
		return
	}
	mediaType, _, err := mime.ParseMediaType(r.Header.Get("Content-Type"))
	if err != nil || mediaType != nativePackageArchiveContentType {
		writeProjectDesignSystemError(w, http.StatusUnsupportedMediaType, "native_package_media_type_invalid", "native design package must be an application/zip payload")
		return
	}
	if r.ContentLength > nativePackageArchiveMaxBytes {
		writeProjectDesignSystemError(w, http.StatusRequestEntityTooLarge, "native_package_too_large", "native design package exceeds the upload limit")
		return
	}
	r.Body = http.MaxBytesReader(w, r.Body, nativePackageArchiveMaxBytes)
	archive, err := io.ReadAll(r.Body)
	if err != nil {
		var maxBytesError *http.MaxBytesError
		if errors.As(err, &maxBytesError) {
			writeProjectDesignSystemError(w, http.StatusRequestEntityTooLarge, "native_package_too_large", "native design package exceeds the upload limit")
			return
		}
		writeProjectDesignSystemError(w, http.StatusBadRequest, "native_package_read_failed", "failed to read native design package")
		return
	}
	validated, err := projectdesignsystem.ValidateV2Archive(archive, binding)
	if err != nil || validated.Manifest.ContentDigest != contentDigest {
		writeProjectDesignSystemError(w, http.StatusUnprocessableEntity, "native_package_invalid", "native design package does not match its task binding or digest")
		return
	}
	if h.Storage == nil {
		writeProjectDesignSystemError(w, http.StatusServiceUnavailable, "native_package_storage_unavailable", "native design package storage is unavailable")
		return
	}

	digestHex := strings.TrimPrefix(contentDigest, "sha256:")
	objectKey := fmt.Sprintf(
		"%s/%s/%s/%s/%s.zip",
		nativePackageObjectKeyRoot,
		binding.WorkspaceID,
		binding.DesignSystemID,
		binding.TaskID,
		digestHex,
	)
	filename := nativePackageUploadFilenameRoot + digestHex[:12] + ".zip"
	if _, err := h.Storage.Upload(r.Context(), objectKey, archive, nativePackageArchiveContentType, filename); err != nil {
		writeProjectDesignSystemError(w, http.StatusInternalServerError, "native_package_upload_failed", "failed to upload native design package")
		return
	}
	writeJSON(w, http.StatusOK, projectDesignSystemPackageUploadResponse{
		ObjectKey:     objectKey,
		ContentDigest: contentDigest,
	})
}

func (h *Handler) nativePackageBindingForTask(r *http.Request, task db.AgentTaskQueue, workspaceID string) (projectdesignsystem.PackageBinding, error) {
	var taskContext service.ProjectDesignSystemTaskContext
	if err := json.Unmarshal(task.Context, &taskContext); err != nil || taskContext.Type != service.ProjectDesignSystemTaskContextType {
		return projectdesignsystem.PackageBinding{}, errors.New("invalid project design system task context")
	}
	if taskContext.Operation == service.ProjectDesignSystemRepositoryAnalysis || !validNativePackageOperation(taskContext.Operation) {
		return projectdesignsystem.PackageBinding{}, errors.New("task does not produce a native design package")
	}
	if taskContext.WorkspaceID != workspaceID || taskContext.AgentID != uuidToString(task.AgentID) {
		return projectdesignsystem.PackageBinding{}, errors.New("task context does not match daemon task ownership")
	}
	workspaceUUID, err := util.ParseUUID(workspaceID)
	if err != nil {
		return projectdesignsystem.PackageBinding{}, errors.New("invalid project design system workspace")
	}
	systemUUID, err := util.ParseUUID(taskContext.ProjectDesignSystemID)
	if err != nil {
		return projectdesignsystem.PackageBinding{}, errors.New("invalid project design system id")
	}
	system, err := h.Queries.GetProjectDesignSystemInWorkspace(r.Context(), db.GetProjectDesignSystemInWorkspaceParams{
		ID:          systemUUID,
		WorkspaceID: workspaceUUID,
	})
	if errors.Is(err, pgx.ErrNoRows) {
		return projectdesignsystem.PackageBinding{}, errors.New("project design system not found")
	}
	if err != nil {
		return projectdesignsystem.PackageBinding{}, fmt.Errorf("load project design system: %w", err)
	}
	if uuidToString(system.ProjectID) != taskContext.ProjectID ||
		!system.CurrentAgentID.Valid || uuidToString(system.CurrentAgentID) != taskContext.AgentID ||
		!system.ActiveTaskID.Valid || uuidToString(system.ActiveTaskID) != uuidToString(task.ID) ||
		!system.ActiveOperation.Valid || system.ActiveOperation.String != string(taskContext.Operation) {
		return projectdesignsystem.PackageBinding{}, errors.New("project design system active task binding changed")
	}
	inputDigest, err := projectdesignsystem.SnapshotDigest(system.InputSnapshot)
	if err != nil {
		return projectdesignsystem.PackageBinding{}, errors.New("invalid project design system input snapshot")
	}
	pinnedInputDigest, pinnedBaseDigest := nativePackagePinnedDigests(task.Context, taskContext.BasePackage)
	if pinnedInputDigest != "" && pinnedInputDigest != inputDigest {
		return projectdesignsystem.PackageBinding{}, errors.New("project design system input snapshot binding changed")
	}
	if taskContext.Operation != service.ProjectDesignSystemGenerate && !validNativePackageDigest(pinnedBaseDigest) {
		return projectdesignsystem.PackageBinding{}, errors.New("project design system base package digest is required")
	}
	if taskContext.Operation == service.ProjectDesignSystemGenerate {
		pinnedBaseDigest = ""
	}
	return projectdesignsystem.PackageBinding{
		WorkspaceID:         workspaceID,
		ProjectID:           taskContext.ProjectID,
		DesignSystemID:      taskContext.ProjectDesignSystemID,
		TaskID:              uuidToString(task.ID),
		AgentID:             taskContext.AgentID,
		Operation:           string(taskContext.Operation),
		InputSnapshotSHA256: inputDigest,
		BasePackageSHA256:   pinnedBaseDigest,
	}, nil
}

func nativePackagePinnedDigests(taskJSON, basePackageJSON json.RawMessage) (string, string) {
	var taskFields struct {
		InputSnapshotSHA256 string `json:"input_snapshot_sha256"`
		BasePackageSHA256   string `json:"base_package_sha256"`
	}
	_ = json.Unmarshal(taskJSON, &taskFields)
	if taskFields.BasePackageSHA256 != "" {
		return taskFields.InputSnapshotSHA256, taskFields.BasePackageSHA256
	}
	var basePackage struct {
		ContentDigest   string `json:"content_digest"`
		IntegritySHA256 string `json:"integrity_sha256"`
	}
	_ = json.Unmarshal(basePackageJSON, &basePackage)
	baseDigest := basePackage.ContentDigest
	if baseDigest == "" && len(basePackage.IntegritySHA256) == 64 {
		baseDigest = "sha256:" + basePackage.IntegritySHA256
	}
	return taskFields.InputSnapshotSHA256, baseDigest
}

func validNativePackageOperation(operation service.ProjectDesignSystemOperation) bool {
	switch operation {
	case service.ProjectDesignSystemGenerate, service.ProjectDesignSystemAdjust, service.ProjectDesignSystemRegenerate:
		return true
	default:
		return false
	}
}

func validNativePackageDigest(value string) bool {
	if len(value) != len("sha256:")+64 || !strings.HasPrefix(value, "sha256:") {
		return false
	}
	raw := strings.TrimPrefix(value, "sha256:")
	decoded, err := hex.DecodeString(raw)
	return err == nil && hex.EncodeToString(decoded) == raw
}
