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
	"io/fs"
	"net/url"
	"os"
	"os/exec"
	"path/filepath"
	"sort"
	"strings"
	"sync"
	"time"

	"github.com/multica-ai/multica/server/internal/agentguard"
)

const (
	repositorySetupCacheVersion             = "v1"
	repositorySetupSharedCacheVersion       = "v2"
	maxRepositorySetupInputBytes            = 16 << 20
	maxRepositorySetupCacheFiles            = 20000
	maxRepositorySetupCacheBytes            = 1 << 30
	maxRepositorySetupOutput                = 64 << 10
	maxRepositorySetupToolBytes             = 256 << 20
	repositoryToolProbeTimeout              = 5 * time.Second
	repositorySetupSharedCacheTTL           = 14 * 24 * time.Hour
	repositorySetupSharedStagingTTL         = time.Hour
	repositorySetupMaxSharedArtifacts       = 64
	repositorySetupMaxSharedBytes     int64 = 16 << 30
	repositorySetupMaxGCEntries             = 256
)

const repositorySetupSharedCacheMarkerContent = "repository-setup-artifacts-v2\n"

const (
	repositorySetupSharedCacheDisabled           = "disabled"
	repositorySetupSharedCacheSkippedCredentials = "skipped_credentials"
	repositorySetupSharedCacheHit                = "hit"
	repositorySetupSharedCachePublished          = "published"
)

type repositorySetupConfig struct {
	Steps           []string          `json:"steps"`
	TimeoutSeconds  int               `json:"timeout_seconds"`
	StepDirectories map[string]string `json:"step_directories,omitempty"`
}

func (s *repositorySetupConfig) UnmarshalJSON(data []byte) error {
	type setupAlias repositorySetupConfig
	decoder := json.NewDecoder(bytes.NewReader(data))
	decoder.DisallowUnknownFields()
	if err := decoder.Decode((*setupAlias)(s)); err != nil {
		return err
	}
	var fields map[string]json.RawMessage
	if err := json.Unmarshal(data, &fields); err != nil {
		return err
	}
	if value, present := fields["step_directories"]; present && bytes.Equal(bytes.TrimSpace(value), []byte("null")) {
		return errors.New("repository setup step_directories cannot be null")
	}
	return nil
}

type repositorySetupStepResult struct {
	Name      string `json:"name"`
	Status    string `json:"status"`
	Directory string `json:"directory,omitempty"`
}

type repositorySetupResult struct {
	Env               map[string]string
	SidecarExcludes   []string
	Steps             []repositorySetupStepResult
	ToolEvidence      []repositorySetupToolEvidence
	CacheKey          string
	CacheHit          bool
	SharedCacheStatus string
}

type repositorySetupTool struct {
	step        string
	path        string
	version     string
	args        []string
	inputFiles  []string
	workDir     string
	relativeDir string
}

var repositorySetupLocks sync.Map // cache artifact path -> *repositorySetupArtifactLock
var repositorySetupLocksMu sync.Mutex

type repositorySetupArtifactLock struct {
	held       chan struct{}
	references int
}

// prepareRepositorySetup executes only the two reviewed setup templates from
// the matching authorized primary repository. It returns private cache env for
// the subsequent provider launch and never runs shell text from the repository.
func prepareRepositorySetup(ctx context.Context, task Task, provider, workDir, envRoot string) (repositorySetupResult, error) {
	return prepareRepositorySetupWithCache(ctx, task, provider, workDir, envRoot, "")
}

// prepareRepositorySetupWithSharedCache uses a workspace-owned immutable
// artifact pool while keeping every command's writable caches under envRoot.
// Callers must pass the daemon's configured WorkspacesRoot, not infer it from
// the task path.
func prepareRepositorySetupWithSharedCache(ctx context.Context, task Task, provider, workDir, envRoot, workspacesRoot string) (repositorySetupResult, error) {
	return prepareRepositorySetupWithCache(ctx, task, provider, workDir, envRoot, workspacesRoot)
}

