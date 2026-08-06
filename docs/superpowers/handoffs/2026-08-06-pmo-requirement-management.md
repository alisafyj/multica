# PMO Requirement Management Handoff

## Resume Location

- Worktree: `/private/tmp/multica-worktrees/feat-pm-task-sync`
- Branch: `feat/pm-task-sync`
- Base: `origin/main` at `18daaea6f`
- Current committed HEAD: `825c80475`
- The worktree intentionally contains uncommitted Task 4 code. Do not discard it.

Read these first:

- `CLAUDE.md`
- `docs/superpowers/specs/2026-08-06-pmo-requirement-management-design.md`
- `docs/superpowers/plans/2026-08-06-pmo-requirement-management.md`

All shell commands in this repository must be prefixed with `rtk`.

## Completed And Committed

1. `999a48a39 docs: design PMO requirement management`
2. `f8db757c1 docs: plan PMO requirement management`
3. `03271ccd7 feat(pmo): add sync persistence`
   - Added migrations `278` through `287` for `pmo_sync_config`,
     `pmo_sync_run`, and `pmo_sync_link`.
   - Added migration lint coverage for no foreign keys and one concurrent
     statement per index migration.
   - Added tenant-scoped PMO sqlc queries and generated code.
4. `83ff8733e feat(pmo): validate agent snapshots`
   - Added a strict 2 MiB JSON boundary with unknown-field and trailing-content
     rejection.
   - Validates complete snapshots, schema version, stable identities, status
     sets, dates, sizes, hierarchy shape, and duplicate requirement/task IDs.
   - Normalizes strings before diff or persistence.
5. `825c80475 feat(pmo): add three-way diff engine`
   - Added the field decision matrix: unchanged, incoming, local-only,
     converged, and conflict.
   - Added deterministic project/top-level issue/child issue ordering.
   - Added external-removal actions and unresolved-assignee warnings without
     destructive local behavior.

## Verification Already Run

The following checks passed before their commits:

```bash
rtk go -C server test ./internal/migrations ./pkg/db/generated -count=1
rtk go -C server test ./internal/service -run 'TestParsePMOSnapshot' -count=1
rtk go -C server test ./internal/service -run 'PMO' -count=1
```

GitNexus `detect-changes --scope compare --base-ref main` reported LOW risk and
zero affected execution flows for Tasks 1 through 3.

## Current Uncommitted Task 4 Work

Modified tracked files:

- `server/internal/handler/handler.go`
  - Added `PMOService` to `Handler`.
  - Kept the existing `handler.New` function signature unchanged.
- `server/cmd/server/router.go`
  - Added `/api/pmo` configuration and run routes inside the existing
    authenticated workspace group.

New untracked files:

- `server/internal/service/pmo.go`
  - Config create/update/delete.
  - Manual run creation under a config row lock.
  - PMO run and Agent task creation in one transaction.
  - Post-commit task notification.
  - Generic strict JSON prompt using only the runtime root key.
- `server/internal/service/pmo_test.go`
  - Prompt privacy and strict-contract test.
- `server/internal/handler/pmo.go`
  - Config and run HTTP response types.
  - List/create/update/delete config handlers.
  - Start/list/get run handlers.
  - Owner/admin write gates, member read gates, workspace scoping, and Agent
    invocation checks.
- `server/internal/handler/pmo_test.go`
  - Non-invokable Agent rejection.
  - Schedule-before-first-apply rejection.
  - Second-active-run conflict.

The RED phase was observed before these implementations: the focused tests
failed only because the PMO service and handler symbols did not exist.

After implementation, this command compiled both packages successfully, the
service test passed, and the handler test stopped during database fixture setup:

```bash
rtk go -C server test ./internal/handler ./internal/service \
  -run 'PMOConfig|PMORun|PMOSyncPrompt' -count=1
```

Current blocker output:

```text
Failed to set up handler test fixture: relation "workspace" does not exist
```

This is a local test-database setup issue, not a Go compile failure. Set up and
migrate the isolated worktree database before treating Task 4 as green.

