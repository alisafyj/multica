//go:build !windows

package agent

import (
	"context"
	"crypto/rand"
	"encoding/json"
	"encoding/xml"
	"errors"
	"fmt"
	"io"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"strings"
	"sync"
	"syscall"
	"testing"
	"time"
)

func TestCodexRealCLISilentToolOutlivesSemanticInactivity(t *testing.T) {
	if os.Getenv("MULTICA_TEST_REAL_CODEX_WATCHDOG") != "1" {
		t.Skip("set MULTICA_TEST_REAL_CODEX_WATCHDOG=1 and MULTICA_TEST_REAL_CODEX_BIN to run the isolated real-CLI watchdog probe")
	}
	realCodex := strings.TrimSpace(os.Getenv("MULTICA_TEST_REAL_CODEX_BIN"))
	if !filepath.IsAbs(realCodex) {
		t.Fatal("MULTICA_TEST_REAL_CODEX_BIN must be an absolute path")
	}
	if _, err := os.Stat(realCodex); err != nil {
		t.Fatalf("real Codex CLI unavailable: %v", err)
	}

	const (
		semanticInactivity = 500 * time.Millisecond
		toolSleep          = 1500 * time.Millisecond
		minimumToolElapsed = 1300 * time.Millisecond
	)
	root, err := filepath.EvalSymlinks(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	home := filepath.Join(root, "home")
	codexHome := filepath.Join(root, "codex-home")
	cwd := filepath.Join(root, "repo")
	appPIDPath := filepath.Join(root, "app-server.pid")
	toolPIDPath := filepath.Join(cwd, "tool.pid")
	for _, dir := range []string{home, codexHome, cwd} {
		if err := os.MkdirAll(dir, 0o700); err != nil {
			t.Fatal(err)
		}
	}
	gitCtx, gitCancel := context.WithTimeout(t.Context(), 5*time.Second)
	defer gitCancel()
	gitInit := exec.CommandContext(gitCtx, "git", "init", "--quiet", "--template=", cwd)
	gitInit.Dir = root
	gitInit.Env = []string{
		"PATH=" + os.Getenv("PATH"), "HOME=" + home,
		"GIT_CONFIG_NOSYSTEM=1", "GIT_CONFIG_GLOBAL=" + os.DevNull,
	}
	if output, err := gitInit.CombinedOutput(); err != nil {
		t.Fatalf("initialize fixture Git main repository: %v: %s", err, output)
	}
	if info, err := os.Stat(filepath.Join(cwd, ".git")); err != nil || !info.IsDir() {
		t.Fatal("fixture must have its own Git directory, not a linked worktree")
	}
	agentsMarker := "MULTICA_R02_PRIMARY_AGENTS_" + rand.Text()
	if err := os.WriteFile(filepath.Join(cwd, "AGENTS.md"), []byte("# Fixture Repository Rules\n\nRetain this repository rule: "+agentsMarker+"\n"), 0o600); err != nil {
		t.Fatal(err)
	}

	type providerObservation struct {
		RequestCount       int
		FirstRequestAt     time.Time
		ContinuationAt     time.Time
		ToolAdvertised     bool
		ToolOutputObserved bool
		AgentsRuleObserved bool
		RequestCwdMatches  bool
		Unexpected         []string
	}
	var (
		observation   providerObservation
		observationMu sync.Mutex
	)
	callID := "call_real_watchdog_sleep"
	command := fmt.Sprintf("printf '%%s\\n' \"$$\" > %s; /bin/sleep %.3f; printf 'real-watchdog-tool-complete\\n'", realCodexWatchdogShellQuote(toolPIDPath), toolSleep.Seconds())

	provider := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost || r.URL.Path != "/v1/responses" {
			observationMu.Lock()
			observation.Unexpected = append(observation.Unexpected, r.Method+" "+r.Host+r.URL.RequestURI())
			observationMu.Unlock()
			w.WriteHeader(http.StatusNotFound)
			return
		}
		body, err := io.ReadAll(io.LimitReader(r.Body, 1<<20))
		if err != nil {
			t.Errorf("read provider request: %v", err)
			w.WriteHeader(http.StatusInternalServerError)
			return
		}
		var request map[string]any
		if err := json.Unmarshal(body, &request); err != nil {
			t.Errorf("decode provider request: %v", err)
			w.WriteHeader(http.StatusBadRequest)
			return
		}

		observationMu.Lock()
		observation.RequestCount++
		requestNumber := observation.RequestCount
		now := time.Now()
		if requestNumber == 1 {
			observation.FirstRequestAt = now
			// The marker exists only in AGENTS.md, never in the explicit prompt or SSE fixture.
			if input, ok := request["input"].([]any); ok {
				for _, raw := range input {
					item, _ := raw.(map[string]any)
					if item["type"] != "message" || item["role"] != "user" {
						continue
					}
					content, _ := item["content"].([]any)
					for _, rawPart := range content {
						part, _ := rawPart.(map[string]any)
						if part["type"] != "input_text" {
							continue
						}
						text, _ := part["text"].(string)
						if strings.HasPrefix(text, "# AGENTS.md instructions for "+cwd+"\n") && strings.Contains(text, agentsMarker) {
							observation.AgentsRuleObserved = true
						}
						var environment struct {
							XMLName xml.Name `xml:"environment_context"`
							Cwd     string   `xml:"cwd"`
						}
						if xml.Unmarshal([]byte(text), &environment) == nil && environment.Cwd == cwd {
							observation.RequestCwdMatches = true
						}
					}
				}
			}
			if tools, ok := request["tools"].([]any); ok {
				for _, raw := range tools {
					tool, _ := raw.(map[string]any)
					if tool["name"] == "exec_command" {
						observation.ToolAdvertised = true
					}
				}
			}
		} else if requestNumber == 2 {
			observation.ContinuationAt = now
			if input, ok := request["input"].([]any); ok {
				for _, raw := range input {
					item, _ := raw.(map[string]any)
					if item["type"] == "function_call_output" && item["call_id"] == callID && strings.Contains(fmt.Sprint(item["output"]), "real-watchdog-tool-complete") {
						observation.ToolOutputObserved = true
					}
				}
			}
		}
		observationMu.Unlock()

		w.Header().Set("Content-Type", "text/event-stream")
		w.Header().Set("Cache-Control", "no-cache")
		if requestNumber == 1 {
			item := map[string]any{
				"type": "function_call", "id": "fc_real_watchdog_sleep", "call_id": callID,
				"name": "exec_command", "arguments": mustRealCodexWatchdogJSON(t, map[string]any{"cmd": command}), "status": "completed",
			}
			writeRealCodexWatchdogResponse(t, w, "resp_real_watchdog_tool", item)
			return
		}
		item := map[string]any{
			"type": "message", "id": "msg_real_watchdog_final", "status": "completed", "role": "assistant",
			"content": []any{map[string]any{"type": "output_text", "text": "real-watchdog-complete", "annotations": []any{}}},
		}
		writeRealCodexWatchdogResponse(t, w, "resp_real_watchdog_final", item)
	}))
	t.Cleanup(provider.Close)

	providerURL := provider.URL + "/v1"
	config := strings.Join([]string{
		`model = "watchdog-probe-model"`,
		`model_provider = "local_fixture"`,
		`approval_policy = "never"`,
		`sandbox_mode = "workspace-write"`,
		`disable_response_storage = true`,
		`[model_providers.local_fixture]`,
		`name = "Loopback watchdog fixture"`,
		`base_url = ` + strconv.Quote(providerURL),
		`wire_api = "responses"`,
		`requires_openai_auth = false`,
		`[features]`,
		`responses_websockets = false`,
		`multi_agent = false`,
		`memories = false`,
		`plugins = false`,
		``,
	}, "\n")
	if err := os.WriteFile(filepath.Join(codexHome, "config.toml"), []byte(config), 0o600); err != nil {
		t.Fatal(err)
	}

	wrapper := filepath.Join(root, "codex-isolated")
	wrapperBody := "#!/bin/sh\n" +
		"if [ \"${1:-}\" = \"app-server\" ]; then " + realCodexWatchdogWriteIdentityCommand(appPIDPath) + "; fi\n" +
		"env -i " +
		"PATH=" + realCodexWatchdogShellQuote(os.Getenv("PATH")) + " " +
		"HOME=" + realCodexWatchdogShellQuote(home) + " " +
		"CODEX_HOME=" + realCodexWatchdogShellQuote(codexHome) + " " +
		"TMPDIR=" + realCodexWatchdogShellQuote(root) + " " +
		"HTTP_PROXY=" + realCodexWatchdogShellQuote(provider.URL) + " " +
		"HTTPS_PROXY=" + realCodexWatchdogShellQuote(provider.URL) + " " +
		"ALL_PROXY=" + realCodexWatchdogShellQuote(provider.URL) + " " +
		"NO_PROXY='127.0.0.1,localhost,::1' OTEL_SDK_DISABLED=true " +
		realCodexWatchdogShellQuote(realCodex) + " \"$@\"\n"
	if err := os.WriteFile(wrapper, []byte(wrapperBody), 0o700); err != nil {
		t.Fatal(err)
	}

	backend, err := New("codex", Config{
		ExecutablePath: wrapper,
		CodexVersion:   "0.153.4-real-loopback-probe",
		Logger:         slog.New(slog.NewTextHandler(io.Discard, nil)),
		WorkDir:        cwd,
	})
	if err != nil {
		t.Fatal(err)
	}
	ctx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
	t.Cleanup(func() {
		cleanupRealCodexWatchdogProcessGroup(t, cancel, appPIDPath)
	})
	executionStarted := time.Now()
	session, err := backend.Execute(ctx, "Execute the fixture-provided command and then finish.", ExecOptions{
		Cwd:                       cwd,
		Timeout:                   10 * time.Second,
		SemanticInactivityTimeout: semanticInactivity,
		HandshakeTimeout:          5 * time.Second,
		ThreadHandshakeTimeout:    5 * time.Second,
	})
	if err != nil {
		t.Fatal(err)
	}

	var toolStarted, toolCompleted bool
	for message := range session.Messages {
		if message.Type == MessageToolUse && message.Tool == "exec_command" && message.CallID == callID {
			toolStarted = true
		}
		if message.Type == MessageToolResult && message.CallID == callID {
			toolCompleted = true
		}
	}
	result, ok := <-session.Result
	if !ok {
		t.Fatal("result channel closed without a value")
	}
	executionElapsed := time.Since(executionStarted)
	if result.Status != "completed" || result.Output != "real-watchdog-complete" {
		t.Fatalf("real CLI result: status=%q output=%q error=%q", result.Status, result.Output, result.Error)
	}
	if !toolStarted || !toolCompleted {
		t.Fatalf("real command lifecycle missing: started=%t completed=%t", toolStarted, toolCompleted)
	}

	observationMu.Lock()
	observed := observation
	observationMu.Unlock()
	toolElapsed := observed.ContinuationAt.Sub(observed.FirstRequestAt)
	if observed.RequestCount != 2 || !observed.ToolAdvertised || !observed.ToolOutputObserved {
		t.Fatalf("provider continuation incomplete: %+v", observed)
	}
	if !observed.AgentsRuleObserved || !observed.RequestCwdMatches {
		t.Fatalf("first native model request missing primary repository context: agents_rule=%t cwd_matches=%t", observed.AgentsRuleObserved, observed.RequestCwdMatches)
	}
	if toolElapsed < minimumToolElapsed || toolElapsed <= semanticInactivity {
		t.Fatalf("tool elapsed %s, want >= %s and > inactivity %s", toolElapsed, minimumToolElapsed, semanticInactivity)
	}
	if len(observed.Unexpected) != 0 {
		t.Fatalf("unexpected loopback/proxy requests: %v", observed.Unexpected)
	}

	appPID := readRealCodexWatchdogPID(t, appPIDPath)
	toolPID := readRealCodexWatchdogPID(t, toolPIDPath)
	if !waitRealCodexWatchdogPIDGone(toolPID, 2*time.Second) {
		t.Fatalf("tool process %d remains alive", toolPID)
	}
	if !waitRealCodexWatchdogProcessGroupGone(appPID, 2*time.Second) {
		t.Fatalf("Codex process group %d remains alive", appPID)
	}
	versionCtx, versionCancel := context.WithTimeout(ctx, 2*time.Second)
	defer versionCancel()
	versionOutput, err := exec.CommandContext(versionCtx, wrapper, "--version").Output()
	if err != nil {
		t.Fatalf("read isolated Codex version: %v", err)
	}
	version := strings.TrimSpace(string(versionOutput))
	if version != "codex-cli 0.153.4" {
		t.Fatalf("real CLI probe requires codex-cli 0.153.4, got %q", version)
	}
	receipt := map[string]any{
		"codex_version":                 version,
		"provider":                      "loopback-responses-no-auth",
		"semantic_inactivity_ms":        semanticInactivity.Milliseconds(),
		"actual_tool_elapsed_ms":        toolElapsed.Milliseconds(),
		"execution_elapsed_ms":          executionElapsed.Milliseconds(),
		"tool_lifecycle_complete":       true,
		"result_status":                 result.Status,
		"result_output":                 result.Output,
		"provider_request_count":        observed.RequestCount,
		"primary_git_repository":        true,
		"agents_rule_in_first_request":  observed.AgentsRuleObserved,
		"first_request_cwd_matches":     observed.RequestCwdMatches,
		"agents_marker_source":          "fixture-primary-repository/AGENTS.md only",
		"r02_scope":                     "Codex native AGENTS loading into Responses input only",
		"unexpected_network_requests":   0,
		"global_credentials_inherited":  false,
		"external_model_used":           false,
		"codex_process_group_reaped":    true,
		"command_process_reaped":        true,
		"test_context_deadline_seconds": 15,
		"execution_deadline_seconds":    10,
	}
	if receiptPath := strings.TrimSpace(os.Getenv("MULTICA_TEST_REAL_CODEX_WATCHDOG_RECEIPT")); receiptPath != "" {
		encoded, err := json.MarshalIndent(receipt, "", "  ")
		if err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(receiptPath, append(encoded, '\n'), 0o600); err != nil {
			t.Fatalf("write watchdog receipt: %v", err)
		}
	}
	t.Logf("real Codex watchdog evidence: %s", mustRealCodexWatchdogJSON(t, receipt))
}

