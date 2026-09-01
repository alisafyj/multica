# Design Center End-to-End MVP 权威交接文档

> 更新时间：2026-09-02（Asia/Shanghai）
>
> 当前仓库：`https://github.com/alisafyj/multica`
>
> 当前集成分支：`codex/design-center-end-to-end-mvp`
>
> Tasks 10–12 执行分支：`codex/design-center-end-to-end-mvp-tasks-10-12`
>
> **本文件是本轮 Design Center End-to-End MVP 的最高优先级交接事实源。**

旧聊天、旧机器路径、旧数据库/profile、历史执行说明与本文冲突时，以本文为准。产品长期事实仍结合 `docs/product/design-center/README.md` 的 confirmed / proposal / superseded 状态判断。

---

## 0. 当前结论与下一步

Group 3（Task 7–9）已经完成并通过真实 Gate，当前不再处于 Task 7 暂停点。

接下来按顺序执行：

1. Task 10：Multica Design Source Adapter 与真实仓库还原；
2. Task 11：Figma Source Adapter 复用同一还原契约；
3. Task 12：Issue 右侧栏统一设计选择与还原入口；
4. 每个 Task 保留聚焦提交和独立验证；
5. Tasks 10–12 完成后停止并汇报，Task 13、14 不在当前授权范围。

执行前按顺序读取：

1. 本文件；
2. `docs/product/design-center/README.md`；
3. `DESIGN.md`；
4. `docs/superpowers/plans/design-center-active-index.md`；
5. `docs/superpowers/plans/2026-08-31-design-center-end-to-end-mvp-roadmap.md` 的 Task 10–12；
6. `docs/superpowers/specs/2026-08-26-unified-design-asset-implementation-design.md`；
7. Task 12 需要的 `docs/superpowers/specs/2026-08-27-issue-design-automation-design.md` 相关章节。

---

## 1. 用户目标与当前授权

### 1.1 总目标

```text
真实项目与仓库
→ 仓库专属设计体系
→ 两份真实 Multica Design Document
→ 监控 / Audit / Preview / 调整 / 保存
→ 统一 Multica / Figma design_ref 与 frame_ref
→ 统一 Implementation Prompt / Context / Result
→ Issue 选择设计稿并在真实仓库实现代码
→ 测试、构建和真实渲染
→ 用户验收完整链路
```
### 1.2 当前授权边界

- 已确认更新本文后开始 Task 10–12。
- 允许在当前机器使用 Docker/PostgreSQL 建立隔离验证环境。
- 原机器 Task 7 数据库和 profile 不迁移、不依赖。
- 不使用 mock，不以 HTTP 200、typecheck 或单元测试替代真实产品验收。
- 当前只做到 Task 12；不自动开始 Task 13 或 Task 14。
- 不 Push、不创建 PR、不合入 `main`。
- 当前执行分支完成后不自动合并回集成分支；合并需用户再次确认。

---

## 2. 文档权威顺序

1. 本文件：当前完成状态、分支、执行边界和下一步；
2. `docs/product/design-center/README.md`：Design Center 长期产品事实；
3. `docs/superpowers/plans/design-center-active-index.md`：文档导航；
4. `docs/superpowers/plans/2026-08-31-design-center-end-to-end-mvp-roadmap.md`：Task 1–14 路线；
5. `docs/superpowers/specs/2026-08-26-unified-design-asset-implementation-design.md`：Task 8–11 契约；
6. `docs/superpowers/specs/2026-08-27-issue-design-automation-design.md`：Task 12 产品入口。

延后文档 `docs/superpowers/plans/2026-08-31-design-center-finder-repository-workspaces.md` 不属于当前任务，不提前实现完整 Finder、多项目 Tab 或视觉精雕。

---

## 3. Git 与工作树状态

### 3.1 MVP 集成分支

```text
分支：codex/design-center-end-to-end-mvp
HEAD：5ba52ff5a12538759f90e3ec12c00e10a322d5b0
状态：clean；相对 origin 本地领先 41 个提交
```

`5ba52ff5a` 是 Group 3 集成提交：

```text
merge(group-3): validate unified design implementation contracts
```

### 3.2 Tasks 10–12 执行分支

```text
分支：codex/design-center-end-to-end-mvp-tasks-10-12
基线：5ba52ff5a12538759f90e3ec12c00e10a322d5b0
状态：从 Group 3 完成基线创建的隔离 worktree
```

Task 10、11、12 在该分支顺序实施。每项先写失败测试，再写最小实现，执行定向验证并形成聚焦提交。

### 3.3 保留工作树

Group 3、Task 7、Task 8、Task 9 的历史工作树仍保留，只用于审计，不在 Tasks 10–12 中继续写入、清理或复用。

---

## 4. 已完成能力

### 4.1 Group 1 / Task 1–3

- repository grounding completion 基线；
- 最小项目/仓库视角和设计稿仓库关联；
- “关联仓库”与 `repository_grounded=true` 分离；
- 仓库工作流精确读取 repository-specific saved design system；
- 跨项目、跨仓库和删除清理边界覆盖。

### 4.2 Group 2 / Task 4–6

- Home 和 Repository 入口复用同一创建表单；
- Repository 入口预填项目、仓库和 GitHub 来源；
- 固定仓库 scope，禁止 repository workflow fallback；
- Design Document revision 保存设计体系 revision provenance；
- Preview、调整、保存和基于最新体系分叉链路接入。

### 4.3 Group 3 / Task 7–9

Group 3 已合入集成分支并通过最终审查：`APPROVED · Architecture CLEAR`。

关键提交：

```text
40edc1e0c merge(task-7): validate repository design production MVP
0171aaa67 merge(task-8): add unified design and frame references
2a5624986 merge(task-9): define implementation context and result contracts
5ba52ff5a merge(group-3): validate unified design implementation contracts
```

