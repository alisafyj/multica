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
	"net"
	"net/http"
	"net/http/httptest"
	"net/url"
	"os"
	"os/exec"
	"path/filepath"
	"reflect"
	"sort"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/multica-ai/multica/server/internal/daemon/repocache"
)

type claudeOrdinaryRepositorySpec struct {
	Path                string `json:"path"`
	Revision            string `json:"revision"`
	SetupCache          bool   `json:"setup_cache,omitempty"`
	SetupPNPM           bool   `json:"setup_pnpm,omitempty"`
	GuardedIssueOutcome bool   `json:"guarded_issue_outcome,omitempty"`
}

func (s claudeOrdinaryRepositorySpec) validateSetup() error {
	if s.SetupPNPM && !s.SetupCache {
		return errors.New("repository.setup_pnpm requires repository.setup_cache")
	}
	if s.GuardedIssueOutcome && (!s.SetupCache || !s.SetupPNPM) {
		return errors.New("repository.guarded_issue_outcome requires repository.setup_cache and repository.setup_pnpm")
	}
	return nil
}

type claudeOrdinaryProbeSpec struct {
	Schema          string `json:"schema"`
	Root            string `json:"root"`
	WorkDir         string `json:"workdir"`
	SettingsFile    string `json:"settings_file"`
	ConfigDirectory string `json:"config_directory"`
	LogPath         string `json:"log_path"`
	ProviderURL     string `json:"provider_url"`
	Binary          struct {
		Path    string `json:"path"`
		SHA256  string `json:"sha256"`
		Version string `json:"version"`
	} `json:"binary"`
	TestBinary *struct {
		Path    string `json:"path"`
		SHA256  string `json:"sha256"`
		Version string `json:"version"`
	} `json:"test_binary,omitempty"`
	Environment    map[string]string             `json:"environment"`
	RequestedModel string                        `json:"requested_model"`
	DeclaredBudget *float64                      `json:"declared_budget_usd,omitempty"`
	Repository     *claudeOrdinaryRepositorySpec `json:"repository,omitempty"`
}

func claudeOrdinaryCapabilityBudgetUSD(declared *float64) (float64, error) {
	if declared == nil {
		return 1, nil
	}
	if *declared != 3 {
		return 0, fmt.Errorf("registered ordinary probe declared budget must equal USD 3")
	}
	return *declared, nil
}

func TestClaudeOrdinaryRepositorySpecGuardedIssueOutcomeValidation(t *testing.T) {
	tests := []struct {
		name    string
		raw     string
		wantErr string
	}{
		{name: "missing preserves legacy", raw: `{}`},
		{name: "false preserves legacy", raw: `{"guarded_issue_outcome":false}`},
		{name: "setup pnpm still requires setup cache", raw: `{"setup_pnpm":true}`, wantErr: "repository.setup_pnpm requires repository.setup_cache"},
		{name: "setup cache and pnpm preserve legacy", raw: `{"setup_cache":true,"setup_pnpm":true}`},
		{name: "guarded requires setup cache", raw: `{"guarded_issue_outcome":true}`, wantErr: "repository.guarded_issue_outcome requires repository.setup_cache and repository.setup_pnpm"},
		{name: "guarded requires setup pnpm", raw: `{"setup_cache":true,"guarded_issue_outcome":true}`, wantErr: "repository.guarded_issue_outcome requires repository.setup_cache and repository.setup_pnpm"},
		{name: "guarded accepts full setup", raw: `{"setup_cache":true,"setup_pnpm":true,"guarded_issue_outcome":true}`},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			var spec claudeOrdinaryRepositorySpec
			if err := json.Unmarshal([]byte(test.raw), &spec); err != nil {
				t.Fatal(err)
			}
			err := spec.validateSetup()
			if test.wantErr == "" {
				if err != nil {
					t.Fatalf("validate setup: %v", err)
				}
				return
			}
			if err == nil || err.Error() != test.wantErr {
				t.Fatalf("validate setup error=%v, want %q", err, test.wantErr)
			}
		})
	}
}

type claudeOrdinarySyncWriter struct{ file *os.File }

func (w claudeOrdinarySyncWriter) Write(data []byte) (int, error) {
	n, err := w.file.Write(data)
	if err != nil {
		return n, err
	}
	if err := w.file.Sync(); err != nil {
		return n, err
	}
	return n, nil
}

func newClaudeOrdinaryControlHandler(
	taskID, runtimeID string,
	repository *claudeOrdinaryRepositorySpec,
	guardedIssueOutcome bool,
	record func(key string, accepted bool),
) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		key := r.Method + " " + r.URL.Path
		if repository != nil && key == http.MethodGet+" /api/daemon/workspaces/claude-ordinary-workspace/repos" {
			record(key, true)
			_ = json.NewEncoder(w).Encode(WorkspaceReposResponse{
				WorkspaceID: "claude-ordinary-workspace",
				Repos:       []RepoData{{URL: repository.Path, Ref: repository.Revision}},
			})
			return
		}
		expected := map[string]bool{
			http.MethodPost + " /api/daemon/runtimes/" + runtimeID + "/tasks/" + taskID + "/prepare-lease": true,
			http.MethodPost + " /api/daemon/tasks/" + taskID + "/start":                                    true,
			http.MethodPost + " /api/daemon/tasks/" + taskID + "/progress":                                 true,
			http.MethodPost + " /api/daemon/tasks/" + taskID + "/messages":                                 true,
			http.MethodPost + " /api/daemon/tasks/" + taskID + "/session":                                  true,
		}
		if !expected[key] {
			record(key, false)
			http.Error(w, "unexpected diagnostic control request", http.StatusNotFound)
			return
		}
		record(key, true)
		_, _ = io.Copy(io.Discard, io.LimitReader(r.Body, 2<<20))
		if guardedIssueOutcome && key == http.MethodPost+" /api/daemon/tasks/"+taskID+"/start" {
			_ = json.NewEncoder(w).Encode(map[string]any{"issue_start": IssueStartState{
				Version: 1, BaselineAccepted: true, Status: "in_progress", Revision: 7,
				ETag: `W/"issue:claude-ordinary-issue:7"`, UpdatedAt: time.Now().UTC().Format(time.RFC3339),
			}})
			return
		}
		w.WriteHeader(http.StatusOK)
	})
}

