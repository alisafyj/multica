package handler

import (
	"context"
	"crypto/hmac"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"net/http"
	"path"
	"strconv"
	"strings"
	"time"

	"github.com/go-chi/chi/v5"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgtype"
	"github.com/multica-ai/multica/server/internal/auth"
	"github.com/multica-ai/multica/server/internal/opendesign"
	db "github.com/multica-ai/multica/server/pkg/db/generated"
)

const (
	openDesignArchivePreviewSchema              = "multica.open-design-archive-preview/v1"
	openDesignArchivePreviewAccessTokenVersion  = "v1"
	openDesignArchivePreviewAccessTokenLifetime = 30 * time.Minute
	openDesignArchivePreviewCSP                 = "default-src 'self' data: blob:; script-src 'self' 'unsafe-inline'; style-src 'self' 'unsafe-inline'; img-src 'self' data: blob:; font-src 'self' data:; connect-src 'none'; object-src 'none'; base-uri 'none'; form-action 'none'; frame-ancestors 'self'; sandbox allow-scripts"
)

type openDesignArchivePreviewResponse struct {
	Schema                  string                     `json:"schema"`
	Slot                    string                     `json:"slot"`
	ContentDigest           string                     `json:"content_digest"`
	ResourceAccessToken     string                     `json:"resource_access_token"`
	ResourceAccessExpiresAt string                     `json:"resource_access_expires_at"`
	Targets                 []opendesign.PreviewTarget `json:"targets"`
}

type loadedOpenDesignArchivePreview struct {
	Slot          string
	ContentDigest string
	ArtifactIndex []opendesign.ArtifactIndexEntry
	Archive       []byte
}

func (h *Handler) GetProjectDesignSystemArchivePreview(w http.ResponseWriter, r *http.Request) {
	workspaceID, _, ok := h.projectDesignSystemRequestScope(w, r)
	if !ok {
		return
	}
	system, ok := h.loadProjectDesignSystemForRequest(w, r, workspaceID)
	if !ok {
		return
	}
	loaded, err := h.loadOpenDesignArchivePreview(r.Context(), system)
	if err != nil {
		writeProjectDesignSystemRequestError(w, err)
		return
	}
	if _, err := opendesign.ExtractDraftCompatibilityArtifacts(loaded.Archive, loaded.ArtifactIndex, loaded.ContentDigest); err != nil {
		writeProjectDesignSystemError(w, http.StatusConflict, "open_design_preview_evidence_conflict", "Open Design archive preview evidence is inconsistent")
		return
	}
	targets, err := opendesign.DiscoverPreviewTargets(loaded.Archive)
	if err != nil {
		writeProjectDesignSystemError(w, http.StatusConflict, "open_design_preview_evidence_conflict", "Open Design archive preview targets are invalid")
		return
	}
	accessToken, accessExpiresAt := issueOpenDesignArchivePreviewAccessToken(
		uuidToString(system.WorkspaceID),
		uuidToString(system.ID),
		loaded.ContentDigest,
	)

	writeJSON(w, http.StatusOK, openDesignArchivePreviewResponse{
		Schema:                  openDesignArchivePreviewSchema,
		Slot:                    loaded.Slot,
		ContentDigest:           loaded.ContentDigest,
		ResourceAccessToken:     accessToken,
		ResourceAccessExpiresAt: accessExpiresAt.Format(time.RFC3339),
		Targets:                 targets,
	})
}

