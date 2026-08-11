# Superseded: Open Design System Foundation Implementation Plan

> **Recovery note (2026-08-11):** Preserved from `codex/feature-fengchen-dirty-recovery-20260810` as historical context. Do not treat this file as current authority without checking `docs/product/design-center/README.md` and `docs/product/design-center/decision-register.md`.

> **Status: superseded. Do not execute this plan.**
>
> This plan targeted package revision governance, bindings, migration, and design-restore consumption before validating the user-facing project design-system workflow. The confirmed first-stage goal is now `P-008` / `DC-017`: users actively create or generate a project design system, inspect rules, Tokens, components, and an online UI Kit, review or adjust it, and save it as a project asset. A replacement implementation plan may be written only after that product flow is designed and approved.

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Replace the mutable project-default UI profile contract with a reviewable, publishable Open Design package revision that projects pin explicitly, then make design restore consume the pinned package.

**Architecture:** Keep the existing `design_system_profile` table as the single physical identity table during this slice, but stop treating `profile_json` and `is_default` as the active contract. Add immutable `design_system_revision` snapshots and explicit `project_design_system_binding` rows, validate every publishable package in a focused Go domain package, and resolve a fixed Agent pack through one server-side Design Context Resolver. Existing Figma UI-spec uploads become source evidence for a draft revision; only a successful server audit advances that revision to human review, and only a human publish action can bind it to a project.

**Tech Stack:** PostgreSQL migrations and sqlc, Go 1.26.1, `github.com/tdewolff/parse/v2/css` v2.8.14, Chi handlers, existing Agent Task queue/daemon, Qiniu-compatible `storage.Storage`, React Query, Zod, React, Vitest.

## Global Constraints

- Product authority is `docs/product/design-center/README.md`, decision `P-007`, and `docs/product/design-center/decision-register.md`, decision `DC-016`.
- The Open Design contract is pinned to upstream commit `89d6d4ef21baf80f871595abdf6f7de6e941dd44` for this implementation. Record a new evidence entry before changing that contract.
- The minimum package is exactly `manifest.json`, `DESIGN.md`, and `tokens.css`. Database columns are the atomic source of truth; a zip or CDN URL is only a distribution artifact.
- `manifest.source.type` remains `bundled | local | github | shadcn`. Never add `figma`; Figma identity belongs in revision provenance and `artifact_index`.
- Project revision upgrades are always explicit. Publishing a new revision must not move an existing binding unless the request explicitly asks for that exact revision.
- The owner project's first publish action may atomically publish and bind as primary. No Agent task status, self-score, legacy `is_default`, or package upload can perform that action.
- Every legacy `design_system_profile` produces one `pending_review` revision with a blocking `legacy_package_requires_regeneration` audit diagnostic. It produces no published revision and no project binding.
- Do not extend or rewrite `design_component_recipe_set`, `design_template_blueprint`, or the paused `PageSpec` compiler. Keep their legacy reads compiling until their separate removal plan.
- Design restore priority is: selected design revision/frame/layers for explicit structure and content, plus cloud primary design-system revision for Tokens/rules, then local repository `DESIGN.md`, then repository reality. Inspirations are weak references and never inject their Tokens as authoritative values.
- SQL added to `server/pkg/db/queries/design.sql` must not contain `JOIN`; the GitLab pre-receive rule rejects generated `server/pkg/db/generated/design.sql.go` containing join syntax. Assemble related records in Go with bounded separate queries or correlated subqueries.
- Migration number `129` already exists as untracked work. This plan starts at `130` and does not edit, renumber, or discard migration `129`.
- The worktree is dirty. Stage only files listed by the completed task, never run a database reset against the user's current database, and never rewrite unrelated generated files.
- Before editing an existing symbol, run GitNexus upstream impact. Prior inspection found LOW risk for `createDesignSystemProfileFromRevision`, `SetDesignSystemProfileDefault`, `CreateDesignDraftAgentTask`, `designRestoreDesignSystemContext`, and `DesignSystemCard`; re-run impact if the index changes.
- Before every commit run GitNexus `detect_changes()` and `git diff --check`. Execute work in batches of no more than three tasks, then report concrete tests and remaining risk.

## Non-Goals

- Community design-resource catalog, online package editor, native Figma UI Kit write-back, general Design Run, and UI Agent design generation.
- Automatic repository `DESIGN.md` creation or patching.
- Automatic semantic reconciliation of arbitrary prose contradictions. The first audit enforces manifest structure, CSS/token completeness, references, provenance labels, and checksums; human review remains required for visual intent.
- Removing the legacy profile, recipe, template Blueprint, or PageSpec columns/tables in this slice.

---

## Files And Responsibilities

- Modify `server/go.mod` and `server/go.sum`: add the structured CSS parser.
- Create `server/internal/designsystem/manifest.go`: strict Open Design v1 manifest model and path validation.
- Create `server/internal/designsystem/token_schema.go`: pinned A1/A2/B-slot token contract.
- Create `server/internal/designsystem/audit.go`: package audit, diagnostics, normalized artifact index, and Token evidence validation.
- Create `server/internal/designsystem/digest.go`: deterministic package digest and zip builder.
- Create `server/internal/designsystem/*_test.go` and `server/internal/designsystem/testdata/valid/*`: domain fixtures and tests.
- Create `server/migrations/130_open_design_system_foundation.up.sql` and `.down.sql`: revisions, bindings, legacy review revisions, restore-task pins, and immutable-content guards.
- Modify `server/pkg/db/queries/design.sql` and regenerate `server/pkg/db/generated/{design.sql.go,models.go}`: lifecycle, binding, resolver, and pin queries without SQL joins.
- Create `server/internal/handler/design_system.go`: lifecycle request/response types and handlers.
- Create `server/internal/handler/design_system_test.go`: lifecycle, authorization, audit, transaction, legacy, and binding tests.
- Modify `server/cmd/server/router.go` and `server/cmd/server/integration_test.go`: register and authorize lifecycle routes.
- Modify `packages/core/types/design.ts`, `packages/core/api/schemas.ts`, `packages/core/api/client.ts`, `packages/core/api/client.test.ts`, `packages/core/designs/keys.ts`, and `packages/core/designs/queries.ts`: new API contract with malformed-response fallbacks.
- Create `packages/views/designs/design-system-page.tsx` and `.test.tsx`: revision review, package inspection, publish, reject, and binding actions.
- Create `packages/views/designs/design-system-preview.tsx` and `.test.tsx`: safe online UI Kit evidence rendered from audited Token values.
- Modify `packages/views/designs/designs-page.tsx` and `.test.tsx`: unestablished, pending-review, and published/bound states.
- Modify `packages/views/designs/index.ts`, `packages/core/paths/*`, and create `apps/web/app/[workspaceSlug]/(dashboard)/designs/systems/[id]/page.tsx`: shared-view route wiring.
- Modify `server/internal/handler/design_plugin.go`, its tests, and `apps/figma-plugin/ui.html`: Figma provenance import into a draft revision and truthful copy.
- Modify `server/internal/service/task.go`, `server/internal/daemon/types.go`, `server/internal/daemon/prompt.go`, and tests: strict revision-analysis Agent contract.
- Create `server/internal/service/design_context.go` and `.test.go`: unified primary/inspiration resolver.
- Modify `server/internal/handler/design_file.go`, `server/internal/handler/daemon.go`, `server/internal/service/design_restore_context_test.go`, and related tests: pin and consume a resolved design context for restore.
- Create `docs/product/design-center/open-design-foundation-validation.md`: implementation evidence after live validation.

---

## Batch A: Package And Persistence Foundation

### Task 1: Open Design Package Contract, Audit, And Digest

**Files:**
- Modify: `server/go.mod`
- Modify: `server/go.sum`
- Create: `server/internal/designsystem/manifest.go`
- Create: `server/internal/designsystem/token_schema.go`
- Create: `server/internal/designsystem/audit.go`
- Create: `server/internal/designsystem/digest.go`
- Create: `server/internal/designsystem/manifest_test.go`
- Create: `server/internal/designsystem/audit_test.go`
- Create: `server/internal/designsystem/digest_test.go`
- Create: `server/internal/designsystem/testdata/valid/manifest.json`
- Create: `server/internal/designsystem/testdata/valid/DESIGN.md`
- Create: `server/internal/designsystem/testdata/valid/tokens.css`

**Interfaces:**
- Consumes: Open Design manifest and Token contracts fixed at commit `89d6d4e`.
- Produces:

```go
package designsystem

const ManifestSchemaVersion = "od-design-system-project/v1"
const AuditSchemaVersion = "multica-design-system-audit/v1"

type Severity string

const (
	SeverityError   Severity = "error"
	SeverityWarning Severity = "warning"
	SeverityInfo    Severity = "info"
)

type Diagnostic struct {
	Severity Severity `json:"severity"`
	Code     string   `json:"code"`
	Path     string   `json:"path,omitempty"`
	Message  string   `json:"message"`
}

type Manifest struct {
	SchemaVersion      string          `json:"schemaVersion"`
	ID                 string          `json:"id"`
	Name               string          `json:"name"`
	Category           string          `json:"category"`
	Description        string          `json:"description,omitempty"`
	Source             ManifestSource  `json:"source"`
	Files              ManifestFiles   `json:"files"`
	AssetsDir          string          `json:"assetsDir,omitempty"`
	PreviewDir         string          `json:"previewDir,omitempty"`
	Usage              string          `json:"usage,omitempty"`
	ComponentsManifest string          `json:"componentsManifest,omitempty"`
	ImportMode         string          `json:"importMode,omitempty"`
	Craft              *ManifestCraft  `json:"craft,omitempty"`
	Fonts              []ManifestFont  `json:"fonts,omitempty"`
	Preview            *ManifestPreview `json:"preview,omitempty"`
	SourceFiles        *ManifestSourceFiles `json:"sourceFiles,omitempty"`
}

type ManifestSource struct {
	Type        string `json:"type"`
	Origin      string `json:"origin,omitempty"`
	Path        string `json:"path,omitempty"`
	URL         string `json:"url,omitempty"`
	Branch      string `json:"branch,omitempty"`
	Commit      string `json:"commit,omitempty"`
	Reference   string `json:"reference,omitempty"`
	RegistryURL string `json:"registryUrl,omitempty"`
	Item        string `json:"item,omitempty"`
	Homepage    string `json:"homepage,omitempty"`
	ImportedAt  string `json:"importedAt,omitempty"`
}

type ManifestFiles struct {
	Design       string `json:"design"`
	Tokens       string `json:"tokens"`
	DesignTokens string `json:"designTokens,omitempty"`
	Tailwind     string `json:"tailwind,omitempty"`
	Components   string `json:"components,omitempty"`
}

type ManifestCraft struct {
	Applies    []string `json:"applies"`
	Suggested  []string `json:"suggested"`
	Exemptions []string `json:"exemptions"`
}

type ManifestFont struct {
	Family string          `json:"family"`
	File   string          `json:"file"`
	Weight json.RawMessage `json:"weight,omitempty"`
	Style  string          `json:"style,omitempty"`
}

type ManifestPreviewPage struct {
	Path  string `json:"path"`
	Role  string `json:"role,omitempty"`
	Title string `json:"title,omitempty"`
}

type ManifestPreview struct {
	Dir   string                `json:"dir"`
	Pages []ManifestPreviewPage `json:"pages"`
}

type ManifestSourceFiles struct {
	Scanned  string `json:"scanned,omitempty"`
	Evidence string `json:"evidence,omitempty"`
	Tokens   string `json:"tokens,omitempty"`
	Report   string `json:"report,omitempty"`
	Snippets string `json:"snippets,omitempty"`
}

type Package struct {
	ManifestJSON  json.RawMessage
	DesignMD      string
	TokensCSS     string
	Artifacts     []Artifact
	TokenEvidence map[string]TokenEvidence
}

type Artifact struct {
	Path        string            `json:"path"`
	Role        string            `json:"role"`
	MediaType   string            `json:"media_type"`
	SizeBytes   int64             `json:"size_bytes"`
	SHA256      string            `json:"sha256"`
	Storage     string            `json:"storage"`
	ObjectKey   string            `json:"object_key,omitempty"`
	URL         string            `json:"url,omitempty"`
	Provenance  map[string]string `json:"provenance,omitempty"`
}

type TokenEvidence struct {
	Confidence string `json:"confidence"` // high, medium, low, fallback, alias
	SourceKind string `json:"source_kind"`
	SourceRef  string `json:"source_ref,omitempty"`
}

type AuditReport struct {
	SchemaVersion string          `json:"schema_version"`
	Passed        bool            `json:"passed"`
	Diagnostics   []Diagnostic    `json:"diagnostics"`
	Tokens        TokenAudit      `json:"tokens"`
	Artifacts     []Artifact      `json:"artifacts"`
}

type TokenAudit struct {
	Declared             []string                 `json:"declared"`
	Values               map[string]string        `json:"values"`
	MissingRequired       []string                 `json:"missing_required"`
	Extensions           []string                 `json:"extensions"`
	UnresolvedReferences []string                 `json:"unresolved_references"`
	Evidence             map[string]TokenEvidence `json:"evidence"`
}

type TokenLayer string

const (
	A1Identity  TokenLayer = "A1-identity"
	A1Structure TokenLayer = "A1-structure"
	A2          TokenLayer = "A2"
	BSlot       TokenLayer = "B-slot"
)

type TokenSpec struct {
	Name     string
	Layer    TokenLayer
	Fallback string
	AliasTo  string
}

func ParseManifest(raw json.RawMessage) (Manifest, []Diagnostic)
func AuditPackage(input Package) AuditReport
func ContentDigest(input Package) (string, error)
func BuildZip(input Package, extraFiles map[string][]byte) ([]byte, error)
```

- [ ] **Step 1: Write failing manifest and audit tests**

Add table-driven tests with these exact cases:

```go
func TestParseManifestRejectsUnknownFieldsAndFigmaSource(t *testing.T)
func TestParseManifestRejectsUnsafeRelativePaths(t *testing.T)
func TestAuditPackageAcceptsPinnedOpenDesignV1Fixture(t *testing.T)
func TestAuditPackageRejectsMalformedCSS(t *testing.T)
func TestAuditPackageRejectsMissingA1A2AndBSlotTokens(t *testing.T)
func TestAuditPackageRejectsUnresolvedTokenReferences(t *testing.T)
func TestAuditPackageReportsFallbackAliasAndUnprovenTokens(t *testing.T)
func TestAuditPackageRejectsManifestArtifactMismatch(t *testing.T)
```

The valid fixture manifest must use:

```json
{
  "schemaVersion": "od-design-system-project/v1",
  "id": "crm-test",
  "name": "CRM Test",
  "category": "Business",
  "source": {"type": "bundled", "origin": "Multica test fixture"},
  "files": {"design": "DESIGN.md", "tokens": "tokens.css"}
}
```

Use the full `:root` declaration set from the pinned `design-systems/ant/tokens.css` as the fixture's Token values, and the pinned `design-systems/ant/DESIGN.md` as its prose body after changing only the fixture heading/name. Do not shorten either fixture; the audit test must exercise every required Token.

Assert `source.type = "figma"`, unknown top-level keys, absolute paths, `..`, backslashes, missing required Token names, CSS parser errors, and manifest-declared missing files all produce stable diagnostic codes rather than matching error prose.

- [ ] **Step 2: Run tests and confirm the package does not exist**

Run:

```bash
rtk go -C server test ./internal/designsystem -count=1
```

Expected: FAIL because `server/internal/designsystem` and its exported contract do not exist.

- [ ] **Step 3: Add the CSS parser and strict manifest implementation**

Run:

```bash
rtk go -C server get github.com/tdewolff/parse/v2@v2.8.14
```

Implement `Manifest` as a typed union for only `bundled`, `local`, `github`, and `shadcn`. Decode with `json.Decoder.DisallowUnknownFields`, verify exactly one JSON value, enforce the literal core filenames, lowercase slug IDs, non-empty name/category, and safe forward-slash relative paths. Preserve optional `usage`, `componentsManifest`, `importMode`, `craft`, `fonts`, `preview`, and `sourceFiles` fields from the pinned v1 schema.

- [ ] **Step 4: Encode the pinned Token contract**

Define the shared required names by layer, preserving the Open Design values:

```go
var tokenSchema = []TokenSpec{
	{Name: "--bg", Layer: A1Identity}, {Name: "--surface", Layer: A1Identity},
	{Name: "--surface-warm", Layer: BSlot, AliasTo: "var(--surface)"},
	{Name: "--fg", Layer: A1Identity}, {Name: "--fg-2", Layer: BSlot, AliasTo: "var(--fg)"},
	{Name: "--muted", Layer: A1Identity}, {Name: "--meta", Layer: BSlot, AliasTo: "var(--muted)"},
	{Name: "--border", Layer: A1Identity}, {Name: "--border-soft", Layer: BSlot, AliasTo: "var(--border)"},
	{Name: "--accent", Layer: A1Identity},
	{Name: "--accent-on", Layer: A2, Fallback: "#ffffff"},
	{Name: "--accent-hover", Layer: A2, Fallback: "color-mix(in oklab, var(--accent), black 8%)"},
	{Name: "--accent-active", Layer: A2, Fallback: "color-mix(in oklab, var(--accent), black 14%)"},
	{Name: "--success", Layer: A2, Fallback: "#16a34a"},
	{Name: "--warn", Layer: A2, Fallback: "#eab308"},
	{Name: "--danger", Layer: A2, Fallback: "#dc2626"},
	{Name: "--font-display", Layer: A1Identity}, {Name: "--font-body", Layer: A1Identity},
	{Name: "--font-mono", Layer: A2, Fallback: `ui-monospace, "SF Mono", "JetBrains Mono", Menlo, Monaco, Consolas, monospace`},
	{Name: "--text-xs", Layer: A1Structure}, {Name: "--text-sm", Layer: A1Structure},
	{Name: "--text-base", Layer: A1Structure}, {Name: "--text-lg", Layer: A1Structure},
	{Name: "--text-xl", Layer: A1Structure}, {Name: "--text-2xl", Layer: A1Structure},
	{Name: "--text-3xl", Layer: A1Structure}, {Name: "--text-4xl", Layer: A1Structure},
	{Name: "--leading-body", Layer: A1Structure}, {Name: "--leading-tight", Layer: A1Structure},
	{Name: "--tracking-display", Layer: A1Structure},
	{Name: "--space-1", Layer: A2, Fallback: "4px"}, {Name: "--space-2", Layer: A2, Fallback: "8px"},
	{Name: "--space-3", Layer: A2, Fallback: "12px"}, {Name: "--space-4", Layer: A2, Fallback: "16px"},
	{Name: "--space-5", Layer: A2, Fallback: "20px"}, {Name: "--space-6", Layer: A2, Fallback: "24px"},
	{Name: "--space-8", Layer: A2, Fallback: "32px"}, {Name: "--space-12", Layer: A2, Fallback: "48px"},
	{Name: "--section-y-desktop", Layer: A1Structure}, {Name: "--section-y-tablet", Layer: A1Structure},
	{Name: "--section-y-phone", Layer: A1Structure},
	{Name: "--radius-sm", Layer: A2, Fallback: "8px"}, {Name: "--radius-md", Layer: A2, Fallback: "12px"},
	{Name: "--radius-lg", Layer: A2, Fallback: "16px"}, {Name: "--radius-pill", Layer: A2, Fallback: "9999px"},
	{Name: "--elev-flat", Layer: A2, Fallback: "none"},
	{Name: "--elev-ring", Layer: A2, Fallback: "0 0 0 1px var(--border)"},
	{Name: "--elev-raised", Layer: A2, Fallback: "0 2px 8px color-mix(in oklab, var(--fg), transparent 92%)"},
	{Name: "--focus-ring", Layer: A2, Fallback: "0 0 0 3px color-mix(in oklab, var(--accent), transparent 70%)"},
	{Name: "--motion-fast", Layer: A2, Fallback: "150ms"}, {Name: "--motion-base", Layer: A2, Fallback: "200ms"},
	{Name: "--ease-standard", Layer: A2, Fallback: "cubic-bezier(0.2, 0, 0, 1)"},
	{Name: "--container-max", Layer: A1Structure},
	{Name: "--container-gutter-desktop", Layer: A1Structure},
	{Name: "--container-gutter-tablet", Layer: A1Structure},
	{Name: "--container-gutter-phone", Layer: A1Structure},
}
```

Add a source comment pointing to the pinned upstream file and its Apache-2.0 license. Unknown custom properties are listed as C-extensions in the audit report and do not fail publication; unresolved `var(--name)` references do fail.

- [ ] **Step 5: Implement structured CSS audit, provenance normalization, digest, and zip**

Use `css.NewParser(parse.NewInputString(input.TokensCSS), false)` and consume `CustomPropertyGrammar`; do not classify CSS through regular expressions. Require every A1, A2, and B-slot name in the final `:root`, detect duplicates and unresolved `var()` references, validate Token evidence confidence values, and add warnings for required Tokens without source evidence.

Normalize artifact paths, sort by `Path`, and include inline index rows for all three core files. Compute `sha256` over length-prefixed sections named `manifest.json`, `DESIGN.md`, `tokens.css`, then the canonical JSON artifact index; this prevents JSON key order and artifact insertion order from changing the digest. `BuildZip` must sort entries, reject undeclared/unsafe extra paths, and use deterministic timestamps so identical inputs produce identical bytes.

- [ ] **Step 6: Add digest and archive determinism tests**

Add:

```go
func TestContentDigestIsStableAcrossManifestKeyAndArtifactOrder(t *testing.T)
func TestContentDigestChangesWhenAnyCoreFactChanges(t *testing.T)
func TestBuildZipIsDeterministicAndContainsOnlyDeclaredFiles(t *testing.T)
```

Run:

```bash
rtk go -C server test ./internal/designsystem -count=1
```

Expected: PASS with all manifest, Token, provenance, digest, and archive cases.

- [ ] **Step 7: Commit the isolated domain package**

Before committing run GitNexus `detect_changes()` and:

```bash
rtk git diff --check
rtk git add server/go.mod server/go.sum server/internal/designsystem
rtk git commit -m "feat(designs): add Open Design package audit"
```

### Task 2: Revision, Binding, And Restore-Pin Schema

**Files:**
- Create: `server/migrations/130_open_design_system_foundation.up.sql`
- Create: `server/migrations/130_open_design_system_foundation.down.sql`
- Modify: `server/pkg/db/queries/design.sql`
- Generated: `server/pkg/db/generated/design.sql.go`
- Generated: `server/pkg/db/generated/models.go`
- Create: `server/internal/handler/design_system_schema_test.go`

**Interfaces:**
- Consumes: `design_system_profile` as the one stable identity table and existing Project/design/Agent Task foreign keys.
- Produces: `db.DesignSystemRevision`, `db.ProjectDesignSystemBinding`, lifecycle queries, and idempotent restore-context pin queries.

- [ ] **Step 1: Write failing schema integration tests**

Add tests that use the handler test database directly:

```go
func TestDesignSystemIdentityAllowsNoFigmaSource(t *testing.T)
func TestDesignSystemPublishedRevisionContentIsImmutable(t *testing.T)
func TestProjectAllowsOnlyOnePrimaryDesignSystemBinding(t *testing.T)
func TestProjectBindingRejectsDraftRevision(t *testing.T)
func TestRestoreTaskDesignContextPinIsWriteOnce(t *testing.T)
```

The tests must assert database constraint names/error SQLSTATEs where applicable, not English server messages.

- [ ] **Step 2: Run the schema tests and confirm migration 130 is absent**

Run:

```bash
rtk go -C server test ./internal/handler -run 'Test(DesignSystemIdentity|DesignSystemPublished|ProjectAllowsOnlyOnePrimary|ProjectBindingRejectsDraft|RestoreTaskDesignContextPin)' -count=1
```

Expected: FAIL because the revision/binding tables and restore pin columns do not exist.

- [ ] **Step 3: Create migration 130**

The up migration must perform these operations in this order:

```sql
ALTER TABLE design_system_profile
    ALTER COLUMN source_file_id DROP NOT NULL,
    ALTER COLUMN source_revision_id DROP NOT NULL,
    ADD COLUMN current_published_revision_id uuid,
    ADD COLUMN archived_at timestamptz;

ALTER TABLE design_system_profile
    ADD CONSTRAINT design_system_profile_workspace_identity_unique
    UNIQUE (workspace_id, id);

COMMENT ON COLUMN design_system_profile.is_default IS
    'Legacy evidence only after migration 130; project_design_system_binding is authoritative.';
COMMENT ON COLUMN design_system_profile.profile_json IS
    'Legacy import evidence only after migration 130; design_system_revision package fields are authoritative.';

CREATE TABLE design_system_revision (
    id uuid PRIMARY KEY DEFAULT gen_random_uuid(),
    workspace_id uuid NOT NULL REFERENCES workspace(id) ON DELETE CASCADE,
    design_system_id uuid NOT NULL,
    revision_number integer NOT NULL CHECK (revision_number > 0),
    parent_revision_id uuid,
    status text NOT NULL DEFAULT 'draft'
        CHECK (status IN ('draft', 'pending_review', 'published', 'rejected', 'archived')),
    manifest_json jsonb NOT NULL DEFAULT '{}'::jsonb,
    design_md text NOT NULL DEFAULT '',
    tokens_css text NOT NULL DEFAULT '',
    artifact_index jsonb NOT NULL DEFAULT '[]'::jsonb CHECK (jsonb_typeof(artifact_index) = 'array'),
    package_url text,
    package_object_key text,
    content_digest text,
    audit_report jsonb NOT NULL DEFAULT '{}'::jsonb CHECK (jsonb_typeof(audit_report) = 'object'),
    review_note text,
    source_agent_task_id uuid REFERENCES agent_task_queue(id) ON DELETE SET NULL,
    created_by uuid REFERENCES "user"(id) ON DELETE SET NULL,
    reviewed_by uuid REFERENCES "user"(id) ON DELETE SET NULL,
    published_by uuid REFERENCES "user"(id) ON DELETE SET NULL,
    created_at timestamptz NOT NULL DEFAULT now(),
    reviewed_at timestamptz,
    published_at timestamptz,
    UNIQUE (design_system_id, revision_number),
    UNIQUE (design_system_id, content_digest),
    UNIQUE (source_agent_task_id),
    UNIQUE (workspace_id, id, design_system_id),
    CONSTRAINT design_system_revision_identity_fk
        FOREIGN KEY (workspace_id, design_system_id)
        REFERENCES design_system_profile(workspace_id, id) ON DELETE CASCADE,
    CONSTRAINT design_system_revision_parent_fk
        FOREIGN KEY (workspace_id, parent_revision_id, design_system_id)
        REFERENCES design_system_revision(workspace_id, id, design_system_id) ON DELETE RESTRICT,
    CHECK (status <> 'published' OR (
        content_digest IS NOT NULL AND content_digest <> '' AND
        published_by IS NOT NULL AND published_at IS NOT NULL
    ))
);

CREATE TABLE project_design_system_binding (
    id uuid PRIMARY KEY DEFAULT gen_random_uuid(),
    workspace_id uuid NOT NULL REFERENCES workspace(id) ON DELETE CASCADE,
    project_id uuid NOT NULL REFERENCES project(id) ON DELETE CASCADE,
    design_system_id uuid NOT NULL REFERENCES design_system_profile(id) ON DELETE RESTRICT,
    design_system_revision_id uuid NOT NULL,
    role text NOT NULL CHECK (role IN ('primary', 'inspiration')),
    position integer NOT NULL DEFAULT 0 CHECK (position >= 0),
    created_by uuid REFERENCES "user"(id) ON DELETE SET NULL,
    created_at timestamptz NOT NULL DEFAULT now(),
    UNIQUE (project_id, design_system_revision_id, role),
    CONSTRAINT project_design_system_binding_revision_fk
        FOREIGN KEY (workspace_id, design_system_revision_id, design_system_id)
        REFERENCES design_system_revision(workspace_id, id, design_system_id) ON DELETE RESTRICT
);

CREATE UNIQUE INDEX idx_project_design_system_one_primary
    ON project_design_system_binding(workspace_id, project_id)
    WHERE role = 'primary';

ALTER TABLE design_system_profile
    ADD CONSTRAINT design_system_profile_current_published_revision_fk
    FOREIGN KEY (workspace_id, current_published_revision_id, id)
    REFERENCES design_system_revision(workspace_id, id, design_system_id)
    ON DELETE SET NULL (current_published_revision_id);

ALTER TABLE design_restore_task
    ADD COLUMN primary_design_system_revision_id uuid REFERENCES design_system_revision(id) ON DELETE RESTRICT,
    ADD COLUMN primary_content_digest text,
    ADD COLUMN inspiration_design_system_revision_ids uuid[] NOT NULL DEFAULT '{}',
    ADD COLUMN design_context_resolved_at timestamptz;
```

