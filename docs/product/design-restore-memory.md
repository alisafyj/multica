# Design Restore Memory

> Persistent working memory for Multica's design import, Native Design Viewer, and Agent restore product work. Keep this file current whenever goals, status, blockers, or next steps change.

## Purpose

Multica should let a team attach a real Figma design to an issue, inspect it as native-ish layers, create scoped restore tasks, and let a local Agent implement the design into the bound target repository with traceable output.

The product direction is **Issue-centered design restore**:

1. Figma plugin imports real frame/layer/asset data into Multica.
2. Multica shows a Native Design Viewer for review, inspection, and light edits.
3. The user creates restore work from a design frame or layer selection.
4. A local Agent writes code into the target repo like a frontend engineer.
5. Multica records what was generated and links the restore result back to the issue and design revision.

## Current Baseline

- Branch: `feature/fengchen`
- Multica repo: `/Users/fengyujie/Documents/soyoung/multica`
- Target validation repo: `/Users/fengyujie/Documents/soyoung/gallery-test`
- Backend: `http://localhost:8080`
- Frontend: `http://localhost:3031`
- DB container: `multica-postgres-dev`
- DB URL: `postgres://multica:multica@localhost:5432/multica?sslmode=disable`

### Latest Clean Design Import

- Workspace slug: `amc`
- Workspace id: `e2f576ee-5a61-4844-8dee-719996169571`
- Design file id: `82c1e643-3530-443a-a531-3cb275b0ba1e`
- Current revision id: `a11a7ffb-ac0f-4aec-ab95-036dee9303e5`
- Revision number: `4`
- Status: `valid`
- Frames: `4`
- Layers: `1020`
- Assets: `223`
- Empty URL assets: `0`
- Placeholder assets: `0`
- HTTP assets: `223`
- Fallback layer refs: `82`
- Fallback assets: `82`
- Image layer refs: `28`
- Image assets: `20`

Frames:

- `frame-1` — `个人主页单排 -官号`
- `frame-0-423` — `扫码支付`
- `frame-0-468` — `服务记录+治疗师2位`
- `frame-0-651` — `发布`

### Latest Restore Run

- Issue: AMC-20 `UI设计`
- Issue id: `f1d40329-7e37-4280-a68a-309eee2fdee9`
- Frame restored: `frame-0-468` / `服务记录+治疗师2位`
- Restore task id: `a34e89fa-9eea-4560-b366-58825541c5fd`
- Agent task id: `b479e83f-1468-4b6e-8d69-b376ab9ffffa`
- Agent: `Local UI Restore Agent`
- Agent id: `6ef23397-12b3-4857-adca-a76afbff8b40`
- Runtime id: `4f381116-786f-486f-ab92-848631808c82`
- Target route in gallery-test: `http://localhost:5173/design-restore/a34e89fa9eea`

Target repo output already committed in `gallery-test`:

- `3d4e07e feat: add design restore page`

Main files in target repo:

- `src/views/design-restore/Restorea34e89fa9eeaView.vue`
- `src/components/design-restore/restore-a34e89fa9eea/*.vue`
- `src/router/index.ts`
- `src/views/HomeView.vue`

## Completed Product/Engineering Work

### Figma Plugin Import Stability

Commits:

- `2bc3b56b fix: support figma plugin runtime syntax`
- `65b954ed fix: stream figma plugin asset uploads`
- `3d79272a fix: drop unuploaded figma asset refs`

Current state:

- Plugin avoids unsupported runtime syntax.
- Asset upload streams one asset at a time with ack/backpressure.
- Native JSON no longer embeds raw byte arrays.
- Unuploaded asset references are removed before final import.
- Latest revision has no empty/placeholder asset URLs.

### Native Design Viewer

Commits:

