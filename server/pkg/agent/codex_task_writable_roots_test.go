package agent

import (
	"bytes"
	"context"
	"encoding/json"
	"log/slog"
	"os"
	"path/filepath"
	"reflect"
	"runtime"
	"strings"
	"testing"
	"time"
)

func TestCodexTaskWritablePolicyBackendFailureNeverStartsTurn(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("POSIX-only subprocess fixture")
	}
	for _, prior := range []string{"", "old-thread"} {
		for _, response := range []string{`{"thread":{"id":"unverified"}}`, `{"thread":{"id":"unverified"},"approvalPolicy":"on-request","sandbox":{"type":"dangerFullAccess"}}`} {
			t.Run(prior+response, func(t *testing.T) {
				fake := writeFakeCodexAppServer(t, `DIR="$(dirname "$0")"`+"\n"+
					`echo started >> "$DIR/attempts"`+"\n"+
					`read line`+"\n"+
					`echo '{"jsonrpc":"2.0","id":1,"result":{}}'`+"\n"+
					`read line`+"\n"+
					`read line`+"\n"+
					`echo '{"jsonrpc":"2.0","id":2,"result":{"config":{"approval_policy":"never","sandbox_mode":"workspace-write"}}}'`+"\n"+
					`read line`+"\n"+
					`echo '{"jsonrpc":"2.0","id":3,"result":`+response+`}'`+"\n"+
					`while read line; do printf '%s\n' "$line" >> "$DIR/after-rejection"; done`+"\n")
				var logs bytes.Buffer
				logger := slog.New(slog.NewJSONHandler(&logs, nil))
				backend, err := New("codex", Config{ExecutablePath: fake, Logger: logger, Env: map[string]string{"CODEX_HOME": t.TempDir()}})
				if err != nil {
					t.Fatal(err)
				}
				ctx, cancel := context.WithTimeout(t.Context(), 5*time.Second)
				defer cancel()
				session, err := backend.Execute(ctx, "private fixed fixture", ExecOptions{Cwd: t.TempDir(), ResumeSessionID: prior, CodexTaskWritableRoots: []string{t.TempDir()}, Timeout: 3 * time.Second, HandshakeTimeout: time.Second})
				if err != nil {
					t.Fatal(err)
				}
				for range session.Messages {
				}
				result := <-session.Result
				if result.Status != "failed" || !strings.Contains(result.Error, "did not confirm") {
					t.Fatalf("wrong failure: %+v", result)
				}
				attempts, err := os.ReadFile(filepath.Join(filepath.Dir(fake), "attempts"))
				if err != nil || string(attempts) != "started\n" {
					t.Fatal("authorization failure retried process")
				}
				if _, err := os.Stat(filepath.Join(filepath.Dir(fake), "after-rejection")); !os.IsNotExist(err) {
					t.Fatal("authorization failure sent another RPC or started a turn")
				}
				cleanup := findCodexLifecyclePhase(t, parseJSONLogEntries(t, logs.String()), "cleanup")
				failurePhase := "thread_start_failure"
				if prior != "" {
					failurePhase = "thread_resume_failure"
				}
				failure := findCodexLifecyclePhase(t, parseJSONLogEntries(t, logs.String()), failurePhase)
				if cleanup["reaped"] != true || failure["cleanup_confirmed"] != true {
					t.Fatal("failed task process cleanup was not confirmed")
				}
			})
		}
	}
}

