//go:build !windows

package daemon

// Diagnostic plan:
//  1. Re-exec this test with a scrubbed private HOME/CODEX_HOME and a pinned Codex 0.153.4 binary.
//  2. Send one native `codex exec` turn and one ordinary daemon.runTask turn to the same no-auth loopback provider.
//  3. Summarize only the first actual /responses body from each arm as byte counts, digests, key names, roles, and tool counts.
//  4. Emit a receipt only after native process-group and temporary-root cleanup are proved by the outer process.

import (
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
	"sort"
	"strings"
	"sync"
	"syscall"
	"testing"
	"time"

	"github.com/multica-ai/multica/server/internal/daemon/execenv"
	"github.com/multica-ai/multica/server/internal/daemon/repocache"
)

const (
	codexRequestEnvelopeBinarySHA256 = "b973d440acac501fd2594a43e7ca9ce41e0a65b9dfb28d0d7a7837c99e1261e3"
	codexRequestEnvelopeModel        = "synthetic-request-envelope-model"
	codexRequestEnvelopePrompt       = "Inspect the synthetic repository metadata and return the exact text request-envelope-complete."
	codexRequestEnvelopeFinal        = "request-envelope-complete"
	codexRequestEnvelopeRepoRule     = "Use only repository-local context."
	codexRequestEnvelopeMaxBody      = 4 << 20
	codexRequestEnvelopeReceiptMark  = "CODEX_REQUEST_ENVELOPE_INNER_RECEIPT "
)

type codexRequestEnvelopePart struct {
	Bytes  int    `json:"bytes"`
	SHA256 string `json:"sha256"`
}

type codexRequestEnvelopeRole struct {
	Role  string `json:"role"`
	Items int    `json:"items"`
	codexRequestEnvelopePart
}

type codexRequestEnvelopeInput struct {
	codexRequestEnvelopePart
	Roles     []codexRequestEnvelopeRole     `json:"roles"`
	Inventory *codexRequestEnvelopeInventory `json:"inventory,omitempty"`
}

type codexRequestEnvelopeTools struct {
	codexRequestEnvelopePart
	Count     int                            `json:"count"`
	Inventory *codexRequestEnvelopeInventory `json:"inventory,omitempty"`
}

type codexRequestEnvelopeObservation struct {
	Arm                          string                    `json:"arm"`
	Body                         codexRequestEnvelopePart  `json:"body"`
	KeyNames                     []string                  `json:"key_names"`
	Model                        string                    `json:"model"`
	Instructions                 codexRequestEnvelopePart  `json:"instructions"`
	Input                        codexRequestEnvelopeInput `json:"input"`
	Tools                        codexRequestEnvelopeTools `json:"tools"`
	PromptOccurrences            int                       `json:"prompt_occurrences"`
	AutomaticDeliveryOccurrences int                       `json:"automatic_delivery_occurrences"`
	RepositoryRuleOccurrences    *int                      `json:"repository_rule_occurrences,omitempty"`
	NativeIdentityObserved       bool                      `json:"native_identity_observed"`
}

type codexRequestEnvelopeBriefDelta struct {
	WithoutCompletion codexRequestEnvelopePart `json:"without_completion"`
	WithCompletion    codexRequestEnvelopePart `json:"with_completion"`
	DeltaBytes        int                      `json:"delta_bytes"`
}

type codexRequestEnvelopeReceipt struct {
	SchemaVersion string                            `json:"schema_version"`
	Status        string                            `json:"status"`
	Scope         string                            `json:"scope"`
	OrdinaryMode  bool                              `json:"ordinary_mode"`
	Runtime       map[string]any                    `json:"runtime"`
	Fixture       map[string]any                    `json:"fixture"`
	Provider      map[string]any                    `json:"provider"`
	BriefDelta    codexRequestEnvelopeBriefDelta    `json:"brief_delta"`
	Arms          []codexRequestEnvelopeObservation `json:"arms"`
	Comparison    map[string]int                    `json:"comparison"`
	Cleanup       map[string]any                    `json:"cleanup,omitempty"`
	Limitations   []string                          `json:"limitations"`
}

