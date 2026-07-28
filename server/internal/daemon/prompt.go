package daemon

import (
	"fmt"
	"strings"

	"github.com/multica-ai/multica/server/internal/daemon/execenv"
)

// BuildPrompt constructs the task prompt for an agent CLI.
// Keep this minimal — detailed instructions live in CLAUDE.md / AGENTS.md
// injected by execenv.InjectRuntimeConfig. The provider string is threaded
// through to comment-triggered tasks' per-turn reply template; that template
// is provider-agnostic now (Linux/macOS → quoted-HEREDOC stdin, Windows →
// file) because the shell-layer corruption it guards against is not specific
// to any one provider (MUL-2904).
func BuildPrompt(task Task, provider string) string {
	if task.ChatSessionID != "" {
		return buildChatPrompt(task)
	}
	if task.TriggerCommentID != "" {
		return buildCommentPrompt(task, provider)
	}
	if task.AutopilotRunID != "" {
		return buildAutopilotPrompt(task)
	}
	if task.QuickCreatePrompt != "" {
		return buildQuickCreatePrompt(task)
	}
	if len(task.UIDraftCreateContext) > 0 {
		return buildUIDraftCreatePrompt(task)
	}
	if len(task.DesignRestoreContext) > 0 {
		return buildDesignRestorePrompt(task)
	}
	if len(task.DesignSystemProfileAnalyzeContext) > 0 {
		return buildDesignSystemProfileAnalyzePrompt(task)
	}
	if len(task.ProjectDesignSystemContext) > 0 {
		return buildProjectDesignSystemPrompt()
	}
	var b strings.Builder
	b.WriteString("You are running as a local coding agent for a Multica workspace.\n\n")
	fmt.Fprintf(&b, "Your assigned issue ID is: %s\n\n", task.IssueID)
	fmt.Fprintf(&b, "Start by running `multica issue get %s --output json` to understand your task, then complete it.\n", task.IssueID)
	fmt.Fprintf(&b, "For comment history, follow the rule in your runtime workflow file (assignment-triggered tasks treat the read as mandatory). `multica issue comment list %s --output json` returns all comments for the issue (server caps at 2000). On long-running issues use `--recent 20 --output json` to read the 20 most recently active threads, then page older threads via the stderr `Next thread cursor: ...` line and the matching `--before` / `--before-id` until you have enough history. `--since <RFC3339>` is still available for incremental polling and may combine with `--recent`.\n", task.IssueID)
	return b.String()
}

func buildProjectDesignSystemPrompt() string {
	var b strings.Builder
	b.WriteString("You are running as a project design system designer for a Multica workspace.\n\n")
	b.WriteString("Read `.agent_context/project_design_system/task.json` first. Treat the user brief as the primary intent and references as evidence.\n")
	b.WriteString("For adjust or regenerate operations, read all three base files before designing: `base/DESIGN.md`, `base/tokens.css`, and `base/components.html`.\n\n")
	b.WriteString("Rules:\n")
	b.WriteString("- Use Open Design's stable Token layers. Do not invent unsupported project facts merely to fill a catalog.\n")
	b.WriteString("- Create one coherent direction, not multiple alternatives or a demo switcher.\n")
	b.WriteString("- components.html is a real static UI Kit using tokens.css, with project-relevant components, states, and representative compositions.\n")
	b.WriteString("- Every selectable component or block must have unique `data-design-node-id`, `data-design-node-kind`, and `data-design-node-label` attributes.\n")
	b.WriteString("- Never write scripts, event attributes, imports, forms, external embeds, or arbitrary remote resources.\n")
	b.WriteString("- For adjustment, return a complete mutually consistent replacement of all three files even when the requested scope is local.\n")
	b.WriteString("- Write exact files to `$MULTICA_OUTPUT_DIR/DESIGN.md`, `$MULTICA_OUTPUT_DIR/tokens.css`, and `$MULTICA_OUTPUT_DIR/components.html`.\n")
	b.WriteString("- Do not paste file contents into the final response; report only a short completion summary.\n")
	b.WriteString("- Do not modify a repository, call Figma, upload a design file, or call Multica write commands.\n")
	b.WriteString("- Do not report success unless all three output files have been written.\n")
	return b.String()
}

