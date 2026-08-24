package handler

import (
	"context"
	"encoding/json"
	"net/http"
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
