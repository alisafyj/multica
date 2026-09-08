//go:build !windows

package agent

import (
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"sync"
	"testing"
	"time"
)

const (
	realCodexSSETraceBinarySHA256 = "b973d440acac501fd2594a43e7ca9ce41e0a65b9dfb28d0d7a7837c99e1261e3"
	realCodexSSETraceMaxFiles     = 512
	realCodexSSETraceMaxDiskBytes = 32 << 20
)

func TestCodexRealCLISSETraceDoesNotPersistResponseBody(t *testing.T) {
	if os.Getenv("MULTICA_TEST_REAL_CODEX_SSE_TRACE") != "1" {
		t.Skip("set MULTICA_TEST_REAL_CODEX_SSE_TRACE=1 and MULTICA_TEST_REAL_CODEX_BIN to run the isolated no-auth SSE trace probe")
	}
	realCodex := strings.TrimSpace(os.Getenv("MULTICA_TEST_REAL_CODEX_BIN"))
	if !filepath.IsAbs(realCodex) {
		t.Fatal("MULTICA_TEST_REAL_CODEX_BIN must be an absolute pinned Codex executable")
	}
	binary, err := os.ReadFile(realCodex)
	if err != nil || fmt.Sprintf("%x", sha256.Sum256(binary)) != realCodexSSETraceBinarySHA256 {
		t.Fatal("native Codex executable differs from the frozen 0.153.4 binary")
	}

	root, err := filepath.EvalSymlinks(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	home := filepath.Join(root, "home")
	codexHome := filepath.Join(root, "codex-home")
	cwd := filepath.Join(root, "workspace")
	for _, dir := range []string{home, codexHome, cwd} {
		if err := os.MkdirAll(dir, 0o700); err != nil {
			t.Fatal(err)
		}
	}

	const (
		configuredModel = "sse-trace-request-model"
		fixtureTaskID   = "55555555-5555-4555-8555-555555555555"
		fixtureRuntime  = "66666666-6666-4666-8666-666666666666"
	)
	canary := "sse-trace-body-model-" + strconv.FormatInt(time.Now().UnixNano(), 36)
	var providerMu sync.Mutex
	requestCount := 0
	provider := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost || r.URL.Path != "/v1/responses" {
			t.Errorf("unexpected provider request route")
			w.WriteHeader(http.StatusNotFound)
			return
		}
		body, readErr := io.ReadAll(io.LimitReader(r.Body, 1<<20))
		if readErr != nil {
			t.Errorf("read provider request: %v", readErr)
			w.WriteHeader(http.StatusInternalServerError)
			return
		}
		var request map[string]any
		if json.Unmarshal(body, &request) != nil || request["model"] != configuredModel {
			t.Errorf("provider request did not use configured model")
			w.WriteHeader(http.StatusBadRequest)
			return
		}
		if r.Header.Get("Authorization") != "" || r.Header.Get("X-Api-Key") != "" {
			t.Errorf("no-auth provider received credential headers")
			w.WriteHeader(http.StatusBadRequest)
			return
		}
		providerMu.Lock()
		requestCount++
		providerMu.Unlock()
		w.Header().Set("Content-Type", "text/event-stream")
		w.Header().Set("Cache-Control", "no-cache")
		writeCodexSSETraceResponse(t, w, canary)
	}))
	t.Cleanup(provider.Close)

	config := strings.Join([]string{
		`model = "` + configuredModel + `"`,
		`model_provider = "local_trace"`,
		`approval_policy = "never"`,
		`sandbox_mode = "read-only"`,
		`disable_response_storage = true`,
		`hide_agent_reasoning = true`,
		`[model_providers.local_trace]`,
		`name = "Local SSE trace fixture"`,
		`base_url = ` + strconv.Quote(provider.URL+"/v1"),
		`wire_api = "responses"`,
		`requires_openai_auth = false`,
		`[features]`,
		`responses_websockets = false`,
		`multi_agent = false`,
		`memories = false`,
		`plugins = false`,
		``,
	}, "\n")
	if err := os.WriteFile(filepath.Join(codexHome, "config.toml"), []byte(config), 0o600); err != nil {
		t.Fatal(err)
	}

	pidMarker := filepath.Join(root, "app-server.pid")
	wrapper := filepath.Join(root, "codex-isolated")
	wrapperBody := "#!/bin/sh\n" +
		realCodexWatchdogWriteIdentityCommand(pidMarker) + "\n" +
		"[ \"$RUST_LOG\" = '" + codexResponseMetadataRustLog + "' ] || exit 97\n" +
		"exec env -i " +
		"PATH=" + realCodexWatchdogShellQuote(os.Getenv("PATH")) + " " +
		"HOME=" + realCodexWatchdogShellQuote(home) + " " +
		"CODEX_HOME=" + realCodexWatchdogShellQuote(codexHome) + " " +
		"TMPDIR=" + realCodexWatchdogShellQuote(root) + " " +
		"NO_PROXY='127.0.0.1,localhost,::1' OTEL_SDK_DISABLED=true " +
		"RUST_LOG=\"$RUST_LOG\" " +
		realCodexWatchdogShellQuote(realCodex) + " \"$@\"\n"
	if err := os.WriteFile(wrapper, []byte(wrapperBody), 0o700); err != nil {
		t.Fatal(err)
	}

	var observationLog bytes.Buffer
	logger := slog.New(slog.NewJSONHandler(&observationLog, nil)).With("component", "daemon", "task", fixtureTaskID)
	backend := &codexBackend{cfg: Config{
		ExecutablePath:        wrapper,
		CLIVersion:            "codex-cli 0.153.4",
		CodexVersion:          "0.153.4",
		BuiltinRuntime:        true,
		CodexResponseMetadata: true,
		Env: map[string]string{
			"HOME": home, "CODEX_HOME": codexHome, "TMPDIR": root,
			"NO_PROXY": "127.0.0.1,localhost,::1", "OTEL_SDK_DISABLED": "true",
			"RUST_LOG": "inherited-value-must-be-replaced",
		},
		Logger: logger, WorkDir: cwd,
		TaskID: fixtureTaskID, RuntimeID: fixtureRuntime,
	}}
	ctx, cancel := context.WithTimeout(t.Context(), 20*time.Second)
	defer cancel()
	session, err := backend.executeOnce(ctx, "Return the fixture response.", ExecOptions{Cwd: cwd, Timeout: 15 * time.Second}, 1)
	if err != nil {
		t.Fatal(err)
	}
	identity, ok := waitRealCodexWatchdogProcessIdentity(pidMarker, time.Second)
	if !ok {
		t.Fatal("native Codex process identity was not established")
	}
	var finalText string
	messagesDone := make(chan struct{})
	go func() {
		defer close(messagesDone)
		for message := range session.Messages {
			if message.Type == MessageText {
				finalText = message.Content
			}
		}
	}()
	var result Result
	select {
	case received, open := <-session.Result:
		if !open {
			t.Fatal("product Codex session closed without a result")
		}
		result = received
	case <-ctx.Done():
		t.Fatal("product Codex session did not complete")
	}
	<-messagesDone
	if result.Status != "completed" || result.Output != "sse-trace-complete" {
		t.Fatalf("product protocol result: status=%q output=%q error=%q", result.Status, result.Output, result.Error)
	}
	if result.ExecutionEvidence == nil || result.ExecutionEvidence.ProviderModel.Model == nil ||
		*result.ExecutionEvidence.ProviderModel.Model != canary || result.ExecutionEvidence.ProviderModel.Source != EvidenceSourceProviderEvent {
		t.Fatalf("product provider model evidence = %+v", result.ExecutionEvidence)
	}
	if finalText != "sse-trace-complete" {
		t.Fatalf("protocol final text = %q", finalText)
	}
	providerMu.Lock()
	requests := requestCount
	providerMu.Unlock()
	if requests != 1 {
		t.Fatalf("provider request count = %d, want 1", requests)
	}

	if !waitRealCodexWatchdogProcessGroupGone(identity.pid, time.Second) {
		t.Fatal("product Codex process group remained after successful result")
	}

	logText := observationLog.String()
	if strings.Contains(logText, codexResponseMetadataMarker) || strings.Contains(logText, "resp_sse_trace") {
		t.Fatal("raw SSE trace reached product logs")
	}
	observationCount := 0
	bindingCount := 0
	spanCount := 0
	for _, line := range strings.Split(logText, "\n") {
		var entry struct {
			Time           time.Time                       `json:"time"`
			Component      string                          `json:"component"`
			Task           string                          `json:"task"`
			Message        string                          `json:"msg"`
			Phase          string                          `json:"phase"`
			TaskID         string                          `json:"task_id"`
			RuntimeID      string                          `json:"runtime_id"`
			PID            int                             `json:"pid"`
			ProcessID      int                             `json:"process_id"`
			Attempt        int                             `json:"attempt"`
			Cwd            string                          `json:"cwd"`
			WorkDir        string                          `json:"work_dir"`
			ThreadIDDigest *string                         `json:"thread_id_digest"`
			Metadata       codexResponseMetadataProjection `json:"response_metadata"`
			ExecutionSpan  codexExecutionSpan              `json:"execution_span"`
		}
		if json.Unmarshal([]byte(line), &entry) != nil {
			continue
		}
		if entry.Message == "codex lifecycle" && entry.Phase == "spawn" {
			if entry.Time.IsZero() || entry.Component != "daemon" || entry.Task != fixtureTaskID || entry.TaskID != fixtureTaskID ||
				entry.RuntimeID != fixtureRuntime || entry.PID != identity.pid || entry.Attempt != 1 || entry.Cwd != cwd {
				t.Fatal("spawn lifecycle observation is not bound to the product execution")
			}
			t.Logf("CODEX_SSE_BINDING_LOG %s", line)
			bindingCount++
			continue
		}
		if entry.Message == "codex execution span observed" {
			startedAt, startErr := time.Parse(time.RFC3339Nano, entry.ExecutionSpan.StartedAt)
			endedAt, endErr := time.Parse(time.RFC3339Nano, entry.ExecutionSpan.EndedAt)
			if entry.Time.IsZero() || entry.Component != "daemon" || entry.Task != fixtureTaskID || entry.TaskID != fixtureTaskID ||
				entry.RuntimeID != fixtureRuntime || entry.ProcessID != identity.pid || entry.Attempt != 1 || entry.WorkDir != cwd ||
				entry.ExecutionSpan.Schema != codexExecutionSpanSchema || entry.ExecutionSpan.Scope != "post_spawn_through_cleanup" ||
				entry.ExecutionSpan.ClockSource != "monotonic" || !entry.ExecutionSpan.CleanupConfirmed || entry.ExecutionSpan.DurationMS < 0 ||
				startErr != nil || endErr != nil || endedAt.Before(startedAt) {
				t.Fatalf("execution span observation is not bound to the product execution: %+v", entry.ExecutionSpan)
			}
			t.Logf("CODEX_SSE_SPAN_LOG %s", line)
			spanCount++
			continue
		}
		if entry.Message != "codex response metadata observed" {
			continue
		}
		if entry.TaskID != backend.cfg.TaskID || entry.RuntimeID != backend.cfg.RuntimeID || entry.ProcessID != identity.pid ||
			entry.Time.IsZero() || entry.Component != "daemon" || entry.Task != fixtureTaskID ||
			entry.Attempt != 1 || entry.WorkDir != cwd || entry.ThreadIDDigest == nil {
			t.Fatal("response metadata observation is not bound to the product execution")
		}
		if entry.Metadata.Status != "observed" || entry.Metadata.ActualModel == nil || *entry.Metadata.ActualModel != canary ||
			entry.Metadata.ModelSource != "provider_reported" || len(entry.Metadata.Responses) != 1 {
			t.Fatalf("unexpected response metadata projection: %+v", entry.Metadata)
		}
		t.Logf("CODEX_SSE_METADATA_LOG %s", line)
		observationCount++
	}
	if observationCount != 1 {
		t.Fatalf("response metadata observation count = %d, want 1", observationCount)
	}
	if bindingCount != 1 {
		t.Fatalf("spawn lifecycle binding count = %d, want 1", bindingCount)
	}
	if spanCount != 1 {
		t.Fatalf("execution span observation count = %d, want 1", spanCount)
	}
	files, diskBytes, persisted := scanCodexSSETracePersistence(t, root, canary)
	if persisted != 0 {
		t.Fatalf("response model canary persisted in %d isolated-state files", persisted)
	}
	receipt := map[string]any{
		"schema": "codex_sse_trace_persistence/v1", "provider_requests": requests,
		"protocol_completed": true, "product_filter_observed": true, "provider_model_evidence_observed": true,
		"metadata_observations": observationCount, "binding_observations": bindingCount,
		"execution_span_observations": spanCount,
		"trace_canary_digest":         fmt.Sprintf("sha256:%x", sha256.Sum256([]byte(canary))),
		"configured_model_digest":     fmt.Sprintf("sha256:%x", sha256.Sum256([]byte(configuredModel))),
		"stderr_line_limit":           codexResponseMetadataMaxLineBytes, "stderr_total_limit": codexResponseMetadataMaxTotalBytes,
		"files_scanned": files, "disk_bytes_scanned": diskBytes,
		"disk_file_limit": realCodexSSETraceMaxFiles, "disk_byte_limit": realCodexSSETraceMaxDiskBytes,
		"persisted_canary_matches": persisted, "credentials_used": false, "tools_used": false,
	}
	t.Logf("CODEX_SSE_TRACE_PERSISTENCE %s", mustRealCodexWatchdogJSON(t, receipt))
}

