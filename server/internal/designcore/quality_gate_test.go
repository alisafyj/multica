package designcore

import (
	"strings"
	"testing"
)

func TestEvaluateCompiledDesignReturnsGeneratedStatuses(t *testing.T) {
	input, output := completeQualityGateOutput(t)

	clean := EvaluateCompiledDesign(output.Document, input.PageSpec, input.Blueprint, output.Manifest, nil)
	if clean.Status != "generated" {
		t.Fatalf("clean status = %q, diagnostics = %+v", clean.Status, clean.Diagnostics)
	}
	if clean.Diagnostics.HasErrors() {
		t.Fatalf("clean diagnostics = %+v", clean.Diagnostics)
	}

	warning := EvaluateCompiledDesign(output.Document, input.PageSpec, input.Blueprint, output.Manifest, Diagnostics{{
		Code: "upstream_warning", Severity: DiagnosticWarning, Message: "retain warning",
	}})
	if warning.Status != "generated_with_warnings" {
		t.Fatalf("warning status = %q, diagnostics = %+v", warning.Status, warning.Diagnostics)
	}

	failure := EvaluateCompiledDesign(output.Document, input.PageSpec, input.Blueprint, output.Manifest, Diagnostics{{
		Code: "upstream_error", Severity: DiagnosticError, Message: "retain error",
	}})
	if failure.Status != "compile_failed" {
		t.Fatalf("failure status = %q, diagnostics = %+v", failure.Status, failure.Diagnostics)
	}
}

func TestEvaluateCompiledDesignBlocksTextOverflow(t *testing.T) {
	t.Run("ellipsis", func(t *testing.T) {
		input := completeCompilerInputForTest(t)
		input.PageSpec.Filters[0].Placeholder = strings.Repeat("overflow ", 48)

		output := CompileListPage(input)
		if output.Status != "compile_failed" {
			t.Fatalf("status = %q, diagnostics = %+v", output.Status, output.Diagnostics)
		}
		assertDiagnosticCode(t, output.Diagnostics, "text_overflow")
	})

	t.Run("explicit wrap", func(t *testing.T) {
		input := completeCompilerInputForTest(t)
		input.PageSpec.Filters[0].Placeholder = strings.Repeat("overflow ", 48)
		key := (RecipeKey{Kind: "input", Variant: "default", State: "default"}).String()
		recipe := input.RecipeSet.Recipes[key]
		recipe.Layout.TextOverflow = "wrap"
		input.RecipeSet.Recipes[key] = recipe

		output := CompileListPage(input)
		if output.Status != "generated" {
			t.Fatalf("status = %q, diagnostics = %+v", output.Status, output.Diagnostics)
		}
		assertNoDiagnosticCode(t, output.Diagnostics, "text_overflow")
	})
}

func TestEvaluateCompiledDesignBlocksUnexpectedOverlapButAllowsContainment(t *testing.T) {
	input, output := completeQualityGateOutput(t)
	clean := EvaluateCompiledDesign(output.Document, input.PageSpec, input.Blueprint, output.Manifest, nil)
	assertNoDiagnosticCode(t, clean.Diagnostics, "unexpected_overlap")

	doc := copyQualityGateDocument(t, output.Document)
	parent := qualityGeneratedRoot(t, doc, "filters.name")
	overlap := qualityOverlappingComponent(parent, "gen-quality-overlap")
	addQualityGateChild(t, &doc, overlap)

	report := EvaluateCompiledDesign(doc, input.PageSpec, input.Blueprint, output.Manifest, nil)
	assertDiagnosticCode(t, report.Diagnostics, "unexpected_overlap")
}

