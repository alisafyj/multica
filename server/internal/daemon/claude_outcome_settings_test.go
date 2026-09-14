package daemon

import (
	"context"
	"encoding/json"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"reflect"
	"runtime"
	"strings"
	"testing"
	"time"
)

func TestRunTaskClaudeOutcomeSettingsGrantExactArtifact(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("POSIX shell launch fixture")
	}
	for _, mode := range []string{"supported", "slow-supported", "legacy", "rejected-baseline", "settings-failure"} {
		t.Run(mode, func(t *testing.T) {
			root := t.TempDir()
			capture := filepath.Join(root, "settings.json")
			outcomeCapture := filepath.Join(root, "outcome-path.txt")
			canonicalCapture := filepath.Join(root, "canonical-path.txt")
			tempBase := filepath.Join(root, "temp-real")
			tempAlias := filepath.Join(root, "temp-link")
			if err := os.Mkdir(tempBase, 0o700); err != nil {
				t.Fatal(err)
			}
			if err := os.Symlink(tempBase, tempAlias); err != nil {
				t.Fatal(err)
			}
			t.Setenv("MULTICA_AGENT_TEMP_BASE", tempAlias)
			fake := filepath.Join(root, "claude")
			script := `#!/bin/sh
while [ "$#" -gt 0 ]; do
  if [ "$1" = '--settings' ]; then shift; cp "$1" "$CAPTURE_SETTINGS"; fi
  shift
done
printf '%s' "$MULTICA_ISSUE_OUTCOME_FILE" > "$CAPTURE_OUTCOME"
if [ -n "$MULTICA_ISSUE_OUTCOME_FILE" ]; then
  printf '%s/%s' "$(cd "$(dirname "$MULTICA_ISSUE_OUTCOME_FILE")" && pwd -P)" "$(basename "$MULTICA_ISSUE_OUTCOME_FILE")" > "$CAPTURE_CANONICAL"
  printf '%s' '{"version":1,"outcome":"review_ready"}' > "$MULTICA_ISSUE_OUTCOME_FILE"
fi
IFS= read -r _
printf '%s\n' '{"type":"system","session_id":"sess-outcome-settings"}'
printf '%s\n' '{"type":"result","subtype":"success","is_error":false,"session_id":"sess-outcome-settings","result":"done"}'
`
			if err := os.WriteFile(fake, []byte(script), 0o700); err != nil {
				t.Fatal(err)
			}
			srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				if strings.HasSuffix(r.URL.Path, "/start") {
					if mode == "settings-failure" {
						walkErr := filepath.WalkDir(filepath.Join(root, "workspaces"), func(path string, entry os.DirEntry, err error) error {
							if err != nil {
								return err
							}
							if entry.IsDir() && entry.Name() == "workdir" {
								return os.Mkdir(filepath.Join(filepath.Dir(path), claudeRepositorySetupSettingsFile), 0o700)
							}
							return nil
						})
						if walkErr != nil {
							t.Errorf("create owned settings failure: %v", walkErr)
						}
					}
					updatedAt := ""
					if mode == "slow-supported" {
						updatedAt = time.Now().UTC().Format(time.RFC3339)
					}
					_ = json.NewEncoder(w).Encode(map[string]any{"issue_start": IssueStartState{
						Version: 1, BaselineAccepted: mode != "rejected-baseline", Status: "in_progress",
						Revision: 7, ETag: "W/\"issue:issue-outcome:7\"", UpdatedAt: updatedAt,
					}})
					return
				}
				w.WriteHeader(http.StatusOK)
			}))
			t.Cleanup(srv.Close)
			logs := &lockedBuffer{}
			logger := slog.New(slog.NewJSONHandler(logs, nil))
			d := New(Config{WorkspacesRoot: filepath.Join(root, "workspaces"), ServerBaseURL: srv.URL,
				AgentTimeout: 5 * time.Second, Agents: map[string]AgentEntry{"claude": {Path: fake}}}, logger)
			d.executionEnvironmentCommand = nil
			task := Task{ID: "task-outcome", WorkspaceID: "workspace-outcome", RuntimeID: "runtime-outcome",
				IssueID: "issue-outcome", AgentID: "agent-outcome", AuthToken: "mat_synthetic_outcome",
				ClaimAttempt: 1, ClaimGeneration: 1, IssueStartContractVersion: 1, IssueCompletionContractVersion: 1,
				IssueSnapshot: freshIssueSnapshot("issue-outcome"),
				Agent: &AgentData{ID: "agent-outcome", Name: "outcome fixture", CustomEnv: map[string]string{
					"CAPTURE_SETTINGS": capture, "CAPTURE_OUTCOME": outcomeCapture, "CAPTURE_CANONICAL": canonicalCapture,
				}},
			}
			if mode == "legacy" {
				task.IssueStartContractVersion = 0
			}
			if mode == "slow-supported" {
				task.IssueSnapshot.CapturedAt = time.Now().UTC().Add(-2 * time.Minute).Format(time.RFC3339)
			}
			result, err := d.runTask(context.Background(), task, "claude", 0, logger)
			var authorityEvents []map[string]any
			for _, line := range strings.Split(strings.TrimSpace(logs.String()), "\n") {
				var event map[string]any
				if decodeErr := json.Unmarshal([]byte(line), &event); decodeErr != nil {
					t.Fatal(decodeErr)
				}
				if event["msg"] == "claude issue outcome settings authority" {
					authorityEvents = append(authorityEvents, event)
				}
			}
			if mode == "settings-failure" {
				if err == nil || len(authorityEvents) != 0 {
					t.Fatalf("failed settings persistence advertised authority: err=%v events=%d", err, len(authorityEvents))
				}
				if _, err := os.Stat(outcomeCapture); !os.IsNotExist(err) {
					t.Fatal("provider started after failed settings persistence")
				}
				return
			}
			if err != nil || result.Status != "completed" {
				t.Fatalf("runTask status=%s error=%v", result.Status, err)
			}
			outcome, err := os.ReadFile(outcomeCapture)
			if err != nil {
				t.Fatal(err)
			}
			data, settingsErr := os.ReadFile(capture)
			if mode != "supported" && mode != "slow-supported" {
				if len(outcome) != 0 || !os.IsNotExist(settingsErr) || result.IssueCompletion != nil || len(authorityEvents) != 0 {
					t.Fatalf("unsupported contract gained artifact authority: path=%q settings=%v intent=%v", outcome, settingsErr, result.IssueCompletion)
				}
				return
			}
			if settingsErr != nil {
				t.Fatalf("Claude outcome artifact has no task-local sandbox grant: %v", settingsErr)
			}
			var settings struct {
				Sandbox struct {
					Filesystem struct {
						AllowWrite []string `json:"allowWrite"`
					} `json:"filesystem"`
				} `json:"sandbox"`
			}
			if err := json.Unmarshal(data, &settings); err != nil {
				t.Fatal(err)
			}
			canonical, err := os.ReadFile(canonicalCapture)
			if err != nil {
				t.Fatal(err)
			}
			if string(canonical) == string(outcome) {
				t.Fatal("fixture did not exercise a symlinked temp parent")
			}
			if filepath.Base(string(outcome)) != "issue-outcome.json" || !reflect.DeepEqual(settings.Sandbox.Filesystem.AllowWrite, []string{string(canonical)}) {
				t.Fatalf("grant=%v want canonical exact file %q, not its directory", settings.Sandbox.Filesystem.AllowWrite, canonical)
			}
			if len(authorityEvents) != 1 {
				t.Fatalf("authority events=%d want exactly one", len(authorityEvents))
			}
			event := authorityEvents[0]
			for field, expected := range map[string]any{"task_id": task.ID, "runtime_id": task.RuntimeID,
				"provider": "claude", "cwd": result.WorkDir, "claim_attempt": float64(task.ClaimAttempt),
				"claim_generation": float64(task.ClaimGeneration)} {
				if event[field] != expected {
					t.Fatalf("authority %s=%v want %v", field, event[field], expected)
				}
			}
			wantAuthority := map[string]any{"schema": "claude_issue_outcome_settings_authority/v1",
				"source": "daemon_guarded_issue_outcome", "environment_variable": IssueOutcomeFileEnv,
				"canonical_allow_write_path": string(canonical)}
			if !reflect.DeepEqual(event["authority"], wantAuthority) {
				t.Fatalf("authority projection=%v want=%v", event["authority"], wantAuthority)
			}
			if result.IssueCompletion == nil || result.IssueCompletion.Outcome != IssueCompletionOutcomeReviewReady {
				t.Fatalf("artifact did not produce review-ready completion: %#v", result.IssueCompletion)
			}
			if _, err := os.Stat(filepath.Dir(string(outcome))); !os.IsNotExist(err) {
				t.Fatalf("task temp directory was not removed: %v", err)
			}
		})
	}
}
