# Project Design System Workspace Alignment Implementation Plan

> **Recovery note (2026-08-11):** Preserved from `codex/feature-fengchen-dirty-recovery-20260810` as historical context. Do not treat this file as current authority without checking `docs/product/design-center/README.md` and `docs/product/design-center/decision-register.md`.

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Make the project `设计体系` Tab the complete creation, generation, preview, adjustment, and save workspace, and only expose a usable draft after both artifact validation and a real isolated UI Kit render have succeeded.

**Architecture:** Reuse the existing `project_design_system` identity, `draft`/`saved` package slots, static validators, and preview isolation. Replace the summary-to-detail navigation with one shared workspace embedded in `DesignsPage`; keep the old detail route only as a compatibility wrapper around the same content canvas. Add a browser render receipt to the trusted preview bridge, enrich the specialized Agent task with real lifecycle/activity evidence, and keep save/discard operations transactional.

**Tech Stack:** React 19, TanStack Query, Zod, Base UI/shadcn, Vitest, Go 1.26.1, Chi, PostgreSQL 17, sqlc, existing Agent task queue/daemon, WebSocket events, local Chrome.

## Global Constraints

- Product authority is `docs/product/design-center/README.md` and confirmed decisions `DC-031` through `DC-034` in `docs/product/design-center/decision-register.md`.
- The project `设计体系` Tab is the primary workspace. Do not reintroduce a summary card, a required `打开设计体系` action, or duplicate project/category titles.
- The unestablished state immediately renders the single-screen creation workbench. Target platform and explicit Agent selection remain required; all reference material remains optional.
- The content canvas stays continuous: dynamic rules, visual Tokens, component states, and the online UI Kit are not split into internal tabs.
- The Agent adjustment surface is a closed-by-default drawer on desktop and mobile. It must not reserve a permanent third column.
- Do not display invented percentages or inferred pseudo-stages. Show only persisted task status, selected Agent, real timestamps, elapsed time, latest Agent activity, and actionable warnings.
- `task completed` never creates a usable draft by itself. A future draft must pass the existing three-artifact static validation and a real render receipt from the CSP-locked sandboxed UI Kit.
- A draft is never a downstream project constraint. Existing UI generation and design restore still consume legacy `design_system_profile`; migrating those consumers to the new saved package is a separate follow-up plan.
- Saving atomically replaces `saved` from a verified `draft`. Discarding a first draft returns to the creation workbench; discarding an adjustment restores the last `saved` package.
- No review, approval, rejection, publication, permission workflow, file tree, raw source editor, Token form editor, DOM box selection, or arbitrary canvas editing is introduced.
- SQL added to `server/pkg/db/queries/design.sql` must contain no `JOIN` token because the GitLab pre-receive rule rejects generated SQL containing it.
- Do not reset or recreate the user's database. Migration `131` must preserve existing saved packages.
- The worktree already contains 53 unrelated modified files. Never discard them. Before every code edit run GitNexus upstream impact for the exact symbol; warn before HIGH or CRITICAL risk. Before any commit run GitNexus `detect_changes`, focused tests, and `rtk git diff --check`.
- Run browser acceptance in the user's existing Chrome session at `http://localhost:3031`; do not substitute an isolated Playwright browser.
- Execute one task at a time and stop at its acceptance gate. Do not run this whole plan as one long autonomous job.

---

## Current-State Audit

| Decision | Already present | Current gap |
| --- | --- | --- |
| `DC-031` direct content view | Dynamic sections, Token groups, UI Kit, locator selection | Project Tab renders a summary and `打开设计体系`; detail route repeats project/system headers |
| `DC-032` direct creation workbench | Explicit Agent/platform/brief, optional references, failed-input preservation | An empty-state screen and second `创建设计体系` click still precede the form |
| `DC-033` continuous canvas | Rules, Tokens, and UI Kit already render continuously | Desktop Agent panel is permanently visible; save copy is not first-save/update aware; no discard action |
| `DC-034` verified success | Strict three-file static validation, draft/saved isolation, transactional save, failed-operation preservation | Completion immediately creates `draft` without browser render proof; task feedback lacks last activity/cancel/failure classification; failure/cancel/save invalidation is incomplete |

