# Semantic Generation Asset Analysis Implementation Plan

> **Recovery note (2026-08-11):** Preserved from `codex/feature-fengchen-dirty-recovery-20260810` as historical context. Do not treat this file as current authority without checking `docs/product/design-center/README.md` and `docs/product/design-center/decision-register.md`.

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Make every uploaded list-page template and project UI specification produce validated, versioned `TemplateBlueprint` and `ComponentRecipeSet` assets that the semantic UI Agent pipeline can compile.

**Architecture:** Keep objective extraction deterministic and delegate semantic classification to the existing `Local UI Restore Agent`. UI specification analysis will continue producing `design_system_profile.profile_json` and will additionally build a RecipeSet; template publication will enqueue a separate Blueprint analysis task. Hidden reanalysis endpoints support existing uploads without requiring a new Figma upload, while invalid classifications are rejected by `designcore` and never become valid compiler inputs.

**Tech Stack:** Go 1.26, Chi handlers, PostgreSQL/sqlc, existing daemon task protocol, `server/internal/designcore`, existing Local UI Restore Agent runtime.

## Global Constraints

- Do not classify component or region semantics with regexes, keyword tables, or backend-maintained vocabulary lists.
- Hidden layers remain excluded by deterministic source extraction; the Agent may only reference IDs present in the supplied visible structure.
- Exported images and complete component subtrees remain source-owned and are preserved by Recipe references rather than redrawn.
- First-release Blueprint page type is only `list`; unsupported structures produce typed validation errors.
- Recipe analysis must cover all `designcore` required kinds: `input`, `select`, `date-range`, `primary-button`, `secondary-button`, `text-button`, `table-header`, `table-row`, `status-tag`, and `pagination`.
- A missing or invalid Blueprint/RecipeSet must never silently fall back to `slot_values` or raw text-layer patches.
- Existing uploaded template and UI specification rows must be re-analyzable without another Figma upload.
- This plan adds no user-facing form or status configuration UI. Reanalysis endpoints are an internal operational capability.
- Preserve the existing default-profile compare-and-set behavior when profile reanalysis completes.
- SQL queries must avoid `JOIN` syntax because the repository pre-receive rule rejects generated `design.sql.go` containing SQL joins.
- Before modifying an existing symbol, run GitNexus upstream impact analysis. Warn before HIGH or CRITICAL changes.
- Before committing, run GitNexus `detect_changes --scope all`, focused tests, and `git diff --check`.
- Leave `.playwright-mcp/`, `.superpowers/`, and `recovery/` untouched.

---

## Files And Responsibilities

- `server/internal/handler/design_file.go`: analysis task enqueueing, output parsing, validated persistence, and existing-asset reanalysis handlers.
- `server/internal/handler/design_plugin.go`: template-upload trigger for Blueprint analysis.
- `server/internal/service/task.go`: typed Blueprint analysis task context.
- `server/internal/service/design_generation_assets.go`: versioned Blueprint/Recipe persistence boundary.
- `server/internal/daemon/{types.go,prompt.go,daemon.go}`: claim transport, Agent prompt, and execution environment for Blueprint analysis.
- `server/internal/daemon/execenv/{execenv.go,context.go,runtime_config.go}`: non-issue task rendering and output contract.
- `server/pkg/db/queries/design.sql`: next analysis-version queries without SQL joins.
- `server/cmd/server/router.go`: authenticated reanalysis endpoints.
- Existing colocated Go tests: RED/GREEN coverage for task transport, validation, persistence, upload triggers, and reanalysis.

## Out Of Scope

- Do not switch `ui_agent_draft_create` from patch output to PageSpec in this plan.
- Do not change `design_draft` persistence or review UI in this plan.
- Do not add modal, drawer, popover, detail-page, form-page, dashboard, or C-end Blueprint support.
- Do not display Blueprint or Recipe JSON in the product UI.

---

### Task 1: Produce Component Recipes During UI Specification Analysis

**Files:**
- Modify: `server/pkg/db/queries/design.sql`
- Regenerate: `server/pkg/db/generated/design.sql.go`
- Modify: `server/internal/handler/design_file.go`
- Modify: `server/internal/handler/design_file_test.go`
- Modify: `server/internal/daemon/prompt.go`
- Modify: `server/internal/daemon/prompt_test.go`
- Modify: `server/internal/daemon/execenv/runtime_config.go`
- Modify: `server/internal/daemon/execenv/runtime_config_test.go`

