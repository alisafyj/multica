---
name: multica-working-on-issues
description: "Use when acting on a Multica issue beyond what the brief covers: PR linking vs close intent, reading a linked PR's real state, metadata keys, status-change side effects, sub-issue todo vs backlog."
user-invocable: false
allowed-tools: Bash(multica *), Bash(git *), Bash(gh *)
---

# Working on Multica issues

Product contracts the runtime brief does not fully encode: PR linking vs close
intent, reading linked-PR state, metadata keys, status side effects, and
sub-issue enqueue behavior.

For building mention links, load `multica-mentioning` instead — not this skill.

Every contract below is traced to source in
`references/working-on-issues-source-map.md`.

## Default for code-changing issue work

When an issue run changes code in a checked-out GitHub repo, the default handoff
is to open or update a PR before posting the final Multica issue comment, unless
the user explicitly asked for a local-only change or no PR. This is a default, not
an unconditional command: if no code changed, say no PR is needed. If PR creation
is blocked by auth, failing tests, or missing remote state, report that blocker instead of pretending the run is complete.

Use a routable issue key in the PR title, body, or branch so the webhook can link
the PR back to the issue. If the PR should close the issue on merge, put the key
immediately after a closing keyword in the title or body, for example:

```text
MUL-2759: fix login redirect        # links only
Closes MUL-2759                     # links and records close intent
```

In the final issue comment, include the PR URL when a PR exists. If the task did
not produce a PR because no code changed or the user asked not to create one, say
that explicitly.

Linking and close intent are distinct: a routable issue key in the PR title,
body, or branch links it, while only an adjacent closing keyword in the title or
body records close intent. A bare body mention is reference-only and hidden from
the issue's PR list. Load `references/pr-and-status-details.md` before choosing
link placement beyond the examples above or diagnosing a missing link.

## Read a linked PR's real state

When a step depends on PR state, query Multica's link table — do not infer it
from branch names, GitHub search, memory, or `pr_url` metadata (which can be
stale).

```bash
multica issue pull-requests <issue-id> --output json
```

Use the returned `state` enum for lifecycle and `checks_conclusion` for coarse
CI state. Do not treat missing snapshot data as "no checks." Load
`references/pr-and-status-details.md` whenever a decision depends on draft,
mergeability, current-head snapshot availability, check counts, or a missing PR.

## Metadata: durable custom state

Metadata is a free-form KV bag of durable issue state. Reading metadata is safe.
Writing a metadata key is a state mutation and should be tied to an explicit
task requirement to record that state for later readers or runs. Keys are
whatever your workflow needs — the platform curates no vocabulary; pick short
snake_case names and reuse them consistently within your workspace.

Never store secrets, tokens, or API keys in metadata.
Not metadata: logs or summaries; runtime bookkeeping such as timestamps,
attempt counts, or agent IDs; or other single-run details such as
files touched and investigation notes — those belong in the result comment.

```bash
multica issue metadata set <issue-id> --key <key> --value <value>
multica issue metadata delete <issue-id> --key <stale-key>
```

`--value` is JSON-parsed by default (bool/number are sniffed); pass `--type
string|number|bool` to force a type.

## Custom properties: typed workflow state

Workspaces may define custom issue properties (Severity, Environment, QA
Status, Reviewer, ...). Properties are the typed, user-visible sibling of
metadata: values are validated against the definition (select options, date
format, http(s) URL, member reference), visible in the issue sidebar, and
addressed by name.

- Read what exists before writing: `multica property list` shows the catalog;
  `multica issue property list <issue-id>` shows values set on the issue.
- Set values by property name and option name — the CLI translates to ids:

```bash
multica issue property set <issue-id> --name Environment --value staging
multica issue property set <issue-id> --name Platforms --value "iOS,Android"
multica issue property set <issue-id> --name Reviewer --value Bohan
multica issue property unset <issue-id> --name Environment
```

- A validation error lists the legal options — fix the value and retry.
- `actor` / `multi_actor` properties (Reviewer, Escalation contact, ...) hold
  workspace members only. `--value` takes a member name, email, UUID, short id,
  or an explicit `member:<uuid>`; `multi_actor` takes a comma-separated list
  (duplicates dropped, order kept, max 20).
- Definitions may include an optional catalog icon for visual identification;
  it does not change the property's type or value validation.
- Agents cannot create or edit property definitions (owner/admin humans only).
  If a needed property does not exist, propose it in a comment instead.
- Property vs metadata: if the value is workflow state a human should see and
  filter by, and a definition exists, prefer the property. Metadata stays the
  free-form bag for durable custom issue state.
- `issue list` filters and sorts by property with the same name addressing:

```bash
multica issue list --property "Impact=High" --property "Impact=Medium" --output json
multica issue list --property "QA Status=__none__" --status in_review --output json
multica issue list --sort property:Impact --direction desc --output json
```

