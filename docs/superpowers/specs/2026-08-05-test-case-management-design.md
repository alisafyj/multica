# 测试用例管理设计

日期：2026-08-05

## 1. 背景与目标

Multica 目前没有测试用例这一实体。团队的测试知识散落在 issue 描述、评论和人脑里，无法被复用，也无法被智能体消费。

本设计新增一个 workspace 级的 **测试域（testing）**：

- 用例是**一等结构化实体**，人能照着执行，智能体也能通过 CLI 取走并执行；
- 用例由 AI 基于**项目文档 + 已有代码 + 需求 issue + 公司业务知识**生成，人工审查、编辑、补充；
- 执行有留痕、可观测、可重试，智能体同样具备执行与回写权限；
- 为真机（Android / iOS）、computer-use、浏览器控制**预留能力接口层**，等外部能力落地时只需填充 provider，不改测试域任何表结构与智能体契约。

### 1.1 需求映射

| # | 需求 | 落在哪一节 |
| --- | --- | --- |
| 1 | 侧边栏 tab 入口，与"设计"同级 | §9 前端结构 |
| 2 | 按项目生成；项目可关联多仓库；用例标明关联哪几个仓库 | §2.3 `test_case_repo`、§7 多仓库上下文 |
| 3 | 用例全面，覆盖公司/项目业务逻辑，不只是代码层面 | §7 业务知识注入 |
| 4 | AI 与人工用例均可增删改查，可分组 | §2.1、§4 审查与编辑 |
| 5 | 任何智能体都能获取用例信息（仓库、项目、issue） | §8 Agent 契约 |
| 6 | 执行留痕、可观测、可重试，智能体有权限 | §5 计划/轮次/执行 |
| 7 | 预留 app / 电脑 / 浏览器控制能力接口 | §6 执行能力接口层 |

### 1.2 已确认的产品决策

- 用例挂在 `project` 下，用 `module` 字段 + 标签做扇形分组，**不引入套件树**。
- 完整三层：`test_plan` → `test_run` → `test_run_case`。
- AI 生成的**新增**用例直接落库为 `draft`；**修订/废弃**建议进 `test_case_proposal` 等人工比对采纳（详见 §4.2）。
- 重复生成时 AI 感知已有用例，只产出增量。
- 生成引擎复用现有 agent task 队列 + CLI 回写，不在 server 侧直连 LLM。

---

## 2. 领域模型

**硬约束（CLAUDE.md）**：新表一律**不建外键、不建级联**，关联与依赖清理在应用层事务里做；每个索引一条 `CREATE INDEX CONCURRENTLY` 单语句、单文件迁移。所有表带 `workspace_id`，所有查询按 `workspace_id` 过滤。

迁移从 `server/migrations/280_*` 起（当前最后一个是 `279_runtime_profile_add_reasonix`）。以下编号为示意，实现时按实际顺延。

### 2.1 用例库

```sql
CREATE TABLE test_case (
  id             UUID PRIMARY KEY DEFAULT gen_random_uuid(),
  workspace_id   UUID NOT NULL,
  project_id     UUID NOT NULL,
  case_number    INT  NOT NULL,                    -- 工作区内自增，展示为 TC-42
  title          TEXT NOT NULL,
  module         TEXT NOT NULL DEFAULT '',         -- 扇形分组
  preconditions  TEXT NOT NULL DEFAULT '',
  steps          JSONB NOT NULL DEFAULT '[]',      -- [{index, action, expected, repo?}]
  expected_result TEXT NOT NULL DEFAULT '',
  test_data      JSONB NOT NULL DEFAULT '{}',
  priority       TEXT NOT NULL DEFAULT 'p2' CHECK (priority IN ('p0','p1','p2','p3')),
  case_type      TEXT NOT NULL DEFAULT 'functional'
                 CHECK (case_type IN ('functional','business_flow','api','ui','e2e',
                                      'regression','boundary','exception','permission',
                                      'data_consistency','compatibility','performance','security')),
  scope          TEXT NOT NULL DEFAULT 'single_repo' CHECK (scope IN ('single_repo','cross_repo','no_repo')),
  execution_mode TEXT NOT NULL DEFAULT 'manual' CHECK (execution_mode IN ('manual','agent','both')),
  required_capabilities JSONB NOT NULL DEFAULT '[]',   -- §6
  business_rules_ref    JSONB NOT NULL DEFAULT '[]',   -- §7，指向业务知识的锚点
  status         TEXT NOT NULL DEFAULT 'draft' CHECK (status IN ('draft','active','deprecated')),
  origin         TEXT NOT NULL DEFAULT 'human' CHECK (origin IN ('ai','human')),
  source_refs    JSONB NOT NULL DEFAULT '{}',      -- {issue_ids:[], files:[], attachment_ids:[], commit:""}
  generation_job_id UUID,
  version        INT  NOT NULL DEFAULT 1,
  created_by     UUID, updated_by UUID,
  reviewed_by    UUID, reviewed_at TIMESTAMPTZ,
  created_at     TIMESTAMPTZ NOT NULL DEFAULT now(),
  updated_at     TIMESTAMPTZ NOT NULL DEFAULT now()
);
```

索引（各自单独一个 `CREATE INDEX CONCURRENTLY` 迁移文件）：`UNIQUE (workspace_id, case_number)`、`(workspace_id, project_id, status)`、`(workspace_id, generation_job_id)`。

`steps` 是结构化数组而非 markdown —— 这是"智能体能取走执行"的前提。`steps[].repo` 是可选的仓库别名（见 §2.3），跨仓库用例靠它表达"这一步在哪个系统里做"。

`origin` 的判定规则：由 `test_generation_job` 产出的记为 `ai`，其余一切创建路径（Web/桌面表单、`multica testcase create`，无论调用者是人还是智能体）记为 `human`。`origin` 表达的是"是否由生成任务批量产出、需要审查"，不是"打字的是碳基还是硅基"。

`case_type` 刻意包含 `business_flow` / `permission` / `data_consistency`，让生成任务的输出天然不只停留在代码层面（需求 3）。

**编号**：沿用 issue 的做法（`server/migrations/020_issue_number.up.sql` 的 `workspace.issue_prefix` + `issue_counter`），新增 `workspace.test_case_counter INT NOT NULL DEFAULT 0`，在创建事务内自增。展示为 `TC-42`，工作区内唯一。

