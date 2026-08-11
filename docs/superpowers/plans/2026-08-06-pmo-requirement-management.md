# PMO Requirement Management Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Add a workspace-level `需求管理` page that asks a user-selected Agent for a strict external requirement snapshot, previews a safe three-way diff, materializes approved projects/issues, and repeats every 30 minutes without overwriting local edits.

**Architecture:** Three new `pmo_sync_*` tables hold configurations, immutable run snapshots, and external-to-local links while `project` and `issue` remain canonical. A focused PMO service validates Agent JSON, computes field-level differences, and applies one run transactionally; existing Agent task and scheduler pipelines supply acquisition and recurrence. Shared React Query code and one `packages/views/pmo` page serve both Web and Desktop.

**Tech Stack:** Go 1.26, Chi, pgx/sqlc, PostgreSQL, Next.js App Router, Electron/React Router, React 19, TanStack Query, zod, Vitest/Testing Library, lucide-react.

---

## File Map

Backend ownership:

- `server/migrations/278_*` through `287_*`: create the three tables, concurrent indexes, and primary-key constraints without foreign keys.
- `server/pkg/db/queries/pmo.sql`: all tenant-scoped config/run/link reads and writes.
- `server/internal/service/pmo_contract.go`: strict Agent JSON contract and normalization.
- `server/internal/service/pmo_diff.go`: pure field-level three-way comparison.
- `server/internal/service/pmo.go`: run orchestration, preview persistence, transactional apply, and schedule dispatch.
- `server/internal/handler/pmo.go`: HTTP authorization and response mapping only.
- `server/internal/service/task.go`, `server/internal/handler/daemon.go`, and daemon context files: typed `pmo_sync` task transport and completion/failure hooks.
- `server/internal/scheduler/jobs_pmo.go`: one distributed minute-level scanner for due 30-minute configurations.

Frontend ownership:

- `packages/core/types/pmo.ts`: API types.
- `packages/core/api/{client,schemas}.ts`: parsed PMO API boundary.
- `packages/core/pmo/{queries,mutations}.ts`: workspace-keyed server state.
- `packages/views/pmo/pmo-page.tsx`: the complete shared operational page.
- Existing path, sidebar, icon, route, and locale registries: expose `/{workspace}/pmo` consistently in Web and Desktop.

## Task 1: Persist PMO Configurations, Runs, and Links

**Files:**

- Create: `server/migrations/306_pmo_sync_tables.up.sql`
- Create: `server/migrations/306_pmo_sync_tables.down.sql`
- Create: `server/migrations/307_pmo_sync_config_id_index.up.sql`
- Create: `server/migrations/307_pmo_sync_config_id_index.down.sql`
- Create: `server/migrations/308_pmo_sync_run_id_index.up.sql`
- Create: `server/migrations/308_pmo_sync_run_id_index.down.sql`
- Create: `server/migrations/309_pmo_sync_link_id_index.up.sql`
- Create: `server/migrations/309_pmo_sync_link_id_index.down.sql`
- Create: `server/migrations/310_pmo_sync_primary_keys.up.sql`
- Create: `server/migrations/310_pmo_sync_primary_keys.down.sql`
- Create: `server/migrations/311_pmo_sync_config_root_index.up.sql`
- Create: `server/migrations/311_pmo_sync_config_root_index.down.sql`
- Create: `server/migrations/312_pmo_sync_run_history_index.up.sql`
- Create: `server/migrations/312_pmo_sync_run_history_index.down.sql`
- Create: `server/migrations/313_pmo_sync_run_active_index.up.sql`
- Create: `server/migrations/313_pmo_sync_run_active_index.down.sql`
- Create: `server/migrations/314_pmo_sync_run_agent_task_index.up.sql`
- Create: `server/migrations/314_pmo_sync_run_agent_task_index.down.sql`
- Create: `server/migrations/315_pmo_sync_link_identity_index.up.sql`
- Create: `server/migrations/315_pmo_sync_link_identity_index.down.sql`
- Create: `server/pkg/db/queries/pmo.sql`
- Generate: `server/pkg/db/generated/pmo.sql.go`
- Modify generated: `server/pkg/db/generated/models.go`
- Test: `server/internal/migrations/migrations_lint_test.go`

- [ ] **Step 1: Add a failing migration-shape assertion**

Extend the migration lint file so the PMO schema is expected to have no
`REFERENCES` clauses and every PMO index migration is a single concurrent
statement:

```go
func readMigrationForLint(t *testing.T, name string) string {
    t.Helper()
    raw, err := os.ReadFile(filepath.Join(realMigrationsDir(t), name))
    if err != nil {
        t.Fatal(err)
    }
    return string(raw)
}

func TestPMOSyncMigrationsStayTenantScopedAndConcurrent(t *testing.T) {
    tables := strings.ToUpper(readMigrationForLint(t, "306_pmo_sync_tables.up.sql"))
    if strings.Contains(tables, "REFERENCES") || strings.Contains(tables, "FOREIGN KEY") {
        t.Fatal("PMO sync tables must not create foreign keys")
    }
    indexes := []string{
        "307_pmo_sync_config_id_index.up.sql",
        "308_pmo_sync_run_id_index.up.sql",
        "309_pmo_sync_link_id_index.up.sql",
        "311_pmo_sync_config_root_index.up.sql",
        "312_pmo_sync_run_history_index.up.sql",
        "313_pmo_sync_run_active_index.up.sql",
        "314_pmo_sync_run_agent_task_index.up.sql",
        "315_pmo_sync_link_identity_index.up.sql",
    }
    for _, name := range indexes {
        sql := strings.TrimSpace(readMigrationForLint(t, name))
        if !strings.Contains(strings.ToUpper(sql), "CREATE UNIQUE INDEX CONCURRENTLY") &&
            !strings.Contains(strings.ToUpper(sql), "CREATE INDEX CONCURRENTLY") {
            t.Errorf("%s must create its index concurrently", name)
        }
        if strings.Count(sql, ";") != 1 {
            t.Errorf("%s must contain one statement", name)
        }
    }
}
```

