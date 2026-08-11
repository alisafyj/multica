package handler

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/google/uuid"
)

type designDeleteFixture struct {
	workspaceID string
	fileID      string
	revisionID  string
	assetID     string
	draftID     string
	taskID      string
	planID      string
	mappingID   string
	deliveryID  string
	profileID   string
}

func seedDesignDeleteFixture(t *testing.T, workspaceID, folderID string) designDeleteFixture {
	t.Helper()
	ctx := context.Background()
	nativeJSON, err := json.Marshal(minimalDesignNativeJSON("Delete integrity fixture"))
	if err != nil {
		t.Fatalf("marshal native json: %v", err)
	}
	f := designDeleteFixture{workspaceID: workspaceID}
	if err := testPool.QueryRow(ctx, `
		INSERT INTO design_file (workspace_id, project_id, folder_id, title, source_type, source_ref, created_by)
		VALUES ($1, $2, $3, $4, 'upload', '{}'::jsonb, $5)
		RETURNING id
	`, workspaceID, uuid.NewString(), nullableUUIDForDesignDeleteTest(folderID), "delete-integrity-"+uuid.NewString(), testUserID).Scan(&f.fileID); err != nil {
		t.Fatalf("insert design file: %v", err)
	}
	if err := testPool.QueryRow(ctx, `
		INSERT INTO design_revision (file_id, workspace_id, revision_number, status, native_json, validation_errors, created_by)
		VALUES ($1, $2, 1, 'valid', $3::jsonb, '[]'::jsonb, $4)
		RETURNING id
	`, f.fileID, workspaceID, nativeJSON, testUserID).Scan(&f.revisionID); err != nil {
		t.Fatalf("insert design revision: %v", err)
	}
	if _, err := testPool.Exec(ctx, `UPDATE design_file SET current_revision_id = $1 WHERE id = $2`, f.revisionID, f.fileID); err != nil {
		t.Fatalf("set current revision: %v", err)
	}
	if err := testPool.QueryRow(ctx, `
		INSERT INTO design_asset (file_id, revision_id, workspace_id, asset_key, kind, url, created_by)
		VALUES ($1, $2, $3, $4, 'image', 'https://example.test/delete-integrity.png', $5)
		RETURNING id
	`, f.fileID, f.revisionID, workspaceID, "asset-"+uuid.NewString(), testUserID).Scan(&f.assetID); err != nil {
		t.Fatalf("insert design asset: %v", err)
	}
	if err := testPool.QueryRow(ctx, `
		INSERT INTO design_draft (
			workspace_id, file_id, revision_id, generated_file_id, generated_revision_id,
			title, requirement_core, slot_values, patch, status, validation_errors, created_by
		) VALUES ($1, $2, $3, $2, $3, $4, '{}'::jsonb, '{}'::jsonb, '[]'::jsonb, 'draft', '[]'::jsonb, $5)
		RETURNING id
	`, workspaceID, f.fileID, f.revisionID, "delete-integrity-draft-"+uuid.NewString(), testUserID).Scan(&f.draftID); err != nil {
		t.Fatalf("insert design draft: %v", err)
	}
	if err := testPool.QueryRow(ctx, `
		INSERT INTO design_delivery (
			workspace_id, project_id, source_issue_id, target_issue_id, file_id, revision_id,
			scope, status, delivered_by
		) VALUES ($1, $2, $3, $4, $5, $6, '{}'::jsonb, 'active', $7)
		RETURNING id
	`, workspaceID, uuid.NewString(), uuid.NewString(), uuid.NewString(), f.fileID, f.revisionID, testUserID).Scan(&f.deliveryID); err != nil {
		t.Fatalf("insert design delivery: %v", err)
	}
	if err := testPool.QueryRow(ctx, `
		INSERT INTO design_system_profile (
			workspace_id, project_id, source_file_id, source_revision_id, name, status,
			profile_json, analysis_errors, created_by
		) VALUES ($1, $2, $3, $4, $5, 'analyzed', '{}'::jsonb, '[]'::jsonb, $6)
		RETURNING id
	`, workspaceID, uuid.NewString(), f.fileID, f.revisionID, "delete-integrity-profile-"+uuid.NewString(), testUserID).Scan(&f.profileID); err != nil {
		t.Fatalf("insert design system profile: %v", err)
	}
	if err := testPool.QueryRow(ctx, `
		INSERT INTO design_restore_task (
			workspace_id, file_id, revision_id, status, input, result, created_by
		) VALUES ($1, $2, $3, 'queued', '{}'::jsonb, '{}'::jsonb, $4)
		RETURNING id
	`, workspaceID, f.fileID, f.revisionID, testUserID).Scan(&f.taskID); err != nil {
		t.Fatalf("insert design restore task: %v", err)
	}
	if err := testPool.QueryRow(ctx, `
		INSERT INTO design_restore_plan (workspace_id, restore_task_id, status, plan, created_by)
		VALUES ($1, $2, 'draft', '{}'::jsonb, $3)
		RETURNING id
	`, workspaceID, f.taskID, testUserID).Scan(&f.planID); err != nil {
		t.Fatalf("insert design restore plan: %v", err)
	}
	if err := testPool.QueryRow(ctx, `
		INSERT INTO design_restore_mapping (
			restore_task_id, workspace_id, layer_id, target_path, target_kind, confidence, metadata
		) VALUES ($1, $2, 'layer-1', 'src/page.tsx', 'file', 1, '{}'::jsonb)
		RETURNING id
	`, f.taskID, workspaceID).Scan(&f.mappingID); err != nil {
		t.Fatalf("insert design restore mapping: %v", err)
	}
	t.Cleanup(func() { cleanupDesignDeleteFixture(f) })
	return f
}

