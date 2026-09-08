//go:build !windows

package agent

import (
	"bufio"
	"bytes"
	"context"
	"encoding/json"
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
	"testing"
	"time"
)

// Opt-in, native no-auth probe. The loopback provider only asks Codex to run
// fixed local commands; no installed credentials or external model are used.
func TestCodexTaskWritableRootsNativeNoAuth(t *testing.T) {
	realCodex := strings.TrimSpace(os.Getenv("MULTICA_TEST_CODEX_TASK_WRITABLE_BINARY"))
	if realCodex == "" {
		t.Skip("set MULTICA_TEST_CODEX_TASK_WRITABLE_BINARY to run the isolated native writable-roots probe")
	}
	if !filepath.IsAbs(realCodex) {
		t.Fatal("MULTICA_TEST_CODEX_TASK_WRITABLE_BINARY must be an absolute path")
	}
	if _, err := os.Stat(realCodex); err != nil {
		t.Fatalf("native Codex CLI unavailable: %v", err)
	}
	node, err := exec.LookPath("node")
	if err != nil {
		t.Fatal("local Node fixture runtime unavailable")
	}

	cacheBase, err := os.UserCacheDir()
	if err != nil {
		t.Fatal(err)
	}
	root, err := os.MkdirTemp(cacheBase, "multica-codex-task-writable-")
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() {
		if err := os.RemoveAll(root); err != nil {
			t.Errorf("remove writable-roots fixture: %v", err)
		}
	})
	resolvedRoot, err := filepath.EvalSymlinks(root)
	if err != nil {
		t.Fatal(err)
	}
	if resolvedRoot == "/tmp" || strings.HasPrefix(resolvedRoot, "/tmp/") {
		t.Fatalf("fixture root must not be under /tmp: %s", resolvedRoot)
	}

	home := filepath.Join(resolvedRoot, "home")
	codexHome := filepath.Join(resolvedRoot, "codex-home")
	cwd := filepath.Join(resolvedRoot, "repo")
	taskCache := filepath.Join(resolvedRoot, "task-cache")
	existingRoot := filepath.Join(resolvedRoot, "existing-root")
	outside := filepath.Join(resolvedRoot, "outside")
	for _, dir := range []string{home, codexHome, cwd, taskCache, existingRoot, outside} {
		if err := os.MkdirAll(dir, 0o700); err != nil {
			t.Fatal(err)
		}
	}
	ownerProbe := filepath.Join(outside, "owner-write-probe.txt")
	if err := os.WriteFile(ownerProbe, []byte("owner-write-ok\n"), 0o600); err != nil {
		t.Fatalf("test owner cannot write unrelated-path fixture: %v", err)
	}
	if data, err := os.ReadFile(ownerProbe); err != nil || string(data) != "owner-write-ok\n" {
		t.Fatalf("test owner unrelated-path write was not durable: data=%q err=%v", data, err)
	}
	if err := os.Remove(ownerProbe); err != nil {
		t.Fatal(err)
	}

	gitCtx, gitCancel := context.WithTimeout(t.Context(), 5*time.Second)
	defer gitCancel()
	gitInit := exec.CommandContext(gitCtx, "git", "init", "--quiet", "--template=", cwd)
	gitInit.Env = []string{
		"PATH=" + os.Getenv("PATH"), "HOME=" + home,
		"GIT_CONFIG_NOSYSTEM=1", "GIT_CONFIG_GLOBAL=" + os.DevNull,
	}
	if output, err := gitInit.CombinedOutput(); err != nil {
		t.Fatalf("initialize fixture repository: %v: %s", err, output)
	}

	type providerObservation struct {
		RequestCount int
		ToolOutputs  []string
		Unexpected   []string
	}
	var (
		observation   providerObservation
		observationMu sync.Mutex
	)
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
			t.Errorf("read loopback provider request: %v", err)
			w.WriteHeader(http.StatusInternalServerError)
			return
		}
		var request map[string]any
		if err := json.Unmarshal(body, &request); err != nil {
			t.Errorf("decode loopback provider request: %v", err)
			w.WriteHeader(http.StatusBadRequest)
			return
		}

		observationMu.Lock()
		observation.RequestCount++
		requestNumber := observation.RequestCount
		if requestNumber == 3 || requestNumber == 5 {
			input, _ := request["input"].([]any)
			for _, raw := range input {
				item, _ := raw.(map[string]any)
				if item["type"] == "function_call_output" {
					observation.ToolOutputs = append(observation.ToolOutputs, fmt.Sprint(item["output"]))
				}
			}
		}
		observationMu.Unlock()

		w.Header().Set("Content-Type", "text/event-stream")
		w.Header().Set("Cache-Control", "no-cache")
		if requestNumber == 1 {
			item := map[string]any{
				"type": "message", "id": "msg_task_writable_history", "status": "completed", "role": "assistant",
				"content": []any{map[string]any{"type": "output_text", "text": "task-writable-history-complete", "annotations": []any{}}},
			}
			writeRealCodexWatchdogResponse(t, w, "resp_task_writable_history", item)
			return
		}
		if requestNumber == 2 || requestNumber == 4 {
			runName := map[int]string{2: "start", 4: "resume"}[requestNumber]
			callID := "call_task_writable_" + runName
			outsideProbe := `const fs=require("node:fs");try{fs.writeFileSync(process.argv[1],"escaped\n");console.log("outside-write-succeeded")}catch(error){console.log("outside-write-denied:"+error.code);if(error.code!=="EPERM"&&error.code!=="EACCES")process.exit(23)}`
			command := strings.Join([]string{
				"set -eu",
				"printf '%s\\n' " + realCodexWatchdogShellQuote(runName) + " > " + realCodexWatchdogShellQuote(filepath.Join(taskCache, runName+".txt")),
				"printf '%s\\n' " + realCodexWatchdogShellQuote(runName) + " > " + realCodexWatchdogShellQuote(filepath.Join(cwd, runName+".txt")),
				realCodexWatchdogShellQuote(node) + " -e " + realCodexWatchdogShellQuote(outsideProbe) + " " + realCodexWatchdogShellQuote(filepath.Join(outside, runName+".txt")),
			}, "; ")
			item := map[string]any{
				"type": "function_call", "id": "fc_task_writable_" + runName, "call_id": callID,
				"name": "exec_command", "arguments": mustRealCodexWatchdogJSON(t, map[string]any{"cmd": command}), "status": "completed",
			}
			writeRealCodexWatchdogResponse(t, w, "resp_task_writable_tool_"+runName, item)
			return
		}
		runName := map[int]string{3: "start", 5: "resume"}[requestNumber]
		item := map[string]any{
			"type": "message", "id": "msg_task_writable_" + runName, "status": "completed", "role": "assistant",
			"content": []any{map[string]any{"type": "output_text", "text": "task-writable-" + runName + "-complete", "annotations": []any{}}},
		}
		writeRealCodexWatchdogResponse(t, w, "resp_task_writable_final_"+runName, item)
	}))
	t.Cleanup(provider.Close)

	config := strings.Join([]string{
		`model = "task-writable-probe-model"`,
		`model_provider = "local_fixture"`,
		`approval_policy = "never"`,
		`sandbox_mode = "workspace-write"`,
		`disable_response_storage = true`,
		`hide_agent_reasoning = true`,
		`[sandbox_workspace_write]`,
		// CLI selects a different root; the disk-only root must lose write access.
		`writable_roots = [` + strconv.Quote(outside) + `]`,
		`network_access = false`,
		`exclude_tmpdir_env_var = true`,
		`exclude_slash_tmp = true`,
		`[model_providers.local_fixture]`,
		`name = "Loopback writable-roots fixture"`,
		`base_url = ` + strconv.Quote(provider.URL+"/v1"),
		`wire_api = "responses"`,
		`requires_openai_auth = false`,
		`[features]`,
		`responses_websockets = false`,
		`multi_agent = false`,
		`memories = false`,
		`plugins = false`,
		``,
	}, "\n")
	configPath := filepath.Join(codexHome, "config.toml")
	if err := os.WriteFile(configPath, []byte(config), 0o600); err != nil {
		t.Fatal(err)
	}

	rpcLog := filepath.Join(resolvedRoot, "app-server-rpc.jsonl")
	responseLog := filepath.Join(resolvedRoot, "app-server-response.jsonl")
	argsLog := filepath.Join(resolvedRoot, "app-server-args.txt")
	appPIDPath := filepath.Join(resolvedRoot, "app-server.pid")
	wrapper := filepath.Join(resolvedRoot, "codex-isolated")
	wrapperBody := "#!/bin/sh\n" +
		"if [ \"${1:-}\" = \"app-server\" ]; then\n" +
		"  " + realCodexWatchdogWriteIdentityCommand(appPIDPath) + "\n" +
		"  printf '%s\\n' \"$@\" >> " + realCodexWatchdogShellQuote(argsLog) + "\n" +
		"  /usr/bin/tee -a " + realCodexWatchdogShellQuote(rpcLog) + " | env -i " +
		"PATH=" + realCodexWatchdogShellQuote(os.Getenv("PATH")) + " " +
		"HOME=" + realCodexWatchdogShellQuote(home) + " " +
		"CODEX_HOME=" + realCodexWatchdogShellQuote(codexHome) + " " +
		"TMPDIR=" + realCodexWatchdogShellQuote(os.TempDir()) + " " +
		"HTTP_PROXY=" + realCodexWatchdogShellQuote(provider.URL) + " " +
		"HTTPS_PROXY=" + realCodexWatchdogShellQuote(provider.URL) + " " +
		"ALL_PROXY=" + realCodexWatchdogShellQuote(provider.URL) + " " +
		"NO_PROXY='127.0.0.1,localhost,::1' OTEL_SDK_DISABLED=true " +
		realCodexWatchdogShellQuote(realCodex) + " \"$@\" | /usr/bin/tee -a " + realCodexWatchdogShellQuote(responseLog) + "\n" +
		"  exit $?\n" +
		"fi\n" +
		"exec env -i PATH=" + realCodexWatchdogShellQuote(os.Getenv("PATH")) + " HOME=" + realCodexWatchdogShellQuote(home) + " CODEX_HOME=" + realCodexWatchdogShellQuote(codexHome) + " " + realCodexWatchdogShellQuote(realCodex) + " \"$@\"\n"
	if err := os.WriteFile(wrapper, []byte(wrapperBody), 0o700); err != nil {
		t.Fatal(err)
	}

	versionCtx, versionCancel := context.WithTimeout(t.Context(), 2*time.Second)
	versionOutput, err := exec.CommandContext(versionCtx, wrapper, "--version").Output()
	versionCancel()
	if err != nil {
		t.Fatalf("read isolated Codex version: %v", err)
	}
	if version := strings.TrimSpace(string(versionOutput)); version != "codex-cli 0.153.4" {
		t.Fatalf("native probe requires codex-cli 0.153.4, got %q", version)
	}

	backend, err := New("codex", Config{
		ExecutablePath: wrapper,
		CodexVersion:   "0.153.4-native-task-writable-probe",
		Env:            map[string]string{"HOME": home, "CODEX_HOME": codexHome},
		Logger:         slog.New(slog.NewTextHandler(io.Discard, nil)),
		WorkDir:        cwd,
	})
	if err != nil {
		t.Fatal(err)
	}
	historyCtx, historyCancel := context.WithTimeout(context.Background(), 15*time.Second)
	historySession, err := backend.Execute(historyCtx, "Return the fixed historical fixture answer.", ExecOptions{
		Cwd: cwd,
		CustomArgs: []string{
			"-c", `approval_policy="on-request"`,
			"-c", `sandbox_mode="danger-full-access"`,
		},
		Timeout:                10 * time.Second,
		HandshakeTimeout:       5 * time.Second,
		ThreadHandshakeTimeout: 5 * time.Second,
	})
	if err != nil {
		historyCancel()
		t.Fatal(err)
	}
	for range historySession.Messages {
	}
	historyResult, ok := <-historySession.Result
	if !ok {
		historyCancel()
		t.Fatal("historical result channel closed without a value")
	}
	if historyResult.Status != "completed" || historyResult.Output != "task-writable-history-complete" || historyResult.SessionID == "" {
		historyCancel()
		t.Fatalf("historical native result: %+v", historyResult)
	}
	historicalThreadID := historyResult.SessionID
	historyPID := readRealCodexWatchdogPID(t, appPIDPath)
	if !waitRealCodexWatchdogProcessGroupGone(historyPID, 2*time.Second) {
		cleanupRealCodexWatchdogProcessGroup(t, historyCancel, appPIDPath)
		t.Fatalf("historical app-server process group %d remained alive after result", historyPID)
	}
	historyCancel()
	var historicalResponse json.RawMessage
	for _, response := range readCodexTaskWritableRPCs(t, responseLog) {
		result, _ := response["result"].(map[string]any)
		thread, _ := result["thread"].(map[string]any)
		if thread["id"] != historicalThreadID {
			continue
		}
		historicalResponse, err = json.Marshal(result)
		if err != nil {
			t.Fatal(err)
		}
		break
	}
	if historicalResponse == nil {
		t.Fatalf("native response log has no historical thread %q", historicalThreadID)
	}
	assertCodexTaskWritableAuthorization(t, historicalResponse, "on-request", "dangerFullAccess", nil)

	for _, tc := range []struct {
		name     string
		resumeID string
		want     string
	}{
		{name: "start", want: "task-writable-start-complete"},
		{name: "resume", resumeID: historicalThreadID, want: "task-writable-resume-complete"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			ctx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
			defer cancel()
			session, err := backend.Execute(ctx, "Run the fixture command and finish.", ExecOptions{
				Cwd:                    cwd,
				ResumeSessionID:        tc.resumeID,
				CodexTaskWritableRoots: []string{taskCache},
				CustomArgs: []string{
					"-c", "sandbox_workspace_write.writable_roots=[" + strconv.Quote(existingRoot) + "]",
				},
				Timeout:                10 * time.Second,
				HandshakeTimeout:       5 * time.Second,
				ThreadHandshakeTimeout: 5 * time.Second,
			})
			if err != nil {
				t.Fatal(err)
			}
			for range session.Messages {
			}
			result, ok := <-session.Result
			if !ok {
				t.Fatal("result channel closed without a value")
			}
			if result.Status != "completed" || result.Output != tc.want {
				t.Fatalf("native %s result: status=%q output=%q error=%q authorizations=%s", tc.name, result.Status, result.Output, result.Error, summarizeCodexTaskWritableNativeAuthorizations(responseLog))
			}
			if tc.resumeID != "" && result.SessionID != tc.resumeID {
				t.Fatalf("native resume session ID = %q, want historical thread %q", result.SessionID, tc.resumeID)
			}
			appPID := readRealCodexWatchdogPID(t, appPIDPath)
			if !waitRealCodexWatchdogProcessGroupGone(appPID, 2*time.Second) {
				cleanupRealCodexWatchdogProcessGroup(t, cancel, appPIDPath)
				t.Fatalf("native %s app-server process group %d remained alive after result", tc.name, appPID)
			}
		})
	}

	for _, runName := range []string{"start", "resume"} {
		for _, path := range []string{filepath.Join(taskCache, runName+".txt"), filepath.Join(cwd, runName+".txt")} {
			data, err := os.ReadFile(path)
			if err != nil || strings.TrimSpace(string(data)) != runName {
				t.Fatalf("expected exact writable root output at %s: data=%q err=%v", path, data, err)
			}
		}
		if _, err := os.Stat(filepath.Join(outside, runName+".txt")); !os.IsNotExist(err) {
			t.Fatalf("unrelated path write was not denied for %s: %v", runName, err)
		}
	}

	observationMu.Lock()
	observed := observation
	observationMu.Unlock()
	if observed.RequestCount != 5 || len(observed.ToolOutputs) != 2 {
		t.Fatalf("loopback provider lifecycle incomplete: %+v", observed)
	}
	deniedOutputs := 0
	for _, output := range observed.ToolOutputs {
		if strings.Contains(output, "outside-write-denied:EPERM") || strings.Contains(output, "outside-write-denied:EACCES") {
			deniedOutputs++
			continue
		}
		if strings.Contains(output, "outside-write-succeeded") {
			t.Fatalf("unexpected sandbox tool output: %s", output)
		}
	}
	if deniedOutputs != 2 {
		t.Fatalf("denied tool evidence incomplete: denied=%d outputs=%v", deniedOutputs, observed.ToolOutputs)
	}
	if len(observed.Unexpected) != 0 {
		t.Fatalf("unexpected loopback/proxy requests: %v", observed.Unexpected)
	}
	if after, err := os.ReadFile(configPath); err != nil || !strings.HasPrefix(string(after), config) {
		t.Fatalf("native task setup replaced or modified non-owned config: preserved_prefix=%t err=%v", strings.HasPrefix(string(after), config), err)
	}
	args, err := os.ReadFile(argsLog)
	if err != nil {
		t.Fatal(err)
	}
	customRootOverride := "sandbox_workspace_write.writable_roots=[" + strconv.Quote(existingRoot) + "]"
	if count := strings.Count(string(args), customRootOverride+"\n"); count != 2 {
		t.Fatalf("native app-server custom writable-root overrides = %d, want start and resume; args=%q", count, args)
	}

	rpcs := readCodexTaskWritableRPCs(t, rpcLog)
	threadStarts := 0
	for _, request := range rpcs {
		if request["method"] == "thread/start" {
			threadStarts++
		}
	}
	if threadStarts != 2 {
		t.Fatalf("native thread/start requests = %d, want historical setup plus one fresh product start", threadStarts)
	}
	assertCodexTaskWritableRPCOrder(t, rpcs, "thread/start", cwd, []string{existingRoot, taskCache})
	assertCodexTaskWritableRPCOrder(t, rpcs, "thread/resume", cwd, []string{existingRoot, taskCache})
	assertCodexTaskWritableNativeResponses(t, responseLog, []string{existingRoot, taskCache})
	t.Logf("native Codex task writable-roots evidence: version=0.153.4 provider=loopback-no-auth start=true resume_historical_authorization=true allowed_roots=2 outside_denied=true external_model=false credentials_inherited=false database_used=false")
}

