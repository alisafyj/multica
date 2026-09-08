//go:build !windows

package daemon

import (
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/json"
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

type codexCapabilityExecutable struct {
	Role     string `json:"role"`
	RealPath string `json:"realpath"`
	SHA256   string `json:"sha256"`
	Version  string `json:"version"`
}

type pairWorkspace struct{ cwd, outside, gocache string }

func TestCodexCapabilityPairOwnedGroupCleanupRetainsUnprovenEvidence(t *testing.T) {
	root := t.TempDir()
	childPID := filepath.Join(root, "descendant.pid")
	cmd := exec.Command("/bin/sh", "-c", "sleep 30 & echo $! > "+pairShellQuote(childPID)+"; sleep 0.2; exit 17")
	cmd.SysProcAttr = &syscall.SysProcAttr{Setpgid: true}
	_, err, groupGone := pairRunOwnedCommand(cmd)
	if err == nil || !groupGone {
		t.Fatalf("failed synthetic leader did not reap its owned descendant group: err=%v gone=%t", err, groupGone)
	}
	pidText, err := os.ReadFile(childPID)
	if err != nil {
		t.Fatal(err)
	}
	pid, err := strconv.Atoi(strings.TrimSpace(string(pidText)))
	if err != nil || !errors.Is(syscall.Kill(pid, 0), syscall.ESRCH) {
		t.Fatalf("synthetic descendant remains alive: pid_valid=%t", err == nil)
	}
	recycled := exec.Command("/bin/sleep", "30")
	recycled.SysProcAttr = &syscall.SysProcAttr{Setpgid: true}
	if err := recycled.Start(); err != nil {
		t.Fatal(err)
	}
	owned, ok := pairCaptureOwnedGroup(recycled.Process.Pid)
	if !ok {
		_ = recycled.Process.Kill()
		_ = recycled.Wait()
		t.Fatal("could not capture synthetic recycled-PID guard group")
	}
	waited := make(chan struct{})
	go func() { _ = recycled.Wait(); close(waited) }()
	t.Cleanup(func() {
		_ = recycled.Process.Kill()
		if !pairStopOwnedGroup(owned, time.Second) {
			t.Error("synthetic recycled-PID guard group cleanup was not confirmed")
		}
		select {
		case <-waited:
		case <-time.After(time.Second):
			t.Error("synthetic recycled-PID guard process wait did not finish")
		}
	})
	wrongBirth := owned
	wrongBirth.birth += " mismatch"
	if pairStopOwnedGroup(wrongBirth, 100*time.Millisecond) || syscall.Kill(owned.pid, 0) != nil {
		t.Fatal("birth mismatch killed or accepted an unproven live group")
	}
	if !pairStopOwnedGroup(owned, time.Second) {
		t.Fatal("exact owned group could not be cleaned after mismatch refusal")
	}
	<-waited
	processes := filepath.Join(root, "processes")
	writePairFile(t, filepath.Join(processes, "not-a-pid"), "unproven\n")
	if pairCleanupRecordedGroups(processes) {
		t.Fatal("unproven marker was treated as an owned process group")
	}
	if pairRemoveRootWhenGroupsGone(root, false) {
		t.Fatal("root with unproven process evidence was removed")
	}
	if _, err := os.Stat(filepath.Join(processes, "not-a-pid")); err != nil {
		t.Fatal("unproven process marker was not retained")
	}
}

// TestCodexCapabilityPairRealCLI is a bounded, no-auth comparison of one native
// `codex exec` turn and one product daemon.runTask turn against the same tool call.
func TestCodexCapabilityPairRealCLI(t *testing.T) {
	if os.Getenv("MULTICA_TEST_REAL_CODEX_CAPABILITY") != "1" {
		t.Skip("set MULTICA_TEST_REAL_CODEX_CAPABILITY=1, MULTICA_TEST_REAL_CODEX_BIN, and MULTICA_TEST_CODEX_CAPABILITY_DEFINITION to opt in")
	}
	realCodex := strings.TrimSpace(os.Getenv("MULTICA_TEST_REAL_CODEX_BIN"))
	if !filepath.IsAbs(realCodex) {
		t.Fatal("MULTICA_TEST_REAL_CODEX_BIN must be an explicit absolute path")
	}
	definitionPath := strings.TrimSpace(os.Getenv("MULTICA_TEST_CODEX_CAPABILITY_DEFINITION"))
	if !filepath.IsAbs(definitionPath) {
		t.Fatal("MULTICA_TEST_CODEX_CAPABILITY_DEFINITION must be an explicit absolute path")
	}
	t3SpecPath := strings.TrimSpace(os.Getenv("MULTICA_TEST_CODEX_CAPABILITY_PROBE_SPEC"))
	taskID := os.Getenv("MULTICA_TEST_CODEX_CAPABILITY_TASK")
	if taskID == "" {
		taskID = "T3"
	}
	if taskID != "T3" && taskID != "T4" || taskID == "T4" && t3SpecPath == "" {
		t.Fatal("capability task must be T3 or T4 with a matching explicit spec")
	}
	if t3SpecPath != "" && !filepath.IsAbs(t3SpecPath) {
		t.Fatal("MULTICA_TEST_CODEX_CAPABILITY_PROBE_SPEC must be an explicit absolute path")
	}
	if os.Getenv("MULTICA_TEST_CODEX_CAPABILITY_ROOT") == "" {
		cache, err := os.UserCacheDir()
		if err != nil {
			t.Fatal(err)
		}
		root, err := os.MkdirTemp(cache, "multica-codex-capability-")
		if err != nil {
			t.Fatal(err)
		}
		root, err = filepath.EvalSymlinks(root)
		if err != nil {
			t.Fatal(err)
		}
		if root == "/tmp" || strings.HasPrefix(root, "/tmp/") || root == "/private/tmp" || strings.HasPrefix(root, "/private/tmp/") {
			t.Fatalf("capability fixture must not be under /tmp: %s", root)
		}
		for _, name := range []string{"home", "codex-home", "xdg", "tmp", "processes", "helpers"} {
			if err := os.MkdirAll(filepath.Join(root, name), 0o700); err != nil {
				t.Fatal(err)
			}
		}
		outerTimeout, testTimeout := 80*time.Second, "75s"
		if t3SpecPath != "" {
			outerTimeout, testTimeout = 160*time.Second, "155s"
		}
		ctx, cancel := context.WithTimeout(t.Context(), outerTimeout)
		cmd := exec.CommandContext(ctx, os.Args[0], "-test.run=^TestCodexCapabilityPairRealCLI$", "-test.v", "-test.timeout="+testTimeout)
		cmd.Env = []string{
			"PATH=" + os.Getenv("PATH"), "HOME=" + filepath.Join(root, "home"),
			"CODEX_HOME=" + filepath.Join(root, "codex-home"), "TMPDIR=" + filepath.Join(root, "tmp"),
			"XDG_CONFIG_HOME=" + filepath.Join(root, "xdg"), "XDG_DATA_HOME=" + filepath.Join(root, "xdg"),
			"XDG_STATE_HOME=" + filepath.Join(root, "xdg"), "XDG_CACHE_HOME=" + filepath.Join(root, "xdg"),
			"GIT_CONFIG_NOSYSTEM=1", "GIT_CONFIG_GLOBAL=" + os.DevNull, "OTEL_SDK_DISABLED=true",
			"MULTICA_TEST_REAL_CODEX_CAPABILITY=1", "MULTICA_TEST_REAL_CODEX_BIN=" + realCodex,
			"MULTICA_TEST_CODEX_CAPABILITY_DEFINITION=" + definitionPath,
			"MULTICA_TEST_CODEX_CAPABILITY_ROOT=" + root, "MULTICA_TEST_R12_ROOT=" + root,
		}
		if t3SpecPath != "" {
			cmd.Env = append(cmd.Env, "MULTICA_TEST_CODEX_CAPABILITY_PROBE_SPEC="+t3SpecPath)
			if taskID == "T4" {
				cmd.Env = append(cmd.Env, "MULTICA_TEST_CODEX_CAPABILITY_TASK=T4")
			}
		}
		cmd.SysProcAttr = &syscall.SysProcAttr{Setpgid: true}
		cmd.Cancel = func() error { return syscall.Kill(-cmd.Process.Pid, syscall.SIGKILL) }
		cmd.WaitDelay = 2 * time.Second
		outerGroupGone := false
		t.Cleanup(func() {
			cancel()
			recordedGroupsGone := pairCleanupRecordedGroups(filepath.Join(root, "processes"))
			if !outerGroupGone || !recordedGroupsGone {
				t.Errorf("retaining capability fixture because process ownership or exit could not be proved: %s", root)
				return
			}
			if !pairRemoveRootWhenGroupsGone(root, true) {
				t.Errorf("remove capability fixture: %s", root)
			}
		})
		output, err, outerGroupGone := pairRunOwnedCommand(cmd)
		if err != nil {
			if t3SpecPath != "" {
				pairRelayT3CapabilityReceipt(t, output, taskID)
			}
			t.Fatalf("isolated capability pair failed: %v (output_bytes=%d diagnostics=%q)", err, len(output), pairChildDiagnostics(output))
		}
		if t3SpecPath != "" {
			pairRelayT3CapabilityReceipt(t, output, taskID)
			return
		}
		for _, line := range strings.Split(string(output), "\n") {
			if strings.Contains(line, "CODEX_CAPABILITY_PAIR_RECEIPT ") {
				t.Log(strings.TrimSpace(line))
				return
			}
		}
		t.Fatal("isolated capability pair emitted no receipt")
	}

	root := os.Getenv("MULTICA_TEST_CODEX_CAPABILITY_ROOT")
	executable := readCodexCapabilityExecutable(t)
	resolved, err := filepath.EvalSymlinks(realCodex)
	if err != nil || resolved != executable.RealPath {
		t.Fatalf("opt-in binary does not match frozen provider_cli: match=%t err=%v", resolved == executable.RealPath, err)
	}
	file, err := os.Open(resolved)
	if err != nil {
		t.Fatal(err)
	}
	hash := sha256.New()
	_, copyErr := io.Copy(hash, file)
	closeErr := file.Close()
	actualHash := "sha256:" + fmt.Sprintf("%x", hash.Sum(nil))
	if copyErr != nil || closeErr != nil || actualHash != executable.SHA256 {
		t.Fatalf("frozen native binary hash mismatch: match=%t copy=%v close=%v", actualHash == executable.SHA256, copyErr, closeErr)
	}
	versionCtx, versionCancel := context.WithTimeout(t.Context(), 2*time.Second)
	versionOutput, versionErr := exec.CommandContext(versionCtx, realCodex, "--version").Output()
	versionCancel()
	if versionErr != nil || strings.TrimSpace(string(versionOutput)) != executable.Version {
		t.Fatalf("frozen native binary version mismatch: match=%t err=%v", strings.TrimSpace(string(versionOutput)) == executable.Version, versionErr)
	}
	if t3SpecPath != "" {
		runCodexT3CapabilityPair(t, root, realCodex, executable, t3SpecPath, taskID)
		return
	}

	const protectedMarker = "synthetic-capability-marker-7d91c4"
	workspaces := [2]pairWorkspace{}
	for arm, name := range []string{"direct", "daemon"} {
		base := filepath.Join(root, name)
		workspaces[arm] = pairWorkspace{cwd: filepath.Join(base, "repository"), outside: filepath.Join(base, "outside"), gocache: filepath.Join(base, "repository", ".gocache")}
		initializePairWorkspace(t, workspaces[arm].cwd, workspaces[arm].outside, protectedMarker)
	}

	goBin := mustPairLookPath(t, "go")
	gitBin := mustPairLookPath(t, "git")
	curlBin := mustPairLookPath(t, "curl")
	nodeBin := mustPairLookPath(t, "node")
	var mu sync.Mutex
	type armObservation struct {
		requests                    int
		toolAdvertised, toolOutput  bool
		toolStages                  string
		pid                         int
		birthObserved, execObserved bool
	}
	observed := [2]armObservation{}
	var unexpected []string
	var command string
	provider := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method == http.MethodGet && r.URL.Path == "/probe" {
			_, _ = io.WriteString(w, "loopback-http-ok")
			return
		}
		if r.Method != http.MethodPost || r.URL.Path != "/v1/responses" || r.URL.IsAbs() {
			mu.Lock()
			unexpected = append(unexpected, r.Method+" "+r.Host+r.URL.Path)
			mu.Unlock()
			http.Error(w, "unexpected fixture request", http.StatusNotFound)
			return
		}
		var request map[string]any
		if err := json.NewDecoder(io.LimitReader(r.Body, 2<<20)).Decode(&request); err != nil {
			t.Error("invalid synthetic Responses request")
			w.WriteHeader(http.StatusBadRequest)
			return
		}
		mu.Lock()
		total := observed[0].requests + observed[1].requests
		arm := 0
		if total >= 2 {
			arm = 1
		}
		observed[arm].requests++
		turn := observed[arm].requests
		if turn == 1 {
			observed[arm].toolAdvertised = pairToolAdvertised(request)
			role := []string{"direct", "daemon"}[arm]
			observed[arm].pid, observed[arm].birthObserved, observed[arm].execObserved = pairObserveNativeProcess(root, role, executable.RealPath, executable.SHA256)
		}
		if turn == 2 {
			observed[arm].toolOutput = pairToolOutputProves(request)
			observed[arm].toolStages = pairToolOutputStages(request)
		}
		mu.Unlock()
		w.Header().Set("Content-Type", "text/event-stream")
		if turn == 1 {
			pairWriteSSE(t, w, fmt.Sprintf("tool_%d", arm), map[string]any{"type": "function_call", "id": fmt.Sprintf("fc_%d", arm), "call_id": fmt.Sprintf("call_%d", arm), "name": "exec_command", "arguments": pairJSON(t, map[string]any{"cmd": command}), "status": "completed"})
			return
		}
		pairWriteSSE(t, w, fmt.Sprintf("final_%d", arm), map[string]any{"type": "message", "id": fmt.Sprintf("msg_%d", arm), "status": "completed", "role": "assistant", "content": []any{map[string]any{"type": "output_text", "text": "capability-pair-complete", "annotations": []any{}}}})
	}))
	defer provider.Close()

	q := pairShellQuote
	outsideProbe := `const fs=require("node:fs");try{fs.writeFileSync(process.argv[1],"escape\n");console.log("PAIR_OUTSIDE_SUCCEEDED");process.exit(31)}catch(error){if(error.code!=="EPERM"&&error.code!=="EACCES")process.exit(32);console.log("PAIR_OUTSIDE_DENIED:"+error.code)}`
	command = strings.Join([]string{
		"set -eu", "printf 'invoked\\n' >> tool-invocations.log", "test \"$(/bin/cat protected.txt)\" = " + q(protectedMarker),
		"echo PAIR_READ_OK",
		"printf 'workspace-write-ok\\n' > created.txt", "test -s created.txt", "/bin/rm created.txt", "test ! -e created.txt",
		"echo PAIR_WRITE_DELETE_OK",
		q(gitBin) + " status --porcelain >/dev/null",
		"echo PAIR_GIT_OK",
		"mkdir -p .gotmp", "TMPDIR=\"$PWD/.gotmp\" GOCACHE=\"$PAIR_GOCACHE\" " + q(goBin) + " test ./fixture >/dev/null",
		"echo PAIR_GO_OK",
		"test \"$(" + q(curlBin) + " -fsS " + q(provider.URL+"/probe") + ")\" = loopback-http-ok",
		"echo PAIR_HTTP_OK",
		q(nodeBin) + " -e " + q(outsideProbe) + " \"$PAIR_OUTSIDE/denied.txt\"",
		"printf 'CAPABILITY_OK read write delete git go http outside-denied\\n'",
	}, "; ")
	config := strings.Join([]string{
		`model = "syntheticloopback"`, `model_provider = "syntheticloopback"`, `approval_policy = "never"`, `sandbox_mode = "workspace-write"`,
		`disable_response_storage = true`, `check_for_update_on_startup = false`, `[analytics]`, `enabled = false`,
		`[sandbox_workspace_write]`, `network_access = true`, `exclude_tmpdir_env_var = true`, `exclude_slash_tmp = true`,
		`[model_providers.syntheticloopback]`, `name = "Synthetic loopback"`, `base_url = ` + strconv.Quote(provider.URL+"/v1"), `wire_api = "responses"`, `requires_openai_auth = false`,
		`[features]`, `responses_websockets = false`, `multi_agent = false`, `memories = false`, `plugins = false`, "",
	}, "\n")
	writePairFile(t, filepath.Join(os.Getenv("CODEX_HOME"), "config.toml"), config)
	policyArgs := []string{"-c", `approval_policy="never"`, "-c", `sandbox_mode="workspace-write"`, "-c", `sandbox_workspace_write.network_access=true`, "-c", `sandbox_workspace_write.exclude_tmpdir_env_var=true`, "-c", `sandbox_workspace_write.exclude_slash_tmp=true`, "-c", `analytics.enabled=false`, "-c", `check_for_update_on_startup=false`}
	wrapper := writeCapabilityWrapper(t, root, realCodex, provider.URL, workspaces)

	directCtx, directCancel := context.WithTimeout(t.Context(), 25*time.Second)
	directArgs := append([]string{"exec"}, policyArgs...)
	directArgs = append(directArgs, "--json", "--ephemeral", "--skip-git-repo-check", "-C", workspaces[0].cwd, "Execute the fixture-provided command exactly once, then finish.")
	direct := exec.CommandContext(directCtx, wrapper, directArgs...)
	direct.SysProcAttr = &syscall.SysProcAttr{Setpgid: true}
	direct.Cancel = func() error { return syscall.Kill(-direct.Process.Pid, syscall.SIGKILL) }
	direct.WaitDelay = 2 * time.Second
	directOutput, directErr, directGroupGone := pairRunOwnedCommand(direct)
	directCancel()
	if directErr != nil || !directGroupGone {
		t.Fatalf("direct native Codex arm failed: %v (output_bytes=%d group_gone=%t)", directErr, len(directOutput), directGroupGone)
	}

	control := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method == http.MethodPost && strings.HasPrefix(r.URL.Path, "/api/daemon/tasks/") {
			_, _ = io.Copy(io.Discard, io.LimitReader(r.Body, 2<<20))
			w.WriteHeader(http.StatusOK)
			return
		}
		http.Error(w, "unexpected control request", http.StatusNotFound)
	}))
	defer control.Close()
	logger := slog.New(slog.NewTextHandler(io.Discard, nil))
	cfg := Config{DaemonID: "capability-daemon", Profile: "capability-private", WorkspacesRoot: filepath.Join(root, "workspaces"), ServerBaseURL: control.URL, Agents: map[string]AgentEntry{"codex": {Path: wrapper}}, AgentTimeout: 20 * time.Second, CodexHandshakeTimeout: 5 * time.Second, CodexThreadHandshakeTimeout: 5 * time.Second, DirectAgentMode: true}
	d := New(cfg, logger)
	d.client.SetToken("synthetic-control-token")
	d.setAgentVersion("codex", strings.TrimPrefix(executable.Version, "codex-cli "))
	d.executionEnvironmentCommand = func() ([]string, error) {
		return []string{os.Args[0], "-test.run=^TestCodexR12PreparationHelper$", "--", "r12-preparation-helper"}, nil
	}
	ref, err := json.Marshal(localDirectoryRef{LocalPath: workspaces[1].cwd, DaemonID: cfg.DaemonID})
	if err != nil {
		t.Fatal(err)
	}
	task := Task{ID: "44444444-4444-4444-8444-444444444444", WorkspaceID: "capability-workspace", IssueID: "capability-issue", AgentID: "capability-agent", RuntimeID: "capability-runtime", AuthToken: "mat_synthetic_capability_token", ConciseMode: true, Agent: &AgentData{ID: "capability-agent", Name: "Capability fixture", CustomArgs: policyArgs, McpConfig: json.RawMessage(`{"mcpServers":{}}`)}, ProjectResources: []ProjectResourceData{{ResourceType: "local_directory", ResourceRef: ref}}}
	daemonCtx, daemonCancel := context.WithTimeout(t.Context(), 30*time.Second)
	result, runErr := d.runTask(daemonCtx, task, "codex", 0, logger)
	daemonCancel()
	if runErr != nil || result.Status != "completed" || result.Comment != "capability-pair-complete" {
		t.Fatalf("daemon runTask arm failed: status=%q error=%v", result.Status, runErr)
	}
	for _, workspace := range workspaces {
		invocations, err := os.ReadFile(filepath.Join(workspace.cwd, "tool-invocations.log"))
		if err != nil || string(invocations) != "invoked\n" {
			t.Fatalf("tool invocation journal must contain exactly one entry: bytes=%d err=%v", len(invocations), err)
		}
		if _, err := os.Stat(filepath.Join(workspace.outside, "denied.txt")); !errors.Is(err, os.ErrNotExist) {
			t.Fatalf("outside write was not denied: %v", err)
		}
		if _, err := os.Stat(filepath.Join(workspace.cwd, "created.txt")); !errors.Is(err, os.ErrNotExist) {
			t.Fatalf("workspace delete did not persist: %v", err)
		}
	}
	mu.Lock()
	got, gotUnexpected := observed, unexpected
	mu.Unlock()
	for arm, name := range []string{"direct", "daemon"} {
		if got[arm].requests != 2 || !got[arm].toolAdvertised || !got[arm].toolOutput || got[arm].pid <= 1 || !got[arm].birthObserved || !got[arm].execObserved {
			t.Fatalf("%s protocol proof incomplete: %+v", name, got[arm])
		}
	}
	if len(gotUnexpected) != 0 {
		t.Fatalf("unexpected provider/proxy requests: %v", gotUnexpected)
	}
	processesDir := filepath.Join(root, "processes")
	if !pairCleanupRecordedGroups(processesDir) {
		t.Fatal("not all recorded native process groups could be proved owned and reaped")
	}
	markers, err := os.ReadDir(processesDir)
	if err != nil || len(markers) != 2 {
		t.Fatalf("native execution count=%d, want exactly two: %v", len(markers), err)
	}
	for _, marker := range markers {
		pid, parseErr := strconv.Atoi(marker.Name())
		if parseErr != nil || pid <= 1 || !codexR12WaitGroupGone(pid, 2*time.Second) {
			t.Fatalf("native process was not reaped: %s", marker.Name())
		}
	}
	receipt := map[string]any{
		"verdict": "bounded_capability_pair", "arms": 2,
		"native":         map[string]any{"version": executable.Version, "sha256": executable.SHA256, "processes_per_arm": 1, "exec_identity": true, "reaped": true},
		"protocol":       map[string]any{"responses_per_arm": 2, "tool_calls_per_arm": 1, "tool": "exec_command", "exit_code": 0},
		"checks":         []string{"workspace_read", "workspace_write_delete", "git_status", "go_fixture_test", "localhost_http", "outside_write_denied_EPERM_or_EACCES", "owner_writable_control", "independent_identical_fixtures"},
		"policy":         map[string]any{"approval": "never", "sandbox": "workspace-write", "network_access": true},
		"provider":       map[string]any{"id": "syntheticloopback", "requires_auth": false, "external_model": false},
		"treatment_mode": "daemon_direct_agent_mode", "wrapper_env_scrub": true, "proxy_only_traffic_observation": true,
		"scope": "direct_exec_vs_daemon_runTask_only", "parity_claim": false,
	}
	encoded, err := json.Marshal(receipt)
	if err != nil {
		t.Fatal(err)
	}
	t.Logf("CODEX_CAPABILITY_PAIR_RECEIPT %s", encoded)
}

