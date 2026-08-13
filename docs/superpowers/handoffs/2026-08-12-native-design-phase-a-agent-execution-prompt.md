# Native Design Phase A 执行恢复提示词

> 用途：交给新的 Agent，恢复 Multica Native Design Phase A 的完整产品方案、当前进展、执行起点和阶段交付规则。
>
> 这是一份执行入口，不替代权威规格。Agent 必须先重新读取文件并核对 Git，不得只相信本提示词中的时间点。

## 可复制提示词

```text
你正在继续 Multica 的 Native Design Phase A 实施工作。请先恢复产品和代码上下文，再开始执行。不要恢复已取消的 Open Design Phase B，不要从历史代码反推删除范围，不要使用 feature/fengchen-fixed-v2，不要在未完成当前阶段报告前跳到下一阶段。

仓库：/Users/fengyujie/Documents/soyoung/multica
当前产品主线：feature/fengchen

# 一、必须读取的权威文件

按以下顺序读取：

1. 根目录 `CLAUDE.md`；
2. `docs/product/design-center/README.md`；
3. `docs/product/design-center/decision-register.md`，重点读取 DC-039 至 DC-046；
4. `docs/product/design-center/native-v2-retirement-register.md`；
5. `docs/superpowers/specs/2026-08-12-native-design-slice-driven-evolution-design.md`；
6. `docs/superpowers/specs/2026-08-12-design-home-public-systems-community-templates-design.md`；
7. `docs/superpowers/specs/2026-08-12-native-design-phase-a-design-document-design.md`；
8. `docs/superpowers/handoffs/2026-08-12-native-design-product-resume-prompt.md`；
9. `docs/superpowers/specs/2026-08-05-multica-native-design-engine-design.md`；
10. `docs/product/design-center/project-design-system-validation.md`；
11. `docs/product/design-center/project-design-system-workspace-validation.md`。

以下文件标记为 `superseded`，只能读取历史原因，不得执行其中的任务、命令或删除路线：

- `docs/superpowers/specs/2026-08-12-native-design-phase-1-closure-and-legacy-removal-design.md`；
- `docs/superpowers/plans/2026-08-12-open-design-v1-destructive-removal.md`。

# 二、执行前必须核对的当前事实

在任何代码编辑、迁移、删除或提交前执行并记录：

```bash
git status --short --branch
git log -5 --oneline --decorate
git branch -vv
git diff --stat
git diff --cached --stat
```

以当前 checkout 和权威文档为准。不要混淆：

- `feature/fengchen` 是当前产品主线；
- `feature/fengchen-fixed-v2` 是取消路线的 checkpoint，不合入、不 cherry-pick、不作为实现基线、不计入进度；
- 其他工作树的提交、测试结果和脏文件不自动属于当前 checkout；
- 当前文档可能有用户未提交改动，禁止清理、覆盖或重置无关变更。

# 三、当前产品方案

## 3.1 Phase A 目标

Phase A 交付两条相互衔接的闭环：

1. 项目 Native V2 设计体系的创建、生成、Preview、调整和保存；
2. 设计中心首页的页面设计 task 入口：用户描述需求，选择项目和智能体，使用项目上下文、真实仓库和已保存设计体系生成页面 Design Document，完成 Preview、调整和保存。

Open Design 只作为行为基线。Multica 不运行、分发或托管 Open Design Worker、Daemon、Runtime，也不创建第二套 Project、Issue 或智能体控制面。

## 3.2 三个需求切入点

你必须保持三个需求的边界：

- **首页**：进入当前 Phase A，是跨项目页面设计 task 发起器；项目和智能体必选，任务（Issue）和目标平台可选；创建成功后打开目标项目“设计草稿”。
- **共享设计体系**：属于后续 Slice B；从项目 saved Native V2 package 重新校验、安全剥离后生成不可变共享 revision，不能用 `is_public` 直接公开项目体系。
- **官方/成员/社区模板**：属于后续 Slice C、D、E；模板是页面设计 task 配方，不是设计体系；模板升级或下架不改变历史 task 的固定快照。

Slice B 至 E 不计入当前 Phase A，不得提前实现或混入当前阶段。

## 3.3 页面 Design Document

正式页面产物是轻量、版本化的：

```text
multica.design-document/v1
```

最小 package：

```text
manifest.json
brief.json
prototype/
  index.html
  styles.css
  app.js
