package designcore

import (
	"fmt"
	"strings"
)

type ValidationResult struct {
	Valid  bool     `json:"valid"`
	Errors []string `json:"errors"`
}

var allowedRequirementPageTypes = map[string]struct{}{
	"saas.filter-table-pagination": {},
	"saas.form-page":               {},
	"saas.detail-page":             {},
}

var forbiddenPatchPathSegments = map[string]struct{}{
	"x":        {},
	"y":        {},
	"width":    {},
	"height":   {},
	"children": {},
}

func ValidateNativeJSON(raw []byte) ValidationResult {
	doc, err := ParseNativeJSON(raw)
	if err != nil {
		return ValidationResult{Valid: false, Errors: []string{err.Error()}}
	}
	return ValidateDocument(doc)
}

func ValidateDocument(doc NativeJSON) ValidationResult {
	errors := make([]string, 0)

	if doc.Version != NativeJSONVersion {
		errors = append(errors, fmt.Sprintf("version must be %q", NativeJSONVersion))
	}
	if doc.File.Title == "" {
		errors = append(errors, "file.title is required")
	}
	if doc.File.SourceType == "" {
		errors = append(errors, "file.sourceType is required")
	}
	if len(doc.Frames) == 0 {
		errors = append(errors, "frames must contain at least one frame")
	}
	if len(doc.Layers) == 0 {
		errors = append(errors, "layers must contain at least one layer")
	}

	frameIDs := make(map[string]struct{}, len(doc.Frames))
	for _, frame := range doc.Frames {
		if frame.ID == "" {
			errors = append(errors, "frame.id is required")
			continue
		}
		frameIDs[frame.ID] = struct{}{}
		if frame.RootLayerID == "" {
			errors = append(errors, fmt.Sprintf("frame %s rootLayerId is required", frame.ID))
		} else if _, ok := doc.Layers[frame.RootLayerID]; !ok {
			errors = append(errors, fmt.Sprintf("frame %s references missing root layer %s", frame.ID, frame.RootLayerID))
		}
		if frame.Width < 0 || frame.Height < 0 {
			errors = append(errors, fmt.Sprintf("frame %s has negative size", frame.ID))
		}
	}

	assetIDs := make(map[string]struct{}, len(doc.Assets))
	for id, asset := range doc.Assets {
		assetIDs[id] = struct{}{}
		if asset.ID != "" && asset.ID != id {
			errors = append(errors, fmt.Sprintf("asset map key %s does not match asset.id %s", id, asset.ID))
		}
		if asset.URL == "" {
			errors = append(errors, fmt.Sprintf("asset %s url is required", id))
		}
	}

	for _, frame := range doc.Frames {
		if frame.PreviewAssetID != "" {
			if _, ok := assetIDs[frame.PreviewAssetID]; !ok {
				errors = append(errors, fmt.Sprintf("frame %s references missing preview asset %s", frame.ID, frame.PreviewAssetID))
			}
		}
		if frame.ThumbnailAssetID != "" {
			if _, ok := assetIDs[frame.ThumbnailAssetID]; !ok {
				errors = append(errors, fmt.Sprintf("frame %s references missing thumbnail asset %s", frame.ID, frame.ThumbnailAssetID))
			}
		}
	}

	for id, layer := range doc.Layers {
		if layer.ID != id {
			errors = append(errors, fmt.Sprintf("layer map key %s does not match layer.id %s", id, layer.ID))
		}
		if _, ok := frameIDs[layer.FrameID]; !ok {
			errors = append(errors, fmt.Sprintf("layer %s references missing frame %s", id, layer.FrameID))
		}
		if layer.ParentID != "" {
			if _, ok := doc.Layers[layer.ParentID]; !ok {
				errors = append(errors, fmt.Sprintf("layer %s references missing parent %s", id, layer.ParentID))
			}
		}
		for _, childID := range layer.Children {
			child, ok := doc.Layers[childID]
			if !ok {
				errors = append(errors, fmt.Sprintf("layer %s references missing child %s", id, childID))
				continue
			}
			if child.ParentID != id {
				errors = append(errors, fmt.Sprintf("layer %s child %s has parent %s", id, childID, child.ParentID))
			}
		}
		if layer.Image != nil && layer.Image.AssetID != "" {
			if _, ok := assetIDs[layer.Image.AssetID]; !ok {
				errors = append(errors, fmt.Sprintf("layer %s references missing asset %s", id, layer.Image.AssetID))
			}
		}
		if layer.Width < 0 || layer.Height < 0 {
			errors = append(errors, fmt.Sprintf("layer %s has negative size", id))
		}
	}

	for slotKey, slot := range doc.Slots {
		for _, layerID := range slot.LayerIDs {
			if _, ok := doc.Layers[layerID]; !ok {
				errors = append(errors, fmt.Sprintf("slot %s references missing layer %s", slotKey, layerID))
			}
		}
	}

	for layerID := range doc.ComponentBindings {
		if _, ok := doc.Layers[layerID]; !ok {
			errors = append(errors, fmt.Sprintf("component binding references missing layer %s", layerID))
		}
	}

	return ValidationResult{Valid: len(errors) == 0, Errors: errors}
}