- [ ] **Step 2: Run the migration test and verify the missing files fail**

Run: `rtk go -C server test ./internal/migrations -run TestPMOSyncMigrationsStayTenantScopedAndConcurrent -count=1`

Expected: FAIL because migration 278 does not exist.

- [ ] **Step 3: Create the tables without inline primary keys or foreign keys**

Use this table contract in migration 278:

```sql
CREATE TABLE pmo_sync_config (
    id UUID NOT NULL DEFAULT gen_random_uuid(),
    workspace_id UUID NOT NULL,
    name TEXT NOT NULL CHECK (btrim(name) <> ''),
    agent_id UUID NOT NULL,
    root_external_key TEXT NOT NULL CHECK (btrim(root_external_key) <> ''),
    workload_property_id UUID,
    schedule_enabled BOOLEAN NOT NULL DEFAULT false,
    next_run_at TIMESTAMPTZ,
    last_run_at TIMESTAMPTZ,
    last_applied_at TIMESTAMPTZ,
    created_by UUID NOT NULL,
    created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT now()
);

CREATE TABLE pmo_sync_run (
    id UUID NOT NULL DEFAULT gen_random_uuid(),
    workspace_id UUID NOT NULL,
    config_id UUID NOT NULL,
    agent_task_id UUID,
    trigger TEXT NOT NULL CHECK (trigger IN ('manual', 'scheduled')),
    status TEXT NOT NULL CHECK (status IN (
        'queued', 'running', 'preview_ready', 'applied',
        'applied_with_review', 'failed'
    )),
    source_snapshot JSONB NOT NULL DEFAULT '{}'::jsonb,
    diff JSONB NOT NULL DEFAULT '{}'::jsonb,
    summary JSONB NOT NULL DEFAULT '{}'::jsonb,
    error_code TEXT,
    error_message TEXT,
    requested_by UUID,
    created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    started_at TIMESTAMPTZ,
    completed_at TIMESTAMPTZ,
    applied_at TIMESTAMPTZ
);

CREATE TABLE pmo_sync_link (
    id UUID NOT NULL DEFAULT gen_random_uuid(),
    workspace_id UUID NOT NULL,
    config_id UUID NOT NULL,
    external_type TEXT NOT NULL CHECK (external_type IN ('requirement', 'task', 'assignee')),
    external_key TEXT NOT NULL,
    external_display_number TEXT,
    external_numeric_id BIGINT,
    external_task_id TEXT,
    parent_external_key TEXT,
    local_type TEXT CHECK (local_type IN ('project', 'issue', 'member')),
    local_id UUID,
    baseline_external JSONB NOT NULL DEFAULT '{}'::jsonb,
    baseline_local JSONB NOT NULL DEFAULT '{}'::jsonb,
    external_metadata JSONB NOT NULL DEFAULT '{}'::jsonb,
    externally_removed_at TIMESTAMPTZ,
    created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT now()
);
```

Each index migration contains exactly one statement. Build unique ID indexes
concurrently, attach them as primary keys in migration 282, then add:

```sql
CREATE UNIQUE INDEX CONCURRENTLY pmo_sync_config_workspace_root_idx
ON pmo_sync_config (workspace_id, root_external_key);

CREATE INDEX CONCURRENTLY pmo_sync_run_history_idx
ON pmo_sync_run (workspace_id, config_id, created_at DESC);

CREATE UNIQUE INDEX CONCURRENTLY pmo_sync_run_active_idx
ON pmo_sync_run (workspace_id, config_id)
WHERE status IN ('queued', 'running');

CREATE UNIQUE INDEX CONCURRENTLY pmo_sync_run_agent_task_idx
ON pmo_sync_run (agent_task_id)
WHERE agent_task_id IS NOT NULL;

CREATE UNIQUE INDEX CONCURRENTLY pmo_sync_link_identity_idx
ON pmo_sync_link (workspace_id, config_id, external_type, external_key);
```

- [ ] **Step 4: Add tenant-scoped sqlc queries**

`pmo.sql` must include config CRUD, due-config claim, run lifecycle, link list
and upsert, assignee mapping, and application cleanup. Every query includes
`workspace_id`; representative signatures are:

```sql
-- name: GetPMOSyncConfig :one
SELECT * FROM pmo_sync_config WHERE id = $1 AND workspace_id = $2;

-- name: GetPMOSyncRunByAgentTask :one
SELECT * FROM pmo_sync_run
WHERE agent_task_id = $1 AND workspace_id = $2;

-- name: ListPMOSyncLinks :many
SELECT * FROM pmo_sync_link
WHERE workspace_id = $1 AND config_id = $2
ORDER BY external_type, external_key;
```