assets/
coverage.json
```

- `manifest.json`：身份、输入快照、项目 saved 设计体系、base revision、文件索引和 digest；
- `brief.json`：页面、子页面、状态、弹窗、流程、稳定语义 ID 和需求映射；不恢复大型 PageSpec DSL；
- `prototype/`：完全离线的 HTML/CSS/JavaScript 交互原型，可使用模拟数据，不调用真实 API；
- `coverage.json`：需求、Issue（有则使用，无则映射用户自然语言需求）、页面/状态/流程、设计体系和关键交互覆盖；
- Audit/Preview receipt 与 package content digest 绑定，作为独立证据对象保存，不能把 receipt 放入它自身摘要的 package。

一份 Design Document 可以包含主页面、子页面、状态、弹窗和关键流程。一个项目允许多份 Design Document。每份文档维护不可变 revisions、当前 `draft_revision_id` 和 `saved_revision_id`。用户第一版只看到当前 draft/saved，不提供完整版本时间线。

## 3.4 Agent、任务和工作空间

- 首次生成、调整、重新取证和重试都创建独立 Agent task；不复活终态 task；
- 每份 Design Document 有持续隔离 workspace；每次 task 固定 input snapshot、base revision/digest 和 scope；
- 同一文档第一版只允许一个活动写 task，不自动合并并发分支；
- 页面 task 内自动执行有界只读仓库 Grounding，不增加首页前置扫描；
- 固定 checkout/commit、相对来源、文件 digest、路由、组件、样式、业务事实、冲突和不确定性；
- 不保存绝对路径、凭据、无关业务数据或未授权完整源码；
- Agent 不得修改用户源仓库，也不能直接移动 draft/saved 指针；
- 仓库不可访问时必须明确提示，用户选择仅使用其他资料继续或停止，不得静默声称已 grounding；
- 普通调整沿用原快照；用户主动同步最新仓库时，才重新取证并创建新的 input snapshot。

## 3.5 Draft、Saved、调整和 Issue

- 首个有效 draft 通过全部门禁后才创建 Design Document；失败或取消不留下空文档；
- 每次调整输出完整新 package，不能只修改 Preview 表面；scope 支持文档、页面、状态、弹窗和命名区块；
- base digest 冲突时拒绝调整，不自动合并；
- 用户明确保存后才移动 saved；保存失败 draft 保留、旧 saved 不变；
- 首次未保存 draft 可以继续多轮调整；
- 放弃首次 draft 后从正常文档列表移除；放弃已有 saved 的调整后 draft 恢复 saved；不删除内部历史证据；
- Issue 是可选关联，不自动创建 Issue，不自动改变 Issue 状态、负责人、优先级或完成状态；
- Issue 关联、task、revision 和输入快照必须可追溯。

# 四、不可违反的安全和质量硬要求

Phase A 明确不允许：

- Agent 修改用户源仓库；
- 原型调用真实业务 API；
- 原型携带用户或项目凭据；
- Agent 自己决定 revision 生效；
- 未经 Audit/Preview 的 package 进入 draft；
- draft 被下游 Agent、MCP 或交付链当作项目有效设计稿；
- 使用旧 revision 的验证回执批准新 package；
- Preview iframe 获得 Multica 宿主同源权限；
- 把“能够渲染”描述成“视觉质量已通过”。

必须复用员工本地守护进程已有的 `server/internal/designpreview` 强制浏览器门禁：

```text
Agent task completed
+ 安全收集成功
+ Package Audit 通过
+ 本地 Chrome/Chromium Preview 通过
+ 输入/base digest 一致
+ 原子持久化成功
= 有效 draft
```

浏览器不可用时任务失败，不跳过 Preview，不增加“待验证候选”、前端补验证或无浏览器保存例外。Preview 通过只证明原型能安全运行，不代表人工视觉质量已经通过；严格验收仍需真实用户 Chrome、Network、Console 和人工业务判断。

普通功能切片不得删除历史行、对象、表或约束。发现跨切片、跨数据生命周期、外部消费者、破坏性迁移、范围外 API 或无法独立回滚时，立即停止扩大范围并更新 `native-v2-retirement-register.md`。

# 五、已有工程基础、当前进度与起点

Phase A 不从零开始。开始计划前必须读取正式规格第 17 节，并把已有基础写入 A1-A6 计划，禁止重复建设并行管道。

可直接复用或翻新的主要基础：

- `agent_task_queue` 和本地守护进程的 task 生命周期、工作目录、取消、终态和 GC；
- Native V2 package 的安全收集、文件索引、digest、对象存储、Audit、completion 重验和失败隔离；
- `server/internal/designpreview` 本地 Chrome/Chromium 门禁和 digest 绑定 receipt；
- 项目设计体系的 input snapshot、base digest、draft/saved 隔离、调整、保存和放弃行为；
- `RepositoryDesignContext`、项目仓库分析、相对来源和 commit 校验；
- 设计中心固定首页/项目 Tab、四项内容导航、设计体系工作区和旧设计草稿入口；
- 历史 `design_draft` + 迁移 874 的 semantic PageSpec 路径、设计文件/revision/asset 模型和可选 `issue_id`。

不得把这些基础写成“页面 Design Document 已完成”：`design_document`、不可变 Document Revision、draft/saved revision 指针、`multica.design-document/v1`、首页发起器、文档持续 workspace、交互 Audit、多文档 UI 和真实 CRM 文档验收仍需实现。

当前进度口径：

- 产品设计确认度：100%；
- Phase A 工程完成度：约 40%–45%，当前规划基线约 42%；
- A1 约 55%，A2 约 25%，A3 约 35%，A4 约 60%，A5 约 45%，A6 0%。

这些百分比是可复用基础与剩余工作的保守工程估算，不是阶段通过或测试通过。每完成一个阶段必须按实际结果重算。

下一步从 **A1：Design Document 核心协议与持久化** 开始，而不是从首页 UI 或旧代码删除开始。A1 不是从零新建整套管道，而是在复用现有任务、对象存储、digest、Audit/Preview 和失败隔离基础上，落地新的文档实体、revision、指针与 package/schema。

A1 至 A6 是当前 Slice A 的内部子切片，与后续 Slice B 至 E 正交：

- **A1**：Design Document 核心协议与持久化；
- **A2**：首页任务入口与项目 task 状态；
- **A3**：仓库 Grounding 与持续 workspace；
- **A4**：Audit、Preview 与首个 draft；
- **A5**：调整、保存与放弃；
- **A6**：真实 CRM 严格验收。

开始 A1 前必须先写并获用户批准详细实施计划。不要自行进入代码实现，不要先做 A2，不要先清理旧 PageSpec/Open Design 路径。

A1 实施计划必须明确：

- 新 Design Document/revision/指针与现有 `semantic_design_draft` 的关系；
- 是否新增表、扩展旧表或采用其他持久化方式；
- workspace、project、task、Issue、digest 和对象存储不变量；
- API 与前端范围；
- V2 正向、失败隔离、旧路径负向和范围外回归；
- `native-v2-retirement-register.md` 是否变化；
- migration、回滚和历史数据保留边界。

# 六、每个阶段必须完整汇报

每完成一个阶段（A1、A2、A3、A4、A5、A6），不得只说“完成”或“测试通过”。必须先停止进入下一阶段，提交一份完整阶段报告，等待用户确认。

阶段报告必须包含：

1. 阶段名称、目标和实际完成范围；
2. 与计划相比的偏差、未完成项和新增范围；
3. 实际修改的文件、符号、API、数据库和前端范围；
4. 是否删除了旧路径；若删除，说明替代关系和局部负向合同；若未删除，说明保留原因并更新退役账本；
5. Git 分支、HEAD、工作区状态、提交 hash 和回滚方式；
6. GitNexus `impact` 结果、HIGH/CRITICAL 警告和 `detect_changes` 结果；
7. 实际执行的每一条命令，包含 PASS/FAIL/SKIP、测试数量和关键输出；未执行项必须写 `NOT RUN`；
8. 真实 Agent、真实仓库、真实 Chrome 和人工验收是否执行，不能把 fixture/fake 证据写成真实现场证据；
9. task、revision、draft/saved、digest、Preview receipt 和对象存储的持久化断言；
10. 失败、取消、重试、越权、存储失败和旧路径负向结果；
11. 当前进度：已完成阶段、当前阶段、剩余阶段、风险和阻塞；
12. 下一步建议，但不得未经用户确认自动开始下一阶段。

阶段报告完成后，必须把对应的执行记录写入 `.superpowers/sdd/` 或本阶段指定证据文档，并更新产品/退役账本中实际发生的事实。任何失败或跳过都要诚实记录，不得用定向测试掩盖 broad failure。

# 七、代码和验证硬规则

- 修改任何函数、类或方法前，先运行 GitNexus `impact` upstream；HIGH/CRITICAL 必须先报告 blast radius 和回归矩阵；
- 提交前必须运行 GitNexus `detect_changes()`；
- 不用 find-and-replace 重命名符号；
- 遵守根 `CLAUDE.md` 的 API compatibility、UUID、数据库 migration、状态分离和平台边界规则；
- 新 API 必须有 schema、fallback parsing 和 malformed-response 测试；
- 先写正确位置的失败测试，再写实现；
- 不运行用户安装的 Agent CLI，不设置真实 Agent smoke test 环境，除非用户明确授权；
- 不把 fake verifier、fixture package、受控浏览器测试写成真实 CRM 验收；
- 代码注释使用英文；产品文档和用户报告使用中文；
- 未经用户明确要求不 push、不发布、不删除历史数据、不执行 destructive migration。

# 八、报告格式

每次阶段完成时用以下结构：

```markdown
# Phase A <A1-A6> 阶段报告

