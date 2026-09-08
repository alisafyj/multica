package agent

import (
	"encoding/json"
	"log/slog"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"testing"
	"time"

	"github.com/pelletier/go-toml/v2"
)

func TestEnsureCodexProjectConfigurationWritesCanonicalTrust(t *testing.T) {
	home := t.TempDir()
	cwd := t.TempDir()
	configPath := filepath.Join(home, "config.toml")
	original := []byte("model = \"home-model\"\n")
	if err := os.WriteFile(configPath, original, 0o644); err != nil {
		t.Fatal(err)
	}
	snapshotPath := filepath.Join(home, "config-before-policy.toml")
	if err := os.Link(configPath, snapshotPath); err != nil {
		t.Fatalf("hard-link original config: %v", err)
	}

	canonicalCwd, err := ensureCodexProjectConfiguration(home, cwd, t.TempDir(), "trusted")
	if err != nil {
		t.Fatalf("ensure trusted project configuration: %v", err)
	}
	wantCwd, err := filepath.EvalSymlinks(cwd)
	if err != nil {
		t.Fatal(err)
	}
	if canonicalCwd != wantCwd {
		t.Fatalf("canonical cwd = %q, want %q", canonicalCwd, wantCwd)
	}

	data, err := os.ReadFile(configPath)
	if err != nil {
		t.Fatal(err)
	}
	var config struct {
		Model    string `toml:"model"`
		Projects map[string]struct {
			TrustLevel string `toml:"trust_level"`
		} `toml:"projects"`
	}
	if err := toml.Unmarshal(data, &config); err != nil {
		t.Fatalf("parse generated config: %v\n%s", err, data)
	}
	if config.Model != "home-model" || config.Projects[canonicalCwd].TrustLevel != "trusted" {
		t.Fatalf("generated config = %+v", config)
	}
	info, err := os.Stat(configPath)
	if err != nil {
		t.Fatal(err)
	}
	if got := info.Mode().Perm(); got != 0o600 {
		t.Fatalf("config mode = %o, want 600", got)
	}
	snapshot, err := os.ReadFile(snapshotPath)
	if err != nil {
		t.Fatal(err)
	}
	if string(snapshot) != string(original) {
		t.Fatalf("original hard-link changed during config replacement: %q", snapshot)
	}
	if leftovers, err := filepath.Glob(filepath.Join(home, ".multica-codex-config-*.tmp")); err != nil || len(leftovers) != 0 {
		t.Fatalf("temporary config leftovers = %v, err = %v", leftovers, err)
	}

	if _, err := ensureCodexProjectConfiguration(home, cwd, t.TempDir(), "restricted"); err != nil {
		t.Fatalf("ensure restricted project configuration: %v", err)
	}
	data, err = os.ReadFile(configPath)
	if err != nil {
		t.Fatal(err)
	}
	if err := toml.Unmarshal(data, &config); err != nil {
		t.Fatal(err)
	}
	if got := config.Projects[canonicalCwd].TrustLevel; got != "untrusted" {
		t.Fatalf("restricted trust_level = %q, want untrusted", got)
	}
}

