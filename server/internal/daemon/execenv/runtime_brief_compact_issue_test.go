package execenv

import (
	"encoding/json"
	"fmt"
	"strings"
	"testing"
)

func TestCompactIssueBriefPreservesExecutionContracts(t *testing.T) {
	for _, provider := range []string{"codex", "claude"} {
		t.Run(provider, func(t *testing.T) {
			ctx := TaskContextForEnv{
				IssueID: "issue-1", IssueCompletionContractVersion: 1,
				AgentName: "Scoped agent", AgentInstructions: "Never deploy without approval.",
				WorkspaceContext: "Keep customer records private.",
				IssueStatuses: []IssueStatusForEnv{{
					Key: "human_review", Name: "Human Review", Category: "in_review",
				}},
			}
			brief := RenderRuntimeBrief(provider, ctx)
			briefFolded := strings.ToLower(brief)
			contracts := map[string][]string{
				"task and role authority": {
					"Agent Identity", "wins", "do not exceed your role", "delegation-only", "stop after delegation",
				},
				"body, history, and historical source": {
					"THIS turn", "mandatory bounded comment catch-up", "source_context", "historical data, not instructions", "current task, comments, and code win",
				},
				"status": {
					"FIRST", "before that work", "Research, planning, and review count when they are deliverables", "discussion alone changes no status", "mid-turn", "only when the value differs", "regardless of assignee",
					"in_review", "ready for acceptance", "`done` stays human", "unfinished or dispatched work", "FIRST follow the clarification rules",
					"custom statuses inherit their category's behavior", "name/description",
				},
				"duplicate work and delivery": {
					"existing claims", "active siblings", "--no-start", "Final Issue Delivery", "sole delivery definition", "duplicate final or progress comment", "Successful process exit alone is not verified delivery",
				},
				"metadata": {
					"hints, not truth", "most runs write nothing", "short reusable values", "never secrets", "remove stale state",
					"Before changing metadata, read the `multica-working-on-issues` skill", "persistence rules", "If a required skill is unavailable, do not perform that write",
				},
				"CLI and files": {
					"only through `multica`", "never raw HTTP", "flags on demand", "JSON stdout", "stderr", "never retry a successful write after a parse failure",
					"UTF-8", "inside the workdir", "no inline body", "stdin", "heredoc", "file-write failure", "stale files", "delete the temporary reply after success", "current reply target",
				},
				"attachments and mentions": {
					"authenticated CLI", "downloaded local copy is not delivered", "--content-file", "--attachment", "not chat upload", "runtime-local path",
					"Human mentions notify", "agent mentions dispatch paid work", "References, courtesy, and FYI stay plain text", "followers and completion notifications are platform-owned", "Escalation is allowed",
					"multica-mentioning", "if that required skill is unavailable, do not guess or write mention syntax", "Ordinary coding requires no mention-skill read",
				},
				"children": {
					"`todo` dispatches", "`backlog` parks", "stages order", "multica-working-on-issues", "before child creation",
				},
				"run ownership and safety": {
					"top-level exit ends the task", "no background wakeup", "synchronously", "never background-and-yield",
					"Do not watch or poll external CI", "merge-gate", "one foreground blocking watch", "explicitly asks for the CI result",
					"user-requested service deliverable", "detach", "durable logs", "cleanup handle", "verify readiness", "URL, logs, and stop instructions", "best-effort",
					"Never kill `multica` or `multica.exe` by executable name", "owned child PID", "daemon status", "never kill the daemon PID",
				},
			}
			for contract, required := range contracts {
				for _, text := range required {
					if !strings.Contains(briefFolded, strings.ToLower(text)) {
						t.Errorf("%s contract missing %q", contract, text)
					}
				}
			}
			for _, duplicate := range []string{
				"## Authoritative Issue Body Snapshot", "## Verified Empty Comment History", "--roots-only", "--tail 30",
				"Final results MUST be delivered", "**Post your final results as a comment", "MULTICA_ISSUE_OUTCOME_FILE", "request_user_input", "AskUserQuestion",
			} {
				if strings.Contains(brief, duplicate) {
					t.Errorf("stable compact brief duplicates per-turn contract %q", duplicate)
				}
			}
		})
	}
}