- [ ] **Step 5: Generate and verify SQL code**

Run: `rtk make sqlc`

Expected: PASS and create `server/pkg/db/generated/pmo.sql.go` plus the three
new generated model structs.

Run: `rtk go -C server test ./internal/migrations ./pkg/db/generated -count=1`

Expected: PASS.

- [ ] **Step 6: Commit the schema**

```bash
rtk git add server/migrations server/internal/migrations/migrations_lint_test.go server/pkg/db/queries/pmo.sql server/pkg/db/generated
rtk git commit -m "feat(pmo): add sync persistence"
```

## Task 2: Validate and Normalize the Agent Snapshot

**Files:**

- Create: `server/internal/service/pmo_contract.go`
- Create: `server/internal/service/pmo_contract_test.go`

- [ ] **Step 1: Write strict contract tests**

Cover one complete valid hierarchy and rejection of prose/fences, unknown
fields, wrong schema version, `snapshot_complete != true`, duplicate keys,
duplicate task IDs, invalid project/issue statuses, invalid dates, oversized
strings, and trailing JSON:

```go
func TestParsePMOSnapshotRejectsIncompleteSnapshot(t *testing.T) {
    raw := `{"schema_version":"1","snapshot_complete":false}`
    _, err := ParsePMOSnapshot(raw)
    if !errors.Is(err, ErrIncompletePMOSnapshot) {
        t.Fatalf("expected incomplete snapshot, got %v", err)
    }
}

func TestParsePMOSnapshotPreservesRequirementAndTaskIDs(t *testing.T) {
    got := mustParsePMOSnapshot(t, validPMOSnapshotJSON())
    if got.Parent.Key != "EXT-P-001" || got.Children[0].NumericID != 1002 || got.Children[0].Tasks[0].TaskID != "TASK-001" {
        t.Fatalf("identities were not preserved: %#v", got)
    }
}
```

- [ ] **Step 2: Run the contract tests and verify they fail**

Run: `rtk go -C server test ./internal/service -run 'TestParsePMOSnapshot' -count=1`

Expected: FAIL because `ParsePMOSnapshot` is undefined.

- [ ] **Step 3: Implement the smallest strict decoder**

Define typed structs, cap output at 2 MiB, use `json.Decoder` with
`DisallowUnknownFields`, verify EOF, trim strings, validate contextual status
sets, and flatten the hierarchy to ordered normalized entities:

```go
func ParsePMOSnapshot(output string) (PMOSnapshot, error) {
    if len(output) > maxPMOSnapshotBytes {
        return PMOSnapshot{}, ErrPMOSnapshotTooLarge
    }
    dec := json.NewDecoder(strings.NewReader(output))
    dec.DisallowUnknownFields()
    var snapshot PMOSnapshot
    if err := dec.Decode(&snapshot); err != nil {
        return PMOSnapshot{}, fmt.Errorf("decode pmo snapshot: %w", err)
    }
    if err := requireJSONEOF(dec); err != nil {
        return PMOSnapshot{}, err
    }
    if err := snapshot.Validate(); err != nil {
        return PMOSnapshot{}, err
    }
    return snapshot.Normalize(), nil
}
```

- [ ] **Step 4: Run the contract tests**

Run: `rtk go -C server test ./internal/service -run 'TestParsePMOSnapshot' -count=1`

Expected: PASS.

- [ ] **Step 5: Commit the contract**

```bash
rtk git add server/internal/service/pmo_contract.go server/internal/service/pmo_contract_test.go
rtk git commit -m "feat(pmo): validate agent snapshots"
```

## Task 3: Compute Field-Level Three-Way Differences

**Files:**

- Create: `server/internal/service/pmo_diff.go`
- Create: `server/internal/service/pmo_diff_test.go`

- [ ] **Step 1: Write one table-driven matrix test and hierarchy tests**

Use JSON-compatible scalar values so the same comparator handles project and
issue fields:

```go
func TestDiffPMOFieldMatrix(t *testing.T) {
    cases := []struct {
        name string
        e0, l0, e1, l1 any
        want PMOFieldDecision
    }{
        {"unchanged", "a", "a", "a", "a", PMOUnchanged},
        {"external only", "a", "a", "b", "a", PMOIncoming},
        {"local only", "a", "a", "a", "b", PMOLocalOnly},
        {"converged", "a", "a", "b", "b", PMOConverged},
        {"conflict", "a", "a", "b", "c", PMOConflict},
    }
    for _, tc := range cases {
        t.Run(tc.name, func(t *testing.T) {
            if got := DiffPMOField(tc.e0, tc.l0, tc.e1, tc.l1); got != tc.want {
                t.Fatalf("got %s, want %s", got, tc.want)
            }
        })
    }
}
```

Also test create actions, missing external entities becoming
`external_removed`, child issues retaining parent references, and unresolved
assignees producing warnings without blocking other fields.

- [ ] **Step 2: Run the diff tests and verify they fail**

Run: `rtk go -C server test ./internal/service -run 'TestDiffPMO' -count=1`

Expected: FAIL because diff symbols are undefined.

- [ ] **Step 3: Implement pure comparison and summary types**

Define `PMODiff`, `PMOEntityDiff`, `PMOFieldDiff`, and `PMODiffSummary`.
Compare only approved project/issue fields. Baselines advance only for create,
incoming, converged, or explicit conflict resolution; local-only decisions do
not advance:

```go
func DiffPMOField(e0, l0, e1, l1 any) PMOFieldDecision {
    externalChanged := !reflect.DeepEqual(e0, e1)
    localChanged := !reflect.DeepEqual(l0, l1)
    switch {
    case !externalChanged && !localChanged:
        return PMOUnchanged
    case externalChanged && !localChanged:
        return PMOIncoming
    case !externalChanged && localChanged:
        return PMOLocalOnly
    case reflect.DeepEqual(e1, l1):
        return PMOConverged
    default:
        return PMOConflict
    }
}
```

- [ ] **Step 4: Run all PMO pure tests**

Run: `rtk go -C server test ./internal/service -run 'PMO' -count=1`

Expected: PASS.

- [ ] **Step 5: Commit the diff engine**

```bash
rtk git add server/internal/service/pmo_diff.go server/internal/service/pmo_diff_test.go
rtk git commit -m "feat(pmo): add three-way diff engine"
```

## Task 4: Create Configurations and Dispatch Typed Agent Runs

**Files:**

- Create: `server/internal/service/pmo.go`
- Create: `server/internal/service/pmo_test.go`
- Create: `server/internal/handler/pmo.go`
- Create: `server/internal/handler/pmo_test.go`
- Modify: `server/internal/handler/handler.go`
- Modify: `server/cmd/server/router.go`
- Modify: `server/pkg/db/queries/workspace_delete.sql`
- Generate: `server/pkg/db/generated/workspace_delete.sql.go`

- [ ] **Step 1: Run GitNexus impact before touching existing symbols**

Run:

```bash
rtk npx gitnexus impact --repo /private/tmp/multica-worktrees/feat-pm-task-sync --direction upstream Handler
rtk npx gitnexus impact --repo /private/tmp/multica-worktrees/feat-pm-task-sync --direction upstream workspaceScoped
```

Record direct callers, affected processes, and risk in the implementation
checkpoint. Stop and warn before editing when either result is HIGH or
CRITICAL.

- [ ] **Step 2: Write handler and service failure tests**

Cover owner/admin writes, member read-only access, cross-workspace config/run
404s, archived/no-runtime/non-invokable Agent rejection, duplicate root 409,
schedule enable before first apply 400, and atomic cleanup:

```go
func TestCreatePMOConfigRejectsNonInvokableAgent(t *testing.T) {
    req := newRequest("POST", "/api/pmo/configs?workspace_id="+testWorkspaceID, map[string]any{
        "name": "Example import", "agent_id": privateOtherAgentID,
        "root_external_key": "EXT-P-001",
    })
    w := httptest.NewRecorder()
    testHandler.CreatePMOConfig(w, req)
    if w.Code != http.StatusForbidden {
        t.Fatalf("got %d: %s", w.Code, w.Body.String())
    }
}
```

- [ ] **Step 3: Run the focused tests and verify they fail**

Run: `rtk go -C server test ./internal/handler ./internal/service -run 'PMOConfig|PMORun' -count=1`

Expected: FAIL because PMO handlers/services are undefined.

- [ ] **Step 4: Implement config CRUD and manual run creation**

`PMOService.StartRun` creates the run and `CreateQuickCreateTask` in one
transaction, stores the task ID on the run, then notifies the runtime only
after commit. The context contains no source URL or installed capability name:

```go
const PMOSyncContextType = "pmo_sync"

type PMOSyncContext struct {
    Type        string `json:"type"`
    WorkspaceID string `json:"workspace_id"`
    RequesterID string `json:"requester_id,omitempty"`
    RunID       string `json:"run_id"`
    Prompt      string `json:"prompt"`
}
```

The prompt includes the runtime `root_external_key`, exact JSON schema,
canonical status enums, `snapshot_complete: true`, and “return JSON only”. It
does not include a domain, company, real fixture, nested Agent name, or skill
name.

- [ ] **Step 5: Register workspace-scoped endpoints and cleanup**

Add:

```go
r.Route("/api/pmo", func(r chi.Router) {
    r.Get("/configs", h.ListPMOConfigs)
    r.Post("/configs", h.CreatePMOConfig)
    r.Put("/configs/{id}", h.UpdatePMOConfig)
    r.Delete("/configs/{id}", h.DeletePMOConfig)
    r.Post("/configs/{id}/runs", h.StartPMORun)
    r.Get("/runs", h.ListPMORuns)
    r.Get("/runs/{id}", h.GetPMORun)
})
```

Delete PMO links/runs/configs explicitly during workspace teardown before the
workspace row is deleted.

- [ ] **Step 6: Generate SQL, run tests, and commit**

Run: `rtk make sqlc`

Run: `rtk go -C server test ./internal/handler ./internal/service -run 'PMOConfig|PMORun' -count=1`

Expected: PASS.

```bash
rtk git add server/internal/service/pmo.go server/internal/service/pmo_test.go server/internal/handler/pmo.go server/internal/handler/pmo_test.go server/internal/handler/handler.go server/cmd/server/router.go server/pkg/db/queries/workspace_delete.sql server/pkg/db/generated
rtk git commit -m "feat(pmo): configure and start sync runs"
```

## Task 5: Carry PMO Context Through Agent Completion and Failure

**Files:**