func readCodexCapabilityExecutable(t *testing.T) codexCapabilityExecutable {
	t.Helper()
	data, err := os.ReadFile(os.Getenv("MULTICA_TEST_CODEX_CAPABILITY_DEFINITION"))
	if err != nil {
		t.Fatal(err)
	}
	var definition struct {
		Frozen struct {
			Runtime struct {
				Executables []codexCapabilityExecutable `json:"executables"`
			} `json:"runtime"`
		} `json:"frozen"`
	}
	if err := json.Unmarshal(data, &definition); err != nil {
		t.Fatal(err)
	}
	for _, executable := range definition.Frozen.Runtime.Executables {
		if executable.Role == "provider_cli" {
			return executable
		}
	}
	t.Fatal("frozen harness has no provider_cli executable")
	return codexCapabilityExecutable{}
}

func writePairFile(t *testing.T, path, content string) {
	t.Helper()
	if err := os.MkdirAll(filepath.Dir(path), 0o700); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, []byte(content), 0o600); err != nil {
		t.Fatal(err)
	}
}

func initializePairWorkspace(t *testing.T, cwd, outside, protectedMarker string) {
	t.Helper()
	writePairFile(t, filepath.Join(cwd, "go.mod"), "module capability.fixture\n\ngo 1.26\n")
	writePairFile(t, filepath.Join(cwd, "fixture", "fixture_test.go"), "package fixture\nimport \"testing\"\nfunc TestFixture(t *testing.T) {}\n")
	writePairFile(t, filepath.Join(cwd, "protected.txt"), protectedMarker+"\n")
	if err := os.MkdirAll(outside, 0o700); err != nil {
		t.Fatal(err)
	}
	git := exec.CommandContext(t.Context(), "git", "init", "--quiet", "--template=", cwd)
	git.Env = []string{"PATH=" + os.Getenv("PATH"), "HOME=" + os.Getenv("HOME"), "GIT_CONFIG_NOSYSTEM=1", "GIT_CONFIG_GLOBAL=" + os.DevNull}
	if output, err := git.CombinedOutput(); err != nil {
		t.Fatalf("initialize fixture repository: %v (output_bytes=%d)", err, len(output))
	}
	ownerProbe := filepath.Join(outside, "owner-positive-control")
	writePairFile(t, ownerProbe, "owner-writable\n")
	if err := os.Remove(ownerProbe); err != nil {
		t.Fatal(err)
	}
}

