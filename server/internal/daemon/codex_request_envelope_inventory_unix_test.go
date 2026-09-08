//go:build !windows

package daemon

import (
	"encoding/json"
	"strings"
	"testing"
)

const (
	codexRequestEnvelopeEntryLimit   = 64
	codexRequestEnvelopeSectionLimit = 128
)

type codexRequestEnvelopeInventory struct {
	Entries            []codexRequestEnvelopeInventoryEntry `json:"entries,omitempty"`
	EntryCount         int                                  `json:"entry_count"`
	EntryLimit         int                                  `json:"entry_limit"`
	EntriesIncomplete  bool                                 `json:"entries_incomplete"`
	EntryBytes         int                                  `json:"entry_bytes"`
	ArrayOverhead      int                                  `json:"json_array_overhead_bytes"`
	AccountedBytes     int                                  `json:"accounted_bytes"`
	DecodedTextCount   int                                  `json:"decoded_text_count"`
	DecodedTextBytes   int                                  `json:"decoded_text_bytes"`
	Sections           []codexRequestEnvelopeTextSection    `json:"text_sections,omitempty"`
	SectionCount       int                                  `json:"text_section_count"`
	SectionLimit       int                                  `json:"text_section_limit"`
	SectionsIncomplete bool                                 `json:"text_sections_incomplete"`
}

type codexRequestEnvelopeInventoryEntry struct {
	Ordinal int    `json:"ordinal"`
	Type    string `json:"type"`
	Role    string `json:"role,omitempty"`
	Name    string `json:"name,omitempty"`
	codexRequestEnvelopePart
	Schema codexRequestEnvelopePart `json:"schema,omitempty"`
}

type codexRequestEnvelopeTextSection struct {
	TextOrdinal    int    `json:"text_ordinal"`
	SectionOrdinal int    `json:"section_ordinal"`
	Label          string `json:"label"`
	HeadingBytes   int    `json:"heading_bytes"`
	codexRequestEnvelopePart
}

func codexRequestEnvelopeInputInventory(t *testing.T, input []any) *codexRequestEnvelopeInventory {
	t.Helper()
	inv := newCodexRequestEnvelopeInventory(len(input))
	if input == nil {
		inv.ArrayOverhead = len("null")
	}
	textOrdinal := 0
	for ordinal, raw := range input {
		encoded := codexRequestEnvelopeJSON(t, raw)
		inv.EntryBytes += len(encoded)
		item, _ := raw.(map[string]any)
		if len(inv.Entries) < inv.EntryLimit {
			inv.Entries = append(inv.Entries, codexRequestEnvelopeInventoryEntry{
				Ordinal: ordinal, Type: codexRequestEnvelopeInputType(item), Role: codexRequestEnvelopeRoleLabel(item),
				codexRequestEnvelopePart: codexRequestEnvelopePart{Bytes: len(encoded), SHA256: codexRequestEnvelopeDigest(encoded)},
			})
		}
		for _, text := range codexRequestEnvelopeDecodedTexts(item) {
			inv.DecodedTextCount++
			inv.DecodedTextBytes += len(text)
			sections := codexRequestEnvelopeSplitText(text, textOrdinal)
			inv.SectionCount += len(sections)
			for _, section := range sections {
				if len(inv.Sections) < codexRequestEnvelopeSectionLimit {
					inv.Sections = append(inv.Sections, section)
				}
			}
			textOrdinal++
		}
	}
	inv.finish()
	inv.SectionLimit = codexRequestEnvelopeSectionLimit
	inv.SectionsIncomplete = inv.SectionCount > len(inv.Sections)
	return inv
}