func TestClaudeOrdinaryControlHandlerPrepareLeaseRoute(t *testing.T) {
	const taskID = "88888888-8888-4888-8888-888888888888"
	const runtimeID = "99999999-9999-4999-8999-999999999999"

	tests := []struct {
		name, method, path string
		wantStatus         int
		wantAccepted       bool
	}{
		{
			name: "exact prepare lease", method: http.MethodPost,
			path:         "/api/daemon/runtimes/" + runtimeID + "/tasks/" + taskID + "/prepare-lease",
			wantStatus:   http.StatusOK,
			wantAccepted: true,
		},
		{
			name: "wrong method", method: http.MethodGet,
			path:       "/api/daemon/runtimes/" + runtimeID + "/tasks/" + taskID + "/prepare-lease",
			wantStatus: http.StatusNotFound,
		},
		{
			name: "wrong runtime", method: http.MethodPost,
			path:       "/api/daemon/runtimes/00000000-0000-4000-8000-000000000000/tasks/" + taskID + "/prepare-lease",
			wantStatus: http.StatusNotFound,
		},
		{
			name: "wrong task", method: http.MethodPost,
			path:       "/api/daemon/runtimes/" + runtimeID + "/tasks/00000000-0000-4000-8000-000000000000/prepare-lease",
			wantStatus: http.StatusNotFound,
		},
		{
			name: "wrong path", method: http.MethodPost,
			path:       "/api/daemon/runtimes/" + runtimeID + "/tasks/" + taskID + "/prepare-lease/extra",
			wantStatus: http.StatusNotFound,
		},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			var recordedKey string
			var recordedAccepted bool
			handler := newClaudeOrdinaryControlHandler(taskID, runtimeID, nil, false, func(key string, accepted bool) {
				recordedKey = key
				recordedAccepted = accepted
			})
			recorder := httptest.NewRecorder()
			handler.ServeHTTP(recorder, httptest.NewRequest(test.method, test.path, strings.NewReader(`{}`)))
			if recorder.Code != test.wantStatus {
				t.Fatalf("%s %s status=%d, want %d; body=%q", test.method, test.path, recorder.Code, test.wantStatus, recorder.Body.String())
			}
			if recordedKey != test.method+" "+test.path || recordedAccepted != test.wantAccepted {
				t.Fatalf("recorded key=%q accepted=%t, want key=%q accepted=%t", recordedKey, recordedAccepted, test.method+" "+test.path, test.wantAccepted)
			}
		})
	}
}

