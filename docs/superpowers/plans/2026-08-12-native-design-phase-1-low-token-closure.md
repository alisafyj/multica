# Native Design Phase 1 低令牌收口实施计划

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** 以固定 Native V2 fixture、受控 archive、后端 Preview 和数据库断言完成 Phase A 安全/质量收口，补齐 legacy schema 拒绝、Preview 响应与 capability、create/adjust/regenerate 无 Open Design 环境依赖的契约证据，并据实更新权威文档。

**Architecture:** 保持现有 `multica.project-design-system/v2` 执行链不变：`projectdesignsystem` 负责 archive 与 Audit，`designpreview` 负责受控浏览器验证，handler 负责 Preview capability、完成门禁和 draft/saved 生命周期，daemon 负责收集、Audit、Preview 与上传。本计划只修复自动化证据揭示的 Phase A 缺口；复用现有 V2 completion、daemon、save/discard 和 persistence 套件，不删除或改写 OpenDesign/V1 链路。

**Tech Stack:** Go 1.26.1, PostgreSQL 17, sqlc, Chi, `archive/zip`, HMAC-SHA256 capability, existing object-storage test double, `chromedp`, Go testing, GitNexus.

---

## 范围与硬边界

- 权威规格：`docs/superpowers/specs/2026-08-12-native-design-phase-1-closure-and-legacy-removal-design.md`。
- 只执行规格 Phase A。Phase B 的 Worker/V1/`open_design_run` 删除、migration 877、旧 API `404` 和 clean grep 均被阻塞，另行计划后才能实施。
- 不运行真实 CRM Agent，不访问用户安装的 Agent CLI，不读取 Agent 账户，不消费模型额度。
- 不启动或操控用户 Chrome，不做用户 Chrome Network/Console、截图或 CRM side-by-side grounding。
- fixture/archive 只能证明确定性代码路径、受控 Preview 和持久化语义；不能称为 Task 8 严格验收、完整验收或 full acceptance。
- 现有 completion V2、daemon package、save/discard、persistence 测试只重跑，不为本计划重写。
- 每个生产符号在编辑前必须先刷新 GitNexus 并执行 upstream impact。结果为 HIGH 或 CRITICAL 时，必须先向用户报告调用者、受影响流程和回归矩阵，获得继续指令后才能编辑。
- 所有命令从仓库根执行；进入 Go module 使用 `(cd server && go test ...)`，不依赖 shell 的持久 `cd` 状态。

## 证据预算

每项结论只有一个负责人，避免同一测试被重复解释为多种现场证据。

| 结论 | 唯一证据负责人 | 允许的证据 | 明确限制 |
| --- | --- | --- | --- |
| V2 archive 只接受 V2 schema，并重算 index/digest/binding | Task 1 | `TestValidateV2ArchiveRejectsLegacySchema` 与既有 `v2_archive_test.go` | 不证明 Agent 生成质量 |
| 静态 Audit 拒绝脚本、外连、表单、不安全 CSS 与不可见 UI | Task 2 | `v2_audit_test.go` 和 `designpreview` 定向套件 | 受控 fixture/浏览器，不是用户 Chrome |
| Preview 文件媒体类型、`no-store` 和 capability scope/expiry 正确 | Task 3 | 两个新增 handler 测试 | 不证明人工视觉质量或 CRM grounding |
| create/adjust/regenerate 的 handler 入队不依赖 `MULTICA_OPEN_DESIGN_*` 且不创建 Run | Task 4 | 新增三操作 handler 测试、create 路由集成测试、daemon config-load 测试与数据库断言 | 只验证 config load、enqueue/context/no Run，不证明 daemon finalization 或端到端执行 |
| 坏包不覆盖 draft/saved，完成状态与持久化原子 | Task 5 | 既有 completion V2 与失败隔离套件 | 不注入真实模型恶意输出 |
| daemon 顺序为 collect/Audit/Preview/upload/complete，V2 不调用 Worker supervisor | Task 6 | 既有 daemon fake/verifier 套件 | fake uploader/verifier 不等于现场执行 |
| save/discard 与调整 base digest 正确 | Task 7 | 既有 Native metadata、immutable base、persistence 套件 | 不证明用户界面操作体验 |
| Phase A 最终状态与验收措辞 | Task 9 | 实际命令输出、digest、数据库断言汇总 | 必须保留 CRM Agent/用户 Chrome/full acceptance 缺口 |

任何负责人只能记录自己实际执行并观察到的结果。失败、跳过、环境依赖和测试耗时必须原样记录；不得由其他任务的通过结果代填。

## Task 0：建立基线并刷新 GitNexus

**Files:**
- Read: `docs/superpowers/specs/2026-08-12-native-design-phase-1-closure-and-legacy-removal-design.md`
- Read: `server/internal/projectdesignsystem/v2_{types,archive,audit,preview}.go`
- Read: `server/internal/designpreview/`
- Read: `server/internal/handler/project_design_system.go`
- Read: `server/internal/handler/project_design_system_package_preview.go`

- [ ] **Step 1: 确认工作树与目标分支，不改动文件**

```bash
git status --short
git branch --show-current
```

Expected: 记录已有用户改动；不得清理、覆盖或暂存无关文件。

- [ ] **Step 2: 刷新 GitNexus 索引**

```bash
node .gitnexus/run.cjs analyze
```

Expected: 分析成功，后续 impact 基于当前 checkout；若失败，停止生产代码编辑并记录失败。

- [ ] **Step 3: 运行 Phase A 初始基线**

```bash
(cd server && go test ./internal/projectdesignsystem ./internal/designpreview ./internal/daemon ./internal/handler ./cmd/server -count=1)
pnpm --filter @multica/core test
pnpm --filter @multica/views test
pnpm typecheck
```

