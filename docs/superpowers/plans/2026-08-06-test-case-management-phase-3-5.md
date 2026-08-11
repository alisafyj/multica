# 测试用例管理 · 第三～五期 实施计划

> **For agentic workers:** REQUIRED SUB-SKILL: superpowers:subagent-driven-development or superpowers:executing-plans.

**Goal:** 测试计划与执行轮次（人工 + 智能体）、留痕与重试、以及真机/桌面/浏览器控制的能力接口层。

**依据:** [spec](../specs/2026-08-05-test-case-management-design.md) §5、§6、§14 第三～五期。第一、二期已全部落地（`aff9cc69c` 为止）。

## Global Constraints

前两期的 Global Constraints 全部继续生效。追加：

- **迁移编号从 295 起**（第二期占到 294）。开工前 `ls server/migrations | tail -2` 复核。
- **留痕不可覆盖**：重跑是新建一个 `source_run_id` 指向原轮次的 run，绝不就地重置结果。
- **`case_snapshot` 是硬要求**：`test_run_case` 存执行时的用例快照。用例后续被改不得改写历史轮次的结果，否则回归数据失真。
- **能力平面就是 MCP**。不要为设备再造 runtime——`runtime_profile` 的铁律（`server/internal/handler/runtime_profile.go:28`）是只承载进入兼容模式所需的 `fixed_args`。
- **显式失败优于静默降级**：能力解析不出来的 run 直接落 `blocked` 并写明缺哪个 kind，不进队列。
- **本机无数据库**：`go test ./internal/handler` 会跳过全部测试并仍打印 `ok`。不得引用该包的 `ok` 作为通过证据。用 `-v` 数 `--- PASS`。

---

## 分组与并行边界

| 组 | 内容 | 独占文件 |
| --- | --- | --- |
| P3-A | 计划/轮次后端 | `migrations/295+`、`queries/test_run.sql`、`handler/test_plan.go`、`handler/test_run.go`、`router.go`、`protocol/events.go` |
| P3-B | 计划/轮次前端 | `core/types/testing.ts`、`core/api/*`、`core/testing/**`、`views/testing/test-plans-*.tsx`、`views/testing/test-run-*.tsx`、`locales/*/testing.json`、`apps/**` |
| P4-A | 能力接口层后端 | `migrations/(P3-A 之后)`、`queries/test_capability.sql`、`handler/test_capability.go`、`daemon/capabilities.go`、`service/task.go`、`handler/daemon.go` |
| P4-B | 执行 CLI + 技能 | `cmd/multica/cmd_test.go`、`builtin_skills/multica-running-tests/**` |

P3-A 是关键链路，P4-A 依赖它的 `test_run` 表。P3-B / P4-B 依赖 P3-A / P4-A 的契约但可按契约先行。

---

## Task P3-A1: 计划与轮次表

**Files:** `server/migrations/295_test_run.{up,down}.sql`，`296..302_*_index.{up,down}.sql`

```sql
CREATE TABLE test_plan (
    id           UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    workspace_id UUID NOT NULL,
    project_id   UUID NOT NULL,
    title        TEXT NOT NULL,
    description  TEXT NOT NULL DEFAULT '',
    status       TEXT NOT NULL DEFAULT 'draft' CHECK (status IN ('draft','active','archived')),
    created_by   UUID,
    created_at   TIMESTAMPTZ NOT NULL DEFAULT now(),
    updated_at   TIMESTAMPTZ NOT NULL DEFAULT now()
);

CREATE TABLE test_plan_case (
    plan_id      UUID NOT NULL,
    workspace_id UUID NOT NULL,
    test_case_id UUID NOT NULL,
    position     INT  NOT NULL DEFAULT 0,
    created_at   TIMESTAMPTZ NOT NULL DEFAULT now(),
    PRIMARY KEY (plan_id, test_case_id)
);

CREATE TABLE test_run (
    id                 UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    workspace_id       UUID NOT NULL,
    project_id         UUID NOT NULL,
    plan_id            UUID,                     -- NULL = 临时轮次（直接勾一批用例）
    title              TEXT NOT NULL,
    executor_type      TEXT NOT NULL CHECK (executor_type IN ('member','agent')),
    executor_id        UUID NOT NULL,
    agent_task_id      UUID,
    environment        TEXT NOT NULL DEFAULT '',
    build_ref          TEXT NOT NULL DEFAULT '',
    capability_binding JSONB NOT NULL DEFAULT '{}',
    status             TEXT NOT NULL DEFAULT 'pending'
                       CHECK (status IN ('pending','running','completed','aborted','blocked')),
    source_run_id      UUID,                     -- 重试链
    retry_scope        TEXT CHECK (retry_scope IN ('all','failed_only','selected')),
    error              TEXT,
    started_at         TIMESTAMPTZ,
    completed_at       TIMESTAMPTZ,
    created_by         UUID,
    created_at         TIMESTAMPTZ NOT NULL DEFAULT now(),
    updated_at         TIMESTAMPTZ NOT NULL DEFAULT now()
);

CREATE TABLE test_run_case (
    id               UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    workspace_id     UUID NOT NULL,
    run_id           UUID NOT NULL,
    test_case_id     UUID NOT NULL,
    case_snapshot    JSONB NOT NULL,
    position         INT  NOT NULL DEFAULT 0,
    result           TEXT NOT NULL DEFAULT 'pending'
                     CHECK (result IN ('pending','running','passed','failed','blocked','skipped')),
    notes            TEXT NOT NULL DEFAULT '',
    evidence         JSONB NOT NULL DEFAULT '[]',
    step_results     JSONB NOT NULL DEFAULT '[]',
    duration_ms      INT,
    executed_by_type TEXT CHECK (executed_by_type IN ('member','agent')),
    executed_by_id   UUID,
    executed_at      TIMESTAMPTZ,
    defect_issue_id  UUID,
    created_at       TIMESTAMPTZ NOT NULL DEFAULT now(),
    updated_at       TIMESTAMPTZ NOT NULL DEFAULT now()
);

-- 证据复用现有附件系统，不新建存储
ALTER TABLE attachment ADD COLUMN test_run_case_id UUID;
```