func prepareRepositorySetupWithCache(ctx context.Context, task Task, provider, workDir, envRoot, workspacesRoot string) (repositorySetupResult, error) {
	var result repositorySetupResult
	ref, err := repositorySetupRefForTask(task)
	if err != nil || ref == nil || ref.Setup == nil {
		return result, err
	}
	if strings.TrimSpace(ref.ConfigurationPolicy) != projectConfigurationTrusted {
		return result, errors.New("repository setup requires explicit trusted project configuration")
	}
	if provider != "claude" && provider != "codex" {
		return result, fmt.Errorf("repository setup is unsupported for provider %q", provider)
	}
	if err := validateRepositorySetupConfig(*ref.Setup); err != nil {
		return result, err
	}
	canonicalWorkDir, canonicalEnvRoot, err := validateRepositorySetupRoots(workDir, envRoot)
	if err != nil {
		return result, err
	}

	timeout := time.Duration(ref.Setup.TimeoutSeconds) * time.Second
	setupCtx, cancel := context.WithTimeout(ctx, timeout)
	defer cancel()

	privateCacheRoot := filepath.Join(canonicalEnvRoot, ".multica", "setup-cache")
	probeRoot := filepath.Join(privateCacheRoot, "probes", repositorySetupPathSegment(task.ID))
	if err := ensureRepositorySetupDirectories(canonicalEnvRoot,
		filepath.Join(".multica", "setup-cache", "probes", repositorySetupPathSegment(task.ID)),
	); err != nil {
		return result, err
	}
	probeEnv := repositorySetupCommandEnv(task, map[string]string{
		"GOCACHE":     filepath.Join(probeRoot, "go-build"),
		"GOMODCACHE":  filepath.Join(probeRoot, "go-mod"),
		"GOTOOLCHAIN": "local",
	})
	tools := make([]repositorySetupTool, 0, len(ref.Setup.Steps))
	stepDirectories := make(map[string]string, len(ref.Setup.Steps))
	for _, step := range ref.Setup.Steps {
		directory, err := resolveRepositorySetupDirectory(canonicalWorkDir, ref.Setup.StepDirectories[step])
		if err != nil {
			return result, err
		}
		stepDirectories[step] = directory
	}
	cacheVersion := repositorySetupCacheVersion
	sharedRoot := ""
	sharedEligible := false
	configDigest := ""
	if workspacesRoot != "" {
		sharedRoot, err = prepareRepositorySetupSharedRoot(workspacesRoot)
		if err != nil {
			return result, err
		}
		canonicalWorkspacesRoot := filepath.Dir(sharedRoot)
		insideWorkspaces, relErr := repositorySetupPathInside(canonicalWorkspacesRoot, canonicalEnvRoot)
		if relErr != nil || !insideWorkspaces || canonicalEnvRoot == canonicalWorkspacesRoot {
			return result, errors.New("repository setup environment root must be inside the configured workspaces root")
		}
		if inside, relErr := repositorySetupPathInside(canonicalEnvRoot, sharedRoot); relErr == nil && inside {
			return result, errors.New("repository setup shared cache root must be outside the task environment")
		}
		sharedEligible, configDigest = repositorySetupSharedCacheEligibility(task, ref.Setup.Steps, canonicalWorkDir)
		for _, step := range ref.Setup.Steps {
			for directory := stepDirectories[step]; directory != canonicalWorkDir; directory = filepath.Dir(directory) {
				eligible, _ := repositorySetupSharedCacheEligibility(task, []string{step}, directory)
				sharedEligible = sharedEligible && eligible
			}
		}
		cacheVersion = repositorySetupSharedCacheVersion
		if !sharedEligible {
			result.SharedCacheStatus = repositorySetupSharedCacheSkippedCredentials
		}
	} else {
		result.SharedCacheStatus = repositorySetupSharedCacheDisabled
	}
	keyMaterial := []string{cacheVersion, task.WorkspaceID, task.AgentID, task.RuntimeID}
	if workspacesRoot != "" {
		keyMaterial = append(keyMaterial, provider, configDigest)
	}
	for _, step := range ref.Setup.Steps {
		tool, inputDigest, resolveErr := resolveRepositorySetupTool(setupCtx, stepDirectories[step], canonicalEnvRoot, probeEnv, step)
		if resolveErr != nil {
			return result, resolveErr
		}
		tool.workDir = stepDirectories[step]
		tool.relativeDir = ref.Setup.StepDirectories[step]
		tools = append(tools, tool)
		toolDigest, digestErr := digestRepositorySetupExecutable(setupCtx, tool.path)
		if digestErr != nil {
			return result, digestErr
		}
		result.ToolEvidence = append(result.ToolEvidence, repositorySetupToolEvidence{
			Step: step, Directory: tool.relativeDir, RealPath: tool.path,
			Version: boundedRepositorySetupVersion(step, tool.version), SHA256: toolDigest,
			InputSHA256: inputDigest, InputFiles: append([]string(nil), tool.inputFiles...),
		})
		keyMaterial = append(keyMaterial, step, tool.version, toolDigest, inputDigest)
		if tool.relativeDir != "" {
			keyMaterial = append(keyMaterial, "step-directory/v1", tool.relativeDir)
		}
	}
	keyMaterial = append(keyMaterial, authorizedRepositoryIdentity(ref))
	if workspacesRoot == "" || sharedEligible {
		result.CacheKey = repositorySetupDigest(strings.Join(keyMaterial, "\x00"))
	}

	artifactDir := ""
	artifactBase := ""
	stagingBase := ""
	if workspacesRoot == "" {
		artifactBase = filepath.Join(privateCacheRoot, "artifacts")
		stagingBase = artifactBase
		artifactDir = filepath.Join(artifactBase, result.CacheKey)
	} else if sharedEligible {
		artifactBase = filepath.Join(sharedRoot, "artifacts", repositorySetupSharedCacheVersion)
		stagingBase = filepath.Join(sharedRoot, "staging", repositorySetupSharedCacheVersion)
		artifactDir = filepath.Join(artifactBase, result.CacheKey)
	}
	taskCacheDir := filepath.Join(privateCacheRoot, "tasks", repositorySetupPathSegment(task.ID))
	mutableRoot := filepath.Join(taskCacheDir, result.CacheKey)
	if result.CacheKey == "" {
		mutableRoot = filepath.Join(taskCacheDir, "private")
	}
	if err := ensureRepositorySetupDirectories(canonicalEnvRoot,
		filepath.Join(".multica", "setup-cache", "tasks", repositorySetupPathSegment(task.ID)),
	); err != nil {
		return result, err
	}
	if workspacesRoot == "" {
		if err := ensureRepositorySetupDirectories(canonicalEnvRoot, filepath.Join(".multica", "setup-cache", "artifacts")); err != nil {
			return result, err
		}
	}
	result.Env = map[string]string{
		"GOCACHE":              filepath.Join(mutableRoot, "go-build"),
		"GOMODCACHE":           filepath.Join(mutableRoot, "go-mod"),
		"NPM_CONFIG_STORE_DIR": filepath.Join(mutableRoot, "pnpm-store"),
		"PNPM_STORE_DIR":       filepath.Join(mutableRoot, "pnpm-store"),
	}
	result.SidecarExcludes = []string{privateCacheRoot}

	unlock := func() {}
	if artifactDir != "" {
		if workspacesRoot == "" {
			unlock, err = lockRepositorySetupArtifact(setupCtx, artifactDir)
		} else {
			unlock, err = lockRepositorySetupSharedArtifact(setupCtx, sharedRoot, artifactDir)
		}
		if err != nil {
			return result, fmt.Errorf("repository setup cache wait: %w", err)
		}
	}
	defer unlock()
	if info, statErr := os.Lstat(mutableRoot); statErr == nil && (info.Mode()&os.ModeSymlink != 0 || !info.IsDir()) {
		return result, errors.New("repository setup private task cache is not a safe directory")
	} else if statErr != nil && !errors.Is(statErr, fs.ErrNotExist) {
		return result, fmt.Errorf("repository setup: inspect private task cache: %w", statErr)
	}
	if err := os.RemoveAll(mutableRoot); err != nil {
		return result, fmt.Errorf("repository setup: reset private task cache: %w", err)
	}
	mutableRel, err := filepath.Rel(canonicalEnvRoot, mutableRoot)
	if err != nil || !filepath.IsLocal(mutableRel) {
		return result, errors.New("repository setup private task cache escaped its environment root")
	}
	if err := ensureRepositorySetupDirectories(canonicalEnvRoot, mutableRel); err != nil {
		return result, err
	}
	defer func() {
		cleanupCtx, cleanupCancel := context.WithTimeout(context.Background(), repositoryToolProbeTimeout)
		defer cleanupCancel()
		_ = makeRepositorySetupMutableCacheRemovable(cleanupCtx, mutableRoot)
	}()
	if artifactDir != "" {
		if info, statErr := os.Lstat(artifactDir); statErr == nil {
			if !info.IsDir() || info.Mode()&os.ModeSymlink != 0 {
				return result, errors.New("repository setup cache artifact is not a safe directory")
			}
			if err := copyRepositorySetupTree(setupCtx, artifactDir, mutableRoot); err != nil {
				return result, fmt.Errorf("repository setup: restore cache: %w", err)
			}
			result.CacheHit = true
			if workspacesRoot != "" {
				result.SharedCacheStatus = repositorySetupSharedCacheHit
				_ = os.Chtimes(artifactDir, time.Now(), time.Now())
			}
		} else if !errors.Is(statErr, fs.ErrNotExist) {
			return result, fmt.Errorf("repository setup: inspect cache artifact: %w", statErr)
		}
	}

	setupEnv := make(map[string]string, len(result.Env)+1)
	for key, value := range result.Env {
		setupEnv[key] = value
	}
	setupEnv["GOTOOLCHAIN"] = "local"
	commandEnv := repositorySetupCommandEnv(task, setupEnv)
	for _, tool := range tools {
		stepResult := repositorySetupStepResult{Name: tool.step, Status: "failed", Directory: tool.relativeDir}
		result.Steps = append(result.Steps, stepResult)
		directory, err := resolveRepositorySetupDirectory(canonicalWorkDir, tool.relativeDir)
		if err != nil || directory != tool.workDir {
			return result, errors.New("repository setup step directory changed before execution")
		}
		if err := runRepositorySetupCommand(setupCtx, directory, commandEnv, tool); err != nil {
			if errors.Is(err, context.DeadlineExceeded) || errors.Is(context.Cause(setupCtx), context.DeadlineExceeded) {
				result.Steps[len(result.Steps)-1].Status = "timed_out"
				return result, fmt.Errorf("repository setup step %s timed out after %d seconds", tool.step, ref.Setup.TimeoutSeconds)
			}
			if cause := context.Cause(setupCtx); cause != nil {
				result.Steps[len(result.Steps)-1].Status = "cancelled"
				return result, fmt.Errorf("repository setup step %s cancelled: %w", tool.step, cause)
			}
			return result, err
		}
		result.Steps[len(result.Steps)-1].Status = "completed"
	}

	if err := makeRepositorySetupMutableCacheRemovable(setupCtx, mutableRoot); err != nil {
		return result, fmt.Errorf("repository setup: make private task cache removable: %w", err)
	}
	if artifactDir != "" && !result.CacheHit {
		if workspacesRoot != "" {
			if _, err := pruneRepositorySetupSharedArtifacts(setupCtx, sharedRoot, time.Now(), repositorySetupMaxSharedArtifacts-1); err != nil {
				return result, fmt.Errorf("repository setup: prune shared cache before publication: %w", err)
			}
		}
		if err := publishRepositorySetupCache(setupCtx, stagingBase, artifactDir, mutableRoot, workspacesRoot != ""); err != nil {
			return result, err
		}
		if workspacesRoot != "" {
			result.SharedCacheStatus = repositorySetupSharedCachePublished
		}
	}
	return result, nil
}

