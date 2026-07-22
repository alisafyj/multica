package designcore

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"math"
	"sort"
	"strings"
)

const (
	compileStatusGenerated             = "generated"
	compileStatusGeneratedWithWarnings = "generated_with_warnings"
	compileStatusFailed                = "compile_failed"
)

type QualityMetrics struct {
	TextOverflowCount      int `json:"textOverflowCount"`
	UnexpectedOverlapCount int `json:"unexpectedOverlapCount"`
	OffFrameCount          int `json:"offFrameCount"`
	TemplateResidueCount   int `json:"templateResidueCount"`
	MissingComponentCount  int `json:"missingComponentCount"`
}

type QualityReport struct {
	Status      string         `json:"status"`
	Diagnostics Diagnostics    `json:"diagnostics"`
	Metrics     QualityMetrics `json:"metrics"`
}

// EvaluateCompiledDesign is the blocking final pass for compiler-produced documents.
func EvaluateCompiledDesign(doc NativeJSON, spec PageSpec, blueprint TemplateBlueprint, manifest CompilationManifest, compilerDiagnostics Diagnostics) QualityReport {
	report := QualityReport{Diagnostics: append(Diagnostics(nil), compilerDiagnostics...)}
	quality := qualityEvaluator{
		doc: doc, spec: spec, blueprint: blueprint, manifest: manifest,
		expectations: resolvedComponentExpectations(manifest.ResolvedComponents), report: &report,
	}
	quality.evaluate()
	report.Diagnostics = normalizeCompilerDiagnostics(report.Diagnostics)
	switch {
	case report.Diagnostics.HasErrors():
		report.Status = compileStatusFailed
	case len(report.Diagnostics) > 0:
		report.Status = compileStatusGeneratedWithWarnings
	default:
		report.Status = compileStatusGenerated
	}
	return report
}

type qualityEvaluator struct {
	doc          NativeJSON
	spec         PageSpec
	blueprint    TemplateBlueprint
	manifest     CompilationManifest
	expectations map[string]ResolvedComponentExpectation
	report       *QualityReport
}

func (q *qualityEvaluator) evaluate() {
	if validation := ValidateDocument(q.doc); !validation.Valid {
		q.addError("broken_native_json", strings.Join(validation.Errors, "; "), "document")
	}
	q.evaluateTextOverflow()
	q.evaluateOverlap()
	q.evaluateOffFrame()
	q.evaluateCountConsistency()
	q.evaluateTemplateResidue()
	q.evaluatePaginationPlacement()
	q.evaluateComponentConformance()
}

func (q *qualityEvaluator) evaluateTextOverflow() {
	for _, layerID := range q.sortedLayerIDs() {
		layer := q.doc.Layers[layerID]
		if !isVisibleNativeLayer(q.doc.Layers, layer.ID) || layer.Text == nil {
			continue
		}
		text := structuralLayerText(layer)
		expectation, root, ok := q.expectedComponentAncestor(layer.ID)
		if text == "" || !ok {
			continue
		}
		fontSize := qualityFontSize(layer.Text)
		measuredWidth := MeasureTextWidth(text, TypographyMetrics{FontSize: fontSize})
		availableWidth := layer.Width
		if availableWidth <= 0 {
			availableWidth = root.Width
		}
		overflows := false
		switch expectation.TextOverflow {
		case "ellipsis":
			if measuredWidth > availableWidth && !qualityHasSafeEllipsis(layer, root) {
				overflows = true
			}
		case "wrap":
			lineHeight := qualityNumber(layer.Text["lineHeight"])
			if lineHeight <= 0 {
				lineHeight = fontSize * 1.4
			}
			lineCount := math.Ceil(measuredWidth / math.Max(availableWidth, 1))
			if lineCount < 1 {
				lineCount = 1
			}
			if qualityString(layer.Text, "overflow") != "wrap" || lineCount*lineHeight > layer.Height || !rectContains(qualityRect(root), qualityRect(layer)) {
				overflows = true
			}
		default:
			if measuredWidth > availableWidth {
				overflows = true
			}
		}
		if overflows {
			q.report.Metrics.TextOverflowCount++
			q.addError("text_overflow", fmt.Sprintf("text in layer %q is not safely contained by its overflow policy", layer.ID), "layers."+layer.ID, layer.ID)
		}
	}
}