- Modify: `server/internal/service/task.go`
- Modify: `server/internal/service/design_restore_context_test.go`
- Modify: `server/internal/handler/agent.go`
- Modify: `server/internal/handler/daemon.go`
- Modify: `server/internal/handler/daemon_test.go`
- Modify: `server/internal/daemon/types.go`
- Modify: `server/internal/daemon/prompt.go`
- Modify: `server/internal/daemon/execenv/context.go`
- Modify: `server/internal/daemon/execenv/execenv.go`
- Test: `server/internal/daemon/execenv/context_test.go`

- [ ] **Step 1: Run impact analysis for every shared task symbol**

Run:

```bash
rtk npx gitnexus impact --repo /private/tmp/multica-worktrees/feat-pm-task-sync --direction upstream CompleteTask
rtk npx gitnexus impact --repo /private/tmp/multica-worktrees/feat-pm-task-sync --direction upstream ResolveTaskWorkspaceID
```

Disambiguate `CompleteTask` with
`--file server/internal/handler/daemon.go` if required. Report the blast radius
and warn before changes when risk is HIGH or CRITICAL.

- [ ] **Step 2: Write failing transport and lifecycle tests**

Assert claim responses expose only `pmo_sync_context`, the daemon prompt
contains the strict acquisition request, task workspace resolution comes from
the context, valid completion stores `preview_ready`, invalid completion marks
both task and run failed, and `/fail` marks the run failed:

```go
func TestCompletePMOSyncTaskStoresPreview(t *testing.T) {
    task, run := seedRunningPMOTask(t)
    w := completeTask(t, task.ID, validPMOSnapshotJSON())
    if w.Code != http.StatusOK {
        t.Fatalf("complete: %d %s", w.Code, w.Body.String())
    }
    got := mustGetPMORun(t, run.ID)
    if got.Status != "preview_ready" || len(got.SourceSnapshot) == 0 {
        t.Fatalf("run not prepared: %#v", got)
    }
}
```

- [ ] **Step 3: Run focused tests and verify they fail**

Run: `rtk go -C server test ./internal/handler ./internal/daemon/... ./internal/service -run 'PMOSync|PMOContext' -count=1`

Expected: FAIL because the task pipeline does not recognize `pmo_sync`.

- [ ] **Step 4: Add the typed context with minimal shared branches**

Add parse/render branches next to existing design task contexts. Extend
`TaskService.ResolveTaskWorkspaceID`, claim response types, prompt rendering,
and execution-kind selection. In `Handler.CompleteTask`, parse and prepare the
PMO run before terminal task completion; use
`CompleteTaskWithMutationAndSessionState` so task completion and preview
persistence commit together. Invalid output follows the existing typed-task
failure path and stores only a redacted error.

- [ ] **Step 5: Update task failure handling**

After `TaskService.FailTask` succeeds, detect `PMOSyncContext` and update the
corresponding run:

```go
if pmoCtx, ok := service.ParsePMOSyncContext(task.Context); ok {
    _ = h.PMOService.FailRun(ctx, pmoCtx.RunID, "agent_failed", req.Error)
}
```

Bound the stored error and never log/output the Agent payload.

- [ ] **Step 6: Run task regression tests and commit**

Run: `rtk go -C server test ./internal/handler ./internal/daemon/... ./internal/service -run 'PMOSync|PMOContext|UIDraft|DesignRestore' -count=1`

Expected: PASS.

```bash
rtk git add server/internal/service/task.go server/internal/service/design_restore_context_test.go server/internal/handler/agent.go server/internal/handler/daemon.go server/internal/handler/daemon_test.go server/internal/daemon
rtk git commit -m "feat(pmo): process agent sync results"
```

## Task 6: Apply Previewed Changes Transactionally

**Files:**

- Modify: `server/internal/service/issue.go`
- Modify: `server/internal/service/issue_test.go`
- Modify: `server/internal/service/pmo.go`
- Create: `server/internal/service/pmo_apply_test.go`
- Modify: `server/internal/handler/pmo.go`
- Modify: `server/internal/handler/pmo_test.go`
- Modify: `server/cmd/server/router.go`
- Modify: `server/pkg/db/queries/pmo.sql`
- Generate: `server/pkg/db/generated/pmo.sql.go`

- [ ] **Step 1: Run impact analysis before the issue-service refactor**

Run:

```bash
rtk npx gitnexus impact --repo /private/tmp/multica-worktrees/feat-pm-task-sync --direction upstream Create --file server/internal/service/issue.go
```

Report direct callers and affected issue-creation flows. Warn before editing if
risk is HIGH or CRITICAL.

- [ ] **Step 2: Lock the existing issue behavior with a refactor test**

Add a test that creates an issue through `IssueService.Create`, verifies issue
numbering, parent/project validation, and one `issue:created` event. This must
pass before and after extracting the transaction-local helper.

- [ ] **Step 3: Extract transaction-local issue creation without changing behavior**

Keep public `Create` as the owner of begin/commit and post-commit effects, but
move steps 2-6 into an unexported same-package helper callable by PMO:

```go
func (s *IssueService) createInTx(
    ctx context.Context,
    tx pgx.Tx,
    qtx *db.Queries,
    p IssueCreateParams,
) (IssueCreateResult, error)
```

Add a small `afterCreate` helper for events/analytics/assignment after the
outer transaction commits.

