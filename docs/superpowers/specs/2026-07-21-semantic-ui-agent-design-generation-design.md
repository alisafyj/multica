# Semantic UI Agent Design Generation

Status: Approved design

Date: 2026-07-21

## Summary

Multica will replace text-only mutation of arbitrary Figma layer JSON as the
primary UI Agent generation path with a semantic, compiler-driven pipeline.

The UI Agent will decide what the page contains and express that decision as a
validated `PageSpec`. Multica will combine the `PageSpec` with a semantic
template `Blueprint` and executable project `ComponentRecipe` assets, then use
a deterministic `DesignCompiler` to produce Native Design JSON.

The first release is intentionally limited to structured B-end list pages:
filters, page actions, tables, status tags, row actions, and pagination.

## Why The Existing Approach Fails

The current UI draft path asks the Agent to choose a raw design template and
return `slot_values` plus a safe JSON patch. When a template has no explicit
slot schema, the Agent receives a bounded list of editable text layers and may
only replace text. Layout and tree paths such as `x`, `y`, `width`, `height`,
and `children` are forbidden.

This action space cannot express normal design work:

- add or remove filters and table columns;
- choose and instantiate component variants;
- reflow rows and columns after content changes;
- resize cells for longer content;
- replace ordinary text with a status-tag component;
- remove unrelated template business data;
- change page structure when the requirement does not fit the template.

The CRM customer-list test demonstrated the expected failure mode. The Agent
understood much of the requirement, but the materialized design retained
purchase-related content and layer metadata, left controls unchanged, and
placed longer customer data into old fixed column widths, causing visible text
overlap.

This is an architecture limitation, not primarily a prompt or model-quality
problem.

## Goals

1. Generate reviewable B-end list-page designs from parent Issue PRDs.
2. Keep business decisions in an Agent-readable semantic representation.
3. Apply the project's uploaded UI specification as executable design assets.
4. Use uploaded templates as reusable composition structures, not business-copy
   containers.
5. Produce deterministic, valid Native Design JSON without asking the Agent to
   manipulate layer IDs or geometry.
6. Block structurally broken drafts instead of reporting false success.
7. Retain generation inputs, versions, validation results, and review feedback
   so project-specific design quality can improve over time.

## Non-Goals

The first release will not support:

- free-form C-end composition;
- detail pages or dashboard composition;
- large forms;
- modal, drawer, popover, or multi-frame interaction design;
- arbitrary Figma editing by the Agent;
- model fine-tuning;
- a manual form for designers to label every uploaded layer;
- automatic mutation of local `DESIGN.md`;
- a fallback from semantic generation to arbitrary text-layer patching.

These are future extensions of the same semantic model, not requirements for
proving the first vertical slice.

## Product Flow

```text
Parent Issue / PRD
        |
        v
UI Agent requirement understanding
        |
        v
PageSpec
        |
        +---- Template Blueprint
        |
        +---- design_system_profile + Component Recipes
        |
        v
Design Compiler
        |
        v
Native Design JSON
        |
        v
Automatic quality gate
        |
        v
Reviewable design draft
```

Responsibilities are deliberately separated:

- The UI Agent decides the business information architecture.
- `PageSpec` records those design decisions without geometry.
- `TemplateBlueprint` provides page composition and layout constraints.
- `design_system_profile` provides descriptive project rules and tokens.
- `ComponentRecipe` makes project components executable by the compiler.
- `DesignCompiler` creates, removes, clones, sizes, and places design nodes.
- The quality gate decides whether a generated draft is reviewable.

## Source Precedence

The generation path uses the following ownership model:

1. The Issue PRD owns business content and required behavior.
2. The project UI specification owns visual language and component variants.
3. The selected Blueprint owns composition and layout constraints.
4. Similar approved project PageSpecs provide examples, not overrides.
5. Local repository context remains optional, read-only supporting context.

