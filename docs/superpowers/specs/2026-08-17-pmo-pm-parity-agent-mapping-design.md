# PMO Source-Parity Preview and Agent Assignment Design

**Status:** Approved direction, pending written-spec review
**Date:** 2026-08-17
**Branch:** `pmo-pm-parity-agent-mapping`

## Summary

Fix the PMO detail page so it presents the imported requirement in the same
shape users see in the PM system: requirement information first, followed by a
Milestone-grouped schedule with one task per row. Keep field-level sync
decisions, but show them inside that source-shaped view instead of flattening
every entity field into a separate table row.

Change assignee mappings from external owner to workspace member into external
owner to a concrete Agent owned by that member. Applied projects and issues use
the mapped Agent as their lead or assignee, allowing imported tasks to enter the
normal Agent execution path.

The PM requirement reviewed for this design is `SY-P-20260452`, titled
`院务系统-开单-增加美团订单券码校验-1.0`. Its PRD is:

`https://soyoung.feishu.cn/wiki/Ifl9wASw2iWHL4kEbN1cpF3Ynje`

## Observed Problems

1. The shared Web/Desktop detail view has no usable vertical scroll container
   when the preview is taller than the application shell.
2. The preview flattens one requirement or task into one row per changed field.
   The reviewed run produced more than one hundred rows for only a small number
   of source entities, so users cannot recognize the original PM structure.
3. The normalized task contract retains only `scheme_id`. It does not retain a
   human-readable Milestone name, so the UI cannot reliably reproduce headings
   such as `M4-开发-前端` or `M5-QA-测试`.
4. The normalized requirement contract has no explicit PRD URL or priority.
5. Assignee mappings currently store a workspace user ID with
   `local_type='member'`. Apply then forces project leads and issue assignees to
   `member`, although the product requirement is to assign the responsible
   person's Agent.
6. Automatic email recognition can identify a member, but a member may own zero,
   one, or multiple Agents. A member ID alone is not enough to select an Agent.

## Goals

- Make long PMO detail pages scroll in both Web and Desktop.
- Present the root requirement as a readable summary with a clickable PRD.
- Present schedules by Milestone with one source task per row.
- Preserve the existing preview filters and field-level conflict decisions.
- Map each external owner to a concrete, runnable workspace Agent.
- Apply mapped Agents as project leads and issue assignees.
- Keep unresolved owners visible without failing the rest of the sync.
- Preserve existing PMO tables and the current JSON snapshot storage model.

## Non-Goals

- Rebuilding the entire PM edit form inside Multica.
- Writing changes back to the PM system.
- Creating or synchronizing Multica project stages from PM Milestones.
- Guessing an Agent when one member owns multiple eligible Agents.
- Bulk-reassigning historical issues when this change is deployed.
- Adding a new dependency or a new database table.

## Source Snapshot Contract

Extend the existing schema version `1` with optional display metadata. Keeping
the version avoids rejecting an in-flight or stored version-1 snapshot; new runs
populate the fields, while old runs render conservative fallbacks.

### Requirement metadata

Add these optional fields to `PMORequirement`:

- `priority`: source priority label, for example `P2-3`.
- `prd_url`: canonical PRD URL from the PM requirement field.

The acquisition prompt must request these values explicitly. `prd_url` is
validated as an absolute `http` or `https` URL with the same bounded-string
rules as the rest of the trust boundary. Missing values remain `null` or empty;
the server does not invent a URL.

For older snapshots, the UI may show the first safe absolute URL already present
in the requirement description as a display-only fallback. It must not rewrite
the stored snapshot from that heuristic.

### Task metadata

Add optional `scheme_name` to `PMOTask`. New acquisitions populate the visible
PM Milestone label, such as `M4-开发-后端（包含算法）`.

The existing `scheme_id` remains the stable source identity. The preview groups
by `(scheme_id, scheme_name)` and falls back to `scheme_id` when an old snapshot
does not contain a name. Milestone metadata is for source-parity display only;
it is not added to the synced issue field set in this change.

The optional snapshot fields require no schema change. Agent mappings do:
`pmo_sync_link.local_type` has an existing CHECK constraint, so migration 890
extends it to allow `agent`. Rolling back 890 clears Agent mappings to the
unresolved state before restoring the old constraint; that rollback is
intentionally lossy because the old schema cannot represent Agent IDs.

## Preview Design

### Requirement summary

The preview begins with the root requirement rather than a generic diff table.
It shows:

- display number and title;
- source status and canonical sync status;
- priority;
- source owner;
- planned start and due dates;
- workload when present;
- clickable PRD URL when present;
- entity action and conflict count.

Only values present in the source are rendered. This keeps the view faithful
without reproducing every editable field from the PM form.

Child requirements remain visibly subordinate. Each child requirement gets a
compact requirement heading followed by its own task schedule. Parent-level
tasks remain in the root schedule, preserving the existing hierarchy mapping.

### Schedule

Each requirement schedule uses the PM column order:

| Task | Owner | Start | Finish | Workload | Milestone | Status |
| --- | --- | --- | --- | --- | --- | --- |

Tasks are grouped under their human-readable Milestone heading and keep source
order within each group. Empty placeholder rows from the PM system are not
imported or displayed.

