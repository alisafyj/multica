package designcore

import (
	"encoding/json"
	"fmt"
	"reflect"
	"sort"
	"strings"
	"testing"
)

func TestCompileListPageBuildsCountsAndRecipeBackedContent(t *testing.T) {
	input := completeCompilerInputForTest(t)
	input.PageSpec.Filters = input.PageSpec.Filters[:2]
	input.PageSpec.Table.SampleRows = append(input.PageSpec.Table.SampleRows, map[string]string{
		"customerName": "Example Customer B",
		"phone":        "13800000002",
		"status":       "Active",
		"createdAt":    "2026-07-22 11:00",
	})

	output := CompileListPage(input)
	if output.Diagnostics.HasErrors() {
		t.Fatalf("compile diagnostics: %+v", output.Diagnostics)
	}
	if output.Status != "generated" {
		t.Fatalf("status = %q", output.Status)
	}
	if output.Manifest.FilterCount != 2 || output.Manifest.PageActionCount != 2 || output.Manifest.RowCount != 2 || output.Manifest.ColumnCount != len(input.PageSpec.Table.Columns) || output.Manifest.RowActionCount != 2 {
		t.Fatalf("manifest = %+v", output.Manifest)
	}
	if got := countGeneratedRole(output.Document, "status-tag"); got != 2 {
		t.Fatalf("status tag count = %d", got)
	}
	if got := countGeneratedRole(output.Document, "row-action"); got != 4 {
		t.Fatalf("row action count = %d", got)
	}
	if got := countGeneratedRole(output.Document, "table-header"); got != len(input.PageSpec.Table.Columns) {
		t.Fatalf("table header count = %d", got)
	}

	for rowIndex, row := range input.PageSpec.Table.SampleRows {
		for _, column := range input.PageSpec.Table.Columns {
			path := fmt.Sprintf("table.sampleRows.%d.%s", rowIndex, column.Key)
			root := generatedRootAtSpecPath(t, output.Document, path)
			if !subtreeContainsText(output.Document, root.ID, row[column.Key]) {
				t.Errorf("%s does not contain %q", path, row[column.Key])
			}
			if column.Cell == "status-tag" {
				wantVariant := column.StatusMap[row[column.Key]]
				if got := root.Semantic["recipeVariant"]; got != wantVariant {
					t.Errorf("%s recipeVariant = %#v, want %q", path, got, wantVariant)
				}
				key := (RecipeKey{Kind: "status-tag", Variant: wantVariant, State: "default"}).String()
				wantRecipe := input.RecipeSet.Recipes[key]
				if root.Semantic["recipeState"] != "default" || root.Semantic["recipeSourceRootLayerId"] != wantRecipe.Source.RootLayerID || root.Semantic["recipeSourceFingerprint"] != wantRecipe.Source.Fingerprint {
					t.Errorf("%s recipe provenance = %+v", path, root.Semantic)
				}
				if root.Style["sourceVariant"] != wantVariant {
					t.Errorf("%s did not clone the %q status recipe: %+v", path, wantVariant, root.Style)
				}
				if !subtreeHasTextOverflow(output.Document, root.ID, wantRecipe.Layout.TextOverflow) {
					t.Errorf("%s does not apply text overflow %q", path, wantRecipe.Layout.TextOverflow)
				}
			}
		}
	}

	wantActionKinds := map[string]string{
		"pageActions.create": "primary-button",
		"pageActions.export": "secondary-button",
	}
	for path, wantKind := range wantActionKinds {
		root := generatedRootAtSpecPath(t, output.Document, path)
		if root.Semantic["recipeKind"] != wantKind {
			t.Errorf("%s recipeKind = %#v, want %q", path, root.Semantic["recipeKind"], wantKind)
		}
	}
}

func TestCompileListPageSupportsNarrowViewportLongContentWithScrollAndEllipsis(t *testing.T) {
	input := completeCompilerInputForTest(t)
	input.Blueprint.Constraints.ContentWidth = 260
	input.PageSpec.Table.SampleRows[0]["customerName"] = strings.Repeat("Long customer record ", 20)
	input.PageSpec.Table.SampleRows[0]["createdAt"] = strings.Repeat("2026-07-22 ", 12)

	output := CompileListPage(input)
	if output.Status != "generated" || !output.Manifest.HorizontalScroll {
		t.Fatalf("status=%q scroll=%v diagnostics=%+v", output.Status, output.Manifest.HorizontalScroll, output.Diagnostics)
	}
	assertNoDiagnosticCode(t, output.Diagnostics, "text_overflow")
}

func TestCompileListPageReflowsFiltersActionsRowsAndPagination(t *testing.T) {
	filterY := map[int]float64{}
	for _, count := range []int{0, 1, 3, 4} {
		input := completeCompilerInputForTest(t)
		input.PageSpec.Filters = input.PageSpec.Filters[:count]
		output := CompileListPage(input)
		if output.Diagnostics.HasErrors() {
			t.Fatalf("filters=%d diagnostics: %+v", count, output.Diagnostics)
		}
		filterY[count] = output.Document.Layers[input.Blueprint.Regions["table"].RootLayerID].Y
	}
	if !(filterY[0] < filterY[1] && filterY[1] == filterY[3]) {
		t.Fatalf("table Y by filter count = %+v", filterY)
	}
	input := completeCompilerInputForTest(t)
	wantDelta := input.Blueprint.Constraints.FilterRowHeight + input.Blueprint.Constraints.VerticalGap
	if got := filterY[4] - filterY[3]; got != wantDelta {
		t.Fatalf("fourth-filter table delta = %v, want %v", got, wantDelta)
	}

	withoutActions := completeCompilerInputForTest(t)
	withoutActions.PageSpec.PageActions = nil
	withoutActionsOutput := CompileListPage(withoutActions)
	if withoutActionsOutput.Diagnostics.HasErrors() {
		t.Fatalf("zero page actions diagnostics: %+v", withoutActionsOutput.Diagnostics)
	}
	withActionsOutput := CompileListPage(completeCompilerInputForTest(t))
	withoutY := withoutActionsOutput.Document.Layers[withoutActions.Blueprint.Regions["table"].RootLayerID].Y
	withY := withActionsOutput.Document.Layers[withoutActions.Blueprint.Regions["table"].RootLayerID].Y
	if withoutY >= withY || len(withoutActionsOutput.Document.Layers[withoutActions.Blueprint.Regions["pageActions"].RootLayerID].Children) != 0 {
		t.Fatalf("page actions did not collapse: without=%v with=%v", withoutY, withY)
	}

	oneRow := completeCompilerInputForTest(t)
	oneOutput := CompileListPage(oneRow)
	threeRows := completeCompilerInputForTest(t)
	threeRows.PageSpec.Table.SampleRows = append(threeRows.PageSpec.Table.SampleRows,
		map[string]string{"customerName": "B", "phone": "2", "status": "Active", "createdAt": "2026-07-22 11:00"},
		map[string]string{"customerName": "C", "phone": "3", "status": "Pending", "createdAt": "2026-07-22 12:00"},
	)
	threeOutput := CompileListPage(threeRows)
	if oneOutput.Diagnostics.HasErrors() || threeOutput.Diagnostics.HasErrors() {
		t.Fatalf("row reflow diagnostics: one=%+v three=%+v", oneOutput.Diagnostics, threeOutput.Diagnostics)
	}
	paginationID := oneRow.Blueprint.Regions["pagination"].RootLayerID
	wantPaginationDelta := 2 * oneRow.Blueprint.Constraints.TableRowHeight
	if got := threeOutput.Document.Layers[paginationID].Y - oneOutput.Document.Layers[paginationID].Y; got != wantPaginationDelta {
		t.Fatalf("pagination row delta = %v, want %v", got, wantPaginationDelta)
	}
}

