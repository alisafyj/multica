//go:build !windows

package daemon

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"os"
	"strings"
	"sync"
	"testing"
)

func TestExecutionBudgetTerminalCallbackPreservesSessionAndPoisonPrecedence(t *testing.T) {
	for _, tc := range []struct {
		name, errorText, wantReason string
	}{
		{"ordinary cap", "", "execution_budget_exceeded"},
		{"independent poison", "API Error: 400 invalid_request_error: corrupt image in conversation", "api_invalid_request"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			writeRuntimeMCPSelectionFixture(t, "claude")
			d, argsFile, cleanup := newLeaderReuseTestDaemon(t)
			defer cleanup()
			frame, err := json.Marshal(map[string]any{
				"type": "result", "session_id": "budget-session", "subtype": "error_max_budget_usd",
				"is_error": true, "result": tc.errorText,
				"modelUsage": map[string]any{"fixture-model": map[string]any{"inputTokens": 12, "outputTokens": 3}},
			})
			if err != nil {
				t.Fatal(err)
			}
			script := fmt.Sprintf("#!/bin/sh\nprintf 'launch\\n' >> %q\nIFS= read -r _\nprintf '%%s\\n' '%s'\n", argsFile, frame)
			if err := os.WriteFile(d.cfg.Agents["claude"].Path, []byte(script), 0o755); err != nil {
				t.Fatal(err)
			}
			var mu sync.Mutex
			var failed, completed int
			var payload map[string]any
			srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				mu.Lock()
				defer mu.Unlock()
				if strings.HasSuffix(r.URL.Path, "/fail") {
					failed++
					if err := json.NewDecoder(r.Body).Decode(&payload); err != nil {
						t.Errorf("decode failure callback: %v", err)
					}
				} else if strings.HasSuffix(r.URL.Path, "/complete") {
					completed++
				}
				w.WriteHeader(http.StatusOK)
			}))
			defer srv.Close()
			d.client = NewClient(srv.URL)
			d.cfg.ServerBaseURL = srv.URL
			task := leaderReuseTestTask(collidingTaskIDA)
			task.PriorSessionID = "budget-session"
			result, err := d.runTask(context.Background(), task, "claude", 0, d.logger)
			if err != nil {
				t.Fatal(err)
			}
			if result.Status != "blocked" || result.FailureReason != tc.wantReason || result.SessionID != "budget-session" || result.WorkDir == "" {
				t.Fatalf("unexpected failure result: %+v", result)
			}
			if len(result.Usage) != 1 || result.Usage[0].InputTokens != 12 || result.Usage[0].OutputTokens != 3 {
				t.Fatalf("terminal usage lost: %+v", result.Usage)
			}
			if _, err := os.Stat(result.WorkDir); err != nil {
				t.Fatalf("partial workdir lost: %v", err)
			}
			d.reportTaskResult(context.Background(), task.ID, result, d.logger)
			launches, err := os.ReadFile(argsFile)
			if err != nil || strings.Count(string(launches), "launch\n") != 1 {
				t.Fatalf("expected one provider launch: %q (%v)", launches, err)
			}
			mu.Lock()
			defer mu.Unlock()
			if failed != 1 || completed != 0 {
				t.Fatalf("terminal callbacks: fail=%d complete=%d", failed, completed)
			}
			if payload["failure_reason"] != tc.wantReason || payload["session_id"] != "budget-session" || payload["work_dir"] != result.WorkDir {
				t.Fatalf("callback lost failure/session/workdir: %+v", payload)
			}
		})
	}
}
