# Semantic List Compiler Foundation Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Build the versioned semantic assets, deterministic list-page compiler, and blocking quality gate required to turn a validated `PageSpec` into reviewable Native Design JSON.

**Architecture:** `PageSpec` owns business content, `TemplateBlueprint` owns page composition, and `ComponentRecipeSet` owns executable project appearance. A pure Go `designcore` compiler clones validated source subtrees, computes layout, serializes Native Design JSON, and runs a quality gate before any later workflow may persist a reviewable draft. This plan builds and persists the compiler inputs and proves the CRM customer-list fixture; Agent task dispatch, draft review APIs, and frontend review UI are a separate second plan.

**Tech Stack:** Go 1.26, PostgreSQL migrations, sqlc, existing `server/internal/designcore` Native Design JSON model, standard-library JSON/Unicode/geometry utilities, Go unit and handler integration tests.

## Global Constraints

- First-release page type is only `list`; modal, drawer, popover, detail, form, dashboard, and C-end composition must fail with a typed unsupported-page diagnostic.
- The Agent-facing contract is `PageSpec`; it contains no layer IDs, coordinates, or JSON patch operations.
- New semantic generation never falls back to `slot_values` or arbitrary text-layer patching. Existing patch-based drafts remain readable and unchanged in this plan.
- UI specification Recipes own component appearance; Blueprint owns region composition; PageSpec owns business content; compiler owns geometry and node creation.
- Missing required Blueprint regions, missing structurally safe Recipes, overlap, overflow, off-frame output, inconsistent counts, and template business-copy residue are blocking failures.
- Recipe resolution may use the default variant of the same component kind. Primitive fallback is allowed only when the Recipe set explicitly allows it for that kind and must emit a warning.
- Source component subtrees and asset references are cloned intact. Exported images are preserved rather than redrawn.
- Generated IDs are deterministic for identical inputs. Compiler output must be byte-stable after JSON marshaling through the typed model.
- New database tables are singular `snake_case`, migrations are bidirectional, Go comments are English, and no new frontend code is introduced in this foundation plan.
- Before modifying an existing function, method, or type, run GitNexus `impact({target: "<symbol>", direction: "upstream"})`. Stop and warn the user before a HIGH or CRITICAL result.
- Before every commit, run GitNexus `detect_changes({scope: "working"})`; before the final foundation commit, also run `detect_changes({scope: "compare", base_ref: "main"})`.
- Leave unrelated untracked `.playwright-mcp/` and `.superpowers/` paths untouched and unstaged.

---

## Files And Responsibilities

- Create `server/internal/designcore/diagnostic.go`: shared diagnostics and compilation statuses.
- Create `server/internal/designcore/page_spec.go`: strict list-page semantic contract, parser, and validator.
- Create `server/internal/designcore/blueprint.go`: objective structural extraction and validated Blueprint contracts.
- Create `server/internal/designcore/recipe.go`: executable Component Recipe validation and fallback resolution.
- Create migration `128` and sqlc queries: immutable Blueprint and Recipe-set analysis versions.
- Create `server/internal/service/design_generation_assets.go`: typed persistence and loading boundary.
- Create `server/internal/designcore/document_builder.go`: deterministic subtree cloning and reference rewriting.
- Create `server/internal/designcore/text_measure.go` and `table_layout.go`: safe column sizing and horizontal-scroll strategy.
- Create `server/internal/designcore/compiler*.go`: list-page region and table materialization.
- Create `server/internal/designcore/quality_gate.go`: blocking structural and visual checks.
- Create `server/internal/designcore/testdata/crm_*`: CRM customer-list acceptance fixtures.
- Add colocated Go tests for every new file and compiler pass.

## Out Of Scope For This Plan

- Do not change `server/internal/daemon/prompt.go`, `service.UIDraftCreateContext`, or Agent completion parsing yet.
- Do not change `design_draft` status values or frontend `DesignDraftStatus` yet.
- Do not expose review, revision, reject, or approve endpoints yet.
- Do not route production Issue actions through the compiler until the second implementation plan.
- Do not delete old slot/patch code in this plan; the second plan removes it from new Agent task creation after the semantic path is integrated.

---

### Task 1: PageSpec And Diagnostic Contracts

**Files:**
- Create: `server/internal/designcore/diagnostic.go`
- Create: `server/internal/designcore/page_spec.go`
- Create: `server/internal/designcore/page_spec_test.go`

**Interfaces:**
- Consumes: JSON returned by a future UI Agent task and caller-supplied required PRD item IDs.
- Produces: `ParsePageSpec(raw []byte) (PageSpec, error)`, `ValidatePageSpec(spec PageSpec, requiredRequirementIDs []string) Diagnostics`, and `Diagnostics.HasErrors() bool`.

- [ ] **Step 1: Write failing parser and semantic-validation tests**

Create table-driven tests covering a complete CRM list spec, duplicate filter/column/action keys, unsupported page/control/cell values, unmapped statuses, sample rows missing visible columns, duplicate or missing requirement coverage, and forbidden layer/geometry fields. The acceptance shape is:

```go
func TestParseAndValidatePageSpecAcceptsCompleteListPage(t *testing.T) {
	raw := []byte(`{
	  "version":"1.0",
	  "page":{"type":"list","module":"客户管理","title":"客户档案","breadcrumb":["客户管理","客户档案"],"activeNavigation":"客户信息","density":"standard"},
	  "filters":[
	    {"key":"keyword","label":"客户姓名/手机号","control":"input","placeholder":"请输入客户姓名或手机号","width":"medium"},
	    {"key":"status","label":"客户状态","control":"select","placeholder":"请选择客户状态","width":"narrow"},
	    {"key":"createdAt","label":"创建时间","control":"date-range","placeholder":"请选择创建时间","width":"wide"}
	  ],
	  "pageActions":[{"key":"create","label":"新增客户","variant":"primary"}],
	  "table":{"columns":[
	    {"key":"customerName","title":"客户姓名","cell":"text","width":"medium"},
	    {"key":"phone","title":"手机号","cell":"text","width":"medium"},
	    {"key":"status","title":"客户状态","cell":"status-tag","width":"narrow","statusMap":{"正常":"success","待跟进":"warning"}},
	    {"key":"createdAt","title":"创建时间","cell":"date","width":"wide"}
	  ],"sampleRows":[{"customerName":"示例客户A","phone":"13800000001","status":"正常","createdAt":"2026-07-22 10:00"}],"rowActions":[{"key":"view","label":"查看","variant":"text"}]},
	  "pagination":{"enabled":true,"pageSize":20,"sampleTotal":126},
	  "assumptions":[],"warnings":[],
	  "requirementCoverage":[
	    {"requirementId":"filter-keyword","specPaths":["filters.keyword"]},
	    {"requirementId":"filter-status","specPaths":["filters.status"]},
	    {"requirementId":"filter-created-at","specPaths":["filters.createdAt"]}
	  ]
	}`)
	spec, err := ParsePageSpec(raw)
	if err != nil { t.Fatalf("ParsePageSpec: %v", err) }
	diagnostics := ValidatePageSpec(spec, []string{"filter-keyword", "filter-status", "filter-created-at"})
	if diagnostics.HasErrors() { t.Fatalf("unexpected diagnostics: %+v", diagnostics) }
}

func TestParsePageSpecRejectsLayerAndGeometryFields(t *testing.T) {
	_, err := ParsePageSpec([]byte(`{"version":"1.0","page":{"type":"list","module":"CRM","title":"客户","density":"standard","x":20},"filters":[],"pageActions":[],"table":{"columns":[],"sampleRows":[],"rowActions":[]},"pagination":{"enabled":false},"layerId":"figma-1"}`))
	if err == nil { t.Fatal("expected unknown semantic fields to be rejected") }
}
```

