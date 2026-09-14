package agent

import (
	"crypto/sha256"
	"encoding/json"
	"fmt"
	"log/slog"
	"sort"
	"strings"
	"unicode/utf8"
)

const (
	claudeStreamMetadataSchema         = "claude_stream_metadata/v1"
	claudeStreamMetadataSource         = "claude_stream_json"
	claudeStreamMetadataMaxEvents      = 32768
	claudeStreamMetadataMaxModels      = 12
	claudeStreamMetadataMaxSafeInteger = int64(1<<53 - 1)
)

type claudeStreamMetadataProjection struct {
	Schema             string                           `json:"schema"`
	Source             string                           `json:"source"`
	Status             string                           `json:"status"`
	ConfiguredModel    *string                          `json:"configured_model"`
	MainModel          *string                          `json:"main_model"`
	MainModelSource    string                           `json:"main_model_source"`
	PermissionMode     *string                          `json:"permission_mode"`
	MCPServerCount     *int                             `json:"mcp_server_count"`
	SessionIDDigest    *string                          `json:"session_id_digest"`
	InitCount          int                              `json:"init_count"`
	MainAssistantCount int                              `json:"main_assistant_count"`
	TerminalCount      int                              `json:"terminal_count"`
	MainModels         []string                         `json:"main_models"`
	TerminalSuccess    *bool                            `json:"terminal_success"`
	ModelUsage         []claudeStreamMetadataModelUsage `json:"model_usage"`
	UsageStatus        string                           `json:"usage_status"`
	Usage              *claudeStreamMetadataUsage       `json:"usage"`
	Errors             []claudeStreamMetadataError      `json:"errors"`
}

type claudeStreamMetadataUsage struct {
	UncachedInputTokens   int64 `json:"uncached_input_tokens"`
	CacheReadInputTokens  int64 `json:"cache_read_input_tokens"`
	CacheWriteInputTokens int64 `json:"cache_write_input_tokens"`
	OutputTokens          int64 `json:"output_tokens"`
}

type claudeStreamMetadataModelUsage struct {
	Model       string                     `json:"model"`
	UsageStatus string                     `json:"usage_status"`
	Usage       *claudeStreamMetadataUsage `json:"usage"`
}

type claudeStreamMetadataError struct {
	Code  string `json:"code"`
	Count int    `json:"count"`
}

type claudeStreamMetadataCollector struct {
	eventCount int
	stopped    bool
	finished   bool
	errors     map[string]int

	initCount          int
	mainAssistantCount int
	terminalCount      int
	configuredModel    *string
	permissionMode     *string
	mcpServerCount     *int
	sessionDigest      *string
	sessionConflict    bool
	mainModels         map[string]struct{}
	terminalSuccess    *bool
	modelUsage         map[string]claudeStreamMetadataModelUsage
	usageStatus        string
	usage              *claudeStreamMetadataUsage
}

func newClaudeStreamMetadataCollector() *claudeStreamMetadataCollector {
	return &claudeStreamMetadataCollector{
		errors: make(map[string]int), mainModels: make(map[string]struct{}),
		modelUsage: make(map[string]claudeStreamMetadataModelUsage), usageStatus: "missing",
	}
}

func (c *claudeStreamMetadataCollector) Observe(line []byte) {
	if c.finished || c.stopped {
		return
	}
	if c.eventCount == claudeStreamMetadataMaxEvents {
		c.addError("EVENT_LIMIT")
		c.stopped = true
		return
	}
	c.eventCount++

	var event map[string]json.RawMessage
	if !utf8.Valid(line) || json.Unmarshal(line, &event) != nil || event == nil {
		c.addError("MALFORMED_EVENT")
		return
	}
	eventType, present, valid := claudeStreamMetadataString(event, "type")
	if !present || !valid || eventType == "" {
		c.addError("MALFORMED_EVENT")
		return
	}
	if c.terminalCount > 0 {
		if eventType == "result" {
			c.increment(&c.terminalCount)
			c.addError("TERMINAL_DUPLICATE")
		} else {
			c.addError("EVENT_AFTER_TERMINAL")
		}
		return
	}

	switch eventType {
	case "system":
		subtype, _, _ := claudeStreamMetadataString(event, "subtype")
		if subtype == "init" {
			c.observeInit(event)
		}
	case "assistant":
		c.observeAssistant(event)
	case "result":
		c.observeTerminal(event)
	}
}

