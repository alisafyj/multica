package agent

import (
	"bytes"
	"context"
	"encoding/json"
	"log/slog"
	"path/filepath"
	"regexp"
	"strings"
	"testing"
)

func TestCodexThreadConfigurationProjectionWhitelistsNativeSetupResponse(t *testing.T) {
	t.Parallel()

	rootA := filepath.Join(t.TempDir(), "alpha")
	rootB := filepath.Join(t.TempDir(), "beta")
	projection := projectCodexThreadConfiguration(map[string]any{
		"model":           "private-model",
		"modelProvider":   "private-provider",
		"reasoningEffort": "high",
		"serviceTier":     "priority",
		"approvalPolicy":  "never",
		"sandbox": map[string]any{
			"type":                "workspaceWrite",
			"networkAccess":       true,
			"excludeTmpdirEnvVar": false,
			"excludeSlashTmp":     true,
			"writableRoots":       []any{rootB, rootA, rootB},
			"privateSandboxKey":   "must-not-appear",
		},
		"thread":                map[string]any{"id": "raw-thread", "instructions": "secret-instructions"},
		"mcpServers":            map[string]any{"private": map[string]any{"url": "https://secret.invalid"}},
		"auth":                  map[string]any{"token": "secret-token"},
		"privateArbitraryField": "secret-arbitrary",
	})
	configProjection := projectCodexConfiguration(map[string]any{
		"model": "private-model", "model_provider": "private-provider",
	})
	if projection.Model != configProjection.Model || projection.ModelProvider != configProjection.ModelProvider {
		t.Fatalf("thread and config identifier digests differ: thread=%#v/%#v config=%#v/%#v",
			projection.Model, projection.ModelProvider, configProjection.Model, configProjection.ModelProvider)
	}

	raw, err := json.Marshal(projection)
	if err != nil {
		t.Fatal(err)
	}
	serialized := string(raw)
	for _, forbidden := range []string{
		"private-model", "private-provider", "raw-thread", "secret-instructions",
		"privateSandboxKey", "must-not-appear", "mcpServers", "secret.invalid",
		"secret-token", "privateArbitraryField", "secret-arbitrary",
	} {
		if strings.Contains(serialized, forbidden) {
			t.Fatalf("thread projection leaked %q: %s", forbidden, serialized)
		}
	}

	var got map[string]any
	if err := json.Unmarshal(raw, &got); err != nil {
		t.Fatal(err)
	}
	if got["schema"] != "codex_thread_configuration_projection/v1" ||
		got["scope"] != "native_thread_setup_response_only" || got["completeness"] != "partial" {
		t.Fatalf("thread projection contract fields = %#v", got)
	}
	wantNotProven := []any{
		"turn_overrides", "provider_reported_model", "provider_reported_effort", "complete_configuration",
	}
	if actual := got["not_proven"].([]any); !equalCodexProjectionValues(actual, wantNotProven) {
		t.Fatalf("not_proven = %v, want fixed order %v", actual, wantNotProven)
	}
	wantKeys := []string{
		"schema", "scope", "completeness", "not_proven", "model", "model_provider",
		"reasoning_effort", "service_tier", "approval_policy", "sandbox_policy",
	}
	if len(got) != len(wantKeys) {
		t.Fatalf("thread projection keys = %#v, want exactly %v", got, wantKeys)
	}
	for _, key := range wantKeys {
		if _, ok := got[key]; !ok {
			t.Fatalf("thread projection missing %q: %#v", key, got)
		}
	}
	digestPattern := regexp.MustCompile(`^sha256:[0-9a-f]{64}$`)
	for _, key := range []string{"model", "model_provider"} {
		field := got[key].(map[string]any)
		if field["state"] != "observed" || !digestPattern.MatchString(field["digest"].(string)) {
			t.Fatalf("%s projection = %#v", key, field)
		}
	}
	for key, want := range map[string]string{
		"reasoning_effort": "high", "service_tier": "priority", "approval_policy": "never",
	} {
		field := got[key].(map[string]any)
		if field["state"] != "observed" || field["value"] != want {
			t.Fatalf("%s projection = %#v", key, field)
		}
	}
	sandbox := got["sandbox_policy"].(map[string]any)
	if mode := sandbox["mode"].(map[string]any); mode["state"] != "observed" || mode["value"] != "workspaceWrite" {
		t.Fatalf("sandbox mode = %#v", mode)
	}
	roots := sandbox["writable_roots"].(map[string]any)
	if roots["state"] != "observed" || roots["count"] != float64(2) || !digestPattern.MatchString(roots["digest"].(string)) {
		t.Fatalf("thread roots = %#v", roots)
	}
}

