package handler

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"reflect"
	"strings"
	"testing"

	"github.com/multica-ai/multica/server/internal/agentconfig"
	"github.com/multica-ai/multica/server/internal/daemonws"
	"github.com/multica-ai/multica/server/internal/testutil"
)

func TestR03RuntimeMCPSelectionSaveValidation(t *testing.T) {
	if testHandler == nil {
		t.Fatal("dedicated database required")
	}
	runtimeID := dbfx.Runtime(t, "R03 validation runtime", testutil.Cols{"provider": "codex"})
	agentID := dbfx.Agent(t, "R03 validation agent", runtimeID)
	for i, config := range []string{
		`{"_multica":{"runtimeMcp":{"mode":"typo"}}}`,
		`{"_multica":{"runtimeMcp":{"mode":"allowlist","allow":["same","same"]}}}`,
		`{"_multica":{"runtimeMcp":{"mode":"deny_all","headers":{"Authorization":"synthetic-r03-secret"}}}}`,
		`{"_multica":{"runtimeMCP":{"mode":"deny_all"}}}`,
		`{"_multica":{"RUNTIMEMCP":{}}}`,
	} {
		t.Run(fmt.Sprintf("invalid-%d", i), func(t *testing.T) {
			name := fmt.Sprintf("R03 invalid create %d", i)
			dbfx.Cleanup(t, `DELETE FROM agent WHERE workspace_id=$1 AND name=$2`, testWorkspaceID, name)
			body := map[string]any{"name": name, "runtime_id": runtimeID, "mcp_config": json.RawMessage(config)}
			created := testutil.Call(t, testHandler.CreateAgent, newRequest("POST", "/api/agents", body)).Want(http.StatusBadRequest)
			updated := testutil.Call(t, testHandler.UpdateAgent, withURLParam(newRequest("PUT", "/api/agents/"+agentID, map[string]any{"mcp_config": json.RawMessage(config)}), "id", agentID)).Want(http.StatusBadRequest)
			if strings.Contains(created.Body.String()+updated.Body.String(), "synthetic-r03-secret") {
				t.Fatal("rejected policy leaked in API response")
			}
		})
	}
	for _, provider := range []string{"claude", "unknown"} {
		t.Run(provider, func(t *testing.T) {
			target := dbfx.Runtime(t, "R03 target "+provider, testutil.Cols{"provider": provider})
			name := "R03 create " + provider
			dbfx.Cleanup(t, `DELETE FROM agent WHERE workspace_id=$1 AND name=$2`, testWorkspaceID, name)
			want := http.StatusCreated
			if provider == "unknown" {
				want = http.StatusBadRequest
			}
			testutil.Call(t, testHandler.CreateAgent, newRequest("POST", "/api/agents", map[string]any{"name": name, "runtime_id": target, "mcp_config": json.RawMessage(`{"_multica":{"runtimeMcp":{"mode":"deny_all"}}}`)})).Want(want)
		})
	}
}