func repositorySetupRefForTask(task Task) (*projectRepositoryPolicyRef, error) {
	authorized, err := primaryRepositoryForTask(task)
	if err != nil || authorized == nil {
		return nil, err
	}
	for _, resource := range task.ProjectResources {
		if resource.ResourceType != "github_repo" {
			continue
		}
		var ref projectRepositoryPolicyRef
		if err := json.Unmarshal(resource.ResourceRef, &ref); err != nil {
			return nil, fmt.Errorf("repository setup: invalid github repository resource: %w", err)
		}
		if strings.TrimSpace(ref.URL) == authorized.URL && strings.TrimSpace(ref.Ref) == authorized.Ref {
			return &ref, nil
		}
	}
	return nil, nil
}

func validateRepositorySetupConfig(config repositorySetupConfig) error {
	if config.StepDirectories != nil && len(config.StepDirectories) == 0 {
		return errors.New("repository setup step_directories must be omitted or non-empty")
	}
	if config.TimeoutSeconds < 1 || config.TimeoutSeconds > 900 {
		return errors.New("repository setup timeout_seconds must be between 1 and 900")
	}
	if len(config.Steps) == 0 || len(config.Steps) > 2 {
		return errors.New("repository setup must contain one or two named steps")
	}
	seen := make(map[string]struct{}, len(config.Steps))
	for _, step := range config.Steps {
		if step != "go_mod_download" && step != "pnpm_install" {
			return fmt.Errorf("repository setup step %q is unsupported", step)
		}
		if _, exists := seen[step]; exists {
			return fmt.Errorf("repository setup step %q is duplicated", step)
		}
		seen[step] = struct{}{}
	}
	for step, directory := range config.StepDirectories {
		if _, exists := seen[step]; !exists {
			return errors.New("repository setup step_directories must reference declared steps")
		}
		if !validRepositorySetupRelativeDirectory(directory) {
			return errors.New("repository setup step directory must be a canonical repository-relative path")
		}
	}
	return nil
}

func validRepositorySetupRelativeDirectory(directory string) bool {
	if directory == "" || len(directory) > 512 || strings.TrimSpace(directory) != directory || strings.ContainsAny(directory, "\\:") {
		return false
	}
	for _, character := range directory {
		if character < 32 || character == 127 {
			return false
		}
	}
	for _, part := range strings.Split(directory, "/") {
		if part == "" || strings.TrimSpace(part) != part || strings.HasSuffix(part, ".") || strings.EqualFold(part, ".git") || strings.EqualFold(part, ".multica") {
			return false
		}
	}
	return true
}

func resolveRepositorySetupDirectory(repositoryRoot, relative string) (string, error) {
	if relative == "" {
		return repositoryRoot, nil
	}
	if !validRepositorySetupRelativeDirectory(relative) {
		return "", errors.New("repository setup step directory is invalid")
	}
	current := repositoryRoot
	for _, part := range strings.Split(relative, "/") {
		current = filepath.Join(current, part)
		info, err := os.Lstat(current)
		if err != nil || !info.IsDir() || info.Mode()&os.ModeSymlink != 0 {
			return "", errors.New("repository setup step directory must contain only existing nonsymlink directories")
		}
	}
	return current, nil
}

func validateRepositorySetupRoots(workDir, envRoot string) (string, string, error) {
	canonicalEnvRoot, err := filepath.EvalSymlinks(envRoot)
	if err != nil {
		return "", "", fmt.Errorf("repository setup: resolve environment root: %w", err)
	}
	canonicalWorkDir, err := filepath.EvalSymlinks(workDir)
	if err != nil {
		return "", "", fmt.Errorf("repository setup: resolve working directory: %w", err)
	}
	rel, err := filepath.Rel(canonicalEnvRoot, canonicalWorkDir)
	if err != nil || !filepath.IsLocal(rel) || rel == "." {
		return "", "", errors.New("repository setup working directory must be inside the private environment root")
	}
	for _, path := range []string{canonicalEnvRoot, canonicalWorkDir} {
		info, statErr := os.Stat(path)
		if statErr != nil || !info.IsDir() {
			return "", "", errors.New("repository setup requires existing environment and working directories")
		}
	}
	return canonicalWorkDir, canonicalEnvRoot, nil
}