func (c *claudeStreamMetadataCollector) observeInit(event map[string]json.RawMessage) {
	c.increment(&c.initCount)
	if c.initCount > 1 {
		c.addError("INIT_DUPLICATE")
		return
	}
	if c.mainAssistantCount > 0 {
		c.addError("EVENT_OUT_OF_ORDER")
	}
	c.observeSession(event)

	model, present, valid := claudeStreamMetadataString(event, "model")
	if !present {
		c.addError("CONFIGURATION_MISSING")
	} else if !valid || !validModelEvidenceName(model) {
		c.addError("MODEL_INVALID")
	} else {
		c.configuredModel = stringPointer(model)
	}
	mode, present, valid := claudeStreamMetadataString(event, "permissionMode")
	if !present {
		c.addError("CONFIGURATION_MISSING")
	} else if !valid || !claudeStreamMetadataPermissionMode(mode) {
		c.addError("MALFORMED_EVENT")
	} else {
		c.permissionMode = stringPointer(mode)
	}

	rawServers, present := event["mcp_servers"]
	if !present || string(rawServers) == "null" {
		c.addError("CONFIGURATION_MISSING")
	} else {
		var servers []json.RawMessage
		if json.Unmarshal(rawServers, &servers) != nil {
			c.addError("CONFIGURATION_MISSING")
			c.addError("MALFORMED_EVENT")
		} else if len(servers) > claudeStreamMetadataMaxEvents {
			c.addError("CONFIGURATION_MISSING")
			c.addError("EVENT_LIMIT")
		} else {
			count := len(servers)
			c.mcpServerCount = &count
		}
	}
}

func (c *claudeStreamMetadataCollector) observeAssistant(event map[string]json.RawMessage) {
	if c.initCount == 0 {
		c.addError("EVENT_OUT_OF_ORDER")
	}
	rawParent, present := event["parent_tool_use_id"]
	main := !present || string(rawParent) == "null"
	if present && !main {
		var parent string
		if json.Unmarshal(rawParent, &parent) != nil || parent == "" {
			c.addError("PARENT_INVALID")
			return
		}
		return
	}
	c.increment(&c.mainAssistantCount)
	c.observeSession(event)

	var message map[string]json.RawMessage
	if json.Unmarshal(event["message"], &message) != nil || message == nil {
		c.addError("MODEL_INVALID")
		return
	}
	model, present, valid := claudeStreamMetadataString(message, "model")
	if !present {
		c.addError("MAIN_MODEL_MISSING")
		return
	}
	if !valid || !validModelEvidenceName(model) {
		c.addError("MODEL_INVALID")
		return
	}
	if _, exists := c.mainModels[model]; exists {
		return
	}
	if len(c.mainModels) == claudeStreamMetadataMaxModels {
		c.addError("MODEL_LIMIT")
		return
	}
	c.mainModels[model] = struct{}{}
	if len(c.mainModels) > 1 {
		c.addError("MAIN_MODEL_CONFLICT")
	}
}

