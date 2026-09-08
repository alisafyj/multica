package handler

import (
	"encoding/json"
	"strings"
	"testing"
)

func TestAgentTaskResponseClaimAttemptWireCompatibility(t *testing.T) {
	raw, err := json.Marshal(AgentTaskResponse{ID: "task-1", ClaimAttempt: 3})
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(raw), `"claim_attempt":3`) {
		t.Fatalf("claim response omitted claim attempt: %s", raw)
	}

	legacy, err := json.Marshal(AgentTaskResponse{ID: "task-1"})
	if err != nil {
		t.Fatal(err)
	}
	if strings.Contains(string(legacy), "claim_attempt") {
		t.Fatalf("zero claim attempt broke legacy wire shape: %s", legacy)
	}
}
