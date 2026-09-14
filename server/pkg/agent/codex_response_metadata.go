package agent

import (
	"bytes"
	"crypto/sha256"
	"encoding/json"
	"fmt"
	"io"
	"log/slog"
	"math"
	"regexp"
	"sort"
	"strings"
	"sync"
	"unicode/utf8"
)

const (
	codexResponseMetadataSchema  = "codex_response_metadata/v1"
	codexResponseMetadataSource  = "codex_sse_response_body"
	codexResponseMetadataTarget  = "codex_api::sse::responses"
	codexResponseMetadataMarker  = "codex_api::sse::responses: SSE event: "
	codexResponseMetadataRustLog = "off,codex_api::sse::responses=trace"

	codexResponseMetadataMaxLineBytes  = 1 << 20
	codexResponseMetadataMaxTotalBytes = 16 << 20
	codexResponseMetadataMaxEvents     = 32768
	codexResponseMetadataMaxResponses  = 128
)

var (
	codexResponseMetadataResponseID = regexp.MustCompile(`^resp_[A-Za-z0-9_-]{1,128}$`)
	codexResponseMetadataModelID    = regexp.MustCompile(`^[A-Za-z0-9][A-Za-z0-9._:/-]{0,255}$`)
	codexResponseMetadataThreadID   = regexp.MustCompile(`^[A-Za-z0-9][A-Za-z0-9_-]{0,255}$`)
)

type codexResponseMetadataLimits struct {
	MaxLineBytes  int
	MaxTotalBytes int64
	MaxEvents     int
	MaxResponses  int
}

type codexResponseMetadataProjection struct {
	Schema            string                          `json:"schema"`
	Status            string                          `json:"status"`
	Source            string                          `json:"source"`
	ActualModel       *string                         `json:"actual_model"`
	ActualModelDigest *string                         `json:"actual_model_digest"`
	ModelSource       string                          `json:"model_source"`
	Responses         []codexResponseMetadataResponse `json:"responses"`
	UsageStatus       string                          `json:"usage_status"`
	Usage             *codexResponseMetadataUsage     `json:"usage"`
	Errors            []codexResponseMetadataError    `json:"errors"`
}

type codexResponseMetadataResponse struct {
	ResponseIDDigest string                      `json:"response_id_digest"`
	Model            *string                     `json:"model"`
	ModelDigest      *string                     `json:"model_digest"`
	TerminalStatus   *string                     `json:"terminal_status"`
	UsageStatus      string                      `json:"usage_status"`
	Usage            *codexResponseMetadataUsage `json:"usage"`
}

type codexResponseMetadataUsage struct {
	UncachedInputTokens   int64 `json:"uncached_input_tokens"`
	CacheReadInputTokens  int64 `json:"cache_read_input_tokens"`
	CacheWriteInputTokens int64 `json:"cache_write_input_tokens"`
	OutputTokens          int64 `json:"output_tokens"`
	ResponseCount         int64 `json:"response_count,omitempty"`
}

type codexResponseMetadataError struct {
	Code  string `json:"code"`
	Count int    `json:"count"`
}

type codexResponseMetadataState struct {
	projection codexResponseMetadataResponse
	inProgress bool
}

// codexResponseMetadataWriter removes the opted-in raw SSE trace before it can
// reach stderrTail while retaining only the fixed projection above.
type codexResponseMetadataWriter struct {
	mu     sync.Mutex
	inner  io.Writer
	limits codexResponseMetadataLimits

	line         []byte
	lineOverflow bool
	totalBytes   int64
	eventCount   int
	collecting   bool
	finished     bool

	errors        map[string]int
	responses     map[string]*codexResponseMetadataState
	responseOrder []string
	models        map[string]struct{}
	active        string
	lastSequence  *int64
	result        *codexResponseMetadataProjection
}

func newCodexResponseMetadataWriter(inner io.Writer, limits codexResponseMetadataLimits) *codexResponseMetadataWriter {
	if limits.MaxLineBytes <= 0 {
		limits.MaxLineBytes = codexResponseMetadataMaxLineBytes
	}
	if limits.MaxTotalBytes <= 0 {
		limits.MaxTotalBytes = codexResponseMetadataMaxTotalBytes
	}
	if limits.MaxEvents <= 0 {
		limits.MaxEvents = codexResponseMetadataMaxEvents
	}
	if limits.MaxResponses <= 0 {
		limits.MaxResponses = codexResponseMetadataMaxResponses
	}
	return &codexResponseMetadataWriter{
		inner: inner, limits: limits, collecting: true,
		errors: make(map[string]int), responses: make(map[string]*codexResponseMetadataState),
		models: make(map[string]struct{}),
	}
}

