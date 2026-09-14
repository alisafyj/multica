package agent

import (
	"bytes"
	"encoding/json"
	"log/slog"
	"strings"
	"testing"
)

func TestCodexResponseMetadataProjectionObservedWithoutUsage(t *testing.T) {
	var ordinary bytes.Buffer
	w := newCodexResponseMetadataWriter(&ordinary, codexResponseMetadataLimits{})
	w.Write([]byte("ordinary startup diagnostic\n2026 TRACE codex_api::sse::responses: SSE event: "))
	w.Write([]byte(`{"type":"response.created","sequence_number":0,"response":{"id":"resp_one","status":"in_progress","model":"provider-model"}}` + "\n"))
	w.Write([]byte("2026 TRACE codex_api::sse::responses: SSE event: " +
		`{"type":"response.output_item.done","sequence_number":1}` + "\n"))
	w.Write([]byte("2026 TRACE codex_api::sse::responses: SSE event: " +
		`{"type":"response.completed","sequence_number":2,"response":{"id":"resp_one","status":"completed","model":"provider-model"}}` + "\n"))

	got := w.Finish()
	if ordinary.String() != "ordinary startup diagnostic\n" {
		t.Fatalf("ordinary stderr = %q", ordinary.String())
	}
	if got.Status != "observed" || got.ActualModel == nil || *got.ActualModel != "provider-model" ||
		got.UsageStatus != "missing" || got.Usage != nil || len(got.Responses) != 1 {
		t.Fatalf("unexpected projection: %+v", got)
	}
	if got.Responses[0].TerminalStatus == nil || *got.Responses[0].TerminalStatus != "completed" || got.Responses[0].UsageStatus != "missing" {
		t.Fatalf("unexpected response: %+v", got.Responses[0])
	}
	if len(got.Errors) != 1 || got.Errors[0].Code != "USAGE_MISSING" || got.Errors[0].Count != 1 {
		t.Fatalf("unexpected errors: %+v", got.Errors)
	}
	encoded, err := json.Marshal(got)
	if err != nil {
		t.Fatal(err)
	}
	for _, forbidden := range []string{"resp_one", "response.completed", "ordinary startup diagnostic"} {
		if bytes.Contains(encoded, []byte(forbidden)) {
			t.Fatalf("projection retained forbidden raw value %q", forbidden)
		}
	}
}

func TestCodexResponseMetadataProjectionUsageAndRepeatedModel(t *testing.T) {
	w := newCodexResponseMetadataWriter(&bytes.Buffer{}, codexResponseMetadataLimits{})
	for _, line := range []string{
		`{"type":"response.created","sequence_number":0,"response":{"id":"resp_repeat","status":"in_progress","model":"same-model"}}`,
		`{"type":"response.in_progress","sequence_number":1,"response":{"id":"resp_repeat","status":"in_progress","model":"same-model"}}`,
		`{"type":"response.completed","sequence_number":2,"response":{"id":"resp_repeat","status":"completed","model":"same-model","usage":{"input_tokens":9,"input_tokens_details":{"cached_tokens":3,"cache_write_tokens":2},"output_tokens":4,"output_tokens_details":{"reasoning_tokens":1},"total_tokens":13}}}`,
	} {
		w.Write([]byte("TRACE codex_api::sse::responses: SSE event: " + line + "\n"))
	}
	got := w.Finish()
	if got.Status != "observed" || got.UsageStatus != "observed" || got.Usage == nil {
		t.Fatalf("unexpected projection: %+v", got)
	}
	if *got.Usage != (codexResponseMetadataUsage{UncachedInputTokens: 4, CacheReadInputTokens: 3, CacheWriteInputTokens: 2, OutputTokens: 4, ResponseCount: 1}) {
		t.Fatalf("usage = %+v", *got.Usage)
	}
	if len(got.Errors) != 0 {
		t.Fatalf("same-model repeats conflicted: %+v", got.Errors)
	}
}

