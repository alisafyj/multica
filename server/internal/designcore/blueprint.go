package designcore

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"sort"
	"strings"
)

const TemplateBlueprintVersion = "1.0"

type Rect struct {
	X      float64 `json:"x"`
	Y      float64 `json:"y"`
	Width  float64 `json:"width"`
	Height float64 `json:"height"`
}

type StructuralFrame struct {
	ID          string `json:"id"`
	RootLayerID string `json:"rootLayerId"`
	Name        string `json:"name"`
	Bounds      Rect   `json:"bounds"`
}

type StructuralLayer struct {
	ID           string         `json:"id"`
	FrameID      string         `json:"frameId"`
	ParentID     string         `json:"parentId,omitempty"`
	Name         string         `json:"name"`
	Type         string         `json:"type"`
	ComponentKey string         `json:"componentKey,omitempty"`
	Text         string         `json:"text,omitempty"`
	Children     []string       `json:"children,omitempty"`
	Bounds       Rect           `json:"bounds"`
	Layout       map[string]any `json:"layout,omitempty"`
}

type TemplateStructure struct {
	Frames         map[string]StructuralFrame `json:"frames"`
	Layers         map[string]StructuralLayer `json:"layers"`
	HiddenLayerIDs []string                   `json:"hiddenLayerIds,omitempty"`
}

type RegionClassification struct {
	RootLayerID     string            `json:"rootLayerId"`
	ReplaceChildren bool              `json:"replaceChildren"`
	Bindings        map[string]string `json:"bindings,omitempty"`
}

type BlueprintRegion struct {
	RootLayerID     string            `json:"rootLayerId"`
	ReplaceChildren bool              `json:"replaceChildren"`
	Bounds          Rect              `json:"bounds"`
	Bindings        map[string]string `json:"bindings,omitempty"`
}

type PrototypeClassification struct {
	RootLayerID string            `json:"rootLayerId"`
	Bindings    map[string]string `json:"bindings,omitempty"`
}

type BlueprintPrototype struct {
	RootLayerID string            `json:"rootLayerId"`
	Bounds      Rect              `json:"bounds"`
	Bindings    map[string]string `json:"bindings,omitempty"`
}

type BlueprintConstraints struct {
	ContentWidth      float64 `json:"contentWidth"`
	FilterRowHeight   float64 `json:"filterRowHeight"`
	TableHeaderHeight float64 `json:"tableHeaderHeight"`
	TableRowHeight    float64 `json:"tableRowHeight"`
	HorizontalGap     float64 `json:"horizontalGap"`
	VerticalGap       float64 `json:"verticalGap"`
	FilterColumns     int     `json:"filterColumns"`
	PinFirstColumn    bool    `json:"pinFirstColumn"`
	PinActionColumn   bool    `json:"pinActionColumn"`
}

type BlueprintSourceRefs struct {
	DesignFileID       string `json:"designFileId"`
	DesignRevisionID   string `json:"designRevisionId"`
	TemplateRevisionID string `json:"templateRevisionId"`
}

type BlueprintClassification struct {
	FrameID                string                             `json:"frameId"`
	PageType               string                             `json:"pageType"`
	Regions                map[string]RegionClassification    `json:"regions"`
	Prototypes             map[string]PrototypeClassification `json:"prototypes"`
	Constraints            BlueprintConstraints               `json:"constraints"`
	ShellAllowlistLayerIDs []string                           `json:"shellAllowlistLayerIds,omitempty"`
}

type TemplateBlueprint struct {
	Version                string                        `json:"version"`
	FrameID                string                        `json:"frameId"`
	PageType               string                        `json:"pageType"`
	Regions                map[string]BlueprintRegion    `json:"regions"`
	Prototypes             map[string]BlueprintPrototype `json:"prototypes"`
	Constraints            BlueprintConstraints          `json:"constraints"`
	ShellAllowlistLayerIDs []string                      `json:"shellAllowlistLayerIds,omitempty"`
	SourceRefs             BlueprintSourceRefs           `json:"sourceRefs"`
}

var requiredBlueprintRegions = []string{"shell", "content", "breadcrumb", "pageTitle", "filters", "pageActions", "table", "pagination"}

var requiredBlueprintPrototypes = []string{"pageTitle", "breadcrumbItem", "tableHeaderCell", "tableRow"}

var replaceableBusinessRegions = []string{"breadcrumb", "pageTitle", "filters", "pageActions", "table", "pagination"}

