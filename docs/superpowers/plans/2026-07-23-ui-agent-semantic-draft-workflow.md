# UI Agent Semantic Draft Workflow Implementation Plan

> **Recovery note (2026-08-11):** Preserved from `codex/feature-fengchen-dirty-recovery-20260810` as historical context. Do not treat this file as current authority without checking `docs/product/design-center/README.md` and `docs/product/design-center/decision-register.md`.

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Replace UI Agent slot/patch generation with a strict `PageSpec -> CompileListPage -> reviewable DesignDraft -> approval` workflow for B-end list pages.

**Architecture:** The UI Agent owns business and information-design decisions in `PageSpec`; it never emits layer IDs, geometry, slots, or JSON patches. The server loads the selected template Blueprint and project RecipeSet, compiles deterministically with `designcore.CompileListPage`, persists the compiled Native Design JSON plus lineage and quality evidence, and only exposes `generated` or `generated_with_warnings` drafts for review. Approval materializes the compiled document; critical compiler failures remain diagnostic records and never appear as normal reviewable drafts.

**Tech Stack:** Go 1.26, PostgreSQL/sqlc, existing daemon task protocol, `server/internal/designcore`, React Query, Next.js shared views.

## Global Constraints

- First release supports only `PageSpec.page.type = "list"`.
- UI Agent output contains exactly `title`, `catalog_template_id`, and `page_spec`.
- UI Agent output must contain no layer IDs, coordinates, `slot_values`, or JSON patch operations.
- Cloud assets are authoritative: latest valid project `TemplateBlueprint` + latest valid default-profile `ComponentRecipeSet`.
- `compile_failed` must not emit `design_draft:ready` and must not appear in the normal pending-review list.
- `generated_with_warnings` is reviewable only when the quality report contains no error diagnostics.
- Existing manual/legacy draft APIs remain readable during migration; no new UI Agent task may create a legacy patch draft.
- Approval validates the compiled Native JSON again before creating the formal design file and revision.
- Revision notes create a new PageSpec/draft version; they never patch the compiled Native JSON directly.
- SQL queries must not introduce `JOIN` syntax in generated `design.sql.go`.
- Before changing an existing symbol, run GitNexus upstream impact analysis and warn on HIGH or CRITICAL risk.
- Before committing, run focused tests, `git diff --check`, and `node .gitnexus/run.cjs detect_changes --scope all`.
- Leave `.playwright-mcp/`, `.superpowers/`, and `recovery/` untouched.

---

### Task 1: Replace The UI Agent Output Contract With PageSpec

**Files:**
- Modify: `server/internal/service/task.go`
- Modify: `server/internal/handler/design_file.go`
- Modify: `server/internal/handler/design_file_test.go`
- Modify: `server/internal/daemon/prompt.go`
- Modify: `server/internal/daemon/prompt_test.go`
- Modify: `server/internal/daemon/execenv/runtime_config.go`
- Modify: `server/internal/daemon/execenv/runtime_config_test.go`

**Interfaces:**
- Consumes: Issue + parent Issue, template candidates, analyzed default profile, valid Blueprint/Recipe summaries.
- Produces: strict Agent output `{title, catalog_template_id, page_spec}` and a typed `UIDraftCreateContext` with project/profile identity and optional base-draft revision context.

- [ ] **Step 1: Write failing context and prompt tests**

Require the task context to include `design_system_profile_id`, candidate `template_revision_id`, Blueprint availability, Recipe kinds, `required_requirement_ids`, and optional `base_draft_id/revision_note`. Require the prompt to embed the exact `PageSpec 1.0` shape and explicitly forbid `slot_values`, `patch`, layer IDs, and geometry.

- [ ] **Step 2: Run RED tests**

```bash
cd server
DATABASE_URL='postgres://multica:multica@localhost:15435/multica_codex_stage2?sslmode=disable' go test ./internal/handler -run 'Test(CreateDesignDraftAgentTask|ClaimUIDraftCreateTask).*PageSpec' -count=1 -v
go test ./internal/daemon -run 'TestBuildPromptUIDraft.*PageSpec' -count=1 -v
go test ./internal/daemon/execenv -run 'Test.*UIDraft.*PageSpec' -count=1 -v
```

Expected: FAIL because the current path still requests slot values and safe patches.

- [ ] **Step 3: Implement the strict task contract**

Change the typed output to:

```go
type uiDraftAgentOutput struct {
    Title             string          `json:"title"`
    CatalogTemplateID string          `json:"catalog_template_id"`
    PageSpec          json.RawMessage `json:"page_spec"`
}
```

Use `json.Decoder.DisallowUnknownFields`, reject trailing JSON, and call `designcore.ParsePageSpec`. Candidate summaries may expose semantic page type, regions, constraints, and Recipe kinds, but must not expose raw template/UI-spec layer trees.

- [ ] **Step 4: Run GREEN tests and format**

Run the commands from Step 2, then `gofmt` all changed Go files.

---