func (h *Handler) GetProjectDesignSystemArchivePreviewFile(w http.ResponseWriter, r *http.Request) {
	workspaceID, system, ok := h.openDesignArchivePreviewResourceScope(w, r)
	if !ok {
		return
	}
	loaded, err := h.loadOpenDesignArchivePreview(r.Context(), system)
	if err != nil {
		writeProjectDesignSystemRequestError(w, err)
		return
	}
	digest := "sha256:" + chi.URLParam(r, "digest")
	if digest != loaded.ContentDigest {
		writeProjectDesignSystemError(w, http.StatusConflict, "open_design_preview_digest_conflict", "Open Design archive preview digest is stale")
		return
	}
	artifactPath := strings.TrimPrefix(chi.URLParam(r, "*"), "/")
	artifact, err := opendesign.ReadDraftArchiveArtifact(
		loaded.Archive,
		loaded.ArtifactIndex,
		loaded.ContentDigest,
		artifactPath,
	)
	if err != nil {
		writeProjectDesignSystemError(w, http.StatusNotFound, "open_design_preview_file_not_found", "Open Design archive preview file is unavailable")
		return
	}
	contentType, ok := openDesignArchivePreviewContentType(artifact.Path)
	if !ok {
		writeProjectDesignSystemError(w, http.StatusUnsupportedMediaType, "open_design_preview_file_unsupported", "Open Design archive preview file type is unsupported")
		return
	}

	w.Header().Set("Content-Type", contentType)
	w.Header().Set("Content-Length", strconv.Itoa(len(artifact.Body)))
	w.Header().Set("Content-Disposition", "inline")
	w.Header().Set("Cache-Control", "private, max-age=31536000, immutable")
	w.Header().Set("Content-Security-Policy", openDesignArchivePreviewCSP)
	w.Header().Set("Referrer-Policy", "no-referrer")
	w.Header().Set("X-Content-Type-Options", "nosniff")
	w.Header().Set("X-Frame-Options", "SAMEORIGIN")
	w.Header().Set("X-Open-Design-Content-Digest", loaded.ContentDigest)
	w.Header().Set("X-Open-Design-Workspace-ID", uuidToString(workspaceID))
	w.WriteHeader(http.StatusOK)
	_, _ = w.Write(artifact.Body)
}

func (h *Handler) openDesignArchivePreviewResourceScope(
	w http.ResponseWriter,
	r *http.Request,
) (pgtype.UUID, db.ProjectDesignSystem, bool) {
	workspaceID, ok := parseUUIDOrBadRequest(w, chi.URLParam(r, "workspaceId"), "workspace_id")
	if !ok {
		return pgtype.UUID{}, db.ProjectDesignSystem{}, false
	}
	systemID, ok := parseUUIDOrBadRequest(w, chi.URLParam(r, "systemId"), "project_design_system_id")
	if !ok {
		return pgtype.UUID{}, db.ProjectDesignSystem{}, false
	}
	contentDigest := "sha256:" + chi.URLParam(r, "digest")
	if opendesign.ValidateContentDigest(contentDigest) != nil || !validateOpenDesignArchivePreviewAccessToken(
		chi.URLParam(r, "accessToken"),
		uuidToString(workspaceID),
		uuidToString(systemID),
		contentDigest,
		time.Now(),
	) {
		writeProjectDesignSystemError(w, http.StatusNotFound, "open_design_preview_file_not_found", "Open Design archive preview file is unavailable")
		return pgtype.UUID{}, db.ProjectDesignSystem{}, false
	}
	system, err := h.Queries.GetProjectDesignSystemInWorkspace(r.Context(), db.GetProjectDesignSystemInWorkspaceParams{
		ID: systemID, WorkspaceID: workspaceID,
	})
	if err != nil {
		writeProjectDesignSystemError(w, http.StatusNotFound, "project_design_system_not_found", "project design system not found")
		return pgtype.UUID{}, db.ProjectDesignSystem{}, false
	}
	return workspaceID, system, true
}

func issueOpenDesignArchivePreviewAccessToken(workspaceID, systemID, contentDigest string) (string, time.Time) {
	expiresAt := time.Now().UTC().Add(openDesignArchivePreviewAccessTokenLifetime).Truncate(time.Second)
	expiresUnix := strconv.FormatInt(expiresAt.Unix(), 10)
	signature := signOpenDesignArchivePreviewAccessToken(workspaceID, systemID, contentDigest, expiresUnix)
	return strings.Join([]string{openDesignArchivePreviewAccessTokenVersion, expiresUnix, signature}, "."), expiresAt
}

func validateOpenDesignArchivePreviewAccessToken(token, workspaceID, systemID, contentDigest string, now time.Time) bool {
	parts := strings.Split(token, ".")
	if len(parts) != 3 || parts[0] != openDesignArchivePreviewAccessTokenVersion {
		return false
	}
	expiresUnix, err := strconv.ParseInt(parts[1], 10, 64)
	if err != nil || now.Unix() > expiresUnix {
		return false
	}
	provided, err := hex.DecodeString(parts[2])
	if err != nil || len(provided) != sha256.Size {
		return false
	}
	expected, err := hex.DecodeString(signOpenDesignArchivePreviewAccessToken(workspaceID, systemID, contentDigest, parts[1]))
	return err == nil && hmac.Equal(provided, expected)
}