- [ ] **Step 4: Write failing PMO apply integration tests**

Cover first import, idempotent rerun, hierarchy, field-level incoming updates,
local-only preservation, conflict choices, local edits after preview, external
removal/reappearance, unknown assignee, member mapping, workload property,
and full rollback when a child fails:

```go
func TestApplyPMORunRollsBackWholeHierarchy(t *testing.T) {
    run := seedPreviewWithInvalidSecondChild(t)
    if err := pmoService.ApplyRun(ctx, run.ID, nil); err == nil {
        t.Fatal("expected apply error")
    }
    assertNoPMOProjectOrIssueRows(t, run.ConfigID)
}
```

- [ ] **Step 5: Implement apply under one config lock and transaction**

`ApplyRun` must:

1. lock config and run in the request workspace;
2. re-read canonical project/issues and recompute against the stored snapshot;
3. create/reuse the numeric workload property and persist
   `workload_property_id`;
4. create project, top-level issues, then child issues;
5. update only incoming/converged or explicitly resolved fields;
6. upsert links and baselines with canonical local values;
7. mark missing external links with `externally_removed_at` only;
8. mark the run `applied` or `applied_with_review` and update config times;
9. commit, then publish create effects.

Conflict input is explicit and field-scoped:

```go
type PMOConflictResolution struct {
    ExternalType string `json:"external_type"`
    ExternalKey  string `json:"external_key"`
    Field        string `json:"field"`
    Choice       string `json:"choice"` // external | local
}
```

- [ ] **Step 6: Add apply and mapping endpoints**

```go
r.Post("/runs/{id}/apply", h.ApplyPMORun)
r.Put("/configs/{id}/assignees/{externalKey}", h.SetPMOAssigneeMapping)
```

Both require owner/admin. Mapping validates the selected member belongs to the
workspace and never matches by display name.

- [ ] **Step 7: Generate SQL, run backend tests, and commit**

Run: `rtk make sqlc`

Run: `rtk go -C server test ./internal/service ./internal/handler -run 'IssueService|PMO' -count=1`

Expected: PASS.

```bash
rtk git add server/internal/service/issue.go server/internal/service/issue_test.go server/internal/service/pmo.go server/internal/service/pmo_apply_test.go server/internal/handler/pmo.go server/internal/handler/pmo_test.go server/cmd/server/router.go server/pkg/db/queries/pmo.sql server/pkg/db/generated
rtk git commit -m "feat(pmo): apply requirement diffs"
```

## Task 7: Dispatch Enabled Configurations Every 30 Minutes

**Files:**

- Create: `server/internal/scheduler/jobs_pmo.go`
- Create: `server/internal/scheduler/jobs_pmo_test.go`
- Modify: `server/cmd/server/main.go`
- Create: `server/cmd/server/pmo_schedule_job_test.go`
- Modify: `server/internal/handler/daemon.go`
- Modify: `server/internal/handler/daemon_test.go`

- [ ] **Step 1: Run impact analysis on scheduler registration**

Run:

```bash
rtk npx gitnexus impact --repo /private/tmp/multica-worktrees/feat-pm-task-sync --direction upstream Register --file server/internal/scheduler/manager.go
rtk npx gitnexus impact --repo /private/tmp/multica-worktrees/feat-pm-task-sync --direction upstream CompleteTask --file server/internal/handler/daemon.go
```

Report the risk before editing `main.go` or scheduler symbols.

- [ ] **Step 2: Write failing schedule tests**

Test due dispatch, disabled config skip, first-apply guard, two-runner single
winner, active-run skip, next-run advance, and downtime collapse:

```go
func TestPMOScheduleDispatchCollapsesMissedIntervals(t *testing.T) {
    cfg := seedDuePMOConfig(t, time.Now().Add(-6*time.Hour))
    runPMOSchedulerTick(t)
    assertRunCount(t, cfg.ID, 1)
    assertNextRunNear(t, cfg.ID, time.Now().Add(30*time.Minute))
}
```

- [ ] **Step 3: Run the scheduler tests and verify they fail**

Run: `rtk go -C server test ./internal/scheduler ./cmd/server -run 'PMOSchedule' -count=1`

Expected: FAIL because the PMO job is undefined.

- [ ] **Step 4: Implement one latest-only global scanner**

Register a minute-cadence job that calls `PMOService.DispatchDuePMORuns`. The
service claims due config rows transactionally, advances each to database
`now() + interval '30 minutes'`, creates at most one queued run through the
active unique index, and applies safe changes after a valid scheduled Agent
completion:

```go
const JobNamePMOSyncDispatch = "pmo_sync_dispatch"

func PMOSyncDispatchJob(dispatcher PMOSyncDispatcher) JobSpec {
    return JobSpec{
        Name:        JobNamePMOSyncDispatch,
        Cadence:     time.Minute,
        CatchUpMode: CatchUpLatestOnly,
        CatchUpWindow: 24 * time.Hour,
        RunTimeout: 50 * time.Second,
        StaleTimeout: 2 * time.Minute,
        HeartbeatInterval: 20 * time.Second,
        AllowStaleReentry: true,
        MaxAttempts: 3,
        RetryBackoff: []time.Duration{time.Minute, 5 * time.Minute, 15 * time.Minute},
        Scopes: StaticScopes(ScopeGlobal),
        Handler: func(ctx context.Context, _ HandlerInput) (HandlerResult, error) {
            count, err := dispatcher.DispatchDuePMORuns(ctx)
            return HandlerResult{RowsAffected: count}, err
        },
    }
}
```