func TestCodexThreadConfigurationProjectionDistinguishesMissingAndMalformed(t *testing.T) {
	t.Parallel()

	projection := projectCodexThreadConfiguration(map[string]any{
		"model":           nil,
		"modelProvider":   "https://secret.invalid/provider",
		"reasoningEffort": "invented",
		"approvalPolicy":  7,
		"sandbox":         nil,
	})
	raw, err := json.Marshal(projection)
	if err != nil {
		t.Fatal(err)
	}
	var got map[string]any
	if err := json.Unmarshal(raw, &got); err != nil {
		t.Fatal(err)
	}
	assertState := func(field any, want string) {
		t.Helper()
		if state := field.(map[string]any)["state"]; state != want {
			t.Fatalf("state = %v, want %q in %#v", state, want, field)
		}
	}
	assertState(got["model"], "malformed")
	assertState(got["model_provider"], "malformed")
	assertState(got["reasoning_effort"], "malformed")
	assertState(got["service_tier"], "missing")
	assertState(got["approval_policy"], "malformed")
	for _, field := range got["sandbox_policy"].(map[string]any) {
		assertState(field, "malformed")
	}
}

func TestCodexThreadConfigurationObservationBindsSuccessfulStartAndResume(t *testing.T) {
	t.Parallel()

	for _, tc := range []struct {
		method  string
		priorID string
		resumed bool
	}{
		{method: "thread/start"},
		{method: "thread/resume", priorID: "prior-thread", resumed: true},
	} {
		t.Run(tc.method, func(t *testing.T) {
			var logs bytes.Buffer
			logger := slog.New(slog.NewJSONHandler(&logs, nil))
			client, stdin, _ := newTestCodexClient(t)
			client.cfg = Config{
				Logger: logger, WorkDir: filepath.Join(t.TempDir(), "task"),
				TaskID: "task-123", RuntimeID: "runtime-456",
			}
			client.pid = 321
			client.attempt = 2
			wait := drainRPCScript(t, client, stdin, []rpcResponse{{
				method: tc.method,
				result: json.RawMessage(`{"thread":{"id":"thread-789","instructions":"secret"},` +
					`"model":"gpt-5.6-sol","modelProvider":"syntheticloopback",` +
					`"reasoningEffort":"high","serviceTier":"default","approvalPolicy":"never",` +
					`"sandbox":{"type":"readOnly"},"private":"secret-response"}`),
			}})
			defer wait()

			threadID, resumed, err := client.startOrResumeThread(context.Background(), ExecOptions{
				Cwd: client.cfg.WorkDir, ResumeSessionID: tc.priorID,
			}, logger)
			if err != nil {
				t.Fatal(err)
			}
			if threadID != "thread-789" || resumed != tc.resumed {
				t.Fatalf("thread result = %q resumed=%t", threadID, resumed)
			}
			if calls := stdin.Lines(); len(calls) != 1 {
				t.Fatalf("thread observation made %d RPCs, want exactly 1: %v", len(calls), calls)
			}
			var observations []map[string]any
			for _, entry := range parseJSONLogEntries(t, logs.String()) {
				if entry["msg"] == "codex thread configuration observed" {
					observations = append(observations, entry)
				}
			}
			if len(observations) != 1 {
				t.Fatalf("thread observations = %v, want exactly one", observations)
			}
			entry := observations[0]
			if entry["task_id"] != "task-123" || entry["runtime_id"] != "runtime-456" ||
				entry["pid"] != float64(321) || entry["attempt"] != float64(2) ||
				entry["cwd"] != client.cfg.WorkDir || entry["method"] != tc.method {
				t.Fatalf("thread observation binding = %#v", entry)
			}
			if !regexp.MustCompile(`^sha256:[0-9a-f]{64}$`).MatchString(entry["thread_id_digest"].(string)) {
				t.Fatalf("thread digest = %#v", entry["thread_id_digest"])
			}
			if strings.Contains(logs.String(), "thread-789") || strings.Contains(logs.String(), "secret-response") {
				t.Fatalf("thread observation leaked raw response: %s", logs.String())
			}
		})
	}
}

func TestCodexThreadConfigurationObservationRequiresBoundValidatedThread(t *testing.T) {
	t.Parallel()

	for _, tc := range []struct {
		name     string
		cwd      string
		threadID string
	}{
		{name: "different cwd", cwd: "/neutral", threadID: "thread-1"},
		{name: "invalid thread id", cwd: "/task", threadID: "https://secret.invalid/thread"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			var logs bytes.Buffer
			client := &codexClient{
				cfg: Config{
					Logger: slog.New(slog.NewJSONHandler(&logs, nil)), WorkDir: "/task",
					TaskID: "task", RuntimeID: "runtime",
				},
				pid: 10, attempt: 1,
			}
			observeCodexThreadConfiguration(client, tc.cwd, "thread/start", tc.threadID, map[string]any{"model": "gpt-5"})
			if logs.Len() != 0 {
				t.Fatalf("unbound thread configuration logged: %s", logs.String())
			}
		})
	}
}
