package agent

import (
	"encoding/json"
	"fmt"
	"strings"
	"testing"
)

func TestClaudeStreamMetadataRejectsInvalidUTF8(t *testing.T) {
	c := newClaudeStreamMetadataCollector()
	lines := validClaudeMetadataLines(`{"type":"result","subtype":"success","is_error":false,"session_id":"s","modelUsage":{"m":{"inputTokens":1,"cacheReadInputTokens":0,"cacheCreationInputTokens":0,"outputTokens":1}}}`)
	for index, line := range lines {
		if index == 1 {
			c.Observe([]byte("{\"type\":\"log\",\"message\":\"\xff\"}"))
		}
		c.Observe([]byte(line))
	}
	got := c.Finish()
	if got.Status != "failed" {
		t.Fatalf("invalid UTF-8 status = %q, want failed", got.Status)
	}
	assertClaudeStreamMetadataError(t, got.Errors, "MALFORMED_EVENT")
}

func TestClaudeStreamMetadataMainChildAndMultiModelAggregate(t *testing.T) {
	c := newClaudeStreamMetadataCollector()
	for _, line := range []string{
		`{"type":"system","subtype":"init","session_id":"session-secret","model":"configured-alias","permissionMode":"bypassPermissions","mcp_servers":[]}`,
		`{"type":"assistant","session_id":"session-secret","parent_tool_use_id":null,"message":{"model":"main-model","content":[]}}`,
		`{"type":"assistant","session_id":"session-secret","parent_tool_use_id":"tool-child","message":{"model":"child-model","content":[]}}`,
		`{"type":"result","subtype":"success","is_error":false,"session_id":"session-secret","modelUsage":{"main-model":{"inputTokens":3,"cacheReadInputTokens":4,"cacheCreationInputTokens":5,"outputTokens":6},"child-model":{"inputTokens":7,"cacheReadInputTokens":8,"cacheCreationInputTokens":9,"outputTokens":10}}}`,
	} {
		c.Observe([]byte(line))
	}
	got := c.Finish()
	if got.Status != "observed" || got.ConfiguredModel == nil || *got.ConfiguredModel != "configured-alias" ||
		got.MainModel == nil || *got.MainModel != "main-model" || got.MainModelSource != "assistant_event" ||
		got.PermissionMode == nil || *got.PermissionMode != "bypassPermissions" || got.MCPServerCount == nil || *got.MCPServerCount != 0 || got.SessionIDDigest == nil ||
		got.InitCount != 1 || got.MainAssistantCount != 1 || got.TerminalCount != 1 || got.TerminalSuccess == nil || !*got.TerminalSuccess {
		t.Fatalf("projection = %+v", got)
	}
	if len(got.MainModels) != 1 || got.MainModels[0] != "main-model" || len(got.ModelUsage) != 2 {
		t.Fatalf("models = main:%v usage:%+v", got.MainModels, got.ModelUsage)
	}
	if got.ModelUsage[0].Model != "child-model" || got.ModelUsage[1].Model != "main-model" {
		t.Fatalf("model usage is not sorted: %+v", got.ModelUsage)
	}
	want := claudeStreamMetadataUsage{UncachedInputTokens: 10, CacheReadInputTokens: 12, CacheWriteInputTokens: 14, OutputTokens: 16}
	if got.UsageStatus != "observed" || got.Usage == nil || *got.Usage != want || len(got.Errors) != 0 {
		t.Fatalf("usage = %s %+v errors=%+v", got.UsageStatus, got.Usage, got.Errors)
	}
	encoded, err := json.Marshal(got)
	if err != nil {
		t.Fatal(err)
	}
	for _, forbidden := range []string{"session-secret", "tool-child", "content"} {
		if strings.Contains(string(encoded), forbidden) {
			t.Fatalf("projection leaked %q: %s", forbidden, encoded)
		}
	}
}