func resolveRepositorySetupTool(ctx context.Context, workDir, envRoot string, env []string, step string) (repositorySetupTool, string, error) {
	tool := repositorySetupTool{step: step}
	switch step {
	case "go_mod_download":
		tool.args = []string{"mod", "download"}
		tool.inputFiles = []string{"go.mod", "go.sum"}
		tool.path, _ = exec.LookPath("go")
	case "pnpm_install":
		tool.args = []string{"install", "--frozen-lockfile", "--ignore-scripts", "--ignore-pnpmfile"}
		tool.inputFiles = []string{"package.json", "pnpm-lock.yaml"}
		tool.path, _ = exec.LookPath("pnpm")
	}
	if tool.path == "" {
		return tool, "", fmt.Errorf("repository setup step %s requires its toolchain executable", step)
	}
	resolvedPath, err := filepath.EvalSymlinks(tool.path)
	if err != nil {
		return tool, "", fmt.Errorf("repository setup step %s could not resolve its toolchain executable", step)
	}
	info, err := os.Stat(resolvedPath)
	if err != nil || !info.Mode().IsRegular() || info.Mode()&0o111 == 0 {
		return tool, "", fmt.Errorf("repository setup step %s toolchain executable is not a regular executable", step)
	}
	if inside, _ := repositorySetupPathInside(workDir, resolvedPath); inside {
		return tool, "", fmt.Errorf("repository setup step %s toolchain executable cannot come from the repository", step)
	}
	if inside, _ := repositorySetupPathInside(envRoot, resolvedPath); inside {
		return tool, "", fmt.Errorf("repository setup step %s toolchain executable cannot come from the task environment", step)
	}
	tool.path = resolvedPath

	hash := sha256.New()
	for _, name := range tool.inputFiles {
		content, readErr := readRepositorySetupInput(workDir, name)
		if readErr != nil {
			return tool, "", readErr
		}
		hash.Write([]byte(name))
		hash.Write([]byte{0})
		hash.Write(content)
	}
	probeCtx, cancel := context.WithTimeout(ctx, repositoryToolProbeTimeout)
	defer cancel()
	versionArgs := []string{"--version"}
	if step == "go_mod_download" {
		versionArgs = []string{"version"}
	}
	var output boundedRepositorySetupBuffer
	cmd := exec.CommandContext(probeCtx, tool.path, versionArgs...)
	cmd.Dir = workDir
	cmd.Env = env
	cmd.Stdout, cmd.Stderr = &output, &output
	if err := cmd.Run(); err != nil {
		return tool, "", fmt.Errorf("repository setup step %s could not verify its toolchain version", step)
	}
	tool.version = strings.TrimSpace(output.String())
	if tool.version == "" {
		return tool, "", fmt.Errorf("repository setup step %s returned an empty toolchain version", step)
	}
	return tool, hex.EncodeToString(hash.Sum(nil)), nil
}

func readRepositorySetupInput(workDir, name string) ([]byte, error) {
	path := filepath.Join(workDir, name)
	file, info, err := openRepositorySetupRegularFile(path)
	if err != nil {
		if errors.Is(err, fs.ErrNotExist) {
			return nil, fmt.Errorf("repository setup requires %s", name)
		}
		return nil, fmt.Errorf("repository setup requires %s to be a stable regular file: %w", name, err)
	}
	defer file.Close()
	if info.Size() <= 0 || info.Size() > maxRepositorySetupInputBytes {
		return nil, fmt.Errorf("repository setup requires %s to be non-empty and at most %d bytes", name, maxRepositorySetupInputBytes)
	}
	content, err := io.ReadAll(io.LimitReader(file, maxRepositorySetupInputBytes+1))
	if err != nil {
		return nil, fmt.Errorf("repository setup: read %s: %w", name, err)
	}
	if len(content) > maxRepositorySetupInputBytes {
		return nil, fmt.Errorf("repository setup requires %s to be non-empty and at most %d bytes", name, maxRepositorySetupInputBytes)
	}
	if name == "package.json" {
		var document map[string]any
		if err := json.Unmarshal(content, &document); err != nil || document == nil {
			return nil, errors.New("repository setup requires package.json to contain a JSON object")
		}
	}
	return content, nil
}

func ensureRepositorySetupDirectories(root string, relatives ...string) error {
	for _, relative := range relatives {
		current := root
		for _, component := range strings.Split(filepath.Clean(relative), string(os.PathSeparator)) {
			current = filepath.Join(current, component)
			info, err := os.Lstat(current)
			switch {
			case err == nil && info.IsDir() && info.Mode()&os.ModeSymlink == 0:
				continue
			case err == nil:
				return fmt.Errorf("repository setup cache path %s is not a safe directory", current)
			case errors.Is(err, fs.ErrNotExist):
				if err := os.Mkdir(current, 0o700); err != nil {
					if !errors.Is(err, fs.ErrExist) {
						return fmt.Errorf("repository setup: create private cache directory: %w", err)
					}
					created, statErr := os.Lstat(current)
					if statErr != nil || !created.IsDir() || created.Mode()&os.ModeSymlink != 0 {
						return fmt.Errorf("repository setup cache path %s raced with an unsafe entry", current)
					}
				}
			default:
				return fmt.Errorf("repository setup: inspect private cache directory: %w", err)
			}
		}
	}
	return nil
}

func repositorySetupPathInside(root, path string) (bool, error) {
	rel, err := filepath.Rel(root, path)
	if err != nil {
		return false, err
	}
	return rel == "." || filepath.IsLocal(rel), nil
}

func runRepositorySetupCommand(ctx context.Context, workDir string, env []string, tool repositorySetupTool) error {
	var output boundedRepositorySetupBuffer
	cmd := exec.CommandContext(ctx, tool.path, tool.args...)
	cmd.Dir = workDir
	cmd.Env = env
	cmd.Stdout, cmd.Stderr = &output, &output
	if err := cmd.Run(); err != nil {
		if cause := context.Cause(ctx); cause != nil {
			return cause
		}
		return fmt.Errorf("repository setup step %s failed", tool.step)
	}
	return nil
}

type boundedRepositorySetupBuffer struct {
	bytes.Buffer
}

func (b *boundedRepositorySetupBuffer) Write(p []byte) (int, error) {
	remaining := maxRepositorySetupOutput - b.Len()
	if remaining <= 0 {
		return len(p), nil
	}
	if len(p) > remaining {
		_, _ = b.Buffer.Write(p[:remaining])
		return len(p), nil
	}
	return b.Buffer.Write(p)
}

