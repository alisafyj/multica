package designcore

import (
	"encoding/json"
	"reflect"
	"regexp"
	"testing"
)

func TestDocumentBuilderClonesRecipeSubtreeAndPreservesAsset(t *testing.T) {
	base := compilerTemplateForTest()
	source := recipeSourceWithImageForTest()
	baseBefore := mustMarshalDocumentBuilderTest(t, base)
	sourceBefore := mustMarshalDocumentBuilderTest(t, source)

	builder, err := NewDocumentBuilder(base, "issue-1/pagespec-v1/compiler-v1")
	if err != nil {
		t.Fatalf("NewDocumentBuilder: %v", err)
	}
	clone, err := builder.CloneSubtree(source, "input-root", "filters", "frame-1", Rect{X: 40, Y: 80, Width: 320, Height: 36})
	if err != nil {
		t.Fatalf("CloneSubtree: %v", err)
	}
	if err := builder.BindText(clone, "input-label", "客户姓名"); err != nil {
		t.Fatalf("BindText: %v", err)
	}

	doc := builder.Document()
	root := doc.Layers[clone.RootLayerID]
	if root.ParentID != "filters" || root.FrameID != "frame-1" {
		t.Fatalf("root = %+v", root)
	}
	if got := (Rect{X: root.X, Y: root.Y, Width: root.Width, Height: root.Height}); got != (Rect{X: 40, Y: 80, Width: 320, Height: 36}) {
		t.Fatalf("root bounds = %+v", got)
	}
	if got := doc.Layers[clone.SourceToTarget["input-label"]].Text["characters"]; got != "客户姓名" {
		t.Fatalf("bound text = %#v", got)
	}
	icon := doc.Layers[clone.SourceToTarget["input-icon"]]
	if icon.Image == nil || icon.Image.AssetID == "shared-asset" {
		t.Fatalf("colliding asset was not remapped: %+v", icon.Image)
	}
	if got := doc.Assets[icon.Image.AssetID].URL; got != "https://example.test/search.png" {
		t.Fatalf("asset URL = %q", got)
	}
	if got := doc.Assets["shared-asset"].URL; got != "https://example.test/template.png" {
		t.Fatalf("base collision asset changed: %q", got)
	}
	if got := mustMarshalDocumentBuilderTest(t, base); !reflect.DeepEqual(got, baseBefore) {
		t.Fatal("constructor mutated caller-owned base document")
	}
	if got := mustMarshalDocumentBuilderTest(t, source); !reflect.DeepEqual(got, sourceBefore) {
		t.Fatal("clone mutated caller-owned source document")
	}
}

func TestDocumentBuilderDeepCopiesBaseAtConstruction(t *testing.T) {
	base := compilerTemplateForTest()
	builder, err := NewDocumentBuilder(base, "base-copy")
	if err != nil {
		t.Fatalf("NewDocumentBuilder: %v", err)
	}

	base.File.Title = "mutated"
	base.Frames[0].Name = "mutated"
	base.Frames[0].Board["mode"] = "mutated"
	root := base.Layers["page-root"]
	root.Children[0] = "mutated"
	base.Layers["page-root"] = root
	asset := base.Assets["shared-asset"]
	asset.Metadata["owner"] = "mutated"
	base.Assets["shared-asset"] = asset
	base.Tokens["color"].(map[string]any)["primary"] = "mutated"

	doc := builder.Document()
	if doc.File.Title != "Compiler Template" || doc.Frames[0].Name != "Main" || doc.Frames[0].Board["mode"] != "design" {
		t.Fatalf("base frame/file mutation leaked into builder: %+v %+v", doc.File, doc.Frames[0])
	}
	if got := doc.Layers["page-root"].Children; !reflect.DeepEqual(got, []string{"filters"}) {
		t.Fatalf("base layer mutation leaked into builder: %#v", got)
	}
	if doc.Assets["shared-asset"].Metadata["owner"] != "template" || doc.Tokens["color"].(map[string]any)["primary"] != "#1677ff" {
		t.Fatal("base asset or token mutation leaked into builder")
	}
}

