package handler

import (
	"context"
	"encoding/json"
	"net/http"
	"strings"
	"testing"

	"github.com/multica-ai/multica/server/internal/testutil"
)

// companionIssueOf reads the issue a document points at, and whether one exists
// at all. A design document's issue link is optional, so "no row" is a legitimate
// answer rather than a failure.
func companionIssueOf(t *testing.T, documentID string) (issueID string, title string, assigneeID string, found bool) {
	t.Helper()
	rows, err := testPool.Query(context.Background(), `
		SELECT i.id::text, i.title, coalesce(i.assignee_id::text, '')
		FROM design_document d
		JOIN issue i ON i.id = d.issue_id
		WHERE d.id = $1
	`, parseUUID(documentID))
	if err != nil {
		t.Fatalf("read companion issue: %v", err)
	}
	defer rows.Close()
	if !rows.Next() {
		return "", "", "", false
	}
	if err := rows.Scan(&issueID, &title, &assigneeID); err != nil {
		t.Fatalf("scan companion issue: %v", err)
	}
	return issueID, title, assigneeID, true
}

func createDesignDocumentForCompanionTest(t *testing.T, body map[string]any) DesignDocumentResponse {
	t.Helper()
	response := performProjectDesignSystemRequest(t, testHandler.CreateDesignDocument, http.MethodPost, "/api/design-documents", body)
	if response.Code != http.StatusCreated {
		t.Fatalf("CreateDesignDocument: status = %d, body = %s", response.Code, response.Body.String())
	}
	var created DesignDocumentResponse
	if err := json.NewDecoder(response.Body).Decode(&created); err != nil {
		t.Fatalf("decode create response: %v", err)
	}
	t.Cleanup(func() {
		ctx := context.Background()
		var issueID string
		_ = testPool.QueryRow(ctx, `SELECT coalesce(issue_id::text, '') FROM design_document WHERE id = $1`,
			parseUUID(created.ID)).Scan(&issueID)
		_, _ = testPool.Exec(ctx, `DELETE FROM design_document WHERE id = $1`, parseUUID(created.ID))
		if issueID != "" {
			_, _ = testPool.Exec(ctx, `DELETE FROM issue WHERE id = $1`, parseUUID(issueID))
		}
	})
	return created
}

// The launcher can open a task card next to the run: the issue carries the
// document's title, is assigned to the agent that will do the work, and the
// document points back at it so the two are traceable in both directions.
func TestCreateDesignDocumentOpensACompanionIssueWhenAsked(t *testing.T) {
	projectID := createProjectDesignSystemProject(t, testWorkspaceID, "Companion issue project")
	agentID, _ := createProjectDesignSystemAgent(t, "online")

	created := createDesignDocumentForCompanionTest(t, map[string]any{
		"project_id":   projectID,
		"agent_id":     agentID,
		"platform":     "web",
		"brief":        "客户列表页，支持筛选与批量操作。",
		"title":        "客户列表页",
		"create_issue": true,
	})

	issueID, title, assigneeID, found := companionIssueOf(t, created.ID)
	if !found {
		t.Fatal("create_issue=true produced no companion issue")
	}
	if issueID == "" || title != "客户列表页" {
		t.Fatalf("companion issue = (%q, %q), want the document's title", issueID, title)
	}
	if assigneeID != agentID {
		t.Fatalf("companion issue assignee = %q, want the design agent %q", assigneeID, agentID)
	}

	// This is the bug that actually wedged a user's design task: assigning an
	// agent at create time is normally an instruction to run, so creating the
	// companion issue this way ALSO auto-enqueued an independent task racing
	// the design task itself for the same local directory. The companion is a
	// trace of who owns the work, not a second dispatch.
	var taskCount int
	if err := testPool.QueryRow(context.Background(),
		`SELECT count(*) FROM agent_task_queue WHERE issue_id = $1`, parseUUID(issueID)).Scan(&taskCount); err != nil {
		t.Fatalf("count tasks on the companion issue: %v", err)
	}
	if taskCount != 0 {
		t.Fatalf("companion issue has %d queued tasks, want none — creating it must not also start a run", taskCount)
	}
}