Assert diagnostic codes rather than message substrings. Required codes are `unsupported_page_type`, `duplicate_key`, `unsupported_control`, `unsupported_cell`, `missing_status_mapping`, `incomplete_sample_row`, `missing_requirement_coverage`, and `invalid_spec_path`.

- [ ] **Step 2: Run the tests and verify the contract is absent**

Run:

```bash
cd server && go test ./internal/designcore -run 'Test(ParseAndValidatePageSpec|ParsePageSpec|ValidatePageSpec)' -count=1
```

Expected: FAIL because `PageSpec`, `ParsePageSpec`, and diagnostic types are undefined.

- [ ] **Step 3: Implement strict PageSpec and diagnostic types**

Create these exact public contracts:

```go
type DiagnosticSeverity string
const (
	DiagnosticWarning DiagnosticSeverity = "warning"
	DiagnosticError DiagnosticSeverity = "error"
)
type Diagnostic struct {
	Code string `json:"code"`
	Severity DiagnosticSeverity `json:"severity"`
	Message string `json:"message"`
	Paths []string `json:"paths,omitempty"`
	LayerIDs []string `json:"layerIds,omitempty"`
}
type Diagnostics []Diagnostic
func (d Diagnostics) HasErrors() bool

const PageSpecVersion = "1.0"
type PageSpec struct {
	Version string `json:"version"`
	Page PageIdentity `json:"page"`
	Filters []FilterSpec `json:"filters"`
	PageActions []ActionSpec `json:"pageActions"`
	Table TableSpec `json:"table"`
	Pagination PaginationSpec `json:"pagination"`
	Assumptions []string `json:"assumptions"`
	Warnings []string `json:"warnings"`
	RequirementCoverage []RequirementCoverage `json:"requirementCoverage"`
}
type PageIdentity struct { Type, Module, Title string; Breadcrumb []string; ActiveNavigation, Density string }
type FilterSpec struct { Key, Label, Control, Placeholder, Width string }
type ActionSpec struct { Key, Label, Variant string }
type TableSpec struct { Columns []TableColumnSpec; SampleRows []map[string]string; RowActions []ActionSpec }
type TableColumnSpec struct { Key, Title, Cell, Width, Align string; StatusMap map[string]string }
type PaginationSpec struct { Enabled bool; PageSize, SampleTotal int }
type RequirementCoverage struct { RequirementID string; SpecPaths []string }
```

Add JSON tags matching the example. `ParsePageSpec` uses `json.Decoder.DisallowUnknownFields()`, rejects trailing JSON, and returns before semantic validation for unknown fields. `ValidatePageSpec` allows controls `input`, `select`, `date-range`; action variants `primary`, `secondary`, `text`; cells `text`, `number`, `date`, `status-tag`; widths `narrow`, `medium`, `wide`, `flexible`; alignment `left`, `center`, `right`, or empty; density `standard`, `compact`; and status variants `success`, `warning`, `danger`, `disabled`, `info`.

Coverage paths may only be `filters.<key>`, `pageActions.<key>`, `table.columns.<key>`, `table.rowActions.<key>`, `pagination`, and `page.<field>`. They never use JSON Pointer or layer paths.

- [ ] **Step 4: Run focused tests and format**

Run:

```bash
cd server
gofmt -w internal/designcore/diagnostic.go internal/designcore/page_spec.go internal/designcore/page_spec_test.go
go test ./internal/designcore -run 'Test(ParseAndValidatePageSpec|ParsePageSpec|ValidatePageSpec)' -count=1
```

Expected: PASS.

- [ ] **Step 5: Check scope and commit the semantic contract**

Run GitNexus `detect_changes({scope: "working"})`, then:

```bash
git add server/internal/designcore/diagnostic.go server/internal/designcore/page_spec.go server/internal/designcore/page_spec_test.go
git commit -m "feat(design): add semantic list page spec"
```

---

### Task 2: Blueprint And Component Recipe Analysis Contracts

**Files:**
- Create: `server/internal/designcore/blueprint.go`
- Create: `server/internal/designcore/blueprint_test.go`
- Create: `server/internal/designcore/recipe.go`
- Create: `server/internal/designcore/recipe_test.go`

**Interfaces:**
- Consumes: typed `NativeJSON` and Agent-produced semantic classifications that only reference extracted IDs.
- Produces: `ExtractTemplateStructure`, `BuildTemplateBlueprint`, `BuildComponentRecipeSet`, and `ResolveRecipe`. These functions perform no name-keyword classification.

- [ ] **Step 1: Write failing extraction, Blueprint, and Recipe tests**

Prove hidden layers are excluded; visible hierarchy/bounds/component bindings are retained; classifications cannot invent IDs; prop targets must be descendants and text layers; exact/default fallback stays within the same component kind; primitive fallback requires an executable typed definition; and every required first-release component kind exists.

```go
func TestBuildTemplateBlueprintAcceptsValidatedListClassification(t *testing.T) {
	structure := ExtractTemplateStructure(blueprintSourceDocumentForTest())
	classification := BlueprintClassification{
		FrameID: "frame-1",
		PageType: "list",
		Regions: map[string]RegionClassification{
			"shell": {RootLayerID: "shell"},
			"content": {RootLayerID: "content"},
			"breadcrumb": {RootLayerID: "breadcrumb", ReplaceChildren: true},
			"pageTitle": {RootLayerID: "page-title", ReplaceChildren: true},
			"filters": {RootLayerID: "filters", ReplaceChildren: true},
			"pageActions": {RootLayerID: "page-actions", ReplaceChildren: true},
			"table": {RootLayerID: "table", ReplaceChildren: true},
			"pagination": {RootLayerID: "pagination", ReplaceChildren: true},
		},
		Prototypes: map[string]PrototypeClassification{
			"pageTitle": {RootLayerID: "page-title-prototype", Bindings: map[string]string{"label":"page-title-text"}},
			"breadcrumbItem": {RootLayerID: "breadcrumb-item", Bindings: map[string]string{"label":"breadcrumb-text"}},
			"tableHeaderCell": {RootLayerID: "table-header-cell"},
			"tableRow": {RootLayerID: "table-row"},
		},
		Constraints: BlueprintConstraints{ContentWidth:1120, FilterColumns:3, FilterRowHeight:68, TableHeaderHeight:44, TableRowHeight:52, HorizontalGap:16, VerticalGap:16},
		ShellAllowlistLayerIDs: []string{"sidebar", "topbar"},
	}
	blueprint, diagnostics := BuildTemplateBlueprint(structure, classification, BlueprintSourceRefs{DesignFileID:"file-1", DesignRevisionID:"revision-1", TemplateRevisionID:"template-revision-1"})
	if diagnostics.HasErrors() { t.Fatalf("unexpected diagnostics: %+v", diagnostics) }
	if blueprint.Regions["filters"].RootLayerID != "filters" { t.Fatalf("filters = %+v", blueprint.Regions["filters"]) }
}

func TestResolveRecipeDoesNotCrossComponentKinds(t *testing.T) {
	set := completeRecipeSetForTest()
	delete(set.Recipes, RecipeKey{Kind:"select", Variant:"default", State:"default"}.String())
	_, diagnostics := ResolveRecipe(set, RecipeRequest{Kind:"select", Variant:"default", State:"default"})
	assertDiagnosticCode(t, diagnostics, "missing_recipe")
}
```

