package designcore

import "testing"

func TestValidateDocumentAcceptsMinimalNativeJSON(t *testing.T) {
	doc := NativeJSON{
		Version: NativeJSONVersion,
		File:    FileMeta{Title: "Users", SourceType: "template"},
		Frames:  []Frame{{ID: "frame-1", Name: "Users", RootLayerID: "layer-1", Width: 1440, Height: 900}},
		Layers: map[string]Layer{
			"layer-1": {ID: "layer-1", FrameID: "frame-1", Name: "Page", Type: "frame", Visible: true, Width: 1440, Height: 900},
		},
		Assets: map[string]Asset{},
	}

	result := ValidateDocument(doc)
	if !result.Valid {
		t.Fatalf("expected document to be valid, got errors: %v", result.Errors)
	}
}

func TestValidateRequirementAcceptsSupportedPageType(t *testing.T) {
	result := ValidateRequirement(RequirementCore{
		Version:  NativeJSONVersion,
		Title:    "Users",
		PageType: "saas.filter-table-pagination",
		Entity:   KeyLabel{Key: "user", Label: "User"},
		Fields:   []KeyLabel{{Key: "name", Label: "Name"}},
	})
	if !result.Valid {
		t.Fatalf("expected requirement to be valid, got errors: %v", result.Errors)
	}
}

func TestValidateRequirementRejectsUnsupportedPageType(t *testing.T) {
	result := ValidateRequirement(RequirementCore{
		Version:  NativeJSONVersion,
		Title:    "Dashboard",
		PageType: "dashboard",
		Entity:   KeyLabel{Key: "dashboard", Label: "Dashboard"},
	})
	if result.Valid {
		t.Fatal("expected requirement to be invalid")
	}
}

func TestValidatePatchOperationsRejectsLayoutPatch(t *testing.T) {
	result := ValidatePatchOperations([]byte(`[{"op":"replace","path":"/layers/layer-1/width","value":100}]`))
	if result.Valid {
		t.Fatal("expected layout patch to be invalid")
	}
}

func TestValidatePatchOperationsAcceptsSemanticPatch(t *testing.T) {
	result := ValidatePatchOperations([]byte(`[{"op":"replace","path":"/layers/layer-1/semantic/role","value":"table"}]`))
	if !result.Valid {
		t.Fatalf("expected semantic patch to be valid, got errors: %v", result.Errors)
	}
}

func TestValidateDocumentRejectsBrokenReferences(t *testing.T) {
	doc := NativeJSON{
		Version: NativeJSONVersion,
		File:    FileMeta{Title: "Users", SourceType: "template"},
		Frames:  []Frame{{ID: "frame-1", Name: "Users", RootLayerID: "missing", Width: 1440, Height: 900}},
		Layers: map[string]Layer{
			"layer-1": {ID: "layer-1", FrameID: "frame-1", ParentID: "missing-parent", Name: "Page", Type: "frame", Visible: true, Width: 1440, Height: 900},
		},
		Assets: map[string]Asset{},
	}

	result := ValidateDocument(doc)
	if result.Valid {
		t.Fatal("expected document to be invalid")
	}
	if len(result.Errors) == 0 {
		t.Fatal("expected validation errors")
	}
}
