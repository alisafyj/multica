package designcore

import (
	"encoding/json"
	"reflect"
	"testing"
)

func TestBuildComponentRecipeSetAcceptsCompleteClassifications(t *testing.T) {
	source := recipeSourceDocumentForTest()
	set, diagnostics := BuildComponentRecipeSet("profile-1", "revision-1", ComponentRecipeSetVersion, source, completeRecipeClassificationsForTest(), completePrimitiveFallbacksForTest())
	if diagnostics.HasErrors() {
		t.Fatalf("unexpected diagnostics: %+v", diagnostics)
	}
	if set.Version != ComponentRecipeSetVersion || set.DesignSystemProfileID != "profile-1" || set.SourceRevisionID != "revision-1" {
		t.Fatalf("set identity = %+v", set)
	}
	if set.Tokens["color"].(map[string]any)["primary"] != "#1677ff" {
		t.Fatalf("tokens = %+v", set.Tokens)
	}
	source.Tokens["color"].(map[string]any)["primary"] = "changed"
	if set.Tokens["color"].(map[string]any)["primary"] != "#1677ff" {
		t.Fatal("recipe tokens must be a deep copy")
	}
	for _, kind := range requiredRecipeKindsForTest() {
		key := RecipeKey{Kind: kind, Variant: "default", State: "default"}.String()
		if _, ok := set.Recipes[key]; !ok {
			t.Fatalf("missing required recipe %q", key)
		}
		if set.Recipes[key].Source.Fingerprint == "" {
			t.Fatalf("recipe %q has no fingerprint", key)
		}
	}
}

func TestBuildComponentRecipeSetValidatesSourcesPropsAndRequiredKinds(t *testing.T) {
	source := recipeSourceDocumentForTest()
	tests := []struct {
		name   string
		mutate func(*[]ComponentRecipeClassification, map[string]PrimitiveRecipe)
		code   string
	}{
		{name: "unknown root", mutate: func(items *[]ComponentRecipeClassification, _ map[string]PrimitiveRecipe) {
			(*items)[0].RootLayerID = "invented"
		}, code: "unknown_source_layer"},
		{name: "hidden root", mutate: func(items *[]ComponentRecipeClassification, _ map[string]PrimitiveRecipe) {
			(*items)[0].RootLayerID = "hidden-recipe"
		}, code: "hidden_source_layer"},
		{name: "prop outside root", mutate: func(items *[]ComponentRecipeClassification, _ map[string]PrimitiveRecipe) {
			(*items)[0].Props["label"] = RecipeProp{TargetLayerID: "select-text", Type: "text"}
		}, code: "invalid_recipe_prop"},
		{name: "prop type mismatch", mutate: func(items *[]ComponentRecipeClassification, _ map[string]PrimitiveRecipe) {
			(*items)[0].Props["label"] = RecipeProp{TargetLayerID: "input-shape", Type: "text"}
		}, code: "invalid_recipe_prop"},
		{name: "invalid overflow", mutate: func(items *[]ComponentRecipeClassification, _ map[string]PrimitiveRecipe) {
			(*items)[0].Layout.TextOverflow = "clip"
		}, code: "invalid_recipe_layout"},
		{name: "missing required kind", mutate: func(items *[]ComponentRecipeClassification, _ map[string]PrimitiveRecipe) { *items = (*items)[1:] }, code: "missing_recipe_kind"},
		{name: "raw primitive style", mutate: func(_ *[]ComponentRecipeClassification, fallbacks map[string]PrimitiveRecipe) {
			item := fallbacks["input"]
			item.Style["color"] = "#fff"
			fallbacks["input"] = item
		}, code: "invalid_primitive_style"},
		{name: "missing primitive token", mutate: func(_ *[]ComponentRecipeClassification, fallbacks map[string]PrimitiveRecipe) {
			item := fallbacks["input"]
			item.Style["color"] = "$color.missing"
			fallbacks["input"] = item
		}, code: "unknown_token_reference"},
		{name: "incomplete primitive", mutate: func(_ *[]ComponentRecipeClassification, fallbacks map[string]PrimitiveRecipe) {
			item := fallbacks["input"]
			item.LayerType = ""
			fallbacks["input"] = item
		}, code: "invalid_primitive_recipe"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			classifications := completeRecipeClassificationsForTest()
			fallbacks := completePrimitiveFallbacksForTest()
			tt.mutate(&classifications, fallbacks)
			_, diagnostics := BuildComponentRecipeSet("profile-1", "revision-1", ComponentRecipeSetVersion, source, classifications, fallbacks)
			assertDiagnosticCode(t, diagnostics, tt.code)
		})
	}
}