func buildDesignSystemProfileAnalyzePrompt(task Task) string {
	var b strings.Builder
	b.WriteString("You are running as a design system profile analysis agent for a Multica workspace.\n\n")
	b.WriteString("Use ONLY the design_system_profile_analyze context JSON below as the source of truth. This is a semantic classification task for an uploaded Figma UI specification.\n\n")
	b.WriteString("Your job is to convert cleaned UI specification candidates into an Agent-readable project visual contract.\n\n")
	b.WriteString("Return your final answer as a single JSON object only, with this shape:\n")
	b.WriteString("{\"profile_json\": object, \"analysis_errors\": array, \"summary\": string}\n\n")
	b.WriteString("Rules:\n")
	b.WriteString("- Perform semantic classification from names, text samples, dimensions, colors, and hierarchy summaries. Do not rely on a fixed backend component dictionary.\n")
	b.WriteString("- Respect the naming convention `组件 - 变体 - 状态`, such as `按钮 - 主按钮 - 默认`, while allowing normal design-system extensions when the intent is clear.\n")
	b.WriteString("- Group component examples under `profile_json.components.{kind}.variants[].states` and keep source layer IDs in examples so future agents can trace decisions.\n")
	b.WriteString("- Extract reusable tokens into `profile_json.tokens.colors`, `profile_json.tokens.typography`, `profile_json.tokens.spacing`, and `profile_json.tokens.radius` when the evidence exists.\n")
	b.WriteString("- Add concise `guidelines` and `anti_rules` that UI Agent and UI Restore Agent can follow directly.\n")
	b.WriteString("- Keep warnings in `analysis_errors`; do not fail just because some layers are noisy or partially named.\n")
	b.WriteString("- Do not create files, edit repositories, upload designs, call Figma, or call Multica write commands. The server will store your JSON output.\n")
	b.WriteString("- Do not output markdown fences, prose outside JSON, comments, or trailing text.\n\n")
	b.WriteString("Design system profile analysis context JSON:\n")
	b.Write(task.DesignSystemProfileAnalyzeContext)
	b.WriteString("\n")
	return b.String()
}

