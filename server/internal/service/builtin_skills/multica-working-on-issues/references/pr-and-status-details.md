# PR and status details

Load this reference when PR link placement, linked-PR state, progress status, or
phased sub-issue behavior affects the next action. Source locations live in
`working-on-issues-source-map.md`.

## PR linking and close intent are distinct

The GitHub webhook performs two scans over an incoming PR. They use different
fields and set different state.

**Linking** scans the PR **title, body, OR branch** for a routable issue key
(`PREFIX-NUMBER`, for example `MUL-2759`). Each match writes an issue-to-PR link.

```text
MUL-2759: add built-in issue working skill        # title: linked and shown
agent/matt/mul-2759-working-on-issues             # branch: linked and shown
```

**Close intent** scans the **title or body only — never the branch**. It requires
the key immediately after `Closes`, `Fixes`, or `Resolves` (an optional colon may
precede the whitespace). The adjacency sets close intent, which can advance the
issue to `done` when the PR merges.

```text
Closes MUL-2759                    # links and records close intent
Fixes: MUL-2759                    # links and records close intent
Fix login MUL-2759                 # links only; keyword is not adjacent
```

A title prefix or branch key links without close intent. Put an adjacent closing
keyword in the title or body only when merge should close the issue.

### Reference-only body links

A key found only as a bare body mention — absent from title and branch and not
after a closing keyword — still creates a link row. The row is marked
`reference_only` and excluded from `multica issue pull-requests` and the UI PR
list. Passing mentions therefore do not look like active work or affect the
visible auto-advance gate.

```text
Closes MUL-2759 in the body                        # linked and shown
Related to MUL-2759 in the body (no title/branch)  # linked but hidden
```

To show the PR, put the key in the title or branch, or after a closing keyword
in the body. If a newly opened PR is absent from the list, either no routable key
was observed or its only match was a reference-only body mention.

## Linked PR response and current snapshot

Read Multica's link table rather than branch names, GitHub search, memory, or
stale `pr_url` metadata:

```bash
multica issue pull-requests <issue-id> --output json
```

The response is `{"pull_requests": [...]}`. Each element includes:

- `number`, `html_url`, `title`, and `provider` (`github`, `forgejo`, `gitea`,
  or `gitlab`).
- `state`: one of `merged`, `closed`, `draft`, or `open`. There is no separate `draft` or `merged` boolean.
  The fold order is merged, closed, draft, open.
- `merged_at`: non-null after merge and a second confirmation of merged state.
- `mergeable_state`: GitHub's value; `clean` and `dirty` are surfaced while
  other values round-trip as unknown for compatibility.
- GitHub snapshot fields: `snapshot_available`, `mergeable`,
  `merge_state_status`, `checks_rollup`, `checks_total`, `checks_passed`,
  `checks_failed`, `checks_running`, `failed_check_names`,
  `snapshot_fetched_at`, and `snapshot_stale`.
- `checks_conclusion`: coarse CI state `passed`, `failed`, `pending`, or `null`.
  GitHub derives it from the current API snapshot; Forgejo, Gitea, and GitLab
  derive it from webhook commit statuses and provider-appropriate check counts.

`snapshot_available == true` means the GitHub snapshot feature is enabled and
the snapshot matches the PR's current head. Only then does `checks_rollup == null` mean "no checks".
When false, the feature may be disabled, not fetched yet, or holding only an old
head. Use `state == "merged"` (or non-null `merged_at`) for merge, `state ==
"draft"` for draft, and `checks_conclusion` for coarse CI status.

## Status categories and progress writes

Status updates resolve custom categories, but create-time admission only fences
the built-in terminal keys: a custom terminal status can still enqueue at creation.
Failed-task rollback writes literal `todo`; a merged close-intent PR writes literal `done`.

- `backlog` parks an agent-assigned issue. Moving it to `todo`, or another
  non-done/non-cancelled category, enqueues the assignee.
- When the runtime brief supplies `MULTICA_ISSUE_OUTCOME_FILE`, follow its managed
  completion contract for final status and comment delivery; do not duplicate
  the final response with a CLI comment. Explicit status commands remain available.
