package agent

import (
	"context"
	"crypto/rand"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"io"
	"io/fs"
	"os"
	"path/filepath"
	"reflect"
	"sort"
	"strings"
)

// Bind missing marketplace entries to one observed cache installation. Never
// reproduce Codex's version ordering or promote the generated temporary ID.
type codexPluginCacheBinding struct {
	CacheRoot        string
	Root             string
	RootDevice       uint64
	RootInode        uint64
	DirectoryVersion string
	LatestTarget     string
	LatestDevice     uint64
	LatestInode      uint64
	ManifestPath     string
	ManifestVersion  string
	ManifestHash     string
	MCPPresent       bool
	MCPHash          string
}

func codexPluginCacheComponent(value string) bool {
	if value == "" || len(value) > 128 || value[0] == '.' {
		return false
	}
	for _, r := range value {
		if !(r >= 'a' && r <= 'z' || r >= 'A' && r <= 'Z' || r >= '0' && r <= '9' || r == '-' || r == '_' || r == '.') {
			return false
		}
	}
	return true
}

func codexPluginCacheVersion(value string) bool {
	return codexPluginCacheComponent(strings.ReplaceAll(value, "+", "-"))
}

func readCodexPluginMetadataFile(path string) ([]byte, string, error) {
	failure := codexPluginMCPError("cache metadata could not be verified")
	before, err := os.Lstat(path)
	if err != nil || !before.Mode().IsRegular() || before.Size() > 1<<20 {
		return nil, "", failure
	}
	f, err := os.Open(path)
	if err != nil {
		return nil, "", failure
	}
	opened, statErr := f.Stat()
	data, readErr := io.ReadAll(io.LimitReader(f, (1<<20)+1))
	closeErr := f.Close()
	after, afterErr := os.Lstat(path)
	if statErr != nil || readErr != nil || closeErr != nil || afterErr != nil || len(data) > 1<<20 || !after.Mode().IsRegular() || !os.SameFile(before, opened) || !os.SameFile(opened, after) || before.Size() != after.Size() || !before.ModTime().Equal(after.ModTime()) {
		return nil, "", failure
	}
	digest := sha256.Sum256(data)
	return data, hex.EncodeToString(digest[:]), nil
}

func snapshotCodexPluginCache(home, id string) (codexPluginCacheBinding, error) {
	binding, err := snapshotCodexPluginCacheRoot(home, id)
	if err != nil {
		return binding, err
	}
	doc, err := readCodexPluginCacheManifest(&binding, id)
	if err != nil {
		return binding, err
	}
	failure := codexPluginMCPError("cache installation is unavailable or ambiguous")
	reference, hasReference := doc["mcpServers"]
	if hasReference && reference != "./.mcp.json" {
		return binding, failure
	}
	mcp := filepath.Join(binding.Root, ".mcp.json")
	// The pinned native contract reads this one JSON file (also as the
	// default when mcpServers is absent). Other reference forms are rejected,
	// so the names-input closure is the manifest plus this optional file.
	if _, err := os.Lstat(mcp); os.IsNotExist(err) && !hasReference {
		return binding, nil
	}
	data, digest, err := readCodexPluginMetadataFile(mcp)
	if err != nil {
		return binding, failure
	}
	if _, err := decodeCodexPluginObject(data); err != nil {
		return binding, failure
	}
	binding.MCPPresent, binding.MCPHash = true, digest
	return binding, nil
}

