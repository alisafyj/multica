package daemon

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/url"
	"os"
	"path/filepath"
	"strings"

	"github.com/multica-ai/multica/server/internal/daemon/execenv"
	"github.com/multica-ai/multica/server/internal/daemon/repocache"
)

const (
	primaryRepositoryReceiptFile     = ".primary-repository.json"
	maxPrimaryRepositoryReceiptBytes = 16 << 10
)

type preparedPrimaryRepository struct {
	Version    int    `json:"version"`
	URL        string `json:"url"`
	Ref        string `json:"ref,omitempty"`
	BranchName string `json:"branch_name"`
	WorkDir    string `json:"-"`
}

type primaryRepositoryGitIdentity struct {
	WorkDir    string
	BranchName string
}

// Select only an unambiguous issue repository from the server's authorized
// claim. A project resource is a selection, never an additional authorization.
func primaryRepositoryForTask(task Task) (*RepoData, error) {
	if task.IssueID == "" || task.ChatSessionID != "" || task.IsLeaderTask || taskIsSquadLeader(task) ||
		task.AutopilotRunID != "" || task.QuickCreatePrompt != "" || task.RegenerateQuickActionsFor != "" ||
		len(task.UIDraftCreateContext) > 0 || len(task.DesignRestoreContext) > 0 ||
		testingContextPresent(task.TestGenerationContext) || testingContextPresent(task.TestRunContext) ||
		len(task.DesignSystemProfileAnalyzeContext) > 0 || len(task.TemplateBlueprintAnalyzeContext) > 0 ||
		len(task.ProjectDesignSystemContext) > 0 || len(task.DesignDocumentContext) > 0 || len(task.PMOSyncContext) > 0 {
		return nil, nil
	}
	var resources []ProjectResourceData
	for _, resource := range task.ProjectResources {
		if resource.ResourceType == "local_directory" {
			return nil, nil
		}
		if resource.ResourceType == "github_repo" {
			resources = append(resources, resource)
		}
	}
	if len(resources) > 1 {
		return nil, nil
	}
	if len(resources) == 1 {
		var selected RepoData
		if err := json.Unmarshal(resources[0].ResourceRef, &selected); err != nil {
			return nil, fmt.Errorf("primary repository: invalid project resource: %w", err)
		}
		selected.URL, selected.Ref = strings.TrimSpace(selected.URL), strings.TrimSpace(selected.Ref)
		if err := validatePrimaryRepositoryURL(selected.URL); err != nil {
			return nil, err
		}
		for _, repo := range task.Repos {
			if selected.URL != "" && strings.TrimSpace(repo.URL) == selected.URL {
				if strings.TrimSpace(repo.Ref) != selected.Ref {
					return nil, fmt.Errorf("primary repository: project ref disagrees with the authorized claim")
				}
				return &selected, nil
			}
		}
		return nil, fmt.Errorf("primary repository: selected project repository is absent from the authorized claim")
	}
	if len(task.Repos) != 1 || strings.TrimSpace(task.Repos[0].URL) == "" {
		return nil, nil
	}
	repo := task.Repos[0]
	repo.URL, repo.Ref = strings.TrimSpace(repo.URL), strings.TrimSpace(repo.Ref)
	if err := validatePrimaryRepositoryURL(repo.URL); err != nil {
		return nil, err
	}
	return &repo, nil
}

func validatePrimaryRepositoryURL(raw string) error {
	u, err := url.Parse(raw)
	if err != nil {
		if strings.Contains(raw, "://") {
			return errors.New("primary repository: invalid repository URL")
		}
		return nil // Local paths and SCP-style SSH remotes are not URL userinfo.
	}
	unsafe := u.RawQuery != "" || u.ForceQuery || u.Fragment != ""
	if u.User != nil {
		_, hasPassword := u.User.Password()
		unsafe = unsafe || u.Scheme != "ssh" || hasPassword
	}
	if unsafe {
		return errors.New("primary repository: use a credential-free repository URL without query or fragment; configure authentication through the authorized Git credential mechanism")
	}
	return nil
}

