package handler

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"reflect"
	"slices"
	"strings"
	"sync"
	"testing"

	"github.com/multica-ai/multica/server/internal/testutil"
	db "github.com/multica-ai/multica/server/pkg/db/generated"
)

func TestCodexPluginRuntimeSkillAPILimit(t *testing.T) {
	for _, tc := range []struct {
		name, provider, root, existingScope string
		count                               int
		enable, existing                    bool
		status                              int
	}{
		{name: "last-slot", count: 127, status: http.StatusNoContent},
		{name: "at-limit", count: 128, status: http.StatusBadRequest},
		{name: "historical-over-limit-add", count: 130, status: http.StatusBadRequest},
		{name: "at-limit-idempotent", count: 128, existing: true, status: http.StatusNoContent},
		{name: "historical-over-limit-idempotent", count: 130, existing: true, status: http.StatusNoContent},
		{name: "historical-over-limit-remove", count: 130, existing: true, enable: true, status: http.StatusNoContent},
		{name: "remove-absent", count: 130, enable: true, status: http.StatusNoContent},
		{name: "provider-unaffected", count: 128, root: "provider", status: http.StatusNoContent},
		{name: "universal-unaffected", count: 128, root: "universal", status: http.StatusNoContent},
		{name: "claude-unaffected", provider: "claude", count: 128, status: http.StatusNoContent},
		{name: "other-runtime-unaffected", existingScope: "runtime", count: 128, status: http.StatusNoContent},
		{name: "other-provider-unaffected", existingScope: "provider", count: 128, status: http.StatusNoContent},
	} {
		t.Run(tc.name, func(t *testing.T) {
			provider := tc.provider
			if provider == "" {
				provider = "codex"
			}
			runtimeID := dbfx.Runtime(t, "Skill limit "+tc.name, testutil.Cols{"provider": provider, "runtime_mode": "local"})
			agentID := dbfx.Agent(t, "Skill limit "+tc.name, runtimeID)
			initial := make([]DisabledRuntimeSkill, tc.count)
			for index := range initial {
				initial[index] = DisabledRuntimeSkill{RuntimeID: runtimeID, Provider: provider, Root: "plugin", Plugin: "probe@local", Key: fmt.Sprintf("probe@local:skills/s%03d", index), Name: fmt.Sprintf("probe:s%03d", index)}
				if tc.existingScope == "runtime" {
					initial[index].RuntimeID = "44444444-4444-4444-8444-444444444444"
				}
				if tc.existingScope == "provider" {
					initial[index].Provider = "claude"
				}
			}
			setPluginLimitFixture(t, agentID, initial)
			target := DisabledRuntimeSkill{RuntimeID: runtimeID, Provider: provider, Root: "plugin", Plugin: "probe@local", Key: "probe@local:skills/new", Name: "probe:new"}
			if tc.existing {
				target = initial[0]
			}
			if tc.root != "" {
				target.Root, target.Plugin, target.Key = tc.root, "", "new"
			}
			req := pluginLimitRequest(agentID, target, tc.enable)
			response := testutil.Call(t, testHandler.SetAgentRuntimeSkillEnabled, req).Want(tc.status)
			want := slices.Clone(initial)
			if tc.status == http.StatusBadRequest {
				if !strings.Contains(response.Text(), "128") || !strings.Contains(response.Text(), "plugin") {
					t.Fatal("limit rejection did not explain the supported plugin count")
				}
			} else if tc.enable {
				want = slices.DeleteFunc(want, func(skill DisabledRuntimeSkill) bool { return sameDisabledRuntimeSkill(skill, target) })
			} else {
				want = slices.DeleteFunc(want, func(skill DisabledRuntimeSkill) bool { return sameDisabledRuntimeSkill(skill, target) })
				want = append(want, target)
			}
			if got := getPluginLimitFixture(t, agentID); !reflect.DeepEqual(got, want) {
				t.Fatalf("saved skill state differs: got=%d want=%d", len(got), len(want))
			}
		})
	}
}

func TestCodexPluginRuntimeSkillAPILimitConcurrentUpdates(t *testing.T) {
	runtimeID := dbfx.Runtime(t, "Concurrent skill limit", testutil.Cols{"provider": "codex", "runtime_mode": "local"})
	agentID := dbfx.Agent(t, "Concurrent skill limit", runtimeID)
	initial := make([]DisabledRuntimeSkill, 127)
	for index := range initial {
		initial[index] = DisabledRuntimeSkill{RuntimeID: runtimeID, Provider: "codex", Root: "plugin", Plugin: "probe@local", Key: fmt.Sprintf("probe@local:skills/s%03d", index)}
	}
	setPluginLimitFixture(t, agentID, initial)
	responses := make([]*httptest.ResponseRecorder, 2)
	var group sync.WaitGroup
	for index := range responses {
		responses[index] = httptest.NewRecorder()
		target := DisabledRuntimeSkill{RuntimeID: runtimeID, Provider: "codex", Root: "plugin", Plugin: "probe@local", Key: fmt.Sprintf("probe@local:skills/new%d", index)}
		req := pluginLimitRequest(agentID, target, false)
		group.Add(1)
		go func() {
			defer group.Done()
			testHandler.SetAgentRuntimeSkillEnabled(responses[index], req)
		}()
	}
	group.Wait()
	codes := []int{responses[0].Code, responses[1].Code}
	slices.Sort(codes)
	if !slices.Equal(codes, []int{http.StatusNoContent, http.StatusBadRequest}) || len(getPluginLimitFixture(t, agentID)) != 128 {
		t.Fatalf("concurrent updates exceeded the limit: responses=%v", codes)
	}
}

func pluginLimitRequest(agentID string, skill DisabledRuntimeSkill, enabled bool) *http.Request {
	body := map[string]any{"runtime_id": skill.RuntimeID, "root": skill.Root, "key": skill.Key, "plugin": skill.Plugin, "name": skill.Name, "enabled": enabled}
	return withURLParam(newRequest(http.MethodPut, "/api/agents/"+agentID+"/runtime-skills/enabled", body), "id", agentID)
}

func setPluginLimitFixture(t *testing.T, agentID string, skills []DisabledRuntimeSkill) {
	t.Helper()
	payload, err := json.Marshal(skills)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := testHandler.Queries.UpdateAgentDisabledRuntimeSkills(context.Background(), db.UpdateAgentDisabledRuntimeSkillsParams{ID: parseUUID(agentID), DisabledRuntimeSkills: payload}); err != nil {
		t.Fatal(err)
	}
}

func getPluginLimitFixture(t *testing.T, agentID string) []DisabledRuntimeSkill {
	t.Helper()
	row, err := testHandler.Queries.GetAgent(context.Background(), parseUUID(agentID))
	if err != nil {
		t.Fatal(err)
	}
	return decodeDisabledRuntimeSkills(row.DisabledRuntimeSkills)
}
