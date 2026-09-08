//go:build darwin || linux

package agent

import (
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/json"
	"errors"
	"io"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"os"
	"os/exec"
	"path/filepath"
	"reflect"
	"strings"
	"sync/atomic"
	"testing"
	"time"

	"github.com/multica-ai/multica/server/internal/agentconfig"
	"github.com/pelletier/go-toml/v2"
)

// Opt-in integration probe: copy only enabled IDs, never credentials or provider
// settings. A deliberate validation failure stops Execute before thread creation.
func TestCodexLocalPluginInventoryPreflight(t *testing.T) {
	bin, source := os.Getenv("MULTICA_TEST_CODEX_PLUGIN_BINARY"), os.Getenv("MULTICA_TEST_CODEX_PLUGIN_SOURCE_HOME")
	if bin == "" || source == "" {
		t.Skip("set explicit native binary and source cache home for the no-model inventory probe")
	}
	if !filepath.IsAbs(bin) || !filepath.IsAbs(source) {
		t.Fatal("probe paths must be absolute")
	}
	sourcePath := filepath.Join(source, "config.toml")
	sourceBytes, err := os.ReadFile(sourcePath)
	if err != nil {
		t.Fatal("source configuration unavailable")
	}
	sourceHash := sha256.Sum256(sourceBytes)
	var sourceConfig map[string]any
	if toml.Unmarshal(sourceBytes, &sourceConfig) != nil {
		t.Fatal("source configuration invalid")
	}
	plugins := make(map[string]any)
	configured, _ := sourceConfig["plugins"].(map[string]any)
	bindings := make(map[string]codexPluginCacheBinding)
	for id, entry := range configured {
		settings, _ := entry.(map[string]any)
		if settings["enabled"] != true {
			continue
		}
		plugins[id] = map[string]any{"enabled": true}
		binding, err := snapshotCodexPluginCache(source, id)
		if err != nil {
			t.Errorf("cache binding rejected plugin ID %q", id)
			continue
		}
		bindings[id] = binding
	}
	if t.Failed() {
		t.FailNow()
	}
	if len(plugins) == 0 {
		t.Fatal("probe requires enabled plugins")
	}
	sourceBytes, sourceConfig = nil, nil
	root := t.TempDir()
	home, private, cwd := filepath.Join(root, "home"), filepath.Join(root, "codex"), filepath.Join(root, "repo")
	for _, dir := range []string{home, filepath.Join(private, "plugins"), cwd} {
		if os.MkdirAll(dir, 0o700) != nil {
			t.Fatal("private directory creation failed")
		}
	}
	cache, err := filepath.EvalSymlinks(filepath.Join(source, "plugins", "cache"))
	if err != nil || os.Symlink(cache, filepath.Join(private, "plugins", "cache")) != nil {
		t.Fatal("private cache reference failed")
	}
	var providerRequests, blockedBackgroundRequests atomic.Int32
	local := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if strings.HasPrefix(r.URL.Path, "/v1/") {
			providerRequests.Add(1)
		} else {
			blockedBackgroundRequests.Add(1)
		}
		w.WriteHeader(http.StatusServiceUnavailable)
	}))
	defer local.Close()
	config := map[string]any{
		"model": "local-inventory-probe", "model_provider": "local-probe",
		"approval_policy": "never", "sandbox_mode": "read-only",
		"features": map[string]any{"plugins": true}, "plugins": plugins,
		"model_providers": map[string]any{"local-probe": map[string]any{
			"name": "Local no-auth probe", "base_url": local.URL + "/v1", "wire_api": "responses", "requires_openai_auth": false,
		}},
	}
	raw, err := toml.Marshal(config)
	if err != nil || os.WriteFile(filepath.Join(private, "config.toml"), raw, 0o600) != nil {
		t.Fatal("private no-auth configuration creation failed")
	}
	env := map[string]string{
		"PATH": os.Getenv("PATH"), "HOME": home, "CODEX_HOME": private, "TMPDIR": os.TempDir(),
		"HTTP_PROXY": local.URL, "HTTPS_PROXY": local.URL, "ALL_PROXY": local.URL,
		"http_proxy": local.URL, "https_proxy": local.URL, "all_proxy": local.URL,
		"NO_PROXY": "127.0.0.1,localhost", "no_proxy": "127.0.0.1,localhost", "OTEL_SDK_DISABLED": "true",
	}
	for _, entry := range os.Environ() {
		key, _, _ := strings.Cut(entry, "=")
		if _, ok := env[key]; !ok {
			env[key] = ""
		}
	}
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()
	versionCmd := exec.CommandContext(ctx, bin, "--version")
	versionCmd.Dir, versionCmd.Env, versionCmd.Stderr = cwd, mergeEnv(nil, env), io.Discard
	version, err := versionCmd.Output()
	if err != nil || strings.TrimSpace(string(version)) != "codex-cli 0.153.4" {
		t.Fatal("probe requires the verified native Codex version")
	}
	var diagnostics bytes.Buffer
	b := &codexBackend{cfg: Config{ExecutablePath: bin, Env: env,
		Logger: slog.New(slog.NewJSONHandler(&diagnostics, nil)), CodexVersion: strings.TrimSpace(string(version)), BuiltinRuntime: true}}
	var reached atomic.Int32
	const stop = "local-inventory-preflight-stop"
	started := time.Now()
	session, err := b.Execute(ctx, "", ExecOptions{
		Cwd: cwd, McpConfig: json.RawMessage(`{"mcpServers":{}}`), Timeout: 25 * time.Second,
		HandshakeTimeout: 10 * time.Second, RuntimeMCPSelection: &agentconfig.RuntimeMCPSelection{Mode: "deny_all"},
		ValidateResolvedModelSelection: func(model string) error {
			if model != "local-inventory-probe" {
				return errors.New("unexpected effective model")
			}
			reached.Add(1)
			return errors.New(stop)
		},
	})
	if err != nil {
		t.Fatal("inventory preparation failed before policy validation")
	}
	toolMessages := 0
	for message := range session.Messages {
		if message.Type == MessageToolUse || message.Type == MessageToolResult {
			toolMessages++
		}
	}
	result := <-session.Result
	if reached.Load() != 1 || result.Status != "failed" || result.Error != "codex model selection preflight failed: "+stop || result.RuntimeMCPSelectionFailed || result.SessionID != "" || len(result.Usage) != 0 || toolMessages != 0 {
		t.Fatalf("pre-thread sentinel not proven: reached=%d status=%q policy_failed=%t session_present=%t usage_models=%d tool_messages=%d", reached.Load(), result.Status, result.RuntimeMCPSelectionFailed, result.SessionID != "", len(result.Usage), toolMessages)
	}
	if providerRequests.Load() != 0 {
		t.Error("unexpected request to the loopback model provider")
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
	privateBytes, err := os.ReadFile(filepath.Join(private, "config.toml"))
	var restored map[string]any
	if err != nil || toml.Unmarshal(privateBytes, &restored) != nil || !reflect.DeepEqual(restored["plugins"], plugins) {
		t.Fatal("task-private plugin configuration was not restored")
	}
	entries, err := os.ReadDir(private)
	if err != nil {
		t.Fatal("private home cleanup check failed")
	}
	for _, entry := range entries {
		if strings.HasPrefix(entry.Name(), ".multica-plugin-metadata-") {
			t.Fatal("temporary metadata directory remains")
		}
	}
	afterSource, err := os.ReadFile(sourcePath)
	if err != nil || sha256.Sum256(afterSource) != sourceHash {
		t.Fatal("source configuration changed during probe")
	}
	for id, before := range bindings {
		after, err := snapshotCodexPluginCache(source, id)
		if err != nil || before != after {
			t.Fatal("source cache metadata changed during probe")
		}
	}
	t.Logf("enabled_plugins=%d policy_preflight_passed=true sentinel_reached=1 reaped_processes=%d provider_requests=%d blocked_background_requests=%d plugin_settings_restored=true source_metadata_unchanged=true elapsed_ms=%d", len(plugins), cleanups, providerRequests.Load(), blockedBackgroundRequests.Load(), time.Since(started).Milliseconds())
}