Add a `BEFORE UPDATE` trigger that rejects changes to `manifest_json`, `design_md`, `tokens_css`, `artifact_index`, `content_digest`, `design_system_id`, or `revision_number` once the old status is `pending_review`, `published`, or `rejected`. Enforce the transition matrix `draft -> draft|pending_review`, `pending_review -> pending_review|published|rejected`, and terminal self-transitions only for `published`, `rejected`, and `archived`. Add a `BEFORE DELETE` guard for published revisions. The composite foreign keys must prevent a parent revision or current-published pointer from crossing design-system/workspace boundaries.

Add a `BEFORE INSERT OR UPDATE` binding trigger that checks the referenced revision is `published`, belongs to the same workspace/system, and rejects an `inspiration` row when the project has no primary. Add a `BEFORE DELETE` guard that prevents deleting a primary while inspirations remain; the API must either reject that operation or explicitly remove inspirations in the same transaction. Keep the partial unique index as the concurrency guard for primary replacement.

- [ ] **Step 4: Backfill legacy profiles as blocked review revisions**

Use one set-based `INSERT INTO design_system_revision SELECT` statement without joins. For each existing profile create revision `1` with:

```text
status = pending_review
manifest.source = {type: bundled, origin: "Multica legacy Figma UI specification"}
manifest.files = {design: DESIGN.md, tokens: tokens.css}
design_md = a short migration notice containing the legacy profile name
tokens_css = "/* Legacy UI specification requires regeneration. */\n:root {}\n"
content_digest = NULL
audit_report.passed = false
audit diagnostic code = legacy_package_requires_regeneration
artifact_index = source references to source_file_id, source_revision_id, profile_json, and analysis_errors
```

Include `legacy_was_default` in the audit metadata before changing any legacy flag. Do not set `current_published_revision_id`, do not create a binding, and do not publish. Leave `is_default` physically intact only as historical evidence so old code compiles until Task 8 switches the last consumer.

- [ ] **Step 5: Add lifecycle and pin queries without SQL joins**

Add sqlc queries with these exact names:

```text
GetDesignSystemIdentityInWorkspace
ListOwnedDesignSystemIdentities
CreateDesignSystemIdentity
LockDesignSystemIdentityForUpdate
GetNextDesignSystemRevisionNumber
CreateDesignSystemRevision
GetDesignSystemRevisionInWorkspace
ListDesignSystemRevisions
UpdateDraftDesignSystemRevisionPackage
PublishDesignSystemRevision
RejectDesignSystemRevision
SetDesignSystemCurrentPublishedRevision
ListProjectDesignSystemBindings
GetProjectPrimaryDesignSystemBinding
ListProjectInspirationDesignSystemBindings
DeleteProjectPrimaryDesignSystemBinding
CreateProjectDesignSystemBinding
PinDesignRestoreTaskDesignContext
GetPinnedDesignRestoreTaskDesignContext
```

`PinDesignRestoreTaskDesignContext` must update only where `design_context_resolved_at IS NULL`. List identities, revisions, and bindings separately and assemble API objects in Go. Verify the edited query section contains no `JOIN` token.

- [ ] **Step 6: Regenerate sqlc and run schema tests**

Run:

```bash
rtk make sqlc
rtk rg -n '\bJOIN\b' server/pkg/db/queries/design.sql server/pkg/db/generated/design.sql.go
rtk go -C server test ./internal/handler -run 'Test(DesignSystemIdentity|DesignSystemPublished|ProjectAllowsOnlyOnePrimary|ProjectBindingRejectsDraft|RestoreTaskDesignContextPin)' -count=1
```

Expected: the `rg` command has no output and exits `1`; all selected Go tests PASS.

- [ ] **Step 7: Verify the down migration fails safely when rollback would lose source-less systems**

The down migration must first raise an explicit exception when a post-130 identity has null legacy source IDs. Otherwise it drops restore pins, bindings, triggers/functions, the current revision FK/columns, revisions, restores source `NOT NULL`, and recreates `idx_design_system_profile_default_project`. Test only in a disposable test database; never run it against the user's active database.

- [ ] **Step 8: Commit schema and generated code**

Before committing run GitNexus `detect_changes()` and:

```bash
rtk git diff --check
rtk git add server/migrations/130_open_design_system_foundation.* server/pkg/db/queries/design.sql server/pkg/db/generated/design.sql.go server/pkg/db/generated/models.go server/internal/handler/design_system_schema_test.go
rtk git commit -m "feat(designs): add revisioned design systems"
```

### Task 3: Lifecycle, Review, Publish, And Binding API

**Files:**
- Create: `server/internal/handler/design_system.go`
- Create: `server/internal/handler/design_system_test.go`
- Modify: `server/internal/handler/design_file.go`
- Modify: `server/cmd/server/router.go`
- Modify: `server/cmd/server/integration_test.go`

**Interfaces:**
- Consumes: Task 1 `designsystem.AuditPackage`, Task 2 revision/binding queries.
- Produces:

```text
GET  /api/design-systems?project_id={projectId}
GET  /api/design-systems/{id}?project_id={projectId}
GET  /api/design-system-revisions/{id}
POST /api/design-systems/{id}/revisions
POST /api/design-system-revisions/{id}/publish
POST /api/design-system-revisions/{id}/reject
POST /api/projects/{projectId}/design-system-bindings
DELETE /api/projects/{projectId}/design-system-bindings/{bindingId}
```

The publish request is:

```json
{
  "bind_owner_project_as_primary": true
}
```

The binding request is:

```json
{
  "design_system_revision_id": "revision-uuid",
  "role": "primary",
  "position": 0
}
```

The rejection request is:

```json
{
  "review_note": "The Token evidence does not match the uploaded UI specification."
}
```

- [ ] **Step 1: Write failing lifecycle handler tests**

Add:

```go
func TestListDesignSystemsDistinguishesUnestablishedPendingAndPublished(t *testing.T)
func TestPublishDesignSystemRevisionRejectsFailedAudit(t *testing.T)
func TestPublishAndBindOwnerProjectIsAtomic(t *testing.T)
func TestPublishNewRevisionDoesNotMoveExistingBinding(t *testing.T)
func TestReplacePrimaryBindingRequiresExplicitRequest(t *testing.T)
func TestRejectDesignSystemRevisionRequiresReviewReason(t *testing.T)
func TestLegacySetDefaultEndpointCannotCreateNewBinding(t *testing.T)
func TestListDesignSystemsPreservesLegacyDesktopResponseFields(t *testing.T)
func TestDesignSystemLifecycleRejectsCrossWorkspaceIDs(t *testing.T)
```

The atomic test must force binding insertion failure and assert the revision remains `pending_review` and `current_published_revision_id` remains unchanged.

- [ ] **Step 2: Run tests and confirm lifecycle routes are absent**

Run:

```bash
rtk go -C server test ./internal/handler -run 'Test(ListDesignSystems|PublishDesignSystem|PublishAndBind|PublishNewRevision|ReplacePrimaryBinding|RejectDesignSystem|LegacySetDefault|DesignSystemLifecycle)' -count=1
```

Expected: FAIL on missing new handlers/routes or old behavior.

- [ ] **Step 3: Add API response types and bounded assembly**

Use these public response shapes:

```go
type DesignSystemRevisionSummaryResponse struct {
	ID             string          `json:"id"`
	RevisionNumber int32           `json:"revision_number"`
	Status         string          `json:"status"`
	ContentDigest  *string         `json:"content_digest,omitempty"`
	AuditReport    json.RawMessage `json:"audit_report"`
	CreatedAt      string          `json:"created_at"`
	PublishedAt    *string         `json:"published_at,omitempty"`
}

type ProjectDesignSystemBindingResponse struct {
	ID                     string `json:"id"`
	ProjectID              string `json:"project_id"`
	DesignSystemID         string `json:"design_system_id"`
	DesignSystemRevisionID string `json:"design_system_revision_id"`
	Role                   string `json:"role"`
	Position               int32  `json:"position"`
}

type DesignSystemResponse struct {
	ID                       string                               `json:"id"`
	WorkspaceID              string                               `json:"workspace_id"`
	OwnerProjectID           *string                              `json:"owner_project_id,omitempty"`
	Name                     string                               `json:"name"`
	Description              *string                              `json:"description,omitempty"`
	LatestRevision           *DesignSystemRevisionSummaryResponse `json:"latest_revision,omitempty"`
	CurrentPublishedRevision *DesignSystemRevisionSummaryResponse `json:"current_published_revision,omitempty"`
	ProjectBinding           *ProjectDesignSystemBindingResponse  `json:"project_binding,omitempty"`
	ProjectID                *string                              `json:"project_id,omitempty"`
	SourceFileID             string                               `json:"source_file_id"`
	SourceRevisionID         string                               `json:"source_revision_id"`
	ThumbnailURL             *string                              `json:"thumbnail_url,omitempty"`
	Status                   string                               `json:"status"`
	IsDefault                bool                                 `json:"is_default"`
	ProfileJSON              json.RawMessage                      `json:"profile_json"`
	AnalysisErrors           json.RawMessage                      `json:"analysis_errors"`
	CreatedAt                string                               `json:"created_at"`
	UpdatedAt                string                               `json:"updated_at"`
}
```

Load identities, their revisions, and project bindings through separate bounded queries. A project with no owned or bound identity returns `design_systems: []` and `primary_binding: null`; do not insert an empty identity row.

The trailing profile-shaped fields are API-boundary compatibility for installed desktop clients only. Populate source IDs/profile evidence when legacy data exists, otherwise use empty IDs plus `{}`/`[]`; always return `is_default=false`. New frontend code must ignore these fields, and no server mutation may derive a binding from them.

- [ ] **Step 4: Implement publish/reject/binding transactions**

For publish: parse IDs, require workspace access and `pending_review`, run `AuditPackage` again from database facts, compare the recomputed digest to the stored digest, lock the identity/revision/project, publish the revision, update current published revision, and optionally replace/create owner-project primary inside one transaction. `bind_owner_project_as_primary=false` must never alter bindings.

For binding: require a published revision, verify workspace/project/system ownership, lock the project, explicitly delete the old primary only for a `role=primary` request, and insert the requested fixed revision. An inspiration request never removes another binding.

For rejection: require a non-empty review reason, lock the revision, set `rejected`, `reviewed_by`, `reviewed_at`, and `review_note`; never mutate package content.

Keep `/api/design-systems/{id}/set-default` registered for installed clients, but change it to `410 Gone` with a structured message directing callers to revision publish/binding. It must not mutate `is_default` or create bindings.

- [ ] **Step 5: Register routes and route authorization coverage**

Register the new routes beside existing design routes. Add every mutating route to `server/cmd/server/integration_test.go` authorization matrices so unauthenticated calls cannot distinguish existing IDs from foreign IDs.

- [ ] **Step 6: Run focused backend tests**

Run:

```bash
rtk go -C server test ./internal/handler -run 'Test(ListDesignSystems|PublishDesignSystem|PublishAndBind|PublishNewRevision|ReplacePrimaryBinding|RejectDesignSystem|LegacySetDefault|DesignSystemLifecycle)' -count=1
rtk go -C server test ./cmd/server -run 'Test.*DesignSystem' -count=1
```

Expected: PASS. Confirm the failed-audit and forced-rollback cases leave no published/bound state.

- [ ] **Step 7: Commit the lifecycle API**

Before committing run GitNexus `detect_changes()` and:

```bash
rtk git diff --check
rtk git add server/internal/handler/design_system.go server/internal/handler/design_system_test.go server/internal/handler/design_file.go server/cmd/server/router.go server/cmd/server/integration_test.go
rtk git commit -m "feat(designs): add design system review lifecycle"
```

---

## Batch B: Design Center And Figma Import

### Task 4: Core Types, Zod Boundaries, And Query Options

**Files:**
- Modify: `packages/core/types/design.ts`
- Modify: `packages/core/api/schemas.ts`
- Modify: `packages/core/api/client.ts`
- Modify: `packages/core/api/client.test.ts`
- Modify: `packages/core/designs/keys.ts`
- Modify: `packages/core/designs/queries.ts`

**Interfaces:**
- Consumes: Task 3 API response/request shapes.
- Produces: `DesignSystem`, `DesignSystemRevision`, `ProjectDesignSystemBinding`, lifecycle client methods, and React Query options.

- [ ] **Step 1: Write malformed-response and request-contract tests first**

Add tests for:

```text
valid list with pending and published summaries
malformed list item -> empty list fallback
malformed detail -> identity-preserving empty fallback
publish body preserves bind_owner_project_as_primary exactly
binding body preserves fixed revision_id and role
reject body requires review_note
```

Do not assert only that `fetch` ran; assert parsed return values and exact URL/method/body.

- [ ] **Step 2: Run the core API tests and confirm failure**

Run:

```bash
rtk pnpm --filter @multica/core exec vitest run api/client.test.ts
```

Expected: FAIL because the new schemas/methods do not exist.

- [ ] **Step 3: Replace active profile types with revisioned types**

Define:

```ts
export type DesignSystemRevisionStatus = "draft" | "pending_review" | "published" | "rejected" | "archived";
export type ProjectDesignSystemBindingRole = "primary" | "inspiration";

export interface DesignSystemAuditDiagnostic {
  severity: "error" | "warning" | "info";
  code: string;
  path?: string;
  message: string;
}

export interface DesignSystemAuditReport {
  schema_version: "multica-design-system-audit/v1";
  passed: boolean;
  diagnostics: DesignSystemAuditDiagnostic[];
  tokens: Record<string, unknown>;
  artifacts: Array<Record<string, unknown>>;
}

export interface ProjectDesignSystemBinding {
  id: string;
  project_id: string;
  design_system_id: string;
  design_system_revision_id: string;
  role: ProjectDesignSystemBindingRole;
  position: number;
}

export interface DesignSystemRevisionSummary {
  id: string;
  revision_number: number;
  status: DesignSystemRevisionStatus;
  content_digest?: string | null;
  audit_report: DesignSystemAuditReport;
  created_at: string;
  published_at?: string | null;
}

export interface DesignSystem {
  id: string;
  workspace_id: string;
  owner_project_id?: string | null;
  name: string;
  description?: string | null;
  latest_revision?: DesignSystemRevisionSummary | null;
  current_published_revision?: DesignSystemRevisionSummary | null;
  project_binding?: ProjectDesignSystemBinding | null;
  created_at: string;
  updated_at: string;
}
```

Keep legacy `DesignSystemProfile` exported only where paused compiler code still imports it; mark it as legacy in a source comment and do not use it in new views/queries.

- [ ] **Step 4: Add loose Zod schemas with safe fallbacks**

Use `.loose()` at object boundaries, default arrays to `[]`, nullable optional nested summaries to `null`, and preserve the requested ID in detail fallback objects. Never cast `unknown` directly to the new types.

Add client methods named:

```text
listDesignSystems
getDesignSystem
getDesignSystemRevision
createDesignSystemRevision
reanalyzeDesignSystem
publishDesignSystemRevision
rejectDesignSystemRevision
createProjectDesignSystemBinding
deleteProjectDesignSystemBinding
```

- [ ] **Step 5: Add cache keys and invalidation contract**

Add keys for system list/detail, revision detail, and project bindings. Mutations must invalidate the selected project's design-system list plus the system/revision detail; they must not copy server state into Zustand.

- [ ] **Step 6: Run tests and typecheck core**

Run:

```bash
rtk pnpm --filter @multica/core exec vitest run api/client.test.ts
rtk pnpm --filter @multica/core typecheck
```

Expected: PASS.

- [ ] **Step 7: Commit the typed API boundary**

Before committing run GitNexus `detect_changes()` and:

```bash
rtk git diff --check
rtk git add packages/core/types/design.ts packages/core/api/schemas.ts packages/core/api/client.ts packages/core/api/client.test.ts packages/core/designs/keys.ts packages/core/designs/queries.ts
rtk git commit -m "feat(core): add revisioned design system API"
```

### Task 5: Design Center Three-State UI And Review Page

