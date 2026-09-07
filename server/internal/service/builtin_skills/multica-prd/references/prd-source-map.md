# PRD source map

## Local implementation

- `server/cmd/multica/cmd_chat_prd.go`: `get`, `template`, `draft --source-message --content-file`, and `publish --draft --version --confirmation-message`. Draft files use the existing task-working-directory file guard and preserve UTF-8 JSON content without shell interpolation.
- `server/internal/handler/chat_prd.go`: GET `/api/chat/prd`, GET `/api/chat/prd/template`, POST `/api/chat/prd/draft`, POST `/api/chat/prd/publish`; authenticated task delivery, original-human identity/invoke checks, template/content validation, exact real-message confirmation, immutable confirmed snapshot, claim fencing and durable publication phases.
- `server/internal/handler/chat_history.go`: existing task-only session/history gate. Channel adapters determine whether history exposes native message IDs; unavailable native IDs are a blocker, not permission to fabricate IDs.
- `server/internal/integrations/lark/prd_document.go`: narrow template resolution/copy, native section insertion, owner transfer and readback verification; transport credentials stay on the server. Copy uncertainty must not trigger a second copy.
- `server/internal/integrations/lark/prd_template.go`: reads actual source-template headings before direct drafting and validates them again on save.
- `server/migrations/914_prd_draft.up.sql`, `915_prd_draft_topic_index.up.sql`: one durable row per workspace/installation/chat/topic; no foreign keys or implicit primary-key index; separate concurrent unique-index migration.
- `server/cmd/server/router.go`: task-auth route registration (integration owner).

## Configuration and boundaries

`MULTICA_PRD_TEMPLATE_WIKI_TOKEN` selects the source Wiki template and is server-only. The same authenticated agent (Mika/小码, including Hermes) creates the draft; no delegate configuration or child generation is required. Existing Feishu installation credentials and document permissions must already be configured. The original requester's bound identity must match the active task originator, remain a workspace member and have invocation permission for this agent. No user credential fallback or binding automation is supported.

Template response: `{template_headings: string[]}` in the current task scope. Draft request: `{source_message_id, content: {title, sections: [{heading, body}]}}`; the caller supplies content, never source identity or owner overrides. Unknown fields and obsolete generation references are rejected.
Publish request: `{draft_id, version, confirmation_message_id}` only.
Response: `draft_id`, `version`, `content`, `source_message_id`, `initiator_open_id`, `version_created_at`, `confirmation`, `confirmation_message_id`, `status`, `phase`, `document_id`, `document_url`, `failure` (optional fields where applicable).

Statuses: `draft`, `publishing`, `failed`, `unknown`, `published`. Phases: `resolve`, `copy`, `fill`, `owner`, `verify`. `unknown` without a document ID requires operator reconciliation; never bypass it. Confirmed/published content is immutable. Native template headings must be unique exact matches; plain text only, no opaque Markdown rendering.

## Editable business workflow

`multica-prd` is a compile-time platform API reference delivered to every agent. The separate workspace skill `mika-prd`, bound and enabled for Mika, is the single source for business clarification, writing quality and draft presentation. Do not copy that workflow into the builtin or agent prompt. It adapts the requirement-quality concepts from `sy-ai-workflow/skills/sy-prd-create/SKILL.md` and `references/prd-template.md`, without installing the full SY workflow or authorizing downstream work.

Use the existing workspace **Skills** page to create/edit `mika-prd` and the skill page or agent's **Skills** tab to bind it. Main content is edited as `SKILL.md`; supporting files must not use that reserved path. Creator or workspace owner/admin can edit; binding requires permission to manage the agent. No seed-all-workspaces migration, generic builtin override or separate editor is needed.

- `server/cmd/server/router.go`, `server/internal/handler/skill.go`: authenticated `POST /api/skills` accepts `{name, description, content, files: [{path, content}]}` with the current `X-Workspace-ID`; `PUT /api/skills/{skill-uuid}` edits the same record. Omitted `files` preserves supporting files; `files: []` removes them. Updates publish `skill:updated` after commit.
- `POST /api/agents/{agent-uuid}/skills/add` with `{skill_ids: [skill-uuid]}` adds bindings idempotently without deleting existing ones. `PUT /api/agents/{agent-uuid}/skills` replaces the complete binding list. Both validate workspace and management permission.
- `packages/views/skills/components/skill-detail-page.tsx`: the existing editor saves content/files, adopts the returned version and invalidates skill-list/agent queries. A newer server version refreshes a clean draft or flags a conflict with local edits.
- `server/internal/service/task.go`: each claim loads enabled bound skills and their supporting files from the database, then appends builtins. `server/pkg/skillbundle/hash.go` hashes source, ID, name, description, main content and sorted supporting-file paths/content with SHA-256.
- `server/internal/daemon/skill_cache.go`: bundles are cached under workspace/source/ID/hash and validated against their manifest. A content change changes the next claim's hash; no manual cache purge or daemon restart is required. A cache-miss resolve can receive the current bundle if an edit races with claim preparation, so do not depend on an already-claimed task hot-updating.
- `server/internal/daemon/execenv/hermes_home.go`: prepare/reuse rewrites bound skills under per-task `HERMES_HOME/skills/<slug>/SKILL.md`, including `mika-prd`, while preserving the selected runtime. Verify a saved edit with a **new task**; it does not rewrite an already-running turn or change a stored PRD/confirmed snapshot.

Existing coverage: `builtin_skills_test.go` checks builtin layout/frontmatter; `daemon/skill_bundle_resolve_test.go` checks PRD runtime admission; `pkg/skillbundle/hash_test.go`, `daemon/skill_cache_test.go`, handler skill/bundle tests and Hermes home tests cover delivery infrastructure. The business text is workspace data, not a second compiled workflow.
