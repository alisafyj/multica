# 原生设计 Phase 1 收口与旧链路移除方案

> 日期：2026-08-12
>
> 状态：`superseded`
>
> 替代方案：[Native Design 产品切片演进与渐进清理方案](./2026-08-12-native-design-slice-driven-evolution-design.md)
>
> 历史说明：本文件中的 Phase A 自动化证据要求继续有效；独立、一次性的 Phase B 破坏性移除路线已取消，不得继续按本文件第 5 节及其后续删除方案实施。
>
> 原执行顺序：Phase A 完成并留证后，才可进入 Phase B
>
> 原数据策略：允许永久删除历史 OpenDesign/V1 Run、归档、预览与三文件包，不迁移到 Native V2

## 1. 决策摘要

本方案批准两个连续但独立的变更集：

1. **Phase A：低令牌 Phase 1 收口。** 对 Native V2 做安全与质量复核，使用自动化测试、固定 fixture 和受控 archive 验证包、Audit、Preview、草稿隔离、调整、保存、放弃，以及新任务不创建 `open_design_run`。据实更新验证文档。
2. **Phase B：彻底移除旧 OpenDesign/V1 链路。** 删除 Worker HTTP/SSE client、daemon supervisor 与分支、路由和配置、Run 准备与持久化、V1 三文件生成/完成/响应/预览、历史兼容读取，以及 `open_design_run` 表、查询、生成代码、fixture 和测试。只保留 Native V2。

Phase A **不运行真实 CRM Agent 生成，不验证用户本机 Chrome grounding**。因此它不能证明 Agent 已读取真实 CRM 仓库，也不能证明 UI Kit 与 CRM 的颜色、字体、密度、组件和页面模式在用户 Chrome 中一致。原计划 Task 8 只能记录为“低令牌自动化收口”，**不得称为严格验收、完整验收或 full acceptance**。

## 2. 决策关系

本决定延续 DC-039 的目标架构：Open Design 只作为行为与证据基线，正式执行只使用 Multica Native V2。

DC-039 中“现有数据和代码先隔离、原生链路稳定后再单独清理，不执行破坏性迁移”是迁移初期的临时保留条款。本次用户已经单独批准破坏性清理并接受历史数据丢失，故本决定**取代该临时保留条款**；DC-039 的原生架构、质量门禁和产品语义不变。后续更新决策台账时必须保留 DC-039 原文，并新增一条已确认决策记录此次替代关系，不得回写历史。

原 Phase 1 计划中的“历史 V1/OpenDesign 可读”“历史 archive 可原生转换”和“回滚到 V1”不再是最终验收要求。它们只在 Phase A 尚未进入 Phase B 时短暂成立；Phase B 合入后由“旧接口不可用、旧数据已删除、V2 独立工作”替代。

## 3. 目标与原则

- 以可复现自动化证据诚实关闭 Phase 1 的非现场验证部分。
- 让生成、调整、Audit、Preview、draft/saved 生命周期完全独立于 OpenDesign Worker 和 V1。
- 删除旧执行能力、旧读取兼容和旧持久化，不保留双轨、fallback、转换器或隐藏 feature flag。
- 保留 Open Design 的研究文档与历史证据文本；删除的是活动代码、活动配置、数据库数据和运行时兼容能力。
- 任何失败都 fail closed：不产生坏草稿，不覆盖 draft/saved，不回退到 V1 或 Worker。

## 4. Phase A：低令牌 Phase 1 收口

### 4.1 安全与质量复核

复核 Native V2 的实际信任边界，并把发现转为测试先行的修正：

- archive 收集拒绝符号链接、硬链接、绝对路径、路径穿越、重复或未声明文件、摘要不一致、文件数量和大小超限；
- `manifest.json`、artifact index、object key、content digest、task、project、design system、输入快照和 base digest 必须一致；
- HTML/CSS 禁止脚本、事件属性、表单、嵌入页面、远程 URL、CSS `@import` 和包外资源；Preview 必须禁网并验证真实可见内容；
- Audit 或 Preview 失败、取消、超时、对象存储失败和完成回调重放均不得替换现有 draft/saved；
- 调整必须绑定固定 base digest，生成完整 V2 替换包；局部 scope 只缩小 Agent 关注范围，不允许局部不一致包；
- save 必须原子推进当前 draft，discard 必须恢复最后 saved；首次无 saved 时 discard 回到未建立状态；
- V2 create/adjust/regenerate 的 handler enqueue/context 必须含 `multica.project-design-system/v2`，且不得含 `open_design_run` 或读取任何 `MULTICA_OPEN_DESIGN_*` 配置；这项 handler 证据只证明入队、context 和零 Run，不证明 daemon finalization 或端到端 Agent 执行。daemon 的 collect、Audit、Preview、upload、completion 顺序由独立 finalizer 测试证明。

