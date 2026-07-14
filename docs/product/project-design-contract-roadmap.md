# Project Design Contract Roadmap

Status as of 2026-07-14.

This document is the working outline for the next Multica design-module product line.
It defines how cloud Multica design specifications become the primary project-level design contract that UI Agent and UI Restore Agent can consume.

## Product Goal

All work in this line serves two outcomes:

1. UI Agent creates design drafts that are closer to real project UI and more intelligent than template filling.
2. UI Restore Agent restores designs into code with higher fidelity, moving toward near-100% visual and structural match.

The design contract is not the end product. It is the shared deep resource that makes both outcomes possible.

## Core Principle

Figma UI specification uploads are the primary design-system source.

Cloud `design_system_profile` is the Multica-managed project design contract.

Local repository files, including `DESIGN.md` when present, are read-only auxiliary context.

The priority order is:

```text
cloud design_system_profile > local DESIGN.md > local repository reality
```

Multica must not generate, patch, sync, or overwrite local `DESIGN.md`.

UI Agent and UI Restore Agent are the workers that consume those resources and produce design/code artifacts.

## Deep Resources

These resources should be linked under a project instead of existing as isolated files:

- Local repository resource.
- Cloud `design_system_profile`.
- Source Figma UI specification file and revision.
- Template design files.
- Business design files.
- UI Agent design drafts.
- UI Restore Agent restore artifacts and visual QA results.
- Optional root-level `DESIGN.md` in the local repository as read-only Agent context.

## Target Resource Relationship

```text
project
  -> local repository resource
  -> cloud design_system_profile
       -> source figma design_file / revision
       -> status
  -> UI Agent draft tasks
  -> UI Restore Agent tasks
```

Suggested cloud profile metadata:

```json
{
  "project_id": "...",
  "design_system_profile_id": "...",
  "source_design_file_id": "...",
  "source_revision_id": "...",
  "status": "analyzing | analyzed | failed | archived"
}
```

## Phase 1: Analyze Uploaded Figma UI Specification

Trigger:

- User uploads a Figma UI specification through the Figma plugin.

Worker:

- Local UI Restore Agent.

Server behavior:

- Create or update `design_system_profile`.
- Mark it as `analyzing` while the Agent works.
- Send the Agent candidate layers, tokens, text samples, dimensions, image/asset references, and lightweight hierarchy summaries.

Agent output:

- Agent-readable `profile_json`.
- Analysis warnings.
- Optional notes about local repository conventions when useful, without modifying local files.

Important rule:

- Backend should not maintain a large component-word dictionary.
- Backend only does deterministic hygiene: skip hidden layers, remove obvious draft/backup/noise, validate JSON, version results, and keep failure fallbacks.
- Semantic classification belongs to the Agent.

## Phase 2: Make UI Agent Consume The Cloud Design Profile

Goal:

- UI Agent should design inside the project language, not just edit template JSON.

UI Agent task context should include:

- Parent Issue / PRD.
- Current UI Issue.
- Cloud `design_system_profile`.
- Local `DESIGN.md` as optional read-only context when the target repository already has one.
- Template candidates.
- Similar historical design drafts when available.
- Output policy.

UI Agent responsibilities:

- Understand the requirement.
- Infer page type and interaction states.
- Use the project design contract.
- Choose a template only when it fits.
- Create or modify a design draft.
- Explain which project components/rules it used.

Priority rule:

- If cloud `design_system_profile` conflicts with local `DESIGN.md`, cloud wins.
- If neither cloud nor local design rules cover a detail, the Agent should infer from the local repository and state the assumption.

## Phase 3: Make UI Restore Agent Consume The Same Cloud Profile

Goal:

- UI Restore Agent should restore into the real project system, not create isolated fake UI.

Restore task context should include:

- Restore Pack from the selected design scope.
- Cloud `design_system_profile`.
- Local `DESIGN.md` as optional read-only context when it exists.
- Repository analysis.
- Allowed implementation paths.

UI Restore Agent responsibilities:

- Map design layers to project components and routes.
- Prefer existing project primitives and styling conventions.
- Implement page/state/modal relationships correctly.
- Run visual QA.
- Output restore mapping, used layer IDs, used assets, changed files, screenshots, remaining diffs, and fidelity score.

Priority rule:

- Cloud `design_system_profile` decides the intended design language.
- Local `DESIGN.md` can explain repository-specific implementation conventions, but it cannot override the cloud profile.
- Local repository reality decides feasible code integration paths.

## Phase 4: Build The Evaluation Loop

Goal:

- Improve design generation and restore quality over time.

Data to retain:

- Requirement / PRD.
- `design_system_profile` version.
- Generated design draft.
- Restore result.
- Design screenshot.
- Implementation screenshot.
- Visual difference summary.
- User acceptance or rejection.
- Failure reasons.

This loop turns each UI Agent and UI Restore Agent run into project memory instead of a one-off task.

## Phase 5: Handle Profile Versions And Failures

Goal:

- Support design-system evolution without corrupting project work.

Profile statuses:

- `analyzing`: Local UI Restore Agent is analyzing the uploaded UI specification.
- `analyzed`: profile is available for UI Agent and UI Restore Agent.
- `failed`: profile analysis failed; Agents should fall back to local `DESIGN.md` if present, then repository reality.
- `archived`: profile is no longer active.

Initial UI can stay minimal. The important part is designing the state model early so later product surfaces do not need a rewrite.

## MVP Execution Order

1. Upload UI specification and dispatch Local UI Restore Agent to analyze it.
2. Store Agent-produced `profile_json` as the project cloud `design_system_profile`.
3. Mark one analyzed profile as the project default.
4. Include cloud `design_system_profile` in UI Agent design-draft tasks.
5. Include cloud `design_system_profile` in UI Restore Agent tasks.
6. If the target local repository already has `DESIGN.md`, expose it to Agents as read-only context.
7. Enforce priority: cloud profile > local `DESIGN.md` > repository reality.
8. Record restore/generation outcomes for future evaluation.

## Non-Goals For The First Slice

- Do not build a large manual form for users to tag every UI component.
- Do not maintain a growing backend dictionary of all UI component words.
- Do not generate local `DESIGN.md`.
- Do not patch local `DESIGN.md`.
- Do not sync local `DESIGN.md` with cloud state.
- Do not overwrite local `DESIGN.md`.
- Do not expose complex analysis details to normal users unless they need to resolve a conflict.
- Do not require cloud-side server LLM calls for the first slice; use the existing Agent/daemon task model.

## First Implementation Bias

Use the existing Multica agent-task architecture:

```text
server creates agent_task_queue
daemon claims task
Local UI Restore Agent runs semantic analysis
Agent returns strict JSON
server validates and stores result
```

The first dedicated task type can be:

```text
design_system_profile_analyze
```

This keeps the AI work inside the existing Local Agent model and avoids introducing server-side LLM infrastructure prematurely.
