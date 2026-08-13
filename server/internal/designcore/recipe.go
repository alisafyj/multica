package designcore

import (
	"bytes"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"sort"
	"strings"
)

const ComponentRecipeSetVersion = "1.0"

type RecipeKey struct {
	Kind    string `json:"kind"`
	Variant string `json:"variant"`
	State   string `json:"state"`
}

type RecipeSource struct {
	RevisionID  string `json:"revisionId"`
	RootLayerID string `json:"rootLayerId"`
	Fingerprint string `json:"fingerprint"`
}

type RecipeProp struct {
	TargetLayerID string `json:"targetLayerId"`
	Type          string `json:"type"`
}

type RecipeLayout struct {
	WidthMode    string  `json:"widthMode"`
	TextOverflow string  `json:"textOverflow"`
	Height       float64 `json:"height"`
	MinWidth     float64 `json:"minWidth"`
	OverlayRole  string  `json:"overlayRole,omitempty"`
}

type ComponentRecipe struct {
	Kind    string                `json:"kind"`
	Variant string                `json:"variant"`
	State   string                `json:"state"`
	Source  RecipeSource          `json:"source"`
	Props   map[string]RecipeProp `json:"props"`
	Layout  RecipeLayout          `json:"layout"`
}

type PrimitiveRecipe struct {
	Kind      string                `json:"kind"`
	LayerType string                `json:"layerType"`
	Props     map[string]RecipeProp `json:"props"`
	Style     map[string]any        `json:"style"`
	Layout    RecipeLayout          `json:"layout"`
}

type ComponentRecipeSet struct {
	Version               string                     `json:"version"`
	DesignSystemProfileID string                     `json:"designSystemProfileId"`
	SourceRevisionID      string                     `json:"sourceRevisionId"`
	Tokens                map[string]any             `json:"tokens"`
	Recipes               map[string]ComponentRecipe `json:"recipes"`
	PrimitiveFallbacks    map[string]PrimitiveRecipe `json:"primitiveFallbacks"`
}

type ComponentRecipeClassification struct {
	Kind        string                `json:"kind"`
	Variant     string                `json:"variant"`
	State       string                `json:"state"`
	RootLayerID string                `json:"rootLayerId"`
	Props       map[string]RecipeProp `json:"props"`
	Layout      RecipeLayout          `json:"layout"`
}

type RecipeRequest struct {
	Kind    string `json:"kind"`
	Variant string `json:"variant"`
	State   string `json:"state"`
}

type ResolvedRecipe struct {
	Recipe    *ComponentRecipe `json:"recipe,omitempty"`
	Primitive *PrimitiveRecipe `json:"primitive,omitempty"`
	Fallback  string           `json:"fallback"`
}

var requiredRecipeKinds = []string{"input", "select", "date-range", "primary-button", "secondary-button", "text-button", "table-header", "table-row", "status-tag", "pagination"}

func (k RecipeKey) String() string {
	return k.Kind + "/" + k.Variant + "/" + k.State
}

func BuildComponentRecipeSet(profileID, sourceRevisionID, version string, source NativeJSON, classifications []ComponentRecipeClassification, primitiveFallbacks map[string]PrimitiveRecipe) (ComponentRecipeSet, Diagnostics) {
	set := ComponentRecipeSet{
		Version:               version,
		DesignSystemProfileID: profileID,
		SourceRevisionID:      sourceRevisionID,
		Tokens:                cloneJSONMap(source.Tokens),
		Recipes:               make(map[string]ComponentRecipe, len(classifications)),
		PrimitiveFallbacks:    clonePrimitiveRecipes(primitiveFallbacks),
	}
	for _, classification := range classifications {
		recipe := ComponentRecipe{
			Kind:    classification.Kind,
			Variant: classification.Variant,
			State:   classification.State,
			Source: RecipeSource{
				RevisionID:  sourceRevisionID,
				RootLayerID: classification.RootLayerID,
				Fingerprint: fingerprintRecipeSource(source, classification.RootLayerID),
			},
			Props:  cloneRecipeProps(classification.Props),
			Layout: classification.Layout,
		}
		set.Recipes[(RecipeKey{Kind: recipe.Kind, Variant: recipe.Variant, State: recipe.State}).String()] = recipe
	}
	return set, ValidateComponentRecipeSet(source, set)
}

func ParseComponentRecipeSet(raw []byte) (ComponentRecipeSet, error) {
	var set ComponentRecipeSet
	if err := decodeStrictJSON(raw, &set, "component recipe set"); err != nil {
		return ComponentRecipeSet{}, err
	}
	return set, nil
}