func qualityHasSafeEllipsis(layer, root Layer) bool {
	return qualityString(layer.Text, "overflow") == "ellipsis" &&
		layer.Text["clip"] == true &&
		qualityNumber(layer.Text["maxLines"]) == 1 &&
		rectContains(qualityRect(root), qualityRect(layer))
}

func qualityFontSize(text map[string]any) float64 {
	value, ok := text["fontSize"]
	if !ok {
		return compilerFontSize
	}
	switch typed := value.(type) {
	case float64:
		if typed > 0 && isFinite(typed) {
			return typed
		}
	case float32:
		if typed > 0 && isFinite(float64(typed)) {
			return float64(typed)
		}
	case int:
		if typed > 0 {
			return float64(typed)
		}
	case int64:
		if typed > 0 {
			return float64(typed)
		}
	}
	return compilerFontSize
}

func (q *qualityEvaluator) evaluateOverlap() {
	generated := make([]Layer, 0)
	for _, layerID := range q.sortedLayerIDs() {
		layer := q.doc.Layers[layerID]
		if !isVisibleNativeLayer(q.doc.Layers, layer.ID) || layer.Semantic["generatedBy"] != DesignCompilerVersion {
			continue
		}
		generated = append(generated, layer)
	}
	for left := 0; left < len(generated); left++ {
		for right := left + 1; right < len(generated); right++ {
			first, second := generated[left], generated[right]
			if isNativeDescendantOrSelf(q.doc.Layers, first.ID, second.ID) || isNativeDescendantOrSelf(q.doc.Layers, second.ID, first.ID) {
				continue
			}
			if q.allowsOverlay(first, second) || !rectanglesOverlap(qualityRect(first), qualityRect(second)) {
				continue
			}
			q.report.Metrics.UnexpectedOverlapCount++
			q.addError("unexpected_overlap", fmt.Sprintf("generated layers %q and %q overlap", first.ID, second.ID), "layers."+first.ID, "layers."+second.ID)
		}
	}
}

func (q *qualityEvaluator) allowsOverlay(first, second Layer) bool {
	firstExpectation, firstOK := q.expectations[first.ID]
	secondExpectation, secondOK := q.expectations[second.ID]
	return firstOK && secondOK && firstExpectation.OverlayRole != "" && firstExpectation.OverlayRole == secondExpectation.OverlayRole
}

func (q *qualityEvaluator) evaluateOffFrame() {
	frames := make(map[string]Frame, len(q.doc.Frames))
	for _, frame := range q.doc.Frames {
		frames[frame.ID] = frame
	}
	tableRegion := q.blueprint.Regions["table"].RootLayerID
	for _, layerID := range q.sortedLayerIDs() {
		layer := q.doc.Layers[layerID]
		if !isVisibleNativeLayer(q.doc.Layers, layer.ID) || layer.Semantic["generatedBy"] != DesignCompilerVersion {
			continue
		}
		frame, ok := frames[layer.FrameID]
		if !ok {
			continue
		}
		bounds := Rect{X: frame.X, Y: frame.Y, Width: frame.Width, Height: frame.Height}
		if q.manifest.HorizontalScroll && isNativeDescendantOrSelf(q.doc.Layers, layer.ID, tableRegion) {
			bounds = q.manifest.TableContentBounds
		}
		if bounds.Width <= 0 || bounds.Height <= 0 || !rectContains(bounds, qualityRect(layer)) {
			q.report.Metrics.OffFrameCount++
			q.addError("off_frame", fmt.Sprintf("generated layer %q is outside frame %q", layer.ID, frame.ID), "layers."+layer.ID, layer.ID)
		}
	}
}

