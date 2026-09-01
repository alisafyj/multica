package handler

import (
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"strings"
	"time"

	"github.com/go-chi/chi/v5"
	"github.com/jackc/pgx/v5"
	db "github.com/multica-ai/multica/server/pkg/db/generated"
)

const designImplementationContextSchemaV1 = "multica.design-implementation-context/v1"

type DesignImplementationRequest struct {
	RevisionID        string   `json:"revision_id"`
	FrameRefs         []string `json:"frame_refs"`
	ProjectResourceID string   `json:"project_resource_id"`
	IssueID           string   `json:"issue_id"`
}

type DesignImplementationPaths struct {
	Context           string `json:"context_path"`
	DesignManifest    string `json:"design_manifest_path"`
	DesignPackage     string `json:"design_package_path"`
	Scope             string `json:"scope_path"`
	RepositoryContext string `json:"repository_context_path"`
	Result            string `json:"result_path"`
}

type DesignImplementationSourceCapabilities struct {
	HasLayers       bool `json:"has_layers"`
	HasPrototype    bool `json:"has_prototype"`
	HasAssets       bool `json:"has_assets"`
	HasInteractions bool `json:"has_interactions"`
}

type DesignImplementationContextResponse struct {
	SchemaVersion            string                                 `json:"schema_version"`
	DesignRef                string                                 `json:"design_ref"`
	RevisionID               string                                 `json:"revision_id"`
	ContentDigest            string                                 `json:"content_digest"`
	FrameRefs                []string                               `json:"frame_refs"`
	ProjectID                string                                 `json:"project_id"`
	IssueID                  string                                 `json:"issue_id"`
	ProjectResourceID        string                                 `json:"project_resource_id"`
	DesignTitle              string                                 `json:"design_title"`
	DesignSystemDigest       string                                 `json:"design_system_digest,omitempty"`
	AllowedWritePaths        []string                               `json:"allowed_write_paths"`
	VerificationRequirements []string                               `json:"verification_requirements"`
	Paths                    DesignImplementationPaths              `json:"paths"`
	SourceCapabilities       DesignImplementationSourceCapabilities `json:"source_capabilities"`
}

type DesignImplementationPromptResponse struct {
	Prompt       string                              `json:"prompt"`
	MCPArguments map[string]any                      `json:"mcp_arguments"`
	Context      DesignImplementationContextResponse `json:"context"`
}

func (h *Handler) BuildDesignImplementationPrompt(w http.ResponseWriter, r *http.Request) {
	contextValue, request, repository, ok := h.resolveDesignImplementationRequest(w, r)
	if !ok {
		return
	}
	frameLines := make([]string, len(contextValue.FrameRefs))
	for i, frameRef := range contextValue.FrameRefs {
		frameLines[i] = "- " + frameRef
	}
	prompt := fmt.Sprintf("【任务】\n根据关联设计稿实现当前任务，优先复用目标仓库已有组件和页面结构。\n\n【设计稿】\n标题：%s\n固定版本：%s\n所选 Frame：\n%s\n目标仓库：%s\n\n【执行步骤】\n1. 调用 multica_design_get_implementation_context。\n2. 读取目标仓库路由、组件、状态管理和样式规范。\n3. 根据 Implementation Context 完成实现。\n4. 运行约定验证。\n5. 输出 Frame 到代码文件映射。\n\n【约束】\n禁止整图替代；禁止直接复制 Prototype；保留无关 dirty worktree。\n\n【输出】\n修改文件、复用组件、新增组件、检查结果、视觉验收和阻塞项。",
		contextValue.DesignTitle, contextValue.RevisionID, strings.Join(frameLines, "\n"), designImplementationRepositoryName(repository))
	writeJSON(w, http.StatusOK, DesignImplementationPromptResponse{
		Prompt: prompt,
		MCPArguments: map[string]any{
			"designRef": contextValue.DesignRef, "revisionId": contextValue.RevisionID,
			"frameRefs": contextValue.FrameRefs, "targetRepositoryId": contextValue.ProjectResourceID,
			"issueId": request.IssueID,
		},
		Context: contextValue,
	})
}

func (h *Handler) GetDesignImplementationContext(w http.ResponseWriter, r *http.Request) {
	contextValue, _, _, ok := h.resolveDesignImplementationRequest(w, r)
	if !ok {
		return
	}
	writeJSON(w, http.StatusOK, contextValue)
}