func ValidateComponentRecipeSet(source NativeJSON, set ComponentRecipeSet) Diagnostics {
	diagnostics := Diagnostics{}
	if set.Version != ComponentRecipeSetVersion {
		diagnostics.addError("unsupported_version", fmt.Sprintf("version must be %q", ComponentRecipeSetVersion), "version")
	}
	if set.DesignSystemProfileID == "" || set.SourceRevisionID == "" {
		diagnostics.addError("invalid_recipe_set", "design system profile and source revision are required", "designSystemProfileId", "sourceRevisionId")
	}
	if (len(set.Tokens) > 0 || len(source.Tokens) > 0) && !canonicalJSONEqual(set.Tokens, source.Tokens) {
		diagnostics.addError("recipe_token_drift", "persisted recipe tokens do not match the current source document", "tokens")
	}

	presentKinds := make(map[string]struct{}, len(set.Recipes))
	for key, recipe := range set.Recipes {
		path := "recipes." + key
		wantKey := (RecipeKey{Kind: recipe.Kind, Variant: recipe.Variant, State: recipe.State}).String()
		if key != wantKey || recipe.Kind == "" || recipe.Variant == "" || recipe.State == "" {
			diagnostics.addError("invalid_recipe_key", fmt.Sprintf("recipe key %q does not match its identity", key), path)
		}
		presentKinds[recipe.Kind] = struct{}{}
		validateComponentRecipe(&diagnostics, source, set, recipe, path)
	}
	for _, kind := range requiredRecipeKinds {
		if _, ok := presentKinds[kind]; !ok {
			diagnostics.addError("missing_recipe_kind", fmt.Sprintf("required recipe kind %q is missing", kind), "recipes")
		}
	}
	for key, primitive := range set.PrimitiveFallbacks {
		validatePrimitiveRecipe(&diagnostics, set.Tokens, key, primitive, "primitiveFallbacks."+key)
	}
	return diagnostics
}

func canonicalJSONEqual(left, right any) bool {
	leftJSON, leftErr := json.Marshal(left)
	rightJSON, rightErr := json.Marshal(right)
	return leftErr == nil && rightErr == nil && bytes.Equal(leftJSON, rightJSON)
}

func ResolveRecipe(set ComponentRecipeSet, request RecipeRequest) (ResolvedRecipe, Diagnostics) {
	exactKey := (RecipeKey{Kind: request.Kind, Variant: request.Variant, State: request.State}).String()
	if recipe, ok := set.Recipes[exactKey]; ok && recipe.Kind == request.Kind {
		copy := recipe
		return ResolvedRecipe{Recipe: &copy, Fallback: "exact"}, nil
	}
	defaultKey := (RecipeKey{Kind: request.Kind, Variant: "default", State: "default"}).String()
	if recipe, ok := set.Recipes[defaultKey]; ok && recipe.Kind == request.Kind {
		copy := recipe
		return ResolvedRecipe{Recipe: &copy, Fallback: "default"}, nil
	}
	if primitive, ok := set.PrimitiveFallbacks[request.Kind]; ok {
		diagnostics := Diagnostics{}
		validatePrimitiveRecipe(&diagnostics, set.Tokens, request.Kind, primitive, "primitiveFallbacks."+request.Kind)
		if !diagnostics.HasErrors() {
			copy := primitive
			warning := Diagnostic{
				Code:     "primitive_fallback",
				Severity: DiagnosticWarning,
				Message:  fmt.Sprintf("resolved component kind %q with primitive fallback", request.Kind),
				Paths:    []string{"primitiveFallbacks." + request.Kind},
			}
			return ResolvedRecipe{Primitive: &copy, Fallback: "primitive"}, Diagnostics{warning}
		}
	}
	diagnostics := Diagnostics{}
	diagnostics.addError("missing_recipe", fmt.Sprintf("no executable recipe exists for component kind %q", request.Kind), "recipes")
	return ResolvedRecipe{}, diagnostics
}

