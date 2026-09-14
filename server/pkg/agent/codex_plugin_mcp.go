package agent

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"path/filepath"
	"reflect"
	"sort"
	"strings"
	"time"
	"unicode"
	"unicode/utf8"

	"github.com/multica-ai/multica/server/internal/agentconfig"
	"github.com/pelletier/go-toml/v2"
)

const codexPluginMCPPreflightTimeout = 10 * time.Second

// The names RPCs do not expose commands for the privacy gate. This plan only
// disables plugin MCP, never selects it or reconstructs plugin configuration.
type codexPluginMCPPlan struct {
	inventory       map[string][]string
	home            string
	cache           map[string]codexPluginCacheBinding
	ready           bool
	pluginsDisabled bool
	restore         func() error
}

func codexPluginMCPError(reason string) error {
	return fmt.Errorf("%w: Codex plugin MCP isolation %s", agentconfig.ErrRuntimeMCPSelection, reason)
}

func (b *codexBackend) preparePluginMCP(ctx context.Context, opts ExecOptions) (ExecOptions, error) {
	selection := opts.RuntimeMCPSelection
	if selection == nil || selection.Mode == "inherit" {
		return opts, nil
	}
	// Validate direct SDK callers too, without expanding the public policy API.
	policy := map[string]any{"mode": selection.Mode}
	if selection.Mode == "allowlist" || selection.Allow != nil {
		policy["allow"] = selection.Allow
	}
	raw, _ := json.Marshal(map[string]any{"_multica": map[string]any{"runtimeMcp": policy}})
	validated, err := agentconfig.ParseRuntimeMCPSelection(raw)
	if err != nil || validated.ValidateProvider("codex") != nil {
		return opts, codexPluginMCPError("requires a valid selection")
	}
	// Only the built-in binary's paired version is trustworthy here. A custom
	// profile can wrap a different implementation despite its protocol family.
	version := strings.TrimPrefix(strings.TrimSpace(b.cfg.CodexVersion), "codex-cli ")
	if !b.cfg.BuiltinRuntime || version != "0.153.4" {
		return opts, codexPluginMCPError("requires the verified built-in Codex 0.153.4 contract")
	}
	for _, args := range [][]string{opts.CustomArgs, opts.ExtraArgs, b.cfg.commandAt(b.cfg.ExecutablePath).Prefix} {
		if !codexPluginMCPArgsSafe(args) {
			return opts, codexPluginMCPError("rejects conflicting launch configuration")
		}
	}
	home, err := canonicalExistingDirectory(b.cfg.Env["CODEX_HOME"], "CODEX_HOME")
	if err != nil || rejectGlobalCodexHome(home, b.cfg.Env["HOME"]) != nil {
		return opts, codexPluginMCPError("requires a task-private Codex home")
	}
	if _, err := readBoundedRegularCodexConfig(filepath.Join(home, "config.toml")); err != nil {
		return opts, codexPluginMCPError("could not read task-private configuration")
	}
	if opts.Cwd, err = canonicalExistingDirectory(opts.Cwd, "working directory"); err != nil {
		return opts, codexPluginMCPError("requires a real working directory")
	}
	if !hasManagedCodexMcpConfig(opts.McpConfig) {
		return opts, codexPluginMCPError("requires the resolved ordinary MCP overlay")
	}
	opts.RuntimeMCPSelection = &validated
	opts.codexPluginMCP = &codexPluginMCPPlan{home: home}
	probeOpts := opts
	probeOpts.RequestUserInput = nil
	probeOpts.ValidateResolvedModelSelection = nil
	probeCtx, cancel := context.WithTimeout(ctx, codexPluginMCPBudget(opts))
	defer cancel()
	started := time.Now()
	session, err := b.executeOnce(probeCtx, "", probeOpts, 0)
	if err != nil {
		return opts, codexPluginMCPError("preparation failed")
	}
	for range session.Messages {
	}
	result, ok := <-session.Result
	if !ok || result.Status != "completed" || probeCtx.Err() != nil {
		return opts, codexPluginMCPError("inventory or process cleanup failed")
	}
	if opts.Timeout > 0 {
		opts.Timeout -= time.Since(started)
		if opts.Timeout <= 0 {
			return opts, codexPluginMCPError("preparation deadline exceeded")
		}
	}
	if opts.codexPluginMCP.pluginsDisabled {
		// No plugin subtree is changed, but task cleanup always invokes restore.
		opts.codexPluginMCP.restore = func() error { return nil }
		opts.codexPluginMCP.ready = true
		return opts, nil
	}
	restore, err := codexPluginMCPRestorer(home)
	if err != nil {
		return opts, err
	}
	if err := writeCodexPluginMCPPolicy(home, opts.codexPluginMCP.inventory); err != nil {
		return opts, err
	}
	opts.codexPluginMCP.restore = restore
	opts.codexPluginMCP.ready = true
	return opts, nil
}

