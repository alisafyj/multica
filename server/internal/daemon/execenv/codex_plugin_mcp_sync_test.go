package execenv

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/pelletier/go-toml/v2"
)

func TestCodexPluginMCPConfigSyncFailureRejectsStalePrivatePolicy(t *testing.T) {
	shared, task := t.TempDir(), t.TempDir()
	t.Setenv("CODEX_HOME", shared)
	t.Setenv("HOME", t.TempDir())
	source := filepath.Join(shared, "config.toml")
	if err := os.Symlink("config.toml", source); err != nil {
		t.Skip("symlinks unavailable")
	}
	stale := []byte("[plugins.'sample@local'.mcp_servers.tool]\nenabled = false\n")
	if err := os.WriteFile(filepath.Join(task, "config.toml"), stale, 0o600); err != nil {
		t.Fatal(err)
	}
	err := prepareCodexHomeWithOpts(task, CodexHomeOptions{CodexVersion: "0.153.4"}, testLogger())
	if err == nil || !strings.Contains(err.Error(), "config.toml") {
		t.Fatal("config sync failure silently retained stale plugin policy")
	}
}

func TestCodexPluginMCPConfigSyncInheritDenyInherit(t *testing.T) {
	shared, task := t.TempDir(), t.TempDir()
	t.Setenv("CODEX_HOME", shared)
	t.Setenv("HOME", t.TempDir())
	source := filepath.Join(shared, "config.toml")
	original := []byte("[plugins.'sample@local']\nenabled = true\n")
	if err := os.WriteFile(source, original, 0o600); err != nil {
		t.Fatal(err)
	}
	for _, mode := range []string{"inherit", "deny_all", "inherit"} {
		if err := prepareCodexHomeWithOpts(task, CodexHomeOptions{CodexVersion: "0.153.4"}, testLogger()); err != nil {
			t.Fatal(err)
		}
		path := filepath.Join(task, "config.toml")
		data, _ := os.ReadFile(path)
		var config map[string]any
		if toml.Unmarshal(data, &config) != nil {
			t.Fatal("invalid private config")
		}
		plugin := config["plugins"].(map[string]any)["sample@local"].(map[string]any)
		if _, present := plugin["mcp_servers"]; present {
			t.Fatal("prior deny_all persisted through successful reuse sync")
		}
		if mode == "deny_all" {
			plugin["mcp_servers"] = map[string]any{"tool": map[string]any{"enabled": false}}
			updated, _ := toml.Marshal(config)
			if err := os.WriteFile(path, updated, 0o600); err != nil {
				t.Fatal(err)
			}
		}
	}
	if data, _ := os.ReadFile(source); string(data) != string(original) {
		t.Fatal("shared config changed")
	}
}
