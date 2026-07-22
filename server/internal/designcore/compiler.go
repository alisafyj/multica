package designcore

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"sort"
	"strings"
)

const DesignCompilerVersion = "list-1.0"

const (
	compileStatusCompiled = "compiled"
	compileStatusFailed   = "compile_failed"

	compilerFontSize              = 14.0
	compilerCellHorizontalPadding = 16.0
)

type CompileProvenance struct {
	WorkspaceID       string
	ProjectID         string
	IssueID           string
	AgentTaskID       string
	PageSpecVersion   string
	BlueprintRecordID string
	RecipeSetRecordID string
}

type CompileInput struct {
	PageSpec    PageSpec
	Blueprint   TemplateBlueprint
	RecipeSet   ComponentRecipeSet
	TemplateDoc NativeJSON
	RecipeDoc   NativeJSON
	Provenance  CompileProvenance
}

type CompilationManifest struct {
	FilterCount            int
	PageActionCount        int
	ColumnCount            int
	RowCount               int
	RowActionCount         int
	HorizontalScroll       bool
	GeneratedLayerIDs      []string
	BusinessRegionLayerIDs []string
	TemplateBusinessTexts  []string
}

type CompileOutput struct {
	Status      string
	Document    NativeJSON
	Manifest    CompilationManifest
	Diagnostics Diagnostics
}

type generatedRootMetadata struct {
	Role                    string
	SpecPath                string
	RecipeKind              string
	RecipeVariant           string
	RecipeState             string
	RequestedRecipeVariant  string
	RecipeFallback          string
	RecipeSourceRevisionID  string
	RecipeSourceRootLayerID string
	RecipeSourceFingerprint string
	TextOverflow            string
	Pinned                  string
}

type pendingNavigationBinding struct {
	LayerID string
	Label   string
}

type componentBindingPlan struct {
	Values      map[string]string
	RequiredAll []string
	RequiredAny []string
}

type instantiatedComponent struct {
	RootLayerID string
	Bounds      Rect
	Layout      RecipeLayout
}

type listPageCompiler struct {
	input              CompileInput
	templateSource     NativeJSON
	builder            *DocumentBuilder
	manifest           CompilationManifest
	diagnostics        Diagnostics
	generatedRoots     map[string]generatedRootMetadata
	textOverflow       map[string]string
	navigation         *pendingNavigationBinding
	tableRegionID      string
	horizontalStrategy string
}