func writeRealCodexWatchdogResponse(t *testing.T, w io.Writer, responseID string, item map[string]any) {
	t.Helper()
	envelope := func(status string, output []any, usage any) map[string]any {
		return map[string]any{
			"id": responseID, "object": "response", "created_at": 0, "status": status,
			"error": nil, "incomplete_details": nil, "instructions": nil, "max_output_tokens": nil,
			"model": "watchdog-probe-model", "output": output, "parallel_tool_calls": false,
			"previous_response_id": nil, "reasoning": nil, "store": false, "temperature": nil,
			"text": map[string]any{"format": map[string]any{"type": "text"}}, "tool_choice": "auto",
			"tools": []any{}, "top_p": nil, "truncation": "disabled", "usage": usage,
			"user": nil, "metadata": map[string]any{},
		}
	}
	writeEvent := func(event map[string]any) {
		if _, err := fmt.Fprintf(w, "event: %s\ndata: %s\n\n", event["type"], mustRealCodexWatchdogJSON(t, event)); err != nil {
			t.Errorf("write SSE event: %v", err)
		}
	}
	writeEvent(map[string]any{"type": "response.created", "sequence_number": 0, "response": envelope("in_progress", []any{}, nil)})
	writeEvent(map[string]any{"type": "response.output_item.done", "sequence_number": 1, "output_index": 0, "item": item})
	usage := map[string]any{
		"input_tokens": 1, "input_tokens_details": map[string]any{"cached_tokens": 0},
		"output_tokens": 1, "output_tokens_details": map[string]any{"reasoning_tokens": 0}, "total_tokens": 2,
	}
	writeEvent(map[string]any{"type": "response.completed", "sequence_number": 2, "response": envelope("completed", []any{item}, usage)})
	_, _ = io.WriteString(w, "data: [DONE]\n\n")
}

