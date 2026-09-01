package main

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net/url"
	"os"

	"github.com/multica-ai/multica/server/internal/cli"
)

type designMCPAdapter struct {
	client  *cli.APIClient
	rootDir string
}

func designMCPToolDescriptors() []map[string]any {
	return []map[string]any{
		designMCPToolDescriptor("multica_design_get_restore_pack", "Get a server-built Restore Pack for a design restore scope."),
		designMCPToolDescriptor("multica_design_list_files", "List design files available in the configured workspace."),
		designMCPToolDescriptor("multica_design_list_frames", "List frames for one design file revision."),
		designMCPToolDescriptor("multica_design_list_groups", "List Figma groups for one design file revision."),
		designMCPToolDescriptor("multica_design_get_selection_context", "Get selected layer or bounds context for one design frame."),
		designMCPToolDescriptor("multica_design_get_ui_restore_artifact", "Return UI restore artifact path metadata for frontend implementation."),
		designMCPToolDescriptor("multica_design_get_implementation_context", "Materialize the frozen design implementation context using bounded repository-relative paths."),
	}
}

func designMCPToolDescriptor(name string, description string) map[string]any {
	return map[string]any{
		"name":        name,
		"description": description,
		"inputSchema": map[string]any{
			"type":                 "object",
			"additionalProperties": true,
		},
	}
}

func (a *designMCPAdapter) callTool(ctx context.Context, name string, arguments map[string]any) (any, error) {
	if a == nil || a.client == nil {
		return nil, fmt.Errorf("design MCP adapter is not configured")
	}
	switch name {
	case "multica_design_get_restore_pack":
		return a.getRestorePack(ctx, arguments)
	case "multica_design_list_files":
		return a.listFiles(ctx)
	case "multica_design_list_frames":
		return a.listFrames(ctx, arguments)
	case "multica_design_list_groups":
		return a.listGroups(ctx, arguments)
	case "multica_design_get_selection_context":
		return a.getSelectionContext(ctx, arguments)
	case "multica_design_get_ui_restore_artifact":
		return a.getUIRestoreArtifact(arguments), nil
	case "multica_design_get_implementation_context":
		return a.getImplementationContext(ctx, arguments)
	default:
		return nil, fmt.Errorf("unknown design MCP tool %q", name)
	}
}

const (
	designImplementationContextPath    = ".agent_context/design_implementation/context.json"
	designImplementationManifestPath   = ".agent_context/design_implementation/design/manifest.json"
	designImplementationPackagePath    = ".agent_context/design_implementation/design/package"
	designImplementationScopePath      = ".agent_context/design_implementation/design/scope.json"
	designImplementationRepositoryPath = ".agent_context/design_implementation/repository"
	designImplementationResultPath     = ".agent_context/design_implementation/result/implementation-result.json"
)

type designImplementationContextWire struct {
	SchemaVersion      string         `json:"schema_version"`
	DesignRef          string         `json:"design_ref"`
	RevisionID         string         `json:"revision_id"`
	ContentDigest      string         `json:"content_digest"`
	FrameRefs          []string       `json:"frame_refs"`
	ProjectID          string         `json:"project_id"`
	IssueID            string         `json:"issue_id"`
	ProjectResourceID  string         `json:"project_resource_id"`
	DesignTitle        string         `json:"design_title"`
	DesignSystemDigest string         `json:"design_system_digest,omitempty"`
	AllowedWritePaths  []string       `json:"allowed_write_paths"`
	Verification       []string       `json:"verification_requirements"`
	Capabilities       map[string]any `json:"source_capabilities"`
}

