package service

import (
	"encoding/json"
	"errors"
	"strings"
	"testing"
)

func TestParsePMOSnapshotPreservesRequirementAndTaskIDs(t *testing.T) {
	got := mustParsePMOSnapshot(t, validPMOSnapshotJSON())
	if got.Parent.Key != "EXT-P-001" || got.Children[0].NumericID != 1002 || got.Children[0].Tasks[0].TaskID != "TASK-001" {
		t.Fatalf("identities were not preserved: %#v", got)
	}
	if got.Parent.Title != "Example parent requirement" {
		t.Fatalf("strings were not normalized: %q", got.Parent.Title)
	}
}

func TestParsePMOSnapshotRejectsIncompleteSnapshot(t *testing.T) {
	raw := mutatePMOSnapshotJSON(t, func(snapshot map[string]any) {
		snapshot["snapshot_complete"] = false
	})
	_, err := ParsePMOSnapshot(raw)
	if !errors.Is(err, ErrIncompletePMOSnapshot) {
		t.Fatalf("expected incomplete snapshot, got %v", err)
	}
}

func TestParsePMOSnapshotRejectsNonJSONAndTrailingContent(t *testing.T) {
	tests := map[string]string{
		"prose":          "Here is the snapshot: " + validPMOSnapshotJSON(),
		"markdown fence": "```json\n" + validPMOSnapshotJSON() + "\n```",
		"trailing JSON":  validPMOSnapshotJSON() + `{}`,
	}
	for name, raw := range tests {
		t.Run(name, func(t *testing.T) {
			if _, err := ParsePMOSnapshot(raw); err == nil {
				t.Fatal("expected parse error")
			}
		})
	}
}

func TestParsePMOSnapshotRejectsUnknownFieldsAndSchemaVersions(t *testing.T) {
	tests := map[string]string{
		"unknown field": mutatePMOSnapshotJSON(t, func(snapshot map[string]any) {
			snapshot["unexpected"] = true
		}),
		"schema version": mutatePMOSnapshotJSON(t, func(snapshot map[string]any) {
			snapshot["schema_version"] = "2"
		}),
	}
	for name, raw := range tests {
		t.Run(name, func(t *testing.T) {
			if _, err := ParsePMOSnapshot(raw); err == nil {
				t.Fatal("expected validation error")
			}
		})
	}
}

func TestParsePMOSnapshotRejectsDuplicateIdentities(t *testing.T) {
	tests := map[string]string{
		"requirement key": mutatePMOSnapshotJSON(t, func(snapshot map[string]any) {
			child := snapshot["child_requirements"].([]any)[0].(map[string]any)
			child["key"] = snapshot["parent_requirement"].(map[string]any)["key"]
		}),
		"task id": mutatePMOSnapshotJSON(t, func(snapshot map[string]any) {
			child := snapshot["child_requirements"].([]any)[0].(map[string]any)
			task := child["tasks"].([]any)[0]
			snapshot["tasks"] = []any{task}
		}),
	}
	for name, raw := range tests {
		t.Run(name, func(t *testing.T) {
			if _, err := ParsePMOSnapshot(raw); err == nil {
				t.Fatal("expected duplicate identity error")
			}
		})
	}
}

func TestParsePMOSnapshotRejectsInvalidStatusesAndDates(t *testing.T) {
	tests := map[string]string{
		"project status": mutatePMOSnapshotJSON(t, func(snapshot map[string]any) {
			snapshot["parent_requirement"].(map[string]any)["status"] = "todo"
		}),
		"issue status": mutatePMOSnapshotJSON(t, func(snapshot map[string]any) {
			snapshot["child_requirements"].([]any)[0].(map[string]any)["status"] = "planned"
		}),
		"calendar date": mutatePMOSnapshotJSON(t, func(snapshot map[string]any) {
			snapshot["parent_requirement"].(map[string]any)["start_date"] = "2026-02-30"
		}),
	}
	for name, raw := range tests {
		t.Run(name, func(t *testing.T) {
			if _, err := ParsePMOSnapshot(raw); err == nil {
				t.Fatal("expected validation error")
			}
		})
	}
}

func TestParsePMOSnapshotRejectsOversizedPayloadAndStrings(t *testing.T) {
	tooLong := mutatePMOSnapshotJSON(t, func(snapshot map[string]any) {
		snapshot["parent_requirement"].(map[string]any)["title"] = strings.Repeat("x", maxPMOTitleBytes+1)
	})
	if _, err := ParsePMOSnapshot(tooLong); err == nil {
		t.Fatal("expected oversized title error")
	}

	_, err := ParsePMOSnapshot(strings.Repeat(" ", maxPMOSnapshotBytes+1))
	if !errors.Is(err, ErrPMOSnapshotTooLarge) {
		t.Fatalf("expected payload-too-large error, got %v", err)
	}
}

func mustParsePMOSnapshot(t *testing.T, raw string) PMOSnapshot {
	t.Helper()
	got, err := ParsePMOSnapshot(raw)
	if err != nil {
		t.Fatal(err)
	}
	return got
}

func mutatePMOSnapshotJSON(t *testing.T, mutate func(map[string]any)) string {
	t.Helper()
	var snapshot map[string]any
	if err := json.Unmarshal([]byte(validPMOSnapshotJSON()), &snapshot); err != nil {
		t.Fatal(err)
	}
	mutate(snapshot)
	raw, err := json.Marshal(snapshot)
	if err != nil {
		t.Fatal(err)
	}
	return string(raw)
}

func validPMOSnapshotJSON() string {
	return `{
  "schema_version": "1",
  "snapshot_complete": true,
  "parent_requirement": {
    "key": "EXT-P-001",
    "display_number": "REQ-001",
    "numeric_id": 1001,
    "title": "  Example parent requirement  ",
    "description": "Example description",
    "source_status": "active",
    "status": "in_progress",
    "owner": {"external_id": "user-001", "display_name": "Example User"},
    "start_date": "2026-08-01",
    "due_date": "2026-08-31",
    "workload": null
  },
  "child_requirements": [{
    "key": "EXT-C-001",
    "display_number": "REQ-001-1",
    "numeric_id": 1002,
    "title": "Example child requirement",
    "description": "Example child description",
    "source_status": "active",
    "status": "todo",
    "owner": null,
    "start_date": null,
    "due_date": null,
    "workload": null,
    "tasks": [{
      "task_id": "TASK-001",
      "scheme_id": "SCHEME-001",
      "title": "Example scheduling task",
      "description": "Example task description",
      "source_status": "active",
      "status": "todo",
      "owner": null,
      "start_date": "2026-08-02",
      "due_date": "2026-08-03",
      "workload": 1,
      "updated_at": "2026-08-01T08:00:00Z"
    }]
  }],
  "tasks": []
}`
}
