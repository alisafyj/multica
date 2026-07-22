package designcore

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"math"
	"sort"
	"strconv"
	"strings"
)

type CloneResult struct {
	RootLayerID    string
	SourceToTarget map[string]string
}

type DocumentBuilder struct {
	document  NativeJSON
	namespace string
	sequence  uint64
}

func NewDocumentBuilder(base NativeJSON, namespace string) (*DocumentBuilder, error) {
	if namespace == "" {
		return nil, errors.New("namespace is required")
	}
	document, err := copyNativeDocument(base)
	if err != nil {
		return nil, fmt.Errorf("copy base document: %w", err)
	}
	if err := validateBuilderDocument(document); err != nil {
		return nil, fmt.Errorf("invalid base document: %w", err)
	}
	return &DocumentBuilder{document: document, namespace: namespace}, nil
}

func (b *DocumentBuilder) ClearChildren(rootLayerID string) error {
	candidate, err := copyNativeDocument(b.document)
	if err != nil {
		return fmt.Errorf("copy document: %w", err)
	}
	root, ok := candidate.Layers[rootLayerID]
	if !ok {
		return fmt.Errorf("root layer %q does not exist", rootLayerID)
	}

	removed := make(map[string]struct{})
	visiting := make(map[string]struct{})
	var collect func(string, string) error
	collect = func(layerID, parentID string) error {
		if _, active := visiting[layerID]; active {
			return fmt.Errorf("layer subtree contains a cycle at %q", layerID)
		}
		layer, exists := candidate.Layers[layerID]
		if !exists {
			return fmt.Errorf("layer %q references missing child %q", parentID, layerID)
		}
		if layer.ParentID != parentID {
			return fmt.Errorf("layer %q child %q has parent %q", parentID, layerID, layer.ParentID)
		}
		visiting[layerID] = struct{}{}
		for _, childID := range layer.Children {
			if err := collect(childID, layerID); err != nil {
				return err
			}
		}
		delete(visiting, layerID)
		removed[layerID] = struct{}{}
		return nil
	}
	for _, childID := range root.Children {
		if err := collect(childID, rootLayerID); err != nil {
			return err
		}
	}
	for layerID := range removed {
		delete(candidate.Layers, layerID)
	}
	root.Children = []string{}
	candidate.Layers[rootLayerID] = root
	if err := validateBuilderDocument(candidate); err != nil {
		return fmt.Errorf("clear children: %w", err)
	}
	b.document = candidate
	return nil
}