func (a *designMCPAdapter) getImplementationContext(ctx context.Context, arguments map[string]any) (any, error) {
	designRef := stringArgument(arguments, "designRef")
	revisionID := stringArgument(arguments, "revisionId")
	repositoryID := stringArgument(arguments, "targetRepositoryId")
	issueID := stringArgument(arguments, "issueId")
	frameRefs, err := stringSliceArgument(arguments, "frameRefs")
	if err != nil || designRef == "" || revisionID == "" || repositoryID == "" || issueID == "" || len(frameRefs) == 0 {
		return nil, fmt.Errorf("designRef, revisionId, frameRefs, targetRepositoryId, and issueId are required")
	}
	body := map[string]any{
		"revision_id": revisionID, "frame_refs": frameRefs,
		"project_resource_id": repositoryID, "issue_id": issueID,
	}
	var contextValue designImplementationContextWire
	endpoint := "/api/design-assets/" + url.PathEscape(designRef) + "/implementation-context"
	if err := a.client.PostJSON(ctx, endpoint, body, &contextValue); err != nil {
		return nil, mapDesignMCPAPIError(err)
	}
	if contextValue.SchemaVersion != "multica.design-implementation-context/v1" || contextValue.DesignRef != designRef ||
		contextValue.RevisionID != revisionID || contextValue.ProjectResourceID != repositoryID || contextValue.IssueID != issueID ||
		!equalStrings(contextValue.FrameRefs, frameRefs) {
		return nil, fmt.Errorf("design context response does not match the requested frozen identity")
	}
	if err := materializeDesignImplementationContext(a.rootDir, contextValue); err != nil {
		return nil, fmt.Errorf("context_materialization_failed: %w", err)
	}
	return map[string]any{
		"schema_version":          contextValue.SchemaVersion,
		"context_path":            designImplementationContextPath,
		"design_manifest_path":    designImplementationManifestPath,
		"design_package_path":     designImplementationPackagePath,
		"scope_path":              designImplementationScopePath,
		"repository_context_path": designImplementationRepositoryPath,
		"result_path":             designImplementationResultPath,
		"source_capabilities":     contextValue.Capabilities,
	}, nil
}

func materializeDesignImplementationContext(rootDir string, contextValue designImplementationContextWire) error {
	if rootDir == "" {
		rootDir = "."
	}
	root, err := os.OpenRoot(rootDir)
	if err != nil {
		return err
	}
	defer root.Close()
	for _, directory := range []string{
		".agent_context/design_implementation/design/package",
		".agent_context/design_implementation/repository",
		".agent_context/design_implementation/result",
	} {
		if err := root.MkdirAll(directory, 0o755); err != nil {
			return err
		}
	}
	manifest := map[string]any{
		"schema_version": contextValue.SchemaVersion,
		"design_ref":     contextValue.DesignRef, "revision_id": contextValue.RevisionID,
		"content_digest": contextValue.ContentDigest, "title": contextValue.DesignTitle,
		"design_system_digest": contextValue.DesignSystemDigest, "source_capabilities": contextValue.Capabilities,
	}
	scope := map[string]any{
		"design_ref": contextValue.DesignRef, "revision_id": contextValue.RevisionID,
		"frame_refs": contextValue.FrameRefs, "project_id": contextValue.ProjectID,
		"issue_id": contextValue.IssueID, "project_resource_id": contextValue.ProjectResourceID,
		"allowed_write_paths": contextValue.AllowedWritePaths, "verification_requirements": contextValue.Verification,
	}
	for relative, value := range map[string]any{
		designImplementationContextPath:  contextValue,
		designImplementationManifestPath: manifest,
		designImplementationScopePath:    scope,
	} {
		raw, err := json.MarshalIndent(value, "", "  ")
		if err != nil {
			return err
		}
		raw = append(raw, '\n')
		if err := root.WriteFile(relative, raw, 0o644); err != nil {
			return err
		}
	}
	return nil
}

func stringSliceArgument(arguments map[string]any, key string) ([]string, error) {
	raw, ok := arguments[key]
	if !ok {
		return nil, fmt.Errorf("%s is required", key)
	}
	switch values := raw.(type) {
	case []string:
		return append([]string(nil), values...), nil
	case []any:
		out := make([]string, len(values))
		for i, value := range values {
			var ok bool
			out[i], ok = value.(string)
			if !ok || out[i] == "" {
				return nil, fmt.Errorf("%s must contain non-empty strings", key)
			}
		}
		return out, nil
	default:
		return nil, fmt.Errorf("%s must be an array", key)
	}
}

func equalStrings(left, right []string) bool {
	if len(left) != len(right) {
		return false
	}
	for i := range left {
		if left[i] != right[i] {
			return false
		}
	}
	return true
}

