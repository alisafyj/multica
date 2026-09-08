//go:build !windows

package agent

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"io"
	"log/slog"
	"os"
	"os/exec"
	"path/filepath"
	"reflect"
	"strings"
	"sync/atomic"
	"syscall"
	"testing"
	"time"

	"github.com/multica-ai/multica/server/internal/agentconfig"
)

// Opt-in, local fixture only. This probe never sends turn/start or reads auth.
func TestCodexPluginMCPNativeNoModel(t *testing.T) {
	testCodexPluginMCPNativeNoModel(t, "", true)
}

func TestCodexPluginMCPNativeCacheOnlyNoModel(t *testing.T) {
	for _, variant := range []string{"hash-version", "legacy-missing-version", "legacy-priority", "version-plus", "latest-alias"} {
		t.Run(variant, func(t *testing.T) { testCodexPluginMCPNativeNoModel(t, variant, true) })
	}
}

func TestCodexPluginMCPNativeFeatureDisabledNoModel(t *testing.T) {
	testCodexPluginMCPNativeNoModel(t, "", false)
}

func testCodexPluginMCPNativeNoModel(t *testing.T, cacheVariant string, pluginsEnabled bool) {
	bin := os.Getenv("MULTICA_TEST_CODEX_PLUGIN_BINARY")
	if bin == "" {
		t.Skip("set MULTICA_TEST_CODEX_PLUGIN_BINARY to run the isolated no-model probe")
	}
	if !filepath.IsAbs(bin) {
		t.Fatal("probe requires an explicit binary path")
	}
	node, err := exec.LookPath("node")
	if err != nil {
		t.Fatal("local Node fixture runtime unavailable")
	}
	root := t.TempDir()
	home, codexHome, cwd := filepath.Join(root, "home"), filepath.Join(root, "codex"), filepath.Join(root, "repo")
	market := filepath.Join(root, "marketplace")
	plugin := filepath.Join(market, "plugins", "probe-plugin")
	for _, dir := range []string{home, codexHome, cwd, filepath.Join(market, ".agents", "plugins"), filepath.Join(plugin, ".codex-plugin"), filepath.Join(plugin, "skills", "probe-skill")} {
		if err := os.MkdirAll(dir, 0o700); err != nil {
			t.Fatal(err)
		}
	}
	write := func(path string, value string) {
		t.Helper()
		if err := os.WriteFile(path, []byte(value), 0o600); err != nil {
			t.Fatal(err)
		}
	}
	writeJSON := func(path string, value any) {
		t.Helper()
		raw, err := json.Marshal(value)
		if err != nil {
			t.Fatal(err)
		}
		write(path, string(raw))
	}
	marker := filepath.Join(root, "started")
	fixture := filepath.Join(root, "fixture.mjs")
	write(fixture, `import {createInterface} from 'node:readline';
import {appendFileSync} from 'node:fs';
const name=process.argv[2]; appendFileSync(process.argv[3],name+'\n');
for await(const line of createInterface({input:process.stdin})) {
 let r; try { r=JSON.parse(line); } catch { continue; }
 if(r.id===undefined)continue;
 let result={};
 if(r.method==='initialize')result={protocolVersion:'2024-11-05',capabilities:{tools:{}},serverInfo:{name,version:'1.0'}};
 if(r.method==='tools/list')result={tools:[{name:'probe',description:'Fixed local marker',inputSchema:{type:'object',properties:{}}}]};
 if(r.method==='tools/call')result={content:[{type:'text',text:'proof:'+name}],isError:false};
 if(r.method==='resources/list')result={resources:[]};
 if(r.method==='resources/templates/list')result={resourceTemplates:[]};
 process.stdout.write(JSON.stringify({jsonrpc:'2.0',id:r.id,result})+'\n');
}`)
	server := func(name string) map[string]any {
		return map[string]any{"command": node, "args": []string{fixture, name, marker}, "startup_timeout_sec": 5}
	}
	modelConfig := ""
	if !pluginsEnabled {
		modelConfig = "model = 'no-model-probe'\n"
	}
	writeJSON(filepath.Join(market, ".agents", "plugins", "marketplace.json"), map[string]any{
		"name": "probe-local", "interface": map[string]any{"displayName": "Local probe"}, "plugins": []any{map[string]any{
			"name": "probe-plugin", "source": map[string]any{"source": "local", "path": "./plugins/probe-plugin"}, "policy": map[string]any{"installation": "AVAILABLE", "authentication": "ON_INSTALL"}, "category": "Productivity",
		}},
	})
	writeJSON(filepath.Join(plugin, ".codex-plugin", "plugin.json"), map[string]any{"name": "probe-plugin", "version": "1.0.0", "skills": "./skills/", "mcpServers": "./.mcp.json"})
	writeJSON(filepath.Join(plugin, ".mcp.json"), map[string]any{"plugin_probe": server("plugin_probe")})
	write(filepath.Join(plugin, "skills", "probe-skill", "SKILL.md"), "---\nname: probe-skill\ndescription: Fixed local probe skill.\n---\n# Probe\n")
	write(filepath.Join(codexHome, "config.toml"), modelConfig+"approval_policy = 'never'\nsandbox_mode = 'read-only'\nplugins.'disabled@local'.enabled = false\n[features]\nplugins = true\n")
	env := map[string]string{"PATH": os.Getenv("PATH"), "HOME": home, "CODEX_HOME": codexHome, "TMPDIR": os.TempDir(), "NO_PROXY": "*", "HTTP_PROXY": "", "HTTPS_PROXY": "", "ALL_PROXY": ""}
	// executeOnce merges its environment; blank every ambient key outside this
	// fixture allowlist so even an opt-in run cannot inherit provider credentials.
	for _, entry := range os.Environ() {
		key, _, _ := strings.Cut(entry, "=")
		if _, ok := env[key]; !ok {
			env[key] = ""
		}
	}
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()
	cli := func(args ...string) []byte {
		t.Helper()
		cmd := exec.CommandContext(ctx, bin, args...)
		cmd.Dir = cwd
		cmd.Env = mergeEnv(nil, env)
		cmd.Stderr = io.Discard
		out, err := cmd.Output()
		if err != nil {
			t.Fatalf("local fixture CLI operation failed: %v", err)
		}
		return out
	}
	version := strings.TrimSpace(string(cli("--version")))
	if version != "codex-cli 0.153.4" {
		t.Fatal("unsupported native probe version")
	}
	cli("plugin", "marketplace", "add", market, "--json")
	cli("plugin", "add", "probe-plugin", "--marketplace", "probe-local", "--json")
	if cacheVariant != "" {
		cacheParent := filepath.Join(codexHome, "plugins", "cache", "probe-local", "probe-plugin")
		directoryVersion, manifestVersion, manifestDirectory := "1e285826", "0.1.4", ".codex-plugin"
		switch cacheVariant {
		case "legacy-missing-version":
			directoryVersion, manifestVersion, manifestDirectory = "local", "", ".claude-plugin"
		case "legacy-priority":
			directoryVersion, manifestVersion, manifestDirectory = "0.48.0", "0.48.0", ".claude-plugin"
		case "version-plus":
			directoryVersion, manifestVersion = "0.1.4+build.7", "0.1.4+build.7"
		}
		cacheRoot := filepath.Join(cacheParent, directoryVersion)
		if err := os.Rename(filepath.Join(cacheParent, "1.0.0"), cacheRoot); err != nil {
			t.Fatal("could not create hash-version cache fixture")
		}
		if manifestDirectory != ".codex-plugin" {
			if err := os.Rename(filepath.Join(cacheRoot, ".codex-plugin"), filepath.Join(cacheRoot, manifestDirectory)); err != nil {
				t.Fatal(err)
			}
		}
		manifest := map[string]any{"name": "probe-plugin", "skills": "./skills/", "mcpServers": "./.mcp.json"}
		if manifestVersion != "" {
			manifest["version"] = manifestVersion
		}
		writeJSON(filepath.Join(cacheRoot, manifestDirectory, "plugin.json"), manifest)
		if cacheVariant == "latest-alias" {
			canonicalRoot, err := filepath.EvalSymlinks(cacheRoot)
			if err != nil {
				t.Fatal(err)
			}
			if err := os.Symlink(canonicalRoot, filepath.Join(cacheParent, "latest")); err != nil {
				t.Fatal(err)
			}
		}
		if cacheVariant == "legacy-priority" {
			if err := os.Mkdir(filepath.Join(cacheRoot, ".cursor-plugin"), 0o700); err != nil {
				t.Fatal(err)
			}
			writeJSON(filepath.Join(cacheRoot, ".cursor-plugin", "plugin.json"), map[string]any{"name": "probe-plugin", "version": "9.0.0"})
		}
		t.Logf("native cache binding: directory=%s manifest_version=%q manifest_directory=%s", directoryVersion, manifestVersion, manifestDirectory)
		write(filepath.Join(codexHome, "config.toml"), modelConfig+"approval_policy = 'never'\nsandbox_mode = 'read-only'\nplugins.'disabled@local'.enabled = false\nplugins.'probe-plugin@probe-local'.enabled = true\n[features]\nplugins = true\n")
		if err := os.RemoveAll(filepath.Join(codexHome, "plugins", "marketplaces")); err != nil {
			t.Fatal("could not hide fixture marketplace registry")
		}
		if err := os.Rename(market, filepath.Join(root, "hidden-marketplace")); err != nil {
			t.Fatal("could not hide fixture source marketplace")
		}
	}
	managed, _ := json.Marshal(map[string]any{"mcpServers": map[string]any{"managed_probe": server("managed_probe")}})
	var diagnostics bytes.Buffer
	logger := slog.New(slog.NewJSONHandler(&diagnostics, nil))
	b := &codexBackend{cfg: Config{ExecutablePath: bin, Env: env, Logger: logger, CodexVersion: version, BuiltinRuntime: true}}
	prior := ""
	for _, mode := range []string{"inherit", "deny_all", "inherit"} {
		_ = os.Remove(marker)
		opts := ExecOptions{Cwd: cwd, McpConfig: managed, ResumeSessionID: prior, RuntimeMCPSelection: &agentconfig.RuntimeMCPSelection{Mode: mode}, HandshakeTimeout: 5 * time.Second}
		if !pluginsEnabled {
			opts.CustomArgs = []string{"-c", "features.plugins=false"}
		}
		if err := ensureCodexMcpConfig(filepath.Join(codexHome, "config.toml"), managed, logger); err != nil {
			t.Fatal("managed config materialization failed")
		}
		opts, err = b.preparePluginMCP(ctx, opts)
		if err != nil {
			for _, line := range strings.Split(diagnostics.String(), "\n") {
				var event map[string]any
				if json.Unmarshal([]byte(line), &event) == nil && event["phase"] == "cleanup" {
					t.Logf("native cleanup: reaped=%v", event["reaped"])
				}
			}
			t.Fatalf("%s preparation failed: %v", mode, err)
		}
		if _, err := os.Stat(marker); !os.IsNotExist(err) {
			t.Fatal("metadata discovery started MCP")
		}
		func() {
			if opts.codexPluginMCP != nil {
				defer func() {
					if err := opts.codexPluginMCP.restore(); err != nil {
						t.Error("private policy restoration failed")
					}
				}()
			}
			args := buildCodexArgs(opts, logger)
			if opts.codexPluginMCP != nil {
				args = append(args, opts.codexPluginMCP.launchOverrides()...)
			}
			cmd := exec.CommandContext(ctx, bin, args...)
			cmd.Dir = cwd
			cmd.Env = mergeEnv(nil, env)
			cmd.Stderr = io.Discard
			stdin, err := cmd.StdinPipe()
			if err != nil {
				t.Fatal(err)
			}
			stdout, err := cmd.StdoutPipe()
			if err != nil {
				t.Fatal(err)
			}
			cmd.Cancel = func() error { signalProcessGroup(cmd, syscall.SIGKILL); return nil }
			cmd.WaitDelay = time.Second
			configureProcessGroup(cmd)
			if err := startOwnedProcessTree(cmd, logger); err != nil {
				t.Fatal(err)
			}
			client := &codexClient{cfg: b.cfg, stdin: &lockedWriter{writer: stdin}, pending: make(map[int]*pendingRPC), processDone: make(chan struct{}), handshakeTimeout: 5 * time.Second}
			done := make(chan struct{})
			go func() {
				defer close(done)
				scanner := newAgentStreamScanner(stdout)
				for scanner.Scan() {
					client.handleLine(scanner.Text())
				}
				client.markProcessExited(errCodexProcessExited)
			}()
			defer func() {
				_ = stdin.Close()
				signalProcessGroup(cmd, syscall.SIGKILL)
				<-done
				_ = cmd.Wait()
				releaseProcessGroup(cmd)
			}()
			if _, err := client.request(ctx, "initialize", map[string]any{"clientInfo": map[string]any{"name": "multica-plugin-no-model", "version": "1"}, "capabilities": map[string]any{"experimentalApi": true}}); err != nil {
				t.Fatal("native initialize failed")
			}
			client.notify("initialized")
			checkConfig, checkErr := readCodexConfiguration(ctx, client, cwd, codexConfigurationReadPurposeNeutral)
			value, present := nestedCodexConfigValue(checkConfig.Config, []string{"plugins", "disabled@local", "enabled"})
			if checkErr != nil || !present || value != false {
				t.Fatal("native override lost an unrelated disabled plugin setting")
			}
			if _, err := discoverCodexPluginMCP(ctx, client, cwd); err != nil {
				t.Fatalf("native standalone names parser failed: %v", err)
			}
			if opts.codexPluginMCP != nil {
				if err := verifyCodexPluginMCPPolicy(ctx, client, opts); err != nil {
					observed, inventoryErr := discoverCodexPluginMCP(ctx, client, cwd)
					t.Logf("native readback inventory: valid=%t equal=%t expected_servers=%d observed_servers=%d", inventoryErr == nil, reflect.DeepEqual(observed, opts.codexPluginMCP.inventory), len(opts.codexPluginMCP.inventory["probe-plugin@probe-local"]), len(observed["probe-plugin@probe-local"]))
					config, configErr := readCodexConfiguration(ctx, client, cwd, codexConfigurationReadPurposeNeutral)
					if configErr == nil {
						if plugins, ok := config.Config["plugins"].(map[string]any); ok {
							for name, value := range plugins {
								entry, table := value.(map[string]any)
								enabled, boolean := entry["enabled"].(bool)
								_, known := opts.codexPluginMCP.inventory[name]
								t.Logf("isolated plugin entry: name=%s table=%t enabled_boolean=%t enabled=%t inventoried=%t", name, table, boolean, enabled, known)
							}
						}
						value, present := nestedCodexConfigValue(config.Config, []string{"plugins", "probe-plugin@probe-local", "mcp_servers", "plugin_probe", "enabled"})
						disabled, valid := value.(bool)
						expected, _ := normalizedManagedCodexMcpServers(opts.McpConfig)
						actual, _ := effectiveCodexMcpServers(config.Config)
						t.Logf("native readback config: override_present=%t boolean=%t disabled=%t ordinary_equal=%t", present, valid, valid && !disabled, reflect.DeepEqual(expected, actual))
					}
					t.Fatalf("native policy readback failed: %v", err)
				}
			}
			thread, _, err := client.startOrResumeThread(ctx, opts, logger)
			if err != nil {
				t.Fatalf("native thread setup failed: %v", err)
			}
			prior = thread
			raw, err := client.request(ctx, "mcpServerStatus/list", map[string]any{"threadId": thread, "detail": "toolsAndAuthOnly"})
			if err != nil {
				t.Fatal("native MCP inventory failed")
			}
			var inventory struct {
				Data []struct {
					Name  string         `json:"name"`
					Tools map[string]any `json:"tools"`
				} `json:"data"`
			}
			if json.Unmarshal(raw, &inventory) != nil {
				t.Fatal("native MCP inventory schema failed")
			}
			active := make(map[string]bool)
			for _, server := range inventory.Data {
				_, active[server.Name] = server.Tools["probe"]
			}
			expectPlugin := pluginsEnabled && mode == "inherit"
			if !active["managed_probe"] || active["plugin_probe"] != expectPlugin {
				t.Fatalf("native %s MCP activation mismatch: managed=%t plugin=%t", mode, active["managed_probe"], active["plugin_probe"])
			}
			call, err := client.request(ctx, "mcpServer/tool/call", map[string]any{"threadId": thread, "server": "managed_probe", "tool": "probe", "arguments": map[string]any{}})
			if err != nil || !strings.Contains(string(call), "proof:managed_probe") {
				t.Fatal("managed local tool failed")
			}
			skills, err := client.request(ctx, "skills/list", map[string]any{"cwds": []string{cwd}, "forceReload": true})
			if err != nil || strings.Contains(string(skills), "probe-skill") != pluginsEnabled {
				t.Fatal("plugin skill activation contradicted the feature setting")
			}
			started, _ := os.ReadFile(marker)
			if strings.Contains(string(started), "plugin_probe") != expectPlugin {
				t.Fatal("plugin startup marker contradicts selection")
			}
			t.Logf("native %s: plugin_active=%t managed_call=PASS skill=PASS model_turns=0", mode, active["plugin_probe"])
		}()
	}
	if pluginsEnabled {
		return
	}
	var reached atomic.Int32
	const stop = "feature-disabled-pre-turn-stop"
	diagnostics.Reset()
	session, err := b.Execute(ctx, "", ExecOptions{
		Cwd: cwd, McpConfig: managed, Timeout: 20 * time.Second, HandshakeTimeout: 5 * time.Second,
		CustomArgs: []string{"-c", "features.plugins=false"}, RuntimeMCPSelection: &agentconfig.RuntimeMCPSelection{Mode: "deny_all"},
		ValidateResolvedModelSelection: func(model string) error {
			if model != "no-model-probe" {
				return errors.New("unexpected effective model")
			}
			reached.Add(1)
			return errors.New(stop)
		},
	})
	if err != nil {
		t.Fatalf("feature-disabled preparation failed before model preflight: %v", err)
	}
	toolMessages := 0
	for message := range session.Messages {
		if message.Type == MessageToolUse || message.Type == MessageToolResult {
			toolMessages++
		}
	}
	result := <-session.Result
	if reached.Load() != 1 || result.Status != "failed" || result.Error != "codex model selection preflight failed: "+stop || result.SessionID != "" || len(result.Usage) != 0 || toolMessages != 0 {
		t.Fatalf("feature-disabled pre-turn sentinel not proven: reached=%d status=%q session_present=%t usage_models=%d tool_messages=%d", reached.Load(), result.Status, result.SessionID != "", len(result.Usage), toolMessages)
	}
	cleanups := 0
	for _, line := range strings.Split(diagnostics.String(), "\n") {
		var event map[string]any
		if json.Unmarshal([]byte(line), &event) == nil && event["msg"] == "codex lifecycle" && event["phase"] == "cleanup" {
			cleanups++
			if event["reaped"] != true {
				t.Fatal("owned process cleanup was not confirmed")
			}
		}
	}
	if cleanups != 2 {
		t.Fatalf("expected two reaped preflight processes; observed %d", cleanups)
	}
}
