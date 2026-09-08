package daemon

import (
	"bytes"
	"encoding/json"
	"fmt"
)

// Inputs have already passed the appropriate privacy/broker gates. Agent and
// remote-owned names stay reserved even if their entries were filtered out.
func mergeSelectedProjectMCP(base, project, explicitAgent, remote json.RawMessage) (json.RawMessage, error) {
	if len(project) == 0 {
		return base, nil
	}
	parse := func(raw json.RawMessage) (map[string]json.RawMessage, error) {
		var doc struct {
			Servers map[string]json.RawMessage `json:"mcpServers"`
		}
		if len(bytes.TrimSpace(raw)) > 0 {
			if err := json.Unmarshal(raw, &doc); err != nil {
				return nil, fmt.Errorf("invalid managed MCP configuration")
			}
		}
		if doc.Servers == nil {
			doc.Servers = make(map[string]json.RawMessage)
		}
		return doc.Servers, nil
	}
	merged, err := parse(base)
	if err != nil {
		return nil, err
	}
	selected, err := parse(project)
	if err != nil {
		return nil, err
	}
	reserved := make(map[string]bool)
	for _, raw := range []json.RawMessage{explicitAgent, remote} {
		servers, err := parse(raw)
		if err != nil {
			return nil, err
		}
		for name := range servers {
			reserved[name] = true
		}
	}
	for name, entry := range selected {
		if !reserved[name] {
			merged[name] = entry
		}
	}
	return json.Marshal(map[string]any{"mcpServers": merged})
}

func requireSelectedProjectMCP(policy projectRuntimePolicy) error {
	for _, diagnostic := range policy.MCPDiagnostics {
		if diagnostic.Status != "selected" {
			return fmt.Errorf("selected project MCP server %q is %s; update the project's configuration trust or MCP selection before retrying", diagnostic.Name, diagnostic.Status)
		}
	}
	return nil
}