func buildDesignRestorePrompt(task Task) string {
	var b strings.Builder
	b.WriteString("You are running as a Gallery Native frontend restore agent for a Multica workspace.\n\n")
	b.WriteString("Use ONLY the Multica design restore context JSON below as the design source of truth. If issue_id is present, also run `multica issue get <issue_id> --output json` before editing.\n\n")
	b.WriteString("Your job is to implement the smallest safe frontend code change that matches the restore task.\n\n")
	b.WriteString("If `restore_plan` is present in the context, treat it as the approved execution contract. Follow its selected target, allowed paths, scope, mapping, risks, and steps before falling back to raw item context.\n\n")
	b.WriteString("Rules:\n")
	b.WriteString("- The embedded `item_contexts` are snapshots from Multica `/api/design-files/{design_file_id}/frames/{frame_id}/context`; treat them as authoritative.\n")
	b.WriteString("- When `design_system.status` is `analyzed`, treat `design_system.profile` as the cloud project visual contract. Read its components, variants, states, tokens, guidelines, and anti_rules before implementing the design.\n")
	b.WriteString("- Design-context priority is exactly: Cloud design_system_profile > local DESIGN.md > repository reality. The cloud profile controls intended visual language; local files and repository reality guide feasible implementation without overriding it.\n")
	b.WriteString("- If a root-level `DESIGN.md` already exists in the current repository, read it as read-only implementation context after the cloud profile. Never create, patch, sync, or overwrite `DESIGN.md`.\n")
	b.WriteString("- When an approved `restore_plan` exists, do not ignore it or silently change target paths/scope; report blockers if it cannot be followed.\n")
	b.WriteString("- For production restore plans (`restore_plan.repo.mode == \"production_candidate\"`), write only under `restore_plan.execution.allowedPaths`; do not write prototype HTML or files under `fengchenDoc/gallery-native-agent-test`.\n")
	b.WriteString("- Read `restore_plan.targetStrategy` before editing. When it is `business_module`, behave like a normal programmer: create or update the named business module from moduleName/moduleSlug, then place page, components, and router changes in the planned module paths.\n")
	b.WriteString("- Treat `restore_plan.targets.selected` as a delivery contract, not a single-file dump. If it contains pagePath/routeOwner/componentRoot/routePath, create or update a navigable page, wire the router, and split sections into normal project components.\n")
	b.WriteString("- Different `pageName` values are page or route boundaries. When `restore_plan.targets.pageTargets` exists, implement each page target as a separate navigable page/route or route-owned view, not as a tab inside one page.\n")
	b.WriteString("- Do NOT implement different `pageName` values as tabs, segmented controls, or demo switches. Tabs are allowed only when an explicit tab control exists in item_contexts/design layers. Frames with the same `pageName` may share one page as states, modals, or result states.\n")
	b.WriteString("- Read `restore_plan.interactionFlow` before editing: query parameters are debug/deep-link aids only; the primary user path must be implemented with click handlers, router navigation, and component state.\n")
	b.WriteString("- Do not default to restore-id sandbox directories for production plans. Use `design-restore/restore-*` only when the approved plan explicitly selects a sandbox fallback or reports that business module inference is unavailable.\n")
	b.WriteString("- If the repo already has an obviously matching page/route, you may use it only when it is inside allowedPaths and the plan permits it; otherwise create the planned page and route.\n")
	b.WriteString("- Never write under `restore_plan.execution.forbiddenPaths`; if the requested target conflicts with forbidden paths, return blocked with the conflict.\n")
	b.WriteString("- Default restore mode is `strict-structure`: produce visible HTML/CSS/component structure from layers, not a pasted screenshot.\n")
	b.WriteString("- Do NOT call sy-gallery_* tools or use an external Gallery MCP current session/sketch as source material. Those may point at a different design and must be ignored for this task.\n")
	b.WriteString("- Do NOT delegate implementation to background agents, async lanes, or sub-agents. Finish the file edits, verification, and RESTORE_RESULT_JSON in this task before exiting.\n")
	b.WriteString("- Do NOT invent business copy, names, phone numbers, tabs, or components that are absent from `item_contexts`/assets.\n")
	b.WriteString("- Do NOT use full-frame preview, thumbnail, or full-frame slice assets as the primary result. Forbidden examples: `frame_preview-*`, `frame_thumbnail-*`, and a frame-sized slice.\n")
	b.WriteString("- If structural reconstruction is insufficient, do not fake completion by pasting the screenshot. Either return blocked with a concrete reason, or create a clearly marked centered placeholder saying `缺少可结构化 UI 稿` plus the reason.\n")
	b.WriteString("- Use restore_task_id/design_file_id/revision_id only to cross-check identity; do not substitute another sketch/design ID.\n")
	b.WriteString("- Prefer existing project components and conventions. Do not put the whole design into one monolithic page file when normal components/sections should be split. Respect package boundaries.\n")
	b.WriteString("- Do not change backend unless the issue explicitly requires it.\n")
	b.WriteString("- Run the relevant typecheck/test command before final response.\n")
	b.WriteString("- Visual QA loop is mandatory for completed work: Open the implemented route, capture an implementation screenshot, compare it with the authoritative frame_preview asset from item_contexts, create or describe a side-by-side comparison, then make at least one correction pass for obvious visual mismatches before final response.\n")
	b.WriteString("- For the Visual QA loop, layer JSON controls structure/position/text, while frame_preview controls final visual calibration for image crop, icon shape, spacing, color, and fixed bars. Prefer exported slice assets for icons and small visual elements instead of hand-drawn approximations when those slices exist.\n")
	b.WriteString("- If you cannot open the route or capture screenshots, do not omit the visual review. Put the concrete blocker in `visualReview.remainingDiffs` and `blockers`, and lower `visualFidelityScore` accordingly.\n")
	b.WriteString("- For `ui_generation`, create a UI restore artifact document in the target repo. Use `restore_plan.artifacts.uiRestoreDocument.path` when present, otherwise use `docs/multica/ui-restore/<restore_task_id>.md`. The document must summarize entry routes, changed files, page/state/modal mapping, restoreMapping, checks, blockers, and remaining frontend integration notes.\n")
	b.WriteString("- For `frontend_restore`, if the received delivery or restore input includes `artifactDocPath`, read that artifact document first and treat it as the UI implementation handoff before touching API/state/integration work.\n")
	b.WriteString("- Final response must summarize changed files, checks run, blockers, restore mapping, exact layer text/asset IDs used, Visual QA evidence, and explicitly state `usedFullFramePreview: false` unless blocked.\n")
	b.WriteString("- End your final response with a machine-readable JSON block prefixed by exactly `RESTORE_RESULT_JSON:`. Shape: {\"status\":\"completed|blocked|failed\",\"summary\":string,\"files\":string[],\"checks\":string[],\"blockers\":string[],\"restoreMapping\":array,\"usedLayerIds\":string[],\"usedAssetIds\":string[],\"usedFullFramePreview\":boolean,\"policyViolation\":string,\"artifactDocPath\":string,\"visualFidelityScore\":number,\"visualReview\":{\"implementedRoute\":string,\"designScreenshot\":string,\"implementationScreenshot\":string,\"comparisonScreenshot\":string,\"remainingDiffs\":string[],\"notes\":string}}.\n\n")
	b.WriteString("Design restore context JSON:\n")
	b.Write(task.DesignRestoreContext)
	b.WriteString("\n")
	return b.String()
}

