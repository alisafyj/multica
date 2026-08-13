package handler

import (
	"encoding/hex"
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"strings"

	"github.com/jackc/pgx/v5"
	"github.com/multica-ai/multica/server/internal/projectdesignsystem"
	db "github.com/multica-ai/multica/server/pkg/db/generated"
	"github.com/multica-ai/multica/server/pkg/protocol"
)

const (
	maxProjectDesignSystemPreviewVerificationBytes = 16 << 10
	maxProjectDesignSystemPreviewLocators          = 10_000
	maxProjectDesignSystemPreviewImages            = 10_000
	maxProjectDesignSystemPreviewDimension         = 100_000
)

type ProjectDesignSystemPreviewVerificationRequest struct {
	Status              string `json:"status"`
	Digest              string `json:"digest"`
	Reason              string `json:"reason"`
	LocatorCount        int    `json:"locator_count"`
	VisibleLocatorCount int    `json:"visible_locator_count"`
	BodyWidth           int    `json:"body_width"`
	BodyHeight          int    `json:"body_height"`
	ImageCount          int    `json:"image_count"`
	FailedImageCount    int    `json:"failed_image_count"`
}

type projectDesignSystemPreviewVerificationReport struct {
	Source                 string `json:"source"`
	BridgeStatus           string `json:"bridge_status"`
	Reason                 string `json:"reason,omitempty"`
	Digest                 string `json:"digest"`
	LocatorCount           int    `json:"locator_count"`
	ExpectedLocatorCount   int    `json:"expected_locator_count"`
	VisibleLocatorCount    int    `json:"visible_locator_count"`
	BodyWidth              int    `json:"body_width"`
	BodyHeight             int    `json:"body_height"`
	ImageCount             int    `json:"image_count"`
	FailedImageCount       int    `json:"failed_image_count"`
	StaticValidationPassed bool   `json:"static_validation_passed"`
	DigestMatched          bool   `json:"digest_matched"`
	Accepted               bool   `json:"accepted"`
}

