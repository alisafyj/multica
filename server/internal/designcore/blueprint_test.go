package designcore

import (
	"encoding/json"
	"testing"
)

func TestExtractTemplateStructureRetainsVisibleTypedFacts(t *testing.T) {
	doc := blueprintSourceDocumentForTest()
	structure := ExtractTemplateStructure(doc)

	if len(structure.Frames) != 2 {
		t.Fatalf("frames = %d, want 2", len(structure.Frames))
	}
	if _, ok := structure.Layers["hidden-layer"]; ok {
		t.Fatal("hidden layer must not be extracted")
	}
	if _, ok := structure.Layers["hidden-child"]; ok {
		t.Fatal("descendant of hidden layer must not be extracted")
	}
	if !containsString(structure.HiddenLayerIDs, "hidden-layer") || !containsString(structure.HiddenLayerIDs, "hidden-child") {
		t.Fatalf("hiddenLayerIds = %v", structure.HiddenLayerIDs)
	}
	layer := structure.Layers["page-title-text"]
	if layer.ParentID != "page-title-prototype" || layer.Text != "Source title" {
		t.Fatalf("page title text = %+v", layer)
	}
	if layer.Bounds != (Rect{X: 24, Y: 96, Width: 320, Height: 32}) {
		t.Fatalf("bounds = %+v", layer.Bounds)
	}
	if structure.Layers["filters"].ComponentKey != "FilterForm" {
		t.Fatalf("componentKey = %q", structure.Layers["filters"].ComponentKey)
	}
	if containsString(structure.Layers["frame-root"].Children, "hidden-layer") {
		t.Fatal("visible hierarchy must exclude hidden children")
	}
}

func TestExtractTemplateStructureDoesNotClassifySemanticNames(t *testing.T) {
	doc := blueprintSourceDocumentForTest()
	layer := doc.Layers["filters"]
	layer.Name = "page title table pagination navigation"
	doc.Layers["filters"] = layer

	structure := ExtractTemplateStructure(doc)
	if structure.Layers["filters"].Name != layer.Name {
		t.Fatal("source name must be retained as data")
	}
	if structure.Layers["filters"].ID != "filters" || len(structure.Layers) != 20 {
		t.Fatalf("semantic-looking names must not alter extraction: %+v", structure.Layers["filters"])
	}
}

func TestBuildTemplateBlueprintAcceptsValidatedListClassification(t *testing.T) {
	structure := ExtractTemplateStructure(blueprintSourceDocumentForTest())
	classification := completeBlueprintClassificationForTest()

	blueprint, diagnostics := BuildTemplateBlueprint(structure, classification, BlueprintSourceRefs{
		DesignFileID:       "file-1",
		DesignRevisionID:   "revision-1",
		TemplateRevisionID: "template-revision-1",
	})
	if diagnostics.HasErrors() {
		t.Fatalf("unexpected diagnostics: %+v", diagnostics)
	}
	if blueprint.Version != TemplateBlueprintVersion || blueprint.FrameID != "frame-1" || blueprint.PageType != "list" {
		t.Fatalf("blueprint identity = %+v", blueprint)
	}
	if blueprint.Regions["filters"].RootLayerID != "filters" {
		t.Fatalf("filters = %+v", blueprint.Regions["filters"])
	}
	if blueprint.Prototypes["pageTitle"].Bindings["label"] != "page-title-text" {
		t.Fatalf("page title prototype = %+v", blueprint.Prototypes["pageTitle"])
	}
}

