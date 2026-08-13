package service

import (
	"context"
	"encoding/json"
	"errors"
	"reflect"
	"strings"
	"testing"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgtype"
	"github.com/multica-ai/multica/server/internal/projectdesignsystem"
	"github.com/multica-ai/multica/server/internal/util"
	db "github.com/multica-ai/multica/server/pkg/db/generated"
)

func TestProjectDesignContextResolverUsesOnlyValidatedSavedPackage(t *testing.T) {
	workspaceID := mustDesignContextUUID(t, "e2f576ee-5a61-4844-8dee-719996169571")
	projectID := mustDesignContextUUID(t, "79560402-5bd7-420a-9e16-79e06557507a")
	systemID := mustDesignContextUUID(t, "317ac5d7-00b8-4abd-b4ce-df2ed9f695de")
	sourceTaskID := mustDesignContextUUID(t, "57ec9b56-6fac-4799-a438-e4926443c94e")
	system, saved := validSavedDesignContextFixture(t, workspaceID, projectID, systemID, sourceTaskID)
	store := &fakeProjectDesignContextStore{system: system, saved: saved}

	resolved, err := (ProjectDesignContextResolver{Store: store}).Resolve(context.Background(), ResolveProjectDesignContextParams{
		WorkspaceID: workspaceID,
		ProjectID:   projectID,
	})
	if err != nil {
		t.Fatalf("Resolve() error = %v", err)
	}

	wantPriority := []DesignContextSource{
		DesignContextSourceCloudSaved,
		DesignContextSourceLocalDesignMD,
		DesignContextSourceRepositoryReality,
	}
	if resolved.Version != DesignContextVersion || resolved.Source != DesignContextSourceCloudSaved {
		t.Fatalf("resolved identity = version:%q source:%q", resolved.Version, resolved.Source)
	}
	if resolved.ProjectID != util.UUIDToString(projectID) || !reflect.DeepEqual(resolved.Priority, wantPriority) {
		t.Fatalf("resolved project/priority = project:%q priority:%v", resolved.ProjectID, resolved.Priority)
	}
	if resolved.Digest != saved.IntegritySha256 || resolved.Package == nil {
		t.Fatalf("resolved digest/package = digest:%q package:%#v", resolved.Digest, resolved.Package)
	}
	if resolved.Package.DesignSystemID != util.UUIDToString(systemID) || resolved.Package.SourceTaskID != util.UUIDToString(sourceTaskID) {
		t.Fatalf("resolved package trace = %#v", resolved.Package)
	}
	if resolved.Package.Name != system.Name || resolved.Package.Platform != system.Platform {
		t.Fatalf("resolved package identity = %#v", resolved.Package)
	}
	wantArtifacts := projectdesignsystem.ArtifactInput{
		DesignMD:       saved.DesignMd,
		TokensCSS:      saved.TokensCss,
		ComponentsHTML: saved.ComponentsHtml,
	}
	if resolved.Package.Artifacts != wantArtifacts {
		t.Fatalf("resolved artifacts = %#v", resolved.Package.Artifacts)
	}
	if len(store.packageSlots) != 1 || store.packageSlots[0] != "saved" {
		t.Fatalf("queried package slots = %v, want only saved", store.packageSlots)
	}

	encoded, err := json.Marshal(resolved)
	if err != nil {
		t.Fatalf("marshal resolved context: %v", err)
	}
	var payload map[string]any
	if err := json.Unmarshal(encoded, &payload); err != nil {
		t.Fatalf("decode resolved context: %v", err)
	}
	if payload["source"] != string(DesignContextSourceCloudSaved) || payload["digest"] != saved.IntegritySha256 {
		t.Fatalf("traceable JSON contract = %#v", payload)
	}
	pack, ok := payload["package"].(map[string]any)
	if !ok || pack["design_system_id"] != util.UUIDToString(systemID) {
		t.Fatalf("package JSON = %#v", payload["package"])
	}
}

