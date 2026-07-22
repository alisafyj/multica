package service

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgtype"
	"github.com/multica-ai/multica/server/internal/designcore"
	"github.com/multica-ai/multica/server/internal/util"
	db "github.com/multica-ai/multica/server/pkg/db/generated"
)

var ErrGenerationAssetsMissing = errors.New("semantic design generation assets are missing")
var ErrGenerationAssetsStale = errors.New("semantic design generation assets do not match source revisions")

type GenerationAssetValidationError struct {
	Diagnostics designcore.Diagnostics
}

func (e *GenerationAssetValidationError) Error() string {
	return "semantic design generation assets failed validation"
}

type DesignGenerationAssetStore struct {
	Queries *db.Queries
}

type CompilationAssets struct {
	Blueprint         designcore.TemplateBlueprint
	RecipeSet         designcore.ComponentRecipeSet
	TemplateDoc       designcore.NativeJSON
	RecipeDoc         designcore.NativeJSON
	BlueprintRecordID string
	RecipeSetRecordID string
}

type SaveBlueprintAnalysisParams struct {
	WorkspaceID, TemplateID, TemplateRevisionID, SourceRevisionID pgtype.UUID
	AnalysisVersion                                               int32
	CreatedBy                                                     pgtype.UUID
	Structure                                                     designcore.TemplateStructure
	Blueprint                                                     designcore.TemplateBlueprint
}

type SaveRecipeSetAnalysisParams struct {
	WorkspaceID, DesignSystemProfileID, SourceRevisionID pgtype.UUID
	AnalysisVersion                                      int32
	CreatedBy                                            pgtype.UUID
	RecipeSet                                            designcore.ComponentRecipeSet
}

type LoadCompilationAssetsParams struct {
	WorkspaceID, TemplateRevisionID, DesignSystemProfileID pgtype.UUID
}

func (s DesignGenerationAssetStore) SaveBlueprintAnalysis(ctx context.Context, params SaveBlueprintAnalysisParams) (db.DesignTemplateBlueprint, error) {
	templateRevision, err := s.Queries.GetDesignTemplateRevisionInWorkspace(ctx, db.GetDesignTemplateRevisionInWorkspaceParams{
		ID: params.TemplateRevisionID, WorkspaceID: params.WorkspaceID,
	})
	if err != nil {
		return db.DesignTemplateBlueprint{}, fmt.Errorf("load template revision: %w", err)
	}
	if templateRevision.TemplateID != params.TemplateID || templateRevision.DesignRevisionID != params.SourceRevisionID {
		return db.DesignTemplateBlueprint{}, staleGenerationAssets("blueprint source identity does not match its template revision")
	}

	sourceRevision, err := s.Queries.GetDesignRevisionInWorkspace(ctx, db.GetDesignRevisionInWorkspaceParams{
		ID: params.SourceRevisionID, WorkspaceID: params.WorkspaceID,
	})
	if err != nil {
		return db.DesignTemplateBlueprint{}, fmt.Errorf("load blueprint source revision: %w", err)
	}
	sourceDoc, err := designcore.ParseNativeJSON(sourceRevision.NativeJson)
	if err != nil {
		return db.DesignTemplateBlueprint{}, fmt.Errorf("parse blueprint source document: %w", err)
	}

	diagnostics := designcore.ValidateTemplateBlueprint(designcore.ExtractTemplateStructure(sourceDoc), params.Blueprint)
	if !blueprintSourceRefsMatch(params.Blueprint, sourceRevision, params.TemplateRevisionID) {
		diagnostics = append(diagnostics, designcore.Diagnostic{
			Code: "invalid_blueprint_source", Severity: designcore.DiagnosticError,
			Message: "blueprint source references do not match the persisted template revision", Paths: []string{"sourceRefs"},
		})
	}
	structureJSON, err := json.Marshal(params.Structure)
	if err != nil {
		return db.DesignTemplateBlueprint{}, fmt.Errorf("marshal blueprint structure: %w", err)
	}
	blueprintJSON, err := json.Marshal(params.Blueprint)
	if err != nil {
		return db.DesignTemplateBlueprint{}, fmt.Errorf("marshal template blueprint: %w", err)
	}
	validationErrors, err := json.Marshal(diagnostics)
	if err != nil {
		return db.DesignTemplateBlueprint{}, fmt.Errorf("marshal blueprint diagnostics: %w", err)
	}
	status := "valid"
	if diagnostics.HasErrors() {
		status = "invalid"
	}
	record, err := s.Queries.CreateDesignTemplateBlueprint(ctx, db.CreateDesignTemplateBlueprintParams{
		WorkspaceID: params.WorkspaceID, TemplateID: params.TemplateID, TemplateRevisionID: params.TemplateRevisionID,
		SourceRevisionID: params.SourceRevisionID, AnalysisVersion: params.AnalysisVersion, SchemaVersion: designcore.TemplateBlueprintVersion,
		Status: status, StructureJson: structureJSON, BlueprintJson: blueprintJSON, ValidationErrors: validationErrors, CreatedBy: params.CreatedBy,
	})
	if err != nil {
		return db.DesignTemplateBlueprint{}, fmt.Errorf("create template blueprint: %w", err)
	}
	if diagnostics.HasErrors() {
		return record, &GenerationAssetValidationError{Diagnostics: diagnostics}
	}
	return record, nil
}