func signOpenDesignArchivePreviewAccessToken(workspaceID, systemID, contentDigest, expiresUnix string) string {
	mac := hmac.New(sha256.New, auth.JWTSecret())
	_, _ = mac.Write([]byte(strings.Join([]string{
		openDesignArchivePreviewSchema,
		workspaceID,
		systemID,
		contentDigest,
		expiresUnix,
	}, "\x00")))
	return hex.EncodeToString(mac.Sum(nil))
}

func (h *Handler) loadOpenDesignArchivePreview(
	ctx context.Context,
	system db.ProjectDesignSystem,
) (loadedOpenDesignArchivePreview, error) {
	selected, err := h.Queries.GetProjectDesignSystemPackageBySlot(ctx, db.GetProjectDesignSystemPackageBySlotParams{
		DesignSystemID: system.ID, Slot: "draft", WorkspaceID: system.WorkspaceID,
	})
	if errors.Is(err, pgx.ErrNoRows) {
		selected, err = h.Queries.GetProjectDesignSystemPackageBySlot(ctx, db.GetProjectDesignSystemPackageBySlotParams{
			DesignSystemID: system.ID, Slot: "saved", WorkspaceID: system.WorkspaceID,
		})
	}
	if errors.Is(err, pgx.ErrNoRows) {
		return loadedOpenDesignArchivePreview{}, &projectDesignSystemRequestError{
			status: http.StatusNotFound, code: "open_design_preview_unavailable", message: "Open Design archive preview is unavailable",
		}
	}
	if err != nil {
		return loadedOpenDesignArchivePreview{}, projectDesignSystemInternalError("open_design_preview_lookup_failed", "failed to load Open Design archive preview")
	}
	return h.loadOpenDesignArchivePreviewPackage(ctx, h.Queries, system, selected)
}

func (h *Handler) loadOpenDesignArchivePreviewPackage(
	ctx context.Context,
	queries *db.Queries,
	system db.ProjectDesignSystem,
	selected db.ProjectDesignSystemPackage,
) (loadedOpenDesignArchivePreview, error) {
	if selected.RenderStatus != "passed" {
		return loadedOpenDesignArchivePreview{}, &projectDesignSystemRequestError{
			status: http.StatusConflict, code: "open_design_preview_unavailable", message: "Open Design archive preview has not passed validation",
		}
	}
	if !selected.SourceTaskID.Valid {
		return loadedOpenDesignArchivePreview{}, &projectDesignSystemRequestError{
			status: http.StatusNotFound, code: "open_design_preview_unavailable", message: "Open Design archive preview is unavailable",
		}
	}
	run, err := queries.GetOpenDesignRunByTask(ctx, selected.SourceTaskID)
	if errors.Is(err, pgx.ErrNoRows) {
		return loadedOpenDesignArchivePreview{}, &projectDesignSystemRequestError{
			status: http.StatusNotFound, code: "open_design_preview_unavailable", message: "Open Design archive preview is unavailable",
		}
	}
	if err != nil {
		return loadedOpenDesignArchivePreview{}, projectDesignSystemInternalError("open_design_preview_lookup_failed", "failed to load Open Design archive preview")
	}

	var manifest opendesign.DraftPackageManifest
	if json.Unmarshal(selected.Manifest, &manifest) != nil || manifest.Schema != opendesign.DraftPackageManifestSchema {
		return loadedOpenDesignArchivePreview{}, &projectDesignSystemRequestError{
			status: http.StatusNotFound, code: "open_design_preview_unavailable", message: "Open Design archive preview is unavailable",
		}
	}
	var validation opendesign.DraftPackageValidation
	if json.Unmarshal(selected.Validation, &validation) != nil ||
		validation.Schema != opendesign.DraftPackageValidationSchema ||
		!validation.Passed ||
		opendesign.ValidatePackageAuditReceipt(validation.Audit) != nil ||
		opendesign.ValidatePreviewVerificationReceipt(validation.Preview) != nil ||
		!validation.Audit.Audit.OK ||
		!validation.Preview.Verification.Passed {
		return loadedOpenDesignArchivePreview{}, openDesignArchivePreviewConflict()
	}
	if !openDesignArchivePreviewEvidenceMatches(system, selected, run, manifest, validation) {
		return loadedOpenDesignArchivePreview{}, openDesignArchivePreviewConflict()
	}
	var artifactIndex []opendesign.ArtifactIndexEntry
	if json.Unmarshal(run.ArtifactIndex, &artifactIndex) != nil {
		return loadedOpenDesignArchivePreview{}, openDesignArchivePreviewConflict()
	}
	archive, err := h.readOpenDesignDraftArchive(ctx, run.ArchiveObjectKey.String)
	if err != nil {
		return loadedOpenDesignArchivePreview{}, err
	}
	targets, err := opendesign.DiscoverPreviewTargets(archive)
	if err != nil || opendesign.ValidatePreviewVerificationTargetSet(validation.Preview.Verification, targets) != nil {
		return loadedOpenDesignArchivePreview{}, openDesignArchivePreviewConflict()
	}
	return loadedOpenDesignArchivePreview{
		Slot:          selected.Slot,
		ContentDigest: run.ContentDigest.String,
		ArtifactIndex: artifactIndex,
		Archive:       archive,
	}, nil
}

