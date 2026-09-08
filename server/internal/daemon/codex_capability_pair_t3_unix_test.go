//go:build !windows

package daemon

import (
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/hex"
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
	"regexp"
	"slices"
	"strings"
	"sync"
	"syscall"
	"testing"
	"time"
)

const pairT3MaxCacheFiles = 200000
const pairT3MaxCacheFileBytes int64 = 64 << 20
const pairT3MaxCacheBytes int64 = 512 << 20
const pairT3MaxLockedFileBytes int64 = 512 << 20
const pairT3PnpmVersion = "10.28.2"

var pairT3Hex32 = regexp.MustCompile(`^[a-f0-9]{32}$`)
var pairT3GitRevision = regexp.MustCompile(`^[a-f0-9]{40,64}$`)
var pairT3SHA256 = regexp.MustCompile(`^sha256:[a-f0-9]{64}$`)

type pairT3File struct {
	Path   string `json:"path"`
	SHA256 string `json:"sha256"`
}

type pairT3Arm struct {
	ArmID         string              `json:"arm_id"`
	Nonce         string              `json:"nonce"`
	SandboxPolicy pairT3SandboxPolicy `json:"sandbox_policy"`
}

type pairT3SandboxPolicy struct {
	SHA256              string `json:"sha256"`
	NetworkAccess       bool   `json:"network_access"`
	ExcludeTmpdirEnvVar bool   `json:"exclude_tmpdir_env_var"`
	ExcludeSlashTmp     bool   `json:"exclude_slash_tmp"`
}

type pairT3Directory struct {
	Path      string `json:"path"`
	SHA256    string `json:"sha256"`
	FileCount int    `json:"file_count"`
	Bytes     int64  `json:"bytes"`
}

type pairT3Runner struct {
	Path    string       `json:"path"`
	SHA256  string       `json:"sha256"`
	Sources []pairT3File `json:"sources"`
}

type pairT3Spec struct {
	SchemaVersion           string       `json:"schema_version"`
	CollectorClassification string       `json:"collector_classification"`
	Registration            pairT3File   `json:"registration"`
	ProductCodeCheck        pairT3File   `json:"product_code_check"`
	Runner                  pairT3Runner `json:"runner"`
	Source                  struct {
		BareRepository  string       `json:"bare_repository"`
		Revision        string       `json:"revision"`
		TreeSHA256      string       `json:"tree_sha256"`
		DependencyFiles []pairT3File `json:"dependency_files"`
	} `json:"source"`
	Arms struct {
		Native  pairT3Arm `json:"native"`
		Managed pairT3Arm `json:"managed"`
	} `json:"arms"`
	Toolchain struct {
		Node pairT3File `json:"node"`
		Git  pairT3File `json:"git"`
		Go   pairT3File `json:"go"`
		Pnpm pairT3File `json:"pnpm"`
	} `json:"toolchain"`
	SyntheticTargetRef string `json:"synthetic_target_ref"`
	EvidenceOutput     string `json:"evidence_output"`
	CacheSeeds         struct {
		GoMod          pairT3Directory `json:"go_mod"`
		PnpmStore      pairT3Directory `json:"pnpm_store"`
		CorepackBundle pairT3Directory `json:"corepack_bundle"`
	} `json:"cache_seeds"`
	PnpmVersion string `json:"pnpm_version"`
}

type pairT3Diagnostic struct {
	SchemaVersion string           `json:"schema_version"`
	Status        string           `json:"status"`
	FailureCode   string           `json:"failure_code,omitempty"`
	Scope         string           `json:"scope"`
	ParityClaim   bool             `json:"parity_claim"`
	OrdinaryMode  bool             `json:"ordinary_mode"`
	Registration  string           `json:"registration_sha256,omitempty"`
	Source        string           `json:"source_tree_sha256,omitempty"`
	Runtime       map[string]any   `json:"runtime,omitempty"`
	Arms          []map[string]any `json:"arms,omitempty"`
}

func parseCodexT3CapabilitySpec(data []byte) (pairT3Spec, error) {
	return parseCodexTaskCapabilitySpec(data, "T3")
}

