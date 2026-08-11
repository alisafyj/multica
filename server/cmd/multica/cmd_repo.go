package main

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/multica-ai/multica/server/internal/cli"
	"github.com/spf13/cobra"
)

var repoCmd = &cobra.Command{
	Use:   "repo",
	Short: "Work with repositories",
}

var repoListCmd = &cobra.Command{
	Use:   "list",
	Short: "List workspace repositories",
	Long:  "Lists the repository registry for the current workspace. These are workspace-level repos, separate from project resources.",
	Args:  cobra.NoArgs,
	RunE:  runRepoList,
}

var repoAddCmd = &cobra.Command{
	Use:   "add [url]...",
	Short: "Add repositories to the workspace registry",
	Long: "Adds one or more repository URLs to the current workspace repository registry. " +
		"Existing URLs are not duplicated. Use project resources when you need project-specific context instead.",
	Args: cobra.ArbitraryArgs,
	RunE: runRepoAdd,
}

var repoRemoveCmd = &cobra.Command{
	Use:     "remove [url]...",
	Aliases: []string{"rm"},
	Short:   "Remove repositories from the workspace registry",
	Long:    "Removes one or more repository URLs from the current workspace repository registry.",
	Args:    cobra.ArbitraryArgs,
	RunE:    runRepoRemove,
}

var repoCheckoutCmd = &cobra.Command{
	Use:   "checkout [<url>]",
	Short: "Check out a repository into the working directory",
	Long: "Creates a git worktree from the daemon's bare clone cache. Used by agents to check out repos on demand.\n" +
		"Pass a single URL to check out one repository, or --all to check out every github_repo resource " +
		"listed in .multica/project/resources.json.",
	Args: func(cmd *cobra.Command, args []string) error {
		all, _ := cmd.Flags().GetBool("all")
		if all {
			if len(args) > 0 {
				return fmt.Errorf("--all and a positional URL argument are mutually exclusive")
			}
			return nil
		}
		return exactArgs(1)(cmd, args)
	},
	RunE: runRepoCheckout,
}

var repoCheckoutRef string

func init() {
	repoListCmd.Flags().String("output", "table", "Output format: table or json")

	repoAddCmd.Flags().StringArray("url", nil, "Repository URL to add (may be repeated)")
	repoAddCmd.Flags().String("description", "", "Optional description; only valid when adding one URL")
	repoAddCmd.Flags().String("output", "json", "Output format: table or json")

	repoRemoveCmd.Flags().StringArray("url", nil, "Repository URL to remove (may be repeated)")
	repoRemoveCmd.Flags().String("output", "json", "Output format: table or json")

	repoCheckoutCmd.Flags().StringVar(&repoCheckoutRef, "ref", "", "branch, tag, or commit to check out instead of the remote default branch")
	repoCheckoutCmd.Flags().Bool("all", false, "Check out every github_repo resource from .multica/project/resources.json")

	repoCmd.AddCommand(repoListCmd)
	repoCmd.AddCommand(repoAddCmd)
	repoCmd.AddCommand(repoRemoveCmd)
	repoCmd.AddCommand(repoCheckoutCmd)
}

type workspaceRepo struct {
	URL         string `json:"url"`
	Description string `json:"description,omitempty"`
}

type repoWorkspaceResponse struct {
	ID    string          `json:"id"`
	Name  string          `json:"name"`
	Slug  string          `json:"slug"`
	Repos []workspaceRepo `json:"repos"`
}

type repoMutationResult struct {
	WorkspaceID string          `json:"workspace_id"`
	Added       []workspaceRepo `json:"added,omitempty"`
	Updated     []workspaceRepo `json:"updated,omitempty"`
	Removed     []workspaceRepo `json:"removed,omitempty"`
	Repos       []workspaceRepo `json:"repos"`
}

func repoURLsFromArgsAndFlags(cmd *cobra.Command, args []string) ([]string, error) {
	flagURLs, _ := cmd.Flags().GetStringArray("url")
	raw := append([]string{}, flagURLs...)
	raw = append(raw, args...)
	if len(raw) == 0 {
		return nil, fmt.Errorf("at least one repository URL is required")
	}

	urls := make([]string, 0, len(raw))
	seen := make(map[string]struct{}, len(raw))
	for _, u := range raw {
		u = strings.TrimSpace(u)
		if u == "" {
			return nil, fmt.Errorf("repository URL cannot be empty")
		}
		if _, ok := seen[u]; ok {
			continue
		}
		seen[u] = struct{}{}
		urls = append(urls, u)
	}
	return urls, nil
}