func buildUIDraftCreatePrompt(task Task) string {
	var b strings.Builder
	b.WriteString("You are running as a UI design draft generation agent for a Multica workspace.\n\n")
	b.WriteString("Use the UI draft context JSON below as the source of truth. If it includes `issue`, that issue content has already been embedded for you.\n\n")
	b.WriteString("Your job is to generate controlled DesignDraft data for human review, not to create or edit design files directly.\n\n")
	b.WriteString("Return your final answer as a single JSON object only, with this shape:\n")
	b.WriteString("{\"title\": string, \"catalog_template_id\": string, \"requirement_core\": object, \"slot_values\": object, \"patch\": array}\n\n")
	b.WriteString("Rules:\n")
	b.WriteString("- If the context includes `parent_issue`, treat `parent_issue` as the primary PRD / business requirement source.\n")
	b.WriteString("- When `parent_issue` exists, the current `issue` is the UI design task scope and constraints; do not treat its short title as the full requirement.\n")
	b.WriteString("- If `design_system` is present, treat `design_system.profile` as the project visual contract. Read `components.{kind}.variants[].states`, examples, tokens, patterns, guidelines, and anti_rules before deciding any visual structure.\n")
	b.WriteString("- Design-context priority is exactly: Cloud design_system_profile > local DESIGN.md > repository reality. If a root-level `DESIGN.md` already exists in the current project repository, read it only as auxiliary project context. Never create, patch, sync, or overwrite `DESIGN.md`.\n")
	b.WriteString("- The design system naming convention is usually `组件 - 变体 - 状态`, such as `按钮 - 主按钮 - 默认`; use these compiled variants/states instead of guessing from raw Figma layers.\n")
	b.WriteString("- Templates are structure references only. If a template conflicts with the issue or design_system, the issue and design_system win.\n")
	b.WriteString("- If `template_candidates` is present, choose the best template candidate and return its `id` as `catalog_template_id`.\n")
	b.WriteString("- Choose the best template candidate by matching the issue requirement to `template_profile.page_type`, tags, slots, and component intent.\n")
	b.WriteString("- Selecting a template is not enough: the final JSON must contain actual design changes in `slot_values` or `patch`.\n")
	b.WriteString("- Prefer slot_values when the selected template has a non-empty slot_schema.\n")
	b.WriteString("- If the selected template has an empty slot_schema, use `editable_text_layers` and `patch_hints` from that candidate to produce a non-empty safe text patch.\n")
	b.WriteString("- Use patch only for safe non-layout metadata/text changes. For a visible text replacement, patch both `/layers/{layer_id}/text/characters` and `/layers/{layer_id}/text/text` when both paths are available.\n")
	b.WriteString("- Do not return empty `slot_values: {}` and empty `patch: []`; if you cannot identify any safe change, return a JSON object with a clear `requirement_core.blocked_reason` and no fake completion.\n")
	b.WriteString("- Do not patch layout/tree paths or segments: x, y, width, height, children.\n")
	b.WriteString("- Match every required slot in slot_schema and respect primitive types.\n")
	b.WriteString("- Do not output markdown fences, prose, comments, or extra text.\n\n")
	b.WriteString("UI draft context JSON:\n")
	b.Write(task.UIDraftCreateContext)
	b.WriteString("\n")
	return b.String()
}