**Files:**
- Modify: `packages/views/designs/designs-page.tsx`
- Modify: `packages/views/designs/designs-page.test.tsx`
- Create: `packages/views/designs/design-system-page.tsx`
- Create: `packages/views/designs/design-system-page.test.tsx`
- Create: `packages/views/designs/design-system-preview.tsx`
- Create: `packages/views/designs/design-system-preview.test.tsx`
- Modify: `packages/views/designs/index.ts`
- Modify: `packages/core/paths/*`
- Create: `apps/web/app/[workspaceSlug]/(dashboard)/designs/systems/[id]/page.tsx`

**Interfaces:**
- Consumes: Task 4 query options and lifecycle mutations.
- Produces: `/[workspaceSlug]/designs/systems/{id}` review route and visible unestablished/pending/published states.

- [ ] **Step 1: Write failing view tests for all three states**

Add tests asserting:

```text
empty project: "尚未建立设计体系" and no fabricated card
pending revision: "待审核" plus audit error/warning counts and source label
published primary: "项目主体系" plus vN and digest prefix
published but unbound revision: no "项目主体系" badge
legacy blocked revision: publish action disabled and regeneration diagnostic visible
valid revision: online UI Kit renders nonblank controls from audited Token values
invalid or missing Token values: preview shows an audit blocker instead of applying raw CSS
first owner-project publish: primary button text is "发布并设为项目主体系"
existing primary on older revision: publishing a new revision does not call binding API
reject: requires reason and leaves card in rejected state after invalidation
```

- [ ] **Step 2: Run view tests and confirm the old UI-profile cards fail expectations**

Run:

```bash
rtk pnpm --filter @multica/views exec vitest run designs/designs-page.test.tsx designs/design-system-page.test.tsx designs/design-system-preview.test.tsx
```

Expected: FAIL because only Figma source cards/default badges exist.

- [ ] **Step 3: Replace UI-profile copy and cards with lifecycle states**

Rename visible labels from `UI 规范` to `设计体系`. Keep the design-system area within the existing project-scoped Design Center rather than adding a parallel workspace. Use an unframed empty state for no identity; use cards only for real identities/revisions.

Each card shows name, latest revision/status, audit counts, current project role, source summary, and updated time. It links to the new review route, not directly to the Figma source file.

- [ ] **Step 4: Build the review page**

The detail page contains un-nested full-width sections for:

```text
revision history and selected status
rendered DESIGN.md text
tokens.css source and parsed Token audit summary
manifest metadata
source/artifact index
audit diagnostics
online UI Kit evidence
publish/reject/binding actions
```

Do not add an online editor in this slice. Disable publish when `audit_report.passed=false`. For the first publish with no primary, the main action sends `bind_owner_project_as_primary=true`. Later revisions default to publish-only; a separate explicit confirmation performs project upgrade.

Build the online UI Kit from `audit_report.tokens.values` on a scoped preview root. Map the audited values to React `CSSProperties` custom properties, then render representative button, input, status tag, table row, confirmation dialog, and pagination states. Do not inject `tokens.css` through `dangerouslySetInnerHTML`, do not execute `components.html`, and do not invent a second Token schema. This is a deterministic review surface for the pinned Open Design contract, not an editor.

For a blocked legacy revision, `重新分析` calls the existing authenticated reanalysis route. Task 6 changes that route to create a new mutable draft revision and Agent Task; it must never rewrite the frozen legacy `pending_review` revision.

- [ ] **Step 5: Wire paths and Next.js adapter page**

Add `designSystemDetail(id)` to the workspace path contract and a thin app route that renders the shared view. Keep `packages/views` free of `next/*` imports.

- [ ] **Step 6: Run view tests and typecheck**

Run:

```bash
rtk pnpm --filter @multica/views exec vitest run designs/designs-page.test.tsx designs/design-system-page.test.tsx designs/design-system-preview.test.tsx
rtk pnpm typecheck
```

Expected: PASS with no text overlap in narrow test containers.

- [ ] **Step 7: Commit the Design Center UI**

Before committing run GitNexus `detect_changes()` and:

```bash
rtk git diff --check
rtk git add packages/views/designs packages/core/paths apps/web/app/'[workspaceSlug]'/'(dashboard)'/designs/systems
rtk git commit -m "feat(designs): add design system review workspace"
```

### Task 6: Figma UI-Spec Import To Audited Pending Revision

**Files:**
- Modify: `server/internal/handler/design_plugin.go`
- Modify: `server/internal/handler/design_file.go`
- Modify: `server/internal/handler/daemon.go`
- Modify: `server/internal/service/task.go`
- Modify: `server/internal/service/task_test.go`
- Modify: `server/internal/daemon/types.go`
- Modify: `server/internal/daemon/prompt.go`
- Modify: `server/internal/daemon/prompt_test.go`
- Modify: `server/pkg/db/queries/agent.sql`
- Generated: `server/pkg/db/generated/agent.sql.go`
- Modify: `server/internal/handler/design_file_test.go`
- Modify: `apps/figma-plugin/ui.html`
- Create: `apps/figma-plugin/ui.design-system-copy.test.cjs`

**Interfaces:**
- Consumes: Task 1 audit and Task 2 revision persistence.
- Produces: `design_system_revision_analyze` Agent tasks whose only successful server-side outcome is `pending_review`.

```go
const DesignSystemRevisionAnalyzeContextType = "design_system_revision_analyze"

type DesignSystemRevisionAnalyzeContext struct {
	Type                   string          `json:"type"`
	Prompt                 string          `json:"prompt"`
	RequesterID            string          `json:"requester_id"`
	WorkspaceID            string          `json:"workspace_id"`
	ProjectID              string          `json:"project_id"`
	AgentID                string          `json:"agent_id"`
	DesignSystemID         string          `json:"design_system_id"`
	TargetRevisionID       string          `json:"target_revision_id"`
	SourceFileID           string          `json:"source_file_id"`
	SourceRevisionID       string          `json:"source_revision_id"`
	CandidateLayers        json.RawMessage `json:"candidate_layers"`
	ExtractedTokens        json.RawMessage `json:"extracted_tokens,omitempty"`
	TextSamples            json.RawMessage `json:"text_samples,omitempty"`
	OutputPolicy           json.RawMessage `json:"output_policy"`
}
```

- [ ] **Step 1: Write failing import and malformed-output tests**

Add:

```go
func TestFigmaDesignSystemImportCreatesDraftRevisionWithoutBinding(t *testing.T)
func TestCompleteDesignSystemRevisionAnalyzeTaskCreatesPendingReviewPackage(t *testing.T)
func TestCompleteDesignSystemRevisionAnalyzeTaskRejectsUnknownFieldsAndTrailingText(t *testing.T)
func TestCompleteDesignSystemRevisionAnalyzeTaskStoresAuditFailureAndFailsTask(t *testing.T)
func TestFailedDesignSystemRevisionAnalyzeTaskNeverPublishesOrBinds(t *testing.T)
func TestRetryDesignSystemRevisionAnalyzeTaskTargetsSameDraftRevision(t *testing.T)
func TestReanalyzeLegacyDesignSystemCreatesNewDraftRevision(t *testing.T)
```

Assert four pieces of evidence together: task context contains the source and target IDs, Agent output is strictly parsed, revision package fields differ from legacy profile metadata, and no published/binding row exists.

- [ ] **Step 2: Run focused tests and confirm old profile behavior fails**

Run:

```bash
rtk go -C server test ./internal/handler -run 'Test(FigmaDesignSystemImport|CompleteDesignSystemRevision|FailedDesignSystemRevision|RetryDesignSystemRevision|ReanalyzeLegacyDesignSystem)' -count=1
```

Expected: FAIL because upload currently creates an analyzing profile with `IsDefault: true`.

- [ ] **Step 3: Replace the Agent output contract and prompt**

Require exactly one JSON object with no fences, trailing text, unknown fields, or lenient recovery. Parse it into this exact contract:

```go
type designSystemRevisionAnalyzeOutput struct {
	SchemaVersion string                            `json:"schema_version"`
	Manifest      designsystem.Manifest             `json:"manifest"`
	DesignMD      string                            `json:"design_md"`
	TokensCSS     string                            `json:"tokens_css"`
	TokenEvidence map[string]designsystem.TokenEvidence `json:"token_evidence"`
	Summary       string                            `json:"summary"`
}
```

`schema_version` must equal `multica-open-design-package/v1`. Handler tests must marshal this struct with the complete Task 1 `testdata/valid/DESIGN.md` and `tokens.css`, then separately test unknown fields, trailing prose, empty core files, and a manifest using `source.type=figma`.

The prompt must say that Figma provenance is supplied by Multica and must not be encoded as a new manifest source enum. It must require the full pinned Token schema, mark inferred/defaulted values as `low` or `fallback`, avoid inventing components not evidenced by the upload, and emit no recipe classifications or PageSpec.

- [ ] **Step 4: Create identity/revision/task atomically on plugin upload**

For `asset_type=design_system`, keep the uploaded `design_file`/`design_revision`, create a design-system identity and draft revision, stamp server-owned Figma provenance into `artifact_index`, and enqueue the revision-analysis task in one transaction. The response may include the identity and target revision, but never `is_default=true`, published status, or a binding.