func mustRealCodexWatchdogJSON(t *testing.T, value any) string {
	t.Helper()
	encoded, err := json.Marshal(value)
	if err != nil {
		t.Fatal(err)
	}
	return string(encoded)
}

func realCodexWatchdogShellQuote(value string) string {
	return "'" + strings.ReplaceAll(value, "'", "'\\''") + "'"
}

func realCodexWatchdogWriteIdentityCommand(markerPath string) string {
	return "START_ID=$(/bin/ps -p \"$$\" -o lstart=); printf '%s\\n%s\\n' \"$$\" \"$START_ID\" > " + realCodexWatchdogShellQuote(markerPath)
}

func readRealCodexWatchdogPID(t *testing.T, path string) int {
	t.Helper()
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("read PID marker: %v", err)
	}
	firstLine := strings.SplitN(strings.TrimSpace(string(data)), "\n", 2)[0]
	pid, err := strconv.Atoi(firstLine)
	if err != nil || pid <= 0 {
		t.Fatalf("invalid PID marker %q: %v", data, err)
	}
	return pid
}

func waitRealCodexWatchdogPIDGone(pid int, timeout time.Duration) bool {
	deadline := time.Now().Add(timeout)
	for {
		err := syscall.Kill(pid, 0)
		if errors.Is(err, syscall.ESRCH) {
			return true
		}
		if time.Now().After(deadline) {
			return false
		}
		time.Sleep(10 * time.Millisecond)
	}
}