func TestProjectDesignContextResolverLeavesLocalFallbackForMissingCloudSaved(t *testing.T) {
	workspaceID := mustDesignContextUUID(t, "e2f576ee-5a61-4844-8dee-719996169571")
	projectID := mustDesignContextUUID(t, "79560402-5bd7-420a-9e16-79e06557507a")
	systemID := mustDesignContextUUID(t, "317ac5d7-00b8-4abd-b4ce-df2ed9f695de")
	sourceTaskID := mustDesignContextUUID(t, "57ec9b56-6fac-4799-a438-e4926443c94e")
	system, _ := validSavedDesignContextFixture(t, workspaceID, projectID, systemID, sourceTaskID)

	tests := []struct {
		name  string
		store *fakeProjectDesignContextStore
	}{
		{name: "no project design system", store: &fakeProjectDesignContextStore{systemErr: pgx.ErrNoRows}},
		{name: "no saved package", store: &fakeProjectDesignContextStore{system: system, savedErr: pgx.ErrNoRows}},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			resolved, err := (ProjectDesignContextResolver{Store: test.store}).Resolve(context.Background(), ResolveProjectDesignContextParams{
				WorkspaceID: workspaceID,
				ProjectID:   projectID,
			})
			if err != nil {
				t.Fatalf("Resolve() error = %v", err)
			}
			if resolved.Source != DesignContextSourceNone || resolved.Digest != "" || resolved.Package != nil {
				t.Fatalf("missing cloud resolution = %#v", resolved)
			}
			wantPriority := []DesignContextSource{
				DesignContextSourceCloudSaved,
				DesignContextSourceLocalDesignMD,
				DesignContextSourceRepositoryReality,
			}
			if !reflect.DeepEqual(resolved.Priority, wantPriority) {
				t.Fatalf("priority = %v, want %v", resolved.Priority, wantPriority)
			}
		})
	}
}

func TestProjectDesignContextResolverRejectsInvalidSavedPackage(t *testing.T) {
	workspaceID := mustDesignContextUUID(t, "e2f576ee-5a61-4844-8dee-719996169571")
	projectID := mustDesignContextUUID(t, "79560402-5bd7-420a-9e16-79e06557507a")
	systemID := mustDesignContextUUID(t, "317ac5d7-00b8-4abd-b4ce-df2ed9f695de")
	sourceTaskID := mustDesignContextUUID(t, "57ec9b56-6fac-4799-a438-e4926443c94e")
	system, validSaved := validSavedDesignContextFixture(t, workspaceID, projectID, systemID, sourceTaskID)

	tests := []struct {
		name   string
		mutate func(*db.ProjectDesignSystemPackage)
	}{
		{name: "render not passed", mutate: func(saved *db.ProjectDesignSystemPackage) { saved.RenderStatus = "pending" }},
		{name: "digest mismatch", mutate: func(saved *db.ProjectDesignSystemPackage) { saved.IntegritySha256 = strings.Repeat("f", 64) }},
		{name: "stored validation failed", mutate: func(saved *db.ProjectDesignSystemPackage) {
			saved.Validation = []byte(`{"passed":false,"diagnostics":[]}`)
		}},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			saved := validSaved
			test.mutate(&saved)
			_, err := (ProjectDesignContextResolver{Store: &fakeProjectDesignContextStore{system: system, saved: saved}}).Resolve(
				context.Background(),
				ResolveProjectDesignContextParams{WorkspaceID: workspaceID, ProjectID: projectID},
			)
			if !errors.Is(err, ErrSavedDesignContextInvalid) {
				t.Fatalf("Resolve() error = %v, want ErrSavedDesignContextInvalid", err)
			}
		})
	}
}

