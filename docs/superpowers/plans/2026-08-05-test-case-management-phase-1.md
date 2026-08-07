# 测试用例管理 · 第一期 实施计划

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** 交付可独立上线的测试用例库 —— 侧边栏新 tab、用例 CRUD、扇形分组、多仓库关联、版本快照，以及智能体只读 CLI 与内置技能。

**Architecture:** 后端新增 `test_case` / `test_case_revision` / `test_case_repo` 三张无外键表和一个 workspace 计数器，走仓库既有的 migration → sqlc → handler → router → protocol event 链路。前端按 `packages/core` 三层（client + schema + queries/mutations）加 `testing` 域，页面放 `packages/views/testing/`，web 与 desktop 各接一层薄壳。CLI 新增 `multica testcase` 只读命令组，配套内置技能 `multica-test-cases`。

**Tech Stack:** Go 1.26 / Chi / sqlc / pgx v5 / PostgreSQL；TypeScript strict / React / TanStack Query / zod；Next.js App Router（web）、react-router-dom（desktop）；Cobra（CLI）；Vitest + Go test + Playwright。

## Global Constraints

依据 [spec](../specs/2026-08-05-test-case-management-design.md)。以下每条对每个 Task 都生效：

- **禁止外键**：新表一律不写 `FOREIGN KEY` / `REFERENCES`，不写级联。关联校验与依赖清理在应用层做；需要原子性时用 `h.TxStarter.Begin` 事务。
- **索引单独成文件**：每个索引一条 `CREATE INDEX CONCURRENTLY IF NOT EXISTS`（或 `CREATE UNIQUE INDEX CONCURRENTLY IF NOT EXISTS`）单语句迁移，文件头写注释 `-- This is intentionally a single statement: concurrent index creation cannot run in a transaction or a multi-command migration.`（照抄 `server/migrations/167_resource_label_namespace_index.up.sql`）。每个 `.up.sql` 配一个 `.down.sql`。
- **多租户**：每张表带 `workspace_id UUID NOT NULL`；每条 sqlc 查询带 `workspace_id` 过滤，包括按 id 取单条。
- **后端 UUID 规则**：路径参数可能是 `TC-42` 这类人类可读 id 的，走 `loadTestCaseForUser` 解析后用 `entity.ID` 写库；纯 UUID 边界输入用 `parseUUIDOrBadRequest(w, s, field)` 并在 `ok=false` 时立即 return。
- **API 兼容**：`packages/core/api/client.ts` 里每个新方法都必须 `return parseWithFallback(raw, XxxSchema, EMPTY_XXX, { endpoint: "..." })`，不得把网络 JSON 直接断言成 `T`。每个新 schema 至少一个畸形响应测试。
- **i18n 强制**：`packages/views/**/*.tsx` 的 `i18next/no-literal-string` 是 **error**，`designs/**` 的豁免不适用于新代码。所有面向用户的字符串走 `useT("testing")`，四个 locale（`en` / `zh-Hans` / `ja` / `ko`）齐全，否则 `packages/views/locales/parity.test.ts` 失败。
- **设计令牌**：只用语义类（`bg-background`、`text-muted-foreground`），字号只用 `packages/ui/styles/tokens.css` 的 `--text-*` 角色刻度（`text-caption` / `text-body` / `text-title` …），不用 Tailwind 默认 `text-sm` / `text-base`。
- **包边界**：`packages/views/` 内禁止 `next/*`、`react-router-dom`、Zustand store 定义；导航一律 `useNavigation()` / `<AppLink>`。共享 store 只能放 `packages/core/`。
- **注释一律英文**；Go 走 `gofmt` / `go vet` / 检查错误返回。
- **迁移编号**：当前最后一个是 `279_runtime_profile_add_reasonix`。本计划占用 `280`–`291`。开工前跑 `ls server/migrations | tail -3` 确认没有被别的分支占用；若被占用，整体顺延并同步更新本计划。
- **CLI 命名登记**：新增默认 agent 命令名要写进 `scripts/agent-cli-command-names.txt`（本期新增的是 `testcase`，属于 `multica` 子命令而非独立可执行文件，**不需要**登记；此条为提醒，勿误加）。
- **文档同 PR**：改动 CLI 命令 / API 字段 / 内置技能覆盖的行为时，同 PR 更新对应 `SKILL.md` 与 `references/*-source-map.md`。

---

## File Structure

**新建**

| 文件 | 职责 |
| --- | --- |
| `server/migrations/280_test_case.up.sql` `.down.sql` | 三张表 + workspace 计数器列 |
| `server/migrations/281..287_*_index.up.sql` `.down.sql` | 七个并发索引，各自一个文件 |
| `server/pkg/db/queries/test_case.sql` | 用例 / 快照 / 仓库关联 / 计数器的全部 sqlc 查询 |
| `server/internal/handler/test_case.go` | 用例 CRUD、审批、仓库关联、版本快照的 HTTP 层 |
| `server/internal/handler/test_case_ref.go` | `TC-42` ↔ UUID 解析与 `loadTestCaseForUser` |
| `server/internal/handler/test_case_test.go` | handler 表驱动测试 |
| `server/internal/handler/test_case_ref_test.go` | ref 解析测试 |
| `server/cmd/multica/cmd_testcase.go` | `multica testcase` 命令组 |
| `server/cmd/multica/cmd_testcase_test.go` | CLI 命令测试 |
| `server/internal/service/builtin_skills/multica-test-cases/SKILL.md` | 智能体侧数据契约 |
| `server/internal/service/builtin_skills/multica-test-cases/references/test-cases-source-map.md` | 行为 → 文件:行 映射 |
| `packages/core/types/testing.ts` | 领域 TS 类型 |
| `packages/core/testing/keys.ts` | query key 工厂 |
| `packages/core/testing/queries.ts` | queryOptions 工厂 |
| `packages/core/testing/mutations.ts` | mutation hooks |
| `packages/core/testing/config.ts` | 枚举的展示标签与颜色 |
| `packages/core/testing/index.ts` | barrel |
| `packages/core/testing/*.test.ts` | keys / mutations 测试 |
| `packages/views/testing/test-cases-page.tsx` | 列表页 |
| `packages/views/testing/test-case-detail.tsx` | 详情页 |
| `packages/views/testing/components/test-case-steps-editor.tsx` | 结构化步骤编辑器 |
| `packages/views/testing/components/test-case-repos-field.tsx` | 关联仓库编辑 |
| `packages/views/testing/case-summary.ts` | 纯函数（分组、筛选、步骤规范化） |
| `packages/views/testing/case-summary.test.ts` | 纯函数测试 |
| `packages/views/testing/*.test.tsx` | 组件测试 |
| `packages/views/testing/index.ts` | barrel |
| `packages/views/locales/{en,zh-Hans,ja,ko}/testing.json` | i18n 命名空间 |
| `apps/web/app/[workspaceSlug]/(dashboard)/tests/page.tsx` | web 列表壳 |
| `apps/web/app/[workspaceSlug]/(dashboard)/tests/[id]/page.tsx` | web 详情壳 |
| `e2e/test-cases.spec.ts` | 端到端 |

**修改**

| 文件 | 改动 |
| --- | --- |
| `server/pkg/db/queries/workspace.sql` | 加 `IncrementTestCaseCounter` |
| `server/cmd/server/router.go` | `/api/test-cases` 路由组 |
| `server/pkg/protocol/events.go` | `test_case:*` 事件常量 |
| `server/cmd/multica/main.go` | 注册 `testcaseCmd` |
| `packages/core/paths/paths.ts` | `tests` / `testCaseDetail` |
| `packages/core/paths/route-icons.ts` | `RouteIconName` / `NavLabelKey` / `WorkspacePageKey` / `WORKSPACE_PAGES` |
| `packages/core/paths/consistency.test.ts` | 两处硬编码清单 |
| `packages/core/diagnostics/diagnostic-context.ts` | `WORKSPACE_ROUTES` |
| `packages/core/api/schemas.ts` | `TestCaseSchema` 等 + `EMPTY_*` |
| `packages/core/api/schemas.test.ts` | 畸形响应测试 |
| `packages/core/api/client.ts` | 用例端点方法 |
| `packages/core/types/index.ts` | 转出 `testing.ts` |
| `packages/core/types/events.ts` | `WSEventType` 扩 `test_case:*` |
| `packages/core/realtime/use-realtime-sync.ts` | `refreshMap.test_case` |
| `packages/core/package.json` | `exports` 加 `./testing` 三项 |
| `packages/views/layout/route-icon-components.tsx` | `FlaskConical` 图标 |
| `packages/views/layout/app-sidebar.tsx` | 本地两个联合 + `workspaceNav` |
| `packages/views/layout/app-sidebar.test.tsx` | `useWorkspacePaths` mock |
| `packages/views/locales/{en,zh-Hans,ja,ko}/layout.json` | `nav.tests` |
| `packages/views/locales/index.ts` | 注册 `testing` 命名空间 |
| `packages/views/editor/utils/link-handler.ts` | `WORKSPACE_ROUTE_SEGMENTS` 加 `tests` |
| `packages/views/package.json` | `exports` 加 `./testing` |
| `apps/desktop/src/renderer/src/routes.tsx` | 两条 session 路由 |

---

## Task 1: 数据库表与索引

**Files:**
- Create: `server/migrations/280_test_case.up.sql`, `server/migrations/280_test_case.down.sql`
- Create: `server/migrations/281_test_case_workspace_number_index.{up,down}.sql`
- Create: `server/migrations/282_test_case_project_status_index.{up,down}.sql`
- Create: `server/migrations/283_test_case_generation_job_index.{up,down}.sql`
- Create: `server/migrations/284_test_case_revision_case_index.{up,down}.sql`
- Create: `server/migrations/285_test_case_repo_case_index.{up,down}.sql`
- Create: `server/migrations/286_test_case_repo_resource_index.{up,down}.sql`
- Create: `server/migrations/287_test_case_module_index.{up,down}.sql`

**Interfaces:**
- Consumes: 无（第一个任务）
- Produces: 表 `test_case`、`test_case_revision`、`test_case_repo`；列 `workspace.test_case_counter INT NOT NULL DEFAULT 0`。后续任务的 sqlc 查询依赖这些列名。

- [ ] **Step 1: 确认迁移编号未被占用**

```bash
ls server/migrations | tail -3
```

Expected: 最后是 `279_runtime_profile_add_reasonix.*`。若不是，把本计划 280–287 整体顺延。

- [ ] **Step 2: 写建表迁移**

`server/migrations/280_test_case.up.sql`：