func codexRequestEnvelopeToolInventory(t *testing.T, tools []any) *codexRequestEnvelopeInventory {
	t.Helper()
	inv := newCodexRequestEnvelopeInventory(len(tools))
	if tools == nil {
		inv.ArrayOverhead = len("null")
	}
	for ordinal, raw := range tools {
		encoded := codexRequestEnvelopeJSON(t, raw)
		inv.EntryBytes += len(encoded)
		tool, _ := raw.(map[string]any)
		if len(inv.Entries) >= inv.EntryLimit {
			continue
		}
		schema := tool["parameters"]
		if schema == nil {
			schema = tool["input_schema"]
		}
		schemaPart := codexRequestEnvelopePart{}
		if schema != nil {
			schemaBytes := codexRequestEnvelopeJSON(t, schema)
			schemaPart = codexRequestEnvelopePart{Bytes: len(schemaBytes), SHA256: codexRequestEnvelopeDigest(schemaBytes)}
		}
		inv.Entries = append(inv.Entries, codexRequestEnvelopeInventoryEntry{
			Ordinal: ordinal, Type: codexRequestEnvelopeToolType(tool), Name: codexRequestEnvelopeToolName(tool),
			codexRequestEnvelopePart: codexRequestEnvelopePart{Bytes: len(encoded), SHA256: codexRequestEnvelopeDigest(encoded)}, Schema: schemaPart,
		})
	}
	inv.finish()
	return inv
}

func newCodexRequestEnvelopeInventory(count int) *codexRequestEnvelopeInventory {
	return &codexRequestEnvelopeInventory{EntryCount: count, EntryLimit: codexRequestEnvelopeEntryLimit, ArrayOverhead: 2 + max(count-1, 0)}
}

func (inv *codexRequestEnvelopeInventory) finish() {
	inv.EntriesIncomplete = inv.EntryCount > len(inv.Entries)
	inv.AccountedBytes = inv.EntryBytes + inv.ArrayOverhead
}

func codexRequestEnvelopeInputType(item map[string]any) string {
	value, _ := item["type"].(string)
	switch value {
	case "message", "function_call", "function_call_output", "computer_call", "computer_call_output", "local_shell_call", "web_search_call", "mcp_call", "mcp_list_tools", "mcp_approval_request", "custom_tool_call", "custom_tool_call_output", "reasoning", "item_reference":
		return value
	default:
		return "unknown"
	}
}

func codexRequestEnvelopeRoleLabel(item map[string]any) string {
	value, _ := item["role"].(string)
	switch value {
	case "developer", "user", "assistant", "system", "tool":
		return value
	case "":
		return "none"
	default:
		return "unknown"
	}
}

func codexRequestEnvelopeToolType(tool map[string]any) string {
	value, _ := tool["type"].(string)
	switch value {
	case "function", "custom", "computer_use_preview", "web_search_preview":
		return value
	default:
		return "unknown"
	}
}

func codexRequestEnvelopeToolName(tool map[string]any) string {
	value, _ := tool["name"].(string)
	switch value {
	case "apply_patch", "exec_command", "shell_command", "write_stdin", "update_plan", "request_user_input", "view_image", "list_mcp_resources", "list_mcp_resource_templates", "read_mcp_resource":
		return value
	case "get_goal", "create_goal", "update_goal", "request_user_input_async":
		return value
	default:
		return "redacted"
	}
}

func codexRequestEnvelopeDecodedTexts(item map[string]any) []string {
	content, _ := item["content"].([]any)
	texts := make([]string, 0, len(content))
	for _, raw := range content {
		part, _ := raw.(map[string]any)
		kind, _ := part["type"].(string)
		text, ok := part["text"].(string)
		if ok && (kind == "input_text" || kind == "output_text" || kind == "text") {
			texts = append(texts, text)
		}
	}
	return texts
}

