package daemon

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"log/slog"
	"os"
	"os/exec"
	"path/filepath"
	"strings"

	"github.com/multica-ai/multica/server/internal/daemon/execenv"
	"github.com/multica-ai/multica/server/internal/daemon/repocache"
	"github.com/multica-ai/multica/server/internal/projectdesignsystem"
	"github.com/multica-ai/multica/server/internal/service"
)

func isProgrammaticFirstProjectDesignSystemTask(task Task) bool {
	if len(task.ProjectDesignSystemContext) == 0 {
		return false
	}
	var taskContext service.ProjectDesignSystemTaskContext
	return json.Unmarshal(task.ProjectDesignSystemContext, &taskContext) == nil &&
		taskContext.Type == service.ProjectDesignSystemTaskContextType &&
		taskContext.Operation == service.ProjectDesignSystemGenerate &&
		taskContext.ExecutionMode == service.ProjectDesignSystemExecutionModeProgrammaticFirst &&
		taskContext.PackageSchema == projectdesignsystem.PackageSchemaV2 &&
		strings.TrimSpace(taskContext.ProjectResourceID) != ""
}

// runProgrammaticFirstProjectDesignSystemTask executes the repository-scoped
// fast path without launching the selected model provider. The agent/runtime row
// still routes the task to the correct machine, but no Agent prompt, memory,
// skills or MCP configuration participates in generation.
func (d *Daemon) runProgrammaticFirstProjectDesignSystemTask(
	ctx context.Context,
	task Task,
	taskLog *slog.Logger,
) (TaskResult, error) {
	var taskContext service.ProjectDesignSystemTaskContext
	if json.Unmarshal(task.ProjectDesignSystemContext, &taskContext) != nil ||
		taskContext.ExecutionMode != service.ProjectDesignSystemExecutionModeProgrammaticFirst {
		return TaskResult{}, errors.New("invalid programmatic project design system context")
	}

	stopPrepareLease := d.startTaskPrepareLeaseExtender(ctx, task, taskLog)
	defer stopPrepareLease()

	rootClaim, err := execenv.ClaimEnvRoot(taskRootDirParams(d.cfg.WorkspacesRoot, task))
	if err != nil {
		return TaskResult{}, fmt.Errorf("claim programmatic execution environment: %w", err)
	}
	defer rootClaim.Release()

	env, err := d.prepareExecutionEnvironment(ctx, execenv.PrepareParams{
		WorkspacesRoot:    d.cfg.WorkspacesRoot,
		Profile:           d.cfg.Profile,
		WorkspaceID:       task.WorkspaceID,
		WorkspaceSlug:     task.WorkspaceSlug,
		TaskID:            task.ID,
		IssueIdentifier:   task.IssueIdentifier,
		AgentName:         "programmatic-design-system",
		EnvRootPreclaimed: true,
		Provider:          "programmatic",
		Task:              programmaticTaskContextForEnv(task),
	})
	if err != nil {
		return TaskResult{}, fmt.Errorf("prepare programmatic execution environment: %w", err)
	}
	result := TaskResult{WorkDir: env.WorkDir, EnvRoot: env.RootDir}

	if err := d.client.StartTask(ctx, task.ID); err != nil {
		return result, fmt.Errorf("start programmatic task: %w", err)
	}
	stopPrepareLease()

	sequence := 1
	report := func(content string) {
		content = strings.TrimSpace(content)
		if content == "" {
			return
		}
		if err := d.client.ReportTaskMessages(ctx, task.ID, []TaskMessageData{{
			Seq: sequence, Type: "text", Content: content,
		}}); err != nil {
			taskLog.Warn("programmatic design system message report failed", "error", err)
		}
		sequence++
	}

	repositoryRoot, repositoryURL, repositoryName, commitSHA, checkoutErr := d.prepareProgrammaticRepository(
		ctx, task, taskContext.ProjectResourceID, env.WorkDir,
	)
	analysis := decodeProgrammaticRepositoryAnalysis(taskContext.RepositoryAnalysis)
	if checkoutErr != nil && analysis == nil {
		return TaskResult{
			Status: "blocked", Comment: "Repository quick extraction failed: " + checkoutErr.Error(),
			WorkDir: env.WorkDir, EnvRoot: env.RootDir,
			FailureReason: "project_design_system_repository_unavailable",
		}, nil
	}
	if checkoutErr != nil {
		taskLog.Warn("programmatic repository checkout unavailable; using frozen analysis", "error", checkoutErr)
		report("仓库 checkout 暂时不可用，正在使用已冻结的仓库分析继续生成快速草稿。")
	} else {
		report(fmt.Sprintf("已固定仓库 %s 和 Commit %s。", repositoryName, shortProgrammaticCommit(commitSHA)))
	}

	projectName, projectDescription := decodeProgrammaticProject(taskContext.Project)
	generated, err := projectdesignsystem.GenerateProgrammaticFirstPackage(
		ctx,
		repositoryRoot,
		env.OutputDir,
		projectdesignsystem.ProgrammaticInput{
			ProjectName:         firstProgrammaticValue(projectName, task.ProjectTitle),
			ProjectDescription:  firstProgrammaticValue(projectDescription, task.ProjectDescription),
			RepositoryName:      repositoryName,
			RepositoryURL:       repositoryURL,
			CommitSHA:           firstProgrammaticValue(commitSHA, analysisCommitSHA(analysis)),
			Platform:            taskContext.Platform,
			Brief:               taskContext.Brief,
			InputSnapshotSHA256: taskContext.InputSnapshotSHA256,
			RepositoryAnalysis:  analysis,
		},
		func(progress projectdesignsystem.ProgrammaticProgress) {
			switch progress.Stage {
			case "inventory":
				report(fmt.Sprintf("已完成有界源码清单，共读取 %d 个高信号文件。", progress.SourceFileCount))
			case "extraction":
				report(fmt.Sprintf("已提取设计 Seed：%d 个颜色信号、%d 个组件模式、%d 个页面模式。", progress.ColorCount, progress.ComponentCount, progress.PagePatternCount))
			case "package":
				report("已生成 Tokens、设计规则、组件状态、页面模式和在线 UI Kit。")
			}
		},
	)
	if err != nil {
		return TaskResult{
			Status: "blocked", Comment: "Programmatic design system generation failed: " + err.Error(),
			WorkDir: env.WorkDir, EnvRoot: env.RootDir,
			FailureReason: "project_design_system_programmatic_generation_failed",
		}, nil
	}
	report("快速草稿候选已生成，正在执行 Package Audit 和真实浏览器 Preview。")
	return TaskResult{
		Status: "completed",
		Comment: fmt.Sprintf(
			"Generated a repository quick draft from %d source files, %d component patterns and %d page patterns without launching an Agent.",
			len(generated.SourceFiles), generated.ComponentCount, generated.PagePatternCount,
		),
		WorkDir: env.WorkDir,
		EnvRoot: env.RootDir,
	}, nil
}

