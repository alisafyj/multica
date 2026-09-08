package main

import (
	"bytes"
	"context"
	"encoding/json"
	"io"
	"net/http"
	"strconv"
	"strings"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/multica-ai/multica/server/internal/auth"
	"github.com/multica-ai/multica/server/internal/daemon"
	"github.com/multica-ai/multica/server/internal/testutil"
)

type routerTaskEvidenceFixture struct {
	rows        *testutil.Fixture
	runtimeID   string
	taskID      string
	issueID     string
	daemonID    string
	daemonToken string
	generation  int64
}

func newRouterTaskEvidenceFixture(t *testing.T) routerTaskEvidenceFixture {
	t.Helper()
	rows := testutil.New(testPool, testWorkspaceID, testUserID)
	daemonID := "router-evidence-" + uuid.NewString()
	runtimeID := rows.Runtime(t, "Router evidence runtime", testutil.Cols{"daemon_id": daemonID})
	agentID := rows.Agent(t, "Router evidence agent", runtimeID)
	issueID := rows.Issue(t, "Router evidence issue", testutil.Cols{
		"status": "in_progress", "assignee_type": "agent", "assignee_id": agentID,
	})
	dispatchedAt := time.Now().UTC().Truncate(time.Microsecond)
	taskID := rows.Task(t, agentID, testutil.Cols{
		"runtime_id": runtimeID, "issue_id": issueID, "status": "running",
		"attempt": 2, "dispatched_at": dispatchedAt, "started_at": dispatchedAt,
	})
	rows.Cleanup(t, `DELETE FROM task_run_evidence WHERE task_id = $1`, taskID)
	rows.Cleanup(t, `DELETE FROM task_pending_input WHERE task_id = $1`, taskID)
	rows.Cleanup(t, `DELETE FROM comment WHERE issue_id = $1`, issueID)
	raw, err := auth.GenerateDaemonToken()
	if err != nil {
		t.Fatal("generate fixture daemon credential")
	}
	rows.Insert(t, "daemon_token", testutil.Cols{
		"token_hash": auth.HashToken(raw), "workspace_id": testWorkspaceID,
		"daemon_id": daemonID, "expires_at": time.Now().Add(time.Hour),
	})
	return routerTaskEvidenceFixture{
		rows: rows, runtimeID: runtimeID, taskID: taskID, issueID: issueID, daemonID: daemonID,
		daemonToken: raw, generation: dispatchedAt.UnixMicro(),
	}
}

func (f routerTaskEvidenceFixture) path(suffix string) string {
	return "/api/daemon/runtimes/" + f.runtimeID + "/tasks/" + f.taskID + suffix
}

func (f routerTaskEvidenceFixture) evidence() daemon.TaskRunEvidenceRequest {
	return daemon.TaskRunEvidenceRequest{
		SchemaVersion: "task_run_evidence/v1", Attempt: 2, ClaimGeneration: f.generation, Revision: 1,
		ProviderReported: daemon.TaskRunEvidenceProviderModel{Source: "missing"},
		Usage:            daemon.TaskRunEvidenceUsage{Source: "missing"},
		ProviderCost:     daemon.TaskRunEvidenceProviderCost{Authority: "missing", Basis: "missing", Source: "missing"},
	}
}

func (f routerTaskEvidenceFixture) pendingInput() map[string]any {
	return map[string]any{
		"version": 1, "claim_generation": f.generation, "blocking": true,
		"request_key": "sha256:" + strings.Repeat("a", 64),
		"questions": []map[string]any{{
			"id": "choice", "header": "Choice", "question": "Which option?",
			"options": []map[string]string{{"label": "First", "description": "The first option."}},
		}},
	}
}

func routerTaskEvidenceRequest(t *testing.T, method, path, token string, body any) (int, []byte) {
	t.Helper()
	var reader io.Reader
	if body != nil {
		encoded, err := json.Marshal(body)
		if err != nil {
			t.Fatal("encode router test request")
		}
		reader = bytes.NewReader(encoded)
	}
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	req, err := http.NewRequestWithContext(ctx, method, testServer.URL+path, reader)
	if err != nil {
		t.Fatal("create router test request")
	}
	req.Header.Set("Authorization", "Bearer "+token)
	req.Header.Set("Content-Type", "application/json")
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatal("router test request failed")
	}
	defer resp.Body.Close()
	raw, err := io.ReadAll(io.LimitReader(resp.Body, 64<<10))
	if err != nil {
		t.Fatal("read router test response")
	}
	return resp.StatusCode, raw
}

func (f routerTaskEvidenceFixture) registerPendingInput(t *testing.T) string {
	t.Helper()
	status, raw := routerTaskEvidenceRequest(t, http.MethodPost, f.path("/pending-inputs"), f.daemonToken, f.pendingInput())
	if status != http.StatusCreated {
		t.Fatalf("register pending input status = %d, want 201", status)
	}
	var response struct {
		ID string `json:"id"`
	}
	if err := json.Unmarshal(raw, &response); err != nil || response.ID == "" {
		t.Fatal("registered pending input has no id")
	}
	return response.ID
}

