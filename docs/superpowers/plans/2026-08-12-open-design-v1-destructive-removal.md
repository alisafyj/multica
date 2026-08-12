# Open Design V1 破坏性移除 TDD 实施计划

> **For agentic workers:** REQUIRED SUB-SKILL: Use `superpowers:subagent-driven-development` (recommended) or `superpowers:executing-plans` task-by-task；行为变更使用 `superpowers:test-driven-development`，异常失败使用 `superpowers:systematic-debugging`，完成前使用 `superpowers:verification-before-completion`。

**Goal:** 先完成 Native V2 Phase A 低令牌自动化收口，再不可逆删除 Open Design Worker/V1 的 API、运行时、兼容读取、持久化和历史数据，使 `multica.project-design-system/v2` 成为唯一活动链路。

**Architecture:** 保留 `designpreview/**`、`projectdesignsystem/**`、V2 package upload/Preview/verification、对象 archive、`project_design_system`、`project_design_system_package`、`agent_task_queue` 和 draft/saved。先把 V2 Preview 使用的四个 token helper 纯绿迁入中性文件，再锁定 404/V2-only 合同；上线前枚举并幂等删除旧对象，migration 877 随后显式删除旧行、索引、表并收紧 V2-only CHECK，不留 alias、fallback、转换器或 feature flag。

**Tech Stack:** Go 1.26.1, PostgreSQL 17, sqlc, Chi, object storage, React 19, TanStack Query, Zod, Vitest, pnpm/Turborepo, GitNexus.

---

## 全局门禁

- Phase A 的唯一执行计划是 `docs/superpowers/plans/2026-08-12-native-design-phase-1-low-token-closure.md`，它是本计划 Phase B 的硬依赖；该计划全部 checkbox、证据、文档和负责人确认未通过时，本文件 Task 3 及以后不得开始。本文件 Task 1-2 只定义依赖验收，不替代或简化 Phase A 原计划。
- Phase A 只能称“低令牌自动化收口”；真实 CRM Agent/仓库 grounding、用户 Chrome 视觉/Network/Console 未验证，不得称严格、完整或 full acceptance。
- Phase B 永久删除旧 Run/package/object；仅有旧 package 的项目回到未建立。migration 870-873 不得编辑/重命名。
- 保留 `DesignPreviewBrowserPath`、`MULTICA_DESIGN_PREVIEW_BROWSER_PATH`、V2 SQL/上传/Preview/verification 和通用 daemon Agent 生命周期。
- migration 877 不得含 FK、`REFERENCES` 或 `CASCADE`。对象删除必须先于 DB migration；任一对象失败即停止。
- Phase B 核心移除是一个可部署原子变更。下列内部 conventional commits 便于 TDD 审阅，但全部中间 commit 都不可发布；Task 12 全绿后的整体才是 release candidate。

## GitNexus 协议

- [ ] 根目录刷新索引并留证：

```bash
node .gitnexus/run.cjs analyze 2>&1 | tee /tmp/multica-gitnexus-refresh.log
```

- [ ] 每次编辑任何 symbol 前，对各 Task 列出的精确 symbol 逐条运行 `node .gitnexus/run.cjs impact SYMBOL --direction upstream --repo multica`（把 `SYMBOL` 替换为该 Task 清单中的原样名称），记录 callers/processes/risk。`HIGH/CRITICAL` 必须先向用户警告并停止，获准后才编辑；若实际改动增加 symbol，先把其准确名称补入本 Task 执行记录再运行 impact。
- [ ] 每次 commit 前运行：

```bash
git diff --check
git diff --cached --stat
node .gitnexus/run.cjs detect-changes --scope staged --repo .
```

预期：diff check 无输出，detect_changes 只含本 Task 预期 flow。重命名不用文本替换，使用 GitNexus rename 或逐处编译核对。

## 证据预算

| 门禁 | 最低证据 |
| --- | --- |
| Phase A | 1 个合法 archive；至少 10 个 archive 拒绝、8 个 HTML/CSS 拒绝；create/adjust/regenerate/save/discard；失败时 draft/saved 字节不变 |
| 无旧 Run | create/adjust/regenerate 前后 `open_design_run` count 不变 |
| V2 Preview | manifest/HTML/asset 200；错 token/digest/workspace 拒绝；CSP/media/cache；不调用旧 validator |
| 404/schema | 9 个 daemon route、evidence、旧 alias 均 404；v1/OpenDesign/legacy/缺失 schema 均拒绝 |
| 数据 | row/project/design-system 数、旧/V2 完整 object key 清单、`V2ObjectKeyConflicts=[]`、report SHA-256、dry-run/删除/重复删除日志 |
| migration | 独立 fresh/upgrade DB；失败全回滚；缺失 record 安全重放；V2 slot/digest/key/Audit/Preview 前后相等 |
| 回归 | 定向 Go/TS、`make test`、`pnpm typecheck`、`pnpm test` |
| 文档 | 五份 Design Center 文档记录命令、统计、数据损失、未验证项 |

每项记录 UTC、commit、命令、exit code、关键输出/digest。未执行写 `NOT RUN`，不得写“通过”。

## 五个 Oracle 审查门

- [ ] **Oracle 1，计划门（任何实现编辑前）：** 对照批准 spec、本计划和 Phase A 计划逐项核对本轮 11 项：877 down 纯注释 no-op；cleanup source 保留到生产 rollout 完成后；S3/local 与 Handler.Storage 一致；V2 key conflict 自动 fail closed；Phase A enqueue 与 finalizer 证据分离；first-discard 精确测试；migration rollback/replay/独立 DB；old-only API boundary；chi.Walk + HTTP route contract；无 rolling overlap 的离线 cutover；旧 desktop 只监控不 fallback。任一缺失即阻塞实现。
- [ ] **Oracle 2，Phase A 证据门（Task 2 后）：** 审核 handler 结论只覆盖 config load、enqueue/context/no Run，daemon finalizer 由独立 tests 证明；不得宣称 legacy env unset 时端到端成功。核对首次 Native V2 discard 为 unestablished，并确认真实 CRM Agent、grounding、用户 Chrome 仍未验证。
- [ ] **Oracle 3，runtime/API 移除门（Tasks 3-5 和 8-10 的 runtime/API diff 完成后、合入 migration release 前）：** 审核 daemon/router/service blast radius；`chi.Walk` 中 11 个旧 pattern 全部缺席且 retained V2 patterns 全部存在；HTTP 旧路由 404、V2 auth/handler behavior 通过；V2 Preview 无 legacy validator；cleanup tool 仍在 release。
- [ ] **Oracle 4，migration 门（Tasks 6-7 后）：** 审核 `V2ObjectKeyConflicts` 为空、对象 receipts、877 up SQL、纯注释 no-op down、failed-statement 全回滚、missing-record replay、独立 fresh/upgrade identity、old-only API boundary、无 FK/CASCADE；确认 Tasks 6-11 同一 artifact 且只能离线切换。
- [ ] **Oracle 5，最终门（Task 12 后）：** 审核 cleanup source 仍保留、旧/新 replicas 无重叠、compare detect_changes、Go/TS/migration/clean grep、五文档和 installed-old-desktop 监控措辞；工具删除只能是后续单独批准 release。

## Phase A：低令牌收口

### Task 1: 执行并验收独立 Phase A 计划

**Plan:** `docs/superpowers/plans/2026-08-12-native-design-phase-1-low-token-closure.md`

**Acceptance files:** `server/internal/projectdesignsystem/v2_archive_test.go`、`server/internal/projectdesignsystem/v2_audit_test.go`、`server/internal/designpreview/browser_test.go`、`server/internal/daemon/project_design_system_package_test.go`、`server/internal/handler/project_design_system_completion_v2_test.go`、`server/internal/handler/project_design_system_package_preview_test.go`、`server/internal/handler/project_design_system_persistence_test.go`、`server/internal/handler/project_design_system_test.go`。