func codexRequestEnvelopeSplitText(text string, textOrdinal int) []codexRequestEnvelopeTextSection {
	type boundary struct {
		start, headingBytes int
		label               string
	}
	boundaries := []boundary{}
	for start := 0; start < len(text); {
		end := strings.IndexByte(text[start:], '\n')
		if end < 0 {
			end = len(text)
		} else {
			end += start + 1
		}
		line := strings.TrimSuffix(strings.TrimSuffix(text[start:end], "\n"), "\r")
		if label, ok := codexRequestEnvelopeSectionLabel(line); ok {
			boundaries = append(boundaries, boundary{start: start, headingBytes: end - start, label: label})
		}
		start = end
	}
	if len(boundaries) == 0 || boundaries[0].start != 0 {
		boundaries = append([]boundary{{label: "prefix"}}, boundaries...)
	}
	sections := make([]codexRequestEnvelopeTextSection, 0, len(boundaries))
	for i, current := range boundaries {
		end := len(text)
		if i+1 < len(boundaries) {
			end = boundaries[i+1].start
		}
		part := []byte(text[current.start:end])
		sections = append(sections, codexRequestEnvelopeTextSection{TextOrdinal: textOrdinal, SectionOrdinal: i, Label: current.label, HeadingBytes: current.headingBytes, codexRequestEnvelopePart: codexRequestEnvelopePart{Bytes: len(part), SHA256: codexRequestEnvelopeDigest(part)}})
	}
	return sections
}

func codexRequestEnvelopeSectionLabel(line string) (string, bool) {
	switch {
	case strings.HasPrefix(line, "# AGENTS.md instructions for "):
		return "agents_md_instructions", true
	case line == "<INSTRUCTIONS>":
		return "instructions", true
	case line == "<environment_context>":
		return "environment_context", true
	case line == "# Multica Agent Runtime":
		return "multica_agent_runtime", true
	case line == "## Agent Identity":
		return "agent_identity", true
	case line == "## Attachments":
		return "attachments", true
	case line == "## Prepared primary repository":
		return "prepared_primary_repository", true
	case line == "## Privacy Security Boundary":
		return "privacy_security_boundary", true
	case line == "## Background Task Safety":
		return "background_task_safety", true
	case line == "## Comment Formatting":
		return "comment_formatting", true
	case line == "## Connected Apps":
		return "connected_apps", true
	case line == "## Final Issue Delivery":
		return "final_issue_delivery", true
	case line == "## Authoritative Issue Body Snapshot":
		return "authoritative_issue_body_snapshot", true
	case line == "## Verified Empty Comment History":
		return "verified_empty_comment_history", true
	case line == "## Important: Always Use the `multica` CLI":
		return "always_use_multica_cli", true
	case line == "## Instruction Precedence":
		return "instruction_precedence", true
	case line == "## Issue Body Formatting":
		return "issue_body_formatting", true
	case line == "## Issue Metadata":
		return "issue_metadata", true
	case line == "## Mentions":
		return "mentions", true
	case line == "## Output":
		return "output", true
	case line == "## Project Context":
		return "project_context", true
	case line == "## Repositories":
		return "repositories", true
	case line == "## Requesting User":
		return "requesting_user", true
	case line == "## Skills" || line == "### Available skills" || line == "### Available Skills":
		return "skills", true
	case line == "## Sub-issue Creation":
		return "sub_issue_creation", true
	case line == "## Task Initiator":
		return "task_initiator", true
	case line == "## Workspace Context":
		return "workspace_context", true
	case line == "## Available Commands":
		return "available_commands", true
	case line == "### Workflow":
		return "workflow", true
	case strings.HasPrefix(line, "# ") || strings.HasPrefix(line, "## ") || strings.HasPrefix(line, "### ") || strings.HasPrefix(line, "#### ") || strings.HasPrefix(line, "##### ") || strings.HasPrefix(line, "###### "):
		return "unknown", true
	default:
		return "", false
	}
}

