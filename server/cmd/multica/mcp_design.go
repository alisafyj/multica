package main

import (
	"context"
	"errors"
	"fmt"
	"net/url"

	"github.com/multica-ai/multica/server/internal/cli"
)

type designMCPAdapter struct {
	client *cli.APIClient
}

func designMCPToolDescriptors() []map[string]any {
	return []map[string]any{
		designMCPToolDescriptor("multica_design_get_restore_pack", "Get a server-built Restore Pack for a design restore scope."),
		designMCPToolDescriptor("multica_design_list_files", "List design files available in the configured workspace."),
		designMCPToolDescriptor("multica_design_list_frames", "List frames for one design file revision."),
		designMCPToolDescriptor("multica_design_list_groups", "List Figma groups for one design file revision."),
		designMCPToolDescriptor("multica_design_get_selection_context", "Get selected layer or bounds context for one design frame."),
		designMCPToolDescriptor("multica_design_get_ui_restore_artifact", "Return UI restore artifact path metadata for frontend implementation."),
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
	default:
		return nil, fmt.Errorf("unknown design MCP tool %q", name)
	}
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