func CompileListPage(input CompileInput) CompileOutput {
	retained, copyErr := copyNativeDocument(input.TemplateDoc)
	if copyErr != nil {
		diagnostics := normalizeCompilerDiagnostics(Diagnostics{{
			Code: "invalid_template_document", Severity: DiagnosticError,
			Message: fmt.Sprintf("copy template document: %v", copyErr), Paths: []string{"templateDoc"},
		}})
		return CompileOutput{Status: compileStatusFailed, Diagnostics: diagnostics}
	}

	compiler := &listPageCompiler{
		input:          input,
		templateSource: retained,
		manifest: CompilationManifest{
			FilterCount:            len(input.PageSpec.Filters),
			PageActionCount:        len(input.PageSpec.PageActions),
			ColumnCount:            len(input.PageSpec.Table.Columns),
			RowCount:               len(input.PageSpec.Table.SampleRows),
			RowActionCount:         len(input.PageSpec.Table.RowActions),
			BusinessRegionLayerIDs: compilerBusinessRegionLayerIDs(input.Blueprint),
		},
		generatedRoots:     make(map[string]generatedRootMetadata),
		textOverflow:       make(map[string]string),
		horizontalStrategy: "none",
	}
	compiler.validateInputs()
	if compiler.diagnostics.HasErrors() {
		return compiler.failedOutput(retained)
	}

	compiler.manifest.TemplateBusinessTexts = collectTemplateBusinessTexts(compiler.templateSource, input.Blueprint)
	builder, err := NewDocumentBuilder(retained, compilerNamespace(input))
	if err != nil {
		compiler.addError("compiler_builder_failed", err.Error(), "templateDoc")
		return compiler.failedOutput(retained)
	}
	compiler.builder = builder

	if err := compiler.clearBusinessRegions(); err != nil {
		compiler.addError("region_clear_failed", err.Error(), "blueprint.regions")
		return compiler.failedOutput(compiler.builder.Document())
	}
	if err := compiler.instantiatePageIdentity(); err != nil {
		compiler.addError("page_identity_failed", err.Error(), "page")
		return compiler.failedOutput(compiler.builder.Document())
	}
	if err := compiler.bindActiveNavigation(); err != nil {
		compiler.addError("navigation_binding_failed", err.Error(), "page.activeNavigation")
		return compiler.failedOutput(compiler.builder.Document())
	}
	tableY, err := compiler.instantiateFiltersAndActions()
	if err != nil || compiler.diagnostics.HasErrors() {
		if err != nil {
			compiler.addError("region_instantiation_failed", err.Error(), "filters", "pageActions")
		}
		return compiler.failedOutput(compiler.builder.Document())
	}
	tableBottom, err := compiler.instantiateTable(tableY)
	if err != nil || compiler.diagnostics.HasErrors() {
		if err != nil {
			compiler.addError("table_instantiation_failed", err.Error(), "table")
		}
		return compiler.failedOutput(compiler.builder.Document())
	}
	if err := compiler.instantiatePagination(tableBottom); err != nil || compiler.diagnostics.HasErrors() {
		if err != nil {
			compiler.addError("pagination_instantiation_failed", err.Error(), "pagination")
		}
		return compiler.failedOutput(compiler.builder.Document())
	}

	retained = compiler.builder.Document()
	final := compiler.builder.Document()
	compiler.applyFinalEdits(&final)
	validation := ValidateDocument(final)
	if !validation.Valid {
		for _, message := range validation.Errors {
			compiler.addError("invalid_compiled_document", message, "document")
		}
		return compiler.failedOutput(retained)
	}

	compiler.manifest.GeneratedLayerIDs = compilerGeneratedLayerIDs(compiler.templateSource, final)
	compiler.diagnostics = normalizeCompilerDiagnostics(compiler.diagnostics)
	return CompileOutput{
		Status:      compileStatusCompiled,
		Document:    final,
		Manifest:    compiler.manifest,
		Diagnostics: compiler.diagnostics,
	}
}

func (c *listPageCompiler) validateInputs() {
	templateValidation := ValidateDocument(c.input.TemplateDoc)
	for _, message := range templateValidation.Errors {
		c.addError("invalid_template_document", message, "templateDoc")
	}
	recipeValidation := ValidateDocument(c.input.RecipeDoc)
	for _, message := range recipeValidation.Errors {
		c.addError("invalid_recipe_document", message, "recipeDoc")
	}
	c.diagnostics = append(c.diagnostics, ValidatePageSpec(c.input.PageSpec, nil)...)
	if templateValidation.Valid {
		structure := ExtractTemplateStructure(c.input.TemplateDoc)
		c.diagnostics = append(c.diagnostics, ValidateTemplateBlueprintForPageSpec(structure, c.input.Blueprint, c.input.PageSpec)...)
	}
	if recipeValidation.Valid {
		c.diagnostics = append(c.diagnostics, ValidateComponentRecipeSet(c.input.RecipeDoc, c.input.RecipeSet)...)
	}
	c.diagnostics = normalizeCompilerDiagnostics(c.diagnostics)
}

func (c *listPageCompiler) failedOutput(document NativeJSON) CompileOutput {
	detached, err := copyNativeDocument(document)
	if err != nil {
		c.addError("compiler_document_copy_failed", err.Error(), "document")
		detached = NativeJSON{}
	}
	if c.builder != nil {
		candidate, candidateErr := copyNativeDocument(detached)
		if candidateErr == nil {
			c.applyFinalEdits(&candidate)
			if ValidateDocument(candidate).Valid {
				detached = candidate
			}
		}
	}
	c.diagnostics = normalizeCompilerDiagnostics(c.diagnostics)
	c.manifest.GeneratedLayerIDs = compilerGeneratedLayerIDs(c.templateSource, detached)
	return CompileOutput{
		Status:      compileStatusFailed,
		Document:    detached,
		Manifest:    c.manifest,
		Diagnostics: c.diagnostics,
	}
}