func (c *claudeStreamMetadataCollector) observeTerminal(event map[string]json.RawMessage) {
	c.increment(&c.terminalCount)
	if c.initCount == 0 || c.mainAssistantCount == 0 {
		c.addError("EVENT_OUT_OF_ORDER")
	}
	c.observeSession(event)

	subtype, subtypePresent, subtypeValid := claudeStreamMetadataString(event, "subtype")
	var isError *bool
	rawIsError, isErrorPresent := event["is_error"]
	if !subtypePresent || !subtypeValid || !isErrorPresent || json.Unmarshal(rawIsError, &isError) != nil || isError == nil {
		c.addError("MALFORMED_EVENT")
		c.observeModelUsage(event["modelUsage"])
		return
	}
	knownErrorSubtype := false
	switch subtype {
	case "success":
	case "error_max_turns", "error_during_execution", "error_max_budget_usd", "error_max_structured_output_retries":
		knownErrorSubtype = true
	default:
		c.addError("MALFORMED_EVENT")
		c.observeModelUsage(event["modelUsage"])
		return
	}
	success := subtype == "success" && !*isError
	c.terminalSuccess = &success
	if !success || knownErrorSubtype {
		c.addError("TERMINAL_FAILED")
	}
	c.observeModelUsage(event["modelUsage"])
}

func (c *claudeStreamMetadataCollector) observeModelUsage(raw json.RawMessage) {
	if len(raw) == 0 || string(raw) == "null" {
		c.addError("USAGE_MISSING")
		return
	}
	var rawByModel map[string]json.RawMessage
	if json.Unmarshal(raw, &rawByModel) != nil || rawByModel == nil {
		c.usageStatus = "failed"
		c.addError("USAGE_INVALID")
		return
	}
	models := make([]string, 0, len(rawByModel))
	for model := range rawByModel {
		models = append(models, model)
	}
	sort.Strings(models)
	if len(models) > claudeStreamMetadataMaxModels {
		c.addError("MODEL_LIMIT")
		models = models[:claudeStreamMetadataMaxModels]
	}
	if len(models) == 0 {
		c.addError("USAGE_MISSING")
		return
	}

	total := claudeStreamMetadataUsage{}
	overall := "observed"
	if len(rawByModel) > claudeStreamMetadataMaxModels {
		overall = "failed"
	}
	for _, model := range models {
		entry := claudeStreamMetadataModelUsage{Model: model, UsageStatus: "failed"}
		if !validModelEvidenceName(model) {
			c.addError("MODEL_INVALID")
			overall = "failed"
			continue
		}
		var native claudeResultModelUsage
		if json.Unmarshal(rawByModel[model], &native) != nil {
			c.addError("USAGE_INVALID")
			c.modelUsage[model] = entry
			overall = "failed"
			continue
		}
		values := []*int64{native.InputTokens, native.CacheReadInputTokens, native.CacheCreationInputTokens, native.OutputTokens}
		missing, invalid, overflow := false, false, false
		for _, value := range values {
			switch {
			case value == nil:
				missing = true
			case *value < 0:
				invalid = true
			case *value > claudeStreamMetadataMaxSafeInteger:
				overflow = true
			}
		}
		if overflow {
			c.addError("USAGE_OVERFLOW")
			c.modelUsage[model] = entry
			overall = "failed"
			continue
		}
		if invalid {
			c.addError("USAGE_INVALID")
			c.modelUsage[model] = entry
			overall = "failed"
			continue
		}
		if missing {
			entry.UsageStatus = "missing"
			c.addError("USAGE_MISSING")
			c.modelUsage[model] = entry
			if overall != "failed" {
				overall = "missing"
			}
			continue
		}

		evidence := claudeResultExecutionEvidence(claudeSDKMessage{ModelUsage: map[string]claudeResultModelUsage{model: native}})
		if evidence == nil || len(evidence.ModelUsage.Entries) != 1 || !evidence.ModelUsage.Entries[0].Usage.Complete {
			c.addError("USAGE_INVALID")
			c.modelUsage[model] = entry
			overall = "failed"
			continue
		}
		usage := claudeStreamMetadataUsage{
			UncachedInputTokens: *native.InputTokens, CacheReadInputTokens: *native.CacheReadInputTokens,
			CacheWriteInputTokens: *native.CacheCreationInputTokens, OutputTokens: *native.OutputTokens,
		}
		entry.UsageStatus, entry.Usage = "observed", &usage
		c.modelUsage[model] = entry
		if !claudeStreamMetadataAddUsage(&total, usage) {
			c.addError("USAGE_OVERFLOW")
			overall = "failed"
		}
	}
	c.usageStatus = overall
	if overall == "observed" {
		c.usage = &total
	}
}

