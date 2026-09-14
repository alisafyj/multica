package handler

import (
	"context"
	"encoding/json"
	"strings"
	"testing"
)

func TestRuntimeLocalSkillImportabilityRoundTrip(t *testing.T) {
	for _, value := range []string{"", `,"can_import":true`, `,"can_import":false`} {
		t.Run(value, func(t *testing.T) {
			var summary RuntimeLocalSkillSummary
			raw := `{"key":"large-skill","name":"Large skill","source_path":"~/skills/large-skill","provider":"codex","root":"provider","can_disable":true,"file_count":0` + value + `}`
			if err := json.Unmarshal([]byte(raw), &summary); err != nil {
				t.Fatal(err)
			}
			store := NewInMemoryLocalSkillListStore()
			req, err := store.Create(context.Background(), "runtime-test")
			if err != nil {
				t.Fatal(err)
			}
			if err := store.Complete(context.Background(), req.ID, []RuntimeLocalSkillSummary{summary}, true, nil, false); err != nil {
				t.Fatal(err)
			}
			got, err := store.Get(context.Background(), req.ID)
			if err != nil || got == nil || len(got.Skills) != 1 || !got.Skills[0].CanDisable {
				t.Fatalf("control metadata lost: result=%+v err=%v", got, err)
			}
			encoded, err := json.Marshal(got.Skills[0])
			if err != nil {
				t.Fatal(err)
			}
			if value == "" {
				if got.Skills[0].CanImport != nil || strings.Contains(string(encoded), "can_import") {
					t.Fatal("old daemon absence became an asserted import capability")
				}
			} else if got.Skills[0].CanImport == nil || !strings.Contains(string(encoded), strings.TrimPrefix(value, ",")) {
				t.Fatalf("explicit import capability changed: %s", encoded)
			}
		})
	}
}

func TestRuntimeLocalSkillImportabilityRejectsNonBoolean(t *testing.T) {
	for _, raw := range []string{`"false"`, `1`, `{}`, `[]`} {
		var summary RuntimeLocalSkillSummary
		if err := json.Unmarshal([]byte(`{"can_import":`+raw+`}`), &summary); err == nil {
			t.Fatalf("non-boolean import capability accepted: %s", raw)
		}
	}
}