func TestCodexRequestEnvelopeInventoryBoundsAndRedacts(t *testing.T) {
	for name, values := range map[string][]any{"nil": nil, "empty": {}} {
		inv := codexRequestEnvelopeInputInventory(t, values)
		if inv.AccountedBytes != len(codexRequestEnvelopeJSON(t, values)) {
			t.Fatalf("%s input accounting=%d, want normalized JSON bytes", name, inv.AccountedBytes)
		}
		tools := codexRequestEnvelopeToolInventory(t, values)
		if tools.AccountedBytes != len(codexRequestEnvelopeJSON(t, values)) {
			t.Fatalf("%s tool accounting=%d, want normalized JSON bytes", name, tools.AccountedBytes)
		}
	}
	input := make([]any, codexRequestEnvelopeEntryLimit+1)
	for i := range input {
		input[i] = map[string]any{"type": "hostile-secret", "role": "private-label", "content": []any{map[string]any{"type": "image", "text": "must-not-decode"}}}
	}
	inv := codexRequestEnvelopeInputInventory(t, input)
	encoded := codexRequestEnvelopeJSON(t, input)
	if !inv.EntriesIncomplete || inv.AccountedBytes != len(encoded) || len(inv.Entries) != codexRequestEnvelopeEntryLimit || inv.Entries[0].Type != "unknown" || inv.Entries[0].Role != "unknown" || inv.DecodedTextCount != 0 {
		t.Fatalf("bounded input inventory mismatch: %#v", inv)
	}
	tools := make([]any, codexRequestEnvelopeEntryLimit+1)
	tools[0] = map[string]any{"type": "function", "name": "hostile-secret", "parameters": map[string]any{"type": "object"}}
	tools[1] = map[string]any{"type": "function", "name": "exec_command"}
	for i := 2; i < len(tools); i++ {
		tools[i] = map[string]any{"type": "private-type", "name": "private-name"}
	}
	toolInv := codexRequestEnvelopeToolInventory(t, tools)
	if !toolInv.EntriesIncomplete || len(toolInv.Entries) != codexRequestEnvelopeEntryLimit || toolInv.AccountedBytes != len(codexRequestEnvelopeJSON(t, tools)) || toolInv.Entries[0].Name != "redacted" || toolInv.Entries[1].Name != "exec_command" || toolInv.Entries[0].Schema.Bytes == 0 {
		t.Fatalf("tool inventory mismatch: %#v", toolInv)
	}
	metadata, err := json.Marshal(struct {
		Input, Tools *codexRequestEnvelopeInventory
	}{inv, toolInv})
	if err != nil || strings.Contains(string(metadata), "hostile-secret") || strings.Contains(string(metadata), "private-") {
		t.Fatalf("inventory metadata leaked an unknown label or name: %s err=%v", metadata, err)
	}
	for _, name := range []string{"shell_command", "request_user_input", "list_mcp_resources", "list_mcp_resource_templates", "read_mcp_resource", "get_goal", "create_goal", "update_goal", "request_user_input_async"} {
		if got := codexRequestEnvelopeToolName(map[string]any{"name": name}); got != name {
			t.Fatalf("known builtin tool %q classified as %q", name, got)
		}
	}
}

func TestCodexRequestEnvelopeTextSectionsPreserveDecodedBytes(t *testing.T) {
	text := "prefix \u96ea\r\n# AGENTS.md instructions for /private\r\nbody\\n\n## Hidden label\n\n## Available Commands\n### Workflow\n### Workflow\n"
	sections := codexRequestEnvelopeSplitText(text, 3)
	var bytes int
	labels := make([]string, 0, len(sections))
	for _, section := range sections {
		bytes += section.Bytes
		labels = append(labels, section.Label)
	}
	want := []string{"prefix", "agents_md_instructions", "unknown", "available_commands", "workflow", "workflow"}
	if bytes != len(text) || strings.Join(labels, ",") != strings.Join(want, ",") || sections[1].HeadingBytes != len("# AGENTS.md instructions for /private\r\n") {
		t.Fatalf("text partition mismatch: bytes=%d labels=%v sections=%#v", bytes, labels, sections)
	}
	empty := codexRequestEnvelopeSplitText("", 0)
	if len(empty) != 1 || empty[0].Label != "prefix" || empty[0].Bytes != 0 {
		t.Fatalf("empty text partition mismatch: %#v", empty)
	}
}

