package service

import (
	"context"
	"testing"

	"github.com/jackc/pgx/v5/pgtype"
	"github.com/multica-ai/multica/server/internal/events"
	"github.com/multica-ai/multica/server/internal/util"
	db "github.com/multica-ai/multica/server/pkg/db/generated"
	"github.com/multica-ai/multica/server/pkg/taskfailure"
)

func TestExecutionBudgetFailurePolicy(t *testing.T) {
	reason := taskfailure.ReasonExecutionBudgetExceeded.String()
	ordinaryDiagnostic := "configured run budget exhausted"

	if got := taskfailure.NormalizeDaemonReason(reason, ordinaryDiagnostic); got != taskfailure.ReasonExecutionBudgetExceeded {
		t.Fatalf("NormalizeDaemonReason(%q) = %q, want exact reason preserved", reason, got)
	}
	if got := taskErrorType(reason); got != "agent_output" {
		t.Fatalf("taskErrorType(%q) = %q, want agent_output", reason, got)
	}
	task := db.AgentTaskQueue{
		IssueID:     pgtype.UUID{Valid: true},
		Attempt:     1,
		MaxAttempts: 3,
	}
	if retryableReasons[reason] || retryEligible(reason, task) {
		t.Fatalf("execution budget failure must not be auto-retry eligible, including with max_attempts=%d", task.MaxAttempts)
	}
	if ResumeUnsafeFailure(reason, ordinaryDiagnostic) {
		t.Fatal("ordinary execution budget failure must remain resume-safe")
	}
	if !ResumeUnsafeFailure(reason, "provider returned 400 invalid_request_error for the recorded conversation") {
		t.Fatal("independent poisoned-session evidence must remain resume-unsafe")
	}
}

func TestFailTaskExecutionBudgetPreservesChatResumePointerWithoutRetry(t *testing.T) {
	pool := newResolveOriginatorPool(t)
	ctx := context.Background()
	q := db.New(pool)
	workspaceID, userID, agentID, _ := seedAttributionFixture(t, pool)

	var runtimeID string
	if err := pool.QueryRow(ctx, `SELECT runtime_id::text FROM agent WHERE id = $1`, agentID).Scan(&runtimeID); err != nil {
		t.Fatalf("load fixture runtime: %v", err)
	}
	var chatSessionID string
	if err := pool.QueryRow(ctx, `
		INSERT INTO chat_session (workspace_id, agent_id, creator_id)
		VALUES ($1, $2, $3)
		RETURNING id`, workspaceID, agentID, userID).Scan(&chatSessionID); err != nil {
		t.Fatalf("seed chat session: %v", err)
	}
	var taskID string
	if err := pool.QueryRow(ctx, `
		INSERT INTO agent_task_queue (
			agent_id, runtime_id, chat_session_id, status, priority, attempt, max_attempts
		) VALUES ($1, $2, $3, 'running', 0, 1, 3)
		RETURNING id`, agentID, runtimeID, chatSessionID).Scan(&taskID); err != nil {
		t.Fatalf("seed running chat task: %v", err)
	}
	t.Cleanup(func() {
		_, _ = pool.Exec(context.Background(), `DELETE FROM agent_task_queue WHERE parent_task_id = $1 OR id = $1`, taskID)
		_, _ = pool.Exec(context.Background(), `DELETE FROM chat_session WHERE id = $1`, chatSessionID)
	})

	const sessionID = "budget-session-exact"
	const workDir = "/tmp/multica-budget-workdir-exact"
	reason := taskfailure.ReasonExecutionBudgetExceeded.String()
	svc := NewTaskService(q, pool, nil, events.New())
	failed, err := svc.FailTask(ctx, util.MustParseUUID(taskID), "configured run budget exhausted", sessionID, workDir, "", reason, false, "", "")
	if err != nil {
		t.Fatalf("FailTask: %v", err)
	}
	if failed.Status != "failed" || failed.FailureReason.String != reason {
		t.Fatalf("failed task status/reason = %q/%q, want failed/%q", failed.Status, failed.FailureReason.String, reason)
	}
	if failed.SessionID.String != sessionID || failed.WorkDir.String != workDir {
		t.Fatalf("failed task resume pointer = %q/%q, want %q/%q", failed.SessionID.String, failed.WorkDir.String, sessionID, workDir)
	}

	var storedSessionID, storedWorkDir pgtype.Text
	if err := pool.QueryRow(ctx, `SELECT session_id, work_dir FROM chat_session WHERE id = $1`, chatSessionID).Scan(&storedSessionID, &storedWorkDir); err != nil {
		t.Fatalf("load chat resume pointer: %v", err)
	}
	if storedSessionID.String != sessionID || storedWorkDir.String != workDir {
		t.Fatalf("chat resume pointer = %q/%q, want %q/%q", storedSessionID.String, storedWorkDir.String, sessionID, workDir)
	}
	var retryChildren int
	if err := pool.QueryRow(ctx, `SELECT count(*) FROM agent_task_queue WHERE parent_task_id = $1`, taskID).Scan(&retryChildren); err != nil {
		t.Fatalf("count retry children: %v", err)
	}
	if retryChildren != 0 {
		t.Fatalf("retry children = %d, want 0 despite max_attempts > 1", retryChildren)
	}
}