## File Structure

- Create `packages/views/designs/project-design-system-workspace.tsx`: state coordinator for unestablished, generating, validating, draft, and saved views inside a project Tab.
- Refactor `packages/views/designs/project-design-system-create.tsx`: creation form only; no empty-state gate and no saved-system summary.
- Create `packages/views/designs/project-design-system-canvas.tsx`: shared continuous content canvas, compact toolbar, scope selection, and closed-by-default adjustment drawer.
- Refactor `packages/views/designs/project-design-system-page.tsx`: compatibility route data loader that renders the shared canvas instead of a second product experience.
- Modify colocated Vitest files plus `packages/views/designs/designs-page.tsx` and `designs-page.test.tsx`.
- Modify `packages/core/types/design.ts`, `packages/core/api/{schemas,client}.ts`, `packages/core/designs/{keys,queries}.ts`, `packages/core/types/events.ts`, and `packages/core/realtime/use-realtime-sync.ts` for render verification and task evidence.
- Modify `server/internal/handler/project_design_system.go` and tests for response state, render receipt, discard, and save events.
- Modify `server/internal/projectdesignsystem/{preview,types}.go` and tests for a trusted render bridge.
- Modify `server/internal/handler/daemon.go`, `server/internal/service/task.go`, and focused tests for correct specialized-task progress routing and lifecycle refresh.
- Modify `server/pkg/protocol/messages.go` so persisted/live task messages carry their actual creation timestamp.
- Create `server/migrations/131_project_design_system_render_validation.{up,down}.sql`.
- Modify `server/pkg/db/queries/design.sql` and regenerate `server/pkg/db/generated/{design.sql.go,models.go}`.
- Modify authenticated routes in `server/cmd/server/router.go` and route coverage in `server/cmd/server/integration_test.go`.

---

### Task 1: Put Creation And Existing Content Directly In The Project Tab

**Files:**
- Create: `packages/views/designs/project-design-system-workspace.tsx`
- Modify: `packages/views/designs/project-design-system-create.tsx`
- Modify: `packages/views/designs/designs-page.tsx`
- Modify: `packages/views/designs/project-design-system-create.test.tsx`
- Modify: `packages/views/designs/designs-page.test.tsx`
- Create: `packages/views/designs/project-design-system-workspace.test.tsx`

**Interfaces:**
- Consumes: current project, Agents, project design files, legacy Figma UI profiles, and `ProjectDesignSystem` query result already loaded by `DesignsPage`.
- Produces: `ProjectDesignSystemWorkspace`, the only state switch used by the project `设计体系` Tab.

- [ ] **Step 1: Run GitNexus impact before touching the three existing components**

Run impact for `DesignsPage`, `ProjectDesignSystemCreate`, and the project-system branch in `DesignsPage`. Stop and report if any result is HIGH or CRITICAL.

- [ ] **Step 2: Replace the old navigation behavior with failing view tests**

Add exact behavioral coverage:

```tsx
it("renders the creation workbench directly for an unestablished project", async () => {
  await openProjectAndDesignSystemTab();
  expect(await screen.findByRole("button", { name: "生成设计体系" })).toBeInTheDocument();
  expect(screen.queryByRole("button", { name: "创建设计体系" })).not.toBeInTheDocument();
  expect(screen.queryByText("尚未建立设计体系")).not.toBeInTheDocument();
});

it("renders saved design-system content directly without a detail link", async () => {
  getProjectDesignSystemForProject.mockResolvedValue(makeSavedSystem());
  await openProjectAndDesignSystemTab();
  expect(await screen.findByRole("heading", { name: "品牌原则" })).toBeInTheDocument();
  expect(screen.queryByRole("link", { name: "打开设计体系" })).not.toBeInTheDocument();
});
```

The system Tab must also omit `搜索设计体系…`; this workspace has content navigation rather than a list search.

- [ ] **Step 3: Run the RED tests**

```bash
rtk pnpm --filter @multica/views exec vitest run designs/designs-page.test.tsx designs/project-design-system-create.test.tsx
```

