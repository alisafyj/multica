# 项目设计体系第一阶段验证记录

> 验证日期：2026-07-29
> 当前结论：第一阶段主成功链路已通过真实 Chrome、Agent 产物、Server 校验、数据库与刷新持久化验证；失败保护及目标自动化套件均已通过。未破坏真实 daemon 或诱导真实模型生成恶意文件，相关剩余验证边界见文末。

## 1. 验证对象

| 对象 | 值 |
| --- | --- |
| 项目 | `staffrnapp` |
| Project ID | `0ad3978e-231e-4237-9005-4803879730a2` |
| Design System ID | `56e71040-0487-48f5-847a-5ca7537bd1dc` |
| Agent | `Local UI Restore Agent` |
| Agent ID | `6ef23397-12b3-4857-adca-a76afbff8b40` |
| Runtime ID | `4f381116-786f-486f-ab92-848631808c82` |
| 真实数据库 | `multica_recovered_20260729`，PostgreSQL `5433` |
| 前端 | `http://localhost:3031` |
| 后端 | `http://localhost:8080` |

验证期间后端 `/health` 返回 `{"status":"ok"}`，设计体系详情路由返回 HTTP 200。

## 2. 首次生成基线

首次成功生成任务：

- Task ID：`c49f8fee-1fe1-4cd3-961b-22b896adc70c`
- Package slot：`draft`
- Package digest：`fae6ac82395b6fce27eaa2e1b1f519d3d1dfadf5239927a4427683ae2e579704`
- Server validation：`passed=true`
- Diagnostics：空数组

调整前文件哈希：

| 文件 | SHA256 |
| --- | --- |
| `DESIGN.md` | `74400d5067d7929965f6a0d523d281030ef0a0f4b55401070ce5c6e24a1dd41b` |
| `tokens.css` | `7badebabdffaabfc8197215ea0e0687382c52148a0143e136e5ade2431c3f327` |
| `components.html` | `e452272c1550c991d97bf534573f9338a371d95ec1d1de1ea6084721ca6b3ba1` |

## 3. 真实组件级调整

在用户本机 Chrome 中点击 UI Kit 唯一的“填写三方订单”后，调整范围显示为“待处理订单卡片”，持久化 scope 为：

```json
{"id":"pending-order-card","kind":"component"}
```

提交的完整调整要求：

> 把待处理订单卡片的状态表达改为更明确的品牌蓝左侧状态边，并把“待处理”标签改为白底品牌蓝描边。保持卡片尺寸、订单内容、操作按钮和其他组件不变；同步更新 DESIGN.md 中的订单卡状态规则、tokens.css 中对应 Component Token，以及 components.html 中的卡片样式，确保三份产物一致。

调整任务：

- Task ID：`d48f91d2-4084-47fb-a203-bf8ba81c96db`
- Agent ID 与所选 Agent 一致
- Agent 实际读取了三份 base 文件并写入三份完整替换产物
- Agent 回读检查了三个非空文件、HTML 片段契约、31 个唯一设计节点和 Token/样式一致性
- Server 最终 validation 为 `passed=true`，diagnostics 为空

不能仅凭任务 `completed` 判定成功。以下是产物替换证据：

| 文件 | 调整前 SHA256 | 调整后 SHA256 |
| --- | --- | --- |
| `DESIGN.md` | `74400d5067d7929965f6a0d523d281030ef0a0f4b55401070ce5c6e24a1dd41b` | `dd6b1e72083a2aeb718f1ea298b903e8d4046a7bdfb4c6e0d374512a29211267` |
| `tokens.css` | `7badebabdffaabfc8197215ea0e0687382c52148a0143e136e5ade2431c3f327` | `6bd29e5b5a43bb98293a772319a9e4b45f8d335fea294b6cf97a7a3c88ce940d` |
| `components.html` | `e452272c1550c991d97bf534573f9338a371d95ec1d1de1ea6084721ca6b3ba1` | `6dff005dcc8492ec07189d87ac2e79d23f24dea4cb9b340e103d3fa9187f496c` |

Package digest 由 `fae6ac82...9704` 变为：

```text
829b6988543d36a790ab20f6837d20a10ef868959c7b04ce8c094e858a29e0fc
```

三个文件的实际 diff 只包含目标范围：

- `DESIGN.md` 新增待处理卡片左侧状态边规则，并细化待处理标签规则；
- `tokens.css` 新增 5 个 `--cmp-order-pending-*` Component Token；
- `components.html` 新增两条限定在 `.order-card.pending` 下的样式，并只给 `pending-order-card` 增加 `pending` class；
- 订单文案、金额、客户信息、履约时间和操作按钮没有变化。

## 4. UI Kit 视觉证据

Chrome 中读取调整后 iframe 的 computed style：

```text
pending-order-card border-left: 3px solid rgb(22, 119, 255)
pending tag background: rgb(255, 255, 255)
pending tag border: 1px solid rgb(22, 119, 255)
pending tag color: rgb(22, 119, 255)
```

实际页面截图：

![已保存的 staffrnapp 项目设计体系](./project-design-system-validation.jpg)

## 5. 保存与持久化

点击“保存为项目设计体系”后：

- 数据库中 package slot 为 `saved`；
- saved digest 与调整后的 draft digest 完全一致；
- source task 为 `d48f91d2-4084-47fb-a203-bf8ba81c96db`；
- 项目下 `project_design_system` 数量仍为 `1`；
- 页面显示“已保存”，不显示“有未保存更改”；
- 浏览器刷新后规则、Tokens 和 UI Kit 样式仍存在；
- 返回设计中心，切换到“设计体系”，再点击“打开设计体系”，仍读取相同内容和 computed style。