func TestDocumentBuilderRewritesCompleteSubtreeReferencesExactly(t *testing.T) {
	builder, err := NewDocumentBuilder(compilerTemplateForTest(), "rewrite-test")
	if err != nil {
		t.Fatalf("NewDocumentBuilder: %v", err)
	}
	clone, err := builder.CloneSubtree(recipeSourceWithImageForTest(), "input-root", "filters", "frame-1", Rect{Width: 200, Height: 32})
	if err != nil {
		t.Fatalf("CloneSubtree: %v", err)
	}

	doc := builder.Document()
	rootID := clone.SourceToTarget["input-root"]
	labelID := clone.SourceToTarget["input-label"]
	iconID := clone.SourceToTarget["input-icon"]
	root := doc.Layers[rootID]
	label := doc.Layers[labelID]
	icon := doc.Layers[iconID]
	if root.ID == "input-root" || label.ID == "input-label" || icon.ID == "input-icon" {
		t.Fatalf("source IDs survived clone: %+v", clone.SourceToTarget)
	}
	if !reflect.DeepEqual(root.Children, []string{labelID, iconID}) {
		t.Fatalf("children = %#v", root.Children)
	}
	if label.ParentID != rootID || icon.ParentID != rootID {
		t.Fatalf("parents = %q, %q; want %q", label.ParentID, icon.ParentID, rootID)
	}
	assetID := icon.Image.AssetID
	checks := map[string]any{
		"text exact":         root.Text["nodeRef"],
		"style exact":        root.Style["nodeRef"],
		"semantic exact":     root.Semantic["nodeRef"],
		"source exact":       root.Source["nodeRef"],
		"shape exact":        root.Shape["nodeRef"],
		"exportable exact":   root.Exportable[0]["nodeRef"],
		"nested node exact":  root.Style["nested"].([]any)[0],
		"nested asset exact": root.Style["nested"].([]any)[1].(map[string]any)["assetRef"],
		"image asset exact":  icon.Image.AssetID,
		"text asset exact":   root.Text["assetRef"],
		"export asset exact": root.Exportable[0]["assetRef"],
	}
	wants := map[string]any{
		"text exact": labelID, "style exact": iconID, "semantic exact": labelID,
		"source exact": iconID, "shape exact": labelID, "exportable exact": iconID,
		"nested node exact": labelID, "nested asset exact": assetID,
		"image asset exact": assetID, "text asset exact": assetID, "export asset exact": assetID,
	}
	for name, got := range checks {
		if got != wants[name] {
			t.Errorf("%s = %#v, want %#v", name, got, wants[name])
		}
	}
	for name, got := range map[string]any{
		"text substring":       root.Text["substring"],
		"style substring":      root.Style["substring"],
		"semantic substring":   root.Semantic["substring"],
		"source substring":     root.Source["substring"],
		"shape substring":      root.Shape["substring"],
		"exportable substring": root.Exportable[0]["substring"],
	} {
		if got != "prefix-input-label-suffix" {
			t.Errorf("%s was partially rewritten: %#v", name, got)
		}
	}
}

func TestDocumentBuilderIsDeterministicForIdenticalOperationSequences(t *testing.T) {
	build := func() ([]byte, []string) {
		t.Helper()
		builder, err := NewDocumentBuilder(compilerTemplateForTest(), "stable-namespace")
		if err != nil {
			t.Fatalf("NewDocumentBuilder: %v", err)
		}
		var ids []string
		for i := 0; i < 2; i++ {
			clone, err := builder.CloneSubtree(recipeSourceWithImageForTest(), "input-root", "filters", "frame-1", Rect{X: float64(i * 20), Width: 200, Height: 32})
			if err != nil {
				t.Fatalf("CloneSubtree[%d]: %v", i, err)
			}
			ids = append(ids, clone.RootLayerID)
		}
		primitiveID, err := builder.AddPrimitiveLayer("filters", Layer{ID: "empty-state", Name: "Empty", Type: "text", Visible: true, Text: map[string]any{"characters": "None"}})
		if err != nil {
			t.Fatalf("AddPrimitiveLayer: %v", err)
		}
		ids = append(ids, primitiveID)
		return mustMarshalDocumentBuilderTest(t, builder.Document()), ids
	}

	first, firstIDs := build()
	second, secondIDs := build()
	if !reflect.DeepEqual(first, second) {
		t.Fatalf("documents differ:\n%s\n%s", first, second)
	}
	if !reflect.DeepEqual(firstIDs, secondIDs) {
		t.Fatalf("IDs differ: %v != %v", firstIDs, secondIDs)
	}
	if firstIDs[0] == firstIDs[1] {
		t.Fatalf("repeated clone reused ID %q", firstIDs[0])
	}
	pattern := regexp.MustCompile(`^gen-[0-9a-f]{20}$`)
	for _, id := range firstIDs {
		if !pattern.MatchString(id) {
			t.Errorf("generated ID %q has wrong format", id)
		}
	}
}

