# Phase A 真实产品 Gate 验收记录

日期：2026-09-01
结论：**PASS**。在隔离的本地 PostgreSQL、Task 7 API/Web、专用 runtime 与真实 headless Chromium 上完成；没有 mock、直接数据库写入产品对象或 HTTP-only 替代验收。

## 真实闭环

- 经真实认证流建立隔离测试身份，未读取或输出任何 cookie、JWT、验证码、令牌或其他秘密。
- 真实 UI 创建/选择 `Dev`、CRM 验收 Project、关联的 `alisafyj/multica` 仓库与在线 `Design Gate Agent`。
- repository analysis 产出真实 checkout 的固定提交 `a7606af71f98` 和仓库相对 source grounding。基于该分析生成、Audit/Preview 并保存仓库专属设计体系。
- Design A（客户列表）和 Design B（客户详情/商机工作流）均从同一精确已保存的仓库体系提交；服务端冻结为 `cloud_saved_repository_design_system`，并携带已保存 package/digest。两个详情 UI 均显示仓库取证与 `Audit 通过`，Preview iframe 均真实渲染，保存操作成功。
- 对已保存的 Design B 发起一次真实调整。调整生成 v2 草稿，保留同一 provenance，重新通过 Audit 与 Preview 后以“保存调整”成功保存为 v2。

## 浏览器证据

- 真实 Preview 在 `1440x960` 和受限 `500x900` viewport 检查；外层与 iframe 均无横向溢出。
- 两种 viewport 均验证主标题和调整后的“当前”状态可见；活动/历史标签点击后 `aria-selected` 更新；编辑资料打开可见表单/抽屉。
- 滚动目标进入视口后，`elementFromPoint` 命中“变更历史”标签或其后代节点，再执行真实点击成功。

## 政策限制

未调用 screenshot、录屏或 trace API。报告不采用 agent 自述的截图检查；视觉证据限制为 DOM、accessibility、computed layout、visibility、`elementFromPoint` 和真实交互状态。

## 真实缺陷修复

真实验收依次暴露跨域 bearer、repository checkout、V2 package binding、仓库体系 provenance、prompt contract、输出根、JS regex audit 和显式 revision binding 问题。所有修复均以失败测试/定向验证和独立聚焦提交完成，最终工作树不保留临时上传诊断或生成的 Next 类型漂移。
