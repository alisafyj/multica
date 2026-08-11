# Recovery Extraction: Design Center

Date: 2026-08-11
Current mainline: `feature/fengchen-design`
Recovery source: `codex/feature-fengchen-dirty-recovery-20260810`
Common base: `bfd95597ce594717afef1235a38fb7a5c5f5d8a8`

## Ground Rules

- Keep `feature/fengchen-design` as the authority.
- Use the recovery branch as a source archive only; do not merge it wholesale.
- Do not restore Open Design Worker/Runtime/Daemon execution paths.
- Do not replace native `PackagePreview` / `/package-preview` with recovery `ArchivePreview` / `/open-design-preview`.
- Move one small slice at a time, with GitNexus impact before symbol edits and `detect-changes` before commits.

## Already Restored Or Covered

- Design Center workspace tabs:
  - fixed non-closable `首页`;
  - closable project tabs;
  - compact project content tabs: `设计稿 / 设计草稿 / 模版 / 设计体系`.
- `packages/views/designs/project-design-system-create.tsx` and its test are identical between current and recovery.
- `packages/views/designs/project-design-system-workspace.tsx` and its test are identical between current and recovery.
- Current mainline keeps native project design-system package schema, preview validation, discard draft, and verification client contracts.

## Do Not Port From Recovery

- `ProjectDesignSystemArchivePreview` types/schemas/client methods.
- `designKeys.projectDesignSystemArchivePreview`.
- `/api/project-design-systems/{id}/open-design-preview` as the primary frontend contract.
- Recovery changes that delete current native files:
  - `project_design_system_package_preview.go`;
  - `project_design_system_package_upload.go`;
  - V2 completion/repository-analysis tests and handlers.
- Recovery docs that resume direct Open Design Worker/Runtime/Daemon integration.

## Ported In Current Slice

- Optional semantic draft metadata in `DesignDraft`:
  - `generation_mode`;
  - `page_spec`;
  - `compiled_native_json`;
  - `quality_report`;
  - `blueprint_id`;
  - `recipe_set_id`;
  - `parent_draft_id`;
  - `version`.
- Expanded `DesignDraftStatus` to include semantic draft terminal/review statuses.
- Added schema parsing and fallbacks for current design draft endpoints:
  - `GET /api/design-drafts`;
  - `POST /api/design-drafts`;
  - `POST /api/design-drafts/agent-tasks`;
  - `GET /api/design-drafts/{id}`;
  - `POST /api/design-drafts/{id}/materialize`.
- Removed the local `generation_mode` cast in `designs-page.tsx`.
- Restored read-only semantic draft detail support:
  - semantic drafts render `compiled_native_json` directly;
  - the page shows `PageSpec`, compile quality, version, and blueprint metadata;
  - template preview loading is skipped for semantic drafts;
  - approve/reject/revise actions remain unported.

## Restored As Historical Context

- Restored recovery-only historical plans under `docs/superpowers/plans/` and added a recovery note to each file:
  - `2026-07-23-semantic-generation-asset-analysis.md`;
  - `2026-07-23-ui-agent-semantic-draft-workflow.md`;
  - `2026-07-28-open-design-system-foundation.md`;
  - `2026-07-29-project-design-system-workspace.md`.
- These documents preserve the old semantic draft / design-system planning context, but they are not current execution authority. Current product authority remains `docs/product/design-center/README.md` and `docs/product/design-center/decision-register.md`.

## Design-System Diff Audit

- `project-design-system-create.tsx`, `project-design-system-workspace.tsx`, `project-design-system-page.tsx`, and `project-design-system-task-activity.tsx` are identical between current mainline and recovery.
- Current mainline intentionally keeps native V2-only contract fields and UI behavior that recovery would remove:
  - `ProjectDesignSystemPackagePreview`;
  - `/api/project-design-systems/{id}/package-preview`;
  - `package_schema`, `preview_targets`, `selection_enabled`;
  - native archive upload, package validation, preview receipt, and repository-analysis completion handlers.
- Recovery still points design-system generation/preview back toward direct Open Design archive/Worker-era contracts. Do not port those changes into `feature/fengchen-design`.

## Semantic Draft Backend Slice

- Restored semantic draft persistence/read-response foundations without enabling review actions:
  - new current-mainline migration `135_semantic_design_draft` based on recovery's old `129` shape;
  - `DesignDraft` generated model fields for `generation_mode`, `page_spec`, `compiled_native_json`, `quality_report`, `blueprint_id`, `recipe_set_id`, `parent_draft_id`, and `version`;
  - generated design draft query projections updated for the new columns;
  - `DesignDraftResponse` now returns those fields.
- `sqlc` was unavailable in the local shell, so the generated Go files were updated manually and verified with focused handler tests.
- Still unported: semantic draft creation from PageSpec, approve/reject/revise APIs, and frontend review actions.