func TestCompileListPageSupportsEmptySingleAndMultipleCollections(t *testing.T) {
	tests := []struct {
		name       string
		mutate     func(*CompileInput)
		columns    int
		rows       int
		rowActions int
		pagination int
	}{
		{
			name: "single",
			mutate: func(input *CompileInput) {
				input.PageSpec.Filters = input.PageSpec.Filters[:1]
				input.PageSpec.PageActions = nil
				input.PageSpec.Table.Columns = input.PageSpec.Table.Columns[:1]
				input.PageSpec.Table.SampleRows = []map[string]string{{"customerName": "Only"}}
				input.PageSpec.Table.RowActions = nil
				input.PageSpec.Pagination.Enabled = false
			},
			columns: 1,
			rows:    1,
		},
		{
			name: "multiple",
			mutate: func(input *CompileInput) {
				input.PageSpec.Table.SampleRows = append(input.PageSpec.Table.SampleRows, map[string]string{
					"customerName": "Second", "phone": "2", "status": "Active", "createdAt": "2026-07-22 11:00",
				})
			},
			columns:    4,
			rows:       2,
			rowActions: 4,
			pagination: 1,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			input := completeCompilerInputForTest(t)
			tt.mutate(&input)
			output := CompileListPage(input)
			if output.Diagnostics.HasErrors() {
				t.Fatalf("diagnostics: %+v", output.Diagnostics)
			}
			if got := countGeneratedRole(output.Document, "table-header"); got != tt.columns {
				t.Errorf("table headers = %d, want %d", got, tt.columns)
			}
			if got := countGeneratedRole(output.Document, "table-row"); got != tt.rows {
				t.Errorf("table rows = %d, want %d", got, tt.rows)
			}
			if got := countGeneratedRole(output.Document, "row-action"); got != tt.rowActions {
				t.Errorf("row actions = %d, want %d", got, tt.rowActions)
			}
			if got := countGeneratedRole(output.Document, "pagination"); got != tt.pagination {
				t.Errorf("pagination = %d, want %d", got, tt.pagination)
			}
			if tt.name == "empty" {
				for _, key := range []string{"breadcrumb", "filters", "pageActions", "table", "pagination"} {
					regionID := input.Blueprint.Regions[key].RootLayerID
					if children := output.Document.Layers[regionID].Children; len(children) != 0 {
						t.Errorf("%s region children = %v", key, children)
					}
				}
			}
		})
	}
}

func TestCompileListPageBlocksSemanticallyEmptyPageSpec(t *testing.T) {
	input := completeCompilerInputForTest(t)
	input.PageSpec.Page.Title = " "
	input.PageSpec.Table.Columns = nil
	input.PageSpec.RequirementCoverage[0].SpecPaths = nil

	output := CompileListPage(input)
	if output.Status != "compile_failed" {
		t.Fatalf("status = %q", output.Status)
	}
	for _, code := range []string{"missing_required_field", "missing_table_column", "missing_requirement_coverage"} {
		assertDiagnosticCode(t, output.Diagnostics, code)
	}
}

func TestCompileListPagePropagatesDefaultAndPrimitiveFallbackWarnings(t *testing.T) {
	t.Run("default", func(t *testing.T) {
		input := completeCompilerInputForTest(t)
		delete(input.RecipeSet.Recipes, (RecipeKey{Kind: "table-row", Variant: "alternate", State: "default"}).String())
		input.PageSpec.Table.SampleRows = append(input.PageSpec.Table.SampleRows, map[string]string{
			"customerName": "Second", "phone": "2", "status": "Active", "createdAt": "2026-07-22 11:00",
		})

		output := CompileListPage(input)
		if output.Diagnostics.HasErrors() {
			t.Fatalf("diagnostics: %+v", output.Diagnostics)
		}
		assertDiagnosticCode(t, output.Diagnostics, "recipe_default_fallback")
	})

	t.Run("primitive", func(t *testing.T) {
		input := completeCompilerInputForTest(t)
		defaultKey := (RecipeKey{Kind: "select", Variant: "default", State: "default"}).String()
		nonDefault := input.RecipeSet.Recipes[defaultKey]
		nonDefault.Variant = "compact"
		nonDefault.State = "focused"
		input.RecipeSet.Recipes[(RecipeKey{Kind: "select", Variant: "compact", State: "focused"}).String()] = nonDefault
		delete(input.RecipeSet.Recipes, defaultKey)

		output := CompileListPage(input)
		if output.Diagnostics.HasErrors() {
			t.Fatalf("diagnostics: %+v", output.Diagnostics)
		}
		assertDiagnosticCode(t, output.Diagnostics, "primitive_fallback")
		root := generatedRootAtSpecPath(t, output.Document, "filters.status")
		if root.Semantic["recipeFallback"] != "primitive" {
			t.Fatalf("recipe fallback = %#v", root.Semantic["recipeFallback"])
		}
		if root.Style["fill"] != "#1677ff" {
			t.Fatalf("resolved fill = %#v", root.Style["fill"])
		}
		spacing, ok := root.Style["spacing"].(map[string]any)
		if !ok || spacing["horizontal"] != float64(12) {
			t.Fatalf("resolved spacing = %#v", root.Style["spacing"])
		}
		if raw, _ := json.Marshal(root.Style); strings.Contains(string(raw), "$") {
			t.Fatalf("primitive style retained token references: %s", raw)
		}
	})
}

func TestCompileListPageRequiresExactMappedStatusRecipe(t *testing.T) {
	for _, variant := range []string{"success", "warning", "danger", "disabled", "info"} {
		t.Run("missing exact "+variant, func(t *testing.T) {
			input := completeCompilerInputForTest(t)
			input.PageSpec.Table.Columns[2].StatusMap["Pending"] = variant
			delete(input.RecipeSet.Recipes, (RecipeKey{Kind: "status-tag", Variant: variant, State: "default"}).String())

			output := CompileListPage(input)
			if output.Status != compileStatusFailed {
				t.Fatalf("status = %q, diagnostics = %+v", output.Status, output.Diagnostics)
			}
			assertDiagnosticCode(t, output.Diagnostics, "missing_recipe")
		})
	}

	t.Run("primitive cannot mask deleted mapped variant", func(t *testing.T) {
		input := completeCompilerInputForTest(t)
		delete(input.RecipeSet.Recipes, (RecipeKey{Kind: "status-tag", Variant: "warning", State: "default"}).String())
		delete(input.RecipeSet.Recipes, (RecipeKey{Kind: "status-tag", Variant: "default", State: "default"}).String())

		output := CompileListPage(input)
		if output.Status != compileStatusFailed {
			t.Fatalf("status = %q, diagnostics = %+v", output.Status, output.Diagnostics)
		}
		assertDiagnosticCode(t, output.Diagnostics, "missing_recipe")
	})

	t.Run("default remains rejected before recipe instantiation", func(t *testing.T) {
		input := completeCompilerInputForTest(t)
		input.PageSpec.Table.Columns[2].StatusMap["Pending"] = "default"

		output := CompileListPage(input)
		if output.Status != compileStatusFailed {
			t.Fatalf("status = %q, diagnostics = %+v", output.Status, output.Diagnostics)
		}
		assertDiagnosticCode(t, output.Diagnostics, "unsupported_status_variant")
		if len(output.Manifest.GeneratedLayerIDs) != 0 || len(generatedLayerIDs(output.Document)) != 0 {
			t.Fatalf("invalid PageSpec instantiated recipes: manifest=%v", output.Manifest.GeneratedLayerIDs)
		}
	})
}