func ExtractTemplateStructure(doc NativeJSON) TemplateStructure {
	structure := TemplateStructure{
		Frames: make(map[string]StructuralFrame, len(doc.Frames)),
		Layers: make(map[string]StructuralLayer, len(doc.Layers)),
	}
	for _, frame := range doc.Frames {
		structure.Frames[frame.ID] = StructuralFrame{
			ID:          frame.ID,
			RootLayerID: frame.RootLayerID,
			Name:        frame.Name,
			Bounds:      Rect{X: frame.X, Y: frame.Y, Width: frame.Width, Height: frame.Height},
		}
	}

	hidden := effectiveHiddenLayers(doc.Layers)
	structure.HiddenLayerIDs = make([]string, 0, len(hidden))
	for id := range hidden {
		structure.HiddenLayerIDs = append(structure.HiddenLayerIDs, id)
	}
	sort.Strings(structure.HiddenLayerIDs)

	for id, layer := range doc.Layers {
		if hidden[id] {
			continue
		}
		children := make([]string, 0, len(layer.Children))
		for _, childID := range layer.Children {
			if !hidden[childID] {
				children = append(children, childID)
			}
		}
		componentKey := ""
		if binding, ok := doc.ComponentBindings[id]; ok {
			componentKey = binding.ComponentKey
		}
		structure.Layers[id] = StructuralLayer{
			ID:           layer.ID,
			FrameID:      layer.FrameID,
			ParentID:     layer.ParentID,
			Name:         layer.Name,
			Type:         layer.Type,
			ComponentKey: componentKey,
			Text:         structuralLayerText(layer),
			Children:     children,
			Bounds:       Rect{X: layer.X, Y: layer.Y, Width: layer.Width, Height: layer.Height},
			Layout:       structuralLayerLayout(layer),
		}
	}

	return structure
}

func BuildTemplateBlueprint(structure TemplateStructure, classification BlueprintClassification, refs BlueprintSourceRefs) (TemplateBlueprint, Diagnostics) {
	blueprint := TemplateBlueprint{
		Version:                TemplateBlueprintVersion,
		FrameID:                classification.FrameID,
		PageType:               classification.PageType,
		Regions:                make(map[string]BlueprintRegion, len(classification.Regions)),
		Prototypes:             make(map[string]BlueprintPrototype, len(classification.Prototypes)),
		Constraints:            classification.Constraints,
		ShellAllowlistLayerIDs: append([]string(nil), classification.ShellAllowlistLayerIDs...),
		SourceRefs:             refs,
	}
	for key, region := range classification.Regions {
		blueprint.Regions[key] = BlueprintRegion{
			RootLayerID:     region.RootLayerID,
			ReplaceChildren: region.ReplaceChildren,
			Bounds:          structure.Layers[region.RootLayerID].Bounds,
			Bindings:        copyStringMap(region.Bindings),
		}
	}
	for key, prototype := range classification.Prototypes {
		blueprint.Prototypes[key] = BlueprintPrototype{
			RootLayerID: prototype.RootLayerID,
			Bounds:      structure.Layers[prototype.RootLayerID].Bounds,
			Bindings:    copyStringMap(prototype.Bindings),
		}
	}
	return blueprint, ValidateTemplateBlueprint(structure, blueprint)
}

func ParseTemplateBlueprint(raw []byte) (TemplateBlueprint, error) {
	var blueprint TemplateBlueprint
	if err := decodeStrictJSON(raw, &blueprint, "template blueprint"); err != nil {
		return TemplateBlueprint{}, err
	}
	return blueprint, nil
}

func ValidateTemplateBlueprint(structure TemplateStructure, blueprint TemplateBlueprint) Diagnostics {
	return validateTemplateBlueprint(structure, blueprint, false)
}

// ValidateTemplateBlueprintForPageSpec enforces the PageSpec-dependent navigation contract.
func ValidateTemplateBlueprintForPageSpec(structure TemplateStructure, blueprint TemplateBlueprint, spec PageSpec) Diagnostics {
	return validateTemplateBlueprint(structure, blueprint, strings.TrimSpace(spec.Page.ActiveNavigation) != "")
}

