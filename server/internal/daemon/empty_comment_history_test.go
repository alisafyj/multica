package daemon

import (
	"encoding/json"
	"strings"
	"testing"
	"time"
)

func issueStartWithEmptyHistory(t *testing.T, mutate func(map[string]any)) Task {
	t.Helper()
	now := time.Now().UTC()
	proof := map[string]any{
		"version": 1, "task_id": "task-1", "workspace_id": "workspace-1", "issue_id": "issue-1",
		"claim_generation": 77, "revision": 8,
		"captured_at": now.Format(time.RFC3339Nano), "expires_at": now.Add(5 * time.Minute).Format(time.RFC3339Nano),
	}
	if mutate != nil {
		mutate(proof)
	}
	raw, err := json.Marshal(map[string]any{
		"version": 1, "baseline_accepted": true, "status": "in_progress", "revision": 8,
		"etag": `W/"issue:issue-1:8"`, "empty_comment_history": proof,
	})
	if err != nil {
		t.Fatal(err)
	}
	var state IssueStartState
	if err := json.Unmarshal(raw, &state); err != nil {
		t.Fatal(err)
	}
	task := Task{
		ID: "task-1", WorkspaceID: "workspace-1", IssueID: "issue-1", ClaimGeneration: 77,
		IssueStartContractVersion: 1, IssueCompletionContractVersion: 1,
		IssueSnapshot: freshIssueSnapshot("issue-1"),
	}
	applyIssueStartState(&task, &state)
	return task
}

func TestVerifiedEmptyHistorySkipsOnlyInitialCommentRead(t *testing.T) {
	task := issueStartWithEmptyHistory(t, nil)
	for _, prompt := range []string{BuildPrompt(task, "claude"), BuildDirectPrompt(task)} {
		for _, required := range []string{
			"## Verified Empty Comment History", "zero comments", "at task start",
			"later comments", "## Authoritative Issue Body Snapshot",
		} {
			if !strings.Contains(prompt, required) {
				t.Fatalf("empty-history prompt missing %q", required)
			}
		}
		if strings.Contains(prompt, "--roots-only") || strings.Contains(prompt, "multica issue get issue-1 --output json") {
			t.Fatal("validated empty history retained redundant bootstrap reads")
		}
	}
}

func TestInvalidEmptyHistoryProofRetainsCommentCatchUp(t *testing.T) {
	for _, tc := range []struct {
		name   string
		mutate func(map[string]any)
	}{
		{"unknown version", func(p map[string]any) { p["version"] = 2 }},
		{"wrong task", func(p map[string]any) { p["task_id"] = "other" }},
		{"wrong workspace", func(p map[string]any) { p["workspace_id"] = "other" }},
		{"wrong issue", func(p map[string]any) { p["issue_id"] = "other" }},
		{"wrong generation", func(p map[string]any) { p["claim_generation"] = 76 }},
		{"wrong revision", func(p map[string]any) { p["revision"] = 7 }},
		{"missing capture", func(p map[string]any) { delete(p, "captured_at") }},
		{"expired", func(p map[string]any) {
			p["captured_at"] = time.Now().Add(-10 * time.Minute).UTC().Format(time.RFC3339Nano)
			p["expires_at"] = time.Now().Add(-5 * time.Minute).UTC().Format(time.RFC3339Nano)
		}},
		{"future", func(p map[string]any) {
			p["captured_at"] = time.Now().Add(2 * time.Minute).UTC().Format(time.RFC3339Nano)
			p["expires_at"] = time.Now().Add(3 * time.Minute).UTC().Format(time.RFC3339Nano)
		}},
		{"excessive lifetime", func(p map[string]any) { p["expires_at"] = time.Now().Add(time.Hour).UTC().Format(time.RFC3339Nano) }},
		{"reversed interval", func(p map[string]any) { p["expires_at"] = time.Now().Add(-time.Minute).UTC().Format(time.RFC3339Nano) }},
	} {
		t.Run(tc.name, func(t *testing.T) {
			task := issueStartWithEmptyHistory(t, tc.mutate)
			for _, prompt := range []string{BuildPrompt(task, "claude"), BuildDirectPrompt(task)} {
				if !strings.Contains(prompt, "multica issue comment list issue-1 --roots-only") ||
					strings.Contains(prompt, "## Verified Empty Comment History") {
					t.Fatal("invalid empty-history proof suppressed required comment read")
				}
			}
		})
	}
}

