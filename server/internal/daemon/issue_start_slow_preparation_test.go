package daemon

import (
	"strings"
	"testing"
	"time"
)

func TestAcceptedIssueStartAfterSlowPreparationKeepsOutcome(t *testing.T) {
	now := time.Now().UTC().Truncate(time.Second)
	snapshot := freshIssueSnapshot("issue-slow")
	snapshot.CapturedAt = now.Add(-2 * time.Minute).Format(time.RFC3339)
	snapshot.ExpiresAt = now.Add(time.Minute).Format(time.RFC3339)
	snapshot.CreatedAt = now.Add(-time.Hour).Format(time.RFC3339)
	snapshot.UpdatedAt = snapshot.CapturedAt
	task := Task{IssueID: snapshot.IssueID, ClaimGeneration: 7,
		IssueStartContractVersion: 1, IssueCompletionContractVersion: 1, IssueSnapshot: snapshot}
	if _, ok := issueStartReportForTask(task); !ok {
		t.Fatal("fixture must have a usable, unexpired start baseline")
	}
	expiresAt := snapshot.ExpiresAt
	applyIssueStartState(&task, &IssueStartState{Version: 1, BaselineAccepted: true,
		Status: "in_progress", Revision: snapshot.Revision + 1,
		ETag: issueTaskSnapshotETag(task.IssueID, snapshot.Revision+1), UpdatedAt: now.Format(time.RFC3339)})
	if !taskSupportsIssueOutcomeArtifact(task) {
		t.Fatal("accepted guarded start after slow preparation lost outcome authority")
	}
	if !strings.Contains(BuildPrompt(task, "claude"), IssueOutcomeFileEnv) {
		t.Fatal("ordinary prompt lost its outcome delivery instruction")
	}
	if snapshot.ExpiresAt != expiresAt {
		t.Fatal("start revalidation extended the original expiry ceiling")
	}
	if snapshot.CapturedAt != now.Format(time.RFC3339) {
		t.Fatal("revalidation did not use the server update timestamp")
	}
	if issueTaskSnapshotUsableAt(task, snapshot, now.Add(2*time.Minute)) {
		t.Fatal("start revalidation allowed an expired snapshot")
	}
}

func TestIssueStartRevalidationPreservesTemporalGuards(t *testing.T) {
	now := time.Now().UTC().Truncate(time.Second)
	for _, test := range []struct {
		name, updatedAt string
		version         int
		expired         bool
		wantAvailable   bool
	}{
		{name: "malformed", version: 1, updatedAt: "not-a-timestamp"},
		{name: "future", version: 1, updatedAt: now.Add(3 * time.Minute).Format(time.RFC3339)},
		{name: "before-created", version: 1, updatedAt: now.Add(-2 * time.Hour).Format(time.RFC3339)},
		{name: "expired", version: 1, updatedAt: now.Format(time.RFC3339), expired: true},
		{name: "unchanged-update", version: 1, updatedAt: now.Add(-2 * time.Minute).Format(time.RFC3339), wantAvailable: true},
		{name: "unsupported-version-zero", version: 0, updatedAt: now.Format(time.RFC3339)},
		{name: "unsupported-version-next", version: 2, updatedAt: now.Format(time.RFC3339)},
	} {
		t.Run(test.name, func(t *testing.T) {
			snapshot := freshIssueSnapshot("issue-time-guard")
			snapshot.CapturedAt = now.Add(-2 * time.Minute).Format(time.RFC3339)
			snapshot.ExpiresAt = now.Add(5 * time.Minute).Format(time.RFC3339)
			snapshot.CreatedAt = now.Add(-time.Hour).Format(time.RFC3339)
			if test.expired {
				snapshot.ExpiresAt = now.Add(-time.Minute).Format(time.RFC3339)
			}
			expiresAt := snapshot.ExpiresAt
			task := Task{IssueID: snapshot.IssueID, ClaimGeneration: 7,
				IssueStartContractVersion: 1, IssueCompletionContractVersion: 1, IssueSnapshot: snapshot}
			applyIssueStartState(&task, &IssueStartState{Version: test.version, BaselineAccepted: true,
				Status: "in_progress", Revision: 8, ETag: issueTaskSnapshotETag(task.IssueID, 8), UpdatedAt: test.updatedAt})
			if taskSupportsIssueOutcomeArtifact(task) != test.wantAvailable ||
				(buildAuthoritativeIssueSnapshotBlock(task) != "") != test.wantAvailable {
				t.Fatal("snapshot/outcome availability did not preserve temporal guards")
			}
			if test.wantAvailable && snapshot.CapturedAt != test.updatedAt {
				t.Fatal("an unchanged update timestamp advanced the capture time")
			}
			if snapshot.ExpiresAt != expiresAt {
				t.Fatal("start revalidation changed the original expiry ceiling")
			}
		})
	}
}