func validateTemplateBlueprint(structure TemplateStructure, blueprint TemplateBlueprint, navigationRequired bool) Diagnostics {
	diagnostics := Diagnostics{}
	if blueprint.Version != TemplateBlueprintVersion {
		diagnostics.addError("unsupported_version", fmt.Sprintf("version must be %q", TemplateBlueprintVersion), "version")
	}
	if _, ok := structure.Frames[blueprint.FrameID]; !ok {
		diagnostics.addError("unknown_frame", fmt.Sprintf("frame %q does not exist", blueprint.FrameID), "frameId")
	}
	if blueprint.PageType != "list" {
		diagnostics.addError("unsupported_page_type", fmt.Sprintf("page type %q is not supported", blueprint.PageType), "pageType")
	}
	for _, key := range requiredBlueprintRegions {
		if _, ok := blueprint.Regions[key]; !ok {
			diagnostics.addError("missing_region", fmt.Sprintf("required region %q is missing", key), "regions."+key)
		}
	}
	if navigationRequired {
		if _, ok := blueprint.Regions["navigation"]; !ok {
			diagnostics.addError("missing_navigation_region", "navigation region is required for an active navigation item", "regions.navigation")
		}
	}
	for _, key := range requiredBlueprintPrototypes {
		if _, ok := blueprint.Prototypes[key]; !ok {
			diagnostics.addError("missing_prototype", fmt.Sprintf("required prototype %q is missing", key), "prototypes."+key)
		}
	}

	hidden := make(map[string]struct{}, len(structure.HiddenLayerIDs))
	for _, id := range structure.HiddenLayerIDs {
		hidden[id] = struct{}{}
	}
	for key, region := range blueprint.Regions {
		path := "regions." + key
		if validateBlueprintLayerReference(&diagnostics, structure, hidden, blueprint.FrameID, region.RootLayerID, path) {
			validateBlueprintBindings(&diagnostics, structure, hidden, blueprint.FrameID, region.RootLayerID, region.Bindings, path)
		}
	}
	for key, prototype := range blueprint.Prototypes {
		path := "prototypes." + key
		if validateBlueprintLayerReference(&diagnostics, structure, hidden, blueprint.FrameID, prototype.RootLayerID, path) {
			validateBlueprintBindings(&diagnostics, structure, hidden, blueprint.FrameID, prototype.RootLayerID, prototype.Bindings, path)
		}
	}
	validateBlueprintRegionRelationships(&diagnostics, structure, blueprint)
	validateBlueprintShellAllowlist(&diagnostics, structure, hidden, blueprint)
	validateBlueprintConstraints(&diagnostics, blueprint.Constraints)
	return diagnostics
}

func validateBlueprintLayerReference(diagnostics *Diagnostics, structure TemplateStructure, hidden map[string]struct{}, frameID, layerID, path string) bool {
	if _, ok := hidden[layerID]; ok {
		addLayerDiagnostic(diagnostics, "hidden_source_layer", fmt.Sprintf("source layer %q is hidden", layerID), path, layerID)
		return false
	}
	layer, ok := structure.Layers[layerID]
	if !ok {
		addLayerDiagnostic(diagnostics, "unknown_source_layer", fmt.Sprintf("source layer %q does not exist", layerID), path, layerID)
		return false
	}
	if layer.FrameID != frameID {
		addLayerDiagnostic(diagnostics, "cross_frame_reference", fmt.Sprintf("source layer %q belongs to frame %q", layerID, layer.FrameID), path, layerID)
		return false
	}
	return true
}

func validateBlueprintBindings(diagnostics *Diagnostics, structure TemplateStructure, hidden map[string]struct{}, frameID, rootID string, bindings map[string]string, path string) {
	for key, targetID := range bindings {
		bindingPath := path + ".bindings." + key
		if !validateBlueprintLayerReference(diagnostics, structure, hidden, frameID, targetID, bindingPath) {
			continue
		}
		target := structure.Layers[targetID]
		if !isStructuralDescendantOrSelf(structure.Layers, targetID, rootID) || !strings.EqualFold(target.Type, "text") {
			addLayerDiagnostic(diagnostics, "invalid_binding", fmt.Sprintf("binding %q must target a text descendant of %q", key, rootID), bindingPath, targetID)
		}
	}
}

func validateBlueprintRegionRelationships(diagnostics *Diagnostics, structure TemplateStructure, blueprint TemplateBlueprint) {
	content, hasContent := blueprint.Regions["content"]
	if !hasContent {
		return
	}
	businessRegions := make([]struct {
		key string
		id  string
	}, 0, len(replaceableBusinessRegions))
	for _, key := range replaceableBusinessRegions {
		region, ok := blueprint.Regions[key]
		if !ok {
			continue
		}
		if !region.ReplaceChildren {
			diagnostics.addError("invalid_region_relationship", fmt.Sprintf("business region %q must replace its children", key), "regions."+key+".replaceChildren")
		}
		if !isStructuralDescendant(structure.Layers, region.RootLayerID, content.RootLayerID) {
			addLayerDiagnostic(diagnostics, "invalid_region_relationship", fmt.Sprintf("business region %q must descend from content", key), "regions."+key, region.RootLayerID)
		}
		businessRegions = append(businessRegions, struct {
			key string
			id  string
		}{key: key, id: region.RootLayerID})
	}
	for i := range businessRegions {
		for j := i + 1; j < len(businessRegions); j++ {
			if isStructuralDescendantOrSelf(structure.Layers, businessRegions[i].id, businessRegions[j].id) || isStructuralDescendantOrSelf(structure.Layers, businessRegions[j].id, businessRegions[i].id) {
				diagnostics.addError("invalid_region_relationship", fmt.Sprintf("business regions %q and %q must not be nested", businessRegions[i].key, businessRegions[j].key), "regions."+businessRegions[i].key, "regions."+businessRegions[j].key)
			}
		}
	}
}