func TestDocumentBuilderRejectsUnauthorizedAndInvalidCloneBindings(t *testing.T) {
	builder, err := NewDocumentBuilder(compilerTemplateForTest(), "binding-test")
	if err != nil {
		t.Fatalf("NewDocumentBuilder: %v", err)
	}
	clone, err := builder.CloneSubtree(recipeSourceWithImageForTest(), "input-root", "filters", "frame-1", Rect{Width: 200, Height: 32})
	if err != nil {
		t.Fatalf("CloneSubtree: %v", err)
	}
	before := mustMarshalDocumentBuilderTest(t, builder.Document())

	if err := builder.BindText(clone, "outside-clone", "nope"); err == nil {
		t.Fatal("BindText accepted an undeclared source target")
	}
	if err := builder.BindText(clone, "input-icon", "nope"); err == nil {
		t.Fatal("BindText accepted a non-text target")
	}
	tampered := CloneResult{RootLayerID: clone.RootLayerID, SourceToTarget: map[string]string{"input-label": "missing-target"}}
	if err := builder.BindText(tampered, "input-label", "nope"); err == nil {
		t.Fatal("BindText accepted a missing mapped target")
	}
	if err := builder.FitCloneLayer(clone, "outside-clone", Rect{Width: 10, Height: 10}); err == nil {
		t.Fatal("FitCloneLayer accepted an undeclared source target")
	}
	if err := builder.FitCloneLayer(tampered, "input-label", Rect{Width: 10, Height: 10}); err == nil {
		t.Fatal("FitCloneLayer accepted a missing mapped target")
	}
	if after := mustMarshalDocumentBuilderTest(t, builder.Document()); !reflect.DeepEqual(after, before) {
		t.Fatal("failed bindings partially mutated the document")
	}
}

func TestDocumentBuilderCloneFailureIsAtomic(t *testing.T) {
	builder, err := NewDocumentBuilder(compilerTemplateForTest(), "atomic-clone")
	if err != nil {
		t.Fatalf("NewDocumentBuilder: %v", err)
	}
	before := mustMarshalDocumentBuilderTest(t, builder.Document())

	brokenChild := recipeSourceWithImageForTest()
	root := brokenChild.Layers["input-root"]
	root.Children = append(root.Children, "missing-child")
	brokenChild.Layers["input-root"] = root
	if _, err := builder.CloneSubtree(brokenChild, "input-root", "filters", "frame-1", Rect{Width: 200, Height: 32}); err == nil {
		t.Fatal("CloneSubtree accepted a missing descendant")
	}
	if after := mustMarshalDocumentBuilderTest(t, builder.Document()); !reflect.DeepEqual(after, before) {
		t.Fatal("missing-child clone partially mutated the document")
	}

	missingAsset := recipeSourceWithImageForTest()
	delete(missingAsset.Assets, "shared-asset")
	if _, err := builder.CloneSubtree(missingAsset, "input-root", "filters", "frame-1", Rect{Width: 200, Height: 32}); err == nil {
		t.Fatal("CloneSubtree accepted a missing image asset")
	}
	if after := mustMarshalDocumentBuilderTest(t, builder.Document()); !reflect.DeepEqual(after, before) {
		t.Fatal("missing-asset clone partially mutated the document")
	}

	if _, err := builder.CloneSubtree(recipeSourceWithImageForTest(), "input-root", "missing-parent", "frame-1", Rect{Width: 200, Height: 32}); err == nil {
		t.Fatal("CloneSubtree accepted a missing target parent")
	}
	if after := mustMarshalDocumentBuilderTest(t, builder.Document()); !reflect.DeepEqual(after, before) {
		t.Fatal("missing-parent clone partially mutated the document")
	}

	valid, err := builder.CloneSubtree(recipeSourceWithImageForTest(), "input-root", "filters", "frame-1", Rect{Width: 200, Height: 32})
	if err != nil {
		t.Fatalf("valid CloneSubtree after failures: %v", err)
	}
	fresh, err := NewDocumentBuilder(compilerTemplateForTest(), "atomic-clone")
	if err != nil {
		t.Fatalf("fresh NewDocumentBuilder: %v", err)
	}
	want, err := fresh.CloneSubtree(recipeSourceWithImageForTest(), "input-root", "filters", "frame-1", Rect{Width: 200, Height: 32})
	if err != nil {
		t.Fatalf("fresh CloneSubtree: %v", err)
	}
	if valid.RootLayerID != want.RootLayerID {
		t.Fatalf("failed operations consumed ID sequence: got %q, want %q", valid.RootLayerID, want.RootLayerID)
	}
}

