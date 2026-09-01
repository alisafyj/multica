# Design Center End-to-End MVP 权威交接文档

> 更新时间：2026-09-01（Asia/Shanghai）
>
> 适用仓库：`/Users/fengyujie/Documents/soyoung/multica`
>
> 当前集成分支：`codex/design-center-end-to-end-mvp`
>
> **本文件是本轮 Design Center End-to-End MVP 的最高优先级交接事实源。**
> 当旧聊天、旧交接、Plan、Spec、活动索引或历史实现说明与本文件冲突时，以本文件为准；产品长期事实仍需结合 `docs/product/design-center/README.md` 判断 confirmed / proposal / superseded，但本轮执行状态、暂停点、分支和下一步以本文为准。

---

## 0. 下一位 Agent 先做什么

先保持只读，不要启动服务，不要修改代码，不要恢复自动推进。

按顺序读取：

1. 本文件；
2. `docs/product/design-center/README.md`；
3. `docs/superpowers/plans/design-center-active-index.md`；
4. 只在需要理解 Task 7–9 时读取 `docs/superpowers/plans/2026-08-31-design-center-end-to-end-mvp-roadmap.md` 对应章节；
5. 当前任务引用的单一 Spec 章节，不扫描整个 Plans/Specs 目录。

随后核验：

```bash
cd /Users/fengyujie/Documents/soyoung/multica/.worktrees/design-center-end-to-end-mvp-integration
git status --short --branch
git log -5 --oneline --decorate

cd /Users/fengyujie/Documents/soyoung/multica/.worktrees/design-center-end-to-end-mvp-task-7
git status --short --branch
make status
```

第一份回复只需要说明：

- 已理解的产品目标；
- 当前分支和暂停点；
- 截图中的首页/仓库页面为什么出现、它属于哪个路由或状态；
- 与用户此前认可的首页设计相比，当前实现发生了什么变化；
- 下一步准备先查什么。

**不要在第一份回复后自行启动 Task 7。先回答用户的问题并等待用户决定。**

---

## 1. 用户当前要求

### 1.1 总目标

先跑通一条真实、可验收的 Design Center MVP：

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

### 1.2 当前交互方式

用户已要求暂停自动执行。接下来先“用户问、Agent 答”。

因此当前不是继续跑 Task 7 的状态，而是：

1. 先解释当前页面为什么与此前设计不一致；
2. 确认正确首页和项目/仓库工作区的信息架构；
3. 用户确认后，才恢复 Group 3。

### 1.3 执行组织

- Group 3 仍由 Task 7、Task 8、Task 9 组成；恢复后以三个 Task 为一组推进。
- 执行方式不做模型、实现者或审查角色限定。
- Agent 可直接完成工作，也可自行选择工具；关键是遵守分支、写集、验证、提交和产品验收边界。
- 不因环境工具的单一误判重复执行同一失败命令；确认底层身份和安全边界后，应选择替代路径继续。
- 当前暂停指令优先于自动推进规则。

---

## 2. 文档权威顺序

### 2.1 第一优先级

1. 本交接文档：当前状态、暂停点、分支、下一步和执行边界；
2. `docs/product/design-center/README.md`：Design Center 长期 confirmed / proposal / superseded 产品事实。

### 2.2 当前执行文档

- `docs/superpowers/plans/design-center-active-index.md`：文档导航；
- `docs/superpowers/plans/2026-08-31-design-center-end-to-end-mvp-roadmap.md`：Task 1–14 路线；
- `docs/superpowers/specs/2026-08-26-unified-design-asset-implementation-design.md`：Task 8–11 的统一设计引用、Prompt、MCP、Context 和 Result 契约。

### 2.3 支撑产品 Spec

- `docs/superpowers/specs/2026-08-26-design-center-project-repository-views-m1-design.md`；
- `docs/superpowers/specs/2026-08-27-issue-design-automation-design.md`；
- `docs/superpowers/specs/2026-08-27-multica-design-center-master-plan.md`。

这些文档提供产品和技术背景；若状态描述与本文不一致，以本文为准。

### 2.4 已完成实施计划，仅用于审计

- `docs/superpowers/plans/2026-08-27-design-file-repository-scope.md`；
- `docs/superpowers/plans/2026-08-27-design-center-repository-read-projection.md`。

它们对应已经完成的仓库关联和读取投影地基，不是当前待执行计划。

### 2.5 延后计划

- `docs/superpowers/plans/2026-08-31-design-center-finder-repository-workspaces.md`。