func (w *codexResponseMetadataWriter) Write(p []byte) (int, error) {
	w.mu.Lock()
	defer w.mu.Unlock()
	written := len(p)
	if w.finished {
		return written, nil
	}
	if w.collecting {
		if int64(len(p)) > math.MaxInt64-w.totalBytes || w.totalBytes+int64(len(p)) > w.limits.MaxTotalBytes {
			w.stop("TOTAL_BYTES_LIMIT")
		} else {
			w.totalBytes += int64(len(p))
		}
	}
	for len(p) > 0 {
		newline := bytes.IndexByte(p, '\n')
		segment := p
		complete := false
		if newline >= 0 {
			segment = p[:newline]
			complete = true
		}
		if !w.lineOverflow {
			if len(segment) > w.limits.MaxLineBytes-len(w.line) {
				w.stop("LINE_BYTES_LIMIT")
				w.line = nil
				w.lineOverflow = true
			} else {
				w.line = append(w.line, segment...)
			}
		}
		if complete {
			if !w.lineOverflow {
				if err := w.processLine(w.line, true); err != nil {
					return 0, err
				}
			}
			w.line = nil
			w.lineOverflow = false
			p = p[newline+1:]
			continue
		}
		break
	}
	return written, nil
}

func (w *codexResponseMetadataWriter) Finish() codexResponseMetadataProjection {
	w.mu.Lock()
	defer w.mu.Unlock()
	if w.result != nil {
		return *w.result
	}
	w.finished = true
	if w.lineOverflow {
		w.stop("TRUNCATED_LINE")
	} else if len(w.line) > 0 {
		w.stop("TRUNCATED_LINE")
		_ = w.processLine(w.line, false)
	}
	for _, digest := range w.responseOrder {
		if w.responses[digest].projection.TerminalStatus == nil {
			w.addError("RESPONSE_TERMINAL_MISSING")
		}
	}
	if len(w.responses) == 0 && len(w.errors) == 0 {
		w.addError("RESPONSE_EVENTS_MISSING")
	}
	projection := w.project()
	w.result = &projection
	return projection
}

func (w *codexResponseMetadataWriter) processLine(line []byte, newline bool) error {
	target := bytes.Index(line, []byte(codexResponseMetadataTarget))
	eventMessage := bytes.Index(line, []byte("SSE event"))
	isTrace := target >= 0 && eventMessage > target && bytes.Contains(line, []byte("TRACE"))
	if !utf8.Valid(line) {
		w.stop("BAD_UTF8")
		if isTrace {
			return nil
		}
	}
	if !isTrace {
		if _, err := w.inner.Write(line); err != nil {
			return err
		}
		if newline {
			_, err := w.inner.Write([]byte{'\n'})
			return err
		}
		return nil
	}
	if !newline {
		w.stop("TRUNCATED_LINE")
		return nil
	}
	if !w.collecting {
		return nil
	}
	w.eventCount++
	if w.eventCount > w.limits.MaxEvents {
		w.stop("EVENT_LIMIT")
		return nil
	}
	var event map[string]any
	payload := bytes.IndexByte(line[eventMessage+len("SSE event"):], '{')
	if payload < 0 {
		w.addError("MALFORMED_EVENT")
		return nil
	}
	payload += eventMessage + len("SSE event")
	if json.Unmarshal(line[payload:], &event) != nil {
		w.addError("MALFORMED_EVENT")
		return nil
	}
	w.observeEvent(event)
	return nil
}

