package handler

import (
	"context"
	"net/http"
	"reflect"
	"testing"

	"github.com/multica-ai/multica/server/internal/testutil"
)

func TestCodexPluginRuntimeSkillAPIRoundTrip(t *testing.T) {
	for _, provider := range []string{"codex", "claude"} {
		t.Run(provider, func(t *testing.T) {
			runtimeID := dbfx.Runtime(t, "Plugin skill "+provider, testutil.Cols{"provider": provider, "runtime_mode": "local"})
			agentID := dbfx.Agent(t, "Plugin skill "+provider, runtimeID)
			key := "review@local:skills/folder"
			if provider == "claude" {
				key = "review:native-name"
			}
			for _, enabled := range []bool{false, false, true} {
				body := map[string]any{"runtime_id": runtimeID, "root": "plugin", "plugin": "review@local", "key": key, "name": "review:native-name", "enabled": enabled}
				req := withURLParam(newRequest(http.MethodPut, "/api/agents/"+agentID+"/runtime-skills/enabled", body), "id", agentID)
				testutil.Call(t, testHandler.SetAgentRuntimeSkillEnabled, req).Want(http.StatusNoContent)
				row, err := testHandler.Queries.GetAgent(context.Background(), parseUUID(agentID))
				if err != nil {
					t.Fatal(err)
				}
				want := []DisabledRuntimeSkill{}
				if !enabled {
					want = append(want, DisabledRuntimeSkill{RuntimeID: runtimeID, Provider: provider, Root: "plugin", Plugin: "review@local", Key: key, Name: "review:native-name"})
				}
				if got := decodeDisabledRuntimeSkills(row.DisabledRuntimeSkills); !reflect.DeepEqual(got, want) {
					t.Fatalf("saved plugin identity differs: got=%+v want=%+v", got, want)
				}
			}
		})
	}
}

func TestCodexPluginRuntimeSkillAPIPreservesRejectionGates(t *testing.T) {
	for _, scenario := range []string{"other-provider", "cloud-runtime", "wrong-runtime", "wrong-workspace", "non-owner", "missing-plugin", "invalid-root"} {
		t.Run(scenario, func(t *testing.T) {
			cols := testutil.Cols{"provider": "codex", "runtime_mode": "local"}
			if scenario == "other-provider" {
				cols["provider"] = "gemini"
			}
			if scenario == "cloud-runtime" {
				cols["runtime_mode"] = "cloud"
			}
			if scenario == "wrong-workspace" {
				cols["workspace_id"] = dbfx.Workspace(t, "Plugin other workspace", "plugin-other-workspace")
			}
			runtimeID := dbfx.Runtime(t, "Rejected plugin skill "+scenario, cols)
			agentID := dbfx.Agent(t, "Rejected plugin skill "+scenario, runtimeID, testutil.Cols{"visibility": "workspace"})
			userID, want := testUserID, http.StatusBadRequest
			body := map[string]any{"runtime_id": runtimeID, "root": "plugin", "plugin": "review@local", "key": "review@local:skills/folder", "name": "review:native-name", "enabled": false}
			switch scenario {
			case "wrong-runtime":
				body["runtime_id"] = dbfx.Runtime(t, "Unassigned plugin runtime", cols)
				want = http.StatusConflict
			case "wrong-workspace":
				want = http.StatusNotFound
			case "non-owner":
				userID = dbfx.User(t, "Plugin non-owner", "plugin-non-owner@example.invalid")
				dbfx.Member(t, testWorkspaceID, userID, "member")
				want = http.StatusForbidden
			case "missing-plugin":
				delete(body, "plugin")
			case "invalid-root":
				body["root"] = "unrecognized"
			}
			req := withURLParam(newRequestAsUser(userID, http.MethodPut, "/api/agents/"+agentID+"/runtime-skills/enabled", body), "id", agentID)
			testutil.Call(t, testHandler.SetAgentRuntimeSkillEnabled, req).Want(want)
			row, err := testHandler.Queries.GetAgent(context.Background(), parseUUID(agentID))
			if err != nil || len(decodeDisabledRuntimeSkills(row.DisabledRuntimeSkills)) != 0 {
				t.Fatal("rejected request changed disabled runtime skills")
			}
		})
	}
}
