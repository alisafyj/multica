package main

import (
	"context"
	"encoding/json"
	"fmt"
	"net/url"
	"os"
	"strings"

	"github.com/spf13/cobra"

	"github.com/multica-ai/multica/server/internal/cli"
)

var testcaseCmd = &cobra.Command{
	Use:   "testcase",
	Short: "Work with test cases",
}

var testcaseListCmd = &cobra.Command{
	Use:   "list",
	Short: "List test cases in the workspace",
	RunE:  runTestcaseList,
}

var testcaseGetCmd = &cobra.Command{
	Use:   "get <TC-42|id>",
	Short: "Get a test case, including its related repositories",
	Args:  exactArgs(1),
	RunE:  runTestcaseGet,
}

var testcaseModulesCmd = &cobra.Command{
	Use:   "modules",
	Short: "List modules and case counts for a project",
	RunE:  runTestcaseModules,
}

var testcaseCreateCmd = &cobra.Command{
	Use:   "create",
	Short: "Create a test case",
	RunE:  runTestcaseCreate,
}

var testcaseUpdateCmd = &cobra.Command{
	Use:   "update <TC-42|id>",
	Short: "Update a test case",
	Args:  exactArgs(1),
	RunE:  runTestcaseUpdate,
}

var testcaseApproveCmd = &cobra.Command{
	Use:   "approve <TC-42|id>",
	Short: "Approve a draft test case",
	Args:  exactArgs(1),
	RunE:  runTestcaseApprove,
}

var testcaseDeleteCmd = &cobra.Command{
	Use:   "delete <TC-42|id>",
	Short: "Delete a test case",
	Args:  exactArgs(1),
	RunE:  runTestcaseDelete,
}

func init() {
	testcaseCmd.AddCommand(
		testcaseListCmd,
		testcaseGetCmd,
		testcaseModulesCmd,
		testcaseCreateCmd,
		testcaseUpdateCmd,
		testcaseApproveCmd,
		testcaseDeleteCmd,
	)

	testcaseListCmd.Flags().String("project", "", "Filter by project id")
	testcaseListCmd.Flags().String("status", "", "Filter by status: draft, active, deprecated")
	testcaseListCmd.Flags().String("module", "", "Filter by module")
	testcaseListCmd.Flags().String("priority", "", "Filter by priority: p0, p1, p2, p3")
	testcaseListCmd.Flags().String("case-type", "", "Filter by case type")
	testcaseListCmd.Flags().String("origin", "", "Filter by origin: ai, human")
	testcaseListCmd.Flags().Bool("digest", false, "Omit steps and test data — a compact index for agent context")
	testcaseListCmd.Flags().String("output", "table", "Output format: table or json")
	testcaseListCmd.Flags().Bool("full-id", false, "Show full UUIDs in table output")

	testcaseGetCmd.Flags().String("output", "json", "Output format: table or json")

	testcaseModulesCmd.Flags().String("project", "", "Project id (required)")
	testcaseModulesCmd.Flags().String("output", "table", "Output format: table or json")

	for _, cmd := range []*cobra.Command{testcaseCreateCmd, testcaseUpdateCmd} {
		cmd.Flags().String("title", "", "Test case title")
		cmd.Flags().String("module", "", "Module used for grouping")
		cmd.Flags().String("priority", "", "Priority: p0, p1, p2, p3")
		cmd.Flags().String("case-type", "", "Case type, e.g. functional, business_flow, boundary")
		cmd.Flags().String("scope", "", "Scope: single_repo, cross_repo, no_repo")
		cmd.Flags().String("execution-mode", "", "Execution mode: manual, agent, both")
		cmd.Flags().String("preconditions", "", "Preconditions (decodes \\n, \\r, \\t, \\\\; pipe via --preconditions-stdin to preserve literal backslashes)")
		cmd.Flags().String("expected", "", "Overall expected result (decodes \\n, \\r, \\t, \\\\; pipe via --expected-stdin to preserve literal backslashes)")
		cmd.Flags().String("steps", "", `Steps as a JSON array: [{"index":1,"action":"…","expected":"…"}]`)
		cmd.Flags().String("output", "json", "Output format: table or json")
	}
	testcaseCreateCmd.Flags().String("project", "", "Project id (required)")
	testcaseUpdateCmd.Flags().String("status", "", "Status: draft, active, deprecated")

	testcaseApproveCmd.Flags().String("output", "json", "Output format: table or json")
}

