package main

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"time"

	"github.com/multica-ai/multica/server/internal/daemon/processtree"
	"github.com/spf13/cobra"
)

const (
	repoToolStatusTimeout     = 10 * time.Second
	repoToolStatusOutputLimit = 64 * 1024
)

var repoToolStatusCmd = &cobra.Command{
	Use:   "tool-status gitnexus",
	Short: "Inspect installed repository tooling without preparing it",
	Long: "Inspect an installed GitNexus index using one bounded status --json call. " +
		"Does not install, initialize, analyze, or write repository/index files. " +
		"The installed tool may maintain its own runtime cache. Nonready outcomes are JSON results, not CLI errors.\n\n" +
		"When task-authorized and permitted by repository rules, the caller may make one targeted repair " +
		"and recheck. Otherwise report the constraint; never bypass required repository checks.",
	Args: func(cmd *cobra.Command, args []string) error {
		if err := cobra.ExactArgs(1)(cmd, args); err != nil {
			return err
		}
		if args[0] != "gitnexus" {
			return errors.New("tool-status supports only gitnexus")
		}
		return nil
	},
	RunE: runRepoToolStatus,
}

func init() {
	repoToolStatusCmd.Flags().String("path", ".", "Path within the Git checkout to inspect")
	repoToolStatusCmd.Flags().String("output", "json", "Output format: json")
	repoCmd.AddCommand(repoToolStatusCmd)
}

type repoToolStatusResult struct {
	SchemaVersion int    `json:"schema_version"`
	Tool          string `json:"tool"`
	Repository    string `json:"repository"`
	Status        string `json:"status"`
	Reason        string `json:"reason"`
}

// Only these versioned evidence fields can influence readiness. Diagnostic
// receipts, drifted filenames, and other subprocess data are never forwarded.
type gitNexusStatus struct {
	SchemaVersion int    `json:"schemaVersion"`
	Repository    string `json:"repository"`
	Status        string `json:"status"`
	Error         string `json:"error"`
	Index         *struct {
		Commit               string   `json:"commit"`
		RunnerIdentityStatus string   `json:"runnerIdentityStatus"`
		IncompleteReasons    []string `json:"incompleteReasons"`
	} `json:"index"`
	Current *struct {
		Commit string `json:"commit"`
	} `json:"current"`
	ContentDrift *struct {
		Status       string `json:"status"`
		CoveredFiles *int   `json:"coveredFiles"`
	} `json:"contentDrift"`
}

func runRepoToolStatus(cmd *cobra.Command, _ []string) error {
	output, _ := cmd.Flags().GetString("output")
	if output != "json" {
		return errors.New("tool-status supports only --output json")
	}
	path, _ := cmd.Flags().GetString("path")
	root, err := repoToolStatusRoot(cmd.Context(), path)
	if err != nil {
		return err
	}
	result := inspectGitNexus(cmd.Context(), root)
	return json.NewEncoder(cmd.OutOrStdout()).Encode(result)
}

func repoToolStatusRoot(ctx context.Context, path string) (string, error) {
	if path == "" {
		return "", errors.New("--path must name a directory within a Git checkout")
	}
	absolute, err := filepath.Abs(path)
	if err != nil {
		return "", errors.New("cannot resolve --path")
	}
	checkout, err := filepath.EvalSymlinks(absolute)
	if err != nil {
		return "", errors.New("--path must name an accessible directory")
	}
	info, err := os.Stat(checkout)
	if err != nil || !info.IsDir() {
		return "", errors.New("--path must name an accessible directory")
	}
	git, err := exec.LookPath("git")
	if err != nil {
		return "", errors.New("git executable is unavailable")
	}
	git, err = filepath.Abs(git)
	if err != nil {
		return "", errors.New("cannot resolve git executable")
	}
	command := exec.Command(git, "rev-parse", "--show-toplevel")
	command.Dir = checkout
	data, reason := captureRepoToolStatus(ctx, command, 3*time.Second)
	if reason != "" {
		return "", fmt.Errorf("cannot resolve --path as a Git working tree (%s)", reason)
	}
	// Remove only Git's record terminator, preserving spaces in directory names.
	root := strings.TrimSuffix(strings.TrimSuffix(string(data), "\n"), "\r")
	if !filepath.IsAbs(root) {
		return "", errors.New("git did not return an absolute checkout root")
	}
	root, err = filepath.EvalSymlinks(root)
	if err != nil {
		return "", errors.New("cannot resolve Git checkout root")
	}
	return filepath.Clean(root), nil
}

func inspectGitNexus(ctx context.Context, root string) repoToolStatusResult {
	result := repoToolStatusResult{
		SchemaVersion: 1,
		Tool:          "gitnexus",
		Repository:    root,
		Status:        "unavailable",
		Reason:        "executable_not_found",
	}
	executable, err := exec.LookPath("gitnexus")
	if err != nil {
		return result
	}
	// Resolve a relative PATH entry before changing the subprocess directory.
	executable, err = filepath.Abs(executable)
	if err != nil {
		result.Reason = "executable_unavailable"
		return result
	}
	command := exec.Command(executable, "status", "--json")
	command.Dir = root
	data, reason := captureRepoToolStatus(ctx, command, repoToolStatusTimeout)
	if reason != "" {
		result.Status = "failed"
		if reason == "json_status_unsupported" {
			result.Status = "unsupported"
		}
		result.Reason = reason
		return result
	}
	result.Status, result.Reason = classifyGitNexusStatus(data, root)
	return result
}

