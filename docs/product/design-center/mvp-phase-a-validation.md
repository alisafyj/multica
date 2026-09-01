# Phase A 真实产品 Gate 验收记录

日期：2026-09-01
结论：**BLOCKED**。本次没有把环境或测试替身包装为真实 Design Document 验收。

## 已验证的真实边界

- `make status`：Task 7 隔离 API 在 `9fde3d81c` 运行，隔离 PostgreSQL 已就绪；Web `13749` 可由控制器持有的前台会话返回真实页面。状态工具将该 listener 标为非本环境 owner，符合当前运行时已知的 launcher ownership false negative。
- 真实 headless Chromium 可启动并进入 Web 的 `/login`。执行期间未调用截图、录屏或 browser trace API；已用 DOM/可访问性文本和交互状态检查替代截图证据。
- 本机 Playwright 期望的 managed Chromium revision 不存在，但已存在的 Chromium headless binary 可以启动。这个工具版本不一致是环境关注项，不是 UI 通过证据。
- 复用的 `TestApiClient` 隔离测试身份可以通过真实 API 身份校验（仅用于登录前置，未输出或保存 cookie/JWT/密钥）。同一身份在当前 Web→API 浏览器会话的 `/api/me` 请求得到 `401`，Web 只显示需要一次性验证码的登录页。按隐私策略，未读取、请求或输出验证码。
- `make daemon` 被 daemon-managed-task guard 正确拒绝：当前任务不得重启、启动或改配人类本地 daemon。无 profile 的 `daemon status` 只能看到 Desktop 管理、连接另一服务端的 daemon，不能作为 Task 7 隔离 API 的 runtime。

## 未执行的 Gate 项

没有合法的同时满足“真实 API + 已认证 Web UI + Task 7 API 上 online runtime”的路径，因此以下项目均未启动：

- Workspace `Dev`、Project `真实 CRM 仓库设计体系与双设计稿端到端验收`、repository association `https://github.com/alisafyj/multica`、Agent `Design Gate Agent` 的重建；
- repository analysis、repository-specific saved design-system revision、Design A/B、调整、Audit、Preview、保存；
- desktop 与受限 viewport 的设计页面布局/可访问性断言、source/status/grounding/provenance 检查。

因此没有截图、任务耗时、调整次数、Audit/Preview receipt、Design Document revision 或保存指针可以记录；声称这些结果会是伪造验收。也没有复现可归因于产品代码的缺陷，故没有 TDD 修复提交。

## 可恢复步骤

1. 由拥有本地 daemon 生命周期权限的操作者，为 Task 7 API `18829` 启动并注册合法的在线 runtime；不要在 daemon-managed task 内绕过 guard。
2. 修复或提供可验证的 Web `13749` 到 Task 7 API 的浏览器认证路径。可复用 E2E 隔离身份仅做前置，但浏览器 cookie/session 必须使真实 `/api/me` 返回已认证状态，且不得读取验证码或现有凭据。
3. 重新从真实 UI/API 创建固定 fixture，依次完成 analysis、仓库专属体系保存、两个文档生成、至少一次调整、Audit/Preview、两份保存和两种 viewport 的非截图断言。
4. 仅在上述真实产物和 provenance 都可见后，将本报告更新为验收结果。

## 证据限制

运行时隐私政策禁止桌面和浏览器截图、录屏及 trace。本 Gate 恢复后仍应采用 DOM、accessibility tree、computed layout/visibility 与交互状态断言，并在报告中保留该视觉证据限制。
