# Multica 设计中心长期产品记忆

> 状态：持续维护
> 最后更新：2026-07-28
> 适用范围：设计中心、设计体系、UI 规范、设计任务、UI Agent、设计稿生成、设计还原、设计 MCP、Open Design 能力接入

## 1. 这份模块解决什么问题

这不是某一阶段的实现计划，而是 Multica 设计产品线的长期事实源。

它负责保存：

- 已经由产品讨论确认的方向；
- 有明确版本和源码依据的外部研究结论；
- 仍在讨论、尚未批准的提案；
- 已暂停、已否决或被替代的历史路线；
- 最终实现前必须解决的开放问题。

历史聊天摘要只能帮助恢复对话，不能替代本模块中的证据和决策状态。

## 2. 强制上下文恢复规则

任何 Agent 在以下情况继续设计产品线工作前，都必须重新读取本文件：

1. 新会话开始；
2. 会话发生上下文压缩；
3. 工作被其他任务打断后恢复；
4. 准备提出产品结论、技术方案或实现计划；
5. 准备修改设计中心、UI Agent、设计体系或 Open Design 接入相关代码。

随后按需要读取：

- [decision-register.md](./decision-register.md)：确认当前哪些内容已经决定，哪些仍是提案；
- [open-design-evidence.md](./open-design-evidence.md)：凡涉及 Open Design 的判断，必须回到对应版本和源码证据；
- [open-design-multica-mapping.md](./open-design-multica-mapping.md)：当前 Open Design 契约到 Multica 云端实体和 Agent 上下文的最小映射提案；
- [project-design-system-validation.md](./project-design-system-validation.md)：项目设计体系第一阶段的真实链路、持久化与失败保护证据，以及尚未完成的验收项；
- 历史文档：只用于了解已有实现和失败经验，不自动继承为当前方案。

恢复后必须遵守：

- 不把聊天摘要中的推断提升为已确认决策；
- 不把旧实现的存在当成继续沿用它的理由；
- 不把某个 Open Design 版本的行为描述成永久事实；
- 不以任务完成、草稿通过或测试通过代替真实产物和视觉质量验证。

## 3. 记录等级

本模块只使用以下状态：

| 状态 | 含义 |
| --- | --- |
| `confirmed` | 用户已经明确确认，可以约束后续方案 |
| `evidence` | 已通过指定版本、源码、运行结果或持久化数据验证 |
| `proposal` | 值得讨论，但尚未获得确认 |
| `open` | 仍需研究或做产品选择 |
| `paused` | 当前停止推进，但可能保留局部价值 |
| `rejected` | 已明确否决，不得悄悄重新引入 |
| `superseded` | 曾经成立，后来被新决策替代 |

只有 `confirmed` 决策可以直接进入实现方针。`evidence` 负责支撑判断，但证据本身不等于 Multica 必须照搬。

## 4. 已确认的产品地基

### P-001 Multica 的目标

`confirmed`

Multica 以项目为切入点，通过 Issue 连接需求、设计、开发和最终落地，并结合云端 Agent 与本地 Agent 完成完整的软件交付流程。

### P-002 人与 Agent 的关系

`confirmed`

人和 Agent 必须共享同一份工作上下文。人可以随时观察、介入和接管，之后也可以把任务重新交还 Agent，不能因为接管导致上下文和产物链断裂。

### P-003 设计能力的产品位置

`confirmed`

设计稿上传、UI 规范、设计体系、设计稿生成、MCP 和设计稿还原都只是需求交付流程中的能力，不是脱离 Project 和 Issue 独立运行的最终产品。

### P-004 Open Design 的产品位置

`confirmed`

Multica 研究并选择性接入 Open Design 的部分能力，用于补充在线设计生产、设计体系和未来模板能力。目标不是复制 Open Design，也不是用它替换 Multica 的 Project、Issue 和 Agent 控制面。

### P-005 设计体系是源，UI 规范是派生产物

`confirmed`

项目设计体系是 Multica 管理的设计事实源。它至少同时包含 Agent 可读规则和机器可执行 Tokens，并根据真实来源逐步增加组件、状态与模式。Multica 根据设计体系生成在线 UI Kit，供人预览、调整和 Agent 使用。

Figma UI 规范不是建立设计体系的硬性要求。已有 Figma UI 规范可以作为可选导入证据；未来也可以由设计体系生成或同步原生 Figma UI Kit，但原生 Figma 写回不属于第一阶段。

没有来源材料、也没有用户主动创建意图的空项目必须保持未建立状态，不能由 Agent 自动猜测设计体系。用户明确发起从零创建时，可以根据产品定位、品牌材料或参考风格生成草稿，预览调整后保存为项目设计体系。

