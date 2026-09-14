package daemon

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestReadIssueOutcomeArtifactUsesDaemonOwnedFinalAndBaseline(t *testing.T) {
	task := Task{IssueID: "issue-1", IssueSnapshot: &IssueTaskSnapshot{
		IssueID: "issue-1", Status: "in_progress", Revision: 12, ETag: `W/"issue:issue-1:12"`,
	}}
	path := filepath.Join(t.TempDir(), "issue-outcome.json")
	if err := os.WriteFile(path, []byte(`{"version":1,"outcome":"review_ready"}`), 0o600); err != nil {
		t.Fatal(err)
	}
	got, err := readIssueOutcomeArtifact(path, task, "verified final")
	if err != nil {
		t.Fatal(err)
	}
	if got == nil || got.Outcome != IssueCompletionOutcomeReviewReady || got.Comment != "verified final" ||
		got.BaseRevision != 12 || got.BaseStatus != "in_progress" || got.BaseETag != `W/"issue:issue-1:12"` {
		t.Fatalf("intent = %#v", got)
	}
}

func TestReadIssueOutcomeArtifactAbsentKeepsDefaultDelivered(t *testing.T) {
	got, err := readIssueOutcomeArtifact("", Task{}, "plain final")
	if err != nil || got != nil {
		t.Fatalf("read disabled artifact = %#v, %v", got, err)
	}
	got, err = readIssueOutcomeArtifact(filepath.Join(t.TempDir(), "missing.json"), Task{}, "plain final")
	if err != nil || got != nil {
		t.Fatalf("read absent artifact = %#v, %v", got, err)
	}
}

func TestResetIssueOutcomeArtifactRemovesPriorAttemptIntent(t *testing.T) {
	path := filepath.Join(t.TempDir(), "issue-outcome.json")
	if err := os.WriteFile(path, []byte(`{"version":1,"outcome":"review_ready"}`), 0o600); err != nil {
		t.Fatal(err)
	}
	if err := resetIssueOutcomeArtifact(path); err != nil {
		t.Fatal(err)
	}
	if _, err := os.Stat(path); !os.IsNotExist(err) {
		t.Fatalf("stale artifact still exists after fresh retry reset: %v", err)
	}
}

func TestApplyIssueStartStateRefreshesCompletionBaselineOnlyWhenAccepted(t *testing.T) {
	task := Task{IssueSnapshot: freshIssueSnapshot("issue-1")}
	applyIssueStartState(&task, &IssueStartState{
		BaselineAccepted: true, Status: "in_progress", Revision: 8,
		ETag: `W/"issue:issue-1:8"`, UpdatedAt: "2026-09-06T00:00:00Z",
	})
	if task.IssueSnapshot.Status != "in_progress" || task.IssueSnapshot.Revision != 8 ||
		task.IssueSnapshot.ETag != `W/"issue:issue-1:8"` || task.IssueSnapshot.UpdatedAt != "2026-09-06T00:00:00Z" {
		t.Fatalf("accepted start baseline = %#v", task.IssueSnapshot)
	}

	task = Task{IssueSnapshot: freshIssueSnapshot("issue-2")}
	applyIssueStartState(&task, &IssueStartState{Status: "in_progress", Revision: 99})
	if task.IssueSnapshot != nil {
		t.Fatalf("unaccepted start retained stale snapshot = %#v", task.IssueSnapshot)
	}
}

func TestRejectedIssueStartFallsBackToLiveReadsWithoutOutcomeAuthority(t *testing.T) {
	for _, state := range []*IssueStartState{nil, {BaselineAccepted: false, Revision: 99}} {
		task := Task{
			IssueID: "issue-1", ClaimGeneration: 7,
			IssueStartContractVersion: 1, IssueCompletionContractVersion: 1,
			IssueSnapshot: freshIssueSnapshot("issue-1"),
		}
		applyIssueStartState(&task, state)
		for _, prompt := range []string{BuildPrompt(task, "claude"), BuildDirectPrompt(task)} {
			if !strings.Contains(prompt, "multica issue get issue-1 --output json") ||
				!strings.Contains(prompt, "multica issue comment list issue-1 --roots-only") {
				t.Fatal("rejected issue baseline did not fall back to live body and comment reads")
			}
			if strings.Contains(prompt, "## Authoritative Issue Body Snapshot") || strings.Contains(prompt, IssueOutcomeFileEnv) {
				t.Fatal("rejected issue baseline retained snapshot or outcome authority")
			}
		}
		if taskSupportsIssueOutcomeArtifact(task) {
			t.Fatal("rejected issue baseline authorizes an outcome artifact")
		}
	}
	applyIssueStartState(nil, nil)
}