func (b *DocumentBuilder) CloneSubtree(source NativeJSON, sourceRootID, targetParentID, targetFrameID string, bounds Rect) (CloneResult, error) {
	if err := validateBounds(bounds); err != nil {
		return CloneResult{}, err
	}
	parent, ok := b.document.Layers[targetParentID]
	if !ok {
		return CloneResult{}, fmt.Errorf("target parent layer %q does not exist", targetParentID)
	}
	if !documentHasFrame(b.document, targetFrameID) {
		return CloneResult{}, fmt.Errorf("target frame %q does not exist", targetFrameID)
	}
	if parent.FrameID != targetFrameID {
		return CloneResult{}, fmt.Errorf("target parent %q belongs to frame %q, not %q", targetParentID, parent.FrameID, targetFrameID)
	}

	sourceCopy, err := copyNativeDocument(source)
	if err != nil {
		return CloneResult{}, fmt.Errorf("copy source document: %w", err)
	}
	orderedLayerIDs, err := collectCloneLayerIDs(sourceCopy, sourceRootID)
	if err != nil {
		return CloneResult{}, err
	}

	operation := b.sequence + 1
	nodeIDs := make(map[string]string, len(orderedLayerIDs))
	targetNodeIDs := make(map[string]struct{}, len(orderedLayerIDs))
	for _, sourceID := range orderedLayerIDs {
		targetID := b.generatedID(operation, sourceID)
		if _, exists := b.document.Layers[targetID]; exists {
			return CloneResult{}, fmt.Errorf("generated layer ID %q already exists", targetID)
		}
		if _, duplicate := targetNodeIDs[targetID]; duplicate {
			return CloneResult{}, fmt.Errorf("generated duplicate layer ID %q", targetID)
		}
		nodeIDs[sourceID] = targetID
		targetNodeIDs[targetID] = struct{}{}
	}

	assetSourceIDs, err := collectCloneAssetIDs(sourceCopy, orderedLayerIDs)
	if err != nil {
		return CloneResult{}, err
	}
	assetIDs := make(map[string]string, len(assetSourceIDs))
	targetAssetIDs := make(map[string]struct{}, len(assetSourceIDs))
	for _, sourceID := range assetSourceIDs {
		targetID := sourceID
		if _, collision := b.document.Assets[sourceID]; collision {
			targetID = b.generatedID(operation, sourceID)
			if _, collision := b.document.Assets[targetID]; collision {
				return CloneResult{}, fmt.Errorf("generated asset ID %q already exists", targetID)
			}
		}
		if _, duplicate := targetAssetIDs[targetID]; duplicate {
			return CloneResult{}, fmt.Errorf("asset remaps produce duplicate target ID %q", targetID)
		}
		assetIDs[sourceID] = targetID
		targetAssetIDs[targetID] = struct{}{}
	}

	clonedLayers := make(map[string]Layer, len(orderedLayerIDs))
	for _, sourceID := range orderedLayerIDs {
		layer := sourceCopy.Layers[sourceID]
		targetID := nodeIDs[sourceID]
		layer.ID = targetID
		layer.FrameID = targetFrameID
		if sourceID == sourceRootID {
			layer.ParentID = targetParentID
			layer.X = bounds.X
			layer.Y = bounds.Y
			layer.Width = bounds.Width
			layer.Height = bounds.Height
		} else {
			mappedParentID, exists := nodeIDs[layer.ParentID]
			if !exists {
				return CloneResult{}, fmt.Errorf("layer %q has parent %q outside cloned subtree", sourceID, layer.ParentID)
			}
			layer.ParentID = mappedParentID
		}
		layer.Children = rewriteStringSlice(layer.Children, nodeIDs)
		layer.Text = rewriteJSONMap(layer.Text, nodeIDs, assetIDs)
		layer.Style = rewriteJSONMap(layer.Style, nodeIDs, assetIDs)
		layer.Semantic = rewriteJSONMap(layer.Semantic, nodeIDs, assetIDs)
		layer.Source = rewriteJSONMap(layer.Source, nodeIDs, assetIDs)
		layer.Shape = rewriteJSONMap(layer.Shape, nodeIDs, assetIDs)
		layer.Exportable = rewriteJSONMaps(layer.Exportable, nodeIDs, assetIDs)
		if layer.Image != nil {
			mappedAssetID, exists := assetIDs[layer.Image.AssetID]
			if !exists {
				return CloneResult{}, fmt.Errorf("layer %q references missing asset %q", sourceID, layer.Image.AssetID)
			}
			layer.Image.AssetID = mappedAssetID
		}
		clonedLayers[targetID] = layer
	}

	clonedAssets := make(map[string]Asset, len(assetSourceIDs))
	for _, sourceID := range assetSourceIDs {
		asset := sourceCopy.Assets[sourceID]
		targetID := assetIDs[sourceID]
		asset.ID = targetID
		clonedAssets[targetID] = asset
	}

	candidate, err := copyNativeDocument(b.document)
	if err != nil {
		return CloneResult{}, fmt.Errorf("copy document: %w", err)
	}
	if candidate.Assets == nil && len(clonedAssets) > 0 {
		candidate.Assets = make(map[string]Asset, len(clonedAssets))
	}
	for targetID, asset := range clonedAssets {
		candidate.Assets[targetID] = asset
	}
	for targetID, layer := range clonedLayers {
		candidate.Layers[targetID] = layer
	}
	parent = candidate.Layers[targetParentID]
	parent.Children = append(parent.Children, nodeIDs[sourceRootID])
	candidate.Layers[targetParentID] = parent
	if err := validateBuilderDocument(candidate); err != nil {
		return CloneResult{}, fmt.Errorf("clone subtree: %w", err)
	}

	b.document = candidate
	b.sequence = operation
	return CloneResult{RootLayerID: nodeIDs[sourceRootID], SourceToTarget: copyLayerIDMap(nodeIDs)}, nil
}

