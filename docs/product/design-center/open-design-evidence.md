# Open Design 研究证据台账

> 研究日期：2026-07-28，补充于 2026-08-04
> 目的：记录可复核事实，不直接替 Multica 做产品决策

## 1. 研究快照

本轮研究使用两个时间点：

| 对象 | 已验证值 | 说明 |
| --- | --- | --- |
| 本机应用 | `/Applications/Open Design.app` | 已确认安装 |
| 本机研究版本 | `0.16.1` | 来自源码快照根 `package.json` |
| 本机研究提交 | `e0d91046dd8dfe7ab9b8f17d0de127c6f5183e4e` | 本轮主要源码证据固定在该提交 |
| 上游仓库 | `nexu-io/open-design` | Apache-2.0，许可证值来自该提交根 `package.json` |
| 研究时上游 HEAD | `89d6d4ef21baf80f871595abdf6f7de6e941dd44` | 2026-07-28 重新执行 `git ls-remote` 获得，并已 fetch 到本地研究仓库 |

本机应用对应的研究提交落后于研究时的上游 HEAD。对设计体系关键路径执行提交间文件差异检查后，`DesignSystemFlow.tsx`、`apps/daemon/src/design-systems/`、`apps/daemon/src/brands/`、`brand-routes.ts`、`prompts/system.ts`、`design-systems/README.md` 和 `docs/design-systems.md` 在 `e0d9104..89d6d4e` 之间没有差异。因此，OD-001 至 OD-006 保留原研究提交依据，OD-007 至 OD-012 同时按 `89d6d4e` 复核。后续涉及 Open Design 当前能力时仍必须重新核对上游。

2026-07-31 对官方 Release 重新核对：GitHub Releases API 返回最新稳定 Tag `open-design-v0.16.1`，`target_commitish` 为 `276b4d8e970bc143d7ad060181a89a834e3d9caf`，发布时间为 2026-07-23，且不是 prerelease。OD-016 至 OD-020 固定在该 Release commit，不使用当日 `main` 推断稳定契约。

固定源码链接：