func (c *listPageCompiler) addError(code, message string, paths ...string) {
	c.diagnostics = append(c.diagnostics, Diagnostic{Code: code, Severity: DiagnosticError, Message: message, Paths: paths})
}

func (c *listPageCompiler) markGeneratedRoot(layerID, role, specPath string, request RecipeRequest, fallback, pinned string) {
	c.generatedRoots[layerID] = generatedRootMetadata{
		Role: role, SpecPath: specPath, RecipeKind: request.Kind, RecipeVariant: request.Variant,
		RecipeState: request.State, RequestedRecipeVariant: request.Variant,
		RecipeFallback: fallback, Pinned: pinned,
	}
}

func (c *listPageCompiler) markResolvedRecipeRoot(layerID, role, specPath string, request RecipeRequest, recipe ComponentRecipe, fallback, pinned string) {
	c.generatedRoots[layerID] = generatedRootMetadata{
		Role: role, SpecPath: specPath,
		RecipeKind: recipe.Kind, RecipeVariant: recipe.Variant, RecipeState: recipe.State,
		RequestedRecipeVariant: request.Variant, RecipeFallback: fallback,
		RecipeSourceRevisionID: recipe.Source.RevisionID, RecipeSourceRootLayerID: recipe.Source.RootLayerID,
		RecipeSourceFingerprint: recipe.Source.Fingerprint, TextOverflow: recipe.Layout.TextOverflow,
		Pinned: pinned,
	}
}

func (c *listPageCompiler) markGeneratedPrototype(layerID, role, specPath, pinned string) {
	c.markGeneratedRoot(layerID, role, specPath, RecipeRequest{}, "", pinned)
}

func (c *listPageCompiler) resolveRecipe(request RecipeRequest, specPath string) (ResolvedRecipe, bool) {
	resolved, diagnostics := ResolveRecipe(c.input.RecipeSet, request)
	c.diagnostics = append(c.diagnostics, diagnostics...)
	if diagnostics.HasErrors() {
		return ResolvedRecipe{}, false
	}
	if resolved.Fallback == "default" && (request.Variant != "default" || request.State != "default") {
		c.diagnostics = append(c.diagnostics, Diagnostic{
			Code: "recipe_default_fallback", Severity: DiagnosticWarning,
			Message: fmt.Sprintf("resolved %s/%s/%s with the same-kind default recipe", request.Kind, request.Variant, request.State),
			Paths:   []string{specPath},
		})
	}
	if resolved.Recipe != nil && resolved.Recipe.Kind != request.Kind {
		c.addError("recipe_kind_mismatch", fmt.Sprintf("resolved recipe kind %q does not match requested kind %q", resolved.Recipe.Kind, request.Kind), specPath)
		return ResolvedRecipe{}, false
	}
	if resolved.Primitive != nil && resolved.Primitive.Kind != request.Kind {
		c.addError("recipe_kind_mismatch", fmt.Sprintf("resolved primitive kind %q does not match requested kind %q", resolved.Primitive.Kind, request.Kind), specPath)
		return ResolvedRecipe{}, false
	}
	return resolved, true
}