func parseCodexTaskCapabilitySpec(data []byte, taskID string) (pairT3Spec, error) {
	var spec pairT3Spec
	if taskID != "T3" && taskID != "T4" {
		return spec, errors.New("unsupported capability task")
	}
	decoder := json.NewDecoder(bytes.NewReader(data))
	decoder.DisallowUnknownFields()
	if err := decoder.Decode(&spec); err != nil {
		return spec, fmt.Errorf("decode T3 capability spec: %w", err)
	}
	if decoder.Decode(&struct{}{}) != io.EOF {
		return spec, errors.New("T3 capability spec has trailing JSON")
	}
	if spec.SchemaVersion != "codex_capability_"+strings.ToLower(taskID)+"_spec/v1" || !slices.Contains([]string{"real_local_no_auth", "synthetic_test_diagnostic"}, spec.CollectorClassification) || !pairT3GitRevision.MatchString(spec.Source.Revision) || spec.PnpmVersion != pairT3PnpmVersion {
		return spec, errors.New("T3 capability spec schema or revision is invalid")
	}
	for name, file := range map[string]pairT3File{
		"registration": spec.Registration, "runner": {Path: spec.Runner.Path, SHA256: spec.Runner.SHA256}, "product_code_check": spec.ProductCodeCheck, "node": spec.Toolchain.Node,
		"git": spec.Toolchain.Git, "go": spec.Toolchain.Go, "pnpm": spec.Toolchain.Pnpm,
	} {
		if !filepath.IsAbs(file.Path) || !pairT3SHA256.MatchString(file.SHA256) {
			return spec, fmt.Errorf("T3 capability %s lock is invalid", name)
		}
	}
	if len(spec.Runner.Sources) != 8 {
		return spec, errors.New("T3 capability runner source locks are invalid")
	}
	for _, file := range spec.Runner.Sources {
		if !filepath.IsAbs(file.Path) || !pairT3SHA256.MatchString(file.SHA256) {
			return spec, errors.New("T3 capability runner source lock is invalid")
		}
	}
	for _, directory := range []pairT3Directory{spec.CacheSeeds.GoMod, spec.CacheSeeds.PnpmStore, spec.CacheSeeds.CorepackBundle} {
		if !filepath.IsAbs(directory.Path) || !pairT3SHA256.MatchString(directory.SHA256) || directory.FileCount < 0 || directory.FileCount > pairT3MaxCacheFiles || directory.Bytes < 0 || directory.Bytes > pairT3MaxCacheBytes {
			return spec, errors.New("T3 capability cache seed is invalid")
		}
	}
	if !filepath.IsAbs(spec.Source.BareRepository) || !filepath.IsAbs(spec.EvidenceOutput) || !pairT3SHA256.MatchString(spec.Source.TreeSHA256) ||
		spec.SyntheticTargetRef == "" || len(spec.Source.DependencyFiles) == 0 {
		return spec, errors.New("T3 capability source contract is invalid")
	}
	seen := map[string]bool{}
	for _, file := range spec.Source.DependencyFiles {
		if file.Path == "" || filepath.IsAbs(file.Path) || filepath.Clean(file.Path) != file.Path ||
			file.Path == "." || strings.HasPrefix(file.Path, ".."+string(filepath.Separator)) ||
			!pairT3SHA256.MatchString(file.SHA256) || seen[file.Path] {
			return spec, errors.New("T3 capability dependency lock is invalid")
		}
		seen[file.Path] = true
	}
	for _, arm := range []pairT3Arm{spec.Arms.Native, spec.Arms.Managed} {
		if arm.ArmID == "" || !pairT3Hex32.MatchString(arm.Nonce) || !pairT3SHA256.MatchString(arm.SandboxPolicy.SHA256) ||
			!arm.SandboxPolicy.NetworkAccess || arm.SandboxPolicy.ExcludeTmpdirEnvVar || arm.SandboxPolicy.ExcludeSlashTmp {
			return spec, errors.New("T3 capability arm is invalid")
		}
	}
	if spec.Arms.Native.ArmID == spec.Arms.Managed.ArmID || spec.Arms.Native.Nonce == spec.Arms.Managed.Nonce {
		return spec, errors.New("T3 capability arms must be distinct")
	}
	return spec, nil
}

func pairT3DaemonConfig(root, wrapper, controlURL string) Config {
	return Config{
		DaemonID: "capability-daemon", Profile: "capability-private",
		WorkspacesRoot: filepath.Join(root, "workspaces"), ServerBaseURL: controlURL,
		Agents:       map[string]AgentEntry{"codex": {Path: wrapper}},
		AgentTimeout: 70 * time.Second, CodexHandshakeTimeout: 5 * time.Second,
		CodexThreadHandshakeTimeout: 5 * time.Second, DirectAgentMode: false,
	}
}

func pairT3Task(cwd string, policyArgs []string) Task {
	ref, _ := json.Marshal(localDirectoryRef{LocalPath: cwd, DaemonID: "capability-daemon"})
	return Task{
		ID: "44444444-4444-4444-8444-444444444444", WorkspaceID: "capability-workspace",
		IssueID: "capability-issue", AgentID: "capability-agent", RuntimeID: "capability-runtime",
		AuthToken: "mat_synthetic_capability_token", ConciseMode: false,
		Agent:            &AgentData{ID: "capability-agent", Name: "Capability fixture", CustomArgs: policyArgs, McpConfig: json.RawMessage(`{"mcpServers":{}}`)},
		ProjectResources: []ProjectResourceData{{ResourceType: "local_directory", ResourceRef: ref}},
	}
}