Expected: 记录每条命令的真实结果。既有失败单独归类，不能预先标为本分支问题，也不能据此跳过后续定向测试。

## Task 1：锁定 V2 archive schema 拒绝契约

**Files:**
- Modify test first: `server/internal/projectdesignsystem/v2_archive_test.go`
- Modify only if RED reveals a defect: `server/internal/projectdesignsystem/v2_archive.go`
- Reference: `server/internal/projectdesignsystem/v2_types.go`

- [ ] **Step 1: 在任何生产编辑前执行 impact gate**

```bash
node .gitnexus/run.cjs impact ValidateV2Archive --direction upstream --repo multica
```

Expected: 保存 risk、直接调用者和受影响流程。HIGH/CRITICAL 时先报告并暂停生产编辑；测试文件仍可添加。

- [ ] **Step 2: 添加精确回归测试**

在 `v2_archive_test.go` 增加：

```go
func TestValidateV2ArchiveRejectsLegacySchema(t *testing.T) {
	collected := collectValidV2(t, validV2Binding())
	entries := readV2ZipEntries(t, collected.Archive)

	var manifest ManifestV2
	if err := json.Unmarshal(entries["manifest.json"], &manifest); err != nil {
		t.Fatalf("decode manifest: %v", err)
	}
	manifest.SchemaVersion = "multica.project-design-system/v1"
	entries["manifest.json"], _ = json.Marshal(manifest)

	pkg, err := ValidateV2Archive(buildV2ZipFromMap(t, entries), validV2Binding())
	assertV2DiagnosticCode(t, pkg.Audit, err, "manifest_schema_invalid")
}
```

- [ ] **Step 3: 运行 characterization/RED gate**

```bash
(cd server && go test ./internal/projectdesignsystem -run '^TestValidateV2ArchiveRejectsLegacySchema$' -count=1 -v)
```

Expected: 当前 `ValidateV2Archive` 已有 schema 检查时测试应 PASS，作为缺失回归证据；若 FAIL，失败必须是接受 legacy schema 或错误诊断码，而不是 fixture 编译错误。

- [ ] **Step 4: 只在 Step 3 暴露行为缺口时做最小 GREEN**

在 `ValidateV2Archive` strict decode 后、binding/index/digest 使用前保留单一拒绝分支：

```go
if manifest.SchemaVersion != PackageSchemaV2 {
	return invalidValidatedV2(
		"manifest_schema_invalid",
		"manifest.json",
		"manifest schema is not V2",
		manifest.ContentDigest,
	)
}
```

若 Step 3 已 PASS，不修改生产代码，不制造无意义 diff。

- [ ] **Step 5: 运行 archive 与 Audit 定向 GREEN**

```bash
(cd server && go test ./internal/projectdesignsystem -run '^(TestValidateV2ArchiveRejectsLegacySchema|TestValidateV2ArchiveRecomputesEveryDigest|TestValidateV2ArchiveBindsTaskInputAndBaseDigest|TestCollectV2DirectoryRejectsSymlinkHardlinkAndTraversal)$' -count=1 -v)
```

Expected: PASS；legacy schema、篡改 digest、binding/base mismatch 和链接/穿越均 fail closed。

- [ ] **Step 6: 提交 archive 契约测试**

```bash
git add server/internal/projectdesignsystem/v2_archive_test.go
git add server/internal/projectdesignsystem/v2_archive.go # 仅在生产文件确有修改时执行
git diff --cached --check
node .gitnexus/run.cjs detect-changes --scope staged --repo multica
git commit -m "test(design): reject legacy native package schemas"
```

Expected: `detect-changes` 只报告预期 archive validation/test 范围。

## Task 2：复核静态 Audit 与受控 Preview 质量边界

**Files:**
- Verify only: `server/internal/projectdesignsystem/v2_audit_test.go`
- Verify only: `server/internal/projectdesignsystem/v2_preview.go`
- Verify only: `server/internal/designpreview/browser_test.go`
- Verify only: `server/internal/designpreview/policy.go`

- [ ] **Step 1: 运行静态安全矩阵**

```bash
(cd server && go test ./internal/projectdesignsystem -run '^(TestAuditV2RejectsScriptsNetworkFormsAndUnsafeCSS|TestAuditV2RequiresVisibleTokenBackedPreview|TestAuditV2RejectsActiveSVGNetworkReferences|TestAuditV2RejectsCompleteStaticDocumentHiding|TestAuditV2RejectsUnsupportedCSSBlockAtRules)$' -count=1 -v)
```

Expected: PASS；脚本、事件属性、表单、iframe、远程 URL、CSS import、active SVG、整页隐藏和不支持的 CSS block at-rule 均被拒绝。

- [ ] **Step 2: 运行受控浏览器 Preview 矩阵**

```bash
(cd server && go test ./internal/designpreview -run '^(TestChromiumVerifierAcceptsVisibleStaticTarget|TestChromiumVerifierAcceptsNativeUIKitTarget|TestChromiumVerifierRejectsBlankAndOverflowingTarget|TestChromiumVerifierRejectsAncestorHiddenAndOffscreenContent|TestChromiumVerifierBlocksOutboundRequests|TestChromiumVerifierReportsBrokenImagesAndConsoleErrors|TestValidateReceiptBindsDigestAndTargetSet)$' -count=1 -v)
```

Expected: PASS；可见 fixture 通过，空白、越界、祖先隐藏、离屏、外连、坏图片、console error 和 receipt digest/target mismatch 被拒绝。若本机没有可执行 Chromium，按真实 skip/fail 记录，不得写成通过。

- [ ] **Step 3: 不做代码提交**

