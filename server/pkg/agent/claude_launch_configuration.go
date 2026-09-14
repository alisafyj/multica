package agent

import (
	"bytes"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"io"
	"log/slog"
	"math"
	"os"
	"path/filepath"
	"runtime"
	"sort"
	"strconv"
	"strings"
	"unicode/utf8"
)

const (
	claudeLaunchConfigurationSchema = "claude_launch_configuration/v1"
	claudeLaunchArgumentLimit       = 256
	claudeLaunchArgumentBytes       = 64 * 1024
	claudeLaunchEnvironmentLimit    = 4096
	claudeLaunchEnvironmentBytes    = 1024 * 1024
	claudeLaunchConfigurationBytes  = 1024 * 1024
	claudeLaunchMCPServerLimit      = 128
)

var claudeLaunchModelPolicyKeys = []string{
	"ANTHROPIC_MODEL",
	"ANTHROPIC_DEFAULT_FABLE_MODEL",
	"ANTHROPIC_DEFAULT_OPUS_MODEL",
	"ANTHROPIC_DEFAULT_SONNET_MODEL",
	"ANTHROPIC_DEFAULT_HAIKU_MODEL",
	"CLAUDE_CODE_SUBAGENT_MODEL",
	"CLAUDE_CODE_SUBAGENT_MODEL_FORCE",
}

var claudeLaunchControlKeys = []string{
	"CLAUDE_CODE_DISABLE_NONESSENTIAL_TRAFFIC",
	"CLAUDE_CODE_DISABLE_AUTO_MEMORY",
	"CLAUDE_CODE_SUBPROCESS_ENV_SCRUB",
	"DISABLE_TELEMETRY",
	"DISABLE_ERROR_REPORTING",
	"DISABLE_AUTOUPDATER",
	"ENABLE_TOOL_SEARCH",
}

var claudeLaunchCredentialKeys = []string{
	"ANTHROPIC_API_KEY",
	"ANTHROPIC_AUTH_TOKEN",
	"CLAUDE_CODE_OAUTH_TOKEN",
}

type claudeLaunchConfiguration struct {
	Schema            string                               `json:"schema"`
	Source            string                               `json:"source"`
	Completeness      string                               `json:"completeness"`
	Status            string                               `json:"status"`
	Errors            []string                             `json:"errors"`
	StdinPromptSHA256 *string                              `json:"stdin_prompt_sha256"`
	Arguments         claudeLaunchConfigurationArguments   `json:"arguments"`
	Environment       claudeLaunchConfigurationEnvironment `json:"environment"`
	Settings          claudeLaunchConfigurationSource      `json:"settings"`
	MCP               claudeLaunchConfigurationMCP         `json:"mcp"`
}

type claudeLaunchConfigurationArguments struct {
	SHA256                *string  `json:"sha256"`
	UnknownOptionCount    int      `json:"unknown_option_count"`
	Model                 *string  `json:"model"`
	Effort                *string  `json:"effort"`
	PermissionMode        *string  `json:"permission_mode"`
	PermissionPromptTool  *string  `json:"permission_prompt_tool"`
	PermissionPrompts     *string  `json:"permission_prompts"`
	SettingSources        *string  `json:"setting_sources"`
	InputFormat           *string  `json:"input_format"`
	OutputFormat          *string  `json:"output_format"`
	Print                 bool     `json:"print"`
	Verbose               bool     `json:"verbose"`
	StrictMCPConfig       bool     `json:"strict_mcp_config"`
	NoSessionPersistence  bool     `json:"no_session_persistence"`
	NoChrome              bool     `json:"no_chrome"`
	MaxBudgetUSD          *float64 `json:"max_budget_usd"`
	MaxTurns              *int64   `json:"max_turns"`
	ResumePresent         bool     `json:"resume_present"`
	AllowedToolsSHA256    *string  `json:"allowed_tools_sha256"`
	DisallowedToolsSHA256 *string  `json:"disallowed_tools_sha256"`
	SystemPromptPresent   bool     `json:"system_prompt_present"`
}