// testcaseDigestFields are the keys kept by --digest. A generation task needs to
// know which cases already exist without paying for every step body, so the
// digest is title-level metadata only.
var testcaseDigestFields = []string{
	"id", "key", "case_number", "title", "module", "case_type",
	"priority", "status", "origin", "scope", "updated_at",
}

func digestTestCase(testCase map[string]any) map[string]any {
	digest := make(map[string]any, len(testcaseDigestFields))
	for _, field := range testcaseDigestFields {
		if value, ok := testCase[field]; ok {
			digest[field] = value
		}
	}
	return digest
}

// formatTestcaseRepos renders repo bindings as "alias(role)" so the table shows
// at a glance which systems a case spans.
func formatTestcaseRepos(testCase map[string]any) string {
	raw, _ := testCase["repos"].([]any)
	parts := make([]string, 0, len(raw))
	for _, entry := range raw {
		repo, ok := entry.(map[string]any)
		if !ok {
			continue
		}
		parts = append(parts, fmt.Sprintf("%s(%s)", strVal(repo, "alias"), strVal(repo, "role")))
	}
	return strings.Join(parts, ", ")
}

func runTestcaseList(cmd *cobra.Command, _ []string) error {
	client, err := newAPIClient(cmd)
	if err != nil {
		return err
	}
	ctx, cancel := cli.APIContext(context.Background())
	defer cancel()

	params := url.Values{}
	if client.WorkspaceID != "" {
		params.Set("workspace_id", client.WorkspaceID)
	}
	for flag, query := range map[string]string{
		"project":   "project_id",
		"status":    "status",
		"module":    "module",
		"priority":  "priority",
		"case-type": "case_type",
		"origin":    "origin",
	} {
		if value, _ := cmd.Flags().GetString(flag); value != "" {
			params.Set(query, value)
		}
	}

	path := "/api/test-cases"
	if len(params) > 0 {
		path += "?" + params.Encode()
	}

	var result map[string]any
	if err := client.GetJSON(ctx, path, &result); err != nil {
		return fmt.Errorf("list test cases: %w", err)
	}
	casesRaw, _ := result["test_cases"].([]any)

	digest, _ := cmd.Flags().GetBool("digest")
	output, _ := cmd.Flags().GetString("output")
	if output == "json" {
		if !digest {
			return cli.PrintJSON(os.Stdout, casesRaw)
		}
		digested := make([]map[string]any, 0, len(casesRaw))
		for _, raw := range casesRaw {
			testCase, ok := raw.(map[string]any)
			if !ok {
				continue
			}
			digested = append(digested, digestTestCase(testCase))
		}
		return cli.PrintJSON(os.Stdout, digested)
	}

	fullID, _ := cmd.Flags().GetBool("full-id")
	headers := []string{"KEY", "TITLE", "MODULE", "TYPE", "PRIO", "STATUS", "ORIGIN", "REPOS"}
	if fullID {
		headers = append([]string{"ID"}, headers...)
	}
	rows := make([][]string, 0, len(casesRaw))
	for _, raw := range casesRaw {
		testCase, ok := raw.(map[string]any)
		if !ok {
			continue
		}
		row := []string{
			strVal(testCase, "key"),
			strVal(testCase, "title"),
			strVal(testCase, "module"),
			strVal(testCase, "case_type"),
			strVal(testCase, "priority"),
			strVal(testCase, "status"),
			strVal(testCase, "origin"),
			formatTestcaseRepos(testCase),
		}
		if fullID {
			row = append([]string{strVal(testCase, "id")}, row...)
		}
		rows = append(rows, row)
	}
	cli.PrintTable(os.Stdout, headers, rows)
	return nil
}

