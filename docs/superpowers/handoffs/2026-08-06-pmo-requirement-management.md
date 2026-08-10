# PMO Requirement Management Handoff

## OpenCode Resume Brief (Latest, 2026-08-07)

This section is the current source of truth. The detailed Task 1-10 history
below is retained for audit context, but its old "Current Uncommitted Task 4"
and "Exact Next Steps" sections are superseded.

### Checkout State

- Branch: `feat/pm-task-sync`
- HEAD: `a20b22fde` (`docs(pmo): note completion status in handoff`)
- Remote-tracking ref: `origin/feat/pm-task-sync` at the same commit
- Working tree was clean before this handoff update.
- Current directory: `/private/tmp/multica-worktrees/feat-pm-task-sync`

Important: the original Git worktree directory disappeared and was reported as
`prunable`. Codex recreated this path as a standalone shared clone for
diagnosis. In this clone, `origin` currently points to the main local checkout,
not directly to the GitHub remote. It is safe for reading and local testing,
but verify `rtk git remote -v` and restore a proper remote/worktree setup before
assuming a push reaches GitHub.

The main checkout is on an unrelated branch and has user-owned modifications
to `AGENTS.md` and `CLAUDE.md`. Do not modify, discard, or reset those changes.

### Implementation Status

The PMO requirement-management implementation is complete through Task 10 and
committed. The branch includes:

- PMO persistence, tenant-scoped sqlc queries, and migrations 306-315.
- Strict Agent snapshot validation and normalization.
- Deterministic three-way diff/conflict handling.
- Config CRUD with user-selected Agent; no Agent or capability is hardcoded.
- Manual preview runs, Agent task result processing, apply flow, and schedule.
- Web and desktop `/pmo` routes, shared React Query data layer, sidebar entry,
  four locale bundles, empty/configured/diff states, and conflict controls.
- Workspace cleanup coverage and duplicate issue-create race fix.
- Integration fixes through `b1cf7280d`, including mounting the config dialog
  from the empty state.

No company domain, external-system credential, real external ID, real Agent
payload, or external capability name was added to the branch.

### Verification Re-run By Codex

The following checks passed on `a20b22fde`:

```text
pnpm typecheck
  6/6 tasks passed

pnpm --filter @multica/web build
  Next.js 16.2.6 production build passed
  /[workspaceSlug]/pmo was included in the route manifest

pnpm --filter @multica/views test
  312 test files passed
  3586 tests passed
```

Development-mode evidence:

- `GET /demo/pmo` returned HTTP 200 after compilation.
- Browser console contained no React, hydration, or PMO runtime error.
- Without the full backend/auth environment the dashboard remained at the
  global loading state; that is expected and is not evidence of a PMO page
  failure.

### Local Preview State And Blocker

The diagnostic Next.js server has been stopped. Nothing is currently listening
for this checkout.

`.env.worktree` was generated locally and is gitignored:

```text
Database: multica_feat_pm_task_sync_221
Backend:  http://localhost:18301
Frontend: http://localhost:13221
```

`rtk make setup-worktree` installed dependencies, then stopped while starting
PostgreSQL because this machine does not expose the Docker Compose CLI plugin:

```text
docker compose version
  docker: unknown command: docker compose

docker-compose version
  Docker Compose version 2.34.0
```

Use the installed Compose v2 standalone executable as the Make override:

```bash
rtk make setup-worktree COMPOSE=docker-compose
rtk make start-worktree COMPOSE=docker-compose
```

Then verify both services before opening the UI:

```bash
rtk curl -fsS http://localhost:18301/health
rtk curl -fsSIL http://localhost:13221
```

Open `http://localhost:13221`, not `http://127.0.0.1:13221`.

### Next.js Warning Found During Diagnosis

When the browser used `127.0.0.1`, Next.js logged:

```text
Blocked cross-origin request to Next.js dev resource /_next/webpack-hmr from
"127.0.0.1". To allow this host in development, add it to allowedDevOrigins.
```