func snapshotCodexPluginCacheRoot(home, id string) (codexPluginCacheBinding, error) {
	failure := codexPluginMCPError("cache installation is unavailable or ambiguous")
	var binding codexPluginCacheBinding
	parts := strings.Split(id, "@")
	if len(parts) != 2 || !codexPluginCacheComponent(parts[0]) || !codexPluginCacheComponent(parts[1]) {
		return binding, failure
	}
	cache, err := filepath.EvalSymlinks(filepath.Join(home, "plugins", "cache"))
	if err != nil || !filepath.IsAbs(cache) {
		return binding, failure
	}
	parent := cache
	for _, component := range []string{parts[1], parts[0]} {
		parent = filepath.Join(parent, component)
		info, err := os.Lstat(parent)
		if err != nil || !info.IsDir() || info.Mode()&os.ModeSymlink != 0 {
			return binding, failure
		}
	}
	entries, err := os.ReadDir(parent)
	if err != nil || len(entries) == 0 || len(entries) > 2 {
		return binding, failure
	}
	version, latest := "", false
	for _, entry := range entries {
		switch {
		case entry.IsDir() && codexPluginCacheVersion(entry.Name()):
			if version != "" {
				return binding, failure
			}
			version = entry.Name()
		case entry.Name() == "latest" && entry.Type()&os.ModeSymlink != 0:
			latest = true
		default:
			return binding, failure
		}
	}
	if version == "" {
		return binding, failure
	}
	root := filepath.Join(parent, version)
	canonical, err := filepath.EvalSymlinks(root)
	if err != nil || canonical != root {
		return binding, failure
	}
	rootInfo, err := os.Stat(root)
	if err != nil {
		return binding, failure
	}
	device, inode, supported := codexPluginCacheFileIdentity(rootInfo)
	if !supported {
		return binding, failure
	}
	binding.RootDevice, binding.RootInode = device, inode
	if latest {
		// Native ignores symlink entries when selecting a version. Permit only
		// this direct alias to the independently selected unique physical root.
		path := filepath.Join(parent, "latest")
		before, beforeErr := os.Lstat(path)
		target, linkErr := os.Readlink(path)
		canonical, canonicalErr := filepath.EvalSymlinks(path)
		targetInfo, targetErr := os.Stat(path)
		after, afterErr := os.Lstat(path)
		if beforeErr != nil || linkErr != nil || canonicalErr != nil || targetErr != nil || afterErr != nil || before.Mode()&os.ModeSymlink == 0 || after.Mode()&os.ModeSymlink == 0 || target != root || canonical != root || !os.SameFile(before, after) || !os.SameFile(rootInfo, targetInfo) {
			return binding, failure
		}
		device, inode, supported := codexPluginCacheFileIdentity(before)
		if !supported {
			return binding, failure
		}
		binding.LatestTarget, binding.LatestDevice, binding.LatestInode = target, device, inode
	}
	// Only the outer shared cache link is allowed. A selected plugin tree
	// cannot introduce additional links or non-regular metadata inputs.
	count := 0
	if err := filepath.WalkDir(root, func(path string, entry fs.DirEntry, err error) error {
		count++
		if err != nil || count > 8192 || len(path) > 4096 || (!entry.IsDir() && !entry.Type().IsRegular()) {
			return failure
		}
		return nil
	}); err != nil {
		return binding, failure
	}
	binding.CacheRoot, binding.Root, binding.DirectoryVersion = cache, root, version
	return binding, nil
}

func readCodexPluginCacheManifest(binding *codexPluginCacheBinding, id string) (map[string]any, error) {
	failure := codexPluginMCPError("cache installation is unavailable or ambiguous")
	name, _, ok := strings.Cut(id, "@")
	if !ok {
		return nil, failure
	}
	root := binding.Root
	// AgentPlugin has different defaults and an additional MCP overlay. This
	// bridge supports only the pinned legacy manifest format.
	if _, err := os.Lstat(filepath.Join(root, "plugin.json")); !os.IsNotExist(err) {
		return nil, failure
	}
	var manifest []byte
	var err error
	// Codex 0.153.4 DISCOVERABLE_PLUGIN_MANIFEST_PATHS order.
	for _, directory := range []string{".codex-plugin", ".claude-plugin", ".cursor-plugin"} {
		path := filepath.Join(root, directory, "plugin.json")
		if _, err := os.Lstat(path); os.IsNotExist(err) {
			continue
		}
		manifest, binding.ManifestHash, err = readCodexPluginMetadataFile(path)
		if err != nil {
			return nil, failure
		}
		binding.ManifestPath = path
		break
	}
	doc, err := decodeCodexPluginObject(manifest)
	if err != nil || doc["name"] != name {
		return nil, failure
	}
	// Legacy installation defaults to "local", but plugin/read preserves a
	// missing manifest version as null. Do not infer its RPC value from the dir.
	version := ""
	if value := doc["version"]; value != nil {
		provided, ok := value.(string)
		if !ok {
			return nil, failure
		}
		if provided = strings.TrimSpace(provided); provided != "" {
			if !codexPluginCacheVersion(provided) {
				return nil, failure
			}
			version = provided
		}
	}
	binding.ManifestVersion = version
	return doc, nil
}

func verifyCodexPluginCacheBindings(plan *codexPluginMCPPlan) error {
	if plan == nil {
		return nil
	}
	for id, expected := range plan.cache {
		actual, err := snapshotCodexPluginCache(plan.home, id)
		if err != nil || actual != expected {
			return codexPluginMCPError("cache metadata changed")
		}
	}
	return nil
}