func TestBuildComponentRecipeSetFingerprintsCompleteSubtreeAndAssets(t *testing.T) {
	source := recipeSourceDocumentForTest()
	set, diagnostics := BuildComponentRecipeSet("profile-1", "revision-1", ComponentRecipeSetVersion, source, completeRecipeClassificationsForTest(), completePrimitiveFallbacksForTest())
	if diagnostics.HasErrors() {
		t.Fatalf("build diagnostics: %+v", diagnostics)
	}
	key := RecipeKey{Kind: "status-tag", Variant: "default", State: "default"}.String()
	original := set.Recipes[key].Source.Fingerprint

	changedChild := recipeSourceDocumentForTest()
	child := changedChild.Layers["status-tag-text"]
	child.Style = map[string]any{"fontWeight": 700}
	changedChild.Layers["status-tag-text"] = child
	diagnostics = ValidateComponentRecipeSet(changedChild, set)
	assertDiagnosticCode(t, diagnostics, "recipe_fingerprint_drift")

	changedAsset := recipeSourceDocumentForTest()
	asset := changedAsset.Assets["status-icon-asset"]
	asset.URL = "https://cdn.example.com/status-v2.png"
	changedAsset.Assets["status-icon-asset"] = asset
	diagnostics = ValidateComponentRecipeSet(changedAsset, set)
	assertDiagnosticCode(t, diagnostics, "recipe_fingerprint_drift")

	reordered := recipeSourceDocumentForTest()
	root := reordered.Layers["status-tag"]
	root.Style = map[string]any{"z": 1, "a": 2}
	reordered.Layers["status-tag"] = root
	set2, diagnostics := BuildComponentRecipeSet("profile-1", "revision-1", ComponentRecipeSetVersion, reordered, completeRecipeClassificationsForTest(), completePrimitiveFallbacksForTest())
	if diagnostics.HasErrors() {
		t.Fatalf("reordered build diagnostics: %+v", diagnostics)
	}
	first := set2.Recipes[key].Source.Fingerprint
	root.Style = map[string]any{"a": 2, "z": 1}
	reordered.Layers["status-tag"] = root
	set3, diagnostics := BuildComponentRecipeSet("profile-1", "revision-1", ComponentRecipeSetVersion, reordered, completeRecipeClassificationsForTest(), completePrimitiveFallbacksForTest())
	if diagnostics.HasErrors() {
		t.Fatalf("second reordered build diagnostics: %+v", diagnostics)
	}
	if first != set3.Recipes[key].Source.Fingerprint {
		t.Fatal("canonical fingerprint changed with map insertion order")
	}
	if original == first {
		t.Fatal("complete subtree style change must alter fingerprint")
	}
}

func TestValidateComponentRecipeSetRejectsPersistedFingerprintDrift(t *testing.T) {
	source := recipeSourceDocumentForTest()
	set, diagnostics := BuildComponentRecipeSet("profile-1", "revision-1", ComponentRecipeSetVersion, source, completeRecipeClassificationsForTest(), completePrimitiveFallbacksForTest())
	if diagnostics.HasErrors() {
		t.Fatalf("build diagnostics: %+v", diagnostics)
	}
	key := RecipeKey{Kind: "input", Variant: "default", State: "default"}.String()
	recipe := set.Recipes[key]
	recipe.Source.Fingerprint = "persisted-drift"
	set.Recipes[key] = recipe

	diagnostics = ValidateComponentRecipeSet(source, set)
	assertDiagnosticCode(t, diagnostics, "recipe_fingerprint_drift")
}