func openDesignArchivePreviewEvidenceMatches(
	system db.ProjectDesignSystem,
	selected db.ProjectDesignSystemPackage,
	run db.OpenDesignRun,
	manifest opendesign.DraftPackageManifest,
	validation opendesign.DraftPackageValidation,
) bool {
	if run.Status != string(opendesign.RunStatusSucceeded) ||
		!run.OpenDesignRunID.Valid || !run.ArchiveObjectKey.Valid || !run.ContentDigest.Valid ||
		uuidToString(run.WorkspaceID) != uuidToString(system.WorkspaceID) ||
		uuidToString(run.ProjectID) != uuidToString(system.ProjectID) ||
		uuidToString(run.DesignSystemID) != uuidToString(system.ID) ||
		uuidToString(run.TaskID) != uuidToString(selected.SourceTaskID) ||
		selected.IntegritySha256 != strings.TrimPrefix(run.ContentDigest.String, "sha256:") {
		return false
	}
	if manifest.Engine.Validate() != nil || manifest.Format != opendesign.DraftPackageFormat ||
		manifest.Run.SupervisorRunID != uuidToString(run.ID) ||
		manifest.Run.WorkerRunID != run.OpenDesignRunID.String ||
		manifest.Run.TaskID != uuidToString(run.TaskID) ||
		manifest.Run.DesignSystemID != uuidToString(run.DesignSystemID) ||
		manifest.Run.Operation != run.Operation ||
		manifest.Archive.ObjectKey != run.ArchiveObjectKey.String ||
		manifest.Archive.ContentDigest != run.ContentDigest.String ||
		validation.Audit.ContentDigest != run.ContentDigest.String ||
		validation.Preview.ContentDigest != run.ContentDigest.String ||
		!openDesignAuditEngineMatches(run, manifest.Engine) ||
		!openDesignAuditEngineMatches(run, validation.Audit.Engine) ||
		!openDesignAuditEngineMatches(run, validation.Preview.Engine) {
		return false
	}
	manifestIndex, err := json.Marshal(manifest.Archive.ArtifactIndex)
	if err != nil || !jsonValuesEqual(run.ArtifactIndex, manifestIndex) {
		return false
	}
	auditJSON, err := json.Marshal(validation.Audit)
	if err != nil || !jsonValuesEqual(run.AuditReport, auditJSON) {
		return false
	}
	previewJSON, err := json.Marshal(validation.Preview)
	return err == nil && jsonValuesEqual(run.PreviewReceipt, previewJSON) && jsonValuesEqual(selected.RenderReport, previewJSON)
}

func openDesignArchivePreviewConflict() error {
	return &projectDesignSystemRequestError{
		status: http.StatusConflict, code: "open_design_preview_evidence_conflict", message: "Open Design archive preview evidence is inconsistent",
	}
}

func openDesignArchivePreviewContentType(artifactPath string) (string, bool) {
	switch strings.ToLower(path.Ext(artifactPath)) {
	case ".html", ".htm":
		return "text/html; charset=utf-8", true
	case ".css":
		return "text/css; charset=utf-8", true
	case ".js", ".mjs":
		return "text/javascript; charset=utf-8", true
	case ".json":
		return "application/json; charset=utf-8", true
	case ".svg":
		return "image/svg+xml", true
	case ".png":
		return "image/png", true
	case ".jpg", ".jpeg":
		return "image/jpeg", true
	case ".gif":
		return "image/gif", true
	case ".webp":
		return "image/webp", true
	case ".avif":
		return "image/avif", true
	case ".ico":
		return "image/x-icon", true
	case ".woff":
		return "font/woff", true
	case ".woff2":
		return "font/woff2", true
	case ".ttf":
		return "font/ttf", true
	case ".otf":
		return "font/otf", true
	default:
		return "", false
	}
}
