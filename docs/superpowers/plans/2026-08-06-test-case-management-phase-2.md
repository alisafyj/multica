# 测试用例管理 · 第二期 实施计划

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development or superpowers:executing-plans. Steps use checkbox (`- [ ]`) syntax.

**Goal:** AI 基于项目文档、仓库代码、需求 issue 与业务知识生成测试用例，人工审查计划、审查产物、采纳增量修订。

**Architecture:** 照搬 `design_restore_task` 的「领域任务表 → 人审计划 → 派发 agent → 回写」骨架，但产物走鉴权 CLI 写库而非 stdout 抓取。新增三张无外键表；生成 agent 在 runtime 里 checkout 多个仓库、读文档与代码、拉已有用例摘要，产出 new / update / obsolete 三类增量。

**Tech Stack:** 同第一期。

**依据:** [spec](../specs/2026-08-05-test-case-management-design.md) §3、§4、§7、§12、§14 第二期。第一期已全部落地并验证（8 个 commit，`fdae56ce5` 为止）。

## Global Constraints

第一期的 Global Constraints 全部继续生效（无外键、索引单独 CONCURRENTLY 迁移、`workspace_id` 过滤、后端 UUID 规则、`parseWithFallback`、i18n 强制、设计令牌、包边界、英文注释）。本期追加：

- **迁移编号从 288 起**（第一期占到 287）。开工前 `ls server/migrations | tail -2` 复核。
- **不要复制 design_restore 的七处已知缺陷**，spec §3.2 逐条列了：无 `skip_plan` 绕过开关；派发前校验 job 非 running；只信 agent 声明的 `status` 字段而非输出里的 "blocked" 子串；所有 narg 用 `COALESCE`；`GetXxxByAgentTask` 必须带 `workspace_id`；定义 websocket 事件而不是 3 秒轮询；`CreateQuickCreateTask` 要传全归因字段。
- **用例数据不走 stdout 抓取**。agent 通过 `multica testcase propose` 写库，服务端做 JSON schema 强校验；stdout 的 `TEST_GENERATION_RESULT_JSON:` 只回传一份汇总用于 `job.result`。
- **brief 逐字节稳定**（MUL-5377）。任何随任务变化的内容放 per-turn 消息或让 agent 主动拉取，绝不进 `buildMetaSkillContentSlim`。
- 第一期既有命名必须复用，不要另造：`TestCaseResponse` / `testCaseToResponse(testCase, repos)` / `loadTestCaseForUser(w, r, ref)` / `formatTestCaseKey(n)` / `writeTestCaseWriteError` / `validateTestCaseRepos` / `testCaseKeys` / `useTestCaseViewStore`。

---

## 工作分组与并行边界

四组，A 是关键链路，C / D 完全独立可并行，B / E 依赖 A 的契约。

| 组 | 内容 | 触碰的文件（互不重叠） |
| --- | --- | --- |
| A | 生成任务后端核心 | `server/migrations/288+`、`server/pkg/db/queries/test_generation.sql`、`server/internal/handler/test_generation*.go`、`server/internal/service/task.go`（新增 context 类型）、`server/internal/handler/daemon.go`（三处钩子）、`server/internal/daemon/prompt.go`、`server/cmd/server/router.go`、`server/pkg/protocol/events.go` |
| B | CLI `propose` + 内置技能更新 | `server/cmd/multica/cmd_testcase.go`、`builtin_skills/multica-test-cases/**` |
| C | `document` 项目资源类型（业务知识） | `server/internal/handler/project_resource.go`、`server/internal/daemon/execenv/runtime_config.go`、`runtime_config_sections.go`、`packages/core/types/project.ts`、`packages/views/projects/components/project-resources-section.tsx`、`server/cmd/multica/cmd_project.go`、`builtin_skills/multica-projects-and-resources/**`、`apps/docs/**` |
| D | §12 多仓库三处修复 | `server/internal/daemon/repocache/cache.go`、`server/cmd/multica/cmd_repo.go`、`server/internal/handler/daemon.go` 的 repos 合并分支 |
| E | 前端：生成任务页 + 审查 diff | `packages/core/types/testing.ts`、`api/schemas.ts`、`api/client.ts`、`packages/core/testing/**`、`packages/views/testing/**`、`packages/core/paths/**`、`apps/web`、`apps/desktop` |