Task 7 真实验收报告：

```text
docs/product/design-center/mvp-phase-a-validation.md
```

结论为 PASS，真实完成：

- 隔离 Workspace、Project、`alisafyj/multica` Repository association 和 Agent；
- repository analysis；
- repository-specific saved design system revision；
- Design A（CRM 客户列表）；
- Design B（CRM 客户详情/商机流程）；
- Design B 一次调整；
- 三个 revision 的 Audit、Preview 和保存；
- desktop 与受限 viewport 的真实 Chromium DOM、可访问性、布局和交互检查。

Task 8 已提供来源无关、版本化、不透明的 `design_ref` / `frame_ref` 与 Frame API，覆盖 saved Multica revision、valid Figma revision、篡改、跨 workspace/project、stale revision 和 draft rejection。

Task 9 已提供统一 Implementation Prompt / Context / Result 契约：

- `POST /api/design-assets/{design_ref}/implementation-prompt`；
- MCP `get_implementation_context`；
- `implementation-result/v1`；
- API 与 MCP 固定同一个 context identity；
- Prompt 只供用户审阅/预填，不自动发评论、不启动 Agent、不 commit/Push/PR、不改变 Issue 状态。

---

## 5. Task 10–12 执行契约

### 5.1 Task 10：Multica Design Source Adapter

- 从 saved Design Document package 确定性提取 Prototype、semantic brief、coverage、assets、pages/states/flows、design-system provenance 和 grounding evidence；
- 执行前读取目标 checkout 的 framework、routes、components、state、styling 和 commands；
- 优先复用仓库现有组件与 tokens；
- 写入只允许发生在用户选择的目标 repository/worktree；
- 执行仓库专属 typecheck/tests/build 和真实页面渲染检查；
- 失败保留 dirty worktree 和结构化证据；
- 返回 `implementation-result/v1` 和人类摘要；
- 用 Phase A 的真实 Multica design 和真实目标仓库验证。

预期聚焦提交：

```text
feat(designs): restore Multica designs into repository code
```

### 5.2 Task 11：Figma Source Adapter

- 解析 valid Design File revision 和所选 Frame；
- 输出与 Task 10 完全相同的 Implementation Context；
- 复用相同 Agent 执行、结果、验证、失败恢复和目标仓库规则；
- 外层 selector/workflow 保持来源无关，只显示 source badge/metadata 差异；
- 不做 Figma 与 Multica 自动内容匹配或合并；
- 用真实 Figma design、同一目标仓库和同一 result schema 验证。

预期聚焦提交：

```text
feat(designs): restore Figma through unified implementation context
```

### 5.3 Task 12：Issue 统一设计选择与还原入口

产品入口：

```text
Issue 右侧栏 → UI Design → 选择已有设计
```

- 一个 selector 展示 saved Multica 与 valid Figma designs，带可见来源 badge；
- 选择一个 page/frame 和 target repository；
- association 只能预填目标仓库，用户可确认或更换，不静默改变 asset association；
- 生成 implementation prompt 并预填现有 Issue comment editor；
- 用户可编辑并手动发送，发送走普通 Agent/comment 路径；
- 展示 pending/running/completed/blocked 状态和结构化 result card；
- 设计/还原动作不自动改变 Issue status；
- 两种来源通过同一组件，外层 Issue workflow 不按来源分叉。

预期聚焦提交：

```text
feat(issues): implement tasks from unified design assets
```

---

## 6. 验证与安全边界

- 不访问、输出或提交密码、token、Cookie、验证码、私钥、数据库凭据或用户私有数据。
- 不调用 screenshot、录屏或 trace；真实 UI 验收使用可见 DOM、accessibility、computed layout、`elementFromPoint` 和真实交互状态。
- 不停止、重启、删除或改配不属于当前隔离 worktree 的服务、容器、数据库或 profile。
- 新验证环境必须使用隔离端口和数据库，不占用宿主 `5432`。
- 页面能加载、HTTP 200、typecheck、单元测试均不能单独证明产品验收通过。
- saved Multica revision 与 valid Figma revision 才能进入还原链路。
- 设计选择、Prompt 生成、发送或还原均不自动改变 Issue 状态。
- 失败时保留目标 worktree 的变更和结构化错误证据，不伪造成功结果。
- 不自动 commit 目标用户仓库中的还原产物；产品仓库自身 Task 提交除外。

---

## 7. 当前明确的 NOT DONE

- Task 10 Multica Source Adapter 和真实仓库还原；
- Task 11 Figma Source Adapter 和同契约真实验证；
- Task 12 Issue 统一设计选择、Prompt 预填、状态与结果卡；
- Task 13 Issue 发起 Multica Design 创建与恢复；
- Task 14 全链路最终产品 Gate；
- Tasks 10–12 合回集成分支；
- Push、PR、`main` 合并和发布；
- Post-MVP Finder/视觉精雕。

---

## 8. 给下一位 Agent 的恢复提示

```text
先读取本交接、Design Center README、根 DESIGN.md、active index、roadmap Task 10–12 和统一设计资产 Spec。

核验：
- 集成基线为 5ba52ff5a；
- 执行分支为 codex/design-center-end-to-end-mvp-tasks-10-12；
- Group 3 与 Task 7–9 历史工作树只读保留；
- Task 7 报告为 docs/product/design-center/mvp-phase-a-validation.md，结论 PASS。

继续顺序：Task 10 → Task 11 → Task 12。每项使用 TDD、聚焦提交、真实仓库/API/UI 验证。不得使用 mock 或 HTTP 200 代替产品验收。不得自动开始 Task 13/14，不 Push、不建 PR、不合 main；完成后等待用户确认是否合回集成分支。
```
