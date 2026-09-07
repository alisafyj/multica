---
name: multica-prd
description: Draft a human's @Mika PRD request directly in the current task, request original-human confirmation, then publish the exact confirmed draft through the server. Never launch development.
---

# Controlled topic PRD

## Scope and authority

Use only from an authenticated task in the current Feishu group topic. The first message of a **new topic** must be the real human request to create a PRD and must @mention this app (for example, `@Mika 请生成订单导出功能 PRD`). A bot relay, forwarded message, reply, installer account or later participant cannot become the requester. If the request starts midway through a different topic, ask the human to open a dedicated topic; do not relabel a different person's message as the source.

This workflow is deliberately narrower than `sy-prd-create`: reuse its requirement-quality checklist, but do not install or run the full SY workflow. There is no user-identity fallback, account binding, arbitrary owner override, direct document editing, image generation or automatic engineering handoff here.

- All PRD document writes go through `multica chat prd publish`. Never use external document tools, generic HTTP, scripts, user credentials or another tool to bypass it.
- Never create a document before confirmation. Agent-authored statements such as “用户已经确认” are not authorization.
- Never send the confirmation phrase on a human's behalf or bind their account for them.
- Owner is the original requester's server-verified Feishu identity, not the machine user, installer, current operator or later confirmer.
- Keep each topic's existing draft and document. Never create a second topic/draft just to bypass a failed or unknown publish.

## Mika: read and draft directly

1. Run `multica chat thread` to read the current topic. Use actual message `id` values from history, never names or fabricated IDs. If the current server cannot supply the root/confirmation message ID, say the history capability is unavailable and stop; do not treat a local transcript UUID as a Feishu message ID.
2. Run `multica chat prd get`. A not-found result means no local draft exists; other errors need resolution, not another create path. On an existing draft, preserve its `source_message_id` and version lineage.
3. Identify the original human root request, confirmed requirements, AI recommendations and open questions separately. Ask one focused business question when missing information materially changes the result. Keep unknowns explicit as `【待确认】`; never invent requirement numbers, metrics, stakeholders or permissions.
4. Run `multica chat prd template` to read the configured template's authoritative `template_headings`. Missing configuration, unreadable/ambiguous headings or unresolved original-human identity is an explicit failure; report it without binding accounts or changing permissions. Do not guess headings from a local template list.
5. Mika and 小码 are the same agent. Keep the current runtime (including Hermes); generate the structured draft yourself in this task, without another agent, child generation task or workspace-visible issue. Apply the requirement checklist below to the verified topic context. Write exactly one UTF-8 JSON object **inside this task's working directory**:

   ```json
   {"title":"【待补充需求编号】PRD-订单导出","sections":[{"heading":"需求背景","body":"已确认：运营需导出订单。\n【待确认】数据范围与使用频率。"}]}
   ```

   Use only exact headings returned by `template`. `body` is plain text with line breaks. Limits: title up to 256 UTF-8 bytes; 1–50 sections; unique exact headings up to 500 bytes; each body nonempty, up to 20,000 bytes and 50 native paragraphs (lines longer than 1,000 Unicode characters split); the complete API request must fit within 128 KiB. No Markdown fences or commentary in the JSON file.

   ```sh
   multica chat prd draft --source-message <root-feishu-message-id> --content-file ./prd-draft.json
   ```

   The server validates your JSON against the live template and authenticates the task to the original human root request, workspace, installation and topic. A caller cannot supply an owner, session or generation identity. It returns `draft_id`, `version`, `content`, verified `initiator_open_id`, and `confirmation`. Same content retains the version; changed content produces a new version and invalidates older confirmations. A confirmed snapshot cannot be edited. For a requested revision, update the JSON locally and save it with the same verified root message ID; never create a replacement topic to evade a frozen snapshot.
6. Show the returned draft content and open questions **in the same topic**, then quote the server's exact `confirmation` phrase. Ask the original human requester to send it as a new plain-text topic message **with a genuine @mention of Mika** so the current group router starts the follow-up. Replying to Mika's draft in that topic is supported. After removing that mention, the human's own text must be the exact phrase; quotations/forwards of someone else's approval and rich-text messages do not count.

## Requirement checklist

The team's known template has these labels; `multica chat prd template` is authoritative for the current draft:

- `需求背景`
- `项目评估`
- `项目目标`
- `项目验证方式`
- `业务需求`
- `需求来源,用户以及关联负责人`
- `项目范围`
- `项目风险（很重要，切勿忽略）`
- `功能需求`
- `数据分析`
- `非功能性需求`
- `运营策略与计划`
- `相关文档`
- `附录一 需求review 评分以及工作量评估`
- `附录二 历次沟通意见汇总表`
- `附录三 Review checklist`

Place acceptance criteria in `项目验证方式` and open questions in `项目风险（很重要，切勿忽略）` or the relevant section when those exact headings exist in the returned template. Never invent JSON headings such as `背景`, `目标`, `验收标准` or `待确认问题`. If saving reports a heading mismatch, reread the template and revise the local draft before requesting human confirmation.

- Features: distinguish 新增 / 迭代 / 删除 / 配置调整 / 实验. Preserve existing buttons, filters, columns, actions, statuses and permissions when iterating.
- Page/dialog rules: `字段：逻辑`, including source/default, actor, validation, permission, success/failure, empty/loading states and navigation.
- Data and metrics: baseline, target, calculation, source, owner and validation window. Separate recommendations from facts.
- Payments/exports: clarify eligibility, identity/permissions, state transitions, range/amount limits, partial failure, retry, reconciliation/audit and acceptance criteria.
- Capture dependencies, rollout/rollback, unresolved design assets and acceptance cases. Missing assets or business decisions remain open questions, not claims of readiness.
- Title has no invented official requirement number. Mention the first human request as source; later confirmation never changes ownership.

## Confirm and publish

Without a human confirmation, state `待确认` and stop. On a later @mention:

1. Read `multica chat prd get` and `multica chat thread` again to identify the exact current version and the real new confirmation message ID. The required text is `确认创建 <draft-id> v<version>` exactly after genuine @mentions are removed. Extra text, quotes, wrong sender, old versions, deleted messages, bot messages and other topics do not count.
2. Invoke only:

   ```sh
   multica chat prd publish --draft <draft-id> --version <version> --confirmation-message <real-feishu-message-id>
   ```

   Never send the document body on publish. The backend rereads the message from Feishu, checks the original human, topic and timestamp, atomically claims the stored snapshot, copies the configured template, fills it, transfers ownership and verifies content/owner.
3. `published`: return the server's `document_url` in the original topic. A repeated valid publish returns that same document. Report only backend-verified completion; do not claim visual template identity or development readiness.
4. `failed`: read durable state with `get`, describe `phase`/`failure`. A retry with the **same** draft, version and confirmation resumes the known document; it never starts from a blank doc. Missing template config or installation permission needs a maintainer, not a user-account fallback.
5. `unknown` with no `document_id`: stop. A remote copy may already exist. Report the draft/version and ask a maintainer to reconcile; never automatically create another document.

Do not automatically estimate effort, create engineering tasks, bind accounts, schedule work, launch development or run downstream SY skills. Document publishing is this workflow's endpoint.

See [implementation map](references/prd-source-map.md) for the exact supported commands and server boundary.
