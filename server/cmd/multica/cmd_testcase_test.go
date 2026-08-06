package main

import (
	"strings"
	"testing"

	"github.com/spf13/cobra"

	"github.com/multica-ai/multica/server/internal/cli"
)

// newTestcaseWriteTestCmd mirrors the flag surface shared by
// testcaseCreateCmd / testcaseUpdateCmd so the body builder can be exercised
// without a server.
func newTestcaseWriteTestCmd() *cobra.Command {
	c := &cobra.Command{Use: "write"}
	c.Flags().String("title", "", "")
	c.Flags().String("module", "", "")
	c.Flags().String("priority", "", "")
	c.Flags().String("case-type", "", "")
	c.Flags().String("scope", "", "")
	c.Flags().String("execution-mode", "", "")
	c.Flags().String("status", "", "")
	c.Flags().String("preconditions", "", "")
	c.Flags().String("preconditions-stdin", "", "")
	c.Flags().String("expected", "", "")
	c.Flags().String("steps", "", "")
	c.Flags().String("output", "json", "")
	return c
}

func TestTestcaseBodyOnlyIncludesChangedFlags(t *testing.T) {
	cmd := newTestcaseWriteTestCmd()
	if err := cmd.Flags().Set("module", "订单"); err != nil {
		t.Fatalf("set module: %v", err)
	}

	body, err := testcaseBodyFromFlags(cmd)
	if err != nil {
		t.Fatalf("build body: %v", err)
	}
	if len(body) != 1 {
		t.Fatalf("body = %v, want only the module field", body)
	}
	if body["module"] != "订单" {
		t.Fatalf("module = %v, want 订单", body["module"])
	}
}

func TestTestcaseBodyTreatsEmptyStringAsAClear(t *testing.T) {
	cmd := newTestcaseWriteTestCmd()
	if err := cmd.Flags().Set("module", ""); err != nil {
		t.Fatalf("set module: %v", err)
	}

	body, err := testcaseBodyFromFlags(cmd)
	if err != nil {
		t.Fatalf("build body: %v", err)
	}
	value, present := body["module"]
	if !present {
		t.Fatal("an explicitly empty --module must reach the server as a clear")
	}
	if value != "" {
		t.Fatalf("module = %v, want the empty string", value)
	}
}

func TestTestcaseBodyRejectsMalformedSteps(t *testing.T) {
	cmd := newTestcaseWriteTestCmd()
	if err := cmd.Flags().Set("steps", "not-json"); err != nil {
		t.Fatalf("set steps: %v", err)
	}

	_, err := testcaseBodyFromFlags(cmd)
	if err == nil {
		t.Fatal("malformed --steps should fail client-side")
	}
	if !strings.Contains(err.Error(), "--steps must be valid JSON") {
		t.Fatalf("error should name the flag, got: %v", err)
	}
}

func TestTestcaseBodyParsesSteps(t *testing.T) {
	cmd := newTestcaseWriteTestCmd()
	if err := cmd.Flags().Set("steps", `[{"index":1,"action":"点击下单","expected":"跳转支付页"}]`); err != nil {
		t.Fatalf("set steps: %v", err)
	}

	body, err := testcaseBodyFromFlags(cmd)
	if err != nil {
		t.Fatalf("build body: %v", err)
	}
	steps, ok := body["steps"].([]map[string]any)
	if !ok || len(steps) != 1 {
		t.Fatalf("steps = %#v, want one parsed step", body["steps"])
	}
	if steps[0]["action"] != "点击下单" {
		t.Fatalf("action = %v", steps[0]["action"])
	}
}

func TestTestcasePathSendsTheRefVerbatim(t *testing.T) {
	client := &cli.APIClient{WorkspaceID: "ws-1"}

	got := testcasePath(client, "TC-42", "")
	if got != "/api/test-cases/TC-42?workspace_id=ws-1" {
		t.Fatalf("path = %q", got)
	}

	got = testcasePath(client, "TC-42", "approve")
	if got != "/api/test-cases/TC-42/approve?workspace_id=ws-1" {
		t.Fatalf("approve path = %q", got)
	}
}

func TestTestcasePathEscapesTheRef(t *testing.T) {
	client := &cli.APIClient{WorkspaceID: "ws-1"}
	got := testcasePath(client, "TC 42/../secrets", "")
	if strings.Contains(got, "..") && !strings.Contains(got, "%2F") {
		t.Fatalf("ref must be escaped, got %q", got)
	}
}

func TestDigestTestCaseOmitsStepBodies(t *testing.T) {
	full := map[string]any{
		"id":          "case-1",
		"key":         "TC-1",
		"title":       "下单成功",
		"module":      "订单",
		"status":      "active",
		"steps":       []any{map[string]any{"action": "点击下单"}},
		"test_data":   map[string]any{"amount": 129},
		"source_refs": map[string]any{"issue_ids": []any{"MUL-1"}},
	}

	digest := digestTestCase(full)

	if _, present := digest["steps"]; present {
		t.Fatal("digest must not carry step bodies")
	}
	if _, present := digest["test_data"]; present {
		t.Fatal("digest must not carry test data")
	}
	if digest["key"] != "TC-1" || digest["title"] != "下单成功" || digest["module"] != "订单" {
		t.Fatalf("digest lost identifying fields: %v", digest)
	}
}

func TestFormatTestcaseRepos(t *testing.T) {
	summary := formatTestcaseRepos(map[string]any{
		"repos": []any{
			map[string]any{"alias": "admin-web", "role": "driver"},
			map[string]any{"alias": "mobile-app", "role": "verifier"},
		},
	})
	if summary != "admin-web(driver), mobile-app(verifier)" {
		t.Fatalf("summary = %q", summary)
	}

	if got := formatTestcaseRepos(map[string]any{}); got != "" {
		t.Fatalf("missing repos should render empty, got %q", got)
	}
}
