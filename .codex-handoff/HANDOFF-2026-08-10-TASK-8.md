# Multica Native Design Engine Phase 1 - Task 8 Handoff

> Snapshot time: 2026-08-10, Asia/Shanghai
> Status: Tasks 1-7 complete; Task 8 stopped at a committed security checkpoint.

## 1. Goal

Complete Multica Native Design Engine Phase 1 for the project design-system vertical slice.

The production path must use Multica's existing Project, Agent, daemon, task queue, object storage, Audit, browser preview, draft, save, and discard lifecycle. Open Design is a behavioral reference only. The product must not run, distribute, or host an Open Design Worker, Daemon, or Runtime.

Task 8 is the live acceptance task. It is complete only when one real staffrnapp CRM workflow proves:

- repository analysis uses the selected local Agent and real repository sources;
- generation produces a bound `multica.project-design-system/v2` archive;
- server Audit and daemon Chrome verification pass;
- the user's Chrome shows a nonblank, grounded UI Kit without external requests or template residue;
- adjustment, atomic save, invalid-package isolation, and discard restoration work;
- no new `open_design_run` is created;
- the staffrnapp source repository is restored to its exact pre-task state.

## 2. Hard Boundaries

- Work only in `/Users/fengyujie/.config/superpowers/worktrees/multica/native-design-engine`.
- Branch: `codex/multica-native-design-engine`.
- Code checkpoint before this document: `991ad72c8`; the actual handoff HEAD is the commit containing this document.
- Never edit, clean, reset, or stop services in `/Users/fengyujie/Documents/soyoung/multica` on `feature/fengchen`.
- Never merge `codex/open-design-native-slots`.
- Do not start an Open Design Worker, Daemon, or Runtime.
- Use strict SDD: implementer, Spec review, Quality review, fix loop, scoped re-review.
- Before editing a function, run GitNexus upstream impact and report HIGH/CRITICAL risk.
- Before every commit, stage exact files and run `node .gitnexus/run.cjs detect-changes --scope staged --repo multica`.
- Do not launch another CRM Agent until the user chooses the low-token or strict-complete path in section 9.

## 3. Authoritative Files

Read these after every resume or context compaction:

1. `AGENTS.md`
2. `DEV-WORKFLOW-SOP.md` from the main checkout, read-only
3. `CLAUDE.md`
4. `docs/product/design-center/README.md`
5. `docs/superpowers/plans/2026-08-06-multica-native-design-system-phase-1.md`
6. This handoff
7. `.codex-handoff/PROMPT-2026-08-10-TASK-8.md`

The ignored SDD ledger remains at:

`/Users/fengyujie/.config/superpowers/worktrees/multica/native-design-engine/.superpowers/sdd/2026-08-06-multica-native-design-system-phase-1/`

Its current state has been copied into this tracked handoff so the next session does not depend on ignored files.

## 4. Overall Progress

| Task | Result |
| --- | --- |
| 1. V2 package and server Audit boundary | Complete |
| 2. Multica-owned browser preview verification | Complete |
| 3. Migration 134, immutable V2 ZIP upload, fixed-byte retry | Complete |
| 4. Native Agent design workspace | Complete |
| 5. Daemon collect, audit, preview, upload, finalize gate | Complete |
| 6. Server re-verification and atomic persistence gate | Complete |
| 7. Route new tasks to native execution while preserving historical lifecycle | Complete |
| 8. Real CRM workflow and evidence | In progress, safe stop |

Task 1-7 completion range ends at `209cebb2c`. Task 8 code and security fixes are committed through `7ccf8c6ec`; `991ad72c8` is the pre-handoff state checkpoint.

## 5. Task 8 Work Completed

### 5.1 Repository-analysis contract

The following commits implemented the missing live repository-analysis lifecycle:

| Commit | Purpose |
| --- | --- |
| `1a51a385a` | expose repository resources to analysis |
| `87d999848` | scope analysis resources to daemon |
| `e81ed5df4` | complete repository-analysis tasks |
| `4d5d3afae` | harden analysis completion |
| `10acc4aef` | expose repository-analysis API to the frontend client |
| `2ab565b2e` | route analysis to the native read-only workspace |
| `8a300be70` | skip legacy package collection and clean analysis sidecars |

Observed failures that drove these commits:

- Task `33f02565-67f0-4f54-bf6c-073bdd23920b` failed with `unsupported operation "repository_analysis"`.
- Task `b2915f88-5b30-4c77-9b11-b514367a129a`, runtime `10289085-9452-4667-a5c2-8c49358c8b6b`, session `019fea86-c998-7bd0-9b70-ed0390045b0d`, successfully read the real CRM repository but the old daemon then required `DESIGN.md` and failed with `project_design_system_artifacts_invalid`.
- Neither task created an `open_design_run`.

