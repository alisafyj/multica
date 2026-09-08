# working-on-issues source map

Evidence layer for `SKILL.md`. Every contract the skill states is traced to a
current `file:line` here. Lines were re-derived against the current checkout;
the prior skill cited lines that have since moved (see the "drifted" column).
Re-confirm with the verification command at the bottom before relying on an
exact line.

## `multica issue pull-requests` — read PR links from Multica

| Behavior | File:line | Drifted from |
|---|---|---|
| CLI command `pull-requests <id>` (alias `prs`) | `server/cmd/multica/cmd_issue.go:184-190` | `:104` |
| `runIssuePullRequests` handler | `server/cmd/multica/cmd_issue.go:812` | new citation |
| Calls `GET /api/issues/<id>/pull-requests` | `server/cmd/multica/cmd_issue.go:827` | `:522` |
| API route registration | `server/cmd/server/router.go:2024` | `:480` |
| Handler `ListPullRequestsForIssue` → `Queries.ListPullRequestsByIssue` | `server/internal/handler/github.go:963-968` | `:466` |
| Row → response mapper `issuePullRequestRowToResponse` | `server/internal/handler/github.go:228` | `:149` |

The CLI resolves the issue ref, GETs the endpoint, and (for `--output json`)
prints the raw `{"pull_requests": [...]}` body. Only `--output` is accepted; the
default `table` shows `NUMBER STATE TITLE URL`.

## PR response shape

`GitHubPullRequestResponse` struct: `server/internal/handler/github.go:64`. JSON
fields the agent can read off each element of `pull_requests`:

- `provider` (`json:"provider"`, line 69)
- `number` (`json:"number"`, line 73)
- `html_url` (`json:"html_url"`, line 76)
- `title` (`json:"title"`, line 74)
- `state` (`json:"state"`, line 75) — the folded lifecycle enum (see below)
- `merged_at` (`json:"merged_at"`, line 80), `closed_at` (line 81)
- `mergeable_state` (`json:"mergeable_state"`, line 86) — mirrors GitHub; UI only
  surfaces `clean`/`dirty`, other values round-trip as unknown
- `snapshot_available` (`json:"snapshot_available"`, line 106) — for GitHub,
  true only when the App snapshot feature is enabled and the snapshot head
  matches the current PR head (`currentGitHubSnapshotAvailable`, lines 281-287)
- `mergeable` / `merge_state_status` (lines 96, 100) — conflict-only verdict vs
  the complete merge gate; "ready" requires `merge_state_status == "clean"`
- `checks_rollup` (`json:"checks_rollup"`, line 111) and run-level
  `checks_total` / `checks_passed` / `checks_failed` / `checks_running`
  (lines 117-120), plus `failed_check_names` (line 124)
- `checks_conclusion` (`json:"checks_conclusion"`, line 114) — coarse
  `"passed"`/`"failed"`/`"pending"` or `null`; GitHub derives it only from an
  available current-head snapshot (mapper lines 265-277), while self-hosted VCS
  providers use `aggregateChecksConclusion` (line 298)

There is **no** standalone `draft` or `merged` boolean in the response. The
PR lifecycle is encoded in the single `state` string by `derivePRState`
(`server/internal/handler/github.go:1746`):

```
merged   → if PullRequest.Merged
closed   → else if PullRequest.State == "closed"
draft    → else if PullRequest.Draft
open     → otherwise
```

`derivePRState` is called when the webhook upserts the row
(`server/internal/handler/github.go:1535`), so `state` is what the list endpoint
returns. "Is it merged?" = `state == "merged"` (or `merged_at != null`); "is it a
draft?" = `state == "draft"`. Combine with `checks_conclusion` for CI status.

## Two distinct webhook paths: link vs close-intent

Both run inside the `pull_request` webhook handler, gated by the workspace
auto-link flag (`workspaceAutoLinkPRsEnabled`, `github.go:1579`).

### Path 1 — link (title OR body OR branch)