When a Blueprint conflicts with the PRD, the Agent must select another
Blueprint or report that no supported structure exists. It must not preserve
irrelevant template content to make the result fit.

## Template Blueprint

### Purpose

A Blueprint is a semantic, versioned representation of an uploaded Figma
template. The UI Agent sees the Blueprint summary, not hundreds of raw template
layers.

### Analysis Pipeline

```text
Uploaded Figma template
  -> deterministic structural extraction
  -> Agent semantic classification
  -> Blueprint validation
  -> versioned TemplateBlueprint
```

Deterministic extraction reads only objective source facts:

- visible hierarchy;
- dimensions and positions;
- Auto Layout metadata;
- component instances;
- repeated rows, columns, and controls;
- text and asset references;
- clipping and parent-child relationships.

The Agent classifies semantic regions and prototypes:

- shell and navigation;
- breadcrumb;
- filter region;
- page-action region;
- table header and row prototypes;
- status-tag location;
- row-action region;
- pagination.

### Contract

```json
{
  "version": "1.0",
  "pageType": "list",
  "regions": {
    "shell": {},
    "breadcrumb": {},
    "filters": {},
    "pageActions": {},
    "table": {},
    "pagination": {}
  },
  "prototypes": {
    "filterField": {},
    "primaryButton": {},
    "tableHeaderCell": {},
    "tableRow": {},
    "statusTag": {},
    "rowAction": {}
  },
  "constraints": {
    "contentWidth": 1690,
    "filterColumns": 3,
    "tableHeaderHeight": 50,
    "tableRowHeight": 60,
    "horizontalGap": 16
  },
  "sourceRefs": {
    "designFileId": "uuid",
    "revisionId": "uuid",
    "layerIds": {}
  }
}
```

The Blueprint stores structural source references and cloneable prototypes. It
does not promote sample supplier, purchase, product, or customer copy into
generation inputs.

Blueprints are regenerated and versioned when the source template revision
changes. A Blueprint that cannot be validated as a supported list-page
structure is not offered to UI Agent tasks.

## Executable UI Specification

### Descriptive And Executable Assets

The existing `design_system_profile` remains the descriptive project contract:
tokens, component semantics, guidelines, and anti-rules.

The compiler additionally requires `ComponentRecipe` records. A Recipe binds a
semantic component, variant, and state to a cloneable source subtree and a
small set of editable properties.

```json
{
  "kind": "input",
  "variant": "default",
  "source": {
    "revisionId": "uuid",
    "rootLayerId": "input-default"
  },
  "props": {
    "label": {
      "targetLayerId": "label-text",
      "type": "text"
    },
    "placeholder": {
      "targetLayerId": "placeholder-text",
      "type": "text"
    }
  },
  "layout": {
    "widthMode": "fill",
    "height": 36,
    "minWidth": 180
  }
}
```

The first release must support Recipes for:

- input;
- select;
- date range;
- primary button;
- secondary button;
- text button;
- table header;
- table row;
- status tag;
- pagination.

Recipes are inferred from uploaded UI specification names using the existing
`component - variant - state` convention and source hierarchy. Designers do
not fill out a per-layer form.

When a requested Recipe is missing, the compiler applies this internal policy:

1. use an available default variant of the same component kind;
2. use a token-built primitive only when that component kind explicitly allows
   primitive fallback;
3. record a warning identifying the fallback;
4. fail compilation when no structurally safe fallback exists.

Full source subtrees and their asset references are preserved so exported
images and component internals are cloned rather than redrawn heuristically.

## PageSpec

### Purpose

`PageSpec` is the only UI Agent generation output consumed by the compiler. It
contains semantic design decisions and never contains Figma layer IDs, pixel
coordinates, or raw JSON patches.

### List Page Contract