// Capture only plugin settings in memory. Restoring after process cleanup keeps
// a later inherit execution from treating our private false values as user intent.
func codexPluginMCPRestorer(home string) (func() error, error) {
	failure := codexPluginMCPError("could not restore task-private plugin settings")
	path := filepath.Join(home, "config.toml")
	raw, err := readBoundedRegularCodexConfig(path)
	if err != nil {
		return nil, failure
	}
	var original map[string]any
	if toml.Unmarshal(raw, &original) != nil {
		return nil, failure
	}
	plugins, present := original["plugins"]
	return func() error {
		raw, err := readBoundedRegularCodexConfig(path)
		if err != nil {
			return failure
		}
		var config map[string]any
		if toml.Unmarshal(raw, &config) != nil || config == nil {
			return failure
		}
		if present {
			config["plugins"] = plugins
		} else {
			delete(config, "plugins")
		}
		encoded, err := toml.Marshal(config)
		if err != nil || len(encoded) > maxCodexProjectPolicyConfigBytes {
			return failure
		}
		if writePrivateCodexConfig(path, encoded) != nil {
			return failure
		}
		return nil
	}, nil
}

func codexPluginMCPBudget(opts ExecOptions) time.Duration {
	budget := codexPluginMCPPreflightTimeout
	for _, duration := range []time.Duration{opts.HandshakeTimeout, opts.Timeout} {
		if duration > 0 && duration < budget {
			budget = duration
		}
	}
	return budget
}

func codexPluginMCPArgsSafe(args []string) bool {
	for i := 0; i < len(args); i++ {
		arg := args[i]
		if arg == "--" || arg == "--profile" || strings.HasPrefix(arg, "--profile=") || strings.HasPrefix(arg, "-p") {
			return false
		}
		var assignment string
		switch {
		case arg == "-c" || arg == "--config":
			i++
			if i == len(args) {
				return false
			}
			assignment = args[i]
		case strings.HasPrefix(arg, "--config="):
			assignment = strings.TrimPrefix(arg, "--config=")
		case strings.HasPrefix(arg, "-c"):
			assignment = strings.TrimPrefix(arg, "-c")
		default:
			continue
		}
		// Codex allows bare string values. Parse only the key as TOML so quoted
		// keys and dotted/profile forms cannot bypass the protected roots.
		key, _, found := strings.Cut(assignment, "=")
		var parsed map[string]any
		if !found || toml.Unmarshal([]byte(strings.TrimSpace(key)+" = false"), &parsed) != nil || len(parsed) != 1 {
			return false
		}
		for root := range parsed {
			switch root {
			case "plugins", "profiles", "mcp_servers":
				return false
			}
		}
	}
	return true
}

type codexPluginRequester interface {
	request(context.Context, string, any) (json.RawMessage, error)
}

func discoverCodexPluginMCP(ctx context.Context, client codexPluginRequester, cwd string) (map[string][]string, error) {
	failure := codexPluginMCPError("inventory is unavailable or incomplete")
	raw, err := client.request(ctx, "plugin/installed", map[string]any{"cwds": []string{cwd}})
	if err != nil {
		return nil, failure
	}
	doc, err := decodeCodexPluginObject(raw)
	if err != nil {
		return nil, failure
	}
	marketplaces, ok := doc["marketplaces"].([]any)
	if !ok || len(marketplaces) > 64 {
		return nil, failure
	}
	if value, present := doc["marketplaceLoadErrors"]; present {
		failures, ok := value.([]any)
		if !ok || len(failures) != 0 {
			return nil, failure
		}
	}
	inventory := make(map[string][]string)
	seen := make(map[string]bool)
	pluginCount, serverCount := 0, 0
	for _, value := range marketplaces {
		market, ok := value.(map[string]any)
		if !ok {
			return nil, failure
		}
		marketName, _ := market["name"].(string)
		plugins, ok := market["plugins"].([]any)
		if !ok || !codexPluginNameValid(marketName, 128) {
			return nil, failure
		}
		params := map[string]any{}
		if path := market["path"]; path != nil {
			localPath, ok := path.(string)
			if !ok || !filepath.IsAbs(localPath) || len(localPath) > 4096 {
				return nil, failure
			}
			params["marketplacePath"] = localPath
		} else {
			params["remoteMarketplaceName"] = marketName
		}
		for _, value := range plugins {
			pluginCount++
			if pluginCount > 128 {
				return nil, failure
			}
			summary, ok := value.(map[string]any)
			if !ok {
				return nil, failure
			}
			id, name, installed, enabled, valid := codexPluginSummary(summary)
			if !valid || seen[id] {
				return nil, failure
			}
			seen[id] = true
			if !installed {
				continue
			}
			params["pluginName"] = name
			raw, err := client.request(ctx, "plugin/read", params)
			if err != nil {
				return nil, failure
			}
			read, err := decodeCodexPluginObject(raw)
			if err != nil {
				return nil, failure
			}
			detail, ok := read["plugin"].(map[string]any)
			if !ok {
				return nil, failure
			}
			readSummary, _ := detail["summary"].(map[string]any)
			rid, rname, rinstalled, renabled, valid := codexPluginSummary(readSummary)
			if !valid || rid != id || rname != name || !rinstalled || renabled != enabled || detail["marketplaceName"] != marketName {
				return nil, failure
			}
			names, ok := detail["mcpServers"].([]any)
			if !ok {
				return nil, failure
			}
			servers := make([]string, 0, len(names))
			unique := make(map[string]bool)
			for _, value := range names {
				serverCount++
				name, ok := value.(string)
				if !ok || !codexPluginNameValid(name, 128) || unique[name] || serverCount > 512 {
					return nil, failure
				}
				unique[name] = true
				servers = append(servers, name)
			}
			sort.Strings(servers)
			inventory[id] = servers
		}
	}
	return inventory, nil
}