func TestCodexResponseMetadataProjectionConflictAndIncomplete(t *testing.T) {
	w := newCodexResponseMetadataWriter(&bytes.Buffer{}, codexResponseMetadataLimits{})
	for _, line := range []string{
		`{"type":"response.created","sequence_number":1,"response":{"id":"resp_conflict","status":"in_progress","model":"model-a"}}`,
		`{"type":"response.completed","sequence_number":2,"response":{"id":"resp_conflict","status":"completed","model":"model-b","usage":null}}`,
		`{"type":"response.created","sequence_number":0,"response":{"id":"resp_open","status":"in_progress"}}`,
	} {
		w.Write([]byte("TRACE codex_api::sse::responses: SSE event: " + line + "\n"))
	}
	got := w.Finish()
	if got.Status != "conflicting" || got.ActualModel != nil || got.ModelSource != "missing" {
		t.Fatalf("unexpected conflict projection: %+v", got)
	}
	assertCodexResponseMetadataError(t, got.Errors, "MODEL_CONFLICT")
	assertCodexResponseMetadataError(t, got.Errors, "RESPONSE_TERMINAL_MISSING")
}

func TestCodexResponseMetadataProjectionRejectsNullAndUnsafeUsageAggregate(t *testing.T) {
	t.Run("explicit null model", func(t *testing.T) {
		w := newCodexResponseMetadataWriter(&bytes.Buffer{}, codexResponseMetadataLimits{})
		w.Write([]byte("TRACE codex_api::sse::responses: SSE event: " +
			`{"type":"response.created","sequence_number":0,"response":{"id":"resp_null","status":"in_progress","model":null}}` + "\n"))
		got := w.Finish()
		if got.Status != "failed" {
			t.Fatalf("status = %q", got.Status)
		}
		assertCodexResponseMetadataError(t, got.Errors, "MALFORMED_EVENT")
	})
	t.Run("aggregate safe integer", func(t *testing.T) {
		terminal := "completed"
		responses := []codexResponseMetadataResponse{
			{TerminalStatus: &terminal, UsageStatus: "observed", Usage: &codexResponseMetadataUsage{OutputTokens: 1<<53 - 1}},
			{TerminalStatus: &terminal, UsageStatus: "observed", Usage: &codexResponseMetadataUsage{OutputTokens: 1}},
		}
		status, usage, code := codexResponseMetadataAggregateUsage(responses)
		if status != "failed" || usage != nil || code != "USAGE_INVALID" {
			t.Fatalf("aggregate = %q %+v %q", status, usage, code)
		}
	})
}

func TestCodexResponseMetadataWriterFiltersSplitMalformedAndOverflow(t *testing.T) {
	var ordinary bytes.Buffer
	w := newCodexResponseMetadataWriter(&ordinary, codexResponseMetadataLimits{MaxLineBytes: 96, MaxTotalBytes: 4096, MaxEvents: 8, MaxResponses: 2})
	w.Write([]byte("keep one\nTRACE codex_api::sse::res"))
	w.Write([]byte("ponses: SSE event: {malformed}\nkeep two\n"))
	w.Write([]byte("ordinary codex_api::sse::responses: SSE event: mention\n"))
	w.Write([]byte("TRACE codex_api::sse::responses: SSE event: " + strings.Repeat("x", 100) + "\n"))
	w.Write([]byte("keep three"))
	got := w.Finish()
	if ordinary.String() != "keep one\nkeep two\nordinary codex_api::sse::responses: SSE event: mention\nkeep three" {
		t.Fatalf("ordinary stderr = %q", ordinary.String())
	}
	assertCodexResponseMetadataError(t, got.Errors, "MALFORMED_EVENT")
	assertCodexResponseMetadataError(t, got.Errors, "LINE_BYTES_LIMIT")
	if strings.Contains(ordinary.String(), "{malformed}") || strings.Contains(ordinary.String(), strings.Repeat("x", 32)) {
		t.Fatal("raw trace reached ordinary stderr")
	}
}