⚠️ C 独占 `runtime_config_sections.go`（`writeProjectContext`）。D 的「无人主动 checkout」修复因此走 CLI `--all` 路线，不碰该文件。
⚠️ A 与 D 都会碰 `server/internal/handler/daemon.go`，但改的是不同函数（A 改 Start/Complete/Fail 钩子与 claim 分支，D 改 `resp.Repos` 的合并分支）。仍需串行落地或人工合并，**不要同时派给两个 agent**。

---

## Task A1: 生成任务三张表

**Files:** `server/migrations/288_test_generation.{up,down}.sql`，`289..294_*_index.{up,down}.sql`

**Produces:** 表 `test_generation_job`、`test_generation_plan`、`test_case_proposal`。

- [ ] **Step 1: 建表迁移** `288_test_generation.up.sql`

```sql
-- AI test case generation. No FOREIGN KEY / cascade by repository rule.
CREATE TABLE test_generation_job (
    id            UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    workspace_id  UUID NOT NULL,
    project_id    UUID NOT NULL,
    agent_id      UUID,
    agent_task_id UUID,
    status        TEXT NOT NULL DEFAULT 'queued'
                  CHECK (status IN ('queued', 'running', 'completed', 'failed', 'cancelled')),
    input         JSONB NOT NULL DEFAULT '{}',
    result        JSONB NOT NULL DEFAULT '{}',
    error         TEXT,
    created_by    UUID,
    created_at    TIMESTAMPTZ NOT NULL DEFAULT now(),
    updated_at    TIMESTAMPTZ NOT NULL DEFAULT now()
);

-- The human-reviewed scope contract: which repos / paths / modules / business
-- rules this run may cover. Approval gates dispatch.
CREATE TABLE test_generation_plan (
    id           UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    workspace_id UUID NOT NULL,
    job_id       UUID NOT NULL,
    status       TEXT NOT NULL DEFAULT 'draft'
                 CHECK (status IN ('draft', 'approved', 'dispatched', 'archived')),
    plan         JSONB NOT NULL DEFAULT '{}',
    review_notes TEXT NOT NULL DEFAULT '',
    approved_by  UUID,
    approved_at  TIMESTAMPTZ,
    created_by   UUID,
    created_at   TIMESTAMPTZ NOT NULL DEFAULT now(),
    updated_at   TIMESTAMPTZ NOT NULL DEFAULT now()
);

-- AI suggestions that must NOT silently overwrite a human-approved case.
-- New cases land directly in test_case(status='draft'); only update/obsolete
-- come through here for side-by-side review.
CREATE TABLE test_case_proposal (
    id             UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    workspace_id   UUID NOT NULL,
    job_id         UUID NOT NULL,
    target_case_id UUID NOT NULL,
    kind           TEXT NOT NULL CHECK (kind IN ('update', 'obsolete')),
    payload        JSONB NOT NULL DEFAULT '{}',
    rationale      TEXT NOT NULL DEFAULT '',
    status         TEXT NOT NULL DEFAULT 'pending'
                   CHECK (status IN ('pending', 'accepted', 'rejected')),
    reviewed_by    UUID,
    reviewed_at    TIMESTAMPTZ,
    created_at     TIMESTAMPTZ NOT NULL DEFAULT now()
);
```

- [ ] **Step 2: 六个索引迁移**（各自单文件单语句，注释头照抄 `server/migrations/281_*.up.sql`）

| 编号 | 索引 |
| --- | --- |
| 289 | `test_generation_job (workspace_id, updated_at DESC)` |
| 290 | `test_generation_job (agent_task_id)` |
| 291 | `test_generation_job (workspace_id, project_id, status)` |
| 292 | `UNIQUE test_generation_plan (job_id) WHERE status IN ('draft','approved','dispatched')` — 每个 job 至多一个活跃计划 |
| 293 | `test_case_proposal (workspace_id, target_case_id, status)` |
| 294 | `test_case_proposal (job_id)` |

- [ ] **Step 3: 验证** `make server`，日志无 `migration failed`。
- [ ] **Step 4: 提交** `feat(testing): add test generation job tables`

