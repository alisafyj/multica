//go:build !windows

package agent

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"log/slog"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"syscall"
	"testing"
	"time"

	"github.com/multica-ai/multica/server/internal/agentconfig"
	"github.com/pelletier/go-toml/v2"
)

func pluginMCPTestOptions(t *testing.T) (Config, ExecOptions) {
	t.Helper()
	home, cwd := t.TempDir(), t.TempDir()
	if err := os.WriteFile(filepath.Join(home, "config.toml"), []byte("[plugins.\"probe@local\"]\nenabled = true\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	return Config{CodexVersion: "0.153.4", BuiltinRuntime: true, Env: map[string]string{"CODEX_HOME": home}}, ExecOptions{
		Cwd: cwd, McpConfig: json.RawMessage(`{"mcpServers":{}}`),
		RuntimeMCPSelection: &agentconfig.RuntimeMCPSelection{Mode: "deny_all"},
		HandshakeTimeout:    time.Second,
	}
}

func runPluginMCPTest(t *testing.T, ctx context.Context, script string, cfg Config, opts ExecOptions) (Result, error, string) {
	t.Helper()
	ctx, cancel := context.WithTimeout(ctx, 8*time.Second)
	defer cancel()
	var logs bytes.Buffer
	cfg.ExecutablePath = writeFakeCodexAppServer(t, script)
	cfg.Logger = slog.New(slog.NewTextHandler(&logs, nil))
	b, err := New("codex", cfg)
	if err != nil {
		t.Fatal(err)
	}
	session, err := b.Execute(ctx, "fixed offline test", opts)
	if err != nil {
		return Result{}, err, logs.String()
	}
	for range session.Messages {
	}
	result := <-session.Result
	return result, nil, logs.String()
}

const pluginInstalledFixture = `{"marketplaces":[{"name":"local","path":"/fixed/marketplace.json","plugins":[{"id":"probe@local","name":"probe","installed":true,"enabled":true}]}],"marketplaceLoadErrors":[]}`
const pluginReadFixture = `{"plugin":{"summary":{"id":"probe@local","name":"probe","installed":true,"enabled":true},"marketplaceName":"local","mcpServers":["raw.server","unicode_工具"]}}`

func pluginEffectiveFixture() map[string]any {
	return map[string]any{"plugins": map[string]any{"probe@local": map[string]any{
		"enabled": true, "mcp_servers": map[string]any{
			"raw.server": map[string]any{"enabled": false}, "unicode_工具": map[string]any{"enabled": false},
		},
	}}, "mcp_servers": map[string]any{}}
}

func pluginMCPProtocolScript(t *testing.T, effective map[string]any, resumeFallback bool, inventoryError bool) string {
	t.Helper()
	initialEffective := pluginEffectiveFixture()
	if features, ok := effective["features"]; ok {
		initialEffective["features"] = features
	}
	initialConfig, err := json.Marshal(map[string]any{"config": initialEffective})
	if err != nil {
		t.Fatal(err)
	}
	config, err := json.Marshal(map[string]any{"config": effective})
	if err != nil {
		t.Fatal(err)
	}
	response := func(raw string) string { return `printf '{"id":%s,"result":%s}\n' "$id" ` + shellQuote(raw) }
	installed := response(pluginInstalledFixture)
	if inventoryError {
		installed = `printf '{"id":%s,"error":{"code":1,"message":"synthetic-plugin-secret"}}\n' "$id"`
	}
	resume := response(`{"thread":{"id":"thr-plugin"}}`)
	if resumeFallback {
		resume = `printf '{"id":%s,"error":{"code":1,"message":"thread not found"}}\n' "$id"`
	}
	return strings.Join([]string{
		`phase=first; if [ -f "$CODEX_HOME/discovery-exited" ]; then phase=second; fi`,
		`if [ "$phase" = first ]; then touch "$CODEX_HOME/discovery-exited"; fi`,
		`if [ "$phase" = second ]; then cp "$CODEX_HOME/config.toml" "$CODEX_HOME/test-sealed.toml"; fi`,
		`printf '%s\n' "$@" >> "$CODEX_HOME/test-argv"`,
		`printf '%s\n' "$phase" >> "$CODEX_HOME/launches"`,
		`while IFS= read -r line; do`,
		`printf '%s %s\n' "$phase" "$line" >> "$CODEX_HOME/test-rpc"`,
		`id=$(printf '%s' "$line" | sed -n 's/.*"id":\([0-9]*\).*/\1/p')`,
		`case "$line" in`,
		`*'"method":"initialize"'*) ` + response(`{}`) + ` ;;`,
		`*'"method":"plugin/installed"'*) ` + installed + ` ;;`,
		`*'"method":"plugin/read"'*) ` + response(pluginReadFixture) + ` ;;`,
		`*'"method":"config/read"'*) if [ "$phase" = first ]; then ` + response(string(initialConfig)) + `; else ` + response(string(config)) + `; fi ;;`,
		`*'"method":"thread/resume"'*) ` + resume + ` ;;`,
		`*'"method":"thread/start"'*) ` + response(`{"thread":{"id":"thr-plugin"}}`) + ` ;;`,
		`*'"method":"turn/start"'*) ` + response(`{}`),
		`echo '{"method":"item/completed","params":{"threadId":"thr-plugin","turnId":"turn-plugin","item":{"type":"agentMessage","id":"final","phase":"final_answer","text":"Done"}}}'`,
		`echo '{"method":"turn/completed","params":{"threadId":"thr-plugin","turn":{"id":"turn-plugin","status":"completed"}}}' ;;`,
		`esac`, `done`, `touch "$CODEX_HOME/discovery-exited"`, "",
	}, "\n")
}

func TestCodexPluginMCPTwoPhaseStartResumeAndFallback(t *testing.T) {
	for _, mode := range []string{"deny_all", "allowlist"} {
		for _, path := range []string{"start", "resume", "fallback"} {
			t.Run(mode+"/"+path, func(t *testing.T) {
				cfg, opts := pluginMCPTestOptions(t)
				opts.RuntimeMCPSelection.Mode = mode
				if mode == "allowlist" {
					opts.RuntimeMCPSelection.Allow = []string{"ordinary"}
				}
				if path != "start" {
					opts.ResumeSessionID = "prior-thread"
				}
				opts.ThinkingLevel, opts.ServiceTier = "high", "default"
				opts.McpConfig = json.RawMessage(`{"mcpServers":{"ordinary":{"command":"safe-tool","env":{"TOKEN":"synthetic-plugin-secret"}}}}`)
				effective := pluginEffectiveFixture()
				effective["mcp_servers"] = map[string]any{"ordinary": map[string]any{"command": "safe-tool", "env": map[string]any{"TOKEN": "synthetic-plugin-secret"}}}
				skill := filepath.Join(cfg.Env["CODEX_HOME"], "skill-marker")
				if err := os.WriteFile(skill, []byte("retained"), 0o600); err != nil {
					t.Fatal(err)
				}
				result, err, logs := runPluginMCPTest(t, context.Background(), pluginMCPProtocolScript(t, effective, path == "fallback", false), cfg, opts)
				if err != nil || result.Status != "completed" || result.Output != "Done" {
					t.Fatalf("result=%+v err=%v", result, err)
				}
				launches, _ := os.ReadFile(filepath.Join(cfg.Env["CODEX_HOME"], "launches"))
				if string(launches) != "first\nsecond\n" {
					t.Fatalf("unbounded or overlapping launches: %q", launches)
				}
				rpcs, _ := os.ReadFile(filepath.Join(cfg.Env["CODEX_HOME"], "test-rpc"))
				for _, line := range strings.Split(string(rpcs), "\n") {
					if strings.HasPrefix(line, "first ") && (strings.Contains(line, `"method":"thread/`) || strings.Contains(line, `"method":"turn/`)) {
						t.Fatal("discovery started a thread or model turn")
					}
					if strings.Contains(line, `"method":"thread/start"`) || strings.Contains(line, `"method":"thread/resume"`) {
						var packet struct {
							Params struct {
								Config      map[string]any `json:"config"`
								ServiceTier string         `json:"serviceTier"`
							} `json:"params"`
						}
						if json.Unmarshal([]byte(strings.TrimPrefix(line, "second ")), &packet) != nil {
							t.Fatal("bad fixture request")
						}
						config := packet.Params.Config
						rawDisabled, rawOK := nestedCodexConfigValue(config, []string{"plugins", "probe@local", "mcp_servers", "raw.server", "enabled"})
						unicodeDisabled, unicodeOK := nestedCodexConfigValue(config, []string{"plugins", "probe@local", "mcp_servers", "unicode_工具", "enabled"})
						if !rawOK || rawDisabled != false || !unicodeOK || unicodeDisabled != false || config["model_reasoning_effort"] != "high" || packet.Params.ServiceTier != "default" {
							t.Fatalf("thread overrides missing or replaced: %v", config)
						}
					}
				}
				argv, _ := os.ReadFile(filepath.Join(cfg.Env["CODEX_HOME"], "test-argv"))
				if !strings.Contains(string(argv), `plugins={ "probe@local" = { mcp_servers = { "raw.server" = { enabled = false }`) {
					t.Fatal("launch override missing")
				}
				if strings.Contains(logs+string(argv)+string(rpcs)+result.Error, "synthetic-plugin-secret") {
					t.Fatal("configuration secret escaped private file")
				}
				data, _ := os.ReadFile(filepath.Join(cfg.Env["CODEX_HOME"], "test-sealed.toml"))
				var config map[string]any
				if toml.Unmarshal(data, &config) != nil {
					t.Fatal("private TOML invalid")
				}
				value, ok := nestedCodexConfigValue(config, []string{"plugins", "probe@local", "mcp_servers", "raw.server", "enabled"})
				if !ok || value != false {
					t.Fatal("raw server name was split or not disabled")
				}
				value, ok = nestedCodexConfigValue(config, []string{"plugins", "probe@local", "enabled"})
				if !ok || value != true {
					t.Fatal("plugin disabled instead of its MCP")
				}
				info, _ := os.Stat(filepath.Join(cfg.Env["CODEX_HOME"], "config.toml"))
				if info.Mode().Perm() != 0o600 {
					t.Fatal("private config permissions widened")
				}
				if got, _ := os.ReadFile(skill); string(got) != "retained" {
					t.Fatal("plugin skill was changed")
				}
			})
		}
	}
}

func TestCodexPluginMCPInventoryFailureIsSafe(t *testing.T) {
	cfg, opts := pluginMCPTestOptions(t)
	result, err, logs := runPluginMCPTest(t, context.Background(), pluginMCPProtocolScript(t, pluginEffectiveFixture(), false, true), cfg, opts)
	if !errors.Is(err, agentconfig.ErrRuntimeMCPSelection) || result.Status == "completed" {
		t.Fatal("inventory failure did not fail closed")
	}
	if strings.Contains(logs+err.Error(), "synthetic-plugin-secret") {
		t.Fatal("raw inventory error leaked")
	}
	rpcs, _ := os.ReadFile(filepath.Join(cfg.Env["CODEX_HOME"], "test-rpc"))
	if strings.Contains(string(rpcs), `"method":"thread/`) || strings.Contains(string(rpcs), `"method":"turn/`) {
		t.Fatal("failed inventory reached thread or turn")
	}
}

func TestCodexPluginMCPProjectReenableFailsWithoutProjectPolicy(t *testing.T) {
	for _, variant := range []string{"reenable", "missing", "new-plugin", "extra-server", "ordinary-extra"} {
		t.Run(variant, func(t *testing.T) {
			cfg, opts := pluginMCPTestOptions(t)
			effective := pluginEffectiveFixture()
			plugins := effective["plugins"].(map[string]any)
			switch variant {
			case "reenable":
				plugins["probe@local"].(map[string]any)["mcp_servers"].(map[string]any)["raw.server"] = map[string]any{"enabled": true}
			case "missing":
				delete(effective, "plugins")
			case "new-plugin":
				plugins["unlisted@local"] = map[string]any{"enabled": true}
			case "extra-server":
				plugins["probe@local"].(map[string]any)["mcp_servers"].(map[string]any)["unlisted"] = map[string]any{"enabled": true}
			case "ordinary-extra":
				effective["mcp_servers"] = map[string]any{"unexpected": map[string]any{"command": "synthetic-plugin-secret"}}
			}
			result, err, logs := runPluginMCPTest(t, context.Background(), pluginMCPProtocolScript(t, effective, false, false), cfg, opts)
			if err != nil || result.Status != "failed" || !strings.Contains(result.Error, agentconfig.ErrRuntimeMCPSelection.Error()) {
				t.Fatalf("result=%+v err=%v", result, err)
			}
			if !result.RuntimeMCPSelectionFailed {
				t.Fatal("asynchronous selection failure lost structural classification")
			}
			if strings.Contains(logs+result.Error, "synthetic-plugin-secret") {
				t.Fatal("effective config leaked")
			}
			rpcs, _ := os.ReadFile(filepath.Join(cfg.Env["CODEX_HOME"], "test-rpc"))
			if strings.Contains(string(rpcs), `"method":"thread/`) || strings.Contains(string(rpcs), `"method":"turn/`) {
				t.Fatal("conflicting project config reached thread/turn")
			}
		})
	}
}

func TestCodexPluginMCPInheritDoesNotInspectPlugins(t *testing.T) {
	for _, explicit := range []bool{false, true} {
		cfg, opts := pluginMCPTestOptions(t)
		cfg.CodexVersion = "unknown"
		if explicit {
			opts.RuntimeMCPSelection.Mode = "inherit"
		} else {
			opts.RuntimeMCPSelection = nil
		}
		opts.CustomArgs = []string{"-c", "plugins={}"}
		result, err, _ := runPluginMCPTest(t, context.Background(), pluginMCPProtocolScript(t, pluginEffectiveFixture(), false, true), cfg, opts)
		if err != nil || result.Status != "completed" {
			t.Fatalf("inherit changed: status=%s err=%v", result.Status, err)
		}
		rpcs, _ := os.ReadFile(filepath.Join(cfg.Env["CODEX_HOME"], "test-rpc"))
		if strings.Contains(string(rpcs), `"method":"plugin/`) {
			t.Fatal("inherit enumerated plugins")
		}
		launches, _ := os.ReadFile(filepath.Join(cfg.Env["CODEX_HOME"], "launches"))
		if string(launches) != "first\n" {
			t.Fatal("inherit spawned extra process")
		}
	}
}

func TestCodexPluginMCPSecondInitializeFailureIsSafe(t *testing.T) {
	cfg, opts := pluginMCPTestOptions(t)
	script := pluginMCPProtocolScript(t, pluginEffectiveFixture(), false, false)
	script = strings.Replace(script, `*'"method":"initialize"'*)`, `*'"method":"initialize"'*) if [ "$phase" = second ]; then printf '{"id":%s,"error":{"code":1,"message":"synthetic-plugin-secret"}}\n' "$id"; continue; fi;`, 1)
	result, err, logs := runPluginMCPTest(t, context.Background(), script, cfg, opts)
	if err != nil || result.Status != "failed" {
		t.Fatalf("result=%+v err=%v", result, err)
	}
	if strings.Contains(logs+result.Error, "synthetic-plugin-secret") {
		t.Fatal("second initialize raw error leaked")
	}
}

func TestCodexPluginMCPInventoryProcessCancellation(t *testing.T) {
	for _, cancelParent := range []bool{false, true} {
		cfg, opts := pluginMCPTestOptions(t)
		opts.HandshakeTimeout = 80 * time.Millisecond
		ctx, cancel := context.WithCancel(context.Background())
		defer cancel()
		if cancelParent {
			time.AfterFunc(40*time.Millisecond, cancel)
		}
		script := pluginMCPProtocolScript(t, pluginEffectiveFixture(), false, false)
		script = strings.Replace(script, `*'"method":"plugin/installed"'*)`, `*'"method":"plugin/installed"'*) continue;`, 1)
		started := time.Now()
		result, err, _ := runPluginMCPTest(t, ctx, script, cfg, opts)
		if !errors.Is(err, agentconfig.ErrRuntimeMCPSelection) || result.Status == "completed" || time.Since(started) > 3*time.Second {
			t.Fatal("inventory cancellation failed or was unbounded")
		}
		launches, _ := os.ReadFile(filepath.Join(cfg.Env["CODEX_HOME"], "launches"))
		if strings.Count(string(launches), "\n") > 1 {
			t.Fatal("canceled inventory launched second process")
		}
	}
}

func TestCodexPluginMCPUnexpectedPreflightActivityIsSafe(t *testing.T) {
	cfg, opts := pluginMCPTestOptions(t)
	script := pluginMCPProtocolScript(t, pluginEffectiveFixture(), false, false)
	script = strings.Replace(script, `*'"method":"plugin/installed"'*)`, `*'"method":"plugin/installed"'*) echo '{"method":"error","params":{"error":{"message":"synthetic-plugin-secret"},"willRetry":false}}';`, 1)
	result, err, logs := runPluginMCPTest(t, context.Background(), script, cfg, opts)
	if !errors.Is(err, agentconfig.ErrRuntimeMCPSelection) || result.Status == "completed" {
		t.Fatal("unexpected preflight activity did not fail closed")
	}
	if strings.Contains(logs+err.Error(), "synthetic-plugin-secret") {
		t.Fatal("preflight notification leaked")
	}
}

func TestCodexPluginMCPPassiveRemoteControlStatusIsDiscarded(t *testing.T) {
	cfg, opts := pluginMCPTestOptions(t)
	script := pluginMCPProtocolScript(t, pluginEffectiveFixture(), false, false)
	script = strings.Replace(script, `*'"method":"plugin/installed"'*)`, `*'"method":"plugin/installed"'*) echo '{"method":"remoteControl/status/changed","params":{"status":"synthetic-plugin-secret"}}';`, 1)
	result, err, logs := runPluginMCPTest(t, context.Background(), script, cfg, opts)
	if err != nil || result.Status != "completed" {
		t.Fatal("passive native status blocked names-only preparation")
	}
	if strings.Contains(logs+result.Error, "synthetic-plugin-secret") {
		t.Fatal("passive notification was forwarded")
	}
}

func TestCodexPluginMCPUnconfirmedCleanupDoesNotLaunchTask(t *testing.T) {
	codexCleanupConfirmationOverride.Store(-1)
	t.Cleanup(func() { codexCleanupConfirmationOverride.Store(0) })
	cfg, opts := pluginMCPTestOptions(t)
	_, err, _ := runPluginMCPTest(t, context.Background(), pluginMCPProtocolScript(t, pluginEffectiveFixture(), false, false), cfg, opts)
	if !errors.Is(err, agentconfig.ErrRuntimeMCPSelection) {
		t.Fatal("unconfirmed cleanup allowed task process")
	}
	launches, _ := os.ReadFile(filepath.Join(cfg.Env["CODEX_HOME"], "launches"))
	if string(launches) != "first\n" {
		t.Fatal("unconfirmed discovery cleanup launched another process")
	}
}

func TestCodexPluginMCPCustomRuntimeAndInvalidPolicyFailClosed(t *testing.T) {
	for _, variant := range []string{"custom-runtime", "invalid-mode", "plugin-name", "no-managed-overlay", "global-home"} {
		t.Run(variant, func(t *testing.T) {
			cfg, opts := pluginMCPTestOptions(t)
			switch variant {
			case "custom-runtime":
				cfg.BuiltinRuntime = false
			case "invalid-mode":
				opts.RuntimeMCPSelection.Mode = "DENY_ALL"
			case "plugin-name":
				opts.RuntimeMCPSelection = &agentconfig.RuntimeMCPSelection{Mode: "allowlist", Allow: []string{"plugin:source"}}
			case "no-managed-overlay":
				opts.McpConfig = nil
			case "global-home":
				cfg.Env["HOME"] = t.TempDir()
				cfg.Env["CODEX_HOME"] = filepath.Join(cfg.Env["HOME"], ".codex")
				if err := os.Mkdir(cfg.Env["CODEX_HOME"], 0o700); err != nil {
					t.Fatal(err)
				}
			}
			marker := filepath.Join(t.TempDir(), "spawned")
			_, err, _ := runPluginMCPTest(t, context.Background(), "touch "+shellQuote(marker)+"\n", cfg, opts)
			if !errors.Is(err, agentconfig.ErrRuntimeMCPSelection) {
				t.Fatal("unsupported policy/runtime accepted")
			}
			if _, err := os.Stat(marker); !os.IsNotExist(err) {
				t.Fatal("unsupported preparation spawned native process")
			}
		})
	}
}

func TestCodexPluginMCPInheritDenyInheritSameHome(t *testing.T) {
	cfg, opts := pluginMCPTestOptions(t)
	for _, mode := range []string{"inherit", "deny_all", "inherit"} {
		opts.RuntimeMCPSelection.Mode = mode
		_ = os.Remove(filepath.Join(cfg.Env["CODEX_HOME"], "discovery-exited"))
		result, err, _ := runPluginMCPTest(t, context.Background(), pluginMCPProtocolScript(t, pluginEffectiveFixture(), false, false), cfg, opts)
		if err != nil || result.Status != "completed" {
			t.Fatalf("mode=%s status=%s err=%v", mode, result.Status, err)
		}
		data, err := os.ReadFile(filepath.Join(cfg.Env["CODEX_HOME"], "config.toml"))
		if err != nil {
			t.Fatal(err)
		}
		var config map[string]any
		if toml.Unmarshal(data, &config) != nil {
			t.Fatal("invalid private config")
		}
		if _, present := nestedCodexConfigValue(config, []string{"plugins", "probe@local", "mcp_servers"}); present {
			t.Fatalf("%s retained task-owned plugin MCP overrides", mode)
		}
		if value, _ := nestedCodexConfigValue(config, []string{"plugins", "probe@local", "enabled"}); value != true {
			t.Fatal("plugin activation was not restored")
		}
	}
}

func TestCodexPluginMCPDiscoveryReapsDetachedStdioHelper(t *testing.T) {
	cfg, opts := pluginMCPTestOptions(t)
	opts.HandshakeTimeout = 5 * time.Second
	pidFile := filepath.Join(cfg.Env["CODEX_HOME"], "helper-pid")
	t.Cleanup(func() {
		raw, _ := os.ReadFile(pidFile)
		pid, _ := strconv.Atoi(strings.TrimSpace(string(raw)))
		if pid > 0 {
			_ = syscall.Kill(pid, syscall.SIGKILL)
		}
	})
	script := pluginMCPProtocolScript(t, pluginEffectiveFixture(), false, false)
	script = strings.Replace(script, `while IFS= read -r line; do`, `if [ "$phase" = first ]; then sleep 30 </dev/null >/dev/null 2>&1 & echo $! > "$CODEX_HOME/helper-pid"; fi`+"\n"+`while IFS= read -r line; do`, 1)
	result, err, _ := runPluginMCPTest(t, context.Background(), script, cfg, opts)
	if err != nil || result.Status != "completed" {
		t.Fatalf("discovery helper prevented bounded cleanup: status=%s err=%v", result.Status, err)
	}
}

func TestCodexPluginMCPVerifiedPolicyPreservesNativeThreadFailure(t *testing.T) {
	for _, method := range []string{"start", "resume"} {
		t.Run(method, func(t *testing.T) {
			cfg, opts := pluginMCPTestOptions(t)
			script := pluginMCPProtocolScript(t, pluginEffectiveFixture(), false, false)
			if method == "resume" {
				opts.ResumeSessionID = "prior-thread"
				script = strings.Replace(script, `*'"method":"thread/resume"'*)`, `*'"method":"thread/resume"'*) exit 0;`, 1)
			} else {
				script = strings.Replace(script, `*'"method":"thread/start"'*)`, `*'"method":"thread/start"'*) printf '{"id":%s,"error":{"code":1,"message":"native model configuration rejected"}}\n' "$id"; continue;`, 1)
			}
			result, err, _ := runPluginMCPTest(t, context.Background(), script, cfg, opts)
			if err != nil || result.Status != "failed" {
				t.Fatalf("status=%s err=%v", result.Status, err)
			}
			if result.RuntimeMCPSelectionFailed || !strings.Contains(result.Error, "thread/"+method) {
				t.Fatal("verified policy reclassified a native thread failure")
			}
		})
	}
}

func TestCodexPluginMCPUnknownVersionRejectsBeforeSpawn(t *testing.T) {
	for _, version := range []string{"", "unknown", "0.153.3", "0.153.40", "0.154.0"} {
		t.Run(version, func(t *testing.T) {
			cfg, opts := pluginMCPTestOptions(t)
			cfg.CodexVersion = version
			marker := filepath.Join(t.TempDir(), "spawned")
			result, err, _ := runPluginMCPTest(t, context.Background(), "touch "+shellQuote(marker)+"\n", cfg, opts)
			if err == nil {
				t.Fatalf("unsupported plugin contract did not fail before spawn: %s", result.Status)
			}
			if _, err := os.Stat(marker); !os.IsNotExist(err) {
				t.Fatal("unsupported contract spawned native process")
			}
		})
	}
}

func TestCodexPluginMCPConflictingArgsRejectBeforeSpawn(t *testing.T) {
	for _, args := range [][]string{
		{"-c", `plugins."probe@local".enabled=true`},
		{`--config=plugins={}`},
		{`-c"plugins"."probe@local".mcp_servers.raw.enabled=true`},
		{"--profile", "unsafe"}, {"-punsafe"},
		{"-c", `profiles.unsafe.plugins={}`},
		{"-c", `mcp_servers.hidden.command="synthetic-plugin-secret"`},
	} {
		for _, source := range []string{"custom", "extra", "prefix"} {
			t.Run(source+"/"+args[0], func(t *testing.T) {
				cfg, opts := pluginMCPTestOptions(t)
				switch source {
				case "custom":
					opts.CustomArgs = args
				case "extra":
					opts.ExtraArgs = args
				case "prefix":
					cfg.LaunchPrefix = args
				}
				marker := filepath.Join(t.TempDir(), "spawned")
				_, err, logs := runPluginMCPTest(t, context.Background(), "touch "+shellQuote(marker)+"\n", cfg, opts)
				if err == nil {
					t.Fatal("conflicting launch override was accepted")
				}
				if _, err := os.Stat(marker); !os.IsNotExist(err) {
					t.Fatal("conflicting override reached native process")
				}
				if strings.Contains(logs+err.Error(), "synthetic-plugin-secret") {
					t.Fatal("secret was logged")
				}
			})
		}
	}
}