func TestCompileListPageResolvesPrimitiveTokenAliasesRecursively(t *testing.T) {
	t.Run("alias chain", func(t *testing.T) {
		input := completeCompilerInputForTest(t)
		forcePrimitiveRecipeForTest(&input, "select")
		input.RecipeSet.Tokens["alias"] = map[string]any{
			"control": "$alias.fill",
			"fill":    "$color.primary",
		}
		input.RecipeDoc.Tokens = cloneJSONMap(input.RecipeSet.Tokens)
		primitive := input.RecipeSet.PrimitiveFallbacks["select"]
		primitive.Style = map[string]any{"fill": "$alias.control"}
		input.RecipeSet.PrimitiveFallbacks["select"] = primitive
		before := mustMarshalCompilerTest(t, input)

		output := CompileListPage(input)
		if output.Diagnostics.HasErrors() {
			t.Fatalf("diagnostics: %+v", output.Diagnostics)
		}
		root := generatedRootAtSpecPath(t, output.Document, "filters.status")
		if root.Style["fill"] != "#1677ff" || containsTokenReference(root.Style) {
			t.Fatalf("resolved style = %#v", root.Style)
		}
		if after := mustMarshalCompilerTest(t, input); !reflect.DeepEqual(before, after) {
			t.Fatal("alias resolution mutated CompileInput tokens")
		}
	})

	t.Run("nested token objects and arrays", func(t *testing.T) {
		input := completeCompilerInputForTest(t)
		forcePrimitiveRecipeForTest(&input, "select")
		input.RecipeSet.Tokens["component"] = map[string]any{
			"style": map[string]any{
				"border":  map[string]any{"color": "$color.text", "width": 1.0},
				"shadows": []any{"$color.primary", map[string]any{"color": "$color.text", "opacity": 0.5}},
			},
		}
		input.RecipeDoc.Tokens = cloneJSONMap(input.RecipeSet.Tokens)
		primitive := input.RecipeSet.PrimitiveFallbacks["select"]
		primitive.Style = map[string]any{"theme": "$component.style"}
		input.RecipeSet.PrimitiveFallbacks["select"] = primitive
		before := mustMarshalCompilerTest(t, input)

		output := CompileListPage(input)
		if output.Diagnostics.HasErrors() {
			t.Fatalf("diagnostics: %+v", output.Diagnostics)
		}
		root := generatedRootAtSpecPath(t, output.Document, "filters.status")
		if containsTokenReference(root.Style) {
			t.Fatalf("resolved style retained token reference: %#v", root.Style)
		}
		theme, ok := root.Style["theme"].(map[string]any)
		if !ok {
			t.Fatalf("theme = %#v", root.Style["theme"])
		}
		border := theme["border"].(map[string]any)
		shadows := theme["shadows"].([]any)
		if border["color"] != "#111111" || border["width"] != float64(1) || shadows[0] != "#1677ff" {
			t.Fatalf("nested resolved style = %#v", theme)
		}
		if after := mustMarshalCompilerTest(t, input); !reflect.DeepEqual(before, after) {
			t.Fatal("nested token resolution mutated CompileInput tokens")
		}
	})

	t.Run("missing alias", func(t *testing.T) {
		input := completeCompilerInputForTest(t)
		forcePrimitiveRecipeForTest(&input, "select")
		input.RecipeSet.Tokens["alias"] = "$missing.token"
		input.RecipeDoc.Tokens = cloneJSONMap(input.RecipeSet.Tokens)
		primitive := input.RecipeSet.PrimitiveFallbacks["select"]
		primitive.Style = map[string]any{"fill": "$alias"}
		input.RecipeSet.PrimitiveFallbacks["select"] = primitive

		output := CompileListPage(input)
		if output.Status != compileStatusFailed {
			t.Fatalf("status = %q", output.Status)
		}
		assertDiagnosticCode(t, output.Diagnostics, "primitive_token_missing")
	})

	t.Run("alias cycle", func(t *testing.T) {
		input := completeCompilerInputForTest(t)
		forcePrimitiveRecipeForTest(&input, "select")
		input.RecipeSet.Tokens["alias"] = map[string]any{"a": "$alias.b", "b": "$alias.a"}
		input.RecipeDoc.Tokens = cloneJSONMap(input.RecipeSet.Tokens)
		primitive := input.RecipeSet.PrimitiveFallbacks["select"]
		primitive.Style = map[string]any{"fill": "$alias.a"}
		input.RecipeSet.PrimitiveFallbacks["select"] = primitive

		output := CompileListPage(input)
		if output.Status != compileStatusFailed {
			t.Fatalf("status = %q", output.Status)
		}
		assertDiagnosticCode(t, output.Diagnostics, "primitive_token_cycle")
	})
}

func TestCompileListPageRejectsMissingContractsAndInvalidSources(t *testing.T) {
	tests := []struct {
		name   string
		mutate func(*CompileInput)
		code   string
	}{
		{name: "unsupported page", mutate: func(input *CompileInput) { input.PageSpec.Page.Type = "detail" }, code: "unsupported_page_type"},
		{name: "status mapping", mutate: func(input *CompileInput) { delete(input.PageSpec.Table.Columns[2].StatusMap, "Pending") }, code: "missing_status_mapping"},
		{name: "navigation binding", mutate: func(input *CompileInput) {
			region := input.Blueprint.Regions["navigation"]
			region.Bindings = nil
			input.Blueprint.Regions["navigation"] = region
		}, code: "missing_navigation_binding"},
		{name: "recipe prop", mutate: func(input *CompileInput) {
			key := (RecipeKey{Kind: "status-tag", Variant: "warning", State: "default"}).String()
			recipe := input.RecipeSet.Recipes[key]
			delete(recipe.Props, "value")
			input.RecipeSet.Recipes[key] = recipe
		}, code: "missing_recipe_prop"},
		{name: "status recipe", mutate: func(input *CompileInput) {
			for key, recipe := range input.RecipeSet.Recipes {
				if recipe.Kind == "status-tag" {
					delete(input.RecipeSet.Recipes, key)
				}
			}
			info := compilerRecipeForTest(input.RecipeDoc, "status-tag", "info", "default", "status-tag-info", "value")
			input.RecipeSet.Recipes[(RecipeKey{Kind: "status-tag", Variant: "info", State: "default"}).String()] = info
			delete(input.RecipeSet.PrimitiveFallbacks, "status-tag")
		}, code: "missing_recipe"},
		{name: "invalid recipe document", mutate: func(input *CompileInput) { input.RecipeDoc.File.Title = "" }, code: "invalid_recipe_document"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			input := completeCompilerInputForTest(t)
			tt.mutate(&input)
			output := CompileListPage(input)
			if output.Status != "compile_failed" || !output.Diagnostics.HasErrors() {
				t.Fatalf("output = %+v", output)
			}
			assertDiagnosticCode(t, output.Diagnostics, tt.code)
			if tt.name == "invalid recipe document" && !ValidateDocument(output.Document).Valid {
				t.Fatalf("retained template is invalid: %+v", ValidateDocument(output.Document))
			}
			if tt.name == "status recipe" {
				generatedRootAtSpecPath(t, output.Document, "page.title")
			}
		})
	}
}