func TestR03RuntimeMCPSelectionMetadataEnvelopeRoundTrip(t *testing.T) {
	if testHandler == nil {
		t.Fatal("dedicated database required")
	}
	runtimeID := dbfx.Runtime(t, "R03 metadata runtime", testutil.Cols{"provider": "codex"})
	for _, tc := range []struct{ name, policy string }{
		{"missing", ""},
		{"inherit", `,"runtimeMcp":{"mode":"inherit"}`},
		{"deny_all", `,"runtimeMcp":{"mode":"deny_all"}`},
	} {
		t.Run(tc.name, func(t *testing.T) {
			config := json.RawMessage(`{"_multica":{"future":{"enabled":true},"futureOther":{"mode":"deny_all"}` + tc.policy + `},"mcpServers":{"agent":{"command":"agent-server"}},"overlayExtension":true}`)
			var want any
			if err := json.Unmarshal(config, &want); err != nil {
				t.Fatal(err)
			}
			assertConfig := func(raw json.RawMessage) {
				t.Helper()
				var got any
				if err := json.Unmarshal(raw, &got); err != nil || !reflect.DeepEqual(got, want) {
					t.Fatal("API dropped or changed metadata, policy, or MCP overlay")
				}
			}
			name := "R03 metadata create " + tc.name
			dbfx.Cleanup(t, `DELETE FROM agent WHERE workspace_id=$1 AND name=$2`, testWorkspaceID, name)
			var created AgentResponse
			testutil.Call(t, testHandler.CreateAgent, newRequest("POST", "/api/agents", map[string]any{"name": name, "runtime_id": runtimeID, "mcp_config": config})).Want(http.StatusCreated).JSON(&created)
			assertConfig(created.McpConfig)
			agentID := dbfx.Agent(t, "R03 metadata update "+tc.name, runtimeID)
			var updated AgentResponse
			testutil.Call(t, testHandler.UpdateAgent, withURLParam(newRequest("PUT", "/api/agents/"+agentID, map[string]any{"mcp_config": config}), "id", agentID)).Want(http.StatusOK).JSON(&updated)
			assertConfig(updated.McpConfig)
			for _, id := range []string{created.ID, agentID} {
				var saved AgentResponse
				testutil.Call(t, testHandler.GetAgent, withURLParam(newRequest("GET", "/api/agents/"+id, nil), "id", id)).Want(http.StatusOK).JSON(&saved)
				assertConfig(saved.McpConfig)
			}
		})
	}
}

func TestR03RuntimeMCPSelectionCodexNameAPIEarlyFailure(t *testing.T) {
	if testHandler == nil {
		t.Fatal("dedicated database required")
	}
	codexRuntimeID := dbfx.Runtime(t, "R03 name Codex runtime", testutil.Cols{"provider": "codex"})
	claudeRuntimeID := dbfx.Runtime(t, "R03 name Claude runtime", testutil.Cols{"provider": "claude"})
	for _, tc := range []struct {
		name, runtimeID, serverName string
		accepted                    bool
	}{
		{"codex_ascii", codexRuntimeID, "Docs_01-x", true},
		{"codex_colon", codexRuntimeID, "plugin:source", false},
		{"codex_unicode", codexRuntimeID, "\u5de5\u5177", false},
		{"claude_colon", claudeRuntimeID, "plugin:source", true},
		{"claude_unicode", claudeRuntimeID, "\u5de5\u5177", true},
	} {
		t.Run(tc.name, func(t *testing.T) {
			config, err := json.Marshal(map[string]any{"_multica": map[string]any{"runtimeMcp": map[string]any{"mode": "allowlist", "allow": []string{tc.serverName}}}})
			if err != nil {
				t.Fatal(err)
			}
			initial := json.RawMessage(`{"_multica":{"runtimeMcp":{"mode":"inherit"}}}`)
			agentID := dbfx.Agent(t, "R03 name update "+tc.name, tc.runtimeID, testutil.Cols{"mcp_config": initial})
			before, err := testHandler.Queries.GetAgent(context.Background(), parseUUID(agentID))
			if err != nil {
				t.Fatal(err)
			}
			name := "R03 name create " + tc.name
			dbfx.Cleanup(t, `DELETE FROM agent WHERE workspace_id=$1 AND name=$2`, testWorkspaceID, name)
			createStatus, updateStatus := http.StatusBadRequest, http.StatusBadRequest
			if tc.accepted {
				createStatus, updateStatus = http.StatusCreated, http.StatusOK
			}
			created := testutil.Call(t, testHandler.CreateAgent, newRequest("POST", "/api/agents", map[string]any{"name": name, "runtime_id": tc.runtimeID, "mcp_config": json.RawMessage(config)})).Want(createStatus)
			updated := testutil.Call(t, testHandler.UpdateAgent, withURLParam(newRequest("PUT", "/api/agents/"+agentID, map[string]any{"mcp_config": json.RawMessage(config)}), "id", agentID)).Want(updateStatus)
			after, err := testHandler.Queries.GetAgent(context.Background(), parseUUID(agentID))
			if err != nil {
				t.Fatal(err)
			}
			if !tc.accepted {
				if strings.Contains(created.Body.String()+updated.Body.String(), tc.serverName) {
					t.Fatal("name validation response leaked rejected name")
				}
				if string(after.McpConfig) != string(before.McpConfig) || after.RuntimeID != before.RuntimeID {
					t.Fatal("rejected update changed saved agent")
				}
				var count int
				if err := testPool.QueryRow(context.Background(), `SELECT COUNT(*) FROM agent WHERE workspace_id=$1 AND name=$2`, testWorkspaceID, name).Scan(&count); err != nil {
					t.Fatal(err)
				}
				if count != 0 {
					t.Fatal("rejected create persisted an agent")
				}
			} else {
				selection, err := agentconfig.ParseRuntimeMCPSelection(after.McpConfig)
				if err != nil || selection.Mode != "allowlist" || !reflect.DeepEqual(selection.Allow, []string{tc.serverName}) {
					t.Fatal("accepted name did not survive persistence")
				}
				if tc.runtimeID == claudeRuntimeID {
					testutil.Call(t, testHandler.UpdateAgent, withURLParam(newRequest("PUT", "/api/agents/"+agentID, map[string]any{"runtime_id": codexRuntimeID}), "id", agentID)).Want(http.StatusBadRequest)
					moved, err := testHandler.Queries.GetAgent(context.Background(), parseUUID(agentID))
					if err != nil {
						t.Fatal(err)
					}
					if moved.RuntimeID != after.RuntimeID || string(moved.McpConfig) != string(after.McpConfig) {
						t.Fatal("rejected runtime move changed saved agent")
					}
				}
			}
		})
	}
}