func TestDocumentBuilderHandlesNilMapsAndIsolatesReturnedDocuments(t *testing.T) {
	base := compilerTemplateForTest()
	base.Assets = nil
	base.Tokens = nil
	base.Layers["filters"] = Layer{ID: "filters", FrameID: "frame-1", ParentID: "page-root", Name: "Filters", Type: "frame", Visible: true}
	builder, err := NewDocumentBuilder(base, "nil-maps")
	if err != nil {
		t.Fatalf("NewDocumentBuilder: %v", err)
	}
	source := recipeSourceWithImageForTest()
	source.Assets = nil
	root := source.Layers["input-root"]
	root.Children = []string{"input-label"}
	root.Text, root.Style, root.Semantic, root.Source, root.Shape, root.Exportable = nil, nil, nil, nil, nil, nil
	source.Layers["input-root"] = root
	delete(source.Layers, "input-icon")
	clone, err := builder.CloneSubtree(source, "input-root", "filters", "frame-1", Rect{Width: 100, Height: 20})
	if err != nil {
		t.Fatalf("CloneSubtree with nil maps: %v", err)
	}

	first := builder.Document()
	first.File.Title = "mutated"
	first.Frames[0].Name = "mutated"
	first.Frames[0].Board["mode"] = "mutated"
	first.Layers["filters"] = Layer{}
	first.Layers[clone.RootLayerID].Children[0] = "mutated"
	first.Assets = map[string]Asset{"added": {ID: "added", URL: "https://example.test/added.png"}}
	first.Tokens = map[string]any{"mutated": true}

	second := builder.Document()
	if second.File.Title != "Compiler Template" || second.Frames[0].Name != "Main" || second.Frames[0].Board["mode"] != "design" {
		t.Fatalf("returned frame/file data leaked into builder: %+v %+v", second.File, second.Frames[0])
	}
	if second.Layers["filters"].ID != "filters" || second.Layers[clone.RootLayerID].Children[0] == "mutated" {
		t.Fatal("returned layer maps or slices leaked into builder")
	}
	if _, ok := second.Assets["added"]; ok {
		t.Fatal("returned asset map leaked into builder")
	}
	if second.Tokens != nil {
		t.Fatalf("returned token map leaked into builder: %#v", second.Tokens)
	}
}

func TestDocumentBuilderMutationMethodsValidateAndRemainAtomic(t *testing.T) {
	builder, err := NewDocumentBuilder(compilerTemplateForTest(), "mutation-test")
	if err != nil {
		t.Fatalf("NewDocumentBuilder: %v", err)
	}
	clone, err := builder.CloneSubtree(recipeSourceWithImageForTest(), "input-root", "filters", "frame-1", Rect{Width: 200, Height: 32})
	if err != nil {
		t.Fatalf("CloneSubtree: %v", err)
	}
	if err := builder.FitCloneLayer(clone, "input-label", Rect{X: 12, Y: 8, Width: 140, Height: 18}); err != nil {
		t.Fatalf("FitCloneLayer: %v", err)
	}
	label := builder.Document().Layers[clone.SourceToTarget["input-label"]]
	if got := (Rect{X: label.X, Y: label.Y, Width: label.Width, Height: label.Height}); got != (Rect{X: 12, Y: 8, Width: 140, Height: 18}) {
		t.Fatalf("fitted bounds = %+v", got)
	}
	if err := builder.SetBounds("filters", Rect{X: 5, Y: 6, Width: 700, Height: 120}); err != nil {
		t.Fatalf("SetBounds: %v", err)
	}
	primitive := Layer{ID: "primitive-text", Name: "Primitive", Type: "text", Visible: true, Width: 80, Height: 20, Text: map[string]any{"characters": "Ready"}}
	primitiveID, err := builder.AddPrimitiveLayer("filters", primitive)
	if err != nil {
		t.Fatalf("AddPrimitiveLayer: %v", err)
	}
	doc := builder.Document()
	if got := doc.Layers[primitiveID]; got.ParentID != "filters" || got.FrameID != "frame-1" || got.ID != primitiveID {
		t.Fatalf("primitive = %+v", got)
	}
	if primitive.ID != "primitive-text" || primitive.ParentID != "" || primitive.FrameID != "" {
		t.Fatalf("AddPrimitiveLayer mutated caller layer: %+v", primitive)
	}

	before := mustMarshalDocumentBuilderTest(t, doc)
	if err := builder.SetBounds("filters", Rect{Width: -1, Height: 10}); err == nil {
		t.Fatal("SetBounds accepted negative bounds")
	}
	if _, err := builder.AddPrimitiveLayer("filters", Layer{ID: "broken", Name: "Broken", Type: "image", Visible: true, Image: &ImageData{AssetID: "missing"}}); err == nil {
		t.Fatal("AddPrimitiveLayer accepted a missing asset")
	}
	if after := mustMarshalDocumentBuilderTest(t, builder.Document()); !reflect.DeepEqual(after, before) {
		t.Fatal("failed mutation partially changed the document")
	}

	if err := builder.ClearChildren("filters"); err != nil {
		t.Fatalf("ClearChildren: %v", err)
	}
	doc = builder.Document()
	if len(doc.Layers["filters"].Children) != 0 {
		t.Fatalf("filters children = %#v", doc.Layers["filters"].Children)
	}
	for _, removedID := range append([]string{primitiveID}, clone.SourceToTarget["input-root"], clone.SourceToTarget["input-label"], clone.SourceToTarget["input-icon"]) {
		if _, ok := doc.Layers[removedID]; ok {
			t.Errorf("ClearChildren retained descendant %q", removedID)
		}
	}
}

