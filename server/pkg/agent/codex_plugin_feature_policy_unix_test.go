//go:build !windows

package agent

import (
	"context"
	"encoding/json"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"

	"github.com/pelletier/go-toml/v2"
)

func TestCodexPluginFeatureDisabledWireStartResumeAndFallback(t *testing.T) {
	for _, path := range []string{"start", "resume", "fallback"} {
		t.Run(path, func(t *testing.T) {
			cfg, opts := pluginMCPTestOptions(t)
			if path != "start" {
				opts.ResumeSessionID = "prior-thread"
			}
			opts.CustomArgs = []string{"-c", "features.plugins=false"}
			opts.McpConfig = json.RawMessage(`{"mcpServers":{"ordinary":{"command":"safe-tool"}}}`)
			effective := pluginEffectiveFixture()
			effective["features"] = map[string]any{"plugins": false}
			effective["mcp_servers"] = map[string]any{"ordinary": map[string]any{"command": "safe-tool"}}

			configPath := filepath.Join(cfg.Env["CODEX_HOME"], "config.toml")
			before, err := os.ReadFile(configPath)
			if err != nil {
				t.Fatal(err)
			}
			result, err, _ := runPluginMCPTest(t, context.Background(), pluginMCPProtocolScript(t, effective, path == "fallback", false), cfg, opts)
			if err != nil || result.Status != "completed" || result.Output != "Done" {
				t.Fatalf("result=%+v err=%v", result, err)
			}
			after, err := os.ReadFile(configPath)
			if err != nil {
				t.Fatal(err)
			}
			var beforeConfig, afterConfig map[string]any
			if toml.Unmarshal(before, &beforeConfig) != nil || toml.Unmarshal(after, &afterConfig) != nil {
				t.Fatal("private plugin configuration is not valid TOML")
			}
			if !reflect.DeepEqual(afterConfig["plugins"], beforeConfig["plugins"]) {
				t.Fatal("feature-disabled preparation changed the plugin configuration subtree")
			}
			plugin := afterConfig["plugins"].(map[string]any)["probe@local"].(map[string]any)
			if plugin["enabled"] != true || plugin["mcp_servers"] != nil {
				t.Fatalf("feature-disabled preparation wrote plugin MCP policy: %v", plugin)
			}
			ordinaryCommand, ok := nestedCodexConfigValue(afterConfig, []string{"mcp_servers", "ordinary", "command"})
			if !ok || ordinaryCommand != "safe-tool" {
				t.Fatalf("feature-disabled preparation changed ordinary MCP: %v", ordinaryCommand)
			}

			rawRPC, err := os.ReadFile(filepath.Join(cfg.Env["CODEX_HOME"], "test-rpc"))
			if err != nil {
				t.Fatal(err)
			}
			threadMethods := make([]string, 0, 2)
			for _, line := range strings.Split(strings.TrimSpace(string(rawRPC)), "\n") {
				phase, packet, ok := strings.Cut(line, " ")
				if !ok {
					t.Fatalf("malformed protocol capture: %q", line)
				}
				if strings.Contains(packet, `"method":"plugin/`) {
					t.Fatalf("plugin RPC reached feature-disabled protocol: %s", packet)
				}
				var request struct {
					Method string `json:"method"`
					Params struct {
						Config map[string]any `json:"config"`
					} `json:"params"`
				}
				if json.Unmarshal([]byte(packet), &request) != nil {
					t.Fatalf("invalid protocol capture: %q", packet)
				}
				if phase == "first" && (strings.HasPrefix(request.Method, "thread/") || strings.HasPrefix(request.Method, "turn/")) {
					t.Fatalf("preflight reached %s", request.Method)
				}
				if phase != "second" || (request.Method != "thread/start" && request.Method != "thread/resume") {
					continue
				}
				threadMethods = append(threadMethods, request.Method)
				features, ok := request.Params.Config["features"].(map[string]any)
				if !ok || features["plugins"] != false {
					t.Fatalf("%s omitted disabled plugin feature: %v", request.Method, request.Params.Config)
				}
			}
			want := map[string][]string{
				"start":    {"thread/start"},
				"resume":   {"thread/resume"},
				"fallback": {"thread/resume", "thread/start"},
			}[path]
			if strings.Join(threadMethods, ",") != strings.Join(want, ",") {
				t.Fatalf("thread methods = %v, want %v", threadMethods, want)
			}
		})
	}
}
