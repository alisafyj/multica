package designcore

import (
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
		doc: doc, spec: spec, blueprint: blueprint, manifest: manifest, report: &report,
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
	doc       NativeJSON
	spec      PageSpec
	blueprint TemplateBlueprint
	manifest  CompilationManifest
	report    *QualityReport
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
		if text == "" || q.textWrapEnabled(layer) || layer.Text["overflow"] == "ellipsis" {
			continue
		}
		if _, enabled := layer.Text["overflow"]; !enabled {
			continue
		}
		fontSize := qualityFontSize(layer.Text)
		if MeasureTextWidth(text, TypographyMetrics{FontSize: fontSize})+2*compilerCellHorizontalPadding > layer.Width {
			q.report.Metrics.TextOverflowCount++
			q.addError("text_overflow", fmt.Sprintf("text in layer %q exceeds its available width", layer.ID), "layers."+layer.ID, layer.ID)
		}
	}
}

func (q *qualityEvaluator) textWrapEnabled(layer Layer) bool {
	if layer.Text["overflow"] == "wrap" {
		return true
	}
	for current := layer.ID; current != ""; {
		currentLayer, ok := q.doc.Layers[current]
		if !ok {
			return false
		}
		if currentLayer.Semantic["textOverflow"] == "wrap" {
			return true
		}
		current = currentLayer.ParentID
	}
	return false
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
	byParent := make(map[string][]Layer)
	for _, layerID := range q.sortedLayerIDs() {
		layer := q.doc.Layers[layerID]
		if !isVisibleNativeLayer(q.doc.Layers, layer.ID) || layer.Semantic["generatedBy"] != DesignCompilerVersion {
			continue
		}
		byParent[layer.ParentID] = append(byParent[layer.ParentID], layer)
	}
	parentIDs := make([]string, 0, len(byParent))
	for parentID := range byParent {
		parentIDs = append(parentIDs, parentID)
	}
	sort.Strings(parentIDs)
	for _, parentID := range parentIDs {
		siblings := byParent[parentID]
		for left := 0; left < len(siblings); left++ {
			for right := left + 1; right < len(siblings); right++ {
				first, second := siblings[left], siblings[right]
				if q.allowsOverlay(first, second) || !rectanglesOverlap(qualityRect(first), qualityRect(second)) {
					continue
				}
				q.report.Metrics.UnexpectedOverlapCount++
				q.addError("unexpected_overlap", fmt.Sprintf("generated sibling layers %q and %q overlap", first.ID, second.ID), "layers."+first.ID, "layers."+second.ID)
			}
		}
	}
}

func (q *qualityEvaluator) allowsOverlay(first, second Layer) bool {
	return qualityOverlayRole(first) != "" || qualityOverlayRole(second) != ""
}

func qualityOverlayRole(layer Layer) string {
	if role, ok := layer.Semantic["overlayRole"].(string); ok {
		return strings.TrimSpace(role)
	}
	return ""
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
		within := rectanglesOverlap(Rect{X: frame.X, Y: frame.Y, Width: frame.Width, Height: frame.Height}, qualityRect(layer))
		if q.manifest.HorizontalScroll && isNativeDescendantOrSelf(q.doc.Layers, layer.ID, tableRegion) {
			within = layer.X+layer.Width > q.doc.Layers[tableRegion].X && layer.Y+layer.Height > frame.Y && layer.Y < frame.Y+frame.Height
		}
		if !within {
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
	for _, layerID := range q.sortedLayerIDs() {
		layer := q.doc.Layers[layerID]
		if layer.Semantic["generatedBy"] != DesignCompilerVersion || !qualityRoleRequiresRecipe(qualityString(layer.Semantic, "generationRole")) {
			continue
		}
		kind := qualityString(layer.Semantic, "recipeKind")
		variant := qualityString(layer.Semantic, "recipeVariant")
		state := qualityString(layer.Semantic, "recipeState")
		fallback := qualityString(layer.Semantic, "recipeFallback")
		if kind == "" || variant == "" || state == "" || (fallback != "exact" && fallback != "default" && fallback != "primitive") {
			q.report.Metrics.MissingComponentCount++
			q.addError("unresolved_recipe", fmt.Sprintf("generated component %q has no resolved recipe metadata", layer.ID), "layers."+layer.ID, layer.ID)
			continue
		}
		if fallback == "primitive" {
			continue
		}
		if qualityString(layer.Semantic, "recipeSourceRevisionId") == "" || qualityString(layer.Semantic, "recipeSourceRootLayerId") == "" || qualityString(layer.Semantic, "recipeSourceFingerprint") == "" {
			q.addError("component_nonconformance", fmt.Sprintf("generated component %q is missing cloned recipe provenance", layer.ID), "layers."+layer.ID, layer.ID)
			continue
		}
		if sourceVariant, exists := layer.Style["sourceVariant"]; exists && sourceVariant != variant {
			q.addError("component_nonconformance", fmt.Sprintf("generated component %q source variant does not match resolved recipe variant", layer.ID), "layers."+layer.ID, layer.ID)
		}
	}
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

func qualityString(values map[string]any, key string) string {
	value, _ := values[key].(string)
	return value
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