```sql
-- Test case library. No FOREIGN KEY / cascade by repository rule: relationships
-- (project_id, generation_job_id, project_resource_id) are validated in
-- application code and cleaned up in explicit transactions.
CREATE TABLE test_case (
    id                    UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    workspace_id          UUID NOT NULL,
    project_id            UUID NOT NULL,
    case_number           INT  NOT NULL,
    title                 TEXT NOT NULL,
    module                TEXT NOT NULL DEFAULT '',
    preconditions         TEXT NOT NULL DEFAULT '',
    steps                 JSONB NOT NULL DEFAULT '[]',
    expected_result       TEXT NOT NULL DEFAULT '',
    test_data             JSONB NOT NULL DEFAULT '{}',
    priority              TEXT NOT NULL DEFAULT 'p2'
                          CHECK (priority IN ('p0','p1','p2','p3')),
    case_type             TEXT NOT NULL DEFAULT 'functional'
                          CHECK (case_type IN ('functional','business_flow','api','ui','e2e',
                                               'regression','boundary','exception','permission',
                                               'data_consistency','compatibility','performance','security')),
    scope                 TEXT NOT NULL DEFAULT 'single_repo'
                          CHECK (scope IN ('single_repo','cross_repo','no_repo')),
    execution_mode        TEXT NOT NULL DEFAULT 'manual'
                          CHECK (execution_mode IN ('manual','agent','both')),
    required_capabilities JSONB NOT NULL DEFAULT '[]',
    business_rules_ref    JSONB NOT NULL DEFAULT '[]',
    status                TEXT NOT NULL DEFAULT 'draft'
                          CHECK (status IN ('draft','active','deprecated')),
    origin                TEXT NOT NULL DEFAULT 'human'
                          CHECK (origin IN ('ai','human')),
    source_refs           JSONB NOT NULL DEFAULT '{}',
    generation_job_id     UUID,
    version               INT  NOT NULL DEFAULT 1,
    created_by            UUID,
    updated_by            UUID,
    reviewed_by           UUID,
    reviewed_at           TIMESTAMPTZ,
    created_at            TIMESTAMPTZ NOT NULL DEFAULT now(),
    updated_at            TIMESTAMPTZ NOT NULL DEFAULT now()
);

-- Snapshot of the case as it was BEFORE each change, so review is reversible.
CREATE TABLE test_case_revision (
    id              UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    workspace_id    UUID NOT NULL,
    test_case_id    UUID NOT NULL,
    version         INT  NOT NULL,
    snapshot        JSONB NOT NULL,
    change_kind     TEXT NOT NULL
                    CHECK (change_kind IN ('human_edit','proposal_accepted','status_change','restore')),
    changed_by      UUID,
    changed_by_type TEXT NOT NULL DEFAULT 'member'
                    CHECK (changed_by_type IN ('member','agent')),
    note            TEXT NOT NULL DEFAULT '',
    created_at      TIMESTAMPTZ NOT NULL DEFAULT now()
);

-- Which repositories of the project a case touches. Bound to
-- project_resource.id (stable) rather than a repo URL (mutable).
CREATE TABLE test_case_repo (
    test_case_id        UUID NOT NULL,
    workspace_id        UUID NOT NULL,
    project_resource_id UUID NOT NULL,
    alias               TEXT NOT NULL,
    role                TEXT NOT NULL DEFAULT 'under_test'
                        CHECK (role IN ('under_test','driver','verifier','fixture')),
    path_globs          JSONB NOT NULL DEFAULT '[]',
    created_at          TIMESTAMPTZ NOT NULL DEFAULT now(),
    PRIMARY KEY (test_case_id, project_resource_id, role)
);

-- Human-readable case key TC-<n>, allocated per workspace exactly like
-- workspace.issue_counter (migration 020).
ALTER TABLE workspace ADD COLUMN test_case_counter INT NOT NULL DEFAULT 0;
```

`server/migrations/280_test_case.down.sql`：

```sql
ALTER TABLE workspace DROP COLUMN IF EXISTS test_case_counter;
DROP TABLE IF EXISTS test_case_repo;
DROP TABLE IF EXISTS test_case_revision;
DROP TABLE IF EXISTS test_case;
```

- [ ] **Step 3: 写七个索引迁移（各自单文件、单语句）**

每个 `.up.sql` 都以同一行注释开头。内容依次为：

`281_test_case_workspace_number_index.up.sql`
```sql
-- This is intentionally a single statement: concurrent index creation cannot
-- run in a transaction or a multi-command migration.
CREATE UNIQUE INDEX CONCURRENTLY IF NOT EXISTS test_case_workspace_number_idx
    ON test_case (workspace_id, case_number);
```

`282_test_case_project_status_index.up.sql`
```sql
-- This is intentionally a single statement: concurrent index creation cannot
-- run in a transaction or a multi-command migration.
CREATE INDEX CONCURRENTLY IF NOT EXISTS test_case_project_status_idx
    ON test_case (workspace_id, project_id, status);
```

`283_test_case_generation_job_index.up.sql`
```sql
-- This is intentionally a single statement: concurrent index creation cannot
-- run in a transaction or a multi-command migration.
CREATE INDEX CONCURRENTLY IF NOT EXISTS test_case_generation_job_idx
    ON test_case (workspace_id, generation_job_id);
```

`284_test_case_revision_case_index.up.sql`
```sql
-- This is intentionally a single statement: concurrent index creation cannot
-- run in a transaction or a multi-command migration.
CREATE INDEX CONCURRENTLY IF NOT EXISTS test_case_revision_case_idx
    ON test_case_revision (test_case_id, version DESC);
```

`285_test_case_repo_case_index.up.sql`
```sql
-- This is intentionally a single statement: concurrent index creation cannot
-- run in a transaction or a multi-command migration.
CREATE INDEX CONCURRENTLY IF NOT EXISTS test_case_repo_case_idx
    ON test_case_repo (test_case_id);
```

`286_test_case_repo_resource_index.up.sql`
```sql
-- This is intentionally a single statement: concurrent index creation cannot
-- run in a transaction or a multi-command migration.
CREATE INDEX CONCURRENTLY IF NOT EXISTS test_case_repo_resource_idx
    ON test_case_repo (workspace_id, project_resource_id);
```

`287_test_case_module_index.up.sql`
```sql
-- This is intentionally a single statement: concurrent index creation cannot
-- run in a transaction or a multi-command migration.
CREATE INDEX CONCURRENTLY IF NOT EXISTS test_case_module_idx
    ON test_case (workspace_id, project_id, module);
```

每个对应的 `.down.sql` 是单行 `DROP INDEX CONCURRENTLY IF EXISTS <name>;`。

- [ ] **Step 4: 跑迁移验证**

```bash
make server
```

Expected: 服务启动日志里迁移全部成功，无 `migration failed`。若数据库未起，先 `make dev`。

- [ ] **Step 5: 提交**

```bash
git add server/migrations
git commit -m "feat(testing): add test case library tables and indexes"
```

---

## Task 2: sqlc 查询

**Files:**
- Create: `server/pkg/db/queries/test_case.sql`
- Modify: `server/pkg/db/queries/workspace.sql`（在 `IncrementIssueCounter` 之后追加）

**Interfaces:**
- Consumes: Task 1 的表与列。
- Produces: 生成的 Go 方法 `ListTestCases`、`GetTestCaseInWorkspace`、`GetTestCaseByNumber`、`CreateTestCase`、`UpdateTestCase`、`DeleteTestCase`、`ListTestCaseRepos`、`ListTestCaseReposForCases`、`ReplaceTestCaseRepos`（由 `DeleteTestCaseRepos` + `CreateTestCaseRepo` 组合）、`CreateTestCaseRevision`、`ListTestCaseRevisions`、`IncrementTestCaseCounter`，以及类型 `db.TestCase`、`db.TestCaseRepo`、`db.TestCaseRevision`。

- [ ] **Step 1: 写 `server/pkg/db/queries/test_case.sql`**

```sql
-- name: ListTestCases :many
SELECT * FROM test_case
WHERE workspace_id = $1
  AND (sqlc.narg('project_id')::uuid IS NULL OR project_id = sqlc.narg('project_id'))
  AND (sqlc.narg('status')::text IS NULL OR status = sqlc.narg('status'))
  AND (sqlc.narg('module')::text IS NULL OR module = sqlc.narg('module'))
  AND (sqlc.narg('priority')::text IS NULL OR priority = sqlc.narg('priority'))
  AND (sqlc.narg('case_type')::text IS NULL OR case_type = sqlc.narg('case_type'))
  AND (sqlc.narg('origin')::text IS NULL OR origin = sqlc.narg('origin'))
ORDER BY case_number DESC;

-- name: GetTestCaseInWorkspace :one
SELECT * FROM test_case
WHERE id = $1 AND workspace_id = $2;

-- name: GetTestCaseByNumber :one
SELECT * FROM test_case
WHERE workspace_id = $1 AND case_number = $2;

-- name: ListTestCaseModules :many
SELECT module, count(*)::bigint AS case_count
FROM test_case
WHERE workspace_id = $1 AND project_id = $2
GROUP BY module
ORDER BY module ASC;

-- name: CreateTestCase :one
INSERT INTO test_case (
    workspace_id, project_id, case_number, title, module, preconditions,
    steps, expected_result, test_data, priority, case_type, scope,
    execution_mode, required_capabilities, business_rules_ref, status,
    origin, source_refs, generation_job_id, created_by, updated_by
) VALUES (
    $1, $2, $3, $4, $5, $6, $7, $8, $9, $10, $11, $12,
    $13, $14, $15, $16, $17, $18, $19, $20, $21
) RETURNING *;

-- name: UpdateTestCase :one
UPDATE test_case SET
    title                 = COALESCE(sqlc.narg('title'), title),
    module                = COALESCE(sqlc.narg('module'), module),
    preconditions         = COALESCE(sqlc.narg('preconditions'), preconditions),
    steps                 = COALESCE(sqlc.narg('steps'), steps),
    expected_result       = COALESCE(sqlc.narg('expected_result'), expected_result),
    test_data             = COALESCE(sqlc.narg('test_data'), test_data),
    priority              = COALESCE(sqlc.narg('priority'), priority),
    case_type             = COALESCE(sqlc.narg('case_type'), case_type),
    scope                 = COALESCE(sqlc.narg('scope'), scope),
    execution_mode        = COALESCE(sqlc.narg('execution_mode'), execution_mode),
    required_capabilities = COALESCE(sqlc.narg('required_capabilities'), required_capabilities),
    business_rules_ref    = COALESCE(sqlc.narg('business_rules_ref'), business_rules_ref),
    status                = COALESCE(sqlc.narg('status'), status),
    reviewed_by           = COALESCE(sqlc.narg('reviewed_by'), reviewed_by),
    reviewed_at           = COALESCE(sqlc.narg('reviewed_at'), reviewed_at),
    updated_by            = sqlc.narg('updated_by'),
    version               = version + 1,
    updated_at            = now()
WHERE id = $1 AND workspace_id = $2
RETURNING *;

-- name: DeleteTestCase :exec
-- Defense-in-depth: workspace_id is a SQL-layer tenant guard. See DeleteProject.
DELETE FROM test_case WHERE id = $1 AND workspace_id = $2;

-- name: ListTestCaseRepos :many
SELECT * FROM test_case_repo
WHERE test_case_id = $1
ORDER BY alias ASC, role ASC;

-- name: ListTestCaseReposForCases :many
SELECT * FROM test_case_repo
WHERE test_case_id = ANY(sqlc.arg('case_ids')::uuid[])
ORDER BY test_case_id, alias ASC, role ASC;

-- name: CreateTestCaseRepo :one
INSERT INTO test_case_repo (
    test_case_id, workspace_id, project_resource_id, alias, role, path_globs
) VALUES ($1, $2, $3, $4, $5, $6)
RETURNING *;

-- name: DeleteTestCaseRepos :exec
DELETE FROM test_case_repo WHERE test_case_id = $1 AND workspace_id = $2;

-- name: CreateTestCaseRevision :one
INSERT INTO test_case_revision (
    workspace_id, test_case_id, version, snapshot, change_kind,
    changed_by, changed_by_type, note
) VALUES ($1, $2, $3, $4, $5, $6, $7, $8)
RETURNING *;

-- name: ListTestCaseRevisions :many
SELECT * FROM test_case_revision
WHERE test_case_id = $1 AND workspace_id = $2
ORDER BY version DESC
LIMIT $3;
```