func TestFailTaskExecutionBudgetLeavesIssueStatusUnchanged(t *testing.T) {
	pool := newResolveOriginatorPool(t)
	ctx := context.Background()
	q := db.New(pool)
	_, userID, agentID, issueID := seedAttributionFixture(t, pool)

	var runtimeID, initialStatus string
	if err := pool.QueryRow(ctx, `SELECT runtime_id::text FROM agent WHERE id = $1`, agentID).Scan(&runtimeID); err != nil {
		t.Fatalf("load fixture runtime: %v", err)
	}
	if err := pool.QueryRow(ctx, `SELECT status FROM issue WHERE id = $1`, issueID).Scan(&initialStatus); err != nil {
		t.Fatalf("load initial issue status: %v", err)
	}
	var taskID string
	if err := pool.QueryRow(ctx, `
		INSERT INTO agent_task_queue (
			agent_id, runtime_id, issue_id, status, priority, attempt, max_attempts,
			originator_user_id, accountable_user_id
		) VALUES ($1, $2, $3, 'running', 0, 1, 3, $4, $4)
		RETURNING id`, agentID, runtimeID, issueID, userID).Scan(&taskID); err != nil {
		t.Fatalf("seed running issue task: %v", err)
	}
	t.Cleanup(func() {
		_, _ = pool.Exec(context.Background(), `DELETE FROM agent_task_queue WHERE parent_task_id = $1 OR id = $1`, taskID)
	})

	reason := taskfailure.ReasonExecutionBudgetExceeded.String()
	svc := NewTaskService(q, pool, nil, events.New())
	failed, err := svc.FailTask(ctx, util.MustParseUUID(taskID), "configured run budget exhausted", "issue-budget-session", "/tmp/multica-issue-budget", "", reason, false, "", "")
	if err != nil {
		t.Fatalf("FailTask: %v", err)
	}
	if failed.Status != "failed" || failed.FailureReason.String != reason {
		t.Fatalf("failed task status/reason = %q/%q, want failed/%q", failed.Status, failed.FailureReason.String, reason)
	}
	var finalStatus string
	if err := pool.QueryRow(ctx, `SELECT status FROM issue WHERE id = $1`, issueID).Scan(&finalStatus); err != nil {
		t.Fatalf("load final issue status: %v", err)
	}
	if finalStatus != initialStatus {
		t.Fatalf("issue status changed from %q to %q after task failure", initialStatus, finalStatus)
	}
	var retryChildren int
	if err := pool.QueryRow(ctx, `SELECT count(*) FROM agent_task_queue WHERE parent_task_id = $1`, taskID).Scan(&retryChildren); err != nil {
		t.Fatalf("count retry children: %v", err)
	}
	if retryChildren != 0 {
		t.Fatalf("retry children = %d, want 0 despite max_attempts > 1", retryChildren)
	}
}