func TestEmptyHistoryIsRevalidatedAtPromptGeneration(t *testing.T) {
	for _, tc := range []struct {
		name   string
		mutate func(*Task)
	}{
		{"new generation", func(task *Task) { task.ClaimGeneration++ }},
		{"new issue revision", func(task *Task) { task.IssueSnapshot.Revision++ }},
		{"expired body", func(task *Task) {
			task.IssueSnapshot.ExpiresAt = time.Now().Add(-time.Minute).UTC().Format(time.RFC3339Nano)
		}},
		{"new comment", func(task *Task) {
			task.TriggerCommentID = "new-comment"
			task.TriggerCommentContent = "new requirement"
		}},
		{"rejected restart", func(task *Task) { applyIssueStartState(task, &IssueStartState{}) }},
		{"old server", func(task *Task) {
			applyIssueStartState(task, &IssueStartState{BaselineAccepted: true, Status: "in_progress", Revision: 8, ETag: `W/"issue:issue-1:8"`})
		}},
	} {
		t.Run(tc.name, func(t *testing.T) {
			task := issueStartWithEmptyHistory(t, nil)
			tc.mutate(&task)
			if prompt := buildRequiredCommentCatchUpPrompt(task); !strings.Contains(prompt, "--roots-only") || strings.Contains(prompt, "## Verified Empty Comment History") {
				t.Fatal("stale or contradicted empty-history proof remained usable")
			}
		})
	}
}

func TestEmptyHistoryCannotBeInjectedThroughClaimJSON(t *testing.T) {
	task := issueStartWithEmptyHistory(t, nil)
	raw, err := json.Marshal(task)
	if err != nil {
		t.Fatal(err)
	}
	var wire map[string]any
	if err := json.Unmarshal(raw, &wire); err != nil {
		t.Fatal(err)
	}
	if _, exists := wire["empty_comment_history"]; exists {
		t.Fatal("start-only proof leaked into serialized claim")
	}
	wire["empty_comment_history"] = task.emptyCommentHistory
	raw, err = json.Marshal(wire)
	if err != nil {
		t.Fatal(err)
	}
	var claimed Task
	if err := json.Unmarshal(raw, &claimed); err != nil {
		t.Fatal(err)
	}
	if got := buildVerifiedEmptyCommentHistoryPrompt(claimed, time.Now().UTC()); got != "" {
		t.Fatal("claim JSON manufactured start-only proof")
	}
}

func TestEmptyHistoryExpiryAndFutureCommentRemainBounded(t *testing.T) {
	task := issueStartWithEmptyHistory(t, nil)
	proof := task.emptyCommentHistory
	if proof == nil {
		t.Fatal("accepted start did not retain proof")
	}
	expires, err := time.Parse(time.RFC3339Nano, proof.ExpiresAt)
	if err != nil {
		t.Fatal(err)
	}
	// Keep the body valid to isolate expiration of the separate history proof.
	task.IssueSnapshot.ExpiresAt = expires.Add(time.Minute).Format(time.RFC3339Nano)
	if got := buildVerifiedEmptyCommentHistoryPrompt(task, expires); got != "" {
		t.Fatal("history proof remained valid at its expiration")
	}
	task = issueStartWithEmptyHistory(t, nil)
	task.NewCommentCount = 1
	if got := buildVerifiedEmptyCommentHistoryPrompt(task, time.Now().UTC()); got != "" {
		t.Fatal("known new comment did not invalidate empty history")
	}
}