func verifyCodexPluginMetadataBridge(opened *os.Root, sourceInfo os.FileInfo, directory, link, root, path, digest string) error {
	failure := codexPluginMCPError("temporary cache metadata could not be verified")
	info, err := os.Lstat(directory)
	openedInfo, openedErr := opened.Stat(".")
	if err != nil || openedErr != nil || !codexPluginMetadataOwnedDirectory(info) || !os.SameFile(openedInfo, info) {
		return failure
	}
	for _, dir := range []string{filepath.Join(directory, ".agents"), filepath.Dir(path)} {
		info, err := os.Lstat(dir)
		if err != nil || !info.IsDir() || info.Mode()&os.ModeSymlink != 0 {
			return failure
		}
	}
	info, err = os.Lstat(link)
	target, linkErr := os.Readlink(link)
	canonical, canonicalErr := filepath.EvalSymlinks(link)
	targetInfo, targetErr := os.Stat(link)
	if err != nil || info.Mode()&os.ModeSymlink == 0 || linkErr != nil || target != root || canonicalErr != nil || canonical != root || targetErr != nil || !os.SameFile(sourceInfo, targetInfo) {
		return failure
	}
	_, actual, err := readCodexPluginMetadataFile(path)
	if err != nil || digest != actual {
		return failure
	}
	return nil
}

func createCodexPluginMetadataDirectory(parent *os.Root, home string) (string, error) {
	base := ".multica-plugin-metadata-" + rand.Text()
	if err := parent.Mkdir(base, 0o700); err != nil {
		return "", codexPluginMCPError("temporary cache metadata could not be created")
	}
	return filepath.Join(home, base), nil
}

func codexPluginCacheSourceInfo(binding codexPluginCacheBinding) (os.FileInfo, error) {
	failure := codexPluginMCPError("cache source identity changed")
	info, err := os.Stat(binding.Root)
	if err != nil || !info.IsDir() {
		return nil, failure
	}
	device, inode, supported := codexPluginCacheFileIdentity(info)
	if !supported || device != binding.RootDevice || inode != binding.RootInode {
		return nil, failure
	}
	return info, nil
}

func readCodexPluginCacheNames(ctx context.Context, client codexPluginRequester, home, id string, binding codexPluginCacheBinding) (names []string, resultErr error) {
	failure := codexPluginMCPError("temporary cache metadata could not be verified")
	before, err := snapshotCodexPluginCache(home, id)
	if err != nil || before != binding {
		return nil, failure
	}
	sourceInfo, err := codexPluginCacheSourceInfo(binding)
	if err != nil {
		return nil, failure
	}
	parent, err := os.OpenRoot(home)
	if err != nil {
		return nil, failure
	}
	defer func() {
		if parent.Close() != nil {
			names, resultErr = nil, failure
		}
	}()
	parentInfo, parentErr := parent.Stat(".")
	currentParent, currentErr := os.Lstat(home)
	if parentErr != nil || currentErr != nil || !currentParent.IsDir() || !os.SameFile(parentInfo, currentParent) {
		return nil, failure
	}
	directory, err := createCodexPluginMetadataDirectory(parent, home)
	if err != nil {
		return nil, failure
	}
	base := filepath.Base(directory)
	var opened *os.Root
	defer func() {
		closeErr := error(nil)
		if opened != nil {
			closeErr = opened.Close()
		}
		removeErr := parent.RemoveAll(base)
		_, statErr := parent.Lstat(base)
		if closeErr != nil || removeErr != nil || !os.IsNotExist(statErr) {
			names, resultErr = nil, codexPluginMCPError("temporary cache metadata cleanup failed")
		}
	}()
	created, err := os.Lstat(directory)
	if err != nil || !codexPluginMetadataOwnedDirectory(created) {
		return nil, failure
	}
	opened, err = parent.OpenRoot(base)
	if err != nil {
		return nil, failure
	}
	openedInfo, err := opened.Stat(".")
	if err != nil || !os.SameFile(created, openedInfo) {
		return nil, failure
	}
	market := "multica-local-" + strings.TrimPrefix(filepath.Base(directory), ".multica-plugin-metadata-")
	plugin := strings.Split(id, "@")[0]
	link := filepath.Join(directory, "plugin")
	if opened.Symlink(binding.Root, "plugin") != nil {
		return nil, failure
	}
	manifestDir := filepath.Join(directory, ".agents", "plugins")
	if opened.MkdirAll(filepath.Join(".agents", "plugins"), 0o700) != nil {
		return nil, failure
	}
	generated, err := json.Marshal(map[string]any{
		"name": market, "interface": map[string]any{"displayName": "Local metadata"},
		"plugins": []any{map[string]any{
			"name": plugin, "source": map[string]any{"source": "local", "path": "./plugin"},
			"policy":   map[string]any{"installation": "AVAILABLE", "authentication": "ON_INSTALL"},
			"category": "Productivity",
		}},
	})
	if err != nil {
		return nil, failure
	}
	path := filepath.Join(manifestDir, "marketplace.json")
	if opened.WriteFile(filepath.Join(".agents", "plugins", "marketplace.json"), generated, 0o600) != nil {
		return nil, failure
	}
	digest := sha256.Sum256(generated)
	generatedHash := hex.EncodeToString(digest[:])
	if verifyCodexPluginMetadataBridge(opened, sourceInfo, directory, link, binding.Root, path, generatedHash) != nil {
		return nil, failure
	}
	raw, err := client.request(ctx, "plugin/read", map[string]any{"pluginName": plugin, "marketplacePath": path})
	if err != nil {
		return nil, failure
	}
	after, err := snapshotCodexPluginCache(home, id)
	if err != nil || after != binding || verifyCodexPluginMetadataBridge(opened, sourceInfo, directory, link, binding.Root, path, generatedHash) != nil {
		return nil, failure
	}
	doc, err := decodeCodexPluginObject(raw)
	if err != nil {
		return nil, failure
	}
	detail, _ := doc["plugin"].(map[string]any)
	summary, _ := detail["summary"].(map[string]any)
	temporaryID, name, installed, enabled, valid := codexPluginSummary(summary)
	source, _ := summary["source"].(map[string]any)
	var expectedVersion any
	if binding.ManifestVersion != "" {
		expectedVersion = binding.ManifestVersion
	}
	nativeVersion, versionPresent := summary["localVersion"]
	if !valid || temporaryID != plugin+"@"+market || name != plugin || installed || enabled || detail["marketplaceName"] != market || detail["marketplacePath"] != path || source["type"] != "local" || source["path"] != link || len(source) != 2 || !versionPresent || nativeVersion != expectedVersion || summary["installPolicy"] != "AVAILABLE" || summary["authPolicy"] != "ON_INSTALL" || summary["disabledReason"] != nil {
		return nil, failure
	}
	values, ok := detail["mcpServers"].([]any)
	if !ok || len(values) > 512 {
		return nil, failure
	}
	seen := make(map[string]bool)
	names = make([]string, 0, len(values))
	for _, value := range values {
		name, ok := value.(string)
		if !ok || !codexPluginNameValid(name, 128) || seen[name] {
			return nil, failure
		}
		seen[name] = true
		names = append(names, name)
	}
	sort.Strings(names)
	return names, nil
}

