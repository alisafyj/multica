package main

import (
	"context"
	"strings"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/multica-ai/multica/server/internal/auth"
	"github.com/multica-ai/multica/server/internal/daemon"
	"github.com/multica-ai/multica/server/internal/testutil"
)

type routerOrdinaryClaimFixture struct {
	runtimeID string
	taskID    string
	daemonID  string
}

func newRouterOrdinaryClaimFixture(t *testing.T, bound bool) routerOrdinaryClaimFixture {
	t.Helper()
	rows := testutil.New(testPool, testWorkspaceID, testUserID)
	var daemonID string
	cols := testutil.Cols{}
	if bound {
		daemonID = "router-claim-" + uuid.NewString()
		cols["daemon_id"] = daemonID
	}
	runtimeID := rows.Runtime(t, "Ordinary claim runtime", cols)
	agentID := rows.Agent(t, "Ordinary claim agent", runtimeID)
	issueID := rows.Issue(t, "Ordinary claim issue", testutil.Cols{
		"assignee_type": "agent", "assignee_id": agentID,
	})
	taskID := rows.Task(t, agentID, testutil.Cols{"runtime_id": runtimeID, "issue_id": issueID})
	rows.Cleanup(t, `DELETE FROM comment WHERE issue_id = $1`, issueID)
	rows.Cleanup(t, `DELETE FROM task_run_evidence WHERE task_id = $1`, taskID)
	rows.Cleanup(t, `DELETE FROM task_token WHERE task_id = $1`, taskID)
	if bound {
		rows.Cleanup(t, `DELETE FROM daemon_token WHERE workspace_id = $1 AND daemon_id = $2`, testWorkspaceID, daemonID)
	}
	return routerOrdinaryClaimFixture{runtimeID: runtimeID, taskID: taskID, daemonID: daemonID}
}

func (f routerOrdinaryClaimFixture) claim(t *testing.T, batch bool) *daemon.Task {
	t.Helper()
	client := daemon.NewClient(testServer.URL)
	client.SetToken(testToken)
	t.Cleanup(client.CloseIdleConnections)
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	var task *daemon.Task
	if batch {
		tasks, err := client.ClaimTasks(ctx, f.daemonID, []string{f.runtimeID}, 1)
		if err != nil || len(tasks) != 1 {
			t.Fatal("ordinary batch claim did not return exactly one task")
		}
		task = tasks[0]
	} else {
		var err error
		task, err = client.ClaimTask(ctx, f.runtimeID)
		if err != nil {
			t.Fatal("ordinary claim failed through the actual router")
		}
	}
	if task == nil || task.ID != f.taskID || task.ClaimGeneration <= 0 {
		t.Fatal("ordinary claim did not return the expected leased task")
	}
	if len(task.RemoteMCPConnections) != 0 {
		t.Fatal("ordinary claim unexpectedly contains remote connections")
	}
	return task
}

func TestRouterOrdinaryClaimPersistsDaemonCredentialWithoutRemoteConnections(t *testing.T) {
	for _, mode := range []string{"single", "batch"} {
		t.Run(mode, func(t *testing.T) {
			f := newRouterOrdinaryClaimFixture(t, true)
			before := time.Now().Add(24 * time.Hour)
			task := f.claim(t, mode == "batch")
			after := time.Now().Add(24 * time.Hour)
			if !strings.HasPrefix(task.RemoteMCPDaemonToken, "mdt_") || !strings.HasPrefix(task.AuthToken, "mat_") {
				t.Fatal("ordinary bound claim is missing distinct daemon and agent credentials")
			}
			ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
			defer cancel()
			var workspaceID, daemonID string
			var expiresAt time.Time
			if err := testPool.QueryRow(ctx, `SELECT workspace_id, daemon_id, expires_at FROM daemon_token WHERE token_hash = $1`, auth.HashToken(task.RemoteMCPDaemonToken)).Scan(&workspaceID, &daemonID, &expiresAt); err != nil {
				t.Fatal("claim daemon credential hash was not persisted")
			}
			if workspaceID != testWorkspaceID || daemonID != f.daemonID {
				t.Fatal("claim daemon credential has incorrect workspace or daemon binding")
			}
			if expiresAt.Before(before.Truncate(time.Microsecond)) || expiresAt.After(after) {
				t.Fatal("claim daemon credential does not have the expected 24-hour expiry")
			}
			var agentTokenCount, daemonTokenCount int
			if err := testPool.QueryRow(ctx, `SELECT count(*) FROM task_token WHERE task_id = $1 AND token_hash = $2 AND workspace_id = $3`, f.taskID, auth.HashToken(task.AuthToken), testWorkspaceID).Scan(&agentTokenCount); err != nil {
				t.Fatal("read claim agent credential persistence")
			}
			if err := testPool.QueryRow(ctx, `SELECT count(*) FROM daemon_token WHERE workspace_id = $1 AND daemon_id = $2`, testWorkspaceID, f.daemonID).Scan(&daemonTokenCount); err != nil {
				t.Fatal("read claim daemon credential count")
			}
			if agentTokenCount != 1 || daemonTokenCount != 1 {
				t.Fatal("claim did not persist exactly one credential of each kind")
			}
		})
	}
}

func TestRouterOrdinaryClaimCredentialUploadsPreparationEvidence(t *testing.T) {
	f := newRouterOrdinaryClaimFixture(t, true)
	task := f.claim(t, false)
	client := daemon.NewClient(testServer.URL)
	client.SetToken(task.RemoteMCPDaemonToken)
	t.Cleanup(client.CloseIdleConnections)
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	req := daemon.TaskRunEvidenceRequest{
		SchemaVersion: "task_run_evidence/v1", Attempt: int32(task.ClaimAttempt),
		ClaimGeneration: task.ClaimGeneration, Revision: 1,
		ProviderReported: daemon.TaskRunEvidenceProviderModel{Source: "missing"},
		Usage:            daemon.TaskRunEvidenceUsage{Source: "missing"},
		ProviderCost:     daemon.TaskRunEvidenceProviderCost{Authority: "missing", Basis: "missing", Source: "missing"},
	}
	if err := client.ReportTaskRunEvidence(ctx, f.runtimeID, f.taskID, req); err != nil {
		t.Fatal("real ordinary claim credential could not upload preparation evidence")
	}
	var count int
	if err := testPool.QueryRow(ctx, `SELECT count(*) FROM task_run_evidence WHERE task_id = $1 AND attempt = $2 AND claim_generation = $3 AND revision = 1`, f.taskID, task.ClaimAttempt, task.ClaimGeneration).Scan(&count); err != nil || count != 1 {
		t.Fatal("real claim preparation evidence was not persisted for the exact lease")
	}
}

func TestRouterOrdinaryUnboundClaimPreservesLegacyResponseWithoutDaemonCredential(t *testing.T) {
	f := newRouterOrdinaryClaimFixture(t, false)
	task := f.claim(t, false)
	if task.RemoteMCPDaemonToken != "" {
		t.Fatal("unbound ordinary claim unexpectedly received a daemon credential")
	}
	if !strings.HasPrefix(task.AuthToken, "mat_") {
		t.Fatal("unbound ordinary claim lost its agent credential")
	}
}
