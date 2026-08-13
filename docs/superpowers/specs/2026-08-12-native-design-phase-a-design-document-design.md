# Native Design Phase A：页面 Design Document 产品与技术方案

> 日期：2026-08-12
>
> 状态：`confirmed`
>
> 当前范围：项目 Native V2 设计体系闭环与设计中心首页页面设计闭环
>
> 后续范围：工作区共享设计体系、官方模板、工作区成员模板、跨工作区社区模板
>
> 行为基线：Open Design 的分层资源包、持续工作空间、Package Audit、真实 Preview 与坏草稿隔离
>
> 明确排除：Open Design Worker/Daemon/Runtime、大型 PageSpec DSL、中心化浏览器服务、无浏览器降级路径

## 1. 决策摘要

Native Design Phase A 交付两条相互衔接的产品闭环：

1. 用户为项目创建、调整并保存 Native V2 设计体系；
2. 用户从设计中心首页输入页面设计需求，选择项目和智能体，使用项目上下文、真实仓库与已保存设计体系生成页面 Design Document，完成预览、调整与保存。

页面设计正式产物采用轻量、版本化的 `multica.design-document/v1`。一份 Design Document 可以包含一个主页面、相关子页面、状态、弹窗和关键流程。它以语义 brief 表达需求关系，以完全离线的可运行原型表达真实布局与交互，以 coverage 表达需求覆盖和质量检查，但不恢复大型 PageSpec、通用 Scene Graph 或逐像素布局 DSL。

每次首次生成或调整都创建独立智能体 task，固定输入快照与 base revision，在该 Design Document 的持续隔离工作空间中执行。通过安全收集、Package Audit、现有本地 `designpreview` 浏览器强制门禁和服务端完整性校验后，系统创建新的不可变 revision，并原子移动 `draft` 指针。只有用户明确保存后，`saved` 才移动；下游智能体、MCP 和交付链始终只读取 `saved`。

Phase A 不引入新的中心化 Chromium 服务，也不增加无浏览器降级、待验证候选态或前端补验证协议。员工本地守护进程继续自动解析本机 Chrome/Chromium 并执行现有 Preview 门禁；浏览器不可用时任务失败，不形成或覆盖 draft。

## 2. 产品目标与成功标准

### 2.1 用户价值

Phase A 完成后，用户应当能够：

- 为项目建立一套可生成、可理解、可预览、可调整并可保存的设计体系；
- 从设计中心首页描述真实页面设计需求，而不必先进入项目内部寻找入口；
- 明确选择目标项目和执行智能体，并可选关联已有任务（Issue）与目标平台；
- 自动使用项目已保存设计体系作为视觉强约束；
- 由所选智能体读取其运行时可访问的真实项目仓库，形成有来源的页面设计；
- 获得包含完整页面、状态、弹窗和关键流程的可运行离线原型；
- 对整份文档、页面、状态、弹窗或命名区块发起自然语言调整；
- 在坏产物、失败、取消或保存失败时继续保有最近一次有效内容；
- 明确保存后，让该设计稿成为下游可消费的项目页面设计事实。

### 2.2 产品成功不能由什么代替

以下任一项都不能单独代表 Phase A 成功：

- 智能体 task 显示 `completed`；
- 输出目录中存在文件；
- package schema 可以解析；
- 自动测试通过；
- 页面 DOM 非空；
- 原型能够渲染；
- 数据库存在 Design Document 或 revision 行；
- 旧 Open Design Worker 实验曾经通过。

严格成功必须同时具备真实用户需求、真实智能体、真实仓库 grounding、项目已保存设计体系、完整产物、Package Audit、本地浏览器 Preview、draft/saved 隔离、用户 Chrome 验收以及人工视觉和业务质量判断。

## 3. 范围与三个新增需求的分期

### 3.1 新 Phase A：设计中心首页页面设计入口

首页是跨项目页面设计任务发起器，不是无项目画布，不创建第二套 Project，也不从首页创建设计体系。

第一版负责：

- 自然语言页面设计需求；
- 截图、参考设计和需求附件；
- 必选项目；
- 必选执行智能体；
- 可选关联已有任务（Issue）；
- 可选目标平台；
- 最近页面设计任务与 Design Document；
- 创建成功后进入目标项目“设计草稿”；
- 首页和项目 Tab 读取同一服务端 task/document 状态。

### 3.2 Slice B：工作区共享设计体系

不直接给项目设计体系增加 `is_public`。共享对象必须从项目当前 `saved` Native V2 package 重新校验、安全剥离并生成独立、不可变的共享 revision。

