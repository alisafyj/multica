package handler

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"strconv"
	"strings"
	"testing"
	"time"

	"github.com/go-chi/chi/v5"
	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgtype"
	"github.com/multica-ai/multica/server/internal/testutil"
)

func validPendingInputRegistration() registerTaskPendingInputRequest {
	return registerTaskPendingInputRequest{
		Version:         1,
		RequestKey:      "sha256:" + strings.Repeat("a", 64),
		Blocking:        true,
		ClaimGeneration: time.Now().UnixMicro(),
		Questions: []taskPendingInputQuestion{
			{
				ID:       "deployment",
				Header:   "Deployment target",
				Question: "Which environment should receive this change?",
				Options: []taskPendingInputOption{
					{Label: "Staging", Description: "Deploy to the staging environment."},
					{Label: "Production", Description: "Deploy to production."},
				},
			},
			{
				ID:         "notes",
				Header:     "Release notes",
				Question:   "What should the release note mention?",
				Options:    []taskPendingInputOption{},
				AllowOther: true,
			},
		},
	}
}

func TestNormalizePendingInputRequestRejectsUnsupportedShapes(t *testing.T) {
	tests := []struct {
		name   string
		mutate func(*registerTaskPendingInputRequest)
	}{
		{name: "nonblocking", mutate: func(r *registerTaskPendingInputRequest) { r.Blocking = false }},
		{name: "secret", mutate: func(r *registerTaskPendingInputRequest) { r.Questions[0].Secret = true }},
		{name: "too many questions", mutate: func(r *registerTaskPendingInputRequest) {
			r.Questions = append(r.Questions, r.Questions[0], r.Questions[0])
		}},
		{name: "duplicate question id", mutate: func(r *registerTaskPendingInputRequest) { r.Questions[1].ID = r.Questions[0].ID }},
		{name: "question id surrounding whitespace", mutate: func(r *registerTaskPendingInputRequest) { r.Questions[0].ID = " deployment" }},
		{name: "option label surrounding whitespace", mutate: func(r *registerTaskPendingInputRequest) { r.Questions[0].Options[0].Label = "Staging " }},
		{name: "nul in question", mutate: func(r *registerTaskPendingInputRequest) { r.Questions[0].Question = "Which\x00 environment?" }},
		{name: "nul in option description", mutate: func(r *registerTaskPendingInputRequest) { r.Questions[0].Options[0].Description = "Stage\x00 target" }},
		{name: "request key surrounding whitespace", mutate: func(r *registerTaskPendingInputRequest) { r.RequestKey += " " }},
		{name: "too many options", mutate: func(r *registerTaskPendingInputRequest) {
			r.Questions[0].Options = make([]taskPendingInputOption, pendingInputMaxOptions+1)
			for i := range r.Questions[0].Options {
				r.Questions[0].Options[i].Label = uuid.NewString()
			}
		}},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			request := validPendingInputRegistration()
			tc.mutate(&request)
			if _, _, err := normalizedPendingInputRequest(request); err == nil {
				t.Fatal("expected validation error")
			}
		})
	}
}