func discoverCodexPluginMCPWithCache(ctx context.Context, client codexPluginRequester, opts ExecOptions) (map[string][]string, error) {
	plan := opts.codexPluginMCP
	// Plans without a private home cannot broaden native discovery (pure callers).
	if plan == nil || plan.home == "" {
		return discoverCodexPluginMCP(ctx, client, opts.Cwd)
	}
	failure := codexPluginMCPError("configured plugin cache inventory could not be verified")
	raw, err := client.request(ctx, "config/read", map[string]any{"cwd": opts.Cwd, "includeLayers": false})
	if err != nil {
		return nil, failure
	}
	doc, err := decodeCodexPluginObject(raw)
	if err != nil {
		return nil, failure
	}
	config, ok := doc["config"].(map[string]any)
	if !ok {
		return nil, failure
	}
	disabled, err := codexPluginsExplicitlyDisabled(config)
	if err != nil || plan.ready && plan.pluginsDisabled != disabled {
		return nil, failure
	}
	if !plan.ready {
		plan.pluginsDisabled = disabled
	}
	// The native plugin RPCs are unavailable when the feature is disabled.
	// Bind that effective state instead of inferring it from an RPC failure.
	if disabled {
		return make(map[string][]string), nil
	}
	inventory, err := discoverCodexPluginMCP(ctx, client, opts.Cwd)
	if err != nil {
		return nil, err
	}
	plugins, err := codexStringMap(config, "plugins")
	if err != nil || len(plugins) > 128 {
		return nil, failure
	}
	bindings := make(map[string]codexPluginCacheBinding)
	for id, value := range plugins {
		if _, discovered := inventory[id]; discovered {
			continue
		}
		plugin, ok := value.(map[string]any)
		if !ok {
			return nil, failure
		}
		enabled, ok := plugin["enabled"].(bool)
		if !ok {
			return nil, failure
		}
		if !enabled {
			continue
		}
		binding, err := snapshotCodexPluginCache(plan.home, id)
		if err != nil {
			return nil, failure
		}
		if plan.ready && plan.cache[id] != binding {
			return nil, failure
		}
		names, err := readCodexPluginCacheNames(ctx, client, plan.home, id, binding)
		if err != nil {
			return nil, err
		}
		bindings[id], inventory[id] = binding, names
	}
	count := 0
	for _, names := range inventory {
		count += len(names)
	}
	if len(inventory) > 128 || count > 512 || plan.ready && !reflect.DeepEqual(bindings, plan.cache) {
		return nil, failure
	}
	if !plan.ready {
		plan.cache = bindings
	}
	return inventory, verifyCodexPluginCacheBindings(plan)
}