func (c *listPageCompiler) instantiateComponent(parentID string, bounds Rect, request RecipeRequest, bindings componentBindingPlan, role, specPath, pinned string) (instantiatedComponent, error) {
	resolved, ok := c.resolveRecipe(request, specPath)
	if !ok {
		return instantiatedComponent{}, nil
	}
	props, layout := resolvedRecipePropsAndLayout(resolved)
	if !validateComponentBindingPlan(props, bindings) {
		c.addError("missing_recipe_prop", fmt.Sprintf("recipe %s/%s/%s does not declare the required label/value prop", request.Kind, request.Variant, request.State), specPath)
		return instantiatedComponent{}, nil
	}
	bounds = c.componentBounds(bounds, resolved, layout)
	if resolved.Recipe != nil {
		clone, err := c.builder.CloneSubtree(c.input.RecipeDoc, resolved.Recipe.Source.RootLayerID, parentID, c.input.Blueprint.FrameID, bounds)
		if err != nil {
			return instantiatedComponent{}, err
		}
		c.markResolvedRecipeRoot(clone.RootLayerID, role, specPath, request, *resolved.Recipe, resolved.Fallback, pinned)
		if err := c.bindRecipeClone(clone, props, bindings, bounds, layout.TextOverflow); err != nil {
			return instantiatedComponent{}, err
		}
		return instantiatedComponent{RootLayerID: clone.RootLayerID, Bounds: bounds, Layout: layout}, nil
	}
	if resolved.Primitive == nil {
		c.addError("missing_recipe", fmt.Sprintf("no executable recipe exists for component kind %q", request.Kind), specPath)
		return instantiatedComponent{}, nil
	}
	return c.instantiatePrimitive(parentID, bounds, request, *resolved.Primitive, bindings, role, specPath, pinned)
}

func resolvedRecipePropsAndLayout(resolved ResolvedRecipe) (map[string]RecipeProp, RecipeLayout) {
	if resolved.Recipe != nil {
		return resolved.Recipe.Props, resolved.Recipe.Layout
	}
	if resolved.Primitive != nil {
		return resolved.Primitive.Props, resolved.Primitive.Layout
	}
	return nil, RecipeLayout{}
}

func validateComponentBindingPlan(props map[string]RecipeProp, plan componentBindingPlan) bool {
	for _, key := range plan.RequiredAll {
		if _, ok := props[key]; !ok {
			return false
		}
	}
	if len(plan.RequiredAny) > 0 {
		found := false
		for _, key := range plan.RequiredAny {
			if _, ok := props[key]; ok {
				found = true
				break
			}
		}
		if !found {
			return false
		}
	}
	return true
}

func (c *listPageCompiler) componentBounds(bounds Rect, resolved ResolvedRecipe, layout RecipeLayout) Rect {
	if bounds.Width <= 0 {
		bounds.Width = layout.MinWidth
		if resolved.Recipe != nil {
			if source, ok := c.input.RecipeDoc.Layers[resolved.Recipe.Source.RootLayerID]; ok && source.Width > bounds.Width {
				bounds.Width = source.Width
			}
		}
	}
	if bounds.Height <= 0 {
		bounds.Height = layout.Height
	}
	return bounds
}

func (c *listPageCompiler) bindRecipeClone(clone CloneResult, props map[string]RecipeProp, plan componentBindingPlan, bounds Rect, overflow string) error {
	keys := declaredCompilerBindingKeys(props, plan.Values)
	for index, key := range keys {
		prop := props[key]
		if err := c.builder.BindText(clone, prop.TargetLayerID, plan.Values[key]); err != nil {
			return err
		}
		if err := c.builder.FitCloneLayer(clone, prop.TargetLayerID, compilerTextBounds(bounds, index, len(keys))); err != nil {
			return err
		}
		targetID := clone.SourceToTarget[prop.TargetLayerID]
		c.textOverflow[targetID] = overflow
	}
	return nil
}