func TestR03RuntimeMCPSelectionUpdatePreservesExtensionsAndNullClear(t *testing.T) {
	if testHandler == nil {
		t.Fatal("dedicated database required")
	}
	runtimeID := dbfx.Runtime(t, "R03 update runtime", testutil.Cols{"provider": "codex"})
	unknownID := dbfx.Runtime(t, "R03 unknown update runtime", testutil.Cols{"provider": "unknown"})
	config := json.RawMessage(`{"_multica":{"runtimeMcp":{"mode":"allowlist","allow":["Docs"]}},"mcpServers":{"agent":{"command":"agent-server"}},"overlayExtension":{"keep":true}}`)
	agentID := dbfx.Agent(t, "R03 update agent", runtimeID, testutil.Cols{"mcp_config": config})
	update := func(body map[string]any, status int) {
		t.Helper()
		testutil.Call(t, testHandler.UpdateAgent, withURLParam(newRequest("PUT", "/api/agents/"+agentID, body), "id", agentID)).Want(status)
	}
	read := func() json.RawMessage {
		t.Helper()
		row, err := testHandler.Queries.GetAgent(context.Background(), parseUUID(agentID))
		if err != nil {
			t.Fatal(err)
		}
		return row.McpConfig
	}
	update(map[string]any{"description": "unchanged selection"}, http.StatusOK)
	if got := read(); !strings.Contains(string(got), "overlayExtension") || !strings.Contains(string(got), "Docs") {
		t.Fatal("omission erased selection/extension")
	}
	update(map[string]any{"mcp_config": config}, http.StatusOK)
	if got := read(); !strings.Contains(string(got), "overlayExtension") {
		t.Fatal("save erased overlay extension")
	}
	update(map[string]any{"runtime_id": unknownID}, http.StatusBadRequest)
	update(map[string]any{"runtime_id": unknownID, "mcp_config": nil}, http.StatusOK)
	if got := read(); got != nil {
		t.Fatal("null did not clear mcp_config")
	}
	update(map[string]any{"mcp_config": config}, http.StatusBadRequest)
	update(map[string]any{"mcp_config": json.RawMessage(`{"_multica":{"runtimeMcp":{"mode":"inherit"}},"extension":true}`)}, http.StatusOK)
}