func mustPairLookPath(t *testing.T, name string) string {
	t.Helper()
	path, err := exec.LookPath(name)
	if err != nil {
		t.Fatalf("required fixture command %s unavailable", name)
	}
	return path
}
func pairShellQuote(value string) string { return "'" + strings.ReplaceAll(value, "'", "'\\''") + "'" }
func pairJSON(t *testing.T, value any) string {
	t.Helper()
	data, err := json.Marshal(value)
	if err != nil {
		t.Fatal(err)
	}
	return string(data)
}

func pairChildDiagnostics(output []byte) []string {
	var diagnostics []string
	for _, line := range strings.Split(string(output), "\n") {
		if strings.Contains(line, ".go:") || strings.Contains(line, "FAIL") {
			diagnostics = append(diagnostics, strings.TrimSpace(line))
		}
	}
	return diagnostics
}

type pairOwnedGroup struct {
	pid   int
	birth string
}

func pairRunOwnedCommand(cmd *exec.Cmd) ([]byte, error, bool) {
	var output bytes.Buffer
	cmd.Stdout, cmd.Stderr = &output, &output
	if cmd.WaitDelay == 0 {
		cmd.WaitDelay = 500 * time.Millisecond
	}
	if err := cmd.Start(); err != nil {
		return output.Bytes(), err, false
	}
	owned, ok := pairCaptureOwnedGroup(cmd.Process.Pid)
	if !ok {
		if group, err := syscall.Getpgid(cmd.Process.Pid); err == nil && group == cmd.Process.Pid {
			_ = syscall.Kill(-group, syscall.SIGKILL)
		} else {
			_ = cmd.Process.Kill()
		}
		err := cmd.Wait()
		return output.Bytes(), err, false
	}
	err := cmd.Wait()
	return output.Bytes(), err, pairStopOwnedGroup(owned, 2*time.Second)
}