- [ ] **Step 2: 在 `server/pkg/db/queries/workspace.sql` 的 `IncrementIssueCounter` 之后追加**

```sql
-- name: IncrementTestCaseCounter :one
-- Mirrors IncrementIssueCounter: takes the workspace row lock so concurrent
-- creates in the same workspace cannot allocate the same case number.
UPDATE workspace SET test_case_counter = test_case_counter + 1
WHERE id = $1
RETURNING test_case_counter;
```

- [ ] **Step 3: 生成代码**

```bash
make sqlc
```

Expected: 无错误；`server/pkg/db/generated/` 下出现 `TestCase` / `TestCaseRepo` / `TestCaseRevision` 结构体。

- [ ] **Step 4: 编译验证**

```bash
cd server && go build ./... && go vet ./pkg/db/...
```

Expected: 无输出。

- [ ] **Step 5: 提交**

```bash
git add server/pkg/db server/migrations
git commit -m "feat(testing): add test case sqlc queries"
```

---

## Task 3: `TC-42` 引用解析

**Files:**
- Create: `server/internal/handler/test_case_ref.go`
- Create: `server/internal/handler/test_case_ref_test.go`

**Interfaces:**
- Consumes: `db.GetTestCaseInWorkspaceParams`、`db.GetTestCaseByNumberParams`（Task 2）。
- Produces:
  - `func parseTestCaseNumber(s string) (int32, bool)` —— `"TC-42"` → `(42, true)`；`"tc-42"` 同样接受；其他返回 `(0, false)`。
  - `func (h *Handler) loadTestCaseForUser(w http.ResponseWriter, r *http.Request, ref string) (db.TestCase, bool)` —— 解析 `TC-42` 或 UUID，取不到写 404 并返回 `ok=false`。后续所有按 id 的 handler 都用它。
  - `func formatTestCaseKey(n int32) string` —— `42` → `"TC-42"`。

- [ ] **Step 1: 写失败测试**

`server/internal/handler/test_case_ref_test.go`：

```go
package handler

import "testing"

func TestParseTestCaseNumber(t *testing.T) {
	cases := []struct {
		in   string
		want int32
		ok   bool
	}{
		{"TC-42", 42, true},
		{"tc-42", 42, true},
		{"  TC-7  ", 7, true},
		{"TC-0", 0, false},
		{"TC--1", 0, false},
		{"TC-", 0, false},
		{"42", 0, false},
		{"MUL-42", 0, false},
		{"", 0, false},
		{"00000000-0000-0000-0000-000000000000", 0, false},
	}
	for _, c := range cases {
		got, ok := parseTestCaseNumber(c.in)
		if got != c.want || ok != c.ok {
			t.Errorf("parseTestCaseNumber(%q) = (%d, %v), want (%d, %v)", c.in, got, ok, c.want, c.ok)
		}
	}
}

func TestFormatTestCaseKey(t *testing.T) {
	if got := formatTestCaseKey(42); got != "TC-42" {
		t.Errorf("formatTestCaseKey(42) = %q, want \"TC-42\"", got)
	}
}
```

- [ ] **Step 2: 跑测试确认失败**

```bash
cd server && go test ./internal/handler -run TestParseTestCaseNumber -count=1
```

Expected: 编译失败，`undefined: parseTestCaseNumber`。

- [ ] **Step 3: 实现**

`server/internal/handler/test_case_ref.go`：

```go
package handler

import (
	"fmt"
	"net/http"
	"strconv"
	"strings"

	db "github.com/multica-ai/multica/server/pkg/db/generated"
)

// testCaseKeyPrefix is the human-readable key namespace for test cases.
// Unlike issues, the prefix is fixed rather than workspace-configurable:
// test case keys are only ever resolved inside an already workspace-scoped
// request, so a per-workspace prefix would add ambiguity without adding
// disambiguation.
const testCaseKeyPrefix = "TC-"

// formatTestCaseKey renders the human-readable key for a case number.
func formatTestCaseKey(number int32) string {
	return fmt.Sprintf("%s%d", testCaseKeyPrefix, number)
}

// parseTestCaseNumber accepts "TC-42" (case-insensitive, surrounding space
// tolerated) and returns the case number. Anything else — a bare number, a
// UUID, another prefix — returns ok=false so the caller falls through to UUID
// resolution.
func parseTestCaseNumber(ref string) (int32, bool) {
	trimmed := strings.TrimSpace(ref)
	if len(trimmed) <= len(testCaseKeyPrefix) {
		return 0, false
	}
	if !strings.EqualFold(trimmed[:len(testCaseKeyPrefix)], testCaseKeyPrefix) {
		return 0, false
	}
	n, err := strconv.Atoi(trimmed[len(testCaseKeyPrefix):])
	if err != nil || n <= 0 {
		return 0, false
	}
	return int32(n), true
}

// loadTestCaseForUser resolves a path param that may be either a TC-42 key or
// a UUID into the owning workspace's test case. Per the repository UUID rule,
// every write that follows must use the returned entity.ID, never the raw ref.
func (h *Handler) loadTestCaseForUser(w http.ResponseWriter, r *http.Request, ref string) (db.TestCase, bool) {
	workspaceID := h.resolveWorkspaceID(r)
	wsUUID, ok := parseUUIDOrBadRequest(w, workspaceID, "workspace_id")
	if !ok {
		return db.TestCase{}, false
	}
	if number, isKey := parseTestCaseNumber(ref); isKey {
		testCase, err := h.Queries.GetTestCaseByNumber(r.Context(), db.GetTestCaseByNumberParams{
			WorkspaceID: wsUUID,
			CaseNumber:  number,
		})
		if err != nil {
			writeError(w, http.StatusNotFound, "test case not found")
			return db.TestCase{}, false
		}
		return testCase, true
	}
	idUUID, ok := parseUUIDOrBadRequest(w, ref, "test case id")
	if !ok {
		return db.TestCase{}, false
	}
	testCase, err := h.Queries.GetTestCaseInWorkspace(r.Context(), db.GetTestCaseInWorkspaceParams{
		ID:          idUUID,
		WorkspaceID: wsUUID,
	})
	if err != nil {
		writeError(w, http.StatusNotFound, "test case not found")
		return db.TestCase{}, false
	}
	return testCase, true
}
```

- [ ] **Step 4: 跑测试确认通过**

```bash
cd server && go test ./internal/handler -run 'TestParseTestCaseNumber|TestFormatTestCaseKey' -count=1
```

Expected: `ok`。

- [ ] **Step 5: 提交**

```bash
git add server/internal/handler/test_case_ref.go server/internal/handler/test_case_ref_test.go
git commit -m "feat(testing): resolve TC-<n> test case references"
```

---

## Task 4: 用例 CRUD handler

**Files:**
- Create: `server/internal/handler/test_case.go`
- Create: `server/internal/handler/test_case_test.go`
- Modify: `server/pkg/protocol/events.go`（在 Project events 块之后）

**Interfaces:**
- Consumes: Task 2 的 sqlc 方法；Task 3 的 `loadTestCaseForUser` / `formatTestCaseKey`。
- Produces:
  - `type TestCaseStep struct { Index int32; Action, Expected, Repo string }`（JSON tag `index` / `action` / `expected` / `repo`）
  - `type TestCaseRepoResponse struct { ProjectResourceID, Alias, Role string; PathGlobs []string }`
  - `type TestCaseResponse struct { ... Key string ... }`（`key` 是 `TC-42`）
  - `func testCaseToResponse(c db.TestCase, repos []db.TestCaseRepo) TestCaseResponse`
  - handler 方法：`ListTestCases`、`GetTestCase`、`CreateTestCase`、`UpdateTestCase`、`DeleteTestCase`、`ApproveTestCase`、`ListTestCaseRevisions`、`ListTestCaseModules`
  - 事件常量 `protocol.EventTestCaseCreated` / `Updated` / `Deleted`

- [ ] **Step 1: 加事件常量**

`server/pkg/protocol/events.go`，在 `EventProjectResourceDeleted` 之后：

```go
	// Test case events
	EventTestCaseCreated = "test_case:created"
	EventTestCaseUpdated = "test_case:updated"
	EventTestCaseDeleted = "test_case:deleted"
```

- [ ] **Step 2: 写失败测试**

`server/internal/handler/test_case_test.go`。仓库里 handler 测试跑在真实测试库上（见同目录既有测试如何取 `testHandler` / `testWorkspaceID` / `newRequest`），照抄同样的夹具风格。核心用例：