本任务是既有证据复核。只有测试真实失败且根因属于批准的 Phase A 安全边界时，先对拟编辑符号运行 impact，再另起最小 RED/GREEN 修复提交；不得顺手重构 parser 或浏览器实现。

## Task 3：补齐 Native Preview 媒体类型、缓存与 capability 契约

**Files:**
- Modify test first: `server/internal/handler/project_design_system_package_preview_test.go`
- Modify after RED: `server/internal/handler/project_design_system_package_preview.go`
- Modify only if capability helper requires correction: `server/internal/handler/project_design_system_open_design_preview.go`

- [ ] **Step 1: 运行 production symbol impact gates**

```bash
node .gitnexus/run.cjs impact GetProjectDesignSystemPackagePreview --direction upstream --repo multica
node .gitnexus/run.cjs impact GetProjectDesignSystemPackagePreviewFile --direction upstream --repo multica
node .gitnexus/run.cjs impact issueOpenDesignArchivePreviewAccessToken --direction upstream --repo multica
node .gitnexus/run.cjs impact validateOpenDesignArchivePreviewAccessToken --direction upstream --repo multica
```

Expected: 保存 risk 与路由调用者。任一 HIGH/CRITICAL 时先报告 blast radius 和本任务回归矩阵，暂停生产编辑。

- [ ] **Step 2: 添加媒体类型与缓存测试**

新增 `TestNativePackagePreviewFileServesMediaTypeAndNoStore`。使用 `newNativeV2CompletionFixture` 完成合法 V2 package，通过 `GetProjectDesignSystemPackagePreview` 取得 digest/capability，再通过 `GetProjectDesignSystemPackagePreviewFile` 请求 `ui-kit/index.html` 和 `assets/crm-mark.svg`：

```go
for _, tt := range []struct {
	path        string
	contentType string
}{
	{path: "ui-kit/index.html", contentType: "text/html; charset=utf-8"},
	{path: "assets/crm-mark.svg", contentType: "image/svg+xml"},
} {
	t.Run(tt.path, func(t *testing.T) {
		response := performNativePackagePreviewFileRequest(t, fixture, tt.path)
		if response.Code != http.StatusOK {
			t.Fatalf("preview file status = %d, body = %s", response.Code, response.Body.String())
		}
		if got := response.Header().Get("Content-Type"); got != tt.contentType {
			t.Fatalf("Content-Type = %q, want %q", got, tt.contentType)
		}
		if got := response.Header().Get("Cache-Control"); got != "no-store" {
			t.Fatalf("Cache-Control = %q, want no-store", got)
		}
		if got := response.Header().Get("X-Content-Type-Options"); got != "nosniff" {
			t.Fatalf("X-Content-Type-Options = %q, want nosniff", got)
		}
	})
}
```

`performNativePackagePreviewFileRequest` 必须走两个真实 handler，不能直接调用 `nativePackagePreviewContentType`。实现为：

```go
func performNativePackagePreviewFileRequest(t *testing.T, fixture *nativeV2CompletionFixture, artifactPath string) *httptest.ResponseRecorder {
	t.Helper()
	systemID := uuidToString(fixture.Completion.System.ID)
	manifestResponse := performProjectDesignSystemIDRequest(
		t,
		testHandler.GetProjectDesignSystemPackagePreview,
		http.MethodGet,
		"/api/project-design-systems/"+systemID+"/package-preview",
		systemID,
		nil,
	)
	if manifestResponse.Code != http.StatusOK {
		t.Fatalf("package preview status = %d, body = %s", manifestResponse.Code, manifestResponse.Body.String())
	}
	var preview projectDesignSystemPackagePreviewResponse
	if err := json.NewDecoder(manifestResponse.Body).Decode(&preview); err != nil {
		t.Fatalf("decode package preview: %v", err)
	}
	if preview.ContentDigest != fixture.Collected.Manifest.ContentDigest || preview.ResourceAccessToken == "" {
		t.Fatalf("package preview capability = %+v", preview)
	}

	response := httptest.NewRecorder()
	request := newRequest(http.MethodGet, "/api/project-design-system-previews/native/file", nil)
	route := chi.NewRouteContext()
	route.URLParams.Add("workspaceId", testWorkspaceID)
	route.URLParams.Add("systemId", systemID)
	route.URLParams.Add("digest", strings.TrimPrefix(preview.ContentDigest, "sha256:"))
	route.URLParams.Add("accessToken", preview.ResourceAccessToken)
	route.URLParams.Add("*", artifactPath)
	request = request.WithContext(context.WithValue(request.Context(), chi.RouteCtxKey, route))
	testHandler.GetProjectDesignSystemPackagePreviewFile(response, request)
	return response
}
```

- [ ] **Step 3: 运行 RED**

```bash
(cd server && go test ./internal/handler -run '^TestNativePackagePreviewFileServesMediaTypeAndNoStore$' -count=1 -v)
```

Expected: FAIL，当前响应为 `Cache-Control: private, max-age=31536000, immutable`，期望 `no-store`；媒体类型断言应已通过。若失败原因不是缓存 header，先修正测试 fixture。

- [ ] **Step 4: 最小 GREEN 修复 Preview 文件 header**

在 `GetProjectDesignSystemPackagePreviewFile` 改为：

```go
w.Header().Set("Cache-Control", "no-store")
```

保留现有精确 `Content-Type`、`Content-Length`、`nosniff`、CSP、digest 和 workspace header。不要改路由或历史 Preview fallback。

- [ ] **Step 5: 添加 capability scope/expiry 测试**

新增 `TestNativePackagePreviewCapabilityIsScopedAndExpires`，使用 helper 返回的 `expiresAt`，不使用 sleep：