func (h *Handler) resolveDesignImplementationRequest(w http.ResponseWriter, r *http.Request) (DesignImplementationContextResponse, DesignImplementationRequest, db.ProjectResource, bool) {
	workspaceID, requesterID, ok := h.projectDesignSystemRequestScope(w, r)
	if !ok {
		return DesignImplementationContextResponse{}, DesignImplementationRequest{}, db.ProjectResource{}, false
	}
	var request DesignImplementationRequest
	decoder := json.NewDecoder(http.MaxBytesReader(w, r.Body, 1<<20))
	decoder.DisallowUnknownFields()
	if err := decoder.Decode(&request); err != nil {
		writeProjectDesignSystemError(w, http.StatusBadRequest, "invalid_request", "implementation context request is invalid; select the design and repository again")
		return DesignImplementationContextResponse{}, DesignImplementationRequest{}, db.ProjectResource{}, false
	}
	if err := decoder.Decode(&struct{}{}); !errors.Is(err, io.EOF) {
		writeProjectDesignSystemError(w, http.StatusBadRequest, "invalid_request", "implementation context request must contain one JSON object")
		return DesignImplementationContextResponse{}, DesignImplementationRequest{}, db.ProjectResource{}, false
	}
	claim, err := parseDesignAssetRef(chi.URLParam(r, "designRef"), time.Now())
	if err != nil {
		writeProjectDesignSystemError(w, http.StatusBadRequest, "design_ref_invalid", "design reference is invalid or expired; select the design again")
		return DesignImplementationContextResponse{}, DesignImplementationRequest{}, db.ProjectResource{}, false
	}
	if claim.WorkspaceID != uuidToString(workspaceID) || claim.UserID != uuidToString(requesterID) {
		writeProjectDesignSystemError(w, http.StatusForbidden, "forbidden", "design reference is not available to this user or workspace")
		return DesignImplementationContextResponse{}, DesignImplementationRequest{}, db.ProjectResource{}, false
	}
	if request.RevisionID != claim.RevisionID {
		writeProjectDesignSystemError(w, http.StatusConflict, "revision_not_restorable", "requested revision does not match the frozen design reference")
		return DesignImplementationContextResponse{}, DesignImplementationRequest{}, db.ProjectResource{}, false
	}
	issue, ok := h.loadIssueForUser(w, r, strings.TrimSpace(request.IssueID))
	if !ok {
		return DesignImplementationContextResponse{}, DesignImplementationRequest{}, db.ProjectResource{}, false
	}
	if !issue.ProjectID.Valid || uuidToString(issue.ProjectID) != claim.ProjectID {
		writeProjectDesignSystemError(w, http.StatusConflict, "project_mismatch", "issue and design must belong to the same project")
		return DesignImplementationContextResponse{}, DesignImplementationRequest{}, db.ProjectResource{}, false
	}
	repositoryID, err := parseDesignAssetClaimUUID(strings.TrimSpace(request.ProjectResourceID))
	if err != nil {
		writeProjectDesignSystemError(w, http.StatusNotFound, "repository_not_found", "target repository was not found; select it again")
		return DesignImplementationContextResponse{}, DesignImplementationRequest{}, db.ProjectResource{}, false
	}
	repository, err := h.Queries.GetProjectResourceInWorkspace(r.Context(), db.GetProjectResourceInWorkspaceParams{ID: repositoryID, WorkspaceID: workspaceID})
	if errors.Is(err, pgx.ErrNoRows) {
		writeProjectDesignSystemError(w, http.StatusNotFound, "repository_not_found", "target repository was not found; select it again")
		return DesignImplementationContextResponse{}, DesignImplementationRequest{}, db.ProjectResource{}, false
	}
	if err != nil {
		writeDesignAssetResolveError(w, err)
		return DesignImplementationContextResponse{}, DesignImplementationRequest{}, db.ProjectResource{}, false
	}
	if repository.ResourceType != projectResourceTypeGitHubRepo || uuidToString(repository.ProjectID) != claim.ProjectID {
		writeProjectDesignSystemError(w, http.StatusConflict, "project_mismatch", "target repository and design must belong to the same project")
		return DesignImplementationContextResponse{}, DesignImplementationRequest{}, db.ProjectResource{}, false
	}
	frames, err := h.resolveDesignImplementationFrames(r, claim)
	if err != nil {
		writeDesignAssetResolveError(w, err)
		return DesignImplementationContextResponse{}, DesignImplementationRequest{}, db.ProjectResource{}, false
	}
	if err := validateDesignImplementationFrameRefs(claim, request.FrameRefs, frames); err != nil {
		writeDesignAssetResolveError(w, err)
		return DesignImplementationContextResponse{}, DesignImplementationRequest{}, db.ProjectResource{}, false
	}
	title, designSystemDigest, capabilities, err := h.designImplementationMetadata(r, claim)
	if err != nil {
		writeDesignAssetResolveError(w, err)
		return DesignImplementationContextResponse{}, DesignImplementationRequest{}, db.ProjectResource{}, false
	}
	return DesignImplementationContextResponse{
		SchemaVersion: designImplementationContextSchemaV1, DesignRef: chi.URLParam(r, "designRef"),
		RevisionID: claim.RevisionID, ContentDigest: claim.ContentDigest, FrameRefs: append([]string(nil), request.FrameRefs...),
		ProjectID: claim.ProjectID, IssueID: request.IssueID, ProjectResourceID: request.ProjectResourceID,
		DesignTitle: title, DesignSystemDigest: designSystemDigest, AllowedWritePaths: []string{"."},
		VerificationRequirements: []string{"repository typecheck/tests/build as applicable", "real rendered preview for changed UI"},
		Paths: DesignImplementationPaths{
			Context:           ".agent_context/design_implementation/context.json",
			DesignManifest:    ".agent_context/design_implementation/design/manifest.json",
			DesignPackage:     ".agent_context/design_implementation/design/package",
			Scope:             ".agent_context/design_implementation/design/scope.json",
			RepositoryContext: ".agent_context/design_implementation/repository",
			Result:            ".agent_context/design_implementation/result/implementation-result.json",
		},
		SourceCapabilities: capabilities,
	}, request, repository, true
}