```sql
CREATE TABLE test_case_revision (
  id           UUID PRIMARY KEY DEFAULT gen_random_uuid(),
  workspace_id UUID NOT NULL,
  test_case_id UUID NOT NULL,
  version      INT  NOT NULL,
  snapshot     JSONB NOT NULL,                     -- 变更前的完整用例字段
  change_kind  TEXT NOT NULL CHECK (change_kind IN ('human_edit','proposal_accepted','status_change','restore')),
  changed_by   UUID, changed_by_type TEXT NOT NULL DEFAULT 'member' CHECK (changed_by_type IN ('member','agent')),
  note         TEXT NOT NULL DEFAULT '',
  created_at   TIMESTAMPTZ NOT NULL DEFAULT now()
);
```

每次人工编辑、采纳 AI 修订、状态变更时写一条**变更前快照**，支持对比与回退。审查功能的核心价值是可回退。

**分组**：`module` 字段承担主分组。若需要更自由的多维标签，复用现有标签体系而不是新建目录（`server/migrations/162_resource_labels.up.sql`）：在 `issue_label.resource_type` 的 CHECK 里加 `'test_case'`，新增无外键的 `test_case_to_label(test_case_id, label_id, created_at)`，并把 `DeleteTestCaseLabelAssignmentsByLabel` 加进 `DeleteLabel`（`server/internal/handler/label.go:352`）的事务循环。

### 2.2 AI 生成

```sql
CREATE TABLE test_generation_job (
  id             UUID PRIMARY KEY DEFAULT gen_random_uuid(),
  workspace_id   UUID NOT NULL,
  project_id     UUID NOT NULL,
  agent_id       UUID,
  agent_task_id  UUID,
  status         TEXT NOT NULL DEFAULT 'queued'
                 CHECK (status IN ('queued','running','completed','failed','cancelled')),
  input          JSONB NOT NULL DEFAULT '{}',   -- {issue_ids, modules, instructions, attachment_ids, resource_ids, path_globs}
  result         JSONB NOT NULL DEFAULT '{}',   -- {summary, stats:{new,updated,obsolete}, blockers, session_id, work_dir}
  error          TEXT,
  created_by     UUID,
  created_at     TIMESTAMPTZ NOT NULL DEFAULT now(),
  updated_at     TIMESTAMPTZ NOT NULL DEFAULT now()
);

CREATE TABLE test_generation_plan (
  id            UUID PRIMARY KEY DEFAULT gen_random_uuid(),
  workspace_id  UUID NOT NULL,
  job_id        UUID NOT NULL,
  status        TEXT NOT NULL DEFAULT 'draft' CHECK (status IN ('draft','approved','dispatched','archived')),
  plan          JSONB NOT NULL DEFAULT '{}',   -- 覆盖哪些仓库/路径/模块/业务规则、预计产出量
  review_notes  TEXT NOT NULL DEFAULT '',
  approved_by   UUID, approved_at TIMESTAMPTZ,
  created_by    UUID,
  created_at    TIMESTAMPTZ NOT NULL DEFAULT now(),
  updated_at    TIMESTAMPTZ NOT NULL DEFAULT now()
);
-- 单独一个迁移文件：
-- CREATE UNIQUE INDEX CONCURRENTLY ... ON test_generation_plan (job_id)
--   WHERE status IN ('draft','approved','dispatched');   -- 每个 job 至多一个活跃计划

CREATE TABLE test_case_proposal (
  id            UUID PRIMARY KEY DEFAULT gen_random_uuid(),
  workspace_id  UUID NOT NULL,
  job_id        UUID NOT NULL,
  target_case_id UUID NOT NULL,
  kind          TEXT NOT NULL CHECK (kind IN ('update','obsolete')),
  payload       JSONB NOT NULL DEFAULT '{}',   -- 建议后的完整用例字段（update 时）
  rationale     TEXT NOT NULL DEFAULT '',
  status        TEXT NOT NULL DEFAULT 'pending' CHECK (status IN ('pending','accepted','rejected')),
  reviewed_by   UUID, reviewed_at TIMESTAMPTZ,
  created_at    TIMESTAMPTZ NOT NULL DEFAULT now()
);
```

### 2.3 多仓库关联（需求 2）

```sql
CREATE TABLE test_case_repo (
  test_case_id        UUID NOT NULL,
  workspace_id        UUID NOT NULL,
  project_resource_id UUID NOT NULL,       -- 指向 project_resource.id，应用层校验
  alias               TEXT NOT NULL,       -- 用例步骤里引用的短名，如 "billing-api"
  role                TEXT NOT NULL DEFAULT 'under_test'
                      CHECK (role IN ('under_test','driver','verifier','fixture')),
  path_globs          JSONB NOT NULL DEFAULT '[]',
  created_at          TIMESTAMPTZ NOT NULL DEFAULT now(),
  PRIMARY KEY (test_case_id, project_resource_id, role)
);
```

绑 `project_resource_id` 而不是 repo URL：URL 会变，资源 id 在工作区内稳定，且已经随 claim 下发给智能体（`resp.ProjectResources`，`server/internal/handler/daemon.go`）。

`role` 让"后台改数据 → app 查看"这类跨库用例可被机器理解：

```json
{
  "title": "后台调价后，移动端订单页展示新价且进行中订单不受影响",
  "scope": "cross_repo",
  "repos": [
    { "alias": "admin-web",   "project_resource_id": "…a1", "role": "driver" },
    { "alias": "billing-api", "project_resource_id": "…b2", "role": "under_test",
      "path_globs": ["internal/pricing/**"] },
    { "alias": "mobile-app",  "project_resource_id": "…c3", "role": "verifier" }
  ],
  "steps": [
    { "index": 1, "repo": "admin-web",  "action": "将套餐 A 单价从 99 改为 129 并保存",
      "expected": "保存成功，审计日志出现一条调价记录" },
    { "index": 2, "repo": "mobile-app", "action": "新建订单并查看订单页",
      "expected": "展示 129；调价前创建的进行中订单仍按 99 结算" }
  ]
}
```

### 2.4 计划与执行

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
  id              UUID PRIMARY KEY DEFAULT gen_random_uuid(),
  workspace_id    UUID NOT NULL,
  project_id      UUID NOT NULL,
  plan_id         UUID,                    -- 允许临时轮次（直接勾一批用例）
  title           TEXT NOT NULL,
  executor_type   TEXT NOT NULL CHECK (executor_type IN ('member','agent')),
  executor_id     UUID NOT NULL,
  agent_task_id   UUID,                    -- executor_type='agent' 时的任务句柄
  environment     TEXT NOT NULL DEFAULT '',
  build_ref       TEXT NOT NULL DEFAULT '',
  capability_binding JSONB NOT NULL DEFAULT '{}',  -- §6，冻结的能力解析结果
  status          TEXT NOT NULL DEFAULT 'pending'
                  CHECK (status IN ('pending','running','completed','aborted','blocked')),
  source_run_id   UUID,                    -- 重试来源，形成重试链
  retry_scope     TEXT CHECK (retry_scope IN ('all','failed_only','selected')),
  error           TEXT,
  started_at      TIMESTAMPTZ, completed_at TIMESTAMPTZ,
  created_by      UUID,
  created_at      TIMESTAMPTZ NOT NULL DEFAULT now(),
  updated_at      TIMESTAMPTZ NOT NULL DEFAULT now()
);

