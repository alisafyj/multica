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
	input, output := completeQualityGateOutput(t)
	doc := copyQualityGateDocument(t, output.Document)
	text := qualityGeneratedTextLayer(t, doc)
	text.Text["characters"] = strings.Repeat("overflow", 32)
	text.Text["overflow"] = "clip"
	doc.Layers[text.ID] = text

	report := EvaluateCompiledDesign(doc, input.PageSpec, input.Blueprint, output.Manifest, nil)
	assertDiagnosticCode(t, report.Diagnostics, "text_overflow")
}

func TestEvaluateCompiledDesignBlocksUnexpectedOverlapButAllowsContainment(t *testing.T) {
	input, output := completeQualityGateOutput(t)
	clean := EvaluateCompiledDesign(output.Document, input.PageSpec, input.Blueprint, output.Manifest, nil)
	assertNoDiagnosticCode(t, clean.Diagnostics, "unexpected_overlap")

	doc := copyQualityGateDocument(t, output.Document)
	parent := qualityGeneratedRoot(t, doc, "filters.name")
	overlap := Layer{
		ID: "gen-quality-overlap", FrameID: parent.FrameID, ParentID: parent.ParentID,
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
	addQualityGateChild(t, &doc, overlap)

	report := EvaluateCompiledDesign(doc, input.PageSpec, input.Blueprint, output.Manifest, nil)
	assertDiagnosticCode(t, report.Diagnostics, "unexpected_overlap")
}

func TestEvaluateCompiledDesignBlocksOffFrame(t *testing.T) {
	input, output := completeQualityGateOutput(t)
	doc := copyQualityGateDocument(t, output.Document)
	layer := qualityGeneratedRoot(t, doc, "filters.name")
	layer.X = 100000
	doc.Layers[layer.ID] = layer

	report := EvaluateCompiledDesign(doc, input.PageSpec, input.Blueprint, output.Manifest, nil)
	assertDiagnosticCode(t, report.Diagnostics, "off_frame")
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
	delete(layer.Semantic, "recipeSourceFingerprint")
	doc.Layers[layer.ID] = layer

	report := EvaluateCompiledDesign(doc, input.PageSpec, input.Blueprint, output.Manifest, nil)
	assertDiagnosticCode(t, report.Diagnostics, "component_nonconformance")
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

func qualityGeneratedTextLayer(t *testing.T, doc NativeJSON) Layer {
	t.Helper()
	for _, layer := range doc.Layers {
		if layer.Text["overflow"] != nil && layer.Text["characters"] != nil {
			return layer
		}
	}
	t.Fatal("generated text layer not found")
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

func assertNoDiagnosticCode(t *testing.T, diagnostics Diagnostics, want string) {
	t.Helper()
	for _, diagnostic := range diagnostics {
		if diagnostic.Code == want {
			t.Fatalf("unexpected diagnostic %q: %+v", want, diagnostics)
		}
	}
}