完整 Finder、多项目 Tab、批量关联、实时精雕等内容继续冻结到 End-to-End MVP 验收之后。不要因为当前首页有问题就直接执行整份 Finder 计划；先查明当前页面与用户已确认设计的偏差。

---

## 3. Git 与工作树状态

### 3.1 主 checkout

```text
路径：/Users/fengyujie/Documents/soyoung/multica
分支：main
代码 HEAD：a7606af71
```

主 checkout 仍有用户保留修改和未跟踪文档。不要清理、重置、提交或覆盖。

本次判断：

- `AGENTS.md`、`CLAUDE.md` 的修改只更新 GitNexus symbol/relationship 数量；这是易漂移的本地索引快照，不属于 Design Center 产品事实，因此没有带入 MVP 集成分支；
- 9 份 Design Center Plan/Spec 有产品、执行或审计价值，已带入 MVP 集成分支；
- 原交接文档已重写为本文。

### 3.2 MVP 集成分支

```text
路径：/Users/fengyujie/Documents/soyoung/multica/.worktrees/design-center-end-to-end-mvp-integration
分支：codex/design-center-end-to-end-mvp
文档归档前代码 HEAD：e06467e5c
```

`e06467e5c` 已包含 Task 1–6 / Group 1–2 的产品代码和测试。

本文和相关 Plan/Spec 会作为独立文档提交位于该代码基线之上。下一位 Agent 应使用 `git rev-parse HEAD` 获取最新文档提交，不要假定 HEAD 仍是 `e06467e5c`。

### 3.3 Group 3

```text
路径：/Users/fengyujie/Documents/soyoung/multica/.worktrees/design-center-end-to-end-mvp-group-3
分支：codex/design-center-end-to-end-mvp-group-3
HEAD：e06467e5c
状态：Task 7–9 尚未合入
```

### 3.4 Task 7

```text
路径：/Users/fengyujie/Documents/soyoung/multica/.worktrees/design-center-end-to-end-mvp-task-7
分支：codex/design-center-end-to-end-mvp-task-7
HEAD：e06467e5c
状态：clean，无代码、报告或提交
```

不要把集成分支的文档提交误当成 Task 7 产品代码。需要继续 Task 7 时，先从集成工作树读取本文，再进入 Task 7 工作树。

---

## 4. 已完成的产品能力：Task 1–6

### 4.1 Group 1 / Task 1–3

已完成：

- 修复 repository grounding completion 基线；
- 建立最小项目/仓库视角和单份设计稿仓库关联；
- 区分“关联仓库”和“已经通过 grounding”；
- 仓库工作流读取精确 repository-specific design system；
- 禁止仓库工作流静默回退到项目通用设计体系；
- 跨项目、跨仓库和删除清理边界已覆盖。

Group 1 已合入 MVP 集成分支。

### 4.2 Group 2 / Task 4–6

已完成：

- Home 和 Repository 入口复用同一个设计体系创建表单；
- Repository 入口预填项目、仓库和 GitHub 来源；
- 创建和生成固定仓库 scope，不允许 repository workflow fallback；
- Design Document revision 保存 design-system revision provenance；
- 历史 Design Document 不随设计体系后续修改而漂移；
- 生成监控、Preview、调整、保存与“基于最新体系分叉”链路已接入；
- 与 V2 repository provenance 相关的测试修复已完成。

Group 2 已合入 MVP 集成分支，集成代码基线为 `e06467e5c`。

### 4.3 重要边界

以上表示工程能力已经落地，不表示最终 UI、首页信息架构、真实视觉质量或完整产品链路已经被用户接受。

特别是：lint、测试、HTTP 200、页面能打开，都不能替代用户可见的 UI 验收。

---

## 5. 当前暂停位置：Task 7

### 5.1 Task 7 原目标

真实执行：

```text
真实项目/仓库
→ repository-specific saved design system revision
→ Design A：列表/表格/表单
→ Design B：详情/工作流/状态/Overlay
→ 监控 / Audit / Preview
→ 至少一次调整
→ 保存两份 Design Document
→ 验证 source / status / grounding / provenance
```

Task 7 最终只应提交真实中文验收报告：

```text
docs/product/design-center/mvp-phase-a-validation.md
```

如果验收暴露产品或工具缺陷，修复必须是独立聚焦提交，不能把失败包装成 PASS 报告。

### 5.2 已完成的真实环境恢复

曾经完成并验证：