func programmaticTaskContextForEnv(task Task) execenv.TaskContextForEnv {
	return execenv.TaskContextForEnv{
		TaskID:                     task.ID,
		AgentID:                    task.AgentID,
		AgentName:                  "程序化快速生成引擎",
		Repos:                      convertReposForEnv(task.Repos),
		ProjectID:                  task.ProjectID,
		ProjectTitle:               task.ProjectTitle,
		ProjectDescription:         task.ProjectDescription,
		ProjectResources:           convertProjectResourcesForEnv(task.ProjectResources),
		ProjectDesignSystemContext: strings.TrimSpace(string(task.ProjectDesignSystemContext)),
	}
}

func (d *Daemon) prepareProgrammaticRepository(
	ctx context.Context,
	task Task,
	projectResourceID string,
	workDir string,
) (root, repositoryURL, repositoryName, commitSHA string, err error) {
	repositoriesDir := filepath.Join(workDir, "repositories")
	if err := os.MkdirAll(repositoriesDir, 0o755); err != nil {
		return "", "", "", "", err
	}

	selectedResources := make([]ProjectResourceData, 0, 1)
	for _, resource := range task.ProjectResources {
		if resource.ID == strings.TrimSpace(projectResourceID) {
			selectedResources = append(selectedResources, resource)
			break
		}
	}
	if len(selectedResources) == 0 {
		return "", "", "", "", errors.New("selected repository resource is missing from the task claim")
	}
	resourceName, resourceURL, resourceRef := selectedProgrammaticResource(selectedResources)
	if resourceURL != "" {
		if d.repoCache == nil {
			return "", resourceURL, resourceName, "", errors.New("repository cache is unavailable")
		}
		if err := d.ensureRepoReady(ctx, task.WorkspaceID, resourceURL); err != nil {
			return "", resourceURL, resourceName, "", err
		}
		params := repocache.WorktreeParams{
			WorkspaceID:         task.WorkspaceID,
			RepoURL:             resourceURL,
			WorkDir:             repositoriesDir,
			Ref:                 resourceRef,
			AgentName:           "programmatic-design-system",
			TaskID:              task.ID,
			PeerURLs:            []string{resourceURL},
			CoAuthoredByEnabled: false,
			IsolatedGitMetadata: true,
		}
		var checkout *repocache.WorktreeResult
		var checkoutErr error
		if contextCache, ok := d.repoCache.(interface {
			CreateWorktreeContext(context.Context, repocache.WorktreeParams) (*repocache.WorktreeResult, error)
		}); ok {
			checkout, checkoutErr = contextCache.CreateWorktreeContext(ctx, params)
		} else {
			checkout, checkoutErr = d.repoCache.CreateWorktree(params)
		}
		if checkoutErr != nil {
			return "", resourceURL, resourceName, "", checkoutErr
		}
		commit, commitErr := programmaticGitCommit(ctx, checkout.Path)
		if commitErr != nil {
			return "", resourceURL, resourceName, "", commitErr
		}
		return checkout.Path, resourceURL, firstProgrammaticValue(resourceName, repositoryNameFromURL(resourceURL)), commit, nil
	}

	assignment, assignmentErr := findLocalDirectoryAssignment(selectedResources, d.cfg.DaemonID)
	if assignmentErr != nil {
		return "", "", resourceName, "", assignmentErr
	}
	if assignment == nil {
		return "", "", resourceName, "", errors.New("selected repository resource is unavailable on this runtime")
	}
	target := filepath.Join(repositoriesDir, "repository")
	command := exec.CommandContext(ctx, "git", "clone", "--quiet", "--no-hardlinks", assignment.RealPath, target)
	if output, cloneErr := command.CombinedOutput(); cloneErr != nil {
		return "", "", firstProgrammaticValue(resourceName, assignment.DisplayName()), "", fmt.Errorf("clone local repository: %w: %s", cloneErr, strings.TrimSpace(string(output)))
	}
	commit, commitErr := programmaticGitCommit(ctx, target)
	if commitErr != nil {
		return "", "", firstProgrammaticValue(resourceName, assignment.DisplayName()), "", commitErr
	}
	return target, "", firstProgrammaticValue(resourceName, assignment.DisplayName()), commit, nil
}

