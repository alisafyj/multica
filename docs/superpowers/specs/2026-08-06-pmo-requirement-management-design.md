# PMO Requirement Management Design

**Status:** Approved
**Date:** 2026-08-06

## Summary

Add a workspace-level PMO page named `需求管理` that imports an external
requirement tree through a user-selected Multica Agent. Multica does not call
the external system directly and does not store its credentials or domain in
source-controlled configuration. The Agent returns a versioned JSON document;
Multica validates it, compares it with the last applied baseline and current
local data, previews changes, and materializes approved items as canonical
Multica projects and issues.

Manual runs are preview-first. A fixed 30-minute schedule may be enabled only
after the configuration has completed at least one manually approved run.
Scheduled runs apply non-conflicting changes and leave conflicts for a human.

## Naming

| Surface | Name |
| --- | --- |
| User-visible page | `需求管理` |
| Workspace route | `/{workspace}/pmo` |
| Frontend/backend module | `pmo` |
| API prefix | `/api/pmo` |
| New tables | `pmo_sync_config`, `pmo_sync_run`, `pmo_sync_link` |

`pmo` is intentionally source-specific in v1 because the product connects to
one external requirement system. A future multi-source feature can introduce a
generic requirements layer when that concrete need exists.

## Goals

- Let an owner or admin select any eligible workspace Agent; never hardcode an
  Agent, runtime, nested Agent, skill, domain, or credential.
- Import one parent requirement as a project and preserve child requirement
  and scheduling-task hierarchy as issues.
- Support both manual preview/apply and fixed 30-minute scheduled runs.
- Detect external-only changes, local-only changes, and true concurrent edits
  field by field.
- Never silently overwrite a local edit or hard-delete canonical local data.
- Retain stable external identities independently from Multica IDs.
- Surface unknown assignees as unresolved mappings while still importing the
  rest of the requirement tree.
- Reuse the existing Agent task pipeline, project/issue services, scheduler,
  React Query, navigation adapters, and create dialogs.

## Non-Goals

- Writing changes back to the external system.
- Direct HTTP integration, credential storage, browser automation, or a
  committed external URL.
- Supporting more than one external system in v1.
- Importing test cases or adding a test-case generation page.
- Automatically resolving true conflicts.
- Hard-deleting projects or issues when an external item disappears.
- Building duplicate project or issue creation forms on the PMO page.
- Supporting an arbitrary schedule editor; v1 uses a fixed 30-minute interval.

## Architecture

The feature has five focused parts:

1. **PMO handlers and queries** own workspace authorization, configuration,
   runs, previews, conflict resolutions, and assignee mappings.
2. **PMO sync service** validates Agent output, normalizes hierarchy, computes
   field-level three-way differences, and applies an approved run in a
   transaction.
3. **Existing Agent task pipeline** dispatches a typed `pmo_sync` task to the
   selected Agent. Completion passes the result to the PMO sync service instead
   of creating data directly from unvalidated text.
4. **Existing scheduler** scans due enabled configurations and enqueues at most
   one active run per configuration. Missed intervals collapse to the latest
   run rather than producing a catch-up burst.
5. **Shared PMO page** in `packages/views` uses `packages/core` API schemas,
   queries, and mutations. Web and Desktop supply only route wiring.

The data path is:

```text
manual button or 30-minute scheduler
        |
        v
pmo_sync_run (queued) -> selected Agent task
        |
        v
strict JSON result -> validate -> normalize -> three-way diff
        |
        +--> manual: preview_ready, wait for confirmation
        |
        `--> scheduled: transactionally apply safe actions,
                       retain conflicts for review