它未来只作为页面任务的弱参考进入输入快照，不能覆盖项目 saved 设计体系，也不能隐式建立或替换项目体系。Slice B 不进入 Phase A 完成条件。

### 3.3 Slice C 至 E：模板与社区

模板是页面设计 task 的可执行配方，不是设计体系。后续依次建设：

- Slice C：官方模板目录；
- Slice D：工作区成员模板发布；
- Slice E：跨工作区社区模板。

模板只作为任务配方和弱参考，不能覆盖用户明确需求、项目 saved 设计体系、权限边界与 Audit/Preview 门禁。Slice C 至 E 均不进入 Phase A。

### 3.4 四类实体必须分开

| 实体 | 职责 |
| --- | --- |
| Project design system | 项目当前已保存的设计事实源和页面任务视觉强约束 |
| Published design-system resource | 安全剥离、不可变的共享视觉弱参考 |
| Template resource | 页面设计 task 的版本化执行配方 |
| Applied template snapshot | 某次 task 实际固定使用的模板、体系和用户输入 |

不得用一个 `is_public` 字段或一套旧 template 实体同时承载这些语义。

## 4. 核心产品对象

### 4.1 Design Document

Design Document 是项目中的稳定页面设计资产身份。

一份文档可以包含：

- 一个主页面；
- 相关子页面；
- 加载、空、错误、成功等页面状态；
- 弹窗、抽屉、菜单等浮层；
- 关键用户流程；
- 用户需求与可选任务（Issue）的覆盖关系。

一个项目允许同时拥有多份 Design Document。文档可以关联一个任务（Issue），但不依赖该关联才能创建或保存。

### 4.2 Document Revision

Document Revision 是 Design Document 的一次完整、不可变版本。每个 revision 对应一份完整的 `multica.design-document/v1` package。

旧 revision 不允许原地修改。首次生成、全局调整、局部调整和重新取证成功后都创建新 revision。

### 4.3 Draft 与 Saved 指针

每份 Design Document 维护：

```text
draft_revision_id
saved_revision_id
```

- `draft_revision_id`：当前最新且已通过全部门禁的候选 revision；
- `saved_revision_id`：用户明确确认、允许下游消费的当前 revision。

第一版用户界面只表达当前 draft 与 saved，不提供完整版本时间线、逐版本 diff 或任意历史恢复。内部 revision、task 和验证证据继续保留。

### 4.4 智能体 task

首次生成、调整、重新取证和失败重试都创建独立 task。task 固定本次输入快照和 base revision，不复活已经结束的旧 task。

同一 Design Document 第一版只允许一个活动写 task，不自动合并并发分支。

### 4.5 持续智能体工作空间

每份 Design Document 拥有一个持续隔离工作空间。它可保留设计分析、中间上下文和可复用工作产物，但不能改写旧输入快照、旧 revision 或直接控制 draft/saved 指针。

首次有效 draft 形成前，task 使用候选工作空间身份；Design Document 原子创建后，该工作空间归属文档并用于后续独立调整 task。

## 5. 首页到项目的真实用户闭环

```text
设计中心首页
→ 输入页面设计需求和可选附件
→ 选择项目和智能体
→ 可选关联任务（Issue）和目标平台
→ 服务端固定初始 task 上下文
→ 创建页面设计智能体 task
→ 打开或聚焦目标项目 Tab
→ 进入“设计草稿”的进行中 task 区
→ 智能体在运行时执行有界只读仓库取证
→ 生成完整 Design Document package
→ Package Audit
→ 本地 Chrome/Chromium Preview
→ 全部门禁通过
→ 原子创建 Design Document 和首个不可变 revision
→ revision 成为当前 draft
→ 进行中 task 原位转为 Design Document
→ 用户预览、调整或保存
```

首页和项目 Tab 始终读取同一个服务端 task，不创建本地伪 task 或伪 draft。

首次生成失败、取消或产生坏包时：

- 保留 task、输入快照、活动事件和错误证据；
- 保留用户输入以供修改后重试；
- 不创建空 Design Document；
- 不污染项目文档列表；
- 不形成或覆盖 draft/saved。

## 6. 项目信息架构

项目“设计草稿”Tab 包含四个层次。

### 6.1 进行中 task 区

展示尚未形成首个有效 draft 的页面设计 task：

- 需求摘要；
- 智能体；
- 可选关联任务（Issue）；
- 开始时间；
- 运行时长；
- 最后活动；
- 可由真实事件证明的状态；
- 停止操作。

不显示虚构百分比，不提前创建空文档。

### 6.2 Design Document 列表

每份文档显示：

- 用户可编辑标题；
- 需求摘要；
- 可选关联任务（Issue）；
- 当前状态；
- 智能体；
- 更新时间；
- 页面与状态数量的简洁摘要。