func TestEnsureCodexProjectConfigurationRejectsGlobalOrNonRegularHome(t *testing.T) {
	userHome := t.TempDir()
	globalCodexHome := filepath.Join(userHome, ".codex")
	if err := os.Mkdir(globalCodexHome, 0o700); err != nil {
		t.Fatal(err)
	}
	if _, err := ensureCodexProjectConfiguration(globalCodexHome, t.TempDir(), userHome, "trusted"); err == nil || !strings.Contains(err.Error(), "user-global") {
		t.Fatalf("global home error = %v", err)
	}

	privateHome := t.TempDir()
	target := filepath.Join(t.TempDir(), "config.toml")
	if err := os.WriteFile(target, []byte("model = \"x\"\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	if err := os.Symlink(target, filepath.Join(privateHome, "config.toml")); err != nil {
		t.Fatal(err)
	}
	if _, err := ensureCodexProjectConfiguration(privateHome, t.TempDir(), userHome, "trusted"); err == nil || !strings.Contains(err.Error(), "regular file") {
		t.Fatalf("symlink config error = %v", err)
	}
}

func TestEnsureCodexMcpConfigReplacesPrivateConfigAtomically(t *testing.T) {
	home := t.TempDir()
	configPath := filepath.Join(home, "config.toml")
	original := []byte("model = \"preserved\"\n")
	if err := os.WriteFile(configPath, original, 0o644); err != nil {
		t.Fatal(err)
	}
	snapshotPath := filepath.Join(home, "config-before-mcp.toml")
	if err := os.Link(configPath, snapshotPath); err != nil {
		t.Fatalf("hard-link original config: %v", err)
	}

	managed := json.RawMessage(`{"mcpServers":{"managed":{"command":"safe","env":{"TOKEN":"secret"}}}}`)
	if err := ensureCodexMcpConfig(configPath, managed, slog.Default()); err != nil {
		t.Fatal(err)
	}
	snapshot, err := os.ReadFile(snapshotPath)
	if err != nil {
		t.Fatal(err)
	}
	if string(snapshot) != string(original) {
		t.Fatalf("original hard-link changed during MCP config replacement: %q", snapshot)
	}
	info, err := os.Stat(configPath)
	if err != nil {
		t.Fatal(err)
	}
	if got := info.Mode().Perm(); got != 0o600 {
		t.Fatalf("managed MCP config mode = %o, want 600", got)
	}
	if leftovers, err := filepath.Glob(filepath.Join(home, ".multica-codex-config-*.tmp")); err != nil || len(leftovers) != 0 {
		t.Fatalf("temporary config leftovers = %v, err = %v", leftovers, err)
	}
}

func TestCodexProjectConfigurationPreflightAllowsManagedParity(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("fake app-server fixture uses a POSIX shell")
	}
	codexHome := t.TempDir()
	cwd := t.TempDir()
	if err := os.Mkdir(filepath.Join(cwd, ".codex"), 0o700); err != nil {
		t.Fatal(err)
	}
	canonicalCwd, err := filepath.EvalSymlinks(cwd)
	if err != nil {
		t.Fatal(err)
	}
	managed := json.RawMessage(`{"mcpServers":{"managed":{"command":"safe","args":["--stdio"]},"disabled-managed":{"command":"never","enabled":false}}}`)
	config := map[string]any{
		"approval_policy":          "never",
		"sandbox_mode":             "workspace-write",
		"shell_environment_policy": map[string]any{"inherit": "none"},
		"features":                 map[string]any{"multi_agent": false, "memories": false},
		"memories":                 map[string]any{"generate_memories": false, "use_memories": false},
		"mcp_servers": map[string]any{
			"managed":          map[string]any{"command": "safe", "args": []any{"--stdio"}},
			"disabled-managed": map[string]any{"command": "never", "enabled": false},
			"disabled-project": map[string]any{"command": "never", "enabled": false},
		},
		"model": "project-native-model",
	}
	layers := []any{map[string]any{"name": map[string]any{"type": "project", "dotCodexFolder": filepath.Join(canonicalCwd, ".codex")}}}
	fakePath := writeFakeCodexAppServer(t, codexProjectPreflightSuccessScript(t, config, layers))

	result := executeFakeCodexWithConfig(t, fakePath, Config{
		Logger:  slog.Default(),
		Env:     map[string]string{"CODEX_HOME": codexHome, "HOME": t.TempDir()},
		WorkDir: cwd,
	}, ExecOptions{
		Cwd:                        cwd,
		Timeout:                    3 * time.Second,
		HandshakeTimeout:           time.Second,
		ProjectConfigurationPolicy: "trusted",
		McpConfig:                  managed,
	})
	if result.Status != "completed" || result.Output != "Done" {
		t.Fatalf("preflight parity result = %+v", result)
	}
}

