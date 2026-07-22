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
	compileStatusCompiled = compileStatusGenerated

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
	PageSpec               PageSpec
	RequiredRequirementIDs []string
	Blueprint              TemplateBlueprint
	RecipeSet              ComponentRecipeSet
	TemplateDoc            NativeJSON
	RecipeDoc              NativeJSON
	Provenance             CompileProvenance
}

type CompilationManifest struct {
	FilterCount            int
	PageActionCount        int
	ColumnCount            int
	RowCount               int
	RowActionCount         int
	HorizontalScroll       bool
	TableContentBounds     Rect
	GeneratedLayerIDs      []string
	BusinessRegionLayerIDs []string
	TemplateBusinessTexts  []string
	ResolvedComponents     []ResolvedComponentExpectation
}

type ResolvedComponentExpectation struct {
	GeneratedRootLayerID string
	RecipeKind           string
	RecipeVariant        string
	RecipeState          string
	RequestedVariant     string
	Fallback             string
	SourceRevisionID     string
	SourceRootLayerID    string
	SourceFingerprint    string
	OutputFingerprint    string
	TextOverflow         string
	OverlayRole          string
}

type CompileOutput struct {
	Status      string
	Document    NativeJSON
	Manifest    CompilationManifest
	Diagnostics Diagnostics
	Quality     QualityReport
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
	OverlayRole             string
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
	compiler.manifest.ResolvedComponents = compilerResolvedComponentExpectations(compiler.generatedRoots, final)
	quality := EvaluateCompiledDesign(final, input.PageSpec, input.Blueprint, compiler.manifest, compiler.diagnostics)
	return CompileOutput{
		Status:      quality.Status,
		Document:    final,
		Manifest:    compiler.manifest,
		Diagnostics: quality.Diagnostics,
		Quality:     quality,
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
	requiredIDs, requirementDiagnostics := normalizeRequiredRequirementIDs(c.input.RequiredRequirementIDs)
	c.diagnostics = append(c.diagnostics, requirementDiagnostics...)
	c.diagnostics = append(c.diagnostics, ValidatePageSpec(c.input.PageSpec, requiredIDs)...)
	c.validateSourceIdentity()
	if templateValidation.Valid {
		structure := ExtractTemplateStructure(c.input.TemplateDoc)
		c.diagnostics = append(c.diagnostics, ValidateTemplateBlueprintForPageSpec(structure, c.input.Blueprint, c.input.PageSpec)...)
	}
	if recipeValidation.Valid {
		c.diagnostics = append(c.diagnostics, ValidateComponentRecipeSet(c.input.RecipeDoc, c.input.RecipeSet)...)
	}
	c.diagnostics = normalizeCompilerDiagnostics(c.diagnostics)
}

func normalizeRequiredRequirementIDs(source []string) ([]string, Diagnostics) {
	diagnostics := Diagnostics{}
	if len(source) == 0 {
		diagnostics.addError("missing_required_requirement_ids", "semantic compilation requires at least one required requirement ID", "requiredRequirementIds")
		return []string{}, diagnostics
	}
	seen := make(map[string]struct{}, len(source))
	result := make([]string, 0, len(source))
	for index, rawID := range source {
		id := strings.TrimSpace(rawID)
		path := fmt.Sprintf("requiredRequirementIds.%d", index)
		if id == "" {
			diagnostics.addError("blank_required_requirement_id", "required requirement IDs must not be blank", path)
			continue
		}
		if _, exists := seen[id]; exists {
			diagnostics.addError("duplicate_required_requirement_id", fmt.Sprintf("required requirement ID %q is duplicated", id), path)
			continue
		}
		seen[id] = struct{}{}
		result = append(result, id)
	}
	if len(result) == 0 {
		diagnostics.addError("missing_required_requirement_ids", "semantic compilation requires at least one non-blank required requirement ID", "requiredRequirementIds")
	}
	sort.Strings(result)
	return result, diagnostics
}

func (c *listPageCompiler) validateSourceIdentity() {
	validateOptionalSourceIdentity(
		&c.diagnostics,
		c.input.TemplateDoc.Source,
		c.input.Blueprint.SourceRefs.DesignRevisionID,
		"template",
		"templateDoc.source.revisionId",
	)
	validateOptionalSourceIdentity(
		&c.diagnostics,
		c.input.RecipeDoc.Source,
		c.input.RecipeSet.SourceRevisionID,
		"recipe",
		"recipeDoc.source.revisionId",
	)
}

func validateOptionalSourceIdentity(diagnostics *Diagnostics, source map[string]any, expected, scope, path string) {
	value, present := source["revisionId"]
	if !present {
		return
	}
	revisionID, ok := value.(string)
	if !ok {
		diagnostics.addError("invalid_"+scope+"_source_identity", fmt.Sprintf("%s source revisionId must be a string when present", scope), path)
		return
	}
	if revisionID == "" {
		return
	}
	if revisionID != expected {
		diagnostics.addError(scope+"_source_identity_mismatch", fmt.Sprintf("%s source revision %q does not match %q", scope, revisionID, expected), path)
	}
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
		Quality: QualityReport{
			Status:      compileStatusFailed,
			Diagnostics: c.diagnostics,
		},
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
		OverlayRole: recipe.Layout.OverlayRole,
		Pinned:      pinned,
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

func (c *listPageCompiler) resolveExactRecipe(request RecipeRequest, specPath string) (ResolvedRecipe, bool) {
	key := (RecipeKey{Kind: request.Kind, Variant: request.Variant, State: request.State}).String()
	recipe, ok := c.input.RecipeSet.Recipes[key]
	if !ok || recipe.Kind != request.Kind || recipe.Variant != request.Variant || recipe.State != request.State {
		c.addError("missing_recipe", fmt.Sprintf("exact recipe %s is required", key), specPath)
		return ResolvedRecipe{}, false
	}
	copy := recipe
	return ResolvedRecipe{Recipe: &copy, Fallback: "exact"}, true
}

func (c *listPageCompiler) instantiateComponent(parentID string, bounds Rect, request RecipeRequest, bindings componentBindingPlan, role, specPath, pinned string) (instantiatedComponent, error) {
	resolved, ok := c.resolveRecipe(request, specPath)
	if !ok {
		return instantiatedComponent{}, nil
	}
	return c.instantiateResolvedComponent(parentID, bounds, request, resolved, bindings, role, specPath, pinned)
}

func (c *listPageCompiler) instantiateExactRecipeComponent(parentID string, bounds Rect, request RecipeRequest, bindings componentBindingPlan, role, specPath, pinned string) (instantiatedComponent, error) {
	resolved, ok := c.resolveExactRecipe(request, specPath)
	if !ok {
		return instantiatedComponent{}, nil
	}
	return c.instantiateResolvedComponent(parentID, bounds, request, resolved, bindings, role, specPath, pinned)
}

func (c *listPageCompiler) instantiateResolvedComponent(parentID string, bounds Rect, request RecipeRequest, resolved ResolvedRecipe, bindings componentBindingPlan, role, specPath, pinned string) (instantiatedComponent, error) {
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
		code := "primitive_style_resolution_failed"
		if resolutionError, ok := err.(*primitiveTokenResolutionError); ok {
			code = resolutionError.Code
		}
		c.addError(code, err.Error(), specPath)
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
	metadata.OverlayRole = primitive.Layout.OverlayRole
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
	resolver := primitiveTokenResolver{
		tokens: tokens,
		active: make(map[string]int),
	}
	resolved, err := resolver.resolveStyleValue(style)
	if err != nil {
		return nil, err
	}
	result, ok := resolved.(map[string]any)
	if !ok {
		return nil, fmt.Errorf("primitive style must be an object")
	}
	if reference, found := firstCompilerTokenReference(result); found {
		return nil, &primitiveTokenResolutionError{
			Code:    "primitive_token_unresolved",
			Message: fmt.Sprintf("primitive style retained unresolved token reference %q", reference),
		}
	}
	return result, nil
}

func resolvePrimitiveStyleValue(tokens map[string]any, value any) (any, error) {
	resolver := primitiveTokenResolver{
		tokens: tokens,
		active: make(map[string]int),
	}
	return resolver.resolveStyleValue(value)
}

type primitiveTokenResolutionError struct {
	Code    string
	Message string
}

func (e *primitiveTokenResolutionError) Error() string {
	return e.Message
}

type primitiveTokenResolver struct {
	tokens map[string]any
	stack  []string
	active map[string]int
}

func (r *primitiveTokenResolver) resolveStyleValue(value any) (any, error) {
	switch typed := value.(type) {
	case string:
		if !strings.HasPrefix(typed, "$") {
			return nil, fmt.Errorf("primitive style leaf %q is not a token reference", typed)
		}
		return r.resolveReference(strings.TrimPrefix(typed, "$"))
	case map[string]any:
		result := make(map[string]any, len(typed))
		keys := make([]string, 0, len(typed))
		for key := range typed {
			keys = append(keys, key)
		}
		sort.Strings(keys)
		for _, key := range keys {
			child, err := r.resolveStyleValue(typed[key])
			if err != nil {
				return nil, err
			}
			result[key] = child
		}
		return result, nil
	case []any:
		result := make([]any, len(typed))
		for index, child := range typed {
			resolved, err := r.resolveStyleValue(child)
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

func (r *primitiveTokenResolver) resolveReference(path string) (any, error) {
	if index, cycling := r.active[path]; cycling {
		cycle := append(append([]string(nil), r.stack[index:]...), path)
		return nil, &primitiveTokenResolutionError{
			Code:    "primitive_token_cycle",
			Message: "primitive token alias cycle: " + strings.Join(cycle, " -> "),
		}
	}
	value, ok := lookupCompilerToken(r.tokens, path)
	if !ok {
		return nil, &primitiveTokenResolutionError{
			Code:    "primitive_token_missing",
			Message: fmt.Sprintf("primitive token reference $%s does not exist", path),
		}
	}
	r.active[path] = len(r.stack)
	r.stack = append(r.stack, path)
	resolved, err := r.resolveTokenValue(value)
	r.stack = r.stack[:len(r.stack)-1]
	delete(r.active, path)
	return resolved, err
}

func (r *primitiveTokenResolver) resolveTokenValue(value any) (any, error) {
	switch typed := value.(type) {
	case string:
		if strings.HasPrefix(typed, "$") {
			return r.resolveReference(strings.TrimPrefix(typed, "$"))
		}
		return detachCompilerJSONValue(typed)
	case map[string]any:
		result := make(map[string]any, len(typed))
		keys := make([]string, 0, len(typed))
		for key := range typed {
			keys = append(keys, key)
		}
		sort.Strings(keys)
		for _, key := range keys {
			child, err := r.resolveTokenValue(typed[key])
			if err != nil {
				return nil, err
			}
			result[key] = child
		}
		return result, nil
	case []any:
		result := make([]any, len(typed))
		for index, child := range typed {
			resolved, err := r.resolveTokenValue(child)
			if err != nil {
				return nil, err
			}
			result[index] = resolved
		}
		return result, nil
	default:
		return detachCompilerJSONValue(typed)
	}
}

func firstCompilerTokenReference(value any) (string, bool) {
	switch typed := value.(type) {
	case string:
		if strings.HasPrefix(typed, "$") {
			return typed, true
		}
	case map[string]any:
		keys := make([]string, 0, len(typed))
		for key := range typed {
			keys = append(keys, key)
		}
		sort.Strings(keys)
		for _, key := range keys {
			if reference, found := firstCompilerTokenReference(typed[key]); found {
				return reference, true
			}
		}
	case []any:
		for _, child := range typed {
			if reference, found := firstCompilerTokenReference(child); found {
				return reference, true
			}
		}
	}
	return "", false
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
		if c.textOverflow[layerID] == "ellipsis" {
			layer.Text["clip"] = true
			layer.Text["maxLines"] = 1
		} else {
			delete(layer.Text, "clip")
			delete(layer.Text, "maxLines")
		}
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

func compilerResolvedComponentExpectations(generatedRoots map[string]generatedRootMetadata, document NativeJSON) []ResolvedComponentExpectation {
	rootIDs := make([]string, 0, len(generatedRoots))
	for rootID, metadata := range generatedRoots {
		if metadata.RecipeKind != "" {
			rootIDs = append(rootIDs, rootID)
		}
	}
	sort.Strings(rootIDs)
	result := make([]ResolvedComponentExpectation, 0, len(rootIDs))
	for _, rootID := range rootIDs {
		metadata := generatedRoots[rootID]
		result = append(result, ResolvedComponentExpectation{
			GeneratedRootLayerID: rootID,
			RecipeKind:           metadata.RecipeKind,
			RecipeVariant:        metadata.RecipeVariant,
			RecipeState:          metadata.RecipeState,
			RequestedVariant:     metadata.RequestedRecipeVariant,
			Fallback:             metadata.RecipeFallback,
			SourceRevisionID:     metadata.RecipeSourceRevisionID,
			SourceRootLayerID:    metadata.RecipeSourceRootLayerID,
			SourceFingerprint:    metadata.RecipeSourceFingerprint,
			OutputFingerprint:    fingerprintGeneratedSubtree(document, rootID),
			TextOverflow:         metadata.TextOverflow,
			OverlayRole:          metadata.OverlayRole,
		})
	}
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
