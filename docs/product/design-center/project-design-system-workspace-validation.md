# 项目设计体系工作区 Task 5 验证记录

> 验证日期：2026-07-30
> 当前结论：创建、真实 Agent 生成、静态校验、浏览器渲染校验、首次保存、局部调整隔离和放弃恢复均已通过真实 Chrome 与数据库验证。本文只记录本次可复现证据，不以 Agent task 的 `completed` 作为成功依据。

## 1. 验证对象

| 对象 | 值 |
| --- | --- |
| Project | `Design` (`79560402-5bd7-420a-9e16-79e06557507a`) |
| Design System | `317ac5d7-00b8-4abd-b4ce-df2ed9f695de` |
| Agent | `Local UI Restore Agent` (`6ef23397-12b3-4857-adca-a76afbff8b40`) |
| Runtime | `4f381116-786f-486f-ab92-848631808c82` |
| 数据库 | `multica_recovered_20260729`，PostgreSQL `5433` |
| Chrome 页面 | `http://localhost:3031/amc/designs` |
| 后端 | `http://localhost:8080`，验证时 `/health` 返回 `{"status":"ok"}` |

## 2. 十项真实链路证据

| # | 验收点 | 证据 |
| --- | --- | --- |
| 1 | 项目 Tab 直接进入创建工作台 | 在 Chrome 的 `Design` 项目中选择“设计体系”后，未建立态直接展示创建输入；没有摘要列表或二级详情入口。 |
| 2 | Agent、平台、brief 和 references 进入任务上下文 | 首次任务 context 持久化了 `agent_id=6ef23397...`、`platform=web`、完整中文 brief、`references=[]`、Project 与 Workspace ID，以及三份必需产物和禁止脚本的 output policy。 |
| 3 | 真实执行状态与消息实时变化 | Chrome 保持在当前页面时，任务从 queued/running 原地更新至产物校验；没有手动刷新。数据库保存了 48 条任务消息，其中 21 条 tool use、21 条 tool result，时间跨度为 `03:49:24Z` 至 `03:56:50Z`。 |
| 4 | task 完成后存在三份非空产物 | 首次任务 `57ec9b56-6fac-4799-a438-e4926443c94e` 完成后，`DESIGN.md=6034`、`tokens.css=4754`、`components.html=39017` 字节，总计 `49805` 字节；Server 静态校验通过。 |
| 5 | UI Kit 非空且回执匹配 | 浏览器可信预览桥返回 `77/77` 可见 locator、`1299x2200` body、`0` 失败图片、digest matched、static validation passed。只有该回执被 Server 接受后，候选内容才进入可用草稿状态。 |
| 6 | 成功回执后才能保存 | 首次生成的 package digest 为 `527dcc18a95056b7cc5f0815207846d688fd7f862c7ce3e0b215f4a7ba30854a`；渲染状态为 `passed` 后页面才提供“保存为项目设计体系”。 |
| 7 | 局部调整按需打开并保留 scope | 调整范围为 `{"kind":"token_group","id":"button"}`，要求只把主按钮圆角从 `6px` 改为 `8px`。任务 `91027a86-651b-4d1b-afa0-8505e948ff2c` 实际读取三份 base 文件并输出三份完整替换文件。 |
| 8 | 首次保存原子形成 saved | 首次保存后 saved digest 与验证通过的 draft digest 同为 `527dcc18...0854a`，source task 仍是首次生成任务；系统保持一套当前设计体系。 |
| 9 | 后续调整不污染 saved | 调整候选 digest 为 `9008194548e84ec53bf33b7ab391dc664df10194f489a2cfc716869316255243`，页面可见 `--button-primary-radius: 8px`；调整期间 saved digest 始终保持 `527dcc18...0854a`。 |
| 10 | 放弃调整精确恢复 saved | 在 Chrome 的更多菜单中确认“放弃本次调整”后，页面恢复“已保存”，不再显示未保存更改，新增的 Button Token 消失。数据库只剩一个 `saved` slot，数量为 `1`，digest 精确恢复为 `527dcc18...0854a`，active task、active operation 和 last error 均为空。 |

## 3. 离屏自动渲染复验

最初的可信预览桥在布局尚未稳定时测量，曾出现 `empty_body`；改用 `requestAnimationFrame` 后，又因 Chrome 对离屏 iframe 的节流出现 `measurement_failed`。最终实现不再依赖 animation frame：字体和图片完成后等待 `100ms`，只对 `empty_body` 与 `no_visible_locator` 做最多 8 次、每次 `100ms` 的有界重试。