CREATE TABLE test_run_case (
  id               UUID PRIMARY KEY DEFAULT gen_random_uuid(),
  workspace_id     UUID NOT NULL,
  run_id           UUID NOT NULL,
  test_case_id     UUID NOT NULL,
  case_snapshot    JSONB NOT NULL,          -- 执行时的用例快照
  position         INT  NOT NULL DEFAULT 0,
  result           TEXT NOT NULL DEFAULT 'pending'
                   CHECK (result IN ('pending','running','passed','failed','blocked','skipped')),
  notes            TEXT NOT NULL DEFAULT '',
  evidence         JSONB NOT NULL DEFAULT '[]',   -- [{attachment_id, kind, markdown_url, captured_at}]
  step_results     JSONB NOT NULL DEFAULT '[]',   -- [{index, result, actual, note}]
  duration_ms      INT,
  executed_by_type TEXT CHECK (executed_by_type IN ('member','agent')),
  executed_by_id   UUID,
  executed_at      TIMESTAMPTZ,
  defect_issue_id  UUID,                    -- 失败一键开 issue 的回链
  created_at       TIMESTAMPTZ NOT NULL DEFAULT now(),
  updated_at       TIMESTAMPTZ NOT NULL DEFAULT now()
);
```

`case_snapshot` 是刻意的：用例后续被修改不得改写历史轮次结果，否则回归数据失真。

**重试**（需求 6）通过新建一个 `source_run_id` 指向原轮次的新 run 实现，而不是就地重置结果 —— 留痕不可被覆盖。`retry_scope='failed_only'` 时只复制上一轮 `failed`/`blocked` 的 `test_run_case`。

---

## 3. AI 生成流程

复用 `design_restore_task` 的成熟骨架（`server/internal/handler/design_file.go:2606` 起、`server/internal/handler/daemon.go:2985/3222/3923`），并修掉该实现里已知的几处缺陷。

```
创建 job (queued)
   └─ 生成计划 plan (draft)              ← 服务端规则引擎产出「覆盖哪些仓库/路径/模块/业务规则」
        └─ 人工编辑 + 审批 (approved)
             └─ 派发 → CreateQuickCreateTask(context.type = "test_generation")
                  └─ daemon 认领 → buildTestGenerationPrompt
                       └─ agent 读文档/代码/issue/业务知识 → multica testcase propose --stdin
                            └─ agent 输出 TEST_GENERATION_RESULT_JSON: 汇总
                                 └─ /complete 钩子写 job.result + 状态