func TestReadIssueOutcomeArtifactRejectsInvalidPresentArtifact(t *testing.T) {
	base := Task{IssueSnapshot: &IssueTaskSnapshot{IssueID: "issue-1", Status: "todo", Revision: 4, ETag: `W/"issue:issue-1:4"`}}
	for _, tc := range []struct {
		name, body, output string
	}{
		{name: "delivered is implicit", body: `{"version":1,"outcome":"delivered"}`, output: "final"},
		{name: "unknown field", body: `{"version":1,"outcome":"blocked","extra":true}`, output: "blocked"},
		{name: "multiple records", body: "{\"version\":1,\"outcome\":\"blocked\"}\n{\"version\":1,\"outcome\":\"blocked\"}", output: "blocked"},
		{name: "blank final", body: `{"version":1,"outcome":"blocked"}`, output: "  "},
	} {
		t.Run(tc.name, func(t *testing.T) {
			path := filepath.Join(t.TempDir(), "issue-outcome.json")
			if err := os.WriteFile(path, []byte(tc.body), 0o600); err != nil {
				t.Fatal(err)
			}
			if got, err := readIssueOutcomeArtifact(path, base, tc.output); err == nil || got != nil {
				t.Fatalf("read invalid artifact = %#v, %v", got, err)
			}
		})
	}
}

func TestReadIssueOutcomeArtifactRejectsActualContentOverLimit(t *testing.T) {
	path := filepath.Join(t.TempDir(), "issue-outcome.json")
	if err := os.WriteFile(path, []byte(strings.Repeat(" ", issueOutcomeArtifactBytes+1)), 0o600); err != nil {
		t.Fatal(err)
	}
	if got, err := readIssueOutcomeArtifact(path, Task{}, "final"); err == nil || got != nil {
		t.Fatalf("read oversized artifact = %#v, %v", got, err)
	}
}

func TestIssueOutcomePromptRequiresExplicitStartCapability(t *testing.T) {
	snapshot := freshIssueSnapshot("issue-1")
	snapshot.Status = "in_progress"
	snapshot.Revision = 2
	snapshot.ETag = `W/"issue:issue-1:2"`
	capable := Task{
		IssueID: "issue-1", ClaimGeneration: 7, IssueCompletionContractVersion: 1, IssueStartContractVersion: 1,
		IssueSnapshot: snapshot,
	}
	prompt := buildIssueCompletionDeliveryPrompt(capable)
	for _, want := range []string{IssueOutcomeFileEnv, `{"version":1,"outcome":"review_ready"}`, `{"version":1,"outcome":"blocked"}`, "required verification has passed", "verification cannot be completed", "exactly one JSON object", "does not infer"} {
		if !strings.Contains(prompt, want) {
			t.Fatalf("capable prompt missing %q:\n%s", want, prompt)
		}
	}
	legacy := capable
	legacy.IssueStartContractVersion = 0
	if prompt := buildIssueCompletionDeliveryPrompt(legacy); strings.Contains(prompt, IssueOutcomeFileEnv) {
		t.Fatalf("legacy prompt advertises outcome artifact:\n%s", prompt)
	}
	incomplete := capable
	incomplete.IssueSnapshot = freshIssueSnapshot("issue-1")
	incomplete.IssueSnapshot.Complete = false
	if prompt := buildIssueCompletionDeliveryPrompt(incomplete); strings.Contains(prompt, IssueOutcomeFileEnv) {
		t.Fatalf("incomplete snapshot advertises outcome artifact:\n%s", prompt)
	}
}

func TestIssueOutcomePromptWritesArtifactDirectlyWithoutProbeOrReadback(t *testing.T) {
	snapshot := freshIssueSnapshot("issue-1")
	snapshot.Status = "in_progress"
	snapshot.Revision = 2
	snapshot.ETag = `W/"issue:issue-1:2"`
	prompt := buildIssueCompletionDeliveryPrompt(Task{
		IssueID: "issue-1", ClaimGeneration: 7, IssueCompletionContractVersion: 1, IssueStartContractVersion: 1,
		IssueSnapshot: snapshot,
	})
	for _, want := range []string{
		"Use one tool call to write the exact outcome artifact directly",
		"do not separately discover its path, inspect the environment, probe CLI or filesystem capabilities, or read the file back solely for the outcome",
		"If the write fails, report the failure and do not claim the outcome succeeded",
	} {
		if !strings.Contains(prompt, want) {
			t.Fatalf("outcome prompt missing %q:\n%s", want, prompt)
		}
	}
	if strings.Count(prompt, "Use one tool call to write the exact outcome artifact directly") != 1 {
		t.Fatalf("direct-write guidance must appear exactly once:\n%s", prompt)
	}
}
