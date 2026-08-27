package handler

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
)

// Coverage links are the relation that lets a requirement report whether it was
// tested and a case say what it was written for. Before them the testing
// surface touched issues only through `test_run_case.defect_issue_id`, which
// records a bug coming OUT of one execution and answers neither question.

func linkCaseIssues(t *testing.T, ref string, issueIDs []string) *httptest.ResponseRecorder {
	t.Helper()
	w := httptest.NewRecorder()
	req := withURLParam(
		newRequest("POST", "/api/test-cases/"+ref+"/issues?workspace_id="+testWorkspaceID,
			map[string]any{"issue_ids": issueIDs}),
		"ref", ref,
	)
	testHandler.LinkTestCaseIssues(w, req)
	return w
}

func listCaseIssues(t *testing.T, ref string) []TestCaseIssueLinkResponse {
	t.Helper()
	w := httptest.NewRecorder()
	req := withURLParam(
		newRequest("GET", "/api/test-cases/"+ref+"/issues?workspace_id="+testWorkspaceID, nil),
		"ref", ref,
	)
	testHandler.ListTestCaseIssues(w, req)
	if w.Code != http.StatusOK {
		t.Fatalf("listCaseIssues: got %d, want 200: %s", w.Code, w.Body.String())
	}
	var out struct {
		Issues []TestCaseIssueLinkResponse `json:"issues"`
	}
	if err := json.NewDecoder(w.Body).Decode(&out); err != nil {
		t.Fatalf("listCaseIssues decode: %v", err)
	}
	return out.Issues
}

func listIssueCases(t *testing.T, issueID string) []IssueTestCaseLinkResponse {
	t.Helper()
	w := httptest.NewRecorder()
	req := withURLParam(
		newRequest("GET", "/api/issues/"+issueID+"/test-cases?workspace_id="+testWorkspaceID, nil),
		"id", issueID,
	)
	testHandler.ListIssueTestCases(w, req)
	if w.Code != http.StatusOK {
		t.Fatalf("listIssueCases: got %d, want 200: %s", w.Code, w.Body.String())
	}
	var out struct {
		Cases []IssueTestCaseLinkResponse `json:"cases"`
	}
	if err := json.NewDecoder(w.Body).Decode(&out); err != nil {
		t.Fatalf("listIssueCases decode: %v", err)
	}
	return out.Cases
}

func TestLinkTestCaseIssuesReadsBackFromBothDirections(t *testing.T) {
	projectID := newTestRunProject(t)
	tc := createTestCaseForRun(t, projectID)
	issueID := dbfx.Issue(t, "covered requirement")

	if w := linkCaseIssues(t, tc.Key, []string{issueID}); w.Code != http.StatusOK {
		t.Fatalf("link: got %d, want 200: %s", w.Code, w.Body.String())
	}

	// Case -> issues, resolved for display rather than as a bare id.
	issues := listCaseIssues(t, tc.Key)
	if len(issues) != 1 {
		t.Fatalf("linked issues = %d, want 1", len(issues))
	}
	if issues[0].IssueID != issueID {
		t.Errorf("issue_id = %q, want %q", issues[0].IssueID, issueID)
	}
	if issues[0].IssueTitle != "covered requirement" {
		t.Errorf("issue_title = %q, want the issue's real title", issues[0].IssueTitle)
	}
	if issues[0].IssueIdentifier == "" || issues[0].IssueIdentifier == "-0" {
		t.Errorf("issue_identifier = %q, want a real prefixed identifier", issues[0].IssueIdentifier)
	}
	if issues[0].Origin != "human" {
		t.Errorf("origin = %q, want human for a hand-drawn link", issues[0].Origin)
	}

	// Issue -> cases, the direction the task card needs.
	cases := listIssueCases(t, issueID)
	if len(cases) != 1 {
		t.Fatalf("covering cases = %d, want 1", len(cases))
	}
	if cases[0].TestCaseID != tc.ID {
		t.Errorf("test_case_id = %q, want %q", cases[0].TestCaseID, tc.ID)
	}
	if cases[0].CaseKey != tc.Key {
		t.Errorf("case_key = %q, want %q", cases[0].CaseKey, tc.Key)
	}
	// Never executed is null, not "pending": "pending" would claim the case is
	// queued in a round it was never added to.
	if cases[0].LatestResult != nil {
		t.Errorf("latest_result = %v, want null for a case that never ran", *cases[0].LatestResult)
	}
}

func TestLinkTestCaseIssuesIsIdempotent(t *testing.T) {
	projectID := newTestRunProject(t)
	tc := createTestCaseForRun(t, projectID)
	issueID := dbfx.Issue(t, "twice-linked requirement")

	for i := 0; i < 2; i++ {
		if w := linkCaseIssues(t, tc.Key, []string{issueID}); w.Code != http.StatusOK {
			t.Fatalf("link %d: got %d, want 200: %s", i, w.Code, w.Body.String())
		}
	}
	if issues := listCaseIssues(t, tc.Key); len(issues) != 1 {
		t.Fatalf("linked issues after two identical links = %d, want 1", len(issues))
	}
}

// No foreign key backs this table, so an unverified id would create a link to
// nothing that only surfaces as a row the join silently drops.
func TestLinkTestCaseIssuesRejectsAnUnknownIssue(t *testing.T) {
	projectID := newTestRunProject(t)
	tc := createTestCaseForRun(t, projectID)

	w := linkCaseIssues(t, tc.Key, []string{"00000000-0000-0000-0000-0000000000ff"})
	if w.Code != http.StatusBadRequest {
		t.Fatalf("link to a nonexistent issue: got %d, want 400: %s", w.Code, w.Body.String())
	}
	if issues := listCaseIssues(t, tc.Key); len(issues) != 0 {
		t.Fatalf("a rejected link still wrote %d rows", len(issues))
	}
}