```go
package handler

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

func TestCreateTestCaseAllocatesSequentialKeys(t *testing.T) {
	body := `{"project_id":"` + testProjectID + `","title":"下单成功","steps":[{"index":1,"action":"点击下单","expected":"跳转支付页"}]}`
	w := httptest.NewRecorder()
	testHandler.CreateTestCase(w, newRequest("POST", "/api/test-cases?workspace_id="+testWorkspaceID, strings.NewReader(body)))
	if w.Code != http.StatusCreated {
		t.Fatalf("status = %d, want 201: %s", w.Code, w.Body.String())
	}
	var first TestCaseResponse
	if err := json.NewDecoder(w.Body).Decode(&first); err != nil {
		t.Fatalf("decode: %v", err)
	}
	if !strings.HasPrefix(first.Key, "TC-") {
		t.Fatalf("key = %q, want TC- prefix", first.Key)
	}
	if first.Status != "draft" && first.Status != "active" {
		t.Fatalf("unexpected status %q", first.Status)
	}

	w2 := httptest.NewRecorder()
	testHandler.CreateTestCase(w2, newRequest("POST", "/api/test-cases?workspace_id="+testWorkspaceID, strings.NewReader(body)))
	var second TestCaseResponse
	_ = json.NewDecoder(w2.Body).Decode(&second)
	if second.CaseNumber != first.CaseNumber+1 {
		t.Fatalf("case_number = %d, want %d", second.CaseNumber, first.CaseNumber+1)
	}
}

func TestCreateTestCaseRejectsUnknownPriority(t *testing.T) {
	body := `{"project_id":"` + testProjectID + `","title":"x","priority":"urgent"}`
	w := httptest.NewRecorder()
	testHandler.CreateTestCase(w, newRequest("POST", "/api/test-cases?workspace_id="+testWorkspaceID, strings.NewReader(body)))
	if w.Code != http.StatusBadRequest {
		t.Fatalf("status = %d, want 400", w.Code)
	}
	if !strings.Contains(w.Body.String(), "p0") {
		t.Fatalf("error should list valid values, got %s", w.Body.String())
	}
}

func TestUpdateTestCaseWritesRevisionSnapshot(t *testing.T) {
	// create, then update the title, then assert a revision row exists whose
	// snapshot holds the OLD title and whose version is the pre-update version.
}

func TestDeleteTestCaseRemovesRepoBindings(t *testing.T) {
	// create with repos, delete, assert test_case_repo has no rows for the id.
}

func TestGetTestCaseAcceptsKeyAndUUID(t *testing.T) {
	// GET /{TC-n} and GET /{uuid} return the same case.
}
```

`origin` 与人工创建的关系（spec §2.1）：任何非生成任务的创建路径都记 `human`，`status` 默认 `active`；生成任务写入时才是 `ai` + `draft`。第一期没有生成任务，所以 `CreateTestCase` 恒为 `human` + `active`，`ApproveTestCase` 在第一期只对被手工改成 `draft` 的用例有意义 —— 保留端点，第二期才会大量使用。

- [ ] **Step 3: 跑测试确认失败**

```bash
cd server && go test ./internal/handler -run TestCreateTestCase -count=1
```

Expected: 编译失败，`undefined: TestCaseResponse` 等。

- [ ] **Step 4: 实现 handler**

`server/internal/handler/test_case.go` 要点（照抄 `server/internal/handler/project.go` 的结构）：

1. **响应类型与转换**

```go
type TestCaseStep struct {
	Index    int32  `json:"index"`
	Action   string `json:"action"`
	Expected string `json:"expected"`
	Repo     string `json:"repo,omitempty"`
}

type TestCaseRepoResponse struct {
	ProjectResourceID string   `json:"project_resource_id"`
	Alias             string   `json:"alias"`
	Role              string   `json:"role"`
	PathGlobs         []string `json:"path_globs"`
}

type TestCaseResponse struct {
	ID                   string                 `json:"id"`
	WorkspaceID          string                 `json:"workspace_id"`
	ProjectID            string                 `json:"project_id"`
	CaseNumber           int32                  `json:"case_number"`
	Key                  string                 `json:"key"`
	Title                string                 `json:"title"`
	Module               string                 `json:"module"`
	Preconditions        string                 `json:"preconditions"`
	Steps                []TestCaseStep         `json:"steps"`
	ExpectedResult       string                 `json:"expected_result"`
	TestData             map[string]any         `json:"test_data"`
	Priority             string                 `json:"priority"`
	CaseType             string                 `json:"case_type"`
	Scope                string                 `json:"scope"`
	ExecutionMode        string                 `json:"execution_mode"`
	RequiredCapabilities []map[string]any       `json:"required_capabilities"`
	BusinessRulesRef     []string               `json:"business_rules_ref"`
	Status               string                 `json:"status"`
	Origin               string                 `json:"origin"`
	SourceRefs           map[string]any         `json:"source_refs"`
	GenerationJobID      *string                `json:"generation_job_id"`
	Version              int32                  `json:"version"`
	Repos                []TestCaseRepoResponse `json:"repos"`
	CreatedBy            *string                `json:"created_by"`
	UpdatedBy            *string                `json:"updated_by"`
	ReviewedBy           *string                `json:"reviewed_by"`
	ReviewedAt           *string                `json:"reviewed_at"`
	CreatedAt            string                 `json:"created_at"`
	UpdatedAt            string                 `json:"updated_at"`
}
```

`testCaseToResponse` 用 `json.Unmarshal` 解 JSONB 列；解析失败时**填零值而不是报错**（列有 DEFAULT，且下游 UI 已按防御式写法处理），并 `slog.Warn` 记一条。`Repos` 恒为非 nil 切片（`emit_empty_slices` 已开，保持 JSON 里是 `[]` 而不是 `null`）。

2. **枚举校验**：照抄 `validateProjectEnum`，定义 `validTestCasePriorities`、`validTestCaseTypes`、`validTestCaseScopes`、`validTestCaseExecutionModes`、`validTestCaseStatuses`、`validTestCaseRepoRoles`，在 create/update 里预校验，返回 400 并列出合法值。

3. **`CreateTestCase`**：解析 body → 校验 `title` 非空、`project_id` 必填 → `GetProjectInWorkspace` 校验项目属于本工作区（应用层替代外键）→ 校验每条 repo 的 `project_resource_id` 通过 `GetProjectResourceInWorkspace` 且 `resource.ProjectID == project.ID` → 开事务 `h.TxStarter.Begin` → `qtx.IncrementTestCaseCounter` 拿号 → `qtx.CreateTestCase` → 逐条 `qtx.CreateTestCaseRepo` → commit → `h.publish(protocol.EventTestCaseCreated, workspaceID, "member", userID, map[string]any{"test_case": resp})` → 201。

4. **`UpdateTestCase`**：`loadTestCaseForUser` 取到实体 → 先用**当前**实体构造 snapshot 并 `CreateTestCaseRevision(version = current.Version, change_kind = "human_edit")` → 再 `UpdateTestCase`（`version` 由 SQL 自增）→ 若 body 带 `repos`，在同一事务里 `DeleteTestCaseRepos` + 重建 → publish `Updated`。快照与更新必须同一事务，否则崩在中间会留下没有对应变更的快照。

5. **`ApproveTestCase`**：`draft → active`，写 `change_kind = "status_change"` 的快照，落 `reviewed_by` / `reviewed_at`，publish `Updated`。已是 `active` 时返回 409 `test case is already active`。

6. **`DeleteTestCase`**：事务内 `DeleteTestCaseRepos` → 删除该用例的 revision 行（新增 `DeleteTestCaseRevisions` 查询）→ `DeleteTestCase` → publish `Deleted`，payload 是 `{"test_case_id": uuidToString(id)}`。

7. **`ListTestCases`**：读 query 参数 `project_id` / `status` / `module` / `priority` / `case_type` / `origin` 组装 narg → `ListTestCases` → 用 `ListTestCaseReposForCases` 批量取关联仓库（**不要 N+1**）→ `writeJSON(w, 200, map[string]any{"test_cases": resp, "total": len(resp)})`。

8. **`ListTestCaseModules`**：返回 `{"modules": [{"module": "订单", "case_count": 12}]}`，给列表页左侧分组树用。

9. **`ListTestCaseRevisions`**：`limit` 默认 50，上限 200。

10. **写错误映射**：照抄 `writeProjectWriteError` 加一个 `writeTestCaseWriteError`，`isCheckViolation` → 400，其余 → `slog.Error` + 500。

- [ ] **Step 5: 跑测试确认通过**

```bash
cd server && go test ./internal/handler -run TestCase -count=1
```

Expected: `ok`。

- [ ] **Step 6: 提交**

```bash
git add server/internal/handler/test_case.go server/internal/handler/test_case_test.go server/pkg/protocol/events.go
git commit -m "feat(testing): add test case CRUD handlers"
```

---

## Task 5: 路由注册

**Files:**
- Modify: `server/cmd/server/router.go`（紧跟 `// Projects` 路由组之后）

**Interfaces:**
- Consumes: Task 4 的 handler 方法。
- Produces: HTTP 端点，供 Task 6 的 CLI 与 Task 9 的前端客户端使用。

- [ ] **Step 1: 加路由组**

```go
			// Test cases
			r.Route("/api/test-cases", func(r chi.Router) {
				// Literal sub-paths must be registered before the {ref} route.
				r.Get("/modules", h.ListTestCaseModules)
				r.Get("/", h.ListTestCases)
				r.Post("/", h.CreateTestCase)
				r.Route("/{ref}", func(r chi.Router) {
					r.Get("/", h.GetTestCase)
					r.Put("/", h.UpdateTestCase)
					r.Delete("/", h.DeleteTestCase)
					r.Post("/approve", h.ApproveTestCase)
					r.Get("/revisions", h.ListTestCaseRevisions)
				})
			})
```

`{ref}` 而不是 `{id}`：它同时接受 `TC-42` 与 UUID，handler 里用 `chi.URLParam(r, "ref")` 取。

- [ ] **Step 2: 编译并冒烟**

```bash
cd server && go build ./... && go vet ./cmd/server/...
```

Expected: 无输出。

- [ ] **Step 3: 手工冒烟（可选但推荐）**

```bash
make server
```

另一个终端：

```bash
curl -s "http://localhost:8080/api/test-cases?workspace_id=<ws-uuid>" -H "Authorization: Bearer <token>" | head -c 200
```

Expected: `{"test_cases":[],"total":0}`。

- [ ] **Step 4: 提交**

```bash
git add server/cmd/server/router.go
git commit -m "feat(testing): register test case routes"
```

---

## Task 6: `multica testcase` CLI

**Files:**
- Create: `server/cmd/multica/cmd_testcase.go`
- Create: `server/cmd/multica/cmd_testcase_test.go`
- Modify: `server/cmd/multica/main.go`

**Interfaces:**
- Consumes: Task 5 的端点。
- Produces: `multica testcase list|get|modules|create|update|delete`。第一期智能体侧只承诺**只读**（`list` / `get` / `modules`），写命令供人在终端用。

- [ ] **Step 1: 写失败测试**

`server/cmd/multica/cmd_testcase_test.go`，照抄 `cmd_project_test.go` 的 `newXxxTestCmd()` 风格 —— 构造一个只带 RunE 会读到的 flag 的裸 `cobra.Command`，用 `httptest` 假服务端断言请求 URL 与输出：