func pairCaptureOwnedGroup(pid int) (pairOwnedGroup, bool) {
	group, err := syscall.Getpgid(pid)
	if err != nil || group != pid {
		return pairOwnedGroup{}, false
	}
	birth, err := pairProcessBirth(pid)
	return pairOwnedGroup{pid: pid, birth: birth}, err == nil
}

func pairProcessBirth(pid int) (string, error) {
	ctx, cancel := context.WithTimeout(context.Background(), time.Second)
	defer cancel()
	output, err := exec.CommandContext(ctx, "/bin/ps", "-p", strconv.Itoa(pid), "-o", "lstart=").Output()
	return strings.Join(strings.Fields(string(output)), " "), err
}

func pairStopOwnedGroup(owned pairOwnedGroup, timeout time.Duration) bool {
	if owned.pid <= 1 || owned.birth == "" {
		return false
	}
	if errors.Is(syscall.Kill(-owned.pid, 0), syscall.ESRCH) {
		return true
	}
	if currentBirth, err := pairProcessBirth(owned.pid); err == nil {
		if currentBirth != owned.birth {
			return false
		}
	} else if leaderErr := syscall.Kill(owned.pid, 0); leaderErr == nil || errors.Is(leaderErr, syscall.EPERM) {
		return false
	}
	if err := syscall.Kill(-owned.pid, syscall.SIGKILL); err != nil && !errors.Is(err, syscall.ESRCH) {
		return false
	}
	deadline := time.Now().Add(timeout)
	for time.Now().Before(deadline) {
		if errors.Is(syscall.Kill(-owned.pid, 0), syscall.ESRCH) {
			return true
		}
		time.Sleep(10 * time.Millisecond)
	}
	return false
}