func (q *qualityEvaluator) evaluateCountConsistency() {
	roles := make(map[string]int)
	generatedIDs := make([]string, 0)
	for _, layerID := range q.sortedLayerIDs() {
		layer := q.doc.Layers[layerID]
		if layer.Semantic["generatedBy"] != DesignCompilerVersion {
			continue
		}
		roles[qualityString(layer.Semantic, "generationRole")]++
	}
	for layerID := range q.doc.Layers {
		if strings.HasPrefix(layerID, "gen-") {
			generatedIDs = append(generatedIDs, layerID)
		}
	}
	sort.Strings(generatedIDs)
	expected := map[string]int{
		"filter-control": q.manifest.FilterCount,
		"page-action":    q.manifest.PageActionCount,
		"table-header":   q.manifest.ColumnCount,
		"table-row":      q.manifest.RowCount,
		"row-action":     q.manifest.RowCount * q.manifest.RowActionCount,
		"pagination":     0,
	}
	if q.spec.Pagination.Enabled {
		expected["pagination"] = 1
	}
	statusCount := 0
	for _, column := range q.spec.Table.Columns {
		if column.Cell == "status-tag" {
			statusCount += q.manifest.RowCount
		}
	}
	expected["status-tag"] = statusCount
	expected["table-cell"] = q.manifest.RowCount*q.manifest.ColumnCount - statusCount
	for role, want := range expected {
		if got := roles[role]; got != want {
			q.addError("count_mismatch", fmt.Sprintf("generated %s count is %d, want %d", role, got, want), "manifest")
			return
		}
	}
	if !sameStringSlices(generatedIDs, manifestGeneratedIDs(q.manifest)) {
		q.addError("count_mismatch", "manifest generated layer IDs do not match the document", "manifest.generatedLayerIds")
	}
}

func (q *qualityEvaluator) evaluateTemplateResidue() {
	templateTexts := make(map[string]struct{}, len(q.manifest.TemplateBusinessTexts))
	for _, text := range q.manifest.TemplateBusinessTexts {
		if normalized := normalizeQualityText(text); normalized != "" {
			templateTexts[normalized] = struct{}{}
		}
	}
	if len(templateTexts) == 0 {
		return
	}
	specTexts := qualityPageSpecTexts(q.spec)
	for _, regionID := range q.manifest.BusinessRegionLayerIDs {
		for _, layerID := range qualityDescendants(q.doc.Layers, regionID) {
			layer := q.doc.Layers[layerID]
			if !isVisibleNativeLayer(q.doc.Layers, layer.ID) || q.isShellAllowlisted(layer.ID) {
				continue
			}
			text := normalizeQualityText(structuralLayerText(layer))
			if text == "" {
				continue
			}
			if _, stale := templateTexts[text]; stale {
				if _, declared := specTexts[text]; !declared {
					q.report.Metrics.TemplateResidueCount++
					q.addError("template_residue", fmt.Sprintf("template business text %q remains in layer %q", structuralLayerText(layer), layer.ID), "layers."+layer.ID, layer.ID)
				}
			}
		}
	}
}

func (q *qualityEvaluator) isShellAllowlisted(layerID string) bool {
	for _, allowlisted := range q.blueprint.ShellAllowlistLayerIDs {
		if isNativeDescendantOrSelf(q.doc.Layers, layerID, allowlisted) {
			return true
		}
	}
	return false
}

func (q *qualityEvaluator) evaluatePaginationPlacement() {
	if !q.spec.Pagination.Enabled {
		return
	}
	pagination, ok := q.generatedLayerByRole("pagination")
	if !ok {
		return
	}
	lastRowBottom := math.Inf(-1)
	for _, layerID := range q.sortedLayerIDs() {
		layer := q.doc.Layers[layerID]
		if layer.Semantic["generatedBy"] == DesignCompilerVersion && layer.Semantic["generationRole"] == "table-row" {
			lastRowBottom = math.Max(lastRowBottom, layer.Y+layer.Height)
		}
	}
	if !math.IsInf(lastRowBottom, -1) && pagination.Y < lastRowBottom {
		q.addError("pagination_misplaced", "pagination starts before the last table row ends", "pagination", pagination.ID)
	}
}