func (w *codexResponseMetadataWriter) observeEvent(event map[string]any) {
	eventType, ok := event["type"].(string)
	if !ok || eventType == "" {
		w.addError("MALFORMED_EVENT")
		return
	}
	sequence, sequenceOK := codexResponseMetadataInteger(event["sequence_number"])
	if _, present := event["sequence_number"]; present && (!sequenceOK || sequence < 0) {
		w.addError("MALFORMED_EVENT")
		return
	}
	if eventType == "response.headers" {
		return
	}
	terminal := eventType == "response.completed" || eventType == "response.failed" || eventType == "response.incomplete"
	lifecycle := eventType == "response.created" || eventType == "response.in_progress" || terminal
	if !lifecycle {
		w.observeSequence(sequence, sequenceOK)
		return
	}
	response, ok := event["response"].(map[string]any)
	if !ok {
		w.addError("MALFORMED_EVENT")
		return
	}
	responseID, _ := response["id"].(string)
	if !codexResponseMetadataResponseID.MatchString(responseID) {
		w.addError("MALFORMED_EVENT")
		return
	}
	digest := codexResponseMetadataDigest(responseID)
	expectedStatus := strings.TrimPrefix(eventType, "response.")
	if eventType == "response.created" {
		expectedStatus = "in_progress"
	}
	if response["status"] != expectedStatus {
		w.addError("MALFORMED_EVENT")
		return
	}
	rawModel, modelFieldPresent := response["model"]
	model, modelPresent, valid := codexResponseMetadataModel(rawModel, modelFieldPresent, terminal)
	if !valid {
		w.addError("MALFORMED_EVENT")
		return
	}
	if eventType == "response.created" {
		if _, exists := w.responses[digest]; exists {
			w.addError("RESPONSE_ID_COLLISION")
			return
		}
		if len(w.responses) >= w.limits.MaxResponses {
			w.stop("RESPONSE_LIMIT")
			return
		}
		if w.active != "" {
			w.addError("RESPONSE_EVENT_OUT_OF_ORDER")
		}
		state := &codexResponseMetadataState{projection: codexResponseMetadataResponse{
			ResponseIDDigest: digest, UsageStatus: "missing",
		}}
		w.responses[digest] = state
		w.responseOrder = append(w.responseOrder, digest)
		w.active = digest
		w.lastSequence = nil
		w.observeSequence(sequence, sequenceOK)
		w.observeModel(state, model, modelPresent)
		return
	}
	state := w.responses[digest]
	if state == nil {
		w.addError("RESPONSE_CREATED_MISSING")
		return
	}
	if state.projection.TerminalStatus != nil {
		if terminal {
			w.addError("RESPONSE_TERMINAL_CONFLICT")
		} else {
			w.addError("RESPONSE_EVENT_OUT_OF_ORDER")
		}
		return
	}
	if w.active != digest {
		w.addError("RESPONSE_EVENT_OUT_OF_ORDER")
	}
	w.observeSequence(sequence, sequenceOK)
	w.observeModel(state, model, modelPresent)
	if eventType == "response.in_progress" {
		if state.inProgress {
			w.addError("RESPONSE_EVENT_OUT_OF_ORDER")
		}
		state.inProgress = true
		return
	}
	status := expectedStatus
	state.projection.TerminalStatus = &status
	if eventType == "response.completed" {
		usage, present := response["usage"]
		if !present || usage == nil {
			w.addError("USAGE_MISSING")
		} else if parsed, ok := codexResponseMetadataParseUsage(usage); ok {
			state.projection.UsageStatus = "observed"
			state.projection.Usage = &parsed
		} else {
			state.projection.UsageStatus = "failed"
			w.addError("USAGE_INVALID")
		}
	} else if eventType == "response.failed" {
		w.addError("RESPONSE_TERMINAL_FAILED")
	} else {
		w.addError("RESPONSE_TERMINAL_INCOMPLETE")
	}
	if w.active == digest {
		w.active = ""
		w.lastSequence = nil
	}
}

func (w *codexResponseMetadataWriter) observeSequence(sequence int64, present bool) {
	if !present {
		return
	}
	if w.active == "" || (w.lastSequence != nil && sequence <= *w.lastSequence) {
		w.addError("SEQUENCE_OUT_OF_ORDER")
		return
	}
	value := sequence
	w.lastSequence = &value
}

func (w *codexResponseMetadataWriter) observeModel(state *codexResponseMetadataState, model string, present bool) {
	if !present {
		return
	}
	conflict := (state.projection.Model != nil && *state.projection.Model != model) || (len(w.models) > 0 && !codexResponseMetadataSetHas(w.models, model))
	if state.projection.Model == nil {
		value, digest := model, codexResponseMetadataDigest(model)
		state.projection.Model, state.projection.ModelDigest = &value, &digest
	}
	w.models[model] = struct{}{}
	if conflict {
		w.addError("MODEL_CONFLICT")
	}
}

func (w *codexResponseMetadataWriter) stop(code string) {
	if w.collecting {
		w.addError(code)
		w.collecting = false
	}
}

func (w *codexResponseMetadataWriter) addError(code string) {
	w.errors[code]++
}