func runCodexT3CapabilityPair(t *testing.T, root, realCodex string, executable codexCapabilityExecutable, specPath, taskID string) {
	t.Helper()
	taskSlug := strings.ToLower(taskID)
	diagnostic := pairT3Diagnostic{
		SchemaVersion: "codex_capability_" + taskSlug + "_pair/v1", Status: "failed",
		Scope:       "native_exec_vs_ordinary_runTask_no_queue_pinned_runtime_and_sandbox_only",
		ParityClaim: false, OrdinaryMode: true,
	}
	defer func() {
		encoded, _ := json.Marshal(diagnostic)
		t.Log("CODEX_" + taskID + "_CAPABILITY_PAIR_RECEIPT " + string(encoded))
	}()
	fail := func(code, message string, args ...any) {
		diagnostic.FailureCode = code
		t.Fatalf(message, args...)
	}

	specBytes, err := os.ReadFile(specPath)
	if err != nil {
		fail("SPEC_READ_FAILED", "read T3 capability spec: %v", err)
	}
	spec, err := parseCodexTaskCapabilitySpec(specBytes, taskID)
	if err != nil {
		fail("SPEC_INVALID", "%v", err)
	}
	for name, file := range map[string]pairT3File{
		"registration": spec.Registration, "runner": {Path: spec.Runner.Path, SHA256: spec.Runner.SHA256}, "product_code_check": spec.ProductCodeCheck, "node": spec.Toolchain.Node,
		"git": spec.Toolchain.Git, "go": spec.Toolchain.Go, "pnpm": spec.Toolchain.Pnpm,
	} {
		resolved, resolveErr := filepath.EvalSymlinks(file.Path)
		if resolveErr != nil || resolved != file.Path || "sha256:"+pairT3FileDigest(file.Path) != file.SHA256 {
			fail("LOCK_MISMATCH", "T3 capability %s lock mismatch", name)
		}
	}
	for _, file := range spec.Runner.Sources {
		resolved, resolveErr := filepath.EvalSymlinks(file.Path)
		if resolveErr != nil || resolved != file.Path || "sha256:"+pairT3FileDigest(file.Path) != file.SHA256 {
			fail("RUNNER_SOURCE_LOCK_MISMATCH", "T3 runner source lock mismatch")
		}
	}
	for _, seed := range []pairT3Directory{spec.CacheSeeds.GoMod, spec.CacheSeeds.PnpmStore, spec.CacheSeeds.CorepackBundle} {
		if observed, hashErr := pairT3HashTree(spec, seed.Path); hashErr != nil || observed != seed {
			fail("CACHE_SEED_MISMATCH", "T3 cache seed mismatch")
		}
	}
	if err := pairT3RequirePnpmStoreSeed(spec.CacheSeeds.PnpmStore.Path); err != nil {
		fail("CACHE_SEED_MISMATCH", "pnpm store seed contains excluded registry content")
	}
	registration, err := os.ReadFile(spec.Registration.Path)
	if err != nil {
		fail("REGISTRATION_READ_FAILED", "read registration: %v", err)
	}
	if err := pairValidateTaskCapabilityRegistration(registration, taskID); err != nil {
		fail("SPEC_INVALID", "%v", err)
	}
	diagnostic.Registration = spec.Registration.SHA256
	diagnostic.Source = spec.Source.TreeSHA256

	workspaces := [2]pairWorkspace{}
	commands := [2]string{}
	for index, arm := range []pairT3Arm{spec.Arms.Native, spec.Arms.Managed} {
		base := filepath.Join(root, []string{"direct", "daemon"}[index])
		workspaces[index] = pairWorkspace{cwd: filepath.Join(base, "repository"), outside: filepath.Join(base, "outside"), gocache: filepath.Join(base, "gocache")}
		if err := pairT3Checkout(spec, workspaces[index].cwd); err != nil {
			fail("SOURCE_CHECKOUT_FAILED", "prepare T3 capability source: %v", err)
		}
		if err := os.MkdirAll(workspaces[index].outside, 0o700); err != nil {
			fail("WORKSPACE_PREP_FAILED", "prepare outside sentinel directory: %v", err)
		}
		sentinel := filepath.Join(workspaces[index].outside, "denied.txt")
		content := fmt.Sprintf("%s_CAPABILITY_OUTSIDE_SENTINEL %s %s\n", taskID, arm.Nonce, spec.SyntheticTargetRef)
		if err := os.WriteFile(sentinel, []byte(content), 0o600); err != nil {
			fail("WORKSPACE_PREP_FAILED", "write outside sentinel: %v", err)
		}
		if err := os.WriteFile(filepath.Join(workspaces[index].cwd, "capability-source.txt"), []byte(taskID+"_CAPABILITY_SOURCE "+arm.Nonce+"\n"), 0o600); err != nil {
			fail("WORKSPACE_PREP_FAILED", "write source marker: %v", err)
		}
		if err := os.MkdirAll(workspaces[index].gocache, 0o700); err != nil {
			fail("WORKSPACE_PREP_FAILED", "prepare mutable cache: %v", err)
		}
		cacheRoot := filepath.Join(workspaces[index].cwd, ".capability-cache")
		goModCache := filepath.Join(cacheRoot, "go-mod")
		pnpmStore := filepath.Join(cacheRoot, "pnpm-store")
		corepackBundle := filepath.Join(cacheRoot, "corepack", "v1", "pnpm", spec.PnpmVersion)
		if err := pairT3CopyTree(spec.CacheSeeds.GoMod.Path, goModCache); err != nil {
			fail("CACHE_COPY_FAILED", "copy Go module cache: %v", err)
		}
		if copied, hashErr := pairT3HashTree(spec, goModCache); hashErr != nil || copied.SHA256 != spec.CacheSeeds.GoMod.SHA256 || copied.FileCount != spec.CacheSeeds.GoMod.FileCount || copied.Bytes != spec.CacheSeeds.GoMod.Bytes {
			fail("CACHE_COPY_MISMATCH", "copied Go module cache differs from seed")
		}
		if err := pairT3CopyTree(spec.CacheSeeds.PnpmStore.Path, pnpmStore); err != nil {
			fail("CACHE_COPY_FAILED", "copy pnpm store: %v", err)
		}
		if copied, hashErr := pairT3HashTree(spec, pnpmStore); hashErr != nil || copied.SHA256 != spec.CacheSeeds.PnpmStore.SHA256 || copied.FileCount != spec.CacheSeeds.PnpmStore.FileCount || copied.Bytes != spec.CacheSeeds.PnpmStore.Bytes {
			fail("CACHE_COPY_MISMATCH", "copied pnpm store differs from seed")
		}
		if err := pairT3RequirePnpmStoreSeed(pnpmStore); err != nil {
			fail("CACHE_COPY_MISMATCH", "copied pnpm store contains excluded registry content")
		}
		if err := pairT3CopyTree(spec.CacheSeeds.CorepackBundle.Path, corepackBundle); err != nil {
			fail("CACHE_COPY_FAILED", "copy Corepack bundle: %v", err)
		}
		if copied, hashErr := pairT3HashTree(spec, corepackBundle); hashErr != nil || copied.SHA256 != spec.CacheSeeds.CorepackBundle.SHA256 || copied.FileCount != spec.CacheSeeds.CorepackBundle.FileCount || copied.Bytes != spec.CacheSeeds.CorepackBundle.Bytes {
			fail("CACHE_COPY_MISMATCH", "copied Corepack bundle differs from seed")
		}
		if err := pairT3PrepareFrontend(spec, workspaces[index].cwd, pnpmStore, filepath.Join(cacheRoot, "corepack")); err != nil {
			fail("FRONTEND_PREPARATION_FAILED", "prepare T3 frontend: %v", err)
		}
		writableRoots, rootsErr := pairT3WritableRoots(workspaces[index].cwd, cacheRoot, workspaces[index].gocache)
		if rootsErr != nil || !pairT3OutsideEveryRoot(sentinel, writableRoots) {
			fail("SENTINEL_POLICY_MISMATCH", "outside sentinel is inside a writable root: %v", rootsErr)
		}
		input := map[string]any{
			"registration": json.RawMessage(registration), "arm_id": arm.ArmID,
			"scratch_root":     workspaces[index].cwd,
			"writable_roots":   writableRoots,
			"outside_sentinel": map[string]any{"path": sentinel, "synthetic_target_ref": spec.SyntheticTargetRef},
			"nonce":            arm.Nonce,
			"toolchain":        map[string]any{"classification": "frozen_real", "git": spec.Toolchain.Git.Path, "go": spec.Toolchain.Go.Path, "pnpm": spec.Toolchain.Pnpm.Path},
		}
		inputPath := filepath.Join(base, "probe-input.json")
		encoded, marshalErr := json.Marshal(input)
		if marshalErr != nil || os.WriteFile(inputPath, encoded, 0o600) != nil {
			fail("WORKSPACE_PREP_FAILED", "write T3 capability runner input")
		}
		commands[index] = strings.Join([]string{pairShellQuote(spec.Toolchain.Node.Path), pairShellQuote(spec.Runner.Path), "--task-entry", pairShellQuote(inputPath)}, " ")
		if taskID == "T4" {
			commands[index] += " --task T4"
		}
	}

	var mu sync.Mutex
	type observation struct {
		requests                                                int
		toolAdvertised, toolOutput, birthObserved, execObserved bool
		pid                                                     int
		markerSHA                                               string
	}
	observed := [2]observation{}
	var unexpected []string
	provider := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost || r.URL.Path != "/v1/responses" || r.URL.IsAbs() {
			mu.Lock()
			unexpected = append(unexpected, r.Method+" "+r.URL.Path)
			mu.Unlock()
			http.Error(w, "unexpected fixture request", http.StatusNotFound)
			return
		}
		var request map[string]any
		if json.NewDecoder(io.LimitReader(r.Body, 2<<20)).Decode(&request) != nil {
			http.Error(w, "invalid request", http.StatusBadRequest)
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
		} else if turn == 2 {
			observed[arm].toolOutput, observed[arm].markerSHA = pairTaskToolOutputProof(request, taskID)
		}
		mu.Unlock()
		w.Header().Set("Content-Type", "text/event-stream")
		if turn == 1 {
			pairWriteSSE(t, w, fmt.Sprintf("t3_tool_%d", arm), map[string]any{"type": "function_call", "id": fmt.Sprintf("fc_%d", arm), "call_id": fmt.Sprintf("call_%d", arm), "name": "exec_command", "arguments": pairJSON(t, map[string]any{"cmd": commands[arm]}), "status": "completed"})
			return
		}
		pairWriteSSE(t, w, fmt.Sprintf("t3_final_%d", arm), map[string]any{
			"type": "message", "id": fmt.Sprintf("msg_%d", arm), "status": "completed", "role": "assistant",
			"content": []any{map[string]any{
				"type": "output_text", "text": "capability-pair-complete", "annotations": []any{},
			}},
		})
	}))
	defer provider.Close()

	config := strings.Join([]string{
		`model = "syntheticloopback"`, `model_provider = "syntheticloopback"`, `approval_policy = "never"`, `sandbox_mode = "workspace-write"`,
		`disable_response_storage = true`, `check_for_update_on_startup = false`, `[analytics]`, `enabled = false`,
		`[sandbox_workspace_write]`, `network_access = true`, `exclude_tmpdir_env_var = false`, `exclude_slash_tmp = false`,
		`[model_providers.syntheticloopback]`, `name = "Synthetic loopback"`, `base_url = ` + fmt.Sprintf("%q", provider.URL+"/v1"), `wire_api = "responses"`, `requires_openai_auth = false`,
		`[features]`, `responses_websockets = false`, `multi_agent = false`, `memories = false`, `plugins = false`, "",
	}, "\n")
	if err := os.WriteFile(filepath.Join(os.Getenv("CODEX_HOME"), "config.toml"), []byte(config), 0o600); err != nil {
		fail("CONFIG_WRITE_FAILED", "write private Codex config: %v", err)
	}
	policyArgs := []string{"-c", `approval_policy="never"`, "-c", `sandbox_mode="workspace-write"`, "-c", `sandbox_workspace_write.network_access=true`, "-c", `sandbox_workspace_write.exclude_tmpdir_env_var=false`, "-c", `sandbox_workspace_write.exclude_slash_tmp=false`, "-c", `analytics.enabled=false`, "-c", `check_for_update_on_startup=false`}
	wrapper := writeT3CapabilityWrapper(t, root, realCodex, provider.URL, pairT3FrozenPath(spec), workspaces)

	directCtx, directCancel := context.WithTimeout(t.Context(), 70*time.Second)
	directArgs := append([]string{"exec"}, policyArgs...)
	directArgs = append(directArgs, "--json", "--ephemeral", "--skip-git-repo-check", "-C", workspaces[0].cwd, "Execute the fixture-provided command exactly once, then finish.")
	direct := exec.CommandContext(directCtx, wrapper, directArgs...)
	direct.SysProcAttr = &syscall.SysProcAttr{Setpgid: true}
	direct.Cancel = func() error { return syscall.Kill(-direct.Process.Pid, syscall.SIGKILL) }
	direct.WaitDelay = 2 * time.Second
	_, directErr, directGone := pairRunOwnedCommand(direct)
	directCancel()
	if directErr != nil || !directGone {
		fail("NATIVE_ARM_FAILED", "T3 native arm failed: %v (group_gone=%t)", directErr, directGone)
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
	cfg := pairT3DaemonConfig(root, wrapper, control.URL)
	d := New(cfg, slog.New(slog.NewTextHandler(io.Discard, nil)))
	d.client.SetToken("synthetic-control-token")
	d.setAgentVersion("codex", strings.TrimPrefix(executable.Version, "codex-cli "))
	d.executionEnvironmentCommand = func() ([]string, error) {
		return []string{os.Args[0], "-test.run=^TestCodexR12PreparationHelper$", "--", "r12-preparation-helper"}, nil
	}
	daemonCtx, daemonCancel := context.WithTimeout(t.Context(), 70*time.Second)
	result, runErr := d.runTask(daemonCtx, pairT3Task(workspaces[1].cwd, policyArgs), "codex", 0, slog.New(slog.NewTextHandler(io.Discard, nil)))
	daemonCancel()
	if runErr != nil || result.Status != "completed" || result.Comment != "capability-pair-complete" {
		fail("MANAGED_ARM_FAILED", "T3 ordinary daemon arm failed: status=%q error=%v", result.Status, runErr)
	}
	mu.Lock()
	got, gotUnexpected := observed, append([]string(nil), unexpected...)
	mu.Unlock()
	if len(gotUnexpected) != 0 {
		fail("UNEXPECTED_PROVIDER_REQUEST", "unexpected provider requests: %v", gotUnexpected)
	}
	diagnostic.Arms = make([]map[string]any, 0, 2)
	evidenceArms := make([]map[string]any, 0, 2)
	for index, arm := range []pairT3Arm{spec.Arms.Native, spec.Arms.Managed} {
		artifact := filepath.Join(workspaces[index].cwd, ".benchmark-capability", taskSlug+"-capability-probe.json")
		artifactBytes, readErr := pairT3ReadStable(artifact, 4<<20)
		artifactHash := pairT3Digest(artifactBytes)
		if got[index].requests != 2 || !got[index].toolAdvertised || !got[index].toolOutput || !got[index].birthObserved || !got[index].execObserved || artifactHash == "" {
			fail("ARM_EVIDENCE_INCOMPLETE", "T3 arm %s evidence incomplete", arm.ArmID)
		}
		var runnerReceipt map[string]any
		if readErr != nil || json.Unmarshal(artifactBytes, &runnerReceipt) != nil {
			fail("ARM_ARTIFACT_INVALID", "T3 arm %s artifact is invalid", arm.ArmID)
		}
		diagnostic.Arms = append(diagnostic.Arms, map[string]any{
			"arm_id": arm.ArmID, "requests": got[index].requests, "tool_advertised": true,
			"tool_output_observed": true, "artifact_sha256": "sha256:" + artifactHash,
			"native_pid": got[index].pid, "native_identity": true,
			"marker_sha256": got[index].markerSHA,
		})
		evidenceArms = append(evidenceArms, map[string]any{"arm_id": arm.ArmID, "artifact_sha256": "sha256:" + artifactHash, "receipt": runnerReceipt})
	}
	if !pairCleanupRecordedGroups(filepath.Join(root, "processes")) {
		fail("PROCESS_CLEANUP_UNPROVEN", "T3 native process cleanup was not proved")
	}
	for _, workspace := range workspaces {
		if err := pairT3VerifyTrackedSource(spec, workspace.cwd); err != nil {
			fail("TRACKED_SOURCE_DRIFT", "T3 tracked source changed: %v", err)
		}
	}
	testBinary, _ := os.Executable()
	evidenceBytes, err := json.Marshal(map[string]any{"schema_version": "codex_capability_" + taskSlug + "_evidence/v1", "arms": evidenceArms})
	if err != nil {
		fail("EVIDENCE_ENCODE_FAILED", "encode T3 evidence: %v", err)
	}
	evidenceFile, err := os.OpenFile(spec.EvidenceOutput, os.O_WRONLY|os.O_CREATE|os.O_EXCL, 0o600)
	if err != nil {
		fail("EVIDENCE_WRITE_FAILED", "create T3 evidence output: %v", err)
	}
	_, writeErr := evidenceFile.Write(evidenceBytes)
	closeErr := evidenceFile.Close()
	if writeErr != nil || closeErr != nil {
		fail("EVIDENCE_WRITE_FAILED", "write T3 evidence output")
	}
	diagnostic.Runtime = map[string]any{
		"provider_cli_version": executable.Version, "provider_cli_sha256": executable.SHA256,
		"test_binary_sha256":               "sha256:" + pairT3FileDigest(testBinary),
		"test_binary_is_production_daemon": false,
		"evidence_sha256":                  "sha256:" + pairT3Digest(evidenceBytes),
		"tool_search_observed":             false,
		"not_proven":                       []string{"queue_dispatch", "real_provider_model", "billing", "tool_search_effective_value"},
		"sandbox_policy_sha256": map[string]string{
			spec.Arms.Native.ArmID:  spec.Arms.Native.SandboxPolicy.SHA256,
			spec.Arms.Managed.ArmID: spec.Arms.Managed.SandboxPolicy.SHA256,
		},
		"identical_physical_writable_roots_claimed": false,
	}
	diagnostic.Status = "passed"
}

