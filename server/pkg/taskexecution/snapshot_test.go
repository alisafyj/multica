package taskexecution

import (
	"encoding/json"
	"strings"
	"testing"
	"time"
)

func sampleSnapshot() Snapshot {
	started := time.Date(2026, 9, 5, 12, 0, 0, 0, time.UTC)
	return Snapshot{
		SchemaVersion: 1, Provider: "hermes", RequestedModel: "model-a",
		DaemonVersion: "0.4.37-sso.10", DaemonCommit: "445681286", CommunityBaseVersion: "v0.4.37",
		StartedAt: started,
		Phases:    []Phase{{Name: Prepare, StartedAt: started, DurationMS: 10, Status: Running}},
	}
}

func TestDecodeRejectsInvalidSnapshots(t *testing.T) {
	valid, err := json.Marshal(sampleSnapshot())
	if err != nil {
		t.Fatal(err)
	}
	for name, body := range map[string]string{
		"negative duration":          strings.Replace(string(valid), `"duration_ms":10`, `"duration_ms":-1`, 1),
		"missing historical mode":    strings.Replace(string(valid), `"concise_mode":false,`, "", 1),
		"null historical mode":       strings.Replace(string(valid), `"direct_agent_mode":false`, `"direct_agent_mode":null`, 1),
		"unknown schema":             strings.Replace(string(valid), `"schema_version":1`, `"schema_version":2`, 1),
		"unknown phase":              strings.Replace(string(valid), `"name":"prepare"`, `"name":"llm"`, 1),
		"unfinished without running": strings.Replace(string(valid), `"status":"running"`, `"status":"completed"`, 1),
		"task identity in payload":   strings.Replace(string(valid), `"schema_version":1`, `"task_id":"another-task","schema_version":1`, 1),
		"multiple objects":           string(valid) + `{}`,
	} {
		t.Run(name, func(t *testing.T) {
			if _, err := Decode([]byte(body)); err == nil {
				t.Fatal("invalid snapshot was accepted")
			}
		})
	}
}

func TestSnapshotRejectsOverlapAndStaleReplacement(t *testing.T) {
	before := sampleSnapshot()
	after := sampleSnapshot()
	after.Phases[0].Status = Completed
	after.Phases = append(after.Phases, Phase{Name: Execute, StartedAt: after.StartedAt.Add(5 * time.Millisecond), Status: Running})
	if err := after.Validate(); err == nil {
		t.Fatal("overlapping phases accepted")
	}
	after.Phases[1].StartedAt = after.StartedAt.Add(10 * time.Millisecond)
	if err := after.Validate(); err != nil || !after.CanReplace(before) {
		t.Fatalf("forward phase transition rejected: %v", err)
	}
	if before.CanReplace(after) {
		t.Fatal("late prepare snapshot can erase execute")
	}
	changedMode := after
	changedMode.DirectAgentMode = true
	if changedMode.CanReplace(after) {
		t.Fatal("same execution can rewrite historical mode")
	}
	after.Phases[1].Status = Failed
	finished := after.Phases[1].StartedAt
	after.FinishedAt = &finished
	if err := after.Validate(); err != nil {
		t.Fatal(err)
	}
	if before.CanReplace(after) {
		t.Fatal("late running snapshot can erase terminal telemetry")
	}
	olderStart := sampleSnapshot()
	olderStart.StartedAt = olderStart.StartedAt.Add(-time.Second)
	olderStart.Phases[0].StartedAt = olderStart.StartedAt
	if olderStart.CanReplace(after) {
		t.Fatal("older execution can overwrite newer execution")
	}
}