func TestCompileListPageRequiresExplicitRequirementCoverage(t *testing.T) {
	tests := []struct {
		name   string
		mutate func(*CompileInput)
		code   string
	}{
		{
			name: "missing required ID set",
			mutate: func(input *CompileInput) {
				input.RequiredRequirementIDs = nil
			},
			code: "missing_required_requirement_ids",
		},
		{
			name: "missing coverage",
			mutate: func(input *CompileInput) {
				input.PageSpec.RequirementCoverage = nil
			},
			code: "missing_requirement_coverage",
		},
		{
			name: "duplicate required ID",
			mutate: func(input *CompileInput) {
				input.RequiredRequirementIDs = []string{"REQ-LIST-PAGE", "REQ-LIST-PAGE"}
			},
			code: "duplicate_required_requirement_id",
		},
		{
			name: "blank required ID",
			mutate: func(input *CompileInput) {
				input.RequiredRequirementIDs = []string{"REQ-LIST-PAGE", "  "}
			},
			code: "blank_required_requirement_id",
		},
		{
			name: "invalid coverage path",
			mutate: func(input *CompileInput) {
				input.PageSpec.RequirementCoverage[0].SpecPaths = []string{"table.columns.invented"}
			},
			code: "invalid_spec_path",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			input := completeCompilerInputForTest(t)
			tt.mutate(&input)
			before := mustMarshalCompilerTest(t, input)
			output := CompileListPage(input)
			if output.Status != compileStatusFailed || !output.Diagnostics.HasErrors() {
				t.Fatalf("output = %+v", output)
			}
			assertDiagnosticCode(t, output.Diagnostics, tt.code)
			if after := mustMarshalCompilerTest(t, input); !reflect.DeepEqual(before, after) {
				t.Fatal("requirement validation mutated CompileInput")
			}
		})
	}
}

func TestCompileListPageValidatesOptionalNativeSourceIdentity(t *testing.T) {
	tests := []struct {
		name    string
		mutate  func(*CompileInput)
		code    string
		success bool
	}{
		{name: "present identities match", mutate: func(*CompileInput) {}, success: true},
		{name: "missing identities are allowed", mutate: func(input *CompileInput) {
			delete(input.TemplateDoc.Source, "revisionId")
			delete(input.RecipeDoc.Source, "revisionId")
		}, success: true},
		{name: "empty identities are allowed", mutate: func(input *CompileInput) {
			input.TemplateDoc.Source["revisionId"] = ""
			input.RecipeDoc.Source["revisionId"] = ""
		}, success: true},
		{name: "template mismatch", mutate: func(input *CompileInput) {
			input.TemplateDoc.Source["revisionId"] = "wrong-design-revision"
		}, code: "template_source_identity_mismatch"},
		{name: "template wrong type", mutate: func(input *CompileInput) {
			input.TemplateDoc.Source["revisionId"] = 7.0
		}, code: "invalid_template_source_identity"},
		{name: "recipe mismatch", mutate: func(input *CompileInput) {
			input.RecipeDoc.Source["revisionId"] = "wrong-ui-spec-revision"
		}, code: "recipe_source_identity_mismatch"},
		{name: "recipe wrong type", mutate: func(input *CompileInput) {
			input.RecipeDoc.Source["revisionId"] = []any{"ui-spec-revision-9"}
		}, code: "invalid_recipe_source_identity"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			input := completeCompilerInputForTest(t)
			tt.mutate(&input)
			before := mustMarshalCompilerTest(t, input)
			output := CompileListPage(input)
			if tt.success {
				if output.Diagnostics.HasErrors() || output.Status != compileStatusCompiled {
					t.Fatalf("output = %+v", output)
				}
			} else {
				if output.Status != compileStatusFailed {
					t.Fatalf("status = %q", output.Status)
				}
				assertDiagnosticCode(t, output.Diagnostics, tt.code)
			}
			if after := mustMarshalCompilerTest(t, input); !reflect.DeepEqual(before, after) {
				t.Fatal("source identity validation mutated CompileInput")
			}
		})
	}
}

func TestCompileListPageEarlyInvalidInputRetainsDetachedValidTemplate(t *testing.T) {
	input := completeCompilerInputForTest(t)
	input.RequiredRequirementIDs = nil
	output := CompileListPage(input)
	if output.Status != compileStatusFailed {
		t.Fatalf("status = %q", output.Status)
	}
	if validation := ValidateDocument(output.Document); !validation.Valid {
		t.Fatalf("retained document is invalid: %v", validation.Errors)
	}
	if !reflect.DeepEqual(mustMarshalCompilerTest(t, output.Document), mustMarshalCompilerTest(t, input.TemplateDoc)) {
		t.Fatal("early failure did not retain the detached template")
	}
	output.Document.File.Title = "mutated retained output"
	if input.TemplateDoc.File.Title == output.Document.File.Title {
		t.Fatal("retained failure document aliases TemplateDoc")
	}
}

func TestCompileListPageClearsBusinessTextResidueAndProtectsShell(t *testing.T) {
	input := completeCompilerInputForTest(t)
	wantTexts := []string{
		"STALE breadcrumb", "STALE filters", "STALE page actions", "STALE page title", "STALE pagination", "STALE table",
	}
	shellBefore := map[string]Layer{}
	for _, id := range input.Blueprint.ShellAllowlistLayerIDs {
		shellBefore[id] = input.TemplateDoc.Layers[id]
	}

	output := CompileListPage(input)
	if output.Diagnostics.HasErrors() {
		t.Fatalf("diagnostics: %+v", output.Diagnostics)
	}
	if !reflect.DeepEqual(output.Manifest.TemplateBusinessTexts, wantTexts) {
		t.Fatalf("template business texts = %v, want %v", output.Manifest.TemplateBusinessTexts, wantTexts)
	}
	for _, stale := range wantTexts {
		if documentContainsText(output.Document, stale) {
			t.Errorf("retained cleared template text %q", stale)
		}
	}
	for id, before := range shellBefore {
		after := output.Document.Layers[id]
		if before.X != after.X || before.Y != after.Y || before.Width != after.Width || before.Height != after.Height || !reflect.DeepEqual(before.Children, after.Children) {
			t.Errorf("allowlisted shell layer %s changed geometry/tree: before=%+v after=%+v", id, before, after)
		}
	}
	labelID := input.Blueprint.Regions["navigation"].Bindings["label"]
	label := output.Document.Layers[labelID]
	if label.Text["characters"] != input.PageSpec.Page.ActiveNavigation || label.Semantic["active"] != true {
		t.Fatalf("active navigation target = %+v", label)
	}
	for index := range input.PageSpec.Page.Breadcrumb {
		root := generatedRootAtSpecPath(t, output.Document, fmt.Sprintf("page.breadcrumb.%d", index))
		if len(root.Children) != 1 {
			t.Fatalf("breadcrumb %d children = %v", index, root.Children)
		}
		text := output.Document.Layers[root.Children[0]]
		if text.X != root.X || text.Y != root.Y || text.Width != root.Width || text.Height != root.Height {
			t.Errorf("breadcrumb %d label bounds = %+v, root bounds = %+v", index, text, root)
		}
	}
}