func (w *codexResponseMetadataWriter) project() codexResponseMetadataProjection {
	responses := make([]codexResponseMetadataResponse, 0, len(w.responseOrder))
	for _, digest := range w.responseOrder {
		responses = append(responses, w.responses[digest].projection)
	}
	usageStatus, usage, usageError := codexResponseMetadataAggregateUsage(responses)
	if usageError != "" {
		w.addError(usageError)
	}
	status := codexResponseMetadataStatus(w.errors)
	var actualModel, actualModelDigest *string
	modelSource := "missing"
	if status == "observed" && len(w.models) == 1 {
		for model := range w.models {
			value, digest := model, codexResponseMetadataDigest(model)
			actualModel, actualModelDigest = &value, &digest
		}
		modelSource = "provider_reported"
	}
	errors := make([]codexResponseMetadataError, 0, len(w.errors))
	for code, count := range w.errors {
		errors = append(errors, codexResponseMetadataError{Code: code, Count: count})
	}
	sort.Slice(errors, func(i, j int) bool { return errors[i].Code < errors[j].Code })
	return codexResponseMetadataProjection{
		Schema: codexResponseMetadataSchema, Status: status, Source: codexResponseMetadataSource,
		ActualModel: actualModel, ActualModelDigest: actualModelDigest, ModelSource: modelSource,
		Responses: responses, UsageStatus: usageStatus, Usage: usage, Errors: errors,
	}
}

func codexResponseMetadataParseUsage(value any) (codexResponseMetadataUsage, bool) {
	usage, ok := value.(map[string]any)
	if !ok {
		return codexResponseMetadataUsage{}, false
	}
	input, inputOK := codexResponseMetadataInteger(usage["input_tokens"])
	output, outputOK := codexResponseMetadataInteger(usage["output_tokens"])
	details, detailsOK := usage["input_tokens_details"].(map[string]any)
	cached, cachedOK := codexResponseMetadataInteger(details["cached_tokens"])
	if !inputOK || !outputOK || !detailsOK || !cachedOK || input < 0 || output < 0 || cached < 0 {
		return codexResponseMetadataUsage{}, false
	}
	written := int64(0)
	if raw, present := details["cache_write_tokens"]; present {
		var writtenOK bool
		written, writtenOK = codexResponseMetadataInteger(raw)
		if !writtenOK || written < 0 {
			return codexResponseMetadataUsage{}, false
		}
	}
	if cached > input || written > input-cached {
		return codexResponseMetadataUsage{}, false
	}
	if raw, present := usage["total_tokens"]; present {
		total, ok := codexResponseMetadataInteger(raw)
		if !ok || total < 0 || input > math.MaxInt64-output || total != input+output {
			return codexResponseMetadataUsage{}, false
		}
	}
	if outputDetails, ok := usage["output_tokens_details"].(map[string]any); ok {
		if raw, present := outputDetails["reasoning_tokens"]; present {
			reasoning, ok := codexResponseMetadataInteger(raw)
			if !ok || reasoning < 0 || reasoning > output {
				return codexResponseMetadataUsage{}, false
			}
		}
	}
	return codexResponseMetadataUsage{
		UncachedInputTokens: input - cached - written, CacheReadInputTokens: cached,
		CacheWriteInputTokens: written, OutputTokens: output,
	}, true
}

func codexResponseMetadataAggregateUsage(responses []codexResponseMetadataResponse) (string, *codexResponseMetadataUsage, string) {
	if len(responses) == 0 {
		return "missing", nil, ""
	}
	for _, response := range responses {
		if response.UsageStatus == "failed" {
			return "failed", nil, ""
		}
		if response.UsageStatus != "observed" || response.Usage == nil || response.TerminalStatus == nil || *response.TerminalStatus != "completed" {
			return "missing", nil, ""
		}
	}
	total := codexResponseMetadataUsage{ResponseCount: int64(len(responses))}
	for _, response := range responses {
		values := []struct {
			dst   *int64
			value int64
		}{
			{&total.UncachedInputTokens, response.Usage.UncachedInputTokens},
			{&total.CacheReadInputTokens, response.Usage.CacheReadInputTokens},
			{&total.CacheWriteInputTokens, response.Usage.CacheWriteInputTokens},
			{&total.OutputTokens, response.Usage.OutputTokens},
		}
		for _, value := range values {
			if value.value < 0 || *value.dst > (1<<53-1)-value.value {
				return "failed", nil, "USAGE_INVALID"
			}
			*value.dst += value.value
		}
	}
	return "observed", &total, ""
}

