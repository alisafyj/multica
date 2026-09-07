# PRD source map

## Local implementation

- `server/cmd/multica/cmd_chat_prd.go`: `multica chat prd get`, `draft --source-message --content-file`, `publish --draft --version --confirmation-message`; safe workdir file loading uses `resolveTextFlag` in `cmd_issue.go`, with UTF-8/JSON checks.
- `server/internal/handler/chat_prd.go`: GET `/api/chat/prd`, POST `/api/chat/prd/draft`, POST `/api/chat/prd/publish`; current authenticated chat/task delivery, source-root identity, exact real-message confirmation, immutable confirmed snapshot, claim fencing and durable publication phases.
- `server/internal/handler/chat_history.go`: existing task-only session/history gate. Channel adapters determine whether history exposes native message IDs; unavailable native IDs are a blocker, not permission to fabricate IDs.
- `server/internal/integrations/lark/prd_document.go`: narrow template resolution/copy, native section insertion, owner transfer and readback verification; transport credentials stay on the server. Copy uncertainty must not trigger a second copy.
- `server/migrations/912_prd_draft.up.sql`, `913_prd_draft_topic_index.up.sql`: one durable row per workspace/installation/chat/topic; no foreign keys or implicit primary-key index; separate concurrent unique-index migration.
- `server/cmd/server/router.go`: task-auth route registration (integration owner).

## Configuration and boundaries

`MULTICA_PRD_TEMPLATE_WIKI_TOKEN` selects the source Wiki template. It is server-only, not supplied by an agent. Existing Feishu installation credentials and document permissions must already be configured. No user credential fallback or binding automation is supported.

Draft request: `{source_message_id, content: {title, sections: [{heading, body}]}}`.
Publish request: `{draft_id, version, confirmation_message_id}` only.
Response: `draft_id`, `version`, `content`, `source_message_id`, `initiator_open_id`, `version_created_at`, `confirmation`, `confirmation_message_id`, `status`, `phase`, `document_id`, `document_url`, `failure` (optional fields where applicable).

Statuses: `draft`, `publishing`, `failed`, `unknown`, `published`. Phases: `resolve`, `copy`, `fill`, `owner`, `verify`. `unknown` without a document ID requires operator reconciliation; never bypass it. Confirmed/published content is immutable. Native template headings must be unique exact matches; plain text only, no opaque Markdown rendering.

## Product reference

Adapted with development authorization from `sy-ai-workflow/skills/sy-prd-create/SKILL.md` and `references/prd-template.md`: original requester ownership, explicit missing fields, template-first creation, field-by-field requirements, and no unconfirmed downstream development. This builtin skill intentionally excludes the full SY workflow, user-authority fallback, prototypes, scheduling and issue creation.