func TestCompileListPagePinsOnlyWhenHorizontalScrollIsActive(t *testing.T) {
	wide := completeCompilerInputForTest(t)
	wide.PageSpec.Filters = nil
	wide.PageSpec.PageActions = nil
	wide.Blueprint.Constraints.ContentWidth = 300
	wide.Blueprint.Constraints.PinFirstColumn = true
	wide.Blueprint.Constraints.PinActionColumn = true
	wideOutput := CompileListPage(wide)
	if wideOutput.Diagnostics.HasErrors() || !wideOutput.Manifest.HorizontalScroll {
		t.Fatalf("wide output: manifest=%+v diagnostics=%+v", wideOutput.Manifest, wideOutput.Diagnostics)
	}
	if countPinned(wideOutput.Document, "left") == 0 || countPinned(wideOutput.Document, "right") == 0 {
		t.Fatalf("pinned generated roots are incomplete")
	}
	table := wideOutput.Document.Layers[wide.Blueprint.Regions["table"].RootLayerID]
	if table.Semantic["horizontalScroll"] != true || table.Semantic["clipContent"] != true {
		t.Fatalf("table scroll metadata = %+v", table.Semantic)
	}

	fitting := completeCompilerInputForTest(t)
	fitting.Blueprint.Constraints.ContentWidth = 900
	fitting.Blueprint.Constraints.PinFirstColumn = true
	fitting.Blueprint.Constraints.PinActionColumn = true
	fittingOutput := CompileListPage(fitting)
	if fittingOutput.Diagnostics.HasErrors() || fittingOutput.Manifest.HorizontalScroll {
		t.Fatalf("fitting output: manifest=%+v diagnostics=%+v", fittingOutput.Manifest, fittingOutput.Diagnostics)
	}
	if countPinned(fittingOutput.Document, "left") != 0 || countPinned(fittingOutput.Document, "right") != 0 {
		t.Fatal("pin metadata was emitted without horizontal scrolling")
	}
}

func TestCompileListPageRecordsRecipeDeclaredOverlay(t *testing.T) {
	input := completeCompilerInputForTest(t)
	key := (RecipeKey{Kind: "input", Variant: "default", State: "default"}).String()
	recipe := input.RecipeSet.Recipes[key]
	recipe.Layout.OverlayRole = "dropdown"
	input.RecipeSet.Recipes[key] = recipe

	output := CompileListPage(input)
	if output.Diagnostics.HasErrors() {
		t.Fatalf("diagnostics: %+v", output.Diagnostics)
	}
	root := generatedRootAtSpecPath(t, output.Document, "filters.name")
	for _, expectation := range output.Manifest.ResolvedComponents {
		if expectation.GeneratedRootLayerID == root.ID {
			if expectation.OverlayRole != "dropdown" {
				t.Fatalf("overlay expectation = %+v", expectation)
			}
			return
		}
	}
	t.Fatalf("resolved expectation for %q not found", root.ID)
}

func TestCompileListPageWritesCompleteStableManifestAndProvenance(t *testing.T) {
	input := completeCompilerInputForTest(t)
	output := CompileListPage(input)
	if output.Diagnostics.HasErrors() {
		t.Fatalf("diagnostics: %+v", output.Diagnostics)
	}

	generated := generatedLayerIDs(output.Document)
	if !reflect.DeepEqual(output.Manifest.GeneratedLayerIDs, generated) || !sortedUnique(output.Manifest.GeneratedLayerIDs) {
		t.Fatalf("generated IDs = %v, want %v", output.Manifest.GeneratedLayerIDs, generated)
	}
	wantRegions := []string{"breadcrumb", "filters", "page-actions", "page-title", "pagination", "table"}
	if !reflect.DeepEqual(output.Manifest.BusinessRegionLayerIDs, wantRegions) || !sortedUnique(output.Manifest.BusinessRegionLayerIDs) {
		t.Fatalf("business region IDs = %v", output.Manifest.BusinessRegionLayerIDs)
	}
	for _, layer := range output.Document.Layers {
		if !strings.HasPrefix(layer.ID, "gen-") || layer.ParentID == "" {
			continue
		}
		if _, isGeneratedRoot := layer.Semantic["generatedBy"]; isGeneratedRoot {
			if layer.Semantic["generatedBy"] != DesignCompilerVersion || layer.Semantic["generationRole"] == "" || layer.Semantic["specPath"] == "" {
				t.Errorf("generated root metadata = %+v", layer.Semantic)
			}
		}
	}

	generation, ok := output.Document.Source["generation"].(map[string]any)
	if !ok {
		t.Fatalf("generation source = %#v", output.Document.Source["generation"])
	}
	wants := map[string]any{
		"compilerVersion":          DesignCompilerVersion,
		"pageSpecVersion":          input.Provenance.PageSpecVersion,
		"blueprintRecordId":        input.Provenance.BlueprintRecordID,
		"recipeSetRecordId":        input.Provenance.RecipeSetRecordID,
		"templateSourceRevisionId": input.Blueprint.SourceRefs.TemplateRevisionID,
		"designSourceRevisionId":   input.Blueprint.SourceRefs.DesignRevisionID,
		"uiSpecSourceRevisionId":   input.RecipeSet.SourceRevisionID,
		"workspaceId":              input.Provenance.WorkspaceID,
		"projectId":                input.Provenance.ProjectID,
		"issueId":                  input.Provenance.IssueID,
		"agentTaskId":              input.Provenance.AgentTaskID,
		"horizontalScrollStrategy": "none",
	}
	for key, want := range wants {
		if got := generation[key]; got != want {
			t.Errorf("generation.%s = %#v, want %#v", key, got, want)
		}
	}
}

func TestCompileListPageNeverMutatesInputsAndRetainsDetachedFailureDocument(t *testing.T) {
	for _, failure := range []bool{false, true} {
		input := completeCompilerInputForTest(t)
		if failure {
			input.PageSpec.Page.Type = "detail"
		}
		before := mustMarshalCompilerTest(t, input)
		output := CompileListPage(input)
		if after := mustMarshalCompilerTest(t, input); !reflect.DeepEqual(after, before) {
			t.Fatalf("failure=%v input mutated\nbefore=%s\nafter=%s", failure, before, after)
		}
		if failure {
			if output.Status != "compile_failed" || output.Document.File.Title != input.TemplateDoc.File.Title {
				t.Fatalf("failure output = %+v", output)
			}
			output.Document.File.Title = "mutated output"
			if input.TemplateDoc.File.Title == "mutated output" {
				t.Fatal("failure document aliases the template input")
			}
		}
	}
}