func nullableUUIDForDesignDeleteTest(value string) any {
	if value == "" {
		return nil
	}
	return value
}

func cleanupDesignDeleteFixture(f designDeleteFixture) {
	ctx := context.Background()
	_, _ = testPool.Exec(ctx, `DELETE FROM design_component_recipe_set WHERE source_revision_id = $1`, f.revisionID)
	_, _ = testPool.Exec(ctx, `DELETE FROM design_template_blueprint WHERE source_revision_id = $1`, f.revisionID)
	_, _ = testPool.Exec(ctx, `DELETE FROM design_template_revision WHERE design_revision_id = $1`, f.revisionID)
	_, _ = testPool.Exec(ctx, `DELETE FROM design_restore_mapping WHERE id = $1`, f.mappingID)
	_, _ = testPool.Exec(ctx, `DELETE FROM design_restore_plan WHERE id = $1`, f.planID)
	_, _ = testPool.Exec(ctx, `DELETE FROM design_restore_task WHERE id = $1`, f.taskID)
	_, _ = testPool.Exec(ctx, `DELETE FROM design_delivery WHERE id = $1`, f.deliveryID)
	_, _ = testPool.Exec(ctx, `DELETE FROM design_system_profile WHERE id = $1`, f.profileID)
	_, _ = testPool.Exec(ctx, `DELETE FROM design_asset WHERE id = $1`, f.assetID)
	_, _ = testPool.Exec(ctx, `DELETE FROM design_draft WHERE id = $1`, f.draftID)
	_, _ = testPool.Exec(ctx, `DELETE FROM design_revision WHERE id = $1`, f.revisionID)
	_, _ = testPool.Exec(ctx, `DELETE FROM design_file WHERE id = $1`, f.fileID)
}

func seedAdditionalDesignRevision(t *testing.T, f designDeleteFixture, frameID string, makeCurrent bool) string {
	t.Helper()
	nativeJSON := minimalDesignNativeJSON("Additional revision")
	frames := nativeJSON["frames"].([]map[string]any)
	frames[0]["id"] = frameID
	layers := nativeJSON["layers"].(map[string]any)
	layers["layer-1"].(map[string]any)["frameId"] = frameID
	raw, err := json.Marshal(nativeJSON)
	if err != nil {
		t.Fatalf("marshal additional revision: %v", err)
	}
	var revisionID string
	if err := testPool.QueryRow(context.Background(), `
		INSERT INTO design_revision (file_id, workspace_id, revision_number, status, native_json, validation_errors, created_by)
		VALUES ($1, $2, 2, 'valid', $3::jsonb, '[]'::jsonb, $4)
		RETURNING id
	`, f.fileID, f.workspaceID, raw, testUserID).Scan(&revisionID); err != nil {
		t.Fatalf("insert additional design revision: %v", err)
	}
	if makeCurrent {
		if _, err := testPool.Exec(context.Background(), `UPDATE design_file SET current_revision_id = $1 WHERE id = $2`, revisionID, f.fileID); err != nil {
			t.Fatalf("set additional current revision: %v", err)
		}
	}
	t.Cleanup(func() {
		_, _ = testPool.Exec(context.Background(), `DELETE FROM design_revision WHERE id = $1`, revisionID)
	})
	return revisionID
}