func (s DesignGenerationAssetStore) SaveRecipeSetAnalysis(ctx context.Context, params SaveRecipeSetAnalysisParams) (db.DesignComponentRecipeSet, error) {
	profile, err := s.Queries.GetDesignSystemProfileInWorkspace(ctx, db.GetDesignSystemProfileInWorkspaceParams{
		ID: params.DesignSystemProfileID, WorkspaceID: params.WorkspaceID,
	})
	if err != nil {
		return db.DesignComponentRecipeSet{}, fmt.Errorf("load design system profile: %w", err)
	}
	if profile.SourceRevisionID != params.SourceRevisionID {
		return db.DesignComponentRecipeSet{}, staleGenerationAssets("recipe source identity does not match its design system profile")
	}
	sourceRevision, err := s.Queries.GetDesignRevisionInWorkspace(ctx, db.GetDesignRevisionInWorkspaceParams{
		ID: params.SourceRevisionID, WorkspaceID: params.WorkspaceID,
	})
	if err != nil {
		return db.DesignComponentRecipeSet{}, fmt.Errorf("load recipe source revision: %w", err)
	}
	sourceDoc, err := designcore.ParseNativeJSON(sourceRevision.NativeJson)
	if err != nil {
		return db.DesignComponentRecipeSet{}, fmt.Errorf("parse recipe source document: %w", err)
	}

	diagnostics := designcore.ValidateComponentRecipeSet(sourceDoc, params.RecipeSet)
	if params.RecipeSet.DesignSystemProfileID != util.UUIDToString(profile.ID) || params.RecipeSet.SourceRevisionID != util.UUIDToString(sourceRevision.ID) {
		diagnostics = append(diagnostics, designcore.Diagnostic{
			Code: "invalid_recipe_set", Severity: designcore.DiagnosticError,
			Message: "recipe set identity does not match persisted sources", Paths: []string{"designSystemProfileId", "sourceRevisionId"},
		})
	}
	recipeSetJSON, err := json.Marshal(params.RecipeSet)
	if err != nil {
		return db.DesignComponentRecipeSet{}, fmt.Errorf("marshal component recipe set: %w", err)
	}
	validationErrors, err := json.Marshal(diagnostics)
	if err != nil {
		return db.DesignComponentRecipeSet{}, fmt.Errorf("marshal recipe diagnostics: %w", err)
	}
	status := "valid"
	if diagnostics.HasErrors() {
		status = "invalid"
	}
	record, err := s.Queries.CreateDesignComponentRecipeSet(ctx, db.CreateDesignComponentRecipeSetParams{
		WorkspaceID: params.WorkspaceID, DesignSystemProfileID: params.DesignSystemProfileID, SourceRevisionID: params.SourceRevisionID,
		AnalysisVersion: params.AnalysisVersion, SchemaVersion: designcore.ComponentRecipeSetVersion, Status: status,
		RecipesJson: recipeSetJSON, ValidationErrors: validationErrors, CreatedBy: params.CreatedBy,
	})
	if err != nil {
		return db.DesignComponentRecipeSet{}, fmt.Errorf("create component recipe set: %w", err)
	}
	if diagnostics.HasErrors() {
		return record, &GenerationAssetValidationError{Diagnostics: diagnostics}
	}
	return record, nil
}