第一版不自动按页面类型分类，也不展示内部 revision 数量。

### 6.3 文档详情工作区

依次展示：

1. 文档标题、状态、关联任务和更新时间；
2. 页面、状态和关键流程导航；
3. 可运行原型 Preview；
4. 需求覆盖和未覆盖项；
5. 当前项目设计体系约束摘要；
6. 调整、保存与放弃操作。

调整面板默认关闭。用户可以发起全局调整，也可以从页面、状态、弹窗或命名区块发起局部调整。

### 6.4 最近 task 记录

失败、取消和已完成 task 保留可追溯记录，但不与 Design Document 资产混排。

## 7. `multica.design-document/v1` 产物协议

### 7.1 资源包结构

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

`prototype/` 可以按实际复杂度增加内部文件，但所有文件必须由 `manifest.json` 声明，且不得引用 package 外内容。

浏览器验证回执不写入它所绑定的 package，否则会形成循环摘要。Package Audit receipt、Preview receipt 和验证截图是与 package content digest 绑定的独立证据对象；统一证据导出时可以与原始 package 一起归档。

### 7.2 `manifest.json`

负责确定性身份和完整性，至少固定：

- schema version；
- Design Document、revision、project、workspace 和可选 `issue_id`；
- task、智能体和目标平台；
- input snapshot digest；
- base revision 与 base content digest；
- 项目 saved 设计体系 revision/digest；
- 后续可选共享体系与模板 revision/digest；
- 文件索引、媒体类型、大小和逐文件 digest；
- package content digest；
- prototype 入口；
- Preview 目标集合；
- 产物协议版本。

不得包含本地绝对路径、凭据、完整源码或未授权附件地址。

### 7.3 `brief.json`

它是轻量语义层，表达：

- 文档目标和需求摘要；
- 页面与子页面；
- 页面状态；
- 弹窗、抽屉和其他浮层；
- 关键用户流程；
- 模拟数据场景；
- 页面、状态与需求之间的映射；
- 页面、状态和命名区块的稳定语义 ID；
- 可访问性和关键交互要求；
- 明确非目标。

它不表达逐像素坐标、完整组件树或 CSS 实现，不演进为 PageSpec DSL。

### 7.4 `prototype/`

它是视觉布局和交互的真实表达。第一版允许 package 内 HTML、CSS 和 JavaScript，支持：

- 页面切换；
- Tab、筛选和排序；
- 弹窗、抽屉和菜单；
- 表单输入和本地校验；
- 加载、空、错误和成功状态；
- 模拟数据和本地状态转换；
- 项目设计体系 Tokens 和授权资产。

第一版禁止：

- 网络请求与 WebSocket；
- 真实项目 API；
- 用户或项目凭据；
- 外部脚本、样式、CDN 和远程字体；
- Service Worker；
- 自由服务器命令；
- 修改用户源仓库；
- 依赖浏览器扩展或宿主页面同源权限。

原型必须在断网环境中完整运行。它用于验证设计和关键流程，不等同于生产前端代码。

### 7.5 `coverage.json`

至少记录：

- 用户需求覆盖；
- 可选任务（Issue）需求覆盖；
- 页面、状态、弹窗和流程覆盖；
- 项目 saved 设计体系一致性；
- 关键交互目标；
- 模板残留检查；
- 未覆盖项及原因；
- 智能体声明的检查结果。

智能体自评不能直接作为通过依据；系统仍需独立验证可确定的项目。

### 7.6 独立验证证据

Audit/Preview receipt 至少绑定：

- package content digest；
- verifier 与浏览器版本；
- 实际测试的 prototype 入口和目标；
- 页面可见性、尺寸和溢出结果；
- 资源、Console 与网络结果；
- 关键交互结果；
- 验证截图及其 digest；
- 最终 verdict；
- verified_at。

Agent 不能生成或伪造系统通过回执。Package 变化后旧回执自动失效。

## 8. 输入快照与仓库 Grounding

### 8.1 首次 task 输入快照

至少固定：

- workspace 与 project；
- 可选 `issue_id` 和当时可读取的需求内容；
- 用户自然语言需求；
- 附件引用和内容 digest；
- 所选智能体；
- 可选目标平台；
- 当前项目 saved 设计体系 revision/digest；
- checkout/commit；
- 仓库取证结果；
- task 协议和产物 schema 版本。

后续共享体系和模板切片可以增量加入固定的 resource/revision/digest 与 applied template snapshot。

### 8.2 task 内自动只读 Grounding