func TestCodexTaskWritablePolicy(t *testing.T) {
	root := t.TempDir()
	cache, existing := filepath.Join(root, "cache"), filepath.Join(root, "existing")
	base := func() map[string]any {
		return map[string]any{"approval_policy": "never", "sandbox_mode": "workspace-write", "sandbox_workspace_write": map[string]any{
			"writable_roots": []any{existing}, "network_access": true, "exclude_tmpdir_env_var": true, "exclude_slash_tmp": true,
		}}
	}
	t.Run("merge preserves selected settings and existing RPC configuration", func(t *testing.T) {
		config := base()
		before, _ := json.Marshal(config)
		policy, err := resolveCodexTaskWritablePolicy(config, []string{cache, cache})
		if err != nil || policy == nil {
			t.Fatalf("missing policy: %v", err)
		}
		params := map[string]any{"config": map[string]any{"model_reasoning_effort": "medium", "plugins": map[string]any{"fixture": true}}}
		applyCodexTaskWritablePolicy(params, policy)
		if params["approvalPolicy"] != "never" || params["sandbox"] != "workspace-write" {
			t.Fatal("resume must bind current authorization")
		}
		cfg := params["config"].(map[string]any)
		if cfg["model_reasoning_effort"] != "medium" || cfg["plugins"] == nil {
			t.Fatal("existing RPC configuration replaced")
		}
		workspace := cfg["sandbox_workspace_write"].(map[string]any)
		if !reflect.DeepEqual(workspace["writable_roots"], []string{existing, cache}) || workspace["network_access"] != true || workspace["exclude_tmpdir_env_var"] != true || workspace["exclude_slash_tmp"] != true {
			t.Fatalf("unexpected policy: %#v", workspace)
		}
		after, _ := json.Marshal(config)
		if string(before) != string(after) {
			t.Fatal("effective configuration mutated")
		}
	})
	for _, mode := range []string{"danger-full-access", "read-only", ""} {
		t.Run("unchanged "+mode, func(t *testing.T) {
			config := base()
			config["sandbox_mode"] = mode
			p, err := resolveCodexTaskWritablePolicy(config, []string{cache})
			if err != nil || p != nil {
				t.Fatalf("non-selected policy changed: %v", err)
			}
		})
	}
	t.Run("on-request unchanged", func(t *testing.T) {
		config := base()
		config["approval_policy"] = "on-request"
		p, err := resolveCodexTaskWritablePolicy(config, []string{cache})
		if err != nil || p != nil {
			t.Fatal("approval policy changed")
		}
	})
	t.Run("no setup unchanged", func(t *testing.T) {
		p, err := resolveCodexTaskWritablePolicy(base(), nil)
		if err != nil || p != nil {
			t.Fatal("zero roots must be a no-op")
		}
	})
	for _, field := range []string{"permissions", "default_permissions", "sandbox_permissions"} {
		t.Run("reject ambiguous "+field, func(t *testing.T) {
			config := base()
			config[field] = "selected-profile"
			if _, err := resolveCodexTaskWritablePolicy(config, []string{cache}); err == nil {
				t.Fatal("ambiguous permissions accepted")
			}
		})
	}
	for _, field := range []string{"writable_roots", "network_access", "exclude_tmpdir_env_var", "exclude_slash_tmp"} {
		t.Run("reject malformed "+field, func(t *testing.T) {
			config := base()
			config["sandbox_workspace_write"].(map[string]any)[field] = 42
			if _, err := resolveCodexTaskWritablePolicy(config, []string{cache}); err == nil {
				t.Fatal("malformed sandbox field accepted")
			}
		})
	}
	t.Run("reject relative grant", func(t *testing.T) {
		if _, err := resolveCodexTaskWritablePolicy(base(), []string{"../cache"}); err == nil {
			t.Fatal("relative grant accepted")
		}
	})
}

func TestCodexTaskWritablePolicyRejectsThreadWithoutAuthorization(t *testing.T) {
	for _, prior := range []string{"", "historical-thread"} {
		for _, response := range []string{`{"thread":{"id":"unsafe"}}`, `{"thread":{"id":"unsafe"},"approvalPolicy":"on-request","sandbox":{"type":"dangerFullAccess"}}`} {
			t.Run(prior+response, func(t *testing.T) {
				client, stdin, _ := newTestCodexClient(t)
				method := "thread/start"
				if prior != "" {
					method = "thread/resume"
				}
				wait := drainRPCScript(t, client, stdin, []rpcResponse{
					{method: "config/read", result: json.RawMessage(`{"config":{"approval_policy":"never","sandbox_mode":"workspace-write"}}`)},
					{method: method, result: json.RawMessage(response)},
				})
				defer wait()
				ctx, cancel := context.WithTimeout(t.Context(), 200*time.Millisecond)
				defer cancel()
				thread, resumed, err := client.startOrResumeThread(ctx, ExecOptions{Cwd: t.TempDir(), ResumeSessionID: prior, CodexTaskWritableRoots: []string{t.TempDir()}}, slog.Default())
				if err == nil || thread != "" || resumed {
					t.Fatal("unverified authorization accepted")
				}
				if len(stdin.Lines()) != 2 {
					t.Fatal("authorization rejection must not fall back or issue another request")
				}
			})
		}
	}
}

func TestCodexTaskWritablePolicyResponse(t *testing.T) {
	root := t.TempDir()
	policy := &codexTaskWritablePolicy{workspace: map[string]any{"writable_roots": []string{root}, "network_access": true, "exclude_tmpdir_env_var": false, "exclude_slash_tmp": false}}
	response := func() map[string]any {
		return map[string]any{"approvalPolicy": "never", "sandbox": map[string]any{"type": "workspaceWrite", "writableRoots": []string{root}, "networkAccess": true, "excludeTmpdirEnvVar": false, "excludeSlashTmp": false}}
	}
	for _, mutation := range []string{"none", "missing", "approval", "type", "roots", "network", "tmp", "missing_network"} {
		t.Run(mutation, func(t *testing.T) {
			value := response()
			sb := value["sandbox"].(map[string]any)
			switch mutation {
			case "missing":
				delete(value, "sandbox")
			case "approval":
				value["approvalPolicy"] = "on-request"
			case "type":
				sb["type"] = "dangerFullAccess"
			case "roots":
				sb["writableRoots"] = []string{filepath.Dir(root)}
			case "network":
				sb["networkAccess"] = false
			case "tmp":
				sb["excludeSlashTmp"] = true
			case "missing_network":
				delete(sb, "networkAccess")
			}
			raw, _ := json.Marshal(value)
			if err := verifyCodexTaskWritablePolicy(raw, policy); (err == nil) != (mutation == "none") {
				t.Fatalf("verification err=%v for %s", err, mutation)
			}
		})
	}
	if err := verifyCodexTaskWritablePolicy(nil, nil); err != nil {
		t.Fatal(err)
	}
}