func TestResolveRecipeUsesExactThenSameKindDefaultThenPrimitive(t *testing.T) {
	set := completeRecipeSetForTest(t)
	exactKey := RecipeKey{Kind: "input", Variant: "compact", State: "focused"}.String()
	exact := set.Recipes[RecipeKey{Kind: "input", Variant: "default", State: "default"}.String()]
	exact.Variant = "compact"
	exact.State = "focused"
	set.Recipes[exactKey] = exact

	resolved, diagnostics := ResolveRecipe(set, RecipeRequest{Kind: "input", Variant: "compact", State: "focused"})
	if diagnostics.HasErrors() || resolved.Recipe == nil || resolved.Fallback != "exact" {
		t.Fatalf("exact resolution = %+v, diagnostics = %+v", resolved, diagnostics)
	}
	delete(set.Recipes, exactKey)
	resolved, diagnostics = ResolveRecipe(set, RecipeRequest{Kind: "input", Variant: "compact", State: "focused"})
	if diagnostics.HasErrors() || resolved.Recipe == nil || resolved.Recipe.Kind != "input" || resolved.Fallback != "default" {
		t.Fatalf("default resolution = %+v, diagnostics = %+v", resolved, diagnostics)
	}
	delete(set.Recipes, RecipeKey{Kind: "input", Variant: "default", State: "default"}.String())
	resolved, diagnostics = ResolveRecipe(set, RecipeRequest{Kind: "input", Variant: "compact", State: "focused"})
	if diagnostics.HasErrors() || resolved.Primitive == nil || resolved.Primitive.Kind != "input" || resolved.Fallback != "primitive" {
		t.Fatalf("primitive resolution = %+v, diagnostics = %+v", resolved, diagnostics)
	}
}

func TestResolveRecipeDoesNotCrossComponentKinds(t *testing.T) {
	set := completeRecipeSetForTest(t)
	delete(set.Recipes, RecipeKey{Kind: "select", Variant: "default", State: "default"}.String())
	delete(set.PrimitiveFallbacks, "select")
	_, diagnostics := ResolveRecipe(set, RecipeRequest{Kind: "select", Variant: "default", State: "default"})
	assertDiagnosticCode(t, diagnostics, "missing_recipe")
}

func TestValidateComponentRecipeSetRejectsSameRevisionTokenMutation(t *testing.T) {
	source := recipeSourceDocumentForTest()
	set := completeRecipeSetForTest(t)
	set.Tokens = cloneJSONMap(source.Tokens)
	set.Tokens["color"] = map[string]any{"primary": "#000000"}

	assertDiagnosticCode(t, ValidateComponentRecipeSet(source, set), "recipe_token_drift")
}

func TestResolveRecipePrimitiveFallbackEmitsWarning(t *testing.T) {
	set := completeRecipeSetForTest(t)
	delete(set.Recipes, RecipeKey{Kind: "input", Variant: "default", State: "default"}.String())

	resolved, diagnostics := ResolveRecipe(set, RecipeRequest{Kind: "input", Variant: "compact", State: "focused"})
	if resolved.Primitive == nil || resolved.Primitive.Kind != "input" || resolved.Fallback != "primitive" {
		t.Fatalf("primitive resolution = %+v", resolved)
	}
	want := Diagnostics{{
		Code:     "primitive_fallback",
		Severity: DiagnosticWarning,
		Message:  `resolved component kind "input" with primitive fallback`,
		Paths:    []string{"primitiveFallbacks.input"},
	}}
	if !reflect.DeepEqual(diagnostics, want) {
		t.Fatalf("diagnostics = %+v, want %+v", diagnostics, want)
	}
	if diagnostics.HasErrors() {
		t.Fatalf("primitive fallback warning must not be an error: %+v", diagnostics)
	}
}

func TestResolveRecipeRejectsNonExecutablePrimitive(t *testing.T) {
	set := completeRecipeSetForTest(t)
	delete(set.Recipes, RecipeKey{Kind: "select", Variant: "default", State: "default"}.String())
	primitive := set.PrimitiveFallbacks["select"]
	primitive.Style["color"] = "#fff"
	set.PrimitiveFallbacks["select"] = primitive

	_, diagnostics := ResolveRecipe(set, RecipeRequest{Kind: "select", Variant: "missing", State: "missing"})
	assertDiagnosticCode(t, diagnostics, "missing_recipe")
}

func TestParseComponentRecipeSetIsStrict(t *testing.T) {
	set := completeRecipeSetForTest(t)
	raw, err := json.Marshal(set)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := ParseComponentRecipeSet(raw); err != nil {
		t.Fatalf("ParseComponentRecipeSet: %v", err)
	}
	unknown := append(raw[:len(raw)-1], []byte(`,"invented":true}`)...)
	if _, err := ParseComponentRecipeSet(unknown); err == nil {
		t.Fatal("expected unknown field to be rejected")
	}
	if _, err := ParseComponentRecipeSet(append(raw, []byte(` {}`)...)); err == nil {
		t.Fatal("expected trailing JSON to be rejected")
	}
}