---

## Task A2: sqlc 查询

**Files:** `server/pkg/db/queries/test_generation.sql`

**Produces:** `CreateTestGenerationJob`、`UpdateTestGenerationJob`、`GetTestGenerationJobInWorkspace`、`GetTestGenerationJobByAgentTask`（**带 workspace_id**）、`GetReusableTestGenerationJob`、`ListTestGenerationJobs`、`CreateTestGenerationPlan`、`GetTestGenerationPlanByJob`、`UpdateTestGenerationPlan`、`MarkTestGenerationPlanDispatched`、`CreateTestCaseProposal`、`ListTestCaseProposalsForCase`、`ListTestCaseProposalsForJob`、`GetTestCaseProposalInWorkspace`、`UpdateTestCaseProposalStatus`、`DeleteTestCaseProposalsForCase`。

要点：
- 每个 `UpdateXxx` 的所有可选字段一律 `COALESCE(sqlc.narg('x'), x)`，**包括 `error` 与 `review_notes`** —— design_restore 在这里踩过坑，部分更新会把 error 置空。
- `MarkTestGenerationPlanDispatched` 带 `WHERE status = 'approved'`。
- `GetTestGenerationJobByAgentTask` 必须 `AND workspace_id = $2`。
- `DeleteTestCaseProposalsForCase` 供 `DeleteTestCase` 的清理事务调用 —— 第一期的删除事务要补这一句（见 Task A6）。

- [ ] Step 1: 写查询 → Step 2: `make sqlc` → Step 3: `cd server && go build ./...` → Step 4: 提交

---

## Task A3: 生成任务 handler

**Files:** `server/internal/handler/test_generation.go`、`test_generation_test.go`

**Produces:** `CreateTestGenerationJob`、`ListTestGenerationJobs`、`GetTestGenerationJob`、`GetTestGenerationPlan`、`GenerateTestGenerationPlan`、`UpdateTestGenerationPlan`、`ApproveTestGenerationPlan`、`DispatchTestGenerationJob`，以及响应类型 `TestGenerationJobResponse` / `TestGenerationPlanResponse` / `TestCaseProposalResponse`。

`plan` JSON 形状（服务端 `buildDefaultTestGenerationPlan` 生成，人可编辑）：

```json
{
  "version": "1.0",
  "repos": [{ "project_resource_id": "…", "alias": "billing-api", "path_globs": ["internal/pricing/**"] }],
  "issues": ["MUL-1234"],
  "modules": ["订单"],
  "knowledge_refs": ["qa-domain-knowledge/references/billing-rules.md"],
  "attachment_ids": [],
  "expected_case_types": ["business_flow", "boundary", "permission"],
  "existing_case_digest_count": 42,
  "instructions": ""
}
```

`buildDefaultTestGenerationPlan` 的规则：从 `project_resource` 拉出全部 `github_repo` 与 `document` 资源填 `repos` / `knowledge_refs`；`existing_case_digest_count` 来自 `ListTestCases` 的计数；`expected_case_types` 默认给业务向的三类，明确表达"不只测代码"。

状态机与守卫：
- `GenerateTestGenerationPlan`：无活跃计划则建 `draft`；已有 `draft` 则重新生成覆盖；非 `draft` 返回 409。
- `UpdateTestGenerationPlan`：非 `draft` 返回 409。
- `ApproveTestGenerationPlan`：校验 `repos` 非空且每个 `project_resource_id` 属于本 project；盖 `approved_by` / `approved_at`。**不提供 `skip_plan`**。
- `DispatchTestGenerationJob`：job 已 `running` 返回 409（design_restore 缺这条会孤儿化前一个 task）；agent 必须非 archived 且有 `runtime_id`；`CreateQuickCreateTask` 传全 `originator_user_id` / `accountable_user_id` / `originator_source`；随后 `UpdateTestGenerationJob{status:"queued", agent_task_id}` + `MarkTestGenerationPlanDispatched`。

- [ ] Step 1: 先写测试（覆盖：审批前派发 409、重复派发 409、非 draft 改计划 409、审批校验 repos 归属、部分更新不清空 error） → Step 2: 跑测试确认失败 → Step 3: 实现 → Step 4: 跑通 → Step 5: 提交

