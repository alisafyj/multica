# Native Design 收口执行交接提示词

将下面代码块完整粘贴到新的 OpenCode 会话。

```text
继续执行 Multica Native Design Phase 1 收口与旧 Open Design/V1 链路删除工作。

工作区：/Users/fengyujie/Documents/soyoung/multica
目标分支：feature/fengchen
规划时 HEAD：b91c8f9ee

先恢复事实，不要立即编辑：
1. 读取 CLAUDE.md。
2. 读取 docs/product/design-center/README.md。
3. 读取 docs/superpowers/specs/2026-08-12-native-design-phase-1-closure-and-legacy-removal-design.md。
4. 读取 docs/superpowers/plans/2026-08-12-native-design-closure-execution-summary.md。
5. 读取 docs/superpowers/plans/2026-08-12-native-design-phase-1-low-token-closure.md。
6. Phase A 完成并获批前，不执行 docs/superpowers/plans/2026-08-12-open-design-v1-destructive-removal.md。

当前状态：
- 只有规格和计划文档新增，尚未提交。
- 没有实现、迁移、配置或测试改动。
- 不得清理、覆盖或回退用户工作树改动。
- 不得 push 本地 main。
- Oracle Gate 1 的 11 项风险已写回计划；最后两处纯计划矛盾已直接修正并通过 git diff --check。用户已要求结束旧会话，因此没有第三轮 Oracle 复审。不要再扩展规划，直接按 Phase A 详细计划执行。

执行模式：
- 使用 subagent-driven-development 执行 Phase A，writer scope 必须明确且不重叠。
- 行为变化必须使用 test-driven-development。
- 完成声明前使用 verification-before-completion。
- 遇到失败先使用 systematic-debugging。
- 编辑任何 function/class/method 前刷新 GitNexus 索引并执行 upstream impact；HIGH/CRITICAL 必须先向用户警告。
- 每个 commit 前执行 detect_changes。除非用户明确要求，不要 commit 或 push。

Phase A 目标：
- 固定 V2 archive 拒绝旧 schema 的合同。
- 修复 Native package Preview 当前 immutable cache，使 capability 资源 no-store，并验证 media type/nosniff。
- 验证 capability scope/expiry。
- 验证 legacy env unset 时 daemon config 可加载，create/adjust/regenerate 可入队 V2 context 且不创建 open_design_run。
- 独立验证 daemon collect/Audit/Preview/upload/completion finalizer。
- 增加首次 Native V2 draft discard 返回 unestablished 的测试。
- 重跑坏包不覆盖 draft/saved、adjust base digest、save/discard 和 persistence 套件。
- 更新三份产品证据文档，只写实际结果。

关键限制：
- 不运行真实 CRM Agent，不访问用户 Agent CLI，不消费模型额度。
- 不把受控 Preview 称为用户 Chrome 验收。
- 不宣称真实仓库 grounding、严格 Task 8、完整验收或 full acceptance。
- handler config/enqueue/context/no Run 与 daemon finalizer 是两组独立证据，不能拼成端到端成功声明。
- Phase A 不删除 Worker、V1、open_design_run、旧 API 或历史数据。

立即动作：
1. git status --short、git branch --show-current、git rev-parse --short HEAD。
2. 创建执行 todo，第一项是 Phase A Task 0 基线与 GitNexus 刷新。
3. 按 Phase A 计划逐 Task 派发和验证，不再写新的总体计划。
4. Phase A 完成后执行 Oracle Gate 2，并向用户请求是否批准进入 Phase B。

Phase B 的不可变边界：
- 用户已批准永久丢弃历史 OpenDesign/V1 数据，不迁移到 V2。
- migration 877 down 是注释 no-op；无 FK/CASCADE。
- 自动阻断 legacy/V2 object key 重叠。
- cleanup tool 保留到生产 rollout 完成。
- 877 必须离线切换，禁止旧/新 server rolling overlap。
- 详细执行只服从 Phase B 计划，不凭本提示词自行简化。
```

## 交接文件状态

该提示词反映 2026-08-12 规划结束时状态。新会话必须以实际 `git status`、当前 HEAD 和权威文档为准；若工作树已经变化，先判断变化来源，不得覆盖未知改动。