func writeCodexSSETraceResponse(t *testing.T, w io.Writer, model string) {
	t.Helper()
	item := map[string]any{
		"type": "message", "id": "msg_sse_trace", "status": "completed", "role": "assistant",
		"content": []any{map[string]any{"type": "output_text", "text": "sse-trace-complete", "annotations": []any{}}},
	}
	envelope := func(status string, output []any, usage any) map[string]any {
		return map[string]any{
			"id": "resp_sse_trace", "object": "response", "created_at": 0, "status": status,
			"error": nil, "incomplete_details": nil, "instructions": nil, "max_output_tokens": nil,
			"model": model, "output": output, "parallel_tool_calls": false,
			"previous_response_id": nil, "reasoning": nil, "store": false, "temperature": nil,
			"text": map[string]any{"format": map[string]any{"type": "text"}}, "tool_choice": "auto",
			"tools": []any{}, "top_p": nil, "truncation": "disabled", "usage": usage,
			"user": nil, "metadata": map[string]any{},
		}
	}
	write := func(event map[string]any) {
		_, err := fmt.Fprintf(w, "event: %s\ndata: %s\n\n", event["type"], mustRealCodexWatchdogJSON(t, event))
		if err != nil {
			t.Errorf("write SSE trace fixture: %v", err)
		}
	}
	write(map[string]any{"type": "response.created", "sequence_number": 0, "response": envelope("in_progress", []any{}, nil)})
	write(map[string]any{"type": "response.output_item.done", "sequence_number": 1, "output_index": 0, "item": item})
	usage := map[string]any{
		"input_tokens": 1, "input_tokens_details": map[string]any{"cached_tokens": 0},
		"output_tokens": 1, "output_tokens_details": map[string]any{"reasoning_tokens": 0}, "total_tokens": 2,
	}
	write(map[string]any{"type": "response.completed", "sequence_number": 2, "response": envelope("completed", []any{item}, usage)})
	_, _ = io.WriteString(w, "data: [DONE]\n\n")
}

