package daemon

import (
	"context"
	"encoding/json"
	"errors"
	"io"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/multica-ai/multica/server/pkg/taskexecution"
)

func TestHandleTaskRetainsExecutionSnapshots(t *testing.T) {
	for _, scenario := range []string{"completed", "failed", "poll_cancelled", "shutdown_cancelled", "prepare_failed"} {
		t.Run(scenario, func(t *testing.T) {
			var mu sync.Mutex
			var snapshots []taskexecution.Snapshot
			srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				switch {
				case strings.HasSuffix(r.URL.Path, "/execution"):
					body, err := io.ReadAll(r.Body)
					if err != nil {
						t.Error(err)
						w.WriteHeader(http.StatusBadRequest)
						return
					}
					snapshot, err := taskexecution.Decode(body)
					if err != nil {
						t.Errorf("daemon sent invalid snapshot: %v", err)
						w.WriteHeader(http.StatusBadRequest)
						return
					}
					mu.Lock()
					snapshots = append(snapshots, *snapshot)
					mu.Unlock()
				case strings.HasSuffix(r.URL.Path, "/status"):
					status := "running"
					if scenario == "poll_cancelled" {
						status = "cancelled"
					}
					_ = json.NewEncoder(w).Encode(map[string]string{"status": status})
				}
			}))
			defer srv.Close()
			ctx, cancel := context.WithCancel(context.Background())
			defer cancel()
			d := &Daemon{
				cfg:    Config{CLIVersion: "0.4.37-sso.10", CLICommit: "445681286"},
				client: NewClient(srv.URL), logger: slog.New(slog.NewTextHandler(io.Discard, nil)),
				workspaces:         make(map[string]*workspaceState),
				runtimeIndex:       map[string]Runtime{"rt-1": {ID: "rt-1", Provider: "hermes"}},
				cancelPollInterval: time.Hour,
			}
			if scenario == "poll_cancelled" {
				d.cancelPollInterval = time.Millisecond
			}
			d.runner = taskRunnerFunc(func(ctx context.Context, _ Task, _ string, _ int, _ *slog.Logger) (TaskResult, error) {
				execution := taskExecutionFromContext(ctx)
				execution.start()
				if scenario == "prepare_failed" {
					return TaskResult{}, errors.New("prepare failed")
				}
				execution.beginExecute()
				if scenario == "shutdown_cancelled" {
					cancel()
				}
				if scenario == "poll_cancelled" || scenario == "shutdown_cancelled" {
					select {
					case <-ctx.Done():
						return TaskResult{Status: "cancelled"}, ctx.Err()
					case <-time.After(2 * time.Second):
						t.Error("task cancellation was not observed")
						return TaskResult{}, errors.New("cancellation was not observed")
					}
				}
				if scenario == "failed" {
					return TaskResult{}, errors.New("provider failed")
				}
				return TaskResult{Status: "completed"}, nil
			})
			d.handleTask(ctx, Task{ID: "task-1", RuntimeID: "rt-1", WorkspaceID: "workspace-1", ConciseMode: true, Agent: &AgentData{Model: "requested-model"}}, 0)
			mu.Lock()
			defer mu.Unlock()
			wantCount, wantPhaseCount := 4, 3
			wantStatus := scenario
			if scenario == "poll_cancelled" || scenario == "shutdown_cancelled" {
				wantStatus = "cancelled"
			}
			if scenario == "prepare_failed" {
				wantCount, wantPhaseCount, wantStatus = 3, 2, "failed"
			}
			if len(snapshots) != wantCount {
				t.Fatalf("observed %d snapshots, want %d including terminal report", len(snapshots), wantCount)
			}
			last := snapshots[len(snapshots)-1]
			if last.FinishedAt == nil || len(last.Phases) != wantPhaseCount || last.Phases[len(last.Phases)-1].Status != wantStatus {
				t.Fatalf("terminal measurements missing: %+v", last)
			}
			if last.Phases[len(last.Phases)-2].Status != wantStatus {
				t.Fatalf("failed/cancelled measured phase relabeled: %+v", last.Phases)
			}
			if last.DirectAgentMode || !last.ConciseMode {
				t.Fatal("concise mode was conflated with daemon-wide direct mode")
			}
			for i := 1; i < len(snapshots); i++ {
				if !snapshots[i].CanReplace(snapshots[i-1]) {
					t.Fatal("daemon snapshots are not monotonic")
				}
			}
		})
	}
}

func TestExecutionTelemetryUnavailableIsNonFatal(t *testing.T) {
	var executionRequests, completedRequests int
	var mu sync.Mutex
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch {
		case strings.HasSuffix(r.URL.Path, "/execution"):
			mu.Lock()
			executionRequests++
			mu.Unlock()
			w.WriteHeader(http.StatusNotFound)
		case strings.HasSuffix(r.URL.Path, "/complete"):
			mu.Lock()
			completedRequests++
			mu.Unlock()
		case strings.HasSuffix(r.URL.Path, "/status"):
			_ = json.NewEncoder(w).Encode(map[string]string{"status": "running"})
		}
	}))
	defer srv.Close()
	d := &Daemon{
		client: NewClient(srv.URL), logger: slog.New(slog.NewTextHandler(io.Discard, nil)),
		workspaces:         make(map[string]*workspaceState),
		runtimeIndex:       map[string]Runtime{"rt-1": {ID: "rt-1", Provider: "hermes"}},
		cancelPollInterval: time.Hour,
	}
	d.runner = taskRunnerFunc(func(ctx context.Context, _ Task, _ string, _ int, _ *slog.Logger) (TaskResult, error) {
		execution := taskExecutionFromContext(ctx)
		execution.start()
		execution.beginExecute()
		return TaskResult{Status: "completed"}, nil
	})
	d.handleTask(context.Background(), Task{ID: "task-1", RuntimeID: "rt-1", WorkspaceID: "workspace-1"}, 0)
	mu.Lock()
	defer mu.Unlock()
	if executionRequests != 1 || completedRequests != 1 {
		t.Fatalf("old server must receive one capability miss and a completed task: execution=%d completed=%d", executionRequests, completedRequests)
	}
}

func TestExecutionCommunityBaseDoesNotGuessDevelopmentVersions(t *testing.T) {
	for version, want := range map[string]string{
		"0.4.37-sso.10": "v0.4.37", "v0.4.37-sso.10": "v0.4.37",
		"dev": "", "fork-445681286": "", "0.4.37-sso.10-dirty": "",
	} {
		if got := executionCommunityBase(version); got != want {
			t.Fatalf("community base for %q = %q, want %q", version, got, want)
		}
	}
}
