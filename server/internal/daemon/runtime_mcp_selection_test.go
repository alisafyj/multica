package daemon

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/multica-ai/multica/server/internal/handler"
)

func writeRuntimeMCPSelectionFixture(t *testing.T, provider string) {
	t.Helper()
	home := t.TempDir()
	t.Setenv("HOME", home)
	t.Setenv("CODEX_HOME", filepath.Join(home, ".codex"))
	if err := os.MkdirAll(filepath.Join(home, ".codex"), 0o700); err != nil {
		t.Fatal(err)
	}
	file := filepath.Join(home, ".claude.json")
	config := `{"mcpServers":{"docs":{"command":"docs-server","env":{"FIXTURE":"local-selection-secret"}},"extra":{"command":"extra-server"},"off":{"command":"off-server","enabled":false},"disabled":{"command":"disabled-server","disabled":true},"screen-control":{"command":"screen-server"},"bad":null}}`
	if provider == "codex" {
		file = filepath.Join(home, ".codex", "config.toml")
		config = "[mcp_servers.docs]\ncommand = 'docs-server'\n[mcp_servers.docs.env]\nFIXTURE = 'local-selection-secret'\n[mcp_servers.extra]\ncommand = 'extra-server'\n[mcp_servers.off]\ncommand = 'off-server'\nenabled = false\n[mcp_servers.disabled]\ncommand = 'disabled-server'\ndisabled = true\n[mcp_servers.screen-control]\ncommand = 'screen-server'\n[mcp_servers.bad]\nenabled = 'invalid'\n"
	}
	if err := os.WriteFile(file, []byte(config), 0o600); err != nil {
		t.Fatal(err)
	}
}

func runtimeMCPSelectionNames(t *testing.T, raw json.RawMessage) map[string]map[string]any {
	t.Helper()
	var doc struct {
		Servers map[string]map[string]any `json:"mcpServers"`
	}
	if err := json.Unmarshal(raw, &doc); err != nil {
		t.Fatal(err)
	}
	if strings.Contains(string(raw), "_multica") {
		t.Fatal("runtime selection envelope reached provider configuration")
	}
	return doc.Servers
}

func TestRuntimeMCPSelectionModesAndOverlayPrecedence(t *testing.T) {
	for _, provider := range []string{"codex", "claude"} {
		t.Run(provider, func(t *testing.T) {
			writeRuntimeMCPSelectionFixture(t, provider)
			for _, tc := range []struct {
				name, policy        string
				wantDocs, wantExtra bool
			}{
				{"missing", "", true, true},
				{"inherit", `,"_multica":{"runtimeMcp":{"mode":"inherit"}}`, true, true},
				{"deny_all", `,"_multica":{"runtimeMcp":{"mode":"deny_all"}}`, false, false},
				{"allowlist", `,"_multica":{"runtimeMcp":{"mode":"allowlist","allow":["docs"]}}`, true, false},
				{"empty_allowlist", `,"_multica":{"runtimeMcp":{"mode":"allowlist","allow":[]}}`, false, false},
			} {
				t.Run(tc.name, func(t *testing.T) {
					agentConfig := json.RawMessage(`{"mcpServers":{"agent":{"command":"agent-server"}},"overlayExtension":{"enabled":true}` + tc.policy + `}`)
					resolved, err := handler.ResolveAgentMcpConfig([]handler.WorkspaceMcpBinding{{Name: "workspace", Config: json.RawMessage(`{"command":"workspace-server"}`)}}, agentConfig)
					if err != nil {
						t.Fatal(err)
					}
					if !strings.Contains(string(resolved), "overlayExtension") {
						t.Fatal("workspace merge dropped extension")
					}
					merged, err := mergeRuntimeAndAgentMcpConfig(provider, resolved)
					if err != nil {
						t.Fatal(err)
					}
					servers := runtimeMCPSelectionNames(t, merged)
					if (servers["docs"] != nil) != tc.wantDocs || (servers["extra"] != nil) != tc.wantExtra {
						t.Fatalf("unexpected runtime selection for %s", tc.name)
					}
					if servers["agent"]["command"] != "agent-server" || servers["workspace"]["command"] != "workspace-server" {
						t.Fatal("selection filtered explicit overlays")
					}
					if tc.wantDocs && servers["docs"]["env"].(map[string]any)["FIXTURE"] != "local-selection-secret" {
						t.Fatal("selected runtime secret did not stay local")
					}
				})
			}
			merged, err := mergeRuntimeAndAgentMcpConfig(provider, json.RawMessage(`{"_multica":{"runtimeMcp":{"mode":"allowlist","allow":["docs"]}},"mcpServers":{"docs":{"command":"agent-wins"}}}`))
			if err != nil {
				t.Fatal(err)
			}
			if runtimeMCPSelectionNames(t, merged)["docs"]["command"] != "agent-wins" {
				t.Fatal("agent overlay lost precedence")
			}
		})
	}
}

func TestRuntimeMCPSelectionRequiredNamesFailClosedBeforeOverlays(t *testing.T) {
	for _, provider := range []string{"codex", "claude"} {
		t.Run(provider, func(t *testing.T) {
			writeRuntimeMCPSelectionFixture(t, provider)
			for _, name := range []string{"missing", "off", "disabled", "screen-control", "bad", "Docs"} {
				t.Run(name, func(t *testing.T) {
					raw, _ := json.Marshal(map[string]any{"_multica": map[string]any{"runtimeMcp": map[string]any{"mode": "allowlist", "allow": []string{name}}}, "mcpServers": map[string]any{name: map[string]any{"command": "overlay-cannot-rescue-required-runtime"}}})
					merged, err := mergeRuntimeAndAgentMcpConfig(provider, raw)
					if err == nil || merged != nil {
						t.Fatal("unavailable required runtime entry did not fail closed")
					}
					if strings.Contains(err.Error(), "local-selection-secret") {
						t.Fatal("runtime secret escaped in error")
					}
				})
			}
		})
	}
}