func (a *designMCPAdapter) getRestorePack(ctx context.Context, arguments map[string]any) (any, error) {
	scope, ok := arguments["scope"].(map[string]any)
	if !ok {
		return nil, fmt.Errorf("scope is required")
	}
	designFileID := stringArgument(scope, "designFileId")
	if designFileID == "" {
		return nil, fmt.Errorf("scope.designFileId is required")
	}
	body := map[string]any{"scope": scope}
	if detailLevel := stringArgument(arguments, "detailLevel"); detailLevel != "" {
		body["detailLevel"] = detailLevel
	}
	var out map[string]any
	path := "/api/design-files/" + url.PathEscape(designFileID) + "/restore-pack"
	if err := a.client.PostJSON(ctx, path, body, &out); err != nil {
		return nil, mapDesignMCPAPIError(err)
	}
	return out, nil
}

func (a *designMCPAdapter) listFiles(ctx context.Context) (any, error) {
	var out map[string]any
	if err := a.client.GetJSON(ctx, "/api/design-files", &out); err != nil {
		return nil, mapDesignMCPAPIError(err)
	}
	return out, nil
}

func (a *designMCPAdapter) listFrames(ctx context.Context, arguments map[string]any) (any, error) {
	designFileID := stringArgument(arguments, "designFileId")
	if designFileID == "" {
		return nil, fmt.Errorf("designFileId is required")
	}
	path := "/api/design-files/" + url.PathEscape(designFileID) + "/context"
	if revisionID := stringArgument(arguments, "revisionId"); revisionID != "" {
		path += "?revision_id=" + url.QueryEscape(revisionID)
	}
	var out map[string]any
	if err := a.client.GetJSON(ctx, path, &out); err != nil {
		return nil, mapDesignMCPAPIError(err)
	}
	return map[string]any{
		"designFileId": out["designFileId"],
		"revisionId":   out["revisionId"],
		"frames":       out["frames"],
	}, nil
}

func (a *designMCPAdapter) listGroups(ctx context.Context, arguments map[string]any) (any, error) {
	frames, err := a.listFrames(ctx, arguments)
	if err != nil {
		return nil, err
	}
	return map[string]any{
		"groups":  []any{},
		"frames":  frames,
		"message": "Group scope is available from Design Center copied scope JSON. Use multica_design_get_restore_pack with kind=figma_group when you have groupId/frameIds.",
	}, nil
}

func (a *designMCPAdapter) getSelectionContext(ctx context.Context, arguments map[string]any) (any, error) {
	designFileID := stringArgument(arguments, "designFileId")
	frameID := stringArgument(arguments, "frameId")
	if designFileID == "" {
		return nil, fmt.Errorf("designFileId is required")
	}
	if frameID == "" {
		return nil, fmt.Errorf("frameId is required")
	}
	body := map[string]any{}
	for _, key := range []string{"layerIds", "selectionBounds", "includeIntersectingLayers"} {
		if value, ok := arguments[key]; ok {
			body[key] = value
		}
	}
	path := "/api/design-files/" + url.PathEscape(designFileID) + "/frames/" + url.PathEscape(frameID) + "/selection-context"
	if revisionID := stringArgument(arguments, "revisionId"); revisionID != "" {
		path += "?revision_id=" + url.QueryEscape(revisionID)
	}
	var out map[string]any
	if err := a.client.PostJSON(ctx, path, body, &out); err != nil {
		return nil, mapDesignMCPAPIError(err)
	}
	return out, nil
}

func (a *designMCPAdapter) getUIRestoreArtifact(arguments map[string]any) any {
	artifactDocPath := stringArgument(arguments, "artifactDocPath")
	projectLocalPath := stringArgument(arguments, "projectLocalPath")
	return map[string]any{
		"artifactDocPath":  artifactDocPath,
		"projectLocalPath": projectLocalPath,
		"message":          "Read artifactDocPath from the local target repository and use it as the UI restore handoff document.",
	}
}

func stringArgument(values map[string]any, key string) string {
	value, _ := values[key].(string)
	return value
}

func mapDesignMCPAPIError(err error) error {
	var httpErr *cli.HTTPError
	if errors.As(err, &httpErr) && httpErr.StatusCode == 401 {
		return fmt.Errorf("not_authenticated: run 'multica login'")
	}
	return err
}