- `extractIdentifiers` regex helper: `server/internal/handler/github.go:1780`
- driving regex `identifierRe` (`\b([a-z][a-z0-9]{0,9})-(\d+)\b`, case-insensitive):
  `server/internal/handler/github.go:1036`
- call site: `server/internal/handler/github.go:1580` —
  `extractIdentifiers(p.PullRequest.Title, p.PullRequest.Body, p.PullRequest.Head.Ref)`

Every `PREFIX-NUMBER` mention in **title, body, or branch** resolves to an issue
in the workspace and writes a link row (`LinkIssueToPullRequest`, `github.go:1640`).
This is what `multica issue pull-requests` later reads back.

**Reference-only flag (MUL-3739).** The link row carries a `reference_only`
boolean (`migrations/127_issue_pull_request_reference_only.up.sql`). The handler
computes a `qualifyingIdents` set = identifiers in **title or branch** (any
`extractIdentifiers` match) ∪ **body closing keywords** (`closingIdents`). A
linked identifier NOT in that set was matched only by a bare body mention, so its
row is written with `reference_only = true`. Both `ListPullRequestsByIssue` and
`GetIssuePullRequestCloseAggregate` filter `AND NOT reference_only`, so
reference-only links are hidden from the CLI / UI PR list **and** excluded from
the auto-advance gate (an open body-only mention must not silently block the
issue from reaching `done` while invisible in the list). The row still exists for
edit-time close-intent tracking. `reference_only` follows the same
`preserve_close_intent` terminal gate as `close_intent`.

Drifted from the prior skill's `github.go:727` citation, which pointed at the old
call-site location for the link logic.

### Path 2 — close intent (title OR body only, keyword-adjacent)

- `extractClosingIdentifiers` regex helper: `server/internal/handler/github.go:1803`
- driving regex `closingIdentifierRe`
  (`\b(?:close[sd]?|fix(?:e[sd])?|resolve[sd]?)[:\s]+([a-z][a-z0-9]{0,9})-(\d+)\b`):
  `server/internal/handler/github.go:1047`
- call site: `server/internal/handler/github.go:1589` —
  `extractClosingIdentifiers(p.PullRequest.Title, p.PullRequest.Body)` (no branch arg)

Only a `PREFIX-NUMBER` immediately after a closing keyword
(`Closes`/`Fixes`/`Resolves`, optional `:` then whitespace) sets the link row's
`close_intent` flag — the gate that auto-advances the issue to `done` on merge.
`Fix MUL-1` closes; `Fix login MUL-1` does not (adjacency). Branch names are
deliberately excluded (function doc, `github.go:1796-1802`): a branch like
`mul-1/fix-login` links but must never declare close intent.

Drifted from the prior skill's `github.go:736` citation.

Net: a bare title prefix (`MUL-2759: ...`) or a branch ref links only (shown in
the PR list); `Closes MUL-2759` links **and** records close intent; a bare body
mention with no title/branch ref and no closing keyword links as `reference_only`
and is hidden from the PR list.

## Status side effects (enqueue contracts)