func TestClaudeStreamMetadataMapsTerminalOneMinuteConfiguredModelUsage(t *testing.T) {
	tests := []struct {
		name          string
		configured    string
		modelUsage    string
		wantStatus    string
		wantUsage     *claudeStreamMetadataUsage
		wantLabels    []string
		wantUsageCode bool
	}{
		{
			name:       "lowercase suffix",
			configured: "glm-5.3-flash[1m]",
			modelUsage: `"glm-5.3-flash[1m]":{"inputTokens":1,"cacheReadInputTokens":2,"cacheCreationInputTokens":3,"outputTokens":4},` +
				`"child-model":{"inputTokens":5,"cacheReadInputTokens":6,"cacheCreationInputTokens":7,"outputTokens":8}`,
			wantStatus: "observed",
			wantUsage:  &claudeStreamMetadataUsage{UncachedInputTokens: 6, CacheReadInputTokens: 8, CacheWriteInputTokens: 10, OutputTokens: 12},
			wantLabels: []string{"child-model", "glm-5.3-flash[1m]"},
		},
		{
			name:       "uppercase configured suffix",
			configured: "glm-5.3-flash[1M]",
			modelUsage: `"glm-5.3-flash[1m]":{"inputTokens":1,"cacheReadInputTokens":2,"cacheCreationInputTokens":3,"outputTokens":4}`,
			wantStatus: "observed",
			wantUsage:  &claudeStreamMetadataUsage{UncachedInputTokens: 1, CacheReadInputTokens: 2, CacheWriteInputTokens: 3, OutputTokens: 4},
			wantLabels: []string{"glm-5.3-flash[1m]"},
		},
		{
			name:       "uppercase usage suffix",
			configured: "glm-5.3-flash[1m]",
			modelUsage: `"glm-5.3-flash[1M]":{"inputTokens":1,"cacheReadInputTokens":2,"cacheCreationInputTokens":3,"outputTokens":4}`,
			wantStatus: "observed",
			wantUsage:  &claudeStreamMetadataUsage{UncachedInputTokens: 1, CacheReadInputTokens: 2, CacheWriteInputTokens: 3, OutputTokens: 4},
			wantLabels: []string{"glm-5.3-flash[1M]"},
		},
		{
			name:          "invalid other model",
			configured:    "glm-5.3-flash[1m]",
			modelUsage:    `"other-model":{"inputTokens":1,"cacheReadInputTokens":2,"cacheCreationInputTokens":3,"outputTokens":4}`,
			wantStatus:    "missing",
			wantLabels:    []string{"other-model"},
			wantUsageCode: true,
		},
		{
			name:          "different context suffix",
			configured:    "glm-5.3-flash[1m]",
			modelUsage:    `"glm-5.3-flash[2m]":{"inputTokens":1,"cacheReadInputTokens":2,"cacheCreationInputTokens":3,"outputTokens":4}`,
			wantStatus:    "missing",
			wantLabels:    []string{"glm-5.3-flash[2m]"},
			wantUsageCode: true,
		},
		{
			name:          "same prefix other model",
			configured:    "glm-5.3-flash[1m]",
			modelUsage:    `"glm-5.3-flash-pro[1m]":{"inputTokens":1,"cacheReadInputTokens":2,"cacheCreationInputTokens":3,"outputTokens":4}`,
			wantStatus:    "missing",
			wantLabels:    []string{"glm-5.3-flash-pro[1m]"},
			wantUsageCode: true,
		},
		{
			name:          "exact and configured suffix are ambiguous",
			configured:    "glm-5.3-flash[1m]",
			modelUsage:    `"glm-5.3-flash":{"inputTokens":1,"cacheReadInputTokens":2,"cacheCreationInputTokens":3,"outputTokens":4},"glm-5.3-flash[1m]":{"inputTokens":5,"cacheReadInputTokens":6,"cacheCreationInputTokens":7,"outputTokens":8}`,
			wantStatus:    "missing",
			wantLabels:    []string{"glm-5.3-flash", "glm-5.3-flash[1m]"},
			wantUsageCode: true,
		},
		{
			name:          "failed mapped usage",
			configured:    "glm-5.3-flash[1m]",
			modelUsage:    `"glm-5.3-flash[1m]":{"inputTokens":-1,"cacheReadInputTokens":2,"cacheCreationInputTokens":3,"outputTokens":4}`,
			wantStatus:    "failed",
			wantLabels:    []string{"glm-5.3-flash[1m]"},
			wantUsageCode: true,
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			c := newClaudeStreamMetadataCollector()
			for _, line := range []string{
				fmt.Sprintf(`{"type":"system","subtype":"init","session_id":"s","model":%q,"permissionMode":"default","mcp_servers":[]}`, tc.configured),
				`{"type":"assistant","session_id":"s","message":{"model":"glm-5.3-flash"}}`,
				`{"type":"result","subtype":"success","is_error":false,"session_id":"s","modelUsage":{` + tc.modelUsage + `}}`,
			} {
				c.Observe([]byte(line))
			}

			got := c.Finish()
			if got.Status != tc.wantStatus || got.ConfiguredModel == nil || *got.ConfiguredModel != tc.configured ||
				got.MainModel == nil || *got.MainModel != "glm-5.3-flash" || got.UsageStatus != map[bool]string{true: "failed", false: "observed"}[tc.name == "failed mapped usage"] {
				t.Fatalf("projection = %+v", got)
			}
			if len(got.ModelUsage) != len(tc.wantLabels) {
				t.Fatalf("model usage = %+v", got.ModelUsage)
			}
			for i, label := range tc.wantLabels {
				if got.ModelUsage[i].Model != label {
					t.Fatalf("model usage labels = %+v, want %v", got.ModelUsage, tc.wantLabels)
				}
			}
			if tc.wantUsage != nil && (got.Usage == nil || *got.Usage != *tc.wantUsage) {
				t.Fatalf("usage = %+v, want %+v", got.Usage, tc.wantUsage)
			}
			if tc.wantUsageCode {
				assertClaudeStreamMetadataError(t, got.Errors, "MAIN_MODEL_USAGE_MISSING")
			} else if len(got.Errors) != 0 {
				t.Fatalf("unexpected errors = %+v", got.Errors)
			}
		})
	}
}

