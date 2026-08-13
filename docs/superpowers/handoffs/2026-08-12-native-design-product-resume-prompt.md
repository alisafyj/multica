# Native Design 产品讨论恢复提示词

> 用途：把下面“可复制提示词”完整交给任何新的 Agent，用于恢复 Multica Design Center、Native V2、新 Phase A、首页、共享设计体系和模板的产品上下文。
>
> 注意：本文件是恢复入口，不是新的事实源。Agent 必须重新读取下列权威文档并核对当前 Git 状态，不能把本提示词中的时间点状态当成永久事实。

## 可复制提示词

```text
你正在继续 Multica 的 Design Center / Native V2 产品工作。请先恢复上下文，再进入问答；不要立即写代码、创建实施计划、恢复旧 Phase B，或从历史分支迁移提交。

仓库：/Users/fengyujie/Documents/soyoung/multica

一、强制读取顺序

1. 读取根 CLAUDE.md；它要求任何 Design Center、设计体系、UI Agent、设计生成、设计还原、Design MCP 或 Open Design 工作先读取产品记忆。
2. 读取 docs/product/design-center/README.md，作为 Design Center 当前产品记忆入口。
3. 读取 docs/product/design-center/decision-register.md，重点核对：
   - DC-017：第一阶段项目设计体系闭环；
   - DC-031 至 DC-036：首页/项目 Tab、设计体系工作台、连续内容画布、真实产物门禁、设计上下文和仓库分析；
   - DC-039：以 Open Design 为行为基线，使用 Multica 原生基础设施；
   - DC-040：取消独立大爆炸式 Phase B，改用 Native V2 产品切片内渐进清理；
   - DC-041：设计中心首页、工作区共享设计体系和社区模板分期。
4. 读取 docs/superpowers/specs/2026-08-12-native-design-slice-driven-evolution-design.md。
5. 读取 docs/product/design-center/native-v2-retirement-register.md。
6. 读取 docs/superpowers/specs/2026-08-12-design-home-public-systems-community-templates-design.md。
7. 按当前问题需要读取：
   - docs/product/design-center/open-design-evidence.md：Open Design 固定版本源码和真实实验事实；
   - docs/product/design-center/design-center-issue-product-overview.md：历史产品全景，只作历史，不覆盖当前决策；
   - docs/superpowers/specs/2026-08-05-multica-native-design-engine-design.md：Native V2 原生引擎方向；
   - docs/product/design-center/project-design-system-validation.md：已执行验证证据。
8. 以下两份文档已 superseded，只能读历史，禁止继续执行其中 Phase B 任务、命令和验收：
   - docs/superpowers/specs/2026-08-12-native-design-phase-1-closure-and-legacy-removal-design.md；
   - docs/superpowers/plans/2026-08-12-open-design-v1-destructive-removal.md。

二、先核对当前仓库事实

在作任何结论前，只读核对：

- git status --short --branch
- git log -5 --oneline --decorate
- git branch -vv

不要混淆：

- 当前产品基线应从 feature/fengchen 的实际 HEAD 和工作树判断；
- feature/fengchen-fixed-v2 是被取消的独立 Phase B 路线 checkpoint，不合入、不 cherry-pick、不作为实现基线，也不计入产品进度；
- 如果分支或文档状态已变化，以当前 Git 和权威文档为准，并指出与本提示词时间点的差异。

三、产品目标和长期记忆

Multica 是以 Project 和 Issue 为主控制面的人与 Agent 协作软件交付平台。设计能力不是独立 AI 设计玩具，而是需求、设计、开发和交付流水线的一部分。

项目设计体系是项目的设计事实源：

- 项目可以没有设计体系；
- 用户主动选择 Agent 和目标平台，通过自然语言、仓库事实、品牌资料和可选参考创建；
- Agent 生成完整 Native V2 package；
- Multica 负责输入快照、任务编排、隔离工作区、安全收集、对象存储、Package Audit、Chromium Preview、draft/saved 生命周期和权限；
- 用户在线查看设计规则、Tokens、组件状态和 UI Kit，调整后保存；
- 下游 Agent 只读取已保存体系，不把未保存 draft 当项目强约束；
- 不引入待审核、批准、驳回等设计审核流。

Open Design 的角色：

- 作为产品流程、能力语义、分层包、模板、Audit、Preview 和失败隔离的核心行为参照；
- 不运行、分发或托管 Open Design Worker/Daemon/Runtime 作为正式产品依赖；
- 历史 Worker/Phase 0 真实实验保留为证据，不自动成为当前架构。

四、已确认的渐进清理规则

独立、一次性、跨 daemon/handler/router/SQL/frontend/migration 的 Phase B 已取消。后续只沿 Native V2 产品功能切片推进。

使用两级规则：

1. 切片内部已经无调用、已被完整替代的 V1/Worker 死代码必须删除，并阻塞切片完成。
2. 跨切片、跨数据生命周期或仍有旧 Desktop、CLI、MCP、运维等外部消费者的残留，不得扩大当前切片，必须登记到 native-v2-retirement-register.md。

退役账本状态只有：

- active
- write-retired
- unreferenced
- retired
- data-pending

普通功能切片不删除历史行、对象、表或约束。open_design_run、non-V2 package rows 和历史 archive/evidence/Preview 对象的不可逆清理必须最后单独提出、审批和验证。

实现中新发现以下任一情况时停止扩大清理并登记账本：进入第二个产品能力、改变多个数据生命周期、需要 destructive migration/对象删除、关闭多个范围外 API、修改通用 Agent lifecycle、同时协调 Server+daemon+Web/Desktop+DB、无法独立回滚、存在外部消费者、超出批准文件/符号/API/数据范围。

五、新 Phase A 当前边界

路线状态 confirmed；新 Phase A 的产品设计已经完成确认，实施尚未开始。

已确认进入新 Phase A 的新增产品能力：设计中心首页页面设计任务入口及其 `multica.design-document/v1` 页面 Design Document 闭环。完整增量方案见 `docs/superpowers/specs/2026-08-12-native-design-phase-a-design-document-design.md` 和 DC-042 至 DC-046。

首页第一版：

- 是跨项目页面设计任务发起器，不是无项目画布，不创建第二套 Project；
- 用户自然语言输入页面设计需求，可附截图/参考/附件；
- 目标项目和执行 Agent 必选，目标平台可选；
- 第一版只生成项目内页面设计稿或可运行原型；
- 设计体系创建继续留在项目“设计体系”Tab；
- 任务创建成功后打开/聚焦目标项目 Tab，切到“设计草稿”；
- 首页和项目 Tab 读取同一个服务端 task/draft；
- 创建失败保留输入，不创建半任务，不导航；
- 项目 saved 设计体系自动作为强约束；
- 不默认使用旧 PageSpec 编译器；
- 第一版不做无项目设计、自动意图识别、社区信息流、多模板组合或自动交付前端。

新 Phase A 已确认：

- `multica.design-document/v1` 页面设计产物协议；
- 真实 Agent 在任务内执行方式；
- 任务内自动只读项目仓库 grounding；
- 离线交互原型、现有本地 `designpreview` 强制 Preview、调整、保存和放弃闭环；
- 用户 Chrome 视觉/Network/Console 和人工质量验收要求；
- 失败隔离、持久化证据和 A1 至 A6 内部子切片。

当前产品设计进度为 100% 已确认；代码实施尚未开始。先前约 48% 只是确认前的范围讨论基线，不再作为当前进度。

六、后续独立切片

Slice B：工作区共享设计体系资源库

- 不给项目体系直接加 is_public；
- 从项目 saved Native V2 package 重新校验和安全剥离，生成不可变共享 revision；
- 第一阶段仅工作区成员可发现和使用；
- 项目后续调整不影响已发布 revision；
- 首页固定 resource_id/revision_id/digest/sanitized snapshot 作为弱参考；
- 项目 saved 体系始终是强约束；
- 无项目体系时只能“仅本次参考”或显式“复制到项目 draft”，不能隐式建立体系；
- 下架阻止新引用，历史任务继续读取固定 revision；
- 来源项目删除不删除已发布资源。

Slice C：官方模板目录

- 模板是页面设计任务配方，不是设计体系；
- 固定 template revision 和 manifest digest；
- 首页应用模板时固定项目、Agent、用户需求、设计体系 revision、附件和平台快照；
- 先验证任务效率和生成质量，不建设 Marketplace 治理。

Slice D：工作区成员模板发布

- 从成功任务或已保存设计资产显式发布；
- 安全剥离并重新 Audit/Preview；
- 工作区权限、版本、下架、归档；
- 禁止任意脚本、外连、二进制和自由服务器命令。

Slice E：跨工作区社区模板

- 远期建设发现、作者、许可、举报、审核、下架、封禁、推荐和派生关系；
- 不属于新 Phase A，不进入当前实施计划。

七、四类实体必须分开

- Project design system：项目当前强约束事实源；
- Published design-system resource：安全剥离的不可变共享视觉参考；
- Template resource：页面设计任务配方；
- Applied template snapshot：某次任务实际使用的固定上下文。

不要用一个 is_public 或旧 template 表直接承载全部语义。现有 design_template_library/design_catalog_template/design_template_revision 只作为迁移输入与部分基础；先定义产品契约，再决定数据库实现。

八、当前工作方式

当前进入产品问答模式：

- 一次只问一个真正改变方案的问题；
- 不把 proposal 提前写成 confirmed；
- 不从历史实现反推产品；
- 不创建实施计划，不写代码，直到新 Phase A 规格逐节确认并经用户批准；
- 如果需要修改产品决策，更新 README、decision-register 和对应 specs，保留历史 superseded 文档；
- 任何实现计划以后都必须按 Native V2 产品切片组织，并携带局部清理门禁、退役账本变化、V2 正向、失败隔离、旧路径负向、范围外业务回归、持久化不变量和实际验证命令。

九、2026-08-12 后续更新

本提示词生成后的产品问答已经完成新 Phase A 的详细确认。新的 Agent 必须继续读取：

- docs/superpowers/specs/2026-08-12-native-design-phase-a-design-document-design.md；
- decision-register.md 中的 DC-042 至 DC-046。

已确认：页面产物为 `multica.design-document/v1`；一个项目可有多份 Design Document；每份文档使用不可变 revisions 与 draft/saved 指针；项目和智能体必选、任务（Issue）可选；页面 task 内自动完成有界只读仓库 Grounding；每份文档拥有持续工作空间、每次调整使用独立 task；离线交互原型直接复用现有本地 `designpreview` 强制门禁且无降级；Phase A 按 A1 至 A6 内部子切片实施。

你现在应：

1. 简短报告读取的事实源、当前分支与工作树状态；
2. 用 confirmed / open / superseded 区分结论；
3. 等待用户复核书面规格；
4. 用户批准后才使用 writing-plans 为 A1 至 A6 编写详细实施计划；
5. 不运行旧 Phase B，不使用 feature/fengchen-fixed-v2，不提前开始代码实现。
```