func validateBlueprintShellAllowlist(diagnostics *Diagnostics, structure TemplateStructure, hidden map[string]struct{}, blueprint TemplateBlueprint) {
	shell, ok := blueprint.Regions["shell"]
	if !ok {
		return
	}
	for index, layerID := range blueprint.ShellAllowlistLayerIDs {
		path := fmt.Sprintf("shellAllowlistLayerIds.%d", index)
		if !validateBlueprintLayerReference(diagnostics, structure, hidden, blueprint.FrameID, layerID, path) {
			continue
		}
		if !isStructuralDescendant(structure.Layers, layerID, shell.RootLayerID) {
			addLayerDiagnostic(diagnostics, "unsafe_shell_allowlist", fmt.Sprintf("allowlisted layer %q must descend from shell", layerID), path, layerID)
		}
	}
}

func validateBlueprintConstraints(diagnostics *Diagnostics, constraints BlueprintConstraints) {
	values := []struct {
		path  string
		value float64
	}{
		{path: "constraints.contentWidth", value: constraints.ContentWidth},
		{path: "constraints.filterRowHeight", value: constraints.FilterRowHeight},
		{path: "constraints.tableHeaderHeight", value: constraints.TableHeaderHeight},
		{path: "constraints.tableRowHeight", value: constraints.TableRowHeight},
		{path: "constraints.horizontalGap", value: constraints.HorizontalGap},
		{path: "constraints.verticalGap", value: constraints.VerticalGap},
	}
	for _, item := range values {
		if item.value <= 0 {
			diagnostics.addError("invalid_constraint", fmt.Sprintf("%s must be positive", item.path), item.path)
		}
	}
	if constraints.FilterColumns < 1 || constraints.FilterColumns > 6 {
		diagnostics.addError("invalid_constraint", "constraints.filterColumns must be between 1 and 6", "constraints.filterColumns")
	}
}

func effectiveHiddenLayers(layers map[string]Layer) map[string]bool {
	hidden := make(map[string]bool)
	var markDescendants func(string)
	markDescendants = func(id string) {
		if hidden[id] {
			return
		}
		hidden[id] = true
		for _, childID := range layers[id].Children {
			markDescendants(childID)
		}
	}
	for id, layer := range layers {
		if !layer.Visible {
			markDescendants(id)
		}
	}
	return hidden
}

func structuralLayerText(layer Layer) string {
	for _, key := range []string{"characters", "text"} {
		if value, ok := layer.Text[key].(string); ok {
			return value
		}
	}
	return ""
}

func structuralLayerLayout(layer Layer) map[string]any {
	layout := map[string]any{
		"rotation": layer.Rotation,
		"opacity":  layer.Opacity,
	}
	if sourceLayout, ok := layer.Source["layout"].(map[string]any); ok {
		for key, value := range sourceLayout {
			layout[key] = cloneJSONValue(value)
		}
	}
	return layout
}

func isStructuralDescendant(layers map[string]StructuralLayer, layerID, ancestorID string) bool {
	if layerID == ancestorID {
		return false
	}
	return isStructuralDescendantOrSelf(layers, layerID, ancestorID)
}

func isStructuralDescendantOrSelf(layers map[string]StructuralLayer, layerID, ancestorID string) bool {
	visited := map[string]struct{}{}
	for current := layerID; current != ""; {
		if current == ancestorID {
			return true
		}
		if _, seen := visited[current]; seen {
			return false
		}
		visited[current] = struct{}{}
		layer, ok := layers[current]
		if !ok {
			return false
		}
		current = layer.ParentID
	}
	return false
}

func addLayerDiagnostic(diagnostics *Diagnostics, code, message, path, layerID string) {
	*diagnostics = append(*diagnostics, Diagnostic{Code: code, Severity: DiagnosticError, Message: message, Paths: []string{path}, LayerIDs: []string{layerID}})
}

func copyStringMap(source map[string]string) map[string]string {
	if source == nil {
		return nil
	}
	result := make(map[string]string, len(source))
	for key, value := range source {
		result[key] = value
	}
	return result
}

func cloneJSONValue(value any) any {
	raw, err := json.Marshal(value)
	if err != nil {
		return value
	}
	var cloned any
	if err := json.Unmarshal(raw, &cloned); err != nil {
		return value
	}
	return cloned
}

func decodeStrictJSON(raw []byte, target any, name string) error {
	decoder := json.NewDecoder(bytes.NewReader(raw))
	decoder.DisallowUnknownFields()
	if err := decoder.Decode(target); err != nil {
		return err
	}
	var trailing any
	if err := decoder.Decode(&trailing); !errors.Is(err, io.EOF) {
		if err == nil {
			return fmt.Errorf("%s must contain exactly one JSON value", name)
		}
		return err
	}
	return nil
}