首页不要求用户预先运行独立仓库分析。所选智能体在其运行时可访问范围内完成有界只读取证，至少形成：

- checkout/commit 身份；
- 相对来源路径；
- 文件内容 digest；
- 框架、路由和页面入口；
- 可复用组件和变体；
- Tokens、样式和字体事实；
- 布局、信息密度和交互模式；
- 与需求相关的业务对象；
- 冲突、缺失事实和不确定性；
- 事实与推断的明确区分。

长期快照不保存绝对路径、凭据、环境变量、无关业务数据或未授权完整源码。

### 8.3 仓库不可访问

如果所选智能体运行时不能访问仓库：

1. 在设计执行前明确提示缺失的是仓库访问；
2. 用户选择仅使用项目描述、关联任务、附件和 saved 设计体系继续，或停止并更换智能体/运行时；
3. 继续时输入快照明确记录 `repository_grounding=unavailable`；
4. coverage 和结果页面明确提示未经过仓库现实约束。

不得静默降级并把结果描述为已 grounding。

### 8.4 后续调整与重新取证

普通调整默认沿用当前输入快照中的仓库事实。用户主动选择“同步最新仓库”时：

- 重新读取 checkout/commit；
- 重新生成仓库事实快照；
- 显示仓库变化摘要；
- 用户确认后创建新的调整 task；
- 新 task 固定新的 input snapshot；
- 旧 revision 继续保留原始仓库依据。

## 9. 持续工作空间与智能体 task

工作空间逻辑结构：

```text
context/
  input-snapshots/
  repository-facts/
  design-system/
reference/
work/
output/
```

- `context/`：固定输入，只读；
- `reference/`：授权附件和参考资产，只读；
- `work/`：智能体中间分析和实验；
- `output/`：本次 task 的最终 package staging。

每个 task 固定：

```text
document_id（首次有效 draft 前为空）
operation
input_snapshot_id
base_revision_id
base_content_digest
agent_id
workspace_id
project_id
optional issue_id
target scope
```

工作空间规则：

- 旧输入快照和旧 revision 不可修改；
- 智能体不能直接更新 draft/saved；
- 智能体不能修改用户源仓库；
- 每个 task 只写自己的 staging/output；
- 系统有界收集最终产物；
- 拒绝符号链接、硬链接、路径穿越和工作空间外引用；
- 同一文档第一版只允许一个活动写 task。

## 10. 调整、保存与放弃

### 10.1 调整范围

第一版支持：

- 整份文档；
- 指定页面；
- 指定页面状态；
- 指定弹窗或抽屉；
- manifest/brief 中声明的命名区块。

scope 使用稳定语义身份，不把临时 CSS selector 或 DOM 路径写入长期协议。任意 DOM 点选、框选和所见即所得编辑不进入 Phase A。

### 10.2 调整 task

调整固定：

```text
document_id
base_revision_id
base_content_digest
input_snapshot_id
scope
instruction
agent_id
```

即使只修改一个区块，智能体也必须输出完整 Design Document package。通过 Audit/Preview 后创建新 revision，并原子更新 draft 指针。

如果提交的 base digest 不等于当前 draft digest，系统拒绝本次调整，要求用户刷新并基于当前 draft 重新提交。第一版不自动合并分支。

### 10.3 首次保存

用户点击“保存设计稿”后，Server 重新确认 revision、digest 和验证回执，并原子设置：

```text
saved_revision_id = draft_revision_id
```

保存不复制第二套页面资产、不重新生成 package、不允许智能体代替用户执行，也不自动改变关联任务（Issue）状态。保存失败时 draft 保留，saved 不变。

### 10.4 保存后的继续调整

已有 saved 的文档产生新 draft 时：

- 用户默认 Preview 当前 draft；
- 下游智能体、MCP 和交付链继续读取 saved；
- 保存成功后 saved 原子切换到当前 draft；
- 保存失败时旧 saved 有效，新 draft 不丢。

### 10.5 放弃

首次 draft 尚未保存时，放弃会清除 draft 指针并从正常文档列表移除该文档；内部 revision 与 task 证据按保留策略留存。

已有 saved 时，放弃调整只让 draft 指针恢复到 saved，不重跑智能体、不复制 revision、不删除历史证据。

### 10.6 重试

失败或取消后保留需求、项目、智能体、可选关联任务、平台和附件选择。用户可以修改后重试；每次重试创建新 task 和新输入快照，不复活旧 task。

## 11. 任务（Issue）关系

项目与智能体必选，任务（Issue）可选。