func TestCodexRequestEnvelopeInventoryMarksSectionLimit(t *testing.T) {
	text := strings.Repeat("## Unknown\n", codexRequestEnvelopeSectionLimit+1)
	inv := codexRequestEnvelopeInputInventory(t, []any{map[string]any{"type": "message", "role": "user", "content": []any{map[string]any{"type": "input_text", "text": text}}}})
	if !inv.SectionsIncomplete || inv.SectionCount != codexRequestEnvelopeSectionLimit+1 || len(inv.Sections) != codexRequestEnvelopeSectionLimit || inv.DecodedTextBytes != len(text) {
		t.Fatalf("section bounds mismatch: %#v", inv)
	}
}

func TestCodexRequestEnvelopeInventoryPreservesSummaryFields(t *testing.T) {
	input := []any{map[string]any{"type": "message", "role": "user", "content": []any{map[string]any{"type": "input_text", "text": "# Multica Agent Runtime\n"}}}}
	tools := []any{map[string]any{"type": "function", "name": "exec_command", "parameters": map[string]any{"type": "object"}}}
	request := map[string]any{"model": "fixture", "instructions": "brief", "input": input, "tools": tools}
	body := codexRequestEnvelopeJSON(t, request)
	got := summarizeCodexRequestEnvelope(t, "fixture", body, request)
	if got.Body.Bytes != len(body) || got.Instructions.Bytes != len("brief") || got.Input.Bytes != len(codexRequestEnvelopeJSON(t, input)) || got.Tools.Bytes != len(codexRequestEnvelopeJSON(t, tools)) || got.Tools.Count != 1 || len(got.Input.Roles) != 1 {
		t.Fatalf("legacy summary fields changed: %#v", got)
	}
	if got.Input.Inventory == nil || got.Tools.Inventory == nil || got.Input.Inventory.Sections[0].Label != "multica_agent_runtime" {
		t.Fatalf("summary inventory missing: %#v", got)
	}
	if got.RepositoryRuleOccurrences == nil || *got.RepositoryRuleOccurrences != 0 {
		t.Fatalf("observed zero repository rule count was not preserved: %#v", got.RepositoryRuleOccurrences)
	}
}

func TestParseCodexRequestEnvelopeReceiptAcceptsV1WithoutInventory(t *testing.T) {
	old := `{"schema_version":"codex_request_envelope_diagnostic/v1","status":"captured","scope":"old","ordinary_mode":true,"runtime":{},"fixture":{},"provider":{},"brief_delta":{"without_completion":{"bytes":1,"sha256":"a"},"with_completion":{"bytes":2,"sha256":"b"},"delta_bytes":1},"arms":[{"arm":"native_exec","body":{"bytes":3,"sha256":"c"},"key_names":[],"model":"m","instructions":{"bytes":4,"sha256":"d"},"input":{"bytes":5,"sha256":"e","roles":[]},"tools":{"bytes":2,"sha256":"f","count":0},"prompt_occurrences":1,"automatic_delivery_occurrences":0,"native_identity_observed":true}],"comparison":{},"limitations":[]}`
	receipt, err := parseCodexRequestEnvelopeReceipt([]byte(codexRequestEnvelopeReceiptMark + old + "\n"))
	if err != nil || receipt.SchemaVersion != "codex_request_envelope_diagnostic/v1" || receipt.Arms[0].Input.Inventory != nil || receipt.Arms[0].Tools.Inventory != nil || receipt.Arms[0].RepositoryRuleOccurrences != nil {
		t.Fatalf("old receipt compatibility mismatch: receipt=%#v err=%v", receipt, err)
	}
	encoded, err := json.Marshal(receipt)
	if err != nil || strings.Contains(string(encoded), `"inventory"`) {
		t.Fatalf("old receipt gained inventory: %s err=%v", encoded, err)
	}
}