func TestCodexRequestEnvelopeRealCLIReportsOrdinaryRunTaskContribution(t *testing.T) {
	if os.Getenv("MULTICA_TEST_REAL_CODEX_REQUEST_ENVELOPE") != "1" {
		t.Skip("set MULTICA_TEST_REAL_CODEX_REQUEST_ENVELOPE=1 and MULTICA_TEST_REAL_CODEX_BIN to run the isolated no-auth request-envelope diagnostic")
	}
	realCodex := strings.TrimSpace(os.Getenv("MULTICA_TEST_REAL_CODEX_BIN"))
	if !filepath.IsAbs(realCodex) {
		t.Fatal("MULTICA_TEST_REAL_CODEX_BIN must be an explicit absolute path")
	}
	if os.Getenv("MULTICA_TEST_CODEX_REQUEST_ENVELOPE_ROOT") == "" {
		runCodexRequestEnvelopeOuter(t, realCodex)
		return
	}
	runCodexRequestEnvelopeInner(t, realCodex, os.Getenv("MULTICA_TEST_CODEX_REQUEST_ENVELOPE_ROOT"))
}

func runCodexRequestEnvelopeOuter(t *testing.T, realCodex string) {
	t.Helper()
	if _, err := codexRequestEnvelopeNativeSessionArgs(os.Getenv("MULTICA_TEST_CODEX_REQUEST_NATIVE_SESSION")); err != nil {
		t.Fatal(err)
	}
	cache, err := os.UserCacheDir()
	if err != nil {
		t.Fatal(err)
	}
	root, err := os.MkdirTemp(cache, "multica-codex-request-envelope-")
	if err != nil {
		t.Fatal(err)
	}
	root, err = filepath.EvalSymlinks(root)
	if err != nil {
		t.Fatal(err)
	}
	for _, name := range []string{"home", "codex-home", "tmp", "xdg", "processes"} {
		if err := os.MkdirAll(filepath.Join(root, name), 0o700); err != nil {
			t.Fatal(err)
		}
	}
	removed := false
	t.Cleanup(func() {
		if !removed {
			_ = pairCleanupRecordedGroups(filepath.Join(root, "processes"))
			_ = os.RemoveAll(root)
		}
	})

	ctx, cancel := context.WithTimeout(t.Context(), 90*time.Second)
	defer cancel()
	cmd := exec.CommandContext(ctx, os.Args[0], "-test.run=^TestCodexRequestEnvelopeRealCLIReportsOrdinaryRunTaskContribution$", "-test.v", "-test.timeout=85s")
	cmd.Env = []string{
		"PATH=/usr/bin:/bin:/usr/sbin:/sbin",
		"HOME=" + filepath.Join(root, "home"),
		"CODEX_HOME=" + filepath.Join(root, "codex-home"),
		"TMPDIR=" + filepath.Join(root, "tmp"),
		"XDG_CONFIG_HOME=" + filepath.Join(root, "xdg"),
		"XDG_DATA_HOME=" + filepath.Join(root, "xdg"),
		"XDG_STATE_HOME=" + filepath.Join(root, "xdg"),
		"XDG_CACHE_HOME=" + filepath.Join(root, "xdg"),
		"GIT_CONFIG_NOSYSTEM=1",
		"GIT_CONFIG_GLOBAL=" + os.DevNull,
		"OTEL_SDK_DISABLED=true",
		"MULTICA_TEST_REAL_CODEX_REQUEST_ENVELOPE=1",
		"MULTICA_TEST_REAL_CODEX_BIN=" + realCodex,
		"MULTICA_TEST_CODEX_REQUEST_PROJECT_POLICY=" + os.Getenv("MULTICA_TEST_CODEX_REQUEST_PROJECT_POLICY"),
		"MULTICA_TEST_CODEX_REQUEST_NATIVE_SESSION=" + os.Getenv("MULTICA_TEST_CODEX_REQUEST_NATIVE_SESSION"),
		"MULTICA_TEST_CODEX_REQUEST_ENVELOPE_ROOT=" + root,
		"MULTICA_TEST_R12_ROOT=" + root,
	}
	cmd.SysProcAttr = &syscall.SysProcAttr{Setpgid: true}
	cmd.Cancel = func() error { return syscall.Kill(-cmd.Process.Pid, syscall.SIGKILL) }
	cmd.WaitDelay = 2 * time.Second
	output, runErr, outerGroupGone := pairRunOwnedCommand(cmd)
	if runErr != nil {
		t.Fatalf("isolated request-envelope diagnostic failed: %v (output_bytes=%d diagnostics=%q)", runErr, len(output), pairChildDiagnostics(output))
	}
	receipt, err := parseCodexRequestEnvelopeReceipt(output)
	if err != nil {
		t.Fatal(err)
	}
	recordedGroupsGone := pairCleanupRecordedGroups(filepath.Join(root, "processes"))
	entries, readErr := os.ReadDir(filepath.Join(root, "processes"))
	if readErr != nil || len(entries) != 2 {
		t.Fatalf("native execution markers=%d, want 2: %v", len(entries), readErr)
	}
	removed = pairRemoveRootWhenGroupsGone(root, outerGroupGone && recordedGroupsGone)
	if !removed {
		t.Fatalf("request-envelope cleanup was not proved for %s", root)
	}
	receipt.Cleanup = map[string]any{
		"outer_process_group_gone":   outerGroupGone,
		"native_process_groups_gone": recordedGroupsGone,
		"native_process_count":       len(entries),
		"temporary_root_removed":     true,
	}
	encoded, err := json.Marshal(receipt)
	if err != nil {
		t.Fatal(err)
	}
	t.Logf("CODEX_REQUEST_ENVELOPE_RECEIPT %s", encoded)
}