```go
func TestNativePackagePreviewCapabilityIsScopedAndExpires(t *testing.T) {
	workspaceID := "11111111-1111-4111-8111-111111111111"
	systemID := "22222222-2222-4222-8222-222222222222"
	digest := "sha256:" + strings.Repeat("a", 64)

	token, expiresAt := issueOpenDesignArchivePreviewAccessToken(workspaceID, systemID, digest)
	if !validateOpenDesignArchivePreviewAccessToken(token, workspaceID, systemID, digest, expiresAt.Add(-time.Second)) {
		t.Fatal("fresh capability was rejected")
	}
	for _, mismatch := range []struct{ workspaceID, systemID, digest string }{
		{workspaceID: "33333333-3333-4333-8333-333333333333", systemID: systemID, digest: digest},
		{workspaceID: workspaceID, systemID: "44444444-4444-4444-8444-444444444444", digest: digest},
		{workspaceID: workspaceID, systemID: systemID, digest: "sha256:" + strings.Repeat("b", 64)},
	} {
		if validateOpenDesignArchivePreviewAccessToken(token, mismatch.workspaceID, mismatch.systemID, mismatch.digest, expiresAt.Add(-time.Second)) {
			t.Fatalf("capability accepted mismatched scope: %+v", mismatch)
		}
	}
	if validateOpenDesignArchivePreviewAccessToken(token, workspaceID, systemID, digest, expiresAt.Add(time.Second)) {
		t.Fatal("expired capability was accepted")
	}
}
```

- [ ] **Step 6: 运行 capability characterization/GREEN**

```bash
(cd server && go test ./internal/handler -run '^(TestNativePackagePreviewFileServesMediaTypeAndNoStore|TestNativePackagePreviewCapabilityIsScopedAndExpires|TestNativePackagePreviewCSPTrustsOnlyTheBridge|TestNativePackageRetrievalRejectsForeignSelfConsistentArchiveBinding)$' -count=1 -v)
```

Expected: PASS。若 capability 测试初次即 PASS，不修改 token helper；若 FAIL，只修复 scope/expiry 比较，不在 Phase A 重命名 Open Design helper。

- [ ] **Step 7: 提交 Preview 安全修复**

```bash
git add server/internal/handler/project_design_system_package_preview.go server/internal/handler/project_design_system_package_preview_test.go
git add server/internal/handler/project_design_system_open_design_preview.go # 仅在 helper 确有修改时执行
git diff --cached --check
node .gitnexus/run.cjs detect-changes --scope staged --repo multica
git commit -m "fix(design): harden native package preview access"
```

Expected: `detect-changes` 仅影响 Native package Preview 读取/header/capability 流。

## Task 4：证明 config load 与 create/adjust/regenerate 入队无 Open Design 环境依赖

**Files:**
- Modify tests: `server/internal/handler/project_design_system_test.go`
- Modify integration test: `server/cmd/server/router_open_design_test.go`
- Modify daemon config test: `server/internal/daemon/config_test.go`
- Modify production only if RED reveals a defect: `server/internal/handler/project_design_system.go`

- [ ] **Step 1: 运行 task context 生产符号 impact gates**

```bash
node .gitnexus/run.cjs impact CreateProjectDesignSystem --direction upstream --repo multica
node .gitnexus/run.cjs impact AdjustProjectDesignSystem --direction upstream --repo multica
node .gitnexus/run.cjs impact RegenerateProjectDesignSystem --direction upstream --repo multica
node .gitnexus/run.cjs impact enqueueExistingProjectDesignSystemTask --direction upstream --repo multica
node .gitnexus/run.cjs impact marshalProjectDesignSystemTaskContext --direction upstream --repo multica
```

Expected: `marshalProjectDesignSystemTaskContext` 可能为 HIGH；必须按硬边界先报告后再编辑生产代码。测试可先写，不能先改实现。

- [ ] **Step 2: 添加测试专用环境清理 helper**

在 `project_design_system_test.go` 增加非并行 helper，真实 unset 并在 cleanup 恢复：

```go
func unsetOpenDesignEnvironmentForTest(t *testing.T) {
	t.Helper()
	for _, name := range []string{
		"MULTICA_OPEN_DESIGN_ENABLED",
		"MULTICA_OPEN_DESIGN_WORKER_URL",
		"MULTICA_OPEN_DESIGN_WORKER_TOKEN",
		"MULTICA_OPEN_DESIGN_ARTIFACT_ROOT",
		"MULTICA_OPEN_DESIGN_BROWSER_PATH",
	} {
		value, existed := os.LookupEnv(name)
		if err := os.Unsetenv(name); err != nil {
			t.Fatalf("unset %s: %v", name, err)
		}
		t.Cleanup(func() {
			if existed {
				_ = os.Setenv(name, value)
			} else {
				_ = os.Unsetenv(name)
			}
		})
	}
}
```

测试不得调用 `t.Parallel()`，避免进程环境竞争。

- [ ] **Step 3: 添加三操作 handler 测试**

新增 `TestNativeProjectDesignSystemOperationsDoNotRequireOpenDesignEnvironment`，不使用 `t.Parallel()`。测试必须按以下三个 subtest 实现，不用循环隐藏操作差异：