```

The selected Agent is a normal Multica Agent ID stored on the configuration.
Any nested runtime routing or installed capability remains part of that Agent's
runtime configuration and is opaque to PMO code.

## Agent Output Contract

The task prompt requires one JSON object with no Markdown fence or prose. The
top-level object uses `schema_version: "1"` and contains a parent requirement,
zero or more child requirements, and scheduling tasks. The server rejects
unknown schema versions and validates the entire trust boundary before
producing a preview.

A fictional shape is:

```json
{
  "schema_version": "1",
  "snapshot_complete": true,
  "parent_requirement": {
    "key": "EXT-P-001",
    "display_number": "REQ-001",
    "numeric_id": 1001,
    "title": "Example parent requirement",
    "description": "Example description",
    "source_status": "active",
    "status": "in_progress",
    "owner": {
      "external_id": "user-001",
      "display_name": "Example User"
    },
    "start_date": "2026-08-01",
    "due_date": "2026-08-31",
    "workload": null
  },
  "child_requirements": [
    {
      "key": "EXT-C-001",
      "display_number": "REQ-001-1",
      "numeric_id": 1002,
      "title": "Example child requirement",
      "description": "Example child description",
      "source_status": "active",
      "status": "todo",
      "owner": null,
      "start_date": null,
      "due_date": null,
      "workload": null,
      "tasks": [
        {
          "task_id": "TASK-001",
          "scheme_id": "SCHEME-001",
          "title": "Example scheduling task",
          "description": "Example task description",
          "source_status": "active",
          "status": "todo",
          "owner": null,
          "start_date": "2026-08-02",
          "due_date": "2026-08-03",
          "workload": 1,
          "updated_at": "2026-08-01T08:00:00Z"
        }
      ]
    }
  ],
  "tasks": []
}
```

The exact implementation schema must enforce:

- non-empty stable keys and unique requirement/task identities within a
  payload;
- `snapshot_complete` is exactly `true`, so an incomplete acquisition cannot
  be interpreted as external deletion;
- preservation of requirement key, display number, and numeric ID;
- preservation of task ID independently from requirement identities;
- valid canonical Multica statuses and calendar dates;
- no hierarchy cycles or task appearing under multiple parents;
- bounded payload, string, and collection sizes;
- rejection of partial/truncated JSON and unknown structural fields;
- normalization before any value is compared or persisted.

`source_status` is retained for audit and display. `status` must already be a
valid canonical Multica status. An invalid canonical status fails the run; it
is never guessed or silently converted by the server.

The task prompt must not name a company, internal domain, real identifier,
installed skill, runtime sub-agent, or credential. Those values exist only in
runtime Agent configuration and user-entered database records.

## Hierarchy Mapping

The mapping is deterministic:

- Parent requirement -> Multica project.
- Child requirement -> top-level issue in that project.
- Scheduling task under a child requirement -> child issue under that issue.
- Parent scheduling task when no child requirements exist -> top-level issue
  in the project.
- Parent scheduling tasks in a payload that also contains child requirements
  remain top-level issues; they are not attached to an arbitrary child.

Issue `project_id` and `parent_issue_id` express the canonical hierarchy. PMO
link rows preserve the external hierarchy and IDs, but do not replace project
or issue ownership of business data.

## Synced Fields

V1 compares and applies only these source-owned candidate fields:

- Project: title, description, status, lead, start date, due date.
- Issue: title, description, status, assignee, start date, due date, workload,
  project, and parent issue.

External workload is stored in the existing issue properties model under one
PMO-owned property definition created for the workspace on first use. It is not
added as a new column on `issue`.

Priority, labels, attachments, comments, acceptance criteria, resources,
custom properties other than the PMO workload property, and all other local
fields are never changed by PMO sync.

## Three-Way Comparison

Every linked entity stores the last successfully acknowledged external and
local values. For each synced field:

```text
E0 = last applied/acknowledged external value
L0 = last applied/acknowledged local value
E1 = value in this run's normalized Agent payload
L1 = current canonical Multica value
```

The decision matrix is:

| External | Local | Result |
| --- | --- | --- |
| `E1 == E0` | `L1 == L0` | unchanged |
| `E1 != E0` | `L1 == L0` | safe incoming update |
| `E1 == E0` | `L1 != L0` | preserve local; do not advance the baseline |
| `E1 != E0` | `L1 != L0`, `E1 == L1` | converged; acknowledge both baselines |
| `E1 != E0` | `L1 != L0`, `E1 != L1` | conflict |

Comparison is field-level, not whole-entity. An external title update may be
applied while a locally edited description remains untouched.

For each conflict, an owner or admin chooses:

- **Use external:** write `E1` locally, then acknowledge both baselines.
- **Keep local:** leave `L1` in place, then acknowledge `E1` and `L1` as the
  new baselines.

A preview stores the normalized external snapshot, but apply always re-reads
current local values and recomputes the decisions. A local edit made after the
preview therefore becomes a conflict instead of being overwritten.

## Creation, Removal, and Identity Mapping

An external entity with no link produces a create action. Creation order is
project, top-level issues, then child issues. The apply transaction writes the
canonical entity and its link together.

If a previously linked external entity is absent from a valid complete
payload, sync sets `externally_removed_at` on its link and shows an
`external_removed` action. It never deletes, archives, cancels, or otherwise
changes the local project or issue automatically. If the entity returns in a
later payload, the marker is cleared and normal comparison resumes.

External assignees use `external_id` as the stable key. A
`pmo_sync_link` row with external type `assignee` maps it to a workspace member.
If no mapping exists:

- project lead or issue assignee remains unassigned;
- the original external ID and display name remain in the link snapshot;
- the run records an unresolved-assignee warning, not a fatal error;
- the PMO page lists the identity in the mapping queue.

Saving a member mapping recomputes the pending preview or takes effect on the
next run. Mapping never guesses by display name.

## Data Model

All three tables are new. Existing `project` and `issue` remain canonical.
There are no database foreign keys or cascading actions. The service validates
workspace ownership and performs dependent cleanup explicitly.

### `pmo_sync_config`

One row represents one external root requirement:

- `id`, `workspace_id`;
- user-visible `name`;
- selected `agent_id`;
- `root_external_key` entered at runtime;
- `schedule_enabled`, `next_run_at`, `last_run_at`, `last_applied_at`;
- `created_by`, `created_at`, `updated_at`.

The fixed v1 schedule is 30 minutes and is not stored as a configurable cron
expression. A configuration cannot enable scheduling until `last_applied_at`
is present.

### `pmo_sync_run`

One row represents one immutable Agent acquisition and its current processing
state:

- `id`, `workspace_id`, `config_id`, nullable `agent_task_id`;
- `trigger` (`manual` or `scheduled`);
- `status` (`queued`, `running`, `preview_ready`, `applied`,
  `applied_with_review`, or `failed`);
- normalized `source_snapshot`, computed `diff`, and summary counts as JSONB;
- redacted `error_code` and `error_message`;
- nullable `requested_by`, plus created, started, completed, and applied times.

The raw Agent transcript is not duplicated here. The run stores only validated
normalized source data and computed output. Logs never include the payload,
titles, descriptions, external identities, or Agent result.

### `pmo_sync_link`

One row links one stable external identity to zero or one canonical local
entity:

- `id`, `workspace_id`, `config_id`;
- `external_type` (`requirement`, `task`, or `assignee`);
- `external_key`, plus nullable display number, numeric ID, and task ID;
- nullable `parent_external_key`;
- nullable `local_type` (`project`, `issue`, or `member`) and `local_id`;
- `baseline_external`, `baseline_local`, and current external metadata as
  JSONB;
- nullable `externally_removed_at`, plus `created_at` and `updated_at`.

The application enforces valid external/local type pairs. A unique concurrent
index on `(config_id, external_type, external_key)` prevents duplicate links.
A partial unique concurrent index permits at most one active run per config.
Every index is created in its own single-statement migration as required by the
repository migration rules.

## Run State and Scheduling

Manual flow:

1. Owner/admin starts a run for a configuration.
2. Server creates `pmo_sync_run` in `queued` and enqueues a typed Agent task.
3. Agent claim moves the run to `running`.
4. Completion validates and normalizes the result, computes the diff, and
   stores `preview_ready` without changing projects or issues.
5. User reviews safe changes, local-only changes, conflicts, removals, and
   unresolved assignees.
6. Apply revalidates local state and commits selected safe actions and conflict
   resolutions in one transaction.

Scheduled flow:

1. The existing scheduler scans enabled configs whose `next_run_at` is due.
2. It atomically claims a config, advances `next_run_at` by 30 minutes, and
   creates one scheduled run.
3. Valid completion applies new entities, safe incoming fields, and converged
   fields in one transaction.
4. Local-only fields, removals, unresolved assignees, and conflicts remain for
   review. The run ends as `applied_with_review` when any review item exists.

Only the latest missed interval runs after downtime. Manual and scheduled
starts return a conflict response when another active run exists for that
configuration.

## API and Authorization

The API surface is workspace-scoped under `/api/pmo`:

- list/create/update/delete configurations;
- start a manual run;
- list runs and read one run with its diff;
- apply a preview with explicit conflict resolutions;
- create/update an assignee mapping.

All responses consumed by the UI use `parseWithFallback` and zod schemas.
Owner/admin membership is required to create or change configurations, trigger
runs, apply changes, or edit mappings. Workspace members may view the page,
configuration state, run summaries, and previews. Every handler validates that
the config, run, selected Agent, local entity, and mapped member belong to the
request workspace.

Deleting a configuration is allowed only when no run is active. Application
code deletes its run/link rows and configuration in one transaction; canonical
projects and issues remain untouched.

## Page Design

`/{workspace}/pmo` is a top-level workspace page alongside Design. It is a
dense operational surface, shared by Web and Desktop.

The page contains:

- configuration selector and compact create/edit control;
- selected Agent control, external root key, fixed schedule toggle, and last
  run status;
- icon actions for immediate sync, new project, and new issue;
- diff table filterable by create, update, local-only, conflict,
  external-removed, and unresolved-assignee state;
- explicit Apply button for manual previews;
- assignee mapping queue;
- recent run history with status, trigger, timestamps, counts, and redacted
  failure reason.

The new-project and new-issue actions open existing creation workflows. The PMO
feature does not own duplicate forms. Data is fetched through React Query with
workspace-scoped keys; Zustand is unnecessary for durable sync state. The page
uses existing navigation and Agent selection helpers and adds English and
Chinese translations following repository terminology.

## Failure Handling

- Agent unavailable, task failure, timeout, or cancellation: mark the run
  failed and write no canonical data.
- Non-JSON output, schema mismatch, duplicate identity, invalid hierarchy,
  invalid status/date, or size limit violation: mark failed with a redacted
  validation error and write no canonical data.
- Database error during apply: roll back the entire apply transaction; retain
  the preview for retry.
- Workspace or permission mismatch: reject before Agent dispatch or apply.
- Concurrent run: return conflict and preserve the active run.
- Local change after preview: recompute and promote the affected field to a
  conflict.
- Unknown assignee: import unassigned and retain a mapping warning.
- External omission: mark externally removed only after a complete, valid
  payload; never interpret a failed or partial run as deletion.
- Scheduler restart or downtime: run only the latest due occurrence and avoid
  catch-up bursts.

Retries create a new run and call the Agent again. Applying a stored preview
after a database failure retries the same normalized external snapshot and
rechecks local values.

## Testing

Backend tests cover:

- strict schema acceptance and rejection cases;
- hierarchy normalization for child-requirement and no-child variants;
- all three-way comparison matrix branches at field granularity;
- create ordering, stable identity persistence, and idempotent reruns;
- local edits after preview becoming conflicts;
- external removal and later reappearance;
- unresolved and resolved assignee mappings;
- transactional rollback and workspace/role boundaries;
- one-active-run concurrency and latest-only schedule behavior;
- Agent completion, failure, and invalid-output run transitions.

Frontend tests cover:

- API response fallback parsing;
- workspace-scoped query keys and invalidation;
- loading, empty, preview, conflict, unresolved, applied, and failed states;
- Agent selection, schedule guard, apply confirmation, mapping, and retry;
- reuse of existing project/issue creation actions;
- shared-page behavior without Next.js or React Router mocks.

Default tests use a fake Agent executable and fictional payloads such as
`EXT-P-001`. They must not resolve or invoke a user-installed Agent CLI. After
automated verification, one explicitly authorized local smoke test may use a
runtime-only external key in preview mode. No real identifier or output is
committed as a fixture or logged.

## Delivery Boundary

V1 is complete when an owner/admin can configure a selected Agent, preview and
apply one external requirement tree, enable the 30-minute schedule, observe
safe automatic updates, resolve conflicts and assignee mappings, and audit run
history without any source-controlled external system details.

Test-case generation, multiple external sources, arbitrary schedules,
write-back, automatic conflict policy, and full external-payload retention are
left for separate designs after real v1 usage demonstrates the need.