func assertCodexTaskWritableAuthorization(t *testing.T, response json.RawMessage, approval, sandboxType string, writableRoots []string) {
	t.Helper()
	var actual struct {
		ApprovalPolicy string `json:"approvalPolicy"`
		Sandbox        struct {
			Type                string   `json:"type"`
			WritableRoots       []string `json:"writableRoots"`
			NetworkAccess       bool     `json:"networkAccess"`
			ExcludeSlashTmp     bool     `json:"excludeSlashTmp"`
			ExcludeTmpdirEnvVar bool     `json:"excludeTmpdirEnvVar"`
		} `json:"sandbox"`
	}
	if err := json.Unmarshal(response, &actual); err != nil {
		t.Fatalf("decode native thread authorization: %v: %s", err, response)
	}
	if actual.ApprovalPolicy != approval || actual.Sandbox.Type != sandboxType {
		t.Fatalf("native thread authorization = approval %q sandbox %q, want %q %q", actual.ApprovalPolicy, actual.Sandbox.Type, approval, sandboxType)
	}
	if writableRoots != nil && !equalCodexTaskWritableRoots(actual.Sandbox.WritableRoots, writableRoots) {
		t.Fatalf("native response writable roots = %v, want %v", actual.Sandbox.WritableRoots, writableRoots)
	}
}