// buildQuickCreatePrompt constructs a prompt for quick-create tasks. The
// user typed a single natural-language sentence in the create-issue modal;
// the agent's job is to translate it into one `multica issue create` CLI
// invocation, using its judgment to decide whether fetching referenced URLs
// would produce a better issue. No issue exists yet, so the agent must NOT
// call `multica issue get` or attempt to comment — there's nothing to read
// or reply to.
func buildQuickCreatePrompt(task Task) string {
	var b strings.Builder
	b.WriteString("You are running as a quick-create assistant for a Multica workspace.\n\n")
	b.WriteString("A user captured the following input via the quick-create modal. There is NO existing issue. Your job is to create a well-formed issue from this input with a single `multica issue create` command.\n\n")
	fmt.Fprintf(&b, "User input:\n> %s\n\n", task.QuickCreatePrompt)

	b.WriteString("Field rules:\n\n")

	// title
	b.WriteString("- **title**: required. A concise but semantically rich summary. If the input references external resources (PRs, issues, URLs), use your judgment on whether fetching the resource would produce a meaningfully better title — e.g. \"review PR #123\" → \"Review PR #123: Refactor auth module to OAuth2\". Strip filler words but preserve key semantic information.\n\n")

	// description — the core optimization
	b.WriteString("- **description**: The description is the executing agent's primary context. Aim for high fidelity — they should grasp the user's intent as if they had read the raw input themselves. Use a two-section structure:\n\n")
	b.WriteString("  1. **User request** — Faithfully restate what the user wants in their own words. Preserve specific names, identifiers, file paths, code snippets, and technical terms verbatim. Strip non-spec material before writing it (this is removal, not paraphrasing): verbal routing wrappers about creating the issue or routing it (e.g. \"create an issue\", \"分配给 X\", \"让 @X 处理\") and pure conversational fillers (e.g. \"对吧？\"). When in doubt, keep it.\n\n")
	b.WriteString("     CC exception: `multica issue create` has no `--subscriber` flag, and the platform auto-subscribes members whose `[@Name](mention://member/<uuid>)` link appears in the description. When the user wrote \"cc @Y\", strip the verbal \"cc\" wrapper from the User request body and append a final `CC: <mention link(s)>` line to the description so the cc routing still fires.\n\n")
	b.WriteString("  2. **Context** — include ONLY when the input cited external resources AND you successfully fetched them AND they produced verifiable facts worth recording. Summarize facts only (e.g. \"PR #45 changes auth to JWT\"), not interpretation or unsolicited reference implementations. If you have nothing factual to add, omit the section entirely — never use it as an apology log for resources you could not fetch.\n\n")
	b.WriteString("  Hard rules: never invent requirements, implementation details, or acceptance criteria the user did not express; never reduce multi-sentence input to a single vague sentence; never echo the title.\n\n")

	// priority
	b.WriteString("- **priority**: one of `urgent`, `high`, `medium`, `low`, or omit. Map P0/P1 → urgent/high; \"asap\" → urgent. If unspecified, omit.\n\n")

	// assignee
	b.WriteString("- **assignee**:\n")
	b.WriteString("    - When the user names someone (\"assign to X\" / \"@X\"), call `multica workspace member list --output json`, `multica agent list --output json`, and `multica squad list --output json` and find the matching entity by display name. Squads are first-class assignees too — a squad name (e.g. \"Super Human\") routes work to the squad leader, who then delegates. On a clean unambiguous match, prefer `--assignee-id <uuid>` using the `user_id` (member) or `id` (agent or squad) from that JSON — UUID matching is exact and robust to name collisions in workspaces with overlapping names. `--assignee <name>` (fuzzy) is acceptable as a fallback when names are unambiguous. On no match or ambiguous match, do NOT pass either flag — instead append a final line to the description: `Unrecognized assignee: X`.\n")
	b.WriteString("    - Treat bare @-routing as an assignee directive even when the user did not write the English word \"assign\". This includes Chinese imperatives like `让 @独立团 review 这个 PR`, `给 @X 处理`, or `交给 @X`; strip the leading `@`/`＠` before matching display names. Do not keep that routing wrapper or `@Name` in the description unless it is a true CC-style notification rather than ownership. If the matched entity is a squad, pass the squad's `id` as `--assignee-id`, not the leader agent's id.\n")
	agentID := ""
	agentName := ""
	if task.Agent != nil {
		agentID = task.Agent.ID
		agentName = task.Agent.Name
	}
	switch {
	case task.SquadID != "":
		// The user opened quick-create with a SQUAD selected. The task
		// runs on the squad's leader agent, but the squad is the expected
		// owner — assigning to the leader would mask the squad's
		// delegation flow. Always point the default at the squad UUID.
		if task.SquadName != "" {
			fmt.Fprintf(&b, "    - When the user did NOT name an assignee, default to the picker SQUAD %q: pass `--assignee-id %q` (the squad's UUID). The user opened quick-create with the squad selected; you (the leader agent) are running on the squad's behalf, so the squad — not you — is the expected owner. Never leave the issue unassigned, and do not assign it to your own agent UUID.\n\n", task.SquadName, task.SquadID)
		} else {
			fmt.Fprintf(&b, "    - When the user did NOT name an assignee, default to the picker SQUAD: pass `--assignee-id %q` (the squad's UUID). The user opened quick-create with the squad selected; you (the leader agent) are running on the squad's behalf, so the squad — not you — is the expected owner. Never leave the issue unassigned, and do not assign it to your own agent UUID.\n\n", task.SquadID)
		}
	case agentID != "":
		fmt.Fprintf(&b, "    - When the user did NOT name an assignee, default to YOURSELF: pass `--assignee-id %q` (your agent UUID). The picker agent is the expected owner because the user opened quick-create with you selected — never leave the issue unassigned. Use the UUID flag, not `--assignee <name>`, so the assignment is unambiguous even when other agents share part of your name.\n\n", agentID)
	case agentName != "":
		fmt.Fprintf(&b, "    - When the user did NOT name an assignee, default to YOURSELF: pass `--assignee %q`. The picker agent is the expected owner because the user opened quick-create with you selected — never leave the issue unassigned.\n\n", agentName)
	default:
		b.WriteString("    - When the user did NOT name an assignee, default to YOURSELF (the picker agent): pass `--assignee-id <your agent UUID>` (preferred) or `--assignee <your agent name>`. Never leave the issue unassigned.\n\n")
	}

	// project — pinned by the modal when the user picked one, otherwise
	// omitted so the platform routes to the workspace default. Always pass
	// the UUID (never a name) so the issue lands in the right project even
	// when several share a title.
	if task.ProjectID != "" {
		if task.ProjectTitle != "" {
			fmt.Fprintf(&b, "- **project**: required for this run. Pass `--project %q` so the new issue lands in project %q (the user picked it in the quick-create modal). Do not infer a different project from the prompt text — the modal selection is authoritative.\n", task.ProjectID, task.ProjectTitle)
		} else {
			fmt.Fprintf(&b, "- **project**: required for this run. Pass `--project %q` so the new issue lands in the project the user picked in the quick-create modal. Do not infer a different project from the prompt text — the modal selection is authoritative.\n", task.ProjectID)
		}
	} else {
		b.WriteString("- **project**: omit. The platform will route the issue to the workspace default.\n")
	}
	// parent — pinned by the modal when the user opened it from "Add sub
	// issue" on an existing issue. Pass the UUID (never the identifier) so
	// the create lands the sub-issue under the right parent even when the
	// workspace prefix changes; the identifier is included in the prose
	// purely as human-readable context for the agent.
	if task.ParentIssueID != "" {
		if task.ParentIssueIdentifier != "" {
			fmt.Fprintf(&b, "- **parent**: required for this run. Pass `--parent %q` so the new issue is filed as a sub-issue of %s (the user opened quick-create from that issue's \"Add sub issue\" entry). Do not infer a different parent from the prompt text — the modal entry point is authoritative.\n", task.ParentIssueID, task.ParentIssueIdentifier)
		} else {
			fmt.Fprintf(&b, "- **parent**: required for this run. Pass `--parent %q` so the new issue is filed as a sub-issue of the parent the user picked in the quick-create modal. Do not infer a different parent from the prompt text — the modal entry point is authoritative.\n", task.ParentIssueID)
		}
	}
	b.WriteString("- **status**: omit (defaults to `todo`).\n")
	b.WriteString("- **attachments**: do NOT pass `--attachment`. The flag only accepts LOCAL file paths. Any image URL in the user input is already markdown — keep it inline in `--description` instead.\n\n")

	// output format
	b.WriteString("Output format:\n")
	b.WriteString("- Run exactly one `multica issue create --output json` invocation. Do not retry for any reason — even on non-zero exit. The issue may already exist; another attempt would create a duplicate.\n")
	b.WriteString("- Parse the JSON response to read the created issue's `identifier` (preferred) or `id` (fallback). Do not scrape human output and do not assume any workspace issue prefix such as `MUL-`; workspaces can use custom prefixes.\n")
	b.WriteString("- After success, print exactly one line: `Created <identifier-or-id>: <title>` and exit. No commentary, no follow-up tool calls.\n")
	b.WriteString("- Do NOT call `multica issue get` or `multica issue comment add` — there is no issue to query or comment on.\n")
	b.WriteString("- On CLI error or JSON parse error, exit with the error as the only output. The platform writes a failure notification automatically.\n")
	return b.String()
}