func TestCodexProjectConfigurationPreflightFailsBeforeThreadStart(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("fake app-server fixture uses a POSIX shell")
	}
	for _, tc := range []struct {
		name      string
		baseline  map[string]any
		effective map[string]any
		mcp       json.RawMessage
		layer     map[string]any
		want      string
	}{
		{
			name:      "managed MCP shadow",
			baseline:  map[string]any{"mcp_servers": map[string]any{}},
			effective: map[string]any{"mcp_servers": map[string]any{"shadow": map[string]any{"command": "do-not-leak-this"}}},
			mcp:       json.RawMessage(`{"mcpServers":{}}`),
			want:      `unexpected_enabled=["shadow"]`,
		},
		{
			name:      "managed MCP override",
			baseline:  map[string]any{"mcp_servers": map[string]any{}},
			effective: map[string]any{"mcp_servers": map[string]any{"managed": map[string]any{"command": "do-not-leak-this"}}},
			mcp:       json.RawMessage(`{"mcpServers":{"managed":{"command":"safe"}}}`),
			want:      `changed=["managed"]`,
		},
		{
			name:      "sandbox override",
			baseline:  map[string]any{"sandbox_mode": "workspace-write"},
			effective: map[string]any{"sandbox_mode": "danger-full-access"},
			want:      "protected Codex setting sandbox_mode",
		},
		{
			name:      "permission profile override",
			baseline:  map[string]any{"default_permissions": ":workspace"},
			effective: map[string]any{"default_permissions": ":full-access"},
			want:      "protected Codex setting default_permissions",
		},
		{
			name:      "sandbox permission override omitted from effective config",
			baseline:  map[string]any{"sandbox_mode": "workspace-write"},
			effective: map[string]any{"sandbox_mode": "workspace-write"},
			layer:     map[string]any{"sandbox_permissions": []any{"disk-full-read-access"}},
			want:      "directly sets protected Codex setting sandbox_permissions",
		},
	} {
		t.Run(tc.name, func(t *testing.T) {
			codexHome := t.TempDir()
			cwd := t.TempDir()
			if err := os.Mkdir(filepath.Join(cwd, ".codex"), 0o700); err != nil {
				t.Fatal(err)
			}
			canonicalCwd, err := filepath.EvalSymlinks(cwd)
			if err != nil {
				t.Fatal(err)
			}
			marker := filepath.Join(t.TempDir(), "thread-started")
			layers := []any{map[string]any{
				"name":   map[string]any{"type": "project", "dotCodexFolder": filepath.Join(canonicalCwd, ".codex")},
				"config": tc.layer,
			}}
			fakePath := writeFakeCodexAppServer(t, codexProjectPreflightFailureScript(t, tc.baseline, tc.effective, layers, marker))

			result := executeFakeCodexWithConfig(t, fakePath, Config{
				Logger:  slog.Default(),
				Env:     map[string]string{"CODEX_HOME": codexHome, "HOME": t.TempDir()},
				WorkDir: cwd,
			}, ExecOptions{
				Cwd:                        cwd,
				Timeout:                    3 * time.Second,
				HandshakeTimeout:           time.Second,
				ProjectConfigurationPolicy: "trusted",
				McpConfig:                  tc.mcp,
			})
			if result.Status != "failed" || !strings.Contains(result.Error, tc.want) {
				t.Fatalf("preflight failure result = %+v", result)
			}
			if strings.Contains(result.Error, "do-not-leak-this") {
				t.Fatalf("preflight error leaked configuration value: %q", result.Error)
			}
			if _, err := os.Stat(marker); !os.IsNotExist(err) {
				t.Fatalf("thread/start was sent after failed preflight: %v", err)
			}
		})
	}
}

func TestCodexMcpNormalizationSkipsOnlyExplicitlyDisabledServers(t *testing.T) {
	expected, err := normalizedManagedCodexMcpServers(json.RawMessage(`{"mcpServers":{"active":{"command":"safe"},"disabled":{"command":"never","enabled":false},"malformed":{"command":"inspect","enabled":"false"}}}`))
	if err != nil {
		t.Fatal(err)
	}
	if _, ok := expected["disabled"]; ok {
		t.Fatal("managed explicitly disabled server was included")
	}
	if _, ok := expected["active"]; !ok {
		t.Fatal("managed active server was omitted")
	}
	if _, ok := expected["malformed"]; !ok {
		t.Fatal("managed malformed enabled state was silently treated as disabled")
	}

	actual, err := effectiveCodexMcpServers(map[string]any{"mcp_servers": map[string]any{
		"active":    map[string]any{"command": "safe"},
		"disabled":  map[string]any{"command": "never", "enabled": false},
		"malformed": map[string]any{"command": "inspect", "enabled": "false"},
	}})
	if err != nil {
		t.Fatal(err)
	}
	if _, ok := actual["disabled"]; ok {
		t.Fatal("effective explicitly disabled server was included")
	}
	if _, ok := actual["active"]; !ok {
		t.Fatal("effective active server was omitted")
	}
	if _, ok := actual["malformed"]; !ok {
		t.Fatal("effective malformed enabled state was silently treated as disabled")
	}
}

