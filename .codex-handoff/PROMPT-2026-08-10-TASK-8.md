# Continuation Prompt - Multica Native Design Engine Phase 1 Task 8

You are the controller Agent continuing Multica Native Design Engine Phase 1 through strict subagent-driven development.

## Objective

Finish Task 8, "Verify The Real CRM Workflow And Record Evidence", without expanding beyond the project design-system Phase 1 vertical slice.

Multica must use its native Project, Agent, daemon, task queue, object storage, Audit, preview, draft, save, and discard lifecycle. Open Design is a behavioral reference only. Never run, distribute, or host an Open Design Worker, Daemon, or Runtime.

## Read First

1. `/Users/fengyujie/.config/superpowers/worktrees/multica/native-design-engine/AGENTS.md`
2. `/Users/fengyujie/Documents/soyoung/multica/DEV-WORKFLOW-SOP.md` read-only
3. `/Users/fengyujie/.config/superpowers/worktrees/multica/native-design-engine/CLAUDE.md`
4. `/Users/fengyujie/.config/superpowers/worktrees/multica/native-design-engine/docs/product/design-center/README.md`
5. `/Users/fengyujie/.config/superpowers/worktrees/multica/native-design-engine/docs/superpowers/plans/2026-08-06-multica-native-design-system-phase-1.md`, Task 8 only
6. `/Users/fengyujie/.config/superpowers/worktrees/multica/native-design-engine/.codex-handoff/HANDOFF-2026-08-10-TASK-8.md`

Do not rely on conversation summaries over these files.

## Repository Boundary

- The only writable checkout is `/Users/fengyujie/.config/superpowers/worktrees/multica/native-design-engine`.
- Branch: `codex/multica-native-design-engine`.
- Expected history contains code checkpoint `991ad72c8`; start from the handoff-document commit or a user-created descendant.
- The main checkout `/Users/fengyujie/Documents/soyoung/multica` on `feature/fengchen` has user work. Never edit, clean, reset, or stop its services.
- Never merge `codex/open-design-native-slots`.
- Preserve unrelated dirty files everywhere.

## Execution Discipline

- Use strict SDD: fresh implementer, independent Spec review, independent Quality review, fix loop, scoped re-review.
- Give every subagent a narrow read set, write set, and completion commands.
- Use `high` reasoning for subagents; do not use ultra.
- Do not scan the whole repository or read files over 200 lines in full.
- Before editing any function, run GitNexus upstream impact. Report HIGH or CRITICAL blast radius before editing.
- Before every commit, stage exact files, run `git diff --check`, then `node .gitnexus/run.cjs detect-changes --scope staged --repo multica`.
- Keep command output bounded. Redirect daemon/test logs to `/private/tmp` and return only exit codes and key lines.
- Do not claim completion without fresh controller verification.
- Stop after Task 8; do not autonomously start another phase.

## Current State

Tasks 1-7 are complete. Task 8 is in progress at a committed safe stop.

Relevant pre-handoff commits:

```text
991ad72c8 docs: checkpoint native design validation
7ccf8c6ec fix(design): prevent reused sidecar symlink escapes
de2c8033b fix(design): harden native sidecar cleanup
8a300be70 fix(design): complete repository analysis lifecycle
2ab565b2e fix(design): route repository analysis to native workspace
10acc4aef fix(design): expose repository analysis API
```

`8a300be70` passed Spec review. Quality review found a symlink traversal, unowned chmod, lost retry manifest, and weak discriminator. `de2c8033b` fixed the local path and passed full daemon plus execenv tests. Scoped re-review then found the same symlink class remained in cloud Reuse. `7ccf8c6ec` fixes that path and has five focused GREEN tests, but it has not received scoped Quality re-review and has not had the full two-package controller rerun.

Do not call the review complete yet.

## Current Services and Data

At handoff:

- backend `http://localhost:18460`, PID `13511`, still running;
- frontend `http://localhost:13380`, PID `93768`, still running;
- isolated Task 8 daemon port `19698`, stopped;
- main-checkout daemon PID `33869`, running and forbidden to stop;
- PostgreSQL container `multica-postgres-1`, host port `5433`;
- DB `multica_native_design_engine_380`.

CRM IDs:

```text
workspace:     b78a816f-d1bb-4838-b702-813bef485d45
project:       14af6d72-602a-4c39-b1be-1ac7f8de663e
design system: 569aa0c2-d218-41fd-bf92-88b32ed06f8a
agent:         c806469f-6d4d-4b29-ae58-41f4a3778922
runtime:       10289085-9452-4667-a5c2-8c49358c8b6b
source:        /Users/fengyujie/Documents/soyoung/prime/staffrnapp
```