## Impact Analysis Already Run

- `handler.Handler`: HIGH, exact. 29 direct callers, 50 total impacted symbols,
  one affected server startup process. The user was warned before editing.
- `packages/core/paths.workspaceScoped`: LOW, zero callers.
- `server/cmd/server.NewRouterWithOptions`: LOW. Four direct callers, six total
  impacted symbols, one server startup process.
- `DeleteWorkspaceLeafData`: analysis was started but interrupted. Run it again
  before editing `server/pkg/db/queries/workspace_delete.sql`.

## Exact Next Steps

1. Inspect the uncommitted Task 4 diff; do not reset or regenerate it away.

```bash
rtk git status --short
rtk git diff -- server/internal/handler/handler.go server/cmd/server/router.go
```

2. Run the required impact analysis before workspace cleanup changes.

```bash
rtk node .gitnexus/run.cjs impact DeleteWorkspaceLeafData \
  --direction upstream \
  --repo /private/tmp/multica-worktrees/feat-pm-task-sync \
  --file server/pkg/db/generated/workspace_delete.sql.go \
  --depth 3 --include-tests
```

3. Add explicit PMO link, run, and config deletion to
   `server/pkg/db/queries/workspace_delete.sql` in that order. Regenerate sqlc.

```bash
rtk go run github.com/sqlc-dev/sqlc/cmd/sqlc@v1.31.1 generate
```

4. Set up/migrate the worktree database, then rerun the focused Task 4 tests.
   Use the repository worktree targets and inspect `Makefile` before running.

```bash
rtk make worktree-env
rtk make setup-worktree
rtk go -C server test ./internal/handler ./internal/service \
  -run 'PMOConfig|PMORun|PMOSyncPrompt' -count=1
```

5. Review the current Task 4 code before commit. In particular:

- confirm the PMO Agent task attribution matches scheduled-run policy;
- decide whether changing `root_external_key` after the first apply must be
  rejected because existing links belong to the prior root;
- make `decodePMORequest` reject trailing JSON bodies;
- confirm selected-Agent runtime overlays are intentionally unnecessary or
  add the same overlay behavior as other Agent enqueue paths;
- ensure task/context failure paths redact Agent payloads.

6. Run GitNexus change detection, then commit Task 4 only after all focused
   tests pass.

```bash
rtk node .gitnexus/run.cjs detect-changes --scope compare --base-ref main \
  --repo /private/tmp/multica-worktrees/feat-pm-task-sync --limit 100
rtk git add server/internal/service/pmo.go server/internal/service/pmo_test.go \
  server/internal/handler/pmo.go server/internal/handler/pmo_test.go \
  server/internal/handler/handler.go server/cmd/server/router.go \
  server/pkg/db/queries/workspace_delete.sql server/pkg/db/generated
rtk git commit -m "feat(pmo): configure and start sync runs"
```

7. Continue Tasks 5 through 10 from the implementation plan. Task 5 is
   required before a PMO Agent task can complete into a stored preview.

## Non-Negotiable Constraints

- The user chooses an existing Multica Agent. Never hardcode an Agent,
  runtime, nested Agent, or installed capability.
- Multica stores no external-system credential and directly calls no external
  system from PMO code.
- Manual runs are preview-first.
- Scheduled runs apply only non-conflicting safe changes.
- Preserve local edits with field-level three-way comparison.
- External deletion marks `externally_removed_at`; it never deletes canonical
  projects or issues.
- Unknown external assignees remain unassigned and enter the mapping queue.
- Do not commit company names, domains, internal URLs, real IDs, secrets,
  external capability names, or real Agent output.
- Use fictional fixtures such as `EXT-P-001` only.
- Before editing an existing function/class/method, run GitNexus upstream
  impact analysis and report HIGH/CRITICAL risk before editing.
- Before every commit, run GitNexus `detect-changes` against `main`.
- Keep TDD order: failing test, observed expected failure, minimal code, green
  test, then refactor.