### 5.2 Sidecar security hardening

Independent Spec review passed for `8a300be70`. Independent Quality review found one Critical and two Important cleanup defects plus one Minor discriminator gap:

- manifest paths could traverse an Agent-planted intermediate symlink and delete an external same-name file;
- ordinary or pre-existing V2 directories could be chmodded without manifest ownership;
- partial cleanup could delete the retry manifest;
- `repository_analysis` did not also require `type == "project_design_system_task"`.

Fixes:

| Commit | Purpose |
| --- | --- |
| `de2c8033b` | no-follow local cleanup, manifest-owned chmod, failure-retained manifest, strict task discriminator |
| `7ccf8c6ec` | extend no-follow deletion to cloud Reuse and managed skill cleanup |

Verification state is deliberately incomplete:

- After `de2c8033b`, controller reran full `internal/daemon/execenv` and `internal/daemon`; both passed.
- Scoped Quality re-review closed three findings but kept the cloud Reuse symlink path open.
- `7ccf8c6ec` added the cloud Reuse fix. Its implementer reported two RED escape tests, then five focused GREEN tests, `git diff --check`, and LOW staged detect-changes.
- `7ccf8c6ec` has not received the required scoped Quality re-review.
- Full `internal/daemon/execenv` plus `internal/daemon` has not been rerun after `7ccf8c6ec`.

Do not describe the security review as closed until both remaining gates pass.

## 6. Current Live State

### 6.1 Isolated worktree services

Observed at handoff:

- backend: `http://localhost:18460`, PID `13511`, binary `/tmp/multica-task8-native-server`;
- frontend: `http://localhost:13380`, PID `93768`;
- isolated Task 8 daemon health port `19698`: stopped;
- main-checkout daemon PID `33869`: still running and must not be stopped.

Chrome was previously bound to:

`http://localhost:13380/native-phase1-crm/designs`

The prior Node/browser session used persistent variables `chrome` and `task8Tab`. A new session must rediscover or rebind them through the Chrome plugin; do not assume they survive.

### 6.2 Database

- PostgreSQL container: `multica-postgres-1`, host port `5433`.
- Database: `multica_native_design_engine_380`.
- Project design system: `569aa0c2-d218-41fd-bf92-88b32ed06f8a`.
- Current active task: `1d276b49-e199-4506-a505-398f0e206e2a`.
- Active operation: `repository_analysis`.
- DB task status: `running`.
- Session: `019feaa0-ccdb-78c3-934a-90736f868ef2`.
- Current `open_design_run` count: `0`.

The task is an orphaned live-at-DB row. The isolated daemon was stopped while the Agent was running; the Agent process was cancelled, but the failure callback inherited the cancelled context and did not update the DB. The rebuilt daemon must converge this task through the existing orphan recovery path before any retry.

### 6.3 CRM fixtures

- Workspace: `b78a816f-d1bb-4838-b702-813bef485d45`.
- Project: `14af6d72-602a-4c39-b1be-1ac7f8de663e`.
- Design system: `569aa0c2-d218-41fd-bf92-88b32ed06f8a`.
- Agent: `c806469f-6d4d-4b29-ae58-41f4a3778922` (`Local UI Restore Agent`).
- Runtime: `10289085-9452-4667-a5c2-8c49358c8b6b`.
- Source repository: `/Users/fengyujie/Documents/soyoung/prime/staffrnapp`.

Source repository state at handoff:

```text
 M .vscode/settings.json
```

This is the user's pre-existing change. Its SHA256 remains:

`d65f8b33315baee2afcd8dc049d2ede4b249ada305bebc1739cc12decd15764a`

There is no `.agent_context`, managed `AGENTS.md`, or `.multica` residue at the safe stop.

## 7. Exact Resume Gates

Perform these in order. Do not skip directly to Chrome or generation.

### Gate A: close the security review

1. Confirm branch, HEAD, and clean status.
2. Generate or read a review package for `de2c8033b..7ccf8c6ec`.
3. Resume the prior Quality reviewer only if its context is still available; otherwise use a fresh high-reasoning reviewer.
4. Review only the cloud Reuse no-follow fix and prior Critical scenario.
5. If any Critical or Important remains, return to the same implementer for a scoped fix and re-review.
6. Run fresh controller verification:

```bash
cd /Users/fengyujie/.config/superpowers/worktrees/multica/native-design-engine/server
go test -buildvcs=false ./internal/daemon/execenv ./internal/daemon -count=1
```