- [ ] Impact：`CollectV2Directory`、`ValidateV2Archive`、`AuditV2`、`ChromiumVerifier.Verify`、`finalizeProjectDesignSystemResult`、`prepareProjectDesignSystemCompletion`、`persistProjectDesignSystemCompletion`、`createProjectDesignSystemTask`、`enqueueExistingProjectDesignSystemTask`。
- [ ] 按独立 Phase A 计划逐 Task 执行 TDD；不新增本文件自造的 aggregate test。必须存在并通过的新增测试是 `TestValidateV2ArchiveRejectsLegacySchema`、`TestNativePackagePreviewFileServesMediaTypeAndNoStore`、`TestNativePackagePreviewCapabilityIsScopedAndExpires`、`TestNativeProjectDesignSystemOperationsDoNotRequireOpenDesignEnvironment`、`TestNativeProjectDesignSystemCreateDoesNotRequireOpenDesignEnvironment`。
- [ ] 精确重跑既有安全测试：`TestValidateV2ArchiveRecomputesEveryDigest`、`TestValidateV2ArchiveBindsTaskInputAndBaseDigest`、`TestCollectV2DirectoryRejectsSymlinkHardlinkAndTraversal`、`TestAuditV2RejectsScriptsNetworkFormsAndUnsafeCSS`、`TestAuditV2RequiresVisibleTokenBackedPreview`、`TestAuditV2RejectsActiveSVGNetworkReferences`、`TestAuditV2RejectsCompleteStaticDocumentHiding`、`TestAuditV2RejectsUnsupportedCSSBlockAtRules`，以及 Phase A 计划列出的七个 `designpreview` tests。
- [ ] 精确重跑既有 lifecycle tests：`TestCompleteProjectDesignSystemV2CreatesPassedDraftAfterAllEvidenceMatches`、`TestCompleteProjectDesignSystemV2RejectsWrongTaskInputAgentAndBaseDigest`、`TestCompleteProjectDesignSystemV2RejectsMissingOrMutatedStoredArchive`、`TestCompleteProjectDesignSystemV2RejectsAuditOrPreviewFailure`、`TestCompleteProjectDesignSystemV2DoesNotReplaceExistingDraftOnFailure`、`TestCompleteProjectDesignSystemV2NeverChangesSavedOnFailure`、`TestCompleteProjectDesignSystemV2IsAtomicWithTaskCompletion`、`TestProjectDesignSystemFailureAndCancellationPreserveExistingPackage`、`TestSaveProjectDesignSystemDraftCopiesNativeArchiveColumns`、`TestFinalizeProjectDesignSystemPackageCollectsAuditsPreviewsAndUploads`、`TestFinalizeProjectDesignSystemPackageBlocksBeforeUploadOnStaticAuditFailure`、`TestFinalizeProjectDesignSystemPackageBlocksBeforeCompletionOnPreviewFailure`、`TestFinalizeProjectDesignSystemPackageRejectsMissingBrowser`、`TestFinalizeProjectDesignSystemPackageReturnsTaskBoundReceipt`、`TestHandleTaskDoesNotCallOpenDesignSupervisorForV2Context`、`TestAdjustProjectDesignSystemUsesImmutableV2BaseReference`、`TestSaveAndDiscardProjectDesignSystemPreserveNativeArchiveMetadata`、`TestProjectDesignSystemDraftUpsertLeavesSavedUntouched`、`TestSaveProjectDesignSystemCopiesDraftAndDeletesDraftAtomically`。
- [ ] Characterization/RED 与 GREEN 命令完全采用 Phase A 计划 Tasks 1-8；本依赖验收只执行汇总命令：

```bash
(cd server && go test ./internal/projectdesignsystem ./internal/designpreview ./internal/daemon ./internal/handler ./cmd/server -count=1)
pnpm --filter @multica/core test
pnpm --filter @multica/views test
pnpm typecheck
```

预期：全部 PASS 且不是 `no tests to run`。需要 gofmt 时，只格式化 Phase A 计划当步明确修改的 `.go` 文件，不对目录运行 gofmt。每个 Phase A commit 按独立计划运行 detect_changes；无差异不创建空 commit。

### Task 2: Phase A 文档与硬确认

**Files:** `docs/product/design-center/project-design-system-validation.md`、`README.md`、`decision-register.md`。

- [ ] 写入实际命令、fixture/archive digest、DB 断言和失败；明确：handler config/enqueue/context/no Run 与 daemon V2 finalizer 是两组独立证据，不构成 env-unset 端到端成功；已通过自动化安全/V2/Audit/受控 Preview/draft/adjust/save/discard；未验证真实 CRM Agent、grounding、用户 Chrome；不构成 full acceptance。不得复用 V1 OD 现场证据。
- [ ] `rg -n '低令牌|未验证|full acceptance|CRM|Chrome' docs/product/design-center/{project-design-system-validation.md,README.md,decision-register.md}` 后提交 `docs(design): record native v2 phase one closure`。
- [ ] **PHASE A GATE:** 用户/发布负责人明确确认“Phase A 低令牌收口通过，可进入 Phase B”；否则停止。

## Phase B：不可逆移除

### Task 3: 第一依赖，纯绿迁移四个 Preview helper

**Files:** `server/internal/handler/project_design_system_open_design_preview.go`、`server/internal/handler/project_design_system_package_preview.go`、`server/internal/handler/project_design_system_open_design_preview_test.go`、`server/internal/handler/project_design_system_package_preview_test.go`。

- [ ] Impact：四个旧 symbol、`GetProjectDesignSystemPackagePreview`、`GetProjectDesignSystemPackagePreviewFile`。
- [ ] 基线 GREEN：`(cd server && go test ./internal/handler -run 'PackagePreview|ArchivePreview|AccessToken|ProjectDesignSystemResponseRendersV2' -count=1 -v)`。
- [ ] 从旧文件迁入 `project_design_system_package_preview.go` 并中性重命名：

```go
issuePackagePreviewAccessToken
validatePackagePreviewAccessToken
signPackagePreviewAccessToken
(*Handler).packagePreviewResourceScope
```

保持 token wire bytes/version/lifetime 不变；更新 V2/旧测试调用。
- [ ] GREEN 后确认 `rg -n 'issueOpenDesignArchivePreviewAccessToken|validateOpenDesignArchivePreviewAccessToken|signOpenDesignArchivePreviewAccessToken|openDesignArchivePreviewResourceScope' server/internal/handler` 无输出。
- [ ] detect_changes 后提交 `refactor(design): neutralize package preview tokens`。

### Task 4: 先写替代 RED 合同

**Files:** `server/cmd/server/router_open_design_test.go`、`server/internal/handler/project_design_system_package_preview_test.go`、`server/internal/handler/project_design_system_completion_v2_test.go`、`server/internal/handler/project_design_system_test.go`、创建 `server/cmd/migrate/open_design_v1_removal_test.go`。

- [ ] Impact：`NewRouter`、`GetProjectDesignSystemPackagePreview`、`prepareProjectDesignSystemCompletion`、`marshalProjectDesignSystemTaskContext`。
- [ ] 在 `router_open_design_test.go` 增加 concrete 404 contract：

