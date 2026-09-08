//go:build darwin || linux

package agent

import (
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/json"
	"fmt"
	"io"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"sync/atomic"
	"syscall"
	"testing"
	"time"
)

// Only config and thread RPCs are sent. The local provider must receive no requests.
func TestCodexPluginSkillPolicyNativeNoModel(t *testing.T) {
	bin := os.Getenv("MULTICA_TEST_CODEX_PLUGIN_BINARY")
	if bin == "" {
		t.Skip("set MULTICA_TEST_CODEX_PLUGIN_BINARY for the pinned no-model policy test")
	}
	if !filepath.IsAbs(bin) {
		t.Fatal("native binary path must be absolute")
	}
	file, err := os.Open(bin)
	if err != nil {
		t.Fatal("native binary unavailable")
	}
	hash := sha256.New()
	n, readErr := io.Copy(hash, io.LimitReader(file, (512<<20)+1))
	closeErr := file.Close()
	if readErr != nil || closeErr != nil || n > 512<<20 || fmt.Sprintf("%x", hash.Sum(nil)) != "b973d440acac501fd2594a43e7ca9ce41e0a65b9dfb28d0d7a7837c99e1261e3" {
		t.Fatal("native binary differs from the fixed 0.153.4 executable")
	}
	cfg, opts := pluginSkillPolicyOptions(t)
	selected := opts.CodexPluginSkillBindings[0].SkillPath
	cachePluginWrite(t, filepath.Join(filepath.Dir(filepath.Dir(filepath.Dir(selected))), ".mcp.json"), "{}")
	private := t.TempDir()
	env := map[string]string{"PATH": "/usr/bin:/bin", "HOME": filepath.Join(private, "home"), "CODEX_HOME": cfg.Env["CODEX_HOME"],
		"XDG_CONFIG_HOME": filepath.Join(private, "xdg-config"), "XDG_CACHE_HOME": filepath.Join(private, "xdg-cache"),
		"XDG_DATA_HOME": filepath.Join(private, "xdg-data"), "TMPDIR": filepath.Join(private, "tmp"), "NO_PROXY": "*"}
	for key, value := range env {
		if key == "PATH" || key == "NO_PROXY" {
			continue
		}
		if err := os.MkdirAll(value, 0o700); err != nil {
			t.Fatal(err)
		}
	}
	var providerRequests atomic.Int32
	provider := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		providerRequests.Add(1)
		w.WriteHeader(http.StatusForbidden)
	}))
	defer provider.Close()
	configPath := filepath.Join(env["CODEX_HOME"], "config.toml")
	cachePluginWrite(t, configPath, fmt.Sprintf(`model = "local-policy-probe"
model_provider = "local_fixture"
approval_policy = "never"
sandbox_mode = "read-only"
check_for_update_on_startup = false
[analytics]
enabled = false
[otel]
exporter = "none"
[model_providers.local_fixture]
name = "Local fixture"
base_url = %q
wire_api = "responses"
requires_openai_auth = false
[plugins.'probe@local']
enabled = true
[[skills.config]]
path = %q
enabled = false
`, provider.URL+"/v1", selected))
	if err := os.MkdirAll(filepath.Join(opts.Cwd, ".codex"), 0o700); err != nil {
		t.Fatal(err)
	}
	other := filepath.Join(private, "unrelated", "SKILL.md")
	cachePluginWrite(t, filepath.Join(opts.Cwd, ".codex", "config.toml"), fmt.Sprintf("[[skills.config]]\npath = %q\nenabled = true\n[[skills.config]]\npath = %q\nenabled = true\n", selected, other))
	if opts.Cwd, err = ensureCodexProjectConfiguration(env["CODEX_HOME"], opts.Cwd, env["HOME"], "trusted"); err != nil {
		t.Fatal("private project configuration failed")
	}
	ctx, cancel := context.WithTimeout(t.Context(), 20*time.Second)
	defer cancel()
	var diagnostics syncBuffer
	logger := slog.New(slog.NewJSONHandler(&diagnostics, nil))
	versionCmd := exec.CommandContext(ctx, bin, "--version")
	versionCmd.Env = mergeEnv(nil, env)
	versionCmd.Dir = opts.Cwd
	var version bytes.Buffer
	versionCmd.Stdout, versionCmd.Stderr = &version, io.Discard
	configureProcessGroup(versionCmd)
	versionCmd.Cancel = func() error { signalProcessGroup(versionCmd, syscall.SIGKILL); return nil }
	versionCmd.WaitDelay = time.Second
	if err := startOwnedProcessTree(versionCmd, logger); err != nil {
		t.Fatal("native version process failed")
	}
	versionErr := versionCmd.Wait()
	signalProcessGroup(versionCmd, syscall.SIGKILL)
	versionGone := waitRealCodexWatchdogProcessGroupGone(versionCmd.Process.Pid, time.Second)
	releaseProcessGroup(versionCmd)
	if versionErr != nil || !versionGone || strings.TrimSpace(version.String()) != "codex-cli 0.153.4" {
		t.Fatal("native version/cleanup verification failed")
	}
	cfg.ExecutablePath, cfg.CLIVersion, cfg.Env, cfg.Logger = bin, strings.TrimSpace(version.String()), env, logger
	b := &codexBackend{cfg: cfg}
	opts, err = b.prepareCodexPluginSkillPolicy(opts)
	if err != nil {
		t.Fatal("selected installation preparation failed")
	}
	cmd := exec.CommandContext(ctx, bin, "app-server", "--listen", "stdio://")
	cmd.Dir, cmd.Env, cmd.Stderr = opts.Cwd, mergeEnv(nil, env), io.Discard
	stdin, err := cmd.StdinPipe()
	if err != nil {
		t.Fatal(err)
	}
	stdout, err := cmd.StdoutPipe()
	if err != nil {
		t.Fatal(err)
	}
	configureProcessGroup(cmd)
	cmd.Cancel = func() error { signalProcessGroup(cmd, syscall.SIGKILL); return nil }
	cmd.WaitDelay = time.Second
	if err := startOwnedProcessTree(cmd, logger); err != nil {
		t.Fatal("native app-server failed")
	}
	var requests bytes.Buffer
	client := &codexClient{cfg: cfg, stdin: io.MultiWriter(&requests, stdin), pending: make(map[int]*pendingRPC), processDone: make(chan struct{}), handshakeTimeout: 5 * time.Second, threadHandshakeTimeout: 5 * time.Second}
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
		_ = cmd.Wait()
		<-done
		gone := waitRealCodexWatchdogProcessGroupGone(cmd.Process.Pid, time.Second)
		releaseProcessGroup(cmd)
		if !gone {
			t.Error("native process group cleanup was not proved")
		}
		if providerRequests.Load() != 0 {
			t.Error("no-model probe reached local provider")
		}
	}()
	if _, err := client.request(ctx, "initialize", map[string]any{"clientInfo": map[string]any{"name": "multica-skill-policy-no-model", "version": "1"}, "capabilities": map[string]any{"experimentalApi": true}}); err != nil {
		t.Fatal("native initialize failed")
	}
	client.notify("initialized")
	effective, err := readCodexConfiguration(ctx, client, opts.Cwd, codexConfigurationReadPurposeTaskEffective)
	if err != nil {
		t.Fatal("native effective configuration unavailable")
	}
	entries, err := codexPluginSkillEntries(effective.Config)
	if err != nil {
		t.Fatal("native skills config shape is unsupported")
	}
	if len(entries) != 2 || entries[0]["enabled"] != true {
		t.Fatal("project override was not exercised")
	}
	thread, resumed, err := client.startOrResumeThread(ctx, opts, logger)
	if err != nil || resumed || thread == "" {
		t.Fatalf("native selected-skill start failed: %v", err)
	}
	opts.ResumeSessionID = thread
	// A zero-turn thread has no rollout to resume in this version. Exercise the
	// real rejection/fallback without creating a model turn to seed history.
	if actual, resumed, err := client.startOrResumeThread(ctx, opts, logger); err != nil || resumed || actual == "" || actual == thread {
		t.Fatalf("native zero-turn resume/fallback failed: %v", err)
	}
	missingRollout := false
	for _, line := range strings.Split(diagnostics.String(), "\n") {
		var event map[string]any
		if json.Unmarshal([]byte(line), &event) == nil && event["msg"] == "codex thread/resume failed; falling back to thread/start" {
			missingRollout = strings.HasPrefix(fmt.Sprint(event["error"]), "thread/resume: no rollout found for thread id ")
		}
	}
	if !missingRollout {
		t.Fatal("native resume failed for a reason other than the zero-turn rollout boundary")
	}
	threadRequests := 0
	for _, line := range strings.Split(strings.TrimSpace(requests.String()), "\n") {
		var request struct {
			Method string `json:"method"`
			Params struct {
				Config map[string]any `json:"config"`
			} `json:"params"`
		}
		if json.Unmarshal([]byte(line), &request) != nil {
			t.Fatal("invalid native request record")
		}
		if request.Method == "turn/start" {
			t.Fatal("no-model probe sent turn/start")
		}
		if request.Method != "thread/start" && request.Method != "thread/resume" {
			continue
		}
		threadRequests++
		pinned, err := codexPluginSkillEntries(request.Params.Config)
		if err != nil || len(pinned) != 2 {
			t.Fatal("native thread skills pin missing")
		}
		for _, entry := range pinned {
			if entry["enabled"] != (entry["path"] == other) {
				t.Fatal("native thread selected/unrelated skill policy mismatch")
			}
		}
	}
	if threadRequests != 3 {
		t.Fatal("native start/resume-attempt/fallback count mismatch")
	}
	t.Log("pinned CLI: project override observed; start/resume-attempt/fallback pins verified; turn/start=0; provider requests=0")
}
