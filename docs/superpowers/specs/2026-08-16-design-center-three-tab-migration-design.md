# 设计中心三 Tab 迁移方案

> 状态：已确认产品方向，实施中
> 日期：2026-08-16
> 参照基线：`open-design-v0.19.2`
> 决策依据：DC-047 至 DC-055；替代 DC-025
> 适用范围：设计中心首页、社区模板、设计体系，以及 Design Document 工作区中的 tweaks 与 critique

## 1. 决策摘要

Multica 从 Open Design 迁移的范围收窄为三个 tab 内部的能力：**首页**、**社区**、**设计体系**。Studio 不迁——它的等价物是 Multica 项目内的 Design Document 工作区。

迁移方式沿用 P-010：以 Open Design 为行为基线，由 Multica 用现有 Project、Issue、Agent、daemon、任务队列和对象存储原生实现，不运行、分发或托管其 Worker、Daemon 和 Runtime。

三项与既有方案不同的产品决定：

1. 设计体系按仓库划分，替代 P-008 / DC-025 的“每个项目一套”；
2. 发起设计任务时仓库可选，选中才做有界只读取证；
3. tweaks 与 critique 进入产品，但都不得成为 draft 形成的判定条件。

## 2. 范围

### 2.1 范围内

| Tab | 迁入的能力 |
| --- | --- |
| 首页 | 统一设计任务发起器：自由文本输入、场景 chip 轨、场景插画、设计体系与模板选择器 |
| 社区 | 模板画廊：两级分面、媒体预览卡、详情、「用这个 prompt」回填首页、「Remix」直接发起 |
| 设计体系 | 体系目录（搜索 / 分类 / 全屏预览）、Showcase·Token·`DESIGN.md` 三视图、多来源创建向导 |

### 2.2 范围外

Studio 外壳、自动化、集成、看板、成员、团队工作区、插件市场管理面、164 个功能 skill、本地 daemon 与 26 个 CLI 适配器、Electron 外壳、BYOK 代理、MCP server、clipper 扩展、HTML/PDF/PPTX/MP4 导出，以及 deck / image / video / audio 产物形态本身。

首页 chip 与社区分类会**展示**产物类型的概念，但第一阶段只实现 prototype 一种真实产物。

### 2.3 一个上游事实

Open Design 已在 v0.13.0（commit `29b138f7a`，#4691）把 Brands（中文标签“设计系统”）并入设计体系，品牌提取降级为创建向导的一个来源。`BrandsTab.tsx`、`/brands` 路由和 `entry.navBrands` 在 0.19.2 已无导航入口。

Multica 照搬合并后的模型：**不建独立的品牌套件实体**，品牌提取只是设计体系创建向导的来源之一。

## 3. 前置：生产端契约对齐

### 3.1 问题

`handler/project_design_system.go:1228` 起，除仓库分析外的所有 generate / adjust / regenerate 任务都被标记 `PackageSchema = multica.project-design-system/v2`（三个生产调用点均把 `openDesignRun` 传 `nil`，worker 路径已无生产调用方）。

但智能体收到的产物指令是 V1 三文件契约。`v2_archive.go` 的 `classifyV2Artifact` 只接受：

```text
DESIGN.md   tokens.css   source/index.json   USAGE.md
design-tokens.json   components.manifest.json
ui-kit/index.html   preview/*.html   assets/**   fonts/**
```

`components.html` 不在其中。实测结果：

- 三文件目录 → `archive_path_undeclared: file is outside the V2 package contract`
- 去掉 `components.html` → `V2 package requires at least one UI Kit or Preview target`

两条都在 `daemon/project_design_system_package.go:154` 落到 `status = blocked` / `FailureReason = project_design_system_audit_failed`，在 Audit 之前失败，走不到 Preview 和 draft。

### 3.2 第二处缺陷

预览服务注入的 `<link rel="stylesheet" href="tokens.css">` 是相对路径，而 `classifyV2Artifact` 只接受下一级目录中的预览目标（`ui-kit/index.html`、`preview/*.html`）。实测两者分别解析到 `/<prefix>/ui-kit/tokens.css` 和 `/<prefix>/preview/tokens.css`，均返回 404。