```go
func TestLegacyOpenDesignRoutesReturnNotFound(t *testing.T) {
	router := NewRouter(nil, realtime.NewHub(), events.New(), analytics.NoopClient{}, nil)
	legacyPatterns := map[string]bool{
		"GET /api/daemon/tasks/{taskId}/open-design/base-archive": false,
		"POST /api/daemon/tasks/{taskId}/open-design/preflight": false,
		"POST /api/daemon/tasks/{taskId}/open-design/start": false,
		"POST /api/daemon/tasks/{taskId}/open-design/events": false,
		"POST /api/daemon/tasks/{taskId}/open-design/archive": false,
		"POST /api/daemon/tasks/{taskId}/open-design/result": false,
		"POST /api/daemon/tasks/{taskId}/open-design/audit": false,
		"POST /api/daemon/tasks/{taskId}/open-design/preview": false,
		"POST /api/daemon/tasks/{taskId}/open-design/terminal": false,
		"GET /api/project-design-systems/{id}/open-design-preview": false,
		"GET /api/project-design-systems/{id}/open-design-runs/{runId}/evidence": false,
	}
	retainedPatterns := map[string]bool{
		"POST /api/daemon/tasks/{taskId}/project-design-system/package": false,
		"GET /api/daemon/tasks/{taskId}/project-design-system/base-package": false,
		"GET /api/project-design-systems/{id}/package-preview": false,
		"GET /api/project-design-system-previews/{workspaceId}/{systemId}/{digest}/{accessToken}/files/*": false,
		"POST /api/project-design-systems/{id}/preview-verification": false,
	}
	if err := chi.Walk(router, func(method, route string, _ http.Handler, _ ...func(http.Handler) http.Handler) error {
		key := method + " " + route
		if _, exists := legacyPatterns[key]; exists { legacyPatterns[key] = true }
		if _, exists := retainedPatterns[key]; exists { retainedPatterns[key] = true }
		return nil
	}); err != nil { t.Fatalf("walk router: %v", err) }
	for pattern, found := range legacyPatterns {
		if found { t.Errorf("legacy route remains registered: %s", pattern) }
	}
	for pattern, found := range retainedPatterns {
		if !found { t.Errorf("retained V2 route missing: %s", pattern) }
	}

	httpCases := []struct {
		name, method, path string
	}{
		{"base archive", http.MethodGet, "/api/daemon/tasks/00000000-0000-4000-8000-000000000001/open-design/base-archive"},
		{"preflight", http.MethodPost, "/api/daemon/tasks/00000000-0000-4000-8000-000000000001/open-design/preflight"},
		{"start", http.MethodPost, "/api/daemon/tasks/00000000-0000-4000-8000-000000000001/open-design/start"},
		{"events", http.MethodPost, "/api/daemon/tasks/00000000-0000-4000-8000-000000000001/open-design/events"},
		{"archive", http.MethodPost, "/api/daemon/tasks/00000000-0000-4000-8000-000000000001/open-design/archive"},
		{"result", http.MethodPost, "/api/daemon/tasks/00000000-0000-4000-8000-000000000001/open-design/result"},
		{"audit", http.MethodPost, "/api/daemon/tasks/00000000-0000-4000-8000-000000000001/open-design/audit"},
		{"preview", http.MethodPost, "/api/daemon/tasks/00000000-0000-4000-8000-000000000001/open-design/preview"},
		{"terminal", http.MethodPost, "/api/daemon/tasks/00000000-0000-4000-8000-000000000001/open-design/terminal"},
		{"preview alias", http.MethodGet, "/api/project-design-systems/00000000-0000-4000-8000-000000000001/open-design-preview"},
		{"evidence", http.MethodGet, "/api/project-design-systems/00000000-0000-4000-8000-000000000001/open-design-runs/00000000-0000-4000-8000-000000000002/evidence"},
	}
	for _, tt := range httpCases {
		t.Run(tt.name, func(t *testing.T) {
			recorder := httptest.NewRecorder()
			request := httptest.NewRequest(tt.method, tt.path, nil)
			router.ServeHTTP(recorder, request)
			if recorder.Code != http.StatusNotFound {
				t.Fatalf("%s %s status = %d, want 404; body = %s", tt.method, tt.path, recorder.Code, recorder.Body.String())
			}
		})
	}
}
```

在同文件增加 retained V2 behavior tests：未认证请求访问 package Preview、resource 和 daemon upload/base routes 必须得到现有 auth boundary 的 401/403/404；使用现有 authenticated project fixture 请求 `/api/project-design-systems/{id}/package-preview` 必须到达 `GetProjectDesignSystemPackagePreview` 并返回 fixture 对应的 200/schema；daemon token fixture 请求 package upload/base 必须到达对应 handler。`chi.Walk` 只证明注册表，不能替代 auth/handler behavior。

- [ ] 在 handler test 增加 non-V2 table contract。每个 case 以现有 `project_design_system_package` fixture 写入 `package_schema`（missing case 用 task completion payload 缺失 schema，而非违反 DB NOT NULL），调用实际 completion/response/Preview boundary，并断言没有 draft upsert、现有 saved digest 不变、HTTP 400/409 且 error code 为 `project_design_system_package_schema_unsupported`：

```go
v1Schema := "multica.project-design-system/v1"
openDesignSchema := "multica.open-design-draft-package/v1"
legacySchema := "legacy"
tests := []struct {
	name   string
	schema *string
}{
	{"v1", &v1Schema},
	{"open design", &openDesignSchema},
	{"legacy", &legacySchema},
	{"missing", nil},
}
for _, tt := range tests {
	t.Run(tt.name, func(t *testing.T) {
		fixture := createProjectDesignSystemCompletionFixture(t, service.ProjectDesignSystemGenerate)
		queries := db.New(testPool)
		upsertProjectDesignSystemPackageForTest(t, queries, fixture.System.ID, "saved", "saved-before-schema-rejection", strings.Repeat("s", 64))
		response := completeProjectDesignSystemTaskWithSchemaForTest(t, fixture.TaskID, tt.schema)
		assertProjectDesignSystemErrorCode(t, response, http.StatusConflict, "project_design_system_package_schema_unsupported")
		if _, err := queries.GetProjectDesignSystemPackageBySlot(context.Background(), db.GetProjectDesignSystemPackageBySlotParams{DesignSystemID: fixture.System.ID, Slot: "draft", WorkspaceID: fixture.System.WorkspaceID}); !errors.Is(err, pgx.ErrNoRows) {
			t.Fatalf("draft exists after schema rejection: %v", err)
		}
		assertProjectDesignSystemPackageDigest(t, queries, fixture.System.ID, "saved", strings.Repeat("s", 64))
	})
}
```

`completeProjectDesignSystemTaskWithSchemaForTest` 必须构造真实 `TaskCompleteRequest` JSON：schema 非 nil 时写入 `project_design_system_package.schema_version`，nil 时完全省略该字段；使用 `completeProjectDesignSystemTaskForTest` 发给 `testHandler.CompleteTask`，不得直接调用 validator。DB migration 后 non-V2 row insert 由 migration test 断言 CHECK violation。

- [ ] 在 `project_design_system_package_preview_test.go` 增加 standalone test，明确复用现有 `newNativeV2CompletionFixture`、`fixture.completeTask`、`fixture.buildPackagePayload`、`performProjectDesignSystemIDRequest` 和 Phase A 的 `performNativePackagePreviewFileRequest`：

```go
func TestV2PackagePreviewWorksWithoutLegacyValidator(t *testing.T) {
	fixture := newNativeV2CompletionFixture(t, service.ProjectDesignSystemGenerate)
	completed := fixture.completeTask(t, fixture.buildPackagePayload(t, nil))
	if completed.Code != http.StatusOK {
		t.Fatalf("complete V2 package: status = %d, body = %s", completed.Code, completed.Body.String())
	}
	_, err := testPool.Exec(context.Background(), `
		UPDATE project_design_system_package
		SET design_md = 'not markdown', tokens_css = 'not css', components_html = '<script>legacy()</script>'
		WHERE design_system_id = $1 AND slot = 'draft'`, fixture.Completion.System.ID)
	if err != nil {
		t.Fatalf("poison legacy columns: %v", err)
	}
	systemID := uuidToString(fixture.Completion.System.ID)
	manifest := performProjectDesignSystemIDRequest(t, testHandler.GetProjectDesignSystemPackagePreview, http.MethodGet, "/api/project-design-systems/"+systemID+"/package-preview", systemID, nil)
	if manifest.Code != http.StatusOK {
		t.Fatalf("V2 preview manifest: status = %d, body = %s", manifest.Code, manifest.Body.String())
	}
	for _, artifactPath := range []string{"ui-kit/index.html", "tokens.css", "assets/crm-mark.svg"} {
		file := performNativePackagePreviewFileRequest(t, fixture, artifactPath)
		if file.Code != http.StatusOK {
			t.Fatalf("V2 preview %s: status = %d, body = %s", artifactPath, file.Code, file.Body.String())
		}
	}
}
```

该测试在删除 `projectdesignsystem.Validate`/`BuildPreviewHTML` 和 `internal/opendesign` 后仍必须通过，作为 live V2 endpoint 无 legacy validator 依赖的删除门。

