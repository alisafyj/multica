# Recovery Extraction: Design Center

Date: 2026-08-11
Current mainline: `feature/fengchen-design`
Recovery source: `codex/feature-fengchen-dirty-recovery-20260810`
Common base: `bfd95597ce594717afef1235a38fb7a5c5f5d8a8`

## Ground Rules

- Keep `feature/fengchen-design` as the authority.
- Use the recovery branch as a source archive only; do not merge it wholesale.
- Do not restore Open Design Worker/Runtime/Daemon execution paths.
- Do not replace native `PackagePreview` / `/package-preview` with recovery `ArchivePreview` / `/open-design-preview`.
- Move one small slice at a time, with GitNexus impact before symbol edits and `detect-changes` before commits.

## Already Restored Or Covered

- Design Center workspace tabs:
  - fixed non-closable `首页`;
  - closable project tabs;
  - compact project content tabs: `设计稿 / 设计草稿 / 模版 / 设计体系`.
- `packages/views/designs/project-design-system-create.tsx` and its test are identical between current and recovery.
- `packages/views/designs/project-design-system-workspace.tsx` and its test are identical between current and recovery.
- Current mainline keeps native project design-system package schema, preview validation, discard draft, and verification client contracts.

## Do Not Port From Recovery

- `ProjectDesignSystemArchivePreview` types/schemas/client methods.
- `designKeys.projectDesignSystemArchivePreview`.
- `/api/project-design-systems/{id}/open-design-preview` as the primary frontend contract.
- Recovery changes that delete current native files:
  - `project_design_system_package_preview.go`;
  - `project_design_system_package_upload.go`;
  - V2 completion/repository-analysis tests and handlers.
- Recovery docs that resume direct Open Design Worker/Runtime/Daemon integration.

## Ported In Current Slice

- Optional semantic draft metadata in `DesignDraft`:
  - `generation_mode`;
  - `page_spec`;
  - `compiled_native_json`;
  - `quality_report`;
  - `blueprint_id`;
  - `recipe_set_id`;
  - `parent_draft_id`;
  - `version`.
- Expanded `DesignDraftStatus` to include semantic draft terminal/review statuses.
- Added schema parsing and fallbacks for current design draft endpoints:
  - `GET /api/design-drafts`;
  - `POST /api/design-drafts`;
  - `POST /api/design-drafts/agent-tasks`;
  - `GET /api/design-drafts/{id}`;
  - `POST /api/design-drafts/{id}/materialize`.
- Removed the local `generation_mode` cast in `designs-page.tsx`.
- Restored read-only semantic draft detail support:
  - semantic drafts render `compiled_native_json` directly;
  - the page shows `PageSpec`, compile quality, version, and blueprint metadata;
  - template preview loading is skipped for semantic drafts;
  - approve/reject/revise actions remain unported.

## Candidate Next Slice

- Recovery semantic PageSpec review chain:
  - frontend `design-draft-page.tsx` review actions;
  - `approveDesignDraft`, `rejectDesignDraft`, `reviseDesignDraft`;
  - backend routes and handlers for `/approve`, `/reject`, `/revise`;
  - SQL queries for semantic draft approval/revision/rejection.

This slice is not automatically portable: current product memory says PageSpec as a general design engine is paused. Only port it if the desired behavior is explicitly to restore the old semantic draft review flow, not as part of native project design-system Phase 1.
