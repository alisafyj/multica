//go:build darwin

package execenv

import (
	"bufio"
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"syscall"
	"testing"
	"time"
)

func TestCodexPluginSkillPolicyNative(t *testing.T) {
	binary := os.Getenv("MULTICA_TEST_CODEX_PLUGIN_BINARY")
	if binary == "" {
		t.Skip("set MULTICA_TEST_CODEX_PLUGIN_BINARY to opt into the no-model native test")
	}
	data, err := os.ReadFile(binary)
	if err != nil {
		t.Fatal(err)
	}
	digest := sha256.Sum256(data)
	if !filepath.IsAbs(binary) || hex.EncodeToString(digest[:]) != "b973d440acac501fd2594a43e7ca9ce41e0a65b9dfb28d0d7a7837c99e1261e3" {
		t.Fatal("native fixture requires the pinned Codex 0.153.4 binary")
	}
	for _, skillsRoot := range []string{"./skills/", "./skills/folder"} {
		t.Run(skillsRoot, func(t *testing.T) {
			root := t.TempDir()
			home, shared, private := filepath.Join(root, "home"), filepath.Join(root, "shared"), filepath.Join(root, "private")
			workdir, market := filepath.Join(root, "workdir"), filepath.Join(root, "marketplace")
			put := func(path, value string) {
				t.Helper()
				if err := os.MkdirAll(filepath.Dir(path), 0o700); err != nil {
					t.Fatal(err)
				}
				if err := os.WriteFile(path, []byte(value), 0o600); err != nil {
					t.Fatal(err)
				}
			}
			for _, path := range []string{home, workdir, filepath.Join(private, "plugins"), filepath.Join(root, "tmp")} {
				if err := os.MkdirAll(path, 0o700); err != nil {
					t.Fatal(err)
				}
			}
			put(filepath.Join(shared, "config.toml"), "model = 'synthetic-no-model'\napproval_policy = 'never'\nsandbox_mode = 'read-only'\n[features]\nplugins = true\n")
			put(filepath.Join(market, ".agents", "plugins", "marketplace.json"), `{"name":"local","plugins":[{"name":"probe","source":{"source":"local","path":"./plugins/probe"}}]}`)
			manifest, err := json.Marshal(map[string]string{"name": "probe", "version": "1.0.0", "skills": skillsRoot})
			if err != nil {
				t.Fatal(err)
			}
			put(filepath.Join(market, "plugins", "probe", ".codex-plugin", "plugin.json"), string(manifest))
			put(filepath.Join(market, "plugins", "probe", "skills", "folder", "SKILL.md"), "---\nname: native-name\ndescription: Synthetic no-model policy fixture\n---\n")
			put(filepath.Join(market, "plugins", "probe", "skills", "folder", "nested", "SKILL.md"), "---\nname: nested\ndescription: Nested no-model policy fixture\n---\n")
			env := []string{"PATH=/usr/bin:/bin:/usr/sbin:/sbin", "HOME=" + home, "TMPDIR=" + filepath.Join(root, "tmp"), "RUST_LOG=off"}
			newCommand := func(codexHome string, args ...string) (*exec.Cmd, context.CancelFunc) {
				t.Helper()
				ctx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
				cmd := exec.CommandContext(ctx, "/usr/bin/sandbox-exec", append([]string{"-p", "(version 1)(allow default)(deny network*)", binary}, args...)...)
				cmd.Dir, cmd.Env = workdir, append(append([]string{}, env...), "CODEX_HOME="+codexHome)
				cmd.SysProcAttr = &syscall.SysProcAttr{Setpgid: true}
				cmd.Cancel = func() error { return syscall.Kill(-cmd.Process.Pid, syscall.SIGKILL) }
				cmd.WaitDelay, cmd.Stderr = time.Second, io.Discard
				return cmd, cancel
			}
			checkExited := func(cmd *exec.Cmd) {
				t.Helper()
				// Children may be reaped just after their parent; inspect without signaling.
				for attempt := 0; attempt < 40; attempt++ {
					if err := syscall.Kill(-cmd.Process.Pid, 0); errors.Is(err, syscall.ESRCH) {
						return
					} else if err != nil {
						t.Fatal("native fixture process group could not be inspected")
					}
					time.Sleep(25 * time.Millisecond)
				}
				t.Fatal("native fixture process group did not exit")
			}
			for _, args := range [][]string{
				{"plugin", "marketplace", "add", market, "--json"},
				{"plugin", "add", "probe", "--marketplace", "local", "--json"},
			} {
				cmd, cancel := newCommand(shared, args...)
				err := cmd.Run()
				cancel()
				if err != nil {
					t.Fatalf("native fixture installation: %v", err)
				}
				checkExited(cmd)
			}
			sharedConfig, err := os.ReadFile(filepath.Join(shared, "config.toml"))
			if err != nil {
				t.Fatal(err)
			}
			if err := os.Rename(market, market+"-hidden"); err != nil {
				t.Fatal(err)
			}
			if err := os.Symlink(filepath.Join(shared, "plugins", "cache"), filepath.Join(private, "plugins", "cache")); err != nil {
				t.Fatal(err)
			}
			configPath := filepath.Join(private, "config.toml")
			put(configPath, string(sharedConfig))
			observe := func(wantEnabled bool) {
				t.Helper()
				cmd, cancel := newCommand(private, "app-server", "--listen", "stdio://")
				defer cancel()
				input, err := cmd.StdinPipe()
				if err != nil {
					t.Fatal(err)
				}
				output, err := cmd.StdoutPipe()
				if err != nil {
					t.Fatal(err)
				}
				if err := cmd.Start(); err != nil {
					t.Fatal(err)
				}
				defer func() {
					_ = input.Close()
					if err := cmd.Wait(); err != nil {
						t.Errorf("native fixture cleanup: %v", err)
					}
					checkExited(cmd)
				}()
				scanner := bufio.NewScanner(output)
				scanner.Buffer(make([]byte, 4096), 1<<20)
				request := func(id int, method string, params any) json.RawMessage {
					t.Helper()
					if method != "initialize" && method != "skills/list" {
						t.Fatal("native fixture forbids model and thread requests")
					}
					if err := json.NewEncoder(input).Encode(map[string]any{"jsonrpc": "2.0", "id": id, "method": method, "params": params}); err != nil {
						t.Fatal(err)
					}
					for scanner.Scan() {
						var response struct {
							ID     int             `json:"id"`
							Result json.RawMessage `json:"result"`
							Error  json.RawMessage `json:"error"`
						}
						if json.Unmarshal(scanner.Bytes(), &response) != nil {
							t.Fatal("invalid native response")
						}
						if response.ID != id {
							continue
						}
						if len(response.Error) != 0 {
							t.Fatal("native request returned an error")
						}
						return response.Result
					}
					t.Fatalf("native response missing: %v", scanner.Err())
					return nil
				}
				request(1, "initialize", map[string]any{"clientInfo": map[string]string{"name": "multica-skill-policy-test", "version": "1"}, "capabilities": map[string]bool{"experimentalApi": true}})
				if err := json.NewEncoder(input).Encode(map[string]any{"jsonrpc": "2.0", "method": "initialized"}); err != nil {
					t.Fatal(err)
				}
				raw := request(2, "skills/list", map[string]any{"cwds": []string{workdir}, "forceReload": true})
				var result struct {
					Data []struct {
						Errors []json.RawMessage `json:"errors"`
						Skills []struct {
							PluginID string `json:"pluginId"`
							Name     string `json:"name"`
							Enabled  *bool  `json:"enabled"`
						} `json:"skills"`
					} `json:"data"`
				}
				if json.Unmarshal(raw, &result) != nil || len(result.Data) != 1 || len(result.Data[0].Errors) != 0 {
					t.Fatal("native skill listing is incomplete")
				}
				found := make(map[string]bool)
				for _, skill := range result.Data[0].Skills {
					if skill.PluginID == "probe@local" {
						if found[skill.Name] {
							t.Fatal("duplicated native plugin skill")
						}
						found[skill.Name] = true
						expected := wantEnabled
						if skill.Name == "probe:nested" {
							expected = true
						} else if skill.Name != "probe:native-name" {
							t.Fatal("unexpected native plugin skill")
						}
						if skill.Enabled == nil || *skill.Enabled != expected {
							t.Fatal("product-generated policy disagrees with native skill metadata")
						}
					}
				}
				if len(found) != 2 {
					t.Fatal("native plugin skill missing or duplicated")
				}
			}
			observe(true)
			bindings, err := ensureCodexDisabledSkillsConfig(configPath, private, []RuntimeSkillRefForEnv{{Root: "plugin", Plugin: "probe@local", Key: "probe@local:skills/folder", Name: "unused-client-name"}}, nil)
			if err != nil {
				t.Fatal(err)
			}
			if len(bindings) != 1 || bindings[0].Key != "probe@local:skills/folder" || bindings[0].InstallationDigest == "" {
				t.Fatal("native policy was written without its installation binding")
			}
			observe(false)
			put(configPath, string(sharedConfig))
			if _, err := ensureCodexDisabledSkillsConfig(configPath, private, nil, nil); err != nil {
				t.Fatal(err)
			}
			observe(true)
			after, err := os.ReadFile(filepath.Join(shared, "config.toml"))
			if err != nil || string(after) != string(sharedConfig) {
				t.Fatal("task policy changed the shared configuration")
			}
		})
	}
}