- 隔离 PostgreSQL：宿主端口 `15432`；
- 数据库：`multica_design_center_end_to_end_mvp_task_7_392`；
- migrations：576 条，最新 `909_design_document_revision_repository_grounding`；
- API：`18472`，`/health`、`/readyz` 通过，commit `e06467e5c`；
- Web：通过单组件替代路径在 `13392` 真实返回页面；
- daemon profile：`dev-design_center_end_to_end_mvp_task_7-392`；
- daemon/runtime 曾经真实运行；
- 未停止、复用或修改 WindRider 占用的 `5432`；
- Web launcher 的 process-group ownership 检查曾误杀已经返回 200 的 Next 进程，因此继续时不要机械重复相同 `make up`。

### 5.3 已创建的真实数据

Task 7 隔离数据库中已创建：

- Workspace：`Dev`；
- Project：`真实 CRM 仓库设计体系与双设计稿端到端验收`；
- Repository：`coder-zkl1988/multica`；
- Agent：`Design Gate Agent`；
- 可用 runtime：曾验证存在多个 online runtime，实际选择过 Codex runtime；
- Repository analysis task：曾启动，后因用户要求暂停而安全取消。

没有完成：

- repository analysis 成功结果；
- repository-specific design system generation；
- Audit / Preview；
- saved design-system revision；
- Design A / Design B；
- adjustment；
- 两份文档保存；
- Task 7 验收报告或提交。

### 5.4 当前环境已停止

用户要求暂停后已经完成：

- repository analysis task 状态变为 `cancelled`；
- API、Web、daemon、desktop 均停止；
- 隔离数据库、环境注册、Workspace、Project、Repository 和 Agent 保留；
- Task 7 worktree clean；
- 没有运行中的后台执行任务；
- Task 8、Task 9 尚未开始。

恢复时优先复用现有隔离数据和 profile，不要重新创建重复 Workspace、Project、Repository 或 Agent。

---

## 6. 当前最高优先级未解决问题：首页与页面结构不一致

用户查看 Task 7 截图后明确提出：

- “这个图片是什么？”
- “为什么还会有这样的一个页面？”
- “首页为什么不是我之前的设计了？”

截图中可见：

- 固定“首页”Tab；
- 项目/仓库选择条；
- 空的“设计稿”“设计草稿”区块；
- 仓库专属“创建设计体系”表单；
- 侧边栏 Designs 入口。

这张图只证明当前 Task 7 分支能够进入某个 Design Center 项目/仓库状态，不证明它是用户此前认可的首页，也不证明当前信息架构正确。

**目前尚未完成根因诊断。禁止把截图中的页面直接当作已验收 UI。**

下一位 Agent 首先要只读查明：

1. 截图实际 URL、路由和组件；
2. 它是 Home、项目工作区、仓库工作区，还是路由/状态混合导致的错误页面；
3. Task 2 / Task 4 的最小 MVP 视图是否覆盖或替换了此前用户认可的首页；
4. 此前认可的首页设计对应哪个组件、Spec、截图或提交；
5. 当前差异是有意的 MVP 临时界面、状态选择错误，还是 UI 回归；
6. 需要恢复原首页、调整路由，还是把仓库创建表单放回正确层级。

完成诊断后先向用户解释，不要立即改代码。用户确认修复方向后再继续。

---

## 7. 恢复 Group 3 的正确顺序

用户确认首页/路由处理方向后：

### 7.1 先处理 UI 偏差

- 修改前读取 `docs/product/design-center/README.md`；
- 复现用户截图和此前认可首页；
- 修改已有函数/组件前执行 GitNexus upstream impact；
- HIGH / CRITICAL 风险先告知用户；
- 使用最小写集修复；
- 真实浏览器验证，不以测试或 HTTP 200 代替；
- 修复作为独立提交，不 amend。

### 7.2 再恢复 Task 7

- 复用 Task 7 worktree 和已有隔离数据库；
- 检查 `make status`；
- 恢复 API、Web、daemon/runtime；
- Web launcher 若继续误判，不重复同一失败命令，使用已验证的单组件环境执行路径，并记录 listener PID、cwd、端口和 commit identity；
- 复用现有 Workspace / Project / Repository / Agent；
- 重新发起 repository analysis；
- 完成 repository-specific design system、Design A/B、调整、Audit、Preview、保存；
- 生成中文验收报告；
- 验证通过后提交 Task 7。

### 7.3 Task 8

目标：统一 `design_ref` / `frame_ref` 和 Frame API。

核心边界：

