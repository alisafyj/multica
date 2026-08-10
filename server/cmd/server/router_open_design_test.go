package main

import (
	"bytes"
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/go-chi/chi/v5"
	"github.com/multica-ai/multica/server/internal/analytics"
	"github.com/multica-ai/multica/server/internal/events"
	"github.com/multica-ai/multica/server/internal/realtime"
)

func TestOpenDesignFeatureFlagUsesNativeV2Task(t *testing.T) {
	t.Setenv("MULTICA_OPEN_DESIGN_ENABLED", "true")
	ctx := context.Background()

	var projectID string
	if err := testPool.QueryRow(ctx, `
		INSERT INTO project (workspace_id, title)
		VALUES ($1, 'Open Design router flag')
		RETURNING id
	`, testWorkspaceID).Scan(&projectID); err != nil {
		t.Fatalf("create project: %v", err)
	}

	var runtimeID string
	if err := testPool.QueryRow(ctx, `
		INSERT INTO agent_runtime (
			workspace_id, daemon_id, name, runtime_mode, provider,
			status, device_info, metadata, last_seen_at
		)
		VALUES ($1, NULL, 'Open Design router runtime', 'cloud', 'opencode',
			'online', 'router test', '{}'::jsonb, now())
		RETURNING id
	`, testWorkspaceID).Scan(&runtimeID); err != nil {
		t.Fatalf("create runtime: %v", err)
	}

	var agentID string
	if err := testPool.QueryRow(ctx, `
		INSERT INTO agent (
			workspace_id, name, description, runtime_mode, runtime_config,
			runtime_id, visibility, max_concurrent_tasks, owner_id
		)
		VALUES ($1, 'Open Design router agent', '', 'cloud', '{}'::jsonb,
			$2, 'workspace', 1, $3)
		RETURNING id
	`, testWorkspaceID, runtimeID, testUserID).Scan(&agentID); err != nil {
		t.Fatalf("create agent: %v", err)
	}
	t.Cleanup(func() {
		_, _ = testPool.Exec(context.Background(), `DELETE FROM project WHERE id = $1`, projectID)
		_, _ = testPool.Exec(context.Background(), `DELETE FROM agent WHERE id = $1`, agentID)
		_, _ = testPool.Exec(context.Background(), `DELETE FROM agent_runtime WHERE id = $1`, runtimeID)
	})

	router := NewRouter(testPool, realtime.NewHub(), events.New(), analytics.NoopClient{}, nil)
	server := httptest.NewServer(router)
	t.Cleanup(server.Close)

	body, err := json.Marshal(map[string]any{
		"project_id": projectID,
		"agent_id":   agentID,
		"platform":   "web",
		"brief":      "Create a compact CRM design system.",
	})
	if err != nil {
		t.Fatalf("marshal request: %v", err)
	}
	req, err := http.NewRequest(http.MethodPost, server.URL+"/api/project-design-systems", bytes.NewReader(body))
	if err != nil {
		t.Fatalf("create request: %v", err)
	}
	req.Header.Set("Authorization", "Bearer "+testToken)
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("X-Workspace-ID", testWorkspaceID)
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatalf("create design system: %v", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusAccepted {
		t.Fatalf("create design system status = %d, want %d", resp.StatusCode, http.StatusAccepted)
	}

	var response struct {
		ID         string `json:"id"`
		ActiveTask *struct {
			ID string `json:"id"`
		} `json:"active_task"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&response); err != nil {
		t.Fatalf("decode design system response: %v", err)
	}
	if response.ActiveTask == nil {
		t.Fatal("create response has no active task")
	}
	var packageSchema string
	if err := testPool.QueryRow(ctx, `
		SELECT context ->> 'package_schema'
		FROM agent_task_queue
		WHERE id = $1
	`, response.ActiveTask.ID).Scan(&packageSchema); err != nil {
		t.Fatalf("load native design task: %v", err)
	}
	if packageSchema != "multica.project-design-system/v2" {
		t.Fatalf("native design task package schema = %q", packageSchema)
	}
	var openDesignRunCount int
	if err := testPool.QueryRow(ctx, `
		SELECT COUNT(*)
		FROM open_design_run
		WHERE design_system_id = $1
	`, response.ID).Scan(&openDesignRunCount); err != nil {
		t.Fatalf("count Open Design runs: %v", err)
	}
	if openDesignRunCount != 0 {
		t.Fatalf("Open Design run count = %d, want 0", openDesignRunCount)
	}
}

func TestOpenDesignDaemonLifecycleRoutesAreRegistered(t *testing.T) {
	router := NewRouter(nil, realtime.NewHub(), events.New(), analytics.NoopClient{}, nil)
	want := map[string]bool{
		"/api/daemon/tasks/{taskId}/open-design/base-archive": false,
		"/api/daemon/tasks/{taskId}/open-design/preflight":    false,
		"/api/daemon/tasks/{taskId}/open-design/start":        false,
		"/api/daemon/tasks/{taskId}/open-design/events":       false,
		"/api/daemon/tasks/{taskId}/open-design/archive":      false,
		"/api/daemon/tasks/{taskId}/open-design/result":       false,
		"/api/daemon/tasks/{taskId}/open-design/audit":        false,
		"/api/daemon/tasks/{taskId}/open-design/preview":      false,
		"/api/daemon/tasks/{taskId}/open-design/terminal":     false,
	}
	err := chi.Walk(router, func(method, route string, _ http.Handler, _ ...func(http.Handler) http.Handler) error {
		if method == http.MethodPost || method == http.MethodGet {
			if _, exists := want[route]; exists {
				want[route] = true
			}
		}
		return nil
	})
	if err != nil {
		t.Fatalf("walk router: %v", err)
	}
	for route, found := range want {
		if !found {
			t.Errorf("POST %s is not registered", route)
		}
	}
}

func TestOpenDesignArchivePreviewRoutesAreRegistered(t *testing.T) {
	router := NewRouter(nil, realtime.NewHub(), events.New(), analytics.NoopClient{}, nil)
	want := map[string]bool{
		"/api/project-design-systems/{id}/open-design-preview":                                        false,
		"/api/project-design-system-previews/{workspaceId}/{systemId}/{digest}/{accessToken}/files/*": false,
	}
	err := chi.Walk(router, func(method, route string, _ http.Handler, _ ...func(http.Handler) http.Handler) error {
		if method == http.MethodGet {
			if _, exists := want[route]; exists {
				want[route] = true
			}
		}
		return nil
	})
	if err != nil {
		t.Fatalf("walk router: %v", err)
	}
	for route, found := range want {
		if !found {
			t.Errorf("GET %s is not registered", route)
		}
	}
}