```

### 3.1 计划先行的理由

计划这一层不是仪式感。生成任务一旦派发就会烧掉整个上下文窗口，范围错了代价很高。计划阶段让人先确认"这次覆盖哪 3 个仓库的哪些目录、依据哪几条业务规则、预计产出多少条"，成本极低。

`plan` JSON 形状：

```json
{
  "version": "1.0",
  "repos": [{ "project_resource_id": "…", "alias": "billing-api", "path_globs": ["internal/pricing/**"] }],
  "issues": ["MUL-1234", "MUL-1250"],
  "modules": ["订单", "计费"],
  "knowledge_refs": ["qa-domain-knowledge/references/billing-rules.md"],
  "attachment_ids": ["…"],
  "expected_case_types": ["business_flow", "boundary", "permission"],
  "existing_case_digest_count": 42,
  "instructions": "重点覆盖跨时区结算"
}
```

### 3.2 与既有实现的差异（刻意修正）

| design_restore 的问题 | 本设计的做法 |
| --- | --- |
| `skip_plan` 开关可完全绕过人工审批，前端还是个"开发模式"复选框 | **不提供该开关**。审批是真实闸门 |
| 派发前不校验服务端可行性，只靠 UI 禁用按钮；重复派发会孤儿化前一个 task | 派发前检查 job 状态非 `running`，并发派发返回 409 |
| 用输出里是否含"blocked"/"阻塞"子串判定失败 | **只信 agent 声明的 `status` 字段** |
| `UpdateXxx` 的 `error = sqlc.narg('error')` 没有 COALESCE，任何部分更新都会把 error 置空 | 全部字段用 `COALESCE(sqlc.narg(...), col)` |
| `GetXxxByAgentTask` 没有 workspace 过滤 | 一律带 `workspace_id` |
| 没有 websocket 事件，靠 3 秒轮询 | 定义 `test_generation_job:updated` 等事件（§10），不轮询 |
| `CreateQuickCreateTask` 未传归因字段 | 传全 `originator_user_id` / `accountable_user_id` / `originator_source` |
| 结果解析靠 stdout 抓 `RESTORE_RESULT_JSON:` | 用例本体走 **CLI 写库**（`multica testcase propose`），stdout 只回传一份**汇总**用于 job.result。用例数据不经过文本抓取 |

最后一条是关键分歧：设计还原任务的产物是少量映射关系，抓 stdout 尚可接受；测试用例是几十上百条结构化记录，必须走鉴权 API 写库，且服务端做 JSON schema 强校验。

---

## 4. 审查、编辑与增量

### 4.1 新增用例

生成的新用例直接写入 `test_case`，`status='draft'`、`origin='ai'`、`generation_job_id` 回填。列表页默认有"AI 生成待审"筛选。人工可逐条编辑 / 通过 / 删除；通过后 `status='active'`、`reviewed_by`/`reviewed_at` 落章。支持批量通过。

### 4.2 修订与废弃走 proposal

AI 的修订/废弃建议**不能直接覆盖**已定稿的 `active` 用例 —— 否则人工审查过的内容会被静默改掉，"人工审查"这个承诺就是假的。所以：

- `new` → 直接落 `test_case(draft)`；
- `update` / `obsolete` → 落 `test_case_proposal(pending)`，在用例详情页以 diff 形式呈现，人工采纳时写 `test_case_revision` 快照并 bump `version`。

对 `status='draft'` 的用例，AI 的 `update` 可直接改写（尚未定稿，无需 diff 审查），避免第二次生成在 draft 上堆 proposal。

### 4.3 增量感知

生成任务的 brief 里注入当前 project 已有用例的**摘要清单**（`case_number` + `title` + `module` + `status`，不含步骤正文，控制 token），并在提示词里要求输出分为 `new` / `update` / `obsolete` 三类。摘要清单通过 `multica testcase list --project <id> --digest --output json` 由 agent 自行拉取，而不是塞进逐字节稳定的 brief（见 §7 的 MUL-5377 约束）。

### 4.4 人工补录

人工新建用例走同一张表，`origin='human'`、`status` 直接 `active`。CRUD 权限与 issue 一致（工作区成员可写）。AI 与人工用例在列表里用 `origin` 徽章区分，但**能力完全对等**：都能编辑、分组、加入计划、被智能体读取（需求 4）。

---

## 5. 执行、留痕与可观测（需求 6）

### 5.1 人工执行

轮次详情页逐条勾选 `passed` / `failed` / `blocked` / `skipped`，可填 `notes`、传截图（走现有附件系统）。失败一键创建缺陷 issue，回写 `defect_issue_id`，issue 描述自动带上用例快照与轮次链接。

### 5.2 智能体执行

创建轮次时 `executor_type='agent'`，派发路径与 §3 相同（`context.type = "test_run"`）。智能体：

1. `multica test run get <run-id> --output json` 拿到轮次与用例清单；
2. `multica test capability list --run <run-id>` 确认可用能力（§6）；
3. 按 `test_case_repo` 逐个 `multica repo checkout <url>`；
4. 逐条执行，`multica test result set <run-case-id> --result passed --note "…"`；
5. `multica test evidence add <run-case-id> --file ./shot.png --kind screenshot`；
6. 失败时 `multica test defect open <run-case-id> --title "…"` 创建缺陷 issue。

**权限**：智能体用任务态 `MULTICA_TOKEN`（`X-Actor-Source == "task_token"`）调用上述写接口。写权限限制在**本次 run 的 run-case** 上 —— token 绑定 (agent, task)，服务端校验 `test_run.agent_task_id == 中间件注入的 X-Task-ID`。读接口（用例、项目、仓库、issue）对任何智能体开放（需求 5），写接口严格绑定轮次。

### 5.3 可观测

- 轮次详情页展示实时进度（已执行/总数、通过率、耗时）。
- 复用 `designRestoreExecutionStatusToResponse`（`server/internal/handler/design_file.go:1630`）的做法，join `agent_task` + `agent_runtime` + 最新 task message，派生一个合成的 `execution_status {phase, reason, severity}`，用来回答"agent 是不是卡住了"，无需新增列。
- 轮次页可直接跳转到底层 agent task 的会话，看到智能体每一步在做什么。

### 5.4 重试

轮次详情页「重跑」→ 选 `全部` / `仅失败` / `选中项` → 新建 run，`source_run_id` 指向原轮次。原轮次记录不可变。用例列表页可看某条用例的历史结果时间线（跨轮次），这是回归价值的核心视图。

---

## 6. 测试执行能力接口层（需求 7）

真机、桌面、浏览器控制由你的外部工作独立交付。本层职责是**先把插槽定义好**：让 `test_case` 声明"我需要什么"，让 `test_run` 解析出"谁能提供"，等设备控制落地时只需填一个 provider。

**结论先行：能力平面就是 MCP。** 仓库里已经把 MCP 当作 provider 无关的能力平面（`server/pkg/agent/browser_mcp_config.go` 已经识别并加固 `playwright` / `chrome-devtools`），且 `ExecOptions.McpConfig` 是 20 个 agent backend 共用的字段。走 MCP 的能力**零 backend 改动**；不走 MCP 的能力要教会 20 个 backend。

### 6.1 能力表

```sql
CREATE TABLE test_capability (
  id             UUID PRIMARY KEY DEFAULT gen_random_uuid(),
  workspace_id   UUID NOT NULL,
  daemon_id      TEXT NOT NULL,            -- 能力永远绑在一台物理机器上
  runtime_id     UUID,
  kind           TEXT NOT NULL CHECK (kind IN ('android_device','ios_device','computer_use','browser')),
  capability_key TEXT NOT NULL,            -- 稳定标识，如 'android:emulator-5554'
  target         JSONB NOT NULL DEFAULT '{}',  -- {serial,model,os_version} / {browser,channel} / {display}
  status         TEXT NOT NULL DEFAULT 'unknown'
                 CHECK (status IN ('available','busy','offline','unknown')),
  probe          JSONB NOT NULL DEFAULT '{}',  -- {ok, reason, probed_at, provider_version}
  last_probe_at  TIMESTAMPTZ,
  created_at     TIMESTAMPTZ NOT NULL DEFAULT now(),
  updated_at     TIMESTAMPTZ NOT NULL DEFAULT now()
);
-- 单独迁移：UNIQUE (workspace_id, daemon_id, capability_key)；INDEX (workspace_id, kind, status)
```

`target` 用 JSONB 而非列，理由与 `project_resource.resource_ref` 一致：新增能力种类只改一个校验分支，零迁移。

### 6.2 声明与解析

`test_case.required_capabilities` 存**需求**而非实例：

```json
[{"kind":"android_device","match":{"os_version":">=13"},"optional":false},
 {"kind":"browser","match":{"browser":"chromium"}}]
