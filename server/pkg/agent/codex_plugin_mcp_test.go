package agent

import (
	"context"
	"encoding/json"
	"errors"
	"os"
	"path/filepath"
	"reflect"
	"strconv"
	"strings"
	"testing"
	"time"

	"github.com/multica-ai/multica/server/internal/agentconfig"
	"github.com/pelletier/go-toml/v2"
)

type pluginMCPRequesterFunc func(context.Context, string, any) (json.RawMessage, error)

func (fn pluginMCPRequesterFunc) request(ctx context.Context, method string, params any) (json.RawMessage, error) {
	return fn(ctx, method, params)
}

const pluginMCPInstalledJSON = `{"marketplaces":[{"name":"local","path":"/fixture/marketplace.json","plugins":[{"id":"sample@local","name":"sample","enabled":true,"installed":true}]}]}`
const pluginMCPReadJSON = `{"plugin":{"summary":{"id":"sample@local","name":"sample","enabled":true,"installed":true},"marketplaceName":"local","mcpServers":["raw.name"]}}`

func TestCodexPluginMCPInventorySchema(t *testing.T) {
	for _, tc := range []struct{ name, installed, read string }{
		{"no-inventory", `{}`, pluginMCPReadJSON},
		{"null-inventory", `{"marketplaces":null}`, pluginMCPReadJSON},
		{"duplicate-field", `{"marketplaces":[],"marketplaces":[]}`, pluginMCPReadJSON},
		{"wrong-case", `{"Marketplaces":[]}`, pluginMCPReadJSON},
		{"load-error", `{"marketplaces":[],"marketplaceLoadErrors":[{"message":"synthetic-plugin-secret"}]}`, pluginMCPReadJSON},
		{"missing-enabled", strings.Replace(pluginMCPInstalledJSON, `"enabled":true,`, "", 1), pluginMCPReadJSON},
		{"nonbool-enabled", strings.Replace(pluginMCPInstalledJSON, `"enabled":true`, `"enabled":"true"`, 1), pluginMCPReadJSON},
		{"null-installed", strings.Replace(pluginMCPInstalledJSON, `"installed":true`, `"installed":null`, 1), pluginMCPReadJSON},
		{"relative-marketplace", strings.Replace(pluginMCPInstalledJSON, `/fixture/marketplace.json`, `relative.json`, 1), pluginMCPReadJSON},
		{"missing-read", pluginMCPInstalledJSON, `{}`},
		{"no-servers", pluginMCPInstalledJSON, strings.Replace(pluginMCPReadJSON, `,"mcpServers":["raw.name"]`, "", 1)},
		{"null-servers", pluginMCPInstalledJSON, strings.Replace(pluginMCPReadJSON, `["raw.name"]`, `null`, 1)},
		{"duplicate-server", pluginMCPInstalledJSON, strings.Replace(pluginMCPReadJSON, `["raw.name"]`, `["raw.name","raw.name"]`, 1)},
		{"empty-server", pluginMCPInstalledJSON, strings.Replace(pluginMCPReadJSON, `raw.name`, "", 1)},
		{"identity-mismatch", pluginMCPInstalledJSON, strings.Replace(pluginMCPReadJSON, `sample@local`, `other@local`, 1)},
		{"enabled-drift", pluginMCPInstalledJSON, strings.Replace(pluginMCPReadJSON, `"enabled":true`, `"enabled":false`, 1)},
		{"marketplace-mismatch", pluginMCPInstalledJSON, strings.Replace(pluginMCPReadJSON, `"marketplaceName":"local"`, `"marketplaceName":"elsewhere"`, 1)},
		{"oversized-packet", strings.Repeat(" ", 1<<20) + pluginMCPInstalledJSON, pluginMCPReadJSON},
		{"too-many-markets", `{"marketplaces":[` + strings.Repeat(`{"name":"m","plugins":[]},`, 64) + `{"name":"m","plugins":[]}]}`, pluginMCPReadJSON},
		{"too-many-servers", pluginMCPInstalledJSON, strings.Replace(pluginMCPReadJSON, `["raw.name"]`, `["`+strings.Join(makePluginMCPNames(513), `","`)+`"]`, 1)},
	} {
		t.Run(tc.name, func(t *testing.T) {
			client := pluginMCPRequesterFunc(func(_ context.Context, method string, _ any) (json.RawMessage, error) {
				if method == "plugin/installed" {
					return json.RawMessage(tc.installed), nil
				}
				return json.RawMessage(tc.read), nil
			})
			_, err := discoverCodexPluginMCP(context.Background(), client, "/fixture")
			if !errors.Is(err, agentconfig.ErrRuntimeMCPSelection) {
				t.Fatal("incomplete inventory accepted")
			}
			if strings.Contains(err.Error(), "synthetic-plugin-secret") {
				t.Fatal("raw error escaped")
			}
		})
	}
}

