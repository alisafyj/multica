package handler

import (
	"context"
	"fmt"
	"testing"
	"time"

	"github.com/jackc/pgx/v5/pgtype"
	db "github.com/multica-ai/multica/server/pkg/db/generated"
)

func TestDesignQueriesPreserveCatalogRelationsAndManagedSourceFiltering(t *testing.T) {
	ctx := context.Background()
	queries := db.New(testPool)
	fixture := createGenerationAssetFixture(t)

	if _, err := queries.UpdateDesignCatalogTemplateCurrentRevision(ctx, db.UpdateDesignCatalogTemplateCurrentRevisionParams{
		ID: fixture.TemplateID, WorkspaceID: fixture.WorkspaceID, CurrentRevisionID: fixture.TemplateRevisionID,
	}); err != nil {
		t.Fatalf("publish template revision: %v", err)
	}

	got, err := queries.GetDesignCatalogTemplate(ctx, db.GetDesignCatalogTemplateParams{
		ID: fixture.TemplateID, WorkspaceID: fixture.WorkspaceID,
	})
	if err != nil {
		t.Fatalf("get catalog template: %v", err)
	}
	assertCatalogTemplateRelations(t, got.DesignRevisionID, got.TemplateRevisionNumber, got.SlotSchema, got.DesignFileID, got.DesignFileTitle, fixture)

	unpublished, err := queries.CreateDesignCatalogTemplate(ctx, db.CreateDesignCatalogTemplateParams{
		WorkspaceID: fixture.WorkspaceID, LibraryID: fixture.TemplateLibraryID,
		Key: fmt.Sprintf("unpublished-%d", time.Now().UnixNano()), Name: "Unpublished", Category: "list",
		Metadata: []byte(`{}`), CreatedBy: fixture.UserID,
	})
	if err != nil {
		t.Fatalf("create unpublished catalog template: %v", err)
	}
	t.Cleanup(func() {
		_, _ = testPool.Exec(context.Background(), `DELETE FROM design_catalog_template WHERE id = $1`, unpublished.ID)
	})
	unpublishedGet, err := queries.GetDesignCatalogTemplate(ctx, db.GetDesignCatalogTemplateParams{
		ID: unpublished.ID, WorkspaceID: fixture.WorkspaceID,
	})
	if err != nil {
		t.Fatalf("get unpublished catalog template: %v", err)
	}
	assertEmptyCatalogTemplateRelations(t, unpublishedGet.CurrentRevisionID, unpublishedGet.DesignRevisionID, unpublishedGet.TemplateRevisionNumber, unpublishedGet.SlotSchema, unpublishedGet.DesignFileID, unpublishedGet.DesignFileTitle)

	listed, err := queries.ListDesignCatalogTemplates(ctx, db.ListDesignCatalogTemplatesParams{
		WorkspaceID: fixture.WorkspaceID, Column2: fixture.TemplateLibraryID, Column3: "list",
	})
	if err != nil {
		t.Fatalf("list catalog templates: %v", err)
	}
	found := false
	foundUnpublished := false
	for _, item := range listed {
		if item.ID == fixture.TemplateID {
			assertCatalogTemplateRelations(t, item.DesignRevisionID, item.TemplateRevisionNumber, item.SlotSchema, item.DesignFileID, item.DesignFileTitle, fixture)
			found = true
		}
		if item.ID == unpublished.ID {
			assertEmptyCatalogTemplateRelations(t, item.CurrentRevisionID, item.DesignRevisionID, item.TemplateRevisionNumber, item.SlotSchema, item.DesignFileID, item.DesignFileTitle)
			foundUnpublished = true
		}
	}
	if !found {
		t.Fatalf("catalog template %s not found", uuidToString(fixture.TemplateID))
	}
	if !foundUnpublished {
		t.Fatalf("unpublished catalog template %s not found", uuidToString(unpublished.ID))
	}

	regular := createDesignFileForTest(t, "Visible direct-query design")
	regularID := parseUUID(regular.File.ID)
	if _, err := testPool.Exec(ctx, `UPDATE design_file SET project_id = $1 WHERE id = $2`, fixture.ProjectID, regularID); err != nil {
		t.Fatalf("scope regular design file: %v", err)
	}

	workspaceFiles, err := queries.ListDesignFiles(ctx, fixture.WorkspaceID)
	if err != nil {
		t.Fatalf("list workspace design files: %v", err)
	}
	assertManagedDesignFileVisibility(t, workspaceFiles, regularID, fixture.TemplateSourceFileID, fixture.RecipeSourceFileID)

	projectFiles, err := queries.ListDesignFilesByProject(ctx, db.ListDesignFilesByProjectParams{
		WorkspaceID: fixture.WorkspaceID, ProjectID: fixture.ProjectID,
	})
	if err != nil {
		t.Fatalf("list project design files: %v", err)
	}
	assertManagedDesignFileVisibility(t, projectFiles, regularID, fixture.TemplateSourceFileID, fixture.RecipeSourceFileID)
}

func assertEmptyCatalogTemplateRelations(
	t *testing.T,
	currentRevisionID, designRevisionID pgtype.UUID,
	revisionNumber pgtype.Int4,
	slotSchema []byte,
	designFileID pgtype.UUID,
	designFileTitle pgtype.Text,
) {
	t.Helper()
	if currentRevisionID.Valid || designRevisionID.Valid || revisionNumber.Valid || slotSchema != nil || designFileID.Valid || designFileTitle.Valid {
		t.Fatalf("unpublished relations are not empty: current=%+v revision=%+v number=%+v slots=%s file=%+v title=%+v", currentRevisionID, designRevisionID, revisionNumber, slotSchema, designFileID, designFileTitle)
	}
}

func assertCatalogTemplateRelations(
	t *testing.T,
	designRevisionID pgtype.UUID,
	revisionNumber pgtype.Int4,
	slotSchema []byte,
	designFileID pgtype.UUID,
	designFileTitle pgtype.Text,
	fixture generationAssetFixture,
) {
	t.Helper()
	if designRevisionID != fixture.TemplateSourceRevisionID || !revisionNumber.Valid || revisionNumber.Int32 != 1 {
		t.Fatalf("template revision relation = revision:%s number:%+v", uuidToString(designRevisionID), revisionNumber)
	}
	if string(slotSchema) != "{}" {
		t.Fatalf("slot schema = %s", slotSchema)
	}
	if designFileID != fixture.TemplateSourceFileID || !designFileTitle.Valid || designFileTitle.String != "Generation asset template" {
		t.Fatalf("design file relation = file:%s title:%+v", uuidToString(designFileID), designFileTitle)
	}
}

func assertManagedDesignFileVisibility(t *testing.T, files []db.DesignFile, visibleID, templateSourceID, profileSourceID pgtype.UUID) {
	t.Helper()
	seen := make(map[pgtype.UUID]bool, len(files))
	for _, file := range files {
		seen[file.ID] = true
	}
	if !seen[visibleID] {
		t.Fatalf("regular design file %s is missing", uuidToString(visibleID))
	}
	for _, hiddenID := range []pgtype.UUID{templateSourceID, profileSourceID} {
		if seen[hiddenID] {
			t.Fatalf("managed design file %s is visible", uuidToString(hiddenID))
		}
	}
}