Expected: failures for the old empty-state button, summary card, link, and system-search input.

- [ ] **Step 4: Implement the workspace coordinator**

`ProjectDesignSystemWorkspace` must select exactly one surface:

```ts
if (isLoading) return <ProjectDesignSystemSkeleton />;
if (system.active_task || system.status === "generating") return <ProjectDesignSystemTaskStatus />;
if (system.status === "draft" || system.status === "saved") {
  return <ProjectDesignSystemCanvas system={system} project={project} agents={agents} />;
}
return <ProjectDesignSystemCreate project={project} agents={agents} designFiles={designFiles} legacyProfiles={legacyProfiles} system={system} />;
```

Task 4 will add `validating` to this switch together with the API type, so Task 1 remains independently type-safe. Remove `creationOpen`, the empty-state branch, the saved summary, `AppLink`, and `useWorkspacePaths` from the creation component. Preserve all field values and explicit Agent selection behavior.

- [ ] **Step 5: Embed the coordinator and remove list-only chrome**

In the systems `TabsContent`, render `ProjectDesignSystemWorkspace` with no duplicate wrapper heading. Hide the global asset search when `activeTab === "systems"`; keep it unchanged for designs, drafts, and templates.

- [ ] **Step 6: Run focused tests and typecheck**

```bash
rtk pnpm --filter @multica/views exec vitest run designs/designs-page.test.tsx designs/project-design-system-create.test.tsx
rtk pnpm --filter @multica/views typecheck
rtk git diff --check
```

**Acceptance gate:** In Chrome, an unestablished project opens directly on the full form, and a saved project opens directly on real design-system content without navigation to `/designs/systems/:id`.

---

### Task 2: Make Content Primary And Move Adjustment Into A Drawer

**Files:**
- Create: `packages/views/designs/project-design-system-canvas.tsx`
- Create: `packages/views/designs/project-design-system-canvas.test.tsx`
- Modify: `packages/views/designs/project-design-system-page.tsx`
- Modify: `packages/views/designs/project-design-system-page.test.tsx`
- Modify: `packages/views/designs/project-design-system-workspace.tsx`
- Modify: `packages/views/designs/index.ts`

**Interfaces:**
- Consumes: one renderable `ProjectDesignSystem`, current project, and selectable Agents.
- Produces: one continuous canvas usable both inside `DesignsPage` and by the retained compatibility route.

- [ ] **Step 1: Run GitNexus impact for `ProjectDesignSystemPage`, `AdjustmentPanel`, and `ProjectDesignSystemPreview`**

Report the direct callers and affected web/desktop routes before editing.

- [ ] **Step 2: Add failing canvas tests**

Cover these exact outcomes:

```tsx
it("keeps the adjustment drawer closed until the user requests it", async () => {
  renderCanvas(makeDraftSystem());
  expect(screen.queryByRole("dialog", { name: "调整设计体系" })).not.toBeInTheDocument();
  await user.click(screen.getByRole("button", { name: "调整设计体系" }));
  expect(screen.getByRole("dialog", { name: "调整设计体系" })).toBeInTheDocument();
});

it("opens a section-scoped adjustment from that section", async () => {
  renderCanvas(makeDraftSystem());
  await user.click(screen.getByRole("button", { name: "调整 品牌原则" }));
  expect(screen.getByText("品牌原则", { selector: "[data-adjustment-scope]" })).toBeInTheDocument();
});
```

Also assert that the desktop canvas has no permanently rendered `complementary` region, that the project name is not repeated above the system identity, and that rules, Tokens, and UI Kit remain in one scroll surface.

- [ ] **Step 3: Run the RED tests**

```bash
rtk pnpm --filter @multica/views exec vitest run designs/project-design-system-page.test.tsx designs/project-design-system-canvas.test.tsx
```

- [ ] **Step 4: Extract the shared content canvas**

Move dynamic navigation, Markdown sections, Token previews, UI Kit, adjustment mutations, regeneration, and save mutation into `ProjectDesignSystemCanvas`. The compact toolbar contains only system identity, status, last update, `调整设计体系`, the context-aware save action, and a more menu.