func codexPluginSummary(summary map[string]any) (string, string, bool, bool, bool) {
	id, _ := summary["id"].(string)
	name, _ := summary["name"].(string)
	installed, hasInstalled := summary["installed"].(bool)
	enabled, hasEnabled := summary["enabled"].(bool)
	return id, name, installed, enabled, hasInstalled && hasEnabled && codexPluginNameValid(id, 256) && codexPluginNameValid(name, 128)
}

func codexPluginNameValid(name string, max int) bool {
	return name != "" && len(name) <= max && utf8.ValidString(name) && !strings.ContainsFunc(name, func(r rune) bool {
		return unicode.IsSpace(r) || unicode.IsControl(r) || r == utf8.RuneError
	})
}

// Preserve exact JSON keys and reject duplicates, including in ignored metadata.
func decodeCodexPluginObject(raw json.RawMessage) (map[string]any, error) {
	failure := codexPluginMCPError("received an unsupported inventory schema")
	if len(raw) > 1<<20 || !utf8.Valid(raw) {
		return nil, failure
	}
	decoder := json.NewDecoder(bytes.NewReader(raw))
	var value func(int) (any, error)
	value = func(depth int) (any, error) {
		if depth > 32 {
			return nil, failure
		}
		token, err := decoder.Token()
		if err != nil {
			return nil, failure
		}
		switch token {
		case json.Delim('{'):
			object := make(map[string]any)
			for decoder.More() {
				keyToken, err := decoder.Token()
				key, ok := keyToken.(string)
				if err != nil || !ok {
					return nil, failure
				}
				if _, duplicate := object[key]; duplicate {
					return nil, failure
				}
				child, err := value(depth + 1)
				if err != nil {
					return nil, err
				}
				object[key] = child
			}
			if end, err := decoder.Token(); err != nil || end != json.Delim('}') {
				return nil, failure
			}
			return object, nil
		case json.Delim('['):
			array := make([]any, 0)
			for decoder.More() {
				child, err := value(depth + 1)
				if err != nil {
					return nil, err
				}
				array = append(array, child)
			}
			if end, err := decoder.Token(); err != nil || end != json.Delim(']') {
				return nil, failure
			}
			return array, nil
		default:
			return token, nil
		}
	}
	parsed, err := value(0)
	object, ok := parsed.(map[string]any)
	if err != nil || !ok {
		return nil, failure
	}
	if _, err := decoder.Token(); err != io.EOF {
		return nil, failure
	}
	return object, nil
}

func writeCodexPluginMCPPolicy(home string, inventory map[string][]string) error {
	failure := codexPluginMCPError("could not seal task-private configuration")
	path := filepath.Join(home, "config.toml")
	raw, err := readBoundedRegularCodexConfig(path)
	if err != nil {
		return failure
	}
	config := make(map[string]any)
	if toml.Unmarshal(raw, &config) != nil {
		return failure
	}
	plugins, err := codexStringMap(config, "plugins")
	if err != nil {
		return failure
	}
	for id, names := range inventory {
		plugin, err := codexStringMap(plugins, id)
		if err != nil {
			return failure
		}
		servers, err := codexStringMap(plugin, "mcp_servers")
		if err != nil {
			return failure
		}
		for _, name := range names {
			server, err := codexStringMap(servers, name)
			if err != nil {
				return failure
			}
			server["enabled"] = false
			servers[name] = server
		}
		if len(names) > 0 {
			plugin["mcp_servers"] = servers
		}
		plugins[id] = plugin
	}
	config["plugins"] = plugins
	encoded, err := toml.Marshal(config)
	if err != nil || len(encoded) > maxCodexProjectPolicyConfigBytes {
		return failure
	}
	if err := writePrivateCodexConfig(path, encoded); err != nil {
		return failure
	}
	return nil
}