type claudeLaunchConfigurationEnvironment struct {
	ModelPolicy           map[string]*string `json:"model_policy"`
	Controls              map[string]*string `json:"controls"`
	BaseURLSHA256         *string            `json:"base_url_sha256"`
	ConfigDirectorySHA256 *string            `json:"config_directory_sha256"`
	CredentialVariables   []string           `json:"credential_variables"`
}

type claudeLaunchConfigurationSource struct {
	Source string  `json:"source"`
	Status string  `json:"status"`
	SHA256 *string `json:"sha256"`
}

type claudeLaunchConfigurationMCP struct {
	Source      string  `json:"source"`
	Status      string  `json:"status"`
	SHA256      *string `json:"sha256"`
	ServerCount *int    `json:"server_count"`
}

type claudeLaunchArgumentProjection struct {
	arguments    claudeLaunchConfigurationArguments
	settingsSpec *string
	mcpSpec      *string
	errors       map[string]struct{}
}

func projectClaudeLaunchConfiguration(argv, environment []string, workDir, prompt string) claudeLaunchConfiguration {
	errors := make(map[string]struct{})
	argumentProjection := projectClaudeLaunchArguments(argv)
	mergeClaudeLaunchErrors(errors, argumentProjection.errors)
	environmentProjection, environmentErrors := projectClaudeLaunchEnvironment(environment, runtime.GOOS == "windows")
	mergeClaudeLaunchErrors(errors, environmentErrors)

	settings, _, settingsError := projectClaudeLaunchJSONSource(argumentProjection.settingsSpec, workDir, false)
	if settingsError {
		errors["SETTINGS_INVALID"] = struct{}{}
	}
	mcpSource, mcpCount, mcpError := projectClaudeLaunchJSONSource(argumentProjection.mcpSpec, workDir, true)
	if mcpError {
		errors["MCP_INVALID"] = struct{}{}
	}
	mcp := claudeLaunchConfigurationMCP{
		Source: mcpSource.Source, Status: mcpSource.Status, SHA256: mcpSource.SHA256, ServerCount: mcpCount,
	}

	var promptDigest *string
	if utf8.ValidString(prompt) {
		promptDigest = claudeLaunchStringDigest(prompt)
	} else {
		errors["PROMPT_INVALID"] = struct{}{}
	}

	errorList := make([]string, 0, len(errors))
	for code := range errors {
		errorList = append(errorList, code)
	}
	sort.Strings(errorList)
	status := "observed"
	if len(errorList) != 0 {
		status = "failed"
	}
	return claudeLaunchConfiguration{
		Schema: claudeLaunchConfigurationSchema, Source: "final_command_inputs", Completeness: "controlled_inputs_only",
		Status: status, Errors: errorList, StdinPromptSHA256: promptDigest,
		Arguments: argumentProjection.arguments, Environment: environmentProjection, Settings: settings, MCP: mcp,
	}
}

