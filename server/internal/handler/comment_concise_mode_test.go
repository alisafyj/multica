package handler

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"testing"
)

// TestCreateComment_ConciseModeLandsOnQueuedTask pins the comment concise-mode
// contract end-to-end: a CreateComment carrying concise_mode=true must enqueue
// its triggered run with agent_task_queue.concise_mode = true, while the same
// comment without the flag keeps the column false (SY-326 comment path).
func TestCreateComment_ConciseModeLandsOnQueuedTask(t *testing.T) {
	if testHandler == nil || testPool == nil {
		t.Skip("database not available")
	}

	queuedConciseFor := func(t *testing.T, issueID string) (count int, conciseTrue int) {
		t.Helper()
		if err := testPool.QueryRow(context.Background(), `
			SELECT COUNT(*), COUNT(*) FILTER (WHERE concise_mode)
			FROM agent_task_queue
			WHERE issue_id = $1 AND status = 'queued'
		`, issueID).Scan(&count, &conciseTrue); err != nil {
			t.Fatalf("count queued tasks: %v", err)
		}
		return count, conciseTrue
	}

	assertQueuedTaskConcise := func(t *testing.T, issueID string, wantConcise bool) {
		t.Helper()
		count, conciseTrue := queuedConciseFor(t, issueID)
		if count != 1 {
			t.Fatalf("issue %s: expected exactly 1 queued task, got %d", issueID, count)
		}
		want := 0
		if wantConcise {
			want = 1
		}
		if conciseTrue != want {
			t.Fatalf("issue %s: queued task concise_mode true count = %d, want %d", issueID, conciseTrue, want)
		}
	}

	// An agent-assignee issue: every member comment on it triggers a run.
	newAgentIssue := func(t *testing.T, title string) string {
		t.Helper()
		agentID := createHandlerTestAgent(t, "Concise Comment Agent "+title, nil)
		return createCommentTriggerPreviewIssue(t, title, "agent", agentID)
	}

	// concise_mode=true → the queued run opts into the lightweight prompt.
	conciseIssue := newAgentIssue(t, "comment concise on")
	w := httptest.NewRecorder()
	r := withURLParam(newRequest(http.MethodPost, "/api/issues/"+conciseIssue+"/comments", map[string]any{
		"content":      "run this one light",
		"concise_mode": true,
	}), "id", conciseIssue)
	testHandler.CreateComment(w, r)
	if w.Code != http.StatusCreated {
		t.Fatalf("CreateComment (concise): expected 201, got %d: %s", w.Code, w.Body.String())
	}
	var conciseResp CommentResponse
	if err := json.NewDecoder(w.Body).Decode(&conciseResp); err != nil {
		t.Fatalf("decode comment: %v", err)
	}
	assertQueuedTaskConcise(t, conciseIssue, true)

	// No flag → the standard workflow prompt (column stays false).
	standardIssue := newAgentIssue(t, "comment concise off")
	w = httptest.NewRecorder()
	r = withURLParam(newRequest(http.MethodPost, "/api/issues/"+standardIssue+"/comments", map[string]any{
		"content": "run this one normally",
	}), "id", standardIssue)
	testHandler.CreateComment(w, r)
	if w.Code != http.StatusCreated {
		t.Fatalf("CreateComment (standard): expected 201, got %d: %s", w.Code, w.Body.String())
	}
	assertQueuedTaskConcise(t, standardIssue, false)

	// Explicit false on the wire behaves like omitted: never opts in.
	explicitFalseIssue := newAgentIssue(t, "comment concise explicit false")
	w = httptest.NewRecorder()
	r = withURLParam(newRequest(http.MethodPost, "/api/issues/"+explicitFalseIssue+"/comments", map[string]any{
		"content":      "explicit false stays standard",
		"concise_mode": false,
	}), "id", explicitFalseIssue)
	testHandler.CreateComment(w, r)
	if w.Code != http.StatusCreated {
		t.Fatalf("CreateComment (explicit false): expected 201, got %d: %s", w.Code, w.Body.String())
	}
	assertQueuedTaskConcise(t, explicitFalseIssue, false)
}

// TestCreateComment_ConciseModeMentionPath pins that the flag also reaches a
// @mention-triggered run (EnqueueTaskForMentionWithMode), not just the
// issue-assignee path.
func TestCreateComment_ConciseModeMentionPath(t *testing.T) {
	if testHandler == nil || testPool == nil {
		t.Skip("database not available")
	}

	agentID := createHandlerTestAgent(t, "Concise Mention Agent", nil)
	issueID := createCommentTriggerPreviewIssue(t, "comment concise mention", "", "")
	content := fmt.Sprintf("[@Agent](mention://agent/%s) quick check please", agentID)

	w := httptest.NewRecorder()
	r := withURLParam(newRequest(http.MethodPost, "/api/issues/"+issueID+"/comments", map[string]any{
		"content":      content,
		"concise_mode": true,
	}), "id", issueID)
	testHandler.CreateComment(w, r)
	if w.Code != http.StatusCreated {
		t.Fatalf("CreateComment (mention concise): expected 201, got %d: %s", w.Code, w.Body.String())
	}

	var count, conciseTrue int
	if err := testPool.QueryRow(context.Background(), `
		SELECT COUNT(*), COUNT(*) FILTER (WHERE concise_mode)
		FROM agent_task_queue
		WHERE issue_id = $1 AND agent_id = $2 AND status = 'queued'
	`, issueID, agentID).Scan(&count, &conciseTrue); err != nil {
		t.Fatalf("count mention queued tasks: %v", err)
	}
	if count != 1 || conciseTrue != 1 {
		t.Fatalf("mention path: queued=%d concise=%d, want 1/1", count, conciseTrue)
	}
}