The DB still shows orphan task `1d276b49-e199-4506-a505-398f0e206e2a` as running `repository_analysis`, session `019feaa0-ccdb-78c3-934a-90736f868ef2`. The Agent process was cancelled during daemon shutdown, but the failure callback inherited cancelled context and did not persist. Current `open_design_run` count is `0`.

The staffrnapp repository must contain only its pre-existing user change:

```text
 M .vscode/settings.json
SHA256 d65f8b33315baee2afcd8dc049d2ede4b249ada305bebc1739cc12decd15764a
```

No `.agent_context`, generated `AGENTS.md`, or `.multica` residue is allowed at rest.

## First Actions

1. Run `git status --short --branch` and `git log -8 --oneline` in the isolated worktree.
2. Recheck listeners for `18460`, `13380`, and `19698`; do not restart anything yet.
3. Recheck the orphan task, design-system active fields, `open_design_run` count, staffrnapp status, and `.vscode/settings.json` hash.
4. Generate a review package for `de2c8033b..7ccf8c6ec`.
5. Run a scoped Quality re-review focused only on:
   - cloud `Reuse` passing the real WorkDir into no-follow cleanup;
   - managed skill cleanup not following symlinked parents or owned entries;
   - external sentinel files surviving;
   - prior local-directory protections remaining intact.
6. Fix any Critical or Important finding with the same implementer and rerun scoped review.
7. Run fresh controller verification:

```bash
cd /Users/fengyujie/.config/superpowers/worktrees/multica/native-design-engine/server
go test -buildvcs=false ./internal/daemon/execenv ./internal/daemon -count=1
```

8. Only after review and tests pass, rebuild `/private/tmp/multica-task8-native-cli` from current HEAD.
9. Start only the isolated daemon using profile `task8-native`; redirect output to `/private/tmp/multica-task8-native-daemon.log`.
10. Prove orphan recovery converges task `1d276b49-e199-4506-a505-398f0e206e2a`, clears design-system active fields, keeps `open_design_run=0`, and leaves staffrnapp unchanged.

## Mandatory Token Decision

Before launching another repository-analysis or generation Agent, stop and ask the user to choose:

1. Low-token evidence: one analysis plus one generation, estimated 150k-400k model tokens; deterministic tests cover the rest, and Task 8 remains explicitly incomplete.
2. Strict completion: run the full analysis, generation, adjustment/save, invalid isolation, and later-draft discard matrix, estimated 300k-1M model tokens; only this path can close every Task 8 live-acceptance row.

These are rough execution estimates, not billing data. Do not start an expensive run until the user chooses.

## Strict Completion Sequence

If the user chooses strict completion:

1. Run one real repository analysis and record task/session/runtime/source/digest evidence.
2. Generate one real V2 design system and correlate archive, binding, artifact index, content digest, source index, Audit, Chrome receipt, draft DB row, and zero Open Design runs.
3. Use the user's Chrome at `http://localhost:13380/native-phase1-crm/designs`; verify UI Kit and representative CRM pages side by side, Console, Network, CSP, no external requests, no blank iframe, no overflow, no broken images, and no template residue.
4. Run one scoped adjustment; verify changed input/base digests and complete replacement package.
5. Save, refresh, and prove a stable saved digest.
6. Inject one deterministic invalid package task and prove draft/saved bytes do not change.
7. Create a later valid draft, discard it, and prove the saved package returns.
8. Reconfirm staffrnapp exact restoration and `open_design_run=0`.
9. Update the three authoritative design-center documents only with observed facts.
10. Run Task 8 Spec and Quality reviews, final full-branch review, fresh tests, staged GitNexus detection, and commit.
11. Update `AGENTS.md` three lines.
12. Use `superpowers:finishing-a-development-branch` and ask the user for merge/PR/keep/cleanup. Never merge automatically.

## Acceptance Rule

Task completion, a persisted draft, or passing unit tests alone are not acceptance. Task 8 is complete only when every final acceptance row has observed evidence: no Worker dependency, real Agent input, V2 package integrity, static Audit, real browser output, CRM grounding, draft isolation, save/discard, adjustment, and historical compatibility.

When reporting status, lead with the exact gate reached, the active task/commit, what remains, and whether another model-consuming CRM run has started.