func (d *Daemon) preparePrimaryRepository(ctx context.Context, task Task, provider, agentName, envRoot string) (*preparedPrimaryRepository, error) {
	// Only these providers have explicit project-settings and hook enforcement
	// for a repository mounted at the environment root. Other providers keep
	// the legacy nested checkout until their native configuration is validated.
	if provider != "claude" && provider != "codex" {
		return nil, nil
	}
	repo, err := primaryRepositoryForTask(task)
	if err != nil || repo == nil {
		return nil, err
	}
	if err := d.ensureRepoReady(ctx, task.WorkspaceID, repo.URL); err != nil {
		return nil, fmt.Errorf("prepare primary repository: %w", err)
	}
	workDir := filepath.Join(envRoot, "workdir")
	params := repocache.WorktreeParams{
		WorkspaceID: task.WorkspaceID, RepoURL: repo.URL, WorkDir: workDir,
		CheckoutAtWorkDir: true, Ref: repo.Ref, AgentName: agentName, TaskID: task.ID,
		PeerURLs:            d.taskRepoPeerURLs(task.WorkspaceID, task.ID),
		CoAuthoredByEnabled: d.workspaceCoAuthoredByEnabled(task.WorkspaceID),
		IsolatedGitMetadata: true,
	}
	var result *repocache.WorktreeResult
	if cache, ok := d.repoCache.(interface {
		CreateWorktreeContext(context.Context, repocache.WorktreeParams) (*repocache.WorktreeResult, error)
	}); ok {
		result, err = cache.CreateWorktreeContext(ctx, params)
	} else {
		result, err = d.repoCache.CreateWorktree(params)
	}
	if err != nil {
		return nil, fmt.Errorf("prepare primary repository checkout: %w", err)
	}
	if result == nil || filepath.Clean(result.Path) != filepath.Clean(workDir) {
		return nil, fmt.Errorf("primary repository checkout returned an unexpected working directory")
	}
	identity, err := inspectPrimaryRepositoryGit(ctx, workDir, repo.URL)
	if err != nil {
		return nil, primaryRepositoryPreservedError(workDir, "checkout identity validation failed", err)
	}
	if err := execenv.ValidatePrimaryRepositoryContext(workDir, provider); err != nil {
		return nil, primaryRepositoryPreservedError(workDir, "checkout context validation failed", err)
	}
	state := &preparedPrimaryRepository{Version: 1, URL: repo.URL, Ref: repo.Ref, BranchName: identity.BranchName, WorkDir: identity.WorkDir}
	if err := writePrimaryRepository(envRoot, state); err != nil {
		return nil, primaryRepositoryPreservedError(workDir, "could not record checkout continuity", err)
	}
	return state, nil
}

func primaryRepositoryPreservedError(workDir, action string, err error) error {
	return fmt.Errorf("prepare primary repository: %s; work is preserved at %s: %w", action, workDir, err)
}

// inspectPrimaryRepositoryGit proves that workDir itself is the checkout root,
// its Git metadata is a real local directory, and origin still denotes the
// repository selected by the server. It is deliberately read-only: resumed
// branches, commits, detached HEADs, and dirty files belong to the conversation.
func inspectPrimaryRepositoryGit(ctx context.Context, workDir, expectedURL string) (*primaryRepositoryGitIdentity, error) {
	if cause := context.Cause(ctx); cause != nil {
		return nil, cause
	}
	canonicalWorkDir, err := filepath.Abs(workDir)
	if err != nil {
		return nil, fmt.Errorf("resolve checkout path: %w", err)
	}
	canonicalWorkDir, err = filepath.EvalSymlinks(canonicalWorkDir)
	if err != nil {
		return nil, fmt.Errorf("resolve checkout path: %w", err)
	}
	info, err := os.Stat(canonicalWorkDir)
	if err != nil || !info.IsDir() {
		return nil, fmt.Errorf("checkout path is not a directory")
	}

	gitPath := filepath.Join(canonicalWorkDir, ".git")
	gitInfo, err := os.Lstat(gitPath)
	if err != nil || !gitInfo.IsDir() || gitInfo.Mode()&os.ModeSymlink != 0 {
		return nil, fmt.Errorf("checkout does not have isolated local Git metadata")
	}
	gitDir, err := runGitCommandContext(ctx, canonicalWorkDir, gitCmdTimeout, "rev-parse", "--absolute-git-dir")
	if err != nil {
		return nil, primaryRepositoryGitReadError(ctx, "could not resolve checkout Git metadata")
	}
	if gitDir == "" {
		return nil, fmt.Errorf("could not resolve checkout Git metadata")
	}
	canonicalGitDir, err := filepath.EvalSymlinks(filepath.Clean(filepath.FromSlash(gitDir)))
	if err != nil || !sameExistingDir(canonicalGitDir, gitPath) {
		return nil, fmt.Errorf("checkout Git metadata is outside the checkout")
	}

	gitRoot, err := runGitCommandContext(ctx, canonicalWorkDir, gitCmdTimeout, "rev-parse", "--show-toplevel")
	if err != nil {
		return nil, primaryRepositoryGitReadError(ctx, "could not resolve checkout repository root")
	}
	if gitRoot == "" {
		return nil, fmt.Errorf("could not resolve checkout repository root")
	}
	canonicalGitRoot, err := filepath.EvalSymlinks(filepath.Clean(filepath.FromSlash(gitRoot)))
	if err != nil || !sameExistingDir(canonicalGitRoot, canonicalWorkDir) {
		return nil, fmt.Errorf("checkout path is not the repository root")
	}

	origin, err := runGitCommandContext(ctx, canonicalWorkDir, gitCmdTimeout, "remote", "get-url", "origin")
	if err != nil {
		return nil, primaryRepositoryGitReadError(ctx, "could not resolve checkout origin")
	}
	if strings.TrimSpace(origin) != strings.TrimSpace(expectedURL) {
		return nil, fmt.Errorf("checkout origin does not match the authorized repository")
	}
	branch, err := runGitCommandContext(ctx, canonicalWorkDir, gitCmdTimeout, "rev-parse", "--abbrev-ref", "HEAD")
	if err != nil {
		return nil, primaryRepositoryGitReadError(ctx, "could not resolve checkout HEAD")
	}
	if branch == "HEAD" {
		branch = ""
	}
	return &primaryRepositoryGitIdentity{WorkDir: canonicalWorkDir, BranchName: branch}, nil
}