## Semantic Draft Store Slice

- Restored the lower-level semantic draft save path from recovery without wiring it into task completion yet:
  - `CreateSemanticDesignDraft`;
  - `GetNextSemanticDesignDraftVersion`;
  - `DesignGenerationAssetStore.SaveSemanticDesignDraft`;
  - focused store test for semantic draft field persistence and issue-scoped version increment.
- Still unported: approve/reject/revise handlers, routes, client methods, and review UI actions.

## Semantic Draft PageSpec Creation Slice

- Restored the UI draft Agent creation path from recovery without reintroducing legacy Open Design Worker/Runtime execution:
  - `UIDraftCreateContext` now carries project/design-system context, generation asset, required requirement, revision, and parent draft fields needed by the semantic compiler path;
  - `CreateDesignDraftAgentTask` resolves saved project design context for Agent prompt payload while keeping the legacy `design_system_profile_id` as an internal compiler asset lookup key;
  - Agent output now requires `design_plan` and `page_spec`, rejecting legacy `slot_values`/`patch` output;
  - `CompleteTask` saves semantic PageSpec drafts inside `CompleteTaskWithMutation` so task completion and draft persistence remain atomic;
  - `createDesignDraftFromAgentTaskOutput` compiles PageSpec through the native compiler and saves via `DesignGenerationAssetStore.SaveSemanticDesignDraft`.
- Verification:
  - `go test -C server -buildvcs=false ./internal/handler -run 'Test(DesignGenerationAssetStore|CreateDesignDraftAgentTask|ClaimUIDraftCreateTask|CompleteUIDraftCreateTask|ParseUIDraftAgentOutput)' -count=1` passed with the local `DATABASE_URL`;
  - `go test -C server -buildvcs=false ./internal/handler -run 'Test(CreateDesignDraftFromCatalogTemplate|GetDesignDraftReturnsSemanticMetadata|CreateDesignDraftAgentTask|ClaimUIDraftCreateTask|CompleteUIDraftCreateTask|ParseUIDraftAgentOutput|DesignGenerationAssetStore)' -count=1` passed with the local `DATABASE_URL`;
  - `git diff --check` passed.
- GitNexus `detect-changes --scope staged` reported `critical` because the slice touches central task completion and design draft task creation flows (`CompleteTask`, `CreateDesignDraftAgentTask`, and helpers). Staged diff was checked for scope: no approve/reject/revise endpoints were ported in this slice.

## Design System Profile Recipe Slice

- Restored the design-system Profile analysis asset handoff from recovery:
  - Profile analysis output policy now requires `recipe_classifications` and `primitive_fallbacks`;
  - `parseDesignSystemProfileAnalyzeOutput` rejects outputs that cannot create component recipes;
  - `CompleteTask` keeps the existing atomic profile completion path and, inside the same mutation, builds a `designcore.ComponentRecipeSet`;
  - the recipe set is saved through `DesignGenerationAssetStore.SaveRecipeSetAnalysis` with the next per-profile `analysis_version`.
- Verification:
  - `go test -C server -buildvcs=false ./internal/handler -run '^TestCompleteDesignSystemProfileAnalyzeTaskUpdatesProfile$' -count=1` failed first because no `design_component_recipe_set` row existed, then passed after the fix;
  - `go test -C server -buildvcs=false ./internal/handler -run 'Test(CreateDesignSystemProfileFromDesignFile|ClaimDesignSystemProfileAnalyzeTaskReturnsContext|CompleteDesignSystemProfileAnalyzeTask|ParseDesignSystemProfileAnalyzeOutputRequiresStrictContract|DesignGenerationAssetStore)' -count=1` passed with the local `DATABASE_URL`;
  - `git diff --check` passed;
  - GitNexus `detect-changes --scope staged` reported `medium`.
- Scope note: this slice does not add template Blueprint analysis endpoints/tasks yet, does not port approve/reject/revise, and does not touch native V2 package preview/upload/repository-analysis.

## Candidate Next Slice

- Template Blueprint analysis chain:
  - enqueue a template Blueprint analysis task after publishing a project-scoped catalog template;
  - parse/validate Agent Blueprint classification output;
  - save `design_template_blueprint` with a per-template revision analysis version;
  - keep this separate from project design-system native V2 package preview/upload.
- `server/internal/designcore` recovery diff triage:
  - inspect whether the remaining compiler, blueprint, recipe, and quality changes are already covered by current mainline;
  - port only behavior that current PageSpec/recipe/blueprint tests prove is still missing.

Do not treat the semantic PageSpec approve/reject/revise review chain as an automatic next slice: current product docs say first phase does not introduce review states or review permissions.
