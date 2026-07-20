# Business Module Design Restore Targets Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Guide design restore agents to create normal business modules instead of defaulting to restore-id sandbox directories.

**Architecture:** Extend generated design restore plans with a `targetStrategy` and business-module selected target derived from issue context plus repo analysis. The daemon prompt will treat this strategy as the execution contract: create/update page, components, and router under module-named paths; only use the design-restore sandbox when repo analysis or module inference is unavailable.

**Tech Stack:** Go backend handlers, daemon prompt tests, PostgreSQL-backed handler tests.

---

### Task 1: Pin Business Module Target Behavior

**Files:**
- Modify: `server/internal/handler/design_file_test.go`

- [x] Add a failing handler test where the restore task is bound to child issue `UI设计` whose parent is `服务记录开发`.
- [x] Generate a plan with a local repo resource.
- [x] Assert selected target uses `service-record`, includes `targetStrategy.mode = business_module`, and does not use `design-restore`.

### Task 2: Implement Target Strategy

**Files:**
- Modify: `server/internal/handler/design_file.go`

- [x] Load issue context inside `buildDefaultDesignRestorePlan` when `task.IssueID` is valid.
- [x] Derive module source from parent issue title when current issue is generic UI/design/frontend wording.
- [x] Add helper functions for module title cleanup, slugging Chinese/English module names, and framework-aware page/component/router paths.
- [x] Preserve prototype fallback when no project/repo analysis exists.

### Task 3: Strengthen Agent Prompt

**Files:**
- Modify: `server/internal/daemon/prompt_test.go`
- Modify: `server/internal/daemon/prompt.go`

- [x] Add prompt assertions for `targetStrategy`, `business_module`, and avoiding restore-id sandboxes for production plans.
- [x] Update the design restore prompt so agents follow module-named `pagePath`, `componentRoot`, and `routePath` as a normal programmer would.

### Task 4: Verify

**Files:**
- Test: `server/internal/handler/design_file_test.go`
- Test: `server/internal/daemon/prompt_test.go`

- [x] Run the new handler test and prompt test.
- [x] Run the focused design restore handler and daemon prompt test set.
- [x] Run GitNexus `detect_changes` before reporting completion.