func TestValidatePendingInputAnswersRequiresExactQuestionMap(t *testing.T) {
	request := validPendingInputRegistration()
	questions, err := normalizePendingInputQuestions(request.Questions)
	if err != nil {
		t.Fatal(err)
	}
	valid := map[string]taskPendingInputAnswer{
		"deployment": {Answers: []string{"Staging"}},
		"notes":      {Answers: []string{"Mention the migration."}},
	}
	if _, _, err := validatePendingInputAnswers(questions, valid); err != nil {
		t.Fatalf("valid answers rejected: %v", err)
	}
	delete(valid, "notes")
	if _, _, err := validatePendingInputAnswers(questions, valid); err == nil {
		t.Fatal("missing question answer accepted")
	}
	valid["notes"] = taskPendingInputAnswer{Answers: []string{"note"}}
	valid["extra"] = taskPendingInputAnswer{Answers: []string{"unexpected"}}
	if _, _, err := validatePendingInputAnswers(questions, valid); err == nil {
		t.Fatal("extra question answer accepted")
	}
	valid = map[string]taskPendingInputAnswer{
		"deployment": {Answers: []string{"Unknown"}},
		"notes":      {Answers: []string{"note"}},
	}
	if _, _, err := validatePendingInputAnswers(questions, valid); err == nil {
		t.Fatal("unlisted answer accepted without allow_other")
	}
	for _, invalid := range []string{" Staging", "Staging ", "Stage\x00ing"} {
		invalidAnswers := map[string]taskPendingInputAnswer{
			"deployment": {Answers: []string{invalid}},
			"notes":      {Answers: []string{"note"}},
		}
		if _, _, err := validatePendingInputAnswers(questions, invalidAnswers); err == nil {
			t.Fatalf("invalid answer %q accepted", invalid)
		}
	}
}