func runCodexRequestEnvelopeInner(t *testing.T, realCodex, root string) {
	t.Helper()
	sessionArgs, err := codexRequestEnvelopeNativeSessionArgs(os.Getenv("MULTICA_TEST_CODEX_REQUEST_NATIVE_SESSION"))
	if err != nil {
		t.Fatal(err)
	}
	nativeSessionMode := "persistent"
	if len(sessionArgs) != 0 {
		nativeSessionMode = "ephemeral"
	}
	projectPolicy := os.Getenv("MULTICA_TEST_CODEX_REQUEST_PROJECT_POLICY")
	if projectPolicy == "" {
		projectPolicy = "restricted"
	}
	if projectPolicy != "restricted" && projectPolicy != "trusted" {
		t.Fatal("request-envelope diagnostic project policy must be restricted or trusted")
	}
	resolved, err := filepath.EvalSymlinks(realCodex)
	if err != nil {
		t.Fatal(err)
	}
	binaryDigest := pairFileSHA256(resolved)
	if binaryDigest != "sha256:"+codexRequestEnvelopeBinarySHA256 {
		t.Fatal("native Codex executable differs from the frozen 0.153.4 binary")
	}
	versionCtx, versionCancel := context.WithTimeout(t.Context(), 2*time.Second)
	versionOutput, versionErr := exec.CommandContext(versionCtx, resolved, "--version").Output()
	versionCancel()
	if versionErr != nil || strings.TrimSpace(string(versionOutput)) != "codex-cli 0.153.4" {
		t.Fatalf("frozen native Codex version mismatch: match=%t err=%v", strings.TrimSpace(string(versionOutput)) == "codex-cli 0.153.4", versionErr)
	}

	source := filepath.Join(root, "synthetic-source")
	writePairFile(t, filepath.Join(source, "AGENTS.md"), "# Synthetic request-envelope fixture\n"+codexRequestEnvelopeRepoRule+"\n")
	writePairFile(t, filepath.Join(source, "README.md"), "Synthetic request-envelope fixture.\n")
	runGitForGC(t, source, "init", "-b", "main")
	runGitForGC(t, source, "add", "AGENTS.md", "README.md")
	runGitForGC(t, source, "commit", "-m", "synthetic request envelope fixture")
	sourceCommit := runGitForGC(t, source, "rev-parse", "HEAD")
	sourceTree := runGitForGC(t, source, "rev-parse", "HEAD^{tree}")

	var mu sync.Mutex
	observations := make([]codexRequestEnvelopeObservation, 0, 2)
	var unexpected []string
	provider := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost || r.URL.Path != "/v1/responses" || r.URL.IsAbs() {
			mu.Lock()
			unexpected = append(unexpected, r.Method+" "+r.URL.String())
			mu.Unlock()
			http.Error(w, "unexpected synthetic provider request", http.StatusNotFound)
			return
		}
		if r.Header.Get("Authorization") != "" || r.Header.Get("X-Api-Key") != "" {
			t.Error("no-auth provider received credential headers")
			http.Error(w, "credential header forbidden", http.StatusBadRequest)
			return
		}
		body, readErr := io.ReadAll(io.LimitReader(r.Body, codexRequestEnvelopeMaxBody+1))
		if readErr != nil || len(body) > codexRequestEnvelopeMaxBody {
			t.Error("bounded provider request body read failed")
			http.Error(w, "request body invalid", http.StatusBadRequest)
			return
		}
		var request map[string]any
		if err := json.Unmarshal(body, &request); err != nil {
			t.Error("provider request was not structured JSON")
			http.Error(w, "request JSON invalid", http.StatusBadRequest)
			return
		}
		if request["model"] != codexRequestEnvelopeModel {
			t.Error("provider request did not use the synthetic configured model")
			http.Error(w, "model mismatch", http.StatusBadRequest)
			return
		}
		mu.Lock()
		index := len(observations)
		if index >= 2 {
			unexpected = append(unexpected, "extra /v1/responses request")
			mu.Unlock()
			http.Error(w, "extra model turn", http.StatusBadRequest)
			return
		}
		arm := []string{"native_exec", "ordinary_daemon_runTask"}[index]
		observation := summarizeCodexRequestEnvelope(t, arm, body, request)
		pid, birthObserved, executableObserved := pairObserveNativeProcess(root, []string{"direct", "daemon"}[index], resolved, binaryDigest)
		observation.NativeIdentityObserved = pid > 1 && birthObserved && executableObserved
		observations = append(observations, observation)
		mu.Unlock()
		w.Header().Set("Content-Type", "text/event-stream")
		pairWriteSSE(t, w, fmt.Sprintf("request_envelope_%d", index), map[string]any{
			"type": "message", "id": fmt.Sprintf("msg_request_envelope_%d", index), "status": "completed", "role": "assistant",
			"content": []any{map[string]any{"type": "output_text", "text": codexRequestEnvelopeFinal, "annotations": []any{}}},
		})
	}))
	defer provider.Close()

	const loopbackPlaceholder = "__REQUEST_ENVELOPE_LOOPBACK__"
	configTemplate := strings.Join([]string{
		`model = "` + codexRequestEnvelopeModel + `"`,
		`model_provider = "synthetic_request_envelope"`,
		`approval_policy = "never"`,
		`sandbox_mode = "workspace-write"`,
		`disable_response_storage = true`,
		`check_for_update_on_startup = false`,
		`[analytics]`,
		`enabled = false`,
		`[sandbox_workspace_write]`,
		`network_access = true`,
		`exclude_tmpdir_env_var = true`,
		`exclude_slash_tmp = true`,
		`[model_providers.synthetic_request_envelope]`,
		`name = "Synthetic request envelope"`,
		`base_url = "` + loopbackPlaceholder + `"`,
		`wire_api = "responses"`,
		`requires_openai_auth = false`,
		`[features]`,
		`responses_websockets = false`,
		`multi_agent = false`,
		`memories = false`,
		`plugins = false`,
		"",
	}, "\n")
	config := strings.Replace(configTemplate, loopbackPlaceholder, provider.URL+"/v1", 1)
	configPath := filepath.Join(os.Getenv("CODEX_HOME"), "config.toml")
	writePairFile(t, configPath, config)
	configTemplateDigest := codexRequestEnvelopeDigest([]byte(configTemplate))
	policyArgs := []string{
		"-c", `approval_policy="never"`,
		"-c", `sandbox_mode="workspace-write"`,
		"-c", `sandbox_workspace_write.network_access=true`,
		"-c", `sandbox_workspace_write.exclude_tmpdir_env_var=true`,
		"-c", `sandbox_workspace_write.exclude_slash_tmp=true`,
		"-c", `analytics.enabled=false`,
		"-c", `check_for_update_on_startup=false`,
	}
	wrapper := writeCodexRequestEnvelopeWrapper(t, root, resolved, provider.URL)

	directCtx, directCancel := context.WithTimeout(t.Context(), 25*time.Second)
	directArgs := append([]string{"exec"}, policyArgs...)
	directArgs = append(directArgs, sessionArgs...)
	directArgs = append(directArgs, "--json", "--skip-git-repo-check", "-C", source, codexRequestEnvelopePrompt)
	direct := exec.CommandContext(directCtx, wrapper, directArgs...)
	direct.SysProcAttr = &syscall.SysProcAttr{Setpgid: true}
	direct.Cancel = func() error { return syscall.Kill(-direct.Process.Pid, syscall.SIGKILL) }
	direct.WaitDelay = 2 * time.Second
	directOutput, directErr, directGone := pairRunOwnedCommand(direct)
	directCancel()
	if directErr != nil || !directGone {
		t.Fatalf("native request-envelope arm failed: %v (output_bytes=%d group_gone=%t)", directErr, len(directOutput), directGone)
	}

	control := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method == http.MethodGet && r.URL.Path == "/api/daemon/workspaces/request-envelope-workspace/repos" {
			if err := json.NewEncoder(w).Encode(WorkspaceReposResponse{
				WorkspaceID: "request-envelope-workspace",
				Repos:       []RepoData{{URL: source, Ref: "main"}},
			}); err != nil {
				t.Error("encode synthetic workspace repos response")
			}
			return
		}
		if r.Method == http.MethodPost && strings.HasPrefix(r.URL.Path, "/api/daemon/tasks/") {
			_, _ = io.Copy(io.Discard, io.LimitReader(r.Body, 2<<20))
			w.WriteHeader(http.StatusOK)
			return
		}
		http.Error(w, "unexpected synthetic control request", http.StatusNotFound)
	}))
	defer control.Close()
	logger := slog.New(slog.NewTextHandler(io.Discard, nil))
	cfg := Config{
		DaemonID: "request-envelope-daemon", Profile: "request-envelope-private",
		WorkspacesRoot: filepath.Join(root, "workspaces"), ServerBaseURL: control.URL,
		Agents:       map[string]AgentEntry{"codex": {Path: wrapper}},
		AgentTimeout: 25 * time.Second, CodexHandshakeTimeout: 5 * time.Second,
		CodexThreadHandshakeTimeout: 5 * time.Second, DirectAgentMode: false,
	}
	d := New(cfg, logger)
	d.client.SetToken("synthetic-control-token")
	d.setAgentVersion("codex", "0.153.4")
	d.executionEnvironmentCommand = nil
	resourceRef, err := json.Marshal(map[string]any{
		"url": source, "ref": "main", "configuration_policy": projectPolicy,
	})
	if err != nil {
		t.Fatal(err)
	}
	task := Task{
		ID: "77777777-7777-4777-8777-777777777777", WorkspaceID: "request-envelope-workspace",
		WorkspaceSlug: "request-envelope", IssueID: "request-envelope-issue", IssueIdentifier: "REQ-1",
		AgentID: "request-envelope-agent", RuntimeID: "request-envelope-runtime",
		ClaimGeneration: 1, IssueCompletionContractVersion: 1, AuthToken: "mat_synthetic_request_envelope_token",
		ConciseMode: false, HandoffNote: codexRequestEnvelopePrompt,
		ProjectID: "request-envelope-project", ProjectTitle: "Synthetic request-envelope project",
		ProjectDescription: "Private loopback-only request-envelope metadata.",
		Agent: &AgentData{
			ID: "request-envelope-agent", Name: "Request envelope fixture",
			Instructions: "Use only the synthetic repository and return the requested fixture text.",
			CustomArgs:   policyArgs, McpConfig: json.RawMessage(`{"mcpServers":{}}`),
		},
		Repos:            []RepoData{{URL: source, Ref: "main"}},
		ProjectResources: []ProjectResourceData{{ResourceType: "github_repo", ResourceRef: resourceRef}},
	}
	d.workspaces[task.WorkspaceID] = newWorkspaceState(task.WorkspaceID, nil, "", task.Repos, nil)
	cacheRoot := filepath.Join(root, "repo-cache")
	cache := repocache.New(cacheRoot, logger)
	if err := cache.Sync(task.WorkspaceID, []repocache.RepoInfo{{URL: source}}); err != nil {
		t.Fatalf("sync synthetic primary repository: %v", err)
	}
	d.repoCache = cache

	briefCtx := execenv.TaskContextForEnv{IssueID: task.IssueID, IssueCompletionContractVersion: 1}
	withCompletion := execenv.RenderRuntimeBrief("codex", briefCtx)
	briefCtx.IssueCompletionContractVersion = 0
	withoutCompletion := execenv.RenderRuntimeBrief("codex", briefCtx)

	daemonCtx, daemonCancel := context.WithTimeout(t.Context(), 35*time.Second)
	result, runErr := d.runTask(daemonCtx, task, "codex", 0, logger)
	daemonCancel()
	if runErr != nil || result.Status != "completed" || result.Comment != codexRequestEnvelopeFinal {
		t.Fatalf("ordinary daemon runTask arm failed: status=%q error=%v", result.Status, runErr)
	}
	if result.IssueCompletionContractVersion != 1 {
		t.Fatalf("ordinary daemon result completion contract=%d, want 1", result.IssueCompletionContractVersion)
	}
	if got := runGitForGC(t, source, "status", "--porcelain"); got != "" {
		t.Fatal("synthetic source repository was modified")
	}
	if runGitForGC(t, source, "rev-parse", "HEAD") != sourceCommit || runGitForGC(t, source, "rev-parse", "HEAD^{tree}") != sourceTree {
		t.Fatal("synthetic source repository identity changed")
	}
	if runGitForGC(t, result.WorkDir, "rev-parse", "HEAD^{tree}") != sourceTree {
		t.Fatal("ordinary daemon arm did not use the same synthetic repository tree")
	}
	sourceRules, sourceRulesErr := os.ReadFile(filepath.Join(source, "AGENTS.md"))
	managedRules, managedRulesErr := os.ReadFile(filepath.Join(result.WorkDir, "AGENTS.md"))
	if sourceRulesErr != nil || managedRulesErr != nil || string(sourceRules) != string(managedRules) {
		t.Fatal("actual managed primary repository rules differ from the synthetic source")
	}

	mu.Lock()
	gotObservations := append([]codexRequestEnvelopeObservation(nil), observations...)
	gotUnexpected := append([]string(nil), unexpected...)
	mu.Unlock()
	if len(gotUnexpected) != 0 {
		t.Fatalf("unexpected provider requests: %v", gotUnexpected)
	}
	if len(gotObservations) != 2 {
		t.Fatalf("first-body observations=%d, want 2", len(gotObservations))
	}
	for _, observation := range gotObservations {
		// Codex 0.150+ excludes project instructions for untrusted projects.
		// Passing this negative test does not establish repository-context parity.
		expectedRules := 1
		if observation.Arm == "ordinary_daemon_runTask" && projectPolicy == "restricted" {
			expectedRules = 0
		}
		observedRules := -1
		if observation.RepositoryRuleOccurrences != nil {
			observedRules = *observation.RepositoryRuleOccurrences
		}
		if observedRules != expectedRules {
			t.Fatalf("%s first request repository rule occurrences=%d, want %d for project policy %s", observation.Arm, observedRules, expectedRules, projectPolicy)
		}
		if observation.PromptOccurrences != 1 || !observation.NativeIdentityObserved {
			t.Fatalf("%s request source proof incomplete: prompt_occurrences=%d native_identity=%t", observation.Arm, observation.PromptOccurrences, observation.NativeIdentityObserved)
		}
	}
	if gotObservations[0].AutomaticDeliveryOccurrences != 0 {
		t.Fatal("native fixture prompt unexpectedly contained the Multica automatic-delivery contract")
	}
	if gotObservations[1].AutomaticDeliveryOccurrences == 0 {
		t.Fatal("ordinary daemon first request omitted the automatic-delivery contract")
	}

	receipt := codexRequestEnvelopeReceipt{
		SchemaVersion: "codex_request_envelope_diagnostic/v1",
		Status:        "captured",
		Scope:         "first_actual_responses_body_native_exec_vs_ordinary_daemon_runTask_no_auth_loopback_only",
		OrdinaryMode:  true,
		Runtime: map[string]any{
			"provider_cli_version": "codex-cli 0.153.4",
			"provider_cli_sha256":  binaryDigest,
		},
		Fixture: map[string]any{
			"native_session_mode":                      nativeSessionMode,
			"managed_session_mode":                     "persistent",
			"managed_project_policy":                   projectPolicy,
			"native_project_policy":                    "unconfigured",
			"source_config_template_sha256":            configTemplateDigest,
			"model":                                    codexRequestEnvelopeModel,
			"prompt":                                   codexRequestEnvelopePart{Bytes: len(codexRequestEnvelopePrompt), SHA256: codexRequestEnvelopeDigest([]byte(codexRequestEnvelopePrompt))},
			"synthetic_repo_tree_oid":                  sourceTree,
			"same_tree_both_arms":                      true,
			"same_agents_file_both_arms":               true,
			"agents_file":                              codexRequestEnvelopePart{Bytes: len(sourceRules), SHA256: codexRequestEnvelopeDigest(sourceRules)},
			"repository_rule_present_in_both_requests": *gotObservations[0].RepositoryRuleOccurrences == 1 && *gotObservations[1].RepositoryRuleOccurrences == 1,
		},
		Provider: map[string]any{
			"transport":                   "strict_loopback_http",
			"requires_auth":               false,
			"credential_headers_observed": false,
			"external_model":              false,
			"usage":                       "synthetic_fixture_values_only_not_tokenizer_or_billing_evidence",
		},
		BriefDelta: codexRequestEnvelopeBriefDelta{
			WithoutCompletion: codexRequestEnvelopePart{Bytes: len(withoutCompletion), SHA256: codexRequestEnvelopeDigest([]byte(withoutCompletion))},
			WithCompletion:    codexRequestEnvelopePart{Bytes: len(withCompletion), SHA256: codexRequestEnvelopeDigest([]byte(withCompletion))},
			DeltaBytes:        len(withCompletion) - len(withoutCompletion),
		},
		Arms: gotObservations,
		Comparison: map[string]int{
			"body_bytes_delta":         gotObservations[1].Body.Bytes - gotObservations[0].Body.Bytes,
			"instructions_bytes_delta": gotObservations[1].Instructions.Bytes - gotObservations[0].Instructions.Bytes,
			"input_bytes_delta":        gotObservations[1].Input.Bytes - gotObservations[0].Input.Bytes,
			"developer_bytes_delta":    codexRequestEnvelopeRoleBytes(gotObservations[1], "developer") - codexRequestEnvelopeRoleBytes(gotObservations[0], "developer"),
			"user_bytes_delta":         codexRequestEnvelopeRoleBytes(gotObservations[1], "user") - codexRequestEnvelopeRoleBytes(gotObservations[0], "user"),
			"tools_bytes_delta":        gotObservations[1].Tools.Bytes - gotObservations[0].Tools.Bytes,
			"tool_count_delta":         gotObservations[1].Tools.Count - gotObservations[0].Tools.Count,
		},
		Limitations: []string{
			"does_not_reproduce_historical_T4M1_or_claim_an_empty_HOME_equivalence",
			"does_not_measure_actual_tokens_paid_usage_billing_or_external_model_behavior",
			"does_not_prove_queue_dispatch_or_production_daemon_binary_execution",
			"request_body_difference_is_diagnostic_not_a_pure_causal_attribution",
			"source_checkout_hash_is_reported_by_the_parent_harness_after_coordinated_execution",
			"restricted_policy_negative_test_pass_is_not_repository_instruction_parity",
			"matching_rule_presence_is_not_instruction_adherence_or_full_context_equivalence",
		},
	}
	encoded, err := json.Marshal(receipt)
	if err != nil {
		t.Fatal(err)
	}
	t.Logf("%s%s", codexRequestEnvelopeReceiptMark, encoded)
}

