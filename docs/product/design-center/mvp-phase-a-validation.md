# Phase A 真实产品 Gate 验收记录

日期：2026-09-01
结论：**PENDING UI EVIDENCE**。真实生成、调整、Audit、Preview 与保存闭环已完成，但 Group 3 Gate 要求的三项入口/视图证据仍需在真实 UI 补验，补齐前不得将 Task 7 记为最终 PASS。整个已完成闭环运行在隔离的本地 PostgreSQL、Task 7 API/Web、专用 runtime 与真实 headless Chromium 上；没有 mock、直接数据库写入产品对象或 HTTP-only 替代验收。

## 固定锚点与任务耗时

- Workspace `8b6efa4b-6d23-4a2e-b3dd-8b07c26d2a8c`；Project `bed12d68-a044-408b-9b94-0b746fad423e`；Repository association `d9a0a1c7-5df9-4466-85a4-ee0c01960cf7`；Agent `51248c1f-b7cf-4837-9c44-a96055d1a579`。
- repository analysis task `01a05cd2-08b0-700f-95ce-ae786742aed8`：`completed`，19:54:23 至 19:58:39，实际运行 255 秒；固定 checkout commit `a7606af71f98`。
- repository design system `edf027ca-4385-4109-a044-042505f02de9`；saved package `71ceba5f-ff03-40ae-a126-bf931dc454fc`；生成 task `01a05ce1-9b08-7ca6-abc6-51f64bb8de99` 运行 300 秒；saved digest `sha256:5fa50a865277c5405c89f9d2023398f88489b267591518d6961230041ddc4811`。
- Design A document `f2a055e5-ef1d-473c-bf4c-ae43aeaf851c`；v1 revision `e9703f6d-8edf-4015-8c07-f0a877c4f8c1`；task `01a05d3c-efbc-7226-b8c8-0e005b1f9078` 运行 716 秒；22:09:29 保存。
- Design B document `14f2fbb0-c448-46d3-82d5-f38d6497a0b9`；v1 revision `354fbb9b-e637-469d-9c15-0c70489768dd`；task `01a05d4e-3b6a-7e25-97d1-60fc584e54bc` 运行 849 秒。
- Design B 调整 task `01a05d5c-9f87-78cd-9cdd-9648f1e74e27` 运行 300 秒；v2 revision `e8160d12-081d-4e38-b6c3-3ad7dd879ff7` 以 v1 为 base，22:31:33 保存。总调整次数：1。
- A v1、B v1、B v2 的持久化 Audit 均为 `passed=true`、0 diagnostics，Preview verification 均为 `passed=true`；三个 revision 的 `design_system_digest` 均精确等于上述 saved design system digest。

## 真实闭环

- 经真实认证流建立隔离测试身份，未读取或输出任何 cookie、JWT、验证码、令牌或其他秘密。
- 真实 UI 创建/选择 `Dev`、CRM 验收 Project、关联的 `alisafyj/multica` 仓库与在线 `Design Gate Agent`。
- repository analysis 产出真实 checkout 的固定提交 `a7606af71f98` 和仓库相对 source grounding。基于该分析生成、Audit/Preview 并保存仓库专属设计体系。
- Design A（客户列表）和 Design B（客户详情/商机工作流）均从同一精确已保存的仓库体系提交；服务端冻结为 `cloud_saved_repository_design_system`，并携带已保存 package/digest。两个详情 UI 均显示仓库取证与 `Audit 通过`，Preview iframe 均真实渲染，保存操作成功。
- 对已保存的 Design B 发起一次真实调整。调整生成 v2 草稿，保留同一 provenance，重新通过 Audit 与 Preview 后以“保存调整”成功保存为 v2。

## 尚待真实 UI 补证

- 从 Repository 入口打开创建流程，再从 Home 入口打开同一流程，确认两者进入同一个已预填的表单；当前没有可引用的真实 UI 观测记录。
- 在同一 viewport 并排或逐项核对 Design A 与 Design B 的视觉/组件一致性；共享同一 frozen digest 只能证明约束来源一致，不能替代视觉一致性验收。
- 分别进入 Project view 与 Repository view，确认 source/status/grounding/provenance 的用户可见展示；已有 Design Document 详情页证据不能代替这两个入口的证据。

## 浏览器证据

- 真实 Preview 在 `1440x960` 和受限 `500x900` viewport 检查；外层与 iframe 均无横向溢出。
- 两种 viewport 均验证主标题和调整后的“当前”状态可见；活动/历史标签点击后 `aria-selected` 更新；编辑资料打开可见表单/抽屉。
- 滚动目标进入视口后，`elementFromPoint` 命中“变更历史”标签或其后代节点，再执行真实点击成功。

## 政策限制

未调用 screenshot、录屏或 trace API。报告不采用 agent 自述的截图检查；视觉证据限制为 DOM、accessibility、computed layout、visibility、`elementFromPoint` 和真实交互状态。

## 真实缺陷修复

真实验收依次暴露跨域 bearer、repository checkout、V2 package binding、仓库体系 provenance、prompt contract、输出根、JS regex audit 和显式 revision binding 问题。所有修复均以失败测试/定向验证和独立聚焦提交完成，最终工作树不保留临时上传诊断或生成的 Next 类型漂移。

JS audit 的初始 slash heuristic 随后被审查发现会混淆正则与除法；`2ada8b972 fix(designs): audit scripts from parsed syntax` 已改为 grammar-aware AST 检查，并以控制语句后的正则接受用例和除法中的 `fetch` 拒绝用例覆盖。