func (q *qualityEvaluator) evaluateComponentConformance() {
	seen := make(map[string]struct{}, len(q.expectations))
	for _, layerID := range q.sortedLayerIDs() {
		layer := q.doc.Layers[layerID]
		if layer.Semantic["generatedBy"] != DesignCompilerVersion || !qualityRoleRequiresRecipe(qualityString(layer.Semantic, "generationRole")) {
			continue
		}
		expectation, ok := q.expectations[layer.ID]
		if !ok {
			q.report.Metrics.MissingComponentCount++
			q.addError("unresolved_recipe", fmt.Sprintf("generated component %q has no compiler-resolved recipe expectation", layer.ID), "layers."+layer.ID, layer.ID)
			continue
		}
		seen[layer.ID] = struct{}{}
		if qualityString(layer.Semantic, "recipeKind") == "" || qualityString(layer.Semantic, "recipeVariant") == "" || qualityString(layer.Semantic, "recipeState") == "" || qualityString(layer.Semantic, "recipeFallback") == "" {
			q.report.Metrics.MissingComponentCount++
			q.addError("unresolved_recipe", fmt.Sprintf("generated component %q is missing resolved recipe metadata", layer.ID), "layers."+layer.ID, layer.ID)
			continue
		}
		if !qualityComponentMatchesExpectation(q.doc, layer, expectation) {
			q.addError("component_nonconformance", fmt.Sprintf("generated component %q does not match its compiler-resolved recipe", layer.ID), "layers."+layer.ID, layer.ID)
		}
	}
	for rootID := range q.expectations {
		if _, ok := seen[rootID]; ok {
			continue
		}
		q.report.Metrics.MissingComponentCount++
		q.addError("unresolved_recipe", fmt.Sprintf("compiler-resolved component %q is missing from the document", rootID), "layers."+rootID, rootID)
	}
}

func resolvedComponentExpectations(source []ResolvedComponentExpectation) map[string]ResolvedComponentExpectation {
	result := make(map[string]ResolvedComponentExpectation, len(source))
	for _, expectation := range source {
		if expectation.GeneratedRootLayerID != "" {
			result[expectation.GeneratedRootLayerID] = expectation
		}
	}
	return result
}

func qualityComponentMatchesExpectation(doc NativeJSON, layer Layer, expectation ResolvedComponentExpectation) bool {
	if qualityString(layer.Semantic, "recipeKind") != expectation.RecipeKind ||
		qualityString(layer.Semantic, "recipeVariant") != expectation.RecipeVariant ||
		qualityString(layer.Semantic, "recipeState") != expectation.RecipeState ||
		qualityString(layer.Semantic, "requestedRecipeVariant") != expectation.RequestedVariant ||
		qualityString(layer.Semantic, "recipeFallback") != expectation.Fallback {
		return false
	}
	if qualityString(layer.Semantic, "recipeSourceRevisionId") != expectation.SourceRevisionID ||
		qualityString(layer.Semantic, "recipeSourceRootLayerId") != expectation.SourceRootLayerID ||
		qualityString(layer.Semantic, "recipeSourceFingerprint") != expectation.SourceFingerprint {
		return false
	}
	return expectation.OutputFingerprint != "" && fingerprintGeneratedSubtree(doc, layer.ID) == expectation.OutputFingerprint
}

type fingerprintNamedSlot struct {
	Key     string      `json:"key"`
	Binding SlotBinding `json:"binding"`
}

type fingerprintNamedModule struct {
	Key     string        `json:"key"`
	Binding ModuleBinding `json:"binding"`
}

type fingerprintNamedState struct {
	Key     string       `json:"key"`
	Binding StateBinding `json:"binding"`
}

type fingerprintNamedComponent struct {
	LayerID string           `json:"layerId"`
	Binding ComponentBinding `json:"binding"`
}