- [ ] `server/cmd/migrate/open_design_v1_removal_test.go` 使用同 package `main`，直接调用真实 `runMigrations`。详细 fresh/upgrade/rollback/replay fixtures 见 Task 6；本 Task 先让合同编译并因 877 不存在而 RED。

- [ ] RED：

```bash
(cd server && go test ./cmd/server -run '^TestLegacyOpenDesignRoutesReturnNotFound$' -count=1 -v)
(cd server && go test ./internal/handler -run '^(TestProjectDesignSystemRejectsEveryNonV2PackageSchema|TestV2PackagePreviewWorksWithoutLegacyValidator)$' -count=1 -v)
(cd server && go test ./cmd/migrate -run '^TestMigration877' -count=1 -v)
```

预期：404/schema/migration FAIL；V2 standalone 必须 GREEN，否则先修复且禁止删除旧 validator。
- [ ] 提交测试 `test(design): define v2-only removal contracts`；此 commit 不可发布。

### Task 5: 数据预检与幂等对象清理

**Files:** 创建并保留 `server/cmd/legacy-design-cleanup/main.go`、`server/cmd/legacy-design-cleanup/main_test.go`。它们必须包含在 Phase B release，并保留到生产 rollout 完整结束；删除只能由后续单独批准的 cleanup release 执行，本计划不删除。

- [ ] Impact：`storage.Storage.DeleteObject`、`storage.NewS3StorageFromEnv`、`storage.NewLocalStorageFromEnv`、`migrations.AllVersions`。CLI 必须与项目设计 package upload 使用的 `Handler.Storage` 完全相同：S3 优先、否则 local。不得调用 `storage.NewQiniuStorageFromEnv`，不得使用 `DesignAssetStorage`；Qiniu 只用于 design assets，不属于 project-design archive storage。

```go
type objectDeleter interface {
	DeleteObject(ctx context.Context, key string) error
}

type legacySnapshotStore interface {
	LoadLegacyCleanupSnapshot(ctx context.Context) (LegacyCleanupSnapshot, error)
}

type operatorAttestation struct {
	WritesStopped bool
	TasksDrained  bool
	BackupID      string
}

type LegacyObjectSource struct {
	Kind           string `json:"kind"` // open_design_run or project_design_system_package
	RecordID       string `json:"record_id"`
	WorkspaceID    string `json:"workspace_id"`
	ProjectID      string `json:"project_id"`
	DesignSystemID string `json:"design_system_id"`
	ObjectKey      string `json:"object_key"`
}

type LegacyCleanupSnapshot struct {
	OpenDesignRunCount int64
	LegacyPackageCount int64
	ProjectIDs         []string
	DesignSystemIDs    []string
	UnknownSchemas     []string
	ActiveRunIDs       []string
	ActiveTaskIDs      []string
	Objects            []LegacyObjectSource
	V2ObjectKeys       []string
	V2ObjectKeyConflicts []string
}

type LegacyCleanupReport struct {
	SchemaVersion     string               `json:"schema_version"`
	GeneratedAt       string               `json:"generated_at"`
	DatabaseIdentity  string               `json:"database_identity_sha256"`
	SnapshotDigest    string               `json:"snapshot_sha256"`
	OpenDesignRuns    int64                `json:"open_design_run_count"`
	LegacyPackages    int64                `json:"legacy_package_count"`
	Projects          int                  `json:"project_count"`
	DesignSystems     int                  `json:"design_system_count"`
	UnknownSchemas    []string             `json:"unknown_schemas"`
	ActiveRunIDs      []string             `json:"active_run_ids"`
	ActiveTaskIDs     []string             `json:"active_task_ids"`
	ObjectKeys        []string             `json:"object_keys"`
	ObjectSources     []LegacyObjectSource `json:"object_sources"`
	V2ObjectKeys      []string             `json:"v2_object_keys"`
	V2ObjectKeyConflicts []string          `json:"v2_object_key_conflicts"`
}

type LegacyCleanupResult struct {
	ObjectKey string `json:"object_key"`
	Status    string `json:"status"` // deleted or already_missing
}

type LegacyCleanupReceipt struct {
	SchemaVersion  string                `json:"schema_version"`
	CompletedAt    string                `json:"completed_at"`
	SnapshotDigest string                `json:"snapshot_sha256"`
	WritesStopped  bool                  `json:"writes_stopped_attested"`
	TasksDrained   bool                  `json:"tasks_drained_attested"`
	BackupID       string                `json:"backup_id_attested"`
	Results        []LegacyCleanupResult `json:"results"`
}

func buildLegacyCleanupReport(snapshot LegacyCleanupSnapshot, databaseIdentity string, generatedAt time.Time) (LegacyCleanupReport, error)
func executeLegacyCleanup(ctx context.Context, store legacySnapshotStore, deleter objectDeleter, report LegacyCleanupReport, attestation operatorAttestation) (LegacyCleanupReceipt, error)
```

`LoadLegacyCleanupSnapshot` 执行两个明确来源查询并 union 到内存：

```sql
SELECT id, workspace_id, project_id, design_system_id, archive_object_key
FROM open_design_run
WHERE archive_object_key IS NOT NULL AND archive_object_key <> '';

SELECT package.id, system.workspace_id, system.project_id,
       package.design_system_id, package.archive_object_key
FROM project_design_system_package AS package
JOIN project_design_system AS system ON system.id = package.design_system_id
WHERE package.package_schema IS DISTINCT FROM 'multica.project-design-system/v2'
  AND package.archive_object_key IS NOT NULL
  AND package.archive_object_key <> '';

SELECT DISTINCT archive_object_key
FROM project_design_system_package
WHERE package_schema = 'multica.project-design-system/v2'
  AND archive_object_key IS NOT NULL
  AND archive_object_key <> ''
ORDER BY archive_object_key;
```

旧对象 key 与 V2 key 分别去空、去重、字典序排序；`ObjectSources` 保留所有旧来源映射。`V2ObjectKeyConflicts` 是 `ObjectKeys ∩ V2ObjectKeys` 的排序结果。active Run 是 `status NOT IN ('preflight_failed','canceled','agent_failed','audit_failed','preview_failed','succeeded')`；active task 是相关 design-system task `status NOT IN ('completed','failed','cancelled','canceled')`。unknown schema 是非空且不属于 v1/OpenDesign/legacy/V2 的 distinct 值；任一 active/unknown/conflict 使 preflight exit nonzero，禁止生成可执行 cleanup report和删除对象。
- [ ] Report digest：对不含 `GeneratedAt` 和 `SnapshotDigest` 的规范化 report（包括排序后的 `V2ObjectKeys` 和 `V2ObjectKeyConflicts`）`json.Marshal` 后 SHA-256；cleanup 重新查询 snapshot、重算 digest，必须等于 report `SnapshotDigest`，否则返回 `legacy cleanup snapshot changed`，零对象删除。
- [ ] `--writes-stopped`、`--tasks-drained` 和 `--backup-id` 是**操作员证明/attestation**，CLI 无法从 DB 推断 gateway 已停写或备份可恢复。CLI 只要求三个显式输入（backup ID trim 后非空）、把它们写入 cleanup receipt；DB active counts 只是额外 fail-closed 检查，不得输出“已自动验证备份/停写”。
- [ ] RED test skeletons 使用 fake store/deleter，不访问真实对象：

