---
name: multica-test-cases
description: "Use when reading, writing, or reviewing Multica test cases — including finding which repositories, project, and issues a case relates to. Executing a case and recording results is not covered: that surface does not exist yet."
user-invocable: false
allowed-tools: Bash(multica *)
---

# Test Cases

This skill states WHAT a Multica test case is and what the CLI guarantees about
it, traced to source. Every claim is pinned in
`references/test-cases-source-map.md`; when behavior differs from this document,
the source map is where to re-check it.

## A case is project-scoped and machine-readable

A test case always belongs to exactly one project. There is no workspace-wide
case list without a project: the project is what supplies the repositories and
the durable context a case is written against.

`steps` is a structured JSON array, not markdown:

    [{"index": 1, "action": "点击下单", "expected": "跳转支付页", "repo": "admin-web"}]

`index` is always the running order 1..n — the server renumbers on every write,
so a gap never survives a save. `repo` is optional and, when present, must be
one of the case's own `repos[].alias` values.

## Identity: TC-<n> and UUID are both accepted

Every case carries `case_number` (an int, unique per workspace) and `key` (the
rendered `TC-42`). Both `key` and the UUID `id` are accepted wherever the CLI
takes a case reference — the server resolves them. Prefer the key: it is what
humans quote in issues and comments.

    multica testcase get TC-42 --output json
    multica testcase get 6b1e...-... --output json

## Reading cases

    multica testcase list --project <project-id> --output json
    multica testcase list --project <project-id> --status draft --output json
    multica testcase list --project <project-id> --digest --output json
    multica testcase modules --project <project-id> --output json

`--digest` drops `steps`, `test_data` and every other body field, keeping only
identity and classification. Use it when you need to know which cases already
exist — surveying a library before proposing new cases, or checking for
duplicates — and only fetch full bodies for the handful you actually need.

`modules` returns `{module, case_count}` per module, including an empty-string
module for ungrouped cases.

## Multi-repo cases

A project may bind several repositories. `repos[]` records which ones a case
touches and in what capacity:

| role | meaning |
| --- | --- |
| `under_test` | the system whose behavior is being verified |
| `driver` | where the tester performs the action |
| `verifier` | where the result is observed |
| `fixture` | where test data is prepared |

Roles are what make "change the price in the backend, then check the order page
in the app" machine-readable. A case with `scope: "cross_repo"` is expected to
name at least two repositories with more than one role; the UI flags a case
that does not.

Each binding points at a `project_resource_id`, not a repo URL — so run
`multica project resource list <project-id> --output json` to map an alias back
to a checkout URL, then `multica repo checkout <url>` to fetch the code.

## Writing cases

    multica testcase create --project <id> --title "..." --steps '[{"action":"...","expected":"..."}]'
    multica testcase update TC-42 --priority p0
    multica testcase update TC-42 --steps-stdin < steps.json
    multica testcase approve TC-42
    multica testcase delete TC-42

Only flags you actually pass are sent, so an update never blanks a field you did
not mention. Passing an explicitly empty value (`--module ""`) IS a clear.

Every update writes a snapshot of the case as it was BEFORE the change into its
revision history, so an edit is always reversible. Bumping `version` is the
server's job — never send it.

`approve` moves a case from `draft` to `active` and stamps the reviewer. It
rejects a case that is already active.

## Enums

Sending a value outside these lists returns 400 with the allowed list; it never
reaches the database.

- `status`: `draft`, `active`, `deprecated`
- `origin`: `ai`, `human` — set by the server, not the client
- `priority`: `p0`, `p1`, `p2`, `p3`
- `scope`: `single_repo`, `cross_repo`, `no_repo`
- `execution_mode`: `manual`, `agent`, `both`
- `case_type`: `functional`, `business_flow`, `api`, `ui`, `e2e`, `regression`,
  `boundary`, `exception`, `permission`, `data_consistency`, `compatibility`,
  `performance`, `security`

`case_type` deliberately spans more than code-level testing. A library that is
only `functional` and `api` cases is under-covering the product: business flows,
permission matrices and data-consistency rules are cases too.

## What does not exist yet

There is no `multica test` command group, no generation job, and no run or
result recording. A case is a durable document you can read, write and review —
nothing consumes it automatically yet. Do not invent commands for those.