**Interfaces:**
- Consumes: existing `DesignSystemProfileAnalyzeContext`, visible candidate layers, source Native Design JSON, Agent `profile_json`, and Agent Recipe classifications.
- Produces: one analyzed `design_system_profile` and one valid versioned `design_component_recipe_set` for the same source revision.

- [x] **Step 1: Write failing output-contract and completion tests**

Extend `TestBuildPromptDesignSystemProfileAnalyze` and the runtime-config output test to require the Agent to return:

```json
{
  "profile_json": {"version":"agent-1.0"},
  "analysis_errors": [],
  "summary": "...",
  "recipe_classifications": [
    {
      "kind": "input",
      "variant": "default",
      "state": "default",
      "rootLayerId": "input-root",
      "props": {
        "label": {"targetLayerId":"input-label","type":"text"},
        "placeholder": {"targetLayerId":"input-placeholder","type":"text"}
      },
      "layout": {"widthMode":"fill","textOverflow":"ellipsis","height":36,"minWidth":180}
    }
  ],
  "primitive_fallbacks": {}
}
```

Add handler tests proving that a complete classification creates a valid RecipeSet linked to the analyzed profile, while a missing required kind fails the task and does not create a valid RecipeSet.

- [x] **Step 2: Run RED tests**

Run:

```bash
cd server
DATABASE_URL='postgres://multica:multica@localhost:15435/multica_codex_stage2?sslmode=disable' go test ./internal/handler -run 'TestCompleteDesignSystemProfileAnalyzeTask.*Recipe' -count=1 -v
go test ./internal/daemon -run '^TestBuildPromptDesignSystemProfileAnalyze$' -count=1 -v
go test ./internal/daemon/execenv -run 'Test.*DesignSystemProfileAnalyze.*Output' -count=1 -v
```

Expected: FAIL because the output parser and prompt do not yet require Recipe classifications and no RecipeSet is persisted.

- [x] **Step 3: Add monotonic RecipeSet analysis versions**

Add this sqlc query without a join:

```sql
-- name: GetNextDesignComponentRecipeSetAnalysisVersion :one
SELECT (COALESCE(MAX(analysis_version), 0) + 1)::int
FROM design_component_recipe_set
WHERE workspace_id = $1
  AND design_system_profile_id = $2;
```

Run `make sqlc` and verify generated SQL contains no `JOIN` token in the new query.

- [x] **Step 4: Extend and strictly validate Agent output**

Change `designSystemProfileAnalyzeOutput` to include:

```go
RecipeClassifications []designcore.ComponentRecipeClassification `json:"recipe_classifications"`
PrimitiveFallbacks    map[string]designcore.PrimitiveRecipe       `json:"primitive_fallbacks"`
```

Require at least one classification, reject trailing JSON, and leave semantic validation to `designcore.BuildComponentRecipeSet`. Update the task output policy, daemon prompt, and runtime output instructions to name the exact fields and forbid invented layer IDs.

- [x] **Step 5: Build and persist RecipeSet atomically on valid completion**

In the profile-completion mutation:

1. Load and parse `profileCtx.SourceRevisionID` as `designcore.NativeJSON`.
2. Call `designcore.BuildComponentRecipeSet(profileID, sourceRevisionID, designcore.ComponentRecipeSetVersion, sourceDoc, parsed.RecipeClassifications, parsed.PrimitiveFallbacks)`.
3. Reject diagnostics containing errors with failure reason `design_system_recipe_invalid_output`.
4. Allocate the next analysis version.
5. Save through `service.DesignGenerationAssetStore{Queries: qtx}.SaveRecipeSetAnalysis`.
6. Update the profile to `analyzed` and preserve existing default compare-and-set behavior in the same transaction.

Do not create a valid RecipeSet when `BuildComponentRecipeSet` reports an error.

- [x] **Step 6: Run GREEN tests and formatting**

Run the three commands from Step 2, then:

```bash
cd server
gofmt -w internal/handler/design_file.go internal/handler/design_file_test.go internal/daemon/prompt.go internal/daemon/prompt_test.go internal/daemon/execenv/runtime_config.go internal/daemon/execenv/runtime_config_test.go
go test ./internal/designcore ./internal/service -count=1
```

Expected: all focused tests pass.

---

### Task 2: Add Template Blueprint Analysis Tasks