- 有明确需求任务时固定现有 `issue_id` 和当时可读取的需求；
- 无关联任务时仍可创建探索性 Design Document；
- 系统不自动创建任务（Issue）；
- 后续补充关联不改写历史 revision 输入，只从后续 task/revision 生效；
- Design Document 与 task 可以在任务时间线中显示可追溯事件或链接；
- 保存设计稿不自动改变任务状态、负责人、优先级或完成状态。

设计成功不等于开发交付完成。任何自动工作流属于后续独立能力。

## 12. Package Audit 与现有浏览器强制门禁

### 12.1 成功公式

```text
智能体 task completed
+ 输出安全收集成功
+ Package 结构与 digest 校验通过
+ Design Document Audit 通过
+ 本地 Chrome/Chromium Preview 通过
+ 输入快照和 base revision 绑定一致
+ 服务端原子持久化成功
= 有效 draft revision
```

任一条件失败都不创建新有效 revision、不移动 draft、不影响 saved，也不把 task 描述为设计成功。

### 12.2 Audit

至少验证：

- 必需文件、schema、文件索引、大小、媒体类型和 digest；
- 文件数量、单文件大小和总包大小；
- 路径穿越、绝对路径、符号链接、硬链接和未声明文件；
- 远程 URL、CDN、外部字体、网络模块、Service Worker 和 WebSocket；
- 真实 API 地址、凭据、自由命令、未批准浏览器能力和外部表单提交；
- `brief.json`、prototype、coverage 和 manifest 的身份与目标一致性；
- 项目 saved 设计体系引用和实际 Token 使用；
- base revision/digest；
- 模板残留、无关旧业务字段和占位内容。

### 12.3 本地浏览器 Preview

直接复用现有 `server/internal/designpreview`，由员工本地守护进程自动解析本机 Chrome/Chromium 并运行。Phase A 不引入中心化浏览器服务。

验证至少覆盖：

- 主页面和声明目标可打开；
- 真实可见内容；
- 页面尺寸与阻断性溢出；
- 包内资源加载；
- Console error；
- 外部网络请求；
- 页面或状态切换；
- Tab、筛选或排序；
- 弹窗、抽屉开关；
- 表单本地状态与校验；
- 关键流程从起点到目标状态。

继续保持当前无降级语义：浏览器无法解析时，task 以 `project_design_system_preview_unavailable` 或未来页面文档等价错误失败，不跳过 Preview，不形成或覆盖 draft。

### 12.4 Preview 安全

- 完全断网或拦截所有外连；
- 独立临时浏览器 profile；
- 独立 origin 或严格 sandbox；
- 固定 CSP；
- 不携带 Multica 登录 cookie；
- 不获得宿主页面同源权限；
- 不允许下载、外站跳转或打开新窗口。

### 12.5 人工质量边界

浏览器门禁只证明原型能够安全运行，不判断设计是否美观、是否具有品牌感、信息架构是否最佳或业务流程是否合理。

不得把“能够渲染”描述成“视觉质量已通过”。最终视觉和业务质量由真实用户验收判断。

## 13. 状态与错误处理

### 13.1 用户可见状态

- 首次生成 task 进行中；
- 未保存草稿；
- 已保存；
- 有未保存调整；
- 调整中；
- task 失败或取消。

失败是一次操作结果，不是长期审核状态。Phase A 不增加待审核、批准或驳回。

### 13.2 错误矩阵

| 失败类型 | 页面行为 |
| --- | --- |
| 智能体不可执行 | 保留输入，允许更换智能体 |
| 仓库不可访问 | 明确提示，用户选择无 grounding 继续或停止 |
| 智能体执行失败 | 保留 task 与输入，允许创建新重试 task |
| 用户取消 | 不形成新 revision，保留最近有效状态 |
| Package 收集失败 | 显示产物不完整，不进入 Audit/Preview |
| Package Audit 失败 | 显示安全或一致性失败，不进入 Preview |
| 浏览器不可用 | task 失败，不跳过门禁 |
| Preview 失败 | 显示不可见、资源、Console、网络或交互问题 |
| 对象存储失败 | 不创建 revision，不更新 draft |
| completion 重放或 digest 不匹配 | 拒绝，不修改现有状态 |
| 调整基线冲突 | 要求刷新后基于当前 draft 重试 |
| 保存失败 | 保留 draft，旧 saved 不变 |
| 放弃失败 | 保持当前指针，不伪装成功 |

主界面优先说明失败步骤、有效内容是否安全、输入是否保留以及用户下一步，而不是只显示技术错误。

## 14. Phase A 安全边界

Phase A 明确不允许：