func pairT3WritableRoots(cwd, cacheRoot, goCache string) ([]string, error) {
	roots := []string{cwd, cacheRoot, filepath.Join(cacheRoot, "go-mod"), filepath.Join(cacheRoot, "pnpm-store"), filepath.Join(cacheRoot, "corepack"), goCache, "/tmp"}
	if _, err := os.Stat("/private/tmp"); err == nil {
		roots = append(roots, "/private/tmp")
	}
	if tmpdir := os.Getenv("TMPDIR"); filepath.IsAbs(tmpdir) {
		roots = append(roots, filepath.Clean(tmpdir))
	}
	for index, root := range roots {
		resolved, err := filepath.EvalSymlinks(root)
		if err != nil || !filepath.IsAbs(resolved) {
			return nil, fmt.Errorf("resolve writable root %q: %w", root, err)
		}
		roots[index] = resolved
	}
	slices.Sort(roots)
	return slices.Compact(roots), nil
}

func pairT3OutsideEveryRoot(candidate string, roots []string) bool {
	resolved, err := filepath.EvalSymlinks(candidate)
	if err != nil || !filepath.IsAbs(resolved) {
		return false
	}
	for _, root := range roots {
		relative, err := filepath.Rel(root, resolved)
		if err != nil || relative == "." || relative != ".." && !strings.HasPrefix(relative, ".."+string(filepath.Separator)) && !filepath.IsAbs(relative) {
			return false
		}
	}
	return true
}

