# Multica Native Project Design System Phase 1 Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** 将项目设计体系的首次生成、调整和重新生成切换到 Multica 原生 Agent 设计引擎，并以 `multica.project-design-system/v2`、安全归档、Package Audit 和真实浏览器 Preview 作为产生草稿的共同门禁。

**Architecture:** 继续使用现有 Project、Agent、daemon、`agent_task_queue`、对象存储、`project_design_system` 与 `draft/saved` 槽位。Agent 在隔离工作区完成语义理解和设计，daemon 负责有界收集、静态审计、Chrome 验证与上传，Server 重新校验任务、输入快照、基线包和内容 digest 后才原子替换 `draft`；现有 Open Design archive 只保留历史读取能力，不再进入新任务执行链路。

**Tech Stack:** Go 1.26.1, PostgreSQL 17, sqlc, Chi, existing daemon/provider runtimes, object storage, `chromedp`, React 19, TanStack Query, Zod, Vitest, local Chrome.

---

## Scope And Safety Boundary

- 本计划只实现 `docs/superpowers/specs/2026-08-05-multica-native-design-engine-design.md` 的阶段 1：原生项目设计体系闭环。
- 不实现设计中心首页、在线设计稿、社区模板、设计还原、MCP 或 Issue 交付。
- 不运行、分发或托管 Open Design Worker、Daemon 或 Runtime；不合并 `codex/open-design-native-slots`。
- 不新增通用 `design_run` 表。继续使用 `agent_task_queue` 作为执行记录，并复用 `project_design_system_package` 的 `draft` / `saved` 生命周期。
- `open_design_run` 和 Open Design 专属代码在本阶段不做破坏性删除；新任务不再创建 `open_design_run`，历史 archive 仍可读取。
- Agent 负责设计语义、视觉方向、布局和组件组织。Server/daemon 只负责确定性输入、边界、协议、校验、Preview、状态和权限，不增加关键词组件推导器。
- 新包最小事实内核为 `manifest.json`、`DESIGN.md`、`tokens.css`、`source/index.json`。项目设计体系任务还必须至少提供一个 `ui-kit/index.html` 或 `preview/*.html`，否则不能通过真实视觉门禁。
- Phase 1 的 HTML 静态且禁网：拒绝 Agent 脚本、事件属性、表单、嵌入页面、远程 URL 和 CSS `@import`。图片和字体必须进入包内 `assets/` / `fonts/`。
- 坏包不能创建或覆盖 `draft`；失败不能改变 `saved`。task `completed`、文件存在或 Agent 自评均不能代替静态 Audit 和 Chrome Preview。
- 调整任务绑定当前 `draft` 优先、否则 `saved` 的 immutable digest。调整失败保留调整前包；成功必须输出完整一致的新包。
- 所有新接口继续遵守安装版客户端兼容规则：服务端保留旧 Preview 路由，前端响应使用 Zod `parseWithFallback`，未知字段和历史包不能导致白屏。
- SQL in `server/pkg/db/queries/design.sql` must not contain a `JOIN` token; the GitLab pre-receive hook rejects the generated file.
- 主 checkout `feature/fengchen` 含用户改动，所有实现只在 `/tmp/multica-native-design-engine` 的 `codex/multica-native-design-engine` 分支完成。

## GitNexus Risk Record

2026-08-06 的 upstream impact 结果：

- `createProjectDesignSystemTask`: LOW，直接入口为首次创建。
- `enqueueExistingProjectDesignSystemTask`: LOW，直接入口为调整与重新生成。
- `marshalProjectDesignSystemTaskContext`: **HIGH**，影响首次生成、调整、重新生成和仓库分析。
- `attachProjectDesignSystemArtifacts` / `readProjectDesignSystemArtifacts`: LOW，向上到 daemon `handleTask`。
- `BuildPrompt`: LOW，向上到 daemon `runTask`。
- `prepareProjectDesignSystemCompletion` / `persistProjectDesignSystemCompletion`: LOW，向上到 daemon task completion。
- `projectDesignSystemResponse`: **HIGH**，影响创建、读取、调整、重新生成、保存和放弃草稿。

执行 Task 4 和 Task 7 前必须重新运行对应 impact；若结果仍为 HIGH，必须先向用户说明本批改动和回归矩阵，再编辑。开始代码前先刷新当前 stale index：

```bash
cd /tmp/multica-native-design-engine
rtk node .gitnexus/run.cjs analyze
```

每次提交前统一运行：

```bash
rtk git diff --check
rtk node /Users/fengyujie/Documents/soyoung/multica/.gitnexus/run.cjs \
  detect-changes --scope staged \
  --repo /tmp/multica-native-design-engine
```

## Target Package Contract

Agent 输出目录：

```text
$MULTICA_OUTPUT_DIR/
  DESIGN.md                         required, <= 256 KiB
  tokens.css                       required, <= 256 KiB
  source/index.json                required, <= 256 KiB
  USAGE.md                         optional, <= 256 KiB
  design-tokens.json               optional, <= 512 KiB
  components.manifest.json         optional, <= 512 KiB
  ui-kit/index.html                optional browser target
  preview/*.html                   optional browser targets
  assets/**                        optional local assets
  fonts/**                         optional local fonts
```

daemon 生成 `manifest.json`，Agent 不生成或覆盖它。最终 archive 限制为 512 个普通文件、单文件 16 MiB、解压总量 128 MiB、压缩包 64 MiB、最多 8 个 Preview 目标。只允许上述精确文件和目录；拒绝符号链接、硬链接、设备文件、路径穿越、反斜杠路径、重复路径和未声明顶层文件。

`manifest.json` 的稳定结构：