危险之处在于它不会响亮地失败：Audit 的 `token_usage_missing` 是对 HTML 文本的静态检查，Preview 只检查可见性。两者都通过，于是系统对一个从未应用过设计 Token 的页面出具了通过回执和截图。

### 3.3 根因与修复

两侧规范由不同代码独立规定，没有任何测试跨越这条边界——prompt 测试只断言 prompt 文本，audit 测试只用手写的正确 fixture。

已完成的修复：

| 项 | 位置 |
| --- | --- |
| prompt 改写为 V2 包契约，含 `source/index.json` 完整 schema 与 `input_snapshot_sha256` 来源 | `daemon/prompt.go` 的 `projectDesignSystemPackageContract()` |
| 三处次级契约陈述改为指向包契约 | `execenv/runtime_config.go` ×2、`execenv/context.go` |
| 样式表注入改为绝对路径 `/<prefix>/tokens.css` | `daemon/project_design_system_package.go` 的 `injectBridgeAndTokens` |
| 跨边界回归：按 prompt 声明的文件集构造 package，必须通过真实 `CollectV2Directory` 与 `auditV2Package` | `daemon/project_design_system_prompt_contract_test.go` |
| 每个预览目标解析注入的 href 并实际拉取，必须 200 且为 `text/css` | 同上 |

尚未完成：用真实智能体跑通一次原生 generate，取得非 worker 路径的模块化 archive、零告警 Audit 和 Chrome Preview 证据；随后重算 Phase A 基线。

### 3.4 进度口径影响

DC-046 记录 Phase A 工程基线约 42%，其中 A4 记为 60%。该数字建立在一条从未被原生链路走通的管道上，必须在 3.3 的真实证据取得后重算，不得沿用。

## 4. 设计体系仓库化

### 4.1 动机

`project_resource` 已支持一个项目挂多个 `github_repo`，旧的 `design_repo_analysis` 也已按 `project_resource_id` 分仓库存储。但新的 V2 设计体系是项目级唯一（`UNIQUE (workspace_id, project_id)`），`RepositoryDesignContext` 也没有仓库标识。

一个同时包含 C 端 H5、App 和后台管理系统的项目，三者的设计语言被迫共用一套体系。而 C 端 H5 与后台管理系统同为 `platform=web`，现有 platform 维度分不开它们。

### 4.2 模型

`project_design_system` 增加可空的 `project_resource_id`：

| 取值 | 含义 |
| --- | --- |
| `NULL` | 项目级体系——跨仓库通用，也是不选仓库时使用的那套 |
| 非 `NULL` | 该仓库专属体系 |

现有行天然是项目级，**零数据迁移**。

### 4.3 唯一性

必须拆成两个 partial unique index。PostgreSQL 将 `NULL` 视为互不相等，单一复合唯一键 `(workspace_id, project_id, project_resource_id)` 会放行多条项目级体系。

按仓库迁移规则，每个并发索引单独一个单语句迁移文件，编号从 `877` 起：

- `877`：加列并删除旧的 `UNIQUE (workspace_id, project_id)`；
- `878`：`CREATE UNIQUE INDEX CONCURRENTLY … WHERE project_resource_id IS NOT NULL`；
- `879`：`CREATE UNIQUE INDEX CONCURRENTLY … WHERE project_resource_id IS NULL`。

不加外键。删除仓库时在应用事务内清理其设计体系，沿用 `project_resource_design_cleanup_test.go` 的既有模式。

### 4.4 解析链

在 DC-035 的既有链条前加一级：

```text
选了仓库 → 该仓库专属体系（saved）
             ↓ 没有
          项目级体系（saved）
没选仓库 → 项目级体系（saved）
             ↓ 没有
          local_design_md → repository_reality → none
```

`service/design_context_resolver.go` 的 `ProjectDesignContextStore` 需要一个 resource-scoped 查询；`DesignContextSourceCloudSaved` 需要携带 scope 信息，以便下游能区分“用的是仓库体系还是项目级体系”。