```go
type recordingDeleter struct {
	keys []string
	err  map[string]error
}

type countingSnapshotStore struct {
	snapshot LegacyCleanupSnapshot
	loads    int
}
func (s *countingSnapshotStore) LoadLegacyCleanupSnapshot(context.Context) (LegacyCleanupSnapshot, error) {
	s.loads++
	return s.snapshot, nil
}

func safeSnapshot(keys ...string) LegacyCleanupSnapshot {
	objects := make([]LegacyObjectSource, 0, len(keys))
	for index, key := range keys {
		objects = append(objects, LegacyObjectSource{Kind: "open_design_run", RecordID: fmt.Sprintf("run-%d", index), ObjectKey: key})
	}
	return LegacyCleanupSnapshot{OpenDesignRunCount: int64(len(objects)), ProjectIDs: []string{"project-1"}, DesignSystemIDs: []string{"system-1"}, Objects: objects}
}

func mustReport(t *testing.T, snapshot LegacyCleanupSnapshot) LegacyCleanupReport {
	t.Helper()
	report, err := buildLegacyCleanupReport(snapshot, "db-fingerprint", time.Unix(0, 0).UTC())
	if err != nil { t.Fatal(err) }
	return report
}
func (d *recordingDeleter) DeleteObject(_ context.Context, key string) error {
	d.keys = append(d.keys, key)
	return d.err[key]
}

func TestBuildLegacyCleanupReportEnumeratesDeduplicatesAndSortsBothSources(t *testing.T) {
	snapshot := LegacyCleanupSnapshot{
		OpenDesignRunCount: 2, LegacyPackageCount: 2,
		ProjectIDs: []string{"project-b", "project-a", "project-a"},
		DesignSystemIDs: []string{"system-b", "system-a"},
		Objects: []LegacyObjectSource{
			{Kind: "open_design_run", RecordID: "run-1", ObjectKey: "objects/b.zip"},
			{Kind: "project_design_system_package", RecordID: "package-1", ObjectKey: "objects/a.zip"},
			{Kind: "open_design_run", RecordID: "run-2", ObjectKey: "objects/a.zip"},
		},
		V2ObjectKeys: []string{"objects/v2-b.zip", "objects/v2-a.zip"},
	}
	report, err := buildLegacyCleanupReport(snapshot, "db-fingerprint", time.Unix(0, 0).UTC())
	if err != nil { t.Fatal(err) }
	if diff := cmp.Diff([]string{"objects/a.zip", "objects/b.zip"}, report.ObjectKeys); diff != "" { t.Fatal(diff) }
	if report.Projects != 2 || report.DesignSystems != 2 || report.SnapshotDigest == "" { t.Fatalf("report = %+v", report) }
	if diff := cmp.Diff([]string{"objects/v2-a.zip", "objects/v2-b.zip"}, report.V2ObjectKeys); diff != "" { t.Fatal(diff) }
	if len(report.V2ObjectKeyConflicts) != 0 { t.Fatalf("unexpected conflicts: %v", report.V2ObjectKeyConflicts) }
}

func TestBuildLegacyCleanupReportRejectsActiveAndUnknownState(t *testing.T) {
	for _, snapshot := range []LegacyCleanupSnapshot{
		{ActiveRunIDs: []string{"run-active"}},
		{ActiveTaskIDs: []string{"task-active"}},
		{UnknownSchemas: []string{"future/schema"}},
		{Objects: []LegacyObjectSource{{ObjectKey: "objects/shared.zip"}}, V2ObjectKeys: []string{"objects/shared.zip"}},
	} {
		if _, err := buildLegacyCleanupReport(snapshot, "db", time.Unix(0, 0)); err == nil { t.Fatal("unsafe snapshot accepted") }
	}
}

func TestLoadLegacyCleanupSnapshotDetectsDatabaseV2ObjectKeyConflict(t *testing.T) {
	ctx := context.Background()
	pool, identity := newIsolatedLegacyCleanupDatabase(t, "multica_legacy_cleanup_conflict_20260812")
	t.Logf("database identity: %+v", identity)
	fixture := seedLegacyCleanupDatabaseFixture(t, pool)
	if _, err := pool.Exec(ctx, `UPDATE open_design_run SET archive_object_key = 'objects/shared.zip' WHERE id = $1`, fixture.RunID); err != nil { t.Fatal(err) }
	if _, err := pool.Exec(ctx, `UPDATE project_design_system_package SET archive_object_key = 'objects/shared.zip' WHERE id = $1 AND package_schema = 'multica.project-design-system/v2'`, fixture.V2PackageID); err != nil { t.Fatal(err) }
	snapshot, err := (&postgresLegacySnapshotStore{pool: pool}).LoadLegacyCleanupSnapshot(ctx)
	if err != nil { t.Fatal(err) }
	if diff := cmp.Diff([]string{"objects/shared.zip"}, snapshot.V2ObjectKeyConflicts); diff != "" { t.Fatal(diff) }
	if _, err := buildLegacyCleanupReport(snapshot, "db-fingerprint", time.Unix(0, 0).UTC()); err == nil { t.Fatal("V2 object-key conflict was accepted") }
	deleter := &recordingDeleter{}
	if len(deleter.keys) != 0 { t.Fatalf("DeleteObject called: %v", deleter.keys) }
}

func TestLegacyCleanupReportDigestChangesWithV2ObjectKeys(t *testing.T) {
	left := safeSnapshot("objects/legacy.zip")
	right := safeSnapshot("objects/legacy.zip")
	left.V2ObjectKeys = []string{"objects/v2-a.zip"}
	right.V2ObjectKeys = []string{"objects/v2-b.zip"}
	if mustReport(t, left).SnapshotDigest == mustReport(t, right).SnapshotDigest {
		t.Fatal("V2 object keys were omitted from snapshot digest")
	}
}

func TestCleanupRejectsStaleSnapshotBeforeDelete(t *testing.T) {
	report := mustReport(t, safeSnapshot("objects/a.zip"))
	deleter := &recordingDeleter{}
	store := &countingSnapshotStore{snapshot: safeSnapshot("objects/b.zip")}
	_, err := executeLegacyCleanup(context.Background(), store, deleter, report, operatorAttestation{WritesStopped: true, TasksDrained: true, BackupID: "backup-20260812"})
	if err == nil || len(deleter.keys) != 0 { t.Fatalf("err = %v, deleted = %v", err, deleter.keys) }
}

func TestCleanupRequiresOperatorAttestations(t *testing.T) {
	tests := []operatorAttestation{
		{WritesStopped: false, TasksDrained: true, BackupID: "backup-20260812"},
		{WritesStopped: true, TasksDrained: false, BackupID: "backup-20260812"},
		{WritesStopped: true, TasksDrained: true, BackupID: "   "},
	}
	for _, attestation := range tests {
		store := &countingSnapshotStore{snapshot: safeSnapshot("objects/a.zip")}
		deleter := &recordingDeleter{}
		_, err := executeLegacyCleanup(context.Background(), store, deleter, mustReport(t, store.snapshot), attestation)
		if err == nil || store.loads != 0 || len(deleter.keys) != 0 {
			t.Fatalf("attestation accepted: %+v, loads=%d, deleted=%v", attestation, store.loads, deleter.keys)
		}
	}
}

func TestCleanupDeletesReportedKeysAndIsIdempotent(t *testing.T) {
	store := &countingSnapshotStore{snapshot: safeSnapshot("objects/b.zip", "objects/a.zip")}
	report := mustReport(t, store.snapshot)
	deleter := &recordingDeleter{}
	attestation := operatorAttestation{WritesStopped: true, TasksDrained: true, BackupID: "backup-20260812"}
	for run := 0; run < 2; run++ {
		receipt, err := executeLegacyCleanup(context.Background(), store, deleter, report, attestation)
		if err != nil { t.Fatalf("run %d: %v", run, err) }
		if receipt.SnapshotDigest != report.SnapshotDigest || receipt.BackupID != attestation.BackupID { t.Fatalf("receipt = %+v", receipt) }
	}
	want := []string{"objects/a.zip", "objects/b.zip", "objects/a.zip", "objects/b.zip"}
	if diff := cmp.Diff(want, deleter.keys); diff != "" { t.Fatal(diff) }
}

func TestCleanupStopsOnDeleteFailureAndNeverMutatesDatabase(t *testing.T) {
	store := &countingSnapshotStore{snapshot: safeSnapshot("objects/a.zip", "objects/b.zip")}
	deleter := &recordingDeleter{err: map[string]error{"objects/b.zip": errors.New("delete failed")}}
	_, err := executeLegacyCleanup(context.Background(), store, deleter, mustReport(t, store.snapshot), operatorAttestation{WritesStopped: true, TasksDrained: true, BackupID: "backup-20260812"})
	if err == nil { t.Fatal("delete failure was ignored") }
	if diff := cmp.Diff([]string{"objects/a.zip", "objects/b.zip"}, deleter.keys); diff != "" { t.Fatal(diff) }
	// legacySnapshotStore intentionally has no Exec/Delete method; DB mutation
	// is impossible through executeLegacyCleanup's type boundary.
}
```
- [ ] CLI：