```go
func TestTestcaseListPassesFilters(t *testing.T) {
	// assert GET /api/test-cases?workspace_id=…&project_id=…&status=active
}

func TestTestcaseListJSONOutput(t *testing.T) {
	// --output json prints the raw array, not the table
}

func TestTestcaseGetAcceptsKey(t *testing.T) {
	// `multica testcase get TC-42` hits /api/test-cases/TC-42 verbatim
	// (server-side resolution — do NOT prefix-resolve TC keys client-side)
}

func TestTestcaseDigestOmitsSteps(t *testing.T) {
	// --digest strips steps/test_data from each row so a generation brief
	// can list hundreds of cases cheaply
}
```

- [ ] **Step 2: 跑测试确认失败**

```bash
cd server && go test ./cmd/multica -run TestTestcase -count=1
```

Expected: 编译失败。

- [ ] **Step 3: 实现**

`server/cmd/multica/cmd_testcase.go`，命令定义：

```go
var testcaseCmd = &cobra.Command{Use: "testcase", Short: "Work with test cases"}

var testcaseListCmd = &cobra.Command{
	Use:   "list",
	Short: "List test cases in the workspace",
	RunE:  runTestcaseList,
}
var testcaseGetCmd = &cobra.Command{
	Use:   "get <TC-42|id>",
	Short: "Get a test case, including its related repositories",
	Args:  exactArgs(1),
	RunE:  runTestcaseGet,
}
var testcaseModulesCmd = &cobra.Command{
	Use:   "modules",
	Short: "List modules and case counts for a project",
	RunE:  runTestcaseModules,
}
var testcaseCreateCmd = &cobra.Command{Use: "create", Short: "Create a test case", RunE: runTestcaseCreate}
var testcaseUpdateCmd = &cobra.Command{Use: "update <TC-42|id>", Short: "Update a test case", Args: exactArgs(1), RunE: runTestcaseUpdate}
var testcaseDeleteCmd = &cobra.Command{Use: "delete <TC-42|id>", Short: "Delete a test case", Args: exactArgs(1), RunE: runTestcaseDelete}

func init() {
	testcaseCmd.AddCommand(testcaseListCmd, testcaseGetCmd, testcaseModulesCmd,
		testcaseCreateCmd, testcaseUpdateCmd, testcaseDeleteCmd)

	testcaseListCmd.Flags().String("project", "", "Filter by project id")
	testcaseListCmd.Flags().String("status", "", "Filter by status: draft, active, deprecated")
	testcaseListCmd.Flags().String("module", "", "Filter by module")
	testcaseListCmd.Flags().String("priority", "", "Filter by priority: p0, p1, p2, p3")
	testcaseListCmd.Flags().String("case-type", "", "Filter by case type")
	testcaseListCmd.Flags().String("origin", "", "Filter by origin: ai, human")
	testcaseListCmd.Flags().Bool("digest", false, "Omit steps and test data — a compact index for agent context")
	testcaseListCmd.Flags().String("output", "table", "Output format: table or json")
	testcaseListCmd.Flags().Bool("full-id", false, "Show full UUIDs in table output")

	testcaseGetCmd.Flags().String("output", "json", "Output format: table or json")

	testcaseModulesCmd.Flags().String("project", "", "Project id (required)")
	testcaseModulesCmd.Flags().String("output", "table", "Output format: table or json")

	testcaseCreateCmd.Flags().String("project", "", "Project id (required)")
	testcaseCreateCmd.Flags().String("title", "", "Test case title (required)")
	testcaseCreateCmd.Flags().String("module", "", "Module for grouping")
	testcaseCreateCmd.Flags().String("priority", "p2", "Priority: p0, p1, p2, p3")
	testcaseCreateCmd.Flags().String("case-type", "functional", "Case type")
	testcaseCreateCmd.Flags().String("preconditions", "", "Preconditions (decodes \\n, \\r, \\t, \\\\; pipe via --preconditions-stdin to preserve literal backslashes)")
	testcaseCreateCmd.Flags().String("steps", "", "Steps as a JSON array: [{\"index\":1,\"action\":\"…\",\"expected\":\"…\"}]")
	testcaseCreateCmd.Flags().String("output", "json", "Output format: table or json")
	// update / delete flags mirror create; update gates every field on Changed().
}
```

RunE 的骨架照抄 `runProjectList`：`newAPIClient(cmd)` → `cli.APIContext` → `requireWorkspaceID` → `url.Values` 组装 → `client.GetJSON` → `--output json` 走 `cli.PrintJSON`，否则 `cli.PrintTable`。

要点：
- `--preconditions` 走 `resolveTextFlag(cmd, "preconditions")`，它自动派生 `--preconditions-stdin` / `--preconditions-file` 并强制 `ensureFileFlagWithinWorkdir`。
- `--steps` 是单个 JSON 字符串 flag，用 `json.Unmarshal` 校验，失败返回 `--steps must be valid JSON: %w`。**不要**发明 NDJSON。
- `get` 把 ref 原样拼进 URL（`/api/test-cases/` + `url.PathEscape(ref)`），服务端解析 `TC-42`。**不要**在客户端做前缀解析 —— 用例数量可能很大，与 issue 放弃短前缀是同一个理由。
- `update` 的每个字段都 `cmd.Flags().Changed(name)` 门控，body 为空时返回 `no fields to update; use flags like --title, --status`。
- 表格列：`KEY` / `TITLE` / `MODULE` / `TYPE` / `PRIO` / `STATUS` / `ORIGIN` / `REPOS`（repos 展示为 `alias(role)` 逗号连接）。

`server/cmd/multica/main.go` 两行：在 `// Core commands` 块加 `testcaseCmd.GroupID = groupCore`，并在 `projectCmd` 附近加 `rootCmd.AddCommand(testcaseCmd)`。

- [ ] **Step 4: 跑测试确认通过**

```bash
cd server && go test ./cmd/multica -run TestTestcase -count=1
```

Expected: `ok`。

- [ ] **Step 5: 全量后端检查**

```bash
cd server && go build ./... && go vet ./... && go test ./internal/handler ./cmd/multica -count=1
```

Expected: 全部 `ok`。

- [ ] **Step 6: 提交**

```bash
git add server/cmd/multica
git commit -m "feat(testing): add multica testcase CLI commands"
```

---

## Task 7: 内置技能 `multica-test-cases`

**Files:**
- Create: `server/internal/service/builtin_skills/multica-test-cases/SKILL.md`
- Create: `server/internal/service/builtin_skills/multica-test-cases/references/test-cases-source-map.md`
- Modify: `server/internal/service/builtin_skills_test.go`

**Interfaces:**
- Consumes: Task 6 的 CLI 命令形状、Task 4 的字段语义。
- Produces: 智能体侧的用例数据契约。目录建好即注册（`loadBuiltinSkills` 读嵌入目录），无需改注册代码。

- [ ] **Step 1: 写 SKILL.md**

frontmatter 严格照 `multica-mentioning` 的形状：

```yaml
---
name: multica-test-cases
description: "Use when reading, creating, or updating Multica test cases — including finding which repositories and issues a case relates to. Not for running a test: that is multica-running-tests."
user-invocable: false
allowed-tools: Bash(multica *)
---
```

约束：description 必须加引号（裸 `: ` 会破坏严格 YAML 运行时），≤300 字符，写成触发语 + 反向边界，而不是内容清单。正文 ≤500 行，只写可溯源的契约，不重复 runtime brief 已有的内容。

正文覆盖：
- 用例的字段语义与合法枚举值（与 `test_case` 的 CHECK 约束逐一对应）；
- `steps` 是结构化数组，不是 markdown；`steps[].repo` 引用 `repos[].alias`；
- `TC-42` 是人类可读 key，可直接传给 `multica testcase get`；
- `--digest` 用于低成本索引已有用例；
- `scope` 为 `cross_repo` 时必须至少两个 `repos` 条目且 role 不同；
- 明确说明第一期尚无生成任务与执行轮次，`multica test …` 命令组还不存在。

- [ ] **Step 2: 写 source map**

`references/test-cases-source-map.md`：一张 `Behavior | File:line` 表覆盖正文每条断言，底部一个 `## Verification command` 的 grep 块，让后来的智能体能重新推导行号。

- [ ] **Step 3: 加 eval 测试**

`server/internal/service/builtin_skills_test.go`，仿 `TestWorkingOnIssuesSkillCoversIssueLoopContracts`：

```go
func TestTestCasesSkillCoversDataContract(t *testing.T) {
	skill := loadBuiltinSkillForTest(t, "multica-test-cases")
	assertFrontmatter(t, skill, "user-invocable", "false")
	assertAllowedToolsContains(t, skill, "Bash(multica *)")
	mustContain := []string{
		"multica testcase list",
		"multica testcase get",
		"--digest",
		"cross_repo",
		"steps",
	}
	mustNotContain := []string{
		"multica repo checkout", // owned by the runtime brief
	}
	// … assert skillHasFile(skill, "references/test-cases-source-map.md")
}
```

（`loadBuiltinSkillForTest` / `assertFrontmatter` / `assertAllowedToolsContains` / `skillHasFile` 若同文件已有同名或近似辅助函数，直接复用现成的，不要新造。）

- [ ] **Step 4: 跑测试**

```bash
cd server && go test ./internal/service -run 'BuiltinSkills|TestCasesSkill' -count=1
```

Expected: `ok`。

- [ ] **Step 5: 提交**

```bash
git add server/internal/service/builtin_skills server/internal/service/builtin_skills_test.go
git commit -m "feat(testing): add multica-test-cases builtin skill"
```

---

## Task 8: 前端类型、schema 与 API 客户端

**Files:**
- Create: `packages/core/types/testing.ts`
- Modify: `packages/core/types/index.ts`, `packages/core/api/schemas.ts`, `packages/core/api/schemas.test.ts`, `packages/core/api/client.ts`, `packages/core/types/events.ts`

**Interfaces:**
- Consumes: Task 4 的 JSON 响应形状、Task 5 的端点路径。
- Produces:
  - 类型 `TestCaseStep`、`TestCaseRepo`、`TestCase`、`TestCaseRevision`、`TestCaseModule`、`ListTestCasesResponse`、`ListTestCaseModulesResponse`、`ListTestCaseRevisionsResponse`、`CreateTestCaseRequest`、`UpdateTestCaseRequest`
  - schema `TestCaseSchema`、`ListTestCasesResponseSchema`、… 与 `EMPTY_TEST_CASE`、`EMPTY_LIST_TEST_CASES_RESPONSE` 等
  - client 方法 `listTestCases(params)`、`getTestCase(ref)`、`createTestCase(data)`、`updateTestCase(ref, data)`、`deleteTestCase(ref)`、`approveTestCase(ref)`、`listTestCaseModules(projectId)`、`listTestCaseRevisions(ref)`