- 智能体修改用户源仓库；
- 原型调用真实业务 API；
- 原型携带用户或项目凭据；
- 智能体自行决定 revision 生效；
- 未经 Audit/Preview 的 package 进入 draft；
- draft 被下游智能体、MCP 或交付链当作项目有效设计稿；
- 使用旧 revision 的验证回执批准新 package；
- Preview iframe 获得 Multica 宿主同源权限；
- 把“能够渲染”描述成“视觉质量已通过”。

Phase A 同时不新增：

- 无浏览器时跳过 Preview；
- 待浏览器验证候选状态；
- 前端补验证替代协议；
- 中心化 Chromium 服务；
- 无浏览器也允许保存的例外。

## 15. 明确非目标

- 无项目设计；
- 从首页创建设计体系；
- 自动识别页面设计与设计体系意图；
- 大型 PageSpec 或通用 Scene Graph；
- 任意 DOM 点选和所见即所得编辑；
- 真实 API 联调；
- 智能体修改项目仓库；
- 自动生成前端 PR；
- 完整 revision 时间线和视觉 diff；
- 多人并发编辑和自动合并；
- 任务（Issue）状态自动推进；
- 工作区共享设计体系发布；
- 官方或社区模板；
- Figma 原生写回；
- Native Design JSON；
- 设计还原和最终交付。

## 16. Phase A 严格验收

### 16.1 固定样本

继续使用真实 CRM 项目，至少包含：

- 一条真实页面设计需求；
- 可选关联真实任务（Issue）；
- 当前 CRM 仓库与 commit；
- 项目已保存 Native V2 设计体系；
- 至少一项截图或参考资料；
- 用户明确选择的真实智能体；
- Web 目标平台。

不能用预制固定 package 冒充真实生成。

### 16.2 正向链路

1. 首页提交页面设计需求；
2. task 创建后进入目标项目“设计草稿”；
3. 进行中 task 展示真实智能体活动；
4. 智能体完成只读仓库 grounding；
5. 生成完整 `multica.design-document/v1`；
6. Package Audit 通过；
7. 本地 Chrome/Chromium Preview 通过；
8. 首个不可变 revision 形成；
9. task 转为 Design Document；
10. 用户打开并操作离线原型；
11. 用户对命名区块发起一次局部调整；
12. 新 revision 通过相同门禁；
13. 旧 draft/saved 在调整期间保持安全；
14. 用户明确保存；
15. 刷新后标题、revision、digest、Preview 和 saved 一致；
16. 关联任务显示可追溯链接，但状态未自动改变。

### 16.3 人工质量验收

用户在真实 Multica 页面中检查：

- 是否回应真实需求；
- 是否使用 CRM 业务对象和信息结构；
- 是否遵循项目 saved 设计体系；
- 颜色、字体、间距、密度与组件语言；
- 页面、状态、弹窗和流程完整性；
- 关键交互可理解性；
- 是否存在通用模板感、占位文案或模板残留；
- 是否存在溢出、空白、遮挡或不可读区域；
- 局部调整是否只改变预期范围并保持整体一致。

严格验收记录用户 Chrome 页面、Network、Console、原型截图、调整前后结果和最终人工判断。

### 16.4 失败隔离

首次生成至少覆盖：智能体不可用、仓库不可访问、取消、智能体失败、缺少文件、digest 不一致、外连、危险脚本或凭据、浏览器不可用、页面不可见、JavaScript 启动失败、对象存储失败和 completion 重放。

已有文档调整至少覆盖：base mismatch、调整失败或取消、Audit 失败、Preview 失败、保存失败和放弃失败。

### 16.5 持久化不变量

- workspace/project/可选 `issue_id` 隔离；
- task 绑定所选智能体；
- input snapshot 不可变；
- grounding 绑定 commit 与来源摘要；
- base revision/digest 正确；
- package index 和 content digest 一致；
- Preview receipt 绑定同一 digest；
- revision 不可修改；
- 首次失败不产生 document；
- 调整失败不移动 draft；
- 保存失败不移动 saved；
- 放弃只更新指针，不删除历史证据；
- 下游只读取 saved revision。

## 17. 已有工程基础与进度口径

Phase A 不从零开始。现有 Native V2 项目设计体系闭环、旧页面草稿链路和设计中心工作区已经提供大量可复用基础，但页面 Design Document 本体尚未落地。实施计划必须从这些已有能力出发，不得重复建设并行的任务、对象存储、Audit、Preview 或前端工作区基础。

### 17.1 可直接复用的基础

#### 通用任务与本地运行时

- `agent_task_queue` 的创建、claim、运行、取消、失败和完成生命周期；
- 用户明确选择智能体及其本地运行时；
- 守护进程 task 工作目录、运行环境、GC 和资源回收；
- task 活动、使用量和终态上报。

