package handler

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"testing"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgtype"
	"github.com/multica-ai/multica/server/internal/designcore"
	"github.com/multica-ai/multica/server/internal/service"
	db "github.com/multica-ai/multica/server/pkg/db/generated"
)

type generationAssetFixture struct {
	WorkspaceID              pgtype.UUID
	UserID                   pgtype.UUID
	TemplateID               pgtype.UUID
	TemplateLibraryID        pgtype.UUID
	TemplateRevisionID       pgtype.UUID
	TemplateSourceFileID     pgtype.UUID
	TemplateSourceRevisionID pgtype.UUID
	DesignSystemProfileID    pgtype.UUID
	RecipeSourceFileID       pgtype.UUID
	RecipeSourceRevisionID   pgtype.UUID
	Structure                designcore.TemplateStructure
	Blueprint                designcore.TemplateBlueprint
	RecipeSet                designcore.ComponentRecipeSet
	RecipeDoc                designcore.NativeJSON
}

func TestDesignGenerationAssetStoreLoadsLatestValidVersions(t *testing.T) {
	ctx := context.Background()
	store := service.DesignGenerationAssetStore{Queries: db.New(testPool)}
	fixture := createGenerationAssetFixture(t)

	blueprintRecord, err := store.SaveBlueprintAnalysis(ctx, service.SaveBlueprintAnalysisParams{
		WorkspaceID: fixture.WorkspaceID, TemplateID: fixture.TemplateID,
		TemplateRevisionID: fixture.TemplateRevisionID, SourceRevisionID: fixture.TemplateSourceRevisionID,
		AnalysisVersion: 1, CreatedBy: fixture.UserID, Structure: fixture.Structure, Blueprint: fixture.Blueprint,
	})
	if err != nil {
		t.Fatalf("save blueprint: %v", err)
	}
	recipeRecord, err := store.SaveRecipeSetAnalysis(ctx, service.SaveRecipeSetAnalysisParams{
		WorkspaceID: fixture.WorkspaceID, DesignSystemProfileID: fixture.DesignSystemProfileID,
		SourceRevisionID: fixture.RecipeSourceRevisionID, AnalysisVersion: 1, CreatedBy: fixture.UserID, RecipeSet: fixture.RecipeSet,
	})
	if err != nil {
		t.Fatalf("save recipes: %v", err)
	}

	invalidBlueprint := fixture.Blueprint
	invalidBlueprint.Constraints.ContentWidth = 0
	invalidBlueprintRecord, err := store.SaveBlueprintAnalysis(ctx, service.SaveBlueprintAnalysisParams{
		WorkspaceID: fixture.WorkspaceID, TemplateID: fixture.TemplateID,
		TemplateRevisionID: fixture.TemplateRevisionID, SourceRevisionID: fixture.TemplateSourceRevisionID,
		AnalysisVersion: 2, CreatedBy: fixture.UserID, Structure: fixture.Structure, Blueprint: invalidBlueprint,
	})
	assertGenerationAssetValidationError(t, err)
	if invalidBlueprintRecord.Status != "invalid" {
		t.Fatalf("invalid blueprint record = %+v", invalidBlueprintRecord)
	}
	assertPersistedDiagnostic(t, invalidBlueprintRecord.ValidationErrors, "invalid_constraint", 1)

	invalidRecipeSet := fixture.RecipeSet
	invalidRecipe := invalidRecipeSet.Recipes["input/default/default"]
	invalidRecipe.Source.Fingerprint = "stale-fingerprint"
	invalidRecipeSet.Recipes["input/default/default"] = invalidRecipe
	invalidRecipeRecord, err := store.SaveRecipeSetAnalysis(ctx, service.SaveRecipeSetAnalysisParams{
		WorkspaceID: fixture.WorkspaceID, DesignSystemProfileID: fixture.DesignSystemProfileID,
		SourceRevisionID: fixture.RecipeSourceRevisionID, AnalysisVersion: 2, CreatedBy: fixture.UserID, RecipeSet: invalidRecipeSet,
	})
	assertGenerationAssetValidationError(t, err)
	if invalidRecipeRecord.Status != "invalid" {
		t.Fatalf("invalid recipe record = %+v", invalidRecipeRecord)
	}
	assertPersistedDiagnostic(t, invalidRecipeRecord.ValidationErrors, "recipe_fingerprint_drift", 1)

	assets, err := store.LoadCompilationAssets(ctx, service.LoadCompilationAssetsParams{
		WorkspaceID: fixture.WorkspaceID, TemplateRevisionID: fixture.TemplateRevisionID,
		DesignSystemProfileID: fixture.DesignSystemProfileID,
	})
	if err != nil {
		t.Fatalf("load assets: %v", err)
	}
	if assets.Blueprint.Version != designcore.TemplateBlueprintVersion || assets.RecipeSet.Version != designcore.ComponentRecipeSetVersion {
		t.Fatalf("unexpected assets: %+v", assets)
	}
	if assets.BlueprintRecordID != uuidToString(blueprintRecord.ID) || assets.RecipeSetRecordID != uuidToString(recipeRecord.ID) {
		t.Fatalf("loaded records = %q, %q", assets.BlueprintRecordID, assets.RecipeSetRecordID)
	}
}