func TestUnlinkTestCaseIssueRemovesOnlyThatLink(t *testing.T) {
	projectID := newTestRunProject(t)
	tc := createTestCaseForRun(t, projectID)
	keep := dbfx.Issue(t, "kept requirement")
	drop := dbfx.Issue(t, "dropped requirement")

	if w := linkCaseIssues(t, tc.Key, []string{keep, drop}); w.Code != http.StatusOK {
		t.Fatalf("link: got %d, want 200: %s", w.Code, w.Body.String())
	}

	w := httptest.NewRecorder()
	req := withURLParams(
		newRequest("DELETE", "/api/test-cases/"+tc.Key+"/issues/"+drop+"?workspace_id="+testWorkspaceID, nil),
		"ref", tc.Key, "issueId", drop,
	)
	testHandler.UnlinkTestCaseIssue(w, req)
	if w.Code != http.StatusOK {
		t.Fatalf("unlink: got %d, want 200: %s", w.Code, w.Body.String())
	}

	issues := listCaseIssues(t, tc.Key)
	if len(issues) != 1 || issues[0].IssueID != keep {
		t.Fatalf("after unlink the case covers %d issues, want only the kept one", len(issues))
	}
}

// The link table has no cascade, so both deletes sweep it explicitly. A
// surviving link would keep a dead id on the other side's list forever.
func TestDeletingATestCaseSweepsItsCoverageLinks(t *testing.T) {
	projectID := newTestRunProject(t)
	tc := createTestCaseForRun(t, projectID)
	issueID := dbfx.Issue(t, "requirement outliving its case")

	if w := linkCaseIssues(t, tc.Key, []string{issueID}); w.Code != http.StatusOK {
		t.Fatalf("link: got %d, want 200: %s", w.Code, w.Body.String())
	}

	w := httptest.NewRecorder()
	req := withURLParam(
		newRequest("DELETE", "/api/test-cases/"+tc.Key+"?workspace_id="+testWorkspaceID, nil),
		"ref", tc.Key,
	)
	testHandler.DeleteTestCase(w, req)
	if w.Code != http.StatusOK {
		t.Fatalf("delete case: got %d, want 200: %s", w.Code, w.Body.String())
	}

	if cases := listIssueCases(t, issueID); len(cases) != 0 {
		t.Fatalf("issue still lists %d covering cases after the case was deleted", len(cases))
	}
	var remaining int
	if err := testPool.QueryRow(context.Background(),
		`SELECT count(*) FROM test_case_issue WHERE test_case_id = $1`, tc.ID,
	).Scan(&remaining); err != nil {
		t.Fatalf("count links: %v", err)
	}
	if remaining != 0 {
		t.Errorf("%d orphan link rows survived the case delete", remaining)
	}
}

func TestDeletingAnIssueSweepsItsCoverageLinks(t *testing.T) {
	projectID := newTestRunProject(t)
	tc := createTestCaseForRun(t, projectID)
	issueID := dbfx.Issue(t, "requirement to delete")

	if w := linkCaseIssues(t, tc.Key, []string{issueID}); w.Code != http.StatusOK {
		t.Fatalf("link: got %d, want 200: %s", w.Code, w.Body.String())
	}

	w := httptest.NewRecorder()
	req := withURLParam(
		newRequest("DELETE", "/api/issues/"+issueID+"?workspace_id="+testWorkspaceID, nil),
		"id", issueID,
	)
	testHandler.DeleteIssue(w, req)
	if w.Code != http.StatusNoContent {
		t.Fatalf("delete issue: got %d, want 204: %s", w.Code, w.Body.String())
	}

	if issues := listCaseIssues(t, tc.Key); len(issues) != 0 {
		t.Fatalf("case still covers %d issues after the issue was deleted", len(issues))
	}
}

// The coverage block's whole point is showing whether the linked cases pass,
// so the latest recorded outcome has to travel with the link.
func TestIssueCoverageCarriesTheLatestRecordedResult(t *testing.T) {
	projectID := newTestRunProject(t)
	tc := createTestCaseForRun(t, projectID)
	issueID := dbfx.Issue(t, "requirement with a run")

	if w := linkCaseIssues(t, tc.Key, []string{issueID}); w.Code != http.StatusOK {
		t.Fatalf("link: got %d, want 200: %s", w.Code, w.Body.String())
	}

	run := createTestRunFromCases(t, "Coverage run", []string{tc.ID})
	runCases := listRunCases(t, run.ID)
	setRunCaseResult(t, runCases[0].ID, "failed")

	cases := listIssueCases(t, issueID)
	if len(cases) != 1 {
		t.Fatalf("covering cases = %d, want 1", len(cases))
	}
	if cases[0].LatestResult == nil || *cases[0].LatestResult != "failed" {
		t.Fatalf("latest_result = %v, want failed", cases[0].LatestResult)
	}
	if cases[0].LatestExecutedAt == nil {
		t.Error("latest_executed_at is nil for a case that has a recorded result")
	}

	// A later round supersedes the earlier verdict.
	rerun := createTestRunFromCases(t, "Coverage rerun", []string{tc.ID})
	rerunCases := listRunCases(t, rerun.ID)
	setRunCaseResult(t, rerunCases[0].ID, "passed")

	cases = listIssueCases(t, issueID)
	if cases[0].LatestResult == nil || *cases[0].LatestResult != "passed" {
		t.Fatalf("latest_result after rerun = %v, want passed", cases[0].LatestResult)
	}
}