- [ ] **Step 1: 写类型**

`packages/core/types/testing.ts`。枚举用字符串联合，但 **schema 里保持 `z.string()`**（lenient 原则：后端新增枚举值不能让页面白屏）。

```ts
export type TestCasePriority = "p0" | "p1" | "p2" | "p3";
export type TestCaseStatus = "draft" | "active" | "deprecated";
export type TestCaseOrigin = "ai" | "human";
export type TestCaseScope = "single_repo" | "cross_repo" | "no_repo";
export type TestCaseExecutionMode = "manual" | "agent" | "both";
export type TestCaseRepoRole = "under_test" | "driver" | "verifier" | "fixture";

export interface TestCaseStep {
  index: number;
  action: string;
  expected: string;
  repo?: string;
}

export interface TestCaseRepo {
  project_resource_id: string;
  alias: string;
  role: TestCaseRepoRole;
  path_globs: string[];
}

export interface TestCase {
  id: string;
  workspace_id: string;
  project_id: string;
  case_number: number;
  key: string;
  title: string;
  module: string;
  preconditions: string;
  steps: TestCaseStep[];
  expected_result: string;
  test_data: Record<string, unknown>;
  priority: TestCasePriority;
  case_type: string;
  scope: TestCaseScope;
  execution_mode: TestCaseExecutionMode;
  required_capabilities: Record<string, unknown>[];
  business_rules_ref: string[];
  status: TestCaseStatus;
  origin: TestCaseOrigin;
  source_refs: Record<string, unknown>;
  generation_job_id: string | null;
  version: number;
  repos: TestCaseRepo[];
  created_by: string | null;
  updated_by: string | null;
  reviewed_by: string | null;
  reviewed_at: string | null;
  created_at: string;
  updated_at: string;
}
```

在 `packages/core/types/index.ts` 加 `export * from "./testing";`；在 `packages/core/types/events.ts` 的 `WSEventType` 联合里加 `"test_case:created" | "test_case:updated" | "test_case:deleted"`。

- [ ] **Step 2: 写 schema 与畸形响应测试（先写测试）**

`packages/core/api/schemas.test.ts` 追加：

```ts
describe("TestCaseSchema", () => {
  it("fills defaults when the backend omits fields", () => {
    const parsed = parseWithFallback({ id: "c1", title: "下单" }, TestCaseSchema, EMPTY_TEST_CASE, {
      endpoint: "test",
    });
    expect(parsed.steps).toEqual([]);
    expect(parsed.repos).toEqual([]);
    expect(parsed.status).toBe("draft");
  });

  it("falls back when the payload is not an object", () => {
    const parsed = parseWithFallback("nope", TestCaseSchema, EMPTY_TEST_CASE, { endpoint: "test" });
    expect(parsed).toBe(EMPTY_TEST_CASE);
  });

  it("keeps an unknown status rather than dropping the case", () => {
    const parsed = parseWithFallback({ id: "c1", status: "quarantined" }, TestCaseSchema, EMPTY_TEST_CASE, {
      endpoint: "test",
    });
    expect(parsed.status).toBe("quarantined");
  });

  it("recovers a list response missing the array", () => {
    const parsed = parseWithFallback({}, ListTestCasesResponseSchema, EMPTY_LIST_TEST_CASES_RESPONSE, {
      endpoint: "test",
    });
    expect(parsed.test_cases).toEqual([]);
    expect(parsed.total).toBe(0);
  });
});
```

- [ ] **Step 3: 跑测试确认失败**

```bash
pnpm --filter @multica/core test -- schemas.test.ts
```

Expected: `TestCaseSchema is not defined`。

- [ ] **Step 4: 实现 schema 与 client**

`packages/core/api/schemas.ts`（放在 `ProjectSchema` 附近）：每个字段带 `.default(...)`，枚举保持 `z.string()`，对象用 `.loose()`。同时定义 `EMPTY_TEST_CASE` 等常量。

`packages/core/api/client.ts` 加 `// Test cases` 方法块，每个都走 `parseWithFallback`：

```ts
  async listTestCases(params?: {
    projectId?: string;
    status?: string;
    module?: string;
    priority?: string;
    caseType?: string;
    origin?: string;
  }): Promise<ListTestCasesResponse> {
    const search = new URLSearchParams();
    if (params?.projectId) search.set("project_id", params.projectId);
    if (params?.status) search.set("status", params.status);
    if (params?.module) search.set("module", params.module);
    if (params?.priority) search.set("priority", params.priority);
    if (params?.caseType) search.set("case_type", params.caseType);
    if (params?.origin) search.set("origin", params.origin);
    const raw = await this.fetch<unknown>(`/api/test-cases?${search}`);
    return parseWithFallback(raw, ListTestCasesResponseSchema, EMPTY_LIST_TEST_CASES_RESPONSE, {
      endpoint: "GET /api/test-cases",
    });
  }

  async getTestCase(ref: string): Promise<TestCase> {
    const raw = await this.fetch<unknown>(`/api/test-cases/${encodeURIComponent(ref)}`);
    return parseWithFallback(raw, TestCaseSchema, { ...EMPTY_TEST_CASE, id: ref }, {
      endpoint: "GET /api/test-cases/:ref",
    });
  }
```

`create` / `update` / `approve` 同形；`delete` 返回 `void`。**每一个都要 parseWithFallback**，包括 `listTestCaseModules` 与 `listTestCaseRevisions` —— 不要重蹈 design_restore 里 plan 端点绕过 schema 的覆辙。

- [ ] **Step 5: 跑测试确认通过**

```bash
pnpm --filter @multica/core test -- schemas.test.ts
pnpm typecheck
```

Expected: 测试通过、typecheck 无错。

- [ ] **Step 6: 提交**

```bash
git add packages/core/types packages/core/api
git commit -m "feat(testing): add test case types, schemas and API client methods"
```

---

## Task 9: core `testing` 域（keys / queries / mutations / 实时）

**Files:**
- Create: `packages/core/testing/{keys,queries,mutations,config,index}.ts`
- Create: `packages/core/testing/{keys,mutations}.test.ts`
- Modify: `packages/core/package.json`, `packages/core/realtime/use-realtime-sync.ts`

**Interfaces:**
- Consumes: Task 8 的 client 方法与类型。
- Produces:
  - `testCaseKeys.all(wsId)` / `.list(wsId, filters?)` / `.detail(wsId, ref)` / `.modules(wsId, projectId)` / `.revisions(wsId, ref)`
  - `testCaseListOptions(wsId, filters)` / `testCaseDetailOptions(wsId, ref)` / `testCaseModulesOptions(wsId, projectId)` / `testCaseRevisionsOptions(wsId, ref)`
  - `useCreateTestCase()` / `useUpdateTestCase()` / `useDeleteTestCase()` / `useApproveTestCase()`
  - `TEST_CASE_PRIORITY_LABELS` / `TEST_CASE_TYPE_LABELS` / `TEST_CASE_STATUS_LABELS`（值是 i18n key 而不是中文字面量）

- [ ] **Step 1: 写 keys 测试**

```ts
import { describe, it, expect } from "vitest";
import { testCaseKeys } from "./keys";

describe("testCaseKeys", () => {
  it("scopes every key by workspace id", () => {
    expect(testCaseKeys.all("ws-1")[1]).toBe("ws-1");
    expect(testCaseKeys.list("ws-1")[1]).toBe("ws-1");
    expect(testCaseKeys.detail("ws-1", "TC-2")[1]).toBe("ws-1");
    expect(testCaseKeys.modules("ws-1", "p1")[1]).toBe("ws-1");
  });

  it("derives child keys from the parent key", () => {
    expect(testCaseKeys.list("ws-1").slice(0, 2)).toEqual(testCaseKeys.all("ws-1"));
  });

  it("separates lists with different filters", () => {
    expect(testCaseKeys.list("ws-1", { projectId: "a" })).not.toEqual(
      testCaseKeys.list("ws-1", { projectId: "b" }),
    );
  });
});
```

- [ ] **Step 2: 跑测试确认失败**

```bash
pnpm --filter @multica/core test -- testing/keys.test.ts
```

Expected: 模块不存在。

- [ ] **Step 3: 实现 keys / queries**

```ts
// packages/core/testing/keys.ts
export interface TestCaseListFilters {
  projectId?: string;
  status?: string;
  module?: string;
  priority?: string;
  caseType?: string;
  origin?: string;
}

export const testCaseKeys = {
  all: (wsId: string) => ["test-cases", wsId] as const,
  list: (wsId: string, filters: TestCaseListFilters = {}) =>
    [...testCaseKeys.all(wsId), "list", filters] as const,
  detail: (wsId: string, ref: string) => [...testCaseKeys.all(wsId), "detail", ref] as const,
  modules: (wsId: string, projectId: string) =>
    [...testCaseKeys.all(wsId), "modules", projectId] as const,
  revisions: (wsId: string, ref: string) =>
    [...testCaseKeys.all(wsId), "revisions", ref] as const,
};
```

`queries.ts` 用 `queryOptions` 包 `api.*`，形状照抄 `packages/core/projects/queries.ts`。

- [ ] **Step 4: 实现 mutations**

按 CLAUDE.md 的乐观更新四条判据取舍：

- `useUpdateTestCase`（改标题/优先级/状态/模块，留在原页面）→ **乐观**：`onMutate` patch list 与 detail，`onError` 回滚，`onSettled` 同时失效 list 与 detail（`version` 与 `updated_at` 由服务端决定，必须回源）。
- `useApproveTestCase` → **乐观**（就是一次状态切换）。
- `useCreateTestCase` → **不乐观**：创建后要跳详情页，必须 await 服务端拿到真实 `key` 与 `id`。
- `useDeleteTestCase` → **不乐观**：删除后要导航，`onSuccess` 里 `removeQueries(detail)` + `invalidateQueries(list)`。

mutations 测试用 `setApiInstance` mock `@multica/core/api`，并 mock `../hooks` 的 `useWorkspaceId`：

```ts
it("does not remove the case from cache before the server confirms deletion", async () => {
  // assert list cache still contains the case while the mutation is in flight
});

it("rolls the title back when update fails", async () => {
  // assert cache equals the pre-mutation snapshot after rejection
});
```

- [ ] **Step 5: 接实时事件**

`packages/core/realtime/use-realtime-sync.ts` 的 `refreshMap` 里，在 `project:` 条目附近加：

```ts
      test_case: () => {
        const wsId = getCurrentWsId();
        if (wsId) qc.invalidateQueries({ queryKey: testCaseKeys.all(wsId) });
      },
```

（前缀是事件名 `:` 前的部分，即 `test_case`。）不加 `specificEvents` 精细处理器 —— 用例列表刷新成本低，第一期不需要逐条 patch。