func TestDesignGenerationAssetStoreRejectsStaleAssets(t *testing.T) {
	t.Run("source revision identity", func(t *testing.T) {
		ctx := context.Background()
		store := service.DesignGenerationAssetStore{Queries: db.New(testPool)}
		fixture := createGenerationAssetFixture(t)
		saveGenerationAssetsForTest(t, ctx, store, fixture)

		if _, err := testPool.Exec(ctx, `UPDATE design_system_profile SET source_revision_id = $1 WHERE id = $2`, fixture.TemplateSourceRevisionID, fixture.DesignSystemProfileID); err != nil {
			t.Fatalf("change profile source revision: %v", err)
		}
		_, err := store.LoadCompilationAssets(ctx, service.LoadCompilationAssetsParams{
			WorkspaceID: fixture.WorkspaceID, TemplateRevisionID: fixture.TemplateRevisionID,
			DesignSystemProfileID: fixture.DesignSystemProfileID,
		})
		if !errors.Is(err, service.ErrGenerationAssetsStale) {
			t.Fatalf("load stale identities error = %v", err)
		}
	})

	t.Run("recipe fingerprint", func(t *testing.T) {
		ctx := context.Background()
		store := service.DesignGenerationAssetStore{Queries: db.New(testPool)}
		fixture := createGenerationAssetFixture(t)
		saveGenerationAssetsForTest(t, ctx, store, fixture)

		input := fixture.RecipeDoc.Layers["input"]
		input.Name = "input changed after analysis"
		fixture.RecipeDoc.Layers["input"] = input
		updateGenerationAssetNativeJSON(t, fixture.RecipeSourceRevisionID, fixture.RecipeDoc)

		_, err := store.LoadCompilationAssets(ctx, service.LoadCompilationAssetsParams{
			WorkspaceID: fixture.WorkspaceID, TemplateRevisionID: fixture.TemplateRevisionID,
			DesignSystemProfileID: fixture.DesignSystemProfileID,
		})
		if !errors.Is(err, service.ErrGenerationAssetsStale) {
			t.Fatalf("load stale fingerprint error = %v", err)
		}
	})
}