func TestBuildTemplateBlueprintReportsReferenceDiagnostics(t *testing.T) {
	structure := ExtractTemplateStructure(blueprintSourceDocumentForTest())
	tests := []struct {
		name   string
		mutate func(*BlueprintClassification)
		code   string
	}{
		{name: "unknown frame", mutate: func(c *BlueprintClassification) { c.FrameID = "missing-frame" }, code: "unknown_frame"},
		{name: "unsupported page", mutate: func(c *BlueprintClassification) { c.PageType = "detail" }, code: "unsupported_page_type"},
		{name: "missing region", mutate: func(c *BlueprintClassification) { delete(c.Regions, "filters") }, code: "missing_region"},
		{name: "missing prototype", mutate: func(c *BlueprintClassification) { delete(c.Prototypes, "tableRow") }, code: "missing_prototype"},
		{name: "unknown layer", mutate: func(c *BlueprintClassification) { c.Regions["filters"] = RegionClassification{RootLayerID: "invented"} }, code: "unknown_source_layer"},
		{name: "hidden layer", mutate: func(c *BlueprintClassification) {
			c.Regions["filters"] = RegionClassification{RootLayerID: "hidden-layer"}
		}, code: "hidden_source_layer"},
		{name: "cross frame", mutate: func(c *BlueprintClassification) {
			c.Regions["filters"] = RegionClassification{RootLayerID: "other-root"}
		}, code: "cross_frame_reference"},
		{name: "binding outside root", mutate: func(c *BlueprintClassification) {
			c.Prototypes["pageTitle"] = PrototypeClassification{RootLayerID: "page-title-prototype", Bindings: map[string]string{"label": "breadcrumb-text"}}
		}, code: "invalid_binding"},
		{name: "binding target not text", mutate: func(c *BlueprintClassification) {
			c.Prototypes["pageTitle"] = PrototypeClassification{RootLayerID: "page-title-prototype", Bindings: map[string]string{"label": "page-title-shape"}}
		}, code: "invalid_binding"},
		{name: "business region outside content", mutate: func(c *BlueprintClassification) {
			c.Regions["filters"] = RegionClassification{RootLayerID: "sidebar", ReplaceChildren: true}
		}, code: "invalid_region_relationship"},
		{name: "nested replaceable peers", mutate: func(c *BlueprintClassification) {
			c.Regions["filters"] = RegionClassification{RootLayerID: "table", ReplaceChildren: true}
		}, code: "invalid_region_relationship"},
		{name: "invalid constraint", mutate: func(c *BlueprintClassification) { c.Constraints.FilterColumns = 7 }, code: "invalid_constraint"},
		{name: "unsafe shell allowlist", mutate: func(c *BlueprintClassification) { c.ShellAllowlistLayerIDs = []string{"outside-shell"} }, code: "unsafe_shell_allowlist"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			classification := completeBlueprintClassificationForTest()
			tt.mutate(&classification)
			_, diagnostics := BuildTemplateBlueprint(structure, classification, BlueprintSourceRefs{})
			assertDiagnosticCode(t, diagnostics, tt.code)
		})
	}
}

func TestBuildTemplateBlueprintRequiresReplaceableBusinessRegions(t *testing.T) {
	structure := ExtractTemplateStructure(blueprintSourceDocumentForTest())
	for _, regionKey := range []string{"breadcrumb", "pageTitle", "filters", "pageActions", "table", "pagination"} {
		t.Run(regionKey, func(t *testing.T) {
			classification := completeBlueprintClassificationForTest()
			region := classification.Regions[regionKey]
			region.ReplaceChildren = false
			classification.Regions[regionKey] = region

			_, diagnostics := BuildTemplateBlueprint(structure, classification, BlueprintSourceRefs{})
			if got := countDiagnosticCode(diagnostics, "invalid_region_relationship"); got != 1 {
				t.Fatalf("invalid_region_relationship count = %d, want 1; diagnostics = %+v", got, diagnostics)
			}
		})
	}
}

func TestBuildTemplateBlueprintCannotBypassUnsafeRelationships(t *testing.T) {
	structure := ExtractTemplateStructure(blueprintSourceDocumentForTest())
	tests := []struct {
		name        string
		rootLayerID string
	}{
		{name: "outside content", rootLayerID: "outside-shell"},
		{name: "nested peer", rootLayerID: "table"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			classification := completeBlueprintClassificationForTest()
			classification.Regions["filters"] = RegionClassification{RootLayerID: tt.rootLayerID, ReplaceChildren: false}

			_, diagnostics := BuildTemplateBlueprint(structure, classification, BlueprintSourceRefs{})
			if got := countDiagnosticCode(diagnostics, "invalid_region_relationship"); got != 2 {
				t.Fatalf("invalid_region_relationship count = %d, want 2; diagnostics = %+v", got, diagnostics)
			}
		})
	}
}