- `0a694270 feat: add native design viewer`
- `c48df22c feat: add layer fallback assets`
- `5d0856ba feat: enhance native viewer fidelity`
- `c1e0503c feat: improve native design render fidelity`
- `c27efc78 feat: add lightweight stroke editing`
- `3f7b3ffa feat: add stroke controls to native viewer`
- `0daeceda fix: validate stroke width edits`
- `9a24ad23 feat: add lightweight edit undo`
- `43abcfff feat: add lightweight image replacement`
- `6aef34d9 feat: show design import quality summary`
- `658d166b fix: memoize design frame quality reports`
- `4d798f7b feat: polish native design viewer`
- `9859c686 fix: replace translucent design overlay`

Current state:

- Native Viewer renders text, image, shape, vector/fallback, and local slice/crop assets.
- Uploaded image fills are treated as native-renderable.
- Uploaded shape fallback assets are treated as high-quality local fallback.
- Transparent utility shapes no longer penalize renderability.
- User-facing fidelity percentages and render quality panels are hidden.
- Layer tree is a floating panel, can be collapsed, and width follows expanded tree content.
- Overlay comparison no longer uses translucent stacking; it uses slider-based reveal comparison to avoid double-image ghosting.

### Restore Task Revision Safety

Commit:

- `c781e0f9 fix: refresh design restore tasks by revision`

Current state:

- Design restore task reuse is scoped by `file_id`, `revision_id`, and active task status.
- UI ignores stale restore tasks once current revision is known.
- Mapping parser accepts Agent schema fields such as `sketchId` and `targetFile`.

## Key Design Decisions

1. **Native Viewer is not Online Figma.** Scope is real layer view, inspect, fidelity/fallback awareness, selection context, and light edits. It is not a full vector editor.
2. **Do not use full-frame preview/thumbnail as the primary restore result.** Frame preview is acceptable for debug/overlay/fallback but not as final app code content.
3. **Fidelity metrics are internal debug signals.** They should not be shown as user-facing product claims unless intentionally surfaced in a developer/debug mode.
4. **Fallback can be visually high quality without being fully native.** Local SVG/PNG/crop fallback should score close to native for internal renderability, while still being distinguishable from editable native layers.
5. **Restore output should behave like frontend engineering work.** Agent should create/reuse routes/pages/components, avoid dead artifact files, avoid single-file dumps, and report file mappings.
6. **Revision identity matters.** Restore tasks and mappings must remain tied to the design revision they were created from.

## Current Known Limitations

### Native Viewer / Design Import

- Some content still relies on fallback assets, slices, or local cropped previews rather than fully editable native layer primitives.
- Remaining 1% in internal fidelity is mostly fallback semantics, not missing visuals.
- Layer tree state is not persisted across sessions.
- Overlay comparison is now less noisy, but does not yet provide heatmap/diff visualization.

### Agent Restore

- Target repo output can still depend on `http://localhost:8080/uploads/...` asset URLs.
- No production CDN/direct-upload path is complete yet.
- Agent generated code quality is not automatically scored against the original design.
- Restore result mapping exists, but product UX for mapping review is still basic.
- Automation still uses title heuristics such as `UI/设计` and `前端/frontend` in places.

### Tests / Environment

- Full `go test ./...` has known unrelated failures in `server/pkg/agent` Codex timeout tests.
- Full `go test ./internal/handler` may hit local fixture issues; prefer focused design/restore tests.
- Full `go test ./cmd/server` has known readiness/env issue around invalid `DATABASE_MAX_CONNS`; `go test ./cmd/server -run '^$'` is acceptable for compile check.
- `sqlc` is not installed locally; use `go run github.com/sqlc-dev/sqlc/cmd/sqlc@v1.31.1 generate`.

## Product TODOs

### P0 — Make the Work Recoverable

- Keep this document current after every meaningful design-restore session.
- Add a short link to this document from a more discoverable project doc if needed.
- When a todo is promoted, update this file instead of relying only on chat memory.

### P1 — Restore Quality Closure

- Add automatic post-restore summary in Multica: generated files, route, components, validation status, warnings.
- Strengthen restore mapping display for each design task item.
- Make Agent output prefer reusable project components when available.
- Prevent deploy-hostile localhost asset URLs by copying assets into the target repo or using stable CDN URLs.

### P1 — Design Import Productionization