func TestEvaluateCompiledDesignBlocksOffFrame(t *testing.T) {
	input, output := completeQualityGateOutput(t)
	frame := output.Document.Frames[0]
	for _, edge := range []struct {
		name string
		move func(*Layer)
	}{
		{name: "left", move: func(layer *Layer) { layer.X = frame.X - 1 }},
		{name: "top", move: func(layer *Layer) { layer.Y = frame.Y - 1 }},
		{name: "right", move: func(layer *Layer) { layer.X = frame.X + frame.Width - layer.Width + 1 }},
		{name: "bottom", move: func(layer *Layer) { layer.Y = frame.Y + frame.Height - layer.Height + 1 }},
	} {
		t.Run(edge.name, func(t *testing.T) {
			doc := copyQualityGateDocument(t, output.Document)
			layer := qualityGeneratedRoot(t, doc, "filters.name")
			edge.move(&layer)
			doc.Layers[layer.ID] = layer

			report := EvaluateCompiledDesign(doc, input.PageSpec, input.Blueprint, output.Manifest, nil)
			assertDiagnosticCode(t, report.Diagnostics, "off_frame")
		})
	}

	t.Run("horizontal scroll content bounds", func(t *testing.T) {
		scrollInput := completeCompilerInputForTest(t)
		scrollInput.PageSpec.Filters = nil
		scrollInput.PageSpec.PageActions = nil
		scrollInput.Blueprint.Constraints.ContentWidth = 300
		scrollOutput := CompileListPage(scrollInput)
		if scrollOutput.Status != "generated" || !scrollOutput.Manifest.HorizontalScroll {
			t.Fatalf("scroll output = %+v", scrollOutput)
		}
		bounds := scrollOutput.Manifest.TableContentBounds
		for _, edge := range []struct {
			name string
			move func(*Layer)
		}{
			{name: "left", move: func(layer *Layer) { layer.X = bounds.X - 1 }},
			{name: "top", move: func(layer *Layer) { layer.Y = bounds.Y - 1 }},
			{name: "right", move: func(layer *Layer) { layer.X = bounds.X + bounds.Width - layer.Width + 1 }},
			{name: "bottom", move: func(layer *Layer) { layer.Y = bounds.Y + bounds.Height - layer.Height + 1 }},
		} {
			t.Run(edge.name, func(t *testing.T) {
				doc := copyQualityGateDocument(t, scrollOutput.Document)
				layer := qualityGeneratedRoot(t, doc, "table.sampleRows.0.customerName")
				edge.move(&layer)
				doc.Layers[layer.ID] = layer

				report := EvaluateCompiledDesign(doc, scrollInput.PageSpec, scrollInput.Blueprint, scrollOutput.Manifest, nil)
				assertDiagnosticCode(t, report.Diagnostics, "off_frame")
			})
		}
	})
}

func TestEvaluateCompiledDesignBlocksBrokenNativeJSON(t *testing.T) {
	input, output := completeQualityGateOutput(t)
	doc := copyQualityGateDocument(t, output.Document)
	delete(doc.Layers, doc.Frames[0].RootLayerID)

	report := EvaluateCompiledDesign(doc, input.PageSpec, input.Blueprint, output.Manifest, nil)
	assertDiagnosticCode(t, report.Diagnostics, "broken_native_json")
}

func TestEvaluateCompiledDesignBlocksUnresolvedRecipe(t *testing.T) {
	input, output := completeQualityGateOutput(t)
	doc := copyQualityGateDocument(t, output.Document)
	layer := qualityGeneratedRoot(t, doc, "filters.name")
	delete(layer.Semantic, "recipeKind")
	doc.Layers[layer.ID] = layer

	report := EvaluateCompiledDesign(doc, input.PageSpec, input.Blueprint, output.Manifest, nil)
	assertDiagnosticCode(t, report.Diagnostics, "unresolved_recipe")
}