func TestCodexProjectConfigurationPreflightFailsWhenConfigReadUnsupported(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("fake app-server fixture uses a POSIX shell")
	}
	codexHome := t.TempDir()
	cwd := t.TempDir()
	fakePath := writeFakeCodexAppServer(t, ""+
		`read line`+"\n"+
		`echo '{"jsonrpc":"2.0","id":1,"result":{}}'`+"\n"+
		`read line`+"\n"+
		`read line`+"\n"+
		`echo '{"jsonrpc":"2.0","id":2,"error":{"code":-32601,"message":"method not found"}}'`+"\n")

	result := executeFakeCodexWithConfig(t, fakePath, Config{
		Logger: slog.Default(),
		Env:    map[string]string{"CODEX_HOME": codexHome, "HOME": t.TempDir()},
	}, ExecOptions{
		Cwd:                        cwd,
		Timeout:                    3 * time.Second,
		HandshakeTimeout:           time.Second,
		ProjectConfigurationPolicy: "restricted",
	})
	if result.Status != "failed" || !strings.Contains(result.Error, "config/read unavailable or rejected") {
		t.Fatalf("unsupported config/read result = %+v", result)
	}
}

func TestVerifyCodexProjectLayerRequiresPolicyEvidence(t *testing.T) {
	cwd := t.TempDir()
	dotCodex := filepath.Join(cwd, ".codex")
	if err := os.Mkdir(dotCodex, 0o700); err != nil {
		t.Fatal(err)
	}
	layer := codexConfigLayer{}
	layer.Name.Type = "project"
	layer.Name.DotCodexFolder = dotCodex

	if err := verifyCodexProjectLayer(cwd, "trusted", []codexConfigLayer{layer}); err != nil {
		t.Fatalf("trusted enabled layer: %v", err)
	}
	layer.DisabledReason = "project is untrusted"
	if err := verifyCodexProjectLayer(cwd, "restricted", []codexConfigLayer{layer}); err != nil {
		t.Fatalf("restricted disabled layer: %v", err)
	}
	if err := verifyCodexProjectLayer(cwd, "trusted", []codexConfigLayer{layer}); err == nil {
		t.Fatal("trusted policy accepted a disabled project layer")
	}
	layer.DisabledReason = ""
	if err := verifyCodexProjectLayer(cwd, "restricted", []codexConfigLayer{layer}); err == nil {
		t.Fatal("restricted policy accepted an enabled project layer")
	}
	if err := verifyCodexProjectLayer(cwd, "restricted", nil); err == nil {
		t.Fatal("restricted policy accepted missing layer evidence")
	}
}

func executeFakeCodexWithConfig(t *testing.T, fakePath string, cfg Config, opts ExecOptions) Result {
	t.Helper()
	result, _ := executeFakeCodexCollectingMessagesWithConfig(t, fakePath, cfg, opts, 10*time.Second)
	return result
}

func codexProjectPreflightSuccessScript(t *testing.T, config map[string]any, layers []any) string {
	t.Helper()
	return "" +
		`read line` + "\n" +
		`echo '{"jsonrpc":"2.0","id":1,"result":{}}'` + "\n" +
		`read line` + "\n" +
		`read line` + "\n" +
		codexConfigReadEcho(t, 2, config, nil) + "\n" +
		`read line` + "\n" +
		codexConfigReadEcho(t, 3, config, layers) + "\n" +
		`read line` + "\n" +
		`echo '{"jsonrpc":"2.0","id":4,"result":{"thread":{"id":"thr-policy"}}}'` + "\n" +
		`read line` + "\n" +
		`echo '{"jsonrpc":"2.0","id":5,"result":{}}'` + "\n" +
		`echo '{"jsonrpc":"2.0","method":"item/completed","params":{"threadId":"thr-policy","turnId":"turn-policy","item":{"type":"agentMessage","id":"final","phase":"final_answer","text":"Done"}}}'` + "\n" +
		`echo '{"jsonrpc":"2.0","method":"turn/completed","params":{"threadId":"thr-policy","turn":{"id":"turn-policy","status":"completed"}}}'` + "\n"
}

func codexProjectPreflightFailureScript(t *testing.T, baseline, effective map[string]any, layers []any, marker string) string {
	t.Helper()
	return "" +
		`read line` + "\n" +
		`echo '{"jsonrpc":"2.0","id":1,"result":{}}'` + "\n" +
		`read line` + "\n" +
		`read line` + "\n" +
		codexConfigReadEcho(t, 2, baseline, nil) + "\n" +
		`read line` + "\n" +
		codexConfigReadEcho(t, 3, effective, layers) + "\n" +
		`if read line; then printf reached > ` + shellQuote(marker) + `; fi` + "\n"
}

func codexConfigReadEcho(t *testing.T, id int, config map[string]any, layers []any) string {
	t.Helper()
	packet, err := json.Marshal(map[string]any{
		"jsonrpc": "2.0",
		"id":      id,
		"result":  map[string]any{"config": config, "layers": layers},
	})
	if err != nil {
		t.Fatal(err)
	}
	return "printf '%s\\n' " + shellQuote(string(packet))
}