func TestClaudeStreamMetadataMissingConflictingAndFailedStates(t *testing.T) {
	tests := []struct {
		name   string
		lines  []string
		status string
		codes  []string
	}{
		{name: "missing all", status: "missing", codes: []string{"CONFIGURATION_MISSING", "INIT_MISSING", "MAIN_MODEL_MISSING", "TERMINAL_MISSING", "USAGE_MISSING"}},
		{name: "duplicate init", lines: []string{
			`{"type":"system","subtype":"init","session_id":"s","model":"m","permissionMode":"default","mcp_servers":[]}`,
			`{"type":"system","subtype":"init","session_id":"s","model":"m","permissionMode":"default","mcp_servers":[]}`,
		}, status: "conflicting", codes: []string{"INIT_DUPLICATE"}},
		{name: "duplicate terminal", lines: validClaudeMetadataLines(`{"type":"result","subtype":"success","is_error":false,"session_id":"s","modelUsage":{"m":{"inputTokens":1,"cacheReadInputTokens":0,"cacheCreationInputTokens":0,"outputTokens":1}}}`, `{"type":"result","subtype":"success","is_error":false,"session_id":"s","modelUsage":{}}`), status: "conflicting", codes: []string{"TERMINAL_DUPLICATE"}},
		{name: "main conflict", lines: []string{
			`{"type":"system","subtype":"init","session_id":"s","model":"configured","permissionMode":"default","mcp_servers":[]}`,
			`{"type":"assistant","session_id":"s","message":{"model":"a"}}`,
			`{"type":"assistant","session_id":"s","message":{"model":"b"}}`,
			`{"type":"result","subtype":"success","is_error":false,"session_id":"s","modelUsage":{"a":{"inputTokens":1,"cacheReadInputTokens":0,"cacheCreationInputTokens":0,"outputTokens":1},"b":{"inputTokens":1,"cacheReadInputTokens":0,"cacheCreationInputTokens":0,"outputTokens":1}}}`,
		}, status: "conflicting", codes: []string{"MAIN_MODEL_CONFLICT"}},
		{name: "session conflict", lines: []string{
			`{"type":"system","subtype":"init","session_id":"one","model":"m","permissionMode":"default","mcp_servers":[]}`,
			`{"type":"assistant","session_id":"two","message":{"model":"m"}}`,
			`{"type":"result","subtype":"success","is_error":false,"session_id":"one","modelUsage":{"m":{"inputTokens":1,"cacheReadInputTokens":0,"cacheCreationInputTokens":0,"outputTokens":1}}}`,
		}, status: "failed", codes: []string{"SESSION_MISMATCH"}},
		{name: "invalid parent", lines: []string{
			`{"type":"system","subtype":"init","session_id":"s","model":"m","permissionMode":"default","mcp_servers":[]}`,
			`{"type":"assistant","session_id":"s","parent_tool_use_id":7,"message":{"model":"m"}}`,
			`{"type":"result","subtype":"success","is_error":false,"session_id":"s","modelUsage":{"m":{"inputTokens":1,"cacheReadInputTokens":0,"cacheCreationInputTokens":0,"outputTokens":1}}}`,
		}, status: "failed", codes: []string{"PARENT_INVALID", "MAIN_MODEL_MISSING"}},
		{name: "bad usage", lines: validClaudeMetadataLines(`{"type":"result","subtype":"success","is_error":false,"session_id":"s","modelUsage":{"m":{"inputTokens":-1,"cacheReadInputTokens":0,"cacheCreationInputTokens":0,"outputTokens":1}}}`), status: "failed", codes: []string{"USAGE_INVALID", "MAIN_MODEL_USAGE_MISSING"}},
		{name: "missing usage", lines: validClaudeMetadataLines(`{"type":"result","subtype":"success","is_error":false,"session_id":"s"}`), status: "missing", codes: []string{"USAGE_MISSING", "MAIN_MODEL_USAGE_MISSING"}},
		{name: "terminal error", lines: validClaudeMetadataLines(`{"type":"result","subtype":"success","is_error":true,"session_id":"s","modelUsage":{"m":{"inputTokens":1,"cacheReadInputTokens":0,"cacheCreationInputTokens":0,"outputTokens":1}},"result":"AUTH SECRET"}`), status: "failed", codes: []string{"TERMINAL_FAILED"}},
		{name: "error subtype overrides false flag", lines: validClaudeMetadataLines(`{"type":"result","subtype":"error_max_budget_usd","is_error":false,"session_id":"s","modelUsage":{"m":{"inputTokens":1,"cacheReadInputTokens":0,"cacheCreationInputTokens":0,"outputTokens":1}}}`), status: "failed", codes: []string{"TERMINAL_FAILED"}},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			c := newClaudeStreamMetadataCollector()
			for _, line := range tc.lines {
				c.Observe([]byte(line))
			}
			got := c.Finish()
			if got.Status != tc.status {
				t.Fatalf("status = %q, want %q; projection=%+v", got.Status, tc.status, got)
			}
			for _, code := range tc.codes {
				assertClaudeStreamMetadataError(t, got.Errors, code)
			}
			encoded, _ := json.Marshal(got)
			if strings.Contains(string(encoded), "AUTH SECRET") {
				t.Fatalf("provider text leaked: %s", encoded)
			}
			t.Logf("CLAUDE_STREAM_METADATA_VECTOR %s", encoded)
		})
	}
}