### 4.5 连带修正

`project_design_system.platform` 原为项目级且 `NOT NULL`，一个项目只能声明一个平台，与多形态项目对不上，实践中只能填 `cross_platform` 规避。按仓库划分后其语义变为“该仓库是什么形态”，字段回到它本来表达的东西。

### 4.6 界面

DC-031 要求设计体系 Tab 直接承载内容主视图，不使用摘要列表和二级入口。多体系之后需要一个仓库切换器。

这属于 **scope 切换而非列表页**：内容主视图仍然直接渲染，只是有了 scope。接入点是 `packages/views/designs/project-design-system-workspace.tsx`——它是一个 99 行的分发器（未建立 → Create / 生成中 → TaskActivity / 已有 → Canvas），切换器加在其上。

不得据此退回摘要列表加二级入口的形态。

### 4.7 不引入工作区默认体系

目录只做“挑选 → 复制成项目体系”。要统一视觉时使用「从现有体系复制」，项目内明确留下自己的一份，保持可追溯。

加默认体系会带来两个问题：项目里看不到体系却有风格，来源不直观；更换工作区默认会让所有未建体系的项目产出集体改变，而它们的 saved 内容一行未动，与 DC-034 的原子替换心智冲突。

体系仓库化让「从现有体系复制」从可选项变成主要用法——三仓库项目要建三套体系，先建好一套再复制调整是常态路径。

### 4.8 改动面

已实测：Server 侧 1 个 API 路由（`/api/project-design-systems`）、3 个 handler 调用点、1 个 resolver 接口；前端 1 个 query option、1 个 query key、2 个 realtime 失效点（`use-realtime-sync.ts:1121` 与 `:1405`）。

## 5. 首页发起器

### 5.1 现状

`packages/views/designs/designs-page.tsx:604` 是一个没有内容的 tabpanel。

### 5.2 复刻边界

复刻 Open Design 首页的**视觉与信息架构**，品牌替换为 Multica Design。**不搬运代码**：Open Design 使用 Next.js 16 + React 18 + CSS Modules，Multica 必须按仓库 UI 规范用 shadcn/Base UI、语义 token 和 role-named `--text-*` 字号重写。

`HomeHero.tsx`（4984 行）与 `HomeView.tsx`（3569 行）中的 AMR 余额与登录、DeepSeek Harness 设置、Vela 计费、宠物、campaign 和 onboarding 均不迁。要复刻的是输入卡 + chip 轨 + 场景插画 + 选择器区。

### 5.3 chip

Open Design 的 12 个创建 chip 里有 7 个的 `projectKind` 同为 `prototype`——它们产出同一类东西，区别在绑定不同配方：`example-web-prototype`（直接画界面）、`example-web-clone`（先要 URL，侦察→复刻→审计）、同一 plugin 换 inputs（线框图 / 移动应用）、`od-figma-migration`（把 Figma 画框迁到当前体系）。

chip 的真实维度是**场景配方**，不是产物类型。

第一版只放五个有真实产物支撑的：UI Mockup、网站复刻、线框图、移动应用、**来自 Figma**。「来自模板」和「创建品牌套件」灰态留位，等对应切片点亮。其余八个按第 2.2 节不放。

「来自 Figma」保留的理由：Multica 已有完整的 Figma 导入链路（`design_file` / `design_revision` / `native-renderer`），这个 chip 能把设计 tab 的两半接起来，是 Open Design 没有而 Multica 独有的优势。

### 5.4 输入项

项目与智能体必选；仓库与任务（Issue）可选；附带附件。

发起 API 从第一版就带 `recipe` 字段，第一版只接受上述五个值，模板切片建成后扩展为模板 id，不改 API 契约。

### 5.5 其他

项目侧新增「进行中 task」区。首次失败不产生空文档，不走旧 PageSpec 默认路径。

## 6. 仓库 Grounding