// buildCommentPrompt constructs a prompt for comment-triggered tasks.
// The triggering comment content is embedded directly so the agent cannot
// miss it, even when stale output files exist in a reused workdir.
// The reply instructions (including the current TriggerCommentID as --parent)
// are re-emitted on every turn so resumed sessions cannot carry forward a
// previous turn's --parent UUID.
func buildCommentPrompt(task Task, provider string) string {
	var b strings.Builder
	b.WriteString("You are running as a local coding agent for a Multica workspace.\n\n")
	fmt.Fprintf(&b, "Your assigned issue ID is: %s\n\n", task.IssueID)
	if task.TriggerCommentContent != "" {
		authorLabel := "A user"
		if task.TriggerAuthorType == "agent" {
			name := task.TriggerAuthorName
			if name == "" {
				name = "another agent"
			}
			authorLabel = fmt.Sprintf("Another agent (%s)", name)
		}
		fmt.Fprintf(&b, "[NEW COMMENT] %s just left a new comment. Focus on THIS comment — do not confuse it with previous ones:\n\n", authorLabel)
		fmt.Fprintf(&b, "> %s\n\n", task.TriggerCommentContent)
		if task.TriggerAuthorType == "agent" {
			b.WriteString("⚠️ The triggering comment was posted by another agent. Decide whether a reply is warranted. If you produced actual work this turn (investigated, fixed something, answered a real question), post the result as a normal reply — that is NOT a noise comment, and the standard rule that final results must be delivered via comment still applies. If the triggering comment was a pure acknowledgment, thanks, or sign-off AND you produced no work this turn, do NOT reply — and do NOT post a comment saying 'No reply needed' or similar. Simply exit with no output. Silence is the preferred way to end agent-to-agent threads. If you do reply, do not @mention the other agent as a sign-off (that re-triggers them and starts a loop).\n\n")
		}
		if task.Agent != nil && strings.Contains(task.Agent.Instructions, "## Squad Operating Protocol") {
			fmt.Fprintf(&b, "⚠️ **Squad leader no_action rule:** If you decide no action is needed, call `multica squad activity %s no_action --reason \"...\"` and EXIT. DO NOT post any comment — not even one that says \"no action needed\" or \"exiting silently\". The squad activity call records your decision; a comment is redundant noise.\n\n", task.IssueID)
		}
	}
	fmt.Fprintf(&b, "Start by running `multica issue get %s --output json` to understand your task, then decide how to proceed.\n\n", task.IssueID)
	// Comment-reading pointer. Warm path with new comments: issue-wide
	// since-delta count, but steer the agent to read the triggering thread
	// first. Warm resumed path with no new comments: the trigger is already
	// injected, so don't force a duplicate thread read. Cold path: read the
	// triggering thread, not the flat timeline. Final fallback (no trigger id,
	// shouldn't happen here): plain read.
	if hint := execenv.BuildNewCommentsHint(task.IssueID, task.TriggerCommentID, task.NewCommentsSince, task.NewCommentCount); hint != "" {
		b.WriteString(hint)
	} else if task.PriorSessionID != "" {
		b.WriteString(execenv.BuildResumedCommentsHint(task.IssueID, task.TriggerCommentID))
	} else if cold := execenv.BuildColdCommentsHint(task.IssueID, task.TriggerCommentID); cold != "" {
		b.WriteString(cold)
	} else {
		fmt.Fprintf(&b, "Read the discussion: `multica issue comment list %s --output json` (long issue? use `--recent 20`).\n\n", task.IssueID)
	}
	b.WriteString(execenv.BuildCommentReplyInstructions(provider, task.IssueID, task.TriggerCommentID))
	return b.String()
}