```text
(cd server && go run ./cmd/legacy-design-cleanup preflight --report /tmp/multica-open-design-v1-preflight.json)
(cd server && test -n "$MULTICA_LEGACY_BACKUP_ID" && go run ./cmd/legacy-design-cleanup cleanup --report /tmp/multica-open-design-v1-preflight.json --writes-stopped --tasks-drained --backup-id "$MULTICA_LEGACY_BACKUP_ID")
```

报告含时间、无凭据 DB fingerprint、counts、unknown schemas、active counts、旧/V2 sorted keys、conflicts、SHA-256。cleanup 有限重试 `DeleteObject`；local/S3 的 missing delete 已幂等。任一失败 exit nonzero，绝不 SQL DELETE。
- [ ] RED/GREEN：`(cd server && go test ./cmd/legacy-design-cleanup -count=1 -v)`；只对 `server/cmd/legacy-design-cleanup/main.go` 和 `main_test.go` 运行 gofmt；构建命令 `(cd server && go build -o /tmp/multica-legacy-design-cleanup ./cmd/legacy-design-cleanup)`，再 `shasum -a 256 /tmp/multica-legacy-design-cleanup`。
- [ ] 提交 `feat(design): add legacy data cleanup preflight`。
- [ ] Rollout：停 design writes；等待相关 tasks 终态；DB+object backup；preflight unknown/active=0 且 `V2ObjectKeyConflicts=[]`；dry-run；真实删除；同 report 重跑幂等。V2 key 冲突由工具自动查询和拒绝，不以人工目测替代。任一失败保持 DB 行并禁止 877。

### Task 6: migration 877 与 fresh/upgrade tests

**Files:** 创建 `server/migrations/877_drop_open_design_v1_legacy.up.sql`、`server/migrations/877_drop_open_design_v1_legacy.down.sql`；创建/修改 `server/cmd/migrate/open_design_v1_removal_test.go`；修改 `server/internal/handler/project_design_cleanup_test.go`、`server/internal/handler/workspace_design_cleanup_test.go`。

- [ ] Impact：`runMigrations`、`migrations.Files`、`migrations.ExtractVersion`、两个 project/workspace cleanup tests。
- [ ] `TestMigration877FreshDatabaseIsV2Only` 用 `fmt.Sprintf("multica_legacy_removal_fresh_20260812_%d", os.Getpid())` 生成唯一数据库名；`TestMigration877UpgradeDeletesLegacyAndPreservesV2` 用 `fmt.Sprintf("multica_legacy_removal_upgrade_20260812_%d", os.Getpid())` 生成另一个数据库名。二者只能连接 localhost/127.0.0.1，测试启动时查询并记录 `current_database()`、`inet_server_addr()`、`inet_server_port()`、`current_setting('server_version')`，断言两 DB 名不同且都不等于 handler/dev DB。每个测试独立 create/drop 自己的 DB，不共享 pool/schema/schema_migrations。
- [ ] Fresh test 通过真实 `runMigrations(ctx,pool,runOptions{Direction:"up",Files:migrations.Files("up"),SchemaMigrationsTable:"schema_migrations",AdvisoryLockKey:uniqueLock})` 从零执行；upgrade test 用同一 runner 仅 apply through 876，seed 五个 projects/systems：一个 system 的 V2 draft/saved（不同全部证据字段），三个 systems 分别 v1/OpenDesign/legacy，第五个 old-only system 只含 legacy package；seed 两个 Run 和旧 indexes，再用 runner apply 877。断言 non-V2/Run/index 删除、V2 全字段不变、CHECK 只接受 V2。
- [ ] `TestMigration877FailedStatementRollsBackAllChanges` 在 upgrade DB 的独立 clone 中把真实 877 SQL 追加 `SELECT 1/0;` 后作为**单次 `conn.Exec` migration file**交给 `runMigrations`；断言返回 error、旧 package/table/index/旧 CHECK 全部仍存在、V2 rows 不变、`schema_migrations` 不含 877。该测试锁定 PostgreSQL 对单次 multi-statement Exec 的隐式 transaction 语义。
- [ ] `TestMigration877ReplaysWhenSchemaMigrationRecordIsMissing` 在 upgrade DB 中先用单次 `conn.Exec` 成功执行真实 877 SQL但不插 `schema_migrations` record，再调用 `runMigrations`；断言 runner 安全重放并记录 877，V2 rows 不变、table/index 仍 absent、V2-only CHECK 仍存在；第二次 runner 调用因 record 存在 skip 且状态不变。
- [ ] RED：`(cd server && go test ./cmd/migrate -run '^TestMigration877' -count=1 -v)`。
- [ ] `server/migrations/877_drop_open_design_v1_legacy.up.sql` 使用以下完整 SQL。真实 schema 中旧兼容元数据只存在于 non-V2 `project_design_system_package` 行和 `open_design_run` 行；没有第三张兼容 metadata 表，因此不发明额外 DELETE：

```sql
DELETE FROM project_design_system_package
WHERE package_schema IS DISTINCT FROM 'multica.project-design-system/v2';

DROP INDEX IF EXISTS idx_open_design_run_design_system;
DROP INDEX IF EXISTS idx_open_design_run_workspace_status;
DROP TABLE IF EXISTS open_design_run;

ALTER TABLE project_design_system_package
    DROP CONSTRAINT IF EXISTS project_design_system_package_schema_check;

ALTER TABLE project_design_system_package
    ADD CONSTRAINT project_design_system_package_schema_check
    CHECK (package_schema = 'multica.project-design-system/v2');
```

无 FK、`REFERENCES`、`CASCADE`。`IS DISTINCT FROM` 明确覆盖 NULL，即使 873 后 `NOT NULL` 令新 NULL 不可插入，upgrade SQL 仍对异常历史状态 fail closed。
- [ ] migration lint 要求 up/down 配对；`server/migrations/877_drop_open_design_v1_legacy.down.sql` 必须只有以下不可逆 no-op comment，不得包含任何 SQL statement，不得重建 table、constraint 或 index：

```sql
-- Irreversible by design: migration 877 permanently deletes legacy rows,
-- object references, indexes, and the open_design_run table. No down action.
```

Migration test 读取 down 文件，去除空白/注释后断言剩余内容为空；发布说明明确不能恢复数据/对象/代码。
- [ ] Upgrade 完成后，使用新 binary 的真实 `GetProjectDesignSystem` handler/API fixture 请求第五个 old-only project/system，断言 HTTP 200、`Status == "unestablished"`、`Content.Sections/TokenGroups/PreviewTargets` 长度均为 0、`Content.PreviewHTML == ""`、`HasUnsavedChanges == false`、`ActiveTask == nil`。该 API/status boundary test 连接 upgrade 专用 DB，不使用 handler/dev DB。
- [ ] GREEN：`(cd server && go test ./cmd/migrate -run '^TestMigration877' -count=1 -v)` 和 `(cd server && go test ./internal/handler -run 'TestDelete(Project|Workspace).*Design' -count=1 -v)`。只对本 Task 实际修改的三个 test files 运行 gofmt。
- [ ] 提交 `feat(db): remove legacy open design data`。

**不可部署依赖：** migration 877 在 Tasks 8-10 删除所有编译期/测试期 table 和 sqlc method 引用之前不可应用；Tasks 5-11（包括 retained cleanup tool）必须进入同一个 release artifact。禁止先运行 877 再部署代码，也禁止发布只含 migration/SQL cleanup 的中间 commit。

### Task 7: 删除 SQL/sqlc 和 cleanup CTE

