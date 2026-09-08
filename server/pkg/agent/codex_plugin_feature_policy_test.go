package agent

import (
	"context"
	"encoding/json"
	"errors"
	"reflect"
	"testing"
)

func TestCodexPluginMCPDisabledFeatureInventory(t *testing.T) {
	plan := &codexPluginMCPPlan{home: t.TempDir()}
	opts := ExecOptions{Cwd: t.TempDir(), codexPluginMCP: plan}
	client := pluginMCPRequesterFunc(func(_ context.Context, method string, _ any) (json.RawMessage, error) {
		if method != "config/read" {
			return nil, errors.New("plugin API unavailable when plugins are disabled")
		}
		return json.RawMessage(`{"config":{"features":{"plugins":false},"plugins":{"installed@local":{"enabled":true}}}}`), nil
	})
	inventory, err := discoverCodexPluginMCPWithCache(context.Background(), client, opts)
	if err != nil || len(inventory) != 0 {
		t.Fatalf("disabled feature inventory = %v, %v", inventory, err)
	}
}

func TestCodexPluginMCPDisabledFeatureKeepsOrdinaryOverlay(t *testing.T) {
	plan := &codexPluginMCPPlan{home: t.TempDir()}
	opts := ExecOptions{Cwd: t.TempDir(), codexPluginMCP: plan,
		McpConfig: json.RawMessage(`{"mcpServers":{"ordinary":{"command":"fixture"}}}`)}
	config := map[string]any{
		"features":    map[string]any{"plugins": false},
		"plugins":     map[string]any{"installed@local": map[string]any{"enabled": true}},
		"mcp_servers": map[string]any{"ordinary": map[string]any{"command": "fixture"}},
	}
	client := pluginMCPRequesterFunc(func(_ context.Context, method string, _ any) (json.RawMessage, error) {
		if method != "config/read" {
			return nil, errors.New("plugin API unavailable when plugins are disabled")
		}
		return json.Marshal(map[string]any{"config": config})
	})
	var err error
	plan.inventory, err = discoverCodexPluginMCPWithCache(context.Background(), client, opts)
	if err != nil {
		t.Fatal(err)
	}
	plan.ready = true
	if err := verifyCodexPluginMCPPolicy(context.Background(), client, opts); err != nil {
		t.Fatal(err)
	}
	params := map[string]any{"config": map[string]any{"features": map[string]any{"other": true}}}
	applyCodexPluginMCPOverrides(params, plan)
	features := params["config"].(map[string]any)["features"].(map[string]any)
	if features["plugins"] != false || features["other"] != true {
		t.Fatalf("thread features = %v", features)
	}
	if !reflect.DeepEqual(plan.launchOverrides(), []string{"-c", "features.plugins=false"}) {
		t.Fatal("disabled plugin feature was not bound to the task launch")
	}
	config["mcp_servers"] = map[string]any{"unselected": map[string]any{"command": "fixture"}}
	if err := verifyCodexPluginMCPPolicy(context.Background(), client, opts); err == nil {
		t.Fatal("disabled plugins bypassed ordinary MCP overlay verification")
	}
	config["mcp_servers"] = map[string]any{"ordinary": map[string]any{"command": "fixture"}}
	config["features"] = map[string]any{"plugins": true}
	if err := verifyCodexPluginMCPPolicy(context.Background(), client, opts); err == nil {
		t.Fatal("plugin feature re-enable was accepted")
	}
}

func TestCodexPluginMCPFeatureStateSchema(t *testing.T) {
	for _, tc := range []struct {
		name     string
		config   string
		disabled bool
		invalid  bool
	}{
		{"absent", `{}`, false, false},
		{"empty", `{"features":{}}`, false, false},
		{"enabled", `{"features":{"plugins":true}}`, false, false},
		{"disabled", `{"features":{"plugins":false}}`, true, false},
		{"null features", `{"features":null}`, false, true},
		{"null flag", `{"features":{"plugins":null}}`, false, true},
		{"string flag", `{"features":{"plugins":"false"}}`, false, true},
		{"numeric flag", `{"features":{"plugins":0}}`, false, true},
	} {
		t.Run(tc.name, func(t *testing.T) {
			var config map[string]any
			if err := json.Unmarshal([]byte(tc.config), &config); err != nil {
				t.Fatal(err)
			}
			disabled, err := codexPluginsExplicitlyDisabled(config)
			if disabled != tc.disabled || (err != nil) != tc.invalid {
				t.Fatalf("disabled=%v err=%v", disabled, err)
			}
		})
	}
}

func TestCodexPluginMCPDisabledFeatureRejectsSecondReadDrift(t *testing.T) {
	plan := &codexPluginMCPPlan{home: t.TempDir()}
	opts := ExecOptions{Cwd: t.TempDir(), codexPluginMCP: plan,
		McpConfig: json.RawMessage(`{"mcpServers":{}}`)}
	reads := 0
	client := pluginMCPRequesterFunc(func(_ context.Context, method string, _ any) (json.RawMessage, error) {
		if method != "config/read" {
			t.Fatalf("unexpected request %q", method)
		}
		reads++
		if reads == 3 {
			return json.RawMessage(`{"config":{"features":{"plugins":true},"mcp_servers":{}}}`), nil
		}
		return json.RawMessage(`{"config":{"features":{"plugins":false},"mcp_servers":{}}}`), nil
	})
	var err error
	plan.inventory, err = discoverCodexPluginMCPWithCache(context.Background(), client, opts)
	if err != nil {
		t.Fatal(err)
	}
	plan.ready = true
	if err := verifyCodexPluginMCPPolicy(context.Background(), client, opts); err == nil || reads != 3 {
		t.Fatalf("second-read feature drift accepted, reads=%d error=%v", reads, err)
	}
}