Use `Sheet` at every breakpoint. Its initial state is closed. A section-level icon button sets `{ kind: "section", id }` and opens the drawer. UI Kit locator selection only updates the selected scope; it must not permanently shrink the canvas.

- [ ] **Step 5: Convert the old route into a compatibility wrapper**

`ProjectDesignSystemPage` continues loading by ID for existing links and desktop route compatibility, but renders the same `ProjectDesignSystemCanvas`. Remove its duplicate product layout, permanent third column, and repeated project eyebrow. No new UI should link to this route.

- [ ] **Step 6: Verify the content canvas**

```bash
rtk pnpm --filter @multica/views exec vitest run designs/project-design-system-page.test.tsx designs/project-design-system-canvas.test.tsx designs/project-design-system-preview.test.tsx
rtk pnpm --filter @multica/views typecheck
rtk git diff --check
```

**Acceptance gate:** At desktop width the UI Kit receives the primary content width, the Agent UI consumes no space until opened, and a local section adjustment opens with the exact selected scope.

---

### Task 3: Show Real Agent Activity And Allow The User To Stop It

**Files:**
- Modify: `server/pkg/protocol/messages.go`
- Modify: `server/internal/handler/daemon.go`
- Modify: `server/internal/handler/daemon_test.go`
- Modify: `server/internal/service/task.go`
- Modify: `server/internal/service/project_design_system_task_test.go`
- Modify: `server/internal/handler/project_design_system.go`
- Modify: `server/internal/handler/project_design_system_test.go`
- Modify: `packages/core/types/events.ts`
- Modify: `packages/core/types/design.ts`
- Modify: `packages/core/api/schemas.ts`
- Modify: `packages/core/realtime/use-realtime-sync.ts`
- Modify: `packages/core/realtime/use-realtime-sync-ws-instance.test.tsx`
- Modify: `packages/views/designs/project-design-system-workspace.tsx`
- Modify: `packages/views/designs/project-design-system-canvas.tsx`
- Modify: `packages/views/designs/project-design-system-workspace.test.tsx`
- Modify: `packages/views/designs/project-design-system-canvas.test.tsx`

**Interfaces:**
- `TaskMessagePayload.created_at?: string` is populated by persisted and live messages.
- `ProjectDesignSystemTask` gains `dispatched_at`, `failure_reason`, and `wait_reason` while remaining backward compatible through Zod defaults.
- Existing `api.cancelTaskById(taskId)` is reused; no design-system-specific cancel route is added.

- [ ] **Step 1: Run GitNexus impact for task message publication and lifecycle broadcasting**

Run upstream impact for `ReportTaskProgress`, `broadcastTaskEvent`, `projectDesignSystemTaskResponse`, and the global `task:message` handler. A HIGH or CRITICAL result requires a user checkpoint before editing.

- [ ] **Step 2: Add failing backend tests for truthful routing and timestamps**

Add assertions that:

```go
func TestReportTaskProgressUsesProjectDesignSystemWorkspace(t *testing.T)
func TestTaskMessagePayloadIncludesCreatedAt(t *testing.T)
func TestProjectDesignSystemTaskResponseIncludesLifecycleEvidence(t *testing.T)
```

The progress test must create a non-Issue `project_design_system_task` and prove its `task:progress` event carries the task context workspace rather than an empty workspace.

- [ ] **Step 3: Implement task evidence without fake stages**

Change `ReportTaskProgress` to use `TaskService.ResolveTaskWorkspaceID`. Populate `created_at` from `task_message.created_at` in both daemon-auth and user-auth message responses and in the live message broadcast. Extend the project-system task response with DB-backed dispatch, failure, and wait fields.

Do not expose daemon `step/total` as a percentage. The two coarse daemon progress calls remain internal facts, not a progress bar.

- [ ] **Step 4: Refresh the exact project-system query on task lifecycle events**

When a task lifecycle payload matches a cached `ProjectDesignSystem.active_task.id`, invalidate only that system-by-ID and system-by-project query. Do not invalidate all design assets. The existing `task:message` handler continues writing messages to `chatKeys.taskMessages(taskId)`.