**Files:** 删除 `server/pkg/db/queries/open_design.sql`、`server/pkg/db/generated/open_design.sql.go`；修改 `server/pkg/db/generated/models.go`、`server/pkg/db/queries/project.sql`、`server/pkg/db/queries/workspace_delete.sql`、`server/internal/handler/project_design_cleanup_test.go`、`server/internal/handler/workspace_design_cleanup_test.go`；regenerate `server/pkg/db/generated/`。

- [ ] Impact：`CreateOpenDesignRun`、`FinalizeOpenDesignRun`、`GetOpenDesignRunByTask`、`GetOpenDesignRunForEvidence`、`PersistOpenDesignRunDraft`、`DeleteProject`、`DeleteWorkspaceDesignDependents`。
- [ ] 移除 `deleted_open_design_runs` CTE/计数依赖，保留 package/system 显式清理；tests 不再 seed/count 已删表但继续验证 V2 tenant isolation。
- [ ] `make sqlc` 后 `rg -n 'type OpenDesignRun|CreateOpenDesignRun|GetOpenDesignRun|FinalizeOpenDesignRun|open_design_run' server/pkg/db/{generated,queries}` 无输出。
- [ ] cleanup tests GREEN；提交 `refactor(db): remove open design queries`。

### Task 8: 删除 daemon Worker/V1

**Files:** 删除 `server/internal/daemon/open_design_task.go`、`server/internal/daemon/open_design_task_test.go`、`server/internal/daemon/open_design_client_test.go`、`server/internal/daemon/project_design_system_artifacts_test.go`；修改 `server/internal/daemon/client.go`、`client_test.go`、`config.go`、`config_test.go`、`daemon.go`、`daemon_test.go`、`types.go`、`prompt.go`、`prompt_test.go`、`project_design_system_package.go`、`project_design_system_package_test.go`、`server/internal/daemon/execenv/context.go` 及其现有 project-design-system tests。

- [ ] Impact：`handleOpenDesignTask`、`Daemon.handleTask`、`LoadConfig`、`Client.CompleteTask`、`BuildPrompt`、`attachProjectDesignSystemArtifacts`、`readProjectDesignSystemArtifacts`、`writeProjectDesignSystemContext`。
- [ ] RED contracts：handleTask 只走 V2 finalizer；TaskResult 只带 V2 package；config 只保留 DesignPreviewBrowserPath；prompt/execenv 拒绝 non-v2。
- [ ] 删除 supervisor factory/分支、Worker HTTP/SSE client、旧 base、四个 OpenDesign config/env、V1 prompt、`ProjectDesignSystemArtifacts` 和 collector。保留 V2 finalize/Audit/Preview/upload 与通用 lifecycle。
- [ ] `(cd server && go test ./internal/daemon ./internal/daemon/execenv ./internal/designpreview ./internal/projectdesignsystem -count=1)` GREEN；活动旧符号 grep 为 0。gofmt 仅列出并格式化本 Task 实际修改的 Go 文件，不传目录。
- [ ] 提交 `refactor(daemon): remove open design worker lifecycle`。

### Task 9: 删除 handler/service V1/Run 与 `internal/opendesign`

**Files:** 修改 `server/internal/service/task.go`、`server/internal/service/project_design_system_task_test.go`；删除 `server/internal/handler/project_design_system_open_design.go`、`project_design_system_open_design_base.go`、`project_design_system_open_design_draft.go`、`project_design_system_open_design_evidence.go`、`project_design_system_open_design_lifecycle.go`、`project_design_system_open_design_preview.go` 及同名前缀 tests；修改 `server/internal/handler/project_design_system.go`、`project_design_system_completion.go`、`project_design_system_completion_test.go`、`project_design_system_package_preview.go`、`project_design_system_package_preview_test.go`、`daemon.go`、`daemon_test.go`、`handler.go`；删除 `server/internal/opendesign/**`。

- [ ] Impact：`ProjectDesignSystemTaskContext`、`TaskService.FailTask`、`TaskService.markOpenDesignRunTaskFailed`、`TaskService.markProjectDesignSystemTaskFailed`、`projectDesignSystemResponse`、`prepareProjectDesignSystemCompletion`、`persistProjectDesignSystemCompletion`、`GetProjectDesignSystemPackagePreview`、`GetProjectDesignSystemPackagePreviewFile`、`DownloadProjectDesignSystemBasePackage`、`CompleteTask`。
- [ ] 再跑 V2 standalone：必须 GREEN。随后删除 `OpenDesignRun` context、Run fail sync、prepare/persist、evidence/archive/V1 Validate/BuildPreviewHTML/inline completion/base fallback；completion/response/base/Preview 只接受 V2。
- [ ] `(cd server && go test ./internal/handler ./internal/service ./internal/projectdesignsystem ./internal/designpreview -run 'ProjectDesignSystem|PackagePreview|CompleteTask|V2|Native' -count=1)` GREEN。gofmt 仅格式化本 Task 实际修改的 Go 文件。
- [ ] `test ! -d server/internal/opendesign`；handler/service 旧 schema/Run/import grep 为 0。
- [ ] 提交 `refactor(design): remove legacy handlers and v1 completion`。

### Task 10: router 404 与 V2 route 保留

**Files:** `server/cmd/server/router.go`、`server/cmd/server/router_open_design_test.go`、`server/cmd/server/integration_test.go`。

- [ ] Impact `NewRouter`。
- [ ] 删除 OpenDesign env wiring、9 个 daemon routes、`open-design-preview` alias、evidence route。保留 V2 upload/base/package-preview/resource/preview-verification。使用 Task 4 的 `chi.Walk` map 精确断言 11 个旧 route patterns 全部未注册、5 个 V2 patterns 全部注册。
- [ ] `(cd server && go test ./cmd/server -run '^(TestLegacyOpenDesignRoutesReturnNotFound|TestRetainedProjectDesignSystemRoutesAreRegistered|TestRetainedProjectDesignSystemRoutesEnforceAuthAndReachHandlers|TestProjectDesignSystem.*|TestPackagePreview.*)$' -count=1 -v)`：输出必须包含前三个 exact contract tests；旧接口 HTTP 404，V2 route registration、auth 和 handler behavior 通过。gofmt 仅格式化三个实际修改的 router Go 文件。
- [ ] 提交 `refactor(api): remove legacy open design routes`。

### Task 11: core/views/env/五文档

**Files:** `.env.example`；`packages/core/types/design.ts`、`packages/core/api/schemas.ts`、`schemas.test.ts`、`client.ts`、`client.test.ts`、`packages/core/designs/keys.ts`、`keys.test.ts`；`packages/views/designs/project-design-system-canvas.tsx`、`project-design-system-canvas.test.tsx`、`project-design-system-preview.tsx`、`project-design-system-preview.test.tsx`；`docs/product/design-center/project-design-system-validation.md`、`README.md`、`decision-register.md`、`open-design-engine-integration.md`、`open-design-evidence.md`。

- [ ] Impact exact symbols：`ProjectDesignSystemPreview`、`ProjectDesignSystemCanvas`、`ProjectDesignSystemSchema`、`ProjectDesignSystemPackagePreviewSchema`、`ApiClient.getProjectDesignSystemPackagePreview`、`ApiClient.getProjectDesignSystemPackagePreviewFileURL`。
- [ ] RED TS contracts：non-v2 rejected；只用 package-preview；V2 archive 不调用 legacy verification；旧 response schema 不接受。
- [ ] 删除旧 types/schema/fixtures/alias；malformed/non-v2 fail closed，保留 `parseWithFallback`、V2 target/selection。env 删除 `MULTICA_OPEN_DESIGN_ENABLED`，保留设计 Preview path。
- [ ] 五文档：validation 记录 Phase A/B/统计/对象/fresh/upgrade/未执行；README 当前只写 V2；decision register 新增 confirmed 替代 DC-039 临时保留条款且保留原文；两份 OD 文档标历史非活动且不改写证据。
- [ ] GREEN：

```bash
pnpm --filter @multica/core exec vitest run api/schemas.test.ts api/client.test.ts designs/keys.test.ts
pnpm --filter @multica/views exec vitest run designs/project-design-system-canvas.test.tsx designs/project-design-system-preview.test.tsx
pnpm typecheck
```