Change `POST /api/design-system-profiles/{id}/reanalyze` to preserve its route for installed clients while applying the new semantics: load the identity's legacy Figma source, create revision `N+1` in `draft`, enqueue `design_system_revision_analyze`, and return the new target revision/task IDs. A retry of an active new task reuses its draft revision; a user-triggered reanalysis after a terminal task always creates a new revision.

Change the plugin copy from `上传后会成为当前项目默认设计系统` to `上传后将生成待审核的设计体系修订，通过审核发布后才能用于项目`.

- [ ] **Step 5: Make success/failure state atomic with Agent Task terminal state**

Add `TaskService.FailTaskWithMutation`, mirroring `CompleteTaskWithMutation`, so malformed output/audit failure can atomically fail the Agent Task and persist the draft revision's audit diagnostics. On valid output, run `AuditPackage`, compute digest, build/upload a deterministic zip when `DesignAssetStorage` is available, and atomically complete the Agent Task plus update only the matching `draft` revision to `pending_review`.

If optional zip upload fails, store a warning and keep database core facts reviewable; do not treat the zip as the fact source. If transaction persistence fails after upload, delete the uploaded object best-effort.

- [ ] **Step 6: Remove active recipe/default side effects from this task type**

Do not call `BuildComponentRecipeSet`, `UpdateDesignSystemProfileAnalysis`, `ClearDefaultDesignSystemProfilesForProject`, or `SetDesignSystemProfileDefault`. Keep the old task parser only for already-running legacy tasks until a separate cleanup; new imports and retries use only `design_system_revision_analyze`.

- [ ] **Step 7: Run backend, prompt, and plugin tests**

Run:

```bash
rtk make sqlc
rtk rg -n '\bJOIN\b' server/pkg/db/queries/agent.sql server/pkg/db/generated/agent.sql.go
rtk go -C server test ./internal/handler -run 'Test(FigmaDesignSystemImport|CompleteDesignSystemRevision|FailedDesignSystemRevision|RetryDesignSystemRevision|ReanalyzeLegacyDesignSystem)' -count=1
rtk go -C server test ./internal/service -run 'Test(FailTaskWithMutation|CompleteTaskWithMutation)' -count=1
rtk go -C server test ./internal/daemon -run 'TestBuildDesignSystemRevisionAnalyzePrompt' -count=1
rtk node --test apps/figma-plugin/code.grouped-export.test.cjs apps/figma-plugin/ui.design-system-copy.test.cjs
```

Expected: no SQL join output; all tests PASS.

- [ ] **Step 8: Commit Figma import and Agent analysis**

Before committing run GitNexus `detect_changes()` and:

```bash
rtk git diff --check
rtk git add server/internal/handler/design_plugin.go server/internal/handler/design_file.go server/internal/handler/daemon.go server/internal/service/task.go server/internal/service/task_test.go server/internal/daemon/types.go server/internal/daemon/prompt.go server/internal/daemon/prompt_test.go server/pkg/db/queries/agent.sql server/pkg/db/generated/agent.sql.go server/internal/handler/design_file_test.go apps/figma-plugin/ui.html apps/figma-plugin/ui.design-system-copy.test.cjs
rtk git commit -m "feat(designs): import Figma specs as review revisions"
```

---

## Batch C: Unified Context And Design Restore Consumer

### Task 7: Unified Design Context Resolver

**Files:**
- Create: `server/internal/service/design_context.go`
- Create: `server/internal/service/design_context_test.go`
- Modify: `server/pkg/db/queries/design.sql`
- Generated: `server/pkg/db/generated/design.sql.go`

**Interfaces:**
- Consumes: published fixed revision bindings and Task 1 digest/audit.
- Produces:

```go
type ResolvedDesignContext struct {
	Status       string                    `json:"status"` // resolved or missing
	ProjectID    string                    `json:"project_id"`
	Primary      *DesignSystemAgentPack    `json:"primary,omitempty"`
	Inspirations []DesignSystemAgentRef    `json:"inspirations"`
	ResolvedAt   time.Time                 `json:"resolved_at"`
}

type DesignSystemAgentPack struct {
	DesignSystemID string                  `json:"design_system_id"`
	RevisionID     string                  `json:"revision_id"`
	RevisionNumber int32                   `json:"revision_number"`
	ContentDigest  string                  `json:"content_digest"`
	Manifest       json.RawMessage         `json:"manifest"`
	DesignMD       string                  `json:"design_md"`
	TokensCSS      string                  `json:"tokens_css"`
	OptionalFiles  []designsystem.Artifact `json:"optional_files"`
}

type DesignSystemAgentRef struct {
	DesignSystemID string                  `json:"design_system_id"`
	RevisionID     string                  `json:"revision_id"`
	RevisionNumber int32                   `json:"revision_number"`
	ContentDigest  string                  `json:"content_digest"`
	Name           string                  `json:"name"`
	DesignSummary  string                  `json:"design_summary"`
	OptionalFiles  []designsystem.Artifact `json:"optional_files"`
}

type DesignContextQueries interface {
	GetProjectPrimaryDesignSystemBinding(context.Context, db.GetProjectPrimaryDesignSystemBindingParams) (db.ProjectDesignSystemBinding, error)
	ListProjectInspirationDesignSystemBindings(context.Context, db.ListProjectInspirationDesignSystemBindingsParams) ([]db.ProjectDesignSystemBinding, error)
	GetDesignSystemRevisionInWorkspace(context.Context, db.GetDesignSystemRevisionInWorkspaceParams) (db.DesignSystemRevision, error)
	GetDesignSystemIdentityInWorkspace(context.Context, db.GetDesignSystemIdentityInWorkspaceParams) (db.DesignSystemProfile, error)
}

type DesignContextResolver struct {
	Queries DesignContextQueries
	Now     func() time.Time
}

func (r DesignContextResolver) ResolveProject(ctx context.Context, workspaceID, projectID pgtype.UUID) (ResolvedDesignContext, error)
func (r DesignContextResolver) ResolvePinned(ctx context.Context, workspaceID pgtype.UUID, primary pgtype.UUID, inspirations []pgtype.UUID, resolvedAt time.Time) (ResolvedDesignContext, error)
```

- [ ] **Step 1: Write failing resolver tests**

Add:

```go
func TestResolveProjectReturnsMissingWithoutPrimary(t *testing.T)
func TestResolveProjectReturnsPublishedPrimaryAndOrderedInspirations(t *testing.T)
func TestResolveProjectNeverPromotesInspirationTokens(t *testing.T)
func TestResolveProjectRejectsDigestDrift(t *testing.T)
func TestResolvePinnedIgnoresLaterBindingChanges(t *testing.T)
func TestResolveProjectRejectsCrossWorkspaceBinding(t *testing.T)
```

- [ ] **Step 2: Run resolver tests and confirm the service is absent**

Run:

```bash
rtk go -C server test ./internal/service -run 'TestResolve(Project|Pinned)' -count=1
```

Expected: FAIL because `DesignContextResolver` does not exist.

- [ ] **Step 3: Implement bounded separate-query resolution**

Load the primary binding, then its revision/system, then ordered inspirations through separate queries. Reject non-published rows, recompute and verify every digest from database facts, and return a typed error on drift instead of silently treating a corrupt package as missing.

Primary carries the full Agent pack. Inspiration records carry only ID, revision, digest, name, DESIGN.md summary, and optional artifact index; they must not contain `tokens_css`. A project with no primary returns `status=missing`, an empty inspiration list, and a real `resolved_at`.

- [ ] **Step 4: Add exact-revision queries and regenerate sqlc**

Add queries for revision IDs and ordered binding rows without joins, run `make sqlc`, and verify no generated join syntax.

- [ ] **Step 5: Run resolver and service tests**

Run:

```bash
rtk rg -n '\bJOIN\b' server/pkg/db/queries/design.sql server/pkg/db/generated/design.sql.go
rtk go -C server test ./internal/service -run 'TestResolve(Project|Pinned)' -count=1
```

Expected: no join output; all resolver tests PASS.

- [ ] **Step 6: Commit the resolver**

Before committing run GitNexus `detect_changes()` and:

```bash
rtk git diff --check
rtk git add server/internal/service/design_context.go server/internal/service/design_context_test.go server/pkg/db/queries/design.sql server/pkg/db/generated/design.sql.go
rtk git commit -m "feat(designs): resolve fixed design context packs"
```

### Task 8: Pin Design Context Into Design Restore Dispatch

**Files:**
- Modify: `server/internal/handler/design_file.go`
- Modify: `server/internal/handler/design_file_test.go`
- Modify: `server/internal/service/task.go`
- Modify: `server/internal/service/design_restore_context_test.go`
- Modify: `server/internal/daemon/prompt.go`
- Modify: `server/internal/daemon/prompt_test.go`
- Modify: `server/internal/daemon/types.go`
- Modify: `packages/core/types/design.ts`
- Modify: `packages/core/api/schemas.ts`
- Modify: `packages/core/api/client.test.ts`

**Interfaces:**
- Consumes: Task 7 `ResolveProject` at first dispatch and `ResolvePinned` afterward.
- Produces: write-once restore-task revision/digest pins and `design_context` in `DesignRestoreTaskContext`.