1. `create`：调用 `unsetOpenDesignEnvironmentForTest(t)`；用 `createProjectForDesignTest` 与 `createProjectDesignSystemAgent(t, "online")` 建输入；通过 `performProjectDesignSystemRequest` 调用 `testHandler.CreateProjectDesignSystem`，请求体固定为 `project_id`、`agent_id`、`platform: "web"`、`brief: "Native environment isolation."`；断言 HTTP 202，解码 `ProjectDesignSystemResponse` 后把 active task ID 和 system ID 交给 `assertNativeV2TaskWithoutOpenDesignRun`；断言 operation 为 `service.ProjectDesignSystemGenerate`、`InputSnapshotSHA256` 非空、`BasePackageSHA256` 为空。
2. `adjust`：调用 `unsetOpenDesignEnvironmentForTest(t)`；使用 `newNativeV2CompletionFixture(t, service.ProjectDesignSystemGenerate)` 和 `fixture.completeTask(t, fixture.buildPackagePayload(t, nil))` 形成 V2 draft；用 `json.Marshal(map[string]any{"agent_id": fixture.Completion.AgentID, "platform": "web", "brief": "Native adjustment isolation.", "references": []any{}})` 生成 input snapshot，再执行 `UPDATE project_design_system SET input_snapshot = $1::jsonb WHERE id = $2`；用 `performProjectDesignSystemIDRequest` 调用 `testHandler.AdjustProjectDesignSystem`，请求体为 `agent_id`、`instruction: "Tighten the primary action."`、`scope: {"kind":"all"}`；断言 HTTP 202，active task context operation 为 `service.ProjectDesignSystemAdjust`，`BasePackageSHA256 == fixture.Collected.Manifest.ContentDigest`。
3. `regenerate`：独立创建新的 generate fixture 并完成 V2 draft，写入 brief 为 `Native regeneration isolation.` 的 input snapshot；用 `performProjectDesignSystemIDRequest` 调用 `testHandler.RegenerateProjectDesignSystem`，请求体只含 `agent_id`；断言 HTTP 202，active task context operation 为 `service.ProjectDesignSystemRegenerate`，`BasePackageSHA256 == fixture.Collected.Manifest.ContentDigest`。

每个 subtest 的 active task 均调用下面的 helper，因而统一证明 V2 schema、原始 JSON 不含 `open_design_run` 和数据库 Run count 为零。adjust/regenerate 的前置 V2 draft 由 fixture/fake receipt seed；本 Task 的被测行为止于 handler 入队/context，不执行新任务的 daemon finalizer，不启动 Agent CLI，也不得写成“环境变量 unset 时完整流程成功”。

重复断言使用以下完整 helper：

```go
func assertNativeV2TaskWithoutOpenDesignRun(t *testing.T, taskID, systemID string) service.ProjectDesignSystemTaskContext {
	t.Helper()
	var raw []byte
	if err := testPool.QueryRow(context.Background(), `SELECT context FROM agent_task_queue WHERE id = $1`, taskID).Scan(&raw); err != nil {
		t.Fatalf("load task context: %v", err)
	}
	var object map[string]json.RawMessage
	if err := json.Unmarshal(raw, &object); err != nil {
		t.Fatalf("decode raw task context: %v", err)
	}
	if _, exists := object["open_design_run"]; exists {
		t.Fatalf("task context contains open_design_run: %s", raw)
	}
	var taskContext service.ProjectDesignSystemTaskContext
	if err := json.Unmarshal(raw, &taskContext); err != nil || taskContext.PackageSchema != projectdesignsystem.PackageSchemaV2 {
		t.Fatalf("native task context = %+v, err = %v", taskContext, err)
	}
	var runCount int
	if err := testPool.QueryRow(context.Background(), `SELECT COUNT(*) FROM open_design_run WHERE design_system_id = $1`, systemID).Scan(&runCount); err != nil {
		t.Fatalf("count open_design_run: %v", err)
	}
	if runCount != 0 {
		t.Fatalf("open_design_run count = %d, want 0", runCount)
	}
	return taskContext
}
```

- [ ] **Step 4: 更新 router 集成测试环境前提**

将 `TestOpenDesignFeatureFlagUsesNativeV2Task` 改名为 `TestNativeProjectDesignSystemCreateDoesNotRequireOpenDesignEnvironment`，删除 `MULTICA_OPEN_DESIGN_ENABLED=true`。在 `router_open_design_test.go` 内增加独立的 `unsetOpenDesignRouterEnvironmentForTest`，实现与 handler helper 相同并覆盖 `MULTICA_OPEN_DESIGN_ENABLED`、`MULTICA_OPEN_DESIGN_WORKER_URL`、`MULTICA_OPEN_DESIGN_WORKER_TOKEN`、`MULTICA_OPEN_DESIGN_ARTIFACT_ROOT`、`MULTICA_OPEN_DESIGN_BROWSER_PATH` 五个变量；测试开头调用它。保留真实 HTTP `POST /api/project-design-systems`、V2 context 和零 `open_design_run` 断言。该测试只 enqueue，不启动 daemon，不运行用户 Agent CLI。

- [ ] **Step 5: 添加 daemon config-load 边界测试**

在 `server/internal/daemon/config_test.go` 增加 `TestLoadConfigStartsNativePreviewConfigurationWithLegacyEnvironmentUnset`。使用 `stageFakeAgent(t)`，通过 `t.Setenv` 把五个 `MULTICA_OPEN_DESIGN_*` 设为空，把 `MULTICA_DESIGN_PREVIEW_BROWSER_PATH` 设为 `filepath.Join(t.TempDir(), "native-chromium")`，调用真实 `LoadConfig(Overrides{ServerURL: "http://localhost:0", WorkspacesRoot: t.TempDir()})`；断言 `err == nil`、`cfg.DesignPreviewBrowserPath` 等于 native path、四个 legacy config fields 均为空。该测试只证明 daemon 配置可加载/启动所需配置不依赖 legacy env，不启动 task 或浏览器：