func TestRouterTaskEvidenceAcceptsDaemonCredentialThroughClient(t *testing.T) {
	f := newRouterTaskEvidenceFixture(t)
	client := daemon.NewClient(testServer.URL)
	client.SetToken(f.daemonToken)
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	if err := client.ReportTaskRunEvidence(ctx, f.runtimeID, f.taskID, f.evidence()); err != nil {
		t.Fatal("daemon credential could not upload through the actual router")
	}
	var count int
	if err := testPool.QueryRow(ctx, `SELECT count(*) FROM task_run_evidence WHERE task_id = $1`, f.taskID).Scan(&count); err != nil {
		t.Fatal("read persisted evidence")
	}
	if count != 1 {
		t.Fatalf("persisted evidence rows = %d, want 1", count)
	}
}

func TestRouterTaskEvidenceRejectsLoginCredentialWithoutDaemonAttestation(t *testing.T) {
	f := newRouterTaskEvidenceFixture(t)
	status, raw := routerTaskEvidenceRequest(t, http.MethodPost, f.path("/run-evidence"), testToken, f.evidence())
	if status != http.StatusForbidden || !bytes.Contains(raw, []byte("daemon attestation required")) {
		t.Fatalf("login credential status = %d, want daemon-attestation 403", status)
	}
}

func TestRouterTaskEvidenceRejectsAgentTaskCredential(t *testing.T) {
	f := newRouterTaskEvidenceFixture(t)
	raw, err := auth.GenerateAgentTaskToken()
	if err != nil {
		t.Fatal("generate fixture task credential")
	}
	f.rows.Insert(t, "task_token", testutil.Cols{
		"token_hash": auth.HashToken(raw), "task_id": f.taskID,
		"agent_id":     testutil.Raw("(SELECT agent_id FROM agent_task_queue WHERE id = '" + f.taskID + "')"),
		"workspace_id": testWorkspaceID, "user_id": testUserID, "expires_at": time.Now().Add(time.Hour),
	})
	status, _ := routerTaskEvidenceRequest(t, http.MethodPost, f.path("/run-evidence"), raw, f.evidence())
	if status != http.StatusUnauthorized {
		t.Fatalf("agent task credential status = %d, want 401", status)
	}
}

func TestRouterPendingInputAcceptsDaemonRegistration(t *testing.T) {
	f := newRouterTaskEvidenceFixture(t)
	f.registerPendingInput(t)
}

func TestRouterPendingInputAcceptsDaemonRead(t *testing.T) {
	f := newRouterTaskEvidenceFixture(t)
	id := f.registerPendingInput(t)
	path := f.path("/pending-inputs/"+id) + "?claim_generation=" + strconv.FormatInt(f.generation, 10)
	status, _ := routerTaskEvidenceRequest(t, http.MethodGet, path, f.daemonToken, nil)
	if status != http.StatusOK {
		t.Fatalf("daemon pending-input read status = %d, want 200", status)
	}
}

func TestRouterPendingInputAcceptsDaemonAcknowledgment(t *testing.T) {
	f := newRouterTaskEvidenceFixture(t)
	id := f.registerPendingInput(t)
	answerPath := "/api/issues/" + f.issueID + "/pending-inputs/" + id + "/answer?workspace_id=" + testWorkspaceID
	status, _ := routerTaskEvidenceRequest(t, http.MethodPost, answerPath, testToken, map[string]any{
		"idempotency_key": uuid.NewString(),
		"answers":         map[string]any{"choice": map[string]any{"answers": []string{"First"}}},
	})
	if status != http.StatusOK {
		t.Fatalf("member pending-input answer status = %d, want 200", status)
	}
	status, _ = routerTaskEvidenceRequest(t, http.MethodPost, f.path("/pending-inputs/"+id+"/ack"), f.daemonToken, map[string]any{"claim_generation": f.generation})
	if status != http.StatusOK {
		t.Fatalf("daemon pending-input ack status = %d, want 200", status)
	}
	var acked, delivered bool
	f.rows.QueryRow(t, `
		SELECT pending.acked_at IS NOT NULL,
		       pending.answer_comment_id = ANY(task.delivered_comment_ids)
		FROM task_pending_input AS pending
		JOIN agent_task_queue AS task ON task.id = pending.task_id
		WHERE pending.id = $1
	`, id).Scan(&acked, &delivered)
	if !acked || !delivered {
		t.Fatalf("pending-input ack persisted ack=%t delivery=%t, want both true", acked, delivered)
	}
}

func TestRouterPendingInputRejectsLoginCredentialOnDaemonRoutes(t *testing.T) {
	for _, action := range []string{"register", "read", "ack"} {
		t.Run(action, func(t *testing.T) {
			f := newRouterTaskEvidenceFixture(t)
			id := f.registerPendingInput(t)
			method, path, body := http.MethodPost, f.path("/pending-inputs"), any(f.pendingInput())
			switch action {
			case "read":
				method, path, body = http.MethodGet, f.path("/pending-inputs/"+id)+"?claim_generation="+strconv.FormatInt(f.generation, 10), nil
			case "ack":
				path, body = f.path("/pending-inputs/"+id+"/ack"), map[string]any{"claim_generation": f.generation}
			}
			status, _ := routerTaskEvidenceRequest(t, method, path, testToken, body)
			if status != http.StatusNotFound {
				t.Fatalf("login credential status = %d, want 404", status)
			}
		})
	}
}