```go
const PackageSchemaV2 = "multica.project-design-system/v2"

type PackageBinding struct {
    WorkspaceID        string `json:"workspace_id"`
    ProjectID          string `json:"project_id"`
    DesignSystemID     string `json:"design_system_id"`
    TaskID             string `json:"task_id"`
    AgentID            string `json:"agent_id"`
    Operation          string `json:"operation"`
    InputSnapshotSHA256 string `json:"input_snapshot_sha256"`
    BasePackageSHA256  string `json:"base_package_sha256,omitempty"`
}

type ArtifactIndexEntry struct {
    Path      string `json:"path"`
    Role      string `json:"role"`
    MediaType string `json:"media_type"`
    SizeBytes int64  `json:"size_bytes"`
    SHA256    string `json:"sha256"`
}

type PreviewTarget struct {
    ID   string `json:"id"`
    Kind string `json:"kind"` // ui_kit or preview
    Path string `json:"path"`
}

type ManifestV2 struct {
    SchemaVersion string               `json:"schema_version"`
    Binding       PackageBinding       `json:"binding"`
    ContentDigest string               `json:"content_digest"`
    Files         []ArtifactIndexEntry `json:"files"`
    PreviewTargets []PreviewTarget     `json:"preview_targets"`
    Sections      []Section            `json:"sections"`
    TokenGroups   []TokenGroup         `json:"token_groups"`
    Locators      []Locator            `json:"locators"`
}
```

`source/index.json` 只区分来源事实、冲突和 fallback，不规定组件词典：

```go
const SourceIndexSchemaV1 = "multica.project-design-system-source-index/v1"

type SourceIndex struct {
    SchemaVersion       string             `json:"schema_version"`
    InputSnapshotSHA256 string             `json:"input_snapshot_sha256"`
    Evidence            []SourceEvidence   `json:"evidence"`
    Conflicts           []SourceConflict   `json:"conflicts"`
    Fallbacks           []SourceFallback   `json:"fallbacks"`
}

type SourceEvidence struct {
    ID         string   `json:"id"`
    Kind       string   `json:"kind"`
    Summary    string   `json:"summary"`
    References []string `json:"references"`
}
```

`references` 只能使用任务快照中的 ID、HTTPS URL 或仓库相对路径，不能出现本机绝对路径、凭据或整份源码。

---

### Task 1: Implement The V2 Package And Audit Boundary

**Files:**
- Create: `server/internal/projectdesignsystem/v2_types.go`
- Create: `server/internal/projectdesignsystem/v2_archive.go`
- Create: `server/internal/projectdesignsystem/v2_audit.go`
- Create: `server/internal/projectdesignsystem/v2_preview.go`
- Create: `server/internal/projectdesignsystem/v2_archive_test.go`
- Create: `server/internal/projectdesignsystem/v2_audit_test.go`
- Create: `server/internal/projectdesignsystem/testdata/v2-valid/DESIGN.md`
- Create: `server/internal/projectdesignsystem/testdata/v2-valid/tokens.css`
- Create: `server/internal/projectdesignsystem/testdata/v2-valid/source/index.json`
- Create: `server/internal/projectdesignsystem/testdata/v2-valid/components.manifest.json`
- Create: `server/internal/projectdesignsystem/testdata/v2-valid/ui-kit/index.html`
- Preserve unchanged: current v1 `types.go`, `validate.go`, `preview.go`, and v1 fixtures.

- [ ] **Step 1: Re-run impact for reused v1 parser symbols**

```bash
rtk node .gitnexus/run.cjs impact Validate --direction upstream --repo multica
rtk node .gitnexus/run.cjs impact parseMarkdownSections --direction upstream --repo multica
rtk node .gitnexus/run.cjs impact parseTokens --direction upstream --repo multica
```

Expected: document the reported risk; do not change v1 behavior. V2 may call these parsers through new focused helpers only.

- [ ] **Step 2: Write RED contract tests**

Add these exact tests:

```go
func TestCollectV2DirectoryBuildsDeterministicManifestAndArchive(t *testing.T)
func TestCollectV2DirectoryRequiresStableCoreAndPreview(t *testing.T)
func TestCollectV2DirectoryRejectsUnknownTopLevelFiles(t *testing.T)
func TestCollectV2DirectoryRejectsSymlinkHardlinkAndTraversal(t *testing.T)
func TestCollectV2DirectoryEnforcesFileCountAndByteLimits(t *testing.T)
func TestValidateV2ArchiveRecomputesEveryDigest(t *testing.T)
func TestValidateV2ArchiveBindsTaskInputAndBaseDigest(t *testing.T)
func TestAuditV2RejectsScriptsNetworkFormsAndUnsafeCSS(t *testing.T)
func TestAuditV2RequiresVisibleTokenBackedPreview(t *testing.T)
func TestAuditV2ValidatesSourceIndexWithoutKeywordTaxonomy(t *testing.T)
func TestDiscoverV2PreviewTargetsPrefersUIKitAndSortsPreviews(t *testing.T)
```

The valid fixture must use CRM-like density and local assets, reference at least one declared CSS custom property, and include a stable `data-design-node-id`. Security fixtures must cover `<script>`, `onclick`, `<form>`, `<iframe>`, `javascript:`, `https://`, protocol-relative URLs, CSS `@import`, `url(https://...)`, duplicate archive entries and zip bombs.

- [ ] **Step 3: Run RED**

Workdir: `server`

```bash
rtk go test ./internal/projectdesignsystem -run 'Test(Collect|Validate|Audit|Discover)V2' -count=1 -v
```

Expected: FAIL because V2 symbols do not exist.

- [ ] **Step 4: Implement deterministic collection and validation**

Expose only these package entrypoints:

```go
func SnapshotDigest(raw json.RawMessage) (string, error)
func CollectV2Directory(root string, binding PackageBinding) (CollectedV2Package, error)
func ValidateV2Archive(archive []byte, expected PackageBinding) (ValidatedV2Package, error)
func ReadV2Artifact(archive []byte, index []ArtifactIndexEntry, name string) ([]byte, error)
func DiscoverV2PreviewTargets(index []ArtifactIndexEntry) ([]PreviewTarget, error)
```

Use `archive/zip`, `io/fs`, `path`, `filepath`, `crypto/sha256`, `encoding/json`, `golang.org/x/net/html`, and the existing CSS parser. Do not parse HTML, CSS or JSON with regex. Digest the sorted artifact index using length-prefixed path, media type, byte length and SHA-256 fields; exclude generated `manifest.json` from the content digest to avoid a circular hash.

- [ ] **Step 5: Implement static Package Audit**

Return the existing diagnostic shape with a new report schema:

```go
const AuditSchemaV1 = "multica.project-design-system-audit/v1"

type AuditReport struct {
    SchemaVersion string       `json:"schema_version"`
    Passed        bool         `json:"passed"`
    ContentDigest string       `json:"content_digest"`
    Diagnostics  []Diagnostic `json:"diagnostics"`
}
```

Audit must prove required files, source snapshot binding, declared token use, safe local references, Preview target visibility, unique locators and manifest/index equality. Semantic quality remains Agent-owned; do not infer component categories from names.

- [ ] **Step 6: Run GREEN and all v1 regressions**

```bash
rtk gofmt -w internal/projectdesignsystem
rtk go test ./internal/projectdesignsystem -count=1
```

Expected: current 43 v1 tests plus all V2 tests pass.

- [ ] **Step 7: Commit**

```bash
rtk git add server/internal/projectdesignsystem
rtk git diff --check
rtk node .gitnexus/run.cjs detect-changes --scope staged --repo /tmp/multica-native-design-engine
rtk git commit -m "feat(design): define native design system package v2"
```

---

### Task 2: Extract Multica-Owned Browser Preview Verification

**Files:**
- Create: `server/internal/designpreview/types.go`
- Create: `server/internal/designpreview/browser.go`
- Create: `server/internal/designpreview/policy.go`
- Create: `server/internal/designpreview/browser_test.go`
- Modify: `server/internal/opendesign/browser_verifier.go`
- Modify: `server/internal/opendesign/browser_verifier_test.go`
- Modify: `server/internal/opendesign/preview.go`
- Modify: `server/internal/opendesign/supervisor.go`
- Test: `server/internal/opendesign/supervisor_test.go`

- [ ] **Step 1: Run impact before moving verifier symbols**

```bash
rtk node .gitnexus/run.cjs impact NewChromiumPreviewVerifier --direction upstream --repo multica
rtk node .gitnexus/run.cjs impact PinnedPreviewVerificationPolicy --direction upstream --repo multica
```

If HIGH or CRITICAL, stop this task and report the exact callers before editing.

- [ ] **Step 2: Write RED generic verifier tests**

```go
func TestChromiumVerifierAcceptsVisibleStaticTarget(t *testing.T)
func TestChromiumVerifierRejectsBlankAndOverflowingTarget(t *testing.T)
func TestChromiumVerifierBlocksOutboundRequests(t *testing.T)
func TestChromiumVerifierReportsBrokenImagesAndConsoleErrors(t *testing.T)
func TestValidateReceiptBindsDigestAndTargetSet(t *testing.T)
func TestResolveBrowserPathUsesExplicitExecutableThenInstalledChrome(t *testing.T)
```

The receipt contract is:

```go
const ReceiptSchemaV1 = "multica.design-preview-receipt/v1"

type Receipt struct {
    SchemaVersion string       `json:"schema_version"`
    ContentDigest string       `json:"content_digest"`
    Verification  Verification `json:"verification"`
}
```

- [ ] **Step 3: Run RED**

```bash
rtk go test ./internal/designpreview -count=1 -v
```

Expected: FAIL because the package does not exist.

- [ ] **Step 4: Move generic behavior without changing policy**

Move the current Chrome identity, DOM metrics, screenshot pixel checks, same-origin interception and bounded timeouts into `internal/designpreview`. Keep policy deterministic: one isolated profile, no cache/service worker, 1280x900 viewport, no outbound request, no failed resource, nonblank pixels, visible DOM and bounded dimensions.

Update historical Open Design code to convert its target/receipt types at the package edge. It may import `designpreview`; the new native path must not import `internal/opendesign`.

- [ ] **Step 5: Run generic and historical regressions**

```bash
rtk gofmt -w internal/designpreview internal/opendesign
rtk go test ./internal/designpreview ./internal/opendesign -run 'Preview|Browser|Supervisor' -count=1
```

Expected: generic verifier tests pass and existing Open Design supervisor behavior remains unchanged.

- [ ] **Step 6: Commit**

```bash
rtk git add server/internal/designpreview server/internal/opendesign
rtk git diff --check
rtk node .gitnexus/run.cjs detect-changes --scope staged --repo /tmp/multica-native-design-engine
rtk git commit -m "refactor(design): own browser preview verification"
```

---

### Task 3: Persist And Upload Immutable Native Package Archives

**Files:**
- Create: `server/migrations/134_native_project_design_system_package.up.sql`
- Create: `server/migrations/134_native_project_design_system_package.down.sql`
- Modify: `server/pkg/db/queries/design.sql`
- Regenerate: `server/pkg/db/generated/design.sql.go`
- Regenerate: `server/pkg/db/generated/models.go`
- Create: `server/internal/handler/project_design_system_package_upload.go`
- Create: `server/internal/handler/project_design_system_package_upload_test.go`
- Modify: `server/cmd/server/router.go`
- Modify: `server/cmd/server/integration_test.go`
- Modify: `server/internal/daemon/client.go`
- Modify: `server/internal/daemon/client_test.go`