func (b *DocumentBuilder) BindText(clone CloneResult, sourceTargetLayerID, value string) error {
	targetID, layer, err := b.resolveCloneLayer(clone, sourceTargetLayerID)
	if err != nil {
		return err
	}
	if layer.Type != "text" {
		return fmt.Errorf("clone target %q is type %q, not text", sourceTargetLayerID, layer.Type)
	}
	candidate, err := copyNativeDocument(b.document)
	if err != nil {
		return fmt.Errorf("copy document: %w", err)
	}
	layer = candidate.Layers[targetID]
	if layer.Text == nil {
		layer.Text = make(map[string]any)
	}
	layer.Text["characters"] = value
	candidate.Layers[targetID] = layer
	if err := validateBuilderDocument(candidate); err != nil {
		return fmt.Errorf("bind text: %w", err)
	}
	b.document = candidate
	return nil
}

func (b *DocumentBuilder) FitCloneLayer(clone CloneResult, sourceTargetLayerID string, bounds Rect) error {
	if err := validateBounds(bounds); err != nil {
		return err
	}
	targetID, _, err := b.resolveCloneLayer(clone, sourceTargetLayerID)
	if err != nil {
		return err
	}
	return b.setBounds(targetID, bounds)
}

func (b *DocumentBuilder) SetBounds(layerID string, bounds Rect) error {
	if err := validateBounds(bounds); err != nil {
		return err
	}
	if _, ok := b.document.Layers[layerID]; !ok {
		return fmt.Errorf("layer %q does not exist", layerID)
	}
	return b.setBounds(layerID, bounds)
}

func (b *DocumentBuilder) AddPrimitiveLayer(parentID string, layer Layer) (string, error) {
	if layer.ID == "" {
		return "", errors.New("primitive layer ID is required")
	}
	if len(layer.Children) > 0 {
		return "", errors.New("primitive layer cannot declare children")
	}
	if err := validateBounds(Rect{X: layer.X, Y: layer.Y, Width: layer.Width, Height: layer.Height}); err != nil {
		return "", err
	}
	parent, ok := b.document.Layers[parentID]
	if !ok {
		return "", fmt.Errorf("parent layer %q does not exist", parentID)
	}
	layerCopy, err := copyNativeLayer(layer)
	if err != nil {
		return "", fmt.Errorf("copy primitive layer: %w", err)
	}
	operation := b.sequence + 1
	targetID := b.generatedID(operation, layer.ID)
	if _, exists := b.document.Layers[targetID]; exists {
		return "", fmt.Errorf("generated layer ID %q already exists", targetID)
	}
	layerCopy.ID = targetID
	layerCopy.ParentID = parentID
	layerCopy.FrameID = parent.FrameID
	layerCopy.Children = nil

	candidate, err := copyNativeDocument(b.document)
	if err != nil {
		return "", fmt.Errorf("copy document: %w", err)
	}
	candidate.Layers[targetID] = layerCopy
	parent = candidate.Layers[parentID]
	parent.Children = append(parent.Children, targetID)
	candidate.Layers[parentID] = parent
	if err := validateBuilderDocument(candidate); err != nil {
		return "", fmt.Errorf("add primitive layer: %w", err)
	}
	b.document = candidate
	b.sequence = operation
	return targetID, nil
}

func (b *DocumentBuilder) Document() NativeJSON {
	document, err := copyNativeDocument(b.document)
	if err != nil {
		panic(fmt.Sprintf("copy builder document: %v", err))
	}
	return document
}

func (b *DocumentBuilder) generatedID(operation uint64, sourceID string) string {
	payload := b.namespace + "\x00" + strconv.FormatUint(operation, 10) + "\x00" + sourceID
	sum := sha256.Sum256([]byte(payload))
	return "gen-" + hex.EncodeToString(sum[:])[:20]
}

