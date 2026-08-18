# PMO Source-Parity Preview and Agent Assignment Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Make PMO previews mirror the source requirement and Milestone schedule, add PRD/priority metadata, fix shared scrolling, and assign imported work to the responsible person's concrete Agent.

**Architecture:** Extend the existing version-1 JSON snapshot with optional display metadata while preserving old snapshots. Resolve external owners through member email only as an intermediate lookup, persist concrete Agent IDs in existing PMO sync links, extend the link-type CHECK constraint to allow `agent`, and render the existing field-level diff inside a source-shaped requirement/schedule component.

**Tech Stack:** Go 1.26, Chi handlers, sqlc-generated queries, PostgreSQL JSONB sync snapshots, TypeScript, React, TanStack Query, Vitest, Testing Library, shared `packages/views` Web/Desktop UI.

---

## File Map

**Backend contract and prompt**

- Modify `server/internal/service/pmo_contract.go`: optional priority, PRD URL, and Milestone name fields plus trust-boundary validation.
- Modify `server/internal/service/pmo_contract_test.go`: parsing, normalization, old-snapshot, and invalid-URL coverage.
- Modify `server/internal/service/pmo.go`: acquisition prompt requests the new metadata.
- Modify `server/internal/service/pmo_test.go`: prompt contract assertions if existing coverage lives there.

**Backend Agent mapping and apply**

- Modify `server/internal/service/pmo_assignee.go`: exact-email member lookup followed by unique eligible Agent selection.
- Modify `server/internal/service/pmo_assignee_test.go`: zero/one/multiple Agent resolution coverage.
- Modify `server/internal/service/pmo_apply.go`: persist Agent mappings, upgrade unambiguous legacy member links, and write Agent lead/assignee types.
- Modify `server/internal/service/pmo_apply_test.go`: hierarchy creation/update and unresolved behavior with Agent IDs.
- Modify `server/internal/handler/pmo.go`: accept and validate `agent_id`.
- Modify `server/internal/handler/pmo_apply_test.go`: endpoint body, workspace/runtime validation, and Agent assignment coverage.
- Create `server/migrations/890_pmo_sync_link_agent_type.{up,down}.sql`: allow Agent link types and downgrade them to unresolved on rollback.

**Core API contract**

- Modify `packages/core/types/pmo.ts`: rename mapping request field to `agent_id`.
- Modify `packages/core/api/client.ts`: send `agent_id`.
- Modify `packages/core/api/client.test.ts`: request-body assertion.
- Modify `packages/core/pmo/mutations.ts`: mutation variable becomes `agentId`.

**Shared preview and mapping UI**

- Create `packages/views/pmo/pmo-source-preview.tsx`: defensive snapshot parsing, requirement summary, Milestone grouping, one-task-per-row schedule, and diff-aware cells.
- Modify `packages/views/pmo/pmo-diff.tsx`: expose the existing conflict/value renderer needed by the source-shaped preview and keep owner-reference parsing typed by external type/key.
- Modify `packages/views/pmo/pmo-config-detail-page.tsx`: mount the new preview, provide vertical scrolling, and render Agent mapping options with owner names.
- Modify `packages/views/pmo/pmo-config-detail-page.test.tsx`: source-parity, scrolling, mapping, ambiguity, and mutation coverage.

No generated SQL, platform-specific Web component, Desktop component, or new dependency is required. One CHECK-constraint migration is required because the existing PMO link table only permits `project`, `issue`, and `member` local types.

## Task 1: Extend the Snapshot Contract

**Files:**

- Modify: `server/internal/service/pmo_contract.go:13-75,218-296`
- Modify: `server/internal/service/pmo_contract_test.go:10-175,190-240`
- Modify: `server/internal/service/pmo.go:321-335`

- [ ] **Step 1: Add failing metadata-preservation and URL-validation tests**

Add a valid `priority`, `prd_url`, and `scheme_name` to the shared fixture and assert they survive normalization:

