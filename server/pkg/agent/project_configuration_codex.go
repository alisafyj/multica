package agent

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"reflect"
	"sort"
	"strings"

	"github.com/pelletier/go-toml/v2"
)

const maxCodexProjectPolicyConfigBytes = 1 << 20

var codexProtectedProjectConfigPaths = [][]string{
	{"approval_policy"},
	{"sandbox_mode"},
	{"sandbox_workspace_write"},
	{"sandbox_permissions"},
	{"default_permissions"},
	{"permissions"},
	{"shell_environment_policy"},
	{"windows", "sandbox"},
	{"features", "multi_agent"},
	{"features", "memories"},
	{"memories"},
}

var codexLayerOnlyProtectedProjectConfigPaths = [][]string{
	{"sandbox_permissions"},
}

type codexConfigReadResult struct {
	Config map[string]any     `json:"config"`
	Layers []codexConfigLayer `json:"layers"`
}

type codexConfigurationReadPurpose string

const (
	codexConfigurationReadPurposeNeutral       codexConfigurationReadPurpose = "neutral"
	codexConfigurationReadPurposeTaskEffective codexConfigurationReadPurpose = "task_effective"
)

type codexConfigLayer struct {
	Name struct {
		Type           string `json:"type"`
		DotCodexFolder string `json:"dotCodexFolder"`
	} `json:"name"`
	DisabledReason string         `json:"disabledReason"`
	Config         map[string]any `json:"config"`
}

func ensureCodexProjectConfiguration(codexHome, cwd, childHome, policy string) (string, error) {
	if policy == "" {
		return cwd, nil
	}
	if err := validateProjectConfigurationPolicy(policy); err != nil {
		return "", err
	}

	canonicalHome, err := canonicalExistingDirectory(codexHome, "CODEX_HOME")
	if err != nil {
		return "", err
	}
	if err := rejectGlobalCodexHome(canonicalHome, childHome); err != nil {
		return "", err
	}
	canonicalCwd, err := canonicalExistingDirectory(cwd, "working directory")
	if err != nil {
		return "", err
	}

	configPath := filepath.Join(canonicalHome, "config.toml")
	data, err := readBoundedRegularCodexConfig(configPath)
	if err != nil {
		return "", err
	}
	config := make(map[string]any)
	if len(data) > 0 {
		if err := toml.Unmarshal(data, &config); err != nil {
			return "", fmt.Errorf("parse task-private Codex config.toml: %w", err)
		}
	}
	projects, err := codexStringMap(config, "projects")
	if err != nil {
		return "", err
	}
	project, err := codexStringMap(projects, canonicalCwd)
	if err != nil {
		return "", err
	}
	if policy == "trusted" {
		project["trust_level"] = "trusted"
	} else {
		project["trust_level"] = "untrusted"
	}
	projects[canonicalCwd] = project
	config["projects"] = projects

	updated, err := toml.Marshal(config)
	if err != nil {
		return "", fmt.Errorf("encode task-private Codex config.toml: %w", err)
	}
	if len(updated) > maxCodexProjectPolicyConfigBytes {
		return "", fmt.Errorf("task-private Codex config.toml exceeds %d bytes after project policy", maxCodexProjectPolicyConfigBytes)
	}
	if err := writePrivateCodexConfig(configPath, updated); err != nil {
		return "", fmt.Errorf("write task-private Codex config.toml: %w", err)
	}
	return canonicalCwd, nil
}

func writePrivateCodexConfig(path string, data []byte) error {
	temp, err := os.CreateTemp(filepath.Dir(path), ".multica-codex-config-*.tmp")
	if err != nil {
		return fmt.Errorf("create temporary config: %w", err)
	}
	tempPath := temp.Name()
	defer func() {
		_ = temp.Close()
		if tempPath != "" {
			_ = os.Remove(tempPath)
		}
	}()
	if err := temp.Chmod(0o600); err != nil {
		return fmt.Errorf("chmod temporary config to 0600: %w", err)
	}
	if _, err := temp.Write(data); err != nil {
		return fmt.Errorf("write temporary config: %w", err)
	}
	if err := temp.Sync(); err != nil {
		return fmt.Errorf("sync temporary config: %w", err)
	}
	if err := temp.Close(); err != nil {
		return fmt.Errorf("close temporary config: %w", err)
	}
	if err := os.Rename(tempPath, path); err != nil {
		return fmt.Errorf("replace config atomically: %w", err)
	}
	tempPath = ""
	return nil
}