func projectClaudeLaunchArguments(argv []string) claudeLaunchArgumentProjection {
	result := claudeLaunchArgumentProjection{errors: make(map[string]struct{})}
	if len(argv) == 0 {
		result.errors["ARGUMENT_INVALID"] = struct{}{}
		return result
	}
	if len(argv) > claudeLaunchArgumentLimit {
		result.errors["ARGUMENT_LIMIT"] = struct{}{}
		return result
	}
	validArgv := true
	argumentBytes := 0
	for _, arg := range argv {
		if len(arg) > claudeLaunchArgumentBytes-argumentBytes {
			result.errors["ARGUMENT_LIMIT"] = struct{}{}
			return result
		}
		argumentBytes += len(arg)
		if !utf8.ValidString(arg) || strings.ContainsRune(arg, 0) {
			result.errors["ARGUMENT_INVALID"] = struct{}{}
			validArgv = false
		}
	}
	if validArgv {
		if digest, ok := claudeLaunchJSONDigest(argv); ok {
			result.arguments.SHA256 = &digest
		} else {
			result.errors["ARGUMENT_INVALID"] = struct{}{}
		}
	}

	seen := make(map[string]struct{})
	for index := 1; index < len(argv); index++ {
		raw := argv[index]
		flag, inlineValue, hasInlineValue := strings.Cut(raw, "=")
		canonical := claudeLaunchCanonicalFlag(flag)
		mode := claudeLaunchFlagMode(canonical)
		if mode == claudeLaunchUnknownFlag {
			result.arguments.UnknownOptionCount++
			continue
		}
		identity := canonical
		if identity == "--dangerously-skip-permissions" {
			identity = "--permission-mode"
		}
		if _, duplicate := seen[identity]; duplicate {
			result.errors["ARGUMENT_DUPLICATE"] = struct{}{}
		}
		seen[identity] = struct{}{}
		if mode == claudeLaunchBooleanFlag {
			if hasInlineValue {
				result.errors["ARGUMENT_INVALID"] = struct{}{}
				continue
			}
			result.setClaudeLaunchBoolean(canonical)
			continue
		}
		value := inlineValue
		if !hasInlineValue {
			if index+1 >= len(argv) {
				result.errors["ARGUMENT_INVALID"] = struct{}{}
				continue
			}
			index++
			value = argv[index]
		}
		result.setClaudeLaunchValue(canonical, value)
	}
	return result
}

type claudeLaunchFlagKind int

const (
	claudeLaunchUnknownFlag claudeLaunchFlagKind = iota
	claudeLaunchBooleanFlag
	claudeLaunchValueFlag
)

func claudeLaunchCanonicalFlag(flag string) string {
	switch flag {
	case "-p":
		return "--print"
	case "--allowedTools", "--allowed-tools":
		return "--allowed-tools"
	case "--disallowedTools", "--disallowed-tools":
		return "--disallowed-tools"
	case "--append-system-prompt", "--append-system-prompt-file", "--system-prompt", "--system-prompt-file":
		return "--system-prompt"
	default:
		return flag
	}
}

func claudeLaunchFlagMode(flag string) claudeLaunchFlagKind {
	switch flag {
	case "--print", "--verbose", "--strict-mcp-config", "--no-session-persistence", "--no-chrome", "--dangerously-skip-permissions":
		return claudeLaunchBooleanFlag
	case "--model", "--effort", "--permission-mode", "--permission-prompt-tool", "--permission-prompts", "--setting-sources", "--input-format", "--output-format",
		"--max-budget-usd", "--max-turns", "--resume", "--allowed-tools", "--disallowed-tools", "--system-prompt", "--settings", "--mcp-config":
		return claudeLaunchValueFlag
	default:
		return claudeLaunchUnknownFlag
	}
}

func (projection *claudeLaunchArgumentProjection) setClaudeLaunchBoolean(flag string) {
	switch flag {
	case "--print":
		projection.arguments.Print = true
	case "--verbose":
		projection.arguments.Verbose = true
	case "--strict-mcp-config":
		projection.arguments.StrictMCPConfig = true
	case "--no-session-persistence":
		projection.arguments.NoSessionPersistence = true
	case "--no-chrome":
		projection.arguments.NoChrome = true
	case "--dangerously-skip-permissions":
		value := "bypassPermissions"
		projection.arguments.PermissionMode = &value
	}
}