func makePluginMCPNames(count int) []string {
	names := make([]string, count)
	for i := range names {
		names[i] = "server" + strconv.Itoa(i)
	}
	return names
}

func TestCodexPluginMCPInventoryDisabledAndRemote(t *testing.T) {
	for _, installed := range []bool{false, true} {
		calls := 0
		client := pluginMCPRequesterFunc(func(_ context.Context, method string, params any) (json.RawMessage, error) {
			calls++
			if method == "plugin/installed" {
				raw := strings.ReplaceAll(pluginMCPInstalledJSON, `"enabled":true`, `"enabled":false`)
				raw = strings.ReplaceAll(raw, `"path":"/fixture/marketplace.json"`, `"path":null`)
				if !installed {
					raw = strings.ReplaceAll(raw, `"installed":true`, `"installed":false`)
				}
				return json.RawMessage(raw), nil
			}
			p := params.(map[string]any)
			if p["remoteMarketplaceName"] != "local" || p["pluginName"] != "sample" || p["marketplacePath"] != nil {
				t.Fatal("remote inventory identity lost")
			}
			return json.RawMessage(strings.ReplaceAll(pluginMCPReadJSON, `"enabled":true`, `"enabled":false`)), nil
		})
		inventory, err := discoverCodexPluginMCP(context.Background(), client, "/fixture")
		if err != nil {
			t.Fatal(err)
		}
		if installed && (calls != 2 || !reflect.DeepEqual(inventory["sample@local"], []string{"raw.name"})) {
			t.Fatal("installed-but-disabled plugin lost")
		}
		if !installed && (calls != 1 || len(inventory) != 0) {
			t.Fatal("uninstalled plugin was read")
		}
	}
}

func TestCodexPluginMCPInventoryCancellation(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), time.Millisecond)
	defer cancel()
	client := pluginMCPRequesterFunc(func(ctx context.Context, _ string, _ any) (json.RawMessage, error) {
		<-ctx.Done()
		return nil, errors.New("synthetic-plugin-secret")
	})
	_, err := discoverCodexPluginMCP(ctx, client, "/fixture")
	if !errors.Is(err, agentconfig.ErrRuntimeMCPSelection) || strings.Contains(err.Error(), "synthetic-plugin-secret") {
		t.Fatal("cancellation did not return safe failure")
	}
}