```go
func TestLoadConfigStartsNativePreviewConfigurationWithLegacyEnvironmentUnset(t *testing.T) {
	stageFakeAgent(t)
	for _, name := range []string{"MULTICA_OPEN_DESIGN_WORKER_URL", "MULTICA_OPEN_DESIGN_WORKER_TOKEN", "MULTICA_OPEN_DESIGN_ARTIFACT_ROOT", "MULTICA_OPEN_DESIGN_BROWSER_PATH"} {
		t.Setenv(name, "")
	}
	native := filepath.Join(t.TempDir(), "native-chromium")
	t.Setenv("MULTICA_DESIGN_PREVIEW_BROWSER_PATH", native)
	cfg, err := LoadConfig(Overrides{ServerURL: "http://localhost:0", WorkspacesRoot: t.TempDir()})
	if err != nil { t.Fatalf("LoadConfig: %v", err) }
	if cfg.DesignPreviewBrowserPath != native { t.Fatalf("DesignPreviewBrowserPath = %q, want %q", cfg.DesignPreviewBrowserPath, native) }
	if cfg.OpenDesignWorkerURL != "" || cfg.OpenDesignWorkerToken != "" || cfg.OpenDesignArtifactRoot != "" || cfg.OpenDesignBrowserPath != "" {
		t.Fatalf("legacy config unexpectedly populated: %+v", cfg)
	}
}
```

- [ ] **Step 6: 运行 RED/characterization gate**

```bash
(cd server && go test ./internal/handler -run '^TestNativeProjectDesignSystemOperationsDoNotRequireOpenDesignEnvironment$' -count=1 -v)
(cd server && go test ./cmd/server -run '^TestNativeProjectDesignSystemCreateDoesNotRequireOpenDesignEnvironment$' -count=1 -v)
(cd server && go test ./internal/daemon -run '^TestLoadConfigStartsNativePreviewConfigurationWithLegacyEnvironmentUnset$' -count=1 -v)
```

Expected: 当前原生上下文正确时 PASS，形成缺失证据；若 FAIL，只接受 config load 依赖 legacy env、handler 读取 Open Design env/flag、非 V2 schema、出现 `open_design_run` 或 base/input digest 错误作为生产修复目标。通过仍只证明 config load 与 enqueue/context/no Run。

- [ ] **Step 7: 只在 RED 时做最小 GREEN**

保持三个 handler 通过 `marshalProjectDesignSystemTaskContext(..., nil)` 进入 Native V2 分支。不得读取 `MULTICA_OPEN_DESIGN_*`，不得调用 `prepareOpenDesignRun`/`persistOpenDesignRun`，不得删除历史兼容代码。测试已 PASS 时不修改 `project_design_system.go`。

- [ ] **Step 8: 运行完整 bounded GREEN**

```bash
(cd server && go test ./internal/handler -run '^(TestNativeProjectDesignSystemOperationsDoNotRequireOpenDesignEnvironment|TestCreateProjectDesignSystemAlwaysEnqueuesNativeV2WhenOpenDesignFlagIsTrue|TestRegenerateProjectDesignSystemBindsCurrentBaseDigest|TestAdjustProjectDesignSystemUsesImmutableV2BaseReference)$' -count=1 -v)
(cd server && go test ./cmd/server -run '^(TestNativeProjectDesignSystemCreateDoesNotRequireOpenDesignEnvironment|TestOpenDesignDaemonLifecycleRoutesAreRegistered|TestOpenDesignArchivePreviewRoutesAreRegistered)$' -count=1 -v)
(cd server && go test ./internal/daemon -run '^(TestLoadConfigStartsNativePreviewConfigurationWithLegacyEnvironmentUnset|TestLoadConfig_DesignPreviewBrowserPath_PrefersNativeEnvVar|TestLoadConfig_DesignPreviewBrowserPath_DoesNotFallBackToOpenDesignPath)$' -count=1 -v)
```

Expected: PASS 且每个 regex 命中测试。Phase A 仍保留历史路由；删除路由属于被阻塞的 Phase B。结论仅为 config load + handler enqueue/context/no Run。

- [ ] **Step 9: 提交 bounded 无 Worker 配置依赖证据**

```bash
git add server/internal/handler/project_design_system_test.go server/cmd/server/router_open_design_test.go server/internal/daemon/config_test.go
git add server/internal/handler/project_design_system.go # 仅在生产文件确有修改时执行
git diff --cached --check
node .gitnexus/run.cjs detect-changes --scope staged --repo multica
git commit -m "test(design): prove native tasks ignore worker config"
```

Expected: `detect-changes` 只影响 daemon config load 和项目设计体系 enqueue/context 流。

## Task 5：重跑 completion V2 与失败隔离，不改写套件

**Files:**
- Verify only: `server/internal/handler/project_design_system_completion_v2_test.go`
- Verify only: `server/internal/handler/project_design_system_completion_test.go`
- Verify only: `server/internal/handler/project_design_system_package_upload_test.go`

- [ ] **Step 1: 运行 V2 完成门禁与坏包隔离**

```bash
(cd server && go test ./internal/handler -run '^(TestCompleteProjectDesignSystemV2.*|TestProjectDesignSystemFailureAndCancellationPreserveExistingPackage|TestSaveProjectDesignSystemDraftCopiesNativeArchiveColumns)$' -count=1 -v)
```

Expected: PASS，且不存在“0 tests to run”。坏 digest、Audit/Preview failure、取消和 completion failure 必须保持 draft/saved 不变。

- [ ] **Step 2: 不改写既有测试**

若失败，先定位是 Task 1-4 回归还是既有基线。只有前者才回到对应任务修复；不得放宽 receipt、digest、Audit、Preview 或事务断言。

## Task 6：重跑 daemon package 顺序与无 Worker supervisor 证据

**Files:**
- Verify only: `server/internal/daemon/project_design_system_package_test.go`
- Verify only: `server/internal/daemon/config_test.go`
- Verify only: `server/internal/daemon/prompt_test.go`