- [ ] **Step 5: Build the honest task status surface**

Use `taskMessagesOptions(activeTask.id)` to hydrate and receive the real message stream. Display:

- selected Agent;
- queued/dispatched/running/waiting/failed/cancelled status translated for users;
- actual start time when present;
- elapsed execution time based on `started_at`;
- latest activity timestamp from the newest task message, falling back to started/dispatched/created time;
- an inactivity warning after three minutes without a task message while running;
- `停止任务`, calling `api.cancelTaskById` and then invalidating the two project-system queries.

Keep the user's creation inputs visible or recoverable after failure. Distinguish Agent not picked up, execution failure/cancellation, and invalid artifacts from `active_task` plus `last_error.code`.

- [ ] **Step 6: Run focused verification**

```bash
rtk go test ./internal/handler ./internal/service -run 'Test(ReportTaskProgressUsesProjectDesignSystemWorkspace|TaskMessagePayloadIncludesCreatedAt|ProjectDesignSystemTaskResponseIncludesLifecycleEvidence)' -count=1
rtk pnpm --filter @multica/core exec vitest run realtime/use-realtime-sync-ws-instance.test.tsx api/schemas.test.ts
rtk pnpm --filter @multica/views exec vitest run designs/project-design-system-workspace.test.tsx designs/project-design-system-canvas.test.tsx
rtk pnpm typecheck
rtk git diff --check
```

**Acceptance gate:** A live project-system task changes from queued to running without manual refresh, shows a real last-activity time, warns when stale, and can be stopped from the same Tab.

---

### Task 4: Require A Real Isolated UI Kit Render Before Draft Status

**Files:**
- Create: `server/migrations/131_project_design_system_render_validation.up.sql`
- Create: `server/migrations/131_project_design_system_render_validation.down.sql`
- Modify: `server/pkg/db/queries/design.sql`
- Regenerate: `server/pkg/db/generated/design.sql.go`
- Regenerate: `server/pkg/db/generated/models.go`
- Modify: `server/internal/projectdesignsystem/preview.go`
- Modify: `server/internal/projectdesignsystem/preview_test.go`
- Modify: `server/internal/handler/project_design_system_completion.go`
- Modify: `server/internal/handler/project_design_system.go`
- Modify: `server/internal/handler/project_design_system_completion_test.go`
- Modify: `server/internal/handler/project_design_system_test.go`
- Modify: `server/cmd/server/router.go`
- Modify: `server/cmd/server/integration_test.go`
- Modify: `packages/core/types/design.ts`
- Modify: `packages/core/api/schemas.ts`
- Modify: `packages/core/api/client.ts`
- Modify: `packages/core/api/client.test.ts`
- Modify: `packages/views/designs/project-design-system-preview.tsx`
- Modify: `packages/views/designs/project-design-system-preview.test.tsx`
- Modify: `packages/views/designs/project-design-system-workspace.tsx`

**Interfaces:**
- Package columns: `render_status` (`pending | passed | failed`), `render_report`, and `rendered_at`.
- New response field:

```ts
interface ProjectDesignSystemPreviewValidation {
  status: "none" | "pending" | "passed" | "failed";
  integrity_sha256: string;
  report: Record<string, unknown>;
  verified_at: string | null;
}
```

- New idempotent endpoint: `POST /api/project-design-systems/{id}/preview-verification`.

- [ ] **Step 1: Run GitNexus impact for package persistence, completion, preview generation, and API response assembly**

Inspect `persistProjectDesignSystemCompletion`, `UpsertProjectDesignSystemPackage`, `BuildPreviewHTML`, and `projectDesignSystemResponse`. Warn before proceeding on HIGH or CRITICAL impact.

- [ ] **Step 2: Add the migration and RED persistence tests**

Migration behavior:

```sql
ALTER TABLE project_design_system_package
  ADD COLUMN render_status TEXT NOT NULL DEFAULT 'pending',
  ADD COLUMN render_report JSONB NOT NULL DEFAULT '{}'::jsonb,
  ADD COLUMN rendered_at TIMESTAMPTZ;

UPDATE project_design_system_package
SET render_status = 'passed',
    render_report = '{"source":"pre_render_verification_saved_package"}'::jsonb,
    rendered_at = updated_at
WHERE slot = 'saved';
```

Add the check constraint for `pending`, `passed`, and `failed`. Existing saved packages remain usable; existing draft packages must verify on their next preview.

Tests must prove a newly completed Agent task creates a pending candidate, not a usable draft, and that save rejects pending/failed render status.

- [ ] **Step 3: Extend the trusted preview bridge**

Keep Agent scripts forbidden. The single Multica-owned bridge reads the package digest from a trusted meta element, waits for document fonts and images to settle, and posts one of:

```ts
{
  type: "multica:project-design-system-preview",
  status: "ready",
  digest,
  locator_count,
  visible_locator_count,
  body_width,
  body_height,
  image_count,
  failed_image_count,
}
```

or a bounded failure report. A ready receipt requires positive body dimensions, at least one visible known locator, and zero failed images. Recompute the CSP script hash from the exact trusted bridge constant.

- [ ] **Step 4: Implement idempotent server verification**

The endpoint must:

1. validate workspace access and request bounds;
2. lock the project/system row;
3. load the current draft candidate;
4. reject a stale digest without changing current state;
5. re-run static `Validate` and compare the stored digest;
6. mark the same package `passed` only for valid metrics;
7. record `failed` plus a technical report without deleting the candidate;
8. publish `project_design_system:changed` after commit.

`projectDesignSystemResponse` returns `status: "validating"` while the newest candidate is not render-passed. It may return the candidate preview so the trusted browser can perform verification, but save remains disabled.

Add `validating` to `ProjectDesignSystemStatus`, its Zod normalizer, and the `ProjectDesignSystemWorkspace` state switch in the same step.

- [ ] **Step 5: Wire the browser receipt to the API exactly once per digest**

`ProjectDesignSystemPreview` accepts `integritySha256` and `onVerification`. It accepts verification messages only from its own iframe and only when the message digest matches the prop. The workspace posts the receipt through the new client method, updates both React Query cache entries, and offers `重新验证预览` when the receipt fails or times out.

- [ ] **Step 6: Verify static, API, client, and browser contracts**

```bash
rtk make sqlc
rtk go test ./internal/projectdesignsystem ./internal/handler -run 'Test(ProjectDesignSystem|BuildPreviewHTML)' -count=1
rtk pnpm --filter @multica/core exec vitest run api/client.test.ts api/schemas.test.ts
rtk pnpm --filter @multica/views exec vitest run designs/project-design-system-preview.test.tsx designs/project-design-system-workspace.test.tsx
rtk pnpm typecheck
rtk git diff --check
```

**Acceptance gate:** Completing the Agent task alone leaves the system in `validating`; loading a genuinely nonblank UI Kit in the sandbox produces a matching receipt and only then changes the product state to `草稿`.

---

### Task 5: Complete Discard, Save Copy, Events, And Live Acceptance

**Files:**
- Modify: `server/pkg/db/queries/design.sql`
- Regenerate: `server/pkg/db/generated/design.sql.go`
- Modify: `server/internal/handler/project_design_system.go`
- Modify: `server/internal/handler/project_design_system_test.go`
- Modify: `server/internal/handler/project_design_system_persistence_test.go`
- Modify: `server/cmd/server/router.go`
- Modify: `server/cmd/server/integration_test.go`
- Modify: `packages/core/api/client.ts`
- Modify: `packages/core/api/client.test.ts`
- Modify: `packages/views/designs/project-design-system-canvas.tsx`
- Modify: `packages/views/designs/project-design-system-canvas.test.tsx`
- Create after live proof: `docs/product/design-center/project-design-system-workspace-validation.md`

**Interfaces:**
- New endpoint: `DELETE /api/project-design-systems/{id}/draft`.
- New client method: `discardProjectDesignSystemDraft(id: string): Promise<ProjectDesignSystem>`.
- Save label is derived from `saved_at`: first save uses `保存为项目设计体系`; later draft save uses `保存调整`.