- [ ] **Step 2: Run tests and verify contracts are absent**

Run:

```bash
cd server && go test ./internal/designcore -run 'Test(BuildTemplateBlueprint|ExtractTemplateStructure|BuildComponentRecipeSet|ResolveRecipe)' -count=1
```

Expected: FAIL because Blueprint and Recipe contracts do not exist.

- [ ] **Step 3: Implement structural extraction and Blueprint validation**

Define:

```go
const TemplateBlueprintVersion = "1.0"
type Rect struct { X, Y, Width, Height float64 }
type StructuralFrame struct { ID, RootLayerID, Name string; Bounds Rect }
type StructuralLayer struct { ID, FrameID, ParentID, Name, Type, ComponentKey, Text string; Children []string; Bounds Rect; Layout map[string]any }
type TemplateStructure struct { Frames map[string]StructuralFrame; Layers map[string]StructuralLayer; HiddenLayerIDs []string }
type RegionClassification struct { RootLayerID string; ReplaceChildren bool; Bindings map[string]string }
type BlueprintRegion struct { RootLayerID string; ReplaceChildren bool; Bounds Rect; Bindings map[string]string }
type PrototypeClassification struct { RootLayerID string; Bindings map[string]string }
type BlueprintPrototype struct { RootLayerID string; Bounds Rect; Bindings map[string]string }
type BlueprintConstraints struct {
	ContentWidth, FilterRowHeight, TableHeaderHeight, TableRowHeight float64
	HorizontalGap, VerticalGap float64
	FilterColumns int
	PinFirstColumn, PinActionColumn bool
}
type BlueprintSourceRefs struct { DesignFileID, DesignRevisionID, TemplateRevisionID string }
type BlueprintClassification struct { FrameID, PageType string; Regions map[string]RegionClassification; Prototypes map[string]PrototypeClassification; Constraints BlueprintConstraints; ShellAllowlistLayerIDs []string }
type TemplateBlueprint struct { Version, FrameID, PageType string; Regions map[string]BlueprintRegion; Prototypes map[string]BlueprintPrototype; Constraints BlueprintConstraints; ShellAllowlistLayerIDs []string; SourceRefs BlueprintSourceRefs }
func ExtractTemplateStructure(doc NativeJSON) TemplateStructure
func BuildTemplateBlueprint(structure TemplateStructure, classification BlueprintClassification, refs BlueprintSourceRefs) (TemplateBlueprint, Diagnostics)
func ParseTemplateBlueprint(raw []byte) (TemplateBlueprint, error)
func ValidateTemplateBlueprint(structure TemplateStructure, blueprint TemplateBlueprint) Diagnostics
```

Add JSON tags in camelCase. Extraction uses typed source facts only and must not search names for semantic keywords. Classification explicitly selects one extracted frame; all regions, prototypes, and bindings must belong to that frame. Require regions `shell`, `content`, `breadcrumb`, `pageTitle`, `filters`, `pageActions`, `table`, `pagination` and prototypes `pageTitle`, `breadcrumbItem`, `tableHeaderCell`, `tableRow`; verify references exist and are visible; verify all binding targets descend from their root and text bindings target text layers; verify replaceable business regions are descendants of content and are not nested peers; verify shell allowlist layers descend from shell; require positive constraints and `FilterColumns` from 1 through 6. Both parsers use `DisallowUnknownFields`, and `ValidateTemplateBlueprint` reruns reference and constraint checks for persisted values. An optional `navigation` region is required only when PageSpec has a non-empty `activeNavigation`; absence becomes `missing_navigation_region`. Required diagnostic codes are `unknown_frame`, `cross_frame_reference`, `unsupported_page_type`, `missing_region`, `missing_prototype`, `unknown_source_layer`, `hidden_source_layer`, `invalid_binding`, `invalid_region_relationship`, `invalid_constraint`, `missing_navigation_region`, and `unsafe_shell_allowlist`.

- [ ] **Step 4: Implement executable Recipe validation and resolution**

Define:

```go
const ComponentRecipeSetVersion = "1.0"
type RecipeKey struct { Kind, Variant, State string }
type RecipeSource struct { RevisionID, RootLayerID, Fingerprint string }
type RecipeProp struct { TargetLayerID, Type string }
type RecipeLayout struct { WidthMode, TextOverflow string; Height, MinWidth float64 }
type ComponentRecipe struct { Kind, Variant, State string; Source RecipeSource; Props map[string]RecipeProp; Layout RecipeLayout }
type PrimitiveRecipe struct { Kind, LayerType string; Props map[string]RecipeProp; Style map[string]any; Layout RecipeLayout }
type ComponentRecipeSet struct { Version, DesignSystemProfileID, SourceRevisionID string; Tokens map[string]any; Recipes map[string]ComponentRecipe; PrimitiveFallbacks map[string]PrimitiveRecipe }
type ComponentRecipeClassification struct { Kind, Variant, State, RootLayerID string; Props map[string]RecipeProp; Layout RecipeLayout }
type RecipeRequest struct { Kind, Variant, State string }
type ResolvedRecipe struct { Recipe *ComponentRecipe; Primitive *PrimitiveRecipe; Fallback string }
func (k RecipeKey) String() string
func BuildComponentRecipeSet(profileID, sourceRevisionID, version string, source NativeJSON, classifications []ComponentRecipeClassification, primitiveFallbacks map[string]PrimitiveRecipe) (ComponentRecipeSet, Diagnostics)
func ParseComponentRecipeSet(raw []byte) (ComponentRecipeSet, error)
func ValidateComponentRecipeSet(source NativeJSON, set ComponentRecipeSet) Diagnostics
func ResolveRecipe(set ComponentRecipeSet, request RecipeRequest) (ResolvedRecipe, Diagnostics)
```

Required kinds are `input`, `select`, `date-range`, `primary-button`, `secondary-button`, `text-button`, `table-header`, `table-row`, `status-tag`, and `pagination`. `RecipeLayout.TextOverflow` is `ellipsis` or `wrap`. `BuildComponentRecipeSet` copies `source.Tokens` into `ComponentRecipeSet.Tokens`. Validate source roots and prop ancestry/types, compute `RecipeSource.Fingerprint` from canonical JSON for the complete source subtree, and reject persisted fingerprint drift. Resolution order is exact, same-kind `default/default`, then an executable `PrimitiveRecipe` under the requested kind. It never crosses component kinds. Every primitive style leaf is a `$token.path` reference that must resolve in `ComponentRecipeSet.Tokens`; raw color, radius, typography, and spacing values are invalid.

- [ ] **Step 5: Run contract tests and format**

Run:

```bash
cd server
gofmt -w internal/designcore/blueprint.go internal/designcore/blueprint_test.go internal/designcore/recipe.go internal/designcore/recipe_test.go
go test ./internal/designcore -run 'Test(BuildTemplateBlueprint|ExtractTemplateStructure|BuildComponentRecipeSet|ResolveRecipe)' -count=1
```

Expected: PASS.

- [ ] **Step 6: Check scope and commit analysis contracts**

Run GitNexus `detect_changes({scope: "working"})`, then:

```bash
git add server/internal/designcore/blueprint.go server/internal/designcore/blueprint_test.go server/internal/designcore/recipe.go server/internal/designcore/recipe_test.go
git commit -m "feat(design): define generation asset contracts"
```

---