func (c *listPageCompiler) instantiatePrimitive(parentID string, bounds Rect, request RecipeRequest, primitive PrimitiveRecipe, bindings componentBindingPlan, role, specPath, pinned string) (instantiatedComponent, error) {
	style, err := resolvePrimitiveStyle(c.input.RecipeSet.Tokens, primitive.Style)
	if err != nil {
		c.addError("primitive_style_resolution_failed", err.Error(), specPath)
		return instantiatedComponent{}, nil
	}
	rootID, err := c.builder.AddPrimitiveLayer(parentID, Layer{
		ID: "primitive-" + primitive.Kind, Name: primitive.Kind, Type: primitive.LayerType, Visible: true,
		X: bounds.X, Y: bounds.Y, Width: bounds.Width, Height: bounds.Height, Style: style,
	})
	if err != nil {
		return instantiatedComponent{}, err
	}
	c.markGeneratedRoot(rootID, role, specPath, request, "primitive", pinned)
	metadata := c.generatedRoots[rootID]
	metadata.TextOverflow = primitive.Layout.TextOverflow
	c.generatedRoots[rootID] = metadata
	keys := declaredCompilerBindingKeys(primitive.Props, bindings.Values)
	for index, key := range keys {
		prop := primitive.Props[key]
		textBounds := compilerTextBounds(bounds, index, len(keys))
		targetID, addErr := c.builder.AddPrimitiveLayer(rootID, Layer{
			ID: prop.TargetLayerID, Name: prop.TargetLayerID, Type: prop.Type, Visible: true,
			X: textBounds.X, Y: textBounds.Y, Width: textBounds.Width, Height: textBounds.Height,
			Text: map[string]any{"characters": bindings.Values[key]},
		})
		if addErr != nil {
			return instantiatedComponent{}, addErr
		}
		c.markGeneratedPrototype(targetID, role+"-text-target", specPath+".props."+key, pinned)
		c.textOverflow[targetID] = primitive.Layout.TextOverflow
	}
	return instantiatedComponent{RootLayerID: rootID, Bounds: bounds, Layout: primitive.Layout}, nil
}

func declaredCompilerBindingKeys(props map[string]RecipeProp, values map[string]string) []string {
	result := make([]string, 0, 2)
	for _, key := range []string{"label", "value"} {
		if _, declared := props[key]; !declared {
			continue
		}
		if _, supplied := values[key]; supplied {
			result = append(result, key)
		}
	}
	return result
}

func compilerTextBounds(bounds Rect, index, count int) Rect {
	width := bounds.Width - 2*compilerCellHorizontalPadding
	if width < 0 {
		width = 0
	}
	height := bounds.Height
	if count > 0 {
		height = bounds.Height / float64(count)
	}
	return Rect{
		X:      bounds.X + compilerCellHorizontalPadding,
		Y:      bounds.Y + float64(index)*height,
		Width:  width,
		Height: height,
	}
}

func resolvePrimitiveStyle(tokens map[string]any, style map[string]any) (map[string]any, error) {
	resolved, err := resolvePrimitiveStyleValue(tokens, style)
	if err != nil {
		return nil, err
	}
	result, ok := resolved.(map[string]any)
	if !ok {
		return nil, fmt.Errorf("primitive style must be an object")
	}
	return result, nil
}

func resolvePrimitiveStyleValue(tokens map[string]any, value any) (any, error) {
	switch typed := value.(type) {
	case string:
		if !strings.HasPrefix(typed, "$") {
			return nil, fmt.Errorf("primitive style leaf %q is not a token reference", typed)
		}
		resolved, ok := lookupCompilerToken(tokens, strings.TrimPrefix(typed, "$"))
		if !ok {
			return nil, fmt.Errorf("token reference %q does not exist", typed)
		}
		return detachCompilerJSONValue(resolved)
	case map[string]any:
		result := make(map[string]any, len(typed))
		keys := make([]string, 0, len(typed))
		for key := range typed {
			keys = append(keys, key)
		}
		sort.Strings(keys)
		for _, key := range keys {
			child, err := resolvePrimitiveStyleValue(tokens, typed[key])
			if err != nil {
				return nil, err
			}
			result[key] = child
		}
		return result, nil
	case []any:
		result := make([]any, len(typed))
		for index, child := range typed {
			resolved, err := resolvePrimitiveStyleValue(tokens, child)
			if err != nil {
				return nil, err
			}
			result[index] = resolved
		}
		return result, nil
	default:
		return nil, fmt.Errorf("primitive style leaf has unsupported type %T", value)
	}
}