---

## Task A4: agent 契约与 daemon 钩子

**Files:** `server/internal/service/task.go`、`server/internal/handler/daemon.go`、`server/internal/daemon/prompt.go`

**Produces:**
- `const TestGenerationContextType = "test_generation"` 与 `type TestGenerationContext struct { Type, Prompt, RequesterID, WorkspaceID, ProjectID, AgentID, JobID string; Plan, Input json.RawMessage }`
- claim 分支把 context 透给 `resp.TestGenerationContext`
- `markTestGenerationJobRunning`（StartTask）、`updateTestGenerationJobFromAgentCompletion`（CompleteTask）、`...FromAgentFailure`（FailTask）
- `buildTestGenerationPrompt`

prompt 必须写死的规则：
1. 先 `multica testcase list --project <id> --digest --output json` 拉已有用例，**只产出增量**；
2. 按 plan 的 `repos` 逐个 `multica repo checkout <url>`，只读 `path_globs` 圈定的目录；
3. 读 `knowledge_refs` 指向的业务文档；跑 `multica issue search` 找评论里的历史决策；
4. 用例要覆盖业务流程、权限矩阵、数据一致性、边界与异常，**不止代码层面**；
5. 产物一律通过 `multica testcase propose --job <id> --stdin` 写库，`new` / `update` / `obsolete` 三类；
6. 最后输出 `TEST_GENERATION_RESULT_JSON:` 汇总 `{status, summary, stats:{new,updated,obsolete}, blockers}`。

完成钩子**只读 `summary.status`** 判定成功失败，不扫输出里的 "blocked" 子串。

- [ ] Step 1: 写 marker 解析测试（含缺失 marker、状态为 blocked、输出含 "nothing was blocked" 仍算成功） → Step 2..5 同上

---

## Task A5: `propose` 回写端点

**Files:** `server/internal/handler/test_generation_propose.go`、`test_generation_propose_test.go`

`POST /api/test-generation-jobs/{id}/propose`，body：

```json
{ "items": [
  { "kind": "new", "case": { "title": "…", "module": "…", "steps": [...], "case_type": "business_flow", "repos": [...] } },
  { "kind": "update", "target": "TC-42", "case": { ... }, "rationale": "接口新增了分页参数" },
  { "kind": "obsolete", "target": "TC-7", "rationale": "该入口已下线" }
] }
```

行为：
- 鉴权：`X-Actor-Source == "task_token"`，且 `test_generation_job.agent_task_id` 等于中间件注入的 `X-Task-ID`。不匹配 403。照抄 `server/internal/handler/file.go:502` 的三重门写法。
- `new` → 直接 `test_case(status='draft', origin='ai', generation_job_id=<job>)`，复用第一期的 `validateTestCaseRepos` 与计数器分配。
- `update` / `obsolete` → 目标是 `active` 时进 `test_case_proposal(pending)`；目标是 `draft` 时**直接改写**（尚未定稿，无需 diff 审查，避免第二次生成在 draft 上堆 proposal）。
- 整批一个事务；任一条 schema 不合法则整批 400 并指明 index，不做部分写入。
- 回填 `job.result.stats`。

- [ ] Step 1: 先写测试（越权 token 403、跨 job 的 task_id 403、非法 kind 400、update 命中 draft 直接改写、update 命中 active 进 proposal、批量原子性） → Step 2..5

---

## Task A6: 路由、事件与删除清理

**Files:** `server/cmd/server/router.go`、`server/pkg/protocol/events.go`、`server/internal/handler/test_case.go`（补 proposal 清理）

端点：

```
GET/POST /api/test-generation-jobs
GET      /api/test-generation-jobs/{id}
GET/PUT  /api/test-generation-jobs/{id}/plan
POST     /api/test-generation-jobs/{id}/plan/generate
POST     /api/test-generation-jobs/{id}/plan/approve
POST     /api/test-generation-jobs/{id}/dispatch
POST     /api/test-generation-jobs/{id}/propose
GET      /api/test-cases/{ref}/proposals
POST     /api/test-case-proposals/{id}/accept
POST     /api/test-case-proposals/{id}/reject
```