func classifyGitNexusStatus(data []byte, root string) (string, string) {
	var status gitNexusStatus
	if err := json.Unmarshal(data, &status); err != nil {
		return "unsupported", "invalid_status_output"
	}
	if status.SchemaVersion != 1 {
		return "unsupported", "unsupported_schema"
	}
	if status.Error == "not-git-repository" {
		return "failed", "repository_unavailable"
	}
	// Do not resolve paths from subprocess output: an unexpected path must not
	// cause reads of a different checkout, much less expose it in our report.
	if !filepath.IsAbs(status.Repository) || filepath.Clean(status.Repository) != root {
		return "failed", "repository_mismatch"
	}
	switch status.Error {
	case "not-indexed":
		return "not_indexed", "index_missing"
	case "stale-kuzu-index":
		return "stale", "legacy_index"
	case "":
	default:
		return "unsupported", "unknown_status_error"
	}
	if status.Status != "up-to-date" && status.Status != "stale" {
		return "unsupported", "unknown_index_status"
	}
	if status.Index == nil || status.Current == nil || status.ContentDrift == nil || status.Index.IncompleteReasons == nil {
		return "unsupported", "incomplete_status_output"
	}
	if status.Index.RunnerIdentityStatus != "current" && status.Index.RunnerIdentityStatus != "stale-or-unknown" {
		return "unsupported", "unknown_analyzer_status"
	}
	switch status.ContentDrift.Status {
	case "current", "drifted", "unmeasurable", "not-checked":
	default:
		return "unsupported", "unknown_content_status"
	}
	if strings.TrimSpace(status.Current.Commit) == "" || strings.TrimSpace(status.Index.Commit) == "" {
		return "stale", "revision_unknown"
	}
	if status.Current.Commit != status.Index.Commit {
		return "stale", "revision_changed"
	}
	// GitNexus validates the analyzer receipts itself. Do not replace that
	// verdict with a CLI version check or a second persistent identity cache.
	if status.Index.RunnerIdentityStatus != "current" {
		return "stale", "analyzer_changed_or_unknown"
	}
	if len(status.Index.IncompleteReasons) != 0 {
		return "stale", "incomplete_index"
	}
	switch status.ContentDrift.Status {
	case "drifted":
		return "stale", "content_changed"
	case "unmeasurable":
		return "stale", "content_unmeasurable"
	case "not-checked":
		return "stale", "content_not_checked"
	}
	if status.ContentDrift.CoveredFiles == nil || *status.ContentDrift.CoveredFiles < 0 {
		return "unsupported", "incomplete_content_evidence"
	}
	if status.Status != "up-to-date" {
		return "stale", "index_stale"
	}
	return "ready", "index_current"
}

// Capture only bounded stdout and a fixed stderr signal. Keep all Git commands
// tied to --path instead of inheriting another checkout's GIT_DIR/GIT_WORK_TREE.
func captureRepoToolStatus(parent context.Context, command *exec.Cmd, timeout time.Duration) ([]byte, string) {
	ctx, cancel := context.WithTimeout(parent, timeout)
	defer cancel()
	stdout := &repoToolStatusOutput{cancel: cancel}
	stderr := &repoToolStatusStderr{}
	command.Stdout = stdout
	command.Stderr = stderr
	for _, entry := range os.Environ() {
		if !strings.HasPrefix(entry, "GIT_") {
			command.Env = append(command.Env, entry)
		}
	}
	command.Env = append(command.Env, "GIT_OPTIONAL_LOCKS=0", "GIT_TERMINAL_PROMPT=0")
	err := processtree.Run(ctx, command, time.Second)
	if stdout.exceeded {
		return nil, "output_limit_exceeded"
	}
	if errors.Is(ctx.Err(), context.DeadlineExceeded) {
		return nil, "timed_out"
	}
	if ctx.Err() != nil {
		return nil, "cancelled"
	}
	if err != nil {
		if stderr.unsupported {
			return nil, "json_status_unsupported"
		}
		return nil, "status_command_failed"
	}
	return stdout.buffer.Bytes(), ""
}

type repoToolStatusOutput struct {
	buffer   bytes.Buffer
	cancel   context.CancelFunc
	exceeded bool
}

func (output *repoToolStatusOutput) Write(data []byte) (int, error) {
	n := len(data)
	remaining := repoToolStatusOutputLimit - output.buffer.Len()
	if n > remaining {
		data = data[:remaining]
		output.exceeded = true
		output.cancel()
	}
	_, _ = output.buffer.Write(data)
	return n, nil
}

// Recognize the legacy CLI's unsupported option without retaining stderr.
// Matching across writes prevents pipe chunking from changing the outcome.
type repoToolStatusStderr struct {
	positions   [2]int
	unsupported bool
}

func (output *repoToolStatusStderr) Write(data []byte) (int, error) {
	patterns := [...]string{"unknown option '--json'", "unknown option \"--json\""}
	for _, char := range data {
		if output.unsupported {
			break
		}
		for i, pattern := range patterns {
			if char == pattern[output.positions[i]] {
				output.positions[i]++
				if output.positions[i] == len(pattern) {
					output.unsupported = true
				}
			} else if char == pattern[0] {
				output.positions[i] = 1
			} else {
				output.positions[i] = 0
			}
		}
	}
	return len(data), nil
}