func (s DesignGenerationAssetStore) LoadCompilationAssets(ctx context.Context, params LoadCompilationAssetsParams) (CompilationAssets, error) {
	blueprintRecord, err := s.Queries.GetLatestValidDesignTemplateBlueprint(ctx, db.GetLatestValidDesignTemplateBlueprintParams{
		WorkspaceID: params.WorkspaceID, TemplateRevisionID: params.TemplateRevisionID,
	})
	if errors.Is(err, pgx.ErrNoRows) {
		return CompilationAssets{}, ErrGenerationAssetsMissing
	}
	if err != nil {
		return CompilationAssets{}, fmt.Errorf("load latest template blueprint: %w", err)
	}
	recipeSetRecord, err := s.Queries.GetLatestValidDesignComponentRecipeSet(ctx, db.GetLatestValidDesignComponentRecipeSetParams{
		WorkspaceID: params.WorkspaceID, DesignSystemProfileID: params.DesignSystemProfileID,
	})
	if errors.Is(err, pgx.ErrNoRows) {
		return CompilationAssets{}, ErrGenerationAssetsMissing
	}
	if err != nil {
		return CompilationAssets{}, fmt.Errorf("load latest component recipe set: %w", err)
	}

	templateRevision, err := s.Queries.GetDesignTemplateRevisionInWorkspace(ctx, db.GetDesignTemplateRevisionInWorkspaceParams{
		ID: params.TemplateRevisionID, WorkspaceID: params.WorkspaceID,
	})
	if err != nil {
		return CompilationAssets{}, staleGenerationAssets("template revision is unavailable")
	}
	profile, err := s.Queries.GetDesignSystemProfileInWorkspace(ctx, db.GetDesignSystemProfileInWorkspaceParams{
		ID: params.DesignSystemProfileID, WorkspaceID: params.WorkspaceID,
	})
	if err != nil {
		return CompilationAssets{}, staleGenerationAssets("design system profile is unavailable")
	}
	if blueprintRecord.TemplateRevisionID != templateRevision.ID || blueprintRecord.SourceRevisionID != templateRevision.DesignRevisionID || recipeSetRecord.DesignSystemProfileID != profile.ID || recipeSetRecord.SourceRevisionID != profile.SourceRevisionID {
		return CompilationAssets{}, staleGenerationAssets("asset records do not match current source identities")
	}

	templateSourceRevision, err := s.Queries.GetDesignRevisionInWorkspace(ctx, db.GetDesignRevisionInWorkspaceParams{
		ID: blueprintRecord.SourceRevisionID, WorkspaceID: params.WorkspaceID,
	})
	if err != nil {
		return CompilationAssets{}, staleGenerationAssets("template source revision is unavailable")
	}
	recipeSourceRevision, err := s.Queries.GetDesignRevisionInWorkspace(ctx, db.GetDesignRevisionInWorkspaceParams{
		ID: recipeSetRecord.SourceRevisionID, WorkspaceID: params.WorkspaceID,
	})
	if err != nil {
		return CompilationAssets{}, staleGenerationAssets("recipe source revision is unavailable")
	}
	templateDoc, err := designcore.ParseNativeJSON(templateSourceRevision.NativeJson)
	if err != nil {
		return CompilationAssets{}, staleGenerationAssets("template source document is invalid")
	}
	recipeDoc, err := designcore.ParseNativeJSON(recipeSourceRevision.NativeJson)
	if err != nil {
		return CompilationAssets{}, staleGenerationAssets("recipe source document is invalid")
	}
	blueprint, err := designcore.ParseTemplateBlueprint(blueprintRecord.BlueprintJson)
	if err != nil {
		return CompilationAssets{}, fmt.Errorf("parse persisted template blueprint: %w", err)
	}
	recipeSet, err := designcore.ParseComponentRecipeSet(recipeSetRecord.RecipesJson)
	if err != nil {
		return CompilationAssets{}, fmt.Errorf("parse persisted component recipe set: %w", err)
	}
	if !blueprintSourceRefsMatch(blueprint, templateSourceRevision, templateRevision.ID) || recipeSet.DesignSystemProfileID != util.UUIDToString(profile.ID) || recipeSet.SourceRevisionID != util.UUIDToString(recipeSourceRevision.ID) {
		return CompilationAssets{}, staleGenerationAssets("parsed assets do not match current source identities")
	}
	if designcore.ValidateTemplateBlueprint(designcore.ExtractTemplateStructure(templateDoc), blueprint).HasErrors() || designcore.ValidateComponentRecipeSet(recipeDoc, recipeSet).HasErrors() {
		return CompilationAssets{}, staleGenerationAssets("asset content no longer matches source documents")
	}

	return CompilationAssets{
		Blueprint: blueprint, RecipeSet: recipeSet, TemplateDoc: templateDoc, RecipeDoc: recipeDoc,
		BlueprintRecordID: util.UUIDToString(blueprintRecord.ID), RecipeSetRecordID: util.UUIDToString(recipeSetRecord.ID),
	}, nil
}

func blueprintSourceRefsMatch(blueprint designcore.TemplateBlueprint, sourceRevision db.DesignRevision, templateRevisionID pgtype.UUID) bool {
	return blueprint.SourceRefs.DesignFileID == util.UUIDToString(sourceRevision.FileID) &&
		blueprint.SourceRefs.DesignRevisionID == util.UUIDToString(sourceRevision.ID) &&
		blueprint.SourceRefs.TemplateRevisionID == util.UUIDToString(templateRevisionID)
}

func staleGenerationAssets(detail string) error {
	return fmt.Errorf("%w: %s", ErrGenerationAssetsStale, detail)
}