```go
func TestParsePMOSnapshotPreservesDisplayMetadata(t *testing.T) {
	got := mustParsePMOSnapshot(t, validPMOSnapshotJSON())
	if got.Parent.Priority != "P2-3" {
		t.Fatalf("priority = %q", got.Parent.Priority)
	}
	if got.Parent.PRDURL == nil || *got.Parent.PRDURL != "https://soyoung.feishu.cn/wiki/example" {
		t.Fatalf("prd_url = %#v", got.Parent.PRDURL)
	}
	if got.Children[0].Tasks[0].SchemeName != "M4-开发-前端" {
		t.Fatalf("scheme_name = %q", got.Children[0].Tasks[0].SchemeName)
	}
}

func TestParsePMOSnapshotAllowsOldDisplayMetadataShape(t *testing.T) {
	raw := mutatePMOSnapshotJSON(t, func(snapshot map[string]any) {
		parent := snapshot["parent_requirement"].(map[string]any)
		delete(parent, "priority")
		delete(parent, "prd_url")
		child := snapshot["child_requirements"].([]any)[0].(map[string]any)
		delete(child["tasks"].([]any)[0].(map[string]any), "scheme_name")
	})
	if _, err := ParsePMOSnapshot(raw); err != nil {
		t.Fatalf("old snapshot must remain valid: %v", err)
	}
}

func TestParsePMOSnapshotRejectsUnsafePRDURL(t *testing.T) {
	raw := mutatePMOSnapshotJSON(t, func(snapshot map[string]any) {
		parent := snapshot["parent_requirement"].(map[string]any)
		parent["prd_url"] = "javascript:alert(1)"
	})
	_, err := ParsePMOSnapshot(raw)
	if err == nil || !strings.Contains(err.Error(), "prd_url must be an absolute http or https URL") {
		t.Fatalf("expected prd_url validation error, got %v", err)
	}
}
```

- [ ] **Step 2: Run the focused contract tests and confirm failure**

Run:

```bash
cd server && go test ./internal/service -run 'TestParsePMOSnapshot(PreservesDisplayMetadata|AllowsOldDisplayMetadataShape|RejectsUnsafePRDURL)' -count=1
```

Expected: compile failure because `Priority`, `PRDURL`, and `SchemeName` do not exist.

- [ ] **Step 3: Add optional fields and bounded validation**

Add the fields without changing schema version:

```go
type PMORequirement struct {
	Key           string            `json:"key"`
	DisplayNumber string            `json:"display_number"`
	NumericID     int64             `json:"numeric_id"`
	Title         string            `json:"title"`
	Description   string            `json:"description"`
	SourceStatus  string            `json:"source_status"`
	Status        string            `json:"status"`
	Priority      string            `json:"priority,omitempty"`
	PRDURL        *string           `json:"prd_url,omitempty"`
	Owner         *PMOExternalOwner `json:"owner"`
	StartDate     *string           `json:"start_date"`
	DueDate       *string           `json:"due_date"`
	Workload      *float64          `json:"workload"`
	Tasks         []PMOTask         `json:"tasks"`
}

type PMOTask struct {
	TaskID       string            `json:"task_id"`
	SchemeID     string            `json:"scheme_id"`
	SchemeName   string            `json:"scheme_name,omitempty"`
	Title        string            `json:"title"`
	Description  string            `json:"description"`
	SourceStatus string            `json:"source_status"`
	Status       string            `json:"status"`
	Owner        *PMOExternalOwner `json:"owner"`
	StartDate    *string           `json:"start_date"`
	DueDate      *string           `json:"due_date"`
	Workload     *float64          `json:"workload"`
	UpdatedAt    *string           `json:"updated_at"`
}
```

Validate priority and Milestone names with `validatePMOText`. Validate `prd_url`
with `net/url.Parse`, requiring `URL.IsAbs()` and scheme `http` or `https`, then
trim it during normalization. Do not fetch the URL.

- [ ] **Step 4: Update the acquisition prompt**

Change the example and task description so new runs produce:

```text
parent_requirement also contains optional priority and prd_url.
Each task contains task_id, scheme_id, scheme_name, title, description,
source_status, status, owner, start_date, due_date, workload, and updated_at.
Use the source Milestone display label for scheme_name. Use the explicit PM
PRD-link field for prd_url; do not invent or fetch a URL.
```

- [ ] **Step 5: Run all PMO contract and prompt tests**

Run:

```bash
cd server && go test ./internal/service -run 'Test(ParsePMOSnapshot|BuildPMOSyncPrompt)' -count=1
```

Expected: PASS.

- [ ] **Step 6: Commit the contract slice**

```bash
git add server/internal/service/pmo_contract.go server/internal/service/pmo_contract_test.go server/internal/service/pmo.go server/internal/service/pmo_test.go
git commit -m "feat(pmo): retain source preview metadata"
```

## Task 2: Resolve External Owners to Concrete Agents

**Files:**

- Modify: `server/internal/service/pmo_assignee.go:1-107`
- Modify: `server/internal/service/pmo_assignee_test.go:1-55`
- Modify: `server/internal/service/pmo_apply.go:204-226,884-945`

- [ ] **Step 1: Add failing pure resolver tests**

Replace the user-ID-only resolver expectation with explicit Agent candidates:

```go
type pmoAgentCandidate struct {
	ID           string
	OwnerID      string
	RuntimeBound bool
}

func TestMatchPMOAgentMappingsUsesOnlyUniqueEligibleOwnedAgent(t *testing.T) {
	owners := map[string]*PMOExternalOwner{
		"fengyujie": {ExternalID: "fengyujie", DisplayName: "风尘（冯钰杰）"},
		"multi":     {ExternalID: "multi", DisplayName: "Multiple"},
		"missing":   {ExternalID: "missing", DisplayName: "Missing"},
	}
	emailToUser := map[string]string{
		"fengyujie@soyoung.com": "user-1",
		"multi@soyoung.com":     "user-2",
	}
	agents := []pmoAgentCandidate{
		{ID: "agent-1", OwnerID: "user-1", RuntimeBound: true},
		{ID: "agent-unbound", OwnerID: "user-1", RuntimeBound: false},
		{ID: "agent-2", OwnerID: "user-2", RuntimeBound: true},
		{ID: "agent-3", OwnerID: "user-2", RuntimeBound: true},
	}
	got := matchPMOAgentMappings(owners, emailToUser, agents, nil)
	if got["fengyujie"] != "agent-1" {
		t.Fatalf("unique mapping = %q", got["fengyujie"])
	}
	if got["multi"] != "" || got["missing"] != "" {
		t.Fatalf("ambiguous or missing owner must stay unresolved: %#v", got)
	}
}
```

Add a second test proving an explicit Agent mapping wins over automatic email
resolution.

- [ ] **Step 2: Run the resolver tests and confirm failure**

Run:

```bash
cd server && go test ./internal/service -run 'TestMatchPMOAgentMappings' -count=1
```

Expected: compile failure because the Agent resolver does not exist.

- [ ] **Step 3: Implement the minimal two-stage resolver**

Keep `normalizePMOOwnerEmail` unchanged. Replace the final member-ID result with
an Agent-ID result:

```go
func matchPMOAgentMappings(
	owners map[string]*PMOExternalOwner,
	memberEmailToUserID map[string]string,
	agents []pmoAgentCandidate,
	existing map[string]string,
) map[string]string {
	result := make(map[string]string, len(existing)+len(owners))
	for externalID, agentID := range existing {
		if externalID != "" && agentID != "" {
			result[externalID] = agentID
		}
	}
	owned := map[string][]string{}
	for _, agent := range agents {
		if agent.RuntimeBound {
			owned[agent.OwnerID] = append(owned[agent.OwnerID], agent.ID)
		}
	}
	for externalID := range owners {
		if result[externalID] != "" {
			continue
		}
		userID := memberEmailToUserID[normalizePMOOwnerEmail(externalID)]
		if candidates := owned[userID]; len(candidates) == 1 {
			result[externalID] = candidates[0]
		}
	}
	return result
}
```

`ResolvePMOAssigneeMappings` reuses `ListMembersWithUser` and `ListAgents`.
Convert only unarchived, user-kind, runtime-bound rows into candidates. Do not
check transient runtime online status.

- [ ] **Step 4: Add legacy member-link resolution**

When loading assignee links in apply, split persisted links by `LocalType`:

```go
explicitAgentMappings := map[string]string{}
legacyMemberMappings := map[string]string{}
for _, link := range linkRows {
	if link.ExternalType != pmoExternalTypeAssignee || !link.LocalID.Valid {
		continue
	}
	switch link.LocalType.String {
	case pmoLocalTypeAgent:
		explicitAgentMappings[link.ExternalKey] = util.UUIDToString(link.LocalID)
	case pmoLocalTypeMember:
		legacyMemberMappings[link.ExternalKey] = util.UUIDToString(link.LocalID)
	}
}
```

Resolve a legacy member ID only when that owner has exactly one eligible Agent.
Otherwise leave it unresolved. Agent mappings remain explicit and always win.

- [ ] **Step 5: Run assignee and apply resolver tests**

Run:

```bash
cd server && go test ./internal/service -run 'Test(NormalizePMOOwnerEmail|MatchPMOAgentMappings|ApplyPMORunAutoMaps|ApplyPMORunKeepsExplicit)' -count=1
```

Expected: PASS.

- [ ] **Step 6: Commit the resolver slice**

```bash
git add server/internal/service/pmo_assignee.go server/internal/service/pmo_assignee_test.go server/internal/service/pmo_apply.go server/internal/service/pmo_apply_test.go
git commit -m "fix(pmo): resolve owners to concrete agents"
```

## Task 3: Persist Agent Mappings and Apply Agent Assignment

**Files:**

- Modify: `server/internal/service/pmo_apply.go:540-585,730-756,884-945`
- Modify: `server/internal/service/pmo_apply_test.go:266-372,665-775`
- Modify: `server/internal/handler/pmo.go:396-426`
- Modify: `server/internal/handler/pmo_apply_test.go:100-205`
- Create: `server/migrations/890_pmo_sync_link_agent_type.up.sql`
- Create: `server/migrations/890_pmo_sync_link_agent_type.down.sql`

- [ ] **Step 1: Add failing endpoint and apply tests**

Change the handler test request to Agent identity:

```go
body := map[string]any{"agent_id": agentID}
req := withURLParam(newRequest(http.MethodPut,
	"/api/pmo/configs/"+config.ID+"/assignees/EXT-U-001", body), "id", config.ID)
req = withURLParam(req, "externalKey", "EXT-U-001")
```

After apply, assert polymorphic assignment:

```go
var assigneeType, assigneeID string
if err := pool.QueryRow(ctx,
	`SELECT assignee_type, assignee_id::text FROM issue WHERE title = $1`,
	"Mapped task").Scan(&assigneeType, &assigneeID); err != nil {
	t.Fatal(err)
}
if assigneeType != "agent" || assigneeID != agentID {
	t.Fatalf("assignee = %s/%s", assigneeType, assigneeID)
}
```

Add equivalent root project `lead_type='agent'` coverage. Add rejection tests for
cross-workspace, archived, and runtime-unbound Agents. Keep the unresolved-owner
test asserting that unrelated entities still apply.

- [ ] **Step 2: Run the focused endpoint/apply tests and confirm failure**

Run:

```bash
cd server && go test ./internal/handler ./internal/service -run 'Test(SetPMOAssigneeMappingEndpoint|ApplyPMORunUsesMappedAssigneeViaEndpoint|ApplyPMORunFirstImportCreatesHierarchy|ApplyPMORunUnresolvedAndMappedAssignees)' -count=1
```

Expected: FAIL because the handler expects `member_id` and apply writes member
types.

- [ ] **Step 3: Change the service mapping boundary to Agent ID**

Use the existing workspace Agent query and validate runtime binding:

```go
var ErrPMOAgentNotFound = errors.New("pmo agent not found")

func (s *PMOService) SetAssigneeMapping(
	ctx context.Context,
	workspaceID, configID pgtype.UUID,
	externalKey string,
	agentID pgtype.UUID,
) (db.PmoSyncLink, error) {
	agent, err := s.Queries.GetAgentInWorkspace(ctx, db.GetAgentInWorkspaceParams{
		ID: agentID, WorkspaceID: workspaceID,
	})
	if err != nil || !agent.ID.Valid || agent.ArchivedAt.Valid || !agent.RuntimeID.Valid {
		return db.PmoSyncLink{}, ErrPMOAgentNotFound
	}
	localJSON, _ := json.Marshal(map[string]any{"agent_id": util.UUIDToString(agent.ID)})
	return s.Queries.UpsertPMOSyncLink(ctx, db.UpsertPMOSyncLinkParams{
		WorkspaceID: workspaceID,
		ConfigID: configID,
		ExternalType: pmoExternalTypeAssignee,
		ExternalKey: externalKey,
		BaselineExternal: externalJSON,
		BaselineLocal: localJSON,
		ExternalMetadata: []byte(`{}`),
		LocalType: pgtype.Text{String: pmoLocalTypeAgent, Valid: true},
		LocalID: agent.ID,
	})
}
```