func TestCompileListPageIsByteDeterministicAndAllReferencesValidate(t *testing.T) {
	input := completeCompilerInputForTest(t)
	first := CompileListPage(input)
	second := CompileListPage(input)
	if first.Diagnostics.HasErrors() || second.Diagnostics.HasErrors() {
		t.Fatalf("diagnostics: first=%+v second=%+v", first.Diagnostics, second.Diagnostics)
	}
	firstRaw := mustMarshalCompilerTest(t, first)
	secondRaw := mustMarshalCompilerTest(t, second)
	if !reflect.DeepEqual(firstRaw, secondRaw) {
		t.Fatalf("outputs differ:\n%s\n%s", firstRaw, secondRaw)
	}
	if result := ValidateDocument(first.Document); !result.Valid {
		t.Fatalf("compiled document is invalid: %v", result.Errors)
	}
	if !sortedUnique(first.Manifest.GeneratedLayerIDs) || !sortedUnique(first.Manifest.BusinessRegionLayerIDs) || !sortedUnique(first.Manifest.TemplateBusinessTexts) {
		t.Fatalf("manifest slices are not stable: %+v", first.Manifest)
	}
	if !diagnosticsSorted(first.Diagnostics) {
		t.Fatalf("diagnostics are not sorted: %+v", first.Diagnostics)
	}
}

func completeCompilerInputForTest(t *testing.T) CompileInput {
	t.Helper()
	template := compilerTemplateDocumentForTest()
	structure := ExtractTemplateStructure(template)
	blueprint, diagnostics := BuildTemplateBlueprint(structure, compilerBlueprintClassificationForTest(), BlueprintSourceRefs{
		DesignFileID:       "design-file-1",
		DesignRevisionID:   "design-revision-7",
		TemplateRevisionID: "template-revision-4",
	})
	if diagnostics.HasErrors() {
		t.Fatalf("blueprint diagnostics: %+v", diagnostics)
	}
	recipeDoc := compilerRecipeDocumentForTest()
	recipeSet, diagnostics := BuildComponentRecipeSet(
		"profile-1",
		"ui-spec-revision-9",
		ComponentRecipeSetVersion,
		recipeDoc,
		compilerRecipeClassificationsForTest(),
		compilerPrimitiveFallbacksForTest(),
	)
	if diagnostics.HasErrors() {
		t.Fatalf("recipe diagnostics: %+v", diagnostics)
	}
	return CompileInput{
		PageSpec: PageSpec{
			Version: PageSpecVersion,
			Page: PageIdentity{
				Type: "list", Module: "customers", Title: "Customers", Breadcrumb: []string{"Home", "Customers"},
				ActiveNavigation: "Customers", Density: "standard",
			},
			Filters: []FilterSpec{
				{Key: "name", Label: "Customer name", Control: "input", Placeholder: "Enter name", Width: "medium"},
				{Key: "status", Label: "Status", Control: "select", Placeholder: "Choose status", Width: "narrow"},
				{Key: "created", Label: "Created", Control: "date-range", Placeholder: "Choose dates", Width: "wide"},
				{Key: "owner", Label: "Owner", Control: "select", Placeholder: "Choose owner", Width: "medium"},
			},
			PageActions: []ActionSpec{
				{Key: "create", Label: "Create customer", Variant: "primary"},
				{Key: "export", Label: "Export", Variant: "secondary"},
			},
			Table: TableSpec{
				Columns: []TableColumnSpec{
					{Key: "customerName", Title: "Customer name", Cell: "text", Width: "wide", Align: "left"},
					{Key: "phone", Title: "Phone", Cell: "text", Width: "medium", Align: "left"},
					{Key: "status", Title: "Status", Cell: "status-tag", Width: "narrow", Align: "center", StatusMap: map[string]string{"Pending": "warning", "Active": "success"}},
					{Key: "createdAt", Title: "Created at", Cell: "date", Width: "wide", Align: "left"},
				},
				SampleRows: []map[string]string{{
					"customerName": "Example Customer A", "phone": "13800000001", "status": "Pending", "createdAt": "2026-07-22 10:00",
				}},
				RowActions: []ActionSpec{
					{Key: "view", Label: "View", Variant: "text"},
					{Key: "edit", Label: "Edit", Variant: "text"},
				},
			},
			Pagination:          PaginationSpec{Enabled: true, PageSize: 20, SampleTotal: 57},
			RequirementCoverage: []RequirementCoverage{{RequirementID: "REQ-LIST-PAGE", SpecPaths: []string{"page.title"}}},
		},
		RequiredRequirementIDs: []string{"REQ-LIST-PAGE"},
		Blueprint:              blueprint,
		RecipeSet:              recipeSet,
		TemplateDoc:            template,
		RecipeDoc:              recipeDoc,
		Provenance: CompileProvenance{
			WorkspaceID: "workspace-1", ProjectID: "project-1", IssueID: "issue-1", AgentTaskID: "task-1",
			PageSpecVersion: PageSpecVersion, BlueprintRecordID: "blueprint-record-1", RecipeSetRecordID: "recipe-record-1",
		},
	}
}

func compilerTemplateDocumentForTest() NativeJSON {
	layers := map[string]Layer{}
	add := func(id, parent, layerType string, children []string, x, y, width, height float64) {
		layers[id] = Layer{ID: id, FrameID: "frame-1", ParentID: parent, Children: children, Name: id, Type: layerType, Visible: true, X: x, Y: y, Width: width, Height: height}
	}
	add("frame-root", "", "frame", []string{"shell"}, 0, 0, 1000, 900)
	add("shell", "frame-root", "frame", []string{"navigation", "topbar", "content"}, 0, 0, 1000, 900)
	add("navigation", "shell", "frame", []string{"navigation-label"}, 0, 0, 180, 900)
	add("navigation-label", "navigation", "text", nil, 20, 80, 120, 24)
	add("topbar", "shell", "frame", nil, 180, 0, 820, 64)
	add("content", "shell", "frame", []string{
		"breadcrumb", "page-title", "filters", "page-actions", "table", "pagination",
		"page-title-prototype", "breadcrumb-item-prototype", "table-header-cell-prototype", "table-row-prototype",
	}, 180, 64, 820, 836)
	add("breadcrumb", "content", "frame", []string{"stale-breadcrumb"}, 20, 80, 600, 20)
	add("page-title", "content", "frame", []string{"stale-page-title"}, 20, 108, 600, 40)
	add("filters", "content", "frame", []string{"stale-filters"}, 20, 164, 600, 68)
	add("page-actions", "content", "frame", []string{"stale-page-actions"}, 20, 248, 600, 32)
	add("table", "content", "frame", []string{"stale-table"}, 20, 296, 600, 400)
	add("pagination", "content", "frame", []string{"stale-pagination"}, 20, 712, 600, 32)
	add("stale-breadcrumb", "breadcrumb", "text", nil, 20, 80, 200, 20)
	add("stale-page-title", "page-title", "text", nil, 20, 108, 200, 32)
	add("stale-filters", "filters", "text", nil, 20, 164, 200, 20)
	add("stale-page-actions", "page-actions", "text", nil, 20, 248, 200, 20)
	add("stale-table", "table", "text", nil, 20, 296, 200, 20)
	add("stale-pagination", "pagination", "text", nil, 20, 712, 200, 20)
	add("page-title-prototype", "content", "frame", []string{"page-title-label"}, 20, 108, 320, 40)
	add("page-title-label", "page-title-prototype", "text", nil, 20, 108, 320, 32)
	add("breadcrumb-item-prototype", "content", "frame", []string{"breadcrumb-label"}, 20, 80, 120, 20)
	add("breadcrumb-label", "breadcrumb-item-prototype", "text", nil, 20, 80, 120, 20)
	add("table-header-cell-prototype", "content", "frame", []string{"header-sample"}, 20, 296, 180, 44)
	add("header-sample", "table-header-cell-prototype", "text", nil, 20, 296, 160, 20)
	add("table-row-prototype", "content", "frame", []string{"row-sample"}, 20, 340, 600, 52)
	add("row-sample", "table-row-prototype", "text", nil, 20, 340, 160, 20)

	texts := map[string]string{
		"navigation-label": "Old navigation", "stale-breadcrumb": "STALE breadcrumb", "stale-page-title": "STALE page title",
		"stale-filters": "STALE filters", "stale-page-actions": "STALE page actions", "stale-table": "STALE table",
		"stale-pagination": "STALE pagination", "page-title-label": "Prototype title", "breadcrumb-label": "Prototype breadcrumb",
		"header-sample": "Prototype header", "row-sample": "Prototype row",
	}
	for id, value := range texts {
		layer := layers[id]
		layer.Text = map[string]any{"characters": value}
		layers[id] = layer
	}
	return NativeJSON{
		Version: NativeJSONVersion,
		File:    FileMeta{ID: "template-file", Title: "Semantic list template", SourceType: "template"},
		Frames:  []Frame{{ID: "frame-1", Name: "Desktop", RootLayerID: "frame-root", Width: 1000, Height: 900}},
		Layers:  layers,
		Assets:  map[string]Asset{},
		Source:  map[string]any{"revisionId": "design-revision-7"},
	}
}