func scanCodexSSETracePersistence(t *testing.T, root, canary string) (int, int64, int) {
	t.Helper()
	files := 0
	var bytesRead int64
	matches := 0
	err := filepath.WalkDir(root, func(path string, entry os.DirEntry, walkErr error) error {
		if walkErr != nil {
			return walkErr
		}
		info, err := entry.Info()
		if err != nil {
			return err
		}
		if info.Mode()&os.ModeSymlink != 0 {
			return fmt.Errorf("isolated state contains symlink")
		}
		if !info.Mode().IsRegular() {
			return nil
		}
		files++
		if files > realCodexSSETraceMaxFiles || info.Size() < 0 || bytesRead+info.Size() > realCodexSSETraceMaxDiskBytes {
			return fmt.Errorf("isolated state scan exceeded bounds")
		}
		file, err := os.Open(path)
		if err != nil {
			return err
		}
		data, readErr := io.ReadAll(io.LimitReader(file, info.Size()+1))
		closeErr := file.Close()
		if readErr != nil {
			return readErr
		}
		if closeErr != nil {
			return closeErr
		}
		bytesRead += int64(len(data))
		if strings.Contains(string(data), canary) {
			matches++
		}
		return nil
	})
	if err != nil && !errors.Is(err, os.ErrNotExist) {
		t.Fatal(err)
	}
	return files, bytesRead, matches
}