func readCodexTaskWritableRPCs(t *testing.T, path string) []map[string]any {
	t.Helper()
	file, err := os.Open(path)
	if err != nil {
		t.Fatal(err)
	}
	defer file.Close()
	var result []map[string]any
	scanner := bufio.NewScanner(file)
	for scanner.Scan() {
		var request map[string]any
		if json.Unmarshal(scanner.Bytes(), &request) == nil {
			result = append(result, request)
		}
	}
	if err := scanner.Err(); err != nil {
		t.Fatal(err)
	}
	return result
}

func assertCodexTaskWritableNativeResponses(t *testing.T, path string, wantRoots []string) {
	t.Helper()
	responses := readCodexTaskWritableRPCs(t, path)
	matched := 0
	for _, response := range responses {
		result, _ := response["result"].(map[string]any)
		if result["approvalPolicy"] != "never" {
			continue
		}
		sandbox, _ := result["sandbox"].(map[string]any)
		if sandbox["type"] != "workspaceWrite" {
			continue
		}
		roots := codexTaskWritableStringSlice(sandbox["writableRoots"])
		if !equalCodexTaskWritableRoots(roots, wantRoots) ||
			sandbox["networkAccess"] != false ||
			sandbox["excludeSlashTmp"] != true ||
			sandbox["excludeTmpdirEnvVar"] != true {
			t.Fatalf("native safe authorization response does not match effective task policy: %#v", result)
		}
		matched++
	}
	if matched != 2 {
		t.Fatalf("native safe authorization responses = %d, want start and resume", matched)
	}
}