This warning is unrelated to the PMO implementation and did not prevent the
page request from returning 200. The existing `apps/web/next.config.ts` intends
to derive `allowedDevOrigins` hostnames from `CORS_ALLOWED_ORIGINS`, but uses
`new URL(...).host`, which retains the port. Next.js 16 compares hostnames, so
the likely root-cause fix is `.hostname`. No code change was made because the
user had only requested diagnosis at that point.

If OpenCode fixes this warning, keep it as a minimal, tested change. Run the
required GitNexus upstream impact analysis before editing the symbol and run
`detect-changes --scope compare --base-ref main` before committing.

### Immediate Resume Steps

1. Read `CLAUDE.md`, the design spec, and the implementation plan.
2. Confirm checkout state and remotes; do not touch main-checkout user changes.
3. Run the two `COMPOSE=docker-compose` commands above and wait for both health
   checks to pass.
4. Open the frontend using `localhost` and perform only the authorized manual
   preview smoke: choose an existing Agent, provide a runtime-only external
   key, start one run, and verify `preview_ready`.
5. Do not Apply external data during the smoke test. Do not log or commit the
   external key or Agent output.
6. If the exact reported browser error differs from the HMR warning above,
   collect the complete terminal/browser stack trace before changing code.

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

## Completion Update (2026-08-07)

This section supersedes "Current Uncommitted Task 4 Work" and
"Exact Next Steps" above.

- Committed through Task 9 plus two integration fixes:
  `a88b612e0` Task 4, `ed9a351fc` Task 5, `1120b0e69` Task 6, `97455210e`
  Task 7, `5bf8530f7` Task 8, `dbf027ffe` Task 9, `dce8e7d94` and
  `b1cf7280d` integration fixes.
- Task 4 review items resolved: `root_external_key` is immutable after the
  first applied run (`ErrPMORootKeyLocked`, 400); `decodePMORequest` rejects
  trailing JSON; `StartRun` stamps the runtime MCP overlay like every other
  enqueue path; scheduled trigger uses trigger-owner style attribution with a
  NULL originator and the workspace owner-fallback policy.
- Task 10 gates green: gofmt and sqlc no drift; `go test -race`
  (service/handler/scheduler/cmd); full `scripts/test-go.sh --race`;
  `pnpm typecheck`; `pnpm test`. Two regressions found by `-race` and fixed:
  duplicate-create result propagation in `IssueService.Create`, and the
  workspace-deletion manifest entries for the three PMO tables.
- `make check` step 0 cannot use the docker compose wrapper on this host; the
  five stages were run manually. Playwright: 21-22 of 30 pass; 8 failures are
  pre-existing/environmental and untouched by this branch (agent-mcp x2,
  issue-table x3, onboarding zh-Hans cookie port 13442, settings x2).
- E2E fixtures require `USE_SY_SSO=true` in `.env.worktree` (fixtures sign
  `auth_source: "sso"` HS256 tokens verified with `JWT_SECRET`), plus a dummy
  `SSO_PUBLIC_KEY_PATH`/`SSO_EXPECTED_SUB`; all local-only, and
  `.env.worktree` is gitignored.
- PMO browser smoke verified with fictional data only: `/{slug}/pmo` empty
  state, create dialog (fixed in `b1cf7280d` to mount from the empty state),
  seeded `preview_ready` run rendering the full diff surface (creates,
  incoming, local-only, conflict controls, external-removed row, unresolved
  owner tab, gated Apply), schedule switch off with guard hint; 1440x900 and
  390x844 screenshots show no overflow or overlap.

### Remaining (user action)

- Authorized runtime-only smoke preview, per the design doc: in the running
  UI create one config with the user's selected Agent and a runtime-only
  external key, start one manual run, verify `preview_ready`. Do not Apply,
  do not log payloads, do not commit keys or returned data.

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