func codexRequestEnvelopeNativeSessionArgs(mode string) ([]string, error) {
	switch mode {
	case "", "ephemeral":
		return []string{"--ephemeral"}, nil
	case "persistent":
		return nil, nil
	default:
		return nil, fmt.Errorf("request-envelope diagnostic native session must be ephemeral or persistent")
	}
}

func codexRequestEnvelopeRoleBytes(observation codexRequestEnvelopeObservation, role string) int {
	for _, summary := range observation.Input.Roles {
		if summary.Role == role {
			return summary.Bytes
		}
	}
	return 0
}

func summarizeCodexRequestEnvelope(t *testing.T, arm string, body []byte, request map[string]any) codexRequestEnvelopeObservation {
	t.Helper()
	keys := make([]string, 0, len(request))
	for key := range request {
		keys = append(keys, key)
	}
	sort.Strings(keys)
	instructions, _ := request["instructions"].(string)
	input, _ := request["input"].([]any)
	tools, _ := request["tools"].([]any)
	inputBytes := codexRequestEnvelopeJSON(t, input)
	toolBytes := codexRequestEnvelopeJSON(t, tools)

	byRole := make(map[string][]any)
	repositoryRuleOccurrences := strings.Count(instructions, codexRequestEnvelopeRepoRule)
	for _, raw := range input {
		item, _ := raw.(map[string]any)
		for _, text := range codexRequestEnvelopeDecodedTexts(item) {
			repositoryRuleOccurrences += strings.Count(text, codexRequestEnvelopeRepoRule)
		}
		role, _ := item["role"].(string)
		if role == "" {
			role = "none"
		}
		byRole[role] = append(byRole[role], raw)
	}
	roles := make([]string, 0, len(byRole))
	for role := range byRole {
		roles = append(roles, role)
	}
	sort.Strings(roles)
	roleSummaries := make([]codexRequestEnvelopeRole, 0, len(roles))
	for _, role := range roles {
		encoded := codexRequestEnvelopeJSON(t, byRole[role])
		roleSummaries = append(roleSummaries, codexRequestEnvelopeRole{
			Role: role, Items: len(byRole[role]),
			codexRequestEnvelopePart: codexRequestEnvelopePart{Bytes: len(encoded), SHA256: codexRequestEnvelopeDigest(encoded)},
		})
	}
	return codexRequestEnvelopeObservation{
		Arm:          arm,
		Body:         codexRequestEnvelopePart{Bytes: len(body), SHA256: codexRequestEnvelopeDigest(body)},
		KeyNames:     keys,
		Model:        fmt.Sprint(request["model"]),
		Instructions: codexRequestEnvelopePart{Bytes: len(instructions), SHA256: codexRequestEnvelopeDigest([]byte(instructions))},
		Input: codexRequestEnvelopeInput{
			codexRequestEnvelopePart: codexRequestEnvelopePart{Bytes: len(inputBytes), SHA256: codexRequestEnvelopeDigest(inputBytes)},
			Roles:                    roleSummaries,
			Inventory:                codexRequestEnvelopeInputInventory(t, input),
		},
		Tools: codexRequestEnvelopeTools{
			codexRequestEnvelopePart: codexRequestEnvelopePart{Bytes: len(toolBytes), SHA256: codexRequestEnvelopeDigest(toolBytes)},
			Count:                    len(tools),
			Inventory:                codexRequestEnvelopeToolInventory(t, tools),
		},
		PromptOccurrences:            codexRequestEnvelopeStringOccurrences(input, codexRequestEnvelopePrompt),
		AutomaticDeliveryOccurrences: codexRequestEnvelopeStringOccurrences(input, "Multica posts that exact final response"),
		RepositoryRuleOccurrences:    &repositoryRuleOccurrences,
	}
}