func (c *claudeStreamMetadataCollector) observeSession(event map[string]json.RawMessage) {
	sessionID, present, valid := claudeStreamMetadataString(event, "session_id")
	if !present || !valid || sessionID == "" {
		c.sessionConflict = true
		c.addError("SESSION_MISMATCH")
		return
	}
	digest := claudeStreamMetadataDigest(sessionID)
	if c.sessionDigest == nil {
		c.sessionDigest = &digest
		return
	}
	if *c.sessionDigest != digest {
		c.sessionConflict = true
		c.addError("SESSION_MISMATCH")
	}
}

func (c *claudeStreamMetadataCollector) Finish() claudeStreamMetadataProjection {
	if !c.finished {
		c.finished = true
		if c.initCount == 0 {
			c.addError("INIT_MISSING")
			c.addError("CONFIGURATION_MISSING")
		}
		if c.mainAssistantCount == 0 || len(c.mainModels) == 0 {
			c.addError("MAIN_MODEL_MISSING")
		}
		if c.terminalCount == 0 {
			c.addError("TERMINAL_MISSING")
			c.addError("USAGE_MISSING")
		}
		if len(c.mainModels) == 1 {
			for model := range c.mainModels {
				matches := 0
				var matched claudeStreamMetadataModelUsage
				for usageModel, entry := range c.modelUsage {
					if claudeStreamMetadataMainUsageMatches(usageModel, model, c.configuredModel) {
						matches++
						matched = entry
					}
				}
				if matches != 1 || matched.UsageStatus != "observed" {
					c.addError("MAIN_MODEL_USAGE_MISSING")
				}
			}
		}
	}

	mainModels := make([]string, 0, len(c.mainModels))
	for model := range c.mainModels {
		mainModels = append(mainModels, model)
	}
	sort.Strings(mainModels)
	modelUsage := make([]claudeStreamMetadataModelUsage, 0, len(c.modelUsage))
	models := make([]string, 0, len(c.modelUsage))
	for model := range c.modelUsage {
		models = append(models, model)
	}
	sort.Strings(models)
	for _, model := range models {
		modelUsage = append(modelUsage, c.modelUsage[model])
	}

	var mainModel *string
	mainModelSource := "missing"
	if len(mainModels) == 1 {
		mainModel = stringPointer(mainModels[0])
		mainModelSource = "assistant_event"
	}
	var digest *string
	if !c.sessionConflict {
		digest = c.sessionDigest
	}
	errors := make([]claudeStreamMetadataError, 0, len(c.errors))
	for code, count := range c.errors {
		errors = append(errors, claudeStreamMetadataError{Code: code, Count: count})
	}
	sort.Slice(errors, func(i, j int) bool { return errors[i].Code < errors[j].Code })
	return claudeStreamMetadataProjection{
		Schema: claudeStreamMetadataSchema, Source: claudeStreamMetadataSource, Status: claudeStreamMetadataStatus(c.errors),
		ConfiguredModel: c.configuredModel, MainModel: mainModel, MainModelSource: mainModelSource,
		PermissionMode: c.permissionMode, MCPServerCount: c.mcpServerCount, SessionIDDigest: digest,
		InitCount: c.initCount, MainAssistantCount: c.mainAssistantCount, TerminalCount: c.terminalCount,
		MainModels: mainModels, TerminalSuccess: c.terminalSuccess, ModelUsage: modelUsage,
		UsageStatus: c.usageStatus, Usage: c.usage, Errors: errors,
	}
}

