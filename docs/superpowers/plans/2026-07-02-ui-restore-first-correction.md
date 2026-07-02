# UI Restore First Correction Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Correct the first implementation slice of design delivery so a UI design Issue owns design restore before frontend handoff, while keeping raw design handoff as a hidden fallback-compatible path.

**Architecture:** This slice is intentionally conservative. It reuses the existing `design_restore_task` and `design_delivery` tables, keeps the existing API surface, and changes issue-side orchestration/copy so UI Issues start UI restore first and raw design delivery is treated as fallback. Later phases can add a first-class UI restore artifact table.

**Tech Stack:** Go backend, sqlc/Postgres, Next.js shared views, TanStack Query, Vitest, Go tests.

## Global Constraints

- Do not introduce user-facing fallback strategy selectors.
- Do not expose `metadata.design_role` or policy implementation details in new product copy.
- Preserve existing raw-design delivery data and API compatibility.
- Use TDD for behavioral changes.
- Run GitNexus impact analysis before editing existing functions/classes/methods.
- Keep changes scoped to design restore / issue-side design delivery behavior.

---

## File Structure

- `packages/views/issues/components/issue-design-restore-section.tsx`
  - Owns the Issue-side design card. This slice changes UI Issue copy/action priority and raw handoff scope metadata.
- `packages/views/issues/components/issue-design-restore-section.test.ts`
  - Adds tests for internal handoff source detection and UI restore-first scope metadata.
- `packages/core/types/design.ts`
  - Adds lightweight exported types for handoff source metadata if needed.
- `packages/core/api/schemas.ts`
  - Preserves unknown delivery scope/audit fields through loose parsing; add assertions if new source metadata becomes typed.
- `server/internal/handler/design_delivery.go`
  - If touched, only ensure fallback audit/scope metadata is accepted and preserved.
- `server/internal/handler/design_delivery_test.go`
  - If touched, assert raw-design fallback metadata survives create/list.
- `docs/product/design-restore-memory.md`
  - Update after implementation to record what changed.

## Task 1: Add UI Restore-First Helpers And Tests

**Files:**
- Modify: `packages/views/issues/components/issue-design-restore-section.test.ts`
- Modify: `packages/views/issues/components/issue-design-restore-section.tsx`

**Interfaces:**
- Produces: `deliveryHandoffSource(delivery: DesignDelivery | null): "raw_design_revision" | "ui_restore_artifact" | null`
- Produces: `isRawDesignFallbackDelivery(delivery: DesignDelivery | null): boolean`
- Produces: `createRawDesignFallbackScope(args): Record<string, unknown>`

- [ ] **Step 1: Run GitNexus impact analysis**

Run:

```bash
node .gitnexus/run.cjs impact issue-design-restore-section --direction upstream
```

Expected: identify Issue-side design card tests/importers. If risk is HIGH or CRITICAL, report before continuing.

- [ ] **Step 2: Write failing tests**

Add tests that prove:

```ts
expect(deliveryHandoffSource(delivery({
  scope: {
    source_type: "raw_design_revision",
    items: [],
  },
}))).toBe("raw_design_revision");

expect(deliveryHandoffSource(delivery({
  scope: {
    source_type: "ui_restore_artifact",
    artifact_id: "artifact-1",
    items: [],
  },
}))).toBe("ui_restore_artifact");

expect(isRawDesignFallbackDelivery(delivery({
  scope: {
    source_type: "raw_design_revision",
    fallback_policy: "frontend_full_restore_fallback",
    items: [],
  },
}))).toBe(true);
```

Also test `createRawDesignFallbackScope` returns:

```ts
{
  version: "1.0",
  source: "issue_delivery",
  source_type: "raw_design_revision",
  fallback_policy: "frontend_full_restore_fallback",
  projectId: "...",
  sourceIssueId: "...",
  targetIssueId: "...",
  items: [...]
}
```

- [ ] **Step 3: Run tests and verify RED**

Run:

```bash
corepack pnpm --filter @multica/views exec vitest run issues/components/issue-design-restore-section.test.ts
```

Expected: FAIL because helpers are missing.

- [ ] **Step 4: Implement helpers**

In `issue-design-restore-section.tsx`, export the three helpers. Keep them pure and independent of React.

- [ ] **Step 5: Use helper in `createDelivery`**

Replace inline `scope` construction in `createDelivery` with `createRawDesignFallbackScope(...)`.

- [ ] **Step 6: Run tests and verify GREEN**

Run:

```bash
corepack pnpm --filter @multica/views exec vitest run issues/components/issue-design-restore-section.test.ts
```

Expected: PASS.

## Task 2: Reprioritize UI Issue Actions Toward UI Restore

**Files:**
- Modify: `packages/views/issues/components/issue-design-restore-section.tsx`
- Modify: `packages/views/issues/components/issue-design-restore-section.test.ts` if pure helpers need copy/status coverage

**Interfaces:**
- Consumes: helpers from Task 1.
- Produces: UI Issue behavior where primary restore action is available before frontend handoff.

- [ ] **Step 1: Run GitNexus impact analysis**

Run:

```bash
node .gitnexus/run.cjs impact runRestoreFlow --direction upstream
node .gitnexus/run.cjs impact createDelivery --direction upstream
```

Expected: direct impact limited to issue-side component and tests. If risk is HIGH or CRITICAL, report before continuing.

- [ ] **Step 2: Write or update tests**

If component render tests are practical, assert UI Issue shows the primary restore action before raw handoff. If not practical in this slice, add pure helper tests only and rely on typecheck plus manual smoke.

- [ ] **Step 3: Change UI Issue primary action order**

For UI Issues:

- Keep `交给 Agent 还原` available as the main action after a design/frame exists.
- Move raw design handoff under a secondary/fallback details block.
- Copy should frame direct frontend handoff as fallback/compatibility only in code comments and internal scope metadata, not as a prominent user-facing strategy selector.

- [ ] **Step 4: Ensure frontend Issues still can restore from received fallback delivery**

Do not remove `receivedDesignDelivery` restore behavior in this slice. It is the hidden fallback-compatible path.

- [ ] **Step 5: Run focused tests**

Run:

```bash
corepack pnpm --filter @multica/views exec vitest run issues/components/issue-design-restore-section.test.ts
corepack pnpm --filter @multica/views exec tsc --noEmit --pretty false
```

Expected: PASS.

## Task 3: Prepare Backend Compatibility For Handoff Source Metadata

**Files:**
- Modify only if necessary: `server/internal/handler/design_delivery_test.go`
- Modify only if necessary: `server/internal/handler/design_delivery.go`

**Interfaces:**
- Consumes: raw-design fallback `scope.source_type` and `scope.fallback_policy` from Task 1.
- Produces: backend preserves metadata through create/list/cancel.

- [ ] **Step 1: Run GitNexus impact analysis**

Run:

```bash
node .gitnexus/run.cjs impact CreateDesignDelivery --direction upstream
node .gitnexus/run.cjs impact designDeliveryToResponse --direction upstream
```

Expected: design delivery handler/tests and route wiring. If risk is HIGH or CRITICAL, report before continuing.

- [ ] **Step 2: Write failing backend test only if preservation is not already covered**

Add an assertion to `TestCreateDesignDeliveryPromotesTargetAndSupersedesPrevious` or a focused test that creates a delivery with:

```json
{
  "source_type": "raw_design_revision",
  "fallback_policy": "frontend_full_restore_fallback"
}
```

and verifies those fields survive in the response/list.

- [ ] **Step 3: Implement only if test fails**

If `json.RawMessage` already preserves metadata, no production backend code change is needed.

- [ ] **Step 4: Run focused Go tests**

Run:

```bash
cd server && go test ./internal/handler -run 'Test(CreateDesignDeliveryPromotesTargetAndSupersedesPrevious|CreateDesignDeliverySupersedesPreviousTargetForSourceIssue|CancelDesignDeliveryMarksActiveDeliveryCancelled|CreateDesignRestoreTaskBindsAndReusesDesignDelivery)$' -count=1
```

Expected: PASS.

## Task 4: Update Memory And Final Verification

**Files:**
- Modify: `docs/product/design-restore-memory.md`

**Interfaces:**
- Consumes completed behavior from Tasks 1-3.
- Produces updated persistent memory.

- [ ] **Step 1: Update memory**

Record:

- raw design delivery now carries internal fallback metadata
- UI Issue primary action is UI restore-first
- frontend received raw design delivery remains fallback-compatible

- [ ] **Step 2: Run final checks**

Run:

```bash
corepack pnpm --filter @multica/views exec vitest run issues/components/issue-design-restore-section.test.ts
corepack pnpm --filter @multica/views exec tsc --noEmit --pretty false
git diff --check
node .gitnexus/run.cjs detect-changes
```

Expected: tests/typecheck/check pass; GitNexus changed-flow report reviewed.