事件：`test_generation_job:updated`、`test_case_proposal:created`、`test_case_proposal:updated`。**不做 3 秒轮询。**

`accept` 的语义：`update` → 写 `test_case_revision(change_kind='proposal_accepted')` 快照后套用 payload；`obsolete` → 置 `status='deprecated'` 并同样留快照。两者都在一个事务里连同 proposal 状态一起提交。

`DeleteTestCase` 的清理事务补一句 `DeleteTestCaseProposalsForCase` —— 否则删掉用例会留下悬空 proposal（无外键，必须应用层清）。

- [ ] Step 1..5 同上，含一条「删除用例后 proposal 不残留」的测试

---

## Task B: CLI `propose` 与技能更新

**Files:** `server/cmd/multica/cmd_testcase.go`、`cmd_testcase_test.go`、`builtin_skills/multica-test-cases/**`

- `multica testcase propose --job <job-id> --stdin`：从 `cmd.InOrStdin()` 读整份 JSON（不是 NDJSON），`json.Unmarshal` 校验后 POST。空输入报错。
- `multica testcase proposal list --case <TC-42>` / `accept <id>` / `reject <id>`。
- SKILL.md 增补生成契约：三类增量的判定标准、`--digest` 的用途、`propose` 的 JSON schema、`scope=cross_repo` 必须至少两个不同 role 的 repo、业务向用例类型的要求。同步更新 `references/test-cases-source-map.md` 的行号表。
- 在 `builtin_skills_test.go` 的既有 eval 里补 `mustContain`: `testcase propose`、`obsolete`、`business_flow`。

- [ ] Step 1..5 同上

---

## Task C: `document` 项目资源类型

**Files:** `server/internal/handler/project_resource.go`、`server/internal/daemon/execenv/runtime_config.go`、`runtime_config_sections.go`、`packages/core/types/project.ts`、`packages/views/projects/components/project-resources-section.tsx`、`server/cmd/multica/cmd_project.go`、`builtin_skills/multica-projects-and-resources/**`、`apps/docs/content/docs/project-resources.mdx`（含 .zh/.ja/.ko）

这是让智能体「懂业务」性价比最高的一处改动，**零迁移**（`resource_type TEXT` + `resource_ref JSONB` 本就多态）。

1. `validateAndNormalizeResourceRef`（`project_resource.go:75` 的 switch）加 `case "document"`，ref 形状 `{url, title, summary}`，`url` 必填且限 http/https，`title` 必填，`summary` 可选并限长（建议 500 字符）。
2. `formatProjectResource`（`runtime_config.go:118`）加 document 分支，渲染成 `- 文档《title》: url — summary`。
3. `writeProjectContext`（`runtime_config_sections.go:337`）：document 资源要明确告诉 agent「这些是业务规格，与代码同等重要，生成测试用例前应当阅读」。**同时**把现有那句「资源只是指针，只在相关时才打开」补一个多仓库例外说明（spec §12 第 2 条：现措辞抑制多仓库 checkout）。
4. `ProjectResourceType`（`packages/core/types/project.ts:67`）加 `"document"`。
5. `project-resources-section.tsx` 的 `ResourceRow` 加渲染分支与添加控件（表单三个字段）。字符串走 i18n。
6. `cmd_project.go`：`multica project resource add <id> --type document --url <url> --title <t> [--summary <s>]`。
7. 按 CLAUDE.md，同 PR 更新 `multica-projects-and-resources/SKILL.md` 与 `references/projects-and-resources-source-map.md`。

- [ ] Step 1: 先写 `project_resource` 的校验测试（缺 url 400、非 http scheme 400、summary 超长 400、正常归一化） → Step 2..5

---

## Task D: §12 多仓库三处修复

**Files:** `server/internal/daemon/repocache/cache.go`、`cache_test.go`、`server/cmd/multica/cmd_repo.go`、`server/internal/handler/daemon.go`

