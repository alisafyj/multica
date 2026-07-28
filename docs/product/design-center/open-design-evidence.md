# Open Design 研究证据台账

> 研究日期：2026-07-28
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