- [ ] **Step 6: 加包导出**

`packages/core/package.json` 的 `exports`：

```json
    "./testing": "./testing/index.ts",
    "./testing/queries": "./testing/queries.ts",
    "./testing/mutations": "./testing/mutations.ts",
    "./testing/config": "./testing/config.ts",
```

- [ ] **Step 7: 跑测试确认通过**

```bash
pnpm --filter @multica/core test -- testing
pnpm typecheck
```

Expected: 全绿。

- [ ] **Step 8: 提交**

```bash
git add packages/core/testing packages/core/package.json packages/core/realtime/use-realtime-sync.ts
git commit -m "feat(testing): add core testing domain queries and mutations"
```

---

## Task 10: 路由与 tab 注册契约

**Files:**
- Modify: `packages/core/paths/paths.ts`, `packages/core/paths/route-icons.ts`, `packages/core/paths/consistency.test.ts`, `packages/core/diagnostics/diagnostic-context.ts`, `packages/views/editor/utils/link-handler.ts`

**Interfaces:**
- Consumes: 无运行时依赖。
- Produces: `paths.workspace(slug).tests()` → `/{slug}/tests`；`paths.workspace(slug).testCaseDetail(ref)` → `/{slug}/tests/{ref}`；`WORKSPACE_PAGES.tests`。Task 11 与 Task 14 依赖它们。

这是一层"每处都有测试守着"的契约，五个文件必须一次改齐，否则整个前端测试套件会崩。

- [ ] **Step 1: 加路由构造器**

`packages/core/paths/paths.ts`，在 `designRestoreTaskDetail` 之后：

```ts
    tests: () => `${ws}/tests`,
    testCaseDetail: (ref: string) => `${ws}/tests/${encode(ref)}`,
```

- [ ] **Step 2: 扩三个联合与注册表**

`packages/core/paths/route-icons.ts`：
- `RouteIconName` 加 `| "FlaskConical"`
- `NavLabelKey` 加 `| "tests"`
- `WorkspacePageKey` 加 `| "tests"`
- `WORKSPACE_PAGES` 加 `tests: { segment: "tests", icon: "FlaskConical", navKey: "tests" },`

- [ ] **Step 3: 更新两处硬编码清单**

`packages/core/paths/consistency.test.ts`：parameterless 集合加 `"tests"`；`expectedSegments` 加 `["tests", "tests"]`。

- [ ] **Step 4: 更新遥测路由模板**

`packages/core/diagnostics/diagnostic-context.ts` 的 `WORKSPACE_ROUTES` 加：

```ts
  ["tests"],
  ["tests", ":id"],
```

- [ ] **Step 5: 更新编辑器链接识别**

`packages/views/editor/utils/link-handler.ts:22` 的 `WORKSPACE_ROUTE_SEGMENTS` 加 `"tests"`，否则站内 `/{slug}/tests/...` 链接会被当外链在浏览器新开。

- [ ] **Step 6: 跑相关测试**

```bash
pnpm --filter @multica/core test -- paths diagnostics
```

Expected: 全绿。此时 `route-icons.test.ts` 的自动守卫（遍历真实 builder 断言每个 parameterless 路由都有 `WORKSPACE_PAGES` 条目）也应通过。

- [ ] **Step 7: 提交**

```bash
git add packages/core/paths packages/core/diagnostics packages/views/editor/utils/link-handler.ts
git commit -m "feat(testing): register the tests workspace route"
```

---

## Task 11: 侧边栏 tab 与 i18n

**Files:**
- Modify: `packages/views/layout/route-icon-components.tsx`, `packages/views/layout/app-sidebar.tsx`, `packages/views/layout/app-sidebar.test.tsx`, `packages/views/locales/{en,zh-Hans,ja,ko}/layout.json`, `packages/views/locales/index.ts`
- Create: `packages/views/locales/{en,zh-Hans,ja,ko}/testing.json`

**Interfaces:**
- Consumes: Task 10 的 `WORKSPACE_PAGES.tests` 与 `paths.tests`。
- Produces: 侧边栏可见的 tab；`useT("testing")` 命名空间，供 Task 12/13 的页面使用。

- [ ] **Step 1: 加图标映射**

`packages/views/layout/route-icon-components.tsx`：从 `lucide-react` 引入 `FlaskConical`，加进 `Record<RouteIconName, LucideIcon>`。缺了是编译错误。

- [ ] **Step 2: 加侧边栏项**

`packages/views/layout/app-sidebar.tsx`：该文件**自带**两个与 route-icons 重复的联合，两个都要加 `| "tests"`（`NavKey` 与 `NavLabelKey`），然后在 `workspaceNav` 里 `designs` 之后插入：

```ts
  { key: "tests", labelKey: "tests" },
```

- [ ] **Step 3: 更新侧边栏测试 mock**

`packages/views/layout/app-sidebar.test.tsx` 的 `useWorkspacePaths()` mock 加 `tests: () => "/acme/tests"`。漏了会让 `p[item.key]()` 抛错并拖垮整个 suite。

- [ ] **Step 4: 加 nav 文案（四语）**

各 `layout.json` 的 `nav` 加：`en` → `"tests": "Tests"`；`zh-Hans` → `"tests": "测试"`；`ja` → `"tests": "テスト"`；`ko` → `"tests": "테스트"`。

- [ ] **Step 5: 建 `testing` 命名空间**

四个 `testing.json`，先放页面需要的键（后续任务按需补，但四语必须同步）：

```json
{
  "page": { "title": "测试用例", "empty": "还没有测试用例", "new": "新建用例" },
  "filters": { "all": "全部", "module": "模块", "status": "状态", "priority": "优先级", "type": "类型", "origin": "来源" },
  "status": { "draft": "待审", "active": "已生效", "deprecated": "已废弃" },
  "origin": { "ai": "AI 生成", "human": "人工" },
  "priority": { "p0": "P0", "p1": "P1", "p2": "P2", "p3": "P3" },
  "detail": { "preconditions": "前置条件", "steps": "步骤", "expected": "预期结果", "repos": "关联仓库", "revisions": "变更历史" },
  "steps": { "add": "添加步骤", "action": "操作", "expected": "预期", "repo": "所在仓库", "remove": "删除此步骤" },
  "repos": { "add": "关联仓库", "alias": "别名", "role": "角色", "pathGlobs": "路径范围" },
  "role": { "under_test": "被测", "driver": "驱动", "verifier": "验证", "fixture": "数据准备" },
  "actions": { "approve": "通过审查", "delete": "删除", "save": "保存", "cancel": "取消" }
}
```

（以上为 `zh-Hans`；其余三语同结构、对应语言。）

在 `packages/views/locales/index.ts` 引入四个文件并注册进 `RESOURCES` 的四个 locale 块，键名 `testing`。

- [ ] **Step 6: 跑测试**

```bash
pnpm --filter @multica/views test -- app-sidebar parity
```

Expected: 全绿。`parity.test.ts` 会同时校验四语键齐全、且磁盘上的 JSON 都已注册。

- [ ] **Step 7: 提交**

```bash
git add packages/views/layout packages/views/locales
git commit -m "feat(testing): add the Tests sidebar tab and i18n namespace"
```

---

## Task 12: 用例列表页

**Files:**
- Create: `packages/views/testing/case-summary.ts`, `packages/views/testing/case-summary.test.ts`
- Create: `packages/views/testing/test-cases-page.tsx`, `packages/views/testing/test-cases-page.test.tsx`
- Create: `packages/views/testing/index.ts`
- Modify: `packages/views/package.json`

**Interfaces:**
- Consumes: Task 9 的 `testCaseListOptions` / `testCaseModulesOptions`；Task 11 的 `useT("testing")`。
- Produces: `<TestCasesPage />`（无 props）。Task 14 的平台壳依赖它。

- [ ] **Step 1: 先写纯函数与测试**

`packages/views/testing/case-summary.ts` 承担列表页所有非 React 逻辑，便于单测：

```ts
import type { TestCase, TestCaseStep } from "@multica/core/types";

/** Group cases by module, with an explicit bucket for the empty module. */
export function groupByModule(cases: TestCase[]): { module: string; cases: TestCase[] }[] { /* … */ }

/** Renumber steps to 1..n so a delete in the middle never leaves a gap. */
export function normalizeStepIndexes(steps: TestCaseStep[]): TestCaseStep[] { /* … */ }

/** "admin-web(driver), mobile-app(verifier)" for the list column. */
export function formatRepoSummary(testCase: TestCase): string { /* … */ }

/** A cross_repo case needs at least two repos with distinct roles. */
export function crossRepoWarning(testCase: TestCase): "missing_repos" | "single_role" | null { /* … */ }
```

`case-summary.test.ts` 覆盖：空模块归到一个显式分组、步骤删中间后重编号为 1..n、`formatRepoSummary` 空数组返回空串、`crossRepoWarning` 三条分支。

- [ ] **Step 2: 跑测试确认失败再实现，直到通过**

```bash
pnpm --filter @multica/views test -- case-summary
```

- [ ] **Step 3: 实现列表页**

`test-cases-page.tsx` 结构（布局与交互照 `packages/views/projects/components/projects-page.tsx` 的骨架）：

- 顶部 `PageHeader`（来自 `../layout`），右侧「新建用例」按钮；
- 左栏：项目选择器 + 模块分组树（数据来自 `testCaseModulesOptions`，含每组用例数）；
- 主区：表格列 `KEY` / 标题 / 模块 / 类型 / 优先级 / 状态 / 来源 / 关联仓库 / 更新时间；
- 筛选条：状态、优先级、类型、来源，外加一个「待审」快捷筛选（`status=draft`）；
- 空态用 `t(($) => $.page.empty)`；
- 行点击走 `useRowLink` / `<AppLink>` 到 `paths.testCaseDetail(key)`。

UI 约束：
- 只用语义色与 `--text-*` 角色字号；
- 选中行的激活态必须在 hover 时仍可辨认 —— 用字重或文字色表达，或显式定义 `data-active:hover:`，不要只靠背景色；
- 长标题截断加 `title` 属性；表格横向溢出放进 `overflow-x: auto` 容器；
- 不引入多余局部 state：筛选状态放 `packages/core/testing/` 下的 view store（Zustand，`persist` 只存筛选与列偏好），**不要**定义在 views 里。

- [ ] **Step 4: 写组件测试**

`test-cases-page.test.tsx`：mock `@multica/core/api`（`setApiInstance`）与 `@multica/core` 的 store（用可调用 store 形状：`selectorFn` + `getState`）。断言：

```ts
it("renders the case key and repo summary for each row", async () => { /* … */ });
it("shows the empty state when the workspace has no cases", async () => { /* … */ });
it("filters to drafts when the review shortcut is clicked", async () => { /* … */ });
```