func codexRequestEnvelopeStringOccurrences(value any, needle string) int {
	switch value := value.(type) {
	case string:
		return strings.Count(value, needle)
	case []any:
		total := 0
		for _, item := range value {
			total += codexRequestEnvelopeStringOccurrences(item, needle)
		}
		return total
	case map[string]any:
		total := 0
		for _, item := range value {
			total += codexRequestEnvelopeStringOccurrences(item, needle)
		}
		return total
	default:
		return 0
	}
}

func codexRequestEnvelopeJSON(t *testing.T, value any) []byte {
	t.Helper()
	encoded, err := json.Marshal(value)
	if err != nil {
		t.Fatal(err)
	}
	return encoded
}

func codexRequestEnvelopeDigest(data []byte) string {
	digest := sha256.Sum256(data)
	return fmt.Sprintf("sha256:%x", digest[:])
}

func parseCodexRequestEnvelopeReceipt(output []byte) (codexRequestEnvelopeReceipt, error) {
	var receipt codexRequestEnvelopeReceipt
	found := 0
	for _, line := range strings.Split(string(output), "\n") {
		index := strings.Index(line, codexRequestEnvelopeReceiptMark)
		if index < 0 {
			continue
		}
		found++
		if err := json.Unmarshal([]byte(strings.TrimSpace(line[index+len(codexRequestEnvelopeReceiptMark):])), &receipt); err != nil {
			return receipt, fmt.Errorf("parse inner request-envelope receipt: %w", err)
		}
	}
	if found != 1 {
		return receipt, fmt.Errorf("inner request-envelope receipts=%d, want 1", found)
	}
	return receipt, nil
}