// buildChatPrompt constructs a prompt for interactive chat tasks.
func buildChatPrompt(task Task) string {
	var b strings.Builder
	b.WriteString("You are running as a chat assistant for a Multica workspace.\n")
	b.WriteString("A user is chatting with you directly. Respond to their message.\n\n")
	if task.Agent != nil && len(task.Agent.Skills) > 0 {
		refs := ExtractSlashSkills(task.ChatMessage)
		if len(refs) > 0 {
			agentSkills := make(map[string]string, len(task.Agent.Skills))
			for _, s := range task.Agent.Skills {
				agentSkills[s.ID] = s.Name
			}

			selected := make([]string, 0, len(refs))
			seen := make(map[string]struct{}, len(refs))
			for _, ref := range refs {
				name, ok := agentSkills[ref.ID]
				if !ok {
					continue
				}
				if _, ok := seen[ref.ID]; ok {
					continue
				}
				seen[ref.ID] = struct{}{}
				selected = append(selected, name)
			}

			if len(selected) > 0 {
				b.WriteString("Explicitly selected skills:\n")
				for _, name := range selected {
					fmt.Fprintf(&b, "- %s\n", name)
				}
				b.WriteString("\n")
			}
		}
	}
	fmt.Fprintf(&b, "User message:\n%s\n", task.ChatMessage)
	// List attachments by id + filename so the agent can fetch them via
	// the CLI. We deliberately do NOT inline the URL: chat attachments
	// live behind a signed CDN with a short TTL, so by the time the agent
	// has finished thinking the URL embedded in the markdown body may
	// have expired. `multica attachment download <id>` re-signs at click
	// time and is the only reliable path.
	if len(task.ChatMessageAttachments) > 0 {
		b.WriteString("\nAttachments on this message:\n")
		for _, a := range task.ChatMessageAttachments {
			if a.ContentType != "" {
				fmt.Fprintf(&b, "- id=%s filename=%q content_type=%s\n", a.ID, a.Filename, a.ContentType)
			} else {
				fmt.Fprintf(&b, "- id=%s filename=%q\n", a.ID, a.Filename)
			}
		}
		b.WriteString("Use `multica attachment download <id>` to fetch each file locally before referring to it.\n")
	}
	return b.String()
}