func pairCleanupRecordedGroups(directory string) bool {
	markers, err := os.ReadDir(directory)
	if err != nil {
		return false
	}
	allGone := true
	for _, marker := range markers {
		data, readErr := os.ReadFile(filepath.Join(directory, marker.Name()))
		parts := strings.Split(strings.TrimSpace(string(data)), "\n")
		pid, parseErr := strconv.Atoi(marker.Name())
		if readErr != nil || parseErr != nil || pid <= 1 || len(parts) < 2 || parts[0] != marker.Name() {
			allGone = false
			continue
		}
		if !pairStopOwnedGroup(pairOwnedGroup{pid: pid, birth: strings.Join(strings.Fields(parts[1]), " ")}, 2*time.Second) {
			allGone = false
		}
	}
	return allGone
}

func pairRemoveRootWhenGroupsGone(root string, groupsGone bool) bool {
	return groupsGone && os.RemoveAll(root) == nil
}

func writeCapabilityWrapper(t *testing.T, root, realCodex, providerURL string, workspaces [2]pairWorkspace) string {
	t.Helper()
	q := pairShellQuote
	wrapper := filepath.Join(root, "codex-capability-isolated")
	body := "#!/bin/sh\nset -eu\ncase \"${1:-}\" in\n" +
		"exec) role=direct; pair_outside=" + q(workspaces[0].outside) + "; pair_gocache=" + q(workspaces[0].gocache) + ";;\n" +
		"app-server) role=daemon; pair_outside=" + q(workspaces[1].outside) + "; pair_gocache=" + q(workspaces[1].gocache) + ";;\n" +
		"*) role=other; pair_outside=" + q(workspaces[0].outside) + "; pair_gocache=" + q(workspaces[0].gocache) + ";; esac\n" +
		"if [ \"$role\" != other ]; then start=$(/bin/ps -p \"$$\" -o lstart=); printf '%s\\n%s\\n%s\\n' \"$$\" \"$start\" \"$role\" > " + q(filepath.Join(root, "processes")) + "/$$; fi\n" +
		"exec env -i PATH=" + q(os.Getenv("PATH")) + " HOME=\"$HOME\" CODEX_HOME=\"$CODEX_HOME\" TMPDIR=\"$TMPDIR\" XDG_CONFIG_HOME=\"${XDG_CONFIG_HOME:-}\" XDG_DATA_HOME=\"${XDG_DATA_HOME:-}\" XDG_STATE_HOME=\"${XDG_STATE_HOME:-}\" XDG_CACHE_HOME=\"${XDG_CACHE_HOME:-}\" PAIR_OUTSIDE=\"$pair_outside\" PAIR_GOCACHE=\"$pair_gocache\" GIT_CONFIG_NOSYSTEM=1 GIT_CONFIG_GLOBAL=" + q(os.DevNull) + " HTTP_PROXY=" + q(providerURL) + " HTTPS_PROXY=" + q(providerURL) + " ALL_PROXY=" + q(providerURL) + " NO_PROXY='127.0.0.1,localhost,::1' OTEL_SDK_DISABLED=true " + q(realCodex) + " \"$@\"\n"
	writePairFile(t, wrapper, body)
	if err := os.Chmod(wrapper, 0o700); err != nil {
		t.Fatal(err)
	}
	return wrapper
}