索引（各自单文件单语句 `CONCURRENTLY`）：
`test_plan (workspace_id, project_id, status)`、`test_plan_case (plan_id, position)`、`test_run (workspace_id, project_id, created_at DESC)`、`test_run (agent_task_id)`、`test_run (source_run_id)`、`test_run_case (run_id, position)`、`test_run_case (workspace_id, test_case_id, created_at DESC)`（跨轮次时间线）、`attachment (test_run_case_id) WHERE test_run_case_id IS NOT NULL`。

---

## Task P3-A2: sqlc 与 handler

`server/pkg/db/queries/test_run.sql`：计划 CRUD、`test_plan_case` 增删排序、run CRUD、`ListTestRunCases`、`UpdateTestRunCaseResult`、`ListTestCaseResultTimeline`（按 `test_case_id` 跨轮次）、`CountTestRunResults`（聚合通过率）、以及 `DeleteTestPlanCases` / `DeleteTestRunCases` 供删除事务。全部 narg 用 `COALESCE`。

`handler/test_plan.go` / `handler/test_run.go`：

- `CreateTestRun`：接受 `plan_id` 或显式 `test_case_ids`；**在事务里为每条用例写 `case_snapshot`**（用 `testCaseToResponse` 的 JSON）；`executor_type='member'` 时立即 `pending`。
- `UpdateTestRunCaseResult`：人或 agent 打点。写 `executed_by_*` 与 `executed_at`；全部 case 非 `pending` 时自动把 run 置 `completed` 并写 `completed_at`。
- `OpenTestRunCaseDefect`：创建缺陷 issue（复用 `service.IssueService` 的创建路径，标题带用例 key，描述嵌用例快照与轮次链接），回写 `defect_issue_id`。
- `RetryTestRun`：`{scope: all|failed_only|selected, case_ids?}` → 新建 run，`source_run_id` 指向原轮次，按 scope 复制 `test_run_case`（快照重新取当前用例，因为重跑是针对当前定义）。**不修改原轮次任何行。**
- `GetTestRunExecutionStatus`：照抄 `designRestoreExecutionStatusToResponse`（`handler/design_file.go:1630`）join `agent_task` + `agent_runtime` + 最新 task message，派生 `{phase, reason, severity}`，无需新列。

事件：`test_plan:updated`、`test_run:updated`、`test_run_case:updated`。**不做 3 秒轮询。**

---

## Task P4-A1: 能力表与解析

`server/migrations/(next)_test_capability.up.sql`：

```sql
CREATE TABLE test_capability (
    id             UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    workspace_id   UUID NOT NULL,
    daemon_id      TEXT NOT NULL,
    runtime_id     UUID,
    kind           TEXT NOT NULL
                   CHECK (kind IN ('android_device','ios_device','computer_use','browser')),
    capability_key TEXT NOT NULL,
    target         JSONB NOT NULL DEFAULT '{}',
    status         TEXT NOT NULL DEFAULT 'unknown'
                   CHECK (status IN ('available','busy','offline','unknown')),
    probe          JSONB NOT NULL DEFAULT '{}',
    last_probe_at  TIMESTAMPTZ,
    created_at     TIMESTAMPTZ NOT NULL DEFAULT now(),
    updated_at     TIMESTAMPTZ NOT NULL DEFAULT now()
);
```
索引：`UNIQUE (workspace_id, daemon_id, capability_key)`、`(workspace_id, kind, status)`。

`test_case.required_capabilities` 第一期已有，存需求而非实例：
```json
[{"kind":"android_device","match":{"os_version":">=13"},"optional":false}]
```

`resolveRunCapabilities(ctx, wsID, requirements)`（`handler/test_capability.go`）：按 `kind + match` 过滤 `status='available'`；**要求同一 `daemon_id` 覆盖全部必需项**（跨机执行不在 v1）；无解则 run 落 `blocked`、`error` 写明缺失 kind、**不进队列**；有解则把结果冻结进 `test_run.capability_binding`。