### Task 3: Versioned Blueprint And Recipe-Set Persistence

**Files:**
- Create: `server/migrations/128_design_generation_assets.up.sql`
- Create: `server/migrations/128_design_generation_assets.down.sql`
- Modify: `server/pkg/db/queries/design.sql`
- Generated: `server/pkg/db/generated/design.sql.go`
- Generated: `server/pkg/db/generated/models.go`

**Interfaces:**
- Consumes: validated JSON produced by Task 2.
- Produces: immutable analysis versions and sqlc methods `CreateDesignTemplateBlueprint`, `GetLatestValidDesignTemplateBlueprint`, `CreateDesignComponentRecipeSet`, and `GetLatestValidDesignComponentRecipeSet`.

- [ ] **Step 1: Create bidirectional migration 128**

Create `128_design_generation_assets.up.sql`:

```sql
CREATE TABLE design_template_blueprint (
    id uuid PRIMARY KEY DEFAULT gen_random_uuid(),
    workspace_id uuid NOT NULL REFERENCES workspace(id) ON DELETE CASCADE,
    template_id uuid NOT NULL REFERENCES design_catalog_template(id) ON DELETE CASCADE,
    template_revision_id uuid NOT NULL REFERENCES design_template_revision(id) ON DELETE CASCADE,
    source_revision_id uuid NOT NULL REFERENCES design_revision(id) ON DELETE RESTRICT,
    analysis_version integer NOT NULL,
    schema_version text NOT NULL,
    status text NOT NULL CHECK (status IN ('valid', 'invalid', 'archived')),
    structure_json jsonb NOT NULL,
    blueprint_json jsonb NOT NULL,
    validation_errors jsonb NOT NULL DEFAULT '[]'::jsonb,
    created_by uuid REFERENCES "user"(id) ON DELETE SET NULL,
    created_at timestamptz NOT NULL DEFAULT now(),
    UNIQUE (template_revision_id, analysis_version)
);

CREATE INDEX idx_design_template_blueprint_latest
    ON design_template_blueprint (workspace_id, template_revision_id, analysis_version DESC)
    WHERE status = 'valid';

CREATE TABLE design_component_recipe_set (
    id uuid PRIMARY KEY DEFAULT gen_random_uuid(),
    workspace_id uuid NOT NULL REFERENCES workspace(id) ON DELETE CASCADE,
    design_system_profile_id uuid NOT NULL REFERENCES design_system_profile(id) ON DELETE CASCADE,
    source_revision_id uuid NOT NULL REFERENCES design_revision(id) ON DELETE RESTRICT,
    analysis_version integer NOT NULL,
    schema_version text NOT NULL,
    status text NOT NULL CHECK (status IN ('valid', 'invalid', 'archived')),
    recipes_json jsonb NOT NULL,
    validation_errors jsonb NOT NULL DEFAULT '[]'::jsonb,
    created_by uuid REFERENCES "user"(id) ON DELETE SET NULL,
    created_at timestamptz NOT NULL DEFAULT now(),
    UNIQUE (design_system_profile_id, analysis_version)
);

CREATE INDEX idx_design_component_recipe_set_latest
    ON design_component_recipe_set (workspace_id, design_system_profile_id, analysis_version DESC)
    WHERE status = 'valid';
```

Create `128_design_generation_assets.down.sql`:

```sql
DROP INDEX IF EXISTS idx_design_component_recipe_set_latest;
DROP TABLE IF EXISTS design_component_recipe_set;
DROP INDEX IF EXISTS idx_design_template_blueprint_latest;
DROP TABLE IF EXISTS design_template_blueprint;
```

- [ ] **Step 2: Add exact sqlc queries**

Append to `server/pkg/db/queries/design.sql`:

```sql
-- Semantic design generation assets

-- name: CreateDesignTemplateBlueprint :one
INSERT INTO design_template_blueprint (
    workspace_id, template_id, template_revision_id, source_revision_id,
    analysis_version, schema_version, status, structure_json, blueprint_json,
    validation_errors, created_by
) VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10, $11)
RETURNING *;

-- name: GetLatestValidDesignTemplateBlueprint :one
SELECT * FROM design_template_blueprint
WHERE workspace_id = $1
  AND template_revision_id = $2
  AND status = 'valid'
ORDER BY analysis_version DESC
LIMIT 1;

-- name: CreateDesignComponentRecipeSet :one
INSERT INTO design_component_recipe_set (
    workspace_id, design_system_profile_id, source_revision_id,
    analysis_version, schema_version, status, recipes_json,
    validation_errors, created_by
) VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9)
RETURNING *;

-- name: GetLatestValidDesignComponentRecipeSet :one
SELECT * FROM design_component_recipe_set
WHERE workspace_id = $1
  AND design_system_profile_id = $2
  AND status = 'valid'
ORDER BY analysis_version DESC
LIMIT 1;
```

- [ ] **Step 3: Regenerate sqlc and verify generated query shape**

Run:

```bash
make sqlc
rg -n "CreateDesignTemplateBlueprint|GetLatestValidDesignTemplateBlueprint|CreateDesignComponentRecipeSet|GetLatestValidDesignComponentRecipeSet" server/pkg/db/generated/design.sql.go
```

Expected: all four methods and parameter types are present; sqlc exits 0.

- [ ] **Step 4: Verify migration round trip**

Run `make migrate-up`, then `cd server && go test ./pkg/db/generated -count=1`. Expected: migration 128 applies and the generated package compiles. Run `make migrate-down` once, confirm migration 128 rolls back, then run `make migrate-up` so the checkout ends at version 128.

- [ ] **Step 5: Check generated scope and commit persistence**

Run GitNexus `detect_changes({scope: "working"})`, inspect generated changes for credentials and unexpected SQL, then:

```bash
git add server/migrations/128_design_generation_assets.up.sql server/migrations/128_design_generation_assets.down.sql server/pkg/db/queries/design.sql server/pkg/db/generated/design.sql.go server/pkg/db/generated/models.go
git commit -m "feat(design): persist semantic generation assets"
```

---

### Task 4: Generation Asset Store

**Files:**
- Create: `server/internal/service/design_generation_assets.go`
- Create: `server/internal/handler/design_generation_assets_test.go`

**Interfaces:**
- Consumes: sqlc rows from Task 3 and Task 2 parsers.
- Produces: typed save/load methods for the later Agent workflow.

- [ ] **Step 1: Write failing database-backed store tests**

Use the handler package's existing `testPool`, `createDesignFileForTest`, and catalog/profile setup patterns to insert a template revision and design-system profile. Save valid version 1, save invalid version 2, and prove loading selects the latest valid version rather than the latest row. Add a source-revision mismatch case returning `ErrGenerationAssetsStale`.