1. **workdir 目录名冲突（真 bug）**：`repoNameFromURL`（`cache.go:1356`）只取 basename，`org-a/app` 与 `org-b/app` 都落到 `{workdir}/app`；第二次 checkout 走 `updateExistingWorktree`，用 B 的 baseRef 更新 A 的树，**静默出错**。同文件的 `bareDirName` 已用 host+path 限定正确解决了缓存目录的同名问题（测试在 `cache_test.go:132/147`）。修法：**只在冲突时**加限定前缀，保持现有单仓库布局不变 —— 无条件改名会打乱所有已存在的 workdir。
2. **无人主动 checkout**：`multica repo checkout --all`，从 `.multica/project/resources.json` 读全部 `github_repo` 逐个检出。走 CLI 路线以免与 Task C 抢 `runtime_config_sections.go`。
3. **项目仓库整体覆盖工作区仓库**：`daemon.go` 的 `if len(projectRepos) > 0 { resp.Repos = projectRepos }`（两处：约 1869 与 2632）改为**合并去重**，项目仓库优先。挂一个项目资源不该让工作区其余仓库从 checkout 白名单消失。

- [ ] Step 1: 先写 `repoNameFromURL` / 冲突消歧的表驱动测试（同名不同 host、同名不同 org、单仓库路径不变） → Step 2..5

---

## Task E: 前端

**E1 数据层** — `packages/core/types/testing.ts`、`api/schemas.ts`（+ 畸形响应测试）、`api/client.ts`（全部 `parseWithFallback`）、`packages/core/testing/{keys,queries,mutations}.ts`、`realtime/use-realtime-sync.ts` 加 `test_generation_job` 与 `test_case_proposal` 两个 refreshMap 条目、`packages/core/package.json` exports。

**E2 生成任务页** — `packages/views/testing/test-generation-job-page.tsx`，两栏：左为任务元信息与产物统计，右为计划面板（生成 / 编辑 JSON / 选仓库与路径 / 审批）+ 派发面板（选 agent）+ 结果面板。布局照 `packages/views/designs/design-restore-task-page.tsx`，但**不要复制它的 `skip_plan` 复选框**。纯函数（计划校验、统计聚合）放独立 `.ts` 便于单测。

**E3 审查** — 列表页加「AI 生成待审」快捷筛选（`status=draft` + `origin=ai`）与批量通过；详情页加 proposal diff 面板（左原值右建议、逐字段高亮、采纳 / 拒绝）。

**E4 平台接线** — `paths.testGenerationJobDetail`、`consistency.test.ts` 与 `diagnostic-context.ts` 两处清单、`apps/web/app/[workspaceSlug]/(dashboard)/tests/jobs/[jobId]/page.tsx`、desktop 路由 + `useParams` 包装组件。

四个 locale 的 `testing.json` 同步补键。

- [ ] 每个子任务先写测试再实现，各自提交

---

## Task F: 收尾验证

- [ ] `e2e/test-cases-generation.spec.ts`：建 job → 生成计划 → 编辑范围 → 审批 → 派发（用 `TestApiClient` 打桩 agent 回写）→ 审查 draft → 采纳一条 update proposal。
- [ ] 全量：`pnpm typecheck` / `pnpm lint` / `pnpm test` / `make test`，全绿才算完。任何失败必须修掉或明确说明。
- [ ] 回填 `references/*-source-map.md` 的真实行号。

---

## Self-Review

**spec 第二期覆盖**

| spec 要素 | Task |
| --- | --- |
| `test_generation_job` / `test_generation_plan` / `test_case_proposal` | A1、A2 |
| 计划生成与审批 | A3 |
| 派发 | A3、A4 |
| `testcase propose` 回写 | A5、B |
| 增量 new/update/obsolete | A5、A4（prompt） |
| 待审列表与 diff 面板 | E3 |
| 业务知识（`document` 资源类型） | C |
| §12 三处修复 | D |

**业务知识的另外两条**（`qa-domain-knowledge` 工作区技能、`workspace.context` 铁律）是**用户侧配置，不是代码改动** —— spec §7.2 已说明是零代码路径。本期只在 `apps/docs` 补一篇怎么配的说明，不写代码。

**类型一致性**：`TestGenerationContextType` 值 `"test_generation"`（A4）与 daemon claim 分支、prompt builder 一致；事件前缀 `test_generation_job` / `test_case_proposal`（A6）与 refreshMap 键（E1）一致；`propose` 的 `kind` 三值 `new|update|obsolete`（A5）与 CLI（B）、prompt（A4）、SKILL.md（B）一致。