func waitRealCodexWatchdogProcessGroupGone(pid int, timeout time.Duration) bool {
	deadline := time.Now().Add(timeout)
	for {
		err := syscall.Kill(-pid, 0)
		if errors.Is(err, syscall.ESRCH) {
			return true
		}
		if time.Now().After(deadline) {
			return false
		}
		time.Sleep(10 * time.Millisecond)
	}
}

func cleanupRealCodexWatchdogProcessGroup(t *testing.T, cancel context.CancelFunc, markerPath string) bool {
	t.Helper()
	cancel()
	identity, ok := waitRealCodexWatchdogProcessIdentity(markerPath, 500*time.Millisecond)
	if !ok || waitRealCodexWatchdogProcessGroupGone(identity.pid, time.Second) {
		return false
	}
	pgid, err := syscall.Getpgid(identity.pid)
	if errors.Is(err, syscall.ESRCH) {
		return false
	}
	if err != nil {
		t.Errorf("inspect Codex process group %d during cleanup: %v", identity.pid, err)
		return false
	}
	if pgid != identity.pid {
		t.Errorf("refusing to kill mismatched process group: marker pid=%d current pgid=%d", identity.pid, pgid)
		return false
	}
	currentStartID, err := realCodexWatchdogProcessStartID(identity.pid)
	if err != nil || currentStartID != identity.startID {
		t.Errorf("refusing to kill process group %d whose startup identity changed", identity.pid)
		return false
	}
	if err := syscall.Kill(-identity.pid, syscall.SIGKILL); err != nil && !errors.Is(err, syscall.ESRCH) {
		t.Errorf("kill Codex process group %d during cleanup: %v", identity.pid, err)
		return false
	}
	if !waitRealCodexWatchdogProcessGroupGone(identity.pid, 2*time.Second) {
		t.Errorf("Codex process group %d remained alive after cleanup SIGKILL", identity.pid)
		return false
	}
	return true
}