Use the repository's existing PMO error-to-HTTP pattern for a 404/400 response;
do not expose database errors.

- [ ] **Step 4: Change handler JSON from `member_id` to `agent_id`**

```go
var req struct {
	AgentID string `json:"agent_id"`
}
agentID, ok := parseUUIDOrBadRequest(w, req.AgentID, "agent_id")
if !ok {
	return
}
link, err := h.PMOService.SetAssigneeMapping(r.Context(), workspaceID, configID, externalKey, agentID)
```

- [ ] **Step 5: Persist Agent links and write Agent polymorphic types**

In `upsertAssigneeLinks`, set `local_type='agent'` and baseline
`{"agent_id":"..."}` for newly resolved mappings. Preserve existing explicit
Agent links. Upgrade an unambiguous legacy member link to Agent on successful
apply.

For project and issue writes, use:

```go
LeadType: pgtype.Text{String: pmoLocalTypeAgent, Valid: true}
```

and:

```go
params.AssigneeType = pgtype.Text{String: pmoLocalTypeAgent, Valid: true}
```

for both create and update paths.

- [ ] **Step 6: Run focused and package-level PMO backend tests**

Run:

```bash
cd server && go test ./internal/service ./internal/handler -run 'PMO' -count=1
```

Expected: PASS.

- [ ] **Step 7: Commit the persistence/apply slice**

```bash
git add server/internal/service/pmo_apply.go server/internal/service/pmo_apply_test.go server/internal/handler/pmo.go server/internal/handler/pmo_apply_test.go
git commit -m "fix(pmo): assign imported work to agents"
```

## Task 4: Update the Core API Mapping Contract

**Files:**

- Modify: `packages/core/types/pmo.ts:104-126`
- Modify: `packages/core/api/client.ts:4735-4750`
- Modify: `packages/core/api/client.test.ts:2510-2555`
- Modify: `packages/core/pmo/mutations.ts:91-109`

- [ ] **Step 1: Change the client test to require `agent_id`**

```ts
await client.setPMOAssigneeMapping("ws-1", "cfg-1", "EXT-U-001", "agent-1");

expect(fetchMock).toHaveBeenCalledWith(
  "https://api.example.test/api/pmo/configs/cfg-1/assignees/EXT-U-001",
  expect.objectContaining({
    method: "PUT",
    body: JSON.stringify({ agent_id: "agent-1" }),
  }),
);
```

- [ ] **Step 2: Run the client test and confirm failure**

Run:

```bash
pnpm --filter @multica/core exec vitest run api/client.test.ts -t 'PMO assignee mapping'
```

Expected: FAIL showing `{member_id: ...}`.

- [ ] **Step 3: Rename the request and mutation variable**

```ts
export interface SetPMOAssigneeMappingRequest {
  agent_id: string;
}
```

```ts
async setPMOAssigneeMapping(
  _wsId: string,
  configId: string,
  externalKey: string,
  agentId: string,
) {
  return this.fetch(`/api/pmo/configs/${configId}/assignees/${encodeURIComponent(externalKey)}`, {
    method: "PUT",
    body: JSON.stringify({ agent_id: agentId }),
  });
}
```

The mutation input becomes `{configId, externalKey, agentId}` and passes
`agentId` through unchanged.

- [ ] **Step 4: Run core tests and typecheck**

Run:

```bash
pnpm --filter @multica/core exec vitest run api/client.test.ts pmo
pnpm --filter @multica/core typecheck
```

Expected: PASS.

- [ ] **Step 5: Commit the core contract slice**

