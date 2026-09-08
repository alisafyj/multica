package execenv

import (
	"encoding/json"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"unicode"

	"github.com/multica-ai/multica/server/pkg/agent"
)

const claudeRuntimeSkillSettingsFile = "claude-runtime-skill-settings.json"

// RuntimeSkillRefForEnv identifies a runtime-local skill for provider-specific
// task environment filtering. Provider and runtime are already selected by the
// task, so only the discovery root and provider-native key are needed here.
type RuntimeSkillRefForEnv struct {
	Root   string
	Key    string
	Name   string
	Plugin string
}

func cleanRuntimeSkillKey(key string) (string, bool) {
	cleaned := filepath.Clean(filepath.FromSlash(strings.TrimSpace(key)))
	if cleaned == "." || filepath.IsAbs(cleaned) || cleaned == ".." || strings.HasPrefix(cleaned, ".."+string(filepath.Separator)) {
		return "", false
	}
	return filepath.ToSlash(cleaned), true
}

func prepareClaudeSkillSettings(envRoot string, disabled []RuntimeSkillRefForEnv, workspaceSkills []SkillContextForEnv) (string, error) {
	path := filepath.Join(envRoot, claudeRuntimeSkillSettingsFile)
	if len(disabled) == 0 {
		if err := os.Remove(path); err != nil && !os.IsNotExist(err) {
			return "", err
		}
		return "", nil
	}

	overrides := make(map[string]string)
	deny := make([]string, 0, len(disabled)*2)
	seenDeny := make(map[string]struct{}, len(disabled)*2)
	addDeny := func(rule string) {
		if _, exists := seenDeny[rule]; exists {
			return
		}
		seenDeny[rule] = struct{}{}
		deny = append(deny, rule)
	}
	for _, skill := range disabled {
		key, ok := cleanRuntimeSkillKey(skill.Key)
		if !ok {
			continue
		}
		invocationName := strings.TrimSpace(skill.Name)
		if invocationName == "" {
			invocationName = filepath.Base(filepath.FromSlash(key))
		}
		if workspaceClaimsRuntimeSkill(invocationName, workspaceSkills) {
			continue
		}
		// Claude Code's skillOverrides fully hides personal/project skills.
		// Plugin skills ignore that setting, so the permission deny below is
		// also emitted for every key and is the enforcement path for plugins.
		if skill.Root != "plugin" {
			overrides[invocationName] = "off"
		} else {
			invocationName = key
		}
		addDeny("Skill(" + invocationName + ")")
		addDeny("Skill(" + invocationName + " *)")
	}
	if len(overrides) == 0 && len(deny) == 0 {
		if err := os.Remove(path); err != nil && !os.IsNotExist(err) {
			return "", err
		}
		return "", nil
	}
	payload := map[string]any{
		"skillOverrides": overrides,
		"permissions": map[string]any{
			"deny": deny,
		},
	}
	data, err := json.Marshal(payload)
	if err != nil {
		return "", err
	}
	if err := os.WriteFile(path, data, 0o600); err != nil {
		return "", err
	}
	return path, nil
}

func ensureCodexDisabledSkillsConfig(configPath, codexHome string, disabled []RuntimeSkillRefForEnv, workspaceSkills []SkillContextForEnv) ([]agent.CodexPluginSkillBinding, error) {
	if len(disabled) == 0 {
		return nil, nil
	}
	home := ""
	pluginRoots := make(map[string]agent.CodexPluginSkillRoot)
	var bindings []agent.CodexPluginSkillBinding
	var pluginConfig *os.File
	paths := make([]string, 0, len(disabled))
	seen := make(map[string]struct{}, len(disabled))
	for _, skill := range disabled {
		key, ok := cleanRuntimeSkillKey(skill.Key)
		if !ok {
			if skill.Root == "plugin" {
				return nil, fmt.Errorf("invalid selected Codex plugin skill")
			}
			continue
		}
		var skillPath string
		var binding *agent.CodexPluginSkillBinding
		switch skill.Root {
		case "provider":
			firstKeyPart := strings.SplitN(key, "/", 2)[0]
			if workspaceClaimsRuntimeSkill(firstKeyPart, workspaceSkills) {
				continue
			}
			skillPath = filepath.Join(codexHome, "skills", filepath.FromSlash(key), "SKILL.md")
		case "universal":
			if home == "" {
				var err error
				home, err = os.UserHomeDir()
				if err != nil {
					return nil, fmt.Errorf("resolve user home for disabled Codex skills: %w", err)
				}
			}
			skillPath = filepath.Join(home, ".agents", "skills", filepath.FromSlash(key), "SKILL.md")
		case "plugin":
			if pluginConfig == nil {
				var err error
				pluginConfig, err = openCodexPluginSkillConfig(configPath, codexHome)
				if err != nil {
					return nil, err
				}
				defer pluginConfig.Close()
			}
			root, exists := pluginRoots[skill.Plugin]
			if !exists {
				resolved, err := agent.ResolveCodexPluginSkillRoot(codexHome, skill.Plugin)
				if err != nil || resolved.InstallationDigest == "" {
					return nil, fmt.Errorf("selected Codex plugin skill installation could not be verified")
				}
				root = resolved
				pluginRoots[skill.Plugin] = root
			}
			var err error
			skillPath, err = disabledCodexPluginSkillPath([]agent.CodexPluginSkillRoot{root}, skill)
			if err != nil {
				return nil, err
			}
			binding = &agent.CodexPluginSkillBinding{
				PluginID: skill.Plugin, Key: skill.Key, SkillPath: skillPath, InstallationDigest: root.InstallationDigest,
			}
		default:
			continue
		}
		if _, exists := seen[skillPath]; exists {
			continue
		}
		seen[skillPath] = struct{}{}
		paths = append(paths, skillPath)
		if binding != nil {
			bindings = append(bindings, *binding)
		}
	}
	if len(paths) == 0 {
		return nil, nil
	}
	file := pluginConfig
	if file == nil {
		var err error
		file, err = os.OpenFile(configPath, os.O_CREATE|os.O_APPEND|os.O_WRONLY, 0o600)
		if err != nil {
			return nil, err
		}
		defer file.Close()
	}
	for _, path := range paths {
		block := "\n[[skills.config]]\npath = " + strconv.Quote(filepath.ToSlash(path)) + "\nenabled = false\n"
		if _, err := file.WriteString(block); err != nil {
			return nil, err
		}
	}
	return bindings, nil
}