func TestR03RuntimeMCPSelectionClaimGate(t *testing.T) {
	if testHandler == nil {
		t.Fatal("dedicated database required")
	}
	for _, tc := range []struct {
		name, provider, policy, capability string
		want                               int
	}{
		{"old-deny", "codex", `{"mode":"deny_all"}`, "", 422},
		{"old-allow", "claude", `{"mode":"allowlist","allow":["docs"]}`, "", 422},
		{"old-inherit", "codex", `{"mode":"inherit"}`, "", 200},
		{"old-missing", "claude", "", "", 200},
		{"current-codex", "codex", `{"mode":"deny_all"}`, "runtime-mcp-selection-v1", 200},
		{"current-claude", "claude", `{"mode":"allowlist","allow":["docs"]}`, "runtime-mcp-selection-v1", 200},
		{"false-capability", "codex", `{"mode":"deny_all"}`, "runtime-mcp-selection-v10", 422},
		{"unknown-provider", "unknown", `{"mode":"deny_all"}`, "runtime-mcp-selection-v1", 422},
		{"invalid-policy", "codex", `{"mode":"synthetic-r03-secret"}`, "runtime-mcp-selection-v1", 422},
	} {
		t.Run(tc.name, func(t *testing.T) {
			// Registration metadata alone must not authorize an old claimant.
			runtimeID := dbfx.Runtime(t, "R03 claim "+tc.name, testutil.Cols{"provider": tc.provider, "metadata": json.RawMessage(`{"capabilities":["runtime-mcp-selection-v1"]}`)})
			config := json.RawMessage(`{"mcpServers":{"agent":{"command":"agent-server"}},"overlayExtension":true}`)
			if tc.policy != "" {
				config = json.RawMessage(`{"_multica":{"runtimeMcp":` + tc.policy + `},"mcpServers":{"agent":{"command":"agent-server"}},"overlayExtension":true}`)
			}
			agentID := dbfx.Agent(t, "R03 claim agent "+tc.name, runtimeID, testutil.Cols{"mcp_config": config})
			issueID := dbfx.Issue(t, "R03 claim issue", testutil.Cols{"assignee_type": "agent", "assignee_id": agentID})
			taskID := dbfx.Task(t, agentID, testutil.Cols{"runtime_id": runtimeID, "issue_id": issueID})
			req := withURLParam(newRequest("POST", "/api/daemon/runtimes/"+runtimeID+"/tasks/claim", nil), "runtimeId", runtimeID)
			req.Header.Set("X-Client-Capabilities", tc.capability)
			result := testutil.Call(t, testHandler.ClaimTaskByRuntime, req).Want(tc.want)
			if strings.Contains(result.Body.String(), "synthetic-r03-secret") {
				t.Fatal("invalid policy leaked on claim")
			}
			if tc.want == 200 {
				var response struct {
					Task *AgentTaskResponse `json:"task"`
				}
				result.JSON(&response)
				if response.Task == nil || response.Task.ID != taskID || response.Task.Agent == nil {
					t.Fatal("missing claimed task")
				}
				if !strings.Contains(string(response.Task.Agent.McpConfig), "overlayExtension") {
					t.Fatal("claim dropped overlay extension")
				}
				if tc.policy != "" && !strings.Contains(string(response.Task.Agent.McpConfig), "runtimeMcp") {
					t.Fatal("claim dropped selection envelope")
				}
			} else {
				var status string
				if err := testPool.QueryRow(context.Background(), `SELECT status FROM agent_task_queue WHERE id=$1`, taskID).Scan(&status); err != nil {
					t.Fatal(err)
				}
				if status != "cancelled" {
					t.Fatalf("blocked task status=%s", status)
				}
			}
		})
	}
}