func (projection *claudeLaunchArgumentProjection) setClaudeLaunchValue(flag, value string) {
	invalid := func() { projection.errors["ARGUMENT_INVALID"] = struct{}{} }
	switch flag {
	case "--model":
		if !claudeLaunchSafeToken(value) {
			invalid()
			return
		}
		projection.arguments.Model = cloneClaudeLaunchString(value)
	case "--effort":
		if !claudeLaunchEnum(value, "off", "minimal", "low", "medium", "high", "xhigh", "max") {
			invalid()
			return
		}
		projection.arguments.Effort = cloneClaudeLaunchString(value)
	case "--permission-mode":
		if !claudeLaunchEnum(value, "default", "acceptEdits", "plan", "bypassPermissions", "dontAsk", "manual", "auto") {
			invalid()
			return
		}
		projection.arguments.PermissionMode = cloneClaudeLaunchString(value)
	case "--permission-prompt-tool":
		if !claudeLaunchSafeToken(value) {
			invalid()
			return
		}
		projection.arguments.PermissionPromptTool = cloneClaudeLaunchString(value)
	case "--permission-prompts":
		if !claudeLaunchEnum(value, "host", "none") {
			invalid()
			return
		}
		projection.arguments.PermissionPrompts = cloneClaudeLaunchString(value)
	case "--setting-sources":
		if !claudeLaunchSettingSources(value) {
			invalid()
			return
		}
		projection.arguments.SettingSources = cloneClaudeLaunchString(value)
	case "--input-format":
		if !claudeLaunchEnum(value, "text", "stream-json") {
			invalid()
			return
		}
		projection.arguments.InputFormat = cloneClaudeLaunchString(value)
	case "--output-format":
		if !claudeLaunchEnum(value, "text", "json", "stream-json") {
			invalid()
			return
		}
		projection.arguments.OutputFormat = cloneClaudeLaunchString(value)
	case "--max-budget-usd":
		budget, err := strconv.ParseFloat(value, 64)
		if err != nil || budget <= 0 || math.IsNaN(budget) || math.IsInf(budget, 0) {
			invalid()
			return
		}
		projection.arguments.MaxBudgetUSD = &budget
	case "--max-turns":
		turns, err := strconv.ParseInt(value, 10, 64)
		if err != nil || turns <= 0 || turns > 9007199254740991 {
			invalid()
			return
		}
		projection.arguments.MaxTurns = &turns
	case "--resume":
		if !claudeLaunchBoundedValue(value) {
			invalid()
			return
		}
		projection.arguments.ResumePresent = true
	case "--allowed-tools":
		if !claudeLaunchBoundedValue(value) {
			invalid()
			return
		}
		projection.arguments.AllowedToolsSHA256 = claudeLaunchStringDigest(value)
	case "--disallowed-tools":
		if !claudeLaunchBoundedValue(value) {
			invalid()
			return
		}
		projection.arguments.DisallowedToolsSHA256 = claudeLaunchStringDigest(value)
	case "--system-prompt":
		if !claudeLaunchBoundedValue(value) {
			invalid()
			return
		}
		projection.arguments.SystemPromptPresent = true
	case "--settings":
		if !claudeLaunchBoundedValue(value) {
			invalid()
			return
		}
		projection.settingsSpec = cloneClaudeLaunchString(value)
	case "--mcp-config":
		if !claudeLaunchBoundedValue(value) {
			invalid()
			return
		}
		projection.mcpSpec = cloneClaudeLaunchString(value)
	}
}