func TestEvaluateCompiledDesignBlocksUnmanifestedForgedRecipe(t *testing.T) {
	input, output := completeQualityGateOutput(t)
	doc := copyQualityGateDocument(t, output.Document)
	parent := qualityGeneratedRoot(t, doc, "filters.name")
	forged := qualityOverlappingComponent(parent, "gen-forged-recipe")
	forged.Semantic["recipeKind"] = "fabricated"
	forged.Semantic["recipeVariant"] = "fabricated"
	forged.Semantic["recipeState"] = "default"
	forged.Semantic["recipeFallback"] = "exact"
	forged.Semantic["recipeSourceRevisionId"] = "fabricated"
	forged.Semantic["recipeSourceRootLayerId"] = "fabricated"
	forged.Semantic["recipeSourceFingerprint"] = "fabricated"
	forged.Style = map[string]any{"sourceVariant": "fabricated"}
	addQualityGateChild(t, &doc, forged)

	report := EvaluateCompiledDesign(doc, input.PageSpec, input.Blueprint, output.Manifest, nil)
	assertDiagnosticCode(t, report.Diagnostics, "unresolved_recipe")
}

func TestEvaluateCompiledDesignBlocksCountMismatch(t *testing.T) {
	input, output := completeQualityGateOutput(t)
	manifest := output.Manifest
	manifest.FilterCount++

	report := EvaluateCompiledDesign(output.Document, input.PageSpec, input.Blueprint, manifest, nil)
	assertDiagnosticCode(t, report.Diagnostics, "count_mismatch")
}

func TestEvaluateCompiledDesignBlocksTemplateBusinessResidue(t *testing.T) {
	input, output := completeQualityGateOutput(t)
	doc := copyQualityGateDocument(t, output.Document)
	table := doc.Layers[input.Blueprint.Regions["table"].RootLayerID]
	residue := Layer{
		ID: "residue", FrameID: table.FrameID, ParentID: table.ID, Name: "采购价格", Type: "text", Visible: true,
		X: table.X + 20, Y: table.Y + 20, Width: 100, Height: 24,
		Text: map[string]any{"characters": "STALE table", "fontSize": 14},
	}
	addQualityGateChild(t, &doc, residue)

	report := EvaluateCompiledDesign(doc, input.PageSpec, input.Blueprint, output.Manifest, nil)
	assertDiagnosticCode(t, report.Diagnostics, "template_residue")
}

func TestEvaluateCompiledDesignBlocksMisplacedPagination(t *testing.T) {
	input, output := completeQualityGateOutput(t)
	doc := copyQualityGateDocument(t, output.Document)
	pagination := qualityGeneratedRoot(t, doc, "pagination")
	row := qualityGeneratedRoot(t, doc, "table.sampleRows.0")
	pagination.Y = row.Y
	doc.Layers[pagination.ID] = pagination

	report := EvaluateCompiledDesign(doc, input.PageSpec, input.Blueprint, output.Manifest, nil)
	assertDiagnosticCode(t, report.Diagnostics, "pagination_misplaced")
}

func TestEvaluateCompiledDesignBlocksComponentNonconformance(t *testing.T) {
	input, output := completeQualityGateOutput(t)
	doc := copyQualityGateDocument(t, output.Document)
	layer := qualityGeneratedRoot(t, doc, "filters.name")
	layer.Semantic["recipeKind"] = "fabricated"
	layer.Semantic["recipeVariant"] = "fabricated"
	layer.Semantic["recipeState"] = "default"
	layer.Semantic["recipeFallback"] = "exact"
	layer.Semantic["recipeSourceRevisionId"] = "fabricated"
	layer.Semantic["recipeSourceRootLayerId"] = "fabricated"
	layer.Semantic["recipeSourceFingerprint"] = "fabricated"
	layer.Style["sourceVariant"] = "fabricated"
	doc.Layers[layer.ID] = layer

	report := EvaluateCompiledDesign(doc, input.PageSpec, input.Blueprint, output.Manifest, nil)
	assertDiagnosticCode(t, report.Diagnostics, "component_nonconformance")
}

