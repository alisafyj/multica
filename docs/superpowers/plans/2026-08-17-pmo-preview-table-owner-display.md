# PMO Preview Table And Owner Display Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Replace repeated PMO schedule tables with aligned requirement/task tables and display workspace member names for matched external owners.

**Architecture:** Keep parsing and rendering in the existing shared PMO view. Add one small owner-display resolver beside the source preview types, pass the already-fetched workspace members into the preview, and reuse the resolver in the assignee tab.

**Tech Stack:** React, TypeScript, Tailwind CSS, Vitest, Testing Library

---

### Task 1: Lock The Required UI Behavior

**Files:**
- Modify: `packages/views/pmo/pmo-config-detail-page.test.tsx`

- [x] Add a source snapshot with two tasks in different milestones and assert that the preview has one requirement table, one task table, one task header row, and an independent task ID cell.
- [x] Add workspace member emails to the query fixture and assert that `fengyujie` renders as the matched member name in both preview and assignee mapping.
- [x] Assert that an unmatched email owner renders its email prefix.
- [x] Run `pnpm --filter @multica/views test -- pmo-config-detail-page.test.tsx` and confirm the new assertions fail for the old repeated-table and snapshot-name behavior.

### Task 2: Implement The Minimal Shared Fix

**Files:**
- Modify: `packages/views/pmo/pmo-source-preview.tsx`
- Modify: `packages/views/pmo/pmo-config-detail-page.tsx`

- [x] Add `resolvePMOOwnerDisplay(owner, members)` that matches full email or email prefix and falls back to the external email prefix/ID.
- [x] Pass fetched members to `PMOSourcePreview` and use the resolver for requirement and task owners.
- [x] Replace the loose requirement metadata strip with one fixed-layout information table.
- [x] Replace milestone-grouped tables with one fixed-layout task table and add a separate task ID column.
- [x] Use the same resolver and a responsive two-column grid in the assignee mapping tab.
- [x] Re-run `pnpm --filter @multica/views test -- pmo-config-detail-page.test.tsx` and confirm it passes.

### Task 3: Verify Scope And Quality

**Files:**
- Verify: `packages/views/pmo/pmo-source-preview.tsx`
- Verify: `packages/views/pmo/pmo-config-detail-page.tsx`
- Verify: `packages/views/pmo/pmo-config-detail-page.test.tsx`

- [x] Run the PMO test file again without watch mode.
- [x] Run the narrow package typecheck or root typecheck if no package command exists.
- [x] Run `node .gitnexus/run.cjs detect-changes --scope compare --base-ref main` and confirm only expected PMO symbols and flows are affected.
- [x] Review the final diff for duplicate owner labels, repeated task headers, and unrelated changes.