func (b *DocumentBuilder) resolveCloneLayer(clone CloneResult, sourceTargetLayerID string) (string, Layer, error) {
	targetID, authorized := clone.SourceToTarget[sourceTargetLayerID]
	if !authorized {
		return "", Layer{}, fmt.Errorf("source layer %q is not authorized by clone", sourceTargetLayerID)
	}
	if _, exists := b.document.Layers[clone.RootLayerID]; !exists {
		return "", Layer{}, fmt.Errorf("clone root layer %q does not exist", clone.RootLayerID)
	}
	layer, exists := b.document.Layers[targetID]
	if !exists {
		return "", Layer{}, fmt.Errorf("clone target layer %q does not exist", targetID)
	}
	if !isNativeDescendantOrSelf(b.document.Layers, targetID, clone.RootLayerID) {
		return "", Layer{}, fmt.Errorf("target layer %q is outside clone %q", targetID, clone.RootLayerID)
	}
	return targetID, layer, nil
}

func (b *DocumentBuilder) setBounds(layerID string, bounds Rect) error {
	candidate, err := copyNativeDocument(b.document)
	if err != nil {
		return fmt.Errorf("copy document: %w", err)
	}
	layer := candidate.Layers[layerID]
	layer.X = bounds.X
	layer.Y = bounds.Y
	layer.Width = bounds.Width
	layer.Height = bounds.Height
	candidate.Layers[layerID] = layer
	if err := validateBuilderDocument(candidate); err != nil {
		return fmt.Errorf("set bounds: %w", err)
	}
	b.document = candidate
	return nil
}

func collectCloneLayerIDs(source NativeJSON, rootLayerID string) ([]string, error) {
	if _, ok := source.Layers[rootLayerID]; !ok {
		return nil, fmt.Errorf("source root layer %q does not exist", rootLayerID)
	}
	ordered := make([]string, 0)
	visited := make(map[string]struct{})
	visiting := make(map[string]struct{})
	var visit func(string, string, bool) error
	visit = func(layerID, parentID string, root bool) error {
		if _, active := visiting[layerID]; active {
			return fmt.Errorf("source subtree contains a cycle at %q", layerID)
		}
		if _, seen := visited[layerID]; seen {
			return fmt.Errorf("source subtree references layer %q more than once", layerID)
		}
		layer, exists := source.Layers[layerID]
		if !exists {
			return fmt.Errorf("source layer %q references missing child %q", parentID, layerID)
		}
		if layer.ID != layerID {
			return fmt.Errorf("source layer map key %q does not match layer ID %q", layerID, layer.ID)
		}
		if !root && layer.ParentID != parentID {
			return fmt.Errorf("source layer %q child %q has parent %q", parentID, layerID, layer.ParentID)
		}
		if err := validateBounds(Rect{X: layer.X, Y: layer.Y, Width: layer.Width, Height: layer.Height}); err != nil {
			return fmt.Errorf("source layer %q: %w", layerID, err)
		}
		visiting[layerID] = struct{}{}
		ordered = append(ordered, layerID)
		for _, childID := range layer.Children {
			if err := visit(childID, layerID, false); err != nil {
				return err
			}
		}
		delete(visiting, layerID)
		visited[layerID] = struct{}{}
		return nil
	}
	if err := visit(rootLayerID, "", true); err != nil {
		return nil, err
	}
	return ordered, nil
}