func detachCompilerJSONValue(value any) (any, error) {
	raw, err := json.Marshal(value)
	if err != nil {
		return nil, fmt.Errorf("copy token value: %w", err)
	}
	var detached any
	if err := json.Unmarshal(raw, &detached); err != nil {
		return nil, fmt.Errorf("copy token value: %w", err)
	}
	return detached, nil
}

func lookupCompilerToken(tokens map[string]any, path string) (any, bool) {
	var current any = tokens
	for _, part := range strings.Split(path, ".") {
		object, ok := current.(map[string]any)
		if !ok {
			return nil, false
		}
		current, ok = object[part]
		if !ok {
			return nil, false
		}
	}
	return current, true
}

func (c *listPageCompiler) applyFinalEdits(document *NativeJSON) {
	rootIDs := make([]string, 0, len(c.generatedRoots))
	for layerID := range c.generatedRoots {
		rootIDs = append(rootIDs, layerID)
	}
	sort.Strings(rootIDs)
	for _, layerID := range rootIDs {
		metadata := c.generatedRoots[layerID]
		layer, ok := document.Layers[layerID]
		if !ok {
			continue
		}
		if layer.Semantic == nil {
			layer.Semantic = make(map[string]any)
		}
		layer.Semantic["generatedBy"] = DesignCompilerVersion
		layer.Semantic["generationRole"] = metadata.Role
		layer.Semantic["specPath"] = metadata.SpecPath
		if metadata.RecipeKind != "" {
			layer.Semantic["recipeKind"] = metadata.RecipeKind
			layer.Semantic["recipeVariant"] = metadata.RecipeVariant
			layer.Semantic["recipeState"] = metadata.RecipeState
			layer.Semantic["requestedRecipeVariant"] = metadata.RequestedRecipeVariant
			layer.Semantic["recipeFallback"] = metadata.RecipeFallback
			layer.Semantic["textOverflow"] = metadata.TextOverflow
		}
		if metadata.RecipeSourceRootLayerID != "" {
			layer.Semantic["recipeSourceRevisionId"] = metadata.RecipeSourceRevisionID
			layer.Semantic["recipeSourceRootLayerId"] = metadata.RecipeSourceRootLayerID
			layer.Semantic["recipeSourceFingerprint"] = metadata.RecipeSourceFingerprint
		}
		if metadata.Pinned != "" {
			layer.Semantic["pinned"] = metadata.Pinned
		} else {
			delete(layer.Semantic, "pinned")
		}
		document.Layers[layerID] = layer
	}

	overflowIDs := make([]string, 0, len(c.textOverflow))
	for layerID := range c.textOverflow {
		overflowIDs = append(overflowIDs, layerID)
	}
	sort.Strings(overflowIDs)
	for _, layerID := range overflowIDs {
		layer, ok := document.Layers[layerID]
		if !ok {
			continue
		}
		if layer.Text == nil {
			layer.Text = make(map[string]any)
		}
		layer.Text["overflow"] = c.textOverflow[layerID]
		document.Layers[layerID] = layer
	}

	if c.navigation != nil {
		layer := document.Layers[c.navigation.LayerID]
		if layer.Text == nil {
			layer.Text = make(map[string]any)
		}
		if layer.Semantic == nil {
			layer.Semantic = make(map[string]any)
		}
		layer.Text["characters"] = c.navigation.Label
		layer.Semantic["active"] = true
		layer.Semantic["activeNavigation"] = c.navigation.Label
		document.Layers[c.navigation.LayerID] = layer
	}

	if c.tableRegionID != "" {
		layer := document.Layers[c.tableRegionID]
		if layer.Semantic == nil {
			layer.Semantic = make(map[string]any)
		}
		delete(layer.Semantic, "horizontalScroll")
		delete(layer.Semantic, "clipContent")
		delete(layer.Semantic, "horizontalScrollStrategy")
		if c.manifest.HorizontalScroll {
			layer.Semantic["horizontalScroll"] = true
			layer.Semantic["clipContent"] = true
			layer.Semantic["horizontalScrollStrategy"] = c.horizontalStrategy
		}
		document.Layers[c.tableRegionID] = layer
	}

	if document.Source == nil {
		document.Source = make(map[string]any)
	}
	document.Source["generation"] = map[string]any{
		"compilerVersion":          DesignCompilerVersion,
		"pageSpecVersion":          c.input.Provenance.PageSpecVersion,
		"blueprintRecordId":        c.input.Provenance.BlueprintRecordID,
		"recipeSetRecordId":        c.input.Provenance.RecipeSetRecordID,
		"templateSourceRevisionId": c.input.Blueprint.SourceRefs.TemplateRevisionID,
		"designSourceRevisionId":   c.input.Blueprint.SourceRefs.DesignRevisionID,
		"uiSpecSourceRevisionId":   c.input.RecipeSet.SourceRevisionID,
		"workspaceId":              c.input.Provenance.WorkspaceID,
		"projectId":                c.input.Provenance.ProjectID,
		"issueId":                  c.input.Provenance.IssueID,
		"agentTaskId":              c.input.Provenance.AgentTaskID,
		"horizontalScrollStrategy": c.horizontalStrategy,
	}
}