- [ ] **Step 1: Write failing dispatch/prompt/API evidence tests**

Add:

```go
func TestDispatchDesignRestoreTaskPinsPublishedDesignContext(t *testing.T)
func TestDispatchDesignRestoreTaskPinsMissingContext(t *testing.T)
func TestDispatchDesignRestoreTaskKeepsPinAfterProjectUpgrade(t *testing.T)
func TestDispatchDesignRestoreTaskRejectsPublishedDigestDrift(t *testing.T)
func TestBuildDesignRestorePromptAppliesDesignAndPrimaryPriority(t *testing.T)
func TestBuildDesignRestorePromptTreatsInspirationsAsWeakAndTokenless(t *testing.T)
```

Update TypeScript tests to assert restore-task responses expose `primary_design_system_revision_id`, `primary_content_digest`, `inspiration_design_system_revision_ids`, and `design_context_resolved_at`, with null/empty fallbacks for older servers.

- [ ] **Step 2: Run focused tests and confirm old profile snapshot behavior**

Run:

```bash
rtk go -C server test ./internal/handler -run 'TestDispatchDesignRestoreTask(Pins|Keeps|Rejects)' -count=1
rtk go -C server test ./internal/daemon -run 'TestBuildDesignRestorePrompt(Applies|Treats)' -count=1
rtk pnpm --filter @multica/core exec vitest run api/client.test.ts
```

Expected: FAIL because dispatch still calls `designRestoreDesignSystemContext` and embeds mutable `profile_json`.

- [ ] **Step 3: Pin on first dispatch and resolve exact pins thereafter**

At dispatch, determine the project exactly as today. If `design_context_resolved_at` is null, call `ResolveProject`, write primary revision/digest/inspiration IDs and timestamp with `PinDesignRestoreTaskDesignContext`, then reload the row. If already pinned, call `ResolvePinned`; never consult current project bindings again for that task.

Pin the missing state too by writing only `design_context_resolved_at`; otherwise a later project binding would retroactively alter an already-dispatched task.

- [ ] **Step 4: Replace mutable profile context with the unified pack**

Change the task payload field from legacy `design_system` to `design_context`. The daemon prompt must state:

```text
selected design revision/frame/layers are authoritative for explicit page structure, content, and states
primary DESIGN.md defines visual intent and boundaries
primary tokens.css is the exact cloud Token contract and must not be replaced
inspirations may guide layout/pattern choices but cannot override or supply primary Tokens
local repository DESIGN.md is read-only auxiliary context after cloud primary
repository reality guides implementation feasibility
conflicts are reported, never silently resolved by changing explicit design content
```

Delete `designRestoreDesignSystemContext` only after GitNexus confirms no remaining caller. Do not add a fallback to `GetDefaultDesignSystemProfileForProject`.

- [ ] **Step 5: Expose pinned evidence through backend and core response types**

Add the four pin fields to `DesignRestoreTaskResponse` and Zod schemas with backward-compatible null/empty defaults. This is the evidence surface used to prove a task did not silently follow a later design-system revision.

- [ ] **Step 6: Run focused tests**

Run:

```bash
rtk go -C server test ./internal/handler -run 'TestDispatchDesignRestoreTask(Pins|Keeps|Rejects)' -count=1
rtk go -C server test ./internal/service -run 'Test.*DesignRestore' -count=1
rtk go -C server test ./internal/daemon -run 'TestBuildDesignRestorePrompt(Applies|Treats)' -count=1
rtk pnpm --filter @multica/core exec vitest run api/client.test.ts
```

Expected: PASS, including the binding-upgrade-after-dispatch regression.

- [ ] **Step 7: Commit the first consumer**

Before committing run GitNexus `detect_changes()` and:

```bash
rtk git diff --check
rtk git add server/internal/handler/design_file.go server/internal/handler/design_file_test.go server/internal/service/task.go server/internal/service/design_restore_context_test.go server/internal/daemon/prompt.go server/internal/daemon/prompt_test.go server/internal/daemon/types.go packages/core/types/design.ts packages/core/api/schemas.ts packages/core/api/client.test.ts
rtk git commit -m "feat(designs): pin design systems into restore tasks"
```

### Task 9: Full Verification And Product Evidence

**Files:**
- Create: `docs/product/design-center/open-design-foundation-validation.md`
- Modify: `docs/product/design-center/README.md`
- Modify: `docs/product/design-center/open-design-multica-mapping.md`

**Interfaces:**
- Consumes: Tasks 1-8.
- Produces: repeatable automated and live evidence; no new behavior.

- [ ] **Step 1: Run the focused automated suite**

Run:

```bash
rtk go -C server test ./internal/designsystem ./internal/service ./internal/handler ./internal/daemon -count=1
rtk go -C server test ./cmd/server -count=1
rtk pnpm --filter @multica/core exec vitest run api/client.test.ts
rtk pnpm --filter @multica/views exec vitest run designs/designs-page.test.tsx designs/design-system-page.test.tsx designs/design-system-preview.test.tsx
rtk node --test apps/figma-plugin/code.grouped-export.test.cjs apps/figma-plugin/ui.design-system-copy.test.cjs
rtk pnpm typecheck
rtk git diff --check
```

Record command, date, exit result, and any unrelated baseline failure separately. Do not call the slice complete when a relevant test fails.

- [ ] **Step 2: Apply migration only to an isolated disposable database**

Use a worktree/test database generated by existing Make targets. Apply migrations only through `129`, insert one legacy `design_system_profile` fixture with `is_default=true` and valid source file/revision IDs, then apply migration `130`. Assert it created exactly one blocked `pending_review` revision and zero bindings. Test migration down immediately before creating source-less identities. Never run `make db-reset` or migration down against the user's active local database.

Record these SQL facts:

```sql
SELECT status, count(*) FROM design_system_revision GROUP BY status ORDER BY status;
SELECT count(*) FROM project_design_system_binding;
SELECT count(*) FROM design_system_profile WHERE is_default = true;
```

The third result is historical evidence only; the second must remain zero for migrated legacy profiles.

- [ ] **Step 3: Perform the live Figma-to-review validation**

With backend and frontend started by the repository's documented `make start` flow:

1. Upload one Figma `UI 规范` to a selected project.
2. Capture the created `design_system_id`, target revision ID, Agent Task ID, and task context source IDs.
3. Inspect the Agent's exact final JSON and the persisted core fields; prove `DESIGN.md`/`tokens.css` are new package output rather than unchanged legacy metadata.
4. Verify the Design Center shows `待审核`, audit details, and no project primary binding.
5. Attempt a known-bad package and prove it remains draft/blocked, the Agent Task fails, and no publish/binding occurs.
6. Capture the Design Center and online UI Kit at desktop and mobile-width viewports with the user's Chrome/Playwright setup; verify nonblank rendering, no overlap, no horizontal clipping, and no console/network errors for package data.

- [ ] **Step 4: Perform publish, binding, and restore-pin validation**

1. Publish the valid revision with `发布并设为项目主体系` and capture revision status, content digest, current pointer, and binding row.
2. Create a second valid revision and publish it without upgrade; prove the project binding still points to the first revision.
3. Dispatch a design restore task and capture its four pin fields plus the exact `agent_task_queue.context.design_context`.
4. Explicitly upgrade the project binding to the second revision after dispatch.
5. Re-read the original restore task and prove its revision ID/digest/context remain pinned to the first revision.
6. Inspect the built Agent prompt and prove primary Tokens are present while inspiration Tokens are absent.

Task completion alone is not evidence. The validation document must include API/DB IDs, persisted content/digests, prompt excerpts, and screenshots of the three Design Center states.

- [ ] **Step 5: Update canonical product memory with implementation status**

Link `open-design-foundation-validation.md` from `docs/product/design-center/README.md`. In `open-design-multica-mapping.md`, mark only the implemented first consumer and lifecycle pieces as implemented; retain community resources, online editing, UI Agent design generation, and Figma write-back as unimplemented.

- [ ] **Step 6: Run final change-scope review and commit evidence**

Run GitNexus `detect_changes({scope: "compare", base_ref: "main"})`, inspect every changed process, then:

```bash
rtk git diff --check
rtk git status --short
rtk git add docs/product/design-center/open-design-foundation-validation.md docs/product/design-center/README.md docs/product/design-center/open-design-multica-mapping.md
rtk git commit -m "docs(designs): record Open Design foundation evidence"
```

Expected: only planned design-system/package/restore flows are affected; unrelated dirty files remain unstaged.

---

## Batch Checkpoints

After Batch A, report:

```text
package audit cases and digest evidence
migration/backfill counts from an isolated database
publish/binding transaction tests
files changed and exact commits
```

After Batch B, report:

```text
malformed API fallback tests
screenshots or rendered evidence for unestablished/pending/published states
Figma Agent input, exact output, persisted revision fields, and no-binding proof
```

After Batch C, report:

```text
resolved and pinned revision/digest IDs
binding-upgrade regression proof
actual restore prompt evidence
full automated/live validation results and residual risks
```

Do not continue into the next batch when a checkpoint exposes data migration risk, a bad draft that can publish, a task result that is not persisted, or a restore context that is not fixed to exact revisions.