- [ ] **Step 1: Run impact on route, storage and package upsert symbols**

```bash
rtk node .gitnexus/run.cjs impact UpsertProjectDesignSystemPackage --direction upstream --repo multica
rtk node .gitnexus/run.cjs impact SaveProjectDesignSystemDraft --direction upstream --repo multica
```

- [ ] **Step 2: Write RED migration and upload tests**

Tests must prove:

```go
func TestUploadProjectDesignSystemPackageStoresTaskBoundArchive(t *testing.T)
func TestUploadProjectDesignSystemPackageRejectsForeignDaemonAndNonRunningTask(t *testing.T)
func TestUploadProjectDesignSystemPackageRejectsDigestOrManifestMismatch(t *testing.T)
func TestUploadProjectDesignSystemPackageIsIdempotentForSameTaskAndDigest(t *testing.T)
func TestUploadProjectDesignSystemPackageRejectsOversizedBody(t *testing.T)
func TestSaveProjectDesignSystemDraftCopiesNativeArchiveColumns(t *testing.T)
```

- [ ] **Step 3: Add migration 134**

Add these columns to `project_design_system_package`:

```sql
package_schema TEXT NOT NULL DEFAULT 'legacy',
archive_object_key TEXT,
artifact_index JSONB NOT NULL DEFAULT '[]'::jsonb,
input_snapshot_sha256 TEXT,
base_package_sha256 TEXT
```

Backfill `package_schema` from existing manifest fields into exactly `multica.project-design-system/v1`, `multica.open-design-draft-package/v1`, or `legacy`, then add a check allowing those values plus `multica.project-design-system/v2`. The down migration drops only these five columns and its check constraint; it does not touch migration 130-133 data.

- [ ] **Step 4: Extend sqlc package writes**

`UpsertProjectDesignSystemPackage` and `SaveProjectDesignSystemDraft` must write/copy all five columns. Keep every query JOIN-free. Run:

```bash
make sqlc
! rtk rg -n '\bJOIN\b' pkg/db/generated/design.sql.go
```

Expected: sqlc succeeds and the second command exits 0 with no matches.

- [ ] **Step 5: Add task-scoped archive upload**

Register:

```text
POST /api/daemon/tasks/{taskId}/project-design-system/package
Content-Type: application/zip
X-Multica-Design-Package-Digest: sha256:<64 lowercase hex>
```

The handler must authorize the daemon through `requireDaemonTaskAccessWithWorkspace`, require a running non-repository-analysis design-system task, read at most 64 MiB, call `ValidateV2Archive` against the task binding, and upload to:

```text
project-design-systems/{workspace_id}/{design_system_id}/{task_id}/{digest}.zip
```

Return only:

```json
{"object_key":"...","content_digest":"sha256:..."}
```

Never return a public CDN URL or accept a caller-provided object key.

- [ ] **Step 6: Add daemon client method and retries**

```go
func (c *Client) UploadProjectDesignSystemPackage(
    ctx context.Context,
    taskID string,
    contentDigest string,
    archive []byte,
) (ProjectDesignSystemPackageUpload, error)
```

Use the existing bounded terminal retry schedule and resend the exact immutable bytes. Validate response digest equality before returning.

- [ ] **Step 7: Run focused tests**

```bash
rtk gofmt -w internal/handler internal/daemon
rtk go test ./internal/handler -run 'TestUploadProjectDesignSystemPackage|TestSaveProjectDesignSystemDraftCopiesNativeArchiveColumns' -count=1
rtk go test ./internal/daemon -run 'TestClientUploadProjectDesignSystemPackage' -count=1
rtk go test ./cmd/server -run 'ProjectDesignSystemPackage' -count=1
```

- [ ] **Step 8: Commit**

```bash
rtk git add server/migrations/134_native_project_design_system_package.* server/pkg/db/queries/design.sql server/pkg/db/generated server/internal/handler/project_design_system_package_upload* server/internal/daemon/client* server/cmd/server
rtk git diff --check
rtk node .gitnexus/run.cjs detect-changes --scope staged --repo /tmp/multica-native-design-engine
rtk git commit -m "feat(design): persist native design package archives"
```

---

### Task 4: Materialize The Native Agent Workspace And Contract

**Files:**
- Modify: `server/internal/service/task.go`
- Modify: `server/internal/handler/project_design_system.go`
- Modify: `server/internal/handler/project_design_system_test.go`
- Modify: `server/internal/daemon/execenv/context.go`
- Modify: `server/internal/daemon/execenv/execenv.go`
- Modify: `server/internal/daemon/execenv/execenv_test.go`
- Modify: `server/internal/daemon/execenv/runtime_config.go`
- Modify: `server/internal/daemon/execenv/runtime_config_test.go`
- Modify: `server/internal/daemon/prompt.go`
- Modify: `server/internal/daemon/prompt_test.go`

- [ ] **Step 1: Re-run and report HIGH impact before editing**

```bash
rtk node .gitnexus/run.cjs impact marshalProjectDesignSystemTaskContext --direction upstream --repo multica
rtk node .gitnexus/run.cjs impact writeProjectDesignSystemContext --direction upstream --repo multica
rtk node .gitnexus/run.cjs impact BuildPrompt --direction upstream --repo multica
```

Expected: `marshalProjectDesignSystemTaskContext` may remain HIGH. Report that this task changes all design-system task contexts while repository-analysis output remains on its existing JSON contract.

- [ ] **Step 2: Write RED task-context and workspace tests**

