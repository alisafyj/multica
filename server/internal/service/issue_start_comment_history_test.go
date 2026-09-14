package service

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"testing"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
	"github.com/multica-ai/multica/server/internal/events"
	"github.com/multica-ai/multica/server/internal/testutil"
	"github.com/multica-ai/multica/server/internal/util"
	db "github.com/multica-ai/multica/server/pkg/db/generated"
)

type r05IssueStartFixture struct {
	*testutil.Fixture
	task  db.AgentTaskQueue
	issue db.Issue
}

func newR05IssueStartFixture(t *testing.T, status string) r05IssueStartFixture {
	t.Helper()
	pool := issueServiceTestPool(t)
	bootstrap := testutil.New(pool, "", "")
	suffix := time.Now().UnixNano()
	userID := bootstrap.User(t, "R05 start", fmt.Sprintf("r05-start-%d@example.test", suffix))
	workspaceID := bootstrap.Workspace(t, "R05 start", fmt.Sprintf("r05-start-%d", suffix))
	fx := testutil.New(pool, workspaceID, userID)
	fx.Member(t, workspaceID, userID, "owner")
	runtimeID := fx.Runtime(t, "R05 runtime")
	agentID := fx.Agent(t, "R05 agent", runtimeID)
	issueID := fx.Issue(t, "R05 issue", testutil.Cols{"status": status, "assignee_type": "agent", "assignee_id": agentID})
	taskID := fx.Task(t, agentID, testutil.Cols{
		"runtime_id": runtimeID, "issue_id": issueID, "status": "dispatched", "dispatched_at": testutil.Raw("clock_timestamp()"),
	})
	q := db.New(pool)
	task, err := q.GetAgentTask(context.Background(), util.MustParseUUID(taskID))
	if err != nil {
		t.Fatal("task fixture lookup failed")
	}
	issue, err := q.GetIssue(context.Background(), util.MustParseUUID(issueID))
	if err != nil {
		t.Fatal("issue fixture lookup failed")
	}
	return r05IssueStartFixture{Fixture: fx, task: task, issue: issue}
}

func (f r05IssueStartFixture) baseline() IssueStart {
	return IssueStart{Version: 1, ClaimGeneration: f.task.DispatchedAt.Time.UnixMicro(), BaseRevision: f.issue.Revision, BaseStatus: f.issue.Status}
}

func TestR05EmptyCommentHistoryAccepted(t *testing.T) {
	for _, variant := range []string{"todo", "in_progress", "existing-comments", "other-issue", "other-workspace"} {
		t.Run(variant, func(t *testing.T) {
			status := "todo"
			if variant == "in_progress" {
				status = variant
			}
			fx := newR05IssueStartFixture(t, status)
			issueID := util.UUIDToString(fx.issue.ID)
			switch variant {
			case "existing-comments":
				fx.Comment(t, issueID, "existing history must not be embedded")
			case "other-issue":
				other := fx.Issue(t, "other issue")
				fx.Comment(t, other, "unrelated history")
			case "other-workspace":
				other := fx.Workspace(t, "other workspace", fmt.Sprintf("r05-other-%d", time.Now().UnixNano()))
				fx.Comment(t, issueID, "wrong-tenant historical row", testutil.Cols{"workspace_id": other})
			}
			svc := NewTaskService(db.New(fx.Pool), fx.Pool, nil, events.New())
			before := time.Now().UTC()
			task, state, err := svc.StartTaskWithIssueStart(context.Background(), fx.task.ID, fx.baseline())
			after := time.Now().UTC()
			if err != nil || task == nil || task.Status != "running" || !state.BaselineAccepted {
				t.Fatal("matching start was not accepted")
			}
			if variant == "existing-comments" {
				if state.EmptyCommentHistory != nil {
					t.Fatal("nonempty history issued empty proof")
				}
				return
			}
			proof := state.EmptyCommentHistory
			if proof == nil {
				t.Fatal("accepted empty history omitted proof")
			}
			if proof.Version != 1 || proof.TaskID != util.UUIDToString(fx.task.ID) || proof.WorkspaceID != fx.WorkspaceID || proof.IssueID != issueID || proof.ClaimGeneration != fx.baseline().ClaimGeneration || proof.Revision != state.Issue.Revision {
				t.Fatal("proof binding mismatch")
			}
			wantRevision := fx.issue.Revision
			if status == "todo" {
				wantRevision++
			}
			if proof.Revision != wantRevision {
				t.Fatal("proof did not bind post-start revision")
			}
			captured, captureErr := time.Parse(time.RFC3339Nano, proof.CapturedAt)
			expires, expiryErr := time.Parse(time.RFC3339Nano, proof.ExpiresAt)
			if captureErr != nil || expiryErr != nil || !strings.HasSuffix(proof.CapturedAt, "Z") || !strings.HasSuffix(proof.ExpiresAt, "Z") || captured.Before(before) || captured.After(after) || expires.Sub(captured) != 5*time.Minute {
				t.Fatal("proof capture time or TTL mismatch")
			}
		})
	}
}