func TestDesignGenerationAssetStoreRejectsMismatchedRelationships(t *testing.T) {
	t.Run("save rejects profile source file mismatch", func(t *testing.T) {
		ctx := context.Background()
		store := service.DesignGenerationAssetStore{Queries: db.New(testPool)}
		fixture := createGenerationAssetFixture(t)

		if _, err := testPool.Exec(ctx, `UPDATE design_system_profile SET source_file_id = $1 WHERE id = $2`, fixture.TemplateSourceFileID, fixture.DesignSystemProfileID); err != nil {
			t.Fatalf("change profile source file: %v", err)
		}
		_, err := store.SaveRecipeSetAnalysis(ctx, service.SaveRecipeSetAnalysisParams{
			WorkspaceID: fixture.WorkspaceID, DesignSystemProfileID: fixture.DesignSystemProfileID,
			SourceRevisionID: fixture.RecipeSourceRevisionID, AnalysisVersion: 1, CreatedBy: fixture.UserID, RecipeSet: fixture.RecipeSet,
		})
		if !errors.Is(err, service.ErrGenerationAssetsStale) {
			t.Fatalf("save profile source file mismatch error = %v", err)
		}
	})

	t.Run("load rejects persisted blueprint template mismatch", func(t *testing.T) {
		ctx := context.Background()
		store := service.DesignGenerationAssetStore{Queries: db.New(testPool)}
		fixture := createGenerationAssetFixture(t)
		saveGenerationAssetsForTest(t, ctx, store, fixture)
		mismatchTemplateID := createGenerationAssetCatalogTemplate(t, fixture)

		if _, err := testPool.Exec(ctx, `UPDATE design_template_blueprint SET template_id = $1 WHERE template_revision_id = $2`, mismatchTemplateID, fixture.TemplateRevisionID); err != nil {
			t.Fatalf("change persisted blueprint template: %v", err)
		}
		_, err := store.LoadCompilationAssets(ctx, service.LoadCompilationAssetsParams{
			WorkspaceID: fixture.WorkspaceID, TemplateRevisionID: fixture.TemplateRevisionID,
			DesignSystemProfileID: fixture.DesignSystemProfileID,
		})
		if !errors.Is(err, service.ErrGenerationAssetsStale) {
			t.Fatalf("load blueprint template mismatch error = %v", err)
		}
	})

	t.Run("load rejects profile source file mismatch", func(t *testing.T) {
		ctx := context.Background()
		store := service.DesignGenerationAssetStore{Queries: db.New(testPool)}
		fixture := createGenerationAssetFixture(t)
		saveGenerationAssetsForTest(t, ctx, store, fixture)

		if _, err := testPool.Exec(ctx, `UPDATE design_system_profile SET source_file_id = $1 WHERE id = $2`, fixture.TemplateSourceFileID, fixture.DesignSystemProfileID); err != nil {
			t.Fatalf("change profile source file: %v", err)
		}
		_, err := store.LoadCompilationAssets(ctx, service.LoadCompilationAssetsParams{
			WorkspaceID: fixture.WorkspaceID, TemplateRevisionID: fixture.TemplateRevisionID,
			DesignSystemProfileID: fixture.DesignSystemProfileID,
		})
		if !errors.Is(err, service.ErrGenerationAssetsStale) {
			t.Fatalf("load profile source file mismatch error = %v", err)
		}
	})
}