func compilerBlueprintClassificationForTest() BlueprintClassification {
	return BlueprintClassification{
		FrameID: "frame-1", PageType: "list",
		Regions: map[string]RegionClassification{
			"shell":       {RootLayerID: "shell"},
			"content":     {RootLayerID: "content"},
			"navigation":  {RootLayerID: "navigation", Bindings: map[string]string{"label": "navigation-label"}},
			"breadcrumb":  {RootLayerID: "breadcrumb", ReplaceChildren: true},
			"pageTitle":   {RootLayerID: "page-title", ReplaceChildren: true},
			"filters":     {RootLayerID: "filters", ReplaceChildren: true},
			"pageActions": {RootLayerID: "page-actions", ReplaceChildren: true},
			"table":       {RootLayerID: "table", ReplaceChildren: true},
			"pagination":  {RootLayerID: "pagination", ReplaceChildren: true},
		},
		Prototypes: map[string]PrototypeClassification{
			"pageTitle":       {RootLayerID: "page-title-prototype", Bindings: map[string]string{"label": "page-title-label"}},
			"breadcrumbItem":  {RootLayerID: "breadcrumb-item-prototype", Bindings: map[string]string{"label": "breadcrumb-label"}},
			"tableHeaderCell": {RootLayerID: "table-header-cell-prototype"},
			"tableRow":        {RootLayerID: "table-row-prototype"},
		},
		Constraints: BlueprintConstraints{
			ContentWidth: 820, FilterRowHeight: 68, TableHeaderHeight: 44, TableRowHeight: 52,
			HorizontalGap: 16, VerticalGap: 16, FilterColumns: 3,
		},
		ShellAllowlistLayerIDs: []string{"navigation", "topbar"},
	}
}

func compilerRecipeDocumentForTest() NativeJSON {
	type recipeDefinition struct {
		kind, variant, root, prop string
	}
	definitions := []recipeDefinition{
		{kind: "input", variant: "default", root: "input-default", prop: "label,value"},
		{kind: "select", variant: "default", root: "select-default", prop: "label,value"},
		{kind: "date-range", variant: "default", root: "date-range-default", prop: "label,value"},
		{kind: "primary-button", variant: "default", root: "primary-button-default", prop: "label"},
		{kind: "secondary-button", variant: "default", root: "secondary-button-default", prop: "label"},
		{kind: "text-button", variant: "default", root: "text-button-default", prop: "label"},
		{kind: "table-header", variant: "default", root: "table-header-default", prop: "label"},
		{kind: "table-row", variant: "default", root: "table-row-default", prop: "value"},
		{kind: "table-row", variant: "alternate", root: "table-row-alternate", prop: "value"},
		{kind: "status-tag", variant: "default", root: "status-tag-default", prop: "value"},
		{kind: "status-tag", variant: "warning", root: "status-tag-warning", prop: "value"},
		{kind: "status-tag", variant: "success", root: "status-tag-success", prop: "value"},
		{kind: "status-tag", variant: "info", root: "status-tag-info", prop: "value"},
		{kind: "pagination", variant: "default", root: "pagination-default", prop: "value"},
	}
	layers := map[string]Layer{}
	children := make([]string, 0, len(definitions))
	for index, definition := range definitions {
		propKeys := strings.Split(definition.prop, ",")
		propChildren := make([]string, 0, len(propKeys))
		for propIndex, prop := range propKeys {
			id := definition.root + "-" + prop
			layers[id] = Layer{
				ID: id, FrameID: "recipe-frame", ParentID: definition.root, Name: id, Type: "text", Visible: true,
				X: 8, Y: float64(index*48 + propIndex*16), Width: 160, Height: 16, Text: map[string]any{"characters": prop},
			}
			propChildren = append(propChildren, id)
		}
		layers[definition.root] = Layer{
			ID: definition.root, FrameID: "recipe-frame", ParentID: "recipe-root", Children: propChildren,
			Name: definition.root, Type: "frame", Visible: true, X: 0, Y: float64(index * 48), Width: 200, Height: 32,
			Style: map[string]any{"sourceVariant": definition.variant},
		}
		children = append(children, definition.root)
	}
	layers["recipe-root"] = Layer{ID: "recipe-root", FrameID: "recipe-frame", Children: children, Name: "recipes", Type: "frame", Visible: true, Width: 1200, Height: 900}
	return NativeJSON{
		Version: NativeJSONVersion,
		File:    FileMeta{ID: "recipe-file", Title: "UI specification recipes", SourceType: "ui-spec"},
		Frames:  []Frame{{ID: "recipe-frame", Name: "Components", RootLayerID: "recipe-root", Width: 1200, Height: 900}},
		Layers:  layers,
		Assets:  map[string]Asset{},
		Tokens: map[string]any{
			"color":   map[string]any{"primary": "#1677ff", "text": "#111111"},
			"spacing": map[string]any{"control": 12.0},
		},
		Source: map[string]any{"revisionId": "ui-spec-revision-9"},
	}
}