- Outside that contract, `in_progress` / `in_review` are agent-managed CLI mutations, not `StartTask` / `CompleteTask` side effects.
  Write status whenever work changes the issue's
  real state, including mid-turn; do not derive it from trigger type, run
  lifecycle, or whether the agent is the assignee.
- A turn that advances the issue's own ask writes `in_progress` as soon as that
  is known. A turn that delivered the issue's own ask means `in_review`; work continuing
  beyond the turn, including partial delivery, means `in_progress`; stuck means
  `blocked`. Record a blocker when it is hit and do not exit with stale status.
- A turn that produces none of the issue's own deliverable — answering a
  question or consulting on work owned elsewhere — writes nothing. Activity
  kind does not decide this: research, design, planning, and review count when
  they are the ask. Questions, discussion, and acknowledgements do not.
- For squad leaders, dispatching members is not delivery. Dispatch leaves the
  parent `in_progress`; only a later re-trigger that confirms the overall goal
  is met moves it to `in_review`.
- `in_review` is an accepted explicit status, including while a PR awaits review.
- Child `done` posts a system comment on the parent. A merged PR with close
  intent advances its issue to `done`; do not also flip it manually.
- `cancelled` is terminal and enqueues no work, but does **not** stop tasks already in flight.
  Cancel the task itself to stop a running task.
- A failed issue-triggered task may roll `in_progress` back to `todo` only when
  no active task or retry remains.

## Serial and staged sub-issues

On an agent-assigned issue, `todo` starts work now, `backlog` parks it. A
runnable built-in create status enqueues immediately; `backlog` records the assignee without
triggering. Creating every serial step as `todo` starts the whole chain at once.

```bash
# Parallel work starts now.
multica issue create --title "..." --parent <issue-id> --assignee <agent> --status todo

# Serial work stays parked until its dependency is done.
multica issue create --title "Step 2" --parent <issue-id> --assignee <agent> --status backlog
multica issue status <child-id> todo
```

`--stage <N>` (N >= 1) groups siblings into ordered barriers. The parent wakes
once, when a whole stage finishes: every child in the lowest unfinished stage is
`done` or `cancelled`. A completion that does not close the barrier is silent.
An unstaged sibling set is one implicit stage, so only its last completion wakes
the parent.

Advancement is agent-driven. The server reports the closed barrier and wakes the
parent assignee; that agent decides whether dependencies permit promotion of the
next stage's `backlog` children.

```bash
multica issue create --title "Research A" --parent <id> --assignee <agent> --stage 1 --status todo
multica issue create --title "Research B" --parent <id> --assignee <agent> --stage 1 --status todo
multica issue create --title "Build" --parent <id> --assignee <agent> --stage 2 --status backlog
multica issue create --title "Ship" --parent <id> --assignee <agent> --stage 3 --status backlog
multica issue children <parent-id>
multica issue status <stage-2-child-id> todo
```

`issue children --output json` reports per-stage `done` counts. A custom status
counts as terminal when its `status_category` is `done` or `cancelled`; read that
field rather than matching the custom `status` key to built-in names.

Read every child description before promotion. Promote only work whose stated
dependencies are met; if a description conflicts with the parent breakdown,
leave it in `backlog` and comment to confirm first.

## Incorrect -> correct

```text
Fix login redirect                  # incorrect: no issue key, so it will not link
MUL-2759: fix login redirect        # correct: links the PR
```

```bash
# Incorrect: every serial step fires immediately, with no ordering.
multica issue create --title "Step 2" --parent <issue-id> --assignee <agent> --status todo
multica issue create --title "Step 3" --parent <issue-id> --assignee <agent> --status todo

# Correct: Stage 1 runs; later stages park until each barrier closes.
multica issue create --title "Step 1" --parent <issue-id> --assignee <agent> --stage 1 --status todo
multica issue create --title "Step 2" --parent <issue-id> --assignee <agent> --stage 2 --status backlog
multica issue create --title "Step 3" --parent <issue-id> --assignee <agent> --stage 3 --status backlog
```