func TestDesignGenerationAssetStoreMissingAndCrossWorkspace(t *testing.T) {
	t.Run("missing assets", func(t *testing.T) {
		ctx := context.Background()
		store := service.DesignGenerationAssetStore{Queries: db.New(testPool)}
		fixture := createGenerationAssetFixture(t)

		_, err := store.LoadCompilationAssets(ctx, service.LoadCompilationAssetsParams{
			WorkspaceID: fixture.WorkspaceID, TemplateRevisionID: fixture.TemplateRevisionID,
			DesignSystemProfileID: fixture.DesignSystemProfileID,
		})
		if !errors.Is(err, service.ErrGenerationAssetsMissing) {
			t.Fatalf("load missing assets error = %v", err)
		}
	})

	t.Run("cross workspace", func(t *testing.T) {
		ctx := context.Background()
		store := service.DesignGenerationAssetStore{Queries: db.New(testPool)}
		fixture := createGenerationAssetFixture(t)
		saveGenerationAssetsForTest(t, ctx, store, fixture)
		otherWorkspaceID := createGenerationAssetWorkspace(t)

		_, err := store.LoadCompilationAssets(ctx, service.LoadCompilationAssetsParams{
			WorkspaceID: otherWorkspaceID, TemplateRevisionID: fixture.TemplateRevisionID,
			DesignSystemProfileID: fixture.DesignSystemProfileID,
		})
		if !errors.Is(err, service.ErrGenerationAssetsMissing) {
			t.Fatalf("cross-workspace load error = %v", err)
		}
		_, err = store.SaveBlueprintAnalysis(ctx, service.SaveBlueprintAnalysisParams{
			WorkspaceID: otherWorkspaceID, TemplateID: fixture.TemplateID,
			TemplateRevisionID: fixture.TemplateRevisionID, SourceRevisionID: fixture.TemplateSourceRevisionID,
			AnalysisVersion: 2, CreatedBy: fixture.UserID, Structure: fixture.Structure, Blueprint: fixture.Blueprint,
		})
		if !errors.Is(err, pgx.ErrNoRows) {
			t.Fatalf("cross-workspace save error = %v", err)
		}
	})
}

func assertGenerationAssetValidationError(t *testing.T, err error) {
	t.Helper()
	var validationErr *service.GenerationAssetValidationError
	if !errors.As(err, &validationErr) || len(validationErr.Diagnostics) == 0 {
		t.Fatalf("expected typed validation error with diagnostics, got %v", err)
	}
}

func assertPersistedDiagnostic(t *testing.T, raw []byte, code string, wantCount int) {
	t.Helper()
	var diagnostics designcore.Diagnostics
	if err := json.Unmarshal(raw, &diagnostics); err != nil {
		t.Fatalf("decode persisted diagnostics: %v", err)
	}
	count := 0
	for _, diagnostic := range diagnostics {
		if diagnostic.Code == code {
			count++
		}
	}
	if count != wantCount {
		t.Fatalf("diagnostic %q count = %d, want %d; diagnostics = %+v", code, count, wantCount, diagnostics)
	}
}

func saveGenerationAssetsForTest(t *testing.T, ctx context.Context, store service.DesignGenerationAssetStore, fixture generationAssetFixture) {
	t.Helper()
	if _, err := store.SaveBlueprintAnalysis(ctx, service.SaveBlueprintAnalysisParams{
		WorkspaceID: fixture.WorkspaceID, TemplateID: fixture.TemplateID,
		TemplateRevisionID: fixture.TemplateRevisionID, SourceRevisionID: fixture.TemplateSourceRevisionID,
		AnalysisVersion: 1, CreatedBy: fixture.UserID, Structure: fixture.Structure, Blueprint: fixture.Blueprint,
	}); err != nil {
		t.Fatalf("save blueprint: %v", err)
	}
	if _, err := store.SaveRecipeSetAnalysis(ctx, service.SaveRecipeSetAnalysisParams{
		WorkspaceID: fixture.WorkspaceID, DesignSystemProfileID: fixture.DesignSystemProfileID,
		SourceRevisionID: fixture.RecipeSourceRevisionID, AnalysisVersion: 1, CreatedBy: fixture.UserID, RecipeSet: fixture.RecipeSet,
	}); err != nil {
		t.Fatalf("save recipe set: %v", err)
	}
}

