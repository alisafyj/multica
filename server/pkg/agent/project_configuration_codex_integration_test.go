package agent

import (
	"bufio"
	"context"
	"crypto/sha256"
	"encoding/json"
	"fmt"
	"io"
	"log/slog"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

func TestCodexRealProjectConfigurationConfigRead(t *testing.T) {
	if os.Getenv("MULTICA_TEST_REAL_CODEX_CONFIG_READ") != "1" {
		t.Skip("set MULTICA_TEST_REAL_CODEX_CONFIG_READ=1 to run the local no-model Codex config/read probe")
	}
	execPath := os.Getenv("MULTICA_TEST_REAL_CODEX_BIN")
	if !filepath.IsAbs(execPath) {
		t.Fatal("set MULTICA_TEST_REAL_CODEX_BIN to the pinned native Codex executable")
	}
	executable, err := os.ReadFile(execPath)
	if err != nil {
		t.Fatal("read pinned native Codex executable")
	}
	const expectedBinarySHA256 = "b973d440acac501fd2594a43e7ca9ce41e0a65b9dfb28d0d7a7837c99e1261e3"
	if fmt.Sprintf("%x", sha256.Sum256(executable)) != expectedBinarySHA256 {
		t.Fatal("native Codex executable differs from the frozen 0.153.4 binary")
	}
	nodePath, err := exec.LookPath("node")
	if err != nil {
		t.Skipf("node unavailable for local MCP fixture: %v", err)
	}
	home := t.TempDir()
	cwd := t.TempDir()
	fixturePath := filepath.Join(t.TempDir(), "fixture-mcp.mjs")
	if err := os.WriteFile(fixturePath, []byte(`
import { createInterface } from "node:readline";
const lines = createInterface({ input: process.stdin });
for await (const line of lines) {
  let request;
  try { request = JSON.parse(line); } catch { continue; }
  if (request.id === undefined) continue;
  let result;
  if (request.method === "initialize") {
    result = { protocolVersion: "2024-11-05", capabilities: { tools: {} }, serverInfo: { name: "multica-local-proof", version: "1.0.0" } };
  } else if (request.method === "tools/list") {
    result = { tools: [{ name: "probe", description: "Return local enforcement evidence.", inputSchema: { type: "object", properties: {}, additionalProperties: false } }] };
  } else if (request.method === "tools/call" && request.params?.name === "probe") {
    result = { content: [{ type: "text", text: JSON.stringify({ marker: "multica-local-mcp-proof", authorized: process.env.FIXTURE_TOKEN === "fixture-secret", unapprovedAbsent: !process.env.UNAPPROVED_SECRET }) }], isError: false };
  } else if (request.method === "resources/list") {
    result = { resources: [] };
  } else if (request.method === "resources/templates/list") {
    result = { resourceTemplates: [] };
  } else if (request.method === "ping") {
    result = {};
  } else {
    process.stdout.write(JSON.stringify({ jsonrpc: "2.0", id: request.id, error: { code: -32601, message: "unsupported fixture method" } }) + "\n");
    continue;
  }
  process.stdout.write(JSON.stringify({ jsonrpc: "2.0", id: request.id, result }) + "\n");
}
`), 0o600); err != nil {
		t.Fatal(err)
	}
	if err := os.Mkdir(filepath.Join(cwd, ".codex"), 0o700); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(home, "config.toml"), []byte(strings.Join([]string{
		`approval_policy = "never"`,
		`sandbox_mode = "read-only"`,
		`shell_environment_policy.inherit = "none"`,
		`features.multi_agent = false`,
		`features.memories = false`,
		`memories.generate_memories = false`,
		`memories.use_memories = false`,
		``,
	}, "\n")), 0o600); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(cwd, ".codex", "config.toml"), []byte(strings.Join([]string{
		`model = "project-native-model"`,
		`model_reasoning_effort = "ultra"`,
		``,
	}, "\n")), 0o600); err != nil {
		t.Fatal(err)
	}
	canonicalCwd, err := ensureCodexProjectConfiguration(home, cwd, t.TempDir(), "trusted")
	if err != nil {
		t.Fatalf("prepare trusted config: %v", err)
	}
	managedBytes, err := json.Marshal(map[string]any{"mcpServers": map[string]any{
		"managed":  map[string]any{"command": nodePath, "args": []string{fixturePath}, "env": map[string]string{"FIXTURE_TOKEN": "fixture-secret"}},
		"remote":   map[string]any{"type": "http", "url": "https://example.invalid/mcp", "headers": map[string]string{"X-Test": "ok"}},
		"disabled": map[string]any{"command": nodePath, "args": []string{fixturePath}, "enabled": false},
	}})
	if err != nil {
		t.Fatal(err)
	}
	managed := json.RawMessage(managedBytes)
	if err := ensureCodexMcpConfig(filepath.Join(home, "config.toml"), managed, slog.Default()); err != nil {
		t.Fatalf("prepare managed MCP config: %v", err)
	}

	cmd := exec.Command(execPath, "app-server", "--listen", "stdio://")
	cmd.Dir = canonicalCwd
	cmd.Env = []string{
		"PATH=" + os.Getenv("PATH"),
		"HOME=" + t.TempDir(),
		"CODEX_HOME=" + home,
		"TMPDIR=" + os.TempDir(),
		"UNAPPROVED_SECRET=must-not-reach-tool",
	}
	stdin, err := cmd.StdinPipe()
	if err != nil {
		t.Fatal(err)
	}
	stdout, err := cmd.StdoutPipe()
	if err != nil {
		t.Fatal(err)
	}
	cmd.Stderr = io.Discard
	if err := cmd.Start(); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() {
		_ = stdin.Close()
		if cmd.Process != nil {
			_ = cmd.Process.Kill()
		}
		_ = cmd.Wait()
	})

	var observationLog syncBuffer
	client := &codexClient{
		cfg: Config{
			Logger:    slog.New(slog.NewJSONHandler(&observationLog, nil)).With("component", "daemon"),
			WorkDir:   canonicalCwd,
			TaskID:    "c3ca01ad-1c4a-4b6a-a058-54c4bb88d9ca",
			RuntimeID: "0a157898-cce6-4bde-888e-129eb8cd8fa8",
		},
		pid:              cmd.Process.Pid,
		attempt:          1,
		stdin:            stdin,
		pending:          make(map[int]*pendingRPC),
		processDone:      make(chan struct{}),
		handshakeTimeout: 5 * time.Second,
	}
	readerDone := make(chan struct{})
	go func() {
		defer close(readerDone)
		scanner := bufio.NewScanner(stdout)
		for scanner.Scan() {
			client.handleLine(scanner.Text())
		}
	}()

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	if _, err := client.request(ctx, "initialize", map[string]any{
		"clientInfo":   map[string]any{"name": "multica-config-read-test", "version": "1.0"},
		"capabilities": map[string]any{"experimentalApi": true},
	}); err != nil {
		t.Fatalf("initialize: %v", err)
	}
	client.notify("initialized")
	if err := verifyCodexProjectConfiguration(ctx, client, ExecOptions{
		Cwd:                        canonicalCwd,
		ProjectConfigurationPolicy: "trusted",
		McpConfig:                  managed,
	}); err != nil {
		t.Fatalf("real config/read preflight: %v", err)
	}
	effective, err := readCodexConfiguration(ctx, client, canonicalCwd, codexConfigurationReadPurposeTaskEffective)
	if err != nil {
		t.Fatal(err)
	}
	if got, _ := effective.Config["model"].(string); got != "project-native-model" {
		t.Fatalf("trusted project model = %q, want project-native-model", got)
	}
	if got, _ := effective.Config["model_reasoning_effort"].(string); got != "ultra" {
		t.Fatalf("trusted project reasoning effort = %q, want ultra", got)
	}
	observationCount := 0
	for _, line := range strings.Split(observationLog.String(), "\n") {
		var entry struct {
			Time          time.Time                    `json:"time"`
			Message       string                       `json:"msg"`
			TaskID        string                       `json:"task_id"`
			RuntimeID     string                       `json:"runtime_id"`
			PID           int                          `json:"pid"`
			Attempt       int                          `json:"attempt"`
			Cwd           string                       `json:"cwd"`
			Configuration codexConfigurationProjection `json:"configuration"`
		}
		if json.Unmarshal([]byte(line), &entry) != nil || entry.Message != "codex configuration observed" {
			continue
		}
		if entry.Time.IsZero() || entry.TaskID != client.cfg.TaskID || entry.RuntimeID != client.cfg.RuntimeID ||
			entry.PID != cmd.Process.Pid || entry.Attempt != 1 || entry.Cwd != canonicalCwd {
			t.Fatal("real configuration observation is not bound to the probe process")
		}
		if entry.Configuration.Model != projectCodexConfiguration(effective.Config).Model ||
			entry.Configuration.Model.State != codexConfigObserved || entry.Configuration.Completeness != "partial" {
			t.Fatal("real configuration observation does not match the projected config/read model")
		}
		for _, forbidden := range []string{"project-native-model", "fixture-secret", "example.invalid", "FIXTURE_TOKEN", "mcpServers", "mcp_servers"} {
			if strings.Contains(line, forbidden) {
				t.Fatal("configuration observation contains an unprojected field")
			}
		}
		observationCount++
		t.Logf("CONFIGURATION_OBSERVATION_FIXTURE %s", line)
	}
	if observationCount != 2 {
		t.Fatalf("got %d real configuration observations, want two task-cwd reads and no neutral-cwd read", observationCount)
	}
	threadID, resumed, err := client.startOrResumeThread(ctx, ExecOptions{
		Cwd: canonicalCwd, ThinkingLevel: "medium", ThreadName: "configuration-probe",
	}, client.cfg.Logger)
	if err != nil || resumed || threadID == "" {
		t.Fatalf("start native configuration probe thread: id=%q resumed=%t err=%v", threadID, resumed, err)
	}
	resumedID, resumed, err := client.startOrResumeThread(ctx, ExecOptions{
		Cwd: canonicalCwd, ResumeSessionID: threadID, ThinkingLevel: "medium",
	}, client.cfg.Logger)
	if err != nil || !resumed || resumedID != threadID {
		t.Fatalf("resume native configuration probe thread: id=%q resumed=%t err=%v", resumedID, resumed, err)
	}
	wantThreadDigest := projectCodexDigest(map[string]any{"thread_id": threadID}, "thread_id", "thread_id")
	threadObservationCount := 0
	for _, line := range strings.Split(observationLog.String(), "\n") {
		var entry struct {
			Message        string                             `json:"msg"`
			TaskID         string                             `json:"task_id"`
			RuntimeID      string                             `json:"runtime_id"`
			PID            int                                `json:"pid"`
			Attempt        int                                `json:"attempt"`
			Cwd            string                             `json:"cwd"`
			Method         string                             `json:"method"`
			ThreadIDDigest string                             `json:"thread_id_digest"`
			Configuration  codexThreadConfigurationProjection `json:"configuration"`
		}
		if json.Unmarshal([]byte(line), &entry) != nil || entry.Message != "codex thread configuration observed" {
			continue
		}
		if entry.TaskID != client.cfg.TaskID || entry.RuntimeID != client.cfg.RuntimeID ||
			entry.PID != cmd.Process.Pid || entry.Attempt != 1 || entry.Cwd != canonicalCwd ||
			(entry.Method != "thread/start" && entry.Method != "thread/resume") ||
			entry.ThreadIDDigest != wantThreadDigest.Digest {
			t.Fatal("real thread configuration observation is not bound to the probe process and thread")
		}
		if entry.Configuration.Schema != codexThreadConfigurationProjectionSchema ||
			entry.Configuration.Scope != "native_thread_setup_response_only" ||
			entry.Configuration.Model != projectCodexConfiguration(effective.Config).Model ||
			entry.Configuration.ReasoningEffort.State != codexConfigObserved ||
			entry.Configuration.ReasoningEffort.Value != "medium" {
			t.Fatalf("real thread configuration did not resolve the medium override: %#v", entry.Configuration)
		}
		for _, forbidden := range []string{threadID, "project-native-model", "fixture-secret", "example.invalid", "secret-instructions", "mcpServers"} {
			if strings.Contains(line, forbidden) {
				t.Fatal("thread configuration observation contains an unprojected field")
			}
		}
		threadObservationCount++
		t.Logf("THREAD_CONFIGURATION_OBSERVATION_FIXTURE %s", line)
	}
	if threadObservationCount != 2 {
		t.Fatalf("got %d real thread configuration observations, want start and resume", threadObservationCount)
	}
	threadRaw, err := client.request(ctx, "thread/start", map[string]any{
		"cwd":            canonicalCwd,
		"approvalPolicy": "never",
		"sandbox":        "read-only",
		"ephemeral":      true,
	})
	if err != nil {
		t.Fatalf("start no-model MCP probe thread: %v", err)
	}
	var threadResult struct {
		Thread struct {
			ID string `json:"id"`
		} `json:"thread"`
	}
	if err := json.Unmarshal(threadRaw, &threadResult); err != nil || threadResult.Thread.ID == "" {
		t.Fatalf("decode MCP probe thread: %v, response=%s", err, threadRaw)
	}
	inventory, err := client.request(ctx, "mcpServerStatus/list", map[string]any{
		"threadId": threadResult.Thread.ID,
		"detail":   "toolsAndAuthOnly",
	})
	if err != nil {
		t.Fatalf("list effective MCP tools: %v", err)
	}
	if !strings.Contains(string(inventory), `"name":"managed"`) || !strings.Contains(string(inventory), `"probe"`) {
		t.Fatalf("managed fixture MCP tool not discovered: %s", inventory)
	}
	invocation, err := client.request(ctx, "mcpServer/tool/call", map[string]any{
		"threadId":  threadResult.Thread.ID,
		"server":    "managed",
		"tool":      "probe",
		"arguments": map[string]any{},
	})
	if err != nil {
		t.Fatalf("invoke effective MCP tool: %v", err)
	}
	for _, evidence := range []string{`multica-local-mcp-proof`, `\"authorized\":true`, `\"unapprovedAbsent\":true`} {
		if !strings.Contains(string(invocation), evidence) {
			t.Fatalf("MCP invocation missing %q: %s", evidence, invocation)
		}
	}

	if err := os.WriteFile(filepath.Join(canonicalCwd, ".codex", "config.toml"), []byte(strings.Join([]string{
		`model = "project-native-model"`,
		`sandbox_permissions = ["disk-full-read-access"]`,
		``,
	}, "\n")), 0o600); err != nil {
		t.Fatal(err)
	}
	if err := verifyCodexProjectConfiguration(ctx, client, ExecOptions{
		Cwd:                        canonicalCwd,
		ProjectConfigurationPolicy: "trusted",
		McpConfig:                  managed,
	}); err == nil || !strings.Contains(err.Error(), "directly sets protected Codex setting sandbox_permissions") {
		t.Fatalf("real config/read accepted project sandbox_permissions override: %v", err)
	}

	_ = stdin.Close()
	select {
	case <-readerDone:
	case <-time.After(5 * time.Second):
		t.Fatal("codex app-server did not exit after stdin close")
	}
}
