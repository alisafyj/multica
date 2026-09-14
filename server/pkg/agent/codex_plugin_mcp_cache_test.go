//go:build darwin || linux

package agent

import (
	"context"
	"encoding/json"
	"errors"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"

	"github.com/multica-ai/multica/server/internal/agentconfig"
)

func cachePluginFixture(t *testing.T) (string, string) {
	t.Helper()
	home, err := filepath.EvalSymlinks(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	root := filepath.Join(home, "plugins", "cache", "local", "probe", "1.0.0")
	if err := os.MkdirAll(filepath.Join(root, ".codex-plugin"), 0o700); err != nil {
		t.Fatal(err)
	}
	cachePluginWrite(t, filepath.Join(root, ".codex-plugin", "plugin.json"), `{"name":"probe","version":"1.0.0","mcpServers":"./.mcp.json"}`)
	cachePluginWrite(t, filepath.Join(root, ".mcp.json"), `{"raw.server":{"command":"synthetic-command","env":{"TOKEN":"synthetic-cache-secret"}}}`)
	return home, root
}

func cachePluginWrite(t *testing.T, path, value string) {
	t.Helper()
	if err := os.WriteFile(path, []byte(value), 0o600); err != nil {
		t.Fatal(err)
	}
}

func cachePluginReply(t *testing.T, params any) map[string]any {
	t.Helper()
	request := params.(map[string]any)
	path := request["marketplacePath"].(string)
	raw, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	var manifest map[string]any
	if json.Unmarshal(raw, &manifest) != nil {
		t.Fatal("invalid generated marketplace")
	}
	market := manifest["name"].(string)
	plugin := request["pluginName"].(string)
	directory := filepath.Dir(filepath.Dir(filepath.Dir(path)))
	return map[string]any{"plugin": map[string]any{
		"summary": map[string]any{
			"id": plugin + "@" + market, "name": plugin, "installed": false, "enabled": false,
			"source":       map[string]any{"type": "local", "path": filepath.Join(directory, "plugin")},
			"localVersion": "1.0.0", "installPolicy": "AVAILABLE", "authPolicy": "ON_INSTALL",
		},
		"marketplaceName": market, "marketplacePath": path, "mcpServers": []any{"raw.server"},
	}}
}

func assertCachePluginFailure(t *testing.T, err error) {
	t.Helper()
	if !errors.Is(err, agentconfig.ErrRuntimeMCPSelection) {
		t.Fatal("cache failure lost typed selection error")
	}
	if strings.Contains(err.Error(), "synthetic-cache-secret") {
		t.Fatal("cache secret escaped")
	}
}

func assertNoCachePluginMetadata(t *testing.T, home string) {
	t.Helper()
	matches, err := filepath.Glob(filepath.Join(home, ".multica-plugin-metadata-*"))
	if err != nil || len(matches) != 0 {
		t.Fatal("temporary marketplace retained")
	}
}

func TestCodexPluginMCPCacheBindingAndCleanup(t *testing.T) {
	home, _ := cachePluginFixture(t)
	before, err := snapshotCodexPluginCache(home, "probe@local")
	if err != nil {
		t.Fatal(err)
	}
	client := pluginMCPRequesterFunc(func(_ context.Context, method string, params any) (json.RawMessage, error) {
		if method != "plugin/read" {
			t.Fatal("cache bridge started a non-metadata operation")
		}
		reply := cachePluginReply(t, params)
		summary := reply["plugin"].(map[string]any)["summary"].(map[string]any)
		if summary["id"] == "probe@local" {
			t.Fatal("temporary identity reused original identity")
		}
		return json.Marshal(reply)
	})
	names, err := readCodexPluginCacheNames(context.Background(), client, home, "probe@local", before)
	if err != nil || !reflect.DeepEqual(names, []string{"raw.server"}) {
		t.Fatalf("cache bridge rejected safe fixture: %v", err)
	}
	after, err := snapshotCodexPluginCache(home, "probe@local")
	if err != nil || before != after {
		t.Fatal("cache changed")
	}
	assertNoCachePluginMetadata(t, home)
}

func TestCodexPluginMCPCacheRejectsUnsafeInstallations(t *testing.T) {
	for _, variant := range []string{"multiple-versions", "version-link", "manifest-link", "mcp-link", "other-link", "special-file", "agent-plugin", "external-reference", "dynamic-reference", "version-type", "malformed-mcp", "escape-id"} {
		t.Run(variant, func(t *testing.T) {
			home, root := cachePluginFixture(t)
			id := "probe@local"
			link := func(path string) {
				t.Helper()
				if err := os.RemoveAll(path); err != nil {
					t.Fatal(err)
				}
				if err := os.Symlink(t.TempDir(), path); err != nil {
					t.Fatal(err)
				}
			}
			switch variant {
			case "multiple-versions":
				if err := os.Mkdir(filepath.Join(filepath.Dir(root), "2.0.0"), 0o700); err != nil {
					t.Fatal(err)
				}
			case "version-link":
				link(root)
			case "manifest-link":
				link(filepath.Join(root, ".codex-plugin", "plugin.json"))
			case "mcp-link":
				link(filepath.Join(root, ".mcp.json"))
			case "other-link":
				link(filepath.Join(root, "unrelated-link"))
			case "special-file":
				if err := os.Remove(filepath.Join(root, ".mcp.json")); err != nil {
					t.Fatal(err)
				}
				if err := os.Mkdir(filepath.Join(root, ".mcp.json"), 0o700); err != nil {
					t.Fatal(err)
				}
			case "agent-plugin":
				cachePluginWrite(t, filepath.Join(root, "plugin.json"), "{\"$schema\":\"https://agent-plugins.org/schemas/plugin.v1.json\",\"name\":\"probe\"}")
			case "external-reference":
				cachePluginWrite(t, filepath.Join(root, ".codex-plugin", "plugin.json"), `{"name":"probe","version":"1.0.0","mcpServers":"../synthetic-cache-secret"}`)
			case "dynamic-reference":
				cachePluginWrite(t, filepath.Join(root, ".codex-plugin", "plugin.json"), `{"name":"probe","version":"1.0.0","mcpServers":{"dynamic":"synthetic-cache-secret"}}`)
			case "version-type":
				cachePluginWrite(t, filepath.Join(root, ".codex-plugin", "plugin.json"), "{\"name\":\"probe\",\"version\":42}")
			case "malformed-mcp":
				cachePluginWrite(t, filepath.Join(root, ".mcp.json"), "synthetic-cache-secret")
			case "escape-id":
				id = "../probe@local"
			}
			_, err := snapshotCodexPluginCache(home, id)
			assertCachePluginFailure(t, err)
		})
	}
}

func TestCodexPluginMCPCacheRejectsResponseAndFileDrift(t *testing.T) {
	variants := []string{"id", "name", "market", "market-path", "source", "version", "installed", "enabled", "policy", "disabled", "duplicate-names", "invalid-name", "missing-names", "rpc-secret", "manifest-drift", "mcp-drift", "other-file-drift", "generated-drift", "link-chain", "parent-replaced", "parent-mode"}
	for _, variant := range variants {
		t.Run(variant, func(t *testing.T) {
			home, root := cachePluginFixture(t)
			cachePluginWrite(t, filepath.Join(root, "other-input.json"), "{}")
			binding, err := snapshotCodexPluginCache(home, "probe@local")
			if err != nil {
				t.Fatal(err)
			}
			client := pluginMCPRequesterFunc(func(_ context.Context, _ string, params any) (json.RawMessage, error) {
				reply := cachePluginReply(t, params)
				detail := reply["plugin"].(map[string]any)
				summary := detail["summary"].(map[string]any)
				path := params.(map[string]any)["marketplacePath"].(string)
				directory := filepath.Dir(filepath.Dir(filepath.Dir(path)))
				switch variant {
				case "id", "name":
					summary[variant] = "synthetic-cache-secret"
				case "market":
					detail["marketplaceName"] = "other"
				case "market-path":
					detail["marketplacePath"] = "/other"
				case "source":
					summary["source"].(map[string]any)["path"] = root
				case "version":
					summary["localVersion"] = "2.0.0"
				case "installed", "enabled":
					summary[variant] = true
				case "policy":
					summary["installPolicy"] = "NOT_AVAILABLE"
				case "disabled":
					summary["disabledReason"] = "disabled_by_admin"
				case "duplicate-names":
					detail["mcpServers"] = []any{"raw.server", "raw.server"}
				case "invalid-name":
					detail["mcpServers"] = []any{"bad name"}
				case "missing-names":
					delete(detail, "mcpServers")
				case "rpc-secret":
					return nil, errors.New("synthetic-cache-secret")
				case "manifest-drift":
					cachePluginWrite(t, filepath.Join(root, ".codex-plugin", "plugin.json"), `{"name":"probe","version":"1.0.1"}`)
				case "mcp-drift":
					cachePluginWrite(t, filepath.Join(root, ".mcp.json"), "{}")
				case "other-file-drift":
					cachePluginWrite(t, filepath.Join(root, "other-input.json"), `{"changed":true}`)
				case "generated-drift":
					cachePluginWrite(t, path, "{}")
				case "link-chain":
					if err := os.Rename(filepath.Join(directory, "plugin"), filepath.Join(directory, "indirect")); err != nil {
						t.Fatal(err)
					}
					if err := os.Symlink(filepath.Join(directory, "indirect"), filepath.Join(directory, "plugin")); err != nil {
						t.Fatal(err)
					}
				case "parent-replaced":
					raw, _ := os.ReadFile(path)
					old := filepath.Join(home, "replaced-parent")
					if err := os.Rename(directory, old); err != nil {
						t.Fatal(err)
					}
					t.Cleanup(func() { _ = os.RemoveAll(old) })
					if err := os.MkdirAll(filepath.Dir(path), 0o700); err != nil {
						t.Fatal(err)
					}
					cachePluginWrite(t, path, string(raw))
					if err := os.Symlink(root, filepath.Join(directory, "plugin")); err != nil {
						t.Fatal(err)
					}
				case "parent-mode":
					if err := os.Chmod(directory, 0o755); err != nil {
						t.Fatal(err)
					}
				}
				return json.Marshal(reply)
			})
			_, err = readCodexPluginCacheNames(context.Background(), client, home, "probe@local", binding)
			if variant == "other-file-drift" {
				if err != nil {
					t.Fatal("unrelated file outside the names-input closure changed the binding")
				}
			} else {
				assertCachePluginFailure(t, err)
			}
			assertNoCachePluginMetadata(t, home)
		})
	}
}

func TestCodexPluginMCPCacheDirectoryVersionIsNotManifestVersion(t *testing.T) {
	home, old := cachePluginFixture(t)
	root := filepath.Join(filepath.Dir(old), "1e285826")
	if err := os.Rename(old, root); err != nil {
		t.Fatal(err)
	}
	cachePluginWrite(t, filepath.Join(root, ".codex-plugin", "plugin.json"), "{\"name\":\"probe\",\"version\":\"0.1.4\",\"mcpServers\":\"./.mcp.json\"}")
	binding, err := snapshotCodexPluginCache(home, "probe@local")
	if err != nil || binding.DirectoryVersion != "1e285826" || binding.ManifestVersion != "0.1.4" {
		t.Fatal("cache directory identity and native manifest version were conflated")
	}
	for _, localVersion := range []string{"0.1.4", "1e285826"} {
		client := pluginMCPRequesterFunc(func(_ context.Context, _ string, params any) (json.RawMessage, error) {
			reply := cachePluginReply(t, params)
			reply["plugin"].(map[string]any)["summary"].(map[string]any)["localVersion"] = localVersion
			return json.Marshal(reply)
		})
		_, err := readCodexPluginCacheNames(context.Background(), client, home, "probe@local", binding)
		if localVersion == "0.1.4" {
			if err != nil {
				t.Fatal("native manifest version rejected")
			}
		} else {
			assertCachePluginFailure(t, err)
		}
	}
	assertNoCachePluginMetadata(t, home)
}

func TestCodexPluginMCPCacheLargeUnrelatedFileDoesNotEnterDigestClosure(t *testing.T) {
	home, root := cachePluginFixture(t)
	before, err := snapshotCodexPluginCache(home, "probe@local")
	if err != nil {
		t.Fatal(err)
	}
	f, err := os.Create(filepath.Join(root, "large-unrelated-asset"))
	if err != nil {
		t.Fatal(err)
	}
	if err := f.Truncate(96 << 20); err != nil {
		t.Fatal(err)
	}
	if err := f.Close(); err != nil {
		t.Fatal(err)
	}
	after, err := snapshotCodexPluginCache(home, "probe@local")
	if err != nil || before != after {
		t.Fatal("unrelated plugin size affected the names-input closure")
	}
}

func TestCodexPluginMCPCacheLegacyVersionDefaultsAndPriority(t *testing.T) {
	for _, variant := range []string{"missing", "null", "empty", "whitespace", "plus", "codex-first", "claude-first", "cursor-only"} {
		t.Run(variant, func(t *testing.T) {
			home, root := cachePluginFixture(t)
			doc := map[string]any{"name": "probe"}
			wantVersion, wantDirectory := "", ".codex-plugin"
			switch variant {
			case "null":
				doc["version"] = nil
			case "empty":
				doc["version"] = ""
			case "whitespace":
				doc["version"] = "  "
			case "plus":
				doc["version"], wantVersion = " 0.1.4+build.7 ", "0.1.4+build.7"
			case "codex-first", "claude-first", "cursor-only":
				for _, directory := range []string{".codex-plugin", ".claude-plugin", ".cursor-plugin"} {
					if err := os.MkdirAll(filepath.Join(root, directory), 0o700); err != nil {
						t.Fatal(err)
					}
					cachePluginWrite(t, filepath.Join(root, directory, "plugin.json"), "{\"name\":\"probe\",\"version\":\"2.0.0\"}")
				}
				if variant != "codex-first" {
					if err := os.RemoveAll(filepath.Join(root, ".codex-plugin")); err != nil {
						t.Fatal(err)
					}
					wantDirectory = ".claude-plugin"
				}
				if variant == "cursor-only" {
					if err := os.RemoveAll(filepath.Join(root, ".claude-plugin")); err != nil {
						t.Fatal(err)
					}
					wantDirectory = ".cursor-plugin"
				}
			}
			raw, _ := json.Marshal(doc)
			cachePluginWrite(t, filepath.Join(root, wantDirectory, "plugin.json"), string(raw))
			binding, err := snapshotCodexPluginCache(home, "probe@local")
			if err != nil || binding.ManifestVersion != wantVersion || binding.ManifestPath != filepath.Join(root, wantDirectory, "plugin.json") {
				t.Fatal("legacy manifest priority or version contract mismatched")
			}
			client := pluginMCPRequesterFunc(func(_ context.Context, _ string, params any) (json.RawMessage, error) {
				reply := cachePluginReply(t, params)
				var nativeVersion any
				if wantVersion != "" {
					nativeVersion = wantVersion
				}
				reply["plugin"].(map[string]any)["summary"].(map[string]any)["localVersion"] = nativeVersion
				return json.Marshal(reply)
			})
			if _, err := readCodexPluginCacheNames(context.Background(), client, home, "probe@local", binding); err != nil {
				t.Fatal("native legacy version rejected")
			}
			assertNoCachePluginMetadata(t, home)
		})
	}
}

func TestCodexPluginMCPMetadataCreationConfinedToOpenedHome(t *testing.T) {
	root := t.TempDir()
	home, old := filepath.Join(root, "home"), filepath.Join(root, "opened-home")
	if err := os.Mkdir(home, 0o700); err != nil {
		t.Fatal(err)
	}
	parent, err := os.OpenRoot(home)
	if err != nil {
		t.Fatal(err)
	}
	defer parent.Close()
	if err := os.Rename(home, old); err != nil {
		t.Fatal(err)
	}
	if err := os.Mkdir(home, 0o700); err != nil {
		t.Fatal(err)
	}
	directory, err := createCodexPluginMetadataDirectory(parent, home)
	if err != nil {
		t.Fatal("rooted creation failed")
	}
	base := filepath.Base(directory)
	if _, err := parent.Lstat(base); err != nil {
		t.Fatal("metadata directory escaped opened home")
	}
	assertNoCachePluginMetadata(t, home)
	if err := parent.RemoveAll(base); err != nil {
		t.Fatal(err)
	}
	assertNoCachePluginMetadata(t, old)
}

func TestCodexPluginMCPCacheMissingVersionRequiresExplicitNativeNull(t *testing.T) {
	home, root := cachePluginFixture(t)
	cachePluginWrite(t, filepath.Join(root, ".codex-plugin", "plugin.json"), "{\"name\":\"probe\"}")
	binding, err := snapshotCodexPluginCache(home, "probe@local")
	if err != nil {
		t.Fatal(err)
	}
	for _, variant := range []string{"null", "absent", "local", "number"} {
		t.Run(variant, func(t *testing.T) {
			client := pluginMCPRequesterFunc(func(_ context.Context, _ string, params any) (json.RawMessage, error) {
				reply := cachePluginReply(t, params)
				summary := reply["plugin"].(map[string]any)["summary"].(map[string]any)
				switch variant {
				case "null":
					summary["localVersion"] = nil
				case "absent":
					delete(summary, "localVersion")
				case "local":
					summary["localVersion"] = "local"
				case "number":
					summary["localVersion"] = 1
				}
				return json.Marshal(reply)
			})
			names, err := readCodexPluginCacheNames(context.Background(), client, home, "probe@local", binding)
			if variant == "null" {
				if err != nil || len(names) != 1 {
					t.Fatal("verified legacy native null rejected")
				}
			} else {
				assertCachePluginFailure(t, err)
			}
			assertNoCachePluginMetadata(t, home)
		})
	}
}

func TestCodexPluginMCPCacheCleanupFailureDiscardsNames(t *testing.T) {
	if os.Geteuid() == 0 {
		t.Skip("requires unprivileged permissions")
	}
	home, _ := cachePluginFixture(t)
	binding, err := snapshotCodexPluginCache(home, "probe@local")
	if err != nil {
		t.Fatal(err)
	}
	var directory string
	t.Cleanup(func() {
		if directory != "" {
			if err := os.Chmod(directory, 0o700); err != nil {
				t.Error(err)
			}
			if err := os.RemoveAll(directory); err != nil {
				t.Error(err)
			}
		}
	})
	client := pluginMCPRequesterFunc(func(_ context.Context, _ string, params any) (json.RawMessage, error) {
		reply := cachePluginReply(t, params)
		path := params.(map[string]any)["marketplacePath"].(string)
		directory = filepath.Dir(filepath.Dir(filepath.Dir(path)))
		if err := os.Chmod(directory, 0o500); err != nil {
			t.Fatal(err)
		}
		return json.Marshal(reply)
	})
	names, err := readCodexPluginCacheNames(context.Background(), client, home, "probe@local", binding)
	assertCachePluginFailure(t, err)
	if names != nil || !strings.Contains(err.Error(), "cleanup failed") {
		t.Fatal("cleanup failure did not discard metadata with bounded diagnosis")
	}
}

func TestCodexPluginMCPCacheTwoPhaseBinding(t *testing.T) {
	for _, variant := range []string{"stable", "names", "manifest", "mcp", "root", "removed", "higher-priority-manifest"} {
		t.Run(variant, func(t *testing.T) {
			home, root := cachePluginFixture(t)
			if err := os.Rename(filepath.Join(root, ".codex-plugin"), filepath.Join(root, ".claude-plugin")); err != nil {
				t.Fatal(err)
			}
			phase := 0
			client := pluginMCPRequesterFunc(func(_ context.Context, method string, params any) (json.RawMessage, error) {
				switch method {
				case "plugin/installed":
					return json.RawMessage("{\"marketplaces\":[],\"marketplaceLoadErrors\":[]}"), nil
				case "config/read":
					return json.RawMessage("{\"config\":{\"plugins\":{\"probe@local\":{\"enabled\":true,\"mcp_servers\":{\"raw.server\":{\"enabled\":false}}}},\"mcp_servers\":{}}}"), nil
				case "plugin/read":
					reply := cachePluginReply(t, params)
					if phase == 1 && variant == "names" {
						reply["plugin"].(map[string]any)["mcpServers"] = []any{"changed"}
					}
					return json.Marshal(reply)
				default:
					t.Fatal("bridge issued non-metadata operation")
					return nil, nil
				}
			})
			opts := ExecOptions{Cwd: home, McpConfig: json.RawMessage("{\"mcpServers\":{}}"), codexPluginMCP: &codexPluginMCPPlan{home: home}}
			inventory, err := discoverCodexPluginMCPWithCache(context.Background(), client, opts)
			if err != nil || !reflect.DeepEqual(inventory, map[string][]string{"probe@local": {"raw.server"}}) {
				t.Fatal("first phase identity not bound")
			}
			opts.codexPluginMCP.inventory, opts.codexPluginMCP.ready = inventory, true
			phase = 1
			switch variant {
			case "manifest":
				cachePluginWrite(t, filepath.Join(root, ".claude-plugin", "plugin.json"), "{\"name\":\"probe\",\"version\":\"2.0.0\"}")
			case "mcp":
				cachePluginWrite(t, filepath.Join(root, ".mcp.json"), "{}")
			case "root":
				old := filepath.Join(home, "old-root")
				if err := os.Rename(root, old); err != nil {
					t.Fatal(err)
				}
				if err := os.MkdirAll(filepath.Join(root, ".claude-plugin"), 0o700); err != nil {
					t.Fatal(err)
				}
				for _, file := range []string{".claude-plugin/plugin.json", ".mcp.json"} {
					raw, err := os.ReadFile(filepath.Join(old, file))
					if err != nil {
						t.Fatal(err)
					}
					cachePluginWrite(t, filepath.Join(root, file), string(raw))
				}
			case "removed":
				if err := os.RemoveAll(root); err != nil {
					t.Fatal(err)
				}
			case "higher-priority-manifest":
				if err := os.Mkdir(filepath.Join(root, ".codex-plugin"), 0o700); err != nil {
					t.Fatal(err)
				}
				raw, err := os.ReadFile(filepath.Join(root, ".claude-plugin", "plugin.json"))
				if err != nil {
					t.Fatal(err)
				}
				cachePluginWrite(t, filepath.Join(root, ".codex-plugin", "plugin.json"), string(raw))
			}
			err = verifyCodexPluginMCPPolicy(context.Background(), client, opts)
			if variant == "stable" {
				if err != nil {
					t.Fatal("stable two-phase bridge rejected")
				}
			} else {
				assertCachePluginFailure(t, err)
				if variant != "names" {
					assertCachePluginFailure(t, verifyCodexPluginCacheBindings(opts.codexPluginMCP))
				}
			}
			assertNoCachePluginMetadata(t, home)
		})
	}
}

func TestCodexPluginMCPCacheDefaultMCPFileClosure(t *testing.T) {
	home, root := cachePluginFixture(t)
	cachePluginWrite(t, filepath.Join(root, ".codex-plugin", "plugin.json"), "{\"name\":\"probe\",\"version\":\"1.0.0\"}")
	if err := os.Remove(filepath.Join(root, ".mcp.json")); err != nil {
		t.Fatal(err)
	}
	before, err := snapshotCodexPluginCache(home, "probe@local")
	if err != nil || before.MCPPresent {
		t.Fatal("optional MCP absence incorrectly rejected")
	}
	plan := &codexPluginMCPPlan{home: home, cache: map[string]codexPluginCacheBinding{"probe@local": before}}
	cachePluginWrite(t, filepath.Join(root, ".mcp.json"), "{}")
	assertCachePluginFailure(t, verifyCodexPluginCacheBindings(plan))
	after, err := snapshotCodexPluginCache(home, "probe@local")
	if err != nil || !after.MCPPresent || after.MCPHash == "" {
		t.Fatal("default MCP input not hashed")
	}
}

func TestCodexPluginMCPCacheLatestAlias(t *testing.T) {
	for _, variant := range []string{"valid", "broken", "external", "other-name", "chain", "relative", "regular-latest", "regular-sidecar", "ambiguous", "latest-directory", "child-link"} {
		t.Run(variant, func(t *testing.T) {
			home, root := cachePluginFixture(t)
			parent := filepath.Dir(root)
			alias, target := filepath.Join(parent, "latest"), root
			switch variant {
			case "broken":
				target = filepath.Join(parent, "missing")
			case "external":
				target = t.TempDir()
			case "other-name":
				alias = filepath.Join(parent, "current")
			case "chain":
				target = filepath.Join(home, "indirect")
				if err := os.Symlink(root, target); err != nil {
					t.Fatal(err)
				}
			case "relative":
				target = filepath.Base(root)
			case "regular-latest", "regular-sidecar":
				if variant == "regular-sidecar" {
					alias = filepath.Join(parent, "metadata.json")
				}
				cachePluginWrite(t, alias, "{}")
			case "ambiguous":
				if err := os.Mkdir(filepath.Join(parent, "2.0.0"), 0o700); err != nil {
					t.Fatal(err)
				}
			case "latest-directory":
				if err := os.Mkdir(alias, 0o700); err != nil {
					t.Fatal(err)
				}
			case "child-link":
				if err := os.Symlink(t.TempDir(), filepath.Join(root, "child")); err != nil {
					t.Fatal(err)
				}
			}
			if variant != "regular-latest" && variant != "regular-sidecar" && variant != "latest-directory" {
				if err := os.Symlink(target, alias); err != nil {
					t.Fatal(err)
				}
			}
			binding, err := snapshotCodexPluginCache(home, "probe@local")
			if variant != "valid" {
				assertCachePluginFailure(t, err)
				return
			}
			if err != nil || binding.Root != root || binding.DirectoryVersion != "1.0.0" {
				t.Fatal("exact latest alias blocked or selected as a version")
			}
			client := pluginMCPRequesterFunc(func(_ context.Context, _ string, params any) (json.RawMessage, error) {
				return json.Marshal(cachePluginReply(t, params))
			})
			if _, err := readCodexPluginCacheNames(context.Background(), client, home, "probe@local", binding); err != nil {
				t.Fatal("valid latest alias failed native metadata binding")
			}
			assertNoCachePluginMetadata(t, home)
		})
	}
}

func TestCodexPluginMCPCacheLatestAliasDrift(t *testing.T) {
	for _, variant := range []string{"removed", "replaced", "retargeted", "added", "during-read"} {
		t.Run(variant, func(t *testing.T) {
			home, root := cachePluginFixture(t)
			alias := filepath.Join(filepath.Dir(root), "latest")
			if variant != "added" {
				if err := os.Symlink(root, alias); err != nil {
					t.Fatal(err)
				}
			}
			binding, err := snapshotCodexPluginCache(home, "probe@local")
			if err != nil {
				t.Fatal("initial exact alias rejected")
			}
			mutate := func() {
				t.Helper()
				if variant != "added" {
					if err := os.Rename(alias, filepath.Join(home, "previous-alias")); err != nil {
						t.Fatal(err)
					}
				}
				if variant == "replaced" || variant == "added" {
					if err := os.Symlink(root, alias); err != nil {
						t.Fatal(err)
					}
				} else if variant == "retargeted" {
					if err := os.Symlink(t.TempDir(), alias); err != nil {
						t.Fatal(err)
					}
				}
			}
			if variant == "during-read" {
				client := pluginMCPRequesterFunc(func(_ context.Context, _ string, params any) (json.RawMessage, error) {
					reply := cachePluginReply(t, params)
					mutate()
					return json.Marshal(reply)
				})
				names, err := readCodexPluginCacheNames(context.Background(), client, home, "probe@local", binding)
				assertCachePluginFailure(t, err)
				if names != nil {
					t.Fatal("alias drift retained names")
				}
			} else {
				mutate()
				assertCachePluginFailure(t, verifyCodexPluginCacheBindings(&codexPluginMCPPlan{
					home: home, cache: map[string]codexPluginCacheBinding{"probe@local": binding},
				}))
			}
			assertNoCachePluginMetadata(t, home)
		})
	}
}

func TestCodexPluginMCPCacheSourceIdentityBeforeBridge(t *testing.T) {
	for _, variant := range []string{"stable", "device", "inode", "replaced-after-snapshot"} {
		t.Run(variant, func(t *testing.T) {
			home, root := cachePluginFixture(t)
			binding, err := snapshotCodexPluginCache(home, "probe@local")
			if err != nil {
				t.Fatal(err)
			}
			switch variant {
			case "device":
				binding.RootDevice ^= 1
			case "inode":
				binding.RootInode ^= 1
			case "replaced-after-snapshot":
				if err := os.Rename(root, filepath.Join(home, "previous-root")); err != nil {
					t.Fatal(err)
				}
				if err := os.Mkdir(root, 0o700); err != nil {
					t.Fatal(err)
				}
			}
			info, err := codexPluginCacheSourceInfo(binding)
			if variant == "stable" {
				if err != nil || info == nil {
					t.Fatal("matching source identity rejected")
				}
			} else {
				assertCachePluginFailure(t, err)
				if info != nil {
					t.Fatal("unverified identity available to bridge")
				}
			}
			assertNoCachePluginMetadata(t, home)
		})
	}
}