func ValidateRequirementCore(raw []byte) ValidationResult {
	requirement, err := ParseRequirementCore(raw)
	if err != nil {
		return ValidationResult{Valid: false, Errors: []string{err.Error()}}
	}
	return ValidateRequirement(requirement)
}

func ValidateRequirement(requirement RequirementCore) ValidationResult {
	errors := make([]string, 0)
	if requirement.Version != NativeJSONVersion {
		errors = append(errors, fmt.Sprintf("version must be %q", NativeJSONVersion))
	}
	if requirement.Title == "" {
		errors = append(errors, "title is required")
	}
	if _, ok := allowedRequirementPageTypes[requirement.PageType]; !ok {
		errors = append(errors, fmt.Sprintf("pageType %q is not supported", requirement.PageType))
	}
	if requirement.Entity.Key == "" {
		errors = append(errors, "entity.key is required")
	}
	if requirement.Entity.Label == "" {
		errors = append(errors, "entity.label is required")
	}
	for index, field := range requirement.Fields {
		if field.Key == "" || field.Label == "" {
			errors = append(errors, fmt.Sprintf("fields[%d] key and label are required", index))
		}
	}
	return ValidationResult{Valid: len(errors) == 0, Errors: errors}
}

func ValidateSlotValues(raw []byte) ValidationResult {
	var values map[string]any
	if err := jsonUnmarshalObject(raw, &values); err != nil {
		return ValidationResult{Valid: false, Errors: []string{err.Error()}}
	}
	return ValidationResult{Valid: true, Errors: []string{}}
}

func ValidatePatchOperations(raw []byte) ValidationResult {
	operations, err := ParsePatchOperations(raw)
	if err != nil {
		return ValidationResult{Valid: false, Errors: []string{err.Error()}}
	}

	errors := make([]string, 0)
	for index, operation := range operations {
		if operation.Op != "add" && operation.Op != "replace" && operation.Op != "remove" {
			errors = append(errors, fmt.Sprintf("patch[%d].op %q is not supported", index, operation.Op))
		}
		if !strings.HasPrefix(operation.Path, "/") {
			errors = append(errors, fmt.Sprintf("patch[%d].path must start with /", index))
		}
		for _, segment := range strings.Split(operation.Path, "/") {
			if _, forbidden := forbiddenPatchPathSegments[segment]; forbidden {
				errors = append(errors, fmt.Sprintf("patch[%d].path %s changes layout or tree structure and is not allowed in MVP", index, operation.Path))
				break
			}
		}
	}
	return ValidationResult{Valid: len(errors) == 0, Errors: errors}
}