func TestRuntimeMCPSelectionRejectsUnsupportedProvidersAndTypos(t *testing.T) {
	t.Setenv("HOME", t.TempDir())
	for _, provider := range []string{"unknown", "openclaw", "opencode", "codebuddy", "kimi", "cursor", "codearts"} {
		for _, mode := range []string{"deny_all", "allowlist"} {
			raw := json.RawMessage(`{"_multica":{"runtimeMcp":{"mode":"` + mode + `"`)
			if mode == "allowlist" {
				raw = append(raw, `,"allow":[]`...)
			}
			raw = append(raw, `}}}`...)
			if _, err := mergeRuntimeAndAgentMcpConfig(provider, raw); err == nil {
				t.Errorf("%s silently accepted %s", provider, mode)
			}
		}
	}
	for _, raw := range []string{`{"_multica":{"runtimeMcp":{"mode":"deny-all"}}}`, `{"_multica":{"runtimeMcp":{"Mode":"deny_all"}}}`} {
		if _, err := mergeRuntimeAndAgentMcpConfig("claude", json.RawMessage(raw)); err == nil {
			t.Error("policy typo silently accepted")
		}
	}
}

func TestRuntimeMCPSelectionStripsUnknownMetadata(t *testing.T) {
	for _, provider := range []string{"codex", "claude"} {
		t.Run(provider, func(t *testing.T) {
			writeRuntimeMCPSelectionFixture(t, provider)
			for _, tc := range []struct {
				name, policy        string
				wantDocs, wantExtra bool
			}{
				{"missing", "", true, true},
				{"inherit", `,"runtimeMcp":{"mode":"inherit"}`, true, true},
				{"deny_all", `,"runtimeMcp":{"mode":"deny_all"}`, false, false},
				{"allowlist", `,"runtimeMcp":{"mode":"allowlist","allow":["docs"]}`, true, false},
				{"empty_allowlist", `,"runtimeMcp":{"mode":"allowlist","allow":[]}`, false, false},
			} {
				t.Run(tc.name, func(t *testing.T) {
					raw := json.RawMessage(`{"mcpServers":{"agent":{"command":"agent-server"}},"_multica":{"future":{"token":"synthetic-metadata-secret"},"futureOther":{"mode":"deny_all"}` + tc.policy + `}}`)
					before := string(raw)
					merged, err := mergeRuntimeAndAgentMcpConfig(provider, raw)
					if err != nil {
						t.Fatal(err)
					}
					servers := runtimeMCPSelectionNames(t, merged)
					if strings.Contains(string(merged), "synthetic-metadata-secret") || strings.Contains(string(merged), "future") {
						t.Fatal("unknown metadata reached provider configuration")
					}
					if (servers["docs"] != nil) != tc.wantDocs || (servers["extra"] != nil) != tc.wantExtra || servers["agent"]["command"] != "agent-server" {
						t.Fatal("unknown metadata changed runtime selection or explicit overlay")
					}
					if string(raw) != before {
						t.Fatal("provider preparation mutated saved metadata")
					}
				})
			}
		})
	}
}

func TestRuntimeMCPSelectionCapabilityAdvertised(t *testing.T) {
	for _, capability := range strings.Split(daemonClientCapabilities(), ",") {
		if capability == "runtime-mcp-selection-v1" {
			return
		}
	}
	t.Fatal("daemon does not advertise runtime-mcp-selection-v1")
}

func TestRuntimeMCPSelectionDoesNotReadDeniedRuntimeConfiguration(t *testing.T) {
	for _, provider := range []string{"codex", "claude"} {
		t.Run(provider, func(t *testing.T) {
			writeRuntimeMCPSelectionFixture(t, provider)
			file := filepath.Join(os.Getenv("HOME"), ".claude.json")
			if provider == "codex" {
				file = filepath.Join(os.Getenv("CODEX_HOME"), "config.toml")
			}
			if err := os.WriteFile(file, []byte("synthetic-invalid-config-secret {"), 0o600); err != nil {
				t.Fatal(err)
			}
			for _, policy := range []string{`{"mode":"deny_all"}`, `{"mode":"allowlist","allow":[]}`} {
				merged, err := mergeRuntimeAndAgentMcpConfig(provider, json.RawMessage(`{"_multica":{"runtimeMcp":`+policy+`}}`))
				if err != nil || len(runtimeMCPSelectionNames(t, merged)) != 0 {
					t.Fatal("denied runtime config was read")
				}
			}
			for _, config := range []json.RawMessage{nil, json.RawMessage(`{"_multica":{"runtimeMcp":{"mode":"inherit"}}}`)} {
				if merged, err := mergeRuntimeAndAgentMcpConfig(provider, config); err == nil || merged != nil {
					t.Fatal("inherit silently ignored malformed runtime configuration")
				}
			}
			_, err := mergeRuntimeAndAgentMcpConfig(provider, json.RawMessage(`{"_multica":{"runtimeMcp":{"mode":"allowlist","allow":["docs"]}}}`))
			if err == nil || strings.Contains(err.Error(), "synthetic-invalid-config-secret") {
				t.Fatal("required unreadable runtime did not return a safe failure")
			}
			if original, err := os.ReadFile(file); err != nil || string(original) != "synthetic-invalid-config-secret {" {
				t.Fatal("runtime configuration was modified")
			}
		})
	}
}