### Gate B: rebuild and recover without touching other services

1. Build a new CLI binary from the reviewed HEAD.
2. Start only the Task 8 daemon with profile `task8-native` and health port `19698`.
3. Do not restart backend `18460`, frontend `13380`, or PID `33869`.
4. Confirm the orphan task `1d276b49-e199-4506-a505-398f0e206e2a` leaves `running` and the design system clears `active_task_id` and `active_operation`.
5. Confirm source status and `.vscode/settings.json` hash remain unchanged.
6. Confirm `open_design_run` remains `0`.

Suggested build/start shape:

```bash
cd /Users/fengyujie/.config/superpowers/worktrees/multica/native-design-engine/server
go build -buildvcs=false -o /private/tmp/multica-task8-native-cli.next ./cmd/multica
mv /private/tmp/multica-task8-native-cli.next /private/tmp/multica-task8-native-cli

cd /Users/fengyujie/.config/superpowers/worktrees/multica/native-design-engine
/private/tmp/multica-task8-native-cli --profile task8-native daemon start --foreground
```

Redirect daemon output to `/private/tmp/multica-task8-native-daemon.log` to avoid returning large logs to the model context.

### Gate C: obtain the user's token-scope choice

Do not start another repository analysis until the user explicitly chooses section 9. The user requested a safe stop because a strict live matrix may consume substantial model tokens.

## 8. Remaining Strict Task 8 Work

If the user chooses strict completion:

1. Rerun repository analysis and wait for a successful normalized repository context.
2. Record task ID, session, runtime, source paths, input snapshot digest, DB state, source status, and `open_design_run=0`.
3. Generate one real V2 design system.
4. Correlate archive object key, artifact index, content digest, manifest binding, `source/index.json`, Audit receipt, Chrome receipt, and draft row.
5. Inspect the result in the user's Chrome: UI Kit and representative CRM source pages side by side; capture screenshots, Console, Network, CSP, external-request, overflow, blank iframe, and image checks.
6. Run one scoped adjustment and verify changed input/base digests and complete package replacement.
7. Save atomically, refresh, and prove the saved digest is stable.
8. Inject one deterministic invalid package task and prove draft/saved bytes remain unchanged.
9. Create a later valid draft, discard it, and prove the saved system returns.
10. Reconfirm no source residue and no `open_design_run`.
11. Update:
   - `docs/product/design-center/project-design-system-validation.md`
   - `docs/product/design-center/README.md`
   - `docs/product/design-center/decision-register.md`
12. Run final Task 8 Spec and Quality reviews, then a full-branch final review.
13. Run final verification and GitNexus staged detect, commit the evidence, and update `AGENTS.md`.

Task completion alone is not acceptance.

## 9. Token Scope Decision

The safe-stop user has not selected a continuation mode.

### Low-token evidence path

- One repository-analysis run and one generation run.
- Estimated additional model usage: roughly 150k-400k tokens, highly variable.
- Use deterministic tests for remaining lifecycle behavior.
- Record adjustment/save/discard live rows as incomplete.
- Do not claim full Task 8 or Phase 1 live acceptance.

### Strict-complete path

- Repository analysis, generation, adjustment/save, invalid isolation, and later valid draft/discard.
- Estimated additional model usage: roughly 300k-1M tokens, highly variable.
- Required to close every row in the current Task 8 acceptance matrix.

Do not present either estimate as billing data. The cancelled task did not persist final usage, so the estimate is based on observed run/tool volume rather than an authoritative usage record.

## 10. Known Baseline Failures

Keep baseline failures separate from branch regressions:

- `TestListDesignFilesHidesManagedAssetSources` is the known `t6-b1` baseline failure.
- Historical Open Design compatibility tests can fail under the no-Open-Design-runtime environment; do not delete or rewrite historical compatibility to hide failures.
- Earlier full `@multica/views` and `pnpm typecheck` runs exposed existing core/views contract drift. Re-run current focused matrices and attribute only fresh failures introduced by the current diff.

## 11. Completion Definition

Task 8 and Phase 1 may be marked complete only when all strict acceptance rows have observed evidence: no Worker dependency, real Agent input, V2 integrity, static quality, real visual output, CRM grounding, draft isolation, save/discard, adjustment, and historical compatibility.

After all rows pass:

- perform a final full-branch Spec and Quality review;
- triage deferred Task 4-7 minors recorded in the SDD ledger;
- use `superpowers:finishing-a-development-branch`;
- ask the user to choose merge, PR, keep branch, or cleanup;
- do not merge automatically.
