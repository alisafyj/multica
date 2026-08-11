package service

import (
	"context"
	"encoding/json"
	"testing"

	db "github.com/multica-ai/multica/server/pkg/db/generated"
)

func TestParsePMOSyncContextRoundTrip(t *testing.T) {
	stored := PMOSyncContext{
		Type:        PMOSyncContextType,
		WorkspaceID: "workspace-1",
		RequesterID: "requester-1",
		RunID:       "run-1",
		Prompt:      BuildPMOSyncPrompt("EXT-P-001"),
	}
	contextJSON, err := json.Marshal(stored)
	if err != nil {
		t.Fatalf("marshal pmo context: %v", err)
	}
	parsed, ok := ParsePMOSyncContext(contextJSON)
	if !ok {
		t.Fatal("expected ParsePMOSyncContext to accept a pmo_sync context")
	}
	if parsed != stored {
		t.Fatalf("round-trip mismatch:\n got %#v\nwant %#v", parsed, stored)
	}
}

func TestParsePMOSyncContextRejectsForeignInput(t *testing.T) {
	cases := []struct {
		name string
		raw  []byte
	}{
		{"empty", nil},
		{"invalid json", []byte("not-json")},
		{"foreign type", []byte(`{"type":"quick_create","workspace_id":"workspace-1"}`)},
		{"missing type", []byte(`{"workspace_id":"workspace-1","run_id":"run-1","prompt":"x"}`)},
	}
	for _, tc := range cases {
		tc := tc
		t.Run(tc.name, func(t *testing.T) {
			if _, ok := ParsePMOSyncContext(tc.raw); ok {
				t.Fatalf("ParsePMOSyncContext accepted %s", tc.name)
			}
		})
	}
}

// ResolveTaskWorkspaceID resolves a PMO task's workspace from the context
// JSONB alone. Every other lookup on this branch is short-circuited (no
// issue / chat / autopilot link), so a bare TaskService never touches
// Queries for the happy path.
func TestResolveTaskWorkspaceIDPMOSyncContext(t *testing.T) {
	contextJSON, err := json.Marshal(PMOSyncContext{
		Type:        PMOSyncContextType,
		WorkspaceID: "workspace-pmo",
		RunID:       "run-1",
		Prompt:      BuildPMOSyncPrompt("EXT-P-001"),
	})
	if err != nil {
		t.Fatalf("marshal pmo context: %v", err)
	}
	svc := &TaskService{}
	got := svc.ResolveTaskWorkspaceID(context.Background(), db.AgentTaskQueue{Context: contextJSON})
	if got != "workspace-pmo" {
		t.Fatalf("ResolveTaskWorkspaceID = %q, want %q", got, "workspace-pmo")
	}

	// A task bound to an issue/chat/autopilot is never treated as a PMO
	// context task even when it carries a PMO blob — the parse-level guard
	// rejects it before any workspace resolution.
	bound := db.AgentTaskQueue{Context: contextJSON}
	bound.IssueID.Valid = true
	if _, ok := svc.parsePMOSyncContext(bound); ok {
		t.Fatal("issue-bound task parsed as PMO sync context")
	}
	bound.IssueID.Valid = false
	bound.ChatSessionID.Valid = true
	if _, ok := svc.parsePMOSyncContext(bound); ok {
		t.Fatal("chat-bound task parsed as PMO sync context")
	}
}