func createGenerationAssetFixture(t *testing.T) generationAssetFixture {
	t.Helper()
	ctx := context.Background()
	queries := db.New(testPool)
	workspaceID := parseUUID(testWorkspaceID)
	userID := parseUUID(testUserID)

	templateDesign := createDesignFileForTest(t, "Generation asset template")
	if templateDesign.CurrentRevision == nil {
		t.Fatal("template design has no revision")
	}
	templateDoc := generationAssetTemplateDocument()
	updateGenerationAssetNativeJSON(t, parseUUID(templateDesign.CurrentRevision.ID), templateDoc)

	library, err := queries.EnsureDesignTemplateLibrary(ctx, db.EnsureDesignTemplateLibraryParams{
		WorkspaceID: workspaceID, Key: fmt.Sprintf("generation-assets-%d", time.Now().UnixNano()), Name: "Generation Assets", Metadata: []byte(`{}`), CreatedBy: userID,
	})
	if err != nil {
		t.Fatalf("create template library: %v", err)
	}
	template, err := queries.CreateDesignCatalogTemplate(ctx, db.CreateDesignCatalogTemplateParams{
		WorkspaceID: workspaceID, LibraryID: library.ID, Key: fmt.Sprintf("list-%d", time.Now().UnixNano()), Name: "List", Category: "list", Metadata: []byte(`{}`), CreatedBy: userID,
	})
	if err != nil {
		t.Fatalf("create catalog template: %v", err)
	}
	templateRevision, err := queries.CreateDesignTemplateRevision(ctx, db.CreateDesignTemplateRevisionParams{
		WorkspaceID: workspaceID, TemplateID: template.ID, DesignRevisionID: parseUUID(templateDesign.CurrentRevision.ID), RevisionNumber: 1, Status: "published", SlotSchema: []byte(`{}`), Metadata: []byte(`{}`), CreatedBy: userID,
	})
	if err != nil {
		t.Fatalf("create template revision: %v", err)
	}
	t.Cleanup(func() {
		_, _ = testPool.Exec(context.Background(), `DELETE FROM design_catalog_template WHERE id = $1`, template.ID)
		_, _ = testPool.Exec(context.Background(), `DELETE FROM design_template_library WHERE id = $1`, library.ID)
	})

	recipeDesign := createDesignFileForTest(t, "Generation asset recipes")
	if recipeDesign.CurrentRevision == nil {
		t.Fatal("recipe design has no revision")
	}
	recipeDoc := generationAssetRecipeDocument()
	updateGenerationAssetNativeJSON(t, parseUUID(recipeDesign.CurrentRevision.ID), recipeDoc)
	profile, err := queries.CreateDesignSystemProfile(ctx, db.CreateDesignSystemProfileParams{
		WorkspaceID: workspaceID, SourceFileID: parseUUID(recipeDesign.File.ID), SourceRevisionID: parseUUID(recipeDesign.CurrentRevision.ID), Name: "Generation Assets", Status: "analyzed", ProfileJson: []byte(`{}`), AnalysisErrors: []byte(`[]`), CreatedBy: userID,
	})
	if err != nil {
		t.Fatalf("create design system profile: %v", err)
	}
	t.Cleanup(func() {
		_, _ = testPool.Exec(context.Background(), `DELETE FROM design_system_profile WHERE id = $1`, profile.ID)
	})

	structure := designcore.ExtractTemplateStructure(templateDoc)
	blueprint, diagnostics := designcore.BuildTemplateBlueprint(structure, generationAssetBlueprintClassification(), designcore.BlueprintSourceRefs{
		DesignFileID: uuidToString(parseUUID(templateDesign.File.ID)), DesignRevisionID: templateDesign.CurrentRevision.ID, TemplateRevisionID: uuidToString(templateRevision.ID),
	})
	if diagnostics.HasErrors() {
		t.Fatalf("build fixture blueprint: %+v", diagnostics)
	}
	recipeSet, diagnostics := designcore.BuildComponentRecipeSet(uuidToString(profile.ID), recipeDesign.CurrentRevision.ID, designcore.ComponentRecipeSetVersion, recipeDoc, generationAssetRecipeClassifications(), nil)
	if diagnostics.HasErrors() {
		t.Fatalf("build fixture recipes: %+v", diagnostics)
	}

	return generationAssetFixture{
		WorkspaceID: workspaceID, UserID: userID, TemplateID: template.ID, TemplateLibraryID: library.ID, TemplateRevisionID: templateRevision.ID,
		TemplateSourceFileID: parseUUID(templateDesign.File.ID), TemplateSourceRevisionID: parseUUID(templateDesign.CurrentRevision.ID), DesignSystemProfileID: profile.ID,
		RecipeSourceFileID: parseUUID(recipeDesign.File.ID), RecipeSourceRevisionID: parseUUID(recipeDesign.CurrentRevision.ID), Structure: structure, Blueprint: blueprint, RecipeSet: recipeSet, RecipeDoc: recipeDoc,
	}
}