func repositorySetupCommandEnv(task Task, overrides map[string]string) []string {
	env := make(map[string]string)
	for _, entry := range os.Environ() {
		if key, value, ok := strings.Cut(entry, "="); ok {
			if agentguard.AllowedInheritedEnvKey(key) {
				env[key] = value
			}
		}
	}
	if task.Agent != nil {
		for key, value := range sanitizeAgentEnv(task.Agent.CustomEnv) {
			env[key] = value
		}
	}
	for key, value := range overrides {
		env[key] = value
	}
	keys := make([]string, 0, len(env))
	for key := range env {
		keys = append(keys, key)
	}
	sort.Strings(keys)
	out := make([]string, 0, len(keys))
	for _, key := range keys {
		out = append(out, key+"="+env[key])
	}
	return out
}

func repositorySetupDigest(value string) string {
	sum := sha256.Sum256([]byte(value))
	return hex.EncodeToString(sum[:])
}

func repositorySetupPathSegment(value string) string {
	if value != "" && strings.IndexFunc(value, func(r rune) bool {
		return !(r >= 'a' && r <= 'z') && !(r >= 'A' && r <= 'Z') && !(r >= '0' && r <= '9') && r != '-' && r != '_'
	}) == -1 {
		return value
	}
	return repositorySetupDigest(value)[:24]
}

func lockRepositorySetupArtifact(ctx context.Context, path string) (func(), error) {
	created := &repositorySetupArtifactLock{held: make(chan struct{}, 1)}
	repositorySetupLocksMu.Lock()
	value, _ := repositorySetupLocks.LoadOrStore(path, created)
	lock := value.(*repositorySetupArtifactLock)
	lock.references++
	repositorySetupLocksMu.Unlock()
	releaseReference := func() {
		repositorySetupLocksMu.Lock()
		defer repositorySetupLocksMu.Unlock()
		lock.references--
		if lock.references == 0 {
			repositorySetupLocks.Delete(path)
		}
	}
	select {
	case lock.held <- struct{}{}:
		var once sync.Once
		return func() {
			once.Do(func() {
				<-lock.held
				releaseReference()
			})
		}, nil
	case <-ctx.Done():
		releaseReference()
		return nil, context.Cause(ctx)
	}
}

func lockRepositorySetupSharedArtifact(ctx context.Context, sharedRoot, artifactDir string) (func(), error) {
	localUnlock, err := lockRepositorySetupArtifact(ctx, artifactDir)
	if err != nil {
		return nil, err
	}
	key := filepath.Base(artifactDir)
	lockPath := filepath.Join(sharedRoot, "locks", repositorySetupSharedCacheVersion, key+".lock")
	fileUnlock, acquired, err := lockRepositorySetupFile(ctx, lockPath, true)
	if err != nil || !acquired {
		localUnlock()
		if err != nil {
			return nil, err
		}
		return nil, errors.New("repository setup shared artifact lock was not acquired")
	}
	return func() {
		fileUnlock()
		localUnlock()
	}, nil
}

func tryLockRepositorySetupSharedArtifact(ctx context.Context, sharedRoot, artifactDir string) (func(), bool, error) {
	key := filepath.Base(artifactDir)
	lockPath := filepath.Join(sharedRoot, "locks", repositorySetupSharedCacheVersion, key+".lock")
	return lockRepositorySetupFile(ctx, lockPath, false)
}

type repositorySetupArtifactInfo struct {
	path    string
	modTime time.Time
	size    int64
}

// pruneRepositorySetupSharedArtifacts reclaims only v2 artifacts with canonical
// digest names. A global GC lock serializes scanners, while each artifact lock
// is the active reservation shared with restore/publication.
func pruneRepositorySetupSharedArtifacts(ctx context.Context, sharedRoot string, now time.Time, maxArtifacts int) (int, error) {
	gcUnlock, acquired, err := lockRepositorySetupFile(ctx, filepath.Join(sharedRoot, "gc.lock"), false)
	if err != nil || !acquired {
		return 0, err
	}
	defer gcUnlock()
	if err := pruneRepositorySetupSharedStages(ctx, sharedRoot, now); err != nil {
		return 0, err
	}

	artifactRoot := filepath.Join(sharedRoot, "artifacts", repositorySetupSharedCacheVersion)
	dir, err := os.Open(artifactRoot)
	if err != nil {
		if errors.Is(err, fs.ErrNotExist) {
			return 0, nil
		}
		return 0, err
	}
	entries, readErr := dir.ReadDir(repositorySetupMaxGCEntries + 1)
	closeErr := dir.Close()
	if readErr != nil && !errors.Is(readErr, io.EOF) {
		return 0, readErr
	}
	if closeErr != nil {
		return 0, closeErr
	}
	if len(entries) > repositorySetupMaxGCEntries {
		entries = entries[:repositorySetupMaxGCEntries]
	}

	artifacts := make([]repositorySetupArtifactInfo, 0, len(entries))
	var totalBytes int64
	for _, entry := range entries {
		if !entry.IsDir() || !repositorySetupDigestName(entry.Name()) {
			continue
		}
		path := filepath.Join(artifactRoot, entry.Name())
		info, infoErr := entry.Info()
		if infoErr != nil || info.Mode()&os.ModeSymlink != 0 {
			continue
		}
		size, sizeErr := repositorySetupTreeSize(ctx, path)
		if sizeErr != nil {
			continue
		}
		totalBytes += size
		artifacts = append(artifacts, repositorySetupArtifactInfo{path: path, modTime: info.ModTime(), size: size})
	}
	sort.Slice(artifacts, func(i, j int) bool { return artifacts[i].modTime.Before(artifacts[j].modTime) })
	removed := 0
	remaining := len(artifacts)
	for _, artifact := range artifacts {
		if remaining <= maxArtifacts && totalBytes <= repositorySetupMaxSharedBytes && now.Sub(artifact.modTime) <= repositorySetupSharedCacheTTL {
			break
		}
		if cause := context.Cause(ctx); cause != nil {
			return removed, cause
		}
		unlock, ok, lockErr := tryLockRepositorySetupSharedArtifact(ctx, sharedRoot, artifact.path)
		if lockErr != nil {
			return removed, lockErr
		}
		if !ok {
			continue
		}
		removeErr := makeRepositorySetupMutableCacheRemovable(ctx, artifact.path)
		if removeErr == nil {
			removeErr = os.RemoveAll(artifact.path)
		}
		unlock()
		if removeErr != nil {
			return removed, removeErr
		}
		totalBytes -= artifact.size
		remaining--
		removed++
	}
	return removed, nil
}