func TestEvaluateCompiledDesignPermitsOnlyManifestDeclaredOverlay(t *testing.T) {
	input, output := completeQualityGateOutput(t)

	t.Run("manifest declared", func(t *testing.T) {
		doc := copyQualityGateDocument(t, output.Document)
		parent := qualityGeneratedRoot(t, doc, "filters.name")
		overlay := qualityOverlappingComponent(parent, "gen-declared-overlay")
		overlay.Semantic["overlayRole"] = "untrusted-document-value"
		addQualityGateChild(t, &doc, overlay)
		manifest := output.Manifest
		manifest.FilterCount++
		manifest.GeneratedLayerIDs = append(manifest.GeneratedLayerIDs, overlay.ID)
		manifest.ResolvedComponents = append(manifest.ResolvedComponents, ResolvedComponentExpectation{
			GeneratedRootLayerID: overlay.ID,
			RecipeKind:           "input",
			RecipeVariant:        "default",
			RecipeState:          "default",
			RequestedVariant:     "default",
			Fallback:             "primitive",
			AllowOverlay:         true,
			OverlayRole:          "recipe-overlay",
		})

		report := EvaluateCompiledDesign(doc, input.PageSpec, input.Blueprint, manifest, nil)
		assertNoDiagnosticCode(t, report.Diagnostics, "unexpected_overlap")
	})

	t.Run("document declared only", func(t *testing.T) {
		doc := copyQualityGateDocument(t, output.Document)
		parent := qualityGeneratedRoot(t, doc, "filters.name")
		overlay := qualityOverlappingComponent(parent, "gen-illegal-overlay")
		overlay.Semantic["overlayRole"] = "forged"
		addQualityGateChild(t, &doc, overlay)

		report := EvaluateCompiledDesign(doc, input.PageSpec, input.Blueprint, output.Manifest, nil)
		assertDiagnosticCode(t, report.Diagnostics, "unexpected_overlap")
	})
}

func completeQualityGateOutput(t *testing.T) (CompileInput, CompileOutput) {
	t.Helper()
	input := completeCompilerInputForTest(t)
	output := CompileListPage(input)
	if output.Diagnostics.HasErrors() {
		t.Fatalf("compile diagnostics: %+v", output.Diagnostics)
	}
	return input, output
}

func copyQualityGateDocument(t *testing.T, source NativeJSON) NativeJSON {
	t.Helper()
	copy, err := copyNativeDocument(source)
	if err != nil {
		t.Fatalf("copy document: %v", err)
	}
	return copy
}

func qualityGeneratedRoot(t *testing.T, doc NativeJSON, specPath string) Layer {
	t.Helper()
	for _, layer := range doc.Layers {
		if layer.Semantic["generatedBy"] == DesignCompilerVersion && layer.Semantic["specPath"] == specPath {
			return layer
		}
	}
	t.Fatalf("generated root for %q not found", specPath)
	return Layer{}
}

func addQualityGateChild(t *testing.T, doc *NativeJSON, child Layer) {
	t.Helper()
	parent, ok := doc.Layers[child.ParentID]
	if !ok {
		t.Fatalf("parent %q not found", child.ParentID)
	}
	parent.Children = append(parent.Children, child.ID)
	doc.Layers[parent.ID] = parent
	doc.Layers[child.ID] = child
}

func qualityOverlappingComponent(parent Layer, id string) Layer {
	return Layer{
		ID: id, FrameID: parent.FrameID, ParentID: parent.ParentID,
		Name: "overlap", Type: "rectangle", Visible: true,
		X: parent.X, Y: parent.Y, Width: parent.Width, Height: parent.Height,
		Semantic: map[string]any{
			"generatedBy":            DesignCompilerVersion,
			"generationRole":         "filter-control",
			"specPath":               "filters.overlap",
			"recipeKind":             "input",
			"recipeVariant":          "default",
			"recipeState":            "default",
			"requestedRecipeVariant": "default",
			"recipeFallback":         "primitive",
		},
	}
}

func assertNoDiagnosticCode(t *testing.T, diagnostics Diagnostics, want string) {
	t.Helper()
	for _, diagnostic := range diagnostics {
		if diagnostic.Code == want {
			t.Fatalf("unexpected diagnostic %q: %+v", want, diagnostics)
		}
	}
}
