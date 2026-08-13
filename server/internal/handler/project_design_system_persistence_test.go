package handler

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"testing"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
	"github.com/jackc/pgx/v5/pgtype"
	db "github.com/multica-ai/multica/server/pkg/db/generated"
)

func TestProjectDesignSystemProjectUniqueness(t *testing.T) {
	ctx := context.Background()
	queries := db.New(testPool)
	projectID := createProjectDesignSystemProject(t, testWorkspaceID, "Unique project")

	createProjectDesignSystemForTest(t, queries, parseUUID(testWorkspaceID), projectID, "Unique system")
	_, err := queries.CreateProjectDesignSystem(ctx, db.CreateProjectDesignSystemParams{
		WorkspaceID:   parseUUID(testWorkspaceID),
		ProjectID:     projectID,
		Name:          "Duplicate system",
		Platform:      "web",
		InputSnapshot: []byte(`{"brief":"duplicate"}`),
		CreatedBy:     parseUUID(testUserID),
	})
	assertPostgresCode(t, err, "23505")
}

func TestProjectDesignSystemDraftUpsertLeavesSavedUntouched(t *testing.T) {
	ctx := context.Background()
	queries := db.New(testPool)
	projectID := createProjectDesignSystemProject(t, testWorkspaceID, "Draft isolation")
	system := createProjectDesignSystemForTest(t, queries, parseUUID(testWorkspaceID), projectID, "Draft isolation")

	upsertProjectDesignSystemPackageForTest(t, queries, system.ID, "saved", "saved-v1", strings.Repeat("a", 64))
	upsertProjectDesignSystemPackageForTest(t, queries, system.ID, "draft", "draft-v1", strings.Repeat("b", 64))
	upsertProjectDesignSystemPackageForTest(t, queries, system.ID, "draft", "draft-v2", strings.Repeat("c", 64))

	saved, err := queries.GetProjectDesignSystemPackageBySlot(ctx, db.GetProjectDesignSystemPackageBySlotParams{
		DesignSystemID: system.ID,
		Slot:           "saved",
		WorkspaceID:    parseUUID(testWorkspaceID),
	})
	if err != nil {
		t.Fatalf("get saved package: %v", err)
	}
	if saved.DesignMd != "saved-v1" || saved.IntegritySha256 != strings.Repeat("a", 64) {
		t.Fatalf("saved package changed with draft upsert: %+v", saved)
	}

	draft, err := queries.GetProjectDesignSystemPackageBySlot(ctx, db.GetProjectDesignSystemPackageBySlotParams{
		DesignSystemID: system.ID,
		Slot:           "draft",
		WorkspaceID:    parseUUID(testWorkspaceID),
	})
	if err != nil {
		t.Fatalf("get draft package: %v", err)
	}
	if draft.DesignMd != "draft-v2" || draft.IntegritySha256 != strings.Repeat("c", 64) {
		t.Fatalf("draft package was not atomically replaced: %+v", draft)
	}
}