func runTestcaseGet(cmd *cobra.Command, args []string) error {
	client, err := newAPIClient(cmd)
	if err != nil {
		return err
	}
	ctx, cancel := cli.APIContext(context.Background())
	defer cancel()

	// The ref goes to the server verbatim: TC-<n> keys are resolved there, so
	// the CLI never needs a local prefix index the way short UUIDs would.
	var testCase map[string]any
	if err := client.GetJSON(ctx, testcasePath(client, args[0], ""), &testCase); err != nil {
		return fmt.Errorf("get test case: %w", err)
	}

	output, _ := cmd.Flags().GetString("output")
	if output != "table" {
		return cli.PrintJSON(os.Stdout, testCase)
	}
	cli.PrintTable(os.Stdout,
		[]string{"FIELD", "VALUE"},
		[][]string{
			{"key", strVal(testCase, "key")},
			{"title", strVal(testCase, "title")},
			{"module", strVal(testCase, "module")},
			{"type", strVal(testCase, "case_type")},
			{"priority", strVal(testCase, "priority")},
			{"status", strVal(testCase, "status")},
			{"origin", strVal(testCase, "origin")},
			{"scope", strVal(testCase, "scope")},
			{"execution_mode", strVal(testCase, "execution_mode")},
			{"repos", formatTestcaseRepos(testCase)},
		})
	return nil
}

func runTestcaseModules(cmd *cobra.Command, _ []string) error {
	client, err := newAPIClient(cmd)
	if err != nil {
		return err
	}
	projectID, _ := cmd.Flags().GetString("project")
	if projectID == "" {
		return fmt.Errorf("--project is required")
	}
	ctx, cancel := cli.APIContext(context.Background())
	defer cancel()

	params := url.Values{}
	if client.WorkspaceID != "" {
		params.Set("workspace_id", client.WorkspaceID)
	}
	params.Set("project_id", projectID)

	var result map[string]any
	if err := client.GetJSON(ctx, "/api/test-cases/modules?"+params.Encode(), &result); err != nil {
		return fmt.Errorf("list test case modules: %w", err)
	}
	modulesRaw, _ := result["modules"].([]any)

	output, _ := cmd.Flags().GetString("output")
	if output == "json" {
		return cli.PrintJSON(os.Stdout, modulesRaw)
	}
	rows := make([][]string, 0, len(modulesRaw))
	for _, raw := range modulesRaw {
		module, ok := raw.(map[string]any)
		if !ok {
			continue
		}
		count := ""
		if value, ok := module["case_count"].(float64); ok {
			count = fmt.Sprintf("%d", int64(value))
		}
		rows = append(rows, []string{strVal(module, "module"), count})
	}
	cli.PrintTable(os.Stdout, []string{"MODULE", "CASES"}, rows)
	return nil
}

// testcasePath builds a workspace-scoped path for one case ref, with an
// optional trailing action segment.
func testcasePath(client *cli.APIClient, ref, action string) string {
	path := "/api/test-cases/" + url.PathEscape(ref)
	if action != "" {
		path += "/" + action
	}
	if client.WorkspaceID != "" {
		path += "?workspace_id=" + url.QueryEscape(client.WorkspaceID)
	}
	return path
}

// testcaseBodyFromFlags collects the flags the caller actually set. Every field
// is gated on Changed() so `--module ""` reaches the server as a clear rather
// than being indistinguishable from an omitted flag.
func testcaseBodyFromFlags(cmd *cobra.Command) (map[string]any, error) {
	body := map[string]any{}
	for flag, field := range map[string]string{
		"title":          "title",
		"module":         "module",
		"priority":       "priority",
		"case-type":      "case_type",
		"scope":          "scope",
		"execution-mode": "execution_mode",
		"status":         "status",
	} {
		if cmd.Flags().Lookup(flag) == nil || !cmd.Flags().Changed(flag) {
			continue
		}
		value, _ := cmd.Flags().GetString(flag)
		body[field] = value
	}
	for flag, field := range map[string]string{
		"preconditions": "preconditions",
		"expected":      "expected_result",
	} {
		value, provided, err := resolveTextFlag(cmd, flag)
		if err != nil {
			return nil, err
		}
		if provided {
			body[field] = value
		}
	}
	if cmd.Flags().Changed("steps") {
		raw, _ := cmd.Flags().GetString("steps")
		var steps []map[string]any
		if err := json.Unmarshal([]byte(raw), &steps); err != nil {
			return nil, fmt.Errorf("--steps must be valid JSON: %w", err)
		}
		body["steps"] = steps
	}
	return body, nil
}