func (h *Handler) VerifyProjectDesignSystemPreview(w http.ResponseWriter, r *http.Request) {
	req, ok := decodeProjectDesignSystemPreviewVerificationRequest(w, r)
	if !ok {
		return
	}
	workspaceID, requesterID, ok := h.projectDesignSystemRequestScope(w, r)
	if !ok {
		return
	}
	initialSystem, ok := h.loadProjectDesignSystemForRequest(w, r, workspaceID)
	if !ok {
		return
	}

	tx, err := h.TxStarter.Begin(r.Context())
	if err != nil {
		writeProjectDesignSystemError(w, http.StatusInternalServerError, "transaction_failed", "failed to start preview verification")
		return
	}
	defer tx.Rollback(r.Context())
	queries := h.Queries.WithTx(tx)
	if _, err := queries.LockProjectInWorkspaceForUpdate(r.Context(), db.LockProjectInWorkspaceForUpdateParams{
		ID: initialSystem.ProjectID, WorkspaceID: workspaceID,
	}); err != nil {
		writeProjectDesignSystemError(w, http.StatusNotFound, "project_design_system_not_found", "project design system not found")
		return
	}
	system, err := queries.GetProjectDesignSystemInWorkspaceForUpdate(r.Context(), db.GetProjectDesignSystemInWorkspaceForUpdateParams{
		ID: initialSystem.ID, WorkspaceID: workspaceID,
	})
	if err != nil {
		writeProjectDesignSystemError(w, http.StatusNotFound, "project_design_system_not_found", "project design system not found")
		return
	}
	draft, err := queries.GetProjectDesignSystemPackageBySlot(r.Context(), db.GetProjectDesignSystemPackageBySlotParams{
		DesignSystemID: system.ID, Slot: "draft", WorkspaceID: workspaceID,
	})
	if errors.Is(err, pgx.ErrNoRows) {
		writeProjectDesignSystemError(w, http.StatusConflict, "preview_candidate_required", "a design system preview candidate is required")
		return
	}
	if err != nil {
		writeProjectDesignSystemError(w, http.StatusInternalServerError, "preview_candidate_lookup_failed", "failed to load preview candidate")
		return
	}
	if req.Digest != draft.IntegritySha256 {
		writeProjectDesignSystemError(w, http.StatusConflict, "preview_digest_stale", "preview verification does not match the current candidate")
		return
	}

	changed := false
	if draft.RenderStatus != "passed" && !(draft.RenderStatus == "failed" && req.Status == "failed") {
		renderStatus, report := h.evaluateProjectDesignSystemPreviewVerification(draft, req)
		reportJSON, marshalErr := json.Marshal(report)
		if marshalErr != nil {
			writeProjectDesignSystemError(w, http.StatusInternalServerError, "preview_verification_failed", "failed to record preview verification")
			return
		}
		if _, err := queries.UpdateProjectDesignSystemPackageRenderValidation(r.Context(), db.UpdateProjectDesignSystemPackageRenderValidationParams{
			RenderStatus:    renderStatus,
			RenderReport:    reportJSON,
			DesignSystemID:  system.ID,
			IntegritySha256: draft.IntegritySha256,
			WorkspaceID:     workspaceID,
		}); err != nil {
			writeProjectDesignSystemError(w, http.StatusConflict, "preview_digest_stale", "preview verification does not match the current candidate")
			return
		}
		changed = true
	}
	if changed {
		if err := tx.Commit(r.Context()); err != nil {
			writeProjectDesignSystemError(w, http.StatusInternalServerError, "preview_verification_failed", "failed to commit preview verification")
			return
		}
	} else if err := tx.Rollback(r.Context()); err != nil && !errors.Is(err, pgx.ErrTxClosed) {
		writeProjectDesignSystemError(w, http.StatusInternalServerError, "preview_verification_failed", "failed to finish preview verification")
		return
	}

	response, err := h.projectDesignSystemResponse(r.Context(), system)
	if err != nil {
		writeProjectDesignSystemError(w, http.StatusInternalServerError, "response_failed", "failed to build project design system response")
		return
	}
	if changed {
		h.publish(protocol.EventProjectDesignSystemChanged, uuidToString(workspaceID), "member", uuidToString(requesterID), map[string]any{
			"project_design_system_id": uuidToString(system.ID),
			"project_id":               uuidToString(system.ProjectID),
			"status":                   response.Status,
			"preview_validation":       response.PreviewValidation.Status,
		})
	}
	writeJSON(w, http.StatusOK, response)
}

func decodeProjectDesignSystemPreviewVerificationRequest(w http.ResponseWriter, r *http.Request) (ProjectDesignSystemPreviewVerificationRequest, bool) {
	r.Body = http.MaxBytesReader(w, r.Body, maxProjectDesignSystemPreviewVerificationBytes)
	decoder := json.NewDecoder(r.Body)
	decoder.DisallowUnknownFields()
	var req ProjectDesignSystemPreviewVerificationRequest
	if err := decoder.Decode(&req); err != nil {
		writeProjectDesignSystemError(w, http.StatusBadRequest, "preview_verification_invalid", "invalid preview verification receipt")
		return ProjectDesignSystemPreviewVerificationRequest{}, false
	}
	if err := decoder.Decode(&struct{}{}); !errors.Is(err, io.EOF) {
		writeProjectDesignSystemError(w, http.StatusBadRequest, "preview_verification_invalid", "invalid preview verification receipt")
		return ProjectDesignSystemPreviewVerificationRequest{}, false
	}
	req.Status = strings.TrimSpace(req.Status)
	req.Digest = strings.TrimSpace(req.Digest)
	req.Reason = strings.TrimSpace(req.Reason)
	if !validProjectDesignSystemPreviewVerificationRequest(req) {
		writeProjectDesignSystemError(w, http.StatusBadRequest, "preview_verification_invalid", "invalid preview verification receipt")
		return ProjectDesignSystemPreviewVerificationRequest{}, false
	}
	return req, true
}