#### Native V2 package 管道

- 安全收集、文件索引、逐文件 digest 和 content digest；
- 输入快照摘要与 base package digest；
- 对象存储上传和稳定 object key；
- Package Audit 的诊断、HTML/CSS/URL/凭据检查框架；
- completion 重验、digest 绑定和失败隔离；
- draft/saved 隔离与原子保存行为范式。

#### 浏览器 Preview

- `server/internal/designpreview` 的本地 Chrome/Chromium 解析、独立 profile、同源限制、网络拦截、DOM 可见性、尺寸、资源、Console 和截图检查；
- 与 package digest 绑定的 Preview receipt；
- 守护进程在 Agent 结束后、completion 前执行 Preview 的正式调用点；
- 浏览器不可用时 fail closed、不跳过门禁的现有语义。

#### 仓库与设计上下文

- `RepositoryDesignContext` 的 commit、相对路径、来源和摘要校验；
- 项目设计体系独立仓库分析、来源冻结和 completion 经验；
- Project Design Context Resolver 读取项目 saved 设计体系；
- base package 下载和只读 task sidecar 经验。

#### 设计中心和旧页面草稿

- 固定首页 Tab、项目 Tab 的打开/关闭/切换；
- 项目“设计稿 / 设计草稿 / 模版 / 设计体系”内容导航；
- 设计体系创建、任务活动、内容画布、Preview、调整、保存和放弃 UI；
- `design_draft`、设计文件、revision、资产和可选 `issue_id` 的历史模型；
- 旧 `CreateDesignDraftAgentTask` 的智能体 task、Issue、Design Context 和模板候选接线。

### 17.2 必须翻新的基础

以下代码和模型可以提供模式，但不能直接当成完成结果：

- 项目设计体系的 package `draft`/`saved` 槽位必须翻新为 Design Document 的不可变 revision 和指针语义；
- `multica.project-design-system/v2` 的 manifest/Audit 必须扩展或抽象为独立的 `multica.design-document/v1`；
- 设计体系独立仓库分析不能直接替代页面 task 内自动 Grounding；
- 现有 `designpreview` 要叠加页面、状态、弹窗、表单和关键流程交互检查；
- 旧 `CreateDesignDraftAgentTask` 强制 Issue 或模板并生成 PageSpec，不能成为新首页默认路径；
- 设计体系调整 scope 和包槽位保存行为要翻新为文档级 scope、base revision 冲突和指针移动；
- 现有设计草稿列表要翻新为进行中 task、Design Document 列表和详情工作区。

### 17.3 尚未实现的核心

- `design_document` 稳定身份；
- 不可变 Document Revision；
- `draft_revision_id` / `saved_revision_id`；
- `multica.design-document/v1` package 和 schema；
- 文档级 input snapshot、持续 workspace 和多文档模型；
- 首页页面设计输入与新 task API；
- Design Document 的交互 Audit、首个 draft 原子形成和文档详情 Preview；
- 文档级调整、指针保存/放弃和 base conflict；
- 真实 CRM 页面 Design Document 严格验收。

### 17.4 与历史 `semantic_design_draft` 的关系

仓库中不存在名为 `semantic_design_draft` 的独立表。该名称指现有 `design_draft` 表在迁移 874 后形成的 `generation_mode=semantic_pagespec`、`page_spec`、`compiled_native_json`、`quality_report`、`blueprint_id`、`recipe_set_id`、`parent_draft_id` 和 `version` 语义路径。

A1 必须在不破坏历史 `design_draft` 数据和消费者的前提下确定新 Design Document 持久化方式。旧 PageSpec 路径只在当前子切片证明完整替代且符合 DC-040 门禁后局部停止写入或删除；跨切片消费者和历史数据进入退役账本，不执行破坏性迁移。

### 17.5 当前工程进度

进度使用两个口径：

- **产品设计确认度：100%**；
- **Phase A 工程完成度：约 40%–45%，当前基线记为约 42%**。

42% 是工程复用估算，不是测试通过率或交付声明。估算采用保守分值与工作权重：

| 子切片 | 现有覆盖 | 当前估计 | 主要剩余工作 |
| --- | --- | ---: | --- |
| A1 | 存储、digest、Audit 框架、对象存储和迁移基建 | 55% | 文档实体、revision、指针、新 package/schema |
| A2 | 首页/项目 Tab 壳、task 和旧草稿入口 | 25% | 首页表单、新 task API、进行中 task 区 |
| A3 | 仓库事实契约、工作目录和分析经验 | 35% | task 内 Grounding、文档 workspace、Prompt/Skill |
| A4 | Audit、`designpreview`、receipt、原子 draft 管道 | 60% | 文档 Audit、交互检查、首个文档 draft |
| A5 | 调整、保存、放弃和 draft/saved 隔离语义 | 45% | revision 指针、文档 scope、冲突和多文档 UI |
| A6 | 固定 CRM 样本和验收方法 | 0% | 页面 Design Document 全链路真实验收 |