func requireDesignRowCount(t *testing.T, table, id string, want int) {
	t.Helper()
	var got int
	query := fmt.Sprintf("SELECT count(*) FROM %s WHERE id = $1", table)
	if err := testPool.QueryRow(context.Background(), query, id).Scan(&got); err != nil {
		t.Fatalf("count %s: %v", table, err)
	}
	if got != want {
		t.Fatalf("%s row count = %d, want %d", table, got, want)
	}
}

func requireDesignFixturePresent(t *testing.T, f designDeleteFixture) {
	t.Helper()
	for table, id := range map[string]string{
		"design_file":            f.fileID,
		"design_revision":        f.revisionID,
		"design_asset":           f.assetID,
		"design_draft":           f.draftID,
		"design_restore_task":    f.taskID,
		"design_restore_plan":    f.planID,
		"design_restore_mapping": f.mappingID,
		"design_delivery":        f.deliveryID,
		"design_system_profile":  f.profileID,
	} {
		requireDesignRowCount(t, table, id, 1)
	}
}

func requireDesignFixtureDeleted(t *testing.T, f designDeleteFixture, assetDeleted bool) {
	t.Helper()
	for table, id := range map[string]string{
		"design_revision":        f.revisionID,
		"design_restore_task":    f.taskID,
		"design_restore_plan":    f.planID,
		"design_restore_mapping": f.mappingID,
		"design_delivery":        f.deliveryID,
		"design_system_profile":  f.profileID,
	} {
		requireDesignRowCount(t, table, id, 0)
	}
	assetWant := 1
	if assetDeleted {
		assetWant = 0
	}
	requireDesignRowCount(t, "design_asset", f.assetID, assetWant)
	var fileID, revisionID, generatedFileID, generatedRevisionID *string
	if err := testPool.QueryRow(context.Background(), `
		SELECT file_id::text, revision_id::text, generated_file_id::text, generated_revision_id::text
		FROM design_draft WHERE id = $1
	`, f.draftID).Scan(&fileID, &revisionID, &generatedFileID, &generatedRevisionID); err != nil {
		t.Fatalf("read detached design draft: %v", err)
	}
	if revisionID != nil || generatedRevisionID != nil {
		t.Fatalf("draft revision refs were not detached: revision=%v generated_revision=%v", revisionID, generatedRevisionID)
	}
	if assetDeleted {
		if fileID != nil || generatedFileID != nil {
			t.Fatalf("draft file refs were not detached: file=%v generated_file=%v", fileID, generatedFileID)
		}
	} else {
		if fileID == nil || *fileID != f.fileID || generatedFileID == nil || *generatedFileID != f.fileID {
			t.Fatalf("revision-only delete changed draft file refs: file=%v generated_file=%v", fileID, generatedFileID)
		}
		var assetRevisionID *string
		if err := testPool.QueryRow(context.Background(), `SELECT revision_id::text FROM design_asset WHERE id = $1`, f.assetID).Scan(&assetRevisionID); err != nil {
			t.Fatalf("read detached design asset: %v", err)
		}
		if assetRevisionID != nil {
			t.Fatalf("asset revision ref was not detached: %v", assetRevisionID)
		}
	}
}

func deleteDesignFileForIntegrityTest(t *testing.T, fileID, workspaceID string) *httptest.ResponseRecorder {
	t.Helper()
	req := withURLParam(newRequest("DELETE", "/api/design-files/"+fileID+"?workspace_id="+workspaceID, nil), "id", fileID)
	w := httptest.NewRecorder()
	testHandler.DeleteDesignFile(w, req)
	return w
}

func deleteDesignFrameForIntegrityTest(t *testing.T, fileID, workspaceID string) *httptest.ResponseRecorder {
	t.Helper()
	req := withDesignURLParams(newRequest("DELETE", "/api/design-files/"+fileID+"/frames/frame-1?workspace_id="+workspaceID, nil), "id", fileID, "frameId", "frame-1")
	w := httptest.NewRecorder()
	testHandler.DeleteDesignFrame(w, req)
	return w
}

