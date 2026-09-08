package handler

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	db "github.com/multica-ai/multica/server/pkg/db/generated"
)

func TestAgentQuickCreateCapabilitiesRespectVisibilityAndVersion(t *testing.T) {
	if testHandler == nil || testPool == nil {
		t.Skip("database not available")
	}

	ctx := context.Background()
	allowedMember := createPermissionTestMember(t, "quick-create-capability-allowed@multica.test")
	outsiderMember := createPermissionTestMember(t, "quick-create-capability-outsider@multica.test")

	var runtimeID string
	if err := testPool.QueryRow(ctx, `
		INSERT INTO agent_runtime (
			workspace_id, daemon_id, name, runtime_mode, provider, status,
			device_info, metadata, owner_id, visibility, last_seen_at
		)
		VALUES ($1, NULL, 'Quick Create Capability Runtime', 'cloud',
			'quick_create_capability', 'online', '',
			'{"cli_version":"0.4.3"}'::jsonb, $2, 'private', now())
		RETURNING id
	`, testWorkspaceID, testUserID).Scan(&runtimeID); err != nil {
		t.Fatalf("create runtime: %v", err)
	}
	t.Cleanup(func() {
		testPool.Exec(context.Background(), `DELETE FROM agent_runtime WHERE id = $1`, runtimeID)
	})

	var agentID string
	if err := testPool.QueryRow(ctx, `
		INSERT INTO agent (
			workspace_id, name, description, runtime_mode, runtime_config,
			runtime_id, visibility, permission_mode, max_concurrent_tasks,
			owner_id, instructions, custom_env, custom_args, mcp_config
		)
		VALUES ($1, 'quick-create-capability-agent', '', 'cloud', '{}'::jsonb,
			$2, 'private', 'public_to', 1, $3, '', '{}'::jsonb, '[]'::jsonb, '{}'::jsonb)
		RETURNING id
	`, testWorkspaceID, runtimeID, testUserID).Scan(&agentID); err != nil {
		t.Fatalf("create agent: %v", err)
	}
	t.Cleanup(func() {
		testPool.Exec(context.Background(), `DELETE FROM agent_invocation_target WHERE agent_id = $1`, agentID)
		testPool.Exec(context.Background(), `DELETE FROM agent WHERE id = $1`, agentID)
	})
	if _, err := testPool.Exec(ctx, `
		INSERT INTO agent_invocation_target (agent_id, target_type, target_id, created_by)
		VALUES ($1, 'member', $2, $3)
	`, agentID, allowedMember, testUserID); err != nil {
		t.Fatalf("create invocation target: %v", err)
	}

	assertListCapability := func(t *testing.T, userID string, wantBase, wantFields bool) {
		t.Helper()
		w := httptest.NewRecorder()
		testHandler.ListAgents(w, newRequestAs(userID, http.MethodGet, "/api/agents", nil))
		if w.Code != http.StatusOK {
			t.Fatalf("ListAgents: got %d: %s", w.Code, w.Body.String())
		}
		var agents []AgentResponse
		if err := json.NewDecoder(w.Body).Decode(&agents); err != nil {
			t.Fatalf("decode ListAgents: %v", err)
		}
		for _, got := range agents {
			if got.ID != agentID {
				continue
			}
			if got.QuickCreateSupported == nil || *got.QuickCreateSupported != wantBase {
				t.Fatalf("quick_create_supported = %v, want %v", got.QuickCreateSupported, wantBase)
			}
			if got.QuickCreateFieldsSupported == nil || *got.QuickCreateFieldsSupported != wantFields {
				t.Fatalf("quick_create_fields_supported = %v, want %v", got.QuickCreateFieldsSupported, wantFields)
			}
			return
		}
		t.Fatalf("agent %s not returned", agentID)
	}

	// Both the owner and the allow-listed member receive the same coarse
	// capability projection even though the member cannot list this private runtime.
	assertListCapability(t, testUserID, true, true)
	assertListCapability(t, allowedMember, true, true)
	if _, err := testPool.Exec(ctx, `UPDATE agent_runtime SET status = 'offline' WHERE id = $1`, runtimeID); err != nil {
		t.Fatalf("set runtime offline: %v", err)
	}
	assertListCapability(t, allowedMember, true, true)

	memberRuntimeList := httptest.NewRecorder()
	testHandler.ListAgentRuntimes(memberRuntimeList, newRequestAs(allowedMember, http.MethodGet, "/api/runtimes", nil))
	if memberRuntimeList.Code != http.StatusOK {
		t.Fatalf("ListAgentRuntimes: got %d: %s", memberRuntimeList.Code, memberRuntimeList.Body.String())
	}
	if listContainsRuntimeID(t, memberRuntimeList.Body.Bytes(), runtimeID) {
		t.Fatalf("private runtime %s leaked to allow-listed member", runtimeID)
	}

	outsiderList := httptest.NewRecorder()
	testHandler.ListAgents(outsiderList, newRequestAs(outsiderMember, http.MethodGet, "/api/agents", nil))
	if outsiderList.Code != http.StatusOK {
		t.Fatalf("ListAgents outsider: got %d: %s", outsiderList.Code, outsiderList.Body.String())
	}
	if listContainsAgent(t, outsiderList.Body.Bytes(), agentID) {
		t.Fatalf("non-allow-listed member received agent %s", agentID)
	}
	outsiderDetail := httptest.NewRecorder()
	testHandler.GetAgent(outsiderDetail, withURLParam(
		newRequestAs(outsiderMember, http.MethodGet, "/api/agents/"+agentID, nil), "id", agentID))
	if outsiderDetail.Code != http.StatusForbidden {
		t.Fatalf("GetAgent outsider: got %d, want 403: %s", outsiderDetail.Code, outsiderDetail.Body.String())
	}

	// The detail endpoint must expose the same projection to an authorized member.
	detail := httptest.NewRecorder()
	testHandler.GetAgent(detail, withURLParam(
		newRequestAs(allowedMember, http.MethodGet, "/api/agents/"+agentID, nil), "id", agentID))
	if detail.Code != http.StatusOK {
		t.Fatalf("GetAgent: got %d: %s", detail.Code, detail.Body.String())
	}
	var detailAgent AgentResponse
	if err := json.NewDecoder(detail.Body).Decode(&detailAgent); err != nil {
		t.Fatalf("decode GetAgent: %v", err)
	}
	if detailAgent.QuickCreateSupported == nil || !*detailAgent.QuickCreateSupported {
		t.Fatalf("GetAgent quick_create_supported = %v, want true", detailAgent.QuickCreateSupported)
	}

	if _, err := testPool.Exec(ctx, `
		UPDATE agent_runtime SET metadata = '{"cli_version":"0.3.0"}'::jsonb WHERE id = $1
	`, runtimeID); err != nil {
		t.Fatalf("downgrade runtime metadata: %v", err)
	}
	assertListCapability(t, allowedMember, true, false)

	if _, err := testPool.Exec(ctx, `
		UPDATE agent_runtime SET metadata = '{"cli_version":"0.2.20"}'::jsonb WHERE id = $1
	`, runtimeID); err != nil {
		t.Fatalf("downgrade runtime metadata below base floor: %v", err)
	}
	assertListCapability(t, allowedMember, false, false)

	if _, err := testPool.Exec(ctx, `UPDATE agent_runtime SET metadata = '{}'::jsonb WHERE id = $1`, runtimeID); err != nil {
		t.Fatalf("remove runtime CLI metadata: %v", err)
	}
	assertListCapability(t, allowedMember, false, false)
}

func TestAgentToResponseProjectsUnboundQuickCreateAsUnsupported(t *testing.T) {
	resp := (&Handler{}).agentToResponse(db.Agent{})
	if resp.QuickCreateSupported == nil || *resp.QuickCreateSupported {
		t.Fatalf("quick_create_supported = %v, want false", resp.QuickCreateSupported)
	}
	if resp.QuickCreateFieldsSupported == nil || *resp.QuickCreateFieldsSupported {
		t.Fatalf("quick_create_fields_supported = %v, want false", resp.QuickCreateFieldsSupported)
	}
}

func listContainsRuntimeID(t *testing.T, body []byte, runtimeID string) bool {
	t.Helper()
	var runtimes []AgentRuntimeResponse
	if err := json.Unmarshal(body, &runtimes); err != nil {
		t.Fatalf("decode runtime list: %v", err)
	}
	for _, runtime := range runtimes {
		if runtime.ID == runtimeID {
			return true
		}
	}
	return false
}