After a scheduled task stores a valid preview, the completion hook calls
`PMOService.ApplyRun` with no conflict overrides. If automatic apply fails,
keep the run
`preview_ready` with a redacted apply error so an admin can review and retry;
never discard the acquired snapshot.

- [ ] **Step 5: Register, run tests, and commit**

Run: `rtk go -C server test ./internal/scheduler ./cmd/server -run 'PMOSchedule' -count=1`

Expected: PASS.

```bash
rtk git add server/internal/scheduler/jobs_pmo.go server/internal/scheduler/jobs_pmo_test.go server/cmd/server/main.go server/cmd/server/pmo_schedule_job_test.go server/internal/handler/daemon.go server/internal/handler/daemon_test.go
rtk git commit -m "feat(pmo): schedule requirement sync"
```

## Task 8: Add Parsed PMO Client State

**Files:**

- Create: `packages/core/types/pmo.ts`
- Modify: `packages/core/types/index.ts`
- Modify: `packages/core/api/schemas.ts`
- Modify: `packages/core/api/schemas.test.ts`
- Modify: `packages/core/api/client.ts`
- Modify: `packages/core/api/client.test.ts`
- Create: `packages/core/pmo/queries.ts`
- Create: `packages/core/pmo/mutations.ts`
- Create: `packages/core/pmo/queries.test.ts`

- [ ] **Step 1: Run impact analysis for the API client and workspace paths**

Run:

```bash
rtk npx gitnexus impact --repo /private/tmp/multica-worktrees/feat-pm-task-sync --direction upstream ApiClient
rtk npx gitnexus impact --repo /private/tmp/multica-worktrees/feat-pm-task-sync --direction upstream workspaceScoped
```

Report risk and affected consumers before editing.

- [ ] **Step 2: Write schema/client/query tests first**

Test fallback defaults for optional newer fields, rejection of invalid status,
exact request paths/bodies, and workspace ID in every query key:

```ts
expect(pmoKeys.runs("ws-1", "cfg-1")).toEqual([
  "pmo", "ws-1", "runs", "cfg-1",
]);

expect(parsePMORun({ ...baseRun, status: "future_status" }).status).toBe("failed");
```

- [ ] **Step 3: Run focused core tests and verify they fail**

Run: `rtk pnpm --filter @multica/core test -- pmo api/schemas.test.ts api/client.test.ts`

Expected: FAIL because PMO types and methods do not exist.

- [ ] **Step 4: Implement types, zod boundary, client, queries, and mutations**

Use snake_case only at the network boundary and preserve server enums as
literal unions. Every endpoint result passes through `parseWithFallback`.
Mutations invalidate PMO keys plus affected project and issue lists after
apply; do not optimistically mutate an apply operation that can conflict.

```ts
export const pmoKeys = {
  all: (wsId: string) => ["pmo", wsId] as const,
  configs: (wsId: string) => [...pmoKeys.all(wsId), "configs"] as const,
  runs: (wsId: string, configId: string) =>
    [...pmoKeys.all(wsId), "runs", configId] as const,
  run: (wsId: string, runId: string) =>
    [...pmoKeys.all(wsId), "run", runId] as const,
};
```

- [ ] **Step 5: Run core tests and commit**

Run: `rtk pnpm --filter @multica/core test -- pmo api/schemas.test.ts api/client.test.ts`

Expected: PASS.

```bash
rtk git add packages/core/types/pmo.ts packages/core/types/index.ts packages/core/api packages/core/pmo
rtk git commit -m "feat(pmo): add client data layer"
```

## Task 9: Build the Shared Requirement Management Page

**Files:**

- Create: `packages/views/pmo/index.ts`
- Create: `packages/views/pmo/pmo-page.tsx`
- Create: `packages/views/pmo/pmo-page.test.tsx`
- Create: `packages/views/locales/en/pmo.json`
- Create: `packages/views/locales/zh-Hans/pmo.json`
- Create: `packages/views/locales/ja/pmo.json`
- Create: `packages/views/locales/ko/pmo.json`
- Modify: `packages/views/locales/index.ts`
- Modify: `packages/views/locales/parity.test.ts`
- Modify: `packages/views/locales/{en,zh-Hans,ja,ko}/layout.json`
- Modify: `packages/core/paths/paths.ts`
- Modify: `packages/core/paths/paths.test.ts`
- Modify: `packages/core/paths/route-icons.ts`
- Modify: `packages/core/paths/route-icons.test.ts`
- Modify: `packages/views/layout/route-icon-components.tsx`
- Modify: `packages/views/layout/app-sidebar.tsx`
- Modify: `packages/views/layout/app-sidebar.test.tsx`
- Create: `apps/web/app/[workspaceSlug]/(dashboard)/pmo/page.tsx`
- Modify: `apps/desktop/src/renderer/src/routes.tsx`

- [ ] **Step 1: Run impact analysis for navigation and sidebar symbols**

Run:

```bash
rtk npx gitnexus impact --repo /private/tmp/multica-worktrees/feat-pm-task-sync --direction upstream workspaceScoped
rtk npx gitnexus impact --repo /private/tmp/multica-worktrees/feat-pm-task-sync --direction upstream WORKSPACE_PAGES
rtk npx gitnexus impact --repo /private/tmp/multica-worktrees/feat-pm-task-sync --direction upstream AppSidebar
```