### 4.2 低令牌证据范围

使用固定、可审计的 V2 fixture 和 archive 驱动真实生产解析、校验、持久化与 Preview 代码路径，覆盖：

- 合法 V2 package 的 manifest/index/digest 重算和对象归档读取；
- 坏路径、坏摘要、越权绑定、不安全 HTML/CSS、外连资源、空白或不可见 Preview 的拒绝；
- Audit 通过与拒绝回执；
- create 形成隔离 draft，adjust 正确绑定 base digest 并替换完整包；
- save 后刷新读取相同 digest，后续有效 draft discard 后恢复 saved；
- 无 saved 的首次 draft discard 后不留下有效体系；
- 无效生成或调整对既有 draft/saved 做字节级不变断言；
- 新 create/adjust/regenerate 在全部 Open Design 环境变量 unset 时仍可完成 handler 入队并生成 V2 context，且不创建 `open_design_run`；daemon finalizer 由独立 fixture/fake verifier/uploader 测试证明，二者不得合并表述为“环境变量 unset 时完整端到端成功”；
- V2 Preview API 和资源 API 的 workspace 权限、短期访问令牌、CSP、媒体类型与缓存策略。

fixture 只能证明确定性代码路径和受控渲染行为。它不能替代真实 Agent、真实 CRM 仓库来源、用户 Chrome 网络/控制台检查或人工视觉比对。

### 4.3 Phase A 文档结论

更新 `docs/product/design-center/project-design-system-validation.md`、`docs/product/design-center/README.md` 和 `docs/product/design-center/decision-register.md` 时，只记录实际命令、测试名、fixture/archive digest、数据库断言和观察到的失败。结论必须明确区分：

- 已通过：自动化安全、V2 包完整性、Audit、受控 Preview、draft 隔离、adjust/save/discard、无新 `open_design_run`；
- 未验证：真实 CRM Agent 生成、真实仓库 grounding、用户本机 Chrome 的视觉、Network 和 Console 证据；
- 状态：Task 8 的低令牌替代验证完成，但严格/完整 Task 8 验收未完成，不能据此宣称 full acceptance。

Phase A 不得为了制造“完成”结论复用 2026-07-29 的 V1 三文件现场证据，也不得把旧 Worker 的 OD 系列真实实验冒充 Native V2 现场证据。

## 5. Phase B：旧链路彻底移除

### 5.1 活动执行 API 与历史兼容 API

两类 API 必须分开处理，避免误删 Native V2：

**保留的活动 Native V2 API：** 项目设计体系 create、regenerate、adjust、save、discard、状态/详情读取，以及通用 V2 package Preview manifest、Preview 文件和受控资源读取。即使现有路径或内部 helper 暂含 `open-design-preview` 名称，也必须先收敛为中性的 V2 package API/命名；这些能力不能随旧兼容层一起删除。

**删除的 OpenDesign 执行 API：** daemon task 下的 `open-design/base-archive`、`preflight`、`start`、`events`、`archive`、`result`、`audit`、`preview`、`terminal` 全部路由、handler、请求/响应 contract 与客户端调用。这些接口只服务 Worker Run 生命周期，不提供 V2 能力。

**删除的历史兼容 API：** OpenDesign Run evidence 下载、历史 archive-backed Preview、V1 `components.html` Preview、旧三文件响应字段，以及为安装旧客户端保留的 `/open-design-preview` alias。旧客户端调用这些端点应得到标准 `404`，不返回空成功、不做 V2 猜测转换。

Phase B 后只接受 `multica.project-design-system/v2`。`multica.project-design-system/v1`、`multica.open-design-draft-package/v1`、`legacy` 和缺失 `package_schema` 均不再进入生成、完成、详情响应或 Preview 分支。

### 5.2 删除边界

必须完整删除：