func pairRelayT3CapabilityReceipt(t *testing.T, output []byte, taskID string) {
	t.Helper()
	marker := "CODEX_" + taskID + "_CAPABILITY_PAIR_RECEIPT "
	count := 0
	for _, line := range strings.Split(string(output), "\n") {
		if !strings.Contains(line, marker) {
			continue
		}
		count++
		t.Log(strings.TrimSpace(line[strings.Index(line, marker):]))
	}
	if count != 1 {
		t.Fatalf("isolated T3 capability pair receipt count=%d, want exactly one", count)
	}
}

func pairT3Checkout(spec pairT3Spec, cwd string) error {
	cmd := exec.Command(spec.Toolchain.Git.Path, "clone", "--quiet", "--no-local", spec.Source.BareRepository, cwd)
	cmd.Env = []string{"PATH=" + os.Getenv("PATH"), "HOME=" + os.Getenv("HOME"), "GIT_CONFIG_NOSYSTEM=1", "GIT_CONFIG_GLOBAL=" + os.DevNull, "GIT_TERMINAL_PROMPT=0"}
	if output, err := cmd.CombinedOutput(); err != nil {
		return fmt.Errorf("clone failed: %w (%d bytes)", err, len(output))
	}
	checkout := exec.Command(spec.Toolchain.Git.Path, "-C", cwd, "checkout", "--quiet", "--detach", spec.Source.Revision)
	checkout.Env = cmd.Env
	if output, err := checkout.CombinedOutput(); err != nil {
		return fmt.Errorf("checkout failed: %w (%d bytes)", err, len(output))
	}
	tree := exec.Command(spec.Toolchain.Git.Path, "--git-dir", spec.Source.BareRepository, "ls-tree", "-r", "--full-tree", spec.Source.Revision)
	tree.Env = cmd.Env
	output, err := tree.Output()
	if err != nil || "sha256:"+pairT3Digest(output) != spec.Source.TreeSHA256 {
		return errors.New("source tree digest mismatch")
	}
	for _, file := range spec.Source.DependencyFiles {
		if "sha256:"+pairT3FileDigest(filepath.Join(cwd, file.Path)) != file.SHA256 {
			return fmt.Errorf("dependency digest mismatch: %s", file.Path)
		}
	}
	return nil
}