func validateComponentRecipe(diagnostics *Diagnostics, source NativeJSON, set ComponentRecipeSet, recipe ComponentRecipe, path string) {
	if recipe.Source.RevisionID != set.SourceRevisionID {
		diagnostics.addError("invalid_recipe_source", "recipe revision must match the recipe set source revision", path+".source.revisionId")
	}
	if !validateRecipeSourceLayer(diagnostics, source, recipe.Source.RootLayerID, path+".source.rootLayerId") {
		return
	}
	for key, prop := range recipe.Props {
		propPath := path + ".props." + key
		target, ok := source.Layers[prop.TargetLayerID]
		if !ok || !isVisibleNativeLayer(source.Layers, prop.TargetLayerID) || !isNativeDescendantOrSelf(source.Layers, prop.TargetLayerID, recipe.Source.RootLayerID) || prop.Type == "" || !strings.EqualFold(prop.Type, target.Type) {
			addLayerDiagnostic(diagnostics, "invalid_recipe_prop", fmt.Sprintf("prop %q must target a visible descendant with matching type", key), propPath, prop.TargetLayerID)
		}
	}
	validateRecipeLayout(diagnostics, recipe.Layout, path+".layout")
	wantFingerprint := fingerprintRecipeSource(source, recipe.Source.RootLayerID)
	if wantFingerprint != "" && recipe.Source.Fingerprint != wantFingerprint {
		diagnostics.addError("recipe_fingerprint_drift", "persisted recipe fingerprint does not match its complete source subtree", path+".source.fingerprint")
	}
}

func validateRecipeSourceLayer(diagnostics *Diagnostics, source NativeJSON, layerID, path string) bool {
	layer, ok := source.Layers[layerID]
	if !ok {
		addLayerDiagnostic(diagnostics, "unknown_source_layer", fmt.Sprintf("source layer %q does not exist", layerID), path, layerID)
		return false
	}
	if !layer.Visible || !isVisibleNativeLayer(source.Layers, layerID) {
		addLayerDiagnostic(diagnostics, "hidden_source_layer", fmt.Sprintf("source layer %q is hidden", layerID), path, layerID)
		return false
	}
	return true
}

func validateRecipeLayout(diagnostics *Diagnostics, layout RecipeLayout, path string) {
	if layout.TextOverflow != "ellipsis" && layout.TextOverflow != "wrap" {
		diagnostics.addError("invalid_recipe_layout", "textOverflow must be ellipsis or wrap", path+".textOverflow")
	}
	if layout.Height <= 0 || layout.MinWidth < 0 || layout.WidthMode == "" {
		diagnostics.addError("invalid_recipe_layout", "widthMode and positive height are required, and minWidth cannot be negative", path)
	}
	if layout.OverlayRole != "" && strings.TrimSpace(layout.OverlayRole) == "" {
		diagnostics.addError("invalid_recipe_layout", "overlayRole must be non-blank when declared", path+".overlayRole")
	}
}

func validatePrimitiveRecipe(diagnostics *Diagnostics, tokens map[string]any, key string, primitive PrimitiveRecipe, path string) {
	if key == "" || primitive.Kind != key || primitive.LayerType == "" || len(primitive.Props) == 0 || len(primitive.Style) == 0 {
		diagnostics.addError("invalid_primitive_recipe", fmt.Sprintf("primitive %q is not an executable typed definition", key), path)
	}
	for propKey, prop := range primitive.Props {
		if prop.TargetLayerID == "" || prop.Type == "" {
			diagnostics.addError("invalid_primitive_recipe", fmt.Sprintf("primitive prop %q requires targetLayerId and type", propKey), path+".props."+propKey)
		}
	}
	validateRecipeLayout(diagnostics, primitive.Layout, path+".layout")
	validatePrimitiveStyleLeaves(diagnostics, tokens, primitive.Style, path+".style")
}

func validatePrimitiveStyleLeaves(diagnostics *Diagnostics, tokens map[string]any, value any, path string) {
	switch typed := value.(type) {
	case map[string]any:
		if len(typed) == 0 {
			diagnostics.addError("invalid_primitive_style", "primitive style objects cannot be empty", path)
		}
		for key, child := range typed {
			validatePrimitiveStyleLeaves(diagnostics, tokens, child, path+"."+key)
		}
	case []any:
		if len(typed) == 0 {
			diagnostics.addError("invalid_primitive_style", "primitive style arrays cannot be empty", path)
		}
		for index, child := range typed {
			validatePrimitiveStyleLeaves(diagnostics, tokens, child, fmt.Sprintf("%s.%d", path, index))
		}
	case string:
		if !strings.HasPrefix(typed, "$") || len(typed) == 1 {
			diagnostics.addError("invalid_primitive_style", "every primitive style leaf must be a token reference", path)
			return
		}
		if !tokenReferenceExists(tokens, strings.TrimPrefix(typed, "$")) {
			diagnostics.addError("unknown_token_reference", fmt.Sprintf("token reference %q does not exist", typed), path)
		}
	default:
		diagnostics.addError("invalid_primitive_style", "every primitive style leaf must be a token reference", path)
	}
}