func TestCodexResponseMetadataCollectorBounds(t *testing.T) {
	tests := []struct {
		name   string
		limits codexResponseMetadataLimits
		lines  []string
		code   string
	}{
		{
			name: "total bytes", limits: codexResponseMetadataLimits{MaxTotalBytes: 4},
			lines: []string{"ordinary\n"}, code: "TOTAL_BYTES_LIMIT",
		},
		{
			name: "events", limits: codexResponseMetadataLimits{MaxEvents: 1},
			lines: []string{
				"TRACE codex_api::sse::responses: SSE event: {\"type\":\"response.output_text.delta\"}\n",
				"TRACE codex_api::sse::responses: SSE event: {\"type\":\"response.output_text.done\"}\n",
			}, code: "EVENT_LIMIT",
		},
		{
			name: "responses", limits: codexResponseMetadataLimits{MaxResponses: 1},
			lines: []string{
				"TRACE codex_api::sse::responses: SSE event: {\"type\":\"response.created\",\"response\":{\"id\":\"resp_first\",\"status\":\"in_progress\"}}\n",
				"TRACE codex_api::sse::responses: SSE event: {\"type\":\"response.completed\",\"response\":{\"id\":\"resp_first\",\"status\":\"completed\",\"model\":\"model\"}}\n",
				"TRACE codex_api::sse::responses: SSE event: {\"type\":\"response.created\",\"response\":{\"id\":\"resp_second\",\"status\":\"in_progress\"}}\n",
			}, code: "RESPONSE_LIMIT",
		},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			w := newCodexResponseMetadataWriter(&bytes.Buffer{}, tc.limits)
			for _, line := range tc.lines {
				w.Write([]byte(line))
			}
			assertCodexResponseMetadataError(t, w.Finish().Errors, tc.code)
		})
	}
	if defaults := newCodexResponseMetadataWriter(&bytes.Buffer{}, codexResponseMetadataLimits{}).limits; defaults != (codexResponseMetadataLimits{
		MaxLineBytes: codexResponseMetadataMaxLineBytes, MaxTotalBytes: codexResponseMetadataMaxTotalBytes,
		MaxEvents: codexResponseMetadataMaxEvents, MaxResponses: codexResponseMetadataMaxResponses,
	}) {
		t.Fatalf("default limits = %+v", defaults)
	}
}

func TestCodexResponseMetadataGateAndEnvironment(t *testing.T) {
	cases := []struct {
		name string
		cfg  Config
		want bool
	}{
		{"enabled", Config{CodexResponseMetadata: true, BuiltinRuntime: true, CLIVersion: "0.153.4"}, true},
		{"prefixed version", Config{CodexResponseMetadata: true, BuiltinRuntime: true, CLIVersion: "codex-cli 0.153.4"}, true},
		{"disabled", Config{BuiltinRuntime: true, CLIVersion: "0.153.4"}, false},
		{"custom runtime", Config{CodexResponseMetadata: true, CLIVersion: "0.153.4"}, false},
		{"wrong resolved version", Config{CodexResponseMetadata: true, BuiltinRuntime: true, CLIVersion: "0.153.5", CodexVersion: "0.153.4"}, false},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			if got := codexResponseMetadataEnabled(tc.cfg, 1); got != tc.want {
				t.Fatalf("enabled = %t, want %t", got, tc.want)
			}
		})
	}
	if codexResponseMetadataEnabled(cases[0].cfg, 0) {
		t.Fatal("attempt zero enabled response tracing")
	}
	original := map[string]string{"RUST_LOG": "existing", "KEEP": "value"}
	cloned := codexResponseMetadataEnv(original)
	if original["RUST_LOG"] != "existing" || cloned["RUST_LOG"] != codexResponseMetadataRustLog || cloned["KEEP"] != "value" {
		t.Fatalf("environment clone mismatch: original=%v cloned=%v", original, cloned)
	}
}

func TestCodexResponseMetadataLogOmitsMissingThreadDigest(t *testing.T) {
	var output bytes.Buffer
	logger := slog.New(slog.NewJSONHandler(&output, nil))
	projection := codexResponseMetadataProjection{
		Schema: codexResponseMetadataSchema, Status: "missing", Source: codexResponseMetadataSource,
		ModelSource: "missing", Responses: []codexResponseMetadataResponse{}, UsageStatus: "missing", Errors: []codexResponseMetadataError{},
	}
	logCodexResponseMetadata(logger, Config{TaskID: "task", RuntimeID: "runtime"}, 42, 1, "/work", "", projection)
	var entry map[string]any
	if err := json.Unmarshal(output.Bytes(), &entry); err != nil {
		t.Fatal(err)
	}
	if _, present := entry["thread_id_digest"]; present {
		t.Fatal("missing thread digest was emitted")
	}
	if _, present := entry["metadata"]; present {
		t.Fatal("legacy metadata key was emitted")
	}
	if _, present := entry["response_metadata"]; !present {
		t.Fatal("response_metadata key is missing")
	}
}

func assertCodexResponseMetadataError(t *testing.T, errors []codexResponseMetadataError, code string) {
	t.Helper()
	for _, item := range errors {
		if item.Code == code && item.Count > 0 {
			return
		}
	}
	t.Fatalf("missing error %s in %+v", code, errors)
}
