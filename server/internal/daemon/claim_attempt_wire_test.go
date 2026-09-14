package daemon

import (
	"encoding/json"
	"testing"
)

func TestTaskDecodesClaimAttemptForPerAttemptTelemetry(t *testing.T) {
	var task Task
	if err := json.Unmarshal([]byte(`{"id":"task-1","claim_attempt":4}`), &task); err != nil {
		t.Fatal(err)
	}
	if task.ClaimAttempt != 4 {
		t.Fatalf("claim attempt = %d, want 4", task.ClaimAttempt)
	}
}

func TestTaskDefaultsClaimAttemptForOlderServer(t *testing.T) {
	var task Task
	if err := json.Unmarshal([]byte(`{"id":"task-1"}`), &task); err != nil {
		t.Fatal(err)
	}
	if task.ClaimAttempt != 0 {
		t.Fatalf("older-server claim attempt = %d, want 0", task.ClaimAttempt)
	}
}