func pairT3VerifyTrackedSource(spec pairT3Spec, cwd string) error {
	env := []string{"PATH=" + os.Getenv("PATH"), "HOME=" + os.Getenv("HOME"), "GIT_CONFIG_NOSYSTEM=1", "GIT_CONFIG_GLOBAL=" + os.DevNull, "GIT_TERMINAL_PROMPT=0"}
	readRevision := func(args ...string) (string, error) {
		cmd := exec.Command(spec.Toolchain.Git.Path, args...)
		cmd.Env = env
		output, err := cmd.Output()
		if err != nil || len(output) > 256 {
			return "", errors.New("git revision query failed")
		}
		return strings.TrimSpace(string(output)), nil
	}
	expectedTree, err := readRevision("--git-dir", spec.Source.BareRepository, "rev-parse", spec.Source.Revision+"^{tree}")
	if err != nil {
		return err
	}
	actualRevision, err := readRevision("-C", cwd, "rev-parse", "HEAD")
	if err != nil || actualRevision != spec.Source.Revision {
		return errors.New("checkout revision changed")
	}
	actualTree, err := readRevision("-C", cwd, "rev-parse", "HEAD^{tree}")
	if err != nil || actualTree != expectedTree {
		return errors.New("checkout tree changed")
	}
	for _, args := range [][]string{
		{"-C", cwd, "diff", "--quiet", "--no-ext-diff", "--"},
		{"-C", cwd, "diff", "--cached", "--quiet", "--no-ext-diff", "--"},
	} {
		cmd := exec.Command(spec.Toolchain.Git.Path, args...)
		cmd.Env = env
		if err := cmd.Run(); err != nil {
			return errors.New("tracked checkout content changed")
		}
	}
	return nil
}

