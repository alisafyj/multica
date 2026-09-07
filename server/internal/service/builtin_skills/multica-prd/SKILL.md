---
name: multica-prd
description: Platform API reference for task-scoped Feishu PRD templates, file-first drafts, and guarded publication of an original human's confirmed version.
---

# PRD platform capabilities

This skill describes commands and server guarantees, not PRD writing or clarification policy. Use the agent's bound workspace business skill (for Mika, `mika-prd`) for the workflow and response format. Maintain that business skill in the existing workspace Skills editor; do not duplicate its rules here or in the agent prompt. Saved edits apply to new tasks, not an already-running turn.

## Current-task commands

Use the current task's authenticated CLI environment. No server URL, login, account binding, owner override or cross-workspace selection is required or supported by these commands.

| Command | Contract |
| --- | --- |
| `multica chat thread` | Read the current channel topic/session and its real native message IDs. Local transcript UUIDs are not Feishu message IDs. Missing native IDs block PRD source/confirmation operations. |
| `multica chat prd get` | Read the current topic's durable draft/version/publication state. Only not-found means there is no draft. |
| `multica chat prd template` | Read the configured template's authoritative `template_headings`; missing configuration, permissions or ambiguous headings are errors. |
| `multica chat prd draft --source-message <root-feishu-message-id> --content-file ./prd-draft.json` | Validate and save caller-authored JSON from the current task's working directory. Does not create a Feishu document. |
| `multica chat prd publish --draft <draft-id> --version <version> --confirmation-message <real-feishu-message-id>` | Publish the stored version after server verification of the original human's actual confirmation message. Never submit a document body here. |

## Draft format

Write one UTF-8 JSON object to the content file, without shell interpolation, Markdown fences or commentary:

```json
{"title":"PRD title","sections":[{"heading":"Exact heading returned by template","body":"Plain text body\nNext paragraph"}]}
```

Title: at most 256 UTF-8 bytes. Sections: 1–50, with unique exact live-template headings (at most 500 bytes each). Body: nonempty plain text, at most 20,000 bytes and 50 native paragraphs per section; lines longer than 1,000 Unicode characters split into paragraphs. Complete API request: at most 128 KiB. No caller-supplied source identity, session, owner or generation identity.

Save returns `draft_id`, `version`, `content`, `source_message_id`, verified `initiator_open_id`, and `confirmation`. Same content keeps its version; revisions advance it and invalidate older confirmations. A confirmed snapshot is immutable.

## Server authority and publication

- The source is the original human's root PRD request with a genuine mention of this app in the current Feishu topic. The server checks task, workspace, installation, session/topic, bound human identity, membership and invocation permission. Relays, installers and later participants cannot replace that human.
- No document may be created before confirmation. All PRD document writes go through `publish`; external document tools, generic HTTP, scripts or user credentials must not bypass it.
- `confirmation` is the exact phrase `确认创建 <draft-id> v<version>`. The original human must send it as their own new plain-text message in the same topic and genuinely @mention Mika to trigger the follow-up; a same-topic reply is supported. After genuine mentions are removed, the text must match exactly. Quoted/forwarded approval, bots, other senders/topics, deleted messages and stale versions do not authorize publication. An agent must not send confirmation for a human.
- The server rereads the real message, verifies sender/topic/time, freezes and atomically claims the version, copies the configured template, fills content, transfers ownership to the original requester and verifies content/owner. Skill changes cannot relax these checks.
- `published` returns the verified `document_url`; repeated valid publication returns the same document. `failed` exposes durable `phase`/`failure` via `get`; retrying the same draft/version/confirmation resumes the known document. `unknown` without `document_id` requires maintainer reconciliation because a remote copy may already exist. Never create a replacement topic/draft/document to bypass frozen or uncertain state.

See [implementation map](references/prd-source-map.md) for API shapes, configuration and editable-skill delivery.