func fetchRepoWorkspace(ctx context.Context, client *cli.APIClient, workspaceID string) (repoWorkspaceResponse, error) {
	var ws repoWorkspaceResponse
	if err := client.GetJSON(ctx, "/api/workspaces/"+workspaceID, &ws); err != nil {
		return repoWorkspaceResponse{}, fmt.Errorf("get workspace: %w", err)
	}
	if ws.Repos == nil {
		ws.Repos = []workspaceRepo{}
	}
	return ws, nil
}

func patchWorkspaceRepos(ctx context.Context, client *cli.APIClient, workspaceID string, repos []workspaceRepo) (repoWorkspaceResponse, error) {
	var ws repoWorkspaceResponse
	if err := client.PatchJSON(ctx, "/api/workspaces/"+workspaceID, map[string]any{"repos": repos}, &ws); err != nil {
		return repoWorkspaceResponse{}, fmt.Errorf("update workspace repos: %w", err)
	}
	if ws.Repos == nil {
		ws.Repos = []workspaceRepo{}
	}
	return ws, nil
}

func repoCommandClient(cmd *cobra.Command) (*cli.APIClient, string, error) {
	workspaceID, err := requireWorkspaceID(cmd)
	if err != nil {
		return nil, "", err
	}
	client, err := newAPIClient(cmd)
	if err != nil {
		return nil, "", err
	}
	return client, workspaceID, nil
}

func runRepoList(cmd *cobra.Command, _ []string) error {
	client, workspaceID, err := repoCommandClient(cmd)
	if err != nil {
		return err
	}

	ctx, cancel := cli.APIContext(context.Background())
	defer cancel()

	ws, err := fetchRepoWorkspace(ctx, client, workspaceID)
	if err != nil {
		return err
	}

	output, _ := cmd.Flags().GetString("output")
	if output == "json" {
		return cli.PrintJSON(os.Stdout, ws.Repos)
	}
	if len(ws.Repos) == 0 {
		fmt.Fprintln(os.Stderr, "No repositories found.")
		return nil
	}
	rows := make([][]string, 0, len(ws.Repos))
	for _, repo := range ws.Repos {
		rows = append(rows, []string{repo.URL, repo.Description})
	}
	cli.PrintTable(os.Stdout, []string{"URL", "DESCRIPTION"}, rows)
	return nil
}

func runRepoAdd(cmd *cobra.Command, args []string) error {
	urls, err := repoURLsFromArgsAndFlags(cmd, args)
	if err != nil {
		return err
	}
	description, _ := cmd.Flags().GetString("description")
	descriptionChanged := cmd.Flags().Changed("description")
	if descriptionChanged && len(urls) > 1 {
		return fmt.Errorf("--description can only be used when adding one repository URL")
	}

	client, workspaceID, err := repoCommandClient(cmd)
	if err != nil {
		return err
	}

	ctx, cancel := cli.APIContext(context.Background())
	defer cancel()

	ws, err := fetchRepoWorkspace(ctx, client, workspaceID)
	if err != nil {
		return err
	}

	indexByURL := make(map[string]int, len(ws.Repos))
	for i, repo := range ws.Repos {
		indexByURL[repo.URL] = i
	}

	added := []workspaceRepo{}
	updated := []workspaceRepo{}
	repos := append([]workspaceRepo{}, ws.Repos...)
	for _, u := range urls {
		if idx, ok := indexByURL[u]; ok {
			if descriptionChanged && repos[idx].Description != description {
				repos[idx].Description = description
				updated = append(updated, repos[idx])
			}
			continue
		}
		repo := workspaceRepo{URL: u}
		if descriptionChanged {
			repo.Description = description
		}
		indexByURL[u] = len(repos)
		repos = append(repos, repo)
		added = append(added, repo)
	}

	if len(added) > 0 || len(updated) > 0 {
		ws, err = patchWorkspaceRepos(ctx, client, workspaceID, repos)
		if err != nil {
			return err
		}
	} else {
		ws.Repos = repos
	}

	result := repoMutationResult{
		WorkspaceID: ws.ID,
		Added:       added,
		Updated:     updated,
		Repos:       ws.Repos,
	}
	output, _ := cmd.Flags().GetString("output")
	if output == "json" {
		return cli.PrintJSON(os.Stdout, result)
	}
	if len(added) == 0 && len(updated) == 0 {
		fmt.Fprintln(os.Stdout, "No repository changes.")
		return nil
	}
	rows := make([][]string, 0, len(added)+len(updated))
	for _, repo := range added {
		rows = append(rows, []string{"added", repo.URL, repo.Description})
	}
	for _, repo := range updated {
		rows = append(rows, []string{"updated", repo.URL, repo.Description})
	}
	cli.PrintTable(os.Stdout, []string{"ACTION", "URL", "DESCRIPTION"}, rows)
	return nil
}

