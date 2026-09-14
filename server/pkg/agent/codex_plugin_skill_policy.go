package agent

import (
	"context"
	"errors"
	"io"
	"io/fs"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"unicode"

	"github.com/pelletier/go-toml/v2"
)

// MaxCodexPluginSkillSelections bounds task-selected plugin skills at execution.
const MaxCodexPluginSkillSelections = 128

// CodexPluginSkillSelection is authoritative task intent, not an environment hint.
type CodexPluginSkillSelection struct {
	PluginID string
	Key      string
}

// CodexPluginSkillBinding pins the installation used to render a disabled path.
type CodexPluginSkillBinding struct {
	PluginID           string
	Key                string
	SkillPath          string
	InstallationDigest string
}

type codexPluginSkillPolicy struct {
	home     string
	bindings []CodexPluginSkillBinding
}

// CodexPluginSkillsSupported requires the actual resolved built-in CLI version.
func CodexPluginSkillsSupported(version string, builtin bool) bool {
	return builtin && strings.TrimPrefix(strings.TrimSpace(version), "codex-cli ") == "0.153.4"
}

var errCodexPluginSkillPolicy = errors.New("Codex selected plugin skill policy could not be verified")

func (b *codexBackend) prepareCodexPluginSkillPolicy(opts ExecOptions) (ExecOptions, error) {
	if len(opts.CodexPluginSkillSelections) == 0 && len(opts.CodexPluginSkillBindings) == 0 {
		return opts, nil
	}
	if !CodexPluginSkillsSupported(b.cfg.CLIVersion, b.cfg.BuiltinRuntime) ||
		len(opts.CodexPluginSkillSelections) == 0 || len(opts.CodexPluginSkillSelections) > MaxCodexPluginSkillSelections ||
		len(opts.CodexPluginSkillSelections) != len(opts.CodexPluginSkillBindings) {
		return opts, errCodexPluginSkillPolicy
	}
	for _, args := range [][]string{opts.ExtraArgs, opts.CustomArgs, b.cfg.commandAt(b.cfg.ExecutablePath).Prefix} {
		if !codexPluginSkillArgsSafe(args) {
			return opts, errCodexPluginSkillPolicy
		}
	}
	selected := make(map[CodexPluginSkillSelection]bool)
	for _, selection := range opts.CodexPluginSkillSelections {
		if selected[selection] || selection.PluginID == "" || selection.Key == "" {
			return opts, errCodexPluginSkillPolicy
		}
		selected[selection] = true
	}
	for _, binding := range opts.CodexPluginSkillBindings {
		selection := CodexPluginSkillSelection{PluginID: binding.PluginID, Key: binding.Key}
		if !selected[selection] {
			return opts, errCodexPluginSkillPolicy
		}
		delete(selected, selection)
	}
	if len(selected) != 0 || !filepath.IsAbs(b.cfg.Env["CODEX_HOME"]) {
		return opts, errCodexPluginSkillPolicy
	}
	home, err := canonicalExistingDirectory(b.cfg.Env["CODEX_HOME"], "CODEX_HOME")
	if err != nil || rejectGlobalCodexHome(home, b.cfg.Env["HOME"]) != nil {
		return opts, errCodexPluginSkillPolicy
	}
	if opts.Cwd, err = canonicalExistingDirectory(opts.Cwd, "working directory"); err != nil {
		return opts, errCodexPluginSkillPolicy
	}
	// Retain the environment's original binding across MCP preparation and retries.
	// Resolving a replacement installation must never update this expected value.
	opts.CodexPluginSkillSelections = append([]CodexPluginSkillSelection(nil), opts.CodexPluginSkillSelections...)
	opts.CodexPluginSkillBindings = append([]CodexPluginSkillBinding(nil), opts.CodexPluginSkillBindings...)
	bindings := append([]CodexPluginSkillBinding(nil), opts.CodexPluginSkillBindings...)
	sort.Slice(bindings, func(i, j int) bool { return bindings[i].Key < bindings[j].Key })
	opts.codexPluginSkills = &codexPluginSkillPolicy{home: home, bindings: bindings}
	return opts, verifyCodexPluginSkillPolicy(b.cfg, opts)
}