The external value is the primary cell content. When a field differs locally,
the cell shows a compact state indicator and the local value underneath or in a
popover. A conflict keeps the existing external/local choice control. This
retains review safety without turning each field into a separate table row.

The existing filters operate on whole entities:

- selecting `更新` shows task rows or requirement summaries containing at least
  one incoming update;
- selecting `冲突` shows entities containing at least one conflict;
- selecting `未映射负责人` shows every entity that references an unresolved
  external owner.

### Scrolling

The shared PMO detail page owns one `min-height: 0`, `overflow-y: auto` content
region below its page header. The fix stays in `packages/views`, so Web and
Desktop receive the same behavior without platform-specific copies.

Wide task schedules use horizontal overflow inside the schedule region. Page
vertical scrolling and table horizontal scrolling remain independent.

## Agent Mapping

### Mapping target

Each mapping row represents one stable external owner ID. Its selector lists
eligible workspace Agents, displaying:

`Agent name · owner member name`

The persisted link becomes:

```text
external_type = assignee
external_key  = external owner ID
local_type    = agent
local_id      = agent.id
```

The API request uses `agent_id`; it no longer calls a user UUID a `member_id`.
The server validates that the Agent belongs to the workspace, is not archived,
and is bound to a runtime. Temporary runtime offline status does not invalidate
an existing mapping.

### Automatic recognition

Automatic recognition remains conservative:

1. Normalize the external owner ID into an exact corporate email candidate.
2. Match the email to one workspace member; never match by display name.
3. Find unarchived, runtime-bound Agents whose `owner_id` equals that member's
   user ID.
4. If exactly one Agent matches, use it automatically.
5. If zero or multiple Agents match, keep the owner unresolved and require a
   manual selection.

Explicit mappings always win over automatic recognition.

### Legacy member mappings

Existing `local_type='member'` mappings must never continue assigning imported
work to a member silently.

- If the mapped member owns exactly one eligible Agent, the next preview may
  resolve to that Agent and the next successful apply persists the upgraded
  Agent link.
- If the member owns zero or multiple eligible Agents, show the mapping as
  unresolved and require a manual selection.
- Existing project and issue assignees are not bulk-updated. They change only
  when a later approved PMO apply contains an assignee update.

## Apply Behavior

For resolved owners:

- root requirement project: `lead_type='agent'`, `lead_id=agent.id`;
- child requirement issue: `assignee_type='agent'`,
  `assignee_id=agent.id`;
- scheduling task issue: `assignee_type='agent'`,
  `assignee_id=agent.id`.

For unresolved owners, creation or other safe field updates continue with no
assignee. The diff retains an `unresolved_assignee` warning and the apply does
not fail solely because a mapping is missing.

The PRD remains visible in the imported project description through the
existing description sync. This change does not create a separate project
resource; that can be added later if Agents need a stronger resource contract
than the existing project context provides.

## Error Handling

- Reject malformed or unsupported PRD URLs at the snapshot trust boundary.
- Reject manual mappings to Agents outside the workspace, archived Agents, or
  Agents without a runtime binding.
- Treat ambiguous automatic mappings as unresolved, never as an error for the
  complete run.
- Preserve explicit Agent mappings when a later email lookup result changes.
- Render older snapshots with `scheme_id` and description URL fallbacks rather
  than failing the entire preview.

## Implementation Sequence

1. Lock the desired snapshot metadata and Agent-assignment behavior with focused
   Go and frontend regression tests.
2. Extend and validate the PMO snapshot contract and acquisition prompt.
3. Change mapping resolution, persistence, handler request shape, and apply
   writes from member IDs to Agent IDs.
4. Update core API types/schemas/mutations for `agent_id`.
5. Replace the flattened preview with the requirement summary and grouped task
   schedule; retain existing conflict resolution and filters.
6. Add the shared scroll container and table horizontal overflow.
7. Run focused PMO tests, Go tests for the changed service/handler packages,
   TypeScript typecheck, lint, and Web/Desktop browser smoke checks.

## Verification

The change is complete when fresh evidence proves:

- the reviewed requirement renders with its title, status, priority, owner,
  dates, and PRD link;
- `M4-开发-前端`, `M4-开发-后端（包含算法）`, `M5-QA-测试`, and other Milestones
  appear as group headings;
- every non-empty PM scheduling task appears once with the seven PM columns;
- long previews scroll vertically in Web and Desktop;
- a uniquely matched owner maps to the correct owned Agent;
- ambiguous or missing Agents remain unresolved;
- manual mapping stores an Agent ID and rejects an invalid workspace Agent;
- newly created and updated projects/issues use Agent lead/assignee types;
- an unresolved owner does not block unrelated apply operations;
- stored older snapshots and legacy member mappings follow the documented
  fallback behavior.

## Risks and Limits

- The external acquisition Agent must actually return `scheme_name`, priority,
  and PRD URL. Until a new successful run does so, old previews use fallbacks.
- A member-to-Agent relationship is one-to-many. Automatic selection is safe
  only for exactly one eligible Agent.
- This design mirrors the PM review surface, not the full PM editing product.
  Adding more source fields should be driven by a concrete review or execution
  need rather than copying the whole form.