func compilerRecipeClassificationsForTest() []ComponentRecipeClassification {
	type definition struct {
		kind, variant, root, props string
	}
	definitions := []definition{
		{"input", "default", "input-default", "label,value"},
		{"select", "default", "select-default", "label,value"},
		{"date-range", "default", "date-range-default", "label,value"},
		{"primary-button", "default", "primary-button-default", "label"},
		{"secondary-button", "default", "secondary-button-default", "label"},
		{"text-button", "default", "text-button-default", "label"},
		{"table-header", "default", "table-header-default", "label"},
		{"table-row", "default", "table-row-default", "value"},
		{"table-row", "alternate", "table-row-alternate", "value"},
		{"status-tag", "default", "status-tag-default", "value"},
		{"status-tag", "warning", "status-tag-warning", "value"},
		{"status-tag", "success", "status-tag-success", "value"},
		{"status-tag", "info", "status-tag-info", "value"},
		{"pagination", "default", "pagination-default", "value"},
	}
	result := make([]ComponentRecipeClassification, 0, len(definitions))
	for _, item := range definitions {
		props := map[string]RecipeProp{}
		for _, prop := range strings.Split(item.props, ",") {
			props[prop] = RecipeProp{TargetLayerID: item.root + "-" + prop, Type: "text"}
		}
		result = append(result, ComponentRecipeClassification{
			Kind: item.kind, Variant: item.variant, State: "default", RootLayerID: item.root, Props: props,
			Layout: RecipeLayout{WidthMode: "fixed", TextOverflow: "ellipsis", Height: 32, MinWidth: 80},
		})
	}
	return result
}

func compilerPrimitiveFallbacksForTest() map[string]PrimitiveRecipe {
	propsByKind := map[string][]string{
		"input": {"label", "value"}, "select": {"label", "value"}, "date-range": {"label", "value"},
		"primary-button": {"label"}, "secondary-button": {"label"}, "text-button": {"label"},
		"table-header": {"label"}, "table-row": {"value"}, "status-tag": {"value"}, "pagination": {"value"},
	}
	result := make(map[string]PrimitiveRecipe, len(propsByKind))
	for _, kind := range requiredRecipeKindsForTest() {
		props := map[string]RecipeProp{}
		for _, prop := range propsByKind[kind] {
			props[prop] = RecipeProp{TargetLayerID: "primitive-" + prop, Type: "text"}
		}
		result[kind] = PrimitiveRecipe{
			Kind: kind, LayerType: "frame", Props: props,
			Style:  map[string]any{"fill": "$color.primary", "spacing": map[string]any{"horizontal": "$spacing.control"}},
			Layout: RecipeLayout{WidthMode: "fixed", TextOverflow: "ellipsis", Height: 32, MinWidth: 80},
		}
	}
	return result
}

func forcePrimitiveRecipeForTest(input *CompileInput, kind string) {
	defaultKey := (RecipeKey{Kind: kind, Variant: "default", State: "default"}).String()
	nonDefault := input.RecipeSet.Recipes[defaultKey]
	nonDefault.Variant = "compact"
	nonDefault.State = "focused"
	input.RecipeSet.Recipes[(RecipeKey{Kind: kind, Variant: "compact", State: "focused"}).String()] = nonDefault
	delete(input.RecipeSet.Recipes, defaultKey)
}

func compilerRecipeForTest(source NativeJSON, kind, variant, state, root, prop string) ComponentRecipe {
	return ComponentRecipe{
		Kind: kind, Variant: variant, State: state,
		Source: RecipeSource{RevisionID: "ui-spec-revision-9", RootLayerID: root, Fingerprint: fingerprintRecipeSource(source, root)},
		Props:  map[string]RecipeProp{prop: {TargetLayerID: root + "-" + prop, Type: "text"}},
		Layout: RecipeLayout{WidthMode: "fixed", TextOverflow: "ellipsis", Height: 32, MinWidth: 80},
	}
}

func countGeneratedRole(doc NativeJSON, role string) int {
	count := 0
	for _, layer := range doc.Layers {
		if layer.Semantic["generatedBy"] == DesignCompilerVersion && layer.Semantic["generationRole"] == role {
			count++
		}
	}
	return count
}

func generatedRootAtSpecPath(t *testing.T, doc NativeJSON, path string) Layer {
	t.Helper()
	var matches []Layer
	for _, layer := range doc.Layers {
		if layer.Semantic["generatedBy"] == DesignCompilerVersion && layer.Semantic["specPath"] == path {
			matches = append(matches, layer)
		}
	}
	if len(matches) != 1 {
		t.Fatalf("generated roots at %q = %d", path, len(matches))
	}
	return matches[0]
}

func subtreeContainsText(doc NativeJSON, rootID, want string) bool {
	visited := map[string]bool{}
	var visit func(string) bool
	visit = func(id string) bool {
		if visited[id] {
			return false
		}
		visited[id] = true
		layer, ok := doc.Layers[id]
		if !ok {
			return false
		}
		if layer.Text["characters"] == want || layer.Text["text"] == want {
			return true
		}
		for _, childID := range layer.Children {
			if visit(childID) {
				return true
			}
		}
		return false
	}
	return visit(rootID)
}

func subtreeHasTextOverflow(doc NativeJSON, rootID, want string) bool {
	visited := map[string]bool{}
	var visit func(string) bool
	visit = func(id string) bool {
		if visited[id] {
			return false
		}
		visited[id] = true
		layer, ok := doc.Layers[id]
		if !ok {
			return false
		}
		if layer.Text["overflow"] == want {
			return true
		}
		for _, childID := range layer.Children {
			if visit(childID) {
				return true
			}
		}
		return false
	}
	return visit(rootID)
}

func documentContainsText(doc NativeJSON, want string) bool {
	for _, layer := range doc.Layers {
		if layer.Text["characters"] == want || layer.Text["text"] == want {
			return true
		}
	}
	return false
}

func containsTokenReference(value any) bool {
	switch typed := value.(type) {
	case string:
		return strings.HasPrefix(typed, "$")
	case map[string]any:
		for _, child := range typed {
			if containsTokenReference(child) {
				return true
			}
		}
	case []any:
		for _, child := range typed {
			if containsTokenReference(child) {
				return true
			}
		}
	}
	return false
}

func generatedLayerIDs(doc NativeJSON) []string {
	result := make([]string, 0)
	for id := range doc.Layers {
		if strings.HasPrefix(id, "gen-") {
			result = append(result, id)
		}
	}
	sort.Strings(result)
	return result
}

func countPinned(doc NativeJSON, value string) int {
	count := 0
	for _, layer := range doc.Layers {
		if layer.Semantic["pinned"] == value {
			count++
		}
	}
	return count
}

func sortedUnique(values []string) bool {
	for index := 1; index < len(values); index++ {
		if values[index-1] >= values[index] {
			return false
		}
	}
	return true
}

func diagnosticsSorted(diagnostics Diagnostics) bool {
	keys := make([]string, len(diagnostics))
	for index, diagnostic := range diagnostics {
		keys[index] = string(diagnostic.Severity) + "\x00" + diagnostic.Code + "\x00" + diagnostic.Message + "\x00" + strings.Join(diagnostic.Paths, "\x00") + "\x00" + strings.Join(diagnostic.LayerIDs, "\x00")
	}
	return sort.StringsAreSorted(keys)
}

func mustMarshalCompilerTest(t *testing.T, value any) []byte {
	t.Helper()
	raw, err := json.Marshal(value)
	if err != nil {
		t.Fatal(err)
	}
	return raw
}