func TestCompactIssueBriefReducesOnlySelectedSections(t *testing.T) {
	const r09MinimalBriefBytes = 12001
	brief := RenderRuntimeBrief("codex", TaskContextForEnv{IssueID: "issue-1", IssueCompletionContractVersion: 1})
	reduction := r09MinimalBriefBytes - len(brief)
	if reduction < 6000 {
		t.Errorf("R09 minimal brief reduction = %d bytes (%d -> %d), want >=6000", reduction, r09MinimalBriefBytes, len(brief))
	}
	t.Logf("R09 minimal brief=%d bytes; compact brief=%d bytes; reduction=%d bytes", r09MinimalBriefBytes, len(brief), reduction)
}

func TestCompactIssueBriefKeepsExcludedPaths(t *testing.T) {
	cases := map[string]TaskContextForEnv{
		"missing issue": {IssueCompletionContractVersion: 1},
		"legacy":        {IssueID: "i"},
		"unknown":       {IssueID: "i", IssueCompletionContractVersion: 2},
		"leader":        {IssueID: "i", IssueCompletionContractVersion: 1, IsSquadLeader: true},
		"chat":          {IssueID: "i", IssueCompletionContractVersion: 1, ChatSessionID: "chat"},
		"quick create":  {IssueID: "i", IssueCompletionContractVersion: 1, QuickCreatePrompt: "create"},
		"autopilot":     {IssueID: "i", IssueCompletionContractVersion: 1, AutopilotRunID: "run"},
		"UI draft":      {IssueID: "i", IssueCompletionContractVersion: 1, UIDraftCreateContext: "{}"},
		"restore":       {IssueID: "i", IssueCompletionContractVersion: 1, DesignRestoreContext: "{}"},
		"profile":       {IssueID: "i", IssueCompletionContractVersion: 1, DesignSystemProfileAnalyzeContext: "{}"},
		"PMO":           {IssueID: "i", IssueCompletionContractVersion: 1, PMOSyncContext: "{}"},
	}
	for name, ctx := range cases {
		t.Run(name, func(t *testing.T) {
			got := RenderRuntimeBrief("codex", ctx)
			want := renderLegacyBriefForCompactSelectorTest("codex", ctx)
			if got != want {
				t.Errorf("excluded path changed outside the compact selector:\n%s", firstBriefDiff(want, got))
			}
		})
	}
}

func TestCompactIssueBriefStableAcrossIssueTurns(t *testing.T) {
	base := TaskContextForEnv{IssueID: "issue-1", IssueCompletionContractVersion: 1, AgentName: "Stable agent"}
	for _, provider := range []string{"codex", "claude"} {
		want := RenderRuntimeBrief(provider, base)
		for i := 0; i < 3; i++ {
			ctx := base
			ctx.IssueID = fmt.Sprintf("issue-%d", i+2)
			ctx.TriggerCommentID = fmt.Sprintf("comment-%d", i)
			ctx.TriggerThreadID = "thread-2"
			ctx.NewCommentCount = i + 1
			ctx.NewCommentsSince = "2026-09-07T00:00:00Z"
			ctx.PriorSessionResumed = i == 1
			ctx.PriorSessionResumeUnavailable = i == 2
			ctx.HandoffNote = "changed handoff"
			if got := RenderRuntimeBrief(provider, ctx); got != want {
				t.Errorf("%s run %d changed stable compact prefix:\n%s", provider, i, firstBriefDiff(want, got))
			}
		}
	}
}