func TestTaskPendingInputLifecycle(t *testing.T) {
	ctx := context.Background()
	daemonID := "pending-input-daemon-" + uuid.NewString()
	runtimeID := dbfx.Runtime(t, "Pending input runtime", testutil.Cols{"daemon_id": daemonID})
	agentID := dbfx.Agent(t, "Pending input agent", runtimeID)
	issueID := dbfx.Issue(t, "Pending input issue", testutil.Cols{"status": "in_progress", "assignee_type": "agent", "assignee_id": agentID})
	dispatchedAt := time.Now().UTC().Truncate(time.Microsecond)
	taskID := dbfx.Task(t, agentID, testutil.Cols{
		"runtime_id": runtimeID, "issue_id": issueID, "status": "running",
		"dispatched_at": dispatchedAt, "started_at": dispatchedAt,
	})
	t.Cleanup(func() {
		_, _ = testPool.Exec(ctx, `DELETE FROM task_pending_input WHERE task_id = $1`, taskID)
		_, _ = testPool.Exec(ctx, `DELETE FROM comment WHERE issue_id = $1`, issueID)
	})

	registration := validPendingInputRegistration()
	registration.ClaimGeneration = dispatchedAt.UnixMicro()
	registerRequest := pendingInputDaemonRequest(http.MethodPost,
		"/api/daemon/runtimes/"+runtimeID+"/tasks/"+taskID+"/pending-inputs",
		registration, runtimeID, taskID, "", testWorkspaceID, daemonID)
	var created taskPendingInputResponse
	testutil.Call(t, testHandler.RegisterTaskPendingInput, registerRequest).Want(http.StatusCreated).JSON(&created)
	if created.State != "open" || created.TaskID != taskID || created.IssueID != issueID || created.Answers != nil {
		t.Fatalf("created pending input = %+v", created)
	}
	var questionAuthorType string
	var questionAuthorID, questionSourceTaskID *string
	dbfx.QueryRow(t, `SELECT author_type, author_id::text, source_task_id::text FROM comment WHERE id = $1`, created.QuestionCommentID).
		Scan(&questionAuthorType, &questionAuthorID, &questionSourceTaskID)
	if questionAuthorType != "agent" || questionAuthorID == nil || *questionAuthorID != agentID || questionSourceTaskID == nil || *questionSourceTaskID != taskID {
		t.Fatalf("question comment identity = type=%q author=%v source_task=%v", questionAuthorType, questionAuthorID, questionSourceTaskID)
	}

	secondActive := registration
	secondActive.RequestKey = "sha256:" + strings.Repeat("b", 64)
	registerRequest = pendingInputDaemonRequest(http.MethodPost, registerRequest.URL.Path, secondActive, runtimeID, taskID, "", testWorkspaceID, daemonID)
	testutil.Call(t, testHandler.RegisterTaskPendingInput, registerRequest).Want(http.StatusConflict)

	var raw map[string]json.RawMessage
	registerRequest = pendingInputDaemonRequest(http.MethodPost, registerRequest.URL.Path, registration, runtimeID, taskID, "", testWorkspaceID, daemonID)
	testutil.Call(t, testHandler.RegisterTaskPendingInput, registerRequest).Want(http.StatusOK).JSON(&raw)
	for _, forbidden := range []string{
		"workspace_id", "runtime_id", "agent_id", "request_key", "request_sha256",
		"question_issue_revision", "answer_issue_revision", "answer_comment_id",
		"answered_by", "idempotency_key", "expires_at",
	} {
		if _, leaked := raw[forbidden]; leaked {
			t.Fatalf("public DTO leaked %q", forbidden)
		}
	}

	conflict := registration
	conflict.Questions = append([]taskPendingInputQuestion(nil), registration.Questions...)
	conflict.Questions[0].Question = "A different question"
	registerRequest = pendingInputDaemonRequest(http.MethodPost, registerRequest.URL.Path, conflict, runtimeID, taskID, "", testWorkspaceID, daemonID)
	testutil.Call(t, testHandler.RegisterTaskPendingInput, registerRequest).Want(http.StatusConflict)

	getRequest := pendingInputDaemonRequest(http.MethodGet,
		"/api/daemon/runtimes/"+runtimeID+"/tasks/"+taskID+"/pending-inputs/"+created.ID+"?claim_generation="+strconvFormatInt(registration.ClaimGeneration),
		nil, runtimeID, taskID, created.ID, testWorkspaceID, daemonID)
	testutil.Call(t, testHandler.GetTaskPendingInputForDaemon, getRequest).Want(http.StatusOK)

	listRequest := pendingInputIssueRequest(http.MethodGet, "/api/issues/"+issueID+"/pending-inputs", nil, issueID, "")
	var listed struct {
		Data []taskPendingInputResponse `json:"data"`
	}
	testutil.Call(t, testHandler.ListTaskPendingInputsForIssue, listRequest).Want(http.StatusOK).JSON(&listed)
	if len(listed.Data) != 1 || listed.Data[0].ID != created.ID {
		t.Fatalf("listed pending inputs = %+v", listed.Data)
	}

	idempotencyKey := uuid.NewString()
	answerBody := answerTaskPendingInputRequest{
		IdempotencyKey: idempotencyKey,
		Answers: map[string]taskPendingInputAnswer{
			"deployment": {Answers: []string{"Staging"}},
			"notes":      {Answers: []string{"Mention the migration."}},
		},
	}
	answerRequest := pendingInputIssueRequest(http.MethodPost,
		"/api/issues/"+issueID+"/pending-inputs/"+created.ID+"/answer", answerBody, issueID, created.ID)
	var answered taskPendingInputResponse
	testutil.Call(t, testHandler.AnswerTaskPendingInput, answerRequest).Want(http.StatusOK).JSON(&answered)
	if answered.State != "answered" || answered.AnsweredAt == nil || answered.Answers["deployment"].Answers[0] != "Staging" {
		t.Fatalf("answered pending input = %+v", answered)
	}
	_, canonicalAnswers, err := validatePendingInputAnswers(registration.Questions, answerBody.Answers)
	if err != nil {
		t.Fatal(err)
	}
	var storedAnswers []byte
	dbfx.QueryRow(t, `SELECT answers FROM task_pending_input WHERE id = $1`, created.ID).Scan(&storedAnswers)
	if bytes.Equal(storedAnswers, canonicalAnswers) {
		t.Fatalf("test requires PostgreSQL JSONB serialization to differ from compact JSON: %q", storedAnswers)
	}

	answerRequest = pendingInputIssueRequest(http.MethodPost, answerRequest.URL.Path, answerBody, issueID, created.ID)
	testutil.Call(t, testHandler.AnswerTaskPendingInput, answerRequest).Want(http.StatusOK)
	var replyCount, taskCount int
	dbfx.QueryRow(t, `SELECT count(*) FROM comment WHERE parent_id = $1`, created.QuestionCommentID).Scan(&replyCount)
	dbfx.QueryRow(t, `SELECT count(*) FROM agent_task_queue WHERE issue_id = $1`, issueID).Scan(&taskCount)
	if replyCount != 1 || taskCount != 1 {
		t.Fatalf("idempotent answer created replies=%d tasks=%d, want 1/1", replyCount, taskCount)
	}
	var answerAuthorType string
	var answerAuthorID, answerSourceTaskID *string
	dbfx.QueryRow(t, `SELECT author_type, author_id::text, source_task_id::text FROM comment WHERE parent_id = $1`, created.QuestionCommentID).
		Scan(&answerAuthorType, &answerAuthorID, &answerSourceTaskID)
	if answerAuthorType != "member" || answerAuthorID == nil || *answerAuthorID != testUserID || answerSourceTaskID != nil {
		t.Fatalf("answer comment identity = type=%q author=%v source_task=%v", answerAuthorType, answerAuthorID, answerSourceTaskID)
	}
	conflictingAnswer := answerBody
	conflictingAnswer.Answers = map[string]taskPendingInputAnswer{
		"deployment": {Answers: []string{"Production"}},
		"notes":      {Answers: []string{"Mention the migration."}},
	}
	answerRequest = pendingInputIssueRequest(http.MethodPost, answerRequest.URL.Path, conflictingAnswer, issueID, created.ID)
	testutil.Call(t, testHandler.AnswerTaskPendingInput, answerRequest).Want(http.StatusConflict)

	answerBody.IdempotencyKey = uuid.NewString()
	answerRequest = pendingInputIssueRequest(http.MethodPost, answerRequest.URL.Path, answerBody, issueID, created.ID)
	testutil.Call(t, testHandler.AnswerTaskPendingInput, answerRequest).Want(http.StatusConflict)

	ackBody := map[string]any{"claim_generation": registration.ClaimGeneration}
	ackRequest := pendingInputDaemonRequest(http.MethodPost,
		"/api/daemon/runtimes/"+runtimeID+"/tasks/"+taskID+"/pending-inputs/"+created.ID+"/ack",
		ackBody, runtimeID, taskID, created.ID, testWorkspaceID, daemonID)
	var acked taskPendingInputResponse
	testutil.Call(t, testHandler.AckTaskPendingInput, ackRequest).Want(http.StatusOK).JSON(&acked)
	if acked.AckedAt == nil {
		t.Fatal("acknowledgement did not set acked_at")
	}
	ackRequest = pendingInputDaemonRequest(http.MethodPost, ackRequest.URL.Path, ackBody, runtimeID, taskID, created.ID, testWorkspaceID, daemonID)
	testutil.Call(t, testHandler.AckTaskPendingInput, ackRequest).Want(http.StatusOK)

	for i := 1; i < pendingInputMaxPerClaim; i++ {
		key := fmt.Sprintf("sha256:%064x", i+100)
		_, err := testPool.Exec(ctx, `
			INSERT INTO task_pending_input (
				id, workspace_id, issue_id, task_id, agent_id, runtime_id, claim_generation,
				request_key, request_sha256, version, state, questions, question_comment_id
			) VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $8, 1, 'cancelled', '[]'::jsonb, $9)
		`, uuid.NewString(), testWorkspaceID, issueID, taskID, agentID, runtimeID, registration.ClaimGeneration, key, uuid.NewString())
		if err != nil {
			t.Fatalf("seed pending input capacity row %d: %v", i, err)
		}
	}
	capacityRequest := registration
	capacityRequest.RequestKey = "sha256:" + strings.Repeat("f", 64)
	registerRequest = pendingInputDaemonRequest(http.MethodPost,
		"/api/daemon/runtimes/"+runtimeID+"/tasks/"+taskID+"/pending-inputs",
		capacityRequest, runtimeID, taskID, "", testWorkspaceID, daemonID)
	testutil.Call(t, testHandler.RegisterTaskPendingInput, registerRequest).Want(http.StatusConflict)
}