func canonicalExistingDirectory(path, label string) (string, error) {
	if strings.TrimSpace(path) == "" {
		return "", fmt.Errorf("%s is required for explicit project configuration policy", label)
	}
	info, err := os.Lstat(path)
	if err != nil {
		return "", fmt.Errorf("inspect %s: %w", label, err)
	}
	if info.Mode()&os.ModeSymlink != 0 || !info.IsDir() {
		return "", fmt.Errorf("%s must be a real directory, not a symlink or non-directory", label)
	}
	canonical, err := filepath.EvalSymlinks(path)
	if err != nil {
		return "", fmt.Errorf("canonicalize %s: %w", label, err)
	}
	canonical, err = filepath.Abs(canonical)
	if err != nil {
		return "", fmt.Errorf("make %s absolute: %w", label, err)
	}
	return filepath.Clean(canonical), nil
}

func rejectGlobalCodexHome(codexHome, childHome string) error {
	homes := []string{strings.TrimSpace(childHome)}
	if userHome, err := os.UserHomeDir(); err == nil {
		homes = append(homes, userHome)
	}
	for _, home := range homes {
		if home == "" {
			continue
		}
		candidate, err := filepath.Abs(filepath.Join(home, ".codex"))
		if err != nil {
			continue
		}
		if resolved, resolveErr := filepath.EvalSymlinks(candidate); resolveErr == nil {
			candidate = resolved
		}
		if filepath.Clean(candidate) == codexHome {
			return fmt.Errorf("explicit project configuration policy requires a task-private CODEX_HOME; refusing to modify the user-global Codex home")
		}
	}
	return nil
}

func readBoundedRegularCodexConfig(path string) ([]byte, error) {
	info, err := os.Lstat(path)
	if os.IsNotExist(err) {
		return nil, nil
	}
	if err != nil {
		return nil, fmt.Errorf("inspect task-private Codex config.toml: %w", err)
	}
	if info.Mode()&os.ModeSymlink != 0 || !info.Mode().IsRegular() {
		return nil, fmt.Errorf("task-private Codex config.toml must be a regular file, not a symlink or special file")
	}
	if info.Size() > maxCodexProjectPolicyConfigBytes {
		return nil, fmt.Errorf("task-private Codex config.toml exceeds %d bytes", maxCodexProjectPolicyConfigBytes)
	}
	f, err := os.Open(path)
	if err != nil {
		return nil, fmt.Errorf("open task-private Codex config.toml: %w", err)
	}
	defer f.Close()
	data, err := io.ReadAll(io.LimitReader(f, maxCodexProjectPolicyConfigBytes+1))
	if err != nil {
		return nil, fmt.Errorf("read task-private Codex config.toml: %w", err)
	}
	if len(data) > maxCodexProjectPolicyConfigBytes {
		return nil, fmt.Errorf("task-private Codex config.toml exceeds %d bytes", maxCodexProjectPolicyConfigBytes)
	}
	return data, nil
}

func codexStringMap(parent map[string]any, key string) (map[string]any, error) {
	value, ok := parent[key]
	if !ok {
		return make(map[string]any), nil
	}
	child, ok := value.(map[string]any)
	if !ok {
		return nil, fmt.Errorf("task-private Codex config.toml key %q must be a table", key)
	}
	return child, nil
}