type realCodexWatchdogProcessIdentity struct {
	pid     int
	startID string
}

func waitRealCodexWatchdogProcessIdentity(path string, timeout time.Duration) (realCodexWatchdogProcessIdentity, bool) {
	deadline := time.Now().Add(timeout)
	for {
		data, err := os.ReadFile(path)
		if err == nil {
			parts := strings.SplitN(strings.TrimSpace(string(data)), "\n", 2)
			if len(parts) != 2 {
				return realCodexWatchdogProcessIdentity{}, false
			}
			pid, parseErr := strconv.Atoi(strings.TrimSpace(parts[0]))
			identity := realCodexWatchdogProcessIdentity{pid: pid, startID: strings.TrimSpace(parts[1])}
			return identity, parseErr == nil && identity.pid > 0 && identity.startID != ""
		}
		if !errors.Is(err, os.ErrNotExist) || time.Now().After(deadline) {
			return realCodexWatchdogProcessIdentity{}, false
		}
		time.Sleep(10 * time.Millisecond)
	}
}

func realCodexWatchdogProcessStartID(pid int) (string, error) {
	output, err := exec.Command("/bin/ps", "-p", strconv.Itoa(pid), "-o", "lstart=").Output()
	return strings.TrimSpace(string(output)), err
}

func TestRealCodexWatchdogCleanupKillsExecReplacedProcessGroup(t *testing.T) {
	root := t.TempDir()
	markerPath := filepath.Join(root, "exec-replaced.pid")
	wrapperPath := filepath.Join(root, "exec-replaced-wrapper")
	wrapper := "#!/bin/sh\n" + realCodexWatchdogWriteIdentityCommand(markerPath) + "\nexec /bin/sleep 30\n"
	if err := os.WriteFile(wrapperPath, []byte(wrapper), 0o700); err != nil {
		t.Fatal(err)
	}

	ctx, cancel := context.WithCancel(context.Background())
	cmd := exec.CommandContext(ctx, wrapperPath, "app-server")
	configureProcessGroup(cmd)
	cmd.Cancel = func() error { return nil }
	cmd.WaitDelay = 5 * time.Second
	if err := startOwnedProcessTree(cmd, slog.New(slog.NewTextHandler(io.Discard, nil))); err != nil {
		t.Fatal(err)
	}
	waitDone := make(chan struct{})
	go func() {
		_ = cmd.Wait()
		close(waitDone)
	}()
	t.Cleanup(func() {
		cancel()
		signalProcessGroup(cmd, syscall.SIGKILL)
		select {
		case <-waitDone:
		case <-time.After(2 * time.Second):
			t.Errorf("exec-replaced fixture did not exit during test cleanup")
		}
	})

	identity, ok := waitRealCodexWatchdogProcessIdentity(markerPath, time.Second)
	if !ok {
		t.Fatal("exec-replaced fixture did not write its startup identity")
	}
	command, err := exec.Command("/bin/ps", "-p", strconv.Itoa(identity.pid), "-o", "command=").Output()
	if err != nil {
		t.Fatalf("inspect exec-replaced fixture: %v", err)
	}
	if strings.Contains(string(command), wrapperPath) || !strings.Contains(string(command), "/bin/sleep 30") {
		t.Fatalf("fixture did not replace wrapper process: %q", command)
	}
	if killed := cleanupRealCodexWatchdogProcessGroup(t, cancel, markerPath); !killed {
		t.Fatal("failure cleanup did not take the verified SIGKILL branch")
	}
	select {
	case <-waitDone:
	case <-time.After(2 * time.Second):
		t.Fatal("exec-replaced process was not collected")
	}
	if !waitRealCodexWatchdogProcessGroupGone(identity.pid, time.Second) {
		t.Fatalf("exec-replaced process group %d remains alive", identity.pid)
	}
}