func tokenReferenceExists(tokens map[string]any, path string) bool {
	parts := strings.Split(path, ".")
	var current any = tokens
	for _, part := range parts {
		object, ok := current.(map[string]any)
		if !ok {
			return false
		}
		current, ok = object[part]
		if !ok {
			return false
		}
	}
	return true
}

func fingerprintRecipeSource(source NativeJSON, rootLayerID string) string {
	if _, ok := source.Layers[rootLayerID]; !ok {
		return ""
	}
	type fingerprintPayload struct {
		RootLayerID string  `json:"rootLayerId"`
		Layers      []Layer `json:"layers"`
		Assets      []Asset `json:"assets"`
	}
	payload := fingerprintPayload{RootLayerID: rootLayerID}
	assetIDs := map[string]struct{}{}
	visited := map[string]struct{}{}
	var visit func(string)
	visit = func(layerID string) {
		if _, seen := visited[layerID]; seen {
			return
		}
		layer, ok := source.Layers[layerID]
		if !ok {
			return
		}
		visited[layerID] = struct{}{}
		payload.Layers = append(payload.Layers, layer)
		collectReferencedAssets(layer, source.Assets, assetIDs)
		for _, childID := range layer.Children {
			visit(childID)
		}
	}
	visit(rootLayerID)

	sortedAssetIDs := make([]string, 0, len(assetIDs))
	for assetID := range assetIDs {
		sortedAssetIDs = append(sortedAssetIDs, assetID)
	}
	sort.Strings(sortedAssetIDs)
	for _, assetID := range sortedAssetIDs {
		payload.Assets = append(payload.Assets, source.Assets[assetID])
	}
	raw, err := json.Marshal(payload)
	if err != nil {
		return ""
	}
	sum := sha256.Sum256(raw)
	return hex.EncodeToString(sum[:])
}

func collectReferencedAssets(layer Layer, assets map[string]Asset, result map[string]struct{}) {
	if layer.Image != nil {
		if _, ok := assets[layer.Image.AssetID]; ok {
			result[layer.Image.AssetID] = struct{}{}
		}
	}
	for _, value := range []any{layer.Text, layer.Shape, layer.Exportable, layer.Semantic, layer.Style, layer.Source} {
		walkJSONStrings(value, func(candidate string) {
			if _, ok := assets[candidate]; ok {
				result[candidate] = struct{}{}
			}
		})
	}
}

func walkJSONStrings(value any, visit func(string)) {
	switch typed := value.(type) {
	case string:
		visit(typed)
	case map[string]any:
		for _, child := range typed {
			walkJSONStrings(child, visit)
		}
	case []any:
		for _, child := range typed {
			walkJSONStrings(child, visit)
		}
	case []map[string]any:
		for _, child := range typed {
			walkJSONStrings(child, visit)
		}
	}
}

func isVisibleNativeLayer(layers map[string]Layer, layerID string) bool {
	visited := map[string]struct{}{}
	for current := layerID; current != ""; {
		if _, seen := visited[current]; seen {
			return false
		}
		visited[current] = struct{}{}
		layer, ok := layers[current]
		if !ok || !layer.Visible {
			return false
		}
		current = layer.ParentID
	}
	return true
}

func isNativeDescendantOrSelf(layers map[string]Layer, layerID, ancestorID string) bool {
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

func cloneJSONMap(source map[string]any) map[string]any {
	if source == nil {
		return nil
	}
	raw, err := json.Marshal(source)
	if err != nil {
		return nil
	}
	var result map[string]any
	if err := json.Unmarshal(raw, &result); err != nil {
		return nil
	}
	return result
}

func clonePrimitiveRecipes(source map[string]PrimitiveRecipe) map[string]PrimitiveRecipe {
	if source == nil {
		return nil
	}
	raw, err := json.Marshal(source)
	if err != nil {
		return nil
	}
	var result map[string]PrimitiveRecipe
	if err := json.Unmarshal(raw, &result); err != nil {
		return nil
	}
	return result
}

func cloneRecipeProps(source map[string]RecipeProp) map[string]RecipeProp {
	if source == nil {
		return nil
	}
	result := make(map[string]RecipeProp, len(source))
	for key, value := range source {
		result[key] = value
	}
	return result
}