| Behavior | File:line | Drifted from |
|---|---|---|
| Create-time: ready agent-assigned issue in a runnable project may enqueue; SQL blocks literal backlog/done/cancelled, not custom terminal keys | `server/internal/service/task.go:1183` (`EnqueueTaskForIssueCreateWithMode`); `server/migrations/892_runnable_issue_task_fence.up.sql:75`, `:86`, `:108`; invocation in `server/pkg/db/queries/agent.sql:363` | corrects predictor-only citation |
| `WillEnqueueRun` returns false for `backlog` (parking lot) | `server/internal/service/issue_trigger.go:125-129` | new citation |
| Backlog → non-backlog (not done/cancelled) enqueues on update | `server/internal/service/issue_trigger.go:131-139` | `:2523` |
| Same contract in batch update | `server/internal/handler/issue.go:4589-4598` | new citation |
| Child → terminal notifies + wakes the parent, gated by the stage barrier | `server/internal/handler/issue_child_done.go:70` (`notifyParentOfChildDone`; doc comment at `:18`; barrier gate at `:139`) | func def `:51` |
| Status change (incl. → `cancelled`) does NOT cancel in-flight tasks; only issue deletion does (MUL-4465) | no-cancel note in `server/internal/handler/issue.go:3837-3848` (`UpdateIssue`) and `:4601-4602` (`BatchUpdateIssues`); deletion still cancels at `:4083` (`DeleteIssue`) / `:4698` (`BatchDeleteIssues`) via `CancelTasksForIssue` (`server/internal/service/task.go:2909`) | new citation |
| Legacy runs use explicit CLI progress writes; completion-capable runs return their final answer normally and use the runtime brief's typed outcome for guarded final status | `server/internal/daemon/task_snapshot_prompt.go:97` (`buildIssueCompletionDeliveryPrompt`); `server/internal/service/issue_completion.go:50` (`applyIssueCompletion`) | distinguishes managed completion |
| Runtime brief: status written whenever the work changes it, mid-turn included — starting the issue's own ask → `in_progress` immediately (workflow step 3); delivery → `in_review`, continuing → `in_progress`, stuck → `blocked`; a turn producing none of the issue's own deliverable → no write at any point; the activity kind never decides (research/design/planning/review count as work when they are the ask); no assignee gate; squad leader dispatch is not delivery (MUL-6417) | `server/internal/daemon/execenv/runtime_config_sections.go` (`writeWorkflowIssue`) | new citation |
| Failed task may roll `in_progress` → `todo` when no active task or retry remains | `server/internal/service/task.go` (`HandleFailedTasks`) | new citation |
| Updates resolve custom categories; create paths reject effective backlog but custom terminal creates can still enqueue | `server/internal/issuestatus/issuestatus.go` (`Effective`, `Resolve`); `server/internal/service/issue.go:950` and `:1033`; migration 892 literal-key fence above | records create-time exception |
| Runtime brief lists the workspace's active custom statuses grouped by category; catalog rides the claim payload (MUL-6460) | `server/internal/daemon/execenv/runtime_config_sections.go` (`writeIssueStatusCommand`); claim injection in `server/internal/handler/daemon.go` (`buildClaimedTaskResponse`, status catalog block) | new citation |
| Literal-key exceptions to category rules: failed-task rollback writes the `todo` key; merged close-intent PR writes the `done` key | `server/internal/service/task.go` (`HandleFailedTasks`); `server/internal/handler/github.go` (merge close-intent path) | new citation |

Creation with `--status todo` (or another runnable built-in status) on an agent-assigned
issue fires the agent immediately; `--status backlog` parks it with the assignee
set but no trigger. Promoting `backlog → todo` later fires it then (update path,
`WillEnqueueRun`, lines 125-139).