**禁止**在这里 mock `next/*` 或 `react-router-dom`。

- [ ] **Step 5: 加 barrel 与包导出**

`packages/views/testing/index.ts` 导出 `TestCasesPage`；`packages/views/package.json` 的 `exports` 加 `"./testing": "./testing/index.ts"`。

- [ ] **Step 6: 跑测试与 lint**

```bash
pnpm --filter @multica/views test -- testing
pnpm lint
```

Expected: 全绿。`i18next/no-literal-string` 一条不报。

- [ ] **Step 7: 提交**

```bash
git add packages/views/testing packages/views/package.json
git commit -m "feat(testing): add the test cases list page"
```

---

## Task 13: 用例详情页与步骤编辑器

**Files:**
- Create: `packages/views/testing/test-case-detail.tsx`, `packages/views/testing/test-case-detail.test.tsx`
- Create: `packages/views/testing/components/test-case-steps-editor.tsx`, `.../test-case-steps-editor.test.tsx`
- Create: `packages/views/testing/components/test-case-repos-field.tsx`
- Modify: `packages/views/testing/index.ts`

**Interfaces:**
- Consumes: Task 9 的 `testCaseDetailOptions` / `useUpdateTestCase` / `useApproveTestCase` / `useDeleteTestCase` / `testCaseRevisionsOptions`；Task 12 的 `normalizeStepIndexes` / `crossRepoWarning`。
- Produces: `<TestCaseDetail refId={string} />`（prop 名 `refId`，接受 `TC-42` 或 UUID）。

- [ ] **Step 1: 先写步骤编辑器测试**

```tsx
it("appends a step numbered one past the last", async () => { /* … */ });
it("renumbers remaining steps after deleting a middle step", async () => { /* … */ });
it("keeps the repo selector limited to the case's related repo aliases", async () => { /* … */ });
it("does not emit a change when the action and expected are both blank", async () => { /* … */ });
```

- [ ] **Step 2: 跑测试确认失败，然后实现**

`test-case-steps-editor.tsx` 是受控组件：`value: TestCaseStep[]`、`onChange(next: TestCaseStep[])`、`repoAliases: string[]`。每行是序号 + 操作输入 + 预期输入 + 仓库下拉 + 删除按钮，底部「添加步骤」。删除后调 `normalizeStepIndexes`。不做拖拽排序（YAGNI，第一期不需要）。

- [ ] **Step 3: 实现详情页**

`test-case-detail.tsx`：

- 头部：`TC-42` + 标题（可就地编辑）+ 状态徽章 + 来源徽章 + 「通过审查」（仅 `draft` 显示）+ 删除；
- 主区：前置条件、步骤编辑器、预期结果、测试数据；
- 侧栏：项目、模块、优先级、类型、范围、执行方式、关联仓库（`test-case-repos-field.tsx`）、变更历史（版本 + 变更类型 + 时间 + 操作人）；
- `scope === "cross_repo"` 但 `crossRepoWarning()` 非 null 时，在关联仓库区显示一条警示文案（走 i18n）；
- 保存走 `useUpdateTestCase`（乐观），删除走 `useDeleteTestCase` 并 **await 后**再 `useNavigation().push(paths.tests())`。

`test-case-repos-field.tsx`：从 `projectResources` 查询里筛出 `resource_type === "github_repo"` 的资源供选择，别名默认取仓库名，角色是四选一下拉，`path_globs` 是逗号分隔输入。

- [ ] **Step 4: 写详情页测试**

```tsx
it("shows the approve action only for draft cases", async () => { /* … */ });
it("warns when a cross_repo case has fewer than two repos", async () => { /* … */ });
it("navigates back to the list only after the delete request resolves", async () => { /* … */ });
it("renders the revision history newest first", async () => { /* … */ });
```

- [ ] **Step 5: 跑测试与 lint**

```bash
pnpm --filter @multica/views test -- testing
pnpm lint
```

- [ ] **Step 6: 提交**

```bash
git add packages/views/testing
git commit -m "feat(testing): add the test case detail page and steps editor"
```

---

## Task 14: web 与 desktop 平台接线

**Files:**
- Create: `apps/web/app/[workspaceSlug]/(dashboard)/tests/page.tsx`
- Create: `apps/web/app/[workspaceSlug]/(dashboard)/tests/[id]/page.tsx`
- Modify: `apps/desktop/src/renderer/src/routes.tsx`

**Interfaces:**
- Consumes: Task 12/13 导出的 `TestCasesPage` / `TestCaseDetail`。
- Produces: 两个平台上可访问的 `/{slug}/tests` 与 `/{slug}/tests/{ref}`。

- [ ] **Step 1: web 壳**

`apps/web/app/[workspaceSlug]/(dashboard)/tests/page.tsx`：

```tsx
"use client";

import { TestCasesPage } from "@multica/views/testing";

export default function Page() {
  return <TestCasesPage />;
}
```

`apps/web/app/[workspaceSlug]/(dashboard)/tests/[id]/page.tsx`：

```tsx
"use client";

import { useParams } from "next/navigation";
import { TestCaseDetail } from "@multica/views/testing";

export default function Page() {
  const params = useParams<{ id: string }>();
  return <TestCaseDetail refId={params.id} />;
}
```

- [ ] **Step 2: desktop 路由**

`apps/desktop/src/renderer/src/routes.tsx`，在 `designs` 相关条目附近的 `:workspaceSlug` children 里加：

```tsx
      { path: "tests", element: <TestCasesPage />, handle: { title: "Tests" } },
      { path: "tests/:id", element: <DesktopTestCaseDetailRoute />, handle: { title: "Test Case" } },
```

并在文件里既有的一组小包装组件旁加：

```tsx
function DesktopTestCaseDetailRoute() {
  const { id } = useParams<{ id: string }>();
  return <TestCaseDetail refId={id ?? ""} />;
}
```

（`useParams()` 必须在组件里调用，所以带参路由都要这层包装 —— 与 `DesktopDesignFileRoute` 同理。）

- [ ] **Step 3: 类型检查与构建**

```bash
pnpm typecheck
pnpm build
```

Expected: 无错误。

- [ ] **Step 4: 人工验证**

```bash
make dev
```

浏览器打开 `/{slug}/tests`：侧边栏出现「测试」tab 且图标为烧瓶；建一条用例，编辑步骤，保存，刷新后仍在；删除后回到列表。

- [ ] **Step 5: 提交**

```bash
git add apps/web apps/desktop
git commit -m "feat(testing): wire the tests routes on web and desktop"
```

---

## Task 15: 端到端与文档

**Files:**
- Create: `e2e/test-cases.spec.ts`
- Modify: `server/internal/service/builtin_skills/multica-test-cases/references/test-cases-source-map.md`（补齐实现后的真实行号）

**Interfaces:**
- Consumes: 全部前序任务。
- Produces: 一条覆盖主链路的 e2e。

- [ ] **Step 1: 写 e2e**

`e2e/test-cases.spec.ts`，用 `TestApiClient` 做 setup/teardown：

```ts
test("create, edit, approve and delete a test case", async ({ page }) => {
  // 1. TestApiClient 建 workspace + project
  // 2. 访问 /{slug}/tests，断言空态
  // 3. 新建用例，填标题与两条步骤，保存
  // 4. 断言列表出现 TC-1
  // 5. 进详情，改优先级为 p0，断言乐观更新即时可见且刷新后仍是 p0
  // 6. 删除，断言回到列表且列表为空
});

test("a cross-repo case shows the repo summary in the list", async ({ page }) => {
  // 用 TestApiClient 直接建带两个 repo 绑定的用例，断言列表里
  // 显示 "admin-web(driver), mobile-app(verifier)"
});
```

- [ ] **Step 2: 跑 e2e**

```bash
pnpm exec playwright test e2e/test-cases.spec.ts
```

Expected: 通过。

- [ ] **Step 3: 补齐 source map 行号**

实现落地后行号才稳定，此时回填 `references/test-cases-source-map.md` 的 `Behavior | File:line` 表，并跑：

```bash
cd server && go test ./internal/service -run 'BuiltinSkills|TestCasesSkill' -count=1
```

- [ ] **Step 4: 全量验证**

```bash
pnpm typecheck
pnpm lint
pnpm test
make test
```

Expected: 全绿。任何一项失败都必须修掉再进入下一步 —— 不得以"与本次改动无关"为由跳过而不说明。

- [ ] **Step 5: 提交**

```bash
git add e2e server/internal/service/builtin_skills
git commit -m "test(testing): add test case end-to-end coverage"
```

---

## Self-Review

**Spec 覆盖（第一期范围）**

| spec 要素 | Task |
| --- | --- |
| `test_case` / `test_case_revision` / `test_case_repo` 表、无外键、并发索引 | 1 |
| `workspace.test_case_counter` 与 `TC-42` 编号 | 1、2、3 |
| 用例 CRUD、审批、版本快照 | 4 |
| 扇形分组（`module` + modules 端点） | 2、4、12 |
| 多仓库关联与 `role` | 1、4、13 |
| 智能体只读 CLI | 6 |
| `multica-test-cases` 内置技能 | 7 |
| 侧边栏 tab（四层契约 + i18n） | 10、11 |
| core 三层数据层与 `parseWithFallback` | 8、9 |
| 实时事件 | 4、9 |
| 列表页 / 详情页 / 步骤编辑器 | 12、13 |
| web + desktop 接线 | 14 |
| 测试分布（Go / core / views / e2e） | 3、4、6、7、8、9、12、13、15 |

**不在第一期**（spec 已明确排期，此处仅记录以免误判为遗漏）：`test_generation_job` / `test_generation_plan` / `test_case_proposal`（第二期）、`test_plan` / `test_run` / `test_run_case`（第三期）、`test_capability` 与执行 CLI（第四期）、§12 的三处既有缺陷修复（第二期，因为它们服务于生成期的多仓库 checkout）。

**类型一致性检查**

- `refId` 作为详情组件 prop 名，Task 13 定义、Task 14 使用 —— 一致。
- 路由参数在服务端叫 `{ref}`（Task 5），在 chi 里用 `chi.URLParam(r, "ref")`（Task 4）—— 一致。
- `testCaseKeys.list(wsId, filters)` 的第二参数在 Task 9 定义为可选对象，queries 与 mutations 的失效调用都用 `testCaseKeys.all(wsId)` 做前缀失效 —— 不会因 filters 不同而漏失效。
- `formatRepoSummary` 在 Task 12 定义、Task 15 的 e2e 断言其输出格式 —— 一致。
- 事件前缀 `test_case`（Go 常量值 `test_case:created` 等，Task 4）与 `refreshMap` 的键 `test_case`（Task 9）—— 一致。