func verifyCodexProjectConfiguration(ctx context.Context, client *codexClient, opts ExecOptions) error {
	neutralCwd, err := os.MkdirTemp("", "multica-codex-config-preflight-")
	if err != nil {
		return fmt.Errorf("create neutral config preflight directory: %w", err)
	}
	defer os.RemoveAll(neutralCwd)

	baseline, err := readCodexConfiguration(ctx, client, neutralCwd, codexConfigurationReadPurposeNeutral)
	if err != nil {
		return fmt.Errorf("read task-private baseline configuration: %w", err)
	}
	effective, err := readCodexConfiguration(ctx, client, opts.Cwd, codexConfigurationReadPurposeTaskEffective)
	if err != nil {
		return fmt.Errorf("read effective project configuration: %w", err)
	}
	if err := verifyCodexProjectLayer(opts.Cwd, opts.ProjectConfigurationPolicy, effective.Layers); err != nil {
		return err
	}
	for _, path := range codexProtectedProjectConfigPaths {
		baselineValue, baselineOK := nestedCodexConfigValue(baseline.Config, path)
		effectiveValue, effectiveOK := nestedCodexConfigValue(effective.Config, path)
		if baselineOK != effectiveOK || !reflect.DeepEqual(baselineValue, effectiveValue) {
			return fmt.Errorf("project configuration changes protected Codex setting %s", strings.Join(path, "."))
		}
	}
	if hasManagedCodexMcpConfig(opts.McpConfig) {
		expected, err := normalizedManagedCodexMcpServers(opts.McpConfig)
		if err != nil {
			return err
		}
		actual, err := effectiveCodexMcpServers(effective.Config)
		if err != nil {
			return err
		}
		if !reflect.DeepEqual(expected, actual) {
			return codexMcpConfigurationMismatch(expected, actual)
		}
	}
	return nil
}

func readCodexConfiguration(ctx context.Context, client *codexClient, cwd string, purpose codexConfigurationReadPurpose) (codexConfigReadResult, error) {
	raw, err := client.request(ctx, "config/read", map[string]any{"cwd": cwd, "includeLayers": true})
	if err != nil {
		return codexConfigReadResult{}, fmt.Errorf("config/read unavailable or rejected: %s", sanitizeCodexDiagnostic(err.Error()))
	}
	var result codexConfigReadResult
	if err := json.Unmarshal(raw, &result); err != nil {
		return codexConfigReadResult{}, fmt.Errorf("config/read returned an unsupported response: %w", err)
	}
	if result.Config == nil {
		return codexConfigReadResult{}, fmt.Errorf("config/read returned an unsupported response without config")
	}
	observeCodexConfiguration(client, cwd, purpose, result.Config)
	return result, nil
}

func verifyCodexProjectLayer(cwd, policy string, layers []codexConfigLayer) error {
	dotCodexPath := filepath.Join(cwd, ".codex")
	if info, err := os.Stat(dotCodexPath); os.IsNotExist(err) {
		return nil
	} else if err != nil || !info.IsDir() {
		return fmt.Errorf("inspect project .codex directory for configuration preflight")
	}
	if resolved, err := filepath.EvalSymlinks(dotCodexPath); err == nil {
		dotCodexPath = resolved
	}
	found := false
	for _, layer := range layers {
		if layer.Name.Type != "project" {
			continue
		}
		folder, err := filepath.Abs(layer.Name.DotCodexFolder)
		if err != nil {
			continue
		}
		if resolved, resolveErr := filepath.EvalSymlinks(folder); resolveErr == nil {
			folder = resolved
		}
		if filepath.Clean(folder) != dotCodexPath {
			continue
		}
		found = true
		if policy == "trusted" && layer.DisabledReason != "" {
			return fmt.Errorf("trusted Codex project configuration remained disabled")
		}
		if policy == "restricted" && layer.DisabledReason == "" {
			return fmt.Errorf("restricted Codex project configuration was unexpectedly enabled")
		}
		if policy == "trusted" {
			for _, path := range codexLayerOnlyProtectedProjectConfigPaths {
				if _, ok := nestedCodexConfigValue(layer.Config, path); ok {
					return fmt.Errorf("project configuration directly sets protected Codex setting %s", strings.Join(path, "."))
				}
			}
		}
	}
	if !found {
		return fmt.Errorf("Codex project configuration layer was not reported by config/read for %s policy", policy)
	}
	return nil
}

func nestedCodexConfigValue(config map[string]any, path []string) (any, bool) {
	var current any = config
	for _, key := range path {
		m, ok := current.(map[string]any)
		if !ok {
			return nil, false
		}
		current, ok = m[key]
		if !ok {
			return nil, false
		}
	}
	return current, true
}