- [ ] **Step 1: 运行 daemon V2 定向套件**

```bash
(cd server && go test ./internal/daemon -run '^(TestFinalizeProjectDesignSystemPackageCollectsAuditsPreviewsAndUploads|TestFinalizeProjectDesignSystemPackageBlocksBeforeUploadOnStaticAuditFailure|TestFinalizeProjectDesignSystemPackageBlocksBeforeCompletionOnPreviewFailure|TestFinalizeProjectDesignSystemPackageRejectsMissingBrowser|TestFinalizeProjectDesignSystemPackageReturnsTaskBoundReceipt|TestHandleTaskDoesNotCallOpenDesignSupervisorForV2Context|TestLoadConfig_DesignPreviewBrowserPath_PrefersNativeEnvVar|TestBuildPromptProjectDesignSystemV2UsesEvidenceThenDesignStages|TestBuildPromptProjectDesignSystemV2RequiresCompleteStaticPackage|TestBuildPromptProjectDesignSystemV2NeverMentionsWorkerRuntimeOrFigmaJSON)$' -count=1 -v)
```

Expected: PASS。测试使用 fake executable/verifier/uploader 或缺失路径，不查找或执行用户 Agent CLI；V2 路径不调用 Open Design supervisor。

本 Task 是 V2 package finalization 的唯一负责人；它独立证明 collect -> Audit -> Preview -> upload -> completion gate。不得把 Task 4 的 env-unset enqueue 证据与本 Task 合并为一次端到端运行结论。

- [ ] **Step 2: 运行 daemon package 全包回归**

```bash
(cd server && go test ./internal/daemon -count=1)
```

Expected: PASS；若失败，保存具体 test name 和日志，不用定向套件通过掩盖全包失败。

## Task 7：重跑调整、保存、放弃和 persistence 证据

**Files:**
- Modify: `server/internal/handler/project_design_system_package_preview_test.go`
- Verify only: `server/internal/handler/project_design_system_open_design_preview_test.go`
- Verify only: `server/internal/handler/project_design_system_persistence_test.go`

- [ ] **Step 1: 运行 Native adjust/save/discard 套件**

```bash
(cd server && go test ./internal/handler -run '^(TestAdjustProjectDesignSystemUsesImmutableV2BaseReference|TestSaveAndDiscardProjectDesignSystemPreserveNativeArchiveMetadata|TestDiscardFirstNativeV2DraftReturnsUnestablished|TestProjectDesignSystemDraftUpsertLeavesSavedUntouched|TestSaveProjectDesignSystemCopiesDraftAndDeletesDraftAtomically|TestSaveProjectDesignSystemDraftCopiesNativeArchiveColumns)$' -count=1 -v)
```

Expected: PASS 且输出包含六个测试名；adjust 绑定 immutable V2 base digest，save 复制全部 V2 archive metadata 并删除 draft，后续 discard 恢复未变化的 saved，首次 Native V2 draft discard 返回 `unestablished`。

- [ ] **Step 2: 添加首次 Native V2 discard 精确测试**

在 `project_design_system_package_preview_test.go` 增加 `TestDiscardFirstNativeV2DraftReturnsUnestablished`：用 `newNativeV2CompletionFixture(t, service.ProjectDesignSystemGenerate)` 和真实 completion helper 形成只有 draft、无 saved 的 V2 system；调用 `DiscardProjectDesignSystemDraft`；断言 HTTP 200，响应 `Status == "unestablished"`、`HasUnsavedChanges == false`、`ActiveTask == nil`、`SavedAt == nil`、sections/token groups/preview targets 均为空，并用 `GetProjectDesignSystemPackageBySlot` 断言 draft/saved 都是 `pgx.ErrNoRows`。不得复用 `TestDiscardFirstOpenDesignArchiveDraftReturnsUnestablished` 作为 Native V2 证据。

- [ ] **Step 3: 保持既有 save/discard 与 persistence 套件不放宽**

不新增、不改名、不放宽 `TestSaveAndDiscardProjectDesignSystemPreserveNativeArchiveMetadata`、`TestProjectDesignSystemDraftUpsertLeavesSavedUntouched`、`TestSaveProjectDesignSystemCopiesDraftAndDeletesDraftAtomically` 或 `TestSaveProjectDesignSystemDraftCopiesNativeArchiveColumns`。实际 PASS/FAIL 进入 Task 9 证据记录。

- [ ] **Step 4: 提交首次 Native V2 discard 合同测试**

```bash
git add server/internal/handler/project_design_system_package_preview_test.go
git diff --cached --check
node .gitnexus/run.cjs detect-changes --scope staged --repo multica
git commit -m "test(design): cover first native draft discard"
```

Expected: staged diff 只新增 `TestDiscardFirstNativeV2DraftReturnsUnestablished`；四个既有 persistence/save/discard 测试没有改动。

## Task 8：最终 focused 与 broad verification

**Files:**
- Verify only: all Phase A touched and referenced packages

- [ ] **Step 1: 运行最终 focused Go 矩阵**

```bash
(cd server && go test ./internal/projectdesignsystem -count=1)
(cd server && go test ./internal/designpreview -count=1)
(cd server && go test ./internal/daemon -run 'ProjectDesignSystemPackage|HandleTaskDoesNotCallOpenDesign|BuildPromptProjectDesignSystemV2' -count=1)
(cd server && go test ./internal/handler -run 'ProjectDesignSystem|NativePackagePreview' -count=1)
(cd server && go test ./cmd/server -run 'NativeProjectDesignSystem|OpenDesign' -count=1)
```

Expected: 全部 PASS，且每条命令实际运行测试。记录 test/package 数和耗时。

