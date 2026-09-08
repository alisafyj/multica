package daemon

import (
	"encoding/json"
	"strings"
	"testing"
)

func TestMergeSelectedProjectMCPPrecedence(t *testing.T) {
	base := json.RawMessage(`{"mcpServers":{"runtime":{"command":"base"},"shared":{"command":"runtime"},"agent":{"command":"agent"},"remote":{"url":"http://127.0.0.1:1234/mcp"}}}`)
	project := json.RawMessage(`{"mcpServers":{"shared":{"command":"project"},"new":{"command":"project"},"agent":{"command":"project"},"remote":{"command":"project"}}}`)
	got, err := mergeSelectedProjectMCP(base, project,
		json.RawMessage(`{"mcpServers":{"agent":{"command":"agent"}}}`),
		json.RawMessage(`{"mcpServers":{"remote":{"url":"http://127.0.0.1:1234/mcp"}}}`))
	if err != nil {
		t.Fatal(err)
	}
	var doc struct {
		Servers map[string]map[string]any `json:"mcpServers"`
	}
	if err := json.Unmarshal(got, &doc); err != nil {
		t.Fatal(err)
	}
	if len(doc.Servers) != 5 || doc.Servers["shared"]["command"] != "project" || doc.Servers["agent"]["command"] != "agent" || doc.Servers["remote"]["url"] == nil {
		t.Fatalf("wrong precedence: %s", got)
	}
	if !strings.Contains(string(base), `"command":"runtime"`) {
		t.Fatal("input changed")
	}
}

func TestMergeSelectedProjectMCPPreservesExplicitlyBlockedAgentNames(t *testing.T) {
	got, err := mergeSelectedProjectMCP(json.RawMessage(`{"mcpServers":{}}`),
		json.RawMessage(`{"mcpServers":{"blocked":{"command":"project"}}}`),
		json.RawMessage(`{"mcpServers":{"blocked":{"command":"unapproved"}}}`), nil)
	if err != nil {
		t.Fatal(err)
	}
	if string(got) != `{"mcpServers":{}}` {
		t.Fatalf("project resurrected blocked explicit config: %s", got)
	}
}

func TestProjectMCPReadinessIsActionableAndRedacted(t *testing.T) {
	for _, status := range []string{"missing", "disabled", "blocked_privacy", "unsupported_provider", "trust_required"} {
		err := requireSelectedProjectMCP(projectRuntimePolicy{MCPDiagnostics: []projectMCPDiagnostic{{Name: "docs", Status: status}}})
		if err == nil || !strings.Contains(err.Error(), "docs") || !strings.Contains(err.Error(), status) {
			t.Fatalf("status %s: %v", status, err)
		}
	}
	if err := requireSelectedProjectMCP(projectRuntimePolicy{MCPDiagnostics: []projectMCPDiagnostic{{Name: "docs", Status: "selected"}}}); err != nil {
		t.Fatal(err)
	}
}