- [ ] **Step 1: Run GitNexus impact for save and draft-slot deletion**

Inspect `SaveProjectDesignSystem`, `DeleteProjectDesignSystemPackageSlot`, and `ProjectDesignSystemCanvas` before editing.

- [ ] **Step 2: Add RED transaction tests**

Add exact cases:

```go
func TestDiscardFirstProjectDesignSystemDraftReturnsUnestablished(t *testing.T)
func TestDiscardAdjustmentDraftRestoresSavedPackage(t *testing.T)
func TestDiscardDraftRejectsActiveTask(t *testing.T)
func TestSaveProjectDesignSystemPublishesChangedEventAfterCommit(t *testing.T)
```

For each error injection, assert the previously saved bytes and current draft bytes are unchanged.

- [ ] **Step 3: Implement transactional discard and complete event publication**

Lock the project/system row, reject an active task, delete only the `draft` slot, clear draft-related render failure, and return the recomputed response. Do not delete the identity or input snapshot: the first-draft path therefore returns the prefilled creation workbench; the adjustment path returns the last saved content.

Publish `project_design_system:changed` only after successful save/discard commits. Failed transactions publish nothing.

- [ ] **Step 4: Finish action hierarchy in the canvas**

- Draft with no `saved_at`: primary action `保存为项目设计体系`.
- Draft with `saved_at`: primary action `保存调整`.
- Saved with no draft: no enabled save action.
- More menu: `放弃草稿` when a draft exists and `重新生成设计体系`.
- Destructive discard requires a confirmation dialog explaining whether it returns to creation or restores saved content.

- [ ] **Step 5: Run the complete focused suite**

```bash
rtk make sqlc
rtk go test ./internal/projectdesignsystem ./internal/handler ./internal/service -count=1
rtk pnpm --filter @multica/core exec vitest run api/client.test.ts api/schemas.test.ts realtime/use-realtime-sync-ws-instance.test.tsx
rtk pnpm --filter @multica/views exec vitest run designs/designs-page.test.tsx designs/project-design-system-create.test.tsx designs/project-design-system-canvas.test.tsx designs/project-design-system-preview.test.tsx designs/project-design-system-page.test.tsx
rtk pnpm typecheck
rtk git diff --check
```

- [ ] **Step 6: Validate the real workflow in the user's Chrome and database**

Use one disposable project design-system run and capture evidence for:

1. project Tab opens directly to the creation workbench;
2. the selected Agent/platform/brief/references appear in the persisted task context;
3. queued/running status and task messages change without manual refresh;
4. Agent completion persists three nonempty artifacts but remains `validating`;
5. the visible iframe is nonblank, has positive dimensions, exposes known locators, and sends the matching digest receipt;
6. only the successful receipt yields `草稿` and enables save;
7. adjustment drawer opens only on demand and preserves the selected scope;
8. save moves verified draft bytes to `saved` atomically;
9. a later adjustment leaves `saved` unchanged until `保存调整`;
10. discard restores the exact saved digest.

Write task ID, system ID, draft/saved digests, render report, Chrome URL, and screenshots to `docs/product/design-center/project-design-system-workspace-validation.md`. Do not write “passed” without all ten evidence points.

- [ ] **Step 7: Run final GitNexus and diff-scope verification**

Run `detect_changes()` against `main`, review every affected execution flow, and confirm the diff does not include unrelated dirty-worktree changes. Do not commit until the user asks and the staging boundary is proven.

**Acceptance gate:** The full creation-to-save flow is truthful, recoverable, visually usable, and independently evidenced; a bad task, bad artifact, blank UI Kit, or failed save cannot become the project design system.

---

## Explicitly Deferred Follow-Up

After this workspace passes live acceptance, write a separate plan for a single server-side design-context resolver that serves only the `saved` `project_design_system_package` to UI design generation and design restore, with the existing priority `cloud saved design system > local DESIGN.md > repository reality`. Until that plan is implemented, legacy `design_system_profile` remains the active downstream contract and no claim should be made that the new project design system already constrains those Agents.