Add exact coverage:

```go
func TestMarshalProjectDesignSystemTaskContextPinsV2SchemaAndDigests(t *testing.T)
func TestMarshalRepositoryAnalysisContextKeepsRepositoryContract(t *testing.T)
func TestWriteProjectDesignSystemContextCreatesReadOnlyContextAndReferenceTrees(t *testing.T)
func TestWriteProjectDesignSystemContextRejectsMismatchedBasePackage(t *testing.T)
func TestPrepareProjectDesignSystemWorkspaceSeparatesWorkAndOutput(t *testing.T)
func TestBuildPromptProjectDesignSystemV2UsesEvidenceThenDesignStages(t *testing.T)
func TestBuildPromptProjectDesignSystemV2RequiresCompleteStaticPackage(t *testing.T)
func TestBuildPromptProjectDesignSystemV2NeverMentionsWorkerRuntimeOrFigmaJSON(t *testing.T)
```

- [ ] **Step 3: Extend task context without removing historical read fields**

Add:

```go
PackageSchema       string `json:"package_schema,omitempty"`
InputSnapshotSHA256 string `json:"input_snapshot_sha256,omitempty"`
BasePackageSHA256  string `json:"base_package_sha256,omitempty"`
```

Keep `OpenDesignRun` only so already-queued historical tasks can still be parsed. New non-analysis task contexts set `PackageSchemaV2`, the SHA-256 of canonical `input_snapshot`, and the selected base digest. Repository analysis does not set V2 output policy.

- [ ] **Step 4: Materialize bounded read-only inputs**

Create this logical layout under the daemon-owned env root:

```text
workdir/.agent_context/project_design_system/context/task.json
workdir/.agent_context/project_design_system/context/repository-analysis.json
workdir/.agent_context/project_design_system/reference/index.json
workdir/.agent_context/project_design_system/base/...
workdir/                                      Agent work area
output/project-design-system/                final output only
```

Write sidecars with `0444` files and `0555` context/reference directories after creation. `base/` is populated only for adjust/regenerate and must match `BasePackageSHA256`. Never write secrets, source absolute paths or full repository files.

- [ ] **Step 5: Replace the three-file prompt contract**

The prompt must require one Agent session to:

1. inventory provided evidence and classify facts/conflicts/fallbacks;
2. establish one coherent direction;
3. produce semantic Tokens;
4. design only source- or brief-supported components and page patterns;
5. build a static token-backed UI Kit/Preview with local assets;
6. read back and self-check every final file.

For adjustment, read the immutable base directory and emit a complete replacement. Explicitly forbid task delegation, repository writes, hidden follow-up work, network-dependent final HTML, scripts and invented template residue. Final stdout remains a short summary; package files are authoritative.

- [ ] **Step 6: Run focused regressions**

```bash
rtk gofmt -w internal/service internal/handler internal/daemon/execenv internal/daemon
rtk go test ./internal/handler -run 'TestMarshalProjectDesignSystemTaskContext|ProjectDesignSystem.*Context' -count=1
rtk go test ./internal/daemon/execenv -run 'ProjectDesignSystem' -count=1
rtk go test ./internal/daemon -run 'BuildPromptProjectDesignSystem' -count=1
```

- [ ] **Step 7: Commit**

```bash
rtk git add server/internal/service/task.go server/internal/handler/project_design_system.go server/internal/handler/project_design_system_test.go server/internal/daemon/execenv server/internal/daemon/prompt.go server/internal/daemon/prompt_test.go
rtk git diff --check
rtk node .gitnexus/run.cjs detect-changes --scope staged --repo /tmp/multica-native-design-engine
rtk git commit -m "feat(design): define native agent design workspace"
```

---

### Task 5: Collect, Preview And Upload Agent Packages In The Daemon

**Files:**
- Modify: `server/internal/daemon/config.go`
- Modify: `server/internal/daemon/config_test.go`
- Modify: `server/internal/daemon/types.go`
- Replace implementation: `server/internal/daemon/project_design_system_artifacts.go`
- Replace tests: `server/internal/daemon/project_design_system_artifacts_test.go`
- Create: `server/internal/daemon/project_design_system_package.go`
- Create: `server/internal/daemon/project_design_system_package_test.go`
- Modify: `server/internal/daemon/daemon.go`
- Modify: `server/internal/daemon/client.go`

- [ ] **Step 1: Re-run impact**

```bash
rtk node .gitnexus/run.cjs impact attachProjectDesignSystemArtifacts --direction upstream --repo multica
rtk node .gitnexus/run.cjs impact readProjectDesignSystemArtifacts --direction upstream --repo multica
```

- [ ] **Step 2: Write RED daemon finalization tests**

```go
func TestFinalizeProjectDesignSystemPackageCollectsAuditsPreviewsAndUploads(t *testing.T)
func TestFinalizeProjectDesignSystemPackageBlocksBeforeUploadOnStaticAuditFailure(t *testing.T)
func TestFinalizeProjectDesignSystemPackageBlocksBeforeCompletionOnPreviewFailure(t *testing.T)
func TestFinalizeProjectDesignSystemPackageRejectsMissingBrowser(t *testing.T)
func TestFinalizeProjectDesignSystemPackageReturnsTaskBoundReceipt(t *testing.T)
func TestHandleTaskDoesNotCallOpenDesignSupervisorForV2Context(t *testing.T)
```

The success fake must prove ordering: collect -> audit -> local Preview -> upload -> normal task completion. No stage may be inferred from the Agent stdout.

- [ ] **Step 3: Add generic browser configuration**