```json
{
  "version": "1.0",
  "page": {
    "type": "list",
    "module": "Customer Management",
    "title": "Customer Records",
    "breadcrumb": ["Customer Management", "Customer Records"],
    "activeNavigation": "Customer Information",
    "density": "standard"
  },
  "filters": [
    {
      "key": "keyword",
      "label": "Customer keyword",
      "control": "input",
      "placeholder": "Enter customer name or phone number",
      "width": "medium"
    }
  ],
  "pageActions": [
    {
      "key": "create",
      "label": "Create customer",
      "variant": "primary"
    }
  ],
  "table": {
    "columns": [
      {
        "key": "customerNo",
        "title": "Customer number",
        "cell": "text",
        "width": "medium"
      },
      {
        "key": "status",
        "title": "Status",
        "cell": "status-tag",
        "statusMap": {
          "Active": "success",
          "Follow up": "warning",
          "Lost": "disabled"
        }
      }
    ],
    "sampleRows": [],
    "rowActions": [
      {"key": "edit", "label": "Edit"},
      {"key": "view", "label": "View"}
    ]
  },
  "pagination": {
    "enabled": true,
    "pageSize": 20,
    "sampleTotal": 126
  },
  "assumptions": [],
  "warnings": [],
  "requirementCoverage": []
}
```

Localized business copy remains in the language supplied by the PRD. The
English example above avoids making the schema itself language-dependent.

### Agent Decisions

The Agent may decide:

- module, title, breadcrumb, and information priority;
- filters and their semantic control types;
- page actions and row actions;
- table columns, order, data type, alignment, and semantic width hints;
- status-value mappings to project status variants;
- representative synthetic sample data;
- whether an available Blueprint fits the requirement.

The Agent may not decide:

- layer IDs or parent-child mutation steps;
- pixel coordinates or exact column widths;
- component-internal colors, spacing, or radius;
- arbitrary visual variants absent from the project UI specification.

When important PRD information is missing, the Agent records assumptions and
warnings instead of silently inventing business rules. PRD-provided examples
take precedence. Generated sample rows must be synthetic, must avoid real
personal information, and must populate every visible column.

`requirementCoverage` maps requirement items to PageSpec keys so validation can
detect omissions such as a required phone-number field missing from the table.

## Design Compiler

The compiler is deterministic for a fixed PageSpec, Blueprint, Recipe set, and
compiler version.

### Pass 1: Semantic Validation

Validate:

- supported page type;
- unique keys;
- supported control and cell kinds;
- Recipe availability;
- row and column consistency;
- complete status mappings;
- required Blueprint regions;
- requirement coverage.

Critical errors stop compilation.

### Pass 2: Component Resolution

Resolve every PageSpec control and cell to a concrete Component Recipe. Clone
the complete source subtree, generate fresh IDs, rewrite internal references,
and bind only declared Recipe properties.

### Pass 3: Region Layout

Use Blueprint constraints to place page regions. For the first list-page
compiler:

- filters use the Blueprint grid and wrap to additional rows;
- removed filters close their gaps;
- search and clear actions remain at the filter-region tail;
- page actions use the Blueprint action regions;
- table and pagination move vertically when filter rows change.

### Pass 4: Table Layout

Each semantic data type has minimum, preferred, and maximum width constraints.
The compiler measures header and sample content using the selected typography,
adds cell padding, reserves fixed widths for status and action columns, then
allocates the remaining width to flexible text columns.

If the sum of minimum widths exceeds the content viewport, the compiler uses a
horizontal-scroll table strategy. It must not shrink cells until text overlaps.
The first and action columns may be pinned when the Blueprint supports it.

Long content follows the resolved Recipe policy: ellipsis or wrapping. It may
not paint outside the cell bounds.

### Pass 5: Row Materialization

Create header cells and row cells from prototypes, apply alternating row
styles, instantiate real status tags, and place row actions from Recipes. Row
and column counts derive entirely from PageSpec.

### Pass 6: Native JSON Serialization

The compiler creates and removes layers, rebuilds parent-child relationships,
updates geometry and Auto Layout metadata, preserves asset references, and
records source provenance:

- PageSpec version;
- Blueprint and source template revisions;
- UI specification and Recipe versions;
- compiler version;
- originating Issue and task.

## Quality Gate

Compilation is not successful merely because valid JSON was produced.

The quality gate checks:

- text overflow;
- unexpected bounding-box overlap;
- off-frame elements;
- duplicate or dangling node IDs;
- unresolved Recipes or variants;
- missing PageSpec fields;
- filter, header, row, and cell count consistency;
- template business-copy residue outside an explicit shell allowlist;
- pagination placement;
- component and token conformance.

Draft status follows these outcomes:

- `generated`: all required checks pass;
- `generated_with_warnings`: structurally safe output with explicit non-critical
  fallbacks;
- `compile_failed`: missing content, broken structure, overlap, overflow, or
  another critical violation.

`compile_failed` output is retained for diagnostics but is not shown as a
normal reviewable draft.

The semantic generation path must not fall back to arbitrary text-layer
patching. Historical patch-based drafts remain readable, but new tasks do not
use that path as compatibility behavior.

## Review And Revision

The first review surface provides three actions:

- approve and publish;
- request revision with a natural-language note;
- reject.

A revision note is converted into a new PageSpec version. The Agent does not
patch the compiled Native JSON.

```text
Review note
  -> PageSpec v2
  -> semantic diff from v1
  -> compiler
  -> quality gate
  -> draft version 2
```

Every draft version retains:

- parent and UI Issue identities;
- PRD input;
- design-system profile and Recipe versions;
- Blueprint version;
- PageSpec;
- compiler version;
- Native Design JSON;
- validation results;
- review feedback;
- previous-version identity.

Approval materializes the draft into a formal `design_file` and
`design_revision`, links it to the UI Issue, and makes it available to the UI
restore and MCP delivery paths. Rejected drafts never become formal design
files.

## Project Learning Loop

The first release improves through project memory and retrieval, not model
fine-tuning.

Retain:

- approved and rejected PageSpecs;
- rejection and revision reasons;
- common filter and column combinations;
- status-to-variant mappings;
- Blueprint selection outcomes;
- validation failures and warnings;
- review iteration count.

Future UI Agent tasks may retrieve similar approved PageSpecs from the same
project as examples. They never override the current PRD or design system.

Primary product metrics are:

- requirement coverage;
- first-pass compilation rate;
- overlap and overflow counts;
- template-residue count;
- missing-component count;
- first-draft approval rate;
- revision count;
- time from UI task start to design publication.

The success criterion is not that the Agent returned JSON. It is that a user
can turn the generated draft into a deliverable design with little or no
revision.

## MVP Acceptance Criteria

The first semantic generation slice is complete when:

1. An uploaded list-page template produces a validated Blueprint without a
   manual per-layer form.
2. An uploaded project UI specification produces executable Recipes for all
   required first-release components.
3. A UI Agent task returns a schema-valid PageSpec and never returns layer
   patches.
4. The compiler can add and remove filters, columns, and rows from the selected
   Blueprint.
5. The compiler instantiates project status-tag variants rather than coloring
   ordinary text.
6. Long content does not overlap adjacent cells.
7. Unsupported width is handled with the horizontal-scroll strategy.
8. Template business copy is absent from the generated business regions.
9. Critical quality failures do not appear as normal reviewable drafts.
10. Review feedback creates a new PageSpec and draft version.
11. Approval materializes a formal design file and revision linked to the UI
    Issue.
12. The CRM customer-records requirement used in discovery passes the quality
    gate with zero purchase-template residue and zero overlap.

## Follow-Up Scope

After the list-page compiler is stable, extend the same semantic foundation in
this order:

1. list-page modal and drawer states;
2. B-end detail and description pages;
3. structured form pages;
4. dashboards and mixed business modules;
5. C-end composition primitives;
6. optional direct Figma operation for human-guided refinement.

Each extension adds semantic components and compiler capabilities. It must not
reintroduce arbitrary layer patching as the design model.