**Files:**
- Modify: `server/pkg/db/queries/design.sql`
- Regenerate: `server/pkg/db/generated/design.sql.go`
- Modify: `server/internal/service/task.go`
- Modify: `server/internal/service/design_restore_context_test.go`
- Modify: `server/internal/handler/design_file.go`
- Modify: `server/internal/handler/design_file_test.go`
- Modify: `server/internal/handler/agent.go`
- Modify: `server/internal/handler/daemon.go`
- Modify: `server/internal/daemon/types.go`
- Modify: `server/internal/daemon/prompt.go`
- Modify: `server/internal/daemon/prompt_test.go`
- Modify: `server/internal/daemon/daemon.go`
- Modify: `server/internal/daemon/execenv/execenv.go`
- Modify: `server/internal/daemon/execenv/context.go`
- Modify: `server/internal/daemon/execenv/runtime_config.go`
- Modify: corresponding daemon and execenv tests.

**Interfaces:**
- Consumes: deterministic `TemplateStructure` extracted from one published template revision.
- Produces: an Agent `BlueprintClassification`, validated `TemplateBlueprint`, and versioned `design_template_blueprint` record.

- [x] **Step 1: Write failing task-transport and prompt tests**

Define tests for a new context discriminator and claim field:

```go
const DesignTemplateBlueprintAnalyzeContextType = "design_template_blueprint_analyze"

type DesignTemplateBlueprintAnalyzeContext struct {
    Type               string          `json:"type"`
    Prompt             string          `json:"prompt"`
    RequesterID        string          `json:"requester_id"`
    WorkspaceID        string          `json:"workspace_id"`
    ProjectID          string          `json:"project_id"`
    AgentID            string          `json:"agent_id"`
    TemplateID         string          `json:"template_id"`
    TemplateRevisionID string          `json:"template_revision_id"`
    SourceRevisionID   string          `json:"source_revision_id"`
    Structure          json.RawMessage `json:"structure"`
    SourceRefs         json.RawMessage `json:"source_refs"`
    OutputPolicy       json.RawMessage `json:"output_policy"`
}
```

The prompt must request exactly one JSON object with `classification` and `summary`, and must state that every referenced ID must come from `structure`.

- [x] **Step 2: Run RED transport tests**

Run:

```bash
cd server
go test ./internal/service -run 'TestDesignTemplateBlueprintAnalyzeContext' -count=1 -v
go test ./internal/daemon -run 'TestBuildPromptDesignTemplateBlueprintAnalyze' -count=1 -v
go test ./internal/daemon/execenv -run 'Test.*DesignTemplateBlueprintAnalyze' -count=1 -v
```

Expected: FAIL because the task kind is absent.

- [x] **Step 3: Add monotonic Blueprint analysis versions**

Add:

```sql
-- name: GetNextDesignTemplateBlueprintAnalysisVersion :one
SELECT (COALESCE(MAX(analysis_version), 0) + 1)::int
FROM design_template_blueprint
WHERE workspace_id = $1
  AND template_revision_id = $2;
```

Regenerate sqlc and verify the generated query contains no SQL join.

- [x] **Step 4: Implement claim transport and execution context**

Thread `DesignTemplateBlueprintAnalyzeContext` through handler claim response, daemon `Task`, prompt routing, GC classification, `TaskContextForEnv`, context rendering, and runtime output instructions. Treat it as a non-issue task and set its project/workspace from the typed context.

- [x] **Step 5: Implement strict output parsing and validated persistence**

Add:

```go
type designTemplateBlueprintAnalyzeOutput struct {
    Classification designcore.BlueprintClassification `json:"classification"`
    Summary        string                             `json:"summary"`
}
```

Parse with `json.Decoder.DisallowUnknownFields`, reject trailing JSON, require a non-empty summary, rebuild the Blueprint with `designcore.BuildTemplateBlueprint`, and save through `DesignGenerationAssetStore.SaveBlueprintAnalysis`. Invalid classifications fail the task with `design_template_blueprint_invalid_output` and never become valid inputs.

- [x] **Step 6: Run GREEN transport/completion tests**

Run Step 2 commands plus focused handler completion tests covering valid, unknown-layer, hidden-layer, and unsupported-page classifications. Format changed Go files and require all commands to pass.

---

### Task 3: Trigger Analysis On Upload And Support Existing Assets

**Files:**
- Modify: `server/internal/handler/design_file.go`
- Modify: `server/internal/handler/design_plugin.go`
- Modify: `server/internal/handler/design_file_test.go`
- Modify: `server/cmd/server/router.go`
- Modify: `server/internal/handler/handler_test.go` only if route-level auth coverage requires it.