func TestClaudeStreamMetadataTerminalRequiresExplicitKnownSubtype(t *testing.T) {
	for _, terminal := range []string{
		`{"type":"result","is_error":false,"session_id":"s","modelUsage":{"m":{"inputTokens":1,"cacheReadInputTokens":0,"cacheCreationInputTokens":0,"outputTokens":1}}}`,
		`{"type":"result","subtype":"future_status","is_error":false,"session_id":"s","modelUsage":{"m":{"inputTokens":1,"cacheReadInputTokens":0,"cacheCreationInputTokens":0,"outputTokens":1}}}`,
		`{"type":"result","subtype":"success","is_error":null,"session_id":"s","modelUsage":{"m":{"inputTokens":1,"cacheReadInputTokens":0,"cacheCreationInputTokens":0,"outputTokens":1}}}`,
	} {
		c := newClaudeStreamMetadataCollector()
		for _, line := range validClaudeMetadataLines(terminal) {
			c.Observe([]byte(line))
		}
		got := c.Finish()
		if got.Status != "failed" || got.TerminalSuccess != nil {
			t.Fatalf("projection = %+v", got)
		}
		assertClaudeStreamMetadataError(t, got.Errors, "MALFORMED_EVENT")
	}
}

func TestClaudeStreamMetadataMissingMainModelCannotRecover(t *testing.T) {
	c := newClaudeStreamMetadataCollector()
	for _, line := range []string{
		`{"type":"system","subtype":"init","session_id":"s","model":"configured","permissionMode":"default","mcp_servers":[]}`,
		`{"type":"assistant","session_id":"s","message":{"content":[]}}`,
		`{"type":"assistant","session_id":"s","message":{"model":"main","content":[]}}`,
		`{"type":"result","subtype":"success","is_error":false,"session_id":"s","modelUsage":{"main":{"inputTokens":1,"cacheReadInputTokens":0,"cacheCreationInputTokens":0,"outputTokens":1}}}`,
	} {
		c.Observe([]byte(line))
	}
	got := c.Finish()
	if got.Status != "missing" || got.MainModel == nil || *got.MainModel != "main" || got.MainAssistantCount != 2 {
		t.Fatalf("projection = %+v", got)
	}
	assertClaudeStreamMetadataError(t, got.Errors, "MAIN_MODEL_MISSING")
}