最终复验期间，Chrome 保持在 Button 章节，未滚动到在线 UI Kit，也没有点击“重新验证预览”。调整任务完成后，候选 package 首次自动落为：

```json
{
  "digest": "9008194548e84ec53bf33b7ab391dc664df10194f489a2cfc716869316255243",
  "source": "trusted_preview_bridge",
  "accepted": true,
  "bridge_status": "ready",
  "locator_count": 77,
  "visible_locator_count": 77,
  "expected_locator_count": 77,
  "body_width": 1299,
  "body_height": 2200,
  "image_count": 0,
  "failed_image_count": 0,
  "digest_matched": true,
  "static_validation_passed": true
}
```

这证明离屏 iframe 不需要用户滚动或手动重试即可完成校验。Server 当前保存最终 render report，不保存每次内部布局测量；内部重试因此保持在可信桥中，并且只发出一个终态回执。

## 4. Chrome 视觉证据

放弃测试调整后，页面显示“已保存”，保存按钮不可用，在线 UI Kit 仍正常展示：

![项目设计体系工作区已保存状态](./project-design-system-workspace-validation.jpg)

## 5. 聚焦自动化

```text
go test ./internal/projectdesignsystem ./internal/handler ./internal/service -count=1
  3 packages passed

pnpm --filter @multica/core exec vitest run \
  api/client.test.ts \
  api/schemas.test.ts \
  realtime/use-realtime-sync-ws-instance.test.tsx
  3 files / 74 tests passed

pnpm --filter @multica/views exec vitest run \
  designs/designs-page.test.tsx \
  designs/project-design-system-preview.test.tsx \
  designs/project-design-system-canvas.test.tsx \
  designs/project-design-system-workspace.test.tsx \
  designs/project-design-system-create.test.tsx \
  designs/project-design-system-page.test.tsx
  6 files / 44 tests passed

pnpm typecheck
  6 tasks passed

git diff --check
  passed
```

## 6. GitNexus 影响范围

`detect_changes --scope compare --base-ref main` 对当前分支与工作区的聚合结果为 `125 files / 1940 symbols / 88 flows / critical`；仅未提交改动仍为 `72 files / 607 symbols / 63 flows / critical`。这些数字包含当前分支领先 `main` 的提交和多阶段累计脏文件，不能解释为本次两文件预览修复的独立影响。

对 `BuildPreviewHTML` 的上游分析同样给出 `CRITICAL`，但明确的一级调用者只有项目设计体系响应构建路径。本次代码变化因此只落在可信预览桥与对应测试，并通过项目设计体系 handler、前端工作区和真实 Chrome 链路覆盖。当前不提交整个工作区。

## 7. 当前边界

- 本记录证明第一阶段项目设计体系工作区的真实创建、生成、校验、保存、调整隔离和放弃恢复闭环。
- UI Agent 设计草稿的首次生成和再次调整任务已在后续独立阶段接入统一 Design Context Resolver；设计还原尚未接入。
- 旧 `design_system_profile_id` 仍是 PageSpec 编译器定位 RecipeSet 的内部兼容键，但旧 Profile JSON 已不再进入 UI Agent Prompt。
- 当前工作区包含多阶段累计改动，本文不代表整个脏工作区已经适合一次性提交。

## 8. UI Agent Design Context 自动化接入证据

2026-07-30 的后续小阶段完成了以下自动化验证：

- 首次生成任务持久化 `multica.design-context/v1` 快照，来源为 `cloud_saved_project_design_system` 时包含 saved package 的摘要和三份完整产物，并且不包含旧 `design_system` Profile JSON；
- 再次调整任务重新解析当前有效 saved package，固定新的来源与摘要，同样不暴露旧 Profile JSON；
- daemon Prompt 明确采用 `cloud_saved_project_design_system > local_design_md > repository_reality`，云端内容读取 `DESIGN.md`、`tokens.css` 和 `components.html`，没有云端 saved 时才交由本地 Agent 读取本地规则和仓库现实；
- UI draft 创建/调整 handler 聚焦测试、UI draft Prompt 测试、service/projectdesignsystem 回归和 `go vet` 均通过。

本节只证明任务快照与 Prompt 契约，不证明真实 Agent 已遵循上下文，也不证明生成页面的视觉质量。后端尚未为本次改动重启，真实任务和浏览器验收留在下一小阶段。