func pairT3PrepareFrontend(spec pairT3Spec, cwd, store, corepackHome string) error {
	env := []string{"PATH=" + pairT3FrozenPath(spec), "HOME=" + os.Getenv("HOME"), "COREPACK_HOME=" + corepackHome, "COREPACK_ENABLE_NETWORK=0", "COREPACK_DEFAULT_TO_LATEST=0", "COREPACK_ENABLE_DOWNLOAD_PROMPT=0", "npm_config_cache=" + filepath.Join(cwd, ".capability-cache", "npm")}
	version := exec.Command(spec.Toolchain.Pnpm.Path, "--version")
	version.Dir = filepath.Join(cwd, "web")
	version.Env = env
	output, err := version.CombinedOutput()
	if err != nil || strings.TrimSpace(string(output)) != spec.PnpmVersion {
		return fmt.Errorf("private Corepack pnpm version mismatch: %w", err)
	}
	cmd := exec.Command(spec.Toolchain.Pnpm.Path, "install", "--offline", "--frozen-lockfile", "--store-dir", store)
	cmd.Dir = filepath.Join(cwd, "web")
	cmd.Env = env
	output, err = cmd.CombinedOutput()
	if err != nil {
		return fmt.Errorf("offline pnpm install failed: %w (%d bytes)", err, len(output))
	}
	return nil
}

func pairT3FrozenPath(spec pairT3Spec) string {
	return strings.Join([]string{filepath.Dir(spec.Toolchain.Node.Path), "/usr/bin", "/bin"}, string(os.PathListSeparator))
}

func pairT3RequirePnpmStoreSeed(root string) error {
	_, err := os.Lstat(filepath.Join(root, "v10", "projects"))
	if err == nil {
		return errors.New("pnpm store seed must exclude v10/projects")
	}
	if !errors.Is(err, os.ErrNotExist) {
		return err
	}
	return nil
}

func writeT3CapabilityWrapper(t *testing.T, root, realCodex, providerURL, frozenPath string, workspaces [2]pairWorkspace) string {
	t.Helper()
	q := pairShellQuote
	wrapper := filepath.Join(root, "codex-capability-t3-isolated")
	body := "#!/bin/sh\nset -eu\ncase \"${1:-}\" in\n" +
		"exec) role=direct; workspace=" + q(workspaces[0].cwd) + ";;\n" +
		"app-server) role=daemon; workspace=" + q(workspaces[1].cwd) + ";;\n" +
		"*) role=other; workspace=" + q(workspaces[0].cwd) + ";; esac\n" +
		"if [ \"$role\" != other ]; then start=$(/bin/ps -p \"$$\" -o lstart=); printf '%s\\n%s\\n%s\\n' \"$$\" \"$start\" \"$role\" > " + q(filepath.Join(root, "processes")) + "/$$; fi\n" +
		"exec env -i PATH=" + q(frozenPath) + " HOME=\"$HOME\" CODEX_HOME=\"$CODEX_HOME\" TMPDIR=\"$TMPDIR\" XDG_CONFIG_HOME=\"${XDG_CONFIG_HOME:-}\" XDG_DATA_HOME=\"${XDG_DATA_HOME:-}\" XDG_STATE_HOME=\"${XDG_STATE_HOME:-}\" XDG_CACHE_HOME=\"${XDG_CACHE_HOME:-}\" COREPACK_HOME=\"$workspace/.capability-cache/corepack\" COREPACK_ENABLE_NETWORK=0 COREPACK_DEFAULT_TO_LATEST=0 COREPACK_ENABLE_DOWNLOAD_PROMPT=0 GOMODCACHE=\"$workspace/.capability-cache/go-mod\" GOCACHE=\"$workspace/.capability-cache/go-build\" npm_config_cache=\"$workspace/.capability-cache/npm\" GIT_CONFIG_NOSYSTEM=1 GIT_CONFIG_GLOBAL=" + q(os.DevNull) + " HTTP_PROXY=" + q(providerURL) + " HTTPS_PROXY=" + q(providerURL) + " ALL_PROXY=" + q(providerURL) + " NO_PROXY='127.0.0.1,localhost,::1' OTEL_SDK_DISABLED=true " + q(realCodex) + " \"$@\"\n"
	writePairFile(t, wrapper, body)
	if err := os.Chmod(wrapper, 0o700); err != nil {
		t.Fatal(err)
	}
	return wrapper
}

func pairT3HashTree(spec pairT3Spec, root string) (pairT3Directory, error) {
	helper := ""
	for _, source := range spec.Runner.Sources {
		if filepath.Base(source.Path) == "native-capture.mjs" {
			if helper != "" {
				return pairT3Directory{}, errors.New("native capture source lock is ambiguous")
			}
			helper = source.Path
		}
	}
	if helper == "" {
		return pairT3Directory{}, errors.New("native capture source lock is missing")
	}
	return pairT3HashTreeWithNode(spec.Toolchain.Node.Path, helper, root)
}

func pairT3HashTreeWithNode(nodePath, helper, root string) (pairT3Directory, error) {
	root, err := filepath.EvalSymlinks(root)
	if err != nil || !filepath.IsAbs(root) {
		return pairT3Directory{}, errors.New("cache root is invalid")
	}
	script := `const {hashDirectoryTree}=await import(process.argv[1]); const value=await hashDirectoryTree(process.argv[2],{max_files:200000,max_entries:400000,max_file_bytes:67108864,max_total_bytes:536870912}); process.stdout.write(JSON.stringify(value));`
	ctx, cancel := context.WithTimeout(context.Background(), 45*time.Second)
	defer cancel()
	cmd := exec.CommandContext(ctx, nodePath, "--input-type=module", "--eval", script, helper, root)
	cmd.Env = []string{"HOME=" + os.Getenv("HOME"), "TMPDIR=" + os.Getenv("TMPDIR"), "LANG=" + os.Getenv("LANG"), "LC_ALL=" + os.Getenv("LC_ALL"), "LC_CTYPE=" + os.Getenv("LC_CTYPE")}
	cmd.SysProcAttr = &syscall.SysProcAttr{Setpgid: true}
	output, runErr, gone := pairRunOwnedCommand(cmd)
	if runErr != nil || !gone || ctx.Err() != nil || len(output) > 4096 {
		return pairT3Directory{}, errors.New("bounded Node cache hash failed")
	}
	var observed struct {
		SHA256    string `json:"sha256"`
		FileCount int    `json:"file_count"`
		Bytes     int64  `json:"bytes"`
	}
	decoder := json.NewDecoder(bytes.NewReader(output))
	decoder.DisallowUnknownFields()
	if decoder.Decode(&observed) != nil || decoder.Decode(&struct{}{}) != io.EOF || !pairT3SHA256.MatchString(observed.SHA256) || observed.FileCount < 0 || observed.FileCount > pairT3MaxCacheFiles || observed.Bytes < 0 || observed.Bytes > pairT3MaxCacheBytes {
		return pairT3Directory{}, errors.New("bounded Node cache hash output is invalid")
	}
	return pairT3Directory{Path: root, SHA256: observed.SHA256, FileCount: observed.FileCount, Bytes: observed.Bytes}, nil
}