func verifyCodexPluginSkillPolicy(cfg Config, opts ExecOptions) error {
	plan := opts.codexPluginSkills
	if plan == nil {
		if len(opts.CodexPluginSkillSelections) != 0 || len(opts.CodexPluginSkillBindings) != 0 {
			return errCodexPluginSkillPolicy
		}
		return nil
	}
	if !CodexPluginSkillsSupported(cfg.CLIVersion, cfg.BuiltinRuntime) {
		return errCodexPluginSkillPolicy
	}
	home, err := canonicalExistingDirectory(cfg.Env["CODEX_HOME"], "CODEX_HOME")
	if err != nil || home != plan.home {
		return errCodexPluginSkillPolicy
	}
	for _, binding := range plan.bindings {
		root, err := ResolveCodexPluginSkillRoot(plan.home, binding.PluginID)
		if err != nil || root.InstallationDigest != binding.InstallationDigest {
			return errCodexPluginSkillPolicy
		}
		relative, ok := strings.CutPrefix(binding.Key, binding.PluginID+":")
		if !ok || len(binding.Key) > 512 || !fs.ValidPath(relative) || strings.ContainsAny(relative, "\\:") || strings.ContainsFunc(relative, unicode.IsControl) {
			return errCodexPluginSkillPolicy
		}
		within := "."
		if relative != root.RelativePath {
			within, ok = strings.CutPrefix(relative, root.RelativePath+"/")
			if !ok {
				return errCodexPluginSkillPolicy
			}
		}
		expected := filepath.Join(root.Path, filepath.FromSlash(within), "SKILL.md")
		// EvalSymlinks does not correct case aliases on case-insensitive volumes.
		// Codex's disabled-path identity must match the discovered directory names.
		directory := root.Path
		for _, component := range strings.Split(filepath.Join(filepath.FromSlash(within), "SKILL.md"), string(filepath.Separator)) {
			handle, err := os.Open(directory)
			if err != nil {
				return errCodexPluginSkillPolicy
			}
			entries, readErr := handle.ReadDir(8193)
			closeErr := handle.Close()
			if readErr != nil && !errors.Is(readErr, io.EOF) || closeErr != nil || len(entries) > 8192 {
				return errCodexPluginSkillPolicy
			}
			found := false
			for _, entry := range entries {
				if entry.Name() == component {
					found = true
					break
				}
			}
			if !found {
				return errCodexPluginSkillPolicy
			}
			directory = filepath.Join(directory, component)
		}
		resolved, err := filepath.EvalSymlinks(expected)
		if err != nil || resolved != expected || binding.SkillPath != expected {
			return errCodexPluginSkillPolicy
		}
		info, err := os.Lstat(expected)
		if err != nil || !info.Mode().IsRegular() {
			return errCodexPluginSkillPolicy
		}
	}
	raw, _, err := readCodexPluginMetadataFile(filepath.Join(plan.home, "config.toml"))
	if err != nil {
		return errCodexPluginSkillPolicy
	}
	var config map[string]any
	if toml.Unmarshal(raw, &config) != nil {
		return errCodexPluginSkillPolicy
	}
	entries, err := codexPluginSkillEntries(config)
	if err != nil {
		return err
	}
	states := make(map[string]bool, len(entries))
	for _, entry := range entries {
		states[entry["path"].(string)] = entry["enabled"].(bool)
	}
	for _, binding := range plan.bindings {
		if enabled, present := states[binding.SkillPath]; !present || enabled {
			return errCodexPluginSkillPolicy
		}
	}
	return nil
}

func codexPluginSkillEntries(config map[string]any) ([]map[string]any, error) {
	value, present := config["skills"]
	if !present {
		return nil, nil
	}
	skills, ok := value.(map[string]any)
	if !ok || len(skills) != 1 {
		return nil, errCodexPluginSkillPolicy
	}
	values, ok := skills["config"].([]any)
	if !ok || len(values) > 4096 {
		return nil, errCodexPluginSkillPolicy
	}
	entries := make([]map[string]any, 0, len(values))
	seen := make(map[string]bool)
	for _, value := range values {
		entry, ok := value.(map[string]any)
		if !ok || len(entry) != 2 {
			return nil, errCodexPluginSkillPolicy
		}
		path, pathOK := entry["path"].(string)
		enabled, enabledOK := entry["enabled"].(bool)
		if !pathOK || !enabledOK || !filepath.IsAbs(path) || filepath.Clean(path) != path || len(path) > 4096 || strings.ContainsFunc(path, unicode.IsControl) || seen[path] {
			return nil, errCodexPluginSkillPolicy
		}
		seen[path] = true
		entries = append(entries, map[string]any{"path": path, "enabled": enabled})
	}
	return entries, nil
}

func applyCodexPluginSkillPolicy(ctx context.Context, client *codexClient, opts ExecOptions, params map[string]any) error {
	if err := verifyCodexPluginSkillPolicy(client.cfg, opts); err != nil {
		return err
	}
	if opts.codexPluginSkills == nil {
		return nil
	}
	effective, err := readCodexConfiguration(ctx, client, opts.Cwd, codexConfigurationReadPurposeTaskEffective)
	if err != nil {
		return errCodexPluginSkillPolicy
	}
	entries, err := codexPluginSkillEntries(effective.Config)
	if err != nil {
		return err
	}
	positions := make(map[string]int, len(entries))
	for index, entry := range entries {
		positions[entry["path"].(string)] = index
	}
	for _, binding := range opts.codexPluginSkills.bindings {
		if index, present := positions[binding.SkillPath]; present {
			entries[index]["enabled"] = false
		} else {
			entries = append(entries, map[string]any{"path": binding.SkillPath, "enabled": false})
		}
	}
	config, ok := params["config"].(map[string]any)
	if params["config"] != nil && !ok {
		return errCodexPluginSkillPolicy
	}
	if config == nil {
		config = make(map[string]any)
	}
	config["skills"] = map[string]any{"config": entries}
	params["config"] = config
	// A config RPC may run metadata hooks; recheck immediately before the thread RPC.
	return verifyCodexPluginSkillPolicy(client.cfg, opts)
}

func codexPluginSkillArgsSafe(args []string) bool {
	for index := 0; index < len(args); index++ {
		arg := args[index]
		if arg == "--" || arg == "--profile" || strings.HasPrefix(arg, "--profile=") || strings.HasPrefix(arg, "-p") {
			return false
		}
		var assignment string
		switch {
		case arg == "-c" || arg == "--config":
			index++
			if index >= len(args) {
				return false
			}
			assignment = args[index]
		case strings.HasPrefix(arg, "--config="):
			assignment = strings.TrimPrefix(arg, "--config=")
		case strings.HasPrefix(arg, "-c"):
			assignment = strings.TrimPrefix(arg, "-c")
		default:
			continue
		}
		key, _, found := strings.Cut(assignment, "=")
		var parsed map[string]any
		if !found || toml.Unmarshal([]byte(strings.TrimSpace(key)+" = false"), &parsed) != nil || len(parsed) != 1 {
			return false
		}
		for root := range parsed {
			if root == "skills" || root == "profiles" {
				return false
			}
		}
	}
	return true
}