Add `DesignPreviewBrowserPath` sourced from `MULTICA_DESIGN_PREVIEW_BROWSER_PATH`. When unset, resolve installed Chrome/Chromium from platform-specific known paths and `PATH`; do not read `MULTICA_OPEN_DESIGN_*` for native tasks. An unresolved browser is a task failure with `project_design_system_preview_unavailable`, not a skipped gate.

- [ ] **Step 4: Replace inline artifacts with a receipt**

```go
type ProjectDesignSystemPackageReceipt struct {
    SchemaVersion  string                          `json:"schema_version"`
    ObjectKey      string                          `json:"object_key"`
    ContentDigest  string                          `json:"content_digest"`
    ArtifactIndex  []projectdesignsystem.ArtifactIndexEntry `json:"artifact_index"`
    Audit           projectdesignsystem.AuditReport `json:"audit"`
    Preview         designpreview.Receipt          `json:"preview"`
}
```

`TaskResult` carries this receipt internally; `Client.CompleteTask` sends it as `project_design_system_package`. Remove V2 use of the 2 MiB inline three-file payload while retaining legacy decode support on the Server until Task 7 compatibility tests pass.

- [ ] **Step 5: Build a loopback-only Preview server**

Serve the collected archive on `127.0.0.1:0` using an unguessable per-run prefix. Inject `tokens.css` and the trusted selection/measurement bridge only into validated HTML targets. Apply CSP `default-src 'self' data:; script-src <trusted-hash>; connect-src 'none'; object-src 'none'; frame-src 'none'; form-action 'none'; base-uri 'none'`. Shut down the server and Chrome profile on every success, failure and cancellation path.

- [ ] **Step 6: Finalize before task completion**

Change `handleTask` to call:

```go
result = d.finalizeProjectDesignSystemResult(runCtx, task, result)
```

only after provider exit and before `CompleteTask`. Non-design tasks and repository-analysis tasks remain unchanged. A failed Audit/Preview/upload turns the result into `blocked`, supplies a stable failure reason, sends no package receipt and never calls the Server completion mutation.

- [ ] **Step 7: Run daemon tests**

```bash
rtk gofmt -w internal/daemon
rtk go test ./internal/daemon -run 'ProjectDesignSystemPackage|ProjectDesignSystemArtifacts|HandleTaskDoesNotCallOpenDesign' -count=1
rtk go test ./internal/designpreview ./internal/projectdesignsystem -count=1
```

- [ ] **Step 8: Commit**

```bash
rtk git add server/internal/daemon
rtk git diff --check
rtk node .gitnexus/run.cjs detect-changes --scope staged --repo /tmp/multica-native-design-engine
rtk git commit -m "feat(design): verify native packages before completion"
```

---

### Task 6: Gate Draft Persistence On Independent Server Validation

**Files:**
- Modify: `server/internal/handler/project_design_system_completion.go`
- Modify: `server/internal/handler/project_design_system_completion_test.go`
- Modify: `server/internal/handler/daemon.go`
- Modify: `server/internal/handler/daemon_test.go`
- Modify: `server/internal/service/task.go`
- Modify: `server/internal/service/project_design_system_task_test.go`

- [ ] **Step 1: Re-run completion impacts**

```bash
rtk node .gitnexus/run.cjs impact prepareProjectDesignSystemCompletion --direction upstream --repo multica
rtk node .gitnexus/run.cjs impact persistProjectDesignSystemCompletion --direction upstream --repo multica
rtk node .gitnexus/run.cjs impact CompleteTask --direction upstream --repo multica --file server/internal/handler/daemon.go
```

- [ ] **Step 2: Write RED completion-gate tests**

```go
func TestCompleteProjectDesignSystemV2CreatesPassedDraftAfterAllEvidenceMatches(t *testing.T)
func TestCompleteProjectDesignSystemV2RejectsWrongTaskInputAgentAndBaseDigest(t *testing.T)
func TestCompleteProjectDesignSystemV2RejectsMissingOrMutatedStoredArchive(t *testing.T)
func TestCompleteProjectDesignSystemV2RejectsAuditOrPreviewFailure(t *testing.T)
func TestCompleteProjectDesignSystemV2DoesNotReplaceExistingDraftOnFailure(t *testing.T)
func TestCompleteProjectDesignSystemV2NeverChangesSavedOnFailure(t *testing.T)
func TestCompleteProjectDesignSystemV2IsAtomicWithTaskCompletion(t *testing.T)
```

Each failure test must seed a byte-distinct existing draft and saved package and compare every persisted column after the request.

- [ ] **Step 3: Validate the receipt independently**

Server completion must:

1. parse task context and require V2 schema;
2. derive the expected object key from workspace/system/task/digest;
3. read at most 64 MiB from object storage;
4. rebuild the archive index and content digest;
5. validate manifest binding against task, Agent, project, canonical input snapshot and base digest;
6. rerun static Package Audit;
7. validate Preview receipt schema, digest, target set and `passed=true`;
8. extract bounded `DESIGN.md`, `tokens.css`, optional UI Kit projection and display indexes.

Never trust the receipt's index, audit or preview without recomputing/checking them.

- [ ] **Step 4: Persist a passed V2 draft in the existing transaction**

`CompleteTaskWithMutation` must atomically mark the task complete, upsert `draft`, set `package_schema`, archive metadata, Audit JSON, Preview receipt, `render_status='passed'`, and clear the matching active task. `integrity_sha256` stores the lowercase digest without `sha256:`. No browser-side `/preview-verification` call is required for V2.

- [ ] **Step 5: Preserve legacy completion only for already-issued v1 contexts**

If `package_schema` is absent and the task has no `open_design_run`, accept the existing inline `project_design_system_artifacts` path. New V2 contexts must reject inline artifacts. Open Design contexts continue through their historical supervisor callback path and are not accepted by generic completion.