- `--property` takes one `Name=Value` per flag. Repeating the same property
  matches ANY of its values; different properties must ALL match. Values are
  option names or ids (select types), `true`/`false` (checkbox), a member
  name/email/id (actor types), or the value itself for text, url, number,
  and date (`YYYY-MM-DD`). The reserved value `__none__` matches
  issues where the property is unset (works for every type; it is not
  index-backed, so use it for targeted audits rather than as a default
  listing filter). Only `=` is supported today; the `>=`, `<=` and `!=`
  spellings are reserved for comparison filters and are rejected.
- `--sort property:<name-or-id>` orders select properties by option order —
  an ordinal scale (Low < Medium < High) sorts by meaning — and number/date/
  text/url by value; issues without the property sort last either way.
  Archived properties and types without an order (multi_select, checkbox,
  actor kinds) are rejected up front.

## Status changes have server side effects

A status change is not cosmetic — the server enqueues or skips agent work based
on it. These are the contracts, not advice:

Status updates resolve custom categories. `backlog` parks assigned work;
moving it to a non-terminal category enqueues the assignee. `done` and
`cancelled` enqueue nothing, and changing status — including to `cancelled` —
does not stop tasks already in flight. Child completion may wake the parent, and
a merged PR with close intent writes the literal `done` key. Failed runs may
write the literal `todo` key when no active task or retry remains.
Creation has a known exception: a custom terminal status can still enqueue at creation;
use the built-in `done` / `cancelled` keys when creating terminal issues.

When the runtime brief supplies `MULTICA_ISSUE_OUTCOME_FILE`, follow that managed
completion contract for the final status and delivery. Return the final response
normally; do not duplicate it with a CLI comment. Explicit status commands remain
available for deliberate state changes. A successful process exit alone does not
establish review readiness.

Outside that managed contract, progress is agent-owned, not a `StartTask` / `CompleteTask` side effect:
write `in_progress`, `in_review`, or `blocked` when the issue's actual state
changes, including mid-turn. Do not derive status from trigger kind, run
lifecycle, or assignee identity. Load `references/pr-and-status-details.md`
before deciding whether work, discussion, partial delivery, squad dispatch, or
a custom status category requires a write.

## Claim ownership without duplicating a run

Assigning an active issue to an agent normally starts a run. When the work is
already underway and the write only records ownership or progress, pass
`--no-start` on every command in that flow — suppressing the assignment alone
does not suppress a later status update:

```bash
multica issue assign <issue-id> --to-id <agent-id> --no-start
multica issue update <issue-id> --assignee-id <agent-id> --no-start
multica issue status <issue-id> in_progress --no-start
```

Before self-assigning, check the target issue's comment history for an existing
claim and any `## Active sibling runs` block (its `run-messages` commands show
work in flight). The server also suppresses a trusted self-assignment when the
exact target `(issue, agent)` pair already has a non-terminal task, but it
deliberately keeps same-agent handoffs to a fresh issue starting runs: cross-issue
serial chains and triage batches rely on that.

## Who else is running right now

The `## Active sibling runs` block covers only YOUR OWN other in-flight tasks.
It says nothing about another agent working next to you, and it is a claim-time
snapshot that does not refresh mid-turn. Ask the server when you need the
current answer:

```bash
multica issue runs <issue-id> --active --output json     # in-flight runs on this issue
multica issue runs <issue-id> --siblings --output json   # ...and across the sub-issue family
```

`--active` drops the execution history and returns only `queued` / `dispatched`
/ `running` / `waiting_local_directory` runs. `--siblings` widens the same read
to the issue's family — its parent (or itself, when it has no parent) plus every
child of that parent — and labels each row with the issue it belongs to, which
is how you find another agent already working on a sibling sub-issue before you
open a second PR against the same code.

The family read returns a compact row — task, issue, agent, status, started —
not the full execution-log record. If you need a run's detail, follow the task
id with `multica issue run-messages`.

Rows come back running-first, newest-first within a status, and the family read
is capped at 20. When the cap truncates the answer the CLI prints a warning on
stderr — read it. Without that warning a short list means "nobody else is
there"; with it, the list proves nothing about the runs it did not return.

Both are advisory reads. Nothing here reserves an issue or serialises anything:
a run you see may finish a second later, and one you don't see may start a
second later. Coordinate through the issue's comments — the reads tell you whom
to coordinate with.

## Sub-issues: `todo` starts work now, `backlog` parks it

On an agent-assigned issue, create status decides whether the assignee fires
immediately. A runnable built-in status (e.g. `todo`) enqueues the agent at create
time; `--status backlog` sets the assignee without triggering.

Use `--stage <N>` for ordered barrier groups. The parent wakes once, when a whole stage finishes;
an unstaged sibling set is one implicit stage. Advancement is
agent-driven: inspect the children, then promote only dependencies-ready work:

```bash
multica issue children <parent-id>
multica issue status <child-id> todo
```

Load `references/pr-and-status-details.md` before creating serial/phased work or
interpreting stage completion, custom `status_category`, and promotion behavior.

## References

- `references/pr-and-status-details.md` — load for PR link placement and missing
  links, PR lifecycle/check snapshots, progress-status decisions, and phased or
  staged sub-issues.
- `references/working-on-issues-source-map.md` — source locations for every
  contract. Re-derive before depending on an exact line.
