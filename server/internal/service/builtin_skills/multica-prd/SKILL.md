---
name: multica-prd
description: Coordinate a human's @Mika PRD request through the configured 小码 draft writer, recover its structured JSON for original-human confirmation, then publish the exact confirmed draft through the server. Never launch development.
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

## Mika: read, delegate and recover

1. Run `multica chat thread` to read the current topic. Use actual message `id` values from history, never names or fabricated IDs. If the current server cannot supply the root/confirmation message ID, say the history capability is unavailable and stop; do not treat a local transcript UUID as a Feishu message ID.
2. Run `multica chat prd get`. A not-found result means no local draft exists; other errors need resolution, not another create path. On an existing draft, preserve its `source_message_id` and version lineage.
3. Identify the original human root request, confirmed requirements, AI recommendations and open questions separately. Ask one focused business question when missing information materially changes the result. Keep unknowns explicit as `【待确认】`; never invent requirement numbers, metrics, stakeholders or permissions.
4. Prepare a UTF-8 requirements brief **inside this task's working directory**, separating the original request, confirmed requirements, recommendations, open questions and any requested revision. Do not generate the PRD yourself. Run:

   ```sh
   multica chat prd delegate --source-message <root-feishu-message-id> --brief-file ./prd-brief.txt
   multica chat prd generation --task <returned-task-id> --wait 5m
   ```

   The server resolves the intended 小码 using its operator-configured `MULTICA_PRD_DELEGATE_AGENT_ID`, validates the current workspace and original human's invocation rights, and reads the configured template's actual unique heading labels. There is no fuzzy-name selection, live-ID constant, machine-user substitution, or self-generation fallback. Missing configuration, inaccessible/archived/unbound 小码, unresolved human identity or unreadable/ambiguous template is an explicit failure; report it without binding accounts or changing permissions.

   小码 runs through the existing task queue in a separate private draft-generation chat owned by the original human. It receives the verified root text, your brief and frozen template headings, but no channel-delivery route, project repositories or requester credential overlay. It must return only the JSON draft; it cannot use the topic's PRD publication API. Do not create a workspace-visible issue merely to delegate private topic content.
5. `generation` returns `task_id`, `status`, `source_message_id` and, only when the completed output is valid, `content`. Read the content and surface open questions; never replace it with Mika-authored JSON. `failed`, `cancelled` or `invalid` means stop and report the failure, not silently self-generate. If waiting ends while queued/running, preserve the task ID and state that drafting is pending; on a later @mention use `multica chat prd generation` (without `--task` recovers this topic's latest generation). Never say the draft is complete merely because delegation was accepted. Repeating an identical delegate request recovers the same generation; changed briefs cannot replace a still-running generation.

   After the failure has been addressed, an explicit `delegate --retry` with the same source and brief may start a replacement for a failed/cancelled/invalid generation. It never replaces active work or a valid completed result. Do not automatically loop retries or change the brief just to evade the recorded failure.

   Save the exact server-correlated 小码 result:

   ```sh
   multica chat prd draft --source-message <root-feishu-message-id> --generation-task <completed-task-id>
   ```

   Draft accepts no caller-authored content. The server verifies the generation's workspace, installation, source session/context, topic, root request, delegate identity and source-task lineage, then recovers the completed JSON itself. It returns `draft_id`, `version`, `content`, verified `initiator_open_id`, and `confirmation`. Same content retains the version; changed content produces a new version and invalidates older confirmations. A confirmed snapshot cannot be edited. Requested revisions go back to 小码 through a changed brief, never a local rewrite.
6. Show the returned draft content and open questions **in the same topic**, then quote the server's exact `confirmation` phrase. Ask the original human requester to send it as a new plain-text topic message **with a genuine @mention of Mika** so the current group router starts the follow-up. Replying to Mika's draft in that topic is supported. After removing that mention, the human's own text must be the exact phrase; quotations/forwards of someone else's approval and rich-text messages do not count.

## 小码: delegated draft only

When the server's task prompt says you are the delegated PRD writer, **do not run Mika's coordinator commands above**. Apply the requirement checklist below to the supplied root request and requirements brief. Use only the authoritative `template_headings` supplied in that task, not this document's illustrative team-template list if they differ.

Return exactly one UTF-8 JSON object, no Markdown fences or surrounding commentary:

```json
{"title":"【待补充需求编号】PRD-订单导出","sections":[{"heading":"需求背景","body":"已确认：运营需导出订单。\n【待确认】数据范围与使用频率。"}]}
```

`body` is plain text with line breaks. Limits: title up to 256 UTF-8 bytes; 1–50 sections; unique exact template headings up to 500 bytes; each body nonempty, up to 20,000 bytes and 50 native paragraphs (lines longer than 1,000 Unicode characters split); JSON up to 128 KiB. Unknown business decisions remain `【待确认】`. No document creation/editing/publication, onward delegation, engineering tasks, account binding, external credentials, scheduling or development. Mika recovers your output for the original human's review; your output is never authorization.

## Requirement checklist

The team's known template has these labels; the server-read `template_headings` snapshot is authoritative for each generation:

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

Place acceptance criteria in `项目验证方式` and open questions in `项目风险（很重要，切勿忽略）` or the relevant section when those exact headings exist in the supplied snapshot. Never invent JSON headings such as `背景`, `目标`, `验收标准` or `待确认问题`. A changed template must be read by the server before delegation, not guessed by either agent.

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
