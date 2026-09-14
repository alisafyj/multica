package daemon

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/multica-ai/multica/server/pkg/protocol"
)

func TestTestingContextClaimWireContract(t *testing.T) {
	for _, field := range []string{"test_generation_context", "test_run_context"} {
		for _, transport := range []string{"http", "ws"} {
			t.Run(field+"/"+transport, func(t *testing.T) {
				contextJSON := `{"type":"` + strings.TrimSuffix(field, "_context") + `","id":"job-1","nested":{"values":[1,true,"quoted\"text"]}}`
				body := json.RawMessage(`{"tasks":[{"id":"task-1","runtime_id":"runtime-1","` + field + `":` + contextJSON + `}]}`)
				var tasks []*Task
				if transport == "http" {
					server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
						if r.URL.Path != "/api/daemon/tasks/claim" || r.Method != http.MethodPost {
							t.Errorf("unexpected claim request: %s %s", r.Method, r.URL.Path)
						}
						w.Header().Set("Content-Type", "application/json")
						_, _ = w.Write(body)
					}))
					defer server.Close()
					var err error
					tasks, err = NewClient(server.URL).ClaimTasks(context.Background(), "daemon-1", []string{"runtime-1"}, 1)
					if err != nil {
						t.Fatalf("HTTP claim: %v", err)
					}
				} else {
					client := newWSRPCClient(time.Second)
					client.attach(func(frame []byte) (*wsOutbound, error) {
						var message protocol.Message
						if err := json.Unmarshal(frame, &message); err != nil {
							return nil, err
						}
						var request protocol.RPCRequestPayload
						if err := json.Unmarshal(message.Payload, &request); err != nil {
							return nil, err
						}
						go client.deliver(protocol.RPCResponsePayload{RequestID: request.RequestID, Status: 200, Body: body})
						return &wsOutbound{data: frame}, nil
					})
					var response struct {
						Tasks []*Task `json:"tasks"`
					}
					status, err := client.Call(context.Background(), "tasks.claim", 0, nil, &response)
					if err != nil || status != http.StatusOK {
						t.Fatalf("WS claim: status=%d err=%v", status, err)
					}
					tasks = response.Tasks
				}
				if len(tasks) != 1 || tasks[0].ID != "task-1" || tasks[0].RuntimeID != "runtime-1" {
					t.Fatalf("claim did not preserve task identity: %+v", tasks)
				}
				if got := BuildDirectPrompt(*tasks[0]); got != contextJSON {
					t.Fatalf("direct prompt lost context JSON: got %q want %q", got, contextJSON)
				}
				if got := buildPromptBody(*tasks[0], "codex", ""); !strings.Contains(got, contextJSON) {
					t.Fatalf("ordinary prompt lost context JSON: %q", got)
				}
				encoded, err := json.Marshal(tasks[0])
				if err != nil {
					t.Fatal(err)
				}
				var roundTrip map[string]json.RawMessage
				if err := json.Unmarshal(encoded, &roundTrip); err != nil || string(roundTrip[field]) != contextJSON {
					t.Fatalf("context must round-trip as an object: %s, %v", roundTrip[field], err)
				}
			})
		}
	}
}

func TestTestingContextAbsentAndNullRemainOrdinary(t *testing.T) {
	for _, payload := range []string{`{}`, `{"test_generation_context":null,"test_run_context":null}`} {
		var task Task
		if err := json.Unmarshal([]byte(payload), &task); err != nil {
			t.Fatal(err)
		}
		task.IssueID = "issue-1"
		task.ClaimGeneration = 1
		task.IssueCompletionContractVersion = 1
		task.RemoteMCPDaemonToken = "task-token"
		task.Repos = []RepoData{{URL: "https://github.com/example/app.git"}}
		if !ordinaryIssueUsesStrictModelSelection(task) || !taskSupportsPendingInput(task, "codex") {
			t.Fatalf("absent testing context changed ordinary issue behavior: %s", payload)
		}
		baseline := Task{IssueID: "issue-1", ClaimGeneration: 1, IssueCompletionContractVersion: 1, RemoteMCPDaemonToken: "task-token", Repos: task.Repos}
		if BuildDirectPrompt(task) != BuildDirectPrompt(baseline) {
			t.Fatalf("absent context changed direct prompt: %s", payload)
		}
		if buildPromptBody(task, "codex", "") != buildPromptBody(baseline, "codex", "") {
			t.Fatalf("absent context changed ordinary prompt: %s", payload)
		}
		if repo, err := primaryRepositoryForTask(task); err != nil || repo == nil || repo.URL != task.Repos[0].URL {
			t.Fatalf("absent context changed repository selection: %s, %v", payload, err)
		}
	}
}