选中仓库时，在 task 内对**该仓库**执行有界只读取证，固定 commit、相对来源、结构化事实与不确定性。这同时把 DC-043 原本含糊的“有界”定义为“对这一个仓库取证”。

`RepositoryDesignContext` 需补 `project_resource_id`，与旧 `design_repo_analysis` 的粒度对齐。

未选中仓库时跳过整个 grounding 阶段直接生成，使用项目级设计体系，并在文档中显式标注本次未做仓库取证——不得让用户误以为智能体读过代码。这与 P-008「不增加隐式或强制的前置仓库扫描」一致。

## 7. tweaks 与 critique

两者在 Open Design 属于 Studio，不属于三个 tab，在 Multica 落在项目内的 Design Document 工作区。

### 7.1 tweaks

它是一个 skill 而不是平台 UI：由智能体把产物重构为读取 CSS 自定义属性（`--accent`、`--scale`、`--density`、`--mode`、`--motion`），并附带包内 vanilla-JS 侧栏把控件绑到这些变量，改动持久化到 `localStorage`。用户不必重新提问就能试变体。

**边界**：只允许进入 Design Document 的 `prototype/`——Phase A 第 7.4 节允许包内 HTML/CSS/JavaScript 与本地状态转换。**不得进入设计体系包**：V2 Audit 与设计体系 prompt 均禁止 script、事件属性、表单和外部引用。同一能力在两个包里两种待遇，需要在 prompt 层分开表达。

### 7.2 critique

`apps/daemon/src/critique/` 共 18 个文件、3458 行。真实形态包含：designer / critic / brand / a11y / copy 五个带权重的评审角色；`maxRounds`（1–10）、`scoreScale`、`scoreThreshold` 构成的多轮循环；每轮与总体超时、并发上限；run registry、scoreboard、transcript 与持久化；SSE 流式回放。`ratchet.ts` 是灰度阶段推进建议，属运维工具，不迁。

**边界**：critique 是产物成型前的迭代改进循环，Audit / Preview 是产物成型后的系统门禁。DC-034 不松动——critique 分数达标**不构成 draft 形成条件**，draft 仍只能在 Audit 与 Preview 通过后原子形成。

**配置取舍**：`fallbackPolicy: fail` 与 Multica 的 fail-closed 语义一致，采用；`ship_best` 与 `ship_last` 与「不允许把失败或不完整内容自动推进为已保存状态」冲突，不采用。

## 8. 实施顺序

```text
契约对齐 → 设计体系仓库化 → A1 → A2 → A3 → A4 → A5 → A6 → 体系目录 → 社区模板
```

设计体系仓库化排在 A1 之前，因为 A2 的仓库选择与 A3 的取证边界都依赖它。

体系目录与社区模板排在 A6 之后（DC-054 先窄后宽），A2 只为两个选择器留灰态位置。依据是契约对齐暴露的问题：这条链路至今没被真实跑通过，先证明一条端到端的路能走通，优于同时铺开三条。

每个切片仍须携带 DC-040 的九项门禁：V2 正向合同、失败隔离合同、本切片旧路径负向合同、范围外业务回归、持久化不变量、退役账本变化、实际验证命令、GitNexus `detect_changes`、独立回滚边界。

## 9. 验收

A6 的严格验收范围随本方案扩大：至少覆盖同一项目下两个不同仓库（如 C 端 H5 与后台管理系统），验证同一套流程在不同形态下的产出差异，以及仓库体系与项目级体系的回落是否正确。

A1 至 A5 的自动化通过不能替代 A6。

## 10. 退役账本

Open Design worker 接入已无任何生产调用方：`open_design_run` 表（迁移 870–872）、`daemon/open_design_task.go`、`daemon/client.go` 的 6 个 `/open-design/*` 端点、`handler/project_design_system_open_design_{base,draft,evidence,lifecycle,preview}.go`。

按 DC-040，它应在触达它的切片内登记进 `native-v2-retirement-register.md` 并推进状态。功能切片最多把条目推进到 `retired` 或 `data-pending`；表和历史行的删除须另开规格并取得独立审批。