func TestR05EmptyCommentHistoryRejectsStaleBaselineOrGeneration(t *testing.T) {
	for _, variant := range []string{"comment-after-claim", "stale-generation", "assignment", "terminal"} {
		t.Run(variant, func(t *testing.T) {
			fx := newR05IssueStartFixture(t, "todo")
			baseline := fx.baseline()
			q := db.New(fx.Pool)
			switch variant {
			case "comment-after-claim":
				comment, err := q.CreateComment(context.Background(), db.CreateCommentParams{
					IssueID: fx.issue.ID, WorkspaceID: fx.issue.WorkspaceID, AuthorType: "member",
					AuthorID: util.MustParseUUID(fx.UserID), Content: "new context after claim", Type: "comment",
				})
				if err != nil {
					t.Fatal("comment creation failed")
				}
				t.Cleanup(func() { fx.Pool.Exec(context.Background(), "DELETE FROM comment WHERE id=$1", comment.ID) })
			case "stale-generation":
				baseline.ClaimGeneration--
			case "assignment":
				fx.Exec(t, "UPDATE issue SET assignee_id=NULL WHERE id=$1", fx.issue.ID)
			case "terminal":
				fx.Exec(t, "UPDATE issue SET status='done' WHERE id=$1", fx.issue.ID)
				baseline.BaseStatus = "done"
			}
			svc := NewTaskService(q, fx.Pool, nil, events.New())
			_, state, err := svc.StartTaskWithIssueStart(context.Background(), fx.task.ID, baseline)
			if variant == "stale-generation" {
				if !errors.Is(err, ErrStaleIssueStartClaim) {
					t.Fatal("stale generation not rejected")
				}
				task, lookupErr := q.GetAgentTask(context.Background(), fx.task.ID)
				if lookupErr != nil || task.Status != "dispatched" {
					t.Fatal("stale generation did not roll back start")
				}
			} else if err != nil {
				t.Fatal("baseline rejection changed start behavior")
			}
			if state.BaselineAccepted || state.EmptyCommentHistory != nil {
				t.Fatal("unaccepted baseline issued proof")
			}
		})
	}
}

type r05CommentHistoryTx struct {
	pgx.Tx
	calls int
	args  []any
}

func (tx *r05CommentHistoryTx) Query(ctx context.Context, sql string, args ...any) (pgx.Rows, error) {
	if strings.HasPrefix(sql, "-- name: ListCommentsForIssue ") {
		tx.calls++
		tx.args = args
		_, err := tx.Tx.Exec(ctx, "SELECT 1/0")
		return nil, err
	}
	return tx.Tx.Query(ctx, sql, args...)
}

type r05CommentHistoryTxStarter struct{ tx pgx.Tx }

func (s r05CommentHistoryTxStarter) Begin(context.Context) (pgx.Tx, error) { return s.tx, nil }

func TestR05EmptyCommentHistoryQueryErrorRollsBackStart(t *testing.T) {
	for _, status := range []string{"todo", "in_progress"} {
		t.Run(status, func(t *testing.T) {
			fx := newR05IssueStartFixture(t, status)
			q := db.New(fx.Pool)
			tx, err := fx.Pool.Begin(context.Background())
			if err != nil {
				t.Fatal("begin test transaction failed")
			}
			defer tx.Rollback(context.Background())
			probe := &r05CommentHistoryTx{Tx: tx}
			svc := NewTaskService(q, r05CommentHistoryTxStarter{tx: probe}, nil, events.New())
			task, state, err := svc.StartTaskWithIssueStart(context.Background(), fx.task.ID, fx.baseline())
			var pgErr *pgconn.PgError
			if !errors.As(err, &pgErr) || pgErr.Code != "22012" || task != nil || state.EmptyCommentHistory != nil {
				t.Fatal("comment query error did not abort the start transaction")
			}
			if probe.calls != 1 || len(probe.args) != 3 || probe.args[0] != fx.issue.ID || probe.args[1] != fx.issue.WorkspaceID || probe.args[2] != int32(1) {
				t.Fatal("comment existence query was not workspace/issue scoped with limit one")
			}
			storedTask, taskErr := q.GetAgentTask(context.Background(), fx.task.ID)
			storedIssue, issueErr := q.GetIssue(context.Background(), fx.issue.ID)
			if taskErr != nil || issueErr != nil || storedTask.Status != "dispatched" || storedIssue.Status != status || storedIssue.Revision != fx.issue.Revision {
				t.Fatal("query failure committed task or issue changes")
			}
		})
	}
}