func TestCodexPluginMCPPrivateConfigPreservesPluginAndOverlay(t *testing.T) {
	home := t.TempDir()
	path := filepath.Join(home, "config.toml")
	original := []byte("model = 'unchanged'\n[model_providers.custom]\nexperimental_bearer_token = 'synthetic-plugin-secret'\n[plugins.'sample@local']\nenabled = true\n[plugins.'sample@local'.mcp_servers.'raw.name']\nenabled = true\n[mcp_servers.ordinary]\ncommand = 'retained'\n")
	if err := os.WriteFile(path, original, 0o600); err != nil {
		t.Fatal(err)
	}
	snapshot := filepath.Join(home, "original-hardlink")
	if err := os.Link(path, snapshot); err != nil {
		t.Fatal(err)
	}
	cache := filepath.Join(home, "plugins", "cache", "skill")
	if err := os.MkdirAll(filepath.Dir(cache), 0o700); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(cache, []byte("unchanged"), 0o600); err != nil {
		t.Fatal(err)
	}
	if err := writeCodexPluginMCPPolicy(home, map[string][]string{"sample@local": {"raw.name"}}); err != nil {
		t.Fatal(err)
	}
	data, _ := os.ReadFile(path)
	var config map[string]any
	if toml.Unmarshal(data, &config) != nil {
		t.Fatal("invalid TOML")
	}
	for _, assertion := range []struct {
		path []string
		want any
	}{
		{[]string{"plugins", "sample@local", "enabled"}, true},
		{[]string{"plugins", "sample@local", "mcp_servers", "raw.name", "enabled"}, false},
		{[]string{"mcp_servers", "ordinary", "command"}, "retained"},
		{[]string{"model_providers", "custom", "experimental_bearer_token"}, "synthetic-plugin-secret"},
	} {
		got, ok := nestedCodexConfigValue(config, assertion.path)
		if !ok || got != assertion.want {
			t.Fatal("unrelated private configuration changed")
		}
	}
	if data, _ := os.ReadFile(snapshot); string(data) != string(original) {
		t.Fatal("source hardlink mutated")
	}
	if data, _ := os.ReadFile(cache); string(data) != "unchanged" {
		t.Fatal("plugin cache changed")
	}
}

func TestCodexPluginMCPPrivateConfigRejectsUnsafeFiles(t *testing.T) {
	for _, kind := range []string{"symlink", "malformed", "non-table"} {
		t.Run(kind, func(t *testing.T) {
			home := t.TempDir()
			path := filepath.Join(home, "config.toml")
			switch kind {
			case "symlink":
				target := filepath.Join(t.TempDir(), "config.toml")
				if err := os.WriteFile(target, []byte("plugins = {}"), 0o600); err != nil {
					t.Fatal(err)
				}
				if err := os.Symlink(target, path); err != nil {
					t.Skip("symlinks unavailable")
				}
			case "malformed":
				if err := os.WriteFile(path, []byte("synthetic-plugin-secret[["), 0o600); err != nil {
					t.Fatal(err)
				}
			case "non-table":
				if err := os.WriteFile(path, []byte("plugins = 'synthetic-plugin-secret'"), 0o600); err != nil {
					t.Fatal(err)
				}
			}
			err := writeCodexPluginMCPPolicy(home, map[string][]string{"sample@local": {"raw.name"}})
			if !errors.Is(err, agentconfig.ErrRuntimeMCPSelection) || strings.Contains(err.Error(), "synthetic-plugin-secret") {
				t.Fatal("unsafe file did not fail safely")
			}
		})
	}
}

func TestCodexPluginMCPInventoryDriftAndQuotedOverrides(t *testing.T) {
	plan := &codexPluginMCPPlan{ready: true, inventory: map[string][]string{
		`p."quoted"@local`: {`raw."name"\path`},
		"other@local":      {`raw."name"\path`},
	}}
	for _, assignment := range plan.launchOverrides()[1:] {
		var config map[string]any
		if err := toml.Unmarshal([]byte(assignment), &config); err != nil {
			t.Fatal("invalid escaped override")
		}
		plugins := config["plugins"].(map[string]any)
		for id := range plugins {
			if value, ok := nestedCodexConfigValue(config, []string{"plugins", id, "mcp_servers", `raw."name"\path`, "enabled"}); !ok || value != false {
				t.Fatal("raw name was reinterpreted as a dotted key")
			}
		}
	}
	client := pluginMCPRequesterFunc(func(_ context.Context, method string, _ any) (json.RawMessage, error) {
		if method == "plugin/installed" {
			return json.RawMessage(pluginMCPInstalledJSON), nil
		}
		if method == "plugin/read" {
			return json.RawMessage(pluginMCPReadJSON), nil
		}
		t.Fatal("drifted inventory reached config verification")
		return nil, nil
	})
	err := verifyCodexPluginMCPPolicy(context.Background(), client, ExecOptions{Cwd: "/fixture", codexPluginMCP: plan})
	if !errors.Is(err, agentconfig.ErrRuntimeMCPSelection) {
		t.Fatal("inventory drift accepted")
	}
}
