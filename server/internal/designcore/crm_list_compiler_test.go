package designcore

import (
	"bytes"
	"encoding/json"
	"os"
	"path/filepath"
	"sort"
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
	if countGeneratedRole(first.Document, "status-tag") != 3 {
		t.Fatal("every sample row must instantiate one status tag")
	}
	for _, variant := range []string{"success", "warning", "disabled"} {
		if !containsGeneratedStatusVariant(first.Document, variant) {
			t.Fatalf("status tag variant %q is missing", variant)
		}
	}
	if first.Manifest.FilterCount != 4 || first.Manifest.ColumnCount != 6 || first.Manifest.RowCount != 3 {
		t.Fatalf("manifest = %+v", first.Manifest)
	}
	if first.Quality.Metrics.TextOverflowCount != 0 || first.Quality.Metrics.UnexpectedOverlapCount != 0 || first.Quality.Metrics.TemplateResidueCount != 0 {
		t.Fatalf("quality = %+v", first.Quality)
	}
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

func containsGeneratedStatusVariant(doc NativeJSON, want string) bool {
	for _, layer := range doc.Layers {
		if layer.Semantic["generatedBy"] == DesignCompilerVersion &&
			layer.Semantic["generationRole"] == "status-tag" &&
			layer.Semantic["recipeVariant"] == want {
			return true
		}
	}
	return false
}