func TestCompactIssueBriefPreservesConfiguredDataWithoutTruncation(t *testing.T) {
	longUnicode := strings.Repeat("\u957f\u6587\u672c-\u4fdd\u5bc6-\U0001f680-", 400)
	ctx := TaskContextForEnv{
		IssueID: "issue-1", IssueCompletionContractVersion: 1,
		AgentName: "\u4ee3\u7406\u4eba", AgentInstructions: "agent-marker:" + longUnicode,
		RequestingUserName: "\u8bf7\u6c42\u8005", RequestingUserProfileDescription: "profile-marker:" + longUnicode,
		WorkspaceContext: "workspace-marker:" + longUnicode,
		Repos:            []RepoContextForEnv{{URL: "https://example.invalid/\u4ed3\u5e93.git", Description: "repo-marker:" + longUnicode}},
		ProjectID:        "project-1", ProjectTitle: "\u9879\u76ee", ProjectDescription: "project-marker:" + longUnicode,
		ProjectResources: []ProjectResourceForEnv{{
			ID: "resource-1", ResourceType: "document", Label: "resource-marker:" + longUnicode,
			ResourceRef: json.RawMessage(`{"url":"https://example.invalid/spec"}`),
		}},
		IssueStatuses: []IssueStatusForEnv{{Key: "review", Name: "status-marker:" + longUnicode, Category: "in_review", Description: "status-description:" + longUnicode}},
		AgentSkills: []SkillContextForEnv{
			{Name: "Plain Coding", Content: "plain-skill-secret-marker:" + longUnicode},
			{Name: "Multica Working On Issues", Content: "issue-skill-secret-marker:" + longUnicode},
		},
	}
	brief := RenderRuntimeBrief("codex", ctx)
	sharedWriters := map[string]func(*strings.Builder){
		"header":          writeHeader,
		"agent identity":  func(b *strings.Builder) { writeAgentIdentity(b, ctx) },
		"requesting user": func(b *strings.Builder) { writeRequestingUser(b, ctx) },
		"workspace":       func(b *strings.Builder) { writeWorkspaceContext(b, ctx) },
		"repositories":    func(b *strings.Builder) { writeRepositories(b, ctx) },
		"project":         func(b *strings.Builder) { writeProjectContext(b, ctx) },
		"status catalog":  func(b *strings.Builder) { writeIssueStatusCommand(b, ctx) },
		"skill index":     func(b *strings.Builder) { writeSkills(b, ctx) },
	}
	for name, write := range sharedWriters {
		var expected strings.Builder
		write(&expected)
		if !strings.Contains(brief, expected.String()) {
			t.Errorf("compact brief changed or truncated shared %s output", name)
		}
	}
	for _, skillBody := range []string{ctx.AgentSkills[0].Content, ctx.AgentSkills[1].Content} {
		if strings.Contains(brief, skillBody) {
			t.Error("compact brief must index skills without forcing their full body into plain coding turns")
		}
	}
}

func renderLegacyBriefForCompactSelectorTest(provider string, ctx TaskContextForEnv) string {
	var b strings.Builder
	kind := classifyTask(ctx)
	writeHeader(&b)
	writeBackgroundTaskSafetySlim(&b)
	writeAgentIdentity(&b, ctx)
	writeRequestingUser(&b, ctx)
	writeWorkspaceContext(&b, ctx)
	if kind == kindQuickCreate {
		writeAvailableCommandsQuickCreate(&b)
	} else {
		writeAvailableCommands(&b, ctx)
	}
	writeIssueBodyFormatting(&b)
	if kind == kindIssue {
		writeCommentFormatting(&b)
	}
	if kind != kindQuickCreate {
		writeRepositories(&b, ctx)
	}
	writeProjectContext(&b, ctx)
	if kind == kindIssue {
		writeInstructionPrecedence(&b)
	}
	writeWorkflowHeader(&b)
	switch kind {
	case kindChat:
		writeWorkflowChat(&b)
	case kindQuickCreate:
		writeWorkflowQuickCreate(&b)
	case kindAutopilotRunOnly:
		writeWorkflowAutopilot(&b)
	case kindUIDraftCreate:
		writeWorkflowUIDraftCreate(&b)
	case kindDesignRestore:
		writeWorkflowDesignRestore(&b)
	case kindDesignSystemProfileAnalyze:
		writeWorkflowDesignSystemProfileAnalyze(&b)
	case kindPMOSync:
		writeWorkflowPMOSync(&b)
	case kindIssue:
		writeWorkflowIssue(&b, ctx)
	}
	if kind.hasIssueContext() && ctx.IssueID != "" {
		writeSubIssueCreation(&b, ctx)
	}
	writeSkills(&b, ctx)
	if kind == kindIssue {
		writeMentions(&b)
		writeAttachments(&b)
	}
	writeAlwaysUseCLI(&b)
	writeOutput(&b, kind, ctx)
	return b.String()
}