func compilerNamespace(input CompileInput) string {
	payload := struct {
		CompilerVersion        string
		PageSpecContract       string
		BlueprintContract      string
		RecipeSetContract      string
		NativeJSONContract     string
		Provenance             CompileProvenance
		TemplateRevisionID     string
		DesignRevisionID       string
		RecipeSourceRevisionID string
	}{
		CompilerVersion: DesignCompilerVersion, PageSpecContract: input.PageSpec.Version,
		BlueprintContract: input.Blueprint.Version, RecipeSetContract: input.RecipeSet.Version,
		NativeJSONContract: NativeJSONVersion, Provenance: input.Provenance,
		TemplateRevisionID:     input.Blueprint.SourceRefs.TemplateRevisionID,
		DesignRevisionID:       input.Blueprint.SourceRefs.DesignRevisionID,
		RecipeSourceRevisionID: input.RecipeSet.SourceRevisionID,
	}
	raw, _ := json.Marshal(payload)
	sum := sha256.Sum256(raw)
	return "list-compiler-" + hex.EncodeToString(sum[:])
}

func compilerBusinessRegionLayerIDs(blueprint TemplateBlueprint) []string {
	set := make(map[string]struct{}, len(replaceableBusinessRegions))
	for _, key := range replaceableBusinessRegions {
		if region, ok := blueprint.Regions[key]; ok && region.RootLayerID != "" {
			set[region.RootLayerID] = struct{}{}
		}
	}
	result := make([]string, 0, len(set))
	for layerID := range set {
		result = append(result, layerID)
	}
	sort.Strings(result)
	return result
}

func compilerGeneratedLayerIDs(template, document NativeJSON) []string {
	result := make([]string, 0)
	for layerID := range document.Layers {
		if _, existed := template.Layers[layerID]; !existed {
			result = append(result, layerID)
		}
	}
	sort.Strings(result)
	return result
}

func normalizeCompilerDiagnostics(source Diagnostics) Diagnostics {
	result := make(Diagnostics, len(source))
	for index, diagnostic := range source {
		diagnostic.Paths = append([]string(nil), diagnostic.Paths...)
		diagnostic.LayerIDs = append([]string(nil), diagnostic.LayerIDs...)
		sort.Strings(diagnostic.Paths)
		sort.Strings(diagnostic.LayerIDs)
		result[index] = diagnostic
	}
	sort.SliceStable(result, func(i, j int) bool {
		left := compilerDiagnosticKey(result[i])
		right := compilerDiagnosticKey(result[j])
		return left < right
	})
	return result
}

func compilerDiagnosticKey(diagnostic Diagnostic) string {
	return string(diagnostic.Severity) + "\x00" + diagnostic.Code + "\x00" + diagnostic.Message + "\x00" + strings.Join(diagnostic.Paths, "\x00") + "\x00" + strings.Join(diagnostic.LayerIDs, "\x00")
}