func writeCodexRequestEnvelopeWrapper(t *testing.T, root, realCodex, providerURL string) string {
	t.Helper()
	q := pairShellQuote
	wrapper := filepath.Join(root, "codex-request-envelope-isolated")
	body := "#!/bin/sh\nset -eu\ncase \"${1:-}\" in\n" +
		"exec) role=direct;;\n" +
		"app-server) role=daemon;;\n" +
		"*) role=other;; esac\n" +
		"if [ \"$role\" != other ]; then start=$(/bin/ps -p \"$$\" -o lstart=); printf '%s\\n%s\\n%s\\n' \"$$\" \"$start\" \"$role\" > " + q(filepath.Join(root, "processes")) + "/$$; fi\n" +
		"exec env -i PATH='/usr/bin:/bin:/usr/sbin:/sbin' HOME=\"$HOME\" CODEX_HOME=\"$CODEX_HOME\" TMPDIR=\"$TMPDIR\" " +
		"XDG_CONFIG_HOME=\"${XDG_CONFIG_HOME:-}\" XDG_DATA_HOME=\"${XDG_DATA_HOME:-}\" XDG_STATE_HOME=\"${XDG_STATE_HOME:-}\" XDG_CACHE_HOME=\"${XDG_CACHE_HOME:-}\" " +
		"GIT_CONFIG_NOSYSTEM=1 GIT_CONFIG_GLOBAL=" + q(os.DevNull) + " HTTP_PROXY=" + q(providerURL) + " HTTPS_PROXY=" + q(providerURL) + " ALL_PROXY=" + q(providerURL) + " " +
		"NO_PROXY='127.0.0.1,localhost,::1' OTEL_SDK_DISABLED=true " + q(realCodex) + " \"$@\"\n"
	writePairFile(t, wrapper, body)
	if err := os.Chmod(wrapper, 0o700); err != nil {
		t.Fatal(err)
	}
	return wrapper
}
