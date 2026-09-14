package main

import (
	"context"
	"errors"
	"fmt"
	"io"
	"os"
	"os/exec"
	"strings"

	"github.com/spf13/cobra"

	"github.com/multica-ai/multica/server/internal/cli"
)

var testcaseRecommendCmd = &cobra.Command{
	Use:   "recommend [path ...]",
	Short: "Which cases claim the changed files (change-based regression selection)",
	Long: `Match changed paths against every case's repo bindings (path_globs) and list
the cases that claim them, most-claimed first. Paths come from the arguments,
from --diff <ref> (git diff --name-only <ref> in the current directory), or
from stdin with --stdin, one per line. --run <title> creates a test run over
the recommended cases in the same step and prints it.`,
	RunE: runTestcaseRecommend,
}

func init() {
	testcaseCmd.AddCommand(testcaseRecommendCmd)
	testcaseRecommendCmd.Flags().String("project", "", "Project id (required)")
	testcaseRecommendCmd.Flags().String("repo", "", "Only bindings with this alias (the paths belong to one repository of a multi-repo project)")
	testcaseRecommendCmd.Flags().String("diff", "", "Git ref to diff the working tree against, e.g. origin/main")
	testcaseRecommendCmd.Flags().Bool("stdin", false, "Read paths from stdin, one per line")
	testcaseRecommendCmd.Flags().String("run", "", "Create a test run with this title over the recommended cases")
	testcaseRecommendCmd.Flags().Int("parallelism", 0, "With --run: cap on concurrently dispatched cases (0 = no cap)")
	testcaseRecommendCmd.Flags().String("output", "table", "Output format: table or json")
	testcaseRecommendCmd.Flags().Bool("full-id", false, "Show full UUIDs in table output")
}