func fingerprintGeneratedSubtree(doc NativeJSON, rootLayerID string) string {
	if _, ok := doc.Layers[rootLayerID]; !ok {
		return ""
	}
	payload := struct {
		RootLayerID string                      `json:"rootLayerId"`
		Layers      []Layer                     `json:"layers"`
		Assets      []Asset                     `json:"assets"`
		Slots       []fingerprintNamedSlot      `json:"slots,omitempty"`
		Modules     []fingerprintNamedModule    `json:"modules,omitempty"`
		States      []fingerprintNamedState     `json:"states,omitempty"`
		Components  []fingerprintNamedComponent `json:"components,omitempty"`
	}{RootLayerID: rootLayerID}
	visited := make(map[string]struct{})
	assetIDs := make(map[string]struct{})
	var visit func(string)
	visit = func(layerID string) {
		if _, seen := visited[layerID]; seen {
			return
		}
		layer, ok := doc.Layers[layerID]
		if !ok {
			return
		}
		visited[layerID] = struct{}{}
		payload.Layers = append(payload.Layers, layer)
		collectReferencedAssets(layer, doc.Assets, assetIDs)
		for _, childID := range layer.Children {
			visit(childID)
		}
	}
	visit(rootLayerID)

	assetKeys := sortedQualityMapKeys(doc.Assets)
	for _, key := range assetKeys {
		if _, used := assetIDs[key]; used {
			payload.Assets = append(payload.Assets, doc.Assets[key])
		}
	}
	for _, key := range sortedQualityMapKeys(doc.Slots) {
		if qualityBindingOwnedBySubtree(doc.Slots[key].LayerIDs, visited) {
			payload.Slots = append(payload.Slots, fingerprintNamedSlot{Key: key, Binding: doc.Slots[key]})
		}
	}
	for _, key := range sortedQualityMapKeys(doc.Modules) {
		if qualityBindingOwnedBySubtree(doc.Modules[key].LayerIDs, visited) {
			payload.Modules = append(payload.Modules, fingerprintNamedModule{Key: key, Binding: doc.Modules[key]})
		}
	}
	for _, key := range sortedQualityMapKeys(doc.States) {
		if qualityBindingOwnedBySubtree(doc.States[key].LayerIDs, visited) {
			payload.States = append(payload.States, fingerprintNamedState{Key: key, Binding: doc.States[key]})
		}
	}
	for _, key := range sortedQualityMapKeys(doc.ComponentBindings) {
		if _, owned := visited[key]; owned {
			payload.Components = append(payload.Components, fingerprintNamedComponent{LayerID: key, Binding: doc.ComponentBindings[key]})
		}
	}
	raw, err := json.Marshal(payload)
	if err != nil {
		return ""
	}
	sum := sha256.Sum256(raw)
	return hex.EncodeToString(sum[:])
}

func sortedQualityMapKeys[T any](values map[string]T) []string {
	keys := make([]string, 0, len(values))
	for key := range values {
		keys = append(keys, key)
	}
	sort.Strings(keys)
	return keys
}

func qualityBindingOwnedBySubtree(layerIDs []string, subtree map[string]struct{}) bool {
	if len(layerIDs) == 0 {
		return false
	}
	for _, layerID := range layerIDs {
		if _, ok := subtree[layerID]; !ok {
			return false
		}
	}
	return true
}

func (q *qualityEvaluator) expectedComponentAncestor(layerID string) (ResolvedComponentExpectation, Layer, bool) {
	for current := layerID; current != ""; {
		if expectation, ok := q.expectations[current]; ok {
			root, exists := q.doc.Layers[current]
			return expectation, root, exists
		}
		layer, ok := q.doc.Layers[current]
		if !ok {
			break
		}
		current = layer.ParentID
	}
	return ResolvedComponentExpectation{}, Layer{}, false
}

func qualityRoleRequiresRecipe(role string) bool {
	switch role {
	case "filter-control", "page-action", "table-header", "table-cell", "status-tag", "row-action", "pagination":
		return true
	default:
		return false
	}
}

