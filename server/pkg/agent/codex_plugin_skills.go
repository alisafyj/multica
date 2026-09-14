package agent

import (
	"bytes"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"io/fs"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"unicode"

	"github.com/pelletier/go-toml/v2"
)

// CodexPluginSkillRoot identifies a verified skill directory in a plugin cache.
// Name is the manifest namespace; only PluginID and RelativePath identify paths.
type CodexPluginSkillRoot struct {
	PluginID           string
	Name               string
	Path               string
	RelativePath       string
	InstallationDigest string
}

var errCodexPluginSkillRoots = errors.New("Codex plugin skill roots could not be fully verified")

// ResolveCodexPluginSkillRoot binds an installed root without consulting its
// enabled state. A selected but missing or ambiguous installation is an error.
func ResolveCodexPluginSkillRoot(home, id string) (CodexPluginSkillRoot, error) {
	if !filepath.IsAbs(home) {
		return CodexPluginSkillRoot{}, errCodexPluginSkillRoots
	}
	home, err := filepath.EvalSymlinks(home)
	if err != nil {
		return CodexPluginSkillRoot{}, errCodexPluginSkillRoots
	}
	root, err := codexPluginSkillRoot(home, id)
	if err != nil || root == nil {
		return CodexPluginSkillRoot{}, errCodexPluginSkillRoots
	}
	return *root, nil
}

// CodexPluginSkillRoots reads explicit local plugin configuration without RPCs
// or metadata writes. An error may accompany roots verified for other entries.
func CodexPluginSkillRoots(home string) ([]CodexPluginSkillRoot, error) {
	if !filepath.IsAbs(home) {
		return nil, errCodexPluginSkillRoots
	}
	home, err := filepath.EvalSymlinks(home)
	if os.IsNotExist(err) {
		return nil, nil
	}
	if err != nil {
		return nil, errCodexPluginSkillRoots
	}
	configPath := filepath.Join(home, "config.toml")
	raw, err := readBoundedRegularCodexConfig(configPath)
	if err != nil {
		return nil, errCodexPluginSkillRoots
	}
	var config map[string]any
	if toml.Unmarshal(raw, &config) != nil {
		return nil, errCodexPluginSkillRoots
	}
	disabled, err := codexPluginsExplicitlyDisabled(config)
	if err != nil {
		return nil, errCodexPluginSkillRoots
	}
	if disabled {
		return nil, nil
	}
	plugins, err := codexStringMap(config, "plugins")
	if err != nil {
		return nil, errCodexPluginSkillRoots
	}
	ids := make([]string, 0, len(plugins))
	for id := range plugins {
		ids = append(ids, id)
	}
	sort.Strings(ids)
	var partial error
	if len(ids) > 128 {
		ids = ids[:128]
		partial = errCodexPluginSkillRoots
	}
	var roots []CodexPluginSkillRoot
	for _, id := range ids {
		plugin, ok := plugins[id].(map[string]any)
		if !ok {
			partial = errCodexPluginSkillRoots
			continue
		}
		enabled, ok := plugin["enabled"].(bool)
		if !ok {
			partial = errCodexPluginSkillRoots
			continue
		}
		if !enabled {
			continue
		}
		root, err := codexPluginSkillRoot(home, id)
		if err != nil {
			partial = errCodexPluginSkillRoots
			continue
		}
		if root != nil {
			roots = append(roots, *root)
		}
	}
	current, err := readBoundedRegularCodexConfig(configPath)
	if err != nil || !bytes.Equal(raw, current) {
		return nil, errCodexPluginSkillRoots
	}
	return roots, partial
}

func codexPluginSkillRoot(home, id string) (*CodexPluginSkillRoot, error) {
	binding, err := snapshotCodexPluginCacheRoot(home, id)
	if err != nil {
		return nil, errCodexPluginSkillRoots
	}
	manifest, err := readCodexPluginCacheManifest(&binding, id)
	if err != nil {
		return nil, errCodexPluginSkillRoots
	}
	relative := "skills"
	value, explicit := manifest["skills"]
	if explicit {
		declared, ok := value.(string)
		if !ok || !strings.HasPrefix(declared, "./") {
			return nil, errCodexPluginSkillRoots
		}
		relative = strings.TrimSuffix(strings.TrimPrefix(declared, "./"), "/")
		if relative == "." || len(relative) > 4096 || !fs.ValidPath(relative) || strings.ContainsAny(relative, "\\:") || strings.ContainsFunc(relative, unicode.IsControl) {
			return nil, errCodexPluginSkillRoots
		}
	}
	candidate := binding.Root
	for _, component := range strings.Split(relative, "/") {
		candidate = filepath.Join(candidate, component)
		info, err := os.Lstat(candidate)
		if os.IsNotExist(err) && !explicit {
			return nil, nil
		}
		if err != nil || !info.IsDir() || info.Mode()&os.ModeSymlink != 0 {
			return nil, errCodexPluginSkillRoots
		}
	}
	canonical, err := filepath.EvalSymlinks(candidate)
	if err != nil || canonical != candidate {
		return nil, errCodexPluginSkillRoots
	}
	skillInfo, err := os.Stat(canonical)
	if err != nil {
		return nil, errCodexPluginSkillRoots
	}
	device, inode, supported := codexPluginCacheFileIdentity(skillInfo)
	if !supported {
		return nil, errCodexPluginSkillRoots
	}
	// Recheck root identity and manifest pins after resolving the skill directory.
	after, err := snapshotCodexPluginCacheRoot(home, id)
	if err != nil {
		return nil, errCodexPluginSkillRoots
	}
	if _, err := readCodexPluginCacheManifest(&after, id); err != nil || after != binding {
		return nil, errCodexPluginSkillRoots
	}
	current, err := os.Lstat(canonical)
	if err != nil || !current.IsDir() || !os.SameFile(skillInfo, current) {
		return nil, errCodexPluginSkillRoots
	}
	metadata, err := json.Marshal(struct {
		Schema                string
		PluginID              string
		Installation          codexPluginCacheBinding
		RelativePath          string
		RootDevice, RootInode uint64
	}{"codex_plugin_skill_installation/v1", id, binding, relative, device, inode})
	if err != nil {
		return nil, errCodexPluginSkillRoots
	}
	digest := sha256.Sum256(metadata)
	name, _ := manifest["name"].(string)
	return &CodexPluginSkillRoot{PluginID: id, Name: name, Path: canonical, RelativePath: relative, InstallationDigest: "sha256:" + hex.EncodeToString(digest[:])}, nil
}