- [ ] **Step 2: 运行 broad verification**

```bash
(cd server && go test ./internal/projectdesignsystem ./internal/designpreview ./internal/daemon ./internal/handler ./cmd/server -count=1)
pnpm --filter @multica/core test
pnpm --filter @multica/views test
pnpm typecheck
git diff --check
```

Expected: 全部 PASS。任何失败按真实状态记录；未通过时不得把 Phase A 写成完成。

- [ ] **Step 3: 确认没有用户 Agent CLI 执行**

核对日志与代码路径：daemon 测试使用 fake runner、fake verifier、fake uploader 或显式缺失 executable；没有设置 `MULTICA_RUN_REAL_AGENT_SMOKE=1`，没有运行 `-tags=agentintegration`。

Expected: 无用户 Agent CLI、账户访问或额度消耗。

## Task 9：按实际证据更新权威文档

**Files:**
- Modify: `docs/product/design-center/project-design-system-validation.md`
- Modify: `docs/product/design-center/README.md`
- Modify: `docs/product/design-center/decision-register.md`

- [ ] **Step 1: 汇总实际证据，不先写结论**

为每项保存：commit、命令、测试名、PASS/FAIL/SKIP、fixture archive digest、package content digest、数据库 `package_schema`、create/adjust/regenerate task ID、`open_design_run` count、draft/saved digest 前后值。

Expected: 每个 claim 能回指证据预算中的唯一负责人；不存在未执行命令的“通过”。

- [ ] **Step 2: 更新 `project-design-system-validation.md`**

新增 2026-08-12 Phase A 小节，明确写入：

```text
已验证：Native V2 fixture/archive、Package Audit、受控 Preview、draft 隔离、adjust/save/discard、无新 open_design_run。
未验证：真实 CRM Agent 生成、真实仓库 grounding、用户本机 Chrome 的视觉/Network/Console、人工 side-by-side 比对。
结论：低令牌自动化收口；不是原 Task 8 的严格验收、完整验收或 full acceptance。
```

不得复用 2026-07-29 V1 三文件或 OD Worker 证据冒充 Native V2 现场证据。

- [ ] **Step 3: 更新 `README.md`**

按实际结果写“自动化 Phase A 收口完成”或“部分完成”；保留真实 CRM Agent 与用户 Chrome grounding 缺口。Phase B 是已批准但未执行，不得描述旧链路已删除。

- [ ] **Step 4: 更新 `decision-register.md`**

新增已确认决定记录，说明本次只完成低令牌 Phase A；DC-039 的临时保留条款由已批准规格在未来 Phase B 执行时取代。当前代码和数据尚未删除，不能提前把 Phase B 标为完成。

- [ ] **Step 5: 检查文档禁用措辞**

```bash
rg -n 'full acceptance|完整验收|严格验收|真实 CRM Agent|用户.*Chrome|grounding' docs/product/design-center/project-design-system-validation.md docs/product/design-center/README.md docs/product/design-center/decision-register.md
```

Expected: `full acceptance`、完整/严格验收均为否定语境；真实 CRM Agent、用户 Chrome 和 grounding 均明确标为未验证。

- [ ] **Step 6: 提交实际证据文档**

```bash
git add docs/product/design-center/project-design-system-validation.md docs/product/design-center/README.md docs/product/design-center/decision-register.md
git diff --cached --check
node .gitnexus/run.cjs detect-changes --scope staged --repo multica
git commit -m "docs(design): record low-token phase one evidence"
```

Expected: staged diff 只包含实际执行证据和诚实状态；没有 Phase B 完成声明。

## Task 10：最终变更范围与交付检查

- [ ] **Step 1: 检查所有提交和工作树**

```bash
git status --short
git log --oneline -10
git diff --check
git diff --stat main...HEAD
```

Expected: 只有 Phase A 测试、必要最小生产修复和三份证据文档；不重写提交历史。

- [ ] **Step 2: 对最终分支执行 GitNexus change detection**

```bash
node .gitnexus/run.cjs detect-changes --scope compare --base-ref main --repo multica
```

Expected: 受影响流程仅为 Native V2 archive validation、package Preview 和项目设计体系 enqueue/context；若报告 Worker/V1 删除、migration 或无关流程，停止交付并检查范围。

- [ ] **Step 3: 输出最终限制声明**

最终报告必须保留以下语义：

```text
本次完成的是 Phase A 低令牌自动化收口。未运行真实 CRM Agent，未验证真实仓库 grounding，未在用户 Chrome 中执行视觉、Network 或 Console 验收，因此不能称为原 Task 8 的严格验收、完整验收或 full acceptance。Phase B 仍被阻塞且未实施。
```

## Phase A 完成条件

- `TestValidateV2ArchiveRejectsLegacySchema` 存在并通过。
- `TestNativePackagePreviewFileServesMediaTypeAndNoStore` 存在并通过。
- `TestNativePackagePreviewCapabilityIsScopedAndExpires` 存在并通过。
- daemon config 在 legacy env unset 时可加载 Native Preview 配置；create/adjust/regenerate handler enqueue Native V2、context 无 `open_design_run`、数据库 Run count 为零。该结论不包含 daemon finalization 或端到端 Agent 执行。
- `TestDiscardFirstNativeV2DraftReturnsUnestablished` 存在并通过，且定向 regex 实际命中该测试。
- 既有 completion V2、daemon package、失败隔离、save/discard 和 persistence 套件未被放宽且通过。
- focused 与 broad verification 均有实际结果；失败或 skip 已诚实记录。
- 三份权威文档只写实际证据，明确真实 CRM Agent、用户 Chrome grounding 和 full acceptance 未验证。
- 没有 Phase B 源码删除、migration、旧 API 行为变化或数据清理。