### Task 2: Persist Semantic Draft Lineage And Compiler Evidence

**Files:**
- Create: `server/migrations/129_semantic_design_draft.up.sql`
- Create: `server/migrations/129_semantic_design_draft.down.sql`
- Modify: `server/pkg/db/queries/design.sql`
- Regenerate: `server/pkg/db/generated/design.sql.go`
- Modify: `server/internal/service/design_generation_assets.go`
- Modify: `server/internal/service/design_generation_assets_test.go`

**Interfaces:**
- Consumes: `PageSpec`, selected template revision, default profile, Blueprint/Recipe record IDs, compiler output and quality report.
- Produces: versioned semantic draft storage without repurposing `slot_values` or `patch`.

- [ ] **Step 1: Write failing persistence tests**

Require semantic drafts to persist: `generation_mode`, `page_spec`, `compiled_native_json`, `quality_report`, `blueprint_id`, `recipe_set_id`, `parent_draft_id`, and monotonic `version`. Require a query that returns the latest version for an Issue lineage.

- [ ] **Step 2: Run RED persistence tests**

```bash
cd server
DATABASE_URL='postgres://multica:multica@localhost:15435/multica_codex_stage2?sslmode=disable' go test ./internal/service -run 'TestSemanticDesignDraft' -count=1 -v
```

- [ ] **Step 3: Add migration and sqlc queries**

Add nullable semantic columns for legacy compatibility and update the status constraint to include `generated_with_warnings`, `compile_failed`, `rejected`, and `approved`. Add `CreateSemanticDesignDraft`, `GetNextSemanticDesignDraftVersion`, and `ListReviewableDesignDrafts` without SQL `JOIN` syntax. `ListReviewableDesignDrafts` returns only `generated` and `generated_with_warnings`.

- [ ] **Step 4: Regenerate and verify SQL**

```bash
cd server
make sqlc
rg -n "CreateSemanticDesignDraft|GetNextSemanticDesignDraftVersion|ListReviewableDesignDrafts" pkg/db/generated/design.sql.go
```

Verify the new generated statements contain no `JOIN` token.

---

### Task 3: Compile PageSpec During Agent Completion

**Files:**
- Modify: `server/internal/handler/daemon.go`
- Modify: `server/internal/handler/design_file.go`
- Modify: `server/internal/handler/design_file_test.go`

**Interfaces:**
- Consumes: strict Agent output, `DesignGenerationAssetStore.LoadCompilationAssets`, and `designcore.CompileListPage`.
- Produces: one semantic DesignDraft with compiled Native JSON, quality report, source lineage, and status `generated`, `generated_with_warnings`, or `compile_failed`.

- [ ] **Step 1: Write failing completion tests**

Cover: valid PageSpec creates a reviewable draft; stale/missing assets fail; unknown PageSpec fields fail; requirement coverage omission fails; `compile_failed` persists diagnostics but emits no `design_draft:ready`; warning-only output is reviewable.

- [ ] **Step 2: Run RED completion tests**

```bash
cd server
DATABASE_URL='postgres://multica:multica@localhost:15435/multica_codex_stage2?sslmode=disable' go test ./internal/handler -run 'TestCompleteUIDraftCreateTask.*(PageSpec|Compile|Quality)' -count=1 -v
```

- [ ] **Step 3: Implement transactional completion**

Move semantic draft creation into `TaskService.CompleteTaskWithMutation`. Load assets with:

```go
assets, err := store.LoadCompilationAssets(ctx, service.LoadCompilationAssetsParams{
    WorkspaceID: workspaceID,
    TargetProjectID: projectID,
    TemplateRevisionID: templateRevisionID,
    DesignSystemProfileID: profileID,
})
```

Compile with `designcore.CompileListPage`, including Issue/task IDs in `CompileProvenance`. Persist the retained document and quality diagnostics for all outcomes. Publish `design_draft:ready` only for `generated` and `generated_with_warnings`. There is no slot/patch fallback.

- [ ] **Step 4: Run GREEN completion and compiler tests**

```bash
cd server
DATABASE_URL='postgres://multica:multica@localhost:15435/multica_codex_stage2?sslmode=disable' go test ./internal/handler -run 'TestCompleteUIDraftCreateTask.*(PageSpec|Compile|Quality)' -count=1
go test ./internal/designcore ./internal/service -count=1
```

---

### Task 4: Add Review, Revision, Reject, And Approval APIs

**Files:**
- Modify: `server/cmd/server/router.go`
- Modify: `server/cmd/server/integration_test.go`
- Modify: `server/internal/handler/design_file.go`
- Modify: `server/internal/handler/design_file_test.go`

**Interfaces:**
- Consumes: semantic draft lineage and compiled Native JSON.
- Produces: revision Agent tasks, explicit rejection, and approval that materializes a formal design file/revision.

- [ ] **Step 1: Write failing route and behavior tests**

Add authenticated routes:

```text
POST /api/design-drafts/{id}/revise
POST /api/design-drafts/{id}/reject
POST /api/design-drafts/{id}/approve
```

`revise` requires a non-empty note and creates a fresh UI Agent task carrying the prior PageSpec. `reject` records a reason and status. `approve` accepts only reviewable semantic drafts, revalidates `compiled_native_json`, creates the formal `ai_generated` design file/revision, and marks the draft `approved`.

- [ ] **Step 2: Run RED route tests**

```bash
cd server
DATABASE_URL='postgres://multica:multica@localhost:15435/multica_codex_stage2?sslmode=disable' go test ./internal/handler ./cmd/server -run 'Test(Revise|Reject|Approve)SemanticDesignDraft' -count=1 -v
```

- [ ] **Step 3: Implement handlers and state guards**

Keep `POST /materialize` for legacy drafts. Semantic drafts use `/approve`; reject invalid state transitions with `409`. A revision never mutates its parent row and receives `version = parent.version + 1`.

- [ ] **Step 4: Run GREEN route tests**

Run the command from Step 2 and the existing materialize tests to protect compatibility.

---

### Task 5: Update Core Contracts And Draft Review UI

**Files:**
- Modify: `packages/core/types/design.ts`
- Modify: `packages/core/api/client.ts`
- Modify: `packages/core/designs/keys.ts`
- Modify: `packages/core/designs/queries.ts`
- Modify: `packages/core/realtime/use-realtime-sync.ts`
- Modify: `packages/core/realtime/use-realtime-sync-ws-instance.test.tsx`
- Modify: `packages/views/designs/design-draft-page.tsx`
- Modify: `packages/views/designs/designs-page.tsx`
- Modify: focused tests under `packages/views/designs/`

**Interfaces:**
- Consumes: semantic draft API fields and review endpoints.
- Produces: compiled preview, quality summary, PageSpec inspection, revision note, reject, and approve controls.

- [ ] **Step 1: Write failing frontend contract and interaction tests**

Require semantic statuses, PageSpec/quality fields, approval/rejection/revision API calls, realtime invalidation, and pending-review filtering that excludes `compile_failed`.

- [ ] **Step 2: Implement the review experience**

Render `compiled_native_json` directly with `NativeDesignPreview`; do not synthesize slots or apply patches for semantic drafts. Show concise quality metrics and diagnostics. Use approve/reject command buttons and a revision-note textarea; keep legacy materialize rendering only when `generation_mode === "legacy_patch"`.

- [ ] **Step 3: Run focused frontend verification**

```bash
corepack pnpm --filter @multica/core exec vitest run realtime/use-realtime-sync-ws-instance.test.tsx
corepack pnpm --filter @multica/views exec vitest run designs
corepack pnpm --filter @multica/views exec tsc --noEmit --pretty false
```

---

### Task 6: Prove The CRM Vertical Slice

**Files:**
- Modify: `server/internal/handler/design_file_test.go`
- Add or modify focused Playwright coverage in `e2e/design-semantic-draft.spec.ts`.

**Interfaces:**
- Consumes: CRM Issue requirement, existing CRM Blueprint, existing CRM RecipeSet, and Local UI Restore Agent.
- Produces: one approved generated design linked to the UI Issue, with no purchase-template residue and no critical quality diagnostics.

- [ ] **Step 1: Add an integration test for the full server workflow**

Exercise `create task -> strict PageSpec completion -> compile -> reviewable draft -> approve`. Assert the generated revision contains PageSpec filter/column/row counts, status-tag components, provenance IDs, zero template residue, and zero unexpected overlap.

- [ ] **Step 2: Run complete focused verification**

```bash
cd server
DATABASE_URL='postgres://multica:multica@localhost:15435/multica_codex_stage2?sslmode=disable' go test ./internal/designcore ./internal/service ./internal/handler ./internal/daemon ./internal/daemon/execenv -count=1
cd ..
corepack pnpm --filter @multica/core test
corepack pnpm --filter @multica/views test
git diff --check
node .gitnexus/run.cjs detect_changes --scope all
```

- [ ] **Step 3: Run live CRM acceptance**

Use `multica_restored_20260722`, direct frontend `3031`, backend `8080`, and the established foreground daemon command. Create one CRM UI Issue task, inspect the review draft, submit one revision note if needed, approve it, and verify the resulting design center file/revision. Do not use `make dev`, `make start`, or a random preview port.

---

## Completion Gate

- UI Agent tasks return strict PageSpec and cannot return legacy patches.
- Server compilation is deterministic and uses source-current Blueprint/Recipe assets.
- Critical quality failures are visible diagnostically but never appear as normal review drafts.
- Reviewers can inspect, revise, reject, and approve a semantic draft.
- Approval materializes exactly the compiler-produced Native Design JSON.
- CRM acceptance passes with no template business residue, no unexpected overlap, and a formal design revision linked to the UI Issue.

Do not extend this plan to modal/drawer states, detail pages, form pages, dashboards, C-end pages, or direct Figma writing.
