package designcore

import (
	"bytes"
	"encoding/json"
	"os"
	"path/filepath"
	"sort"
	"strconv"
	"strings"
	"testing"
)

const crmFixtureRequirementID = "REQ-CRM-CUSTOMER-LIST"

func TestCRMCustomerListCompilerAcceptance(t *testing.T) {
	input := loadCRMCompilerFixture(t)
	first := CompileListPage(input)
	second := CompileListPage(input)
	if first.Status != "generated" {
		t.Fatalf("status = %q, diagnostics = %+v", first.Status, first.Diagnostics)
	}
	firstJSON, err := json.Marshal(first.Document)
	if err != nil {
		t.Fatalf("marshal first: %v", err)
	}
	secondJSON, err := json.Marshal(second.Document)
	if err != nil {
		t.Fatalf("marshal second: %v", err)
	}
	if !bytes.Equal(firstJSON, secondJSON) {
		t.Fatal("compiler output is not deterministic")
	}
	text := allVisibleText(first.Document)
	for _, residue := range []string{"采购价格", "新增价格", "产品名称", "供应商"} {
		if strings.Contains(text, residue) {
			t.Fatalf("template residue %q remains", residue)
		}
	}
	for _, required := range []string{"客户姓名", "手机号", "客户状态", "创建时间", "客户编号", "负责人"} {
		if !strings.Contains(text, required) {
			t.Fatalf("required business text %q is missing", required)
		}
	}
	for _, column := range input.PageSpec.Table.Columns {
		path := "table.columns." + column.Key
		header := generatedRootAtSpecPath(t, first.Document, path)
		if !subtreeContainsText(first.Document, header.ID, column.Title) {
			t.Fatalf("%s does not contain header title %q", path, column.Title)
		}
	}
	for rowIndex, row := range input.PageSpec.Table.SampleRows {
		for _, column := range input.PageSpec.Table.Columns {
			path := "table.sampleRows." + strconv.Itoa(rowIndex) + "." + column.Key
			cell := generatedRootAtSpecPath(t, first.Document, path)
			if !subtreeContainsText(first.Document, cell.ID, row[column.Key]) {
				t.Fatalf("%s does not contain value %q", path, row[column.Key])
			}
			if column.Cell != "status-tag" {
				continue
			}
			wantVariant := column.StatusMap[row[column.Key]]
			wantRecipe := input.RecipeSet.Recipes[(RecipeKey{Kind: "status-tag", Variant: wantVariant, State: "default"}).String()]
			if cell.Semantic["recipeVariant"] != wantVariant ||
				cell.Semantic["recipeState"] != "default" ||
				cell.Semantic["recipeSourceRootLayerId"] != wantRecipe.Source.RootLayerID ||
				cell.Semantic["recipeSourceFingerprint"] != wantRecipe.Source.Fingerprint ||
				cell.Style["sourceVariant"] != wantVariant {
				t.Fatalf("%s status recipe = semantic:%+v style:%+v, want variant=%q recipe=%+v", path, cell.Semantic, cell.Style, wantVariant, wantRecipe)
			}
		}
		for _, action := range input.PageSpec.Table.RowActions {
			path := "table.sampleRows." + strconv.Itoa(rowIndex) + ".rowActions." + action.Key
			rowAction := generatedRootAtSpecPath(t, first.Document, path)
			if !subtreeContainsText(first.Document, rowAction.ID, action.Label) {
				t.Fatalf("%s does not contain action label %q", path, action.Label)
			}
		}
	}
	if countGeneratedRole(first.Document, "status-tag") != 3 {
		t.Fatal("every sample row must instantiate one status tag")
	}
	if countGeneratedRole(first.Document, "row-action") != 6 {
		t.Fatal("every sample row must instantiate view and edit actions")
	}
	if first.Manifest.FilterCount != 4 || first.Manifest.ColumnCount != 6 || first.Manifest.RowCount != 3 {
		t.Fatalf("manifest = %+v", first.Manifest)
	}
	if first.Quality.Metrics.TextOverflowCount != 0 || first.Quality.Metrics.UnexpectedOverlapCount != 0 || first.Quality.Metrics.OffFrameCount != 0 || first.Quality.Metrics.TemplateResidueCount != 0 {
		t.Fatalf("quality = %+v", first.Quality)
	}
	selectRoot := generatedRootAtSpecPath(t, first.Document, "filters.status")
	assetLayer, ok := firstAssetLayerInSubtree(first.Document, selectRoot.ID)
	if !ok || assetLayer.Image == nil {
		t.Fatal("select recipe did not preserve its nested image")
	}
	asset := first.Document.Assets[assetLayer.Image.AssetID]
	if asset.URL != "https://static.soyoung.com/multica/crm-select-chevron.png" {
		t.Fatalf("select asset URL = %q", asset.URL)
	}
	if assetLayer.X != selectRoot.X+180 || assetLayer.Y != selectRoot.Y+8 {
		t.Fatalf("select asset bounds = (%v,%v), root = (%v,%v)", assetLayer.X, assetLayer.Y, selectRoot.X, selectRoot.Y)
	}
	if _, ok := first.Document.ComponentBindings[selectRoot.ID]; !ok {
		t.Fatal("select recipe component binding was not cloned")
	}
}