- [ ] 提交 `refactor(design): expose native v2 contracts only`。

### Task 12: 清理工具、全量验证和证据

- [ ] 确认 `server/cmd/legacy-design-cleanup/main.go` 和 `main_test.go` 仍在 release diff/build context；本计划禁止删除。生产 rollout 完成后如需删除，另开批准 spec/plan/release。
- [ ] 按 `using-git-worktrees` 在执行时使用已隔离 worktree。migration tests 只接收一个本地 PostgreSQL 管理连接，并由测试自身创建、迁移、记录 identity 和删除两个 PID 后缀数据库。运行测试前打印并机械校验管理连接：

```bash
printf 'DATABASE_URL=%s POSTGRES_DB=%s POSTGRES_PORT=%s\n' "$DATABASE_URL" "$POSTGRES_DB" "$POSTGRES_PORT"
case "$DATABASE_URL" in postgres://*@localhost:*/*|postgres://*@127.0.0.1:*/*) ;; *) printf 'refusing non-local database\n' >&2; exit 1;; esac
test -n "$POSTGRES_DB"
test "$POSTGRES_DB" != "multica"
```

Expected: 管理连接 host 为 localhost/127.0.0.1，且当前 DB 不是共享 `multica` DB。把输出和测试将创建/删除的名称模式 `multica_legacy_removal_{fresh,upgrade}_20260812_<pid>` 交给用户/操作员，取得明确许可后才运行：

```bash
(cd server && go test ./cmd/migrate -run '^TestMigration877(FreshDatabaseIsV2Only|UpgradeDeletesLegacyAndPreservesV2|FailedStatementRollsBackAllChanges|ReplaysWhenSchemaMigrationRecordIsMissing|DownIsIrreversibleNoOp)$' -count=1 -v)
```

测试必须独占创建和删除 `multica_legacy_removal_fresh_20260812_<pid>` 与 `multica_legacy_removal_upgrade_20260812_<pid>`。fresh fixture 从零运行全部 migrations；upgrade fixture 使用真实 `runMigrations` 只执行 through 876，再 seed 数据并执行 877。测试证据记录两 DB 的 `current_database/inet_server_addr/inet_server_port/server_version`。不得运行 `make db-reset`，不得修改当前管理、handler 或开发数据库。
- [ ] 后端：`(cd server && go test ./internal/projectdesignsystem ./internal/designpreview ./internal/daemon ./internal/handler ./internal/service ./internal/migrations ./cmd/migrate ./cmd/server -count=1)`，再 `make test`。
- [ ] 前端：Task 11 定向命令、`pnpm typecheck`、`pnpm test`。
- [ ] 活动实现 clean grep（不扫 migrations、docs/spec 和三个必需负向合同测试文件）：

```bash
rg -n -i 'open.?design|open_design_run|MULTICA_OPEN_DESIGN|multica\.project-design-system/v1|multica\.open-design-draft-package/v1|ProjectDesignSystemArtifacts|components_html' server/cmd server/internal server/pkg/db/queries server/pkg/db/generated packages apps .env.example \
  --glob '!server/cmd/legacy-design-cleanup/**' \
  --glob '!server/cmd/server/router_open_design_test.go' \
  --glob '!server/internal/handler/project_design_system_test.go' \
  --glob '!server/cmd/migrate/open_design_v1_removal_test.go' \
  --glob '!server/internal/migrations/migrations_lint_test.go'
```

预期 0。
- [ ] Retained cleanup tool 单独审计：`server/cmd/legacy-design-cleanup/**` 中旧符号只能出现在 preflight 查询、V2 key conflict、防 stale snapshot、`DeleteObject` 和审计 receipt；不得出现 Worker client、Run execution、旧 Preview/V1 parse、compatibility read 或 fallback。运行其全包 tests 并确认 source 包含在 release artifact。
- [ ] 负向合同测试单独审计：`router_open_design_test.go` 只允许 11 个 absent/exact-404 cases 和 retained V2 route/auth behavior；`project_design_system_test.go` 只允许 non-V2 rejection table/error assertions；`cmd/migrate/open_design_v1_removal_test.go` 只允许 upgrade seed、rollback/replay、877 static assertions 和删除后断言。不得出现兼容成功、fallback 或旧 route registered 断言。
- [ ] 历史 migration/docs 分离审计：

```bash
rg -n -i 'open.?design|open_design_run|multica\.project-design-system/v1|multica\.open-design-draft-package/v1|legacy' server/migrations/870_open_design_run.* server/migrations/871_idx_open_design_run_design_system.* server/migrations/872_idx_open_design_run_workspace_status.* server/migrations/873_native_project_design_system_package.* server/migrations/877_drop_open_design_v1_legacy.* docs/superpowers docs/product/design-center
```

预期：870-872 保留旧表/index 历史；873 保留不可变旧 schema values；877 up 只包含显式 DELETE/DROP/V2 CHECK，877 down 只有不可逆 no-op comments；docs/spec 只把旧链路描述为历史、批准删除或证据。不得为达到零输出修改 870-873 或删除历史研究证据。
- [ ] 保留检查：`designpreview/**`、`projectdesignsystem/**` 存在；grep 可见 `DesignPreviewBrowserPath`、V2 schema、V2 upload/package Preview。
- [ ] 最终 `git diff --check`、`node .gitnexus/run.cjs detect-changes --scope compare --base-ref main --repo .`、`git status --short`。
- [ ] 自审批准 spec 全覆盖、无未填写项、无空测试、代码骨架类型/symbol 一致；五文档只写实测结果。detect_changes 后提交 `docs(design): record destructive legacy removal evidence`。

## 上线与数据丢失清单

- [ ] Phase A 已确认；严格/full acceptance 明确未完成。
- [ ] 核心 commits 作为一个整体审阅且无中间部署；Phase B artifact 包含 Tasks 5-11 和 retained cleanup tool。
- [ ] 停止设计体系 writes，drain 所有相关 tasks/旧 Runs；DB/object backup ID 与恢复负责人已记录。
- [ ] preflight counts、旧/V2 key lists、report SHA-256 已复核；unknown/active=0；`V2ObjectKeyConflicts=[]`。
- [ ] 旧对象真实删除和幂等重跑均成功，失败数 0，然后才运行 877。
- [ ] **离线 cutover 开始：** 停止所有旧 server replicas，等待连接/进程退出；停止所有旧 release 和人工 migration jobs，确认没有 migrator 持有或等待 advisory lock。此后不得重新启动旧 binary。
- [ ] 使用包含 Tasks 5-11 的新 release artifact 和 retained cleanup tool 运行 877；禁止旧/新 replicas rolling overlap，禁止 migration 与 server startup 并发。
- [ ] 877 成功后只启动新 server/daemon binaries；执行 V2 create/adjust/save/discard/package Preview smoke checks，确认 old-only project API 为 `unestablished`、空 content、无 unsaved changes、无 active task。
- [ ] smoke checks 通过后才恢复 V2 writes；任何失败保持 writes 关闭并向前修复或灾难恢复。
- [ ] 明确接受历史 Run/archive/evidence/Preview/三文件/旧链接永久失效，旧-only 项目未建立。
- [ ] fresh/upgrade、旧 API 404、V2 create/adjust/save/discard/Preview API+DB 闭环通过后才开放 V2 写流量。
- [ ] 已安装旧桌面客户端预期收到旧 API 404/不可用；监控 endpoint/client-version 调用量和错误率，只用于识别升级需求，不恢复 alias、兼容转换或 fallback。
- [ ] `server/cmd/legacy-design-cleanup` source 仍在已部署 release 和仓库中；生产 rollout 完成不自动删除，后续删除需单独批准 cleanup release。
- [ ] 877 前可整体回滚；对象删除或 877 开始后只允许备份灾难恢复或向前修复。down 不恢复数据，旧二进制不得连接清理后 DB。

只有两 Phase、证据预算、GitNexus、clean grep、五文档和本清单全部满足，才能声明“Native V2 独占运行链路与旧链路移除完成”；该声明仍不代表真实 CRM grounding 或原 Task 8 的严格/full acceptance。