func (p *codexPluginMCPPlan) overrides() map[string]any {
	plugins := make(map[string]any)
	for id, names := range p.inventory {
		servers := make(map[string]any)
		for _, name := range names {
			servers[name] = map[string]any{"enabled": false}
		}
		if len(servers) > 0 {
			plugins[id] = map[string]any{"mcp_servers": servers}
		}
	}
	return plugins
}

func (p *codexPluginMCPPlan) launchOverrides() []string {
	var args []string
	if p.pluginsDisabled {
		args = append(args, "-c", "features.plugins=false")
	}
	plugins := p.overrides()
	if len(plugins) == 0 {
		return args
	}
	// Codex 0.153.4 splits -c paths on dots without unquoting segments.
	// Nested TOML values preserve literal plugin/server names instead.
	encoded, _ := jsonValueToCodexTOMLInline(plugins)
	return append(args, "-c", "plugins="+encoded)
}

func codexPluginsExplicitlyDisabled(config map[string]any) (bool, error) {
	features, err := codexStringMap(config, "features")
	if err != nil {
		return false, err
	}
	value, present := features["plugins"]
	if !present {
		return false, nil
	}
	enabled, ok := value.(bool)
	if !ok {
		return false, codexPluginMCPError("requires a boolean plugin feature state")
	}
	return !enabled, nil
}

func verifyCodexPluginMCPPolicy(ctx context.Context, client codexPluginRequester, opts ExecOptions) error {
	failure := codexPluginMCPError("effective configuration or inventory changed")
	inventory, err := discoverCodexPluginMCPWithCache(ctx, client, opts)
	if err != nil {
		return err
	}
	if !reflect.DeepEqual(inventory, opts.codexPluginMCP.inventory) {
		return failure
	}
	raw, err := client.request(ctx, "config/read", map[string]any{"cwd": opts.Cwd, "includeLayers": false})
	if err != nil {
		return failure
	}
	doc, err := decodeCodexPluginObject(raw)
	if err != nil {
		return failure
	}
	config, ok := doc["config"].(map[string]any)
	if !ok {
		return failure
	}
	disabled, err := codexPluginsExplicitlyDisabled(config)
	if err != nil || disabled != opts.codexPluginMCP.pluginsDisabled {
		return failure
	}
	if !disabled {
		plugins, err := codexStringMap(config, "plugins")
		if err != nil {
			return failure
		}
		for id, value := range plugins {
			plugin, ok := value.(map[string]any)
			if !ok {
				return failure
			}
			if _, known := inventory[id]; !known {
				if enabled, ok := plugin["enabled"].(bool); !ok || enabled {
					return failure
				}
			}
		}
		for id, names := range inventory {
			plugin, err := codexStringMap(plugins, id)
			if err != nil {
				return failure
			}
			servers, err := codexStringMap(plugin, "mcp_servers")
			if err != nil {
				return failure
			}
			for _, value := range servers {
				server, ok := value.(map[string]any)
				if !ok {
					return failure
				}
				if enabled, ok := server["enabled"].(bool); !ok || enabled {
					return failure
				}
			}
			for _, name := range names {
				server, err := codexStringMap(servers, name)
				if err != nil {
					return failure
				}
				if enabled, ok := server["enabled"].(bool); !ok || enabled {
					return failure
				}
			}
		}
	}
	expected, err := normalizedManagedCodexMcpServers(opts.McpConfig)
	if err != nil {
		return failure
	}
	actual, err := effectiveCodexMcpServers(config)
	if err != nil || !reflect.DeepEqual(expected, actual) {
		return failure
	}
	return verifyCodexPluginCacheBindings(opts.codexPluginMCP)
}

func applyCodexPluginMCPOverrides(params map[string]any, plan *codexPluginMCPPlan) {
	if plan == nil || !plan.ready {
		return
	}
	config, _ := params["config"].(map[string]any)
	if config == nil {
		config = make(map[string]any)
	}
	if plan.pluginsDisabled {
		features, _ := config["features"].(map[string]any)
		if features == nil {
			features = make(map[string]any)
		}
		features["plugins"] = false
		config["features"] = features
	}
	if plugins := plan.overrides(); len(plugins) > 0 {
		config["plugins"] = plugins
	}
	params["config"] = config
}