```go
func TestDesignGenerationAssetStoreLoadsLatestValidVersions(t *testing.T) {
	store := service.DesignGenerationAssetStore{Queries: db.New(testPool)}
	fixture := createGenerationAssetFixture(t)
	_, err := store.SaveBlueprintAnalysis(context.Background(), service.SaveBlueprintAnalysisParams{
		WorkspaceID: fixture.WorkspaceID, TemplateID: fixture.TemplateID,
		TemplateRevisionID: fixture.TemplateRevisionID, SourceRevisionID: fixture.TemplateSourceRevisionID,
		AnalysisVersion: 1, Structure: fixture.Structure, Blueprint: fixture.Blueprint,
	})
	if err != nil { t.Fatalf("save blueprint: %v", err) }
	_, err = store.SaveRecipeSetAnalysis(context.Background(), service.SaveRecipeSetAnalysisParams{
		WorkspaceID: fixture.WorkspaceID, DesignSystemProfileID: fixture.ProfileID,
		SourceRevisionID: fixture.UISourceRevisionID, AnalysisVersion: 1, RecipeSet: fixture.RecipeSet,
	})
	if err != nil { t.Fatalf("save recipes: %v", err) }
	assets, err := store.LoadCompilationAssets(context.Background(), service.LoadCompilationAssetsParams{
		WorkspaceID: fixture.WorkspaceID, TemplateRevisionID: fixture.TemplateRevisionID,
		DesignSystemProfileID: fixture.ProfileID,
	})
	if err != nil { t.Fatalf("load assets: %v", err) }
	if assets.Blueprint.Version != designcore.TemplateBlueprintVersion || assets.RecipeSet.Version != designcore.ComponentRecipeSetVersion {
		t.Fatalf("unexpected assets: %+v", assets)
	}
}
```

- [ ] **Step 2: Run focused tests and verify the store is absent**

Run:

```bash
cd server && go test ./internal/handler -run TestDesignGenerationAssetStore -count=1
```

Expected: FAIL because the store and parameter types are undefined.

- [ ] **Step 3: Implement strict persistence and loading**

Create these exact public contracts:

```go
var ErrGenerationAssetsMissing = errors.New("semantic design generation assets are missing")
var ErrGenerationAssetsStale = errors.New("semantic design generation assets do not match source revisions")
type DesignGenerationAssetStore struct { Queries *db.Queries }
type CompilationAssets struct {
	Blueprint designcore.TemplateBlueprint
	RecipeSet designcore.ComponentRecipeSet
	TemplateDoc designcore.NativeJSON
	RecipeDoc designcore.NativeJSON
	BlueprintRecordID string
	RecipeSetRecordID string
}
type SaveBlueprintAnalysisParams struct {
	WorkspaceID, TemplateID, TemplateRevisionID, SourceRevisionID pgtype.UUID
	AnalysisVersion int32
	CreatedBy pgtype.UUID
	Structure designcore.TemplateStructure
	Blueprint designcore.TemplateBlueprint
}
type SaveRecipeSetAnalysisParams struct {
	WorkspaceID, DesignSystemProfileID, SourceRevisionID pgtype.UUID
	AnalysisVersion int32
	CreatedBy pgtype.UUID
	RecipeSet designcore.ComponentRecipeSet
}
type LoadCompilationAssetsParams struct {
	WorkspaceID, TemplateRevisionID, DesignSystemProfileID pgtype.UUID
}
func (s DesignGenerationAssetStore) SaveBlueprintAnalysis(ctx context.Context, params SaveBlueprintAnalysisParams) (db.DesignTemplateBlueprint, error)
func (s DesignGenerationAssetStore) SaveRecipeSetAnalysis(ctx context.Context, params SaveRecipeSetAnalysisParams) (db.DesignComponentRecipeSet, error)
func (s DesignGenerationAssetStore) LoadCompilationAssets(ctx context.Context, params LoadCompilationAssetsParams) (CompilationAssets, error)
```

Save methods load the referenced source revisions, marshal typed values, rerun `ValidateTemplateBlueprint` or `ValidateComponentRecipeSet`, persist `status=invalid` plus diagnostics when validation fails, and return a typed validation error. Load parses with Task 2 parsers, loads both source documents into `CompilationAssets`, verifies source revision identities and Recipe fingerprints, and maps `pgx.ErrNoRows` to `ErrGenerationAssetsMissing`. No caller receives raw asset maps.

- [ ] **Step 4: Run store tests**

Run:

```bash
cd server
gofmt -w internal/service/design_generation_assets.go internal/handler/design_generation_assets_test.go
go test ./internal/handler -run TestDesignGenerationAssetStore -count=1
```

Expected: PASS.

- [ ] **Step 5: Check impact and commit the store**

Run GitNexus impact for any existing service test helper changed, then `detect_changes({scope: "working"})` and:

```bash
git add server/internal/service/design_generation_assets.go server/internal/handler/design_generation_assets_test.go
git commit -m "feat(design): load validated generation assets"
```

---

### Task 5: Deterministic Native Document Builder

**Files:**
- Create: `server/internal/designcore/document_builder.go`
- Create: `server/internal/designcore/document_builder_test.go`

**Interfaces:**
- Consumes: Blueprint template Native JSON, Recipe source Native JSON, semantic namespace, target parent/frame, and declared Recipe props.
- Produces: deterministic cloned subtrees with rewritten IDs/references and preserved image assets.

- [ ] **Step 1: Write failing clone and mutation tests**

Prove source input remains unchanged; all node IDs are fresh and stable; `ParentID`, `Children`, and exact string references inside `Text`, `Style`, `Semantic`, `Source`, `Shape`, and `Exportable` are rewritten; asset collisions are remapped; image URLs survive; undeclared prop binding fails; and identical namespaces produce byte-equal documents.

```go
func TestDocumentBuilderClonesRecipeSubtreeAndPreservesAsset(t *testing.T) {
	builder, err := NewDocumentBuilder(compilerTemplateForTest(), "issue-1/pagespec-v1/compiler-v1")
	if err != nil { t.Fatalf("NewDocumentBuilder: %v", err) }
	clone, err := builder.CloneSubtree(recipeSourceWithImageForTest(), "input-root", "filters", "frame-1", Rect{X:40, Y:80, Width:320, Height:36})
	if err != nil { t.Fatalf("CloneSubtree: %v", err) }
	if err := builder.BindText(clone, "input-label", "客户姓名"); err != nil { t.Fatalf("BindText: %v", err) }
	doc := builder.Document()
	if doc.Layers[clone.RootLayerID].ParentID != "filters" { t.Fatalf("root = %+v", doc.Layers[clone.RootLayerID]) }
	icon := doc.Layers[clone.SourceToTarget["input-icon"]]
	if got := doc.Assets[icon.Image.AssetID].URL; got != "https://example.test/search.png" { t.Fatalf("asset URL = %q", got) }
}
```

- [ ] **Step 2: Run tests and verify builder is absent**

Run `cd server && go test ./internal/designcore -run TestDocumentBuilder -count=1`.

Expected: FAIL because `DocumentBuilder` is undefined.

- [ ] **Step 3: Implement deterministic builder**

Create:

```go
type CloneResult struct { RootLayerID string; SourceToTarget map[string]string }
func NewDocumentBuilder(base NativeJSON, namespace string) (*DocumentBuilder, error)
func (b *DocumentBuilder) ClearChildren(rootLayerID string) error
func (b *DocumentBuilder) CloneSubtree(source NativeJSON, sourceRootID, targetParentID, targetFrameID string, bounds Rect) (CloneResult, error)
func (b *DocumentBuilder) BindText(clone CloneResult, sourceTargetLayerID, value string) error
func (b *DocumentBuilder) FitCloneLayer(clone CloneResult, sourceTargetLayerID string, bounds Rect) error
func (b *DocumentBuilder) SetBounds(layerID string, bounds Rect) error
func (b *DocumentBuilder) AddPrimitiveLayer(parentID string, layer Layer) (string, error)
func (b *DocumentBuilder) Document() NativeJSON
```

Generate IDs from SHA-256 of namespace, operation sequence, and source ID, formatted as `gen-` plus the first 20 lowercase hex characters. Deep-copy maps/slices before mutation. Build full node/asset maps before rewriting exact string values recursively; never rewrite substrings. `FitCloneLayer` lets the compiler size declared text/value targets to the available Recipe content box after resizing the root. Append the cloned root to the target parent only after every cloned layer validates.