func pairObserveNativeProcess(root, role, executablePath, executableHash string) (int, bool, bool) {
	markers, err := os.ReadDir(filepath.Join(root, "processes"))
	if err != nil {
		return 0, false, false
	}
	for _, marker := range markers {
		data, readErr := os.ReadFile(filepath.Join(root, "processes", marker.Name()))
		parts := strings.Split(strings.TrimSpace(string(data)), "\n")
		if readErr != nil || len(parts) != 3 || parts[2] != role {
			continue
		}
		pid, parseErr := strconv.Atoi(parts[0])
		if parseErr != nil || pid <= 1 || marker.Name() != parts[0] {
			return 0, false, false
		}
		ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
		birth, birthErr := exec.CommandContext(ctx, "/bin/ps", "-p", parts[0], "-o", "lstart=").Output()
		lsof, lookupErr := exec.LookPath("lsof")
		var image []byte
		var imageErr error
		if lookupErr == nil {
			image, imageErr = exec.CommandContext(ctx, lsof, "-a", "-p", parts[0], "-d", "txt", "-Fn").Output()
		}
		cancel()
		birthMatches := birthErr == nil && strings.TrimSpace(string(birth)) == strings.TrimSpace(parts[1])
		imageMatches := false
		if imageErr == nil {
			for _, line := range strings.Split(string(image), "\n") {
				if !strings.HasPrefix(line, "n") {
					continue
				}
				resolved, resolveErr := filepath.EvalSymlinks(strings.TrimPrefix(line, "n"))
				if resolveErr == nil && resolved == executablePath && pairFileSHA256(resolved) == executableHash {
					imageMatches = true
				}
			}
		}
		return pid, birthMatches, imageMatches
	}
	return 0, false, false
}