```

派发轮次时 `resolveRunCapabilities(ctx, wsID, requirements)` 做一次约束求解：按 `kind + match` 过滤 `status='available'` 的行，要求**同一 `daemon_id` 覆盖全部必需项**（跨机执行不在 v1 范围），得到 `(daemon_id, runtime_id)`。无解则 run 直接落 `blocked`，`error` 写明缺哪个 kind，**不进队列**。解析结果冻结进 `test_run.capability_binding`，`test_run_case` 只引用 `capability_key`，保证重跑可复现。

### 6.3 接入点（全部是现有文件）

| 动作 | 文件 |
| --- | --- |
| daemon 侧枚举能力、脱敏回传 | 复制 `server/internal/handler/runtime_local_skills.go` 的 request/poll/report store；daemon 侧新增 `listRuntimeCapabilities`，与 `server/internal/daemon/runtime_mcp.go:207` 的 `listRuntimeLocalMcpServers` 并列 |
| 派发前注入 MCP overlay | `server/internal/service/task.go:288` `buildRuntimeMCPOverlay` —— 服务端唯一为第二个 capability provider 预留的缝（Composio 是第一个） |
| 认领时合并进 agent 配置 | `server/internal/handler/mcp_overlay.go:36` `mergeMCPOverlay`，**无需改动** |
| 本机 stdio 能力（adb / computer-use）不出网 | `server/internal/daemon/runtime_mcp.go:35` `mergeRuntimeAndAgentMcpConfig` 下沉一层；配合 `multica mcp serve device`（仿 `server/cmd/multica/mcp_design.go`），multica 二进制已在每个任务的 PATH 里，零安装 |
| 任务与机器绑定 | `server/internal/daemon/local_directory.go:66` `findLocalDirectoryAssignment` 的同构兄弟；批量认领已强制 `daemon_id` 校验（`server/internal/handler/daemon.go:1445`），直接复用 |
| 浏览器能力（今天就能跑） | `server/pkg/agent/browser_mcp_config.go` 已识别 `playwright` / `chrome-devtools` |
| 告知智能体已挂载能力 | `server/internal/daemon/execenv/runtime_config_sections.go:200` `BuildConnectedAppsBlock` —— **进 per-turn 消息，不进 brief**，否则破坏 MUL-5377 的 prompt-cache 前缀稳定性 |

### 6.4 智能体侧契约

```bash
multica test capability list --run <run-id> --output json
# [{"capability_key":"android:emulator-5554","kind":"android_device",
#   "mcp_server":"multica-device","target":{"serial":"emulator-5554"}}]
```

具体操作一律走 MCP（`multica-device` / `multica-browser`），CLI 只负责**发现**和**回执** —— 与 brief 里 `writeAlwaysUseCLI` 禁止 `curl` 的原则一致。daemon 另注入 `MULTICA_TEST_RUN_ID`、`MULTICA_TEST_CAPABILITIES`；因 `isBlockedEnvKey`（`server/internal/daemon/daemon.go:6723`）拒绝一切 `MULTICA_*` 键进入 `custom_env`，这两个变量不可被 agent owner 伪造。

内置技能 `multica-running-tests` 写死一条规则：**先 `capability list`，只使用返回的 `capability_key`；列表为空即判 `blocked` 并说明原因，禁止自行寻找 adb / 浏览器。**

### 6.5 证据回流

复用现有附件系统，不新建存储：`attachment` 加一个可空 `test_run_case_id` 列（无外键，索引单独迁移），`UploadFile` 增加同名表单字段，校验沿用 task_token 三重门（`X-Actor-Source == "task_token"` + 表单 task_id 等于中间件注入的 `X-Task-ID` + 上传者是该任务的 agent）。随后把 `{attachment_id, kind, markdown_url, captured_at}` 追加进 `test_run_case.evidence` —— **只存 id 与 `markdown_url`，绝不存 `download_url`**（签名 URL 会过期）。

### 6.6 现在建 vs 留桩

**现在建（不依赖设备工作）**：`test_capability` 表与查询、`required_capabilities` 声明、`resolveRunCapabilities`、`capability_binding` 冻结、`multica test capability list` / `test evidence add`、附件列与链接查询、`multica-running-tests` 技能，以及**浏览器 kind 的端到端联通** —— 用户自带 playwright MCP 即可跑通全链路，作为本层的验收用例。

**留桩（等设备控制落地）**：`capabilityMCPServers(kind, target)` 当前对 `android_device` / `ios_device` / `computer_use` 返回空 map，因此 overlay 为空、`status` 恒为 `unknown`、涉及这些 kind 的 run 直接 `blocked` 并给出"能力未注册"文案 —— 这是**刻意的显式失败，不是静默降级**。设备工作落地时的全部改动是在这个函数和 `listRuntimeCapabilities` 里各加一个分支。测试域的表、CLI、技能、证据链路都不再改动。

**不要做**：不要为设备再造一套 runtime。`runtime_profile` 的铁律（`server/internal/handler/runtime_profile.go:28`）是只承载进入兼容模式所需的 `fixed_args`，把"设备 runtime"塞进 profile 会在评审被拒。

---

## 7. 业务知识注入与多仓库上下文（需求 2、3）

### 7.1 现有可复用的持久知识源

仓库里**没有 RAG、没有 embedding、没有 wiki 实体**（`pgvector/pgvector:pg17` 只是基础镜像，实际启用的扩展是 pgcrypto / pg_bigm / pg_trgm / pg_cron）。业务知识今天通过五条显式、用户可见、可编辑的文本通道到达智能体，全部在 claim 时装配进 brief：

| 知识源 | 存储 | 渲染 |
| --- | --- | --- |
| 工作区级不变量（产品词汇、通用业务规则） | `workspace.context` | `writeWorkspaceContext` |
| 项目级业务规则 | `project.description` | `writeProjectContext` |
| Agent 人格与强制流程 | `agent.instructions` | `writeAgentIdentity`，**优先级高于运行时 workflow** |
| 多文件业务文档 | `skill` + `skill_file` | `writeSkills` 只列 slug；正文落盘到 `.claude/skills/<slug>/references/*.md` |
| 代码与目录指针 | `project_resource` | `writeRepositories` + `.multica/project/resources.json` |

历史决策（常常只存在于评论里）走拉取：`multica issue search` 覆盖标题、描述**和评论正文**。

**装配约定**：brief 必须在同一会话内逐字节稳定（MUL-5377，prompt cache 前缀），所以 brief 只放**索引**（技能 slug、仓库清单、项目描述）；随任务变化的部分（本次覆盖哪些仓库、哪些路径、已有用例摘要）放进 per-turn 消息或让 agent 主动拉取。

### 7.2 让智能体懂业务的落地方案

**零代码改动，立即可做**（这是主路径）：

1. 建工作区技能 `qa-domain-knowledge`，`references/` 下放 `billing-rules.md`、`permission-matrix.md`、`entity-lifecycle.md`、`glossary.md`。挂到 QA 智能体。
   注意 `writeSkills` **只输出技能名**，模型是否打开完全取决于 frontmatter `description` —— 必须写成触发句而非目录，例如：
   `description: "Use when writing or reviewing test cases for billing, permissions, or entity lifecycle. Contains authoritative business rules — read references/ before proposing any test case."`
2. `workspace.context` 放 20–40 行永远成立的产品词汇与铁律，它无条件进入**每个**任务的 brief。
3. `project.description` 放该产品线的 QA 规则。
4. QA 智能体的 `instructions` 里写死："生成用例前必须 (a) 读完 `qa-domain-knowledge` 下所有 references，(b) 跑 `multica issue search <关键词>` 找评论里的历史决策。"

**一处小改动，收益最高**：在 `validateAndNormalizeResourceRef`（`server/internal/handler/project_resource.go:70`）加 `case "document": {url, title, summary}`，并在 `writeProjectContext` 加一条渲染规则。`resource_type TEXT` + `resource_ref JSONB` 本就是多态的，**零迁移**，外部 PRD / 接口文档立刻成为一等项目上下文。

**可选的第三步**：若希望智能体能**沉淀**新学到的业务结论（`issue_metadata` 是 issue 级且明确只放短值），再加一张轻量 `project_knowledge(project_id, key, body, author_type, reviewed_at)`。现有五条通道全是人写的，没有任何项目级、agent 可写、人可审的落点 —— `server/internal/daemon/execenv/codex_memory.go:46` 自己也承认这是产品欠账。**建议放到二期**，先用技能跑通。

### 7.3 多仓库：今天能不能一个任务多仓库

**能，但是机会主义的。** `server/internal/handler/daemon.go` 把项目下**每一条** `github_repo` 都追加进 `resp.Repos`；daemon 侧 `registerTaskRepos` 维护 `taskRepoRefs[taskID][repoURL]` 的按仓库默认 ref；`repocache` 的 `worktreePath := filepath.Join(WorkDir, dirName)` 让 N 次 `multica repo checkout <url>` 落成 workdir 下的兄弟目录。生成 agent 和执行 agent 走的是同一套 claim 载荷。

**必须先修三件事**（见 §12）：目录名冲突、无人主动 checkout、项目仓库整体覆盖工作区仓库。

跨仓库**没有原子性**：每个 checkout 是独立 git 树，无联合提交、无 PR 关联。用例只能表达先后顺序，不能表达事务 —— 这是产品上要接受的限制。

### 7.4 上下文收敛

上下文窗口是硬约束，大仓库不可能整体进 prompt。四层收敛：

- **任务粒度**：一次生成 = 一个 issue 或一个模块，不是整个项目。issue 未绑定 project 时 `writeProjectContext` 直接早退，所以生成任务**必须**绑定项目。
- **路径过滤**：`path_globs` 同时用于生成期（只读哪些目录）和执行期（只 checkout 需要的仓库）。
- **知识按需读**：brief 只承载技能 slug 与触发语，正文靠 `references/*.md` 落盘后由模型自行打开。
- **计划先行**：错误的范围在烧掉整个上下文窗口之前就被人拦下。

⚠️ 自设约束：`UpsertSkillFile`（`server/internal/handler/skill.go:2343`）**没有大小上限**（1 MiB/文件只限 URL 导入路径）。一份 50 MB 的业务文档会被原样落到每台执行机磁盘并撑爆预算。需要在测试域文档里写明并考虑加服务端守卫。

---

## 8. Agent 契约

### 8.1 CLI：`multica testcase` / `multica test`

按 `server/cmd/multica/cmd_project.go` 的形状新增两个命令组：

```bash
# 用例库（任何智能体可读 —— 需求 5）
multica testcase list --project <id> [--module M] [--status active] [--digest] --output json
multica testcase get <TC-42|uuid> --output json          # 含关联的 repos / project / source issues
multica testcase create --title … --steps-stdin --output json
multica testcase update <id> --status active
multica testcase delete <id>
multica testcase propose --job <job-id> --stdin          # 生成任务批量回写（new/update/obsolete）

# 计划与执行
multica test plan list|get|create
multica test run get <run-id> --output json
multica test run start <run-id>
multica test result set <run-case-id> --result passed|failed|blocked|skipped [--note …] [--step-results-stdin]
multica test evidence add <run-case-id> --file ./shot.png --kind screenshot
multica test defect open <run-case-id> --title … --output json
multica test capability list --run <run-id> --output json
```

实现要点：
- 命令注册在 `server/cmd/multica/main.go` 的 core 组，紧邻 `issueCmd` / `projectCmd`；
- 长文本走 `resolveTextFlag`（自动派生 `--x-stdin` / `--x-file`，并强制 `ensureFileFlagWithinWorkdir`）；
- 批量 JSON 走 `cmd.InOrStdin()`（不是 `os.Stdin`）以便单测，参照 `resolveCustomEnv`（`cmd_agent.go:1136`）；不要发明 NDJSON 约定；
- 人类可读 id `TC-42` 走服务端解析（`GET /api/test-cases/{ref}`），参照 `looksLikeIssueIdentifier`；
- 新增默认命令名要加进 `scripts/agent-cli-command-names.txt`，否则 Linux/macOS 测试入口会因环境里存在同名 CLI 而失败。

### 8.2 内置技能

新增两个（目录建好即注册，`multica-` 前缀是强制的）：

- `server/internal/service/builtin_skills/multica-test-cases/` —— 用例的数据契约、字段语义、增量生成的 new/update/obsolete 规则、`propose` 的 JSON schema。
- `server/internal/service/builtin_skills/multica-running-tests/` —— 轮次执行流程、能力发现规则、证据上传、失败开 issue、结果语义（`blocked` 与 `failed` 的区别）。

每个都要配 `references/*-source-map.md`（`行为 | 文件:行` 表 + 底部 grep 验证块），并在 `server/internal/service/builtin_skills_test.go` 加 eval 测试。CLAUDE.md 要求 CLI/API/行为变更与 SKILL.md 同 PR 更新。

### 8.3 QA 智能体

不是新实体 —— 就是一个普通 agent，挂上 `multica-test-cases` + `multica-running-tests` + `qa-domain-knowledge`，`instructions` 写死生成前必读业务知识。**任何**智能体都能读用例（需求 5）；QA 智能体只是配置好的默认执行者。

---

## 9. 前端结构（需求 1）

### 9.1 侧边栏 tab

新增一个与"设计"同级的工作区 tab。它不是一处注册，而是四层契约，每层都有测试守着：

| 层 | 文件 | 改动 |
| --- | --- | --- |
| 路由 | `packages/core/paths/paths.ts` | `tests: () => \`${ws}/tests\`` + 详情路由 `testCaseDetail` / `testPlanDetail` / `testRunDetail` / `testGenerationJobDetail` |
| 注册表 | `packages/core/paths/route-icons.ts` | `WORKSPACE_PAGES.tests = { segment: "tests", icon: "FlaskConical", navKey: "tests" }`，并扩 `RouteIconName` / `NavLabelKey` / `WorkspacePageKey` |
| 图标 | `packages/views/layout/route-icon-components.tsx` | 加 `FlaskConical` 映射（缺了是编译错误） |
| 侧边栏 | `packages/views/layout/app-sidebar.tsx` | `workspaceNav` 加 `{ key: "tests", labelKey: "tests" }`；注意该文件**自带一份重复的 NavKey/NavLabelKey 联合**，两处都要加 |
| i18n | `packages/views/locales/{en,zh-Hans,ja,ko}/layout.json` | `nav.tests`。四语齐全，否则 `parity.test.ts` 挂 |
| 遥测 | `packages/core/diagnostics/diagnostic-context.ts` | `WORKSPACE_ROUTES` 加 `["tests"]` 及各详情模板 |
| 测试 | `packages/core/paths/consistency.test.ts`、`packages/views/layout/app-sidebar.test.tsx` | 两处硬编码清单都要加，否则整个 suite 崩 |

⚠️ `packages/views/eslint.config.mjs` 里 `i18next/no-literal-string` 对 `packages/views/**/*.tsx` 是 **error**，只对 `designs/**` 网开一面。新 tab **不享受豁免**，所以必须从第一天就建 `packages/views/locales/{4 语}/testing.json` 命名空间并注册进 `packages/views/locales/index.ts` 的 `RESOURCES`。

### 9.2 目录命名

`packages/views/test/` 已被测试工具占用（`i18n.tsx` / `setup.ts`），所以业务目录用：

- `packages/core/testing/`（keys.ts / queries.ts / mutations.ts / config.ts）
- `packages/views/testing/`（页面与组件）
- 路由段用 `tests`（单词，符合根路由命名规则）

### 9.3 页面

`packages/views/testing/` 下：

- `test-cases-page.tsx` —— 主列表。左侧 project + module 树，主区列表（`TC-42` / 标题 / 类型 / 优先级 / 状态 / 来源徽章 / 关联仓库 chips / 最近结果），顶部筛选（待审 / 状态 / 优先级 / 类型 / 标签），右上「AI 生成」「新建用例」。
- `test-case-detail.tsx` —— 用例详情 + 步骤编辑器（结构化行编辑，非 markdown）、关联仓库编辑、待处理 proposal 的 diff 面板、版本历史、跨轮次结果时间线。
- `test-generation-job-page.tsx` —— 计划面板（生成 / 编辑 / 审批）+ 派发面板（选 agent）+ 结果面板。布局照抄 `packages/views/designs/design-restore-task-page.tsx` 的两栏结构。
- `test-plans-page.tsx` / `test-plan-detail.tsx`
- `test-run-detail.tsx` —— 逐条结果、证据、执行诊断、重试入口。
- 纯函数（结果聚合、diff 计算、能力匹配）放独立 `.ts` 文件以便无 React 单测，参照 `design-restore-result.ts`。

平台接线：`apps/web/app/[workspaceSlug]/(dashboard)/tests/**/page.tsx` 6–8 行 `"use client"` 壳；`apps/desktop/src/renderer/src/routes.tsx` 加同样一组 session 路由（带 `handle.title`）。

### 9.4 数据层

严格按现有三层：

- `packages/core/api/schemas.ts` 加 `TestCaseSchema` 等（`.loose()`）+ `EMPTY_*`，**每个端点都走 `parseWithFallback`**（不要重复 design_restore 里 plan/mappings 端点绕过 schema 的错误）；
- `packages/core/api/schemas.test.ts` 每个新 schema 至少一个畸形响应测试（CLAUDE.md 强制）；
- `packages/core/testing/keys.ts` —— 所有 key 第二段是 `wsId`；
- `packages/core/testing/mutations.ts` —— 状态切换、优先级、单条结果打点适用乐观更新；创建/删除/派发/审批**必须等服务端返回**再导航或清理；
- `packages/core/package.json` 的 `exports` 加 `./testing`、`./testing/queries`、`./testing/mutations`（漏了会以不透明方式报错）。

---

## 10. API 与实时事件

### 10.1 端点

注册在 `server/cmd/server/router.go` 的 workspace-scoped 组内（字面量子路径要注册在 `/` 之前）：

```
GET    /api/test-cases                          列表（project_id/module/status/priority/origin/label 过滤）
POST   /api/test-cases
GET    /api/test-cases/{ref}                    ref 可为 UUID 或 TC-42
PUT    /api/test-cases/{id}
DELETE /api/test-cases/{id}
POST   /api/test-cases/{id}/approve             draft → active
POST   /api/test-cases/batch-approve
GET    /api/test-cases/{id}/revisions
GET    /api/test-cases/{id}/results             跨轮次结果时间线