func TestTaskPendingInputDaemonBindingAndStaleClaim(t *testing.T) {
	ctx := context.Background()
	daemonID := "pending-input-stale-daemon-" + uuid.NewString()
	runtimeID := dbfx.Runtime(t, "Pending input stale runtime", testutil.Cols{"daemon_id": daemonID})
	agentID := dbfx.Agent(t, "Pending input stale agent", runtimeID)
	issueID := dbfx.Issue(t, "Pending input stale issue", testutil.Cols{"status": "in_progress", "assignee_type": "agent", "assignee_id": agentID})
	dispatchedAt := time.Now().UTC().Truncate(time.Microsecond)
	taskID := dbfx.Task(t, agentID, testutil.Cols{
		"runtime_id": runtimeID, "issue_id": issueID, "status": "running",
		"dispatched_at": dispatchedAt, "started_at": dispatchedAt,
	})
	t.Cleanup(func() {
		_, _ = testPool.Exec(ctx, `DELETE FROM task_pending_input WHERE task_id = $1`, taskID)
		_, _ = testPool.Exec(ctx, `DELETE FROM comment WHERE issue_id = $1`, issueID)
	})

	registration := validPendingInputRegistration()
	registration.ClaimGeneration = dispatchedAt.UnixMicro()
	path := "/api/daemon/runtimes/" + runtimeID + "/tasks/" + taskID + "/pending-inputs"
	var created taskPendingInputResponse
	testutil.Call(t, testHandler.RegisterTaskPendingInput,
		pendingInputDaemonRequest(http.MethodPost, path, registration, runtimeID, taskID, "", testWorkspaceID, daemonID)).
		Want(http.StatusCreated).JSON(&created)

	otherDaemonID := "pending-input-other-daemon-" + uuid.NewString()
	if _, err := testPool.Exec(ctx, `UPDATE agent_runtime SET daemon_id = $2 WHERE id = $1`, runtimeID, otherDaemonID); err != nil {
		t.Fatal(err)
	}
	getPath := path + "/" + created.ID + "?claim_generation=" + strconvFormatInt(registration.ClaimGeneration)
	testutil.Call(t, testHandler.GetTaskPendingInputForDaemon,
		pendingInputDaemonRequest(http.MethodGet, getPath, nil, runtimeID, taskID, created.ID, testWorkspaceID, daemonID)).
		Want(http.StatusNotFound)

	if _, err := testPool.Exec(ctx, `UPDATE agent_runtime SET daemon_id = $2 WHERE id = $1`, runtimeID, daemonID); err != nil {
		t.Fatal(err)
	}
	if _, err := testPool.Exec(ctx, `UPDATE agent_task_queue SET status = 'completed', completed_at = now() WHERE id = $1`, taskID); err != nil {
		t.Fatal(err)
	}
	testutil.Call(t, testHandler.GetTaskPendingInputForDaemon,
		pendingInputDaemonRequest(http.MethodGet, getPath, nil, runtimeID, taskID, created.ID, testWorkspaceID, daemonID)).
		Want(http.StatusConflict)
	var state string
	dbfx.QueryRow(t, `SELECT state FROM task_pending_input WHERE id = $1`, created.ID).Scan(&state)
	if state != "cancelled" {
		t.Fatalf("stale pending input state = %q, want cancelled", state)
	}
}