func TestR03RuntimeMCPSelectionBatchAndRPCGate(t *testing.T) {
	if testHandler == nil {
		t.Fatal("dedicated database required")
	}
	for _, rpc := range []bool{false, true} {
		for _, hasCapability := range []bool{false, true} {
			t.Run(fmt.Sprintf("rpc=%v/capability=%v", rpc, hasCapability), func(t *testing.T) {
				runtimeID := dbfx.Runtime(t, "R03 batch runtime", testutil.Cols{"provider": "codex"})
				agentID := dbfx.Agent(t, "R03 batch agent", runtimeID, testutil.Cols{"mcp_config": json.RawMessage(`{"_multica":{"runtimeMcp":{"mode":"deny_all"}}}`)})
				issueID := dbfx.Issue(t, "R03 batch issue", testutil.Cols{"assignee_type": "agent", "assignee_id": agentID})
				taskID := dbfx.Task(t, agentID, testutil.Cols{"runtime_id": runtimeID, "issue_id": issueID})
				body := map[string]any{"daemon_id": "r03-batch-daemon", "runtime_ids": []string{runtimeID}, "max_tasks": 1}
				capability := ""
				if hasCapability {
					capability = "runtime-mcp-selection-v1"
				}
				var response struct {
					Tasks []AgentTaskResponse `json:"tasks"`
				}
				if rpc {
					raw, _ := json.Marshal(body)
					status, result, err := testHandler.DaemonRPCHandler(context.Background(), daemonws.ClientIdentity{UserID: testUserID, Capabilities: capability}, "tasks.claim", raw)
					if err != nil || status != 200 {
						t.Fatalf("RPC claim status=%d error=%v", status, err)
					}
					if err := json.Unmarshal(result, &response); err != nil {
						t.Fatal(err)
					}
				} else {
					req := newRequest("POST", "/api/daemon/tasks/claim", body)
					req.Header.Set("X-Client-Capabilities", capability)
					testutil.Call(t, testHandler.ClaimTasksByRuntime, req).Want(http.StatusOK).JSON(&response)
				}
				if hasCapability {
					if len(response.Tasks) != 1 || response.Tasks[0].ID != taskID {
						t.Fatal("capable batch/RPC omitted task")
					}
				} else if len(response.Tasks) != 0 {
					t.Fatal("old batch/RPC daemon received restricted task")
				}
				var status string
				if err := testPool.QueryRow(context.Background(), `SELECT status FROM agent_task_queue WHERE id=$1`, taskID).Scan(&status); err != nil {
					t.Fatal(err)
				}
				want := "cancelled"
				if hasCapability {
					want = "dispatched"
				}
				if status != want {
					t.Fatalf("task status=%s, want=%s", status, want)
				}
			})
		}
	}
}

func TestR03RuntimeMCPSelectionSurvivesWorkspaceAndTaskOverlays(t *testing.T) {
	agentConfig := json.RawMessage(`{"_multica":{"runtimeMcp":{"mode":"deny_all"}},"mcpServers":{"shared":{"command":"agent"}},"overlayExtension":true}`)
	resolved, err := ResolveAgentMcpConfig([]WorkspaceMcpBinding{{Name: "workspace", Config: json.RawMessage(`{"command":"workspace"}`)}}, agentConfig)
	if err != nil {
		t.Fatal(err)
	}
	merged, err := mergeMCPOverlay(resolved, json.RawMessage(`{"_multica":{"runtimeMcp":{"mode":"inherit"}},"mcpServers":{"shared":{"command":"task"}}}`))
	if err != nil {
		t.Fatal(err)
	}
	selection, err := agentconfig.ParseRuntimeMCPSelection(merged)
	if err != nil || selection.Mode != "deny_all" {
		t.Fatal("overlay changed agent-owned runtime selection")
	}
	servers := decodeServers(t, merged)
	if servers["workspace"] == nil || servers["shared"].(map[string]any)["command"] != "task" || !strings.Contains(string(merged), "overlayExtension") {
		t.Fatal("overlay semantics changed")
	}
}
