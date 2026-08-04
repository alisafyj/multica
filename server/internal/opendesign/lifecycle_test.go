package opendesign

import "testing"

func TestSupervisorTerminalRunStatusesRemainDistinct(t *testing.T) {
	statuses := []RunStatus{
		RunStatusCanceled,
		RunStatusAgentFailed,
		RunStatusAuditFailed,
		RunStatusPreviewFailed,
	}
	seen := make(map[RunStatus]struct{}, len(statuses))
	for _, status := range statuses {
		if !IsSupervisorTerminalRunStatus(status) {
			t.Fatalf("IsSupervisorTerminalRunStatus(%q) = false", status)
		}
		if _, exists := seen[status]; exists {
			t.Fatalf("duplicate supervisor terminal status %q", status)
		}
		seen[status] = struct{}{}
	}

	for _, status := range []RunStatus{
		RunStatusPreflightPending,
		RunStatusPreflightFailed,
		RunStatusReady,
		RunStatusRunning,
		RunStatusRunSucceeded,
		RunStatusSucceeded,
		RunStatus("unknown"),
	} {
		if IsSupervisorTerminalRunStatus(status) {
			t.Fatalf("IsSupervisorTerminalRunStatus(%q) = true", status)
		}
	}
}