```bash
git add packages/core/types/pmo.ts packages/core/api/client.ts packages/core/api/client.test.ts packages/core/pmo/mutations.ts
git commit -m "fix(pmo): send agent mapping identity"
```

## Task 5: Build the Source-Shaped Preview

**Files:**

- Create: `packages/views/pmo/pmo-source-preview.tsx`
- Modify: `packages/views/pmo/pmo-diff.tsx`
- Modify: `packages/views/pmo/pmo-config-detail-page.tsx:80-430,480-590`
- Modify: `packages/views/pmo/pmo-config-detail-page.test.tsx:408-607`

- [ ] **Step 1: Replace the field-row grouping test with source-parity tests**

Use a run fixture containing the reviewed shape:

```ts
source_snapshot: {
  schema_version: "1",
  snapshot_complete: true,
  parent_requirement: {
    key: "SY-P-20260452",
    display_number: "SY-P-20260452",
    numeric_id: 136076,
    title: "院务系统-开单-增加美团订单券码校验-1.0",
    description: "https://soyoung.feishu.cn/wiki/Ifl9wASw2iWHL4kEbN1cpF3Ynje",
    source_status: "已上线",
    status: "completed",
    priority: "P2-3",
    prd_url: "https://soyoung.feishu.cn/wiki/Ifl9wASw2iWHL4kEbN1cpF3Ynje",
    owner: { external_id: "zhudi@soyoung.com", display_name: "药丸（朱迪）" },
    start_date: "2026-07-21",
    due_date: "2026-08-11",
    workload: 15,
    tasks: [],
  },
  child_requirements: [],
  tasks: [
    {
      task_id: "TASK-FE-1",
      scheme_id: "scheme-fe",
      scheme_name: "M4-开发-前端",
      title: "院务系统-开单处理",
      description: "",
      source_status: "未开始",
      status: "todo",
      owner: { external_id: "fengyujie", display_name: "风尘（冯钰杰）" },
      start_date: "2026-07-24",
      due_date: "2026-07-24",
      workload: 1,
      updated_at: null,
    },
  ],
}
```

Assert:

```ts
expect(screen.getByRole("heading", { name: "院务系统-开单-增加美团订单券码校验-1.0" })).toBeInTheDocument();
expect(screen.getByText("P2-3")).toBeInTheDocument();
expect(screen.getByRole("link", { name: /PRD/ })).toHaveAttribute(
  "href",
  "https://soyoung.feishu.cn/wiki/Ifl9wASw2iWHL4kEbN1cpF3Ynje",
);
expect(screen.getByText("M4-开发-前端")).toBeInTheDocument();
expect(screen.getAllByRole("row", { name: /院务系统-开单处理/ })).toHaveLength(1);
```

Add a second test proving two tasks in the same Milestone render once each and
an old snapshot falls back to `scheme_id` and the first safe description URL.

- [ ] **Step 2: Run the page tests and confirm failure**

Run:

```bash
pnpm --filter @multica/views exec vitest run pmo/pmo-config-detail-page.test.tsx -t 'source preview'
```

Expected: FAIL because the generic diff table has no requirement summary or
Milestone schedule.

- [ ] **Step 3: Add defensive snapshot parsing**

Create a focused source preview module. Parse unknown JSON without casting the
network payload blindly:

```ts
interface SourceOwner {
  externalId: string;
  displayName: string;
}

interface SourceTask {
  taskId: string;
  schemeId: string;
  schemeName: string;
  title: string;
  owner: SourceOwner | null;
  startDate: string | null;
  dueDate: string | null;
  workload: number | null;
  sourceStatus: string;
  status: string;
}

interface SourceRequirement {
  key: string;
  displayNumber: string;
  title: string;
  sourceStatus: string;
  status: string;
  priority: string;
  prdUrl: string | null;
  owner: SourceOwner | null;
  startDate: string | null;
  dueDate: string | null;
  workload: number | null;
  tasks: SourceTask[];
}
```

Use small `isRecord`, `readString`, `readNullableString`, and `readOwner`
helpers. Accept only `http:` and `https:` URLs through the platform `URL` class.
Return `null` for structurally unusable snapshots so the existing empty state
remains available. Attach the top-level snapshot `tasks` array to the parsed
parent requirement and keep each child requirement's own `tasks` array.