func TestClaudeStreamMetadataUsesExistingBoundedModelIdentity(t *testing.T) {
	c := newClaudeStreamMetadataCollector()
	for _, line := range []string{
		`{"type":"system","subtype":"init","session_id":"s","model":"configured [模型]","permissionMode":"default","mcp_servers":[]}`,
		`{"type":"assistant","session_id":"s","message":{"model":"main [模型]"}}`,
		`{"type":"result","subtype":"success","is_error":false,"session_id":"s","modelUsage":{"main [模型]":{"inputTokens":1,"cacheReadInputTokens":0,"cacheCreationInputTokens":0,"outputTokens":1}}}`,
	} {
		c.Observe([]byte(line))
	}
	if got := c.Finish(); got.Status != "observed" {
		t.Fatalf("projection = %+v", got)
	}
}

func TestClaudeStreamMetadataBoundsOverflowAndLateEvents(t *testing.T) {
	t.Run("event limit", func(t *testing.T) {
		c := newClaudeStreamMetadataCollector()
		for i := 0; i <= claudeStreamMetadataMaxEvents; i++ {
			c.Observe([]byte(`{"type":"log"}`))
		}
		got := c.Finish()
		if got.Status != "failed" || got.InitCount > claudeStreamMetadataMaxEvents || got.MainAssistantCount > claudeStreamMetadataMaxEvents || got.TerminalCount > claudeStreamMetadataMaxEvents {
			t.Fatalf("projection = %+v", got)
		}
		assertClaudeStreamMetadataError(t, got.Errors, "EVENT_LIMIT")
	})

	t.Run("model limit", func(t *testing.T) {
		models := make([]string, 0, claudeStreamMetadataMaxModels+1)
		usage := make([]string, 0, claudeStreamMetadataMaxModels+1)
		for i := 0; i <= claudeStreamMetadataMaxModels; i++ {
			model := fmt.Sprintf("m%02d", i)
			models = append(models, fmt.Sprintf(`{"type":"assistant","session_id":"s","message":{"model":%q}}`, model))
			usage = append(usage, fmt.Sprintf(`%q:{"inputTokens":1,"cacheReadInputTokens":0,"cacheCreationInputTokens":0,"outputTokens":1}`, model))
		}
		c := newClaudeStreamMetadataCollector()
		c.Observe([]byte(`{"type":"system","subtype":"init","session_id":"s","model":"configured","permissionMode":"default","mcp_servers":[]}`))
		for _, line := range models {
			c.Observe([]byte(line))
		}
		c.Observe([]byte(`{"type":"result","subtype":"success","is_error":false,"session_id":"s","modelUsage":{` + strings.Join(usage, ",") + `}}`))
		got := c.Finish()
		if got.Status != "failed" || got.UsageStatus != "failed" || got.Usage != nil || len(got.MainModels) > claudeStreamMetadataMaxModels || len(got.ModelUsage) > claudeStreamMetadataMaxModels {
			t.Fatalf("projection = %+v", got)
		}
		assertClaudeStreamMetadataError(t, got.Errors, "MODEL_LIMIT")
	})

	t.Run("safe integer overflow", func(t *testing.T) {
		c := newClaudeStreamMetadataCollector()
		for _, line := range []string{
			`{"type":"system","subtype":"init","session_id":"s","model":"m","permissionMode":"default","mcp_servers":[]}`,
			`{"type":"assistant","session_id":"s","message":{"model":"m"}}`,
			`{"type":"assistant","session_id":"s","parent_tool_use_id":"child","message":{"model":"n"}}`,
			fmt.Sprintf(`{"type":"result","subtype":"success","is_error":false,"session_id":"s","modelUsage":{"m":{"inputTokens":%d,"cacheReadInputTokens":0,"cacheCreationInputTokens":0,"outputTokens":0},"n":{"inputTokens":1,"cacheReadInputTokens":0,"cacheCreationInputTokens":0,"outputTokens":0}}}`, claudeStreamMetadataMaxSafeInteger),
		} {
			c.Observe([]byte(line))
		}
		got := c.Finish()
		if got.UsageStatus != "failed" || got.Usage != nil {
			t.Fatalf("usage = %s %+v", got.UsageStatus, got.Usage)
		}
		assertClaudeStreamMetadataError(t, got.Errors, "USAGE_OVERFLOW")
	})

	t.Run("late event", func(t *testing.T) {
		c := newClaudeStreamMetadataCollector()
		for _, line := range append(validClaudeMetadataLines(`{"type":"result","subtype":"success","is_error":false,"session_id":"s","modelUsage":{"m":{"inputTokens":1,"cacheReadInputTokens":0,"cacheCreationInputTokens":0,"outputTokens":1}}}`), `{"type":"assistant","session_id":"s","message":{"model":"m"}}`) {
			c.Observe([]byte(line))
		}
		got := c.Finish()
		if got.Status != "failed" {
			t.Fatalf("projection = %+v", got)
		}
		assertClaudeStreamMetadataError(t, got.Errors, "EVENT_AFTER_TERMINAL")
	})
}