## 1. 结论
- PASS / PASS_WITH_CONCERNS / BLOCKED

## 2. 实际范围

## 3. 文件、符号、API、数据变化

## 4. 旧路径与退役账本

## 5. GitNexus impact / detect_changes

## 6. 验证命令与结果
| 命令 | 结果 | 证据/限制 |

## 7. 真实现场边界
- Real Agent:
- Real repository grounding:
- User Chrome:
- Human visual review:

## 8. 持久化不变量

## 9. 失败、取消和安全结果

## 10. 未完成、风险和阻塞

## 11. 当前总体进度

## 12. 等待用户确认的下一步
```

当前启动点：先读取上述权威文件、核对 Git，然后为 **A1** 编写详细实施计划并交给用户复核；不要直接修改代码。
```

## 当前交付状态

- 已确认产品规格：`docs/superpowers/specs/2026-08-12-native-design-phase-a-design-document-design.md`；
- 已确认决策：`docs/product/design-center/decision-register.md` 的 DC-042 至 DC-046；
- 当前产品设计进度：100% 已确认；
- 当前 Phase A 工程进度：约 42%（规划基线，范围约 40%–45%）；
- 各子切片基线：A1 约 55%、A2 约 25%、A3 约 35%、A4 约 60%、A5 约 45%、A6 0%；
- 已有基础：智能体 task/本地运行时、Native V2 package、对象存储、digest、Audit、Preview receipt、draft/saved、仓库事实契约、设计中心 Tab/工作区和旧页面草稿路径；
- 未实现本体：Design Document/revision/指针、`multica.design-document/v1`、首页发起器、文档 workspace、交互 Audit、多文档 UI 和真实 CRM 页面文档验收；
- 下一执行阶段：A1；
- A1 至 A6 全部完成后，才可称为 Phase A 实施完成；A1 至 A5 自动化不能替代 A6 真实 CRM 严格验收。