func TestDeleteDesignFileCleansDependentsAndPreservesOtherTenant(t *testing.T) {
	target := seedDesignDeleteFixture(t, testWorkspaceID, "")
	additionalRevisionID := seedAdditionalDesignRevision(t, target, "frame-2", true)
	foreign := seedDesignDeleteFixture(t, uuid.NewString(), "")

	w := deleteDesignFileForIntegrityTest(t, target.fileID, testWorkspaceID)
	if w.Code != http.StatusNoContent {
		t.Fatalf("DeleteDesignFile: expected 204, got %d: %s", w.Code, w.Body.String())
	}
	requireDesignRowCount(t, "design_file", target.fileID, 0)
	requireDesignRowCount(t, "design_revision", additionalRevisionID, 0)
	requireDesignFixtureDeleted(t, target, true)
	requireDesignFixturePresent(t, foreign)
}

func TestDeleteDesignRevisionRejectsProtectedReferencesWithoutPartialDeletion(t *testing.T) {
	for _, table := range []string{"design_template_revision", "design_template_blueprint", "design_component_recipe_set"} {
		t.Run(table, func(t *testing.T) {
			f := seedDesignDeleteFixture(t, testWorkspaceID, "")
			protectedID := uuid.NewString()
			switch table {
			case "design_template_revision":
				_, err := testPool.Exec(context.Background(), `
					INSERT INTO design_template_revision (
						id, workspace_id, template_id, design_revision_id, revision_number, status, slot_schema, metadata, created_by
					) VALUES ($1, $2, $3, $4, 1, 'published', '{}'::jsonb, '{}'::jsonb, $5)
				`, protectedID, testWorkspaceID, uuid.NewString(), f.revisionID, testUserID)
				if err != nil {
					t.Fatalf("insert protected template revision: %v", err)
				}
			case "design_template_blueprint":
				_, err := testPool.Exec(context.Background(), `
					INSERT INTO design_template_blueprint (
						id, workspace_id, template_id, template_revision_id, source_revision_id,
						analysis_version, schema_version, status, structure_json, blueprint_json, validation_errors, created_by
					) VALUES ($1, $2, $3, $4, $5, 1, '1', 'valid', '{}'::jsonb, '{}'::jsonb, '[]'::jsonb, $6)
				`, protectedID, testWorkspaceID, uuid.NewString(), uuid.NewString(), f.revisionID, testUserID)
				if err != nil {
					t.Fatalf("insert protected blueprint: %v", err)
				}
			case "design_component_recipe_set":
				_, err := testPool.Exec(context.Background(), `
					INSERT INTO design_component_recipe_set (
						id, workspace_id, design_system_profile_id, source_revision_id,
						analysis_version, schema_version, status, recipes_json, validation_errors, created_by
					) VALUES ($1, $2, $3, $4, 1, '1', 'valid', '{}'::jsonb, '[]'::jsonb, $5)
				`, protectedID, testWorkspaceID, f.profileID, f.revisionID, testUserID)
				if err != nil {
					t.Fatalf("insert protected recipe set: %v", err)
				}
			}
			t.Cleanup(func() {
				_, _ = testPool.Exec(context.Background(), fmt.Sprintf("DELETE FROM %s WHERE id = $1", table), protectedID)
			})

			frameW := deleteDesignFrameForIntegrityTest(t, f.fileID, testWorkspaceID)
			if frameW.Code != http.StatusConflict {
				t.Fatalf("DeleteDesignFrame with %s reference: expected 409, got %d: %s", table, frameW.Code, frameW.Body.String())
			}
			requireDesignFixturePresent(t, f)
			requireDesignRowCount(t, table, protectedID, 1)

			fileW := deleteDesignFileForIntegrityTest(t, f.fileID, testWorkspaceID)
			if fileW.Code != http.StatusConflict {
				t.Fatalf("DeleteDesignFile with %s reference: expected 409, got %d: %s", table, fileW.Code, fileW.Body.String())
			}
			requireDesignFixturePresent(t, f)
			requireDesignRowCount(t, table, protectedID, 1)
		})
	}
}

func TestDeleteDesignFrameCleansRevisionDependents(t *testing.T) {
	f := seedDesignDeleteFixture(t, testWorkspaceID, "")
	remainingRevisionID := seedAdditionalDesignRevision(t, f, "frame-other", true)
	w := deleteDesignFrameForIntegrityTest(t, f.fileID, testWorkspaceID)
	if w.Code != http.StatusNoContent {
		t.Fatalf("DeleteDesignFrame: expected 204, got %d: %s", w.Code, w.Body.String())
	}
	requireDesignRowCount(t, "design_file", f.fileID, 1)
	requireDesignRowCount(t, "design_revision", remainingRevisionID, 1)
	requireDesignFixtureDeleted(t, f, false)
	var currentRevisionID string
	if err := testPool.QueryRow(context.Background(), `SELECT current_revision_id::text FROM design_file WHERE id = $1`, f.fileID).Scan(&currentRevisionID); err != nil {
		t.Fatalf("read remaining current revision: %v", err)
	}
	if currentRevisionID != remainingRevisionID {
		t.Fatalf("current revision = %s, want %s", currentRevisionID, remainingRevisionID)
	}
}