// An unavailable selected plugin is an error: ignoring it would run the skill
// the agent explicitly disabled. Name is display metadata, never path authority.
func disabledCodexPluginSkillPath(roots []agent.CodexPluginSkillRoot, skill RuntimeSkillRefForEnv) (string, error) {
	failure := fmt.Errorf("selected Codex plugin skill is invalid or unavailable")
	relative, ok := strings.CutPrefix(skill.Key, skill.Plugin+":")
	if !ok || skill.Plugin == "" || len(skill.Key) > 512 || !fs.ValidPath(relative) || strings.ContainsAny(relative, "\\:") || strings.ContainsFunc(relative, unicode.IsControl) {
		return "", failure
	}
	for _, root := range roots {
		if root.PluginID != skill.Plugin {
			continue
		}
		within := relative
		if root.RelativePath != "." {
			if relative == root.RelativePath {
				within = "."
			} else if within, ok = strings.CutPrefix(relative, root.RelativePath+"/"); !ok {
				continue
			}
		}
		candidate := filepath.Join(root.Path, filepath.FromSlash(within), "SKILL.md")
		canonical, err := filepath.EvalSymlinks(candidate)
		if err != nil || canonical != candidate {
			return "", failure
		}
		info, err := os.Lstat(candidate)
		if err != nil || !info.Mode().IsRegular() {
			return "", failure
		}
		return candidate, nil
	}
	return "", failure
}

func openCodexPluginSkillConfig(configPath, codexHome string) (*os.File, error) {
	failure := fmt.Errorf("selected Codex plugin skills require an existing task-private config.toml")
	if filepath.Clean(configPath) != filepath.Join(filepath.Clean(codexHome), "config.toml") {
		return nil, failure
	}
	root, err := openVerifiedCodexHomeRoot(codexHome, "disabled plugin skills")
	if err != nil {
		return nil, failure
	}
	defer root.Close()
	taskInfo, err := root.Stat(".")
	if err != nil {
		return nil, failure
	}
	configInfo, err := root.Lstat("config.toml")
	if err != nil || !configInfo.Mode().IsRegular() {
		return nil, failure
	}
	userHome, err := os.UserHomeDir()
	if err != nil {
		return nil, failure
	}
	for _, shared := range []string{resolveSharedCodexHome(), filepath.Join(userHome, ".codex")} {
		if info, err := os.Stat(shared); err == nil && os.SameFile(info, taskInfo) {
			return nil, failure
		}
		if info, err := os.Stat(filepath.Join(shared, "config.toml")); err == nil && os.SameFile(info, configInfo) {
			return nil, failure
		}
	}
	file, err := root.OpenFile("config.toml", os.O_APPEND|os.O_WRONLY, 0)
	if err != nil {
		return nil, failure
	}
	opened, err := file.Stat()
	if err != nil || !os.SameFile(configInfo, opened) {
		file.Close()
		return nil, failure
	}
	return file, nil
}

func workspaceClaimsRuntimeSkill(name string, workspaceSkills []SkillContextForEnv) bool {
	claim := sanitizeSkillName(name)
	for _, skill := range workspaceSkills {
		if sanitizeSkillName(skill.Name) == claim {
			return true
		}
	}
	return false
}