func runTestcaseCreate(cmd *cobra.Command, _ []string) error {
	client, err := newAPIClient(cmd)
	if err != nil {
		return err
	}
	if client.WorkspaceID == "" {
		if _, err := requireWorkspaceID(cmd); err != nil {
			return err
		}
	}
	projectID, _ := cmd.Flags().GetString("project")
	if projectID == "" {
		return fmt.Errorf("--project is required")
	}
	title, _ := cmd.Flags().GetString("title")
	if strings.TrimSpace(title) == "" {
		return fmt.Errorf("--title is required")
	}
	body, err := testcaseBodyFromFlags(cmd)
	if err != nil {
		return err
	}
	body["project_id"] = projectID
	body["title"] = title

	ctx, cancel := cli.APIContext(context.Background())
	defer cancel()

	path := "/api/test-cases"
	if client.WorkspaceID != "" {
		path += "?workspace_id=" + url.QueryEscape(client.WorkspaceID)
	}
	var created map[string]any
	if err := client.PostJSON(ctx, path, body, &created); err != nil {
		return fmt.Errorf("create test case: %w", err)
	}
	output, _ := cmd.Flags().GetString("output")
	if output == "table" {
		fmt.Fprintf(os.Stderr, "Created %s\n", strVal(created, "key"))
		return nil
	}
	return cli.PrintJSON(os.Stdout, created)
}

func runTestcaseUpdate(cmd *cobra.Command, args []string) error {
	client, err := newAPIClient(cmd)
	if err != nil {
		return err
	}
	body, err := testcaseBodyFromFlags(cmd)
	if err != nil {
		return err
	}
	if len(body) == 0 {
		return fmt.Errorf("no fields to update; use flags like --title, --status, --steps")
	}

	ctx, cancel := cli.APIContext(context.Background())
	defer cancel()

	var updated map[string]any
	if err := client.PutJSON(ctx, testcasePath(client, args[0], ""), body, &updated); err != nil {
		return fmt.Errorf("update test case: %w", err)
	}
	output, _ := cmd.Flags().GetString("output")
	if output == "table" {
		fmt.Fprintf(os.Stderr, "Updated %s\n", strVal(updated, "key"))
		return nil
	}
	return cli.PrintJSON(os.Stdout, updated)
}

func runTestcaseApprove(cmd *cobra.Command, args []string) error {
	client, err := newAPIClient(cmd)
	if err != nil {
		return err
	}
	ctx, cancel := cli.APIContext(context.Background())
	defer cancel()

	var approved map[string]any
	if err := client.PostJSON(ctx, testcasePath(client, args[0], "approve"), map[string]any{}, &approved); err != nil {
		return fmt.Errorf("approve test case: %w", err)
	}
	output, _ := cmd.Flags().GetString("output")
	if output == "table" {
		fmt.Fprintf(os.Stderr, "Approved %s\n", strVal(approved, "key"))
		return nil
	}
	return cli.PrintJSON(os.Stdout, approved)
}

func runTestcaseDelete(cmd *cobra.Command, args []string) error {
	client, err := newAPIClient(cmd)
	if err != nil {
		return err
	}
	ctx, cancel := cli.APIContext(context.Background())
	defer cancel()

	if err := client.DeleteJSON(ctx, testcasePath(client, args[0], "")); err != nil {
		return fmt.Errorf("delete test case: %w", err)
	}
	fmt.Fprintf(os.Stderr, "Test case %s deleted.\n", args[0])
	return nil
}
