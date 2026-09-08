package daemon

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync/atomic"
	"testing"
	"time"

	"github.com/multica-ai/multica/server/pkg/agent"
)

func TestPendingInputBrokerRegistersPollsAndAcksAfterDelivery(t *testing.T) {
	t.Parallel()

	var polls atomic.Int32
	var acked atomic.Bool
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch {
		case r.Method == http.MethodPost && r.URL.Path == "/api/daemon/runtimes/runtime-1/tasks/task-1/pending-inputs":
			var raw map[string]json.RawMessage
			if err := json.NewDecoder(r.Body).Decode(&raw); err != nil {
				t.Fatal(err)
			}
			if _, nested := raw["request"]; nested {
				t.Fatal("registration request was nested")
			}
			encoded, _ := json.Marshal(raw)
			var body struct {
				agent.PendingInputRequest
				ClaimGeneration int64 `json:"claim_generation"`
			}
			if err := json.Unmarshal(encoded, &body); err != nil {
				t.Fatal(err)
			}
			if body.ClaimGeneration != 7 || len(body.RequestKey) != 71 {
				t.Fatalf("unexpected registration: %+v", body)
			}
			_ = json.NewEncoder(w).Encode(map[string]any{"id": "input-1", "state": "open"})
		case r.Method == http.MethodGet && r.URL.Path == "/api/daemon/runtimes/runtime-1/tasks/task-1/pending-inputs/input-1":
			if r.URL.Query().Get("claim_generation") != "7" {
				t.Fatalf("claim_generation query = %q", r.URL.Query().Get("claim_generation"))
			}
			polls.Add(1)
			_ = json.NewEncoder(w).Encode(map[string]any{
				"id": "input-1", "state": "answered",
				"answers": map[string]any{"q1": map[string]any{"answers": []string{"A"}}},
			})
		case r.Method == http.MethodPost && r.URL.Path == "/api/daemon/runtimes/runtime-1/tasks/task-1/pending-inputs/input-1/ack":
			if polls.Load() == 0 {
				t.Fatal("ack arrived before answer poll")
			}
			acked.Store(true)
			_ = json.NewEncoder(w).Encode(map[string]any{"id": "input-1", "state": "answered"})
		default:
			http.NotFound(w, r)
		}
	}))
	defer srv.Close()

	broker := newPendingInputBroker(NewClient(srv.URL), "runtime-1", "task-1", 7, time.Millisecond)
	answer, err := broker.Request(t.Context(), agent.PendingInputRequest{
		Version: agent.PendingInputVersion1, RequestKey: agent.NewPendingInputRequestKey("test", "1"), Blocking: true,
		Questions: []agent.PendingInputQuestion{{ID: "q1", Header: "Choice", Question: "Choose?", AllowOther: true}},
	})
	if err != nil {
		t.Fatal(err)
	}
	if acked.Load() {
		t.Fatal("answer was acknowledged before native delivery")
	}
	if err := answer.MarkDelivered(context.Background()); err != nil {
		t.Fatal(err)
	}
	if !acked.Load() {
		t.Fatal("answer was not acknowledged after native delivery")
	}
}

func TestPendingInputBrokerTreatsOldServerAsUnsupported(t *testing.T) {
	t.Parallel()

	srv := httptest.NewServer(http.NotFoundHandler())
	defer srv.Close()
	broker := newPendingInputBroker(NewClient(srv.URL), "runtime-1", "task-1", 7, time.Millisecond)
	_, err := broker.Request(t.Context(), agent.PendingInputRequest{
		Version: agent.PendingInputVersion1, RequestKey: agent.NewPendingInputRequestKey("test", "old"), Blocking: true,
		Questions: []agent.PendingInputQuestion{{ID: "q1", Question: "Continue?", AllowOther: true}},
	})
	if !errors.Is(err, agent.ErrPendingInputUnsupported) {
		t.Fatalf("error = %v, want ErrPendingInputUnsupported", err)
	}
}

func TestPendingInputBrokerReturnsClosedStateError(t *testing.T) {
	t.Parallel()

	for _, state := range []string{"cancelled", "expired"} {
		t.Run(state, func(t *testing.T) {
			srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				if r.Method == http.MethodPost {
					_ = json.NewEncoder(w).Encode(map[string]any{"id": "input-1", "state": "open"})
					return
				}
				_ = json.NewEncoder(w).Encode(map[string]any{"id": "input-1", "state": state})
			}))
			defer srv.Close()
			broker := newPendingInputBroker(NewClient(srv.URL), "runtime-1", "task-1", 7, time.Millisecond)
			_, err := broker.Request(t.Context(), agent.PendingInputRequest{
				Version: agent.PendingInputVersion1, RequestKey: agent.NewPendingInputRequestKey("test", state), Blocking: true,
				Questions: []agent.PendingInputQuestion{{ID: "q1", Question: "Continue?", AllowOther: true}},
			})
			if err == nil || !strings.Contains(err.Error(), state) {
				t.Fatalf("error = %v, want actionable %s error", err, state)
			}
		})
	}
}

func TestTaskSupportsPendingInputOnlyForOrdinaryCurrentIssueTasks(t *testing.T) {
	t.Parallel()

	ordinary := Task{IssueID: "issue-1", ClaimGeneration: 7, IssueCompletionContractVersion: 1, RemoteMCPDaemonToken: "mdt_fixture"}
	for _, provider := range []string{"claude", "codex"} {
		if !taskSupportsPendingInput(ordinary, provider) {
			t.Fatalf("ordinary %s issue task was not enabled", provider)
		}
	}
	for name, task := range map[string]Task{
		"missing daemon credential": {IssueID: "issue-1", ClaimGeneration: 7, IssueCompletionContractVersion: 1},
		"legacy claim":              {IssueID: "issue-1", IssueCompletionContractVersion: 1},
		"legacy contract":           {IssueID: "issue-1", ClaimGeneration: 7},
		"no issue":                  {ClaimGeneration: 7, IssueCompletionContractVersion: 1},
		"specialized design":        {IssueID: "issue-1", ClaimGeneration: 7, IssueCompletionContractVersion: 1, DesignDocumentContext: json.RawMessage(`{}`)},
	} {
		if name != "missing daemon credential" {
			task.RemoteMCPDaemonToken = ordinary.RemoteMCPDaemonToken
		}
		if taskSupportsPendingInput(task, "claude") {
			t.Fatalf("%s task was enabled", name)
		}
	}
	if taskSupportsPendingInput(ordinary, "opencode") {
		t.Fatal("unsupported provider was enabled")
	}
}