func TestClaudeStreamMetadataMCPServerCount(t *testing.T) {
	tooManyServers := "[" + strings.Repeat("{},", claudeStreamMetadataMaxEvents) + "{}]"
	tests := []struct {
		name      string
		init      string
		wantCount *int
		codes     []string
	}{
		{name: "empty", init: `{"type":"system","subtype":"init","session_id":"s","model":"configured","permissionMode":"default","mcp_servers":[]}`, wantCount: intPointer(0)},
		{name: "nonzero", init: `{"type":"system","subtype":"init","session_id":"s","model":"configured","permissionMode":"default","mcp_servers":[{"name":"one","url":"SECRET"},{"name":"two"}]}`, wantCount: intPointer(2)},
		{name: "missing", init: `{"type":"system","subtype":"init","session_id":"s","model":"configured","permissionMode":"default"}`, codes: []string{"CONFIGURATION_MISSING"}},
		{name: "malformed", init: `{"type":"system","subtype":"init","session_id":"s","model":"configured","permissionMode":"default","mcp_servers":"bad"}`, codes: []string{"CONFIGURATION_MISSING", "MALFORMED_EVENT"}},
		{name: "bounded", init: `{"type":"system","subtype":"init","session_id":"s","model":"configured","permissionMode":"auto","mcp_servers":` + tooManyServers + `}`, codes: []string{"CONFIGURATION_MISSING", "EVENT_LIMIT"}},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			c := newClaudeStreamMetadataCollector()
			for _, line := range []string{tc.init, `{"type":"assistant","session_id":"s","message":{"model":"m"}}`, `{"type":"result","subtype":"success","is_error":false,"session_id":"s","modelUsage":{"m":{"inputTokens":1,"cacheReadInputTokens":0,"cacheCreationInputTokens":0,"outputTokens":1}}}`} {
				c.Observe([]byte(line))
			}
			got := c.Finish()
			if tc.wantCount == nil {
				if got.MCPServerCount != nil {
					t.Fatalf("mcp_server_count = %v, want null", *got.MCPServerCount)
				}
			} else if got.MCPServerCount == nil || *got.MCPServerCount != *tc.wantCount {
				t.Fatalf("mcp_server_count = %v, want %d", got.MCPServerCount, *tc.wantCount)
			}
			for _, code := range tc.codes {
				assertClaudeStreamMetadataError(t, got.Errors, code)
			}
			encoded, _ := json.Marshal(got)
			if strings.Contains(string(encoded), "SECRET") || strings.Contains(string(encoded), `"name"`) || strings.Contains(string(encoded), `"url"`) {
				t.Fatalf("MCP details leaked: %s", encoded)
			}
		})
	}
}