- `server/internal/opendesign` 下 Worker client、SSE、preflight、supervisor、生命周期、callback、result collector、archive/evidence/draft package、旧 Preview verifier 及其测试；
- daemon 的 OpenDesign client、task 分支、supervisor 启停和 orphan recovery 分支；
- Server/daemon 的 OpenDesign routes、handler config、feature flag、Worker URL、artifact root、browser path、Release/commit/digest、adapter/model 等环境变量和 `.env.example` 声明；
- `prepareOpenDesignRun`、`persistOpenDesignRun` 及所有 create/enqueue/adjust 中的旧 Run 分支；
- V1 固定 `DESIGN.md`、`tokens.css`、`components.html` 生成提示、输出读取、inline completion payload、validation、API response 和 Preview fallback；
- 历史 OpenDesign/V1 package 读取、archive 转换、evidence 下载、兼容 alias、schema 分支与前后端类型；
- `open_design.sql`、sqlc 生成的 `OpenDesignRun` 模型与方法、workspace/project 删除中的 Run 清理 CTE；
- 只验证上述旧行为的单元测试、路由测试、fixtures 和 snapshots。

必须保留：

- `server/internal/projectdesignsystem` 的 V2 package contract、collector、Audit 和生命周期；
- `server/internal/designpreview` 或其等价通用 Preview 实现；
- V2 对象存储 archive、artifact index、digest、Preview receipt 和 package row；
- `project_design_system`、`project_design_system_package`、`agent_task_queue`、draft/saved 语义和当前设计中心界面；
- 通用 daemon Agent 执行、取消、超时、恢复和任务完成能力；
- `open-design-evidence.md` 等历史研究记录。文档中的历史名称不属于 clean grep 的活动符号失败。

如果通用 V2 代码仍从 `internal/opendesign` 导入 archive、令牌或 Preview helper，先用已有 V2 测试固定行为，再迁移到中性包并删除旧包；不得留下仅为保住 import 而存在的 OpenDesign 空壳。

## 6. 数据丢失与迁移

### 6.1 明确警告

Phase B 是不可逆的数据删除：

- 所有 `open_design_run` 行永久删除；
- 与历史 OpenDesign/V1 package 关联的 archive object、evidence ZIP、Preview 与三文件内容不迁移到 V2；
- 只拥有历史 V1/OpenDesign package、没有 V2 package 的项目在清理后显示为未建立设计体系；
- 同时存在 V2 与旧 package 的项目保留 V2 draft/saved，只删除旧 package 和旧对象；
- 历史链接、旧桌面客户端兼容入口和证据下载失效。

用户已经接受上述损失。上线前仍需输出待删除行数、project/design-system 数量和对象 key 清单，作为审计证据，不作为迁移或保留请求。预检还必须自动枚举全部 V2 `archive_object_key`，计算旧对象清单与 V2 key 的交集；交集非空时 fail closed，禁止删除对象和执行 migration。

### 6.2 迁移 877

使用新的 fork migration `877_drop_open_design_v1_legacy.up.sql`；`876` 是当前最高迁移号，因此 `877` 是下一个安全的 800+ 编号。不得编辑、重命名或复用已应用的 `870` 至 `873` migration。

迁移必须显式完成：

1. 删除 `project_design_system_package` 中 schema 为 V1、OpenDesign 或 `legacy` 的行，以及缺失 schema 的旧 package 行；V2 行不受影响；
2. 非 V2 兼容元数据只来自真实存在的两处：non-V2 `project_design_system_package` 行本身和 `open_design_run` 行；随这两类行显式删除，不发明第三张 metadata 表；
3. 显式 `DROP INDEX IF EXISTS` 旧 Run 索引；
4. 显式 `DROP TABLE IF EXISTS open_design_run`；
5. 将 package schema 约束收紧为仅允许 `multica.project-design-system/v2`。

不增加外键，不使用 `CASCADE`。应用层在执行 schema migration 前根据数据库对象 key 清单删除旧对象存储内容；对象删除采用可重试、幂等操作。对象清理未完成或发现 V2 object-key conflict 时不得执行数据库删除，否则会失去定位孤儿对象的依据。生产清理必须使用与 `Handler.Storage`/项目设计 package upload 相同的 adapter 选择：S3 优先、否则 local；不得使用 Qiniu `DesignAssetStorage`。

`877_drop_open_design_v1_legacy.down.sql` 必须是明确不可逆的**纯注释 no-op**，不得重建 table、constraint 或 index，不恢复任何数据或兼容代码。生产回滚不得依赖 down migration。