- [ ] **Step 4: Render requirement summary and grouped schedules**

Export one component:

```tsx
export function PMOSourcePreview({
  snapshot,
  diff,
  filter,
  selections,
  onSelectionChange,
}: PMOSourcePreviewProps) {
  const source = useMemo(() => parsePMOSourceView(snapshot), [snapshot]);
  if (!source) return null;
  return (
    <div className="space-y-6 py-4">
      <RequirementSummary requirement={source.parent} diff={diff} />
      <ScheduleTable requirement={source.parent} diff={diff} filter={filter} />
      {source.children.map((child) => (
        <section key={child.key} className="space-y-3">
          <RequirementSummary requirement={child} diff={diff} />
          <ScheduleTable requirement={child} diff={diff} filter={filter} />
        </section>
      ))}
    </div>
  );
}
```

Group with a stable insertion-order `Map` keyed by
`${task.schemeId}\0${task.schemeName}`. Use `schemeName || schemeId` for the
heading. Render `sourceStatus` as the PM-facing task progress and use canonical
`status` only for diff comparison and local apply behavior. Reuse existing
badges, native conflict controls, and semantic tokens; do not create
card-inside-card layouts.

- [ ] **Step 5: Replace the generic preview table**

In `PMOConfigDetailPage`, render `PMOSourcePreview` when a normalized snapshot
is available. Keep the old table only as the malformed/unknown snapshot fallback
needed by installed clients and historical runs. Remove the interim row-span
grouping code because source-shaped rows replace it.

- [ ] **Step 6: Run the full PMO view test file**

Run:

```bash
pnpm --filter @multica/views exec vitest run pmo/pmo-config-detail-page.test.tsx
```

Expected: PASS.

- [ ] **Step 7: Commit the preview slice**

```bash
git add packages/views/pmo/pmo-source-preview.tsx packages/views/pmo/pmo-diff.tsx packages/views/pmo/pmo-config-detail-page.tsx packages/views/pmo/pmo-config-detail-page.test.tsx
git commit -m "fix(pmo): mirror source requirement schedule"
```

## Task 6: Finish Agent Mapping UI and Shared Scrolling

**Files:**

- Modify: `packages/views/pmo/pmo-config-detail-page.tsx:80-215,429-660`
- Modify: `packages/views/pmo/pmo-config-detail-page.test.tsx:608-770`

- [ ] **Step 1: Add failing Agent selector tests**

Mock members and Agents with ownership metadata:

```ts
const members = [
  { user_id: "user-feng", name: "冯钰杰" },
  { user_id: "user-other", name: "其他成员" },
];
const agents = [
  { id: "agent-feng", name: "前端 Agent", owner_id: "user-feng", runtime_bound: true, archived_at: null },
  { id: "agent-other", name: "其他 Agent", owner_id: "user-other", runtime_bound: true, archived_at: null },
  { id: "agent-unbound", name: "未绑定 Agent", owner_id: "user-feng", runtime_bound: false, archived_at: null },
];
```

Assert the selector label and mutation:

```ts
expect(screen.getByRole("option", { name: "前端 Agent · 冯钰杰" })).toBeInTheDocument();
expect(screen.queryByRole("option", { name: /未绑定 Agent/ })).toBeNull();

fireEvent.change(screen.getByLabelText(/Agent fengyujie/), {
  target: { value: "agent-feng" },
});
expect(setMappingMutate).toHaveBeenCalledWith(
  { configId: CONFIG.id, externalKey: "fengyujie", agentId: "agent-feng" },
  expect.anything(),
);
```

Keep the typed `(externalType, externalKey)` owner-reference tests and verify
unresolved filtering covers all referencing tasks.

- [ ] **Step 2: Add the shared scroll assertion**

```ts
const content = screen.getByTestId("pmo-detail-content");
expect(content).toHaveClass("min-h-0", "flex-1", "overflow-y-auto");
expect(screen.getByTestId("pmo-schedule-scroll")).toHaveClass("overflow-x-auto");
```

- [ ] **Step 3: Run the mapping/scroll tests and confirm failure**

Run:

```bash
pnpm --filter @multica/views exec vitest run pmo/pmo-config-detail-page.test.tsx -t 'Agent|scroll|unresolved'
```

