package service

import (
	"strings"
	"testing"
)

func TestWorkingOnIssuesSkillKeepsCoreSmallAndDetailsLoadable(t *testing.T) {
	const (
		detailsPath       = "references/pr-and-status-details.md"
		maxBodyLines      = 260
		maxBodyBytes      = 13_000
		maxReferenceLines = 190
		maxReferenceBytes = 9_500
	)

	skill, ok := findSkill(t, "multica-working-on-issues")
	if !ok {
		return
	}
	_, body, _ := splitFrontmatter(skill.Content)
	details := supportingFileContent(t, skill, detailsPath)

	assertTextBudget(t, "SKILL.md body", body, maxBodyLines, maxBodyBytes)
	assertTextBudget(t, detailsPath, details, maxReferenceLines, maxReferenceBytes)

	for _, want := range []string{
		"Default for code-changing issue work",
		"open or update a PR before posting the final Multica issue comment",
		"if no code changed, say no PR is needed",
		"user explicitly asked for a local-only change or no PR",
		"report that blocker instead of pretending the run is complete",
		"include the PR URL when a PR exists",
		"references/pr-and-status-details.md",
		"references/working-on-issues-source-map.md",
		"MULTICA_ISSUE_OUTCOME_FILE",
		"completion contract for the final status and delivery",
		"do not duplicate it with a CLI comment",
		"available for deliberate state changes",
		"A successful process exit alone does not",
		"establish review readiness",
	} {
		if !strings.Contains(body, want) {
			t.Errorf("compact working-on-issues body missing core contract or load link %q", want)
		}
	}

	joint := body + "\n" + details
	for _, want := range []string{
		"title, body, OR branch",
		"title or body only",
		"never the branch",
		"reference_only",
		"excluded from `multica issue pull-requests`",
		"multica issue pull-requests <issue-id> --output json",
		"snapshot_available == true",
		"Only then does `checks_rollup == null` mean \"no checks\"",
		"There is no separate `draft` or `merged` boolean",
		"`backlog` parks an agent-assigned issue",
		"`cancelled` enqueue nothing",
		"no active task or retry remains",
		"custom terminal status can still enqueue at creation",
		"does **not** stop tasks already in flight",
		"not `StartTask` / `CompleteTask` side effects",
		"delivered the issue's own ask",
		"produces none of the issue's own deliverable",
		"dispatching members is not delivery",
		"`todo` starts work now, `backlog` parks it",
		"when a whole stage finishes",
		"one implicit stage",
		"Advancement is agent-driven",
		"multica issue status <stage-2-child-id> todo",
		"status_category",
	} {
		if !strings.Contains(joint, want) {
			t.Errorf("working-on-issues body plus details missing contract %q", want)
		}
	}
}

func supportingFileContent(t *testing.T, skill AgentSkillData, path string) string {
	t.Helper()
	for _, file := range skill.Files {
		if file.Path == path {
			return file.Content
		}
	}
	t.Fatalf("built-in skill %q missing supporting file %q", skill.Name, path)
	return ""
}

func assertTextBudget(t *testing.T, name, content string, maxLines, maxBytes int) {
	t.Helper()
	lines := strings.Count(content, "\n") + 1
	if lines > maxLines {
		t.Errorf("%s is %d lines, over %d-line loading budget", name, lines, maxLines)
	}
	if bytes := len([]byte(content)); bytes > maxBytes {
		t.Errorf("%s is %d bytes, over %d-byte loading budget", name, bytes, maxBytes)
	}
}