func TestValidateTemplateBlueprintRechecksPersistedValues(t *testing.T) {
	structure := ExtractTemplateStructure(blueprintSourceDocumentForTest())
	blueprint, diagnostics := BuildTemplateBlueprint(structure, completeBlueprintClassificationForTest(), BlueprintSourceRefs{})
	if diagnostics.HasErrors() {
		t.Fatalf("build diagnostics: %+v", diagnostics)
	}

	blueprint.Regions["filters"] = BlueprintRegion{RootLayerID: "hidden-layer", ReplaceChildren: true}
	blueprint.Constraints.ContentWidth = 0
	diagnostics = ValidateTemplateBlueprint(structure, blueprint)
	assertDiagnosticCode(t, diagnostics, "hidden_source_layer")
	assertDiagnosticCode(t, diagnostics, "invalid_constraint")
}

func TestValidateTemplateBlueprintReportsMissingNavigationWhenRequired(t *testing.T) {
	structure := ExtractTemplateStructure(blueprintSourceDocumentForTest())
	blueprint, diagnostics := BuildTemplateBlueprint(structure, completeBlueprintClassificationForTest(), BlueprintSourceRefs{})
	if diagnostics.HasErrors() {
		t.Fatalf("build diagnostics: %+v", diagnostics)
	}

	delete(blueprint.Regions, "navigation")
	diagnostics = ValidateTemplateBlueprintForPageSpec(structure, blueprint, PageSpec{Page: PageIdentity{ActiveNavigation: "Customers"}})
	assertDiagnosticCode(t, diagnostics, "missing_navigation_region")
}

func TestParseTemplateBlueprintIsStrict(t *testing.T) {
	structure := ExtractTemplateStructure(blueprintSourceDocumentForTest())
	blueprint, diagnostics := BuildTemplateBlueprint(structure, completeBlueprintClassificationForTest(), BlueprintSourceRefs{})
	if diagnostics.HasErrors() {
		t.Fatalf("build diagnostics: %+v", diagnostics)
	}
	raw, err := json.Marshal(blueprint)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := ParseTemplateBlueprint(raw); err != nil {
		t.Fatalf("ParseTemplateBlueprint: %v", err)
	}
	unknown := append(raw[:len(raw)-1], []byte(`,"invented":true}`)...)
	if _, err := ParseTemplateBlueprint(unknown); err == nil {
		t.Fatal("expected unknown field to be rejected")
	}
	if _, err := ParseTemplateBlueprint(append(raw, []byte(` {}`)...)); err == nil {
		t.Fatal("expected trailing JSON to be rejected")
	}
}

func completeBlueprintClassificationForTest() BlueprintClassification {
	return BlueprintClassification{
		FrameID:  "frame-1",
		PageType: "list",
		Regions: map[string]RegionClassification{
			"shell":       {RootLayerID: "shell"},
			"content":     {RootLayerID: "content"},
			"navigation":  {RootLayerID: "sidebar"},
			"breadcrumb":  {RootLayerID: "breadcrumb", ReplaceChildren: true},
			"pageTitle":   {RootLayerID: "page-title", ReplaceChildren: true},
			"filters":     {RootLayerID: "filters", ReplaceChildren: true},
			"pageActions": {RootLayerID: "page-actions", ReplaceChildren: true},
			"table":       {RootLayerID: "table", ReplaceChildren: true},
			"pagination":  {RootLayerID: "pagination", ReplaceChildren: true},
		},
		Prototypes: map[string]PrototypeClassification{
			"pageTitle":       {RootLayerID: "page-title-prototype", Bindings: map[string]string{"label": "page-title-text"}},
			"breadcrumbItem":  {RootLayerID: "breadcrumb-item", Bindings: map[string]string{"label": "breadcrumb-text"}},
			"tableHeaderCell": {RootLayerID: "table-header-cell"},
			"tableRow":        {RootLayerID: "table-row"},
		},
		Constraints: BlueprintConstraints{
			ContentWidth:      1120,
			FilterColumns:     3,
			FilterRowHeight:   68,
			TableHeaderHeight: 44,
			TableRowHeight:    52,
			HorizontalGap:     16,
			VerticalGap:       16,
		},
		ShellAllowlistLayerIDs: []string{"sidebar", "topbar"},
	}
}