### P-006 第一版采用 Open Design 设计体系规则

`confirmed`

Multica 第一版直接采用 Open Design 的设计体系基础契约，不再另行设计统一 UI 规范表单或新的 Token 分类。最小正式包为 `manifest.json`、`DESIGN.md` 和 `tokens.css`，并按真实来源选择性增加组件、预览、UI Kit、来源证据、资产和字体。

Token 分层、草稿与发布、一个主体系加多个弱参考等规则沿用 Open Design。Multica 只负责将这些规则适配为云端项目资产，并接入现有 Project、Issue、Design Run 和 Agent，不复制 Open Design 的本地 Project 与 daemon。

### P-007 旧决策：先实现 Open Design 契约的云端映射

`superseded`

现有 `design_system_profile` 演进为设计体系稳定身份，设计内容进入可审核、可发布、可固定引用的 `design_system_revision`，Project 通过 `project_design_system_binding` 使用一个已发布主 revision 和零到多个弱参考 revision。

发布 revision 不可变，Project 不自动追随新版本。owner project 首次发布时可通过“发布并设为项目主体系”完成显式绑定。旧 Figma UI 规范、`profile_json` 和 `is_default` 只迁移为待审核来源与草稿，不能自动发布或自动绑定。

UI 设计和设计还原共用统一 Design Context Resolver，并在任务中固定 revision 与内容摘要。完整方案见 [open-design-multica-mapping.md](./open-design-multica-mapping.md)。

替代原因：这把底层资产治理和设计还原接入误当成了当前产品目标。相关模型可以作为后续技术研究，但不能先于用户可见的设计体系创建与管理闭环实施。

### P-008 第一阶段先完成项目设计体系的创建与管理闭环

`confirmed`

第一阶段的产品目标是在 Multica 的项目设计模块中，实现一套参考 Open Design 的设计体系创建与管理能力，而不是先替换旧 Profile、建设完整版本治理或接入设计还原。

用户主动为项目创建设计体系，并可提供项目定位、品牌资料、参考风格或已有设计资产。Agent 按 Open Design 的规则生成设计体系，Multica 在线展示设计规则、Tokens、组件和可视化 UI Kit，用户可以预览、调整并保存为项目长期资产。

第一阶段以用户能否生成、看懂并获得一套有价值的项目设计体系作为成功标准。Figma UI 规范可以作为输入，也可以在未来由设计体系派生，但不是前置条件；空项目在用户没有主动发起时不得自动生成。

本阶段暂不接入设计还原，不迁移旧 `design_system_profile`，不先建设完整的 revision、binding 和包审计基础设施。数据模型只服务于上述用户闭环，版本治理与 Agent 消费在闭环验证后单独设计。

已确认的第一阶段用户流程：

```mermaid
flowchart LR
    A["设计中心选择项目"] --> B["进入设计体系"]
    B --> C["未建立设计体系"]
    C --> D["创建设计体系"]
    D --> E["选择执行 Agent"]
    E --> F["填写项目与品牌信息"]
    F --> G["添加可选参考资料"]
    G --> H["Agent 生成设计体系草稿"]
    H --> I["在线 UI Kit 预览与调整"]
    I --> J["保存为项目设计体系"]
```

第一阶段不新增前置的仓库扫描 Agent。现有 `design_repo_analysis` 只是设计还原链路中的浅层、本地规则扫描，不作为本流程的依赖；未来若建设通用项目工程分析，应作为独立能力重新设计。

创建输入采用“自然语言为主、结构化信息为辅”。系统自动带入已有项目名称和描述，用户主要描述产品定位、目标用户、核心场景和期望风格；Logo、品牌色、截图、Figma UI 规范和参考设计等材料均为可选输入，不要求用户填写冗长的设计规范表单。

创建页必须由用户选择执行 Agent。目标平台是唯一必选的结构化设计信息，第一版提供 Web、移动端和跨端；平台会直接影响组件形态、交互模式和信息密度，其他设计信息继续由自然语言和可选参考资料表达。Agent 选择属于任务负责人，不计入设计信息表单。

第一阶段不设置任何审核状态或设计审核权限。预览与调整只是普通编辑过程，不产生“待审核、通过、驳回”等状态；拥有项目编辑权限的用户可以调整并保存设计体系。

Agent 每次直接生成一套内部一致的设计体系草稿，不先生成多套风格方向让用户选择。用户通过在线 UI Kit 预览实际效果，需要变化时直接调整当前草稿或重新生成。

第一阶段的 Agent 草稿产物固定包含 `DESIGN.md`、`tokens.css` 和 `components.html`。`DESIGN.md` 表达设计意图与规则，`tokens.css` 提供可执行语义 Tokens，`components.html` 使用这些 Tokens 组成真实组件、状态和组合效果。Multica 在隔离环境中展示静态 HTML/CSS，不执行其中的任意脚本。第一阶段不建设自研画布或固定组件渲染器。