- [ ] **Step 6: Run handler and service tests**

```bash
rtk gofmt -w internal/handler internal/service
rtk go test ./internal/handler -run 'CompleteProjectDesignSystem|ProjectDesignSystemV2' -count=1
rtk go test ./internal/service -run 'ProjectDesignSystem' -count=1
```

- [ ] **Step 7: Commit**

```bash
rtk git add server/internal/handler/project_design_system_completion* server/internal/handler/daemon* server/internal/service/task.go server/internal/service/project_design_system_task_test.go
rtk git diff --check
rtk node .gitnexus/run.cjs detect-changes --scope staged --repo /tmp/multica-native-design-engine
rtk git commit -m "feat(design): gate native design system drafts"
```

---

### Task 7: Switch New Tasks To Native Execution And Preserve The Product Lifecycle

**Files:**
- Modify: `server/internal/handler/project_design_system.go`
- Modify: `server/internal/handler/project_design_system_test.go`
- Create: `server/internal/handler/project_design_system_package_preview.go`
- Create: `server/internal/handler/project_design_system_package_preview_test.go`
- Modify: `server/internal/handler/project_design_system_open_design_preview.go`
- Modify: `server/internal/handler/project_design_system_open_design_preview_test.go`
- Modify: `server/cmd/server/router.go`
- Modify: `packages/core/types/design.ts`
- Modify: `packages/core/api/schemas.ts`
- Modify: `packages/core/api/schemas.test.ts`
- Modify: `packages/core/api/client.ts`
- Modify: `packages/core/designs/keys.ts`
- Modify: `packages/core/designs/keys.test.ts`
- Modify: `packages/views/designs/project-design-system-canvas.tsx`
- Modify: `packages/views/designs/project-design-system-canvas.test.tsx`
- Modify: `packages/views/designs/project-design-system-preview.tsx`
- Modify: `packages/views/designs/project-design-system-preview.test.tsx`

- [ ] **Step 1: Re-run and report HIGH impacts**

```bash
rtk node .gitnexus/run.cjs impact createProjectDesignSystemTask --direction upstream --repo multica
rtk node .gitnexus/run.cjs impact enqueueExistingProjectDesignSystemTask --direction upstream --repo multica
rtk node .gitnexus/run.cjs impact marshalProjectDesignSystemTaskContext --direction upstream --repo multica
rtk node .gitnexus/run.cjs impact projectDesignSystemResponse --direction upstream --repo multica
```

Before edits, explicitly report that `marshalProjectDesignSystemTaskContext` and `projectDesignSystemResponse` are expected HIGH-risk integration points.

- [ ] **Step 2: Write RED routing and lifecycle tests**

```go
func TestCreateProjectDesignSystemAlwaysEnqueuesNativeV2WhenOpenDesignFlagIsTrue(t *testing.T)
func TestAdjustProjectDesignSystemUsesImmutableV2BaseReference(t *testing.T)
func TestAdjustHistoricalV1PackageUsesLegacyReadOnlyBase(t *testing.T)
func TestAdjustHistoricalOpenDesignPackageUsesNativeAllScopeConversion(t *testing.T)
func TestRegenerateProjectDesignSystemBindsCurrentBaseDigest(t *testing.T)
func TestProjectDesignSystemResponseRendersV2ContentWithoutLegacyValidate(t *testing.T)
func TestSaveAndDiscardProjectDesignSystemPreserveNativeArchiveMetadata(t *testing.T)
func TestHistoricalV1AndOpenDesignPackagesRemainReadable(t *testing.T)
```

- [ ] **Step 3: Stop creating new Open Design runs**

Remove `prepareOpenDesignRun` and `persistOpenDesignRun` calls from `createProjectDesignSystemTask` and `enqueueExistingProjectDesignSystemTask`. New generate/adjust/regenerate context must have no `open_design_run`, even when `MULTICA_OPEN_DESIGN_ENABLED=true`. Do not delete historical run routes or tables in this task.

- [ ] **Step 4: Generalize base package download**

Add a task-scoped endpoint for V2 and historical archive bases:

```text
GET /api/daemon/tasks/{taskId}/project-design-system/base-package
```

It returns the exact immutable archive plus digest headers after checking slot, source task, current project/system and task context. V1 inline bases remain supported for old queued tasks. Historical Open Design archives may be consumed only as read-only all-scope bases and the Agent must output V2.

- [ ] **Step 5: Generalize archive Preview while preserving old URLs**

Add:

```text
GET /api/project-design-systems/{id}/package-preview
```

Keep `/open-design-preview` as an API-boundary alias for installed clients. The generic loader detects V2, historical Open Design, or v1; V2 reads `archive_object_key` directly from the package row and validates its manifest/index/digest before serving any file. V2 HTML responses inject only the trusted bridge and use a hash CSP; historical Open Design keeps its existing compatibility policy.

- [ ] **Step 6: Build response content by package schema**

`projectDesignSystemResponse` selects draft before saved, then:

- V2: decode the stored V2 manifest/display indexes and use archive Preview targets;
- v1: run current `projectdesignsystem.Validate` and `BuildPreviewHTML`;
- historical Open Design: use compatibility artifacts and archive targets.

Never run v1 validation on V2 HTML. A malformed package returns empty safe content plus a deterministic package error; it must not panic or white-screen the client.

- [ ] **Step 7: Update drift-tolerant frontend contract**

Add `package_schema`, `preview_targets`, and `selection_enabled` as optional/defaulted fields. Feed malformed/null arrays through `parseWithFallback` tests. `ProjectDesignSystemPreview` accepts selection messages from a V2 archive iframe only when the ID exists in Server locators; it does not submit browser verification for already-passed V2 archives. Keep the same content-first page, adjustment panel, save and discard UI.