func runTestcaseRecommend(cmd *cobra.Command, args []string) error {
	projectID, _ := cmd.Flags().GetString("project")
	if projectID == "" {
		return fmt.Errorf("--project is required")
	}
	paths := append([]string{}, args...)
	if ref, _ := cmd.Flags().GetString("diff"); ref != "" {
		out, err := exec.Command("git", "diff", "--name-only", ref).Output()
		if err != nil {
			return fmt.Errorf("git diff --name-only %s: %w", ref, err)
		}
		paths = append(paths, splitPathLines(string(out))...)
	}
	if fromStdin, _ := cmd.Flags().GetBool("stdin"); fromStdin {
		data, err := readAllStdin()
		if err != nil {
			return err
		}
		paths = append(paths, splitPathLines(data)...)
	}
	paths = dedupePaths(paths)
	if len(paths) == 0 {
		return fmt.Errorf("no paths given: pass them as arguments, --diff <ref>, or --stdin")
	}

	client, err := newAPIClient(cmd)
	if err != nil {
		return err
	}
	ctx, cancel := cli.APIContext(context.Background())
	defer cancel()

	body := map[string]any{"project_id": projectID, "paths": paths}
	if repo, _ := cmd.Flags().GetString("repo"); repo != "" {
		body["repo"] = repo
	}
	path := "/api/test-cases/recommend"
	if client.WorkspaceID != "" {
		path += "?workspace_id=" + client.WorkspaceID
	}
	var result map[string]any
	if err := client.PostJSON(ctx, path, body, &result); err != nil {
		return fmt.Errorf("recommend test cases: %w", err)
	}

	casesRaw, _ := result["cases"].([]any)
	if runTitle, _ := cmd.Flags().GetString("run"); runTitle != "" {
		ids := recommendedCaseIDs(casesRaw)
		if len(ids) == 0 {
			return fmt.Errorf("no case claims these paths; nothing to run")
		}
		runBody := map[string]any{"title": runTitle, "test_case_ids": ids}
		if parallelism, _ := cmd.Flags().GetInt("parallelism"); parallelism > 0 {
			runBody["parallelism"] = parallelism
		}
		runPath := "/api/test-runs"
		if client.WorkspaceID != "" {
			runPath += "?workspace_id=" + client.WorkspaceID
		}
		var run map[string]any
		if err := client.PostJSON(ctx, runPath, runBody, &run); err != nil {
			return fmt.Errorf("create test run: %w", err)
		}
		result["run"] = run
	}

	output, _ := cmd.Flags().GetString("output")
	if output == "json" {
		return cli.PrintJSON(os.Stdout, result)
	}
	fullID, _ := cmd.Flags().GetBool("full-id")
	rows := make([][]string, 0, len(casesRaw))
	for _, raw := range casesRaw {
		rec, ok := raw.(map[string]any)
		if !ok {
			continue
		}
		testCase, _ := rec["test_case"].(map[string]any)
		id := strVal(testCase, "id")
		if !fullID && len(id) > 8 {
			id = id[:8]
		}
		count := ""
		if value, ok := rec["path_count"].(float64); ok {
			count = fmt.Sprintf("%d", int64(value))
		}
		var globs []string
		if matches, ok := rec["matches"].([]any); ok {
			for _, m := range matches {
				match, _ := m.(map[string]any)
				globs = append(globs, strVal(match, "alias")+":"+strVal(match, "glob"))
			}
		}
		rows = append(rows, []string{strVal(testCase, "key"), id, strVal(testCase, "title"), strVal(testCase, "module"), strVal(testCase, "status"), count, strings.Join(globs, ", ")})
	}
	cli.PrintTable(os.Stdout, []string{"KEY", "ID", "TITLE", "MODULE", "STATUS", "PATHS", "CLAIMED BY"}, rows)
	if unmatched, ok := result["unmatched_paths"].([]any); ok && len(unmatched) > 0 {
		fmt.Fprintf(os.Stdout, "\n%d of %d paths are claimed by no case.\n", len(unmatched), len(paths))
	}
	if run, ok := result["run"].(map[string]any); ok {
		fmt.Fprintf(os.Stdout, "\nCreated test run %s (%s) over %d cases; start it with: multica test run start %s\n", strVal(run, "id"), strVal(run, "status"), len(casesRaw), strVal(run, "id"))
	}
	return nil
}

// recommendedCaseIDs keeps the server's ranking order.
func recommendedCaseIDs(casesRaw []any) []string {
	ids := make([]string, 0, len(casesRaw))
	for _, raw := range casesRaw {
		rec, _ := raw.(map[string]any)
		testCase, _ := rec["test_case"].(map[string]any)
		if id := strVal(testCase, "id"); id != "" {
			ids = append(ids, id)
		}
	}
	return ids
}

func readAllStdin() (string, error) {
	var b strings.Builder
	buf := make([]byte, 64*1024)
	for {
		n, err := os.Stdin.Read(buf)
		b.Write(buf[:n])
		if err != nil {
			if errors.Is(err, io.EOF) {
				return b.String(), nil
			}
			return "", fmt.Errorf("read stdin: %w", err)
		}
	}
}

// splitPathLines turns "one path per line" text into paths, tolerating CRLF,
// blank lines and the "M\tpath" shape of git status --porcelain.
func splitPathLines(text string) []string {
	var out []string
	for _, line := range strings.Split(text, "\n") {
		line = strings.TrimSpace(strings.TrimSuffix(line, "\r"))
		if line == "" {
			continue
		}
		if fields := strings.Fields(line); len(fields) == 2 && len(fields[0]) <= 2 && !strings.Contains(fields[0], "/") && !strings.Contains(fields[0], ".") {
			line = fields[1]
		}
		out = append(out, line)
	}
	return out
}

func dedupePaths(paths []string) []string {
	seen := map[string]bool{}
	out := make([]string, 0, len(paths))
	for _, p := range paths {
		p = strings.TrimSpace(p)
		if p == "" || seen[p] {
			continue
		}
		seen[p] = true
		out = append(out, p)
	}
	return out
}