func runRepoRemove(cmd *cobra.Command, args []string) error {
	urls, err := repoURLsFromArgsAndFlags(cmd, args)
	if err != nil {
		return err
	}

	client, workspaceID, err := repoCommandClient(cmd)
	if err != nil {
		return err
	}

	ctx, cancel := cli.APIContext(context.Background())
	defer cancel()

	ws, err := fetchRepoWorkspace(ctx, client, workspaceID)
	if err != nil {
		return err
	}

	removeSet := make(map[string]struct{}, len(urls))
	for _, u := range urls {
		removeSet[u] = struct{}{}
	}
	removedSet := make(map[string]struct{}, len(urls))
	removed := []workspaceRepo{}
	repos := make([]workspaceRepo, 0, len(ws.Repos))
	for _, repo := range ws.Repos {
		if _, ok := removeSet[repo.URL]; ok {
			removed = append(removed, repo)
			removedSet[repo.URL] = struct{}{}
			continue
		}
		repos = append(repos, repo)
	}
	missing := []string{}
	for _, u := range urls {
		if _, ok := removedSet[u]; !ok {
			missing = append(missing, u)
		}
	}
	if len(missing) > 0 {
		return fmt.Errorf("repository not found in workspace registry: %s", strings.Join(missing, ", "))
	}

	ws, err = patchWorkspaceRepos(ctx, client, workspaceID, repos)
	if err != nil {
		return err
	}

	result := repoMutationResult{
		WorkspaceID: ws.ID,
		Removed:     removed,
		Repos:       ws.Repos,
	}
	output, _ := cmd.Flags().GetString("output")
	if output == "json" {
		return cli.PrintJSON(os.Stdout, result)
	}
	rows := make([][]string, 0, len(removed))
	for _, repo := range removed {
		rows = append(rows, []string{repo.URL, repo.Description})
	}
	cli.PrintTable(os.Stdout, []string{"REMOVED URL", "DESCRIPTION"}, rows)
	return nil
}

// checkoutResult is the daemon response for a single repository checkout.
type checkoutResult struct {
	Path       string `json:"path"`
	BranchName string `json:"branch_name"`
}

// doCheckoutRequest sends one checkout request to the daemon and returns the
// path and branch name on success.
func doCheckoutRequest(daemonPort string, reqBody map[string]string) (checkoutResult, error) {
	data, err := json.Marshal(reqBody)
	if err != nil {
		return checkoutResult{}, fmt.Errorf("encode request: %w", err)
	}

	client := &http.Client{Timeout: 5 * time.Minute}
	resp, err := client.Post(
		fmt.Sprintf("http://127.0.0.1:%s/repo/checkout", daemonPort),
		"application/json",
		bytes.NewReader(data),
	)
	if err != nil {
		return checkoutResult{}, fmt.Errorf("connect to daemon: %w", err)
	}
	defer resp.Body.Close()

	body, _ := io.ReadAll(resp.Body)
	if resp.StatusCode != http.StatusOK {
		return checkoutResult{}, fmt.Errorf("checkout failed: %s", string(body))
	}

	var result checkoutResult
	if err := json.Unmarshal(body, &result); err != nil {
		return checkoutResult{}, fmt.Errorf("parse response: %w", err)
	}
	return result, nil
}

