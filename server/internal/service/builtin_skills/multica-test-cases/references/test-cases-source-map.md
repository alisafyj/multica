# Test cases — source map

Every claim in `SKILL.md` traces to a line below. Re-derive against the current
tree before trusting any line number; the behavior is the contract, the line is
a pointer.

## Schema and identity

| Fact | Source |
| --- | --- |
| `test_case` table: project-scoped, `workspace_id` + `project_id` both NOT NULL | `server/migrations/280_test_case.up.sql:5` |
| `steps` is JSONB defaulting to `[]` | `server/migrations/280_test_case.up.sql:12` |
| `case_number` is unique per workspace | `server/migrations/281_test_case_workspace_number_index.up.sql:3` |
| Case numbers come from `workspace.test_case_counter`, incremented under the workspace row lock — same mechanism as issue numbering | `server/migrations/280_test_case.up.sql:75`, `server/pkg/db/queries/workspace.sql:66` |
| The counter is taken inside the create transaction, so concurrent creates cannot collide | `server/internal/handler/test_case.go:583` |
| Key prefix is the fixed literal `TC-` (not workspace-configurable, unlike issue prefixes) | `server/internal/handler/test_case_ref.go:16` |
| `formatTestCaseKey` renders `TC-<n>` | `server/internal/handler/test_case_ref.go:19` |
| `parseTestCaseNumber` accepts `TC-42` case-insensitively with surrounding space; rejects `TC-0`, a bare number, and any other prefix | `server/internal/handler/test_case_ref.go:27` |
| Both a key and a UUID resolve through one loader, and writes then use the resolved entity id | `server/internal/handler/test_case_ref.go:45` |

## Steps

| Fact | Source |
| --- | --- |
| `TestCaseStep` = `{index, action, expected, repo?}` | `server/internal/handler/test_case.go:22` |
| The server renumbers steps 1..n on every write, so a gap never persists | `server/internal/handler/test_case.go:282` |
| Renumbering is applied on create | `server/internal/handler/test_case.go:600` |
| Renumbering is applied on update | `server/internal/handler/test_case.go:679` |
| The step editor offers only the case's own repo aliases | `packages/views/testing/components/test-case-steps-editor.tsx:73` |

## Enums

| Fact | Source |
| --- | --- |
| CHECK constraints for priority / case_type / scope / execution_mode / status / origin | `server/migrations/280_test_case.up.sql:16` |
| Go-side allow-lists mirroring those CHECKs | `server/internal/handler/test_case.go:194` |
| Repo roles `under_test / driver / verifier / fixture` | `server/migrations/280_test_case.up.sql:69`, `server/internal/handler/test_case.go:203` |
| An unknown enum returns 400 naming the allowed values instead of a 500 from the DB CHECK | `server/internal/handler/test_case.go:212` |
| Pre-validation runs before the insert | `server/internal/handler/test_case.go:528` |
| `origin` is set server-side to `human` on every create; only a generation job will write `ai` | `server/internal/handler/test_case.go:611` |

## Multi-repo bindings

| Fact | Source |
| --- | --- |
| `test_case_repo` binds a case to a `project_resource_id`, with an alias, a role and path globs | `server/migrations/280_test_case.up.sql:62` |
| No foreign key — the binding is validated in application code | `server/internal/handler/test_case.go:303` |
| A resource must belong to the same project, or the request is rejected | `server/internal/handler/test_case.go:351` |
| Only `github_repo` and `local_directory` are accepted; other resource types are not repositories | `server/internal/handler/test_case.go:207`, `server/internal/handler/test_case.go:356` |
| The same resource cannot be bound twice under one role | `server/internal/handler/test_case.go:332` |
| Repo bindings are fetched in one batched query for the list view, not per case | `server/internal/handler/test_case.go:405` |
| A `cross_repo` case needs ≥2 repositories and >1 distinct role, else the UI warns | `packages/views/testing/case-summary.ts:60` |

## Reading

| Fact | Source |
| --- | --- |
| `GET /api/test-cases` with project/status/module/priority/case_type/origin filters | `server/pkg/db/queries/test_case.sql:1`, `server/cmd/server/router.go` (`/api/test-cases` group) |
| `GET /api/test-cases/modules` returns `{module, case_count}`, including the empty-string module | `server/pkg/db/queries/test_case.sql:23` |
| Literal sub-paths are registered before `{ref}` so `modules` is not swallowed by the ref route | `server/cmd/server/router.go` (`/api/test-cases` group) |
| `--digest` keeps only identity and classification fields | `server/cmd/multica/cmd_testcase.go:114` |
| `digestTestCase` drops `steps` and `test_data` | `server/cmd/multica/cmd_testcase.go:119` |
| The CLI sends the ref verbatim; TC keys are resolved server-side, never by a local prefix index | `server/cmd/multica/cmd_testcase.go:309` |

## Writing

| Fact | Source |
| --- | --- |
| Only flags the caller changed are sent | `server/cmd/multica/cmd_testcase.go:323` |
| An explicitly empty value is a clear, not an omission | `server/cmd/multica/cmd_testcase_test.go:50` |
| `--steps` must be valid JSON, checked client-side | `server/cmd/multica/cmd_testcase.go:350` |
| Every mutable column uses `COALESCE(narg, col)`, so a partial update cannot blank an unmentioned field | `server/pkg/db/queries/test_case.sql:37` |
| `version` is bumped by the UPDATE statement itself | `server/pkg/db/queries/test_case.sql:59` |
| Update snapshots the pre-change case into `test_case_revision` in the same transaction as the update | `server/internal/handler/test_case.go:719` |
| Approve requires `draft` and returns 409 otherwise | `server/internal/handler/test_case.go:790` |
| Approve stamps `reviewed_by` / `reviewed_at` | `server/internal/handler/test_case.go:814` |
| Delete sweeps repo bindings and revisions in the same transaction — neither has a cascade | `server/internal/handler/test_case.go:838` |

## Verification command

```bash
cd server && rg -n \
  'testCaseKeyPrefix|parseTestCaseNumber|loadTestCaseForUser|normalizeTestCaseSteps|validTestCase|validateTestCaseRepos|testcaseDigestFields|testcasePath|testcaseBodyFromFlags' \
  internal/handler/test_case.go internal/handler/test_case_ref.go cmd/multica/cmd_testcase.go
rg -n 'CREATE TABLE test_case|CHECK \(|test_case_counter' migrations/280_test_case.up.sql
rg -n 'name: (List|Get|Create|Update|Delete)TestCase' pkg/db/queries/test_case.sql
```
