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

## Product reference

Adapted with development authorization from `sy-ai-workflow/skills/sy-prd-create/SKILL.md` and `references/prd-template.md`: original requester ownership, explicit missing fields, template-first creation, field-by-field requirements, and no unconfirmed downstream development. This builtin skill intentionally excludes the full SY workflow, user-authority fallback, prototypes, scheduling and issue creation.