func TestDeleteTaskBatchDeletesTaskPendingInput(t *testing.T) {
	taskID := seedTaskPendingInputForCleanup(t)
	if err := testHandler.Queries.DeleteTaskBatch(context.Background(), []pgtype.UUID{parseUUID(taskID)}); err != nil {
		t.Fatalf("delete task batch: %v", err)
	}
	assertTaskPendingInputCleanup(t, taskID)
}

func TestDeleteUnstartedQuickCreateRetryTaskDeletesTaskPendingInput(t *testing.T) {
	taskID := seedTaskPendingInputForCleanup(t)
	if _, err := testPool.Exec(context.Background(), `
		UPDATE agent_task_queue
		SET status = 'queued', issue_id = NULL, chat_session_id = NULL, autopilot_run_id = NULL
		WHERE id = $1
	`, taskID); err != nil {
		t.Fatal(err)
	}
	deleted, err := testHandler.Queries.DeleteUnstartedQuickCreateRetryTask(context.Background(), parseUUID(taskID))
	if err != nil {
		t.Fatalf("delete quick-create retry task: %v", err)
	}
	if deleted != 1 {
		t.Fatalf("deleted tasks = %d, want 1", deleted)
	}
	assertTaskPendingInputCleanup(t, taskID)
}

func seedTaskPendingInputForCleanup(t *testing.T) string {
	t.Helper()
	runtimeID := dbfx.Runtime(t, "Pending input cleanup runtime")
	agentID := dbfx.Agent(t, "Pending input cleanup agent", runtimeID)
	issueID := dbfx.Issue(t, "Pending input cleanup issue")
	dispatchedAt := time.Now().UTC().Truncate(time.Microsecond)
	taskID := dbfx.Task(t, agentID, testutil.Cols{
		"runtime_id": runtimeID, "issue_id": issueID, "status": "running",
		"dispatched_at": dispatchedAt, "started_at": dispatchedAt,
	})
	t.Cleanup(func() {
		_, _ = testPool.Exec(context.Background(), `DELETE FROM task_pending_input WHERE task_id = $1`, taskID)
	})
	key := "sha256:" + strings.Repeat("c", 64)
	if _, err := testPool.Exec(context.Background(), `
		INSERT INTO task_pending_input (
			id, workspace_id, issue_id, task_id, agent_id, runtime_id, claim_generation,
			request_key, request_sha256, version, state, questions, question_comment_id
		) VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $8, 1, 'open', '[]'::jsonb, $9)
	`, uuid.NewString(), testWorkspaceID, issueID, taskID, agentID, runtimeID, dispatchedAt.UnixMicro(), key, uuid.NewString()); err != nil {
		t.Fatal(err)
	}
	return taskID
}