func (q *qualityEvaluator) generatedLayerByRole(role string) (Layer, bool) {
	for _, layerID := range q.sortedLayerIDs() {
		layer := q.doc.Layers[layerID]
		if layer.Semantic["generatedBy"] == DesignCompilerVersion && layer.Semantic["generationRole"] == role {
			return layer, true
		}
	}
	return Layer{}, false
}

func (q *qualityEvaluator) addError(code, message string, path string, layerIDs ...string) {
	q.report.Diagnostics = append(q.report.Diagnostics, Diagnostic{
		Code: code, Severity: DiagnosticError, Message: message, Paths: []string{path}, LayerIDs: layerIDs,
	})
}

func (q *qualityEvaluator) sortedLayerIDs() []string {
	ids := make([]string, 0, len(q.doc.Layers))
	for layerID := range q.doc.Layers {
		ids = append(ids, layerID)
	}
	sort.Strings(ids)
	return ids
}

func qualityRect(layer Layer) Rect {
	return Rect{X: layer.X, Y: layer.Y, Width: layer.Width, Height: layer.Height}
}

func rectanglesOverlap(first, second Rect) bool {
	return first.X < second.X+second.Width && second.X < first.X+first.Width && first.Y < second.Y+second.Height && second.Y < first.Y+first.Height
}

func rectContains(outer, inner Rect) bool {
	return inner.X >= outer.X && inner.Y >= outer.Y && inner.X+inner.Width <= outer.X+outer.Width && inner.Y+inner.Height <= outer.Y+outer.Height
}

func qualityString(values map[string]any, key string) string {
	value, _ := values[key].(string)
	return value
}

func qualityNumber(value any) float64 {
	switch typed := value.(type) {
	case float64:
		return typed
	case float32:
		return float64(typed)
	case int:
		return float64(typed)
	case int32:
		return float64(typed)
	case int64:
		return float64(typed)
	default:
		return 0
	}
}

func manifestGeneratedIDs(manifest CompilationManifest) []string {
	result := append([]string(nil), manifest.GeneratedLayerIDs...)
	sort.Strings(result)
	return result
}

func sameStringSlices(first, second []string) bool {
	if len(first) != len(second) {
		return false
	}
	for index := range first {
		if first[index] != second[index] {
			return false
		}
	}
	return true
}

func normalizeQualityText(value string) string {
	return strings.ToLower(strings.Join(strings.Fields(strings.TrimSpace(value)), " "))
}

func qualityPageSpecTexts(spec PageSpec) map[string]struct{} {
	result := make(map[string]struct{})
	add := func(value string) {
		if normalized := normalizeQualityText(value); normalized != "" {
			result[normalized] = struct{}{}
		}
	}
	add(spec.Page.Title)
	add(spec.Page.ActiveNavigation)
	for _, value := range spec.Page.Breadcrumb {
		add(value)
	}
	for _, filter := range spec.Filters {
		add(filter.Label)
		add(filter.Placeholder)
	}
	for _, action := range spec.PageActions {
		add(action.Label)
	}
	for _, column := range spec.Table.Columns {
		add(column.Title)
	}
	for _, row := range spec.Table.SampleRows {
		for _, value := range row {
			add(value)
		}
	}
	for _, action := range spec.Table.RowActions {
		add(action.Label)
	}
	if spec.Pagination.Enabled {
		add(fmt.Sprintf("%d / %d", spec.Pagination.PageSize, spec.Pagination.SampleTotal))
	}
	return result
}

func qualityDescendants(layers map[string]Layer, rootID string) []string {
	visited := make(map[string]struct{})
	result := make([]string, 0)
	var visit func(string)
	visit = func(layerID string) {
		if _, seen := visited[layerID]; seen {
			return
		}
		layer, ok := layers[layerID]
		if !ok {
			return
		}
		visited[layerID] = struct{}{}
		result = append(result, layerID)
		for _, childID := range layer.Children {
			visit(childID)
		}
	}
	visit(rootID)
	sort.Strings(result)
	return result
}