func createGenerationAssetCatalogTemplate(t *testing.T, fixture generationAssetFixture) pgtype.UUID {
	t.Helper()
	var templateID pgtype.UUID
	if err := testPool.QueryRow(context.Background(), `
		INSERT INTO design_catalog_template (workspace_id, library_id, key, name, category, metadata, created_by)
		VALUES ($1, $2, $3, 'Mismatch', 'list', '{}'::jsonb, $4)
		RETURNING id
	`, fixture.WorkspaceID, fixture.TemplateLibraryID, fmt.Sprintf("mismatch-%d", time.Now().UnixNano()), fixture.UserID).Scan(&templateID); err != nil {
		t.Fatalf("create mismatch catalog template: %v", err)
	}
	return templateID
}

func createGenerationAssetWorkspace(t *testing.T) pgtype.UUID {
	t.Helper()
	var workspaceID pgtype.UUID
	slug := fmt.Sprintf("generation-assets-%d", time.Now().UnixNano())
	if err := testPool.QueryRow(context.Background(), `
		INSERT INTO workspace (name, slug, description, issue_prefix)
		VALUES ('Generation Asset Boundary', $1, '', 'GAB')
		RETURNING id
	`, slug).Scan(&workspaceID); err != nil {
		t.Fatalf("create cross-workspace fixture: %v", err)
	}
	t.Cleanup(func() { _, _ = testPool.Exec(context.Background(), `DELETE FROM workspace WHERE id = $1`, workspaceID) })
	return workspaceID
}

func updateGenerationAssetNativeJSON(t *testing.T, revisionID pgtype.UUID, doc designcore.NativeJSON) {
	t.Helper()
	raw, err := json.Marshal(doc)
	if err != nil {
		t.Fatalf("marshal native json: %v", err)
	}
	if _, err := testPool.Exec(context.Background(), `UPDATE design_revision SET native_json = $1 WHERE id = $2`, raw, revisionID); err != nil {
		t.Fatalf("update native json: %v", err)
	}
}

func generationAssetBlueprintClassification() designcore.BlueprintClassification {
	return designcore.BlueprintClassification{
		FrameID: "frame-1", PageType: "list",
		Regions: map[string]designcore.RegionClassification{
			"shell": {RootLayerID: "shell"}, "content": {RootLayerID: "content"}, "navigation": {RootLayerID: "sidebar"},
			"breadcrumb": {RootLayerID: "breadcrumb", ReplaceChildren: true}, "pageTitle": {RootLayerID: "page-title", ReplaceChildren: true},
			"filters": {RootLayerID: "filters", ReplaceChildren: true}, "pageActions": {RootLayerID: "page-actions", ReplaceChildren: true},
			"table": {RootLayerID: "table", ReplaceChildren: true}, "pagination": {RootLayerID: "pagination", ReplaceChildren: true},
		},
		Prototypes: map[string]designcore.PrototypeClassification{
			"pageTitle":       {RootLayerID: "page-title-prototype", Bindings: map[string]string{"label": "page-title-text"}},
			"breadcrumbItem":  {RootLayerID: "breadcrumb-item", Bindings: map[string]string{"label": "breadcrumb-text"}},
			"tableHeaderCell": {RootLayerID: "table-header-cell"}, "tableRow": {RootLayerID: "table-row"},
		},
		Constraints:            designcore.BlueprintConstraints{ContentWidth: 1120, FilterColumns: 3, FilterRowHeight: 68, TableHeaderHeight: 44, TableRowHeight: 52, HorizontalGap: 16, VerticalGap: 16},
		ShellAllowlistLayerIDs: []string{"sidebar", "topbar"},
	}
}