func TestDeleteDesignFolderIntegrity(t *testing.T) {
	t.Run("rejects child folder", func(t *testing.T) {
		projectID := createProjectForDesignTest(t, "Folder child integrity")
		parentID := createDesignFolderForTest(t, projectID, "parent-"+uuid.NewString())
		var childID string
		if err := testPool.QueryRow(context.Background(), `
			INSERT INTO design_folder (workspace_id, project_id, parent_id, name, created_by)
			VALUES ($1, $2, $3, $4, $5)
			RETURNING id
		`, testWorkspaceID, projectID, parentID, "child-"+uuid.NewString(), testUserID).Scan(&childID); err != nil {
			t.Fatalf("insert child folder: %v", err)
		}
		f := seedDesignDeleteFixture(t, testWorkspaceID, parentID)
		t.Cleanup(func() {
			_, _ = testPool.Exec(context.Background(), `DELETE FROM design_folder WHERE id IN ($1, $2)`, childID, parentID)
		})

		req := withURLParam(newRequest("DELETE", "/api/design-folders/"+parentID+"?workspace_id="+testWorkspaceID, nil), "id", parentID)
		w := httptest.NewRecorder()
		testHandler.DeleteDesignFolder(w, req)
		if w.Code != http.StatusConflict {
			t.Fatalf("DeleteDesignFolder with child: expected 409, got %d: %s", w.Code, w.Body.String())
		}
		requireDesignRowCount(t, "design_folder", parentID, 1)
		requireDesignRowCount(t, "design_folder", childID, 1)
		requireDesignFixturePresent(t, f)
	})

	t.Run("deletes contained files with full cleanup", func(t *testing.T) {
		projectID := createProjectForDesignTest(t, "Folder cleanup integrity")
		folderID := createDesignFolderForTest(t, projectID, "cleanup-"+uuid.NewString())
		target := seedDesignDeleteFixture(t, testWorkspaceID, folderID)
		siblingFolderID := createDesignFolderForTest(t, projectID, "sibling-"+uuid.NewString())
		sibling := seedDesignDeleteFixture(t, testWorkspaceID, siblingFolderID)
		foreignWorkspaceID := uuid.NewString()
		foreignFolderID := uuid.NewString()
		foreignProjectID := uuid.NewString()
		if _, err := testPool.Exec(context.Background(), `
			INSERT INTO design_folder (id, workspace_id, project_id, name, created_by)
			VALUES ($1, $2, $3, $4, $5)
		`, foreignFolderID, foreignWorkspaceID, foreignProjectID, "foreign-"+uuid.NewString(), testUserID); err != nil {
			t.Fatalf("insert foreign folder: %v", err)
		}
		foreign := seedDesignDeleteFixture(t, foreignWorkspaceID, foreignFolderID)
		t.Cleanup(func() {
			_, _ = testPool.Exec(context.Background(), `DELETE FROM design_folder WHERE id IN ($1, $2, $3)`, folderID, siblingFolderID, foreignFolderID)
		})

		req := withURLParam(newRequest("DELETE", "/api/design-folders/"+folderID+"?workspace_id="+testWorkspaceID, nil), "id", folderID)
		w := httptest.NewRecorder()
		testHandler.DeleteDesignFolder(w, req)
		if w.Code != http.StatusNoContent {
			t.Fatalf("DeleteDesignFolder: expected 204, got %d: %s", w.Code, w.Body.String())
		}
		requireDesignRowCount(t, "design_folder", folderID, 0)
		requireDesignRowCount(t, "design_file", target.fileID, 0)
		requireDesignFixtureDeleted(t, target, true)
		requireDesignRowCount(t, "design_folder", siblingFolderID, 1)
		requireDesignFixturePresent(t, sibling)
		requireDesignRowCount(t, "design_folder", foreignFolderID, 1)
		requireDesignFixturePresent(t, foreign)
	})
}