按 A1 20%、A2 15%、A3 15%、A4 20%、A5 20%、A6 10% 的预计工作权重计算，保守加权约为 41%，考虑通用任务和前端基础的跨切片复用，报告基线取约 42%。该数字只用于规划；每完成一个子切片，必须基于实际产物、验证和剩余工作重新计算，不能按阶段数量机械递增。

## 18. Phase A 实施切片

### A1：Design Document 核心协议与持久化

- `multica.design-document/v1`；
- Design Document 和不可变 revision；
- input snapshot；
- draft/saved 指针；
- package 收集、对象存储和 digest；
- Audit 基础；
- 暂不涉及首页 UI。

### A2：首页任务入口与项目 task 状态

- 首页输入区；
- 项目、智能体、可选任务（Issue）和平台；
- 附件；
- task 创建；
- 项目“设计草稿”的进行中 task 区；
- 首次失败不产生空文档；
- 不运行旧 PageSpec 默认路径。

### A3：仓库 Grounding 与持续工作空间

- task 内有界只读仓库取证；
- commit 和来源摘要；
- Design Document 持续 workspace；
- 首次生成 Prompt/Skill；
- 智能体输出完整 package；
- 源仓库零修改验证。

### A4：Audit、Preview 与首个 Draft

- Design Document 静态 Audit；
- 复用现有本地 `designpreview` 强制门禁；
- 离线交互原型验证；
- digest 绑定 receipt；
- 首个 document/revision/draft 原子形成；
- 项目详情 Preview。

### A5：调整、保存与放弃

- 文档、页面、状态和命名区块调整；
- immutable base revision；
- 新 revision；
- draft/saved 隔离；
- 保存、放弃和基线冲突；
- 多份文档列表。

### A6：真实 CRM 严格验收

- 真实智能体；
- 真实仓库 grounding；
- 真实 saved 设计体系；
- 用户 Chrome；
- Network/Console；
- 局部调整；
- 保存刷新；
- 失败隔离；
- 人工视觉和业务质量结论。

A1 至 A5 的自动化完成不能替代 A6。

## 19. 渐进清理门禁

每个 A 切片只清理自身完整替代的旧路径：

- 首页切片只处理页面任务入口内已被替代的旧生成分支；
- Design Document 切片只处理被 package/revision 完整替代的旧页面草稿路径；
- 不因名称包含 `PageSpec`、`OpenDesign` 或 `V1` 就扩大删除；
- 跨切片编译器、旧 API、历史表和对象进入退役账本；
- 普通切片不删除历史数据、表或约束；
- `feature/fengchen-fixed-v2` 不作为代码来源。

每个实施切片都必须提供：

1. Native V2 正向合同；
2. 失败隔离合同；
3. 本切片旧路径负向合同；
4. 范围外业务回归；
5. 持久化不变量；
6. 退役账本变化；
7. 实际验证命令；
8. GitNexus `detect_changes`；
9. 独立回滚边界。

## 20. Phase A 最终交付

Phase A 完成后，用户获得：

1. 可创建、调整并保存的项目 Native V2 设计体系；
2. 可从设计中心首页发起的页面设计 task；
3. 使用真实项目、可选任务（Issue）、智能体、仓库和 saved 体系的固定输入；
4. 包含完整页面流程的版本化 Design Document；
5. 可运行、可交互、完全离线的页面原型；
6. 与 package digest 绑定的 Audit 和 Preview 证据；
7. 可持续调整但不会污染 saved 的工作空间；
8. 多份项目页面设计资产；
9. 与任务（Issue）可追溯但不自动改变工作流状态的关系；
10. 为共享设计体系和模板留下明确但尚未启用的不可变引用边界。

## 21. 当前状态与下一步

本方案的产品对象、产物协议、输入快照、仓库 Grounding、持续工作空间、强制浏览器门禁、revision 生命周期、界面、错误模型、严格验收与 A1 至 A6 切片均已由用户确认。

下一步只允许：

1. 用户复核本书面规格；
2. 获得批准后，为 A1 至 A6 编写详细实施计划；
3. 计划必须逐切片执行 impact、测试、真实证据与退役账本门禁；
4. 在实施计划获批前不修改产品代码、不迁移取消路线提交、不执行旧 Phase B 或破坏性数据清理。