func summarizeCodexTaskWritableNativeAuthorizations(path string) string {
	data, err := os.ReadFile(path)
	if err != nil {
		return "unavailable: " + err.Error()
	}
	var summaries []string
	scanner := bufio.NewScanner(bytes.NewReader(data))
	for scanner.Scan() {
		var response map[string]any
		if json.Unmarshal(scanner.Bytes(), &response) != nil {
			continue
		}
		result, _ := response["result"].(map[string]any)
		approval, _ := result["approvalPolicy"].(string)
		sandbox, _ := result["sandbox"].(map[string]any)
		sandboxType, _ := sandbox["type"].(string)
		if approval == "" || sandboxType == "" {
			continue
		}
		summaries = append(summaries, fmt.Sprintf("approval=%s sandbox=%s roots=%v network=%v excludeSlashTmp=%v excludeTmpdir=%v", approval, sandboxType, sandbox["writableRoots"], sandbox["networkAccess"], sandbox["excludeSlashTmp"], sandbox["excludeTmpdirEnvVar"]))
	}
	if err := scanner.Err(); err != nil {
		return "unreadable: " + err.Error()
	}
	return strings.Join(summaries, "; ")
}

func assertCodexTaskWritableRPCOrder(t *testing.T, requests []map[string]any, method, cwd string, wantRoots []string) {
	t.Helper()
	setupIndex, configIndex, turnIndex, lastConfigIndex := -1, -1, -1, -1
	for index, request := range requests {
		requestMethod, _ := request["method"].(string)
		params, _ := request["params"].(map[string]any)
		if requestMethod == "config/read" && params["cwd"] == cwd {
			lastConfigIndex = index
		}
		if requestMethod == method {
			if method == "thread/resume" {
				if approval, _ := params["approvalPolicy"].(string); approval != "never" {
					continue
				}
			} else if approval, _ := params["approvalPolicy"].(string); approval != "never" {
				continue
			}
			setupIndex = index
			configIndex = lastConfigIndex
			if params["approvalPolicy"] != "never" || params["sandbox"] != "workspace-write" {
				t.Fatalf("%s authorization params = %#v", method, params)
			}
			config, _ := params["config"].(map[string]any)
			workspace, _ := config["sandbox_workspace_write"].(map[string]any)
			if workspace["network_access"] != false || workspace["exclude_slash_tmp"] != true || workspace["exclude_tmpdir_env_var"] != true {
				t.Fatalf("%s did not preserve workspace-write settings: %#v", method, workspace)
			}
			roots := codexTaskWritableStringSlice(workspace["writable_roots"])
			if !equalCodexTaskWritableRoots(roots, wantRoots) {
				t.Fatalf("%s writable roots = %v, want %v", method, roots, wantRoots)
			}
			continue
		}
		if setupIndex >= 0 && requestMethod == "turn/start" {
			turnIndex = index
			break
		}
	}
	if configIndex < 0 || setupIndex < 0 || turnIndex < 0 || configIndex >= setupIndex || setupIndex >= turnIndex {
		t.Fatalf("RPC order for %s = config/read:%d setup:%d turn/start:%d", method, configIndex, setupIndex, turnIndex)
	}
}

func codexTaskWritableStringSlice(value any) []string {
	values, _ := value.([]any)
	result := make([]string, 0, len(values))
	for _, value := range values {
		if text, ok := value.(string); ok {
			result = append(result, text)
		}
	}
	return result
}

func equalCodexTaskWritableRoots(got, want []string) bool {
	if len(got) != len(want) {
		return false
	}
	counts := make(map[string]int, len(want))
	for _, root := range want {
		counts[root]++
	}
	for _, root := range got {
		counts[root]--
	}
	for _, count := range counts {
		if count != 0 {
			return false
		}
	}
	return true
}