- `design_ref` 是不透明、版本化、来源无关的外部引用；
- Multica 只接受 saved revision，draft 拒绝；
- Figma 固定 valid uploaded revision；
- `frame_ref` 对应 Figma Frame/Group 或 Design Document Page；
- `GET /api/design-assets/{design_ref}/frames`；
- 覆盖 tampering、cross-workspace/project、stale revision、draft rejection 和两种来源；
- 不提前做多 Frame、多 Layer 或任意区域选择。

提交信息：

```text
feat(designs): add unified design and frame references
```

### 7.4 Task 9

目标：Implementation Prompt / Context / Result 契约。

核心边界：

- `POST /api/design-assets/{design_ref}/implementation-prompt`；
- MCP `get_implementation_context`；
- `implementation-result/v1`；
- 固定 workspace/project/issue、design_ref/frame_ref、target repository、saved/valid evidence、design-system provenance、write boundary 和 verification requirements；
- Prompt 只返回给用户审阅/预填，不自动发送评论或启动 Agent；
- 不自动 commit、Push、PR 或改变 Issue 状态；
- API 和 MCP 必须产生同一个冻结 context identity。

提交信息：

```text
feat(designs): define implementation context and result contracts
```

### 7.5 Group 3 收口

- 每个 Task 保留聚焦提交；修复使用独立提交，不 amend；
- Task 7、8、9 分别合入 `codex/design-center-end-to-end-mvp-group-3`；
- 合并使用 `git merge --no-ff`；
- 三个 Task 完成后执行一次 changed-surface Group Gate；
- Gate 通过后 `--no-ff` 合入 `codex/design-center-end-to-end-mvp`；
- 不 Push、不创建 PR、不合入 `main`，除非用户明确批准；
- Group 3 完成后停止并做产品视角汇报。

---

## 8. 环境与安全边界

- `windrider-postgres` 使用宿主机 `5432`；不得停止、重启、删除、改配或作为 Multica DB；
- Task 7 Multica DB 使用隔离端口 `15432`；
- 不删除已有容器、数据库、profile、Workspace、Project、Repository、Agent 或截图证据，除非用户明确要求；
- 不输出 PAT、JWT、Cookie、验证码、数据库密码、API Key 或内部 object key；
- 不通过 mock、HTTP 200、typecheck 或单元测试伪装真实产品验收；
- “页面能渲染”不等于“视觉质量通过”；
- “关联仓库”不等于“repository_grounded=true”；
- Design Document draft 不能进入代码还原；
- 设计创建、保存或还原不能自动改变 Issue 状态；
- 不自动 commit、Push 或创建 PR。

---

## 9. 当前明确的 NOT DONE

- 用户认可的首页与当前截图页面之间的差异尚未诊断；
- Task 7 真实 Gate 未完成；
- `mvp-phase-a-validation.md` 不存在；
- Task 8、Task 9 未开始；
- Group 3 未合入 MVP 集成分支；
- Multica/Figma 统一还原未完成；
- Issue 设计选择/创建/恢复闭环未完成；
- 最终 Task 14 端到端验收未完成；
- Post-MVP Finder/视觉精雕不得提前执行。

---

## 10. 给下一位 Agent 的启动提示词

```text
请先保持只读，不要启动服务、不要修改代码。

当前 Design Center End-to-End MVP 的唯一权威交接文档是：

/Users/fengyujie/Documents/soyoung/multica/.worktrees/design-center-end-to-end-mvp-integration/docs/superpowers/handoffs/2026-09-01-design-center-end-to-end-mvp-handoff.md

先完整读取它，再读取：

/Users/fengyujie/Documents/soyoung/multica/.worktrees/design-center-end-to-end-mvp-integration/docs/product/design-center/README.md
/Users/fengyujie/Documents/soyoung/multica/.worktrees/design-center-end-to-end-mvp-integration/docs/superpowers/plans/design-center-active-index.md

随后只读检查：

1. MVP 集成分支和 Task 7 worktree 的 branch / HEAD / dirty state；
2. Task 7 的 make status，但不要启动服务；
3. 用户截图对应的真实 URL、路由、页面组件和状态来源；
4. 此前用户认可的 Design Center 首页设计对应的代码、文档或提交；
5. 为什么当前截图出现了项目/仓库选择条、空的设计稿区和仓库创建设计体系表单；
6. 当前实现是临时 MVP 页面、路由状态错误还是 UI 回归。

第一份回复只向用户说明：截图是什么、为什么出现、首页为什么变化、当前正确修复方向。不要直接修改代码，等待用户确认后再恢复 Group 3。
```