func firstAssetLayerInSubtree(doc NativeJSON, rootID string) (Layer, bool) {
	for _, layerID := range qualityDescendants(doc.Layers, rootID) {
		layer := doc.Layers[layerID]
		if layer.Image != nil {
			return layer, true
		}
	}
	return Layer{}, false
}

func loadCRMCompilerFixture(t *testing.T) CompileInput {
	t.Helper()
	pageSpecRaw := readCRMFixture(t, "crm_customer_list_page_spec.json")
	blueprintRaw := readCRMFixture(t, "crm_list_blueprint.json")
	templateRaw := readCRMFixture(t, "crm_template_native.json")
	recipeSetRaw := readCRMFixture(t, "crm_recipe_set.json")
	recipeDocRaw := readCRMFixture(t, "crm_ui_spec_native.json")

	pageSpec, err := ParsePageSpec(pageSpecRaw)
	if err != nil {
		t.Fatalf("parse PageSpec: %v", err)
	}
	if diagnostics := ValidatePageSpec(pageSpec, []string{crmFixtureRequirementID}); diagnostics.HasErrors() {
		t.Fatalf("validate PageSpec: %+v", diagnostics)
	}
	templateDoc, err := ParseNativeJSON(templateRaw)
	if err != nil {
		t.Fatalf("parse template NativeJSON: %v", err)
	}
	if validation := ValidateDocument(templateDoc); !validation.Valid {
		t.Fatalf("validate template NativeJSON: %v", validation.Errors)
	}
	blueprint, err := ParseTemplateBlueprint(blueprintRaw)
	if err != nil {
		t.Fatalf("parse TemplateBlueprint: %v", err)
	}
	if diagnostics := ValidateTemplateBlueprintForPageSpec(ExtractTemplateStructure(templateDoc), blueprint, pageSpec); diagnostics.HasErrors() {
		t.Fatalf("validate TemplateBlueprint: %+v", diagnostics)
	}
	recipeDoc, err := ParseNativeJSON(recipeDocRaw)
	if err != nil {
		t.Fatalf("parse UI-spec NativeJSON: %v", err)
	}
	if validation := ValidateDocument(recipeDoc); !validation.Valid {
		t.Fatalf("validate UI-spec NativeJSON: %v", validation.Errors)
	}
	recipeSet, err := ParseComponentRecipeSet(recipeSetRaw)
	if err != nil {
		t.Fatalf("parse ComponentRecipeSet: %v", err)
	}
	if diagnostics := ValidateComponentRecipeSet(recipeDoc, recipeSet); diagnostics.HasErrors() {
		t.Fatalf("validate ComponentRecipeSet: %+v", diagnostics)
	}

	return CompileInput{
		PageSpec:               pageSpec,
		RequiredRequirementIDs: []string{crmFixtureRequirementID},
		Blueprint:              blueprint,
		RecipeSet:              recipeSet,
		TemplateDoc:            templateDoc,
		RecipeDoc:              recipeDoc,
		Provenance: CompileProvenance{
			WorkspaceID:       "crm-workspace",
			ProjectID:         "crm-project",
			IssueID:           "crm-customer-list-issue",
			AgentTaskID:       "crm-semantic-draft-task",
			PageSpecVersion:   PageSpecVersion,
			BlueprintRecordID: "crm-list-blueprint-record",
			RecipeSetRecordID: "crm-ui-recipe-set-record",
		},
	}
}

func readCRMFixture(t *testing.T, name string) []byte {
	t.Helper()
	raw, err := os.ReadFile(filepath.Join("testdata", name))
	if err != nil {
		t.Fatalf("read fixture %s: %v", name, err)
	}
	return raw
}

func allVisibleText(doc NativeJSON) string {
	layerIDs := make([]string, 0, len(doc.Layers))
	for layerID := range doc.Layers {
		layerIDs = append(layerIDs, layerID)
	}
	sort.Strings(layerIDs)
	values := make([]string, 0, len(layerIDs))
	for _, layerID := range layerIDs {
		layer := doc.Layers[layerID]
		if !isVisibleNativeLayer(doc.Layers, layerID) {
			continue
		}
		if text := structuralLayerText(layer); text != "" {
			values = append(values, text)
		}
	}
	return strings.Join(values, "\n")
}
