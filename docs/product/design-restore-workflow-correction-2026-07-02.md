# Design Restore Workflow Correction — UI Restore First

Date: 2026-07-02

This note corrects the product model for Multica design restore before the next implementation phase.
It supersedes the earlier assumption that a UI design Issue should hand a raw Figma design directly to a frontend development Issue.

## Background

The intended user story starts with a parent Issue and two child Issues:

- Parent Issue: `服务记录开发`
- Child Issue A: `UI设计`
- Child Issue B: `前端开发`

Each Issue is associated with:

- a project
- an assignee
- an optional parent Issue

When the `UI设计` Issue is assigned to user A, A enters the UI design phase. A is the owner of that phase.

## Current Deviation

The current Design Delivery MVP drifted toward this workflow:

```text
UI设计 Issue uploads a Figma design
-> UI设计 Issue delivers the raw design to 前端开发 Issue
-> Frontend engineer / Frontend Agent restores the design
```

That is not the desired mainline.

The deviations are:

1. **Delivery happens too early.** A raw uploaded design is treated as ready for frontend consumption before the UI phase is complete.
2. **Restore ownership is assigned to the wrong phase.** Design restore is currently easy to start from the frontend Issue, but the desired model treats visual/page restore as UI work.
3. **`design_delivery` is too raw-design oriented.** It currently behaves like a handoff of fixed design file/revision/scope. The desired frontend handoff should normally carry a UI restore artifact.
4. **UI Issue completion is guarded by the wrong invariant.** A UI Issue should be complete after UI restore has been completed or after an explicit internal fallback path is selected, not merely after raw design delivery.
5. **The UI exposes internal role mechanics.** Buttons such as `标记 UI` and `标记前端` expose metadata concepts that should become implementation details wherever possible.

## Corrected Product Model

The mainline workflow is:

```text
UI设计 Issue
-> A uploads / associates Figma design source
-> A starts design restore
   -> A can assign the restore to a UI Agent
   -> or A can restore it manually through MCP
-> UI restore artifact is produced
-> UI设计 Issue can be completed
-> UI restore artifact is handed off to 前端开发 Issue
-> Frontend engineer / Frontend Agent handles dynamic implementation and integration
```

The responsibility split is:

```text
UI engineer / UI Agent:
  owns "what the page looks like"

Frontend engineer / Frontend Agent:
  owns "what the page does"
```

More concretely:

| Area | Owner |
| --- | --- |
| Figma import and design source selection | UI phase |
| Static page structure | UI phase |
| Visual layout, spacing, typography, colors | UI phase |
| HTML / TSX structure needed to express the design | UI phase |
| CSS / Tailwind / responsive states | UI phase |
| Mock data and visual states | UI phase |
| API requests and data binding | Frontend phase |
| React Query / mutations / realtime wiring | Frontend phase |
| Routing params, permissions, forms, validation | Frontend phase |
| Replacing mock data with real data | Frontend phase |
| Backend integration and joint debugging | Frontend phase |

The product shorthand is:

```text
UI Agent: page-seen work
Frontend Agent: page-used work
```

## Domain Concepts

Use these concepts for the next implementation phase:

- **Design Source**: the uploaded Figma/native design file, revision, frame, layer selection, and assets.
- **UI Restore Task**: work owned by the UI design Issue to convert a Design Source into a usable UI artifact.
- **UI Restore Artifact**: the result of UI restore, such as static components, route/page skeleton, styles, mock data, asset mapping, and implementation notes.
- **Frontend Handoff**: the act of making the UI Restore Artifact available to the frontend development Issue.
- **Raw Design Fallback**: an internal compatibility path where the frontend Issue consumes the raw Design Source directly.

The old `design_delivery` concept should move toward **Frontend Handoff** semantics.
It should normally carry a UI Restore Artifact, not just a raw design revision.

## Desired State Flow

### UI Issue

```text
No design source
-> Design source uploaded / associated
-> UI restore ready
-> UI restore running
-> UI restore completed
-> Handoff ready
-> Handoff sent to frontend
-> UI Issue done
```

Failed or cancelled UI restore should return to a recoverable state:

```text
UI restore failed
-> retry UI restore
-> or use internal raw-design fallback
```

### Frontend Issue

```text
Waiting for UI output
-> UI artifact received
-> Frontend implementation ready
-> Frontend implementation running
-> Frontend done
```

In fallback mode:

```text
Waiting for UI output
-> Raw design source received by internal fallback
-> Frontend performs full visual restore and integration
-> Frontend done
```

## Degradation Strategy

The degradation strategy is required but must stay out of the user-facing workflow.

Normal strategy:

```text
UI owns design restore.
Frontend consumes UI restore artifact.
```

Fallback strategy:

```text
UI only owns design source.
Frontend owns both visual restore and dynamic implementation.
```

The fallback exists for:

- early product rollout while UI Agent / MCP restore is incomplete
- no available UI Agent
- UI restore failure with an operator-approved fallback
- projects that temporarily want frontend to own the full implementation
- compatibility with the current Design Delivery MVP behavior

The fallback must be represented in code and audit data, not as additional primary UI choices.

Suggested internal policy shape:

```ts
type DesignRestoreOwnershipPolicy =
  | "ui_restore_first"
  | "frontend_full_restore_fallback";
```

Suggested handoff source shape:

```ts
type FrontendHandoffSource =
  | { source_type: "ui_restore_artifact"; artifact_id: string }
  | { source_type: "raw_design_revision"; file_id: string; revision_id: string; scope: unknown };
```

Product pages should not show a policy selector by default. They should keep the user's mental model simple:

```text
UI设计: upload design and complete design work.
前端开发: start frontend development when UI work is ready.
```

## Implementation Implications

The next code phase should not simply rename `交付给前端`.
It should adjust the underlying lifecycle.

Recommended direction:

1. Treat the current raw-design `design_delivery` flow as fallback-compatible legacy behavior.
2. Introduce or formalize a UI restore artifact boundary before frontend handoff.
3. Make UI Issue completion depend on either:
   - completed UI restore artifact, or
   - internal fallback handoff using raw design source.
4. Make frontend Issue prefer UI restore artifact when present.
5. Keep raw design restore available to frontend only as an internal fallback.
6. Hide role/policy complexity from the normal Issue UI.

## UX Principles

- The user should not need to understand `metadata.design_role`.
- The user should not see a visible "fallback strategy" selector in the normal flow.
- UI designers should see actions framed around design completion and UI restore.
- Frontend engineers should see actions framed around implementation and integration.
- If there is only one obvious next action, show that action instead of exposing internal branches.

## Acceptance Criteria For The Next Phase

- A `UI设计` Issue can start restore work without first handing the raw design to `前端开发`.
- A UI Agent restore task is owned by the UI Issue.
- A frontend Issue receives a UI restore artifact as the preferred input.
- Raw design direct-to-frontend remains possible as an internal fallback.
- UI Issue completion is guarded by the corrected invariant.
- Existing delivery data can still be read and interpreted as fallback-style handoffs.
- The main UI does not add extra user-facing strategy choices.

## Open Decisions

These should be resolved before database/API changes:

1. Whether to extend `design_delivery` into a generic handoff table or add a new `ui_restore_artifact` / `frontend_handoff` table.
2. What minimum artifact shape is required for UI restore completion.
3. Whether manual MCP restore writes the same artifact record as UI Agent restore.
4. Which project/workspace setting controls the hidden fallback policy, if any.
