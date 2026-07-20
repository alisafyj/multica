# Design System Profile MVP

## Background

Multica's UI Agent draft generation currently proves that an agent can select a template and emit JSON patch operations, but the quality ceiling is too low. In the CRM customer list experiment, the agent understood part of the requirement and changed several text layers, but the final generated design still contained heavy template residue such as procurement navigation, supplier labels, product names, and stale table rows.

The failure is not primarily a user requirement problem. It is a system boundary problem:

- The uploaded template is treated mostly as raw Figma/Gallery JSON.
- The UI Agent is asked to infer semantic page structure from many low-level layers.
- The Agent emits raw patch operations, so it can easily patch one row or one column while missing the rest of the page.
- There is no independent project design language for the Agent to follow.

The next stage should introduce Design System Profile as a first-class asset, independent from business design files and page templates.

## Goal

Build the smallest useful loop for Design System Profile:

1. A user can publish an uploaded design file as a project design system.
2. Multica stores an analyzed design system profile JSON.
3. The design center exposes design systems separately from design files and templates.
4. UI Agent draft tasks can read the project's default design system profile.
5. The implementation keeps a path toward page intent and template blueprint generation without turning the product into a rigid template-filling system.

## Non-goals

This MVP will not build a complete design system platform.

Out of scope for the first slice:

- Full automatic component recognition with high confidence.
- Cross-version design-token diffing.
- Multi-brand theme inheritance.
- Manual token editor.
- Figma plugin upload-type redesign.
- C-end free-form page generation.
- Complete UI DSL / layout engine.
- Replacing all raw patch generation in one step.

The MVP should establish the asset boundary first. Deeper analysis and materialization improvements can then build on it.

## Product Model

Design center should distinguish three asset kinds:

| Asset | Purpose | Lifecycle |
| --- | --- | --- |
| Design File | Concrete business page or generated draft result | Changes with issues and requirements |
| Template | Reusable page or block structure | Changes with page patterns |
| Design System | Project visual language and component rules | Changes slowly with brand/product style |

The mental model:

```text
Design System: how to draw
Template: what structure to reference
Design File: final business page output
```

UI Agent generation should eventually use:

```text
Issue / PRD
+ default Design System Profile
+ optional Template Blueprint / reference design
=> Page Intent
=> materialized Design Draft
```

## User Workflow

### Publish Design System

1. UI designer prepares a Figma page or file containing UI specification material.
2. Designer uploads it through the existing Figma plugin flow as a normal design file.
3. In Multica design detail, the user chooses "Publish as Design System".
4. The user gives it a name, associates it with a project, and can mark it as default.
5. Server creates a design system asset and runs lightweight analysis.
6. Design center shows the asset under the Design System section.

This avoids changing the plugin before the backend/product model is validated.

### Use Design System in UI Agent

1. User creates or opens a UI design issue.
2. User clicks "Let UI Agent generate design draft".
3. Server resolves the project's default design system.
4. The UI Agent task context includes the design system profile JSON and source design file metadata.
5. Agent must use the profile as the visual contract for generated designs.

## UI Design Guidance

The Design System tab should be operational, not a marketing page.

Initial content:

- Project selector follows the existing design center pattern.
- A list of design system assets for the selected project.
- Each item shows name, default status, analysis status, updated time, and source design file.
- Empty state explains that a user can publish a design file as a design system.
- No complex token editor in MVP.

Design detail should expose one action:

- Publish as Design System

Design system detail can initially be simple:

- Source design file link.
- Profile JSON preview.
- Analysis status and errors.
- Set as default.

## Data Model

Add a new table for design system assets.

Suggested shape:

```sql
design_system_profile (
  id uuid primary key,
  workspace_id uuid not null,
  project_id uuid,
  source_file_id uuid not null,
  source_revision_id uuid not null,
  name text not null,
  description text,
  status text not null, -- draft | analyzed | failed | archived
  is_default boolean not null default false,
  profile_json jsonb not null default '{}',
  analysis_errors jsonb not null default '[]',
  created_by uuid,
  created_at timestamptz not null,
  updated_at timestamptz not null
)
```

Constraints:

- Workspace scoping is mandatory.
- At most one default design system per project should be enforced.
- `project_id` may be nullable for workspace-level defaults, but MVP can focus on project-level use.

## Profile JSON

The first profile should be intentionally lightweight and tolerant.

Suggested shape:

```json
{
  "version": "1.0",
  "source": {
    "file_id": "...",
    "revision_id": "..."
  },
  "tokens": {
    "colors": [],
    "typography": [],
    "spacing": [],
    "radius": []
  },
  "components": {
    "button": [],
    "input": [],
    "select": [],
    "tag": [],
    "table": [],
    "modal": [],
    "card": []
  },
  "guidelines": [],
  "confidence": {
    "overall": "low"
  }
}
```