func pruneRepositorySetupSharedStages(ctx context.Context, sharedRoot string, now time.Time) error {
	stagingRoot := filepath.Join(sharedRoot, "staging", repositorySetupSharedCacheVersion)
	dir, err := os.Open(stagingRoot)
	if err != nil {
		if errors.Is(err, fs.ErrNotExist) {
			return nil
		}
		return err
	}
	entries, readErr := dir.ReadDir(repositorySetupMaxGCEntries + 1)
	closeErr := dir.Close()
	if readErr != nil && !errors.Is(readErr, io.EOF) {
		return readErr
	}
	if closeErr != nil {
		return closeErr
	}
	if len(entries) > repositorySetupMaxGCEntries {
		entries = entries[:repositorySetupMaxGCEntries]
	}
	for _, entry := range entries {
		if cause := context.Cause(ctx); cause != nil {
			return cause
		}
		if !entry.IsDir() || entry.Type()&os.ModeSymlink != 0 || !strings.HasPrefix(entry.Name(), ".publish-") {
			continue
		}
		info, infoErr := entry.Info()
		if infoErr != nil || now.Sub(info.ModTime()) <= repositorySetupSharedStagingTTL {
			continue
		}
		path := filepath.Join(stagingRoot, entry.Name())
		if err := makeRepositorySetupMutableCacheRemovable(ctx, path); err != nil {
			continue
		}
		if err := os.RemoveAll(path); err != nil {
			return err
		}
	}
	return nil
}

func pruneRepositorySetupSharedCache(ctx context.Context, workspacesRoot string) (int, error) {
	sharedRoot, err := prepareRepositorySetupSharedRoot(workspacesRoot)
	if err != nil {
		return 0, err
	}
	return pruneRepositorySetupSharedArtifacts(ctx, sharedRoot, time.Now(), repositorySetupMaxSharedArtifacts)
}

func repositorySetupDigestName(name string) bool {
	if len(name) != sha256.Size*2 {
		return false
	}
	_, err := hex.DecodeString(name)
	return err == nil
}

func repositorySetupTreeSize(ctx context.Context, root string) (int64, error) {
	var files int
	var total int64
	err := filepath.WalkDir(root, func(path string, entry fs.DirEntry, walkErr error) error {
		if cause := context.Cause(ctx); cause != nil {
			return cause
		}
		if walkErr != nil {
			return walkErr
		}
		if entry.Type()&os.ModeSymlink != 0 {
			return errors.New("repository setup shared artifact contains a symlink")
		}
		if entry.IsDir() {
			return nil
		}
		info, err := entry.Info()
		if err != nil || !info.Mode().IsRegular() {
			return errors.New("repository setup shared artifact contains an unsafe entry")
		}
		files++
		if files > maxRepositorySetupCacheFiles || info.Size() > maxRepositorySetupCacheBytes-total {
			return errors.New("repository setup shared artifact exceeds its bounds")
		}
		total += info.Size()
		return nil
	})
	return total, err
}

func authorizedRepositoryIdentity(ref *projectRepositoryPolicyRef) string {
	return strings.TrimSpace(ref.URL) + "\x00" + strings.TrimSpace(ref.Ref)
}

func prepareRepositorySetupSharedRoot(workspacesRoot string) (string, error) {
	canonicalRoot, err := filepath.EvalSymlinks(workspacesRoot)
	if err != nil {
		return "", fmt.Errorf("repository setup: resolve workspaces root: %w", err)
	}
	info, err := os.Stat(canonicalRoot)
	if err != nil || !info.IsDir() {
		return "", errors.New("repository setup requires an existing workspaces root")
	}
	if err := ensureRepositorySetupDirectories(canonicalRoot, ".setup-cache"); err != nil {
		return "", err
	}
	sharedRoot := filepath.Join(canonicalRoot, ".setup-cache")
	marker := filepath.Join(sharedRoot, ".multica-managed-v2")
	if markerInfo, statErr := os.Lstat(marker); statErr == nil {
		if !markerInfo.Mode().IsRegular() || markerInfo.Mode()&os.ModeSymlink != 0 || !validRepositorySetupSharedMarker(marker) {
			return "", errors.New("repository setup shared cache ownership marker is invalid")
		}
	} else if errors.Is(statErr, fs.ErrNotExist) {
		entries, readErr := os.ReadDir(sharedRoot)
		if readErr != nil {
			return "", fmt.Errorf("repository setup: inspect unmarked shared cache root: %w", readErr)
		}
		if len(entries) != 0 && !validRepositorySetupSharedMarker(marker) {
			return "", errors.New("repository setup refuses to adopt a non-empty unmarked .setup-cache directory")
		}
		createErr := publishRepositorySetupSharedMarker(canonicalRoot, marker)
		if createErr != nil {
			return "", fmt.Errorf("repository setup: write shared cache ownership marker: %w", createErr)
		}
	} else {
		return "", fmt.Errorf("repository setup: inspect shared cache ownership marker: %w", statErr)
	}
	if err := ensureRepositorySetupDirectories(canonicalRoot,
		filepath.Join(".setup-cache", "artifacts", repositorySetupSharedCacheVersion),
		filepath.Join(".setup-cache", "locks", repositorySetupSharedCacheVersion),
		filepath.Join(".setup-cache", "staging", repositorySetupSharedCacheVersion),
	); err != nil {
		return "", err
	}
	return sharedRoot, nil
}

func publishRepositorySetupSharedMarker(workspacesRoot, marker string) error {
	temporary, err := os.CreateTemp(workspacesRoot, ".multica-setup-cache-marker-")
	if err != nil {
		return err
	}
	temporaryPath := temporary.Name()
	defer os.Remove(temporaryPath)
	if _, err = temporary.WriteString(repositorySetupSharedCacheMarkerContent); err == nil {
		err = temporary.Sync()
	}
	if closeErr := temporary.Close(); err == nil {
		err = closeErr
	}
	if err != nil {
		return err
	}
	if err := os.Link(temporaryPath, marker); err != nil {
		if !errors.Is(err, fs.ErrExist) || !validRepositorySetupSharedMarker(marker) {
			return err
		}
	}
	return nil
}

func validRepositorySetupSharedMarker(marker string) bool {
	file, info, err := openRepositorySetupRegularFile(marker)
	if err != nil || info.Size() != int64(len(repositorySetupSharedCacheMarkerContent)) {
		if file != nil {
			_ = file.Close()
		}
		return false
	}
	defer file.Close()
	content, err := io.ReadAll(io.LimitReader(file, int64(len(repositorySetupSharedCacheMarkerContent))+1))
	return err == nil && string(content) == repositorySetupSharedCacheMarkerContent
}