func validProjectDesignSystemPreviewVerificationRequest(req ProjectDesignSystemPreviewVerificationRequest) bool {
	if req.Status != "ready" && req.Status != "failed" {
		return false
	}
	decodedDigest, err := hex.DecodeString(req.Digest)
	if err != nil || len(decodedDigest) != 32 || strings.ToLower(req.Digest) != req.Digest {
		return false
	}
	if req.Status == "ready" && req.Reason != "" {
		return false
	}
	if req.Status == "failed" && !validProjectDesignSystemPreviewFailureReason(req.Reason) {
		return false
	}
	if req.LocatorCount < 0 || req.LocatorCount > maxProjectDesignSystemPreviewLocators ||
		req.VisibleLocatorCount < 0 || req.VisibleLocatorCount > req.LocatorCount ||
		req.BodyWidth < 0 || req.BodyWidth > maxProjectDesignSystemPreviewDimension ||
		req.BodyHeight < 0 || req.BodyHeight > maxProjectDesignSystemPreviewDimension ||
		req.ImageCount < 0 || req.ImageCount > maxProjectDesignSystemPreviewImages ||
		req.FailedImageCount < 0 || req.FailedImageCount > req.ImageCount {
		return false
	}
	return true
}

func validProjectDesignSystemPreviewFailureReason(reason string) bool {
	switch reason {
	case "invalid_digest", "empty_body", "no_visible_locator", "failed_images", "measurement_failed":
		return true
	default:
		return false
	}
}

func (h *Handler) evaluateProjectDesignSystemPreviewVerification(
	draft db.ProjectDesignSystemPackage,
	req ProjectDesignSystemPreviewVerificationRequest,
) (string, projectDesignSystemPreviewVerificationReport) {
	validated, validationErr := projectdesignsystem.Validate(projectdesignsystem.ArtifactInput{
		DesignMD: draft.DesignMd, TokensCSS: draft.TokensCss, ComponentsHTML: draft.ComponentsHtml,
	}, h.projectDesignSystemAllowedHosts())
	staticValidationPassed := validationErr == nil && validated.Validation.Passed
	digestMatched := staticValidationPassed && validated.Manifest.Digest == draft.IntegritySha256
	expectedLocatorCount := 0
	if staticValidationPassed {
		expectedLocatorCount = len(validated.Manifest.Locators)
	}
	reason := req.Reason
	accepted := req.Status == "ready" &&
		staticValidationPassed &&
		digestMatched &&
		req.LocatorCount == expectedLocatorCount &&
		req.VisibleLocatorCount > 0 &&
		req.BodyWidth > 0 && req.BodyHeight > 0 &&
		req.FailedImageCount == 0
	if !accepted && reason == "" {
		switch {
		case !staticValidationPassed:
			reason = "static_validation_failed"
		case !digestMatched:
			reason = "stored_digest_mismatch"
		case req.BodyWidth <= 0 || req.BodyHeight <= 0:
			reason = "empty_body"
		case req.LocatorCount != expectedLocatorCount:
			reason = "locator_count_mismatch"
		case req.VisibleLocatorCount <= 0:
			reason = "no_visible_locator"
		case req.FailedImageCount > 0:
			reason = "failed_images"
		default:
			reason = "measurement_failed"
		}
	}
	report := projectDesignSystemPreviewVerificationReport{
		Source:                 "trusted_preview_bridge",
		BridgeStatus:           req.Status,
		Reason:                 reason,
		Digest:                 req.Digest,
		LocatorCount:           req.LocatorCount,
		ExpectedLocatorCount:   expectedLocatorCount,
		VisibleLocatorCount:    req.VisibleLocatorCount,
		BodyWidth:              req.BodyWidth,
		BodyHeight:             req.BodyHeight,
		ImageCount:             req.ImageCount,
		FailedImageCount:       req.FailedImageCount,
		StaticValidationPassed: staticValidationPassed,
		DigestMatched:          digestMatched,
		Accepted:               accepted,
	}
	if accepted {
		return "passed", report
	}
	return "failed", report
}