func TestSaveProjectDesignSystemCopiesDraftAndDeletesDraftAtomically(t *testing.T) {
	ctx := context.Background()
	queries := db.New(testPool)
	projectID := createProjectDesignSystemProject(t, testWorkspaceID, "Atomic save")
	system := createProjectDesignSystemForTest(t, queries, parseUUID(testWorkspaceID), projectID, "Atomic save")
	oldSavedDigest := strings.Repeat("d", 64)
	draftDigest := strings.Repeat("e", 64)
	upsertProjectDesignSystemPackageForTest(t, queries, system.ID, "saved", "saved-old", oldSavedDigest)
	upsertProjectDesignSystemPackageForTest(t, queries, system.ID, "draft", "draft-new", draftDigest)

	tx, err := testPool.Begin(ctx)
	if err != nil {
		t.Fatalf("begin rollback transaction: %v", err)
	}
	txQueries := db.New(tx)
	if _, err := txQueries.SaveProjectDesignSystemDraft(ctx, db.SaveProjectDesignSystemDraftParams{
		DesignSystemID: system.ID,
		WorkspaceID:    parseUUID(testWorkspaceID),
	}); err != nil {
		t.Fatalf("copy draft before rollback: %v", err)
	}
	if err := txQueries.DeleteProjectDesignSystemPackageSlot(ctx, db.DeleteProjectDesignSystemPackageSlotParams{
		DesignSystemID: system.ID,
		Slot:           "draft",
		WorkspaceID:    parseUUID(testWorkspaceID),
	}); err != nil {
		t.Fatalf("delete draft before rollback: %v", err)
	}
	if _, err := tx.Exec(ctx, "SELECT 1 / 0"); err == nil {
		t.Fatal("failure injection unexpectedly succeeded")
	}
	if err := tx.Rollback(ctx); err != nil && !errors.Is(err, pgx.ErrTxClosed) {
		t.Fatalf("rollback failed transaction: %v", err)
	}

	assertProjectDesignSystemPackageDigest(t, queries, system.ID, "saved", oldSavedDigest)
	assertProjectDesignSystemPackageDigest(t, queries, system.ID, "draft", draftDigest)

	tx, err = testPool.Begin(ctx)
	if err != nil {
		t.Fatalf("begin save transaction: %v", err)
	}
	txQueries = db.New(tx)
	if _, err := txQueries.SaveProjectDesignSystemDraft(ctx, db.SaveProjectDesignSystemDraftParams{
		DesignSystemID: system.ID,
		WorkspaceID:    parseUUID(testWorkspaceID),
	}); err != nil {
		_ = tx.Rollback(ctx)
		t.Fatalf("copy draft: %v", err)
	}
	if err := txQueries.DeleteProjectDesignSystemPackageSlot(ctx, db.DeleteProjectDesignSystemPackageSlotParams{
		DesignSystemID: system.ID,
		Slot:           "draft",
		WorkspaceID:    parseUUID(testWorkspaceID),
	}); err != nil {
		_ = tx.Rollback(ctx)
		t.Fatalf("delete draft: %v", err)
	}
	if _, err := txQueries.MarkProjectDesignSystemSaved(ctx, db.MarkProjectDesignSystemSavedParams{
		ID:          system.ID,
		WorkspaceID: parseUUID(testWorkspaceID),
	}); err != nil {
		_ = tx.Rollback(ctx)
		t.Fatalf("mark system saved: %v", err)
	}
	if err := tx.Commit(ctx); err != nil {
		t.Fatalf("commit save transaction: %v", err)
	}

	assertProjectDesignSystemPackageDigest(t, queries, system.ID, "saved", draftDigest)
	if _, err := queries.GetProjectDesignSystemPackageBySlot(ctx, db.GetProjectDesignSystemPackageBySlotParams{
		DesignSystemID: system.ID,
		Slot:           "draft",
		WorkspaceID:    parseUUID(testWorkspaceID),
	}); !errors.Is(err, pgx.ErrNoRows) {
		t.Fatalf("draft lookup error = %v, want pgx.ErrNoRows", err)
	}
	savedSystem, err := queries.GetProjectDesignSystemInWorkspace(ctx, db.GetProjectDesignSystemInWorkspaceParams{
		ID:          system.ID,
		WorkspaceID: parseUUID(testWorkspaceID),
	})
	if err != nil {
		t.Fatalf("get saved system: %v", err)
	}
	if !savedSystem.SavedAt.Valid {
		t.Fatal("saved_at was not recorded")
	}
}

func TestProjectDesignSystemWorkspaceIsolation(t *testing.T) {
	ctx := context.Background()
	queries := db.New(testPool)
	foreignWorkspaceID := createProjectDesignSystemWorkspace(t)
	foreignProjectID := createProjectDesignSystemProject(t, uuidToString(foreignWorkspaceID), "Foreign project")

	_, err := queries.CreateProjectDesignSystem(ctx, db.CreateProjectDesignSystemParams{
		WorkspaceID:   parseUUID(testWorkspaceID),
		ProjectID:     foreignProjectID,
		Name:          "Cross-workspace system",
		Platform:      "web",
		InputSnapshot: []byte(`{"brief":"must not cross workspaces"}`),
		CreatedBy:     parseUUID(testUserID),
	})
	if !errors.Is(err, pgx.ErrNoRows) {
		t.Fatalf("cross-workspace create error = %v, want pgx.ErrNoRows", err)
	}

	foreignSystem := createProjectDesignSystemForTest(t, queries, foreignWorkspaceID, foreignProjectID, "Foreign system")
	if _, err := queries.GetProjectDesignSystemInWorkspace(ctx, db.GetProjectDesignSystemInWorkspaceParams{
		ID:          foreignSystem.ID,
		WorkspaceID: parseUUID(testWorkspaceID),
	}); !errors.Is(err, pgx.ErrNoRows) {
		t.Fatalf("cross-workspace read error = %v, want pgx.ErrNoRows", err)
	}
}

func TestProjectDesignSystemPackageRejectsInvalidSlot(t *testing.T) {
	queries := db.New(testPool)
	projectID := createProjectDesignSystemProject(t, testWorkspaceID, "Invalid slot")
	system := createProjectDesignSystemForTest(t, queries, parseUUID(testWorkspaceID), projectID, "Invalid slot")

	_, err := queries.UpsertProjectDesignSystemPackage(context.Background(), db.UpsertProjectDesignSystemPackageParams{
		DesignSystemID:  system.ID,
		Slot:            "published",
		DesignMd:        "# Invalid",
		TokensCss:       ":root { --color-test: #000; }",
		ComponentsHtml:  "<main>Invalid</main>",
		Manifest:        []byte(`{}`),
		Validation:      []byte(`{"passed":true}`),
		IntegritySha256: strings.Repeat("f", 64),
		WorkspaceID:     parseUUID(testWorkspaceID),
	})
	assertPostgresCode(t, err, "23514")
}