- [ ] **Step 4: Run builder and Native JSON tests**

Run:

```bash
cd server
gofmt -w internal/designcore/document_builder.go internal/designcore/document_builder_test.go
go test ./internal/designcore -run 'TestDocumentBuilder|TestValidateDocument' -count=1
```

Expected: PASS.

- [ ] **Step 5: Check scope and commit builder**

Run GitNexus `detect_changes({scope: "working"})`, then:

```bash
git add server/internal/designcore/document_builder.go server/internal/designcore/document_builder_test.go
git commit -m "feat(design): add deterministic native document builder"
```

---

### Task 6: Text Measurement And Table Width Allocation

**Files:**
- Create: `server/internal/designcore/text_measure.go`
- Create: `server/internal/designcore/table_layout.go`
- Create: `server/internal/designcore/table_layout_test.go`

**Interfaces:**
- Consumes: semantic columns, sample rows, content viewport, typography metrics, and fixed action/status widths.
- Produces: `TableLayout` with concrete widths, total width, horizontal-scroll strategy, and optional pinned first/action columns.

- [ ] **Step 1: Write failing allocation tests**

Cover preferred widths within viewport, flexible expansion, Chinese and ASCII long content, minimum total exceeding viewport, status/action reserved widths, and no column below minimum.

```go
func TestAllocateTableLayoutUsesHorizontalScrollBeforeOverlap(t *testing.T) {
	columns := []TableColumnSpec{
		{Key:"customerName", Title:"客户姓名", Cell:"text", Width:"wide"},
		{Key:"phone", Title:"手机号", Cell:"text", Width:"medium"},
		{Key:"company", Title:"所属公司", Cell:"text", Width:"wide"},
		{Key:"status", Title:"客户状态", Cell:"status-tag", Width:"narrow"},
	}
	rows := []map[string]string{{"customerName":"一个用于验证宽度分配的很长示例客户名称","phone":"13800000001","company":"示例科技有限公司华东业务中心","status":"待跟进"}}
	layout, diagnostics := AllocateTableLayout(TableLayoutInput{Columns:columns, Rows:rows, RowActionCount:2, ViewportWidth:620, Typography:TypographyMetrics{FontSize:14}, CellHorizontalPadding:16})
	if diagnostics.HasErrors() { t.Fatalf("unexpected diagnostics: %+v", diagnostics) }
	if !layout.HorizontalScroll || layout.TotalWidth <= 620 { t.Fatalf("layout = %+v", layout) }
	for _, column := range layout.Columns {
		if column.Width < column.MinWidth { t.Fatalf("column below minimum: %+v", column) }
	}
}
```

- [ ] **Step 2: Run tests and verify layout code is absent**

Run `cd server && go test ./internal/designcore -run 'Test(MeasureText|AllocateTableLayout)' -count=1`.

Expected: FAIL because width allocation types are undefined.

- [ ] **Step 3: Implement conservative measurement and allocation**

Create:

```go
type TypographyMetrics struct { FontSize, LetterSpacing float64 }
func MeasureTextWidth(text string, metrics TypographyMetrics) float64
type TableLayoutInput struct {
	Columns []TableColumnSpec
	Rows []map[string]string
	RowActionCount int
	ViewportWidth float64
	Typography TypographyMetrics
	CellHorizontalPadding float64
}
type TableColumnLayout struct { Key string; X, Width, MinWidth, PreferredWidth, MaxWidth float64; Pinned string }
type TableLayout struct { Columns []TableColumnLayout; ActionColumn *TableColumnLayout; TotalWidth float64; HorizontalScroll bool }
func AllocateTableLayout(input TableLayoutInput) (TableLayout, Diagnostics)
```

Use deterministic Unicode factors: CJK/full-width `1.0 * fontSize`, uppercase ASCII `0.68`, lowercase/digits `0.56`, whitespace `0.33`, punctuation `0.4`, plus letter spacing. Width-hint min/preferred/max values are narrow `96/120/160`, medium `140/180/260`, wide `200/260/420`, flexible `160/240/600`. Content plus twice cell padding may raise preferred width to max but never lower min. Reserve 88 pixels per row action capped at 176. If minimum total exceeds viewport, keep minima and set horizontal scroll. Otherwise distribute remaining width to preferred widths, then flexible maxima.

- [ ] **Step 4: Run layout tests and format**

Run:

```bash
cd server
gofmt -w internal/designcore/text_measure.go internal/designcore/table_layout.go internal/designcore/table_layout_test.go
go test ./internal/designcore -run 'Test(MeasureText|AllocateTableLayout)' -count=1
```

Expected: PASS.

- [ ] **Step 5: Check scope and commit layout engine**

Run GitNexus `detect_changes({scope: "working"})`, then:

```bash
git add server/internal/designcore/text_measure.go server/internal/designcore/table_layout.go server/internal/designcore/table_layout_test.go
git commit -m "feat(design): add deterministic table layout"
```

---

### Task 7: List-Page Compiler Passes

**Files:**
- Create: `server/internal/designcore/compiler.go`
- Create: `server/internal/designcore/compiler_regions.go`
- Create: `server/internal/designcore/compiler_table.go`
- Create: `server/internal/designcore/compiler_test.go`

**Interfaces:**
- Consumes: valid PageSpec, Blueprint, Recipe set, template/Recipe source Native JSON, and provenance.
- Produces: `CompileListPage(input CompileInput) CompileOutput`; failures are diagnostics and never trigger patch fallback.

- [ ] **Step 1: Write failing semantic compilation tests**

Test 1/3/4 filters reflowing the table vertically, removed template filters closing gaps, page-action variants, variable column/row counts, actual status-tag Recipe cloning, complete sample cells, pagination moving after rows, default fallback warning, missing status Recipe failure, and unsupported page type failure.

```go
func TestCompileListPageBuildsCountsFromPageSpec(t *testing.T) {
	input := completeCompilerInputForTest()
	input.PageSpec.Filters = input.PageSpec.Filters[:2]
	input.PageSpec.Table.SampleRows = append(input.PageSpec.Table.SampleRows, map[string]string{
		"customerName":"示例客户B","phone":"13800000002","status":"待跟进","createdAt":"2026-07-22 11:00",
	})
	output := CompileListPage(input)
	if output.Diagnostics.HasErrors() { t.Fatalf("compile diagnostics: %+v", output.Diagnostics) }
	if output.Manifest.FilterCount != 2 || output.Manifest.RowCount != 2 || output.Manifest.ColumnCount != len(input.PageSpec.Table.Columns) {
		t.Fatalf("manifest = %+v", output.Manifest)
	}
	if got := countGeneratedRole(output.Document, "status-tag"); got != 2 { t.Fatalf("status tag count = %d", got) }
}
```

- [ ] **Step 2: Run tests and verify compiler is absent**

Run `cd server && go test ./internal/designcore -run TestCompileListPage -count=1`.

Expected: FAIL because compiler types are undefined.

- [ ] **Step 3: Implement compiler orchestration and provenance**

Define:

```go
const DesignCompilerVersion = "list-1.0"
type CompileProvenance struct {
	WorkspaceID, ProjectID, IssueID, AgentTaskID string
	PageSpecVersion, BlueprintRecordID, RecipeSetRecordID string
}
type CompileInput struct {
	PageSpec PageSpec
	Blueprint TemplateBlueprint
	RecipeSet ComponentRecipeSet
	TemplateDoc NativeJSON
	RecipeDoc NativeJSON
	Provenance CompileProvenance
}
type CompilationManifest struct {
	FilterCount, PageActionCount, ColumnCount, RowCount, RowActionCount int
	HorizontalScroll bool
	GeneratedLayerIDs, BusinessRegionLayerIDs, TemplateBusinessTexts []string
}
type CompileOutput struct {
	Status string
	Document NativeJSON
	Manifest CompilationManifest
	Diagnostics Diagnostics
}
func CompileListPage(input CompileInput) CompileOutput
```

The Task 7 pass order is: validate PageSpec and assets; deep-copy template; collect pre-clear business text; clear replaceable regions; instantiate page title and breadcrumb prototypes; bind the declared active-navigation region when supplied; resolve and instantiate filters/actions; allocate and instantiate table; instantiate pagination; write provenance; validate Native JSON. Write `Document.Source["generation"]` with compiler version, PageSpec version, Blueprint/Recipe record IDs, template/UI-spec source revision IDs, workspace/project/Issue/task IDs, and horizontal-scroll strategy. Return internal status `compiled` when these passes succeed and `compile_failed` with retained document and diagnostics on error. Task 8 replaces `compiled` with a quality-gated public status before any workflow integration. Never call slot or patch helpers.

- [ ] **Step 4: Implement region cleanup and vertical reflow**

Use Blueprint region bounds and constraints. Clone `pageTitle` once and `breadcrumbItem` once per PageSpec breadcrumb entry from the immutable pre-clear template source. If `activeNavigation` is present, bind the Blueprint navigation region's declared label target and active state; never infer the target from text matching. Filters fill `FilterColumns` left-to-right and wrap by `FilterRowHeight + VerticalGap`; search/clear controls follow the last filter. Compute table Y from actual filter rows and shift table/pagination roots by delta without changing shell allowlisted layers. Empty semantic arrays leave empty business regions. Mark generated roots with `generatedBy`, `generationRole`, and `specPath` semantic metadata.

- [ ] **Step 5: Implement table, status, actions, and pagination**

Call `AllocateTableLayout`; use Blueprint `tableRow` prototypes as composition containers after clearing sample children; use Blueprint `tableHeaderCell` bounds as header composition constraints; clone Recipe `table-header` and `table-row` appearance into those structures; alternate `table-row` variants `default` and `alternate` when available; bind declared `label`/`value` props; call `FitCloneLayer` so text targets use the allocated content width; apply Recipe `ellipsis` or `wrap` policy to the target text layer; resolve `status-tag` via each column's `statusMap`; clone `text-button` for row actions; clone pagination only when enabled. Instantiate a primitive only from `ResolvedRecipe.Primitive`, attach a warning, and never invent style values. Honor Blueprint `PinFirstColumn` and `PinActionColumn` only when horizontal scrolling is active. Set clipping/horizontal-scroll metadata on the table region when needed. Every body cell value comes from `sampleRows[row][column.Key]`.

- [ ] **Step 6: Run compiler tests**

Run:

```bash
cd server
gofmt -w internal/designcore/compiler.go internal/designcore/compiler_regions.go internal/designcore/compiler_table.go internal/designcore/compiler_test.go
go test ./internal/designcore -run 'TestCompileListPage|TestValidateDocument' -count=1
```

Expected: PASS with valid Native JSON references.

- [ ] **Step 7: Check scope and commit compiler passes**

Run GitNexus `detect_changes({scope: "working"})`, then:

```bash
git add server/internal/designcore/compiler.go server/internal/designcore/compiler_regions.go server/internal/designcore/compiler_table.go server/internal/designcore/compiler_test.go
git commit -m "feat(design): compile semantic list pages"
```

---

### Task 8: Blocking Quality Gate

**Files:**
- Create: `server/internal/designcore/quality_gate.go`
- Create: `server/internal/designcore/quality_gate_test.go`
- Modify: `server/internal/designcore/compiler.go`

**Interfaces:**
- Consumes: compiled document, PageSpec, Blueprint, CompilationManifest, and compiler diagnostics.
- Produces: final `generated`, `generated_with_warnings`, or `compile_failed` status.

- [ ] **Step 1: Run GitNexus impact for compiler orchestration**

Run `impact({target: "CompileListPage", direction: "upstream"})`. Expected: only new `designcore` tests call it. Stop and report HIGH or CRITICAL risk.

- [ ] **Step 2: Write failing quality-gate tests**

Add one test per code: `text_overflow`, `unexpected_overlap`, `off_frame`, `broken_native_json`, `unresolved_recipe`, `count_mismatch`, `template_residue`, `pagination_misplaced`, and `component_nonconformance`. Prove parent/child containment and status text inside a tag do not count as unexpected overlap.

```go
func TestEvaluateCompiledDesignBlocksTemplateBusinessResidue(t *testing.T) {
	input := completeCompilerInputForTest()
	output := compileWithoutQualityGateForTest(input)
	table := output.Document.Layers[input.Blueprint.Regions["table"].RootLayerID]
	residue := Layer{ID:"residue", FrameID:table.FrameID, ParentID:table.ID, Name:"采购价格", Type:"text", Visible:true, X:table.X+20, Y:table.Y+20, Width:100, Height:24, Text:map[string]any{"characters":"采购价格","fontSize":14}}
	output.Document.Layers[residue.ID] = residue
	table.Children = append(table.Children, residue.ID)
	output.Document.Layers[table.ID] = table
	report := EvaluateCompiledDesign(output.Document, input.PageSpec, input.Blueprint, output.Manifest, nil)
	if report.Status != "compile_failed" { t.Fatalf("status = %q", report.Status) }
	assertDiagnosticCode(t, report.Diagnostics, "template_residue")
}
```

- [ ] **Step 3: Run tests and verify gate is absent**

Run `cd server && go test ./internal/designcore -run TestEvaluateCompiledDesign -count=1`.

Expected: FAIL because quality-gate contracts are undefined.

- [ ] **Step 4: Implement quality report and checks**

Define:

```go
type QualityMetrics struct {
	TextOverflowCount, UnexpectedOverlapCount, OffFrameCount int
	TemplateResidueCount, MissingComponentCount int
}
type QualityReport struct {
	Status string `json:"status"`
	Diagnostics Diagnostics `json:"diagnostics"`
	Metrics QualityMetrics `json:"metrics"`
}
func EvaluateCompiledDesign(doc NativeJSON, spec PageSpec, blueprint TemplateBlueprint, manifest CompilationManifest, compilerDiagnostics Diagnostics) QualityReport
```

Text overflow uses `MeasureTextWidth` plus padding unless Recipe metadata explicitly enables wrapping. Overlap compares visible generated siblings, skips ancestor/descendant containment, and permits Recipe-declared overlay roles. Off-frame checks generated bounds against frame or horizontal-scroll content bounds. Count consistency compares semantic roles to manifest. Residue compares normalized business-region text against pre-clear template text, excluding PageSpec text and shell allowlist descendants. Pagination must start below the last row. Generated component metadata must match resolved Recipe keys and source fingerprints, proving component/style conformance for cloned Recipes.

Any error yields `compile_failed`; warning-only output yields `generated_with_warnings`; no diagnostics yields `generated`.

- [ ] **Step 5: Integrate the gate as final compiler pass**

