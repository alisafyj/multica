package service

import (
	"encoding/json"
	"testing"
)

func TestDesignRestoreTaskContextMarshalsDesignSystem(t *testing.T) {
	contextJSON, err := json.Marshal(DesignRestoreTaskContext{
		Type:         DesignRestoreTaskContextType,
		DesignSystem: json.RawMessage(`{"id":"profile-1","status":"analyzed","profile":{"version":"agent-1.0"}}`),
	})
	if err != nil {
		t.Fatalf("marshal design restore context: %v", err)
	}

	var payload map[string]any
	if err := json.Unmarshal(contextJSON, &payload); err != nil {
		t.Fatalf("decode design restore context: %v", err)
	}
	designSystem, ok := payload["design_system"].(map[string]any)
	if !ok || designSystem["id"] != "profile-1" || designSystem["status"] != "analyzed" {
		t.Fatalf("design_system = %#v", payload["design_system"])
	}
}

func TestUIDraftCreateContextMarshalsProjectID(t *testing.T) {
	contextJSON, err := json.Marshal(UIDraftCreateContext{
		Type:        UIDraftCreateContextType,
		WorkspaceID: "workspace-1",
		ProjectID:   "project-1",
	})
	if err != nil {
		t.Fatalf("marshal UI draft context: %v", err)
	}

	var payload map[string]any
	if err := json.Unmarshal(contextJSON, &payload); err != nil {
		t.Fatalf("decode UI draft context: %v", err)
	}
	if payload["project_id"] != "project-1" {
		t.Fatalf("project_id = %#v", payload["project_id"])
	}
}