func collectCloneAssetIDs(source NativeJSON, layerIDs []string) ([]string, error) {
	assetSet := make(map[string]struct{})
	for _, layerID := range layerIDs {
		layer := source.Layers[layerID]
		if layer.Image != nil && layer.Image.AssetID != "" {
			if _, exists := source.Assets[layer.Image.AssetID]; !exists {
				return nil, fmt.Errorf("source layer %q references missing asset %q", layerID, layer.Image.AssetID)
			}
			assetSet[layer.Image.AssetID] = struct{}{}
		}
		for _, value := range []any{layer.Text, layer.Style, layer.Semantic, layer.Source, layer.Shape, layer.Exportable} {
			walkJSONStrings(value, func(candidate string) {
				if _, exists := source.Assets[candidate]; exists {
					assetSet[candidate] = struct{}{}
				}
			})
		}
	}
	assetIDs := make([]string, 0, len(assetSet))
	for assetID := range assetSet {
		asset := source.Assets[assetID]
		if asset.ID != "" && asset.ID != assetID {
			return nil, fmt.Errorf("source asset map key %q does not match asset ID %q", assetID, asset.ID)
		}
		if asset.URL == "" {
			return nil, fmt.Errorf("source asset %q URL is required", assetID)
		}
		assetIDs = append(assetIDs, assetID)
	}
	sort.Strings(assetIDs)
	return assetIDs, nil
}

func rewriteStringSlice(values []string, replacements map[string]string) []string {
	if values == nil {
		return nil
	}
	result := make([]string, len(values))
	for index, value := range values {
		if replacement, exists := replacements[value]; exists {
			result[index] = replacement
		} else {
			result[index] = value
		}
	}
	return result
}

func rewriteJSONMap(value map[string]any, nodeIDs, assetIDs map[string]string) map[string]any {
	if value == nil {
		return nil
	}
	return rewriteJSONValue(value, nodeIDs, assetIDs).(map[string]any)
}

func rewriteJSONMaps(values []map[string]any, nodeIDs, assetIDs map[string]string) []map[string]any {
	if values == nil {
		return nil
	}
	result := make([]map[string]any, len(values))
	for index, value := range values {
		result[index] = rewriteJSONMap(value, nodeIDs, assetIDs)
	}
	return result
}

func rewriteJSONValue(value any, nodeIDs, assetIDs map[string]string) any {
	switch typed := value.(type) {
	case string:
		if replacement, exists := nodeIDs[typed]; exists {
			return replacement
		}
		if replacement, exists := assetIDs[typed]; exists {
			return replacement
		}
		return typed
	case map[string]any:
		result := make(map[string]any, len(typed))
		for key, child := range typed {
			result[key] = rewriteJSONValue(child, nodeIDs, assetIDs)
		}
		return result
	case []any:
		result := make([]any, len(typed))
		for index, child := range typed {
			result[index] = rewriteJSONValue(child, nodeIDs, assetIDs)
		}
		return result
	case []map[string]any:
		return rewriteJSONMaps(typed, nodeIDs, assetIDs)
	default:
		return typed
	}
}

func validateBounds(bounds Rect) error {
	for name, value := range map[string]float64{"x": bounds.X, "y": bounds.Y, "width": bounds.Width, "height": bounds.Height} {
		if math.IsNaN(value) || math.IsInf(value, 0) {
			return fmt.Errorf("bounds %s must be finite", name)
		}
	}
	if bounds.Width < 0 || bounds.Height < 0 {
		return errors.New("bounds width and height must be non-negative")
	}
	return nil
}

func validateBuilderDocument(document NativeJSON) error {
	result := ValidateDocument(document)
	if result.Valid {
		return nil
	}
	return errors.New(strings.Join(result.Errors, "; "))
}

func documentHasFrame(document NativeJSON, frameID string) bool {
	for _, frame := range document.Frames {
		if frame.ID == frameID {
			return true
		}
	}
	return false
}

func copyNativeDocument(source NativeJSON) (NativeJSON, error) {
	raw, err := json.Marshal(source)
	if err != nil {
		return NativeJSON{}, err
	}
	var result NativeJSON
	if err := json.Unmarshal(raw, &result); err != nil {
		return NativeJSON{}, err
	}
	return result, nil
}

func copyNativeLayer(source Layer) (Layer, error) {
	raw, err := json.Marshal(source)
	if err != nil {
		return Layer{}, err
	}
	var result Layer
	if err := json.Unmarshal(raw, &result); err != nil {
		return Layer{}, err
	}
	return result, nil
}

func copyLayerIDMap(source map[string]string) map[string]string {
	result := make(map[string]string, len(source))
	for sourceID, targetID := range source {
		result[sourceID] = targetID
	}
	return result
}