func generationAssetTemplateDocument() designcore.NativeJSON {
	layers := map[string]designcore.Layer{}
	add := func(id, parentID, layerType string, children []string, x, y, width, height float64) {
		layers[id] = designcore.Layer{ID: id, FrameID: "frame-1", ParentID: parentID, Children: children, Name: id, Type: layerType, Visible: true, X: x, Y: y, Width: width, Height: height}
	}
	add("frame-root", "", "frame", []string{"shell"}, 0, 0, 1440, 900)
	add("shell", "frame-root", "frame", []string{"sidebar", "topbar", "content"}, 0, 0, 1440, 900)
	add("sidebar", "shell", "frame", nil, 0, 0, 240, 900)
	add("topbar", "shell", "frame", nil, 240, 0, 1200, 64)
	add("content", "shell", "frame", []string{"breadcrumb", "page-title", "filters", "page-actions", "table", "pagination", "page-title-prototype", "breadcrumb-item", "table-header-cell", "table-row"}, 240, 64, 1200, 836)
	add("breadcrumb", "content", "frame", nil, 24, 72, 500, 20)
	add("page-title", "content", "frame", nil, 24, 96, 500, 40)
	add("filters", "content", "frame", nil, 24, 152, 1120, 68)
	add("page-actions", "content", "frame", nil, 24, 236, 1120, 40)
	add("table", "content", "frame", nil, 24, 292, 1120, 460)
	add("pagination", "content", "frame", nil, 24, 768, 1120, 40)
	add("page-title-prototype", "content", "frame", []string{"page-title-text"}, 24, 96, 320, 32)
	add("page-title-text", "page-title-prototype", "text", nil, 24, 96, 320, 32)
	add("breadcrumb-item", "content", "frame", []string{"breadcrumb-text"}, 24, 72, 120, 20)
	add("breadcrumb-text", "breadcrumb-item", "text", nil, 24, 72, 120, 20)
	add("table-header-cell", "content", "frame", nil, 24, 292, 180, 44)
	add("table-row", "content", "frame", nil, 24, 336, 1120, 52)
	return designcore.NativeJSON{Version: designcore.NativeJSONVersion, Frames: []designcore.Frame{{ID: "frame-1", Name: "Desktop", RootLayerID: "frame-root", Width: 1440, Height: 900}}, Layers: layers}
}

func generationAssetRecipeDocument() designcore.NativeJSON {
	layers := map[string]designcore.Layer{}
	for index, kind := range generationAssetRecipeKinds() {
		layers[kind] = designcore.Layer{ID: kind, FrameID: "recipes", Name: kind, Type: "frame", Visible: true, X: float64(index * 100), Width: 80, Height: 32}
	}
	return designcore.NativeJSON{Version: designcore.NativeJSONVersion, Frames: []designcore.Frame{{ID: "recipes", Name: "Recipes", RootLayerID: "input", Width: 1000, Height: 32}}, Layers: layers, Tokens: map[string]any{}}
}

func generationAssetRecipeClassifications() []designcore.ComponentRecipeClassification {
	classifications := make([]designcore.ComponentRecipeClassification, 0, len(generationAssetRecipeKinds()))
	for _, kind := range generationAssetRecipeKinds() {
		classifications = append(classifications, designcore.ComponentRecipeClassification{Kind: kind, Variant: "default", State: "default", RootLayerID: kind, Layout: designcore.RecipeLayout{WidthMode: "fixed", TextOverflow: "ellipsis", Height: 32}})
	}
	return classifications
}

func generationAssetRecipeKinds() []string {
	return []string{"input", "select", "date-range", "primary-button", "secondary-button", "text-button", "table-header", "table-row", "status-tag", "pagination"}
}