func compilerTemplateForTest() NativeJSON {
	return NativeJSON{
		Version: NativeJSONVersion,
		File:    FileMeta{Title: "Compiler Template", SourceType: "template"},
		Frames: []Frame{{
			ID: "frame-1", Name: "Main", RootLayerID: "page-root", Width: 1440, Height: 900,
			Board: map[string]any{"mode": "design"}, Source: map[string]any{"origin": "template"},
		}},
		Layers: map[string]Layer{
			"page-root": {ID: "page-root", FrameID: "frame-1", Children: []string{"filters"}, Name: "Page", Type: "frame", Visible: true, Width: 1440, Height: 900},
			"filters":   {ID: "filters", FrameID: "frame-1", ParentID: "page-root", Children: []string{}, Name: "Filters", Type: "frame", Visible: true, Width: 1200, Height: 120},
		},
		Assets: map[string]Asset{
			"shared-asset": {ID: "shared-asset", Kind: "image", URL: "https://example.test/template.png", Metadata: map[string]any{"owner": "template"}},
		},
		Tokens: map[string]any{"color": map[string]any{"primary": "#1677ff"}},
	}
}

func recipeSourceWithImageForTest() NativeJSON {
	references := func(nodeRef string) map[string]any {
		return map[string]any{"nodeRef": nodeRef, "assetRef": "shared-asset", "substring": "prefix-input-label-suffix"}
	}
	return NativeJSON{
		Version: NativeJSONVersion,
		File:    FileMeta{Title: "Input Recipe", SourceType: "component"},
		Frames:  []Frame{{ID: "recipe-frame", Name: "Recipe", RootLayerID: "input-root", Width: 200, Height: 32}},
		Layers: map[string]Layer{
			"input-root": {
				ID: "input-root", FrameID: "recipe-frame", Children: []string{"input-label", "input-icon"}, Name: "Input", Type: "frame", Visible: true, Width: 200, Height: 32,
				Text: references("input-label"), Style: map[string]any{"nodeRef": "input-icon", "assetRef": "shared-asset", "substring": "prefix-input-label-suffix", "nested": []any{"input-label", map[string]any{"assetRef": "shared-asset"}}},
				Semantic: references("input-label"), Source: references("input-icon"), Shape: references("input-label"), Exportable: []map[string]any{references("input-icon")},
			},
			"input-label": {ID: "input-label", FrameID: "recipe-frame", ParentID: "input-root", Name: "Label", Type: "text", Visible: true, X: 8, Y: 6, Width: 140, Height: 20, Text: map[string]any{"characters": "Name"}},
			"input-icon":  {ID: "input-icon", FrameID: "recipe-frame", ParentID: "input-root", Name: "Search", Type: "image", Visible: true, X: 176, Y: 8, Width: 16, Height: 16, Image: &ImageData{AssetID: "shared-asset"}},
		},
		Assets: map[string]Asset{
			"shared-asset": {ID: "shared-asset", Kind: "image", URL: "https://example.test/search.png", ContentType: "image/png", Width: 16, Height: 16, Metadata: map[string]any{"component": "search"}},
		},
	}
}

func mustMarshalDocumentBuilderTest(t *testing.T, value any) []byte {
	t.Helper()
	raw, err := json.Marshal(value)
	if err != nil {
		t.Fatalf("json.Marshal: %v", err)
	}
	return raw
}