func projectClaudeLaunchEnvironment(environment []string, caseInsensitive bool) (claudeLaunchConfigurationEnvironment, map[string]struct{}) {
	errors := make(map[string]struct{})
	values := make(map[string]string)
	// Match os/exec deduplication: Windows keys are case-insensitive, last wins.
	normalizeKey := func(key string) string {
		if caseInsensitive {
			return strings.ToLower(key)
		}
		return key
	}
	environmentBytes := 0
	valid := true
	if len(environment) > claudeLaunchEnvironmentLimit {
		errors["ENVIRONMENT_LIMIT"] = struct{}{}
		environment = nil
	}
	for _, entry := range environment {
		if len(entry) > claudeLaunchEnvironmentBytes-environmentBytes {
			errors["ENVIRONMENT_LIMIT"] = struct{}{}
			clear(values)
			break
		}
		environmentBytes += len(entry)
		if !utf8.ValidString(entry) || strings.ContainsRune(entry, 0) {
			valid = false
			continue
		}
		key, value, ok := strings.Cut(entry, "=")
		if !ok || key == "" {
			valid = false
			continue
		}
		values[normalizeKey(key)] = value
	}
	if !valid {
		errors["ENVIRONMENT_INVALID"] = struct{}{}
	}

	result := claudeLaunchConfigurationEnvironment{
		ModelPolicy:         make(map[string]*string, len(claudeLaunchModelPolicyKeys)),
		Controls:            make(map[string]*string, len(claudeLaunchControlKeys)),
		CredentialVariables: make([]string, 0, len(claudeLaunchCredentialKeys)),
	}
	for _, key := range claudeLaunchModelPolicyKeys {
		value, present := values[normalizeKey(key)]
		if !present {
			result.ModelPolicy[key] = nil
			continue
		}
		if key == "CLAUDE_CODE_SUBAGENT_MODEL_FORCE" {
			if value != "0" && value != "1" {
				errors["ENVIRONMENT_INVALID"] = struct{}{}
				result.ModelPolicy[key] = nil
				continue
			}
		} else if !claudeLaunchSafeToken(value) {
			errors["ENVIRONMENT_INVALID"] = struct{}{}
			result.ModelPolicy[key] = nil
			continue
		}
		result.ModelPolicy[key] = cloneClaudeLaunchString(value)
	}
	for _, key := range claudeLaunchControlKeys {
		value, present := values[normalizeKey(key)]
		if !present {
			result.Controls[key] = nil
			continue
		}
		if !claudeLaunchEnum(value, "0", "1", "true", "false") {
			errors["ENVIRONMENT_INVALID"] = struct{}{}
			result.Controls[key] = nil
			continue
		}
		result.Controls[key] = cloneClaudeLaunchString(value)
	}
	if value, present := values[normalizeKey("ANTHROPIC_BASE_URL")]; present {
		result.BaseURLSHA256 = claudeLaunchStringDigest(value)
	}
	if value, present := values[normalizeKey("CLAUDE_CONFIG_DIR")]; present {
		result.ConfigDirectorySHA256 = claudeLaunchStringDigest(value)
	}
	for _, key := range claudeLaunchCredentialKeys {
		if values[normalizeKey(key)] != "" {
			result.CredentialVariables = append(result.CredentialVariables, key)
		}
	}
	sort.Strings(result.CredentialVariables)
	return result, errors
}

func projectClaudeLaunchJSONSource(spec *string, workDir string, mcp bool) (claudeLaunchConfigurationSource, *int, bool) {
	if spec == nil {
		return claudeLaunchConfigurationSource{Source: "absent", Status: "missing"}, nil, false
	}
	source := "file"
	var raw []byte
	var readStatus string
	trimmed := strings.TrimSpace(*spec)
	if strings.HasPrefix(trimmed, "{") || strings.HasPrefix(trimmed, "[") {
		source = "inline"
		raw = []byte(*spec)
	} else {
		path := *spec
		if !filepath.IsAbs(path) {
			path = filepath.Join(workDir, path)
		}
		var ok bool
		raw, readStatus, ok = readClaudeLaunchConfigurationFile(path)
		if !ok {
			return claudeLaunchConfigurationSource{Source: source, Status: readStatus}, nil, true
		}
	}
	digest := claudeLaunchBytesDigest(raw)
	result := claudeLaunchConfigurationSource{Source: source, Status: "failed", SHA256: &digest}
	if len(raw) > claudeLaunchConfigurationBytes || !utf8.Valid(raw) {
		return result, nil, true
	}
	var object map[string]json.RawMessage
	if err := json.Unmarshal(raw, &object); err != nil || object == nil {
		return result, nil, true
	}
	if !mcp {
		result.Status = "observed"
		return result, nil, false
	}
	serversRaw, present := object["mcpServers"]
	if !present {
		return result, nil, true
	}
	var servers map[string]json.RawMessage
	if err := json.Unmarshal(serversRaw, &servers); err != nil || servers == nil || len(servers) > claudeLaunchMCPServerLimit {
		return result, nil, true
	}
	count := len(servers)
	result.Status = "observed"
	return result, &count, false
}