上述文件是系统与 Agent 的内部产物契约，不是用户界面的信息架构。设计体系详情必须像 Open Design 的主视图一样展示具体设计体系内容：将设计意图组织为可阅读的动态章节，将 Tokens 展示为色彩、字体、间距等视觉内容，并将组件与状态展示为在线 UI Kit。第一阶段不展示文件树、原始文件名或代码编辑入口。

草稿调整采用“组件或区块定位 + 自然语言指令”。用户可以对整个体系提出要求，也可以先在 UI Kit 中定位某个组件或区块再描述修改；Agent 必须同步更新受影响的 `DESIGN.md`、`tokens.css` 和 `components.html`，随后刷新预览。第一阶段不提供 Token 表单、代码编辑器或拖拽画布，调整失败时保留修改前草稿。

第一阶段每个项目只维护一套当前设计体系。Agent 生成和调整的内容自动保留为草稿，用户点击“保存为项目设计体系”后进入已保存状态；后续继续调整同一套体系。第一阶段不提供多体系选择、主体系/参考体系绑定或历史版本，彻底重做时必须明确提示将替换当前体系。

## 5. 当前讨论范围

以下内容来自 2026-07-28 的讨论：

1. **项目设计体系**，`confirmed`：项目拥有可生成、预览、调整和保存的云端设计体系；在线 UI Kit 是它的派生视图，Figma UI 规范是可选输入。
2. **设计体系规则基线**，`confirmed`：第一版直接采用 Open Design 的包结构、Token 分层和可选扩展，不再扩展一套平行模型；不照搬其修订审核工作流。
3. **第一阶段产品闭环**，`confirmed`：用户在项目设计模块主动创建或生成设计体系，在线查看设计规则、Tokens、组件与 UI Kit，预览调整后保存为项目长期资产。
4. **设计任务发起器**，`proposal`：以现有设计中心为入口，沿用设计中心的项目切换能力，形成项目范围内的设计工作空间。
5. **设计任务模板**，`proposal`：在设计体系和任务发起器落地后再设计，届时继续研究 Open Design 的模板机制。

当前尚未确认的细节只保留 Multica 落地问题：

- 未建立设计体系的入口和用户主动创建流程如何呈现；
- 创建时最少需要用户提供哪些信息，已有资产如何作为可选输入；
- Agent 生成哪些用户可理解、可预览和可调整的内容；
- 在线 UI Kit 如何真实呈现 Tokens、组件和规则，而不是只展示文件元数据；
- 第一阶段闭环所需的最小持久化模型；
- 设计中心发起的设计任务如何与 Issue 建立强关联；
- 在线设计的主产物格式和编辑模型；
- 何时以及如何支持原生 Figma UI Kit 写回。

## 6. 历史资料的使用方式

下列文档记录了真实实现和阶段性决策，但它们不再自动代表当前产品方向：

- [../design-restore-memory.md](../design-restore-memory.md)：Figma 上传、查看、MCP 和设计还原的长期历史；
- [../project-design-contract-roadmap.md](../project-design-contract-roadmap.md)：早期云端设计契约路线；
- [../../superpowers/specs/2026-07-08-design-system-profile-mvp-design.md](../../superpowers/specs/2026-07-08-design-system-profile-mvp-design.md)：第一版 Design System Profile；
- [../../superpowers/specs/2026-07-21-semantic-ui-agent-design-generation-design.md](../../superpowers/specs/2026-07-21-semantic-ui-agent-design-generation-design.md)：基于 `PageSpec` 和编译器的 B 端结构化生成路线。

使用历史资料时必须先回答：

1. 它描述的是产品目标、实验方案还是已经存在的代码？
2. 它的日期是否早于当前决策？
3. 它是否已经被后续用户反馈暂停、否决或替代？
4. 它能提供什么经验，而不是要求我们继续维护什么包袱？

## 7. 更新协议

每轮讨论结束后，只做以下增量更新：

- 用户明确确认或否决的内容，写入决策台账；
- 新的外部研究结果，写入证据台账并标明版本、提交和日期；
- 新提案先记录为 `proposal`，不得提前改成 `confirmed`；
- 被替代的决策保留原文并改为 `superseded`，不得删除历史；
- 实现结果必须附真实验证证据，不能只记录“任务已完成”。

## 8. 当前下一议题

下一轮按已确认流程逐段设计，继续明确 Agent 选择、生成过程、设计体系内容主视图和保存体验。用户确认完整产品设计后，才能编写新的最小实施计划。