// repositorySetupSharedCacheEligibility returns a digest only for configuration
// that can be classified without reading credential values. Ambiguous auth or
// package-manager configuration still reaches the private setup command, but it
// cannot restore from or publish to the cross-task artifact pool.
func repositorySetupSharedCacheEligibility(task Task, steps []string, workDir string) (bool, string) {
	values := make(map[string]string)
	if task.Agent != nil {
		for key, value := range sanitizeAgentEnv(task.Agent.CustomEnv) {
			upper := strings.ToUpper(strings.TrimSpace(key))
			if repositorySetupManagedEnvName(upper) {
				continue
			}
			if repositorySetupCredentialEnvName(upper) {
				return false, ""
			}
			if !repositorySetupNonSecretConfigEnv(upper) {
				return false, ""
			}
			if repositorySetupURLConfigEnv(upper) && !repositorySetupURLHasNoCredentials(value) {
				return false, ""
			}
			values[upper] = value
		}
	}
	for _, key := range []string{"SSL_CERT_FILE", "SSL_CERT_DIR", "NODE_EXTRA_CA_CERTS"} {
		if value := os.Getenv(key); value != "" {
			values[key] = value
		}
	}
	for _, step := range steps {
		switch step {
		case "go_mod_download":
			if repositorySetupCredentialFilePresent(".netrc") || repositorySetupCredentialFilePresent("_netrc") ||
				repositorySetupCredentialFilePresent(".git-credentials") || repositorySetupGoEnvConfigPresent() {
				return false, ""
			}
		case "pnpm_install":
			if repositorySetupCredentialFilePresent(".npmrc") || repositorySetupPathExists(filepath.Join(workDir, ".npmrc")) {
				return false, ""
			}
		}
	}
	keys := make([]string, 0, len(values))
	for key := range values {
		keys = append(keys, key)
	}
	sort.Strings(keys)
	material := make([]string, 0, len(keys)*2)
	for _, key := range keys {
		material = append(material, key, values[key])
	}
	return true, repositorySetupDigest(strings.Join(material, "\x00"))
}

func repositorySetupManagedEnvName(name string) bool {
	switch name {
	case "GOCACHE", "GOMODCACHE", "GOTOOLCHAIN", "NPM_CONFIG_STORE_DIR", "PNPM_STORE_DIR":
		return true
	default:
		return false
	}
}

func repositorySetupCredentialEnvName(name string) bool {
	for _, fragment := range []string{"TOKEN", "SECRET", "PASSWORD", "PASSWD", "CREDENTIAL", "COOKIE", "PRIVATE_KEY", "API_KEY", "AUTH"} {
		if strings.Contains(name, fragment) {
			return true
		}
	}
	switch name {
	case "NETRC", "GIT_ASKPASS", "SSH_ASKPASS", "SSH_AUTH_SOCK", "NPM_CONFIG_USERCONFIG", "GOENV",
		"GIT_CONFIG_GLOBAL", "GIT_CONFIG_SYSTEM", "GIT_SSH", "GIT_SSH_COMMAND":
		return true
	default:
		return strings.HasPrefix(name, "GIT_CONFIG_") ||
			(strings.HasPrefix(name, "NPM_CONFIG_") && name != "NPM_CONFIG_REGISTRY") ||
			(strings.HasPrefix(name, "PNPM_") && name != "PNPM_CONFIG_REGISTRY" && name != "PNPM_STORE_DIR")
	}
}

func repositorySetupNonSecretConfigEnv(name string) bool {
	switch name {
	case "HTTP_PROXY", "HTTPS_PROXY", "ALL_PROXY", "NO_PROXY",
		"GOPROXY", "GONOPROXY", "GOSUMDB", "GOPRIVATE", "GONOSUMDB", "GOINSECURE", "GOVCS",
		"GOOS", "GOARCH", "GOAMD64", "GOARM", "GOARM64", "GO386", "GOMIPS", "GOMIPS64", "GOPPC64", "GORISCV64", "GOEXPERIMENT", "GOFLAGS",
		"CGO_ENABLED", "CC", "CXX", "PKG_CONFIG",
		"NPM_CONFIG_REGISTRY", "PNPM_CONFIG_REGISTRY", "SSL_CERT_FILE", "SSL_CERT_DIR", "NODE_EXTRA_CA_CERTS":
		return true
	default:
		return false
	}
}

func repositorySetupURLConfigEnv(name string) bool {
	switch name {
	case "HTTP_PROXY", "HTTPS_PROXY", "ALL_PROXY", "GOPROXY", "NPM_CONFIG_REGISTRY", "PNPM_CONFIG_REGISTRY":
		return true
	default:
		return false
	}
}

func repositorySetupURLHasNoCredentials(value string) bool {
	for _, item := range strings.Split(value, ",") {
		item = strings.TrimSpace(item)
		if item == "" || item == "direct" || item == "off" {
			continue
		}
		parsed, err := url.Parse(item)
		if err != nil || parsed.Scheme == "" || parsed.Host == "" || parsed.User != nil || parsed.RawQuery != "" || parsed.Fragment != "" {
			return false
		}
	}
	return true
}

func repositorySetupCredentialFilePresent(name string) bool {
	home, err := os.UserHomeDir()
	if err != nil || strings.TrimSpace(home) == "" {
		return false
	}
	info, err := os.Lstat(filepath.Join(home, name))
	return err == nil && (info.Mode().IsRegular() || info.Mode()&os.ModeSymlink != 0)
}

func repositorySetupGoEnvConfigPresent() bool {
	configDir, err := os.UserConfigDir()
	return err == nil && repositorySetupPathExists(filepath.Join(configDir, "go", "env"))
}

func repositorySetupPathExists(path string) bool {
	_, err := os.Lstat(path)
	return err == nil
}

func digestRepositorySetupExecutable(ctx context.Context, path string) (string, error) {
	file, _, err := openRepositorySetupRegularFile(path)
	if err != nil {
		return "", fmt.Errorf("repository setup: open toolchain executable: %w", err)
	}
	defer file.Close()
	hash := sha256.New()
	written, err := io.Copy(hash, io.LimitReader(repositorySetupContextReader{ctx: ctx, reader: file}, maxRepositorySetupToolBytes+1))
	if err != nil {
		return "", fmt.Errorf("repository setup: hash toolchain executable: %w", err)
	}
	if written > maxRepositorySetupToolBytes {
		return "", fmt.Errorf("repository setup toolchain executable exceeds %d bytes", maxRepositorySetupToolBytes)
	}
	return hex.EncodeToString(hash.Sum(nil)), nil
}