func claudeStreamMetadataMainUsageMatches(usageModel, mainModel string, configuredModel *string) bool {
	if usageModel == mainModel {
		return true
	}
	if configuredModel == nil {
		return false
	}
	configuredBase, configuredNormalized, ok := claudeStreamMetadataTerminalOneMinuteModel(*configuredModel)
	if !ok || configuredBase != mainModel {
		return false
	}
	_, usageNormalized, ok := claudeStreamMetadataTerminalOneMinuteModel(usageModel)
	return ok && usageNormalized == configuredNormalized
}

func claudeStreamMetadataTerminalOneMinuteModel(model string) (base, normalized string, ok bool) {
	const suffix = "[1m]"
	if len(model) < len(suffix) || !strings.EqualFold(model[len(model)-len(suffix):], suffix) {
		return "", "", false
	}
	base = model[:len(model)-len(suffix)]
	return base, base + suffix, true
}

func (c *claudeStreamMetadataCollector) increment(counter *int) {
	if *counter < claudeStreamMetadataMaxEvents {
		*counter = *counter + 1
	}
}

func (c *claudeStreamMetadataCollector) addError(code string) {
	if c.errors[code] < claudeStreamMetadataMaxEvents {
		c.errors[code]++
	}
}

func claudeStreamMetadataAddUsage(total *claudeStreamMetadataUsage, value claudeStreamMetadataUsage) bool {
	pairs := []struct {
		dst   *int64
		value int64
	}{
		{&total.UncachedInputTokens, value.UncachedInputTokens}, {&total.CacheReadInputTokens, value.CacheReadInputTokens},
		{&total.CacheWriteInputTokens, value.CacheWriteInputTokens}, {&total.OutputTokens, value.OutputTokens},
	}
	for _, pair := range pairs {
		if pair.value < 0 || *pair.dst > claudeStreamMetadataMaxSafeInteger-pair.value {
			return false
		}
		*pair.dst += pair.value
	}
	return true
}

func claudeStreamMetadataStatus(errors map[string]int) string {
	failed := map[string]struct{}{
		"MALFORMED_EVENT": {}, "MODEL_INVALID": {}, "SESSION_MISMATCH": {}, "EVENT_LIMIT": {}, "MODEL_LIMIT": {},
		"TERMINAL_FAILED": {}, "EVENT_AFTER_TERMINAL": {}, "EVENT_OUT_OF_ORDER": {}, "PARENT_INVALID": {},
		"USAGE_INVALID": {}, "USAGE_OVERFLOW": {},
	}
	conflicting := map[string]struct{}{"INIT_DUPLICATE": {}, "TERMINAL_DUPLICATE": {}, "MAIN_MODEL_CONFLICT": {}}
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
	if len(errors) > 0 {
		return "missing"
	}
	return "observed"
}

func claudeStreamMetadataString(event map[string]json.RawMessage, key string) (string, bool, bool) {
	raw, present := event[key]
	if !present {
		return "", false, true
	}
	var value string
	if json.Unmarshal(raw, &value) != nil {
		return "", true, false
	}
	return value, true, true
}

func claudeStreamMetadataPermissionMode(value string) bool {
	switch value {
	case "default", "acceptEdits", "bypassPermissions", "plan", "dontAsk", "delegate", "manual", "auto":
		return true
	default:
		return false
	}
}

func claudeStreamMetadataDigest(value string) string {
	return fmt.Sprintf("sha256:%x", sha256.Sum256([]byte(value)))
}

func stringPointer(value string) *string { return &value }

func logClaudeStreamMetadata(logger *slog.Logger, cfg Config, processID int, workDir string, projection claudeStreamMetadataProjection) {
	logger.Info("claude stream metadata observed",
		"task_id", cfg.TaskID,
		"runtime_id", cfg.RuntimeID,
		"process_id", processID,
		"attempt", 1,
		"work_dir", workDir,
		"stream_metadata", projection,
	)
}