migration 验证必须使用与 `cmd/migrate` 等价的 runner 语义：877 任一语句失败时，本次 877 SQL 的前序变更全部回滚且 `schema_migrations` 不记录 877；SQL 已成功但 877 记录缺失时可安全重放，重放后 V2 rows 与 V2-only CHECK 不变。fresh 与 upgrade 分别使用两个唯一命名、相互独立且不与 handler/dev DB 共享的数据库，并记录 host、database、server identity 和 migration record 证据。

## 7. 上线、回滚与失败处理

### 7.1 上线边界

1. 先发布并验证 Phase A；Phase A 不删除数据，可按普通代码版本回滚。
2. Phase B 上线前停止接收设计体系写操作，等待活动相关 Agent task 终态，并确认没有旧 Worker Run 正在执行。
3. 备份数据库并导出待删除统计、旧 object key、全部 V2 object key 和冲突清单；备份只用于灾难恢复，不构成产品兼容承诺。保留 `server/cmd/legacy-design-cleanup` 源码于 Phase B release，并贯穿完成的生产 rollout；删除工具只能进入后续单独批准的 cleanup release。
4. 先幂等删除旧对象。随后执行**离线切换**：停止所有旧 server replicas 和所有 migration jobs，确认不存在旧二进制或并发 migrator；使用同时包含 Tasks 6-11 和 cleanup tool 的新 release 执行 migration 877；migration 成功后只启动新 binaries，完成 V2 smoke checks，最后恢复 V2 写流量。禁止旧/新 server rolling overlap。
5. fresh DB 与旧 schema 升级验证均通过后，才可宣布完成。升级后还必须从 API/status boundary 证明旧-only 项目为 `unestablished`、content 为空、`has_unsaved_changes=false`、`active_task=null`。

### 7.2 回滚边界

- migration 877 **之前**：Phase B 应用可以整体回滚，旧数据仍在。
- 旧对象删除或 migration 877 **开始后**：不支持应用内回滚到 V1/OpenDesign；只能从整库与对象存储备份恢复到迁移前版本，或继续修复 V2。不得运行旧二进制连接已清理数据库。
- Phase B 后续修复必须向前发布；不得重新引入 Worker、V1 parser、双读或隐式 fallback。

### 7.3 失败处理

- 预检发现活动旧 Run、未知 package schema、无法枚举对象或备份失败：停止 rollout，不删除任何数据。
- 对象删除部分失败：保留数据库记录和清单，重试；不得进入 migration 877。
- migration 失败：保持写流量和所有 server replicas 关闭；验证 runner 已回滚 877 的全部 SQL 且未记录 migration，再修复后重跑。若已提交则只做前向修复或灾难恢复。
- V2 create/adjust/Audit/Preview/save/discard 验证失败：不开放写流量，不以 V1 兜底。
- 旧 API 请求：返回标准 `404`；已安装旧桌面客户端预期看到 404/不可用，监控调用量以识别过期客户端，但不恢复兼容端点或 fallback。

## 8. TDD 与验证证据

所有行为变化先提交能失败的测试，再做最小实现并保留 RED/GREEN 命令输出。删除旧行为时，先增加“旧路由 404、旧 schema 拒绝、V2 无旧依赖”的替代契约测试，再删除只维护旧实现的测试。

证据路径：

- Phase A 与最终 Native V2 结果写入 `docs/product/design-center/project-design-system-validation.md`；
- 路线状态和严格验收缺口写入 `docs/product/design-center/README.md`；
- 本次决定对 DC-039 临时保留条款的替代关系写入 `docs/product/design-center/decision-register.md`；
- 旧 Worker 实验继续留在 `open-design-evidence.md`，加历史/非活动说明，不删除或改写证据；
- migration 预检统计、fresh DB、upgrade DB、对象清理和测试命令附在实施验证记录中，不写未经执行的“通过”。

最终验证至少包含：

- Native V2 后端 package/Audit/Preview/daemon/handler 定向套件；
- core/views 的 API schema、query/mutation 与设计体系交互测试及 TypeScript typecheck；
- fresh database 从零执行全部 migrations；
- 从包含 `open_design_run`、V1/OpenDesign package、旧索引和 V2 package 的旧 schema 执行升级，断言旧数据删除、V2 digest 与 slot 不变；
- 877 中间语句失败的全回滚、成功 SQL 缺失 migration record 时的安全重放，以及重放后 V2 rows/CHECK 不变；
- 升级后旧-only 项目的 API/status boundary 为未建立、无内容、无未保存变更和无 active task；
- V2 create、adjust、save、discard、Preview 的 API/数据库闭环；
- 旧 daemon 路由、evidence、历史 Preview alias 返回 `404`；
- clean grep：活动产品源码、SQL、生成代码、配置、测试和环境示例中不存在 Worker/V1/`open_design_run` 活动符号。Phase B release 中保留的 `server/cmd/legacy-design-cleanup` 是唯一运维工具例外，只允许枚举/删除旧对象和 fail-closed 预检，不得提供运行、读取兼容或 fallback。