func runRepoCheckout(cmd *cobra.Command, args []string) error {
	all, _ := cmd.Flags().GetBool("all")
	if all {
		workDir, err := os.Getwd()
		if err != nil {
			return fmt.Errorf("get working directory: %w", err)
		}
		return runRepoCheckoutAll(workDir)
	}

	repoURL := args[0]

	daemonPort := os.Getenv("MULTICA_DAEMON_PORT")
	if daemonPort == "" {
		return fmt.Errorf("MULTICA_DAEMON_PORT not set (this command is intended to be run by an agent inside a daemon task)")
	}

	workspaceID := os.Getenv("MULTICA_WORKSPACE_ID")
	agentName := os.Getenv("MULTICA_AGENT_NAME")
	taskID := os.Getenv("MULTICA_TASK_ID")

	// Use current working directory as the checkout target.
	workDir, err := os.Getwd()
	if err != nil {
		return fmt.Errorf("get working directory: %w", err)
	}

	reqBody := map[string]string{
		"url":           repoURL,
		"workspace_id":  workspaceID,
		"workdir":       workDir,
		"ref":           repoCheckoutRef,
		"agent_name":    agentName,
		"task_id":       taskID,
		"checkout_mode": strings.TrimSpace(os.Getenv("MULTICA_REPO_CHECKOUT_MODE")),
	}

	result, err := doCheckoutRequest(daemonPort, reqBody)
	if err != nil {
		return err
	}

	fmt.Fprintf(os.Stdout, "%s\n", result.Path)
	fmt.Fprintf(os.Stderr, "Checked out %s → %s (branch: %s)\n", repoURL, result.Path, result.BranchName)
	return nil
}

// projectResourcesFileForCheckout is the minimal shape of
// .multica/project/resources.json that --all needs to read.
type projectResourcesFileForCheckout struct {
	Resources []projectResourceEntryForCheckout `json:"resources"`
}

type projectResourceEntryForCheckout struct {
	ResourceType string          `json:"resource_type"`
	ResourceRef  json.RawMessage `json:"resource_ref"`
}

// githubRepoRefForCheckout is the JSONB shape stored in resource_ref for
// resource_type=github_repo entries, as written by the server's
// execenv.writeProjectResources.
type githubRepoRefForCheckout struct {
	URL string `json:"url"`
	Ref string `json:"ref,omitempty"`
}

// runRepoCheckoutAll reads .multica/project/resources.json from workDir,
// iterates every github_repo resource, and checks each one out via the daemon.
// It reports per-repo success or failure to stderr and exits non-zero if any
// checkout fails. It continues past individual failures so the caller sees the
// full picture in one run.
func runRepoCheckoutAll(workDir string) error {
	daemonPort := os.Getenv("MULTICA_DAEMON_PORT")
	if daemonPort == "" {
		return fmt.Errorf("MULTICA_DAEMON_PORT not set (this command is intended to be run by an agent inside a daemon task)")
	}

	workspaceID := os.Getenv("MULTICA_WORKSPACE_ID")
	agentName := os.Getenv("MULTICA_AGENT_NAME")
	taskID := os.Getenv("MULTICA_TASK_ID")
	checkoutMode := strings.TrimSpace(os.Getenv("MULTICA_REPO_CHECKOUT_MODE"))

	resourcesPath := filepath.Join(workDir, ".multica", "project", "resources.json")
	raw, err := os.ReadFile(resourcesPath)
	if err != nil {
		return fmt.Errorf("read %s: %w", resourcesPath, err)
	}

	var file projectResourcesFileForCheckout
	if err := json.Unmarshal(raw, &file); err != nil {
		return fmt.Errorf("parse %s: %w", resourcesPath, err)
	}

	// Collect only github_repo resources.
	type repoEntry struct {
		url string
		ref string
	}
	var repos []repoEntry
	for _, res := range file.Resources {
		if res.ResourceType != "github_repo" {
			continue
		}
		var ref githubRepoRefForCheckout
		if err := json.Unmarshal(res.ResourceRef, &ref); err != nil {
			return fmt.Errorf("parse github_repo resource_ref: %w", err)
		}
		if ref.URL == "" {
			continue
		}
		repos = append(repos, repoEntry{url: ref.URL, ref: ref.Ref})
	}

	if len(repos) == 0 {
		fmt.Fprintln(os.Stderr, "No github_repo resources found in resources.json.")
		return nil
	}

	var errs []error
	for _, repo := range repos {
		reqBody := map[string]string{
			"url":           repo.url,
			"workspace_id":  workspaceID,
			"workdir":       workDir,
			"ref":           repo.ref,
			"agent_name":    agentName,
			"task_id":       taskID,
			"checkout_mode": checkoutMode,
		}
		result, checkErr := doCheckoutRequest(daemonPort, reqBody)
		if checkErr != nil {
			fmt.Fprintf(os.Stderr, "FAIL  %s: %v\n", repo.url, checkErr)
			errs = append(errs, fmt.Errorf("%s: %w", repo.url, checkErr))
			continue
		}
		fmt.Fprintf(os.Stdout, "%s\n", result.Path)
		fmt.Fprintf(os.Stderr, "OK    %s → %s (branch: %s)\n", repo.url, result.Path, result.BranchName)
	}

	return errors.Join(errs...)
}
