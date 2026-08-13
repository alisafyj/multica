# Native Design Phase 1 收口与旧链路删除执行摘要

## 目标

分两个不可颠倒的变更集完成：

1. Phase A：以固定 Native V2 fixture、受控 Preview 和数据库断言完成低令牌自动化收口。
2. Phase B：彻底删除 Open Design Worker、V1 执行/读取/Preview、`open_design_run` 和历史旧数据，使 Native V2 成为唯一活动链路。

Phase A 不运行真实 CRM Agent，不验证真实仓库 grounding 或用户 Chrome。因此不能称为原 Task 8 的严格验收、完整验收或 full acceptance。

## 权威文档

- 规格：`docs/superpowers/specs/2026-08-12-native-design-phase-1-closure-and-legacy-removal-design.md`
- Phase A 详细计划：`docs/superpowers/plans/2026-08-12-native-design-phase-1-low-token-closure.md`
- Phase B 详细计划：`docs/superpowers/plans/2026-08-12-open-design-v1-destructive-removal.md`
- 原 Native 方案：`docs/superpowers/specs/2026-08-05-multica-native-design-engine-design.md`
- 产品事实源：`docs/product/design-center/README.md`

详细计划是执行时的事实源；本文件只用于快速恢复范围和顺序。

## 当前状态

- 分支：`feature/fengchen`
- 规划时 HEAD：`b91c8f9ee`
- 尚未开始实现，没有源码、迁移、配置或测试改动。
- 当前新增内容仅为规格和计划文档，尚未提交。
- 禁止 push 本地 `main`。
- Oracle Gate 1 共发现 11 项风险；均已写回计划。最后复审剩余的两处计划矛盾也已直接修正并通过 `git diff --check`，但按用户要求停止执行，未再发起第三轮 Oracle 复审。

## Phase A 执行摘要

### 必做变更

1. 刷新 GitNexus 索引；编辑生产 symbol 前执行 upstream impact，高/严重风险先告知用户。
2. 增加 V2 archive 拒绝旧 schema 的固定合同测试。
3. 增加 Native package Preview 的 media type、`no-store`、`nosniff` 测试，并修复当前一年 immutable cache。
4. 增加 Preview capability scope/expiry 测试。
5. 证明 Open Design 环境变量 unset 时 daemon config 可加载，create/adjust/regenerate 可入队 V2 context 且不创建 `open_design_run`。
6. 单独重跑 daemon collect -> Audit -> Preview -> upload -> completion finalizer；不得把它和 handler 入队证据合并成端到端结论。
7. 增加首次 Native V2 draft discard 后返回 `unestablished` 的精确测试。
8. 重跑 completion failure、draft/saved 隔离、adjust base digest、save/discard 和 persistence 套件。
9. 更新 validation、README 和 decision register，只记录实际执行证据与未验证项。

### Phase A 完成门禁

- focused 与 broad checks 实际通过，无 `no tests to run`。
- 测试未执行用户 Agent CLI，未设置 real-agent smoke 开关。
- 文档明确区分 handler enqueue、daemon finalizer、受控 Preview 与真实现场验收。
- Oracle Gate 2 审核实际证据后，用户/发布负责人明确批准进入 Phase B。

## Phase B 执行摘要

### 顺序

1. 把四个 V2 Preview token helper 移到中性 ownership，保持 wire contract 不变。
2. 先增加旧路由缺席/404、旧 schema 拒绝、V2 Preview 独立和 migration 877 合同测试。
3. 创建并保留 `server/cmd/legacy-design-cleanup`；枚举旧对象和全部 V2 对象 key，发现重叠自动停止。
4. 停写、drain tasks/runs、备份、执行清理 dry-run 与真实对象删除；对象清理失败时不得执行 migration。
5. 添加 migration 877：删除非 V2 package、旧 indexes 和 `open_design_run`，把 package CHECK 收紧为 V2-only；down 只能是不可逆注释 no-op。
6. 删除 Open Design SQL/sqlc、cleanup CTE、Worker daemon/client/config/V1 prompt/artifacts。
7. 删除 handler/service V1/Run、`internal/opendesign`、旧 API 和历史兼容 Preview。
8. 删除前端旧 schema/alias/config，保留 V2 package Preview、upload、verification 和当前 UI。
9. 使用独立 fresh/upgrade 数据库验证 rollback、replay、V2 字段不变及旧-only 项目 `unestablished`。
10. 完成 Go/TS、migration、clean grep、五份产品文档和 Oracle Gates 3-5。

### 破坏性边界

- 历史 OpenDesign/V1 Run、package、archive、evidence、Preview 与三文件永久丢失，不转换到 V2。
- migration 877 不能与旧 server binary 滚动重叠。
- 发布必须离线切换：停写和 drain -> 停全部旧 replicas/migrators -> retained cleanup tool -> 877 -> 只启动新 binaries -> V2 smoke -> 恢复写流量。
- 清理工具必须保留到生产 rollout 完成；删除只能是后续单独批准的 release。
- 877 开始后只能从 DB+object backup 灾难恢复或向前修复，不能依赖 down。

## 验证与提交规则

- 行为变化遵循 TDD；先观察 RED 或明确记录 characterization 已 GREEN。
- 每个生产 symbol 编辑前执行 GitNexus impact。
- 每个 commit 前执行 `git diff --cached --check` 和 GitNexus `detect_changes({scope: "staged"})`。
- 不运行用户安装的 Agent CLI；真实 smoke 必须另行显式授权。
- Phase B 的 Tasks 6-11 虽可分内部 commit，但只能作为一个不可拆分 release artifact 发布。
- 未收到明确 commit/push 请求时，不提交、不推送。

## 第一动作

新会话只开始 Phase A。先读取 `CLAUDE.md`、产品事实源、规格和 Phase A 详细计划，确认工作树，再加载 `subagent-driven-development`、`test-driven-development`、`verification-before-completion` 和 GitNexus impact 流程。不要提前修改 Phase B 文件。