func codexResponseMetadataStatus(errors map[string]int) string {
	failed := map[string]struct{}{
		"BAD_UTF8": {}, "EVENT_LIMIT": {}, "LINE_BYTES_LIMIT": {}, "MALFORMED_EVENT": {},
		"RESPONSE_EVENT_OUT_OF_ORDER": {}, "RESPONSE_LIMIT": {}, "RESPONSE_TERMINAL_FAILED": {},
		"RESPONSE_TERMINAL_INCOMPLETE": {}, "TOTAL_BYTES_LIMIT": {}, "TRUNCATED_LINE": {}, "SEQUENCE_OUT_OF_ORDER": {},
	}
	conflicting := map[string]struct{}{"MODEL_CONFLICT": {}, "RESPONSE_ID_COLLISION": {}, "RESPONSE_TERMINAL_CONFLICT": {}}
	for code := range errors {
		if _, ok := failed[code]; ok {
			return "failed"
		}
	}
	for code := range errors {
		if _, ok := conflicting[code]; ok {
			return "conflicting"
		}
	}
	for code := range errors {
		if code != "USAGE_INVALID" && code != "USAGE_MISSING" {
			return "missing"
		}
	}
	return "observed"
}

func codexResponseMetadataInteger(value any) (int64, bool) {
	number, ok := value.(float64)
	if !ok || math.IsNaN(number) || math.IsInf(number, 0) || number != math.Trunc(number) || number > 1<<53-1 || number < -(1<<53-1) {
		return 0, false
	}
	return int64(number), true
}

func codexResponseMetadataModel(value any, present, required bool) (string, bool, bool) {
	if !present && !required {
		return "", false, true
	}
	model, ok := value.(string)
	if !ok || !codexResponseMetadataModelID.MatchString(model) || strings.Contains(model, "://") {
		return "", false, false
	}
	return model, true, true
}

func codexResponseMetadataDigest(value string) string {
	return fmt.Sprintf("sha256:%x", sha256.Sum256([]byte(value)))
}

func codexResponseMetadataSetHas(values map[string]struct{}, value string) bool {
	_, ok := values[value]
	return ok
}

func codexResponseMetadataExecutionEvidence(existing *ExecutionEvidence, projection *codexResponseMetadataProjection) *ExecutionEvidence {
	result := cloneExecutionEvidence(existing)
	if projection == nil {
		return result
	}
	if result != nil {
		result.ProviderModel = ProviderModelEvidence{Source: EvidenceSourceMissing}
	}
	if projection.Schema != codexResponseMetadataSchema || projection.Source != codexResponseMetadataSource ||
		projection.Status != "observed" || projection.ModelSource != "provider_reported" ||
		projection.ActualModel == nil || projection.ActualModelDigest == nil ||
		!codexResponseMetadataModelID.MatchString(*projection.ActualModel) || strings.Contains(*projection.ActualModel, "://") ||
		*projection.ActualModelDigest != codexResponseMetadataDigest(*projection.ActualModel) {
		return result
	}
	if result == nil {
		result = &ExecutionEvidence{
			Usage:        UsageEvidence{Source: EvidenceSourceMissing},
			ProviderCost: ProviderCostEvidence{Authority: CostAuthorityMissing, Basis: CostBasisMissing, Source: EvidenceSourceMissing},
		}
	}
	result.ProviderModel = ProviderModelEvidence{Model: cloneEvidenceString(projection.ActualModel), Source: EvidenceSourceProviderEvent}
	return result
}

func codexResponseMetadataEnabled(cfg Config, attempt int) bool {
	version := strings.TrimPrefix(strings.TrimSpace(cfg.CLIVersion), "codex-cli ")
	return attempt > 0 && cfg.CodexResponseMetadata && cfg.BuiltinRuntime && version == "0.153.4"
}

func codexResponseMetadataEnv(env map[string]string) map[string]string {
	cloned := make(map[string]string, len(env)+1)
	for key, value := range env {
		cloned[key] = value
	}
	cloned["RUST_LOG"] = codexResponseMetadataRustLog
	return cloned
}

func logCodexResponseMetadata(logger *slog.Logger, cfg Config, pid, attempt int, workDir, threadID string, projection codexResponseMetadataProjection) {
	if logger == nil {
		return
	}
	fields := []any{
		"task_id", cfg.TaskID,
		"runtime_id", cfg.RuntimeID,
		"process_id", pid,
		"attempt", attempt,
		"work_dir", workDir,
	}
	if codexResponseMetadataThreadID.MatchString(threadID) {
		fields = append(fields, "thread_id_digest", codexResponseMetadataDigest(threadID))
	}
	fields = append(fields, "response_metadata", projection)
	logger.Info("codex response metadata observed", fields...)
}