// TestClaudeOrdinaryCapabilityRealCLI is an opt-in diagnostic for one actual
// ordinary daemon.runTask Claude turn. It is not a T3/T4 admission receipt.
func TestClaudeOrdinaryCapabilityRealCLI(t *testing.T) {
	if os.Getenv("MULTICA_TEST_REAL_CLAUDE_ORDINARY") != "1" {
		t.Skip("set MULTICA_TEST_REAL_CLAUDE_ORDINARY=1 and MULTICA_TEST_CLAUDE_ORDINARY_SPEC to run the isolated loopback diagnostic")
	}

	specPath := strings.TrimSpace(os.Getenv("MULTICA_TEST_CLAUDE_ORDINARY_SPEC"))
	if !filepath.IsAbs(specPath) {
		t.Fatal("MULTICA_TEST_CLAUDE_ORDINARY_SPEC must be an explicit absolute path")
	}
	specFile, err := os.Open(specPath)
	if err != nil {
		t.Fatal(err)
	}
	specInfo, statErr := specFile.Stat()
	if statErr != nil || !specInfo.Mode().IsRegular() || specInfo.Mode().Perm()&0o077 != 0 {
		_ = specFile.Close()
		t.Fatalf("ordinary probe spec must be a private regular file: %v", statErr)
	}
	var spec claudeOrdinaryProbeSpec
	decoder := json.NewDecoder(io.LimitReader(specFile, 1<<20))
	decoder.DisallowUnknownFields()
	decodeErr := decoder.Decode(&spec)
	var trailing any
	trailingErr := decoder.Decode(&trailing)
	closeErr := specFile.Close()
	if decodeErr != nil || !errors.Is(trailingErr, io.EOF) || closeErr != nil {
		t.Fatalf("invalid ordinary probe spec: decode=%v trailing=%v close=%v", decodeErr, trailingErr, closeErr)
	}
	if spec.Schema != "claude_ordinary_probe/v1" || spec.RequestedModel == "" || len(spec.Environment) == 0 {
		t.Fatal("ordinary probe spec has an unsupported schema or missing required values")
	}
	budgetUSD, err := claudeOrdinaryCapabilityBudgetUSD(spec.DeclaredBudget)
	if err != nil {
		t.Fatal(err)
	}
	var testBinaryIdentity map[string]any
	if spec.TestBinary != nil {
		const testArtifactVersion = "go-test-artifact:github.com/multica-ai/multica/server/internal/daemon"
		executable, executableErr := os.Executable()
		resolvedExecutable, resolveErr := filepath.EvalSymlinks(executable)
		if executableErr != nil || resolveErr != nil || resolvedExecutable != spec.TestBinary.Path ||
			spec.TestBinary.Version != testArtifactVersion {
			t.Fatalf("ordinary Go test executable identity mismatch: executable=%q resolved=%q version=%q executable_error=%v resolve_error=%v",
				executable, resolvedExecutable, spec.TestBinary.Version, executableErr, resolveErr)
		}
		executableInfo, statErr := os.Lstat(resolvedExecutable)
		if statErr != nil || !executableInfo.Mode().IsRegular() || executableInfo.Mode()&os.ModeSymlink != 0 {
			t.Fatalf("ordinary Go test executable must be a real regular file: %v", statErr)
		}
		executableFile, openErr := os.Open(resolvedExecutable)
		if openErr != nil {
			t.Fatal(openErr)
		}
		executableHash := sha256.New()
		_, copyErr := io.Copy(executableHash, executableFile)
		closeErr := executableFile.Close()
		actualHash := fmt.Sprintf("sha256:%x", executableHash.Sum(nil))
		if copyErr != nil || closeErr != nil || actualHash != spec.TestBinary.SHA256 {
			t.Fatalf("ordinary Go test executable hash mismatch: match=%t copy=%v close=%v",
				actualHash == spec.TestBinary.SHA256, copyErr, closeErr)
		}
		testBinaryIdentity = map[string]any{
			"identity_observed": true,
			"path":              resolvedExecutable,
			"sha256":            actualHash,
			"version":           testArtifactVersion,
		}
	}

	root, err := filepath.EvalSymlinks(spec.Root)
	if err != nil || !filepath.IsAbs(spec.Root) || root != spec.Root {
		t.Fatalf("ordinary probe root must be an actual resolved absolute path: resolved=%q error=%v", root, err)
	}
	rootInfo, err := os.Lstat(root)
	if err != nil || !rootInfo.IsDir() || rootInfo.Mode()&os.ModeSymlink != 0 || rootInfo.Mode().Perm()&0o077 != 0 {
		t.Fatalf("ordinary probe root must be a private real directory: %v", err)
	}
	insideRoot := func(path string) bool {
		if !filepath.IsAbs(path) {
			return false
		}
		rel, err := filepath.Rel(root, path)
		return err == nil && rel != "." && rel != ".." && !strings.HasPrefix(rel, ".."+string(filepath.Separator))
	}
	requireResolved := func(path string, directory bool) {
		t.Helper()
		if !insideRoot(path) {
			t.Fatalf("probe path escapes owned root: %s", path)
		}
		resolved, err := filepath.EvalSymlinks(path)
		if err != nil || resolved != path {
			t.Fatalf("probe path must be actual and resolved: path=%s resolved=%s error=%v", path, resolved, err)
		}
		info, err := os.Lstat(path)
		if err != nil || info.Mode()&os.ModeSymlink != 0 || directory != info.IsDir() {
			t.Fatalf("probe path has wrong type: path=%s directory=%t error=%v", path, directory, err)
		}
	}
	requireResolved(spec.WorkDir, true)
	requireResolved(spec.ConfigDirectory, true)
	requireResolved(spec.SettingsFile, false)
	settingsData, err := os.ReadFile(spec.SettingsFile)
	if err != nil {
		t.Fatal(err)
	}
	settingsInfo, err := os.Lstat(spec.SettingsFile)
	if err != nil || settingsInfo.Mode().Perm()&0o077 != 0 {
		t.Fatalf("settings_file must be private: %v", err)
	}
	if spec.Environment["CLAUDE_CONFIG_DIR"] != spec.ConfigDirectory {
		t.Fatal("spec environment CLAUDE_CONFIG_DIR does not match config_directory")
	}
	if spec.Repository != nil {
		if err := spec.Repository.validateSetup(); err != nil {
			t.Fatal(err)
		}
		if spec.Repository.Path != spec.WorkDir {
			t.Fatal("repository.path must exactly match the validation workdir")
		}
		if len(spec.Repository.Revision) != 40 {
			t.Fatal("repository.revision must be a 40-character lowercase hex commit")
		}
		for _, char := range spec.Repository.Revision {
			if !strings.ContainsRune("0123456789abcdef", char) {
				t.Fatal("repository.revision must be a 40-character lowercase hex commit")
			}
		}
		isolatedSettings := filepath.Join(spec.ConfigDirectory, "settings.json")
		requireResolved(isolatedSettings, false)
		isolatedInfo, statErr := os.Lstat(isolatedSettings)
		if statErr != nil || isolatedInfo.Mode().Perm()&0o077 != 0 {
			t.Fatalf("isolated user settings must be private: %v", statErr)
		}
		isolatedData, readErr := os.ReadFile(isolatedSettings)
		if readErr != nil || !bytes.Equal(isolatedData, settingsData) {
			t.Fatalf("isolated user settings differ from settings_file: %v", readErr)
		}
	}
	for _, key := range []string{"HOME", "TMPDIR", "XDG_CONFIG_HOME", "XDG_DATA_HOME", "XDG_STATE_HOME", "XDG_CACHE_HOME"} {
		requireResolved(spec.Environment[key], true)
	}
	for _, key := range []string{"GOCACHE", "GOMODCACHE", "COREPACK_HOME"} {
		if path := spec.Environment[key]; path != "" {
			requireResolved(path, true)
		}
	}
	if !insideRoot(spec.LogPath) {
		t.Fatal("log_path escapes owned root")
	}
	resolvedLogParent, err := filepath.EvalSymlinks(filepath.Dir(spec.LogPath))
	if err != nil || filepath.Join(resolvedLogParent, filepath.Base(spec.LogPath)) != spec.LogPath {
		t.Fatalf("log_path parent must be actual and resolved: %v", err)
	}
	if _, err := os.Lstat(spec.LogPath); !errors.Is(err, os.ErrNotExist) {
		t.Fatalf("log_path must initially be absent: %v", err)
	}
	resolvedSpecPath, err := filepath.EvalSymlinks(specPath)
	if err != nil || resolvedSpecPath != specPath || !insideRoot(specPath) {
		t.Fatalf("spec file must be actual, resolved, and under root: %v", err)
	}

	providerURL, err := url.Parse(spec.ProviderURL)
	if err != nil {
		t.Fatalf("provider_url is invalid: %v", err)
	}
	providerIP := net.ParseIP(providerURL.Hostname())
	if providerURL.Scheme != "http" || providerURL.User != nil || providerURL.Port() == "" || providerURL.Path != "" ||
		providerURL.RawQuery != "" || providerURL.Fragment != "" || providerIP == nil || !providerIP.IsLoopback() {
		t.Fatal("provider_url must be an explicit HTTP loopback origin")
	}
	if spec.Environment["ANTHROPIC_BASE_URL"] != spec.ProviderURL ||
		spec.Environment["ANTHROPIC_API_KEY"] != "local-diagnostic-placeholder-not-a-credential" {
		t.Fatal("ordinary probe must use only the declared loopback provider and diagnostic placeholder")
	}
	for _, key := range []string{"ANTHROPIC_AUTH_TOKEN", "CLAUDE_CODE_OAUTH_TOKEN"} {
		if _, ok := spec.Environment[key]; ok {
			t.Fatalf("ordinary probe environment contains forbidden auth key %s", key)
		}
	}
	allowedEnvironment := map[string]bool{
		"ALL_PROXY": true, "ANTHROPIC_API_KEY": true, "ANTHROPIC_BASE_URL": true,
		"ANTHROPIC_MODEL": true, "ANTHROPIC_DEFAULT_FABLE_MODEL": true, "ANTHROPIC_DEFAULT_OPUS_MODEL": true,
		"ANTHROPIC_DEFAULT_SONNET_MODEL": true, "ANTHROPIC_DEFAULT_HAIKU_MODEL": true,
		"CLAUDE_CODE_SUBAGENT_MODEL": true, "CLAUDE_CODE_SUBAGENT_MODEL_FORCE": true,
		"CLAUDE_CODE_DISABLE_AUTO_MEMORY": true, "CLAUDE_CODE_DISABLE_NONESSENTIAL_TRAFFIC": true,
		"CLAUDE_CODE_SUBPROCESS_ENV_SCRUB": true, "CLAUDE_CONFIG_DIR": true,
		"DISABLE_AUTOUPDATER": true, "DISABLE_ERROR_REPORTING": true, "DISABLE_TELEMETRY": true,
		"ENABLE_TOOL_SEARCH": true, "GIT_CONFIG_GLOBAL": true, "GIT_CONFIG_NOSYSTEM": true,
		"GIT_TERMINAL_PROMPT": true, "GOCACHE": true, "GOMODCACHE": true, "COREPACK_HOME": true, "COREPACK_ENABLE_NETWORK": true,
		"GOPROXY": true, "GOSUMDB": true, "GOTOOLCHAIN": true,
		"HOME": true, "HTTP_PROXY": true, "HTTPS_PROXY": true, "LANG": true, "LC_ALL": true,
		"LC_CTYPE": true, "NO_PROXY": true, "PATH": true, "TERM": true, "TMPDIR": true, "TZ": true,
		"XDG_CACHE_HOME": true, "XDG_CONFIG_HOME": true, "XDG_DATA_HOME": true, "XDG_STATE_HOME": true,
	}
	for key := range spec.Environment {
		if !allowedEnvironment[key] {
			t.Fatalf("ordinary probe environment contains undeclared key %s", key)
		}
	}
	if model, supplied := spec.Environment["ANTHROPIC_MODEL"]; supplied {
		if model != spec.RequestedModel {
			t.Fatal("ordinary probe model selector differs from requested model")
		}
		for _, key := range []string{"ANTHROPIC_DEFAULT_FABLE_MODEL", "ANTHROPIC_DEFAULT_OPUS_MODEL",
			"ANTHROPIC_DEFAULT_SONNET_MODEL", "ANTHROPIC_DEFAULT_HAIKU_MODEL", "CLAUDE_CODE_SUBAGENT_MODEL"} {
			if value := spec.Environment[key]; value == "" || len(value) > 256 {
				t.Fatalf("ordinary probe model selector %s must be explicit and bounded", key)
			}
		}
		if force := spec.Environment["CLAUDE_CODE_SUBAGENT_MODEL_FORCE"]; force != "0" && force != "1" {
			t.Fatal("ordinary probe subagent model force must be explicit")
		}
	} else {
		for _, key := range []string{"ANTHROPIC_DEFAULT_FABLE_MODEL", "ANTHROPIC_DEFAULT_OPUS_MODEL",
			"ANTHROPIC_DEFAULT_SONNET_MODEL", "ANTHROPIC_DEFAULT_HAIKU_MODEL", "CLAUDE_CODE_SUBAGENT_MODEL", "CLAUDE_CODE_SUBAGENT_MODEL_FORCE"} {
			if _, supplied := spec.Environment[key]; supplied {
				t.Fatal("ordinary probe model policy must be supplied as a complete inventory")
			}
		}
	}
	if value, set := spec.Environment["COREPACK_ENABLE_NETWORK"]; set && value != "0" {
		t.Fatal("ordinary probe Corepack network must be disabled")
	}
	if spec.Repository != nil && spec.Repository.SetupPNPM {
		goProxy, parseErr := url.Parse(spec.Environment["GOPROXY"])
		expectedPath := filepath.Join(root, "task-cache", "go-mod", "cache", "download")
		expectedURL := (&url.URL{Scheme: "file", Path: expectedPath}).String()
		if parseErr != nil || goProxy.Scheme != "file" || goProxy.Host != "" || goProxy.Opaque != "" ||
			goProxy.RawQuery != "" || goProxy.Fragment != "" || goProxy.Path != expectedPath || goProxy.String() != expectedURL {
			t.Fatalf("ordinary full setup GOPROXY must be the canonical owned module proxy: %v", parseErr)
		}
		requireResolved(expectedPath, true)
	} else if spec.Environment["GOPROXY"] != "off" {
		t.Fatal("ordinary probe GOPROXY must be disabled outside full setup")
	}
	for key, value := range map[string]string{
		"CLAUDE_CODE_DISABLE_AUTO_MEMORY": "1", "CLAUDE_CODE_DISABLE_NONESSENTIAL_TRAFFIC": "1",
		"CLAUDE_CODE_SUBPROCESS_ENV_SCRUB": "1", "DISABLE_AUTOUPDATER": "1",
		"DISABLE_ERROR_REPORTING": "1", "DISABLE_TELEMETRY": "1", "ENABLE_TOOL_SEARCH": "false",
		"GIT_CONFIG_NOSYSTEM": "1", "GIT_TERMINAL_PROMPT": "0", "GOSUMDB": "off",
		"GOTOOLCHAIN": "local",
	} {
		if spec.Environment[key] != value {
			t.Fatalf("ordinary probe environment control %s is not frozen", key)
		}
	}
	for _, key := range []string{"HTTP_PROXY", "HTTPS_PROXY", "ALL_PROXY"} {
		if spec.Environment[key] != spec.ProviderURL {
			t.Fatalf("ordinary probe proxy %s is not the loopback provider", key)
		}
	}
	expectedNoProxy := "127.0.0.1,localhost,::1"
	if spec.Repository != nil && spec.Repository.SetupPNPM {
		expectedNoProxy += ",registry.npmjs.org"
	}
	if spec.Environment["NO_PROXY"] != expectedNoProxy {
		t.Fatal("ordinary probe NO_PROXY is not frozen")
	}

	if !filepath.IsAbs(spec.Binary.Path) {
		t.Fatal("binary.path must be an explicit absolute path")
	}
	resolvedBinary, err := filepath.EvalSymlinks(spec.Binary.Path)
	if err != nil || resolvedBinary != spec.Binary.Path {
		t.Fatalf("binary.path must be the exact pinned real path: resolved=%q error=%v", resolvedBinary, err)
	}
	binaryInfo, err := os.Lstat(resolvedBinary)
	if err != nil || !binaryInfo.Mode().IsRegular() || binaryInfo.Mode()&os.ModeSymlink != 0 {
		t.Fatalf("binary.path must be a real regular file: %v", err)
	}
	binaryFile, err := os.Open(resolvedBinary)
	if err != nil {
		t.Fatal(err)
	}
	hash := sha256.New()
	_, copyErr := io.Copy(hash, binaryFile)
	closeErr = binaryFile.Close()
	actualHash := fmt.Sprintf("sha256:%x", hash.Sum(nil))
	if copyErr != nil || closeErr != nil || actualHash != spec.Binary.SHA256 {
		t.Fatalf("pinned Claude binary hash mismatch: match=%t copy=%v close=%v", actualHash == spec.Binary.SHA256, copyErr, closeErr)
	}
	versionEnvironmentKeys := make([]string, 0, len(spec.Environment))
	for key := range spec.Environment {
		versionEnvironmentKeys = append(versionEnvironmentKeys, key)
	}
	sort.Strings(versionEnvironmentKeys)
	versionEnvironment := make([]string, 0, len(versionEnvironmentKeys))
	for _, key := range versionEnvironmentKeys {
		versionEnvironment = append(versionEnvironment, key+"="+spec.Environment[key])
	}
	versionCtx, versionCancel := context.WithTimeout(t.Context(), 3*time.Second)
	versionCommand := exec.CommandContext(versionCtx, resolvedBinary, "--version")
	versionCommand.Env = versionEnvironment
	versionOutput, versionErr := versionCommand.Output()
	versionCancel()
	if versionErr != nil || strings.TrimSpace(string(versionOutput)) != spec.Binary.Version {
		t.Fatalf("pinned Claude binary version mismatch: match=%t error=%v", strings.TrimSpace(string(versionOutput)) == spec.Binary.Version, versionErr)
	}

	for _, entry := range os.Environ() {
		key, _, _ := strings.Cut(entry, "=")
		if _, declared := spec.Environment[key]; !declared {
			t.Setenv(key, "")
			if err := os.Unsetenv(key); err != nil {
				t.Fatalf("clear ambient environment key %s: %v", key, err)
			}
		}
	}
	for key, value := range spec.Environment {
		t.Setenv(key, value)
	}
	if spec.Repository != nil {
		rootPath, gitErr := runGitCommandContext(t.Context(), spec.Repository.Path, gitCmdTimeout, "rev-parse", "--show-toplevel")
		if gitErr != nil {
			t.Fatalf("repository.path is not a readable local Git repository: %v", gitErr)
		}
		resolvedGitRoot, resolveErr := filepath.EvalSymlinks(filepath.Clean(filepath.FromSlash(rootPath)))
		if resolveErr != nil || resolvedGitRoot != spec.Repository.Path {
			t.Fatalf("repository.path is not the exact Git root: resolved=%q error=%v", resolvedGitRoot, resolveErr)
		}
		revision, gitErr := runGitCommandContext(t.Context(), spec.Repository.Path, gitCmdTimeout, "rev-parse", "HEAD")
		if gitErr != nil || revision != spec.Repository.Revision {
			t.Fatalf("repository seed revision mismatch: actual=%q error=%v", revision, gitErr)
		}
	}

	const taskID = "88888888-8888-4888-8888-888888888888"
	const runtimeID = "99999999-9999-4999-8999-999999999999"
	guardedIssueOutcome := spec.Repository != nil && spec.Repository.GuardedIssueOutcome
	var controlMu sync.Mutex
	var unexpected []string
	seen := make(map[string]int)
	control := httptest.NewServer(newClaudeOrdinaryControlHandler(taskID, runtimeID, spec.Repository, guardedIssueOutcome, func(key string, accepted bool) {
		controlMu.Lock()
		defer controlMu.Unlock()
		if accepted {
			seen[key]++
		} else {
			unexpected = append(unexpected, key)
		}
	}))
	defer control.Close()

	logFile, err := os.OpenFile(spec.LogPath, os.O_CREATE|os.O_EXCL|os.O_WRONLY, 0o600)
	if err != nil {
		t.Fatal(err)
	}
	defer logFile.Close()
	logger := slog.New(slog.NewJSONHandler(claudeOrdinarySyncWriter{file: logFile}, nil)).With("component", "daemon")
	workspacesRoot := filepath.Join(root, "product-workspaces")
	if err := os.MkdirAll(workspacesRoot, 0o700); err != nil {
		t.Fatal(err)
	}
	cfg := Config{
		DaemonID: "claude-ordinary-daemon", Profile: "claude-ordinary-private",
		WorkspacesRoot: workspacesRoot, ServerBaseURL: control.URL,
		Agents:       map[string]AgentEntry{"claude": {Path: resolvedBinary}},
		AgentTimeout: 60 * time.Second, DirectAgentMode: false,
	}
	d := New(cfg, logger)
	d.client.SetToken("synthetic-control-token")
	d.executionEnvironmentCommand = nil
	resourceRef, err := json.Marshal(localDirectoryRef{LocalPath: spec.WorkDir, DaemonID: cfg.DaemonID})
	if err != nil {
		t.Fatal(err)
	}
	var repos []RepoData
	projectResources := []ProjectResourceData{{ResourceType: "local_directory", ResourceRef: resourceRef}}
	if spec.Repository != nil {
		repos = []RepoData{{URL: spec.Repository.Path, Ref: spec.Repository.Revision}}
		repositoryRef := map[string]any{
			"url": spec.Repository.Path, "ref": spec.Repository.Revision,
			"configuration_policy": projectConfigurationTrusted,
		}
		if spec.Repository.SetupCache {
			setup := map[string]any{"steps": []string{"go_mod_download"}, "timeout_seconds": 120}
			if spec.Repository.SetupPNPM {
				setup["steps"] = []string{"go_mod_download", "pnpm_install"}
				setup["step_directories"] = map[string]string{"pnpm_install": "web"}
			}
			repositoryRef["setup"] = setup
		}
		resourceRef, err = json.Marshal(repositoryRef)
		if err != nil {
			t.Fatal(err)
		}
		projectResources = []ProjectResourceData{{ResourceType: "github_repo", ResourceRef: resourceRef}}
		d.workspaces["claude-ordinary-workspace"] = newWorkspaceState("claude-ordinary-workspace", nil, "", repos, nil)
		cache := repocache.New(filepath.Join(root, "repository-cache"), logger)
		if err := cache.Sync("claude-ordinary-workspace", []repocache.RepoInfo{{URL: spec.Repository.Path}}); err != nil {
			t.Fatalf("prepare ordinary repository cache: %v", err)
		}
		d.repoCache = cache
	}
	task := Task{
		ID: taskID, RuntimeID: runtimeID, AgentID: "claude-ordinary-agent",
		WorkspaceID: "claude-ordinary-workspace", WorkspaceSlug: "claude-ordinary",
		IssueID: "claude-ordinary-issue", IssueIdentifier: "ORD-1",
		ClaimAttempt: 1, ClaimGeneration: 1, IssueCompletionContractVersion: 1,
		RemoteMCPDaemonToken: "mdt_synthetic_claude_ordinary",
		AuthToken:            "mat_synthetic_claude_ordinary",
		HandoffNote:          "Execute the two Bash calls supplied by the local diagnostic fixture, then finish.",
		Agent: &AgentData{
			ID: "claude-ordinary-agent", Name: "Claude ordinary diagnostic",
			Model: spec.RequestedModel, ThinkingLevel: "medium",
			CustomArgs: []string{
				"--max-budget-usd", fmt.Sprintf("%g", budgetUSD), "--no-session-persistence", "--no-chrome",
				"--setting-sources", "project", "--settings", spec.SettingsFile,
			},
			CustomEnv: spec.Environment,
			McpConfig: json.RawMessage(`{"mcpServers":{}}`),
		},
		Repos: repos, ProjectResources: projectResources,
	}
	if guardedIssueOutcome {
		task.IssueStartContractVersion = 1
		task.IssueSnapshot = freshIssueSnapshot("claude-ordinary-issue")
	}

	result, runErr := d.runTask(t.Context(), task, "claude", 0, logger)
	if runErr != nil || result.Status != "completed" || result.Comment != "diagnostic-complete" {
		t.Fatalf("ordinary Claude runTask failed: status=%q comment=%q error=%v", result.Status, result.Comment, runErr)
	}
	if result.ClaimAttempt != 1 || result.ClaimGeneration != 1 || result.IssueCompletionContractVersion != 1 {
		t.Fatalf("ordinary Claude result lost task binding: workdir=%q attempt=%d generation=%d completion=%d",
			result.WorkDir, result.ClaimAttempt, result.ClaimGeneration, result.IssueCompletionContractVersion)
	}
	actualRevision := ""
	if spec.Repository == nil {
		if result.WorkDir != spec.WorkDir {
			t.Fatalf("ordinary Claude local-directory workdir=%q, want %q", result.WorkDir, spec.WorkDir)
		}
	} else {
		if result.WorkDir == spec.Repository.Path || !insideRoot(result.WorkDir) {
			t.Fatalf("ordinary Claude repository workdir is not product-created under the owned root: %q", result.WorkDir)
		}
		requireResolved(result.WorkDir, true)
		identity, inspectErr := inspectPrimaryRepositoryGit(t.Context(), result.WorkDir, spec.Repository.Path)
		if inspectErr != nil || identity.WorkDir != result.WorkDir {
			t.Fatalf("ordinary Claude checkout identity mismatch: identity=%+v error=%v", identity, inspectErr)
		}
		actualRevision, err = runGitCommandContext(t.Context(), result.WorkDir, gitCmdTimeout, "rev-parse", "HEAD")
		if err != nil || actualRevision != spec.Repository.Revision {
			t.Fatalf("ordinary Claude checkout revision mismatch: actual=%q error=%v", actualRevision, err)
		}
		seedRevision, seedErr := runGitCommandContext(t.Context(), spec.Repository.Path, gitCmdTimeout, "rev-parse", "HEAD")
		if seedErr != nil || seedRevision != spec.Repository.Revision {
			t.Fatalf("ordinary Claude mutated the repository seed: actual=%q error=%v", seedRevision, seedErr)
		}
		isolatedData, readErr := os.ReadFile(filepath.Join(spec.ConfigDirectory, "settings.json"))
		if readErr != nil || !bytes.Equal(isolatedData, settingsData) {
			t.Fatalf("ordinary Claude changed isolated user settings: %v", readErr)
		}
	}
	control.Close()
	controlMu.Lock()
	gotUnexpected := append([]string(nil), unexpected...)
	gotSeen := make(map[string]int, len(seen))
	for key, count := range seen {
		gotSeen[key] = count
	}
	controlMu.Unlock()
	if len(gotUnexpected) != 0 || gotSeen[http.MethodPost+" /api/daemon/tasks/"+taskID+"/start"] != 1 || gotSeen[http.MethodPost+" /api/daemon/tasks/"+taskID+"/progress"] == 0 {
		t.Fatalf("unexpected diagnostic control traffic: unexpected=%v seen=%v", gotUnexpected, gotSeen)
	}
	if spec.Repository != nil && gotSeen[http.MethodGet+" /api/daemon/workspaces/claude-ordinary-workspace/repos"] != 1 {
		t.Fatalf("ordinary repository refresh count is not exact: seen=%v", gotSeen)
	}
	if err := logFile.Sync(); err != nil {
		t.Fatal(err)
	}
	logData, err := os.ReadFile(spec.LogPath)
	if err != nil || len(logData) == 0 || logData[len(logData)-1] != '\n' {
		t.Fatalf("ordinary daemon JSON log is incomplete: bytes=%d error=%v", len(logData), err)
	}
	logDecoder := json.NewDecoder(strings.NewReader(string(logData)))
	var starts, setups, authorities, launches, spans []map[string]any
	setupIndex, authorityIndex, launchIndex := -1, -1, -1
	entryIndex := 0
	for {
		var entry map[string]any
		if err := logDecoder.Decode(&entry); errors.Is(err, io.EOF) {
			break
		} else if err != nil {
			t.Fatalf("decode ordinary daemon JSON log: %v", err)
		}
		switch entry["msg"] {
		case "claude started":
			starts = append(starts, entry)
		case "claude launch configuration observed":
			launches = append(launches, entry)
			launchIndex = entryIndex
		case "claude execution span observed":
			spans = append(spans, entry)
		case "repository setup evidence":
			setups = append(setups, entry)
			setupIndex = entryIndex
		case "claude issue outcome settings authority":
			authorities = append(authorities, entry)
			authorityIndex = entryIndex
		}
		entryIndex++
	}
	if len(starts) != 1 || len(launches) != 1 || len(spans) != 1 {
		t.Fatalf("ordinary Claude lifecycle evidence is not unique: starts=%d launches=%d spans=%d", len(starts), len(launches), len(spans))
	}
	if starts[0]["cwd"] != result.WorkDir || launches[0]["work_dir"] != result.WorkDir {
		t.Fatalf("ordinary Claude launch workdir differs from result: start=%v launch=%v result=%q", starts[0]["cwd"], launches[0]["work_dir"], result.WorkDir)
	}
	guardedOutcomeGrant := ""
	if !guardedIssueOutcome {
		if len(authorities) != 0 {
			t.Fatalf("legacy ordinary run advertised guarded outcome authority: events=%d", len(authorities))
		}
	} else {
		if len(authorities) != 1 || setupIndex < 0 || authorityIndex < setupIndex || launchIndex <= authorityIndex {
			t.Fatalf("guarded outcome authority is not unique or ordered setup<=authority<launch: events=%d setup_index=%d authority_index=%d launch_index=%d",
				len(authorities), setupIndex, authorityIndex, launchIndex)
		}
		event := authorities[0]
		for field, expected := range map[string]any{
			"task_id": taskID, "runtime_id": runtimeID, "provider": "claude", "cwd": result.WorkDir,
			"claim_attempt": float64(1), "claim_generation": float64(1),
		} {
			if event[field] != expected {
				t.Fatalf("guarded outcome authority %s=%v, want %v", field, event[field], expected)
			}
		}
		eventTime, ok := event["time"].(string)
		if !ok {
			t.Fatalf("guarded outcome authority has no time: %v", event)
		}
		if _, err := time.Parse(time.RFC3339Nano, eventTime); err != nil {
			t.Fatalf("guarded outcome authority time is invalid: %q: %v", eventTime, err)
		}
		authority, ok := event["authority"].(map[string]any)
		if !ok {
			t.Fatalf("guarded outcome authority projection is missing: %v", event)
		}
		guardedOutcomeGrant, ok = authority["canonical_allow_write_path"].(string)
		if !ok || filepath.Base(guardedOutcomeGrant) != "issue-outcome.json" {
			t.Fatalf("guarded outcome authority path is not the exact artifact leaf: %v", authority)
		}
		wantAuthority := map[string]any{
			"schema": "claude_issue_outcome_settings_authority/v1", "source": "daemon_guarded_issue_outcome",
			"environment_variable": IssueOutcomeFileEnv, "canonical_allow_write_path": guardedOutcomeGrant,
		}
		if !reflect.DeepEqual(authority, wantAuthority) {
			t.Fatalf("guarded outcome authority projection=%v, want %v", authority, wantAuthority)
		}
	}
	if spec.Repository != nil {
		configuration, ok := launches[0]["configuration"].(map[string]any)
		if !ok {
			t.Fatalf("ordinary Claude launch configuration is missing: %v", launches[0])
		}
		arguments, ok := configuration["arguments"].(map[string]any)
		settings, settingsOK := configuration["settings"].(map[string]any)
		if !ok || !settingsOK || configuration["status"] != "observed" || arguments["setting_sources"] != "user,project,local" {
			t.Fatalf("trusted repository settings policy was not enforced: configuration=%v", configuration)
		}
		if spec.Repository.SetupCache {
			if len(setups) != 1 || setupIndex < 0 || launchIndex < 0 || setupIndex >= launchIndex {
				t.Fatalf("ordinary repository setup evidence is not unique or did not precede launch: setups=%d setup_index=%d launch_index=%d", len(setups), setupIndex, launchIndex)
			}
			preparation, ok := setups[0]["preparation"].(map[string]any)
			expectedSteps := []any{map[string]any{"name": "go_mod_download", "status": "completed"}}
			if spec.Repository.SetupPNPM {
				expectedSteps = append(expectedSteps, map[string]any{"name": "pnpm_install", "status": "completed", "directory": "web"})
			}
			if !ok || setups[0]["task_id"] != taskID || setups[0]["runtime_id"] != runtimeID ||
				setups[0]["claim_attempt"] != float64(1) || setups[0]["claim_generation"] != float64(1) ||
				setups[0]["provider"] != "claude" || setups[0]["cwd"] != result.WorkDir ||
				preparation["schema"] != "repository_setup_projection/v1" || preparation["scope"] != "claim_preparation" ||
				preparation["revision_state"] != "observed" || preparation["revision"] != actualRevision ||
				!reflect.DeepEqual(preparation["steps"], expectedSteps) {
				t.Fatalf("ordinary repository setup evidence lost its claim or repository binding: %v", setups[0])
			}
			if settings["source"] != "file" || settings["status"] != "observed" {
				t.Fatal("repository setup did not provide task-local Claude settings")
			}
		} else if settings["source"] != "absent" || settings["status"] != "missing" {
			t.Fatal("no-setup repository unexpectedly changed Claude settings")
		}
	}
	pidValue, pidOK := starts[0]["pid"].(float64)
	spanPID, spanPIDOK := spans[0]["process_id"].(float64)
	span, spanOK := spans[0]["execution_span"].(map[string]any)
	if !pidOK || pidValue <= 1 || !spanPIDOK || spanPID != pidValue || !spanOK || span["schema"] != "claude_execution_span/v1" || span["cleanup_confirmed"] != true {
		t.Fatalf("ordinary Claude cleanup span is invalid: start=%v span=%v", starts[0], spans[0])
	}
	pid := int(pidValue)
	if !codexR12WaitGroupGone(pid, 2*time.Second) {
		t.Fatalf("retaining parent-owned scratch because logged Claude process group %d is not proven gone", pid)
	}

	receipt := map[string]any{
		"schema": "claude_ordinary_runtime/v1", "status": "passed", "ordinary_run_task": true,
		"task_id": taskID, "runtime_id": runtimeID, "workdir": result.WorkDir,
		"claim_attempt": result.ClaimAttempt, "claim_generation": result.ClaimGeneration,
		"preparation": map[string]string{"scope": "in_process_not_compared"},
	}
	if testBinaryIdentity != nil {
		receipt["test_binary_identity"] = testBinaryIdentity
	}
	if spec.Repository != nil {
		receipt["repository"] = map[string]string{
			"configuration_policy": projectConfigurationTrusted,
			"revision":             actualRevision,
			"settings_source":      "isolated_user",
		}
		if spec.Repository.SetupCache {
			settingsPath := filepath.Join(result.EnvRoot, "claude-repository-setup-settings.json")
			requireResolved(settingsPath, false)
			settingsData, readErr := os.ReadFile(settingsPath)
			if readErr != nil {
				t.Fatal(readErr)
			}
			var settings struct {
				Sandbox struct {
					Filesystem struct {
						AllowWrite []string `json:"allowWrite"`
					} `json:"filesystem"`
				} `json:"sandbox"`
			}
			if err := json.Unmarshal(settingsData, &settings); err != nil {
				t.Fatal(err)
			}
			roots := settings.Sandbox.Filesystem.AllowWrite
			wantLeaves := []string{"go-build"}
			if spec.Repository.SetupPNPM {
				wantLeaves = append(wantLeaves, "pnpm-store")
			}
			cacheRoots := roots
			if guardedIssueOutcome {
				if len(roots) != len(wantLeaves)+1 || roots[len(wantLeaves)] != guardedOutcomeGrant {
					t.Fatalf("guarded outcome grant is not the exact third allowWrite path: roots=%v authority=%q", roots, guardedOutcomeGrant)
				}
				cacheRoots = roots[:len(wantLeaves)]
			}
			if len(cacheRoots) != len(wantLeaves) {
				t.Fatalf("unexpected setup grants: %v", roots)
			}
			for index, root := range cacheRoots {
				if filepath.Base(root) != wantLeaves[index] {
					t.Fatalf("setup grants are not the exact ordered leaves: %v", roots)
				}
				cacheKeyRoot := filepath.Dir(root)
				if filepath.Dir(cacheKeyRoot) != filepath.Join(result.EnvRoot, ".multica", "setup-cache", "tasks", repositorySetupPathSegment(taskID)) {
					t.Fatal("setup grant escaped its exact task")
				}
				entries, readErr := os.ReadDir(root)
				if readErr != nil || len(entries) == 0 {
					t.Fatalf("setup did not populate granted %s cache: %v", wantLeaves[index], readErr)
				}
			}
			nodeModulesPresent := false
			if spec.Repository.SetupPNPM {
				isolatedData, readErr := os.ReadFile(filepath.Join(spec.ConfigDirectory, "settings.json"))
				if readErr != nil {
					t.Fatal(readErr)
				}
				userSettings := struct {
					Sandbox struct {
						Filesystem struct {
							AllowWrite *[]string `json:"allowWrite"`
						} `json:"filesystem"`
					} `json:"sandbox"`
				}{}
				if err := json.Unmarshal(isolatedData, &userSettings); err != nil || userSettings.Sandbox.Filesystem.AllowWrite == nil || len(*userSettings.Sandbox.Filesystem.AllowWrite) != 0 {
					var grants []string
					if userSettings.Sandbox.Filesystem.AllowWrite != nil {
						grants = *userSettings.Sandbox.Filesystem.AllowWrite
					}
					t.Fatalf("ordinary full setup user settings must contain an empty allowWrite list: grants=%v error=%v", grants, err)
				}
				nodeModules := filepath.Join(result.WorkDir, "web", "node_modules")
				info, statErr := os.Lstat(nodeModules)
				if statErr != nil || !info.IsDir() || info.Mode()&os.ModeSymlink != 0 {
					t.Fatalf("ordinary full setup did not materialize web/node_modules: %v", statErr)
				}
				nodeModulesPresent = true
			}
			digest := fmt.Sprintf("sha256:%x", sha256.Sum256(settingsData))
			configuration := launches[0]["configuration"].(map[string]any)
			if configuration["settings"].(map[string]any)["sha256"] != digest {
				t.Fatal("Claude launch did not observe the actual daemon-owned setup settings")
			}
			receipt["setup_cache"] = map[string]any{
				"settings_file": settingsPath, "settings_sha256": digest,
				"granted_roots": roots, "go_build_cache_populated": true,
				"pnpm_store_populated": spec.Repository.SetupPNPM, "node_modules_present": nodeModulesPresent,
			}
		}
	}
	encoded, err := json.Marshal(receipt)
	if err != nil {
		t.Fatal(err)
	}
	t.Logf("CLAUDE_ORDINARY_RUNTIME_RECEIPT %s", encoded)
}