// Absent or false leaves the tasks page alone. A design run that leaves no
// trace there is a legitimate way to work (DC-045 keeps the link optional).
func TestCreateDesignDocumentLeavesNoIssueWhenNotAsked(t *testing.T) {
	projectID := createProjectDesignSystemProject(t, testWorkspaceID, "No companion issue project")
	agentID, _ := createProjectDesignSystemAgent(t, "online")

	for name, body := range map[string]map[string]any{
		"omitted": {"project_id": projectID, "agent_id": agentID, "platform": "web", "brief": "登录页。"},
		"false":   {"project_id": projectID, "agent_id": agentID, "platform": "web", "brief": "登录页。", "create_issue": false},
	} {
		t.Run(name, func(t *testing.T) {
			created := createDesignDocumentForCompanionTest(t, body)
			if _, _, _, found := companionIssueOf(t, created.ID); found {
				t.Fatal("a companion issue was created without being asked for")
			}
		})
	}
}

// Naming an existing issue already links the document to it. Creating a second
// one would split the trail, so the flag is ignored rather than additive.
func TestCreateDesignDocumentPrefersTheNamedIssueOverACompanion(t *testing.T) {
	projectID := createProjectDesignSystemProject(t, testWorkspaceID, "Named issue project")
	agentID, _ := createProjectDesignSystemAgent(t, "online")
	namedIssueID := dbfx.Issue(t, "Existing design task", testutil.Cols{"project_id": projectID})

	created := createDesignDocumentForCompanionTest(t, map[string]any{
		"project_id":   projectID,
		"agent_id":     agentID,
		"platform":     "web",
		"brief":        "设置页。",
		"issue_id":     namedIssueID,
		"create_issue": true,
	})

	issueID, _, _, found := companionIssueOf(t, created.ID)
	if !found || issueID != namedIssueID {
		t.Fatalf("document issue = (%q, %v), want the named issue %q", issueID, found, namedIssueID)
	}
}

// The launcher sends no title, so the server names the design. It used to fall
// back to the project's name, which gave every document in a project the same
// title — and, once a companion task carried it too, made the SECOND launch in
// a project collide with the first one's task and fail outright.
func TestDesignDocumentTitleComesFromTheBrief(t *testing.T) {
	long := strings.Repeat("订单", 40)
	for name, tc := range map[string]struct{ brief, fallback, want string }{
		"first line names it": {
			brief:    "客户列表页，支持筛选与批量操作。\n第二段说明不参与命名。",
			fallback: "某个项目",
			want:     "客户列表页，支持筛选与批量操作。",
		},
		"single line":                     {brief: "登录页", fallback: "某个项目", want: "登录页"},
		"blank falls back to the project": {brief: "   \n  ", fallback: "某个项目", want: "某个项目"},
		"long brief is cut on a rune boundary": {
			brief:    long,
			fallback: "某个项目",
			want:     string([]rune(long)[:60]) + "…",
		},
	} {
		t.Run(name, func(t *testing.T) {
			if got := designDocumentTitleFromBrief(tc.brief, tc.fallback); got != tc.want {
				t.Fatalf("designDocumentTitleFromBrief() = %q, want %q", got, tc.want)
			}
		})
	}
}

// The bug a user hit: launching a second design in a project reported only
// "failed to create the companion task" and ran no design at all, because the
// active-duplicate guard saw the first launch's task. Two runs are two pieces
// of work, so both get a card.
func TestCreateDesignDocumentOpensACompanionForEveryLaunch(t *testing.T) {
	projectID := createProjectDesignSystemProject(t, testWorkspaceID, "Repeat launch project")
	agentID, _ := createProjectDesignSystemAgent(t, "online")

	body := map[string]any{
		"project_id":   projectID,
		"agent_id":     agentID,
		"platform":     "web",
		"brief":        "同一句话发起两次。",
		"create_issue": true,
	}
	first := createDesignDocumentForCompanionTest(t, body)
	second := createDesignDocumentForCompanionTest(t, body)

	firstIssue, firstTitle, _, firstFound := companionIssueOf(t, first.ID)
	secondIssue, secondTitle, _, secondFound := companionIssueOf(t, second.ID)
	if !firstFound || !secondFound {
		t.Fatalf("companion issues found = (%v, %v), want both", firstFound, secondFound)
	}
	if firstIssue == secondIssue {
		t.Fatal("both documents point at one task — the second launch reused the first one's card")
	}
	// Named from the brief, not from the project, on both runs.
	if firstTitle != "同一句话发起两次。" || secondTitle != "同一句话发起两次。" {
		t.Fatalf("companion titles = (%q, %q), want the brief's first line", firstTitle, secondTitle)
	}
}