Report risk before editing shared navigation.

- [ ] **Step 2: Write page and navigation tests**

Cover the `/acme/pmo` path, sidebar label/icon, loading/empty/failed states,
config create/edit, Agent selection, schedule guard, manual run, diff filters,
apply confirmation, conflict choices, mapping queue, retry, and existing create
dialogs:

```tsx
it("opens existing create workflows", async () => {
  renderPMOPage();
  await user.click(screen.getByRole("button", { name: "新建项目" }));
  expect(useModalStore.getState().modal).toBe("create-project");
  await user.click(screen.getByRole("button", { name: "新建 issue" }));
  expect(useModalStore.getState().modal).toBe("create-issue");
});
```

- [ ] **Step 3: Run focused view/path tests and verify they fail**

Run: `rtk pnpm --filter @multica/core test -- paths pmo`

Run: `rtk pnpm --filter @multica/views test -- pmo-page app-sidebar parity`

Expected: FAIL because the path, namespace, nav item, and page do not exist.

- [ ] **Step 4: Add route and navigation registries**

Add `pmo: () => \`${ws}/pmo\`` to workspace paths, a `pmo` page entry with
the lucide `ClipboardList` icon, a sidebar item after projects, and Web/Desktop
route shells that render the same `PMOPage`.

- [ ] **Step 5: Implement the operational page**

Use a constrained full-width layout, not nested cards. The header owns the
configuration selector, Agent selector, external key, fixed schedule switch,
sync icon button, and existing create actions. Below it, use compact tabs for
Preview, Assignee mappings, and Run history. A table renders field-level old,
external, and local values with explicit conflict controls.

Use stable layouts and existing primitives:

```tsx
<Button size="icon" onClick={() => startRun.mutate(config.id)} aria-label={t(($) => $.actions.sync_now)}>
  <RefreshCw className="size-4" />
</Button>
<Button size="icon" variant="outline" onClick={() => useModalStore.getState().open("create-project")} aria-label={t(($) => $.actions.new_project)}>
  <FolderPlus className="size-4" />
</Button>
```

Unknown values and long titles truncate with a tooltip; action buttons retain
fixed dimensions; mobile stacks controls without overlap.

- [ ] **Step 6: Add all four locale namespaces and run tests**

Run: `rtk pnpm --filter @multica/core test -- paths pmo`

Run: `rtk pnpm --filter @multica/views test -- pmo-page app-sidebar parity`

Expected: PASS.

- [ ] **Step 7: Commit the page**

```bash
rtk git add packages/core/paths packages/views/pmo packages/views/layout packages/views/locales apps/web/app/[workspaceSlug]/\(dashboard\)/pmo apps/desktop/src/renderer/src/routes.tsx
rtk git commit -m "feat(pmo): add requirement management page"
```

## Task 10: Verify the Complete Feature and Preview One Runtime Requirement

**Files:**

- Modify only if verification exposes a defect in files already owned by Tasks 1-9.

- [ ] **Step 1: Run format and generated-code checks**

Run: `rtk make sqlc`

Run: `rtk gofmt -w server/internal/service/pmo*.go server/internal/handler/pmo*.go server/internal/scheduler/jobs_pmo*.go`

Expected: no uncommitted generated drift after formatting and regeneration.

- [ ] **Step 2: Run backend verification**

Run: `rtk go -C server test -race ./internal/service ./internal/handler ./internal/scheduler ./cmd/server`

Expected: PASS.

- [ ] **Step 3: Run frontend verification**

Run: `rtk pnpm typecheck`

Run: `rtk pnpm test`

Expected: PASS.

- [ ] **Step 4: Run migration and repository checks**

Run: `rtk make check`

Expected: PASS, including migration lint, Go tests, TypeScript checks, and
locale parity.

- [ ] **Step 5: Start the isolated worktree application**

Run: `rtk make start-worktree`

Expected: Web is reachable at `http://localhost:13221` and the backend uses the
worktree-isolated database.

- [ ] **Step 6: Verify the page at desktop and mobile widths**

Use Playwright screenshots at 1440x900 and 390x844. Confirm the page is
nonblank, controls do not overlap, long identifiers fit, the selected Agent is
not hardcoded, schedule defaults off, and manual sync is preview-only.

- [ ] **Step 7: Perform the authorized runtime-only smoke preview**

In the UI, create one config with the user's selected Agent and runtime-only
external key, then start one manual run. Verify it reaches `preview_ready` and
shows the expected project/issue hierarchy. Do not press Apply during this
smoke step, do not log the payload, and do not commit the key or returned data.

- [ ] **Step 8: Run GitNexus change detection before the final commit**

Run:

```bash
rtk npx gitnexus detect-changes --repo /private/tmp/multica-worktrees/feat-pm-task-sync --scope compare --base-ref main
```

Expected: only PMO persistence, Agent task integration, scheduler registration,
canonical issue creation refactor, API/client state, and navigation/page flows
are reported. Investigate any unrelated process.

- [ ] **Step 9: Commit verification-only corrections**

When verification required code corrections:

```bash
rtk git add -u
rtk git commit -m "fix(pmo): address integration verification"
```

When no correction was required, leave the branch unchanged.