GET    /api/test-cases/{id}/proposals
POST   /api/test-case-proposals/{id}/accept
POST   /api/test-case-proposals/{id}/reject

POST   /api/test-generation-jobs                创建
GET    /api/test-generation-jobs
GET    /api/test-generation-jobs/{id}
GET    /api/test-generation-jobs/{id}/plan
POST   /api/test-generation-jobs/{id}/plan/generate
PUT    /api/test-generation-jobs/{id}/plan
POST   /api/test-generation-jobs/{id}/plan/approve
POST   /api/test-generation-jobs/{id}/dispatch
POST   /api/test-generation-jobs/{id}/propose   agent 批量回写（task_token）

GET    /api/test-plans        POST /api/test-plans        GET/PUT/DELETE /api/test-plans/{id}
POST   /api/test-plans/{id}/cases                        PUT 排序 / DELETE 移除

GET    /api/test-runs         POST /api/test-runs         GET /api/test-runs/{id}
POST   /api/test-runs/{id}/dispatch                      派发给 agent
POST   /api/test-runs/{id}/retry                         {scope: all|failed_only|selected}
PUT    /api/test-run-cases/{id}/result                   人或 agent 打点
POST   /api/test-run-cases/{id}/defect                   一键开缺陷 issue

GET    /api/test-capabilities                            列出工作区可用能力
POST   /api/runtimes/{id}/capabilities                   请求某 runtime 枚举本机能力
```

后端 UUID 规则：路径参数可能是 `TC-42` 的走 `loadTestCaseForUser` 解析后用 `entity.ID` 写库；纯 UUID 输入用 `parseUUIDOrBadRequest`。

### 10.2 实时事件

在 `server/pkg/protocol/events.go` 加：

```go
EventTestCaseCreated  = "test_case:created"
EventTestCaseUpdated  = "test_case:updated"
EventTestCaseDeleted  = "test_case:deleted"
EventTestGenerationJobUpdated = "test_generation_job:updated"
EventTestRunUpdated   = "test_run:updated"
EventTestRunCaseUpdated = "test_run_case:updated"
```

在 `packages/core/realtime/use-realtime-sync.ts` 的 `refreshMap` 注册失效器即可让增删改实时生效。**明确不复制 design_restore 的 3 秒轮询** —— 那是因为它没有事件才有的补丁。轮次执行时 `test_run_case:updated` 让进度实时跳动，这是"可观测"的核心体验。

WebSocket 事件只做 Query 缓存失效或补丁，绝不把服务端数据镜像进 Zustand。

---

## 11. 权限模型

- 用例的读写权限跟 issue 一致：工作区成员可读可写。
- `draft → active` 的审批：工作区成员即可（不引入新角色，YAGNI）。若后续需要限制，再加。
- 智能体读：任何智能体凭任务态 token 可读用例、计划、轮次（需求 5）。
- 智能体写：`test result set` / `evidence add` / `defect open` 限制在**本次 run** 的 run-case 上，服务端校验 `test_run.agent_task_id == X-Task-ID`。
- `testcase propose` 限制在**本次 job** 上，同样的绑定校验。
- 能力可见性：非机密摘要走成员级 `requireRuntimeCapabilityReadAccess`；任何 target 细节（序列号等）不外泄到工作区其他成员之外。

⚠️ 需要显式记录的风险：Composio overlay 遵循的是**智能体调用权限**而非所有权（`server/internal/integrations/composio/dispatch.go:60`）—— 谁能 @ 这个智能体，谁就继承了 owner 的连接。设备能力继承同样的爆炸半径：**能 @ QA 智能体的人就能驱动 owner 的手机**。这不是本设计引入的，但设备能力会显著放大它，必须在设备工作落地时一并处理（至少给一个 per-run 确认闸门）。

---

## 12. 必须先修的既有缺陷

这三条不修，多仓库测试用例就是纸面功能：

1. **workdir 目录名冲突（真 bug）**：`repoNameFromURL`（`server/internal/daemon/repocache/cache.go:1356`）只取 URL basename，`org-a/app` 与 `org-b/app` 都落到 `{workdir}/app`；第二次 checkout 走 `updateExistingWorktree`，用 B 的 baseRef 更新 A 的树，**静默出错、无报错**。同文件里 `bareDirName` 已用 host+path 限定正确解决了缓存目录的同名问题（有测试在 `cache_test.go:132/147`），照搬到 workdir 路径即可，可只在冲突时加前缀以免改动现有单仓库布局。
2. **无人主动 checkout**：`writeProjectContext` 现在的措辞是"资源只是指针，只在相关时才打开"，反而抑制多仓库。需要 `multica repo checkout --all`，或在 `execenv.Prepare` 里按 `test_case_repo` 预检出 —— daemon 侧不需要新管线，`ws.taskRepoRefs` 已有每仓库 ref。
3. **项目仓库整体覆盖工作区仓库**：`if len(projectRepos) > 0 { resp.Repos = projectRepos }` 的 else 分支意味着挂一个项目资源就会让工作区其余仓库从 checkout 白名单消失。跨仓库项目必须把所有相关仓库都登记为项目资源 —— 这要写进用户文档，或改成合并。

---

## 13. 测试策略

| 测试什么 | 位置 |
| --- | --- |
| 用例 CRUD / 生成 job 状态机 / 轮次结果聚合 / 能力解析 | `server/internal/handler/test_*_test.go` |
| propose 批量写入的 schema 校验与拒绝路径 | `server/internal/handler/test_generation_test.go` |
| task_token 越权写（改别的 run 的结果）必须 403 | `server/internal/handler/test_run_permission_test.go` |
| 查询键、mutation、畸形响应 | `packages/core/testing/*.test.ts`、`packages/core/api/schemas.test.ts` |
| 页面、步骤编辑器、proposal diff 面板 | `packages/views/testing/*.test.tsx` |
| 侧边栏新 tab | `packages/views/layout/app-sidebar.test.tsx`、`packages/core/paths/consistency.test.ts` |
| 内置技能契约 | `server/internal/service/builtin_skills_test.go` |
| 端到端：生成 → 审批 → 派发 → 审查 → 建计划 → 跑轮次 → 重试 | `e2e/testing.spec.ts`（用 `TestApiClient` 做 setup/teardown） |

规则：默认测试**绝不**解析或执行用户安装的 agent CLI，也不得触碰真实设备；真机冒烟测试放 `agentintegration` build tag 后并检查 `MULTICA_RUN_REAL_AGENT_SMOKE=1`。行为性改动优先在正确的包里先写失败测试。

---

## 14. 分期交付

**第一期 —— 用例库（可独立上线）**
`test_case` / `test_case_revision` / `test_case_repo` + CRUD API + 侧边栏 tab + 列表页 + 详情页 + 步骤编辑器 + 分组 + `multica testcase` 只读命令 + `multica-test-cases` 技能。
验收：人工能建、编、分组用例；任何智能体能通过 CLI 读到用例及其关联的项目/仓库/issue。**需求 1、4、5 完成。**

**第二期 —— AI 生成与审查**
`test_generation_job` / `test_generation_plan` / `test_case_proposal` + 计划生成与审批 + 派发 + `testcase propose` 回写 + 增量 new/update/obsolete + 待审列表与 diff 面板 + 业务知识方案落地（`qa-domain-knowledge` 技能 + `document` 资源类型）+ §12 的三处修复。
验收：选一个 project 和几个 issue，产出一批 draft 用例，人工逐条审查通过；第二次生成只出增量。**需求 2、3 完成。**

**第三期 —— 计划、轮次与人工执行**
`test_plan` / `test_plan_case` / `test_run` / `test_run_case` + 人工打点 + 证据上传 + 一键开缺陷 + 重试 + 跨轮次结果时间线 + 实时事件。
验收：建计划、跑一轮人工回归、失败开 issue、重跑仅失败项。**需求 6 的人工部分完成。**

**第四期 —— 智能体执行与能力接口层**
`test_capability` + `resolveRunCapabilities` + `capability_binding` + `multica test` 执行命令组 + task_token 权限 + `multica-running-tests` 技能 + 执行诊断 + 浏览器 kind 端到端打通（设备 kind 留桩）。
验收：把一个轮次派给 QA 智能体，它自主执行、上传截图、回写结果、失败开 issue；轮次页能实时看到进度。**需求 6 的智能体部分、需求 7 完成。**

**第五期（等你的设备控制落地）**
在 `capabilityMCPServers` 与 `listRuntimeCapabilities` 各加分支，接 Android / iOS / computer-use。测试域零改动。

---

## 15. 明确不做

- **不生成自动化测试代码**。用例本身是产物；写测试代码是普通的 issue → agent 流程，不需要新功能。
- **不做测试套件树**。`module` + 标签足够，套件树会带来一层 CRUD 和拖拽排序的成本而收益有限。
- **不做覆盖率统计 / 缺陷分析报表**。等有真实数据再说。
- **不做跨机器执行**。一个轮次的全部必需能力必须由同一台 daemon 提供。
- **不做跨仓库事务语义**。git 层面不存在，产品层不假装存在。
- **不重新启用 provider 原生 memory**。`codex_memory.go` 关闭它是有原因的（跨工作区泄漏，issue #3130）。
- **不引入新角色/权限层**。沿用工作区成员模型。