- [根 package.json](https://github.com/nexu-io/open-design/blob/e0d91046dd8dfe7ab9b8f17d0de127c6f5183e4e/package.json)
- [HomeView.tsx](https://github.com/nexu-io/open-design/blob/e0d91046dd8dfe7ab9b8f17d0de127c6f5183e4e/apps/web/src/components/HomeView.tsx)
- [HomeHero.tsx](https://github.com/nexu-io/open-design/blob/e0d91046dd8dfe7ab9b8f17d0de127c6f5183e4e/apps/web/src/components/HomeHero.tsx)
- [DesignSystemFlow.tsx](https://github.com/nexu-io/open-design/blob/e0d91046dd8dfe7ab9b8f17d0de127c6f5183e4e/apps/web/src/components/DesignSystemFlow.tsx)
- [design-systems.ts](https://github.com/nexu-io/open-design/blob/e0d91046dd8dfe7ab9b8f17d0de127c6f5183e4e/apps/daemon/src/routes/design-systems.ts)
- [apply.ts](https://github.com/nexu-io/open-design/blob/e0d91046dd8dfe7ab9b8f17d0de127c6f5183e4e/apps/daemon/src/plugins/apply.ts)

## 2. 已验证事实

### OD-001 首页是统一的设计任务发起面

`evidence`

`HomeView.tsx` 和 `HomeHero.tsx` 在同一个提交中组合了自由输入、模板、设计体系、运行模式、附件、工作目录和项目上下文。提交动作会创建或进入一个本地设计项目。

这证明 Open Design 把这些输入统一成一次生成任务的上下文。它不证明 Multica 应照搬主页，因为 Multica 已经有 Project 和 Issue 控制面。

### OD-002 模板是可执行包，不只是预览图片

`evidence`

官方 `kanban-board` 示例至少包含：

- `SKILL.md`；
- `example.html`；
- `open-design.json`。

其 manifest 明确声明版本、许可证、作者、标签、HTML 预览、示例需求、Skill、主设计体系、craft 规则、资产、执行阶段和所需能力。

固定源码：

- [open-design.json](https://github.com/nexu-io/open-design/blob/e0d91046dd8dfe7ab9b8f17d0de127c6f5183e4e/plugins/_official/examples/kanban-board/open-design.json)
- [SKILL.md](https://github.com/nexu-io/open-design/blob/e0d91046dd8dfe7ab9b8f17d0de127c6f5183e4e/plugins/_official/examples/kanban-board/SKILL.md)
- [example.html](https://github.com/nexu-io/open-design/blob/e0d91046dd8dfe7ab9b8f17d0de127c6f5183e4e/plugins/_official/examples/kanban-board/example.html)

### OD-003 模板应用时生成固定快照

`evidence`

`apps/daemon/src/plugins/apply.ts` 将模板 manifest、输入和已解析上下文计算为 source digest，并生成包含插件版本、来源、信任级别、权限、上下文、资产、pipeline 和应用时间的 `AppliedPluginSnapshot`。

它的直接价值是让历史任务保留“当时实际使用了什么”，而不是以后始终读取可能已变化的模板。

### OD-004 设计体系是分层资源包

`evidence`

多个内置设计体系重复使用以下文件层次：

- `DESIGN.md`；
- `tokens.css`；
- `design-tokens.json`；
- `components.html`；
- `components.manifest.json`；
- `USAGE.md`；
- `manifest.json`。

这说明 Open Design 同时保存 Agent 可读规则、机器可读 token、组件清单、使用说明和可视化示例，而不是把设计体系压成单个 JSON 或一段 Prompt。

示例目录：[design-systems/ant](https://github.com/nexu-io/open-design/tree/e0d91046dd8dfe7ab9b8f17d0de127c6f5183e4e/design-systems/ant)

### OD-005 设计体系具有生成、修订、审核和发布生命周期

`evidence`

`DesignSystemFlow.tsx` 与 `apps/daemon/src/routes/design-systems.ts` 包含：

- 异步 generation job；
- 根据反馈生成 revision；
- pending revision 的接受或拒绝；
- draft 和 published 状态切换；
- DESIGN.md 编辑与预览；
- token contract rebuild；
- package 文件、preview 和 showcase 读取；
- 独立设计体系 workspace。

这证明其设计体系不是一次性分析结果，而是可持续修订和发布的资产。

### OD-006 Open Design 的 Project 是本地生成工作空间

`evidence`

在研究版本中，首页提交会结合 working directory、模板、设计体系和提示词创建一个本地 project，随后承载 Agent run 与生成产物。它解决的是一次或一组本地设计生成工作的工作区问题。

Multica 已有长期业务 Project，因此这里只能借鉴“执行工作空间”的能力，不能据此引入第二套同名业务实体。

### OD-007 设计体系创建是多来源取证，不以 Figma 为前置条件

`evidence`

`DesignSystemFlow.tsx` 的创建入口支持同时提供：

- 网站或 GitHub 地址；
- 品牌参考预设；
- 品牌描述和补充备注；
- 已有 `DESIGN.md`，或选择已有设计体系作为参考；
- 本地代码目录；
- `.fig` 文件和 Figma URL；
- 图片、Logo、字体、PDF、HTML 等素材。

创建按钮要求至少存在一种有效来源，但不要求必须上传 Figma。来源会先写入项目内的 `context/source-context.md` 和对应快照，再交给后续生成过程使用。

固定源码：

- [DesignSystemFlow.tsx](https://github.com/nexu-io/open-design/blob/89d6d4ef21baf80f871595abdf6f7de6e941dd44/apps/web/src/components/DesignSystemFlow.tsx)

### OD-008 创建流程同时存在快速确定性产物和 Agent 深化

`evidence`

Open Design 的主创建入口不是一次纯 LLM 调用：

1. `POST /api/brands` 先建立品牌记录、设计体系记录和本地 Project 工作空间；网站或已有 `DESIGN.md` 可先经过程序化提取，尽快形成可查看的初始产物。
2. 其他来源被保存为有界的本地证据快照，包括 GitHub、本地代码、Figma 和上传素材。
3. 创建页随后进入独立 Project 会话，由 Agent 阅读证据、补全 `DESIGN.md`、Tokens、预览、UI Kit、资产和来源说明。
4. Agent 完成前必须运行 package audit；草稿仍需人工审核和发布。

`generation-jobs.ts` 本身主要负责步骤状态、确定性文件创建和修订记录，并没有执行模型推理。真正的语义深化发生在 Project Agent 会话中。不能因为界面显示“AI generation job”，就把所有初始产物都理解成 AI 分析结果。

固定源码：

- [brand-routes.ts](https://github.com/nexu-io/open-design/blob/89d6d4ef21baf80f871595abdf6f7de6e941dd44/apps/daemon/src/brand-routes.ts)
- [generation-jobs.ts](https://github.com/nexu-io/open-design/blob/89d6d4ef21baf80f871595abdf6f7de6e941dd44/apps/daemon/src/design-systems/generation-jobs.ts)

### OD-009 设计体系采用双层事实契约和派生缓存

`evidence`

最小正式包必须包含：

```text
manifest.json
DESIGN.md
tokens.css
```

其中 `DESIGN.md` 是 Agent 可读的设计意图和规则，`tokens.css` 是可直接执行的语义 Token 契约，二者必须保持一致。丰富包还可以包含：

```text
USAGE.md
components.html
components.manifest.json
design-tokens.json
tailwind-v4.css
preview/
source/
assets/
fonts/
```

`components.manifest.json`、`design-tokens.json` 和 `tailwind-v4.css` 被明确规定为派生缓存，不是新的事实源。`components.html` 则用于证明 Tokens 能组合成真实控件和布局，而不只是颜色表。

固定源码：

- [设计体系作者指南](https://github.com/nexu-io/open-design/blob/89d6d4ef21baf80f871595abdf6f7de6e941dd44/docs/design-systems.md)
- [设计体系包说明](https://github.com/nexu-io/open-design/blob/89d6d4ef21baf80f871595abdf6f7de6e941dd44/design-systems/README.md)

### OD-010 来源分析保留证据、置信度和 fallback，而不是只给最终结论

`evidence`

本地代码导入会有界扫描 CSS Variables、Tailwind 配置、字体、Logo/图标素材和代表性组件文件，并写出：

- 扫描文件清单；
- 原始 Token 与来源位置；
- Token 到统一 schema 的映射理由；
- `high`、`medium`、`low`、`fallback`、`alias` 置信度；
- 覆盖率、fallback 数量、评分、等级和是否建议重建；
- 代表性源码片段。

这使审核者能够区分“从来源中提取的事实”和“系统为了补齐契约使用的默认值”。但当前实现仍有明显边界：组件文件名只重点识别 `Button`、`Input`、`Card`、`Nav`、`Navbar`、`Sidebar`；Token 角色映射依赖有限的名称启发式；缺失值会落到默认 Token。结果可能结构完整，但并不一定真实反映来源项目。

固定源码：

- [import.ts](https://github.com/nexu-io/open-design/blob/89d6d4ef21baf80f871595abdf6f7de6e941dd44/apps/daemon/src/design-systems/import.ts)
- [token-contract.ts](https://github.com/nexu-io/open-design/blob/89d6d4ef21baf80f871595abdf6f7de6e941dd44/apps/daemon/src/design-systems/token-contract.ts)

### OD-011 草稿深化发生在可接管工作空间，并通过修订审核进入发布态

`evidence`

设计体系详情页不是一个只读结果页。它包含独立 Agent 会话、文件工作区、预览/UI Kit、生成进度、分章节反馈、pending revision、接受或拒绝、Token contract rebuild，以及 `draft / published` 切换。

普通 revision job 只会把反馈追加成候选 `DESIGN.md` 内容；更完整的跨文件修改由工作空间中的 Agent 完成。接受 revision 后才会把候选正文和明确的文件变更写入正式包。草稿在发布前不能作为其他 Project 的活动设计体系。

固定源码：

- [DesignSystemFlow.tsx](https://github.com/nexu-io/open-design/blob/89d6d4ef21baf80f871595abdf6f7de6e941dd44/apps/web/src/components/DesignSystemFlow.tsx)
- [index.ts](https://github.com/nexu-io/open-design/blob/89d6d4ef21baf80f871595abdf6f7de6e941dd44/apps/daemon/src/design-systems/index.ts)

### OD-012 发布后的设计体系会被拆成不同强度的 Agent 上下文

`evidence`

Open Design 在设计任务 Prompt 中按用途注入设计体系：

- `DESIGN.md` 负责视觉意图、规则和边界；
- `tokens.css` 要求原样使用，作为颜色、字体、间距等绑定契约；
- `components.manifest.json` 优先提供紧凑组件清单；
- manifest 不可用时才回退到完整 `components.html`；
- richer files 只注入轻量索引，Agent 按需读取来源证据、预览或其他文件。

存在活动设计体系时，Agent 不再询问主题色、字体氛围或视觉方向，也不得另造 Token。其优先级是：设计体系决定品牌 Token 和视觉方向，通用 craft 规则补充未覆盖的设计质量要求。

这证明设计体系不是一段附加参考文本，而是下游设计任务的强约束上下文；同时也说明它只能约束 Agent 已读取和可执行的部分，不能自动保证最终视觉质量。

固定源码：

- [system.ts](https://github.com/nexu-io/open-design/blob/89d6d4ef21baf80f871595abdf6f7de6e941dd44/apps/daemon/src/prompts/system.ts)

### OD-013 不同体系共享稳定内核，但允许规则和品牌扩展不同

`evidence`

Open Design 没有要求所有项目先填写同一份完整 UI 规范表单。它采用“稳定内核 + 可变外层”：

- 所有正式包共享 `manifest.json`、`DESIGN.md` 和 `tokens.css`；
- `DESIGN.md` 不要求固定章节名称和顺序，只要求内容足够完整；
- 通用 Token schema 提供跨体系可执行的共同语言；
- 组件、预览、字体、资产、来源片段和领域规则按项目证据增量增加。

Token schema 进一步区分：

- `A1-identity`：背景、前景、品牌色、字体等身份值，必须由具体体系决定；
- `A1-structure`：字号阶梯、容器宽度、页面节奏等结构值，必须由具体体系决定；
- `A2`：成功色、基础间距、圆角、动效等允许采用公共 fallback 的值；
- `B-slot`：体系不需要更丰富层级时可以 alias 到已有 Token；
- `C-extension`：只有特定品牌需要的扩展 Token，不能被通用组件依赖。

这使简单项目可以只建立最小契约，成熟项目再表达更丰富组件和领域模式。但当前源码也明确承认：从 `DESIGN.md` 自动派生 A1 Token 的脚本尚未完成，文档内部矛盾如何裁决也仍是开放问题。

固定源码：

- [token-schema.ts](https://github.com/nexu-io/open-design/blob/89d6d4ef21baf80f871595abdf6f7de6e941dd44/packages/contracts/src/design-systems/token-schema.ts)
- [schema AGENTS.md](https://github.com/nexu-io/open-design/blob/89d6d4ef21baf80f871595abdf6f7de6e941dd44/design-systems/_schema/AGENTS.md)

### OD-014 Project 可以没有设计体系，也可以使用一个主体系和多个弱参考

`evidence`

Open Design 的普通 Project 只保存一个 `designSystemId` 作为主设计体系。有效选择的优先级为：本次请求、模板插件快照、Project 绑定、应用默认；全部不存在时就是 `none`。用户创建的设计体系必须先发布，草稿不能绑定到其他 Project。

新建 Project 的选择器还允许多选：第一个是 primary，其余写入 `inspirationDesignSystemIds`。Prompt 明确要求额外参考只能借鉴色彩倾向、字体个性或组件模式，不能覆盖主体系的 Tokens。部分不适用的任务类型会隐藏设计体系选择器。

因此，Open Design 对不同项目的第一层应对不是“强迫每个项目都有完整规范”，而是：

```text
无设计体系
或
一个已发布主体系 + 零到多个弱参考
```

固定源码：

- [chat-prompt-inputs.ts](https://github.com/nexu-io/open-design/blob/89d6d4ef21baf80f871595abdf6f7de6e941dd44/apps/daemon/src/runtimes/chat-prompt-inputs.ts)
- [NewProjectPanel.tsx](https://github.com/nexu-io/open-design/blob/89d6d4ef21baf80f871595abdf6f7de6e941dd44/apps/web/src/components/NewProjectPanel.tsx)
- [server-services.ts](https://github.com/nexu-io/open-design/blob/89d6d4ef21baf80f871595abdf6f7de6e941dd44/apps/daemon/src/design-systems/server-services.ts)

### OD-015 Catalog 中的“设计体系”实际混合了不同语义

`evidence`

当前内置目录包含 151 个包、20 余个分类。它们既包括：

- Apple、Ant、Shopify 等品牌或产品体系参考；
- Material、shadcn 等组件体系参考；
- Dashboard、Application、Enterprise 等页面或产品形态；
- Minimal、Brutalism、Glassmorphism 等纯视觉风格。

这些资源都使用相同的 manifest 和 Token 契约。这让用户可以快速选择不同方向，但也把“项目事实源”“组件体系”“品牌参考”和“视觉灵感”统一称为 design system。

Open Design 通过 primary 与 inspiration 的强弱关系部分缓解了这个问题，但目录模型本身没有彻底区分这些语义。不能因此推导出 Multica 也应把社区风格资源直接当成项目设计体系事实源。

### OD-016 Open Design 正式支持外部编排器准备的 scratch workspace

`evidence`

官方 `docs/orchestrator-workspaces.md` 明确区分 `user-local` 与 `orchestrator-scratch`。外部编排器可以准备 folder-backed scratch workspace，并写入 `sourceLabel`、`sourceRef`、`baseRevision` 和 `writeback: external` provenance。Open Design 可以读写该 workspace，但源码 checkout、凭据、PR、部署、发布和写回策略都留在 Open Design 之外。

`workspace-contract.ts` 对 metadata 做严格解析，只接受 `kind: scratch` 和 `writeback: external`。`/api/runs/:id/result-package` 返回固定 schema `open-design.run-result-package.v1`，包含 run 终态、workspace storage/provenance、事件日志位置、Project 摘要和 artifact manifests；它描述 Open Design 产生了什么，不负责将结果应用回源仓库。

固定源码：

- [orchestrator-workspaces.md](https://github.com/nexu-io/open-design/blob/276b4d8e970bc143d7ad060181a89a834e3d9caf/docs/orchestrator-workspaces.md)
- [workspace-contract.ts](https://github.com/nexu-io/open-design/blob/276b4d8e970bc143d7ad060181a89a834e3d9caf/apps/daemon/src/workspace-contract.ts)
- [workspaces.ts](https://github.com/nexu-io/open-design/blob/276b4d8e970bc143d7ad060181a89a834e3d9caf/packages/contracts/src/api/workspaces.ts)
- [runs.ts](https://github.com/nexu-io/open-design/blob/276b4d8e970bc143d7ad060181a89a834e3d9caf/apps/daemon/src/routes/runs.ts)

### OD-017 Open Design 的 Agent adapter 可以承接 Multica 的用户选 Agent 语义

`evidence`

官方 `docs/agent-adapters.md` 明确说明 Open Design 不实现模型和工具循环，而是把模型调用、工具使用、上下文、权限、恢复和取消交给现有 code-agent CLI。每个 CLI 由一个 `RuntimeAgentDef` 声明，通用引擎负责检测、启动、流解析和取消。固定 Release 包含 Codex、Claude、OpenCode、Cursor、Devin 及其他 adapter。

因此，Multica 不需要把所选 Agent 降级成一段 Prompt，也不应让 Open Design 自动挑选另一个 Agent。正确的薄适配是把 Multica Agent runtime 显式映射到固定版本的 adapter id，并在派发前完成 binary、认证、模型和能力 preflight。

固定源码：

- [agent-adapters.md](https://github.com/nexu-io/open-design/blob/276b4d8e970bc143d7ad060181a89a834e3d9caf/docs/agent-adapters.md)
- [runtimes/types.ts](https://github.com/nexu-io/open-design/blob/276b4d8e970bc143d7ad060181a89a834e3d9caf/apps/daemon/src/runtimes/types.ts)
- [runtimes/registry.ts](https://github.com/nexu-io/open-design/blob/276b4d8e970bc143d7ad060181a89a834e3d9caf/apps/daemon/src/runtimes/registry.ts)

### OD-018 generation job、Agent 深化与 Package Audit 是不同执行层

`evidence`

固定 Release 的 `generation-jobs.ts` 负责来源收集、确定性 draft 创建、文件登记和 revision 记录，并通过内存 job 状态展示步骤；它本身不执行模型推理。真正的语义深化需要进入 design-system workspace 并通过普通 Agent run 完成。

同一版本的 Package Audit 也不是一个可直接远程调用的单一 daemon endpoint。manifest、Token、派生缓存和 rich package 检查主要位于 `scripts/check-design-system-manifests.ts`、`scripts/check-design-system-package-quality.ts` 等 TypeScript guard；其中 package quality 暴露了可调用的 `evaluateDesignSystemPackageQuality`。因此外部接入可以在固定 worker 内增加窄调用包装，但不能把这些规则重新实现为 Multica Go 校验器。

固定源码：

- [generation-jobs.ts](https://github.com/nexu-io/open-design/blob/276b4d8e970bc143d7ad060181a89a834e3d9caf/apps/daemon/src/design-systems/generation-jobs.ts)
- [design-systems.ts routes](https://github.com/nexu-io/open-design/blob/276b4d8e970bc143d7ad060181a89a834e3d9caf/apps/daemon/src/routes/design-systems.ts)
- [package quality guard](https://github.com/nexu-io/open-design/blob/276b4d8e970bc143d7ad060181a89a834e3d9caf/scripts/check-design-system-package-quality.ts)
- [manifest guard](https://github.com/nexu-io/open-design/blob/276b4d8e970bc143d7ad060181a89a834e3d9caf/scripts/check-design-system-manifests.ts)

### OD-019 v0.16.1 仍以确定性 Brand Engine 和分层设计体系包为基础

`evidence`

固定 Release 的 Brand Engine 明确把 URL 或品牌材料收敛为约 20 个字段的 Seed，再由 `deriveTokens` 确定性生成 default、dark、compact 主题、组件 kit 和 landing/email/poster 等产物；核心推导不依赖 LLM。

同一 Release 的设计体系作者指南仍规定最小包为 `manifest.json`、`DESIGN.md` 和 `tokens.css`，丰富包可以声明 `USAGE.md`、组件 fixture/manifest、Design Tokens JSON、Tailwind 映射、preview、source、assets 和 fonts。组件 manifest、Design Tokens JSON 和 Tailwind 文件是派生缓存，不是与事实源竞争的第二套规则。

因此，Multica 直接采用 Open Design 时必须同时保留“确定性基础产物 + Agent workspace 深化”两层，不能再退回一次 LLM Prompt 生成全部规则。

固定源码：

- [Brand Engine README](https://github.com/nexu-io/open-design/blob/276b4d8e970bc143d7ad060181a89a834e3d9caf/apps/daemon/src/brands/engine/README.md)
- [Brand derive.ts](https://github.com/nexu-io/open-design/blob/276b4d8e970bc143d7ad060181a89a834e3d9caf/apps/daemon/src/brands/engine/derive.ts)
- [设计体系作者指南](https://github.com/nexu-io/open-design/blob/276b4d8e970bc143d7ad060181a89a834e3d9caf/docs/design-systems.md)
- [设计体系包说明](https://github.com/nexu-io/open-design/blob/276b4d8e970bc143d7ad060181a89a834e3d9caf/design-systems/README.md)

### OD-020 稳定 Release 版本以 packaged 应用为准，不能用任一 workspace 包版本代替

`evidence`

在 `open-design-v0.16.1` 对应提交中，`apps/packaged/package.json` 的版本是 `0.16.1`，但仓库根 `package.json`、`apps/daemon/package.json`、`apps/desktop/package.json` 和 `tools/release/package.json` 仍为 `0.15.1`。这不是标签漂移，而是上游 workspace 没有统一同步版本号。

稳定发布脚本会显式读取 `apps/packaged/package.json`，要求 release branch 与该版本一致，并据此生成 `open-design-v${packagedVersion}` 标签。因此 Multica 的引擎身份必须固定为 Release Tag、Git commit 和实际构建制品 SHA-256；单独读取 daemon、desktop、根包或 release 工具的 `package.json` 都不能证明所运行的 Open Design 版本。

固定源码：

- [packaged package.json](https://github.com/nexu-io/open-design/blob/276b4d8e970bc143d7ad060181a89a834e3d9caf/apps/packaged/package.json)
- [稳定发布版本解析](https://github.com/nexu-io/open-design/blob/276b4d8e970bc143d7ad060181a89a834e3d9caf/tools/release/src/metadata/prepare-stable.ts)
- [仓库根 package.json](https://github.com/nexu-io/open-design/blob/276b4d8e970bc143d7ad060181a89a834e3d9caf/package.json)

### OD-021 固定 Release 的 headless daemon 可以独立构建、启动和停止

`validated evidence`

2026-07-31 在 macOS arm64 上对 `open-design-v0.16.1` 的固定提交 `276b4d8e970bc143d7ad060181a89a834e3d9caf` 完成 Phase 0 Task 0.1 验证。源码位于 detached、clean 的独立 sparse checkout，使用 Node `24.15.0` 和 Corepack pnpm `10.33.2`。

依赖安装固定使用仓库 `pnpm-lock.yaml`。除 daemon 依赖闭包外，安装范围必须显式包含 `@open-design/metatool`，因为上游根 `postinstall` 会从 `tools-dev` 通过相对路径调用它，但 `tools-dev/package.json` 没有声明该 workspace 依赖：

```bash
corepack pnpm \
  --filter @open-design/daemon... \
  --filter @open-design/metatool \
  install --frozen-lockfile
corepack pnpm --filter @open-design/daemon build
```

fresh daemon build 退出码为 `0`。随后使用独立 `OD_DATA_DIR` 和端口启动公开 headless 入口：

```bash
OD_DATA_DIR="$TASK_DATA_DIR" \
  node apps/daemon/dist/cli.js daemon start \
  --headless --host 127.0.0.1 --port 17456
```

实测结果：

- `/api/health` 返回 `ok: true`；
- `/api/version` 返回可解析的运行时身份；
- `od daemon status --json` 返回正确的 bind host、port、PID 和隔离数据目录；
- `node-pty` 可以成功导入；
- `od daemon stop` 能优雅结束进程，结束后端口不再监听；
- 启停前后上游源码 `git status` 均为 clean。

本次可复核摘要：

| 项目 | 值 |
| --- | --- |
| Lockfile SHA-256 | `90bbe1375eb716240bbb79215c2a12a601abd977fe88587c6c6c6b4df31f6f23` |
| daemon entry SHA-256 | `3f51c82dd73756b4d2531f4599373b1779657e9d2f756d293dee7f9567486ca7` |
| daemon dist tree SHA-256 | `bc0a56497d56f85f7c807fe742022077bc35b1360de39bf298f07b184db1e7de` |
| daemon dist | 1392 files / 12,670,724 bytes |

`daemon dist tree SHA-256` 按相对路径排序后，将“相对路径、NUL、文件内容、NUL”依次送入 SHA-256；它只是本机 Task 0.1 的构建证据，不是未来 OCI image 或正式 worker 制品的最终摘要。

运行时 `/api/version` 自报 `0.15.1`，而 Release packaged 版本为 `0.16.1`，与 OD-020 的 workspace 版本差异一致。Task 0.1 因此只能证明固定提交上的 headless daemon 可以启动，不能证明设计体系创建、Agent 深化、Package Audit、Preview/UI Kit 或取消链路已经通过；这些仍属于后续 Task 0.2 和 Task 0.3。

### OD-022 CRM scratch 创建任务产出了完整可审计包，但原生恢复链路尚未通过

`validated evidence`

2026-07-31 使用固定 Release `open-design-v0.16.1`、现有 Multica `Local UI Restore Agent` 和 CRM 仓库来源材料完成 Phase 0 Task 0.2。Multica 选择的 Agent 映射为 Open Design `opencode` adapter，执行目录为独立的 folder-backed `orchestrator-scratch` workspace，provenance 明确声明 `writeback: external`，没有直接运行在 CRM 源仓库中。

第一次 Run `c942bf46-fbb9-4186-9910-cd453bf5618a` 的事件流证明 Agent 已读取来源、派发只读分析任务并生成文档、Preview、UI Kit 与本地资产，但父 Run 在 600 秒没有收到新的父级输出后以 `inactivity_timeout` 失败。运行期间嵌套任务仍有实际活动，因此当前 watchdog 不能可靠代表 Agent 是否停止工作；该 Run 被标记为 `resumable: true`，但不能作为成功结果。

随后通过官方 continuation 接口创建 Run `910c40dd-7088-4b00-b388-e06fdfde25a5`，沿用同一 Open Design Conversation 和 scratch workspace，直接修复 Package Audit 问题。该 Run 最终为 `succeeded`、`exitCode: 0`、`resumable: false`、`endedWithUnfinishedWork: false`，并返回 `open-design.run-result-package.v1`。但是 continuation 事件中的原生会话恢复状态为 `resume_skipped`，`guardReason: missing_cursor`；它实际创建了新的 OpenCode Session，而不是恢复第一次 Run 的原生 Session。由此只能证明“同一 Conversation 和 workspace 上可以继续完成任务”，不能证明 adapter 的原生会话恢复已经可靠。

对 Agent 结果之外重新执行固定版本的严格 Package Audit，结果为：

```text
797 files inspected
0 errors
0 warnings
exit 0
```

独立结构检查同时确认：

- 10 个 HTML 文件；
- 66 个 SVG；
- 7 张聚焦 Preview；
- `ui_kits/app/app.js` 语法检查通过；
- `preview/manifest.json` 可以正常解析。

完整 workspace 归档为 `13-complete-workspace.tar.gz`，包含 797 个文件、大小 2,131,715 bytes，SHA-256 为 `e695b6734d490206345d28f193e77b07fe39ccb05badfd556b9ed11c8fa34f7a`。重新解压归档后，按相对路径排序并将“相对路径、NUL、文件内容、NUL”依次送入 SHA-256，归档内容与当前 workspace 均得到摘要 `a232aa54f8533342e8a14ac64aa3493c994b7fe12c7cb1f759d33056e066911f`。

随后使用本机 Chrome 对静态包完成真实渲染验收：

- 桌面视口 `1440 x 1000` 与移动视口 `390 x 844` 均检查 `preview/index.html` 和 `ui_kits/app/index.html`；
- 七张聚焦 Preview 均为非空页面，声明的本地 SVG 与 404/401 图片可以加载，页面样式和字体栈正常渲染；
- CRM UI Kit 桌面端无横向溢出，移动端导航转为抽屉、表格转为纵向记录，没有文本遮挡；
- 搜索可以把 3 条示例记录筛到 1 条，筛选抽屉可打开并通过 Escape 关闭；
- 勾选记录后，批量操作和导出按钮会从禁用变为启用；
- 验收期间 Chrome 控制台没有 warning 或 error；静态服务器中唯一的 404 是 Chrome 对未声明 `/favicon.ico` 的隐式请求，包内实际声明的 CSS、JS、SVG 和图片资源均返回 200 或 304。

CRM 源仓库在任务前后保持同一 HEAD `142763d8ca83e3c7bd3cacfb9aa378648e7936c3`。工作区 diff 摘要保持 `e980c85a19c368e8bed53dd0c52fcbbca1e4656e97c28c887352e263f65a61e8`，staged diff 仍为空摘要 `e3b0c44298fc1c149afbf4c8996fb92427ae41e4649b934ca495991b7852b855`，说明 spike 没有修改用户源仓库。

Task 0.2 的结论是：真实 Agent 创建、完整设计体系包、result package、独立 audit、Preview/UI Kit 渲染和源仓库零修改均已取得证据，但任务需要 continuation 才完成，且暴露了父级 inactivity watchdog 和 `missing_cursor` 两项恢复风险。它不代表 Phase 0 已通过；Task 0.3 仍需验证一次真实调整、取消、Agent 失败、audit 失败、preview 失败与 scratch 回收。

### OD-023 CRM scratch 的一次真实调整通过独立审计和 Chrome 验收，但 Task 0.3 尚未完成

`validated evidence`

2026-07-31 在 Task 0.2 的同一 `orchestrator-scratch` workspace 上执行 Phase 0 Task 0.3 的第一个有界子阶段，只验证一次真实调整，不同时混入取消、失败矩阵和 scratch 回收。调整 Run 为 `0f2f8fb4-b4f2-4f18-a025-5a2e14c6741b`，沿用 Project `2c568ec3-88fe-41c1-ab37-dc19b4e7fd7e` 和 Conversation `e9ec5293-c9df-48fc-83c1-8dbb6b25cb94`，使用 `opencode` adapter。目标是补齐移动导航的关闭与焦点恢复，以及客户搜索清空后重新查询的恢复行为，同时保持桌面布局、Tokens、品牌和来源资产不变。

SSE 的第一个 diagnostic 再次稳定复现 `native_session_recovery.state: resume_skipped` 和 `guardReason: missing_cursor`。本次实际创建了新的 OpenCode Session；终态记录为 `captured_not_resumed`。Run 最终为 `succeeded`、`exitCode: 0`、`resumable: false`、`endedWithUnfinishedWork: false`，并可取得 `open-design.run-result-package.v1`，但这些状态只作为执行证据，不作为调整成功结论。

Agent 先扩展 workspace 内的 `tools/check-package.py`，新增移动导航和搜索恢复契约，首次执行得到 5 条预期失败；完成实现后检查转为通过。将调整前归档与当前 workspace 做逐文件比较，797 个文件的总数保持不变，只有以下 7 个文件发生变化：

- `DESIGN.md`；
- `preview/applied-ui-surfaces.html`；
- `tools/check-package.py`；
- `ui_kits/app/README.md`；
- `ui_kits/app/app.css`；
- `ui_kits/app/app.js`；
- `ui_kits/app/index.html`。

调整前 workspace 内容摘要为 `a232aa54f8533342e8a14ac64aa3493c994b7fe12c7cb1f759d33056e066911f`，调整后为 `c82bac51f475697674b7255bc9eddd4a6b04cde356ffc5618ab5c1e11b9ab785`。这证明本次确有产物变化，但摘要变化本身不证明行为正确。

在 Agent 结束后独立执行固定版本检查，结果为：

```text
PACKAGE CHECK OK: 10 HTML files, 66 SVGs, 7 previews
ui_kits/app/app.js syntax: exit 0
preview/manifest.json parse: exit 0
Open Design strict audit: 797 files, 0 errors, 0 warnings, exit 0
```

随后使用用户本机 Chrome 完成真实视觉和交互验收：

- 桌面视口 `1920 x 934` 下，侧栏宽度和内容左边距均为 `170px`，移动关闭按钮与遮罩不可见，3 条记录正常展示；
- 移动视口 `390 x 844` 下，导航打开后侧栏位于 `x: 0`、宽 `170px`，抽屉内关闭按钮和遮罩均可见，焦点进入关闭按钮；
- 分别通过抽屉内按钮、遮罩和 Escape 关闭后，`nav-open` 均被移除、`aria-expanded` 回到 `false`，焦点均返回 `menuToggle`；
- 搜索“周女士”后只显示 1 条；显式清空关键词时保留当前结果，重新点击“查询”后恢复 3 条记录和 `3 条`计数；
- Chrome 控制台没有 warning 或 error；声明的 CSS、JS 和 6 个本地 SVG 均加载完成，静态服务器只记录 200 或 304，没有失败请求；
- 桌面和移动截图分别保存在本次临时证据目录的 `23-task03-desktop.png`、`24-task03-mobile-nav-open.png` 和 `25-task03-mobile-search-restored.png`。

CRM 源仓库仍保持 HEAD `142763d8ca83e3c7bd3cacfb9aa378648e7936c3`，工作区 diff 摘要仍为 `e980c85a19c368e8bed53dd0c52fcbbca1e4656e97c28c887352e263f65a61e8`，staged diff 摘要仍为 `e3b0c44298fc1c149afbf4c8996fb92427ae41e4649b934ca495991b7852b855`。本次调整没有写回 CRM 源仓库。

本子阶段只能证明：同一 scratch workspace 可以由真实 Agent 做一次窄调整，并经过独立 Package Audit、真实 Chrome 交互和源仓库零修改复核。`missing_cursor` 风险仍未解决，Task 0.3 的取消、Agent 失败、audit 失败、preview 失败和 scratch 回收尚未验证，因此 Phase 0 仍不能宣布通过。

### OD-024 真实 Run 可取消且外部编排器可回收 scratch，但失败矩阵尚未验证

`validated evidence`

2026-07-31 继续使用固定 Release `open-design-v0.16.1` 的提交 `276b4d8e970bc143d7ad060181a89a834e3d9caf`，执行 Phase 0 Task 0.3 的第二个有界子阶段，只验证真实取消和 scratch 回收。隔离 worker 使用独立数据目录和 `127.0.0.1:17457`，没有触碰 Multica 的 `3031/8080` 服务。将 OD-023 已验收 workspace 复制为一次性目录后，通过 `/api/import/folder` 导入 Project `37bf061e-d426-4a43-b3f4-7cabdc783a30`；workspace 明确记录 `kind: orchestrator-scratch`、`writeback: external` 和 CRM 来源 revision。

取消 Run 为 `d4a6acb9-3116-4540-8792-758260b7dbf9`，使用 `opencode` adapter。取消前的 SSE 和进程证据同时确认：

- 事件流产生真实 `start`，cwd 指向一次性 scratch；
- OpenCode Session `ses_0487a1a9dffexu4v6Y53JpW9Z0` 进入 `running`；
- Agent 实际调用 `read` 并读取 scratch 内的 `README.md`；
- Run 状态为 `running`，child PID 与 process group ID 均为 `30489`，对应进程为 `opencode run --format json`。

调用 `POST /api/runs/:id/cancel` 后，取消响应、最终状态、SSE 和持久化事件一致显示：

- `status: canceled`、`cancelRequested: true`、`childExited: true`；
- 退出信号为 `SIGTERM`；
- `runtime_close.rpc_close_reason` 为 `cancel_requested`；
- Run 的 `artifactCount` 为 `0`；
- PID `30489` 及 PGID `30489` 下的进程均已不存在；
- 取消后仍可读取 `open-design.run-result-package.v1`，其中保留取消终态和完整 scratch provenance。

导入时项目被上游识别为 prototype，Run 自动应用 `example-web-prototype` 默认场景插件，因此 scratch 在原有 797 个文件之外增加了 `.od-skills/web-prototype-0317aca362/` 下的 6 个插件文件；逐文件比较确认原有 797 个文件内容均未变化。这是上游默认插件注入行为，不是 Agent 对设计体系文件的修改，后续正式薄适配层需要显式控制是否允许默认场景插件。

删除前已将 803 个文件的 scratch 归档为 `12-canceled-scratch.tar.gz`，大小 2,198,984 bytes，SHA-256 为 `b135a9e8f83d4bf75300afbafa77f4e7ca6b47d21b64387026588add61a10faf`；当前 scratch 内容摘要为 `2f6f1b9b9d045aa6ec65d8f94159253364c2b09c6ba80cef9a9a1099a52f0dbb`。重新解压归档后得到相同内容摘要，再由外部编排步骤删除原 scratch 和解压校验副本，两处路径均确认不存在。最终证据包大小为 2,225,937 bytes，SHA-256 为 `1e427fe06df65615e72e92b194722c5105cdcc1d1189230df507b15e2c1a0e34`，位于 `/private/tmp/multica-od-phase0-task03-cancel-20260731/task03-cancel-evidence-final.tar.gz`。隔离 worker 随后停止，`17457` 不再监听。

CRM 源仓库在本子阶段后仍保持 HEAD `142763d8ca83e3c7bd3cacfb9aa378648e7936c3`，工作区 diff 摘要仍为 `e980c85a19c368e8bed53dd0c52fcbbca1e4656e97c28c887352e263f65a61e8`，staged diff 摘要仍为 `e3b0c44298fc1c149afbf4c8996fb92427ae41e4649b934ca495991b7852b855`。

本子阶段证明了：真实 Agent Run 可以被区分为用户取消终态，Agent 子进程会终止，结果包仍可复盘，并且外部编排器可以在归档后完整回收 scratch。它不解决 OD-023 的 `missing_cursor`，也没有验证 Agent 失败、audit 失败和 preview 失败三个不同失败终态，因此 Task 0.3 与 Phase 0 仍不能宣布通过。

### OD-025 真实 Agent 失败未形成可用设计体系，但 Run API 不能跨 worker 重启复盘

`validated evidence`

2026-07-31 执行 Phase 0 Task 0.3 的第三个有界子阶段，只验证 Agent 失败与坏结果隔离。继续使用固定 Release `open-design-v0.16.1` 的提交 `276b4d8e970bc143d7ad060181a89a834e3d9caf`，在独立数据目录和 `127.0.0.1:17458` 启动 worker，并将 OD-023 已验收 workspace 复制为 797 文件的一次性 `orchestrator-scratch`。导入 Project 为 `1fe50343-0dd2-45e1-8157-87e48b9715a9`，provenance 明确声明 `writeback: external`。

本次采用可重复且不更改用户 Agent 配置的受控失败：真实 `opencode` adapter 使用不存在的模型 `opencode/definitely-not-a-model`。Run `e1d8ab40-cf81-4799-bde2-edfce32629da` 的 SSE 证明：

- OpenCode 子进程真实启动，cwd 指向一次性 scratch；
- 第一次执行返回 `AGENT_EXECUTION_FAILED`、`runtime_close.rpc_close_reason: stream_error` 和退出码 `1`；
- 上游按 `same_run_transient` 自动重试一次，第二次仍以相同错误和退出码结束；
- 最终终态为 `failed`，`cancelRequested: false`、`childExited: true`、`exitCode: 1`；
- 最终分类为 `failureCategory: upstream_unavailable`、`failureDetail: upstream_5xx`，Run `artifactCount` 为 `0`；
- 最后记录的 PID/PGID `42835` 已不存在。

这与 OD-024 的 `canceled / user_cancel` 已形成不同终态，但精细分类仍有缺口：OpenCode 对不存在的模型只返回通用 `Unexpected server error`，因此 Open Design 无法将真实原因识别为 `model_unavailable`，只能归为 `upstream_5xx`。正式 Multica 适配层必须在派发前校验所选模型是否来自该 Agent 的可用模型清单，并保留请求模型与原始错误，不能只依赖该粗分类指导用户重试。

坏结果隔离证据如下：

- 失败前可读取的 `open-design.run-result-package.v1` 明确记录 `status: failed`、`errorCode: AGENT_EXECUTION_FAILED` 和 `exitCode: 1`；
- 导入项目在失败前后均保持 `designSystemId: null`；
- worker 的设计体系存储目录与运行前副本逐文件一致，没有新增或修改设计体系；
- scratch 原有 797 个文件逐字节不变，排除默认插件目录后的内容摘要与来源 workspace 同为 `f3d2ce73146e6015a0bc05fc64eacf878bee0804c8e5f4c443e915c0910c2e23`；
- 与 OD-024 相同，上游自动应用 `example-web-prototype` 并注入 6 个 `.od-skills` 文件，使失败 scratch 共 803 个文件；这些文件未被提升为设计体系，最终随 scratch 一并回收。

本次还验证了 worker 生命周期边界。终态时磁盘仍存在 `state.json` 和 `events.jsonl`，但重启同一 worker 后，`GET /api/runs` 返回空列表，同一 Run 的状态、events 和 result-package 接口均返回 `404 run not found`；磁盘 `state.json` 也没有保存 SSE 最终给出的 `failureCategory/failureDetail`。因此 Multica supervisor 必须在 worker 退出前消费并持久化 SSE、终态、分类和 result package，不能把 Open Design Run API 当作可跨重启恢复的任务数据库。原始 events 文件只能作为诊断证据，不能替代 Multica 的持久任务记录。

删除前已将失败 scratch 归档为 `22-failed-scratch.tar.gz`，大小 2,199,392 bytes，SHA-256 为 `850cdac747eceb2ebbc3ca8dd8d6860ef7023c819f317af0c59ed23e1647c86e`；803 文件的内容摘要为 `2f6f1b9b9d045aa6ec65d8f94159253364c2b09c6ba80cef9a9a1099a52f0dbb`，重新解压后摘要一致。随后外部编排步骤删除失败 scratch 和解压校验副本，两处路径均不存在。最终证据包大小为 2,222,985 bytes，SHA-256 为 `637c569269c9634e00834a4a94afceecc46f83a88a2a32287ba8f1f8f56b8f9d`，位于 `/private/tmp/multica-od-phase0-task03-agent-failure-20260731/task03-agent-failure-evidence-final.tar.gz`；隔离 worker 已停止，`17458` 不再监听。

CRM 源仓库仍保持 HEAD `142763d8ca83e3c7bd3cacfb9aa378648e7936c3`，工作区 diff 摘要仍为 `e980c85a19c368e8bed53dd0c52fcbbca1e4656e97c28c887352e263f65a61e8`，staged diff 摘要仍为 `e3b0c44298fc1c149afbf4c8996fb92427ae41e4649b934ca495991b7852b855`。

本子阶段证明了 worker 边界上的 Agent 失败、零产物、未绑定设计体系和失败 scratch 回收，但尚未实现 Multica supervisor 的持久化与 draft gate，因此不能外推为业务链路已经不会创建坏草稿。Task 0.3 仍需分别验证 audit 失败和 preview 失败；`missing_cursor` 与 Run API 重启不可回放也继续作为 Phase 0 风险保留。

### OD-026 Package Audit 可拒绝坏候选，但 supervisor 必须映射独立终态

`validated evidence`

2026-07-31 执行 Phase 0 Task 0.3 的第四个有界子阶段，只验证 Package Audit 失败和坏包隔离。继续使用固定 Release `open-design-v0.16.1` 的提交 `276b4d8e970bc143d7ad060181a89a834e3d9caf`，在独立数据目录和 `127.0.0.1:17459` 启动 worker；将 OD-023 已验收的 797 文件 workspace 复制为一次性 `orchestrator-scratch`，导入 Project `607c2046-17ea-4242-b887-85c7a235b944`，且始终保持 `designSystemId: null`。

失败注入前先执行正向基线。项目级 `GET /api/projects/:id/design-system-package-audit` 与固定版本 CLI `design-system-package-audit --fail-on-warnings` 均得到：

```text
797 files inspected
0 errors
0 warnings
ok: true
CLI exit: 0
```

随后只从一次性候选中移除必需事实文件 `DESIGN.md`，文件数降为 796，其他文件保持不变。再次执行同一套 Audit 后：

- 项目级 API 仍返回 HTTP `200`，但正文为 `audit.ok: false`；
- 严格 CLI 返回 exit `1`；
- API 与 CLI 均返回同一条结构化错误：`severity: error`、`code: missing_required_file`、`path: DESIGN.md`；
- 错误消息明确指出 `DESIGN.md` 是设计体系包的 canonical system rules；
- warnings 仍为空，排除了由 warning gate 造成的歧义。

这说明固定版上游 Audit 可以稳定区分合法包和缺失事实源的坏包，但 Audit 是 Run 结束后的独立验证步骤，不会自动改变 Open Design Run 终态；其 HTTP 状态也不能代表通过。Multica supervisor 必须读取 `audit.ok`、errors 和严格 CLI/同源 endpoint 结果，将失败明确映射为 `audit_failed`，并在该状态下禁止写入 draft。只检查 HTTP `2xx`、Agent Run `succeeded` 或 result package 存在都会错误放行坏包。

坏包隔离证据如下：

- 候选项目在失败 Audit 后仍为 `designSystemId: null`；
- worker 的设计体系存储目录与运行前副本逐文件一致；
- 有效来源 workspace 在失败注入后重新执行严格 Audit，仍为 797 files、0 errors、0 warnings、exit 0；
- 候选与有效来源的唯一文件差异是缺少 `DESIGN.md`；
- 坏候选内容摘要为 `048db69f2d9703e886dbceeb293c4e3ef8648fbbb2cbb4200f2dce0193f13cfb`，有效来源摘要仍为 `f3d2ce73146e6015a0bc05fc64eacf878bee0804c8e5f4c443e915c0910c2e23`。

删除前已将 796 文件的坏候选归档为 `13-failed-audit-candidate.tar.gz`，大小 2,177,030 bytes，SHA-256 为 `eb5fc8a8b3974698c8646e8cf68380752471474d90217b0b2ccb441957222fb7`；重新解压后内容摘要与坏候选一致。随后外部编排步骤删除候选 scratch 和解压校验副本，两处路径均不存在。最终证据包大小为 2,179,565 bytes，SHA-256 为 `95aeddb56a563f48915aaf7fe027cff543da9f6cee045515db2acdd84cad9c62`，位于 `/private/tmp/multica-od-phase0-task03-audit-failure-20260731/task03-audit-failure-evidence-final.tar.gz`；隔离 worker 已停止，`17459` 不再监听。

CRM 源仓库仍保持 HEAD `142763d8ca83e3c7bd3cacfb9aa378648e7936c3`，工作区 diff 摘要仍为 `e980c85a19c368e8bed53dd0c52fcbbca1e4656e97c28c887352e263f65a61e8`，staged diff 摘要仍为 `e3b0c44298fc1c149afbf4c8996fb92427ae41e4649b934ca495991b7852b855`。

本子阶段证明了上游 Audit 失败信号、坏候选隔离和回收，但 Multica supervisor 的 `audit_failed` 持久终态与 draft gate 尚未实现，因此还不能把该信号等同于业务闭环已经通过。Task 0.3 只剩 Preview 失败验证；`missing_cursor`、Run API 重启不可回放和 supervisor 持久化仍是 Phase 0 风险。

### OD-027 Package Audit 无法发现不可见 Preview，必须增加独立渲染门禁

`validated evidence`

2026-08-03 执行 Phase 0 Task 0.3 的第五个有界子阶段，只验证 Preview 失败和坏预览隔离。继续使用固定 Release `open-design-v0.16.1` 的提交 `276b4d8e970bc143d7ad060181a89a834e3d9caf`，在独立数据目录和 `127.0.0.1:17460` 启动 worker。将已验收的 797 文件 workspace 复制为一次性 `orchestrator-scratch`，导入 Project `da230e1a-9e5c-4229-9eb5-ba53e1becac8`；来源与候选在注入前的内容摘要均为 `f3d2ce73146e6015a0bc05fc64eacf878bee0804c8e5f4c443e915c0910c2e23`，项目始终保持 `designSystemId: null`。

失败注入前先建立正向基线。项目级 Package Audit 与固定版本 CLI `design-system-package-audit --fail-on-warnings` 均得到 797 files、0 errors、0 warnings、exit 0。本机 Chrome 通过 Open Design `preview-url` 生成的 scope 加载 `preview/colors-primary.html`，在 `1440 x 1000` 视口下得到：

- HTTP 文档正常加载，标题为 `CRM 颜色`；
- DOM 中有 41 个元素，41 个满足尺寸、display、visibility 和 opacity 的可渲染条件；
- 页面可见文本长度为 317，Chrome 控制台没有 warning 或 error；
- 全页截图为 `1425 x 1064`，采样像素中约 55.29% 不是近白色；
- 截图熵为 `3.4188`，RGB 通道标准差约为 `76.88 / 75.65 / 73.18`，证明截图包含真实视觉信息。

随后只在候选的 `preview/colors-primary.html` 现有 `<style>` 内注入一次 `html,body{visibility:hidden!important}`。文件从 2,613 bytes 变为 2,651 bytes；结构化比较确认候选严格等于“原文件加该唯一规则”，没有其他改动。注入后项目级 Audit 与严格 CLI 仍然全部通过，继续返回 797 files、0 errors、0 warnings、exit 0。这证明 Package Audit 与 Preview 可见性是两个独立门禁，不能用前者替代后者。

使用新的 Preview scope 重新加载候选后，HTTP 仍返回 `200`，响应正文 SHA-256 与候选文件同为 `da966d1884d21f67c6052c3ea5946b4e5ba1e469bc67f8d26815deb4bd1bd6d3`。浏览器同时确认：

- `document.readyState` 为 `complete`，DOM 仍有 41 个元素和 297 字符的源文本；
- `html` 与 `body` 的计算后 `visibility` 均为 `hidden`；
- 可见文本长度和满足可渲染条件的元素数均为 0；
- 页面尺寸仍为 `1425 x 1064`，说明不是空文件或布局坍塌；
- 全页截图只剩统一浅灰背景，截图熵、锐度和三个 RGB 通道标准差均为 0；
- 控制台仍没有 warning 或 error。

本次还暴露了像素判定的一个边界：统一浅灰背景的“非白像素比例”为 100%，因此只统计非白像素会把坏 Preview 错判为有内容。实验 verifier 同时检查 DOM 计算可见性、可渲染元素数量、截图熵和通道方差，最终稳定返回 `passed: false`、`failureCategory: preview_failed`、`failureDetail: rendered_content_not_visible`。该 verifier 目前只是 spike 证据，尚未接入 Multica supervisor。

坏预览隔离证据如下：

- 候选 Project 在 Preview 失败后仍为 `designSystemId: null`；
- worker 的设计体系存储目录与实验前副本逐文件一致；
- 有效来源 workspace 重新执行严格 Audit 后仍为 797 files、0 errors、0 warnings、exit 0；
- 候选与有效来源的唯一差异是 `preview/colors-primary.html`；
- 坏候选仍为 797 个文件，内容摘要为 `aa909e47904b5646437de71d2bce2d414713d15f31119765b7d83f16b8900cd4`。

删除前已将失败候选归档为 `23-failed-preview-candidate.tar.gz`，SHA-256 为 `da9ae388b1fc8d5a256b76c58d81a7ef6d37db03ad2a35e6bacfc8b4a59313cb`；重新解压后仍为 797 个文件且内容摘要一致。随后通过 daemon shutdown API 停止 worker，并删除候选 scratch 和解压校验副本；`17460` 最终不再监听。最终证据包大小为 5,205,303 bytes，SHA-256 为 `c5f39889eb061ef82adc04a514b9afb6f58c092d54a59ca3f457331e69a26ec2`，位于 `/private/tmp/multica-od-phase0-task03-preview-failure-20260731/task03-preview-failure-evidence-final.tar.gz`。

CRM 源仓库仍保持 HEAD `142763d8ca83e3c7bd3cacfb9aa378648e7936c3`，工作区 diff 摘要仍为 `e980c85a19c368e8bed53dd0c52fcbbca1e4656e97c28c887352e263f65a61e8`，staged diff 摘要仍为 `e3b0c44298fc1c149afbf4c8996fb92427ae41e4649b934ca495991b7852b855`。

本子阶段证明了 Preview 失败可以在 Audit 通过、HTTP 正常、DOM 存在且控制台无错误的情况下独立发生，也证明外部编排器可以隔离、归档并回收该候选。至此 Task 0.3 要求的真实调整、取消、Agent 失败、Audit 失败、Preview 失败和 scratch 回收均已有实验依据。但 Multica supervisor 的执行证据持久化、`audit_failed` / `preview_failed` 终态映射和 draft gate 尚未实现，`missing_cursor` 也未解决，因此 Phase 0 仍不能宣布通过。

### OD-028 Multica 已建立 Open Design Supervisor 持久执行骨架，但仍停在 `run_succeeded`

`validated implementation evidence`

2026-08-03 在 Multica Server 和本地 daemon 中完成 Phase 0 production gate 的第一批最小实现。任务创建时会为每次 Open Design 任务持久化独立 `open_design_run`，固定记录 Release、commit、lockfile digest、dist digest、用户所选 Agent 快照、adapter、model、输入快照和 `orchestrator-scratch` provenance。daemon 只接受 loopback worker URL，并在执行前核对固定 worker 制品、真实 adapter inventory、binary、认证和 model；preflight 报告成功写入数据库并将状态转换为 `ready` 后，才创建 scratch、导入 worker workspace 并开始 Run。

Supervisor 现在会在 worker 退出前通过 Multica daemon callback 持久化 Run ID、按序 SSE events 和 `open-design.run-result-package.v1`。事件写入具有按 ID 去重和顺序约束，重复 callback 只接受与既有证据一致的幂等请求。Agent 失败和取消保持独立终态；尚未启动 worker Run 的 scratch 准备失败允许从 `ready` 转为 `agent_failed`。Run 成功只转换为 `run_succeeded`，不会调用旧三文件 `CompleteTask`，也不会创建或批准 design draft。

daemon 与 fake Multica Server、fake Open Design Worker 的集成回归实际验证了以下顺序：

```text
worker probe
-> persisted ready
-> scratch prepare
-> worker workspace import
-> worker run
-> SSE persistence
-> result package persistence
-> run_succeeded
```

同一测试同时确认普通 task runner、旧 `/complete` 和旧 `/fail` 均未在成功路径触发，worker prompt 读取 `.agent_context/project_design_system/task.json`，且没有复用旧 `MULTICA_OUTPUT_DIR` 三文件收集协议。验证结果如下：

- `go test ./internal/opendesign -count=1`：24 passed；
- `go test ./internal/daemon -count=1`：419 passed；
- 聚焦 daemon 接线测试：2 passed；
- SQLC 生成使用固定本机二进制 `/Users/fengyujie/go/bin/sqlc generate` 成功；
- scoped GitNexus `detect-changes`：5 files、12 indexed symbols、0 affected processes、LOW；
- `git diff --check`：通过。

该证据只证明持久 Supervisor 骨架已经进入正式代码，不代表 Phase 0 转为 Go。当前 worker 仍需由外部提前启动并通过 daemon 配置提供 URL、token 和固定制品根目录；archive、artifact index、content digest、Audit、Preview verifier、draft gate、`missing_cursor`、异常退出恢复和 scratch 幂等回收尚未闭合。坏包仍不得进入 design draft，设计中心主流程也不能切换到该链路。

### OD-029 Supervisor 已收集完整 archive 索引并持久化脱敏结果与 content digest

`validated implementation evidence`

2026-08-03 完成 Phase 0 production gate 的第二个有界实现批次。固定版 Open Design Run 成功后，Supervisor 现在依次读取上游原生 `GET /api/projects/:id/export/manifest` 和 `GET /api/projects/:id/archive`，不再把 result package 中的 `artifacts` 数组误当成完整文件清单。collector 使用上游 manifest 提供的角色和 MIME，解析完整 ZIP，并为每个相对路径生成 `path`、`role`、`mime`、`size` 和文件 SHA-256；索引按路径排序，`content_digest` 由排序后的 `path + size + file hash` 稳定生成，与 ZIP entry 顺序和压缩元数据无关。

上传 Server 前会删除 result package 的 `workspace.storage.baseDir` 和 `events.logPath`。上游 manifest 中的 `localPath` 只用于本地解析且不会进入 callback。Server 会再次拒绝带本机路径的 result package、非法或乱序 artifact index，以及与索引不一致的 content digest。`MarkOpenDesignRunSucceeded` 在同一状态转换中原子写入脱敏 `result_package`、`artifact_index` 和 `content_digest`，重复 callback 只有三项证据完全一致时才视为幂等成功。

collector、manifest、archive 或结果持久化失败都会转为 `agent_failed`，不会创建 draft。特别修复了一个既有生命周期缺口：结果 callback 失败后 Supervisor 不再直接返回并让数据库停留在 `running`，而是尝试写入带 `open_design_result_callback_failed` 分类的失败终态。

验证结果如下：

- `go test ./internal/opendesign -count=1`：29 passed；
- `go test ./internal/daemon -count=1`：419 passed；
- `go test ./internal/daemon -run OpenDesign -count=1`：3 passed；
- 使用当前 `.env` 恢复库执行 `TestCreateProjectDesignSystemPersistsPinnedOpenDesignRun`：通过，实际断言三项证据已写入 `open_design_run`；
- collector 回归覆盖本地路径脱敏、manifest 文件缺失拒绝、ZIP 顺序无关 digest 和 Server callback 二次校验；
- SQLC 使用本机现有 `v1.31.1` 生成成功。

本批次尚未上传 archive 到对象存储，`archive_object_key` 继续为空；archive bytes 只在 daemon 当前收集过程中存在。因此 Phase 0 仍为 No-Go。下一批仍需闭合 archive 对象存储、worker 异常退出恢复和 scratch 幂等回收，之后才能进入 Audit、Preview 和 draft 串行门禁。

### OD-030 Supervisor 已持久化完整 archive object，并把它设为 result 前置门禁

`validated implementation evidence`

2026-08-03 完成 Phase 0 production gate 的第三个有界实现批次。Supervisor 在 collector 生成 artifact index 与稳定 content digest 后，必须先通过专用 daemon endpoint 上传原始 ZIP，再提交 result callback。该 endpoint 只接受 `application/zip`，压缩包上限统一为 100 MB；Server 会重新解析 ZIP，拒绝非法相对路径、软链接、重复文件、单文件或总展开大小越界，并重新计算 `path + size + file hash` 摘要核对 daemon 提交的 content digest，不能仅相信请求头。

Server 使用现有统一 `storage.Storage`，按以下确定性 key 保存包：

```text
workspaces/{workspace_id}/design-systems/{design_system_id}/open-design-runs/{task_id}/{content_digest_hex}.zip
```

上传成功后，`RecordOpenDesignRunArchive` 会立即把 `archive_object_key` 和 `content_digest` 写入当前 `running` Run。相同 run、digest 和 key 的重复请求返回同一结果；daemon 使用两次有界重试和单次两分钟上传 timeout，因此响应丢失后可以安全重放。`MarkOpenDesignRunSucceeded` 现在要求 result callback 携带的 key 与已持久化 key、digest 完全一致；未先上传 archive、引用不同 key 或 digest 的 result 均不能进入 `run_succeeded`。

archive 校验、对象存储上传或 archive callback 最终失败都会由 Supervisor 写入 `agent_failed`，错误分类为 `open_design_archive_upload_failed`，不会调用旧 completion，也不会创建 draft。上传成功但后续 Audit 或 Preview 失败时 archive 继续由 Run 引用，作为失败复盘证据，不在当前链路立即删除。

验证结果如下：

- `go test ./internal/opendesign -count=1`：31 passed，覆盖 archive digest 二次校验、上传先于 result，以及上传失败转 `agent_failed`；
- `go test ./internal/daemon -count=1`：419 passed，daemon 集成链路已出现 `/open-design/archive` 且仍不调用旧 completion；
- 使用当前 `.env` 恢复库执行 `TestCreateProjectDesignSystemPersistsPinnedOpenDesignRun`：通过，实际验证未上传 archive 的 result 被拒绝、相同 archive 上传两次得到同一 key、数据库写入 key/digest，随后 result 才进入 `run_succeeded`；
- 路由注册聚焦测试：通过；
- SQLC 使用本机已有 `/Users/fengyujie/go/bin/sqlc generate` 成功。

handler 全包共 1376 个测试通过，仍有两个与本批无关的 dashboard rollup 基线失败：`TestRollupTaskUsageHourlyIdempotentAndWatermark` 与 `TestRollupTaskUsageHourlyCapsWindowAtOneDay` 均因本地 rollup state 无记录失败；archive 聚焦数据库测试独立通过。

本批只闭合 archive 对象存储与 result 前置关系，没有验证真实固定 worker 加实际云端对象存储的完整矩阵，也没有实现外部 worker 异常恢复、scratch 幂等回收、Audit、Preview 或 draft gate。因此 Phase 0 继续保持 No-Go。

### OD-031 Open Design orphan task 已进入持久失败终态，scratch 可由现有 GC 幂等回收

`validated implementation evidence`

2026-08-03 完成 Phase 0 production gate 的第四个有界实现批次。daemon 启动或重新注册 runtime 时仍复用现有 `recover-orphans` 入口；本批把 Open Design Run 纳入同一数据库事务：当旧 daemon 遗留的 `dispatched` / `running` task 被标记为 `failed + runtime_recovery` 时，对应 `open_design_run` 会同时从 `preflight_pending`、`ready` 或 `running` 转为 `agent_failed`，写入明确的失败原因与 `finished_at`。重复恢复不再更新任何行，已经进入 `run_succeeded` 或其他终态的持久证据也不会被降级覆盖。

scratch 生命周期同时前移了 GC 身份写入。`execenv.Prepare` 创建独立环境后，daemon 会在注入运行时配置和调用 worker import 之前写入 `.gc_meta.json`，记录 workspace 与 task 身份；metadata 写入失败时，尚未交给 worker 的 scratch 会立即清理并终止准备。任务执行期间仍由 `activeEnvRoots` 阻止 GC 误删；daemon 异常退出并完成 server-side orphan recovery 后，既有 task GC endpoint 会返回 `failed`，通用 quick-create GC 分支即可安全删除该目录。重复扫描或重复删除保持无副作用，不需要 Open Design 专用清理器。

验证结果如下：

- `go test ./internal/daemon -count=1`：421 passed，包含 scratch metadata 前置写入、活跃目录保护和恢复后 `failed` task 可回收；
- `go test ./internal/opendesign -count=1`：31 passed；
- `go test ./internal/service -count=1`：137 passed；
- 使用当前 `.env` 恢复库执行 Open Design handler 聚焦测试：通过，覆盖 worker 启动前与 Run 执行中两种 orphan，并验证第二次恢复返回零条；
- `/Users/fengyujie/go/bin/sqlc generate`：成功；
- `git diff --check`：通过。

本批没有实现 worker 制品的自动拉起、进程托管或 `missing_cursor` 原生续跑，也没有用真实固定 worker 重跑一次进程 kill/restart 的完整验收矩阵。Audit、Preview 和 draft gate 仍未接入，成功路径仍停在 `run_succeeded`，不会创建 Multica draft。因此 Phase 0 继续保持 No-Go。

### OD-032 Package Audit 已成为持久门禁，失败报告与 `audit_failed` 原子落盘

`validated implementation evidence`

2026-08-03 完成 Phase 0 production gate 的第五个有界实现批次。Supervisor 在 archive 上传和 result package 持久化完成后，调用固定 Open Design `GET /api/projects/:id/design-system-package-audit`，读取上游原生结构化结果中的 `audit.ok`、`filesInspected`、`errors` 和 `warnings`。HTTP `2xx`、文件存在或 Agent 自述成功均不再被视为 Audit 通过。上游响应中的本地绝对 `projectPath` 在 worker client 边界被丢弃，不会进入 daemon callback 或数据库。

Multica 使用 `multica.open-design-package-audit/v1` 回执保存固定引擎身份、当前 package `content_digest` 和脱敏后的上游 Audit 结果。Server 会再次校验回执 schema、引擎 Release/commit/制品摘要、digest、`audit.ok` 与 errors 一致性、issue severity 和相对路径；任何一项与当前持久 Run 不一致都会拒绝。Audit 通过时回执写入 `audit_report`，Run 继续停在 `run_succeeded` 等待后续 Preview，不新增 `audit_succeeded` 状态，也不创建 draft。Audit 拒绝时，同一条 SQL 同时写入完整 `audit_report`、结构化 failure、`audit_failed` 和 `finished_at`，不存在先写报告再切状态的窗口。

callback 重放边界同时完成闭合：完全相同的通过或拒绝报告可幂等重放；digest、引擎或报告内容不同返回 `409`；已经持久化 `audit.ok: true` 的 Run 不能再被迟到的通用 terminal callback 降级为 `audit_failed`。无法请求或无法解析上游 Audit 时 Supervisor 会 fail closed，尝试写入独立 `audit_failed`，不会继续进入 Preview。

验证结果如下：

- RED 阶段先确认缺少 Package Audit 合约、worker API、回执和错误终态时聚焦测试编译失败；
- `go test ./internal/opendesign ./internal/daemon ./internal/service -count=1`：全部通过；
- `go test ./cmd/server -run 'TestOpenDesign|TestMainRouter' -count=1`：通过；
- 使用当前 `.env` 数据库执行 Audit handler 聚焦测试：通过，覆盖 digest 绑定、通过/拒绝原子持久化、重复回调幂等、冲突报告拒绝和通过状态不可降级；
- `/Users/fengyujie/go/bin/sqlc generate`：成功。

handler 全包仍有两个与本批无调用关系的 Dashboard rollup 基线失败：`TestRollupTaskUsageHourlyIdempotentAndWatermark` 与 `TestRollupTaskUsageHourlyCapsWindowAtOneDay` 均因本地测试库缺少 `task_usage_hourly_rollup_state(id=1)` 失败；相关 Dashboard 源文件无工作区改动，隔离重跑可稳定复现。本批没有修改该模块。

本批严格停在 Package Audit，没有实现 Preview verifier、draft gate 或设计中心 UI，也没有重跑固定真实 worker、真实对象存储和进程 kill/restart 的完整矩阵。成功路径仍为带通过 `audit_report` 的 `run_succeeded`。Phase 0 因 Preview、draft、worker 托管、`missing_cursor` 和真实自动化矩阵仍未闭合，继续保持 No-Go。

### OD-033 Preview 已成为持久门禁，只有真实渲染通过才进入 `succeeded`

`validated implementation evidence`

2026-08-03 完成 Phase 0 production gate 的第六个有界实现批次。Supervisor 在 archive、result package 和 `audit.ok: true` 回执之后，从包内 `preview/manifest.json` 和固定 `ui_kits/app/index.html` 发现待验证页面，再向固定 worker 请求与当前 Project、目标文件严格绑定的临时 Preview URL。URL 必须为同源、根相对和 opaque-origin，sandbox 不允许 `allow-same-origin`，CSP 必须禁止外连；文件错配、跨源 URL、缺失 scope 或 sandbox 逃逸都会 fail closed。

daemon 使用配置项 `MULTICA_OPEN_DESIGN_BROWSER_PATH` 启动隔离 Chromium verifier。每次验证创建独立临时 profile 和固定 `1440 x 1000` 视口，拦截跨源请求，并采集 DOM 计算可见性、可渲染元素、文本、图片/资源失败、控制台错误、异常、截图 SHA-256、截图熵和 RGB 通道标准差。统一纯色页面即使不是白色也不能通过。真实本机 Chromium 测试同时加载正常页面和带外部图片的页面：正常页面通过并产生截图证据，外连页面进入 `outbound_request_blocked`，外部测试 Server 收到的请求数保持为 `0`。

验证结果通过 `multica.open-design-preview-verification/v1` 回执绑定固定引擎身份和当前 `content_digest`。Server 再次校验 schema、引擎、digest 和全部视觉信号；SQL 只允许带 `audit.ok: true` 的 `run_succeeded` 首次写入 Preview。通过时，同一事务把 `preview_receipt`、Run `succeeded`、`finished_at` 和 Agent task `completed` 一起落盘；拒绝时，同一条 SQL 写入完整回执、结构化 failure、`preview_failed` 和 `finished_at`，随后由 daemon 既有失败接口终结 Agent task。相同回执可幂等重放，digest、引擎或回执内容不同返回 `409`。

验证结果如下：

- RED 阶段先确认 Handler 尚无 `RecordOpenDesignRunPreview`，聚焦测试编译失败；
- `go test ./internal/opendesign -count=1`：48 passed，包含真实 Chromium 正向渲染与外连阻断；
- `go test ./internal/daemon -run 'OpenDesign' -count=1`：4 passed；
- 使用当前 `.env` 数据库执行 Audit/Preview handler 聚焦测试：通过，覆盖成功事务、拒绝原子终态、重复回调幂等、digest/引擎/回执冲突以及缺失或未通过 Audit 时回滚 task；
- `go test ./cmd/server -run 'OpenDesignDaemonLifecycleRoutes' -count=1`：通过；
- `/Users/fengyujie/go/bin/sqlc generate`：成功；
- `git diff --check`：通过。

本批没有创建 Design Draft，没有修改设计中心 UI，也没有切换现有主流程。尚未用固定真实 worker、实际对象存储和完整 Server/daemon 进程重跑同一次生产矩阵；worker 托管、`missing_cursor`、真实 kill/restart、统一证据归档和 draft gate 仍未闭合，因此 Phase 0 继续保持 No-Go。

### OD-034 通过 Preview 的同一 Run 才能原子产生 draft，saved 保持隔离

`validated implementation evidence`

2026-08-03 完成 Phase 0 production gate 的第七个有界实现批次。Server 收到通过的 Preview 回执后，不再只完成 Agent task 和 Run，而是先从统一对象存储重新读取当前 Run 的完整 ZIP。draft 提取器会重新校验持久化 artifact index、`content_digest` 与 archive 中每个文件的路径、大小和 SHA-256，并只把 archive 内原始 `DESIGN.md`、`colors_and_type.css` 和 `ui_kits/app/index.html` 保存为现有字段的兼容镜像；不会拼接、改写或伪造旧三文件，完整 archive 继续是事实源。

新增 `multica.open-design-draft-package/v1` manifest，明确保存固定引擎、Supervisor Run、worker Run、task、operation、archive object key、完整 artifact index、content digest、脱敏 result package 和三个兼容镜像的来源文件证据。`multica.open-design-draft-validation/v1` 同时保存同一 digest 绑定的 Package Audit 与 Preview 回执。SQL 在 task completion 的同一事务内再次核对 result、artifact index、archive、Audit、Preview、engine、project/design-system/task 绑定和当前 active operation，随后才完成以下写入：

```text
task running -> completed
open_design_run run_succeeded -> succeeded
project_design_system_package draft -> insert or replace, render_status=passed
project_design_system active_task_id -> null
```

该 SQL 只 upsert `slot = draft`，不会读写 `slot = saved`。已有 draft 会被同一设计体系的新 Run 替换；已有 saved 的所有字段和时间戳保持逐字节不变。对象存储 archive 不可读时返回可重试错误，task、Run、active task 和 draft 均不改变；active task 在回调前发生竞争变化时，task completion、Run、draft 和 active task 整体回滚。完全相同的成功回调可幂等重放，冲突回执仍返回 `409`。

验证结果如下：

- RED 阶段使用真实内存 ZIP 和 `storage.GetReader` 往返，确认完整 result、archive、Audit、Preview 下仍因 draft 不存在而失败；
- `go test ./internal/opendesign -count=1`：通过，覆盖 archive/index/digest 重验、原始兼容镜像提取、索引篡改和缺文件拒绝；
- Audit/Preview handler 聚焦矩阵：通过，覆盖旧 draft 替换、saved 完整不变、active task 清理、重复回调幂等、archive 丢失不落盘和 active task 竞争事务回滚；
- `go test ./internal/handler -count=1` 的本轮 Open Design 与持久化测试全部通过；完整包仍仅有两个无调用关系的 Dashboard rollup 基线失败，均因本地测试库缺少 rollup state/watermark 记录；
- `sqlc v1.31.1 generate`：成功。

本批没有修改 `projectDesignSystemResponse`、设计中心 UI、保存接口或下游读取规则，也没有自动把 draft 保存为项目有效设计体系。固定真实 worker、实际对象存储、进程 kill/restart、`missing_cursor` 策略和一次完整自动化重跑矩阵仍未闭合，因此 Phase 0 继续保持 No-Go，现有设计中心主流程仍不得切换。

### OD-035 SSE 断线采用有界续接，cursor 缺口与 worker Run 丢失具有独立失败证据

`validated implementation evidence`

2026-08-03 完成 Phase 0 生命周期恢复策略的有界实现批次。先对固定上游 `open-design-v0.16.1` / `276b4d8e970bc143d7ad060181a89a834e3d9caf` 重新核对：`createChatRunService` 把 Run 和最多 2000 条 SSE 事件保存在进程内 `Map` 与 ring buffer 中；磁盘 `state.json` 和 `events.jsonl` 用于终态补偿与诊断，但不会在 daemon 启动时把原 Run 重新装载进 Run API。worker 重启后，原 `GET /api/runs/:id` 与 `/events` 因而返回 `404 NOT_FOUND`，不存在可由 Multica 调用的原生 Run 续跑能力。事件接口支持 `after` 与 `Last-Event-ID`，但 cursor 早于 ring buffer 时只会静默返回仍保留的事件，不会主动报告缺口。

Multica Supervisor 现在只在同一 worker Run 上做有限续接。每个事件必须先由 Server callback 成功持久化，Supervisor 才推进本地 cursor；SSE 中断或非终态 EOF 后，最多按 `250ms / 1s / 2s` 退避三次，并从最后持久事件 ID 重新连接。事件 ID 必须从 1 连续递增；重连时上游为终态补发的同内容末事件允许幂等忽略，任何跳号、倒退或同 ID 内容变化都会 fail closed 为 `agent_failed + open_design_event_cursor_missing`。若 worker 重启导致 Run API 返回 404，则记录 `agent_failed + open_design_worker_run_missing`；持续不可达或非终态流反复结束则记录 `open_design_event_stream_unavailable`。

SSE 传输错误不再覆盖已经可由 `GetRun` 证明的 `succeeded`、`failed` 或 `canceled` 终态。终态成功仍必须继续经过原有 result、archive、Audit、Preview 和 draft 门禁，不能仅凭状态创建 draft。该恢复路径不会创建新 Run、Conversation 或 Session，也不会在同一 scratch 上伪造续跑。

验证结果如下：

- RED 阶段 5 个聚焦测试分别稳定暴露不续接、终态被传输错误覆盖、cursor 跳号未拒绝、404 被压平成普通字符串和缺少结构化 HTTP 状态的问题；
- GREEN 阶段同一 5 个聚焦测试全部通过，覆盖从最后已持久事件续接、断线后终态收敛、`missing_cursor`、worker restart 后 Run 丢失和结构化 `NOT_FOUND`；
- `go test ./internal/opendesign -count=1`：55 passed；
- `go test ./internal/daemon -run 'OpenDesign' -count=1`：4 passed；
- GitNexus 对 `Supervisor.Run`、`NewSupervisor`、`StreamRunEvents` 和 `workerHTTPError` 的修改影响均为 LOW；
- `git diff --check`：通过。

本批只闭合代码级恢复判定，没有托管 worker 进程，也没有宣称 worker 重启后可以恢复原生 Run。尚未执行固定真实 worker 的 kill/restart 实跑，未把输入、事件、result、archive、Audit、Preview 和终态导出为同一次统一证据包，也未重跑完整自动化矩阵。因此 Phase 0 继续保持 No-Go，设计中心主流程、保存接口和 UI 均不切换。

### OD-036 终态 Run 可导出确定性统一证据包

`validated implementation evidence`

2026-08-03 完成 Phase 0 统一证据归档的有界实现批次。Server 现在可通过鉴权接口 `GET /api/project-design-systems/{id}/open-design-runs/{runId}/evidence` 导出指定终态 Run 的 ZIP；SQL 同时绑定 Supervisor Run、设计体系和 workspace，不使用 `JOIN`，跨项目体系或跨 workspace 的 Run 不可读取。非终态 Run 返回冲突，缺失的对象存储 archive 会明确失败，不会退化为表面完整的证据包。

证据 ZIP 使用 `multica.open-design-run-evidence/v1` manifest，统一包含固定引擎身份、Supervisor/worker/task/project/design-system 身份、Agent 与输入快照、scratch provenance、preflight、连续事件、failure、result package、artifact index、Package Audit 和 Preview 回执；存在原始 Project archive 时同时包含 `project/archive.zip`。Server 在归档前重新读取对象存储并按持久 `content_digest` 校验原始 archive。所有 JSON 会规范化，文件按路径排序，ZIP 时间戳和权限固定；同一输入重复导出必须产生字节级相同的 ZIP 与 `sha256:` digest。

下载响应使用 `application/zip`、`Cache-Control: no-store`、`X-Content-Type-Options: nosniff` 和 attachment 文件名，并通过 `X-Open-Design-Evidence-Digest` 返回统一证据包摘要。本批没有增加设计中心 UI 或新的页面状态，接口仅服务 Phase 0 自动化验收与复盘。

验证结果如下：

- RED 阶段先确认归档器的大小类型错误，以及下载路由未接入时鉴权测试从预期 `401` 变为 `404`；
- `go test ./internal/opendesign -run TestBuildRunEvidenceArchive -count=1`：3 passed，覆盖确定性 ZIP、非终态拒绝和原始 archive digest 不一致拒绝；
- `go test ./internal/opendesign -count=1`：58 passed；
- 使用当前 `.env` 数据库执行 4 个 evidence Handler 测试：全部通过，覆盖重复下载字节一致、非终态拒绝、workspace/设计体系隔离和对象缺失失败；
- `go test ./internal/handler -run 'OpenDesign' -count=1`：通过；
- `go test ./cmd/server -run '^TestProjectDesignSystemRoutesRequireAuth$' -count=1`：通过，新下载路径在未认证时返回 `401`；
- `/Users/fengyujie/go/bin/sqlc generate`：成功；
- `git diff --check`：通过。

handler 全包仍有两个与本批无调用关系的 Dashboard rollup 基线失败：`TestRollupTaskUsageHourlyIdempotentAndWatermark` 与 `TestRollupTaskUsageHourlyCapsWindowAtOneDay` 均因本地测试库没有对应 rollup state/watermark 行失败；本批未修改 Dashboard 代码。

本批证明了统一证据包的代码级构建、隔离读取和确定性下载，但尚未使用固定真实 worker 与实际对象存储执行并归档创建、调整、取消、Agent 失败、Audit 失败、Preview 失败、进程重启和源仓库零修改的同一套自动化矩阵。worker 制品准备和进程托管也仍在 Multica 外部。因此 Phase 0 继续保持 No-Go，不能切换设计中心主流程或自动保存项目有效体系。

### OD-037 正式 Supervisor 已完成一次真实正向闭环，但设计中心尚不能读取原生 archive UI Kit

`validated runtime evidence`

2026-08-03 使用当前 Multica Server、daemon、固定 Open Design worker 和 Local UI Restore Agent 完成一次正式 `generate` 正向任务。Supervisor Run 为 `019fc839-428a-7a77-80a8-8ca94f190e9e`，worker Run 为 `fe65db58-ec11-49f6-ab0d-db0442e4cf2e`，Agent task 为 `b4df96e4-47de-4f14-bc16-04474df272d1`。任务从 23:23:45 运行至 23:34:30，最终同时满足：Run `succeeded`、task `completed`、`active_task_id` 清空、产生一个隔离 draft、saved 数量保持为零。

本次 Run 固定使用 `open-design-v0.16.1` / `276b4d8e970bc143d7ad060181a89a834e3d9caf`，lockfile digest 为 `90bbe137...31f6f23`，dist digest 为 `bc0a5649...db1e7de`。Agent 快照绑定 Multica Agent `6ef23397-12b3-4857-adca-a76afbff8b40` 与 `opencode` adapter，preflight 核对到 binary `1.18.4`、默认模型可用和插件禁用策略。Agent 事件中共有 33 次工具调用，`task`、`designer`、`delegate` 和 `background_task` 调用均为零，证明主 Agent 直接完成任务；Package Audit 命令恰好执行三次，没有再次进入无界修复循环。

最终原生包的 artifact index 包含 27 项，包括 `DESIGN.md`、`tokens.css`、`colors_and_type.css`、八个 Preview 页面和模块化 `ui_kits/app`。上游 Package Audit 检查 55 个文件，最终为零 error、零 warning。独立 Chrome Preview 对 `ui_kits/app/index.html` 验证通过：159 个可渲染元素、559 个可见文本字符、零控制台错误、零资源失败、零外连请求；截图摘要为 `sha256:8c2fa709...be16c01`，包内容摘要为 `sha256:6c4f583b...2efd54`。

同一终态证据接口连续下载两次均返回 123,686 bytes，两个 ZIP 字节级一致，SHA-256 均为 `4e076684da3862e864fea46023b1f022f9c1c30c696483f55b42961139b5ec23`。归档包含输入、Agent、preflight、147 条连续事件、failure、result package、27 项 artifact index、原始 archive、Audit 和 Preview，可脱离 worker 当前内存复盘该正向 Run。

本次正向 Run 同时暴露了目标架构与旧兼容视图之间的真实边界。完整 archive 中的 UI Kit 依赖 `link`、`script` 和 `ui_kits/app/components/*.js`；当前 draft 提取器只把 `ui_kits/app/index.html` 镜像到旧 `components_html` 字段，而 `projectDesignSystemResponse` 仍调用旧 `projectdesignsystem.Validate`。该校验器禁止 `script` / `link`，并要求旧 `data-design-node-*` locator 和 Token 引用协议；校验失败又被响应层静默转换为空 `sections`、`token_groups` 和 `preview_html`。因此同一个原生 UI Kit 已通过独立 Preview，设计中心仍显示“UI Kit 暂时不可用”，且“保存为项目设计体系”不可用；对该 draft 实测“调整”和“重新生成”接口也都返回 `422 base_package_invalid`。

该现象不是重新生成原生包可以解决的内容质量问题，也不能通过放宽旧三文件校验来修补。Phase 2 应让设计中心通过 archive-backed Preview/UI Kit API 直接读取已验证 Open Design manifest/archive，不再把原生包压回旧三文件协议；在 Phase 0 失败矩阵完成并重新作出 Go/No-Go 决策前，仍不修改设计中心 UI、保存接口或切换主流程。

本次输入的 `repository_grounding_required` 为 `false`、`references` 为空，因此它只证明正式 Supervisor 的正向执行与证据闭环，不能证明同一次 Run 已完成 CRM 仓库来源分析或源仓库零修改验收。取消、Agent 失败、Audit 失败、Preview 失败和真实 worker restart 也尚未在这套正式链路中重跑。Phase 0 继续保持 No-Go。

### OD-038 正式 Supervisor 的真实取消链路已收敛且没有产生或覆盖 draft

`validated runtime evidence`

2026-08-03 在独立项目 `OD Cancel 0803` 中创建一次正式 `generate` 任务，并只用于 Phase 0 取消验收。Supervisor Run 为 `019fc858-3443-7ad1-aa33-243519e67e77`，worker Run 为 `d59feb3d-2488-4fbe-951a-07c87192dd9d`，Agent task 为 `250d7581-a41b-44ca-befa-d64dfdb65540`。验收脚本先等待持久 Run 进入 `running` 且 worker Run ID 已落库，再调用用户侧 task cancel API，因此这不是 queued 状态下的提前撤销。

Server 取消接口返回 task `cancelled`。daemon 在轮询窗口内取消同一个 worker Run；上游 Run API 随后返回 `status: canceled`、`cancelRequested: true`，Multica Run 原子收敛为 `canceled`，failure code 为 `open_design_worker_canceled`，`active_task_id` 清空。设计体系回到 `unestablished`，只保留 `project_design_system_cancelled` 的可读错误；package 表中 draft/saved 总数为零。

取消发生在 Agent 已进入运行状态、但尚未生成 result/archive/Audit/Preview 之前。统一证据 ZIP 因而只包含输入、Agent、preflight、16 条连续事件、failure 和 workspace provenance，明确记录 `archive.included: false`，不会伪造空 archive、Audit 或 Preview。证据接口连续下载两次均为 10,185 bytes，字节级一致，SHA-256 均为 `f4d00535160553a4b9818bed087b3572fc2e2ff9471e543dd5df953402732953`。

作为隔离校验，取消前已有的 `OD-037` 正向项目 draft 仍保持原 manifest 摘要 `9babf8c...3f8a8`、内容摘要 `6c4f583b...2efd54` 和原更新时间，没有被本次取消任务覆盖。该结果闭合了正式链路的“真实取消、独立终态、无坏 draft、可复盘证据”验收项；Agent 失败、Audit 失败、Preview 失败、worker restart 和带仓库来源输入仍未在正式矩阵中重跑，Phase 0 继续保持 No-Go。

### OD-039 正式 Supervisor 的真实 Agent 失败链路已隔离收敛

`validated runtime evidence`

2026-08-04 在独立项目 `OD Agent Fail 0804` 中创建正式 `generate` 任务。Supervisor Run 为 `019fca73-2d02-7eff-8dd3-4510a617a3c5`，worker Run 为 `ca827765-60c4-4288-9cc6-f3252c2fcf95`，Agent task 为 `8f22ca4b-2982-49ae-a8aa-2e6e389ccda7`，设计体系为 `41b48b5b-3ba5-41c2-9bc2-12f2b98b00a2`。任务开始前已确认 daemon 活跃任务和 Open Design 活跃 Run 均为零；任务进入 Multica Run `running`、task `running` 且 worker Run ID 已持久化后，再确认 worker 唯一直属 Agent 子进程为本次新启动的 `opencode run --format json`，只向该进程发送 `SIGTERM`，没有停止 worker、Multica daemon、Server 或其他 Agent。

本次 preflight 真实通过：固定引擎仍为 `open-design-v0.16.1` / `276b4d8e970bc143d7ad060181a89a834e3d9caf`，lockfile 与 dist digest 未变化，adapter 为 `opencode`，binary `1.18.4`，model probe 为 `passed`，插件策略为 `disabled`。Open Design 的短 user prompt 不复制业务 brief，而是明确要求主 Agent 首先读取 `.agent_context/project_design_system/task.json`；50 条持久事件中的真实工具回执证明 Agent 已读取该文件，文件中包含完整 CRM brief、项目、平台、输出策略和 Supervisor Run 身份。失败前已产生 12 次工具调用、11 次工具结果和 2 次文本增量，因此不是 preflight 拒绝、排队失败或未执行任务。

Agent 进程退出后，上游 Run 收敛为 `failed`、`cancelRequested: false`、`errorCode: AGENT_EXECUTION_FAILED`；Multica Run 原子收敛为 `agent_failed`，task 为 `failed`、failure reason 为 `open_design_run_failed`，`active_task_id` 清空。该 Run 没有 result package、archive、Audit 或 Preview，独立设计体系的 draft/saved package 数量均为零。`OD-037` 正向 draft 仍保持内容摘要 `6c4f583b...2efd54`、来源 task `b4df96e4...272d1` 和原更新时间 `2026-08-03 15:34:30.217142+00`，没有被失败任务覆盖。

统一证据接口连续下载两次均为 32,465 bytes，两个 ZIP 字节级一致，SHA-256 均为 `55bc63f48ff87d6c020ae063a2b08d5ae6d498495591afffdb370683becd6223`。归档包含输入、Agent、preflight、50 条连续事件、failure、空 artifact index 和 workspace provenance，并明确记录 `archive.included: false`。该结果闭合了正式链路的“真实 Agent 执行失败、独立终态、无坏 draft、既有 draft 隔离和确定性证据下载”验收项；Audit 失败、Preview 失败、worker restart 和带仓库来源输入仍未在正式矩阵中重跑，Phase 0 继续保持 No-Go。

### OD-040 正式 Supervisor 已拒绝真实 Agent 正常结束后的 Audit 坏候选

`validated runtime evidence`

2026-08-04 先执行过一次未编号尝试：要求 Agent 自行生成缺少 `DESIGN.md` 的坏包，但 Agent 在读取任务后转而长时间研究 Audit 源码，没有形成候选包。该任务 `89c31e26-4a2e-4a35-9f02-73529b668654`、Supervisor Run `019fca7d-d369-7e7b-97a1-47dd6e613e2a` 和 worker Run `2b52a0ba-5c27-4907-b208-d515da243c17` 已由用户取消并分别收敛为 task `cancelled`、Multica Run `canceled` 和 worker `canceled`；`active_task_id` 清空，Multica 没有持久化 result、archive、Audit 或 Preview，也没有产生 draft/saved。该取消运行不计入 OD-040 验收。

正式 OD-040 改用有界且可复现的故障注入：从 OD-037 已通过上游 Audit 和独立 Preview 的原始 archive 构造一次性候选，只省略 `DESIGN.md`，并保留其余包文件不变。注入前使用同一固定引擎的 `design-system-package-audit` 离线预演，结果为 26 个文件、唯一一个 `missing_required_file` / `DESIGN.md` error 和零 warning。该夹具只写入新的隔离 `orchestrator-scratch`，不修改源仓库或生产代码；任务输入也明确记录这是 Phase 0 受控失败验证。

正式任务沿用项目 `OD Audit Fail 0804`，设计体系为 `9423219a-72b5-4e1e-83eb-eb1d2cca2825`。Supervisor Run 为 `019fca92-7ab8-7bb4-a45a-03ea6c652279`，worker Run 为 `033d00ad-1ff8-4472-bac9-4d81132e4026`，Agent task 为 `c10f0009-9205-4719-86e1-351477465396`。preflight 继续绑定 `open-design-v0.16.1` / `276b4d8e970bc143d7ad060181a89a834e3d9caf`、同一 lockfile 和 dist digest、Multica Agent `6ef23397-12b3-4857-adca-a76afbff8b40` 与 `opencode` adapter。

28 条持久事件证明 Agent 真实读取了 `.agent_context/project_design_system/task.json`，只执行一次指定 Package Audit，得到唯一预期 error 后输出事实说明；没有执行项目文件写入或修复。Agent 子进程随后以 `exit 0` 正常结束，上游 worker Run 为 `succeeded`、`cancelRequested: false`。对注入前后的 payload 做逐文件比较，除运行时自己的 `AGENTS.md` 和故意缺失的 `DESIGN.md` 外完全一致，证明 Agent 没有静默改变夹具。

正式 Supervisor 随后收集并持久化 result package、26 项 artifact index 和对象存储 archive，内容摘要为 `sha256:d1b5bdcc06a270501f9112806c021e2bad84df7e2d71de6b0ba2d659c12f97c6`。固定上游 Audit 实际检查 45 个文件，返回合法 `ok: false`、唯一 `missing_required_file` / `DESIGN.md` error 和零 warning；Multica Run 原子收敛为 `audit_failed`，task 为 `failed`，`active_task_id` 清空。Preview 没有执行或持久化，该设计体系的 draft/saved 数量均为零。

作为隔离基线，OD-037 正向 draft 仍保持内容摘要 `6c4f583b...2efd54`、来源 task `b4df96e4...272d1` 和原更新时间 `2026-08-03 15:34:30.217142+00`，没有被 Audit 失败覆盖。统一证据接口连续下载两次均为 50,739 bytes，两个 ZIP 字节级一致，SHA-256 均为 `185261a1d74c9a4e99908191414cb6d7877d7e679a5da90b051a31d2d1c1cb16`，响应头摘要也完全一致。证据包包含输入、Agent、preflight、28 条事件、result package、artifact index、原始 archive、Audit 和 failure，并明确不包含 Preview。

该结果闭合了正式链路的“Agent 正常结束后由上游 Audit 拒绝、独立 `audit_failed` 终态、Preview 短路、无坏 draft、既有 draft 隔离和确定性证据下载”验收项。Preview 失败、worker restart 和带仓库来源输入的正式正向任务仍未闭合，Phase 0 继续保持 No-Go。

### OD-041 正式 Supervisor 已在 Audit 通过后拒绝不可见 Preview

`validated runtime evidence`

2026-08-04 在独立项目 `OD Preview Fail 0804` 中执行正式 `generate` 负向任务。项目 ID 为 `a133599f-88d2-47d3-bddb-7236eeeba245`，设计体系为 `5ba8bda5-57c0-4aa9-8c4a-16ec9782d4ec`，Supervisor Run 为 `019fca9b-0d99-781e-989f-69cdc1ddabff`，worker Run 为 `2af83641-ca12-4915-8018-d3762c128691`，Agent task 为 `867313e6-7741-4846-9ad0-2f9a54ce6d52`。

本次继续采用有界且可复现的故障注入。以 OD-037 已通过 Audit 和 Preview 的原始 archive 为基线，只在 `ui_kits/app/index.html` 的既有 `<head>` 中增加一次 `html,body{visibility:hidden!important}`；文件从 12,041 bytes 变为 12,125 bytes，SHA-256 从 `dcb91244...01db5f` 变为 `1a11dd0...ead088`。正式派发前使用同一固定引擎离线执行严格 Package Audit，结果为 27 files、0 errors、0 warnings、exit 0。该规则已在 OD-027 中由真实 Chrome 证明为“HTTP 和 DOM 正常但页面不可见”的独立 Preview 失败类型。

preflight 继续绑定 `open-design-v0.16.1` / `276b4d8e970bc143d7ad060181a89a834e3d9caf`、同一 lockfile 和 dist digest、Multica Agent `6ef23397-12b3-4857-adca-a76afbff8b40` 与 `opencode` adapter。29 条持久事件证明 Agent 读取了任务上下文，只执行一次指定 Package Audit并得到 46 files、0 errors、0 warnings，随后以 `exit 0` 正常结束；上游 worker Run 为 `succeeded`、`cancelRequested: false`。归档后逐文件比较确认，除运行时自己的 `AGENTS.md` 外，候选与 OD-037 基线的唯一差异就是上述 UI Kit 可见性规则，且变更字节与预期完全一致。

正式 Supervisor 收集并持久化 result package、27 项 artifact index 和对象存储 archive，内容摘要为 `sha256:3b46d41b9293eae617b91f0f62e6bf0f8b0d387ccc5d3363ae3cb9d4e1250674`。固定上游 Audit 实际检查 47 个文件，返回 `ok: true`、零 error 和零 warning。随后独立 Chrome `150.0.7871.187` 按固定 `1440 x 1000` 策略加载 `ui_kits/app/index.html`，取得如下回执：

- 文档加载成功且 DOM 存在，控制台 error、失败资源和外连请求均为零；
- `computed_visibility_visible: false`，可渲染元素数和可见文本长度均为零；
- 截图为 6,491 bytes，熵和最大 RGB 通道标准差均为零；
- 目标稳定返回 `passed: false`，失败分类为 `computed_visibility_hidden`。

Multica Run 因而原子收敛为 `preview_failed`，task 为 `failed`，`active_task_id` 清空。该设计体系没有产生 draft 或 saved；OD-037 正向 draft 仍保持内容摘要 `6c4f583b...2efd54`、来源 task `b4df96e4...272d1` 和原更新时间 `2026-08-03 15:34:30.217142+00`，没有被失败候选覆盖。

统一证据接口连续下载两次均为 53,331 bytes，两个 ZIP 字节级一致，SHA-256 均为 `67104ee1f15455012aaecfe4574474af9f2c6ec7e8015291b9e858d72aad3c49`，响应头摘要也完全一致。证据包包含输入、Agent、preflight、29 条事件、result package、artifact index、原始 archive、通过的 Audit、失败的 Preview 和 failure。

该结果闭合了正式链路的“Agent 正常结束、Audit 通过、独立 Chrome Preview 拒绝、`preview_failed` 终态、无坏 draft、既有 draft 隔离和确定性证据下载”验收项。正式正向、取消、Agent 失败、Audit 失败和 Preview 失败矩阵至此均已跑通；worker restart 和带仓库来源输入的正式正向任务仍未闭合，Phase 0 继续保持 No-Go。

### OD-042 正式 Supervisor 已在 worker 硬重启后持久收敛并可复盘

`validated runtime evidence`

2026-08-04 先执行过一次未编号的优雅停机尝试：项目 `OD Worker Restart 0804` 的 task `28bf34ad-01d6-4055-9f84-55a1ebdc4d05` 已进入真实 Agent 运行，但向 worker 发送 `SIGTERM` 后，上游主动把 worker Run `9537de54-7891-46f4-a385-e5c5efdf9920` 收敛为 `canceled`，Multica 也正确记录为 Run `canceled`。这只能证明优雅停机会取消活跃 Run，不能冒充进程丢失或原生恢复，因此不计入 OD-042。

正式 OD-042 在独立项目 `OD Worker Restart 0804 B` 中执行。项目 ID 为 `01dc4056-873c-47bf-8489-2b0e154b7aae`，设计体系为 `4f0b12bd-b7dd-43fd-82e5-7ec46b3bc264`，Supervisor Run 为 `019fcaa7-55ff-773b-a693-008242d8a038`，worker Run 为 `7151cffb-a2f8-4b52-9429-456f051d93f5`，Agent task 为 `fd16369b-fde4-4195-b428-5f8ad8731c33`。

preflight 继续绑定 `open-design-v0.16.1` / `276b4d8e970bc143d7ad060181a89a834e3d9caf`、同一 lockfile 和 dist digest、Multica Agent `6ef23397-12b3-4857-adca-a76afbff8b40` 与 `opencode` adapter。持久事件和 worker API 均证明 Agent 已启动、真实读取 `.agent_context/project_design_system/task.json`，worker Run 为 `running`，Agent 子进程 PID 为 `5415`。此后只对监听 `17456` 的旧 worker PID `4900` 发送 `SIGKILL`，使其无法主动写入 canceled 终态。

随后立即以固定数据目录、固定 dist 和 Node `v24.14.0` / module ABI `137` 启动新 worker，最终监听 PID 为 `5866`。这里发现一个必须保留的运行约束：裸 `node` 在当前 shell 会解析为 Node 23 / ABI 131，无法加载已按 ABI 137 构建的 `better-sqlite3`；worker 必须由固定制品清单或进程管理器显式携带匹配的 Node runtime，不能依赖交互 shell PATH。新 worker 启动日志报告扫描到 1 个 interrupted terminal，但不会把原 Run 重新装载进内存 API；`GET /api/runs/7151cffb-a2f8-4b52-9429-456f051d93f5` 返回 404，新的 Run 列表为空。

Multica Supervisor 通过持久 SSE cursor 和新 worker 的 404 将该任务独立收敛为 Run `agent_failed`、task `failed`，failure code 为 `open_design_worker_run_missing`，`active_task_id` 清空。该 Run 保留 16 条中断前事件和 scratch provenance，没有 result、archive、Audit 或 Preview，也没有产生 draft/saved。worker 被硬杀后遗留的 Agent 子进程成为 PID 1 的孤儿，验收脚本已显式终止 PID `5415`；新 worker 持续健康且活跃 Run 数为零。

作为隔离基线，OD-037 正向 draft 仍保持内容摘要 `6c4f583b...2efd54`、来源 task `b4df96e4...272d1` 和原更新时间 `2026-08-03 15:34:30.217142+00`。统一证据接口连续下载两次均为 10,591 bytes，两个 ZIP 字节级一致，SHA-256 均为 `7d61b2f6a2c481d8e29a9ce29d8a82d0c88e5bcd3ac886d4253718861b19a3e1`，响应头摘要也完全一致。证据包包含输入、Agent、preflight、16 条持久事件、failure、空 artifact index 和 workspace provenance，并明确记录 `archive.included: false`。

该结果证明真实 worker 硬重启后，Multica 不会在同一 scratch 上新建 Session 伪装续跑，也不依赖新 worker 的内存 Run API复盘；它会以可观测 `open_design_worker_run_missing` 终态隔离失败，并保留确定性证据。正式运行矩阵现在只剩带仓库来源输入及源仓库零修改的正向任务，Phase 0 继续保持 No-Go。

### OD-043 仓库来源正式正向任务闭合 Phase 0 最后一项验收

`validated runtime evidence`

2026-08-04 在隔离项目 `OD Repository Grounding 0804` 中执行正式 `generate` 任务。项目 ID 为 `bcf396e2-4150-4c70-8804-82eaaf5485c3`，设计体系为 `f5751bf2-65da-4914-b478-78f5fa2f50b2`，Agent task 为 `91a65d3a-5de7-4c5f-8437-e6771bc8d7ee`，Supervisor Run 为 `019fcb62-9e50-751c-913b-5a322301e38e`，worker Run 为 `c3c4860b-fe5a-48a0-8247-d53476a8c8a3`。任务从 14:07:45 运行至 14:15:26，最终 task 为 `completed`、Run 为 `succeeded`、`active_task_id` 清空，并只生成一个隔离 draft；saved 数量保持为零。

本次输入真实绑定 CRM 源仓库 `/Users/fengyujie/Documents/soyoung/prime/prime-saas-fe` 的固定 commit `142763d8ca83e3c7bd3cacfb9aa378648e7936c3`。持久输入包含 29 个来源文件和 28 条结构化仓库事实，覆盖 Vue 2.7、Element UI 2.15、权限导航、全局壳层、颜色与字体、紧凑表单和表格、对话框、分页、客户管理与服务流程。121 条连续事件证明所选 Agent `6ef23397-12b3-4857-adca-a76afbff8b40` 真实读取任务上下文，并在独立 `orchestrator-scratch` 中生成来源化产物，而不是直接写入源仓库。

固定引擎仍为 `open-design-v0.16.1` / `276b4d8e970bc143d7ad060181a89a834e3d9caf`，lockfile digest 为 `90bbe137...31f6f23`，dist digest 为 `bc0a5649...db1e7de`。最终对象存储 archive 的 content digest 为 `sha256:7286183de5f3eb5a783e87e3433565b0e3b34049796eb84fa3023ef673d78d7b`，artifact index 包含 24 个文件：Open Design 最小事实包、6 张完整 Preview、4 组来源命名的 JSX/浏览器 JavaScript 组件、模块化 UI Kit，以及上游生成的 handoff 与 manifest。

Agent 侧首次 Audit 以 `thin_token_css` 拒绝候选，修复后第二、第三次均为零错误、零警告；正式 Supervisor 随后独立归档并重跑 Audit，检查 48 个文件，最终 `ok: true`、零 error、零 warning。独立 Chrome `150.0.7871.187` 在 `1440 x 1000` 视口下验证 6 张 Preview 和 1 个 UI Kit 共 7 个声明目标：全部 `passed=true`、DOM 与可见文本非空、截图非空，合计零控制台错误、零失败资源、零失败图片和零外连请求。由同一 digest 绑定的 draft `78cd945d-2a1b-4ed5-9526-de54ce90a5b6` 为 `render_status=passed`，没有自动覆盖 saved 槽位。

任务前后对源仓库进行四项独立复核，结果完全一致：HEAD 均为 `142763d8...36c3`，未暂存 diff SHA-256 均为 `e980c85a...61e8`，暂存 diff SHA-256 均为空摘要 `e3b0c442...b855`，`.agent_context/issue_context.md` SHA-256 均为 `4c3e8352...e710`。仓库已有的三项修改和 `.agent_context/` 未跟踪目录保持原样，没有被清理、覆盖或扩散。

统一证据接口连续下载两次均返回 111,322 bytes，两个 ZIP 字节级一致，SHA-256 和响应头摘要均为 `d797a30ad1ea559c21690c2aeaa7300974e2605e93dda623fede7e4edb6aa04c`。证据包包含来源输入、Agent、preflight、121 条事件、result package、24 项 artifact index、原始 archive、正式 Audit、7 目标 Preview 回执、空 failure 和 workspace provenance，可脱离 worker 当前内存完整复盘。

`OD-043` 补齐了正式运行矩阵最后缺少的“带仓库来源输入、同一次正向 production gate、源仓库零修改”证据。结合 `OD-037` 至 `OD-042` 的正向、取消、Agent 失败、Audit 失败、Preview 失败和 worker restart 结果，Phase 0 十项验收条件全部闭合，Go/No-Go 结论转为 **Go**。该结论只批准固定引擎与 Supervisor 门禁进入下一阶段；设计中心 archive-backed 展示、固定 worker 部署托管、保存切换和旧链路清理仍需单独实现与验收。

### OD-044 设计中心已直接读取并展示通过门禁的原生 archive Preview/UI Kit

`validated runtime evidence`

2026-08-04 完成 Phase 2 的第一个有界切片。设计中心不再要求把 Open Design 原生包压回旧三文件协议才能预览：受保护的 manifest 接口只选择当前项目设计体系中 `render_status=passed` 的 draft，缺少 draft 时才读取 passed saved，并重新绑定来源 Run、对象存储 archive、artifact index、content digest 和包内 manifest。Server 从 archive 发现真实 Preview/UI Kit targets；文件接口每次仍按同一 digest 和 artifact index 读取归档内容，不暴露对象存储 key，也不接受未验证包或任意磁盘路径。

原生 UI Kit 在 iframe `sandbox="allow-scripts"` 中运行，未开放 `allow-same-origin`、表单、弹窗或顶层跳转。首次 Chrome 验收发现主 HTML 为 `200`，但相对 CSS 和四个组件脚本均为 `401`，控制台报 `PrimeKit is not defined`。原因不是 archive 缺文件，而是受限 iframe 中的相对资源请求不会携带当前 SameSite 登录 Cookie。修复后，受保护 manifest 签发一个 30 分钟只读 capability；它通过 HMAC 绑定 workspace、design system、content digest 和过期时间，并进入资源 URL 路径，使 archive 内相对资源自动继承。资源路由位于登录 Auth 组外，但没有 capability、范围被篡改、digest 不匹配或 capability 过期时均不能读取文件。响应继续设置严格 CSP、`nosniff`、no-referrer 和受控 MIME 类型，不以降低 iframe 沙箱强度换取资源加载。

在用户本机 Chrome 对设计体系 `f5751bf2-65da-4914-b478-78f5fa2f50b2` 完成修复后验收：6 张 Preview 和 1 个 UI Kit 可切换；UI Kit HTML、`colors_and_type.css`、`tokens.css` 和四个组件脚本全部返回 `200`；页面真实显示 5 列表头和 2 行数据，主题色计算为 `#00AB84`；颜色 Preview 与 UI Kit 往返切换正常，修复后没有新增 `PrimeKit` 控制台错误。archive 不可用时，设计中心仍保留旧 `preview_html` 作为明确降级，不把旧兼容内容冒充原生 archive。

本切片自动化验证结果：

- handler 聚焦测试通过，覆盖 passed manifest、绑定文件流、未验证包拒绝、capability workspace/system/digest 范围和过期拒绝；
- router 聚焦测试通过，覆盖受保护 manifest 路由与 capability 文件路由注册；
- `@multica/core` typecheck 通过，52 个文件、501 个测试全部通过；
- `@multica/views` typecheck 通过，132 个文件、1,267 个测试全部通过；
- 前后端健康检查分别为 `8080 health=ok` 和 `3031 HTTP 200`，验收期间未重启前端或破坏登录态。

`OD-044` 只证明 archive-backed 读取、资源隔离和真实展示，不表示原生包的调整、保存、放弃或 saved 切换已经完成，也不表示固定 worker 已进入部署托管。当前 capability 不会在页面持续打开超过 30 分钟后自动续期；重新加载页面会取得新 capability，长开页面的无刷新续期应作为后续有界可靠性任务处理。

### OD-045 通过门禁的原生 archive 草稿可以原子保存并继续展示

`validated runtime evidence`

2026-08-04 完成 Phase 2 的第二个有界切片。保存接口现在先按 manifest schema 分流：旧兼容包继续使用原有三文件校验；`multica.open-design-draft-package/v1` 原生包则锁定当前 draft，并复用 archive Preview 的完整证据加载器，重新验证来源 Run、对象存储 archive、artifact index、content digest、Package Audit、Preview 回执和固定引擎身份。保存事务仍复用现有原子提升语义，只把通过门禁的同一 draft 复制到 `saved`，随后删除 `draft`；验证失败时不会改变两个 slot。

前端保存条件也从“必须存在旧兼容三文件内容”收敛为“原生 archive targets 已成功加载，或旧兼容内容可渲染”。因此原生 UI Kit 不需要伪装成旧三文件包即可出现“保存为项目设计体系”操作，旧包的降级路径仍然保留。

自动化测试使用包含真实相对脚本资源的 archive 草稿，先证明旧三文件校验会拒绝该候选，再通过原生 archive 门禁完成保存。测试逐字节比较保存前后的 manifest 与 validation，并比较 digest 与 source task，确认四项证据完全不变；同时确认 draft 被删除，saved archive Preview 继续返回原 slot 和 content digest。前端回归则证明只有 archive targets 已加载、旧兼容内容为空时，保存按钮仍可启用。

在用户本机 Chrome 对 OD-044 的同一设计体系 `f5751bf2-65da-4914-b478-78f5fa2f50b2` 完成真实验收。保存前页面显示“草稿 / 有未保存更改”，原生 UI Kit 可见，保存按钮唯一且启用；点击后请求 `POST /api/project-design-systems/{id}/save` 返回 `200`，页面原地切换为“已保存”并显示成功提示。数据库随后确认 `saved_at` 已设置、`draft=0`、`saved=1`、`render_status=passed`，saved digest 仍为 `7286183d...d78d7b`，manifest schema 仍为 `multica.open-design-draft-package/v1`，source task 与对应成功 Open Design Run 的 task 完全一致。保存后在同一页面切换到 `Preview · colors palette` 可见真实 Prime SaaS 色板，再切回 `UI Kit` 仍显示客户管理页面、5 列表头和 2 行数据；相关 archive HTML、CSS 和组件脚本继续返回 `200`。

本切片验证结果：

- Open Design archive 保存与 Preview handler 聚焦测试通过；
- `@multica/views` 全量 132 个测试文件、1,268 个测试全部通过；
- monorepo TypeScript 检查 6 个任务全部通过；
- `git diff --check` 通过；
- 后端 `8080` 健康检查为 `ok`，验收期间未重启前端，用户登录态保持不变。

`OD-045` 只闭合原生 archive 的首次保存与 saved 展示，不表示原生 archive 的 Agent 调整、放弃草稿、重新生成或 capability 自动续期已经完成。下一有界切片应先验证原生 archive 的放弃草稿语义，再进入调整链路。

### OD-046 原生 archive 草稿沿用现有原子放弃语义

`validated implementation evidence`

2026-08-04 完成 Phase 2 的第三个有界切片。影响分析确认 `DiscardProjectDesignSystemDraft` 风险为 `LOW`；接口本身只锁定当前项目与设计体系，在同一事务内确认没有活跃任务、删除 `draft` 并清理草稿错误状态，随后由统一响应选择剩余的 `saved`。该流程不读取旧三文件内容，也不按 manifest schema 分支，因此 Open Design 原生 archive 无需新增一套放弃实现。

本切片补充两条真实原生 archive 数据库回归：

- 首次原生 archive 草稿放弃后，响应回到 `unestablished`，`has_unsaved_changes=false`，draft/saved package 总数为零，archive Preview 明确返回不可用；
- 已有 saved 的调整场景中，先把通过门禁的原生 archive 保存，再建立独立 draft 并执行放弃。放弃后 draft 数量为零，saved 的 manifest、validation、digest 和 source task 与放弃前逐字节一致，响应回到 `saved`，archive Preview 从 saved slot 继续返回原 content digest。

前端回归同时使用“archive targets 已加载、旧兼容内容为空”的设计体系，确认保存操作可用，更多菜单仍提供“放弃草稿”。聚焦结果为后端 2 个原生 archive 放弃测试通过、Canvas 21/21 通过、`git diff --check` 通过。由于 OD-045 的真实设计体系已经进入 saved 且当前没有调整 draft，本切片没有为了演示删除而污染用户资产；数据库事务和前端操作分别在隔离测试中完成验证。

`OD-046` 只闭合首次草稿放弃和放弃调整恢复 saved，不表示 Open Design 原生 Agent 调整任务已经能读取 saved archive、产出新 archive draft 并通过同一套 Audit/Preview 门禁。该调整闭环是下一有界实现阶段。

## 3. 基于证据的初步启示

以下是 `proposal`，不是已确认结论：

1. Multica 的设计体系可以采用分层包思想，但应存储在云端并绑定现有 Project。
2. 一次设计任务应固定使用的需求、设计体系、模板和参考资源版本，保证可追溯。
3. Open Design 的本地 Project 更适合映射为 Multica Issue 下的一次 Design Run 或设计工作空间。
4. 模板应被理解为带规则、示例、来源和能力边界的执行参考，而不是待修改的静态业务 JSON。
5. 设计体系草稿不应在未经审核时静默成为所有 Agent 的新约束。
6. Multica 可以考虑明确区分项目自有的强约束设计体系，以及社区资源、模板和其他体系提供的弱参考，避免把视觉灵感升级成项目事实。
7. Multica 不必预设所有项目拥有同等完整度；可以先定义可发布的最小共同契约，再根据真实来源逐步增加组件、模式、资产和平台扩展。

## 4. 当前证据不能证明什么

本轮研究还不能证明：

- Open Design 的数据结构可以直接用于 Multica 的多租户云端环境；
- HTML/CSS/JS 一定是 Multica 在线设计稿的唯一主格式；
- Open Design 的 Agent 执行质量满足 Multica 的真实 UI 设计要求；
- 社区模板的质量、许可证和安全模型已经适合直接接入；
- 本地 daemon 架构适合 Multica 云端与本地 Agent 协同；
- `89d6d4e` 之后的 Open Design 上游仍会保持相同交互和内部结构。

这些内容必须分别研究和验证，不能从当前证据外推。