func publishRepositorySetupCache(ctx context.Context, stagingRoot, artifactDir, mutableRoot string, immutable bool) error {
	stage, err := os.MkdirTemp(stagingRoot, ".publish-")
	if err != nil {
		return fmt.Errorf("repository setup: create cache publication stage: %w", err)
	}
	defer func() {
		cleanupCtx, cancel := context.WithTimeout(context.Background(), repositoryToolProbeTimeout)
		defer cancel()
		_ = makeRepositorySetupMutableCacheRemovable(cleanupCtx, stage)
		_ = os.RemoveAll(stage)
	}()
	if err := copyRepositorySetupTree(ctx, mutableRoot, stage); err != nil {
		return fmt.Errorf("repository setup: stage cache artifact: %w", err)
	}
	if err := os.Rename(stage, artifactDir); err != nil {
		if info, statErr := os.Lstat(artifactDir); statErr == nil {
			if info.IsDir() && info.Mode()&os.ModeSymlink == 0 {
				return nil
			}
			return errors.New("repository setup cache publication target is unsafe")
		}
		return fmt.Errorf("repository setup: publish cache artifact: %w", err)
	}
	if immutable {
		if err := makeRepositorySetupArtifactImmutable(ctx, artifactDir); err != nil {
			cleanupCtx, cancel := context.WithTimeout(context.Background(), repositoryToolProbeTimeout)
			defer cancel()
			_ = makeRepositorySetupMutableCacheRemovable(cleanupCtx, artifactDir)
			_ = os.RemoveAll(artifactDir)
			return fmt.Errorf("repository setup: seal cache artifact: %w", err)
		}
	}
	return nil
}

func makeRepositorySetupArtifactImmutable(ctx context.Context, root string) error {
	var directories []string
	err := filepath.WalkDir(root, func(path string, entry fs.DirEntry, walkErr error) error {
		if cause := context.Cause(ctx); cause != nil {
			return cause
		}
		if walkErr != nil {
			return walkErr
		}
		if entry.Type()&os.ModeSymlink != 0 {
			return errors.New("repository setup cache artifact contains a symlink")
		}
		if entry.IsDir() {
			directories = append(directories, path)
			return nil
		}
		info, err := entry.Info()
		if err != nil || !info.Mode().IsRegular() {
			return errors.New("repository setup cache artifact contains an unsafe entry")
		}
		return os.Chmod(path, 0o400)
	})
	if err != nil {
		return err
	}
	for i := len(directories) - 1; i >= 0; i-- {
		if err := os.Chmod(directories[i], 0o500); err != nil {
			return err
		}
	}
	return nil
}

func makeRepositorySetupMutableCacheRemovable(ctx context.Context, root string) error {
	entries := 0
	return filepath.WalkDir(root, func(path string, entry fs.DirEntry, walkErr error) error {
		if cause := context.Cause(ctx); cause != nil {
			return cause
		}
		if walkErr != nil {
			return walkErr
		}
		entries++
		if entries > maxRepositorySetupCacheFiles {
			return errors.New("repository setup cache exceeds its file limit")
		}
		if entry.Type()&os.ModeSymlink != 0 {
			return nil
		}
		info, err := entry.Info()
		if err != nil {
			return err
		}
		mode := info.Mode().Perm()
		if entry.IsDir() {
			if mode&0o700 == 0o700 {
				return nil
			}
			return os.Chmod(path, mode|0o700)
		}
		if !info.Mode().IsRegular() || mode&0o600 == 0o600 {
			return nil
		}
		return os.Chmod(path, mode|0o600)
	})
}

func copyRepositorySetupTree(ctx context.Context, source, destination string) error {
	var files int
	var bytesCopied int64
	return filepath.WalkDir(source, func(path string, entry fs.DirEntry, walkErr error) error {
		if cause := context.Cause(ctx); cause != nil {
			return cause
		}
		if walkErr != nil {
			return walkErr
		}
		rel, err := filepath.Rel(source, path)
		if err != nil || !filepath.IsLocal(rel) {
			return errors.New("cache entry escaped its root")
		}
		if rel == "." {
			return nil
		}
		info, err := entry.Info()
		if err != nil {
			return err
		}
		target := filepath.Join(destination, rel)
		if entry.Type()&os.ModeSymlink != 0 {
			return fmt.Errorf("cache entry %s is a symlink", rel)
		}
		if entry.IsDir() {
			// pnpm v10 registers project paths here; these are not reusable dependencies.
			if rel == filepath.Join("pnpm-store", "v10", "projects") {
				return filepath.SkipDir
			}
			return os.MkdirAll(target, 0o700)
		}
		if !info.Mode().IsRegular() {
			return fmt.Errorf("cache entry %s is not a regular file", rel)
		}
		files++
		if files > maxRepositorySetupCacheFiles || info.Size() > maxRepositorySetupCacheBytes-bytesCopied {
			return errors.New("repository setup cache exceeds its file or byte limit")
		}
		if err := os.MkdirAll(filepath.Dir(target), 0o700); err != nil {
			return err
		}
		copied, err := copyRepositorySetupFile(ctx, path, target, maxRepositorySetupCacheBytes-bytesCopied)
		bytesCopied += copied
		return err
	})
}

func copyRepositorySetupFile(ctx context.Context, source, destination string, maxBytes int64) (copied int64, err error) {
	in, info, err := openRepositorySetupRegularFile(source)
	if err != nil {
		return 0, err
	}
	defer in.Close()
	if info.Size() > maxBytes {
		return 0, errors.New("repository setup cache exceeds its byte limit")
	}
	out, err := os.OpenFile(destination, os.O_CREATE|os.O_EXCL|os.O_WRONLY, 0o600)
	if err != nil {
		return 0, err
	}
	defer func() {
		if err != nil {
			_ = os.Remove(destination)
		}
	}()
	copied, copyErr := io.Copy(out, io.LimitReader(repositorySetupContextReader{ctx: ctx, reader: in}, maxBytes+1))
	closeErr := out.Close()
	if copyErr != nil {
		return copied, copyErr
	}
	if copied > maxBytes {
		return copied, errors.New("repository setup cache exceeds its byte limit")
	}
	return copied, closeErr
}

func openRepositorySetupRegularFile(path string) (*os.File, fs.FileInfo, error) {
	before, err := os.Lstat(path)
	if err != nil {
		return nil, nil, err
	}
	if !before.Mode().IsRegular() || before.Mode()&os.ModeSymlink != 0 {
		return nil, nil, errors.New("path is not a regular file")
	}
	file, err := os.Open(path)
	if err != nil {
		return nil, nil, err
	}
	after, lstatErr := os.Lstat(path)
	opened, statErr := file.Stat()
	if lstatErr != nil || statErr != nil || !after.Mode().IsRegular() || after.Mode()&os.ModeSymlink != 0 ||
		!opened.Mode().IsRegular() || !os.SameFile(before, opened) || !os.SameFile(after, opened) {
		_ = file.Close()
		return nil, nil, errors.New("regular file changed while opening")
	}
	return file, opened, nil
}

type repositorySetupContextReader struct {
	ctx    context.Context
	reader io.Reader
}

func (r repositorySetupContextReader) Read(p []byte) (int, error) {
	if cause := context.Cause(r.ctx); cause != nil {
		return 0, cause
	}
	return r.reader.Read(p)
}