func TestClaudeStreamMetadataDropsInvalidModelUsageIdentity(t *testing.T) {
	secretModel := "SECRET\n" + strings.Repeat("x", 1024)
	c := newClaudeStreamMetadataCollector()
	for _, line := range []string{
		`{"type":"system","subtype":"init","session_id":"s","model":"configured","permissionMode":"default","mcp_servers":[]}`,
		`{"type":"assistant","session_id":"s","message":{"model":"main"}}`,
		fmt.Sprintf(`{"type":"result","subtype":"success","is_error":false,"session_id":"s","modelUsage":{%q:{"inputTokens":1,"cacheReadInputTokens":0,"cacheCreationInputTokens":0,"outputTokens":1},"main":{"inputTokens":1,"cacheReadInputTokens":0,"cacheCreationInputTokens":0,"outputTokens":1}}}`, secretModel),
	} {
		c.Observe([]byte(line))
	}
	got := c.Finish()
	encoded, _ := json.Marshal(got)
	if got.Status != "failed" || strings.Contains(string(encoded), "SECRET") || strings.Contains(string(encoded), strings.Repeat("x", 64)) {
		t.Fatalf("invalid model identity retained: %s", encoded)
	}
	assertClaudeStreamMetadataError(t, got.Errors, "MODEL_INVALID")
}

func TestClaudeStreamMetadataProjectionReceipts(t *testing.T) {
	vectors := map[string][]string{
		"observed": validClaudeMetadataLines(`{"type":"result","subtype":"success","is_error":false,"session_id":"s","modelUsage":{"m":{"inputTokens":1,"cacheReadInputTokens":2,"cacheCreationInputTokens":3,"outputTokens":4}}}`),
		"missing":  nil,
		"conflicting": {
			`{"type":"system","subtype":"init","session_id":"s","model":"configured","permissionMode":"default","mcp_servers":[]}`,
			`{"type":"assistant","session_id":"s","message":{"model":"a"}}`,
			`{"type":"assistant","session_id":"s","message":{"model":"b"}}`,
			`{"type":"result","subtype":"success","is_error":false,"session_id":"s","modelUsage":{"a":{"inputTokens":1,"cacheReadInputTokens":0,"cacheCreationInputTokens":0,"outputTokens":1},"b":{"inputTokens":1,"cacheReadInputTokens":0,"cacheCreationInputTokens":0,"outputTokens":1}}}`,
		},
		"failed": validClaudeMetadataLines(`{"type":"result","subtype":"error_max_budget_usd","is_error":false,"session_id":"s","modelUsage":{"m":{"inputTokens":1,"cacheReadInputTokens":0,"cacheCreationInputTokens":0,"outputTokens":1}}}`),
	}
	for _, name := range []string{"observed", "missing", "conflicting", "failed"} {
		c := newClaudeStreamMetadataCollector()
		for _, line := range vectors[name] {
			c.Observe([]byte(line))
		}
		encoded, err := json.Marshal(c.Finish())
		if err != nil {
			t.Fatal(err)
		}
		t.Logf("claude stream metadata %s receipt: %s", name, encoded)
	}
}

func validClaudeMetadataLines(result string, extra ...string) []string {
	lines := []string{
		`{"type":"system","subtype":"init","session_id":"s","model":"configured","permissionMode":"default","mcp_servers":[]}`,
		`{"type":"assistant","session_id":"s","message":{"model":"m"}}`,
		result,
	}
	return append(lines, extra...)
}

func intPointer(value int) *int { return &value }

func assertClaudeStreamMetadataError(t *testing.T, errors []claudeStreamMetadataError, code string) {
	t.Helper()
	for _, item := range errors {
		if item.Code == code && item.Count > 0 {
			return
		}
	}
	t.Fatalf("missing error %s in %+v", code, errors)
}