**Interfaces:**
- Consumes: successful template publication, successful UI specification profile creation, or an authenticated reanalysis request.
- Produces: queued analysis task IDs without requiring another Figma upload.

- [x] **Step 1: Write failing upload-trigger tests**

Add tests proving:

- plugin template upload queues one `design_template_blueprint_analyze` task;
- API template publication queues the same task;
- UI specification upload still queues one profile analysis task whose output policy now requires Recipe classifications;
- no Local UI Restore Agent causes the template/profile publication transaction to fail without leaving a published semantic asset that can never be analyzed;
- duplicate reanalysis requests do not queue a second active task for the same source revision.

- [x] **Step 2: Write failing existing-asset reanalysis tests**

Add authenticated handlers:

```text
POST /api/design-templates/{id}/analyze-blueprint
POST /api/design-system-profiles/{id}/reanalyze
```

Expected response:

```json
{"task_id":"uuid","status":"queued"}
```

The handlers must validate workspace/project ownership, use the current source revision, and reject an already queued/running task with HTTP 409.

- [x] **Step 3: Implement shared enqueue helpers**

Create helpers that select `Local UI Restore Agent`, calculate deterministic structure/candidate payloads, create the task inside the caller's transaction, and notify `TaskService` only after commit. Reuse the helpers from API publication, Figma plugin import, and reanalysis handlers.

- [x] **Step 4: Add route wiring and focused route tests**

Register both routes inside the existing authenticated design route group in `server/cmd/server/router.go`. Verify unauthenticated access is rejected by the existing middleware and cross-workspace IDs do not leak asset existence.

- [x] **Step 5: Run GREEN upload and reanalysis tests**

Run:

```bash
cd server
DATABASE_URL='postgres://multica:multica@localhost:15435/multica_codex_stage2?sslmode=disable' go test ./internal/handler -run 'Test(FigmaPluginImport.*Template.*Blueprint|PublishDesignRevisionAsTemplate.*Blueprint|ReanalyzeDesign)' -count=1 -v
```

Expected: all focused tests pass and task counts remain idempotent.

---

### Task 4: Verify The CRM Assets Without Reuploading

**Files:**
- No production code unless a focused test exposes a defect.
- Read-only verification against `multica_restored_20260722`, followed by authenticated reanalysis requests.

**Interfaces:**
- Consumes: existing CRM template `38dc063d-8670-41e4-92ca-9bee3598f932` and default profile `4a889cd5-7e1a-4d25-96fb-9e5d4ec6cfbc`.
- Produces: one valid Blueprint and one valid RecipeSet for project `8962f4ee-817a-49b1-8902-ab408406a444`.

- [x] **Step 1: Run complete focused verification**

Run:

```bash
cd server
DATABASE_URL='postgres://multica:multica@localhost:15435/multica_codex_stage2?sslmode=disable' go test ./internal/handler -run 'Test(CompleteDesignSystemProfileAnalyzeTask|CompleteDesignTemplateBlueprintAnalyzeTask|ReanalyzeDesign|FigmaPluginImportCanPublishTemplate)' -count=1
go test ./internal/designcore ./internal/service ./internal/daemon ./internal/daemon/execenv -count=1
cd ..
git diff --check
node .gitnexus/run.cjs detect_changes --scope all
```

- [x] **Step 2: Rebuild and restart only the required binaries**

Build `server/bin/multica`, restart the backend against `localhost:15435/multica_restored_20260722`, and restart the daemon with the established direct foreground command. Do not run `make dev` or `make start`.

- [x] **Step 3: Reanalyze the existing CRM assets**

Call the two authenticated reanalysis endpoints for the IDs above. Monitor the corresponding Agent tasks until terminal state; do not create a UI draft task yet.

- [x] **Step 4: Verify persisted semantic assets**

Query by exact source identities and require:

- latest Blueprint status `valid`, schema `1.0`, source revision equal to the template's current design revision;
- latest RecipeSet status `valid`, schema `1.0`, source revision equal to the default profile source revision;
- RecipeSet contains all ten required component kinds;
- no hidden source layer appears as a Recipe root;
- CRM default profile remains `analyzed + is_default=true`.

- [x] **Step 5: Record the next boundary**

After this plan passes, create a separate implementation plan for `UI Agent -> PageSpec -> CompileListPage -> compiled DesignDraft -> approval`. Do not claim semantic UI generation is complete at the end of this asset-analysis plan.

Recorded in `docs/superpowers/plans/2026-07-23-ui-agent-semantic-draft-workflow.md`.