func (h *Handler) resolveDesignImplementationFrames(r *http.Request, claim designAssetRefClaim) ([]DesignAssetFrameResponse, error) {
	switch claim.Kind {
	case "figma":
		return h.resolveFigmaDesignAssetFrames(r, claim)
	case "multica":
		return h.resolveMulticaDesignAssetFrames(r, claim)
	default:
		return nil, designAssetResolveFailure(http.StatusBadRequest, "design_ref_invalid", "design reference is invalid; select the design again")
	}
}

func validateDesignImplementationFrameRefs(design designAssetRefClaim, refs []string, available []DesignAssetFrameResponse) error {
	if len(refs) != 1 {
		return designAssetResolveFailure(http.StatusBadRequest, "frame_ref_invalid", "select exactly one frame")
	}
	availableSelections := make(map[string]struct{}, len(available))
	for _, frame := range available {
		claim, err := parseDesignAssetFrameRef(frame.FrameRef, time.Now())
		if err == nil {
			availableSelections[claim.SelectionKind+"\x00"+claim.SelectionID] = struct{}{}
		}
	}
	seen := make(map[string]struct{}, len(refs))
	for _, raw := range refs {
		if raw == "" {
			return designAssetResolveFailure(http.StatusBadRequest, "frame_ref_invalid", "selected frame reference is invalid; select it again")
		}
		if _, duplicate := seen[raw]; duplicate {
			return designAssetResolveFailure(http.StatusBadRequest, "frame_ref_invalid", "selected frames must be unique")
		}
		seen[raw] = struct{}{}
		frame, err := parseDesignAssetFrameRef(raw, time.Now())
		if err != nil || frame.WorkspaceID != design.WorkspaceID || frame.ProjectID != design.ProjectID || frame.UserID != design.UserID ||
			frame.AssetID != design.AssetID || frame.RevisionID != design.RevisionID || frame.ContentDigest != design.ContentDigest {
			return designAssetResolveFailure(http.StatusBadRequest, "frame_ref_invalid", "selected frame does not belong to the frozen design revision")
		}
		if _, ok := availableSelections[frame.SelectionKind+"\x00"+frame.SelectionID]; !ok {
			return designAssetResolveFailure(http.StatusBadRequest, "frame_ref_invalid", "selected frame is no longer available in this revision")
		}
	}
	return nil
}

func (h *Handler) designImplementationMetadata(r *http.Request, claim designAssetRefClaim) (string, string, DesignImplementationSourceCapabilities, error) {
	workspaceID, err := parseDesignAssetClaimUUID(claim.WorkspaceID)
	if err != nil {
		return "", "", DesignImplementationSourceCapabilities{}, err
	}
	assetID, err := parseDesignAssetClaimUUID(claim.AssetID)
	if err != nil {
		return "", "", DesignImplementationSourceCapabilities{}, err
	}
	switch claim.Kind {
	case "figma":
		file, err := h.Queries.GetDesignFileInWorkspace(r.Context(), db.GetDesignFileInWorkspaceParams{ID: assetID, WorkspaceID: workspaceID})
		return file.Title, "", DesignImplementationSourceCapabilities{HasLayers: true, HasAssets: true, HasInteractions: true}, err
	case "multica":
		document, err := h.Queries.GetDesignDocumentInWorkspace(r.Context(), db.GetDesignDocumentInWorkspaceParams{ID: assetID, WorkspaceID: workspaceID})
		if err != nil {
			return "", "", DesignImplementationSourceCapabilities{}, err
		}
		revisionID, err := parseDesignAssetClaimUUID(claim.RevisionID)
		if err != nil {
			return "", "", DesignImplementationSourceCapabilities{}, err
		}
		revision, err := h.Queries.GetDesignDocumentRevisionInWorkspace(r.Context(), db.GetDesignDocumentRevisionInWorkspaceParams{ID: revisionID, WorkspaceID: workspaceID})
		return document.Title, textToString(revision.DesignSystemDigest), DesignImplementationSourceCapabilities{HasPrototype: true, HasAssets: true, HasInteractions: true}, err
	default:
		return "", "", DesignImplementationSourceCapabilities{}, designAssetResolveFailure(http.StatusBadRequest, "design_ref_invalid", "design reference is invalid")
	}
}

func designImplementationRepositoryName(repository db.ProjectResource) string {
	if repository.Label.Valid && strings.TrimSpace(repository.Label.String) != "" {
		return repository.Label.String
	}
	return uuidToString(repository.ID)
}