## Task P4-A2: overlay provider 与留桩

- `server/internal/integrations/testcapability/dispatch.go`：`BuildTaskOverlay(ctx, originatorUserID, agent)` 仿 `internal/integrations/composio/dispatch.go`，返回 `runtimeapps.MCPOverlayResult`。MCP server 名用 `multica-device` / `multica-browser`（`mergeMCPOverlay` 按名合并，撞名会静默覆盖）。
- `capabilityMCPServers(kind, target)`：**浏览器返回真实条目；`android_device` / `ios_device` / `computer_use` 目前返回空 map**——这是 Phase 5 的接入点。返回空时该 kind 的 run 直接 `blocked`，文案写"能力未注册"。
- `service/task.go` 的 `buildRuntimeMCPOverlay`（:288）union 进来，保持 fail-soft，保持在插入队列行**之前**。
- daemon 侧 `listRuntimeCapabilities`：与 `daemon/runtime_mcp.go:207` 的 `listRuntimeLocalMcpServers` 并列，**只回传脱敏摘要**（绝不带 URL / headers / 命令参数）。第一版只探测浏览器。
- 上报通道复制 `handler/runtime_local_skills.go` 的 request/poll/report store。

**Phase 5 接入点（设备工作落地时只改这两处）**：`capabilityMCPServers` 加 `android_device` / `ios_device` / `computer_use` 分支；`listRuntimeCapabilities` 加对应探测。测试域的表、CLI、技能、证据链路均不改动。

## Task P4-A3: 智能体执行与证据

- `test_run` 的 agent 派发：`service.TestRunContext{Type:"test_run", ...}` + claim 分支 + start/complete/fail 钩子，与 `test_generation` 同构。
- **写权限**：`UpdateTestRunCaseResult` 走 task_token 三重门 —— `X-Actor-Source == "task_token"`、`test_run.agent_task_id == X-Task-ID`、且 run-case 属于该 run。照抄 `handler/test_generation_propose.go:requireTestGenerationTaskToken`。
- **读权限**：用例、计划、轮次对任何智能体开放（需求 5）。
- 证据：`UploadFile` 增加 `test_run_case_id` 表单字段，校验沿用 `handler/file.go:502` 的三重门；追加 `{attachment_id, kind, markdown_url, captured_at}` 进 `evidence`。**只存 id 与 `markdown_url`，绝不存 `download_url`**（签名 URL 会过期）。

---

## Task P4-B: `multica test` 命令组

```bash
multica test run get <run-id> --output json
multica test run start <run-id>
multica test result set <run-case-id> --result passed|failed|blocked|skipped [--note …] [--step-results-stdin]
multica test evidence add <run-case-id> --file ./shot.png --kind screenshot
multica test defect open <run-case-id> --title …
multica test capability list --run <run-id> --output json
multica test plan list|get
```

内置技能 `multica-running-tests`：先 `capability list`，只使用返回的 `capability_key`；列表为空即判 `blocked` 并说明原因，**禁止自行寻找 adb / 浏览器**；具体操作走 MCP，CLI 只负责发现与回执；失败开缺陷；`blocked` 与 `failed` 的区别（前者是环境挡住，后者是被测行为不符）。

---

## Task P3-B / P4-C: 前端

- 数据层：types / schemas（全部 `parseWithFallback`）/ client / keys / queries / mutations / realtime 三个 refreshMap 条目 / exports。
- `test-plans-page.tsx`、`test-plan-detail.tsx`（勾选用例、排序）。
- `test-run-detail.tsx`：逐条结果打点、证据、执行诊断、重试入口（全部/仅失败/选中）、一键开缺陷。
- 用例详情页加**跨轮次结果时间线**——这是回归价值的核心视图。
- 列表页与详情页加发起轮次入口。**每个新页面必须有入口并有测试断言导航**（前两期这里连续踩坑）。
- 四个 locale 同步。

---

## Task F: 验证

- e2e：建计划 → 跑一轮人工 → 一条失败开 issue → 仅失败重跑 → 断言原轮次结果未被改写。
- 全量：`pnpm typecheck` / `pnpm lint` / `pnpm test` / `go build` / `go vet ./...` / 各包 `--- PASS` 计数。
- 回填 source-map 行号。

## Self-Review

| spec 要素 | Task |
| --- | --- |
| `test_plan` / `test_plan_case` / `test_run` / `test_run_case` | P3-A1、P3-A2 |
| 人工打点、证据、一键开缺陷 | P3-A2、P4-A3 |
| 重试（新 run + `source_run_id`） | P3-A2 |
| 跨轮次结果时间线 | P3-A2、P3-B |
| 实时事件替代轮询 | P3-A2 |
| `test_capability` + 解析 + 冻结 | P4-A1 |
| MCP overlay provider + 留桩 | P4-A2 |
| 智能体执行 + task_token 写权限 | P4-A3 |
| `multica test` + `multica-running-tests` | P4-B |
| 执行诊断 | P3-A2（合成 `execution_status`） |
| Phase 5 | P4-A2 的两个接入点，设备驱动落地后各加一个分支 |