- Implement CDN/direct-upload flow for design assets.
- Decide how to handle historical base64/old revision data: migrate, reupload, or mark as legacy.
- Persist import quality diagnostics for developer/admin inspection, not normal user UI.

### P2 — Design Understanding

- Add async Design Understanding pipeline: frame role, page type, section/module recognition.
- Add Template Understanding: identify reusable target-app component patterns before restore.
- Generate task queue suggestions from semantic design regions, not only manual selections.

### P2 — Workflow/Product Hardening

- Replace title heuristics with explicit issue type, labels, or workflow state.
- Show current design revision on restore task UI and warn when revision changes.
- Support explicit re-run from latest revision.

### P3 — Native Viewer UX Polish

- Persist layer panel collapsed/expanded state.
- Add layer panel drag positioning if users need it.
- Add search hit auto-expand.
- Consider diff heatmap or mouse-follow reveal for overlay mode.

## Important Files

Multica:

- `apps/figma-plugin/code.js` — Figma export/runtime logic.
- `apps/figma-plugin/ui.html` — plugin upload UI and asset-upload ack flow.
- `server/internal/handler/design_plugin.go` — plugin import and asset upload endpoints.
- `server/internal/handler/design_fidelity.go` — backend persisted import fidelity report.
- `server/internal/handler/design_file.go` — design file and restore handlers.
- `server/internal/handler/daemon.go` — Agent completion/mapping parsing/policy warnings.
- `packages/views/designs/design-file-page.tsx` — design board page.
- `packages/views/designs/design-frame-page.tsx` — frame detail/native viewer page.
- `packages/views/designs/layer-tree.tsx` — floating layer tree.
- `packages/views/designs/overlay-comparison.ts` — slider reveal overlay helper.
- `packages/views/designs/native-renderer/fidelity.ts` — internal renderability/fidelity scoring.
- `packages/views/designs/native-renderer/` — native frame renderer.
- `packages/views/issues/components/issue-design-restore-section.tsx` — Issue-side design restore card.

Gallery test target:

- `/Users/fengyujie/Documents/soyoung/gallery-test/src/views/design-restore/Restorea34e89fa9eeaView.vue`
- `/Users/fengyujie/Documents/soyoung/gallery-test/src/components/design-restore/restore-a34e89fa9eea/`
- `/Users/fengyujie/Documents/soyoung/gallery-test/src/router/index.ts`

## Useful Commands

Frontend verification:

```bash
pnpm --filter @multica/views exec vitest run designs/overlay-comparison.test.ts designs/native-renderer/fidelity.test.ts issues/components/issue-design-restore-section.test.ts
pnpm --filter @multica/views exec tsc --noEmit --pretty false
git diff --check
npx gitnexus detect-changes
```

Restart frontend on port 3031:

```bash
for pid in $(lsof -ti tcp:3031 || true); do kill "$pid" || true; done
set -a && source .env && set +a
FRONTEND_PORT=3031 NPM_CONFIG_REGISTRY=https://registry.npmjs.org PNPM_CONFIG_REGISTRY=https://registry.npmjs.org nohup pnpm --filter @multica/web dev > "/var/folders/q0/vgjdbrm579942n43js1pr7_m0000gn/T/opencode/multica-frontend.log" 2>&1 &
```

Restart backend on port 8080:

```bash
cd /Users/fengyujie/Documents/soyoung/multica/server
for pid in $(lsof -ti tcp:8080 || true); do kill "$pid" || true; done
set -a && source ../.env && set +a
nohup go run ./cmd/server > "/var/folders/q0/vgjdbrm579942n43js1pr7_m0000gn/T/opencode/multica-backend.log" 2>&1 &
```

Run local daemon:

```bash
cd /Users/fengyujie/Documents/soyoung/multica/server
go build -o bin/multica ./cmd/multica
./bin/multica daemon start
```

## Next Session Startup Checklist

1. Read this file first.
2. Check `git status --short` in Multica and `gallery-test`.
3. Confirm frontend/backend ports if browser validation is needed.
4. For code edits, run GitNexus impact before touching existing symbols.
5. Keep design-restore TODOs here updated before ending the session.