func readClaudeLaunchConfigurationFile(path string) ([]byte, string, bool) {
	info, err := os.Lstat(path)
	if err != nil {
		if os.IsNotExist(err) {
			return nil, "missing", false
		}
		return nil, "failed", false
	}
	if !info.Mode().IsRegular() || info.Size() > claudeLaunchConfigurationBytes {
		return nil, "failed", false
	}
	file, err := os.Open(path)
	if err != nil {
		return nil, "failed", false
	}
	defer file.Close()
	openedInfo, err := file.Stat()
	if err != nil || !openedInfo.Mode().IsRegular() || !os.SameFile(info, openedInfo) {
		return nil, "failed", false
	}
	raw, err := io.ReadAll(io.LimitReader(file, claudeLaunchConfigurationBytes+1))
	if err != nil || len(raw) > claudeLaunchConfigurationBytes {
		return nil, "failed", false
	}
	return raw, "observed", true
}

func logClaudeLaunchConfiguration(logger *slog.Logger, cfg Config, processID int, workDir string, configuration claudeLaunchConfiguration) {
	logger.Info("claude launch configuration observed",
		"task_id", cfg.TaskID,
		"runtime_id", cfg.RuntimeID,
		"process_id", processID,
		"attempt", 1,
		"work_dir", workDir,
		"configuration", configuration,
	)
}

func claudeLaunchJSONDigest(value any) (string, bool) {
	var buffer bytes.Buffer
	encoder := json.NewEncoder(&buffer)
	encoder.SetEscapeHTML(false)
	if err := encoder.Encode(value); err != nil {
		return "", false
	}
	raw := bytes.TrimSuffix(buffer.Bytes(), []byte{'\n'})
	return claudeLaunchBytesDigest(raw), true
}

func claudeLaunchStringDigest(value string) *string {
	digest := claudeLaunchBytesDigest([]byte(value))
	return &digest
}

func claudeLaunchBytesDigest(value []byte) string {
	digest := sha256.Sum256(value)
	return "sha256:" + hex.EncodeToString(digest[:])
}

func claudeLaunchSafeToken(value string) bool {
	if len(value) == 0 || len(value) > 255 || !utf8.ValidString(value) {
		return false
	}
	for index, char := range value {
		if (char >= 'A' && char <= 'Z') || (char >= 'a' && char <= 'z') || (char >= '0' && char <= '9') ||
			(index > 0 && strings.ContainsRune("._/[]@:+-", char)) {
			continue
		}
		return false
	}
	return true
}

func claudeLaunchBoundedValue(value string) bool {
	return value != "" && len(value) <= claudeLaunchConfigurationBytes && utf8.ValidString(value)
}

func claudeLaunchEnum(value string, allowed ...string) bool {
	for _, candidate := range allowed {
		if value == candidate {
			return true
		}
	}
	return false
}

func claudeLaunchSettingSources(value string) bool {
	parts := strings.Split(value, ",")
	if len(parts) == 0 || len(parts) > 3 {
		return false
	}
	seen := make(map[string]struct{}, len(parts))
	for _, part := range parts {
		if !claudeLaunchEnum(part, "user", "project", "local") {
			return false
		}
		if _, duplicate := seen[part]; duplicate {
			return false
		}
		seen[part] = struct{}{}
	}
	return true
}

func cloneClaudeLaunchString(value string) *string {
	cloned := value
	return &cloned
}

func mergeClaudeLaunchErrors(target, source map[string]struct{}) {
	for code := range source {
		target[code] = struct{}{}
	}
}