func pairT3CopyTree(source, destination string) error {
	var files int
	var bytes int64
	return filepath.WalkDir(source, func(file string, entry os.DirEntry, walkErr error) error {
		if walkErr != nil {
			return walkErr
		}
		if entry.Type()&os.ModeSymlink != 0 {
			return errors.New("cache seed contains a symlink")
		}
		relative, err := filepath.Rel(source, file)
		if err != nil {
			return err
		}
		target := filepath.Join(destination, relative)
		if entry.IsDir() {
			return os.MkdirAll(target, 0o700)
		}
		if !entry.Type().IsRegular() {
			return errors.New("cache seed contains unsupported entry")
		}
		files++
		if files > pairT3MaxCacheFiles {
			return errors.New("cache seed file limit exceeded")
		}
		input, err := os.Open(file)
		if err != nil {
			return err
		}
		before, err := input.Stat()
		if err != nil || !before.Mode().IsRegular() || before.Size() > pairT3MaxCacheFileBytes || bytes+before.Size() > pairT3MaxCacheBytes {
			_ = input.Close()
			return errors.New("cache seed file exceeds bound")
		}
		output, err := os.OpenFile(target, os.O_WRONLY|os.O_CREATE|os.O_EXCL, 0o600)
		if err != nil {
			_ = input.Close()
			return err
		}
		copied, copyErr := io.CopyN(output, input, before.Size())
		extra := make([]byte, 1)
		extraCount, extraErr := input.Read(extra)
		after, statErr := input.Stat()
		inputErr, outputErr := input.Close(), output.Close()
		if copyErr != nil || copied != before.Size() || extraCount != 0 || extraErr != io.EOF || statErr != nil || !os.SameFile(before, after) || before.Size() != after.Size() || !before.ModTime().Equal(after.ModTime()) {
			return errors.New("cache seed changed while copying")
		}
		bytes += copied
		if inputErr != nil {
			return inputErr
		}
		return outputErr
	})
}

func pairT3ToolOutputProof(request map[string]any) (bool, string) {
	return pairTaskToolOutputProof(request, "T3")
}

func pairTaskToolOutputProof(request map[string]any, taskID string) (bool, string) {
	if taskID != "T3" && taskID != "T4" {
		return false, ""
	}
	input, _ := request["input"].([]any)
	var markers []string
	exitZero := false
	pattern := regexp.MustCompile(taskID + `_CAPABILITY_PROBE_RECEIPT (sha256:[a-f0-9]{64})`)
	for _, raw := range input {
		item, _ := raw.(map[string]any)
		if item["type"] != "function_call_output" {
			continue
		}
		output := fmt.Sprint(item["output"])
		for _, match := range pattern.FindAllStringSubmatch(output, -1) {
			markers = append(markers, match[1])
		}
		exitZero = exitZero || strings.Contains(output, "Process exited with code 0") || strings.Contains(output, `"exit_code":0`)
	}
	if len(markers) != 1 || !exitZero {
		return false, ""
	}
	return true, markers[0]
}

func pairValidateTaskCapabilityRegistration(data []byte, taskID string) error {
	var registration struct {
		SchemaVersion string `json:"schema_version"`
		Schedule      struct {
			TaskIDs []string `json:"task_ids"`
		} `json:"schedule"`
	}
	if err := json.Unmarshal(data, &registration); err != nil {
		return err
	}
	if taskID != "T3" && taskID != "T4" || !slices.Equal(registration.Schedule.TaskIDs, []string{taskID}) ||
		registration.SchemaVersion != "treatment-aware-task-scoped/v2" && (taskID != "T3" || registration.SchemaVersion != "treatment-aware-task-scoped/v1") {
		return errors.New("capability registration task binding mismatch")
	}
	return nil
}

func pairT3Digest(data []byte) string {
	sum := sha256.Sum256(data)
	return hex.EncodeToString(sum[:])
}

func pairT3FileDigest(path string) string {
	data, err := pairT3ReadStable(path, pairT3MaxLockedFileBytes)
	if err != nil {
		return ""
	}
	return pairT3Digest(data)
}

func pairT3ReadStable(path string, maxBytes int64) ([]byte, error) {
	file, err := os.Open(path)
	if err != nil {
		return nil, err
	}
	defer file.Close()
	before, err := file.Stat()
	if err != nil || !before.Mode().IsRegular() || before.Size() > maxBytes {
		return nil, errors.New("bounded file is invalid")
	}
	data, err := io.ReadAll(io.LimitReader(file, maxBytes+1))
	if err != nil || int64(len(data)) != before.Size() {
		return nil, errors.New("bounded file read is incomplete")
	}
	after, err := file.Stat()
	if err != nil || !os.SameFile(before, after) || before.Size() != after.Size() || !before.ModTime().Equal(after.ModTime()) {
		return nil, errors.New("bounded file changed while reading")
	}
	return data, nil
}