// buildAutopilotPrompt constructs a prompt for run_only autopilot tasks.
func buildAutopilotPrompt(task Task) string {
	var b strings.Builder
	b.WriteString("You are running as a local coding agent for a Multica workspace.\n\n")
	b.WriteString("This task was triggered by an Autopilot in run-only mode. There is no assigned Multica issue for this run.\n\n")
	fmt.Fprintf(&b, "Autopilot run ID: %s\n", task.AutopilotRunID)
	if task.AutopilotID != "" {
		fmt.Fprintf(&b, "Autopilot ID: %s\n", task.AutopilotID)
	}
	if task.AutopilotTitle != "" {
		fmt.Fprintf(&b, "Autopilot title: %s\n", task.AutopilotTitle)
	}
	if task.AutopilotSource != "" {
		fmt.Fprintf(&b, "Trigger source: %s\n", task.AutopilotSource)
	}
	if strings.TrimSpace(string(task.AutopilotTriggerPayload)) != "" {
		fmt.Fprintf(&b, "Trigger payload:\n%s\n", strings.TrimSpace(string(task.AutopilotTriggerPayload)))
	}
	b.WriteString("\nAutopilot instructions:\n")
	if strings.TrimSpace(task.AutopilotDescription) != "" {
		b.WriteString(task.AutopilotDescription)
		b.WriteString("\n\n")
	} else if task.AutopilotTitle != "" {
		fmt.Fprintf(&b, "%s\n\n", task.AutopilotTitle)
	} else {
		b.WriteString("No additional autopilot instructions were provided. Inspect the autopilot configuration before proceeding.\n\n")
	}
	if task.AutopilotID != "" {
		fmt.Fprintf(&b, "Start by running `multica autopilot get %s --output json` if you need the full autopilot configuration, then complete the instructions above.\n", task.AutopilotID)
	} else {
		b.WriteString("Complete the instructions above.\n")
	}
	b.WriteString("Do not run `multica issue get`; this run does not have an issue ID.\n")
	return b.String()
}