func primaryRepositoryGitReadError(ctx context.Context, fallback string) error {
	if cause := context.Cause(ctx); cause != nil {
		return cause
	}
	return errors.New(fallback)
}

func writePrimaryRepository(envRoot string, state *preparedPrimaryRepository) error {
	data, err := json.Marshal(state)
	if err != nil {
		return err
	}
	if len(data) > maxPrimaryRepositoryReceiptBytes {
		return fmt.Errorf("primary repository receipt exceeds size limit")
	}
	root, err := os.OpenRoot(envRoot)
	if err != nil {
		return err
	}
	defer root.Close()
	file, err := root.OpenFile(primaryRepositoryReceiptFile, os.O_WRONLY|os.O_CREATE|os.O_EXCL, 0o600)
	if err != nil {
		return fmt.Errorf("create primary repository receipt: %w", err)
	}
	_, writeErr := file.Write(data)
	return errors.Join(writeErr, file.Close())
}

func readPrimaryRepository(envRoot string) (*preparedPrimaryRepository, error) {
	root, err := os.OpenRoot(envRoot)
	if err != nil {
		return nil, err
	}
	defer root.Close()
	info, err := root.Lstat(primaryRepositoryReceiptFile)
	if errors.Is(err, os.ErrNotExist) {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	if !info.Mode().IsRegular() || info.Size() > maxPrimaryRepositoryReceiptBytes {
		return nil, fmt.Errorf("invalid primary repository receipt file")
	}
	file, err := root.Open(primaryRepositoryReceiptFile)
	if err != nil {
		return nil, err
	}
	defer file.Close()
	data, err := io.ReadAll(io.LimitReader(file, maxPrimaryRepositoryReceiptBytes+1))
	if err != nil {
		return nil, err
	}
	var state preparedPrimaryRepository
	if len(data) > maxPrimaryRepositoryReceiptBytes || json.Unmarshal(data, &state) != nil || state.Version != 1 || state.URL == "" {
		return nil, fmt.Errorf("invalid primary repository receipt")
	}
	state.WorkDir = filepath.Join(envRoot, "workdir")
	return &state, nil
}

// Called only while the prior env-root claim is held. Never fetch or reset a
// resumed checkout: its commits and dirty tree belong to the conversation.
func resumePrimaryRepository(task Task, workDir string) (*preparedPrimaryRepository, error) {
	return resumePrimaryRepositoryContext(context.Background(), task, workDir)
}

func resumePrimaryRepositoryContext(ctx context.Context, task Task, workDir string) (*preparedPrimaryRepository, error) {
	state, err := readPrimaryRepository(filepath.Dir(workDir))
	if err != nil || state == nil {
		return nil, err // Older nested-layout sessions keep their existing layout.
	}
	repo, err := primaryRepositoryForTask(task)
	if err != nil {
		return nil, err
	}
	if repo == nil || repo.URL != state.URL || repo.Ref != state.Ref {
		return nil, fmt.Errorf("primary repository selection or pinned ref changed; previous work is preserved at %s; start a fresh session or restore the prior selection", workDir)
	}
	identity, err := inspectPrimaryRepositoryGit(ctx, workDir, repo.URL)
	if err != nil {
		return nil, fmt.Errorf("primary repository checkout identity is invalid; previous work is preserved at %s; start a fresh session or restore the expected checkout: %w", workDir, err)
	}
	state.WorkDir = identity.WorkDir
	state.BranchName = identity.BranchName
	return state, nil
}

func WithPrimaryRepository(state *preparedPrimaryRepository) PromptOption {
	return func(opts *promptOpts) { opts.primaryRepository = state }
}

func buildPrimaryRepositoryBlock(state *preparedPrimaryRepository) string {
	if state == nil {
		return ""
	}
	return fmt.Sprintf("## Prepared primary repository\n\nThe primary repository is already prepared at your startup working directory %q (initial ref %q). Work there directly and follow its repository instructions. Do not clone or checkout it again as a setup step. Resumed work keeps its existing branch and uncommitted changes. Use `multica repo checkout` only for additional repositories; use explicit Git operations in the primary checkout when the task requires a revision change.\n\n", state.WorkDir, state.Ref)
}

func primaryRepositoryPrompt(brief, prompt string, state *preparedPrimaryRepository) string {
	if state == nil || brief == "" {
		return prompt
	}
	return brief + "\n\n" + prompt
}