func pairFileSHA256(path string) string {
	file, err := os.Open(path)
	if err != nil {
		return ""
	}
	defer file.Close()
	hash := sha256.New()
	if _, err := io.Copy(hash, file); err != nil {
		return ""
	}
	return "sha256:" + fmt.Sprintf("%x", hash.Sum(nil))
}

func pairToolAdvertised(request map[string]any) bool {
	tools, _ := request["tools"].([]any)
	for _, raw := range tools {
		tool, _ := raw.(map[string]any)
		if tool["name"] == "exec_command" {
			return true
		}
	}
	return false
}

func pairToolOutputProves(request map[string]any) bool {
	input, _ := request["input"].([]any)
	for _, raw := range input {
		item, _ := raw.(map[string]any)
		if item["type"] == "function_call_output" {
			output := fmt.Sprint(item["output"])
			exitZero := strings.Contains(output, "Process exited with code 0") || strings.Contains(output, `"exit_code":0`)
			outsideDenied := strings.Contains(output, "PAIR_OUTSIDE_DENIED:EPERM") || strings.Contains(output, "PAIR_OUTSIDE_DENIED:EACCES")
			if exitZero && outsideDenied && !strings.Contains(output, "PAIR_OUTSIDE_SUCCEEDED") && strings.Contains(output, "CAPABILITY_OK read write delete git go http outside-denied") {
				return true
			}
		}
	}
	return false
}