- [ ] **Step 8: Run backend and frontend regressions**

```bash
cd /tmp/multica-native-design-engine/server
rtk go test ./internal/handler -run 'ProjectDesignSystem|OpenDesignArchivePreview' -count=1
rtk go test ./cmd/server -run 'ProjectDesignSystem' -count=1

cd /tmp/multica-native-design-engine
rtk pnpm --filter @multica/core exec vitest run api/schemas.test.ts designs/keys.test.ts
rtk pnpm --filter @multica/views exec vitest run designs/project-design-system-canvas.test.tsx designs/project-design-system-preview.test.tsx
rtk pnpm typecheck
```

Expected: new V2 and historical package matrices pass; TypeScript has no response casts or boundary errors.

- [ ] **Step 9: Commit**

```bash
rtk git add server/internal/handler/project_design_system* server/cmd/server/router.go packages/core/types/design.ts packages/core/api packages/core/designs packages/views/designs/project-design-system-*
rtk git diff --check
rtk node .gitnexus/run.cjs detect-changes --scope staged --repo /tmp/multica-native-design-engine
rtk git commit -m "feat(design): switch design systems to native packages"
```

---

### Task 8: Verify The Real CRM Workflow And Record Evidence

**Files:**
- Modify after validation: `docs/product/design-center/project-design-system-validation.md`
- Modify: `docs/product/design-center/README.md`
- Modify: `docs/product/design-center/decision-register.md`
- Create only if needed for automation: `e2e/tests/project-design-system-native.spec.ts`

- [ ] **Step 1: Run the full focused backend verification before services**

```bash
cd /tmp/multica-native-design-engine/server
rtk go test ./internal/projectdesignsystem ./internal/designpreview ./internal/daemon ./internal/handler ./internal/service ./cmd/server -count=1

cd /tmp/multica-native-design-engine
rtk pnpm --filter @multica/core test
rtk pnpm --filter @multica/views test
rtk pnpm typecheck
rtk git diff --check
```

Record baseline failures separately; do not label unrelated existing failures as caused by this branch.

- [ ] **Step 2: Start only the isolated worktree services**

```bash
cd /tmp/multica-native-design-engine
make setup-worktree
make start-worktree
```

Read `.env.worktree` for actual ports. Do not stop or restart the user's main checkout services and do not use `make dev` in the main checkout.

- [ ] **Step 3: Execute one real CRM generation**

Use the CRM project and a user-selected local Agent. Capture and correlate:

- project design system ID;
- agent task ID and session/activity evidence;
- task context `package_schema`, input snapshot digest and repository-analysis sources;
- object key, artifact index and content digest;
- `source/index.json` evidence;
- static Audit and Chrome receipt;
- draft DB row and absence of any new `open_design_run` row.

Task completion alone is not acceptance.

- [ ] **Step 4: Verify visuals in the user's Chrome**

Open the generated design-system page in the user's local Chrome. Inspect Network and Console, then capture desktop screenshots of the UI Kit and representative real CRM pages side by side. Confirm visible agreement in color, typography, density, controls and page patterns; confirm no blank iframe, overflow, broken image, external request or template residue.

- [ ] **Step 5: Verify adjustment, save, discard and failure isolation**

Run one scoped adjustment and confirm input/base digests change correctly. Save it, refresh, and compare persisted digest. Then inject one invalid package in a test task and prove draft/saved remain byte-for-byte unchanged. Finally discard a later valid draft and confirm the last saved system returns.

- [ ] **Step 6: Update authoritative evidence only with observed facts**

In `project-design-system-validation.md`, record IDs, digests, test commands, screenshots, Chrome evidence, database assertions, historical compatibility and remaining visual gaps. Update README/decision register to mark Phase 1 complete only if every row in the spec's acceptance matrix has evidence; otherwise mark the exact incomplete rows and do not claim completion.

- [ ] **Step 7: Final GitNexus and commit**

```bash
rtk git add docs/product/design-center e2e/tests/project-design-system-native.spec.ts
rtk git diff --check
rtk node .gitnexus/run.cjs detect-changes --scope staged --repo /tmp/multica-native-design-engine
rtk git commit -m "test(design): verify native project design systems"
```

Omit the E2E path from `git add` if no automated file was required.

## Final Acceptance Matrix

| Requirement | Evidence required before completion |
| --- | --- |
| No Worker dependency | New task has no `open_design_run`; daemon has no Worker call; flow succeeds without Open Design Runtime variables |
| Real Agent input | Stored task context digest, repository/reference snapshot, Agent session/activity |
| V2 package integrity | Recomputed artifact index, manifest binding, source index and content digest |
| Static quality | Package Audit passed with no blocking diagnostic |
| Real visual output | daemon Chrome receipt plus user's Chrome screenshot/network/console evidence |
| CRM grounding | side-by-side evidence for color, typography, density, components and page patterns |
| Draft isolation | invalid package leaves previous draft and saved rows unchanged |
| Save/discard | atomic save, refresh-stable digest and correct discard restoration |
| Adjustment | base digest bound, complete replacement package and scoped locator behavior |
| Historical compatibility | v1 and historical Open Design packages remain readable; historical archive can be regenerated through native V2 all-scope conversion |

## Rollback Boundary

- Before Task 7, V2 code is additive and no production task is routed to it; rollback is commit-level with no user-facing data conversion.
- After Task 7, rollback means reverting the routing commit so new tasks return to the prior v1 path. Migration 134 columns and V2 archive objects remain inert and must not be deleted during rollback.
- Never down-migrate 134 against a database containing V2 packages. Data cleanup, removal of `open_design_run`, and deletion of dormant Worker code require a separate approved plan after live acceptance.