MVP analysis may start with extraction rather than perfect classification:

- Colors from fills and strokes.
- Text styles from text layers.
- Component-like groups by names containing button, input, select, tag, table, modal, card, or their Chinese equivalents.
- Example layer references for each detected component.
- Warnings when no obvious components are found.

The profile must be useful to the Agent even if confidence is low. It should carry examples and rules, not just normalized tokens.

## Server APIs

Add design-system endpoints under the existing design API boundary.

Suggested endpoints:

- `GET /api/design-systems?project_id=...`
- `POST /api/design-systems`
- `GET /api/design-systems/{id}`
- `POST /api/design-systems/{id}/reanalyze`
- `POST /api/design-systems/{id}/set-default`

`POST /api/design-systems` accepts:

```json
{
  "source_file_id": "...",
  "source_revision_id": "...",
  "project_id": "...",
  "name": "CRM 后台设计系统",
  "description": "..."
}
```

The handler should validate:

- Source file exists in the workspace.
- Source revision belongs to the source file.
- Project belongs to the workspace when provided.

## UI Agent Task Context

When creating a UI Agent draft task, server should resolve the project design system:

1. Prefer project default design system.
2. If none exists, optionally use workspace default later.
3. If none exists, continue without design system but mark the task context accordingly.

Task context should include:

```json
{
  "design_system": {
    "id": "...",
    "name": "...",
    "status": "analyzed",
    "profile": {}
  }
}
```

Prompt contract:

- The design system is the visual contract.
- Template candidates are structure references, not the only source of truth.
- The Agent should prefer structured page intent over raw patch thinking.
- If a requirement conflicts with template residue, requirement wins.

## Quality Gate

The MVP should add a simple internal evaluator for generated UI drafts.

First checks:

- Required fields from the issue appear in the draft profile or patch output.
- Known template residue terms are not left untouched when the requirement clearly changes domain.
- Table header count and sample row count are coherent enough for review.
- The Agent output is not accepted if it only selects a template and emits no meaningful design change.

This evaluator is not a user-facing blocker at first. It can fail the Agent task or attach internal warnings depending on confidence.

## Figma Plugin

Do not change the plugin in the first slice unless the existing flow blocks testing.

The first path is:

```text
Figma upload as normal design file
=> Design detail
=> Publish as Design System
```

After validation, the plugin can add an upload type:

- Design File
- Template
- Design System

That should be a later usability improvement, not the architectural first step.

## Migration Strategy

No existing design files or templates need to be migrated.

The new table is additive. Existing UI Agent draft generation should continue to work without a design system. When a project has a default design system, the Agent task context includes it.

## Testing

Backend:

- Create design system from existing design file revision.
- Reject source file/revision outside workspace.
- Set default and ensure previous default is cleared for the project.
- List design systems by project.
- UI Agent draft task context includes default design system when present.

Frontend:

- Design center shows Design System section or tab.
- Empty state renders when no design system exists.
- Publish action from design detail creates a design system.
- Default badge/action appears correctly.

Agent flow:

- Task context contains `design_system`.
- Prompt contains the design-system contract.

## Risks

### Risk: Design system analysis is too shallow

Mitigation: Treat analysis as examples and guidance, not absolute truth. The MVP stores profile JSON and source references even when confidence is low.

### Risk: UI becomes crowded

Mitigation: Add a simple design-system section/tab only. Avoid token editing, visual diffing, or component catalogs in MVP.

### Risk: We drift into a template-management product

Mitigation: Keep design system separate from templates. Templates remain references. The long-term target is Page Intent + UI Primitives, not one rigid template per page type.

### Risk: Agent still outputs poor raw patches

Mitigation: This MVP improves context but does not fully solve materialization. The next design slice should introduce Template Blueprint and Page Intent output.

## Open Decisions

The following should be decided before implementation:

1. Product naming: "设计系统" vs "UI 规范".
2. Whether MVP supports workspace-level default design systems or only project-level defaults.
3. Whether publishing as design system appears in design detail only, or also in the design list more menu.
4. Whether failed analysis blocks setting a design system as default.

## Recommended MVP Decisions

Recommended defaults:

1. Use "设计系统" in product UI.
2. Implement project-level default first.
3. Put "发布为设计系统" in design detail first.
4. Allow setting a low-confidence analyzed profile as default, but do not allow failed profiles as default.

This keeps the first implementation small while establishing the right product architecture.