Moving an issue to `cancelled` used to call `CancelTasksForIssue` and stop every
active task on it (the old #940 behavior). MUL-4465 removed that from both
`UpdateIssue` and `BatchUpdateIssues`: a status flip — `cancelled` included —
never cancels tasks now. `CancelTasksForIssue` fires only from the issue-deletion
paths (`DeleteIssue` / `BatchDeleteIssues`), where the owning issue row is going
away, so no task is left orphaned.

## Ownership-only assignment and duplicate-run awareness

| Behavior | Source |
|---|---|
| `issue assign --no-start`, `issue update --no-start`, and `issue status --no-start` send `suppress_run=true` | `server/cmd/multica/cmd_issue.go` (`runIssueAssign`, `runIssueUpdate`, `runIssueStatus`) |
| Update and batch-update apply ownership while skipping dispatch when `suppress_run` is true | `server/internal/handler/issue.go` (`UpdateIssue`, `BatchUpdateIssues`) |
| Trusted direct self-assignment suppresses enqueue only when the target `(issue, agent)` already has a non-terminal task | `server/internal/service/issue_trigger.go` (`WillEnqueueRun`), `server/internal/handler/issue_trigger.go` (`shouldSuppressActiveSelfAssignment`) |
| Claim responses expose a bounded, workspace-scoped snapshot of the same agent's other dispatched/running/waiting issue tasks; queued tasks are excluded | `server/pkg/db/queries/agent.sql` (`ListActiveSiblingIssueTasks`), `server/internal/handler/daemon.go` (`buildClaimedTaskResponse`) |
| Daemon prompts point to the target's comment history and concrete sibling `run-messages` commands | `server/internal/daemon/prompt.go` (`buildActiveSiblingRunsBlock`) |
| `issue runs --active` / `--siblings` send `active=true` / `scope=family` to the task-runs endpoint | `server/cmd/multica/cmd_issue.go` (`runIssueRuns`) |
| `scope=family` roots at the issue's parent, or the issue itself when it has none, and returns in-flight runs on that root plus all of its children | `server/internal/handler/daemon.go` (`ListTasksByIssue`), `server/pkg/db/queries/agent.sql` (`ListActiveTasksByIssueFamily`) |
| Invalid `active` / `scope` values are rejected rather than silently answering with the full history | `server/internal/handler/daemon.go` (`ListTasksByIssue`) |
| The family cap fetches one row past `familyActiveRunCap` and reports truncation on `X-Active-Runs-Truncated`, which the CLI surfaces on stderr | `server/internal/handler/daemon.go` (`familyActiveRunCap`, `HeaderActiveRunsTruncated`), `server/cmd/multica/cmd_issue.go` (`runIssueRuns`) |
| The active path skips usage hydration, which is keyed by issue and spans the full history | `server/internal/handler/daemon.go` (`ListTasksByIssue`), `server/pkg/db/queries/task_usage.sql` (`ListIssueTaskUsage`) |
| The family read returns its own compact row rather than the execution-log record, and skips usage and attribution hydration | `server/internal/handler/daemon.go` (`ActiveRunSummary`), `server/pkg/db/queries/agent.sql` (`ListActiveTasksByIssueFamily`) |

The self-assignment guard is intentionally pair-scoped. It does not treat
"this agent is busy on some other issue" as a reason to suppress a fresh
cross-issue handoff, because serial sub-issue promotion and triage batches rely
on those assignments creating their normal queued runs.

## Sub-issue stages (barrier wake)

| Behavior | File:line |
|---|---|
| `issue.stage` column (nullable, `>= 1`) | `server/migrations/123_issue_stage.up.sql` |
| Stage barrier: notify+wake fire only when the lowest unfinished stage is all-terminal; unstaged set = one implicit stage | `server/internal/handler/issue_child_done.go:492` (`stageBarrierClosed`) |
| Per-stage summary + next stage for the wake comment | `server/internal/handler/issue_child_done.go:524` (`stageProgressSummary`) |
| `--stage` on `issue create` / `issue update` | `server/cmd/multica/cmd_issue.go:523,546` |
| `multica issue children <id>` (sub-issues grouped by stage) | `server/cmd/multica/cmd_issue.go:192-198,945`; stage `done` counting via `isTerminalChildIssue` at `:1045` (reads `status_category`, MUL-6243); route `server/cmd/server/router.go:2012` → `ListChildIssues` at `server/internal/handler/issue.go:2320` |

Advancement is agent-driven: the server only detects the closed barrier and
wakes the parent assignee. Promoting the next stage's `backlog` sub-issues to
`todo` is the woken agent's decision, not a server side effect. When the woken
assignee (often a squad leader) decides the parent is complete, the system
comment explicitly asks for `multica issue status <parent-id> in_review`. Any
turn may move the status on its own too, judged from what the work changes
about the issue — there is no assignee gate (MUL-6417).

## Metadata CLI

| Behavior | File:line |
|---|---|
| `multica issue metadata set <issue-id> --key --value [--type]` | `server/cmd/multica/cmd_issue_metadata.go:80-90,109-112` |
| `multica issue metadata delete <issue-id> --key` | `server/cmd/multica/cmd_issue_metadata.go:93-97,113-114` |
| API routes (PUT/DELETE `/metadata/{key}`) | `server/cmd/server/router.go:2020-2021` |

`--value` is JSON-parsed by default (bool/number sniff); `--type` forces
`string`/`number`/`bool`.

## Custom properties CLI

| Behavior | File:line |
|---|---|
| `multica property list/get/create/update/archive/unarchive` | `server/cmd/multica/cmd_property.go` |
| `multica issue property list/set/unset` (name→id translation) | `server/cmd/multica/cmd_property.go` (`encodeIssuePropertyValue`) |
| `issue list --property "Name=Value"` filter (OR within a definition, AND across; `__none__` unset sentinel; scalar values sent in their stored spelling) | `server/cmd/multica/cmd_property.go` (`buildPropertiesFilterQueryParam`, `resolvePropertyFilterValue`, `propertyNoValueSentinel`) |
| `issue list --sort property:<name-or-id>` (archived + orderless types rejected up front) | `server/cmd/multica/cmd_property.go` (`resolveSortableProperty`, `issueSortablePropertyTypes`); `server/cmd/multica/cmd_issue.go` (`runIssueList`) |
| Server `properties=` filter contract the CLI forwards to | `server/internal/handler/property.go` (`parsePropertiesFilterParam`, `noPropertyValue`) |
| Select sort follows option order (ordinal scales sort by meaning) | `server/internal/handler/property.go` (`propertySortExpr`, `selectPropertySortExpr`) |
| Definition CRUD, admin gate, agent-actor rejection | `server/internal/handler/property.go` (`requirePropertyAdmin`) |
| Optional catalog icon field and allowlist validation | `server/internal/handler/property.go` (`PropertyResponse`, `validatePropertyIcon`) |
| Per-type value validation (self-correcting errors) | `server/internal/handler/property.go` (`validatePropertyValue`) |
| `actor` / `multi_actor` reference parsing, `member` as the only kind, 20-value cap | `server/internal/handler/property.go` (`actorPropertyKinds`, `parseActorRef`, `parseActorRefList`, `maxPropertyActorValues`) |
| Actor references are checked for workspace membership only | `server/internal/handler/property.go` (`resolveActorRefs`) |
| `--value` name / email / id → `member:<uuid>` resolution (same member lookup as `--assignee`) | `server/cmd/multica/cmd_property.go` (`resolveActorPropertyRef`, `memberOnlyKinds`) |
| Shared actor-reference types and helpers | `packages/core/types/property.ts` (`parseActorRef`, `actorRefsFromValue`, `MAX_ISSUE_PROPERTY_ACTOR_VALUES`) |
| API routes (`/api/properties`, PUT/DELETE `/api/issues/{id}/properties/{propertyId}`) | `server/cmd/server/router.go` |

## Verification command

Re-derive any line above before depending on it:

```bash
cd server
rg -n 'pull-requests <id>|runIssuePullRequests|runIssueChildren|isTerminalChildIssue' cmd/multica/cmd_issue.go
rg -n 'ListPullRequestsForIssue|/pull-requests|/metadata/\{key\}|/children' cmd/server/router.go internal/handler/github.go
rg -n 'func issuePullRequestRowToResponse|type GitHubPullRequestResponse struct|func derivePRState|func extractIdentifiers|func extractClosingIdentifiers|closingIdentifierRe' internal/handler/github.go
rg -n 'extractIdentifiers\(|extractClosingIdentifiers\(|derivePRState\(|qualifyingIdents|reference_only|ReferenceOnly' internal/handler/github.go pkg/db/queries/github.sql
rg -n 'WillEnqueueRun|No status change|CancelTasksForIssue' internal/service/issue_trigger.go internal/handler/issue.go internal/service/task.go
rg -n 'func \(h \*Handler\) notifyParentOfChildDone|func stageBarrierClosed|func stageProgressSummary' internal/handler/issue_child_done.go
```