Expected: FAIL because the page still selects workspace members and the source
schedule lacks its final horizontal-scroll wrapper.

- [ ] **Step 4: Render eligible Agent options with owner names**

Derive maps without additional server state:

```ts
const memberNameByUserId = useMemo(
  () => new Map(members.map((member) => [member.user_id, member.name])),
  [members],
);
const eligibleAgents = useMemo(
  () => agents.filter((agent) => !agent.archived_at && agent.runtime_bound === true),
  [agents],
);
```

Use `agent.id` as the option value and
`${agent.name} · ${memberNameByUserId.get(agent.owner_id ?? "") ?? "Unknown owner"}`
as its label. Rename accessible labels from `Workspace member` to `Agent` and
send `agentId` to the mutation.

- [ ] **Step 5: Finalize vertical and horizontal overflow**

The page body below `CollectionPageHeader` must be:

```tsx
<div data-testid="pmo-detail-content" className="min-h-0 flex-1 overflow-y-auto">
  <div className="mx-auto w-full max-w-6xl px-4 pb-8 sm:px-6">
    {content}
  </div>
</div>
```

The schedule table wrapper must be:

```tsx
<div data-testid="pmo-schedule-scroll" className="overflow-x-auto">
  <table className="min-w-[860px] w-full">...</table>
</div>
```

- [ ] **Step 6: Run focused tests, typecheck, and lint**

Run:

```bash
pnpm --filter @multica/views exec vitest run pmo/pmo-config-detail-page.test.tsx
pnpm --filter @multica/views typecheck
pnpm --filter @multica/views lint
```

Expected: tests and typecheck PASS; lint has no new errors.

- [ ] **Step 7: Commit the UI completion slice**

```bash
git add packages/views/pmo/pmo-config-detail-page.tsx packages/views/pmo/pmo-config-detail-page.test.tsx packages/views/pmo/pmo-source-preview.tsx
git commit -m "fix(pmo): map owners to agents and restore scrolling"
```

## Task 7: Integrated Verification

**Files:**

- Review all files changed in Tasks 1-6, including migration 890.
- Do not create a dependency update or any additional migration.

- [ ] **Step 1: Run GitNexus change detection**

Run `detect_changes({scope:"compare", base_ref:"main"})` through the configured
GitNexus MCP surface. Expected: only PMO contract, mapping/apply, core API, and
shared PMO view flows are affected. Stop and investigate any unrelated process.

- [ ] **Step 2: Run backend PMO suites**

```bash
cd server && go test ./internal/service ./internal/handler -run 'PMO' -count=1
```

Expected: PASS.

- [ ] **Step 3: Run frontend focused suites**

```bash
pnpm --filter @multica/core exec vitest run api/client.test.ts pmo
pnpm --filter @multica/views exec vitest run pmo
```

Expected: PASS.

- [ ] **Step 4: Run repository static checks**

```bash
pnpm typecheck
pnpm lint
git diff --check
```

Expected: typecheck PASS, lint has no new errors, and diff check is clean.

- [ ] **Step 5: Smoke-test Web and Desktop layouts**

Start the existing development server and inspect the same shared PMO route at
desktop and narrow widths. Verify:

```text
- requirement summary includes title, status, priority, owner, dates, PRD;
- every real task appears once;
- Milestone headings use scheme_name;
- page scrolls vertically;
- schedule scrolls horizontally when narrow;
- Agent mapping options show Agent name and owner member;
- no text or controls overlap.
```

Use browser screenshots and DOM assertions for Web. Use the Desktop renderer or
the existing local Desktop URL to prove the shared view behaves identically.

- [ ] **Step 6: Review and final commit**

Run a focused read-only review for correctness, API compatibility, security of
URL rendering, and missing tests. Fix findings, repeat the smallest affected
checks, then commit remaining plan/spec updates:

```bash
git add docs/superpowers/specs/2026-08-17-pmo-pm-parity-agent-mapping-design.md docs/superpowers/plans/2026-08-17-pmo-pm-parity-agent-mapping.md
git commit -m "docs(pmo): document source parity and agent mapping"
```

Do not push, deploy, or start a production sync as part of this plan.