func selectedProgrammaticResource(resources []ProjectResourceData) (name, repositoryURL, ref string) {
	if len(resources) == 0 {
		return "", "", ""
	}
	resource := resources[0]
	name = strings.TrimSpace(resource.Label)
	if resource.ResourceType != "github_repo" {
		return name, "", ""
	}
	var payload struct {
		URL string `json:"url"`
		Ref string `json:"ref,omitempty"`
	}
	if json.Unmarshal(resource.ResourceRef, &payload) == nil {
		return name, strings.TrimSpace(payload.URL), strings.TrimSpace(payload.Ref)
	}
	return name, "", ""
}

func decodeProgrammaticProject(raw json.RawMessage) (name, description string) {
	var project struct {
		Name        string `json:"name"`
		Description string `json:"description"`
	}
	if json.Unmarshal(raw, &project) != nil {
		return "", ""
	}
	return strings.TrimSpace(project.Name), strings.TrimSpace(project.Description)
}

func decodeProgrammaticRepositoryAnalysis(raw json.RawMessage) *projectdesignsystem.RepositoryDesignContext {
	if len(raw) == 0 || string(raw) == "null" {
		return nil
	}
	var value projectdesignsystem.RepositoryDesignContext
	if json.Unmarshal(raw, &value) != nil {
		return nil
	}
	validated, err := projectdesignsystem.ValidateRepositoryDesignContext(value)
	if err != nil {
		return nil
	}
	return &validated
}

func analysisCommitSHA(value *projectdesignsystem.RepositoryDesignContext) string {
	if value == nil {
		return ""
	}
	return strings.TrimSpace(value.CommitSHA)
}

func programmaticGitCommit(ctx context.Context, root string) (string, error) {
	output, err := exec.CommandContext(ctx, "git", "-C", root, "rev-parse", "HEAD").Output()
	if err != nil {
		return "", fmt.Errorf("read repository commit: %w", err)
	}
	return strings.TrimSpace(string(output)), nil
}

func repositoryNameFromURL(raw string) string {
	value := strings.TrimSuffix(strings.TrimRight(strings.TrimSpace(raw), "/"), ".git")
	if index := strings.LastIndexAny(value, "/:"); index >= 0 && index+1 < len(value) {
		return value[index+1:]
	}
	return value
}

func shortProgrammaticCommit(value string) string {
	value = strings.TrimSpace(value)
	if len(value) > 12 {
		return value[:12]
	}
	if value == "" {
		return "未记录"
	}
	return value
}

func firstProgrammaticValue(values ...string) string {
	for _, value := range values {
		if strings.TrimSpace(value) != "" {
			return strings.TrimSpace(value)
		}
	}
	return ""
}