第一阶段保存语义是把当前内容转为 `saved` slot，而不是长期同时保留两份相同的 `draft` 与 `saved` 行。

## 6. 失败保护验证

为避免触碰真实业务数据，失败测试使用独立数据库：

```text
multica_task9_validation_a6d4
```

迁移命令：

```bash
DATABASE_URL='postgres://multica:multica@127.0.0.1:5433/multica_task9_validation_a6d4?sslmode=disable' go run ./cmd/migrate up
```

### 6.1 缺少必需文件

```bash
go test ./internal/daemon \
  -run '^TestReadProjectDesignSystemArtifactsRejectsMissingFile$' \
  -count=1
```

结果：通过。daemon 在读取输出目录时拒绝缺失文件。

```bash
go test ./internal/handler \
  -run '^TestCompleteProjectDesignSystemTaskRejectsMissingArtifactPayload$' \
  -count=1
```

结果：通过。Server 返回 `project_design_system_invalid_artifacts`，任务失败且不创建 draft。

### 6.2 不安全 HTML

```bash
go test ./internal/handler \
  -run '^TestCompleteProjectDesignSystemTaskRejectsUnsafeHTMLWithoutReplacingDraft$' \
  -count=1
```

结果：通过。包含 `<script>alert(1)</script>` 的 `components.html` 被拒绝，原 draft 与 saved digest 均保持不变。

### 6.3 失败、取消和超时

```bash
go test ./internal/handler \
  -run '^TestProjectDesignSystemFailureAndCancellationPreserveExistingPackage$' \
  -count=1
```

结果：通过。daemon failure、单任务取消、Agent 批量取消、sweeper timeout、事务后失败处理和取消广播共 6 条路径均保留原 draft/saved package。

### 6.4 Agent 离线

```bash
go test ./internal/handler \
  -run '^TestCreateProjectDesignSystemRequiresExplicitReadyAgent$' \
  -count=1
```

结果：通过。Server 对离线 Agent 返回 HTTP 409 与 `agent_unavailable`，且不会创建 `project_design_system`。

```bash
corepack pnpm --filter @multica/views exec vitest run \
  designs/project-design-system-create.test.tsx \
  -t 'preserves every field when the selected agent becomes unavailable'
```

结果：1 个测试通过。Agent 变为不可用后，Agent 选择、平台、设计目标、品牌色、参考链接、参考设计和 Figma UI 规范选择均保留。

失败测试完成后再次查询真实数据库，saved digest 仍为 `829b6988...e0fc`，系统没有 active task 或 last error；真实 Chrome 刷新后也仍显示相同规则与视觉效果。

## 7. 自动化套件与失败分类

最终回归结果：

| 范围 | 结果 |
| --- | --- |
| `@multica/core` | 52 files / 495 tests 通过 |
| `@multica/views` | 130 files / 1228 tests 通过 |
| TypeScript typecheck | 6 / 6 tasks 通过 |
| `internal/projectdesignsystem` | 通过 |
| `internal/service` | 通过 |
| `internal/daemon` | 通过 |
| `internal/daemon/execenv` | 通过 |
| `internal/handler` | 完整 package 通过 |

`internal/handler` 首次完整回归暴露了 5 个当前工作区失败。它们不能被草率标记为历史基线，因此使用固定到 `2ab1af3b66bb1d75191c80999f500e95bb167ad3` 的干净 detached worktree 和同一隔离数据库完成了分类：

- 干净 `HEAD` 中已有的 `TestParseDesignSystemProfileAnalyzeOutputRequiresStrictContract` 通过，确认其失败来自当前未提交改动；
- 另外 4 个语义 UI 草稿测试在干净 `HEAD` 中尚不存在，属于当前新增链路内部不一致；
- 根因分别为测试把未知字段放错到 `design_plan`、审批重编译丢失原始需求 ID、语义畸形的 advisory profile 未进入恢复路径，以及 stale 测试对 nil asset map 的错误假设；
- 修正后原 5 个测试、完整 `internal/handler` 以及全部目标 Go package 均通过。

GitNexus 对整个未提交工作区报告 `56 files / 529 symbols / 45 flows`、风险 `critical`。这是多阶段累计脏工作区的聚合结果，不是本次小批修正的独立风险结论；本次修改前的 symbol impact 为 LOW，复用测试夹具为 MEDIUM。结构检查在已索引的当前 `HEAD` 中报告 10 个既有循环依赖，均不涉及 `server/internal/projectdesignsystem` 或项目设计体系 handler。当前工作区因此仍不适合直接整体提交。

## 8. 剩余验证边界

以下两项没有在真实业务环境中做破坏性注入，不能描述为真实现场已演练：

1. 当前只有一个在线可选 Agent。为了不停止用户正在使用的 daemon，未在真实 Chrome 提交瞬间强制 Agent 离线；前后端确定性测试已覆盖 `agent_unavailable` 和表单保留。
2. 缺失文件和不安全 HTML 已使用隔离数据库中的真实完成回调代码路径验证，但没有诱导真实模型故意生成恶意产物。

这两项不影响第一阶段正常生成、调整、保存和确定性失败保护的验收结论，但仍是后续生产演练需要保留的风险说明。