func createProjectDesignSystemProject(t *testing.T, workspaceID, title string) pgtype.UUID {
	t.Helper()
	var id pgtype.UUID
	if err := testPool.QueryRow(context.Background(), `
		INSERT INTO project (workspace_id, title, status)
		VALUES ($1, $2, 'planned')
		RETURNING id
	`, workspaceID, fmt.Sprintf("%s-%d", title, time.Now().UnixNano())).Scan(&id); err != nil {
		t.Fatalf("create project: %v", err)
	}
	t.Cleanup(func() {
		_, _ = testPool.Exec(context.Background(), "DELETE FROM project WHERE id = $1", id)
	})
	return id
}

func createProjectDesignSystemWorkspace(t *testing.T) pgtype.UUID {
	t.Helper()
	suffix := time.Now().UnixNano()
	var id pgtype.UUID
	if err := testPool.QueryRow(context.Background(), `
		INSERT INTO workspace (name, slug, description, issue_prefix)
		VALUES ($1, $2, '', $3)
		RETURNING id
	`, fmt.Sprintf("Design System Test %d", suffix), fmt.Sprintf("design-system-test-%d", suffix), "DST").Scan(&id); err != nil {
		t.Fatalf("create workspace: %v", err)
	}
	t.Cleanup(func() {
		_, _ = testPool.Exec(context.Background(), "DELETE FROM workspace WHERE id = $1", id)
	})
	return id
}

func createProjectDesignSystemForTest(
	t *testing.T,
	queries *db.Queries,
	workspaceID pgtype.UUID,
	projectID pgtype.UUID,
	name string,
) db.ProjectDesignSystem {
	t.Helper()
	system, err := queries.CreateProjectDesignSystem(context.Background(), db.CreateProjectDesignSystemParams{
		WorkspaceID:   workspaceID,
		ProjectID:     projectID,
		Name:          name,
		Platform:      "web",
		InputSnapshot: []byte(`{"brief":"persistence test"}`),
		CreatedBy:     parseUUID(testUserID),
	})
	if err != nil {
		t.Fatalf("create project design system: %v", err)
	}
	return system
}

func upsertProjectDesignSystemPackageForTest(
	t *testing.T,
	queries *db.Queries,
	systemID pgtype.UUID,
	slot string,
	designMD string,
	digest string,
) db.ProjectDesignSystemPackage {
	t.Helper()
	pkg, err := queries.UpsertProjectDesignSystemPackage(context.Background(), db.UpsertProjectDesignSystemPackageParams{
		DesignSystemID:  systemID,
		Slot:            slot,
		DesignMd:        designMD,
		TokensCss:       ":root { --color-test: #000; } .test { color: var(--color-test); }",
		ComponentsHtml:  "<main data-design-node-id=\"test\" data-design-node-kind=\"block\" data-design-node-label=\"Test\">Test</main>",
		Manifest:        []byte(`{"schema_version":"test"}`),
		Validation:      []byte(`{"passed":true}`),
		IntegritySha256: digest,
		WorkspaceID:     parseUUID(testWorkspaceID),
	})
	if err != nil {
		t.Fatalf("upsert %s package: %v", slot, err)
	}
	return pkg
}

func assertProjectDesignSystemPackageDigest(
	t *testing.T,
	queries *db.Queries,
	systemID pgtype.UUID,
	slot string,
	want string,
) {
	t.Helper()
	pkg, err := queries.GetProjectDesignSystemPackageBySlot(context.Background(), db.GetProjectDesignSystemPackageBySlotParams{
		DesignSystemID: systemID,
		Slot:           slot,
		WorkspaceID:    parseUUID(testWorkspaceID),
	})
	if err != nil {
		t.Fatalf("get %s package: %v", slot, err)
	}
	if pkg.IntegritySha256 != want {
		t.Fatalf("%s package digest = %q, want %q", slot, pkg.IntegritySha256, want)
	}
}

func assertPostgresCode(t *testing.T, err error, want string) {
	t.Helper()
	var pgErr *pgconn.PgError
	if !errors.As(err, &pgErr) {
		t.Fatalf("error = %v, want PostgreSQL code %s", err, want)
	}
	if pgErr.Code != want {
		t.Fatalf("PostgreSQL code = %s, want %s: %v", pgErr.Code, want, err)
	}
}