func pairToolOutputStages(request map[string]any) string {
	input, _ := request["input"].([]any)
	var outputs strings.Builder
	for _, raw := range input {
		item, _ := raw.(map[string]any)
		if item["type"] == "function_call_output" {
			_, _ = fmt.Fprint(&outputs, item["output"])
		}
	}
	text := outputs.String()
	var stages []string
	for _, stage := range []string{"READ", "WRITE_DELETE", "GIT", "GO", "HTTP"} {
		if strings.Contains(text, "PAIR_"+stage+"_OK") {
			stages = append(stages, stage)
		}
	}
	if strings.Contains(text, "PAIR_OUTSIDE_DENIED") {
		stages = append(stages, "OUTSIDE_DENIED")
	}
	return strings.Join(stages, ",")
}

func pairWriteSSE(t *testing.T, w io.Writer, id string, item map[string]any) {
	t.Helper()
	envelope := func(status string, output []any, usage any) map[string]any {
		return map[string]any{"id": "resp_" + id, "object": "response", "created_at": 0, "status": status, "model": "syntheticloopback", "output": output, "error": nil, "incomplete_details": nil, "instructions": nil, "parallel_tool_calls": false, "previous_response_id": nil, "store": false, "tools": []any{}, "usage": usage}
	}
	events := []map[string]any{{"type": "response.created", "sequence_number": 0, "response": envelope("in_progress", []any{}, nil)}, {"type": "response.output_item.done", "sequence_number": 1, "output_index": 0, "item": item}, {"type": "response.completed", "sequence_number": 2, "response": envelope("completed", []any{item}, map[string]any{"input_tokens": 1, "output_tokens": 1, "total_tokens": 2, "input_tokens_details": map[string]int{"cached_tokens": 0}, "output_tokens_details": map[string]int{"reasoning_tokens": 0}})}}
	for _, event := range events {
		data, err := json.Marshal(event)
		if err != nil {
			t.Error(err)
			return
		}
		if _, err := fmt.Fprintf(w, "event: %s\ndata: %s\n\n", event["type"], data); err != nil {
			t.Error(err)
			return
		}
	}
	_, _ = io.WriteString(w, "data: [DONE]\n\n")
}