Add `Quality QualityReport` to `CompileOutput`. Update `CompileListPage` to call `EvaluateCompiledDesign` once after `ValidateDocument`, copy `Quality.Status` to `CompileOutput.Status`, and retain output on failure. No `compiled` status may leave `CompileListPage` after this task, and there is no boolean success flag callers can ignore.

- [ ] **Step 6: Run quality and compiler tests**

Run:

```bash
cd server
gofmt -w internal/designcore/quality_gate.go internal/designcore/quality_gate_test.go internal/designcore/compiler.go
go test ./internal/designcore -run 'TestEvaluateCompiledDesign|TestCompileListPage' -count=1
```

Expected: PASS.

- [ ] **Step 7: Check scope and commit gate**

Run GitNexus `detect_changes({scope: "working"})`, then:

```bash
git add server/internal/designcore/quality_gate.go server/internal/designcore/quality_gate_test.go server/internal/designcore/compiler.go
git commit -m "feat(design): block invalid generated designs"
```

---

### Task 9: CRM Customer-List Acceptance And Foundation Verification

**Files:**
- Create: `server/internal/designcore/testdata/crm_customer_list_page_spec.json`
- Create: `server/internal/designcore/testdata/crm_list_blueprint.json`
- Create: `server/internal/designcore/testdata/crm_recipe_set.json`
- Create: `server/internal/designcore/testdata/crm_template_native.json`
- Create: `server/internal/designcore/testdata/crm_ui_spec_native.json`
- Create: `server/internal/designcore/crm_list_compiler_test.go`

**Interfaces:**
- Consumes: all Task 1-8 contracts.
- Produces: executable proof of zero purchase-template residue, overlap, and overflow, with complete business content, real status tags, and deterministic output.

- [ ] **Step 1: Add five explicit JSON fixtures**

The PageSpec fixture contains exactly:

- Filters: `customerKeyword`, `phone`, `status`, `createdAt`.
- Columns: `customerNo`, `customerName`, `phone`, `status`, `owner`, `createdAt`.
- Row actions: `view`, `edit`.
- Three synthetic rows with every visible column populated.
- Status mapping: `正常 -> success`, `待跟进 -> warning`, `已流失 -> disabled`.

The template fixture deliberately includes visible replaceable-region text `采购价格`, `新增价格`, `产品名称`, and `供应商`; shell navigation text remains allowlisted. The UI-spec fixture contains full source subtrees and label targets for every required Recipe kind and the three status variants. Blueprint and Recipe references must all resolve in their source documents.

- [ ] **Step 2: Write end-to-end acceptance test**

Create:

```go
func TestCRMCustomerListCompilerAcceptance(t *testing.T) {
	input := loadCRMCompilerFixture(t)
	first := CompileListPage(input)
	second := CompileListPage(input)
	if first.Status != "generated" { t.Fatalf("status = %q, diagnostics = %+v", first.Status, first.Diagnostics) }
	firstJSON, err := json.Marshal(first.Document)
	if err != nil { t.Fatalf("marshal first: %v", err) }
	secondJSON, err := json.Marshal(second.Document)
	if err != nil { t.Fatalf("marshal second: %v", err) }
	if !bytes.Equal(firstJSON, secondJSON) { t.Fatal("compiler output is not deterministic") }
	text := allVisibleText(first.Document)
	for _, residue := range []string{"采购价格", "新增价格", "产品名称", "供应商"} {
		if strings.Contains(text, residue) { t.Fatalf("template residue %q remains", residue) }
	}
	for _, required := range []string{"客户姓名", "手机号", "客户状态", "创建时间", "客户编号", "负责人"} {
		if !strings.Contains(text, required) { t.Fatalf("required business text %q is missing", required) }
	}
	if countGeneratedRole(first.Document, "status-tag") != 3 { t.Fatal("every sample row must instantiate one status tag") }
	if first.Manifest.FilterCount != 4 || first.Manifest.ColumnCount != 6 || first.Manifest.RowCount != 3 {
		t.Fatalf("manifest = %+v", first.Manifest)
	}
	if first.Quality.Metrics.TextOverflowCount != 0 || first.Quality.Metrics.UnexpectedOverlapCount != 0 || first.Quality.Metrics.TemplateResidueCount != 0 {
		t.Fatalf("quality = %+v", first.Quality)
	}
}
```

- [ ] **Step 3: Run CRM acceptance and correct only real gaps**

Run:

```bash
cd server && go test ./internal/designcore -run TestCRMCustomerListCompilerAcceptance -count=1 -v
```

Expected before final corrections: FAIL with a diagnostic code or invariant, never a panic. Correct fixture references or compiler behavior until it passes. Do not weaken assertions or allowlist purchase-domain business text.

- [ ] **Step 4: Run the focused foundation suite**

Run:

```bash
cd server
go test ./internal/designcore ./internal/service -count=1
go test ./internal/handler -run TestDesignGenerationAssetStore -count=1
```

Expected: PASS.

- [ ] **Step 5: Run repository verification relevant to the foundation**

Run:

```bash
make sqlc
cd server && go test ./internal/designcore ./internal/service ./internal/handler -count=1
git diff --check
```

Expected: sqlc produces no uncommitted drift, Go packages pass, and `git diff --check` exits 0. If handler tests are blocked by unavailable PostgreSQL, record the environment error verbatim and retain the pure `designcore` and configured database service results; do not report the handler suite as passed.

- [ ] **Step 6: Run final GitNexus change detection**

Run:

```text
detect_changes({scope: "working"})
detect_changes({scope: "compare", base_ref: "main"})
```

Expected affected area: semantic generation assets and `designcore` compilation. Existing Issue-to-Agent task execution and patch materialization processes must not be modified by this foundation plan.

- [ ] **Step 7: Commit acceptance fixtures**

```bash
git add server/internal/designcore/testdata/crm_customer_list_page_spec.json server/internal/designcore/testdata/crm_list_blueprint.json server/internal/designcore/testdata/crm_recipe_set.json server/internal/designcore/testdata/crm_template_native.json server/internal/designcore/testdata/crm_ui_spec_native.json server/internal/designcore/crm_list_compiler_test.go
git commit -m "test(design): prove CRM semantic list generation"
```

---

## Completion Gate

The foundation is complete only when:

- PageSpec rejects unknown fields and geometry/layer instructions.
- Blueprint and Recipe contracts validate Agent classifications against objective source facts without backend keyword categorization.
- Blueprint and Recipe-set versions are immutable, valid, and source-current when loaded.
- Complete source subtrees and asset references survive deterministic cloning.
- Filters, columns, rows, status tags, actions, and pagination come from semantic counts rather than template copy.
- Long content gets safe widths or horizontal scrolling; no cell drops below minimum width.
- Quality failures return `compile_failed` and cannot be mistaken for reviewable output.
- CRM acceptance has no purchase-domain residue, text overflow, or unexpected overlap.
- Identical inputs produce byte-identical typed Native Design JSON.
- No production Agent task, prompt, draft-review UI, or legacy draft reader changed.

## Next Plan Boundary

After this plan passes, write `UI Agent Semantic Draft Workflow Implementation Plan` covering:

1. template/UI-spec analysis task dispatch and persistence;
2. `UIDraftCreateContext` migration from slot/patch output to strict PageSpec output;
3. Agent completion parsing, compiler invocation, and failure diagnostics;
4. semantic draft status/version lineage and Native JSON persistence;
5. review, revision-note, reject, and approve/materialize APIs;
6. React Query schemas/hooks, design-center review UI, and realtime invalidation;
7. Issue-to-draft-to-approval end-to-end verification.