clean grep 允许历史决策、研究证据、migration 870-873、migration 877 的显式删除、保留的 destructive cleanup tool 和本方案出现旧名称；不允许产品编译目标、运行配置、sqlc query、route、handler、fixture 或当前产品文档把旧链路描述为可用。cleanup tool 必须单独审计为 S3/local 对象删除工具。迁移 lint 必须继续接受已应用 migration 的历史引用。

## 9. 文档更新范围

实施完成时更新：

- `docs/product/design-center/project-design-system-validation.md`：区分 Phase A 自动化证据、未完成现场证据和 Phase B 最终 V2 回归；
- `docs/product/design-center/README.md`：当前能力只描述 Native V2，旧 Worker/V1 仅作为历史；
- `docs/product/design-center/decision-register.md`：新增已确认决策，明确取代 DC-039 的临时保留条款；
- `docs/product/design-center/open-design-engine-integration.md` 与 `open-design-evidence.md`：标记运行代码和数据已移除，但保留实验事实；
- 服务器/daemon 配置与内置 skill 文档：删除旧环境变量、路由和产物说明，只保留 V2 契约。

## 10. 非目标

- Phase 2：设计中心首页与统一任务发起器；
- Phase 3：Issue 在线设计稿生成；
- Phase 4：私有模板与社区模板；
- Phase 5：设计交付、MCP、还原和开发闭环；
- 真实 CRM Agent 生成或用户 Chrome grounding 的补验；
- 把历史 OpenDesign/V1 数据转换为 V2；
- 新建通用 `design_run`、版本历史、审核流或兼容服务；
- 改写、压缩或删除历史 Open Design 研究证据。

## 11. 验收标准

### Phase A

- 安全与质量复核发现均有测试、修正或明确风险结论。
- fixture/archive 自动化证明 V2 package、Audit、Preview、draft 隔离、adjust、save 和 discard。
- handler tests 证明 create/adjust/regenerate 不读取 Open Design flag、成功入队 V2 context 且不创建 `open_design_run`；daemon finalizer tests 独立证明 V2 package finalization，不把两类证据表述为端到端成功。
- 文档明确写出真实 CRM Agent 和用户 Chrome grounding 未验证，Task 8 不是严格/完整验收。

### Phase B

- Worker HTTP/SSE client、supervisor、daemon 分支、路由、配置和环境变量全部删除。
- prepare/persist Run、V1 三文件 generation/completion/response/preview 和历史兼容 API 全部删除。
- `open_design_run` query、sqlc 生成代码、模型、活动测试与 fixture 删除；migration 877 显式清理旧 package、索引和表，无 FK、无 `CASCADE`。
- fresh DB migration 与包含旧数据的 upgrade migration 均通过；升级后 V2 draft/saved、digest、archive 和 Preview 仍可用。
- 877 失败原子回滚与缺失 migration record 的安全重放通过；fresh/upgrade 使用独立唯一 DB，旧-only 项目升级后 API 状态为未建立且无活动内容。
- Native V2 create、adjust、save、discard、Preview 完整通过，失败路径不覆盖有效内容且不回退旧链路。
- 活动产品源码/config/SQL/tests clean grep 不再包含 Worker、V1 或 `open_design_run` 符号；只允许历史文档、已应用 migrations、877 显式删除和 retained destructive cleanup tool 保留必要文字。
- 旧执行与历史兼容端点均返回 `404`，保留的 V2 API 不使用旧 alias 或旧 schema fallback。
- 数据丢失统计、对象清理、迁移、测试和未执行项均有可核对证据，发布说明明确不可逆边界。

只有 Phase A 与 Phase B 各自验收全部满足，才能称“Native V2 独占运行链路与旧链路移除完成”；即使如此，也不得把本次工作描述为真实 CRM grounding 或原 Task 8 的严格/full acceptance。