func completeRecipeSetForTest(t *testing.T) ComponentRecipeSet {
	t.Helper()
	set, diagnostics := BuildComponentRecipeSet("profile-1", "revision-1", ComponentRecipeSetVersion, recipeSourceDocumentForTest(), completeRecipeClassificationsForTest(), completePrimitiveFallbacksForTest())
	if diagnostics.HasErrors() {
		t.Fatalf("build diagnostics: %+v", diagnostics)
	}
	return set
}

func completeRecipeClassificationsForTest() []ComponentRecipeClassification {
	items := make([]ComponentRecipeClassification, 0, len(requiredRecipeKindsForTest()))
	for _, kind := range requiredRecipeKindsForTest() {
		items = append(items, ComponentRecipeClassification{
			Kind:        kind,
			Variant:     "default",
			State:       "default",
			RootLayerID: kind,
			Props:       map[string]RecipeProp{"label": {TargetLayerID: kind + "-text", Type: "text"}},
			Layout:      RecipeLayout{WidthMode: "fixed", TextOverflow: "ellipsis", Height: 32, MinWidth: 80},
		})
	}
	return items
}

func completePrimitiveFallbacksForTest() map[string]PrimitiveRecipe {
	items := make(map[string]PrimitiveRecipe, len(requiredRecipeKindsForTest()))
	for _, kind := range requiredRecipeKindsForTest() {
		items[kind] = PrimitiveRecipe{
			Kind:      kind,
			LayerType: "frame",
			Props:     map[string]RecipeProp{"label": {TargetLayerID: "label", Type: "text"}},
			Style: map[string]any{
				"fill":    "$color.primary",
				"spacing": map[string]any{"horizontal": "$spacing.control"},
			},
			Layout: RecipeLayout{WidthMode: "fixed", TextOverflow: "ellipsis", Height: 32, MinWidth: 80},
		}
	}
	return items
}

func requiredRecipeKindsForTest() []string {
	return []string{"input", "select", "date-range", "primary-button", "secondary-button", "text-button", "table-header", "table-row", "status-tag", "pagination"}
}

func recipeSourceDocumentForTest() NativeJSON {
	layers := map[string]Layer{}
	rootChildren := make([]string, 0, len(requiredRecipeKindsForTest())+1)
	for index, kind := range requiredRecipeKindsForTest() {
		textID := kind + "-text"
		children := []string{textID}
		if kind == "status-tag" {
			children = append(children, "status-icon")
		}
		layers[kind] = Layer{ID: kind, FrameID: "recipe-frame", ParentID: "recipe-root", Children: children, Name: kind, Type: "frame", Visible: true, X: 0, Y: float64(index * 48), Width: 200, Height: 32}
		layers[textID] = Layer{ID: textID, FrameID: "recipe-frame", ParentID: kind, Name: textID, Type: "text", Visible: true, X: 8, Y: float64(index * 48), Width: 160, Height: 20, Text: map[string]any{"characters": kind}}
		rootChildren = append(rootChildren, kind)
	}
	layers["status-icon"] = Layer{ID: "status-icon", FrameID: "recipe-frame", ParentID: "status-tag", Name: "status-icon", Type: "image", Visible: true, X: 176, Y: 384, Width: 16, Height: 16, Image: &ImageData{AssetID: "status-icon-asset"}}
	layers["input-shape"] = Layer{ID: "input-shape", FrameID: "recipe-frame", ParentID: "input", Name: "input-shape", Type: "rectangle", Visible: true, Width: 200, Height: 32}
	input := layers["input"]
	input.Children = append(input.Children, "input-shape")
	layers["input"] = input
	layers["hidden-recipe"] = Layer{ID: "hidden-recipe", FrameID: "recipe-frame", ParentID: "recipe-root", Name: "hidden", Type: "frame", Visible: false, Width: 200, Height: 32}
	rootChildren = append(rootChildren, "hidden-recipe")
	layers["recipe-root"] = Layer{ID: "recipe-root", FrameID: "recipe-frame", Children: rootChildren, Name: "root", Type: "frame", Visible: true, Width: 1200, Height: 800}

	return NativeJSON{
		Version: NativeJSONVersion,
		Frames:  []Frame{{ID: "recipe-frame", Name: "Components", RootLayerID: "recipe-root", Width: 1200, Height: 800}},
		Layers:  layers,
		Assets: map[string]Asset{
			"status-icon-asset": {ID: "status-icon-asset", Kind: "image", URL: "https://cdn.example.com/status.png", ContentType: "image/png"},
		},
		Tokens: map[string]any{
			"color":   map[string]any{"primary": "#1677ff"},
			"spacing": map[string]any{"control": 12.0},
		},
	}
}