func assertTaskPendingInputCleanup(t *testing.T, taskID string) {
	t.Helper()
	var taskCount, pendingCount int
	dbfx.QueryRow(t, `SELECT count(*) FROM agent_task_queue WHERE id = $1`, taskID).Scan(&taskCount)
	dbfx.QueryRow(t, `SELECT count(*) FROM task_pending_input WHERE task_id = $1`, taskID).Scan(&pendingCount)
	if taskCount != 0 || pendingCount != 0 {
		t.Fatalf("remaining rows: task=%d pending_input=%d", taskCount, pendingCount)
	}
}

func pendingInputDaemonRequest(method, path string, body any, runtimeID, taskID, pendingID, workspaceID, daemonID string) *http.Request {
	req := newDaemonTokenRequest(method, path, body, workspaceID, daemonID)
	rctx := chi.NewRouteContext()
	rctx.URLParams.Add("runtimeId", runtimeID)
	rctx.URLParams.Add("taskId", taskID)
	if pendingID != "" {
		rctx.URLParams.Add("pendingId", pendingID)
	}
	return req.WithContext(context.WithValue(req.Context(), chi.RouteCtxKey, rctx))
}

func pendingInputIssueRequest(method, path string, body any, issueID, pendingID string) *http.Request {
	req := newRequest(method, path, body)
	rctx := chi.NewRouteContext()
	rctx.URLParams.Add("issueId", issueID)
	if pendingID != "" {
		rctx.URLParams.Add("pendingId", pendingID)
	}
	return req.WithContext(context.WithValue(req.Context(), chi.RouteCtxKey, rctx))
}

func strconvFormatInt(value int64) string {
	return strconv.FormatInt(value, 10)
}
