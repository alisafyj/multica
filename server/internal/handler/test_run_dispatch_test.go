package handler

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
)

// Dispatch is what makes an agent the executor of a round. It used to record
// only agent_task_id and leave executor_type/executor_id on the member who
// created the run, which broke two things at once:
//
//   - UpdateTestRunCaseResult attributes an agent-written result to
//     run.ExecutorID, so every result the agent reported was filed under a
//     human who never ran it, while still typed "agent".
//   - The run detail page gated its dispatch panel on executor_type == "agent",
//     a value nothing could ever produce before dispatch — so the panel was
//     unreachable and this endpoint had no caller in the product.
func TestDispatchTestRunMakesTheAgentTheExecutor(t *testing.T) {
	projectID := newTestRunProject(t)
	tc := createTestCaseForRun(t, projectID)
	run := createTestRunFromCases(t, "Dispatch executor run", []string{tc.ID})

	if run.ExecutorType != "member" {
		t.Fatalf("a freshly created run should be member-executed, got %q", run.ExecutorType)
	}

	runtimeID := dbfx.Runtime(t, "dispatch-executor-runtime")
	agentID := dbfx.Agent(t, "dispatch-executor-agent", runtimeID)

	w := httptest.NewRecorder()
	req := withURLParam(
		newRequest("POST", "/api/test-runs/"+run.ID+"/dispatch?workspace_id="+testWorkspaceID,
			map[string]any{"agent_id": agentID}),
		"id", run.ID,
	)
	testHandler.DispatchTestRun(w, req)
	if w.Code != http.StatusCreated {
		t.Fatalf("dispatch: got %d, want 201: %s", w.Code, w.Body.String())
	}

	var resp struct {
		TestRun TestRunResponse `json:"test_run"`
	}
	if err := json.NewDecoder(w.Body).Decode(&resp); err != nil {
		t.Fatalf("decode dispatch response: %v", err)
	}
	if resp.TestRun.ExecutorType != "agent" {
		t.Errorf("executor_type after dispatch = %q, want agent", resp.TestRun.ExecutorType)
	}
	if resp.TestRun.ExecutorID != agentID {
		t.Errorf("executor_id after dispatch = %q, want the dispatched agent %q",
			resp.TestRun.ExecutorID, agentID)
	}
	if resp.TestRun.AgentTaskID == nil {
		t.Error("agent_task_id is nil after dispatch")
	}

	// Persisted, not just echoed: the attribution read at result time comes
	// from the row, not from this response.
	var executorType, executorID string
	if err := testPool.QueryRow(context.Background(),
		`SELECT executor_type, executor_id FROM test_run WHERE id = $1`, run.ID,
	).Scan(&executorType, &executorID); err != nil {
		t.Fatalf("read back run: %v", err)
	}
	if executorType != "agent" || executorID != agentID {
		t.Errorf("stored executor = (%s, %s), want (agent, %s)", executorType, executorID, agentID)
	}
}

// The COALESCE guard on the two new UpdateTestRun columns: an update that does
// not mention the executor must not blank it. Start and abort both take this
// path after a dispatch, and a wiped executor_id violates a NOT NULL column.
func TestUpdateTestRunLeavesTheExecutorAloneWhenUnset(t *testing.T) {
	projectID := newTestRunProject(t)
	tc := createTestCaseForRun(t, projectID)
	run := createTestRunFromCases(t, "Executor preservation run", []string{tc.ID})

	runtimeID := dbfx.Runtime(t, "executor-preservation-runtime")
	agentID := dbfx.Agent(t, "executor-preservation-agent", runtimeID)

	w := httptest.NewRecorder()
	req := withURLParam(
		newRequest("POST", "/api/test-runs/"+run.ID+"/dispatch?workspace_id="+testWorkspaceID,
			map[string]any{"agent_id": agentID}),
		"id", run.ID,
	)
	testHandler.DispatchTestRun(w, req)
	if w.Code != http.StatusCreated {
		t.Fatalf("dispatch: got %d, want 201: %s", w.Code, w.Body.String())
	}

	startW := httptest.NewRecorder()
	startReq := withURLParam(
		newRequest("POST", "/api/test-runs/"+run.ID+"/start?workspace_id="+testWorkspaceID, nil),
		"id", run.ID,
	)
	testHandler.StartTestRun(startW, startReq)
	if startW.Code != http.StatusOK {
		t.Fatalf("start: got %d, want 200: %s", startW.Code, startW.Body.String())
	}

	var started TestRunResponse
	if err := json.NewDecoder(startW.Body).Decode(&started); err != nil {
		t.Fatalf("decode start response: %v", err)
	}
	if started.ExecutorType != "agent" || started.ExecutorID != agentID {
		t.Errorf("executor after start = (%s, %s), want (agent, %s)",
			started.ExecutorType, started.ExecutorID, agentID)
	}
}