func TestProjectDesignContextResolverReturnsStoreFailures(t *testing.T) {
	workspaceID := mustDesignContextUUID(t, "e2f576ee-5a61-4844-8dee-719996169571")
	projectID := mustDesignContextUUID(t, "79560402-5bd7-420a-9e16-79e06557507a")
	systemID := mustDesignContextUUID(t, "317ac5d7-00b8-4abd-b4ce-df2ed9f695de")
	system := db.ProjectDesignSystem{ID: systemID, WorkspaceID: workspaceID, ProjectID: projectID}

	tests := []struct {
		name  string
		store *fakeProjectDesignContextStore
		want  string
	}{
		{name: "system lookup", store: &fakeProjectDesignContextStore{systemErr: errors.New("database unavailable")}, want: "load project design system"},
		{name: "saved lookup", store: &fakeProjectDesignContextStore{system: system, savedErr: errors.New("database unavailable")}, want: "load saved project design system package"},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			_, err := (ProjectDesignContextResolver{Store: test.store}).Resolve(context.Background(), ResolveProjectDesignContextParams{
				WorkspaceID: workspaceID,
				ProjectID:   projectID,
			})
			if err == nil || !strings.Contains(err.Error(), test.want) {
				t.Fatalf("Resolve() error = %v, want %q", err, test.want)
			}
		})
	}
}

type fakeProjectDesignContextStore struct {
	system       db.ProjectDesignSystem
	systemErr    error
	saved        db.ProjectDesignSystemPackage
	savedErr     error
	packageSlots []string
}

func (s *fakeProjectDesignContextStore) GetProjectDesignSystemByProject(_ context.Context, _ db.GetProjectDesignSystemByProjectParams) (db.ProjectDesignSystem, error) {
	return s.system, s.systemErr
}

func (s *fakeProjectDesignContextStore) GetProjectDesignSystemPackageBySlot(_ context.Context, params db.GetProjectDesignSystemPackageBySlotParams) (db.ProjectDesignSystemPackage, error) {
	s.packageSlots = append(s.packageSlots, params.Slot)
	return s.saved, s.savedErr
}

func validSavedDesignContextFixture(
	t *testing.T,
	workspaceID pgtype.UUID,
	projectID pgtype.UUID,
	systemID pgtype.UUID,
	sourceTaskID pgtype.UUID,
) (db.ProjectDesignSystem, db.ProjectDesignSystemPackage) {
	t.Helper()
	artifacts := projectdesignsystem.ArtifactInput{
		DesignMD:       "# Atlas System\n\n## Principles\n\n- Keep actions clear.\n",
		TokensCSS:      ":root { --color-action-primary: #2563eb; }\n.primary { background: var(--color-action-primary); }\n",
		ComponentsHTML: `<main data-design-node-id="overview" data-design-node-kind="block" data-design-node-label="Overview"><button class="primary" data-design-node-id="button-primary" data-design-node-kind="component" data-design-node-label="Primary button">Save</button></main>`,
	}
	validated, err := projectdesignsystem.Validate(artifacts, nil)
	if err != nil {
		t.Fatalf("validate fixture: %v", err)
	}
	manifest, err := json.Marshal(validated.Manifest)
	if err != nil {
		t.Fatalf("marshal fixture manifest: %v", err)
	}
	validation, err := json.Marshal(validated.Validation)
	if err != nil {
		t.Fatalf("marshal fixture validation: %v", err)
	}
	savedAt := time.Date(2026, time.July, 30, 3, 58, 25, 0, time.UTC)
	return db.ProjectDesignSystem{
			ID:          systemID,
			WorkspaceID: workspaceID,
			ProjectID:   projectID,
			Name:        "Atlas",
			Platform:    "web",
			SavedAt:     pgtype.Timestamptz{Time: savedAt, Valid: true},
		}, db.ProjectDesignSystemPackage{
			DesignSystemID:  systemID,
			Slot:            "saved",
			DesignMd:        artifacts.DesignMD,
			TokensCss:       artifacts.TokensCSS,
			ComponentsHtml:  artifacts.ComponentsHTML,
			Manifest:        manifest,
			Validation:      validation,
			IntegritySha256: validated.Manifest.Digest,
			SourceTaskID:    sourceTaskID,
			RenderStatus:    "passed",
		}
}

func mustDesignContextUUID(t *testing.T, value string) pgtype.UUID {
	t.Helper()
	id, err := util.ParseUUID(value)
	if err != nil {
		t.Fatalf("parse UUID %q: %v", value, err)
	}
	return id
}