func normalizedManagedCodexMcpServers(raw json.RawMessage) (map[string]any, error) {
	var parsed struct {
		McpServers map[string]map[string]any `json:"mcpServers"`
	}
	if err := json.Unmarshal(raw, &parsed); err != nil {
		return nil, fmt.Errorf("parse managed Codex MCP configuration: %w", err)
	}
	normalized := make(map[string]any, len(parsed.McpServers))
	for name, server := range parsed.McpServers {
		if server == nil {
			return nil, fmt.Errorf("managed Codex MCP server %q must be an object", name)
		}
		normalizedServer := normalizeCodexMcpServerConfig(server)
		if codexMcpServerExplicitlyDisabled(normalizedServer) {
			continue
		}
		normalized[name] = normalizeCodexMcpServerForConfigRead(normalizedServer)
	}
	return normalized, nil
}

func effectiveCodexMcpServers(config map[string]any) (map[string]any, error) {
	value, ok := config["mcp_servers"]
	if !ok || value == nil {
		return map[string]any{}, nil
	}
	servers, ok := value.(map[string]any)
	if !ok {
		return nil, fmt.Errorf("config/read returned an unsupported mcp_servers value")
	}
	normalized := make(map[string]any, len(servers))
	for name, value := range servers {
		server, ok := value.(map[string]any)
		if !ok {
			return nil, fmt.Errorf("config/read returned an unsupported MCP server entry")
		}
		if codexMcpServerExplicitlyDisabled(server) {
			continue
		}
		normalized[name] = normalizeCodexMcpServerForConfigRead(server)
	}
	return normalized, nil
}

func codexMcpServerExplicitlyDisabled(server map[string]any) bool {
	enabled, ok := server["enabled"].(bool)
	return ok && !enabled
}

func codexMcpConfigurationMismatch(expected, actual map[string]any) error {
	missing := make([]string, 0)
	unexpected := make([]string, 0)
	changed := make([]string, 0)
	for name, expectedServer := range expected {
		actualServer, ok := actual[name]
		if !ok {
			missing = append(missing, name)
			continue
		}
		if !reflect.DeepEqual(expectedServer, actualServer) {
			changed = append(changed, name)
		}
	}
	for name := range actual {
		if _, ok := expected[name]; !ok {
			unexpected = append(unexpected, name)
		}
	}
	return fmt.Errorf(
		"effective Codex MCP configuration mismatch: missing=%s unexpected_enabled=%s changed=%s",
		codexMcpServerNamesSummary(missing),
		codexMcpServerNamesSummary(unexpected),
		codexMcpServerNamesSummary(changed),
	)
}

func codexMcpServerNamesSummary(names []string) string {
	if len(names) == 0 {
		return "[]"
	}
	sort.Strings(names)
	const maxNames = 8
	shown := names
	if len(shown) > maxNames {
		shown = shown[:maxNames]
	}
	quoted := make([]string, 0, len(shown))
	for _, name := range shown {
		name = sanitizeCodexDiagnostic(name)
		if len(name) > 64 {
			name = name[:64] + "..."
		}
		quoted = append(quoted, fmt.Sprintf("%q", name))
	}
	summary := "[" + strings.Join(quoted, ", ") + "]"
	if omitted := len(names) - len(shown); omitted > 0 {
		summary += fmt.Sprintf("(+%d more)", omitted)
	}
	return summary
}

func normalizeCodexMcpServerForConfigRead(server map[string]any) map[string]any {
	normalized := make(map[string]any, len(server)+4)
	for key, value := range server {
		if key != "experimental_use_rmcp_client" {
			normalized[key] = value
		}
	}
	if _, ok := normalized["enabled"]; !ok {
		normalized["enabled"] = true
	}
	if _, ok := normalized["environment_id"]; !ok {
		normalized["environment_id"] = "local"
	}
	if _, ok := normalized["tool_timeout_sec"]; !ok {
		normalized["tool_timeout_sec"] = nil
	}
	if _, commandServer := normalized["command"]; commandServer {
		if _, ok := normalized["args"]; !ok {
			normalized["args"] = []any{}
		}
	}
	return normalized
}