func blueprintSourceDocumentForTest() NativeJSON {
	layers := map[string]Layer{}
	add := func(id, frameID, parentID, layerType string, children []string, x, y, width, height float64) {
		layers[id] = Layer{ID: id, FrameID: frameID, ParentID: parentID, Children: children, Name: id, Type: layerType, Visible: true, X: x, Y: y, Width: width, Height: height}
	}
	add("frame-root", "frame-1", "", "frame", []string{"shell", "outside-shell", "hidden-layer"}, 0, 0, 1440, 900)
	add("shell", "frame-1", "frame-root", "frame", []string{"sidebar", "topbar", "content"}, 0, 0, 1440, 900)
	add("outside-shell", "frame-1", "frame-root", "frame", nil, 0, 0, 20, 20)
	add("sidebar", "frame-1", "shell", "frame", nil, 0, 0, 240, 900)
	add("topbar", "frame-1", "shell", "frame", nil, 240, 0, 1200, 64)
	add("content", "frame-1", "shell", "frame", []string{"breadcrumb", "page-title", "filters", "page-actions", "table", "pagination", "page-title-prototype", "breadcrumb-item", "table-header-cell", "table-row"}, 240, 64, 1200, 836)
	add("breadcrumb", "frame-1", "content", "frame", nil, 24, 72, 500, 20)
	add("page-title", "frame-1", "content", "frame", nil, 24, 96, 500, 40)
	add("filters", "frame-1", "content", "frame", nil, 24, 152, 1120, 68)
	add("page-actions", "frame-1", "content", "frame", nil, 24, 236, 1120, 40)
	add("table", "frame-1", "content", "frame", nil, 24, 292, 1120, 460)
	add("pagination", "frame-1", "content", "frame", nil, 24, 768, 1120, 40)
	add("page-title-prototype", "frame-1", "content", "frame", []string{"page-title-text", "page-title-shape"}, 24, 96, 320, 32)
	add("page-title-text", "frame-1", "page-title-prototype", "text", nil, 24, 96, 320, 32)
	title := layers["page-title-text"]
	title.Text = map[string]any{"characters": "Source title"}
	layers["page-title-text"] = title
	add("page-title-shape", "frame-1", "page-title-prototype", "rectangle", nil, 24, 96, 320, 32)
	add("breadcrumb-item", "frame-1", "content", "frame", []string{"breadcrumb-text"}, 24, 72, 120, 20)
	add("breadcrumb-text", "frame-1", "breadcrumb-item", "text", nil, 24, 72, 120, 20)
	add("table-header-cell", "frame-1", "content", "frame", nil, 24, 292, 180, 44)
	add("table-row", "frame-1", "content", "frame", nil, 24, 336, 1120, 52)
	add("hidden-layer", "frame-1", "frame-root", "frame", []string{"hidden-child"}, 0, 0, 100, 100)
	hidden := layers["hidden-layer"]
	hidden.Visible = false
	layers["hidden-layer"] = hidden
	add("hidden-child", "frame-1", "hidden-layer", "text", nil, 0, 0, 50, 20)
	add("other-root", "frame-2", "", "frame", nil, 0, 0, 800, 600)

	return NativeJSON{
		Version: NativeJSONVersion,
		Frames: []Frame{
			{ID: "frame-1", Name: "Desktop", RootLayerID: "frame-root", Width: 1440, Height: 900},
			{ID: "frame-2", Name: "Other", RootLayerID: "other-root", Width: 800, Height: 600},
		},
		Layers: layers,
		ComponentBindings: map[string]ComponentBinding{
			"filters": {ComponentKey: "FilterForm"},
		},
	}
}

func containsString(values []string, want string) bool {
	for _, value := range values {
		if value == want {
			return true
		}
	}
	return false
}

func countDiagnosticCode(diagnostics Diagnostics, code string) int {
	count := 0
	for _, diagnostic := range diagnostics {
		if diagnostic.Code == code {
			count++
		}
	}
	return count
}
