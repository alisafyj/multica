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

func TestCodexConfigurationProjectionWhitelistsExplicitValues(t *testing.T) {
	t.Parallel()

	rootA := filepath.Join(t.TempDir(), "alpha")
	rootB := filepath.Join(t.TempDir(), "beta")
	projection := projectCodexConfiguration(map[string]any{
		"model":                  "private-model-name",
		"model_provider":         "private-provider-name",
		"model_reasoning_effort": "high",
		"service_tier":           "priority",
		"approval_policy":        "never",
		"sandbox_mode":           "workspace-write",
		"sandbox_workspace_write": map[string]any{
			"network_access":         true,
			"exclude_tmpdir_env_var": false,
			"exclude_slash_tmp":      true,
			"writable_roots":         []any{rootA, rootB},
			"secret_workspace_key":   "must-not-appear",
		},
		"features": map[string]any{
			"default_mode_request_user_input": true,
			"fast_mode":                       false,
			"memories":                        true,
			"multi_agent":                     false,
			"plugins":                         true,
			"responses_websockets":            false,
			"private_feature_name":            true,
		},
		"mcp_servers": map[string]any{"secret-server": map[string]any{"url": "https://secret.invalid/token"}},
		"env":         map[string]any{"API_KEY": "secret-api-key"},
		"hooks":       map[string]any{"command": "secret-hook"},
		"plugins":     map[string]any{"secret-plugin": map[string]any{"connection": "secret-connection"}},
		"model_providers": map[string]any{
			"private-provider-name": map[string]any{"base_url": "https://provider.invalid"},
		},
		"raw_origin": "secret-origin",
	})

	raw, err := json.Marshal(projection)
	if err != nil {
		t.Fatal(err)
	}
	serialized := string(raw)
	for _, forbidden := range []string{
		"private-model-name", "private-provider-name", "must-not-appear",
		"private_feature_name", "secret-server", "secret-api-key", "secret-hook",
		"secret-plugin", "secret-connection", "provider.invalid", "raw_origin", "secret-origin",
	} {
		if strings.Contains(serialized, forbidden) {
			t.Fatalf("projection leaked %q: %s", forbidden, serialized)
		}
	}

	var got map[string]any
	if err := json.Unmarshal(raw, &got); err != nil {
		t.Fatal(err)
	}
	if got["schema"] != "codex_configuration_projection/v1" ||
		got["scope"] != "explicit_config_read_values_only" ||
		got["completeness"] != "partial" {
		t.Fatalf("projection contract fields = %#v", got)
	}
	wantNotProven := []any{
		"native_defaults", "layer_provenance", "complete_configuration",
		"provider_reported_model", "thread_permission_confirmation",
	}
	if actual := got["not_proven"].([]any); !equalCodexProjectionValues(actual, wantNotProven) {
		t.Fatalf("not_proven = %v, want fixed order %v", actual, wantNotProven)
	}
	wantTopLevel := []string{
		"schema", "scope", "completeness", "not_proven", "model", "model_provider",
		"reasoning_effort", "service_tier", "approval_policy", "sandbox_mode",
		"workspace_write", "features",
	}
	if len(got) != len(wantTopLevel) {
		t.Fatalf("projection keys = %#v, want exactly %v", got, wantTopLevel)
	}
	for _, key := range wantTopLevel {
		if _, ok := got[key]; !ok {
			t.Fatalf("projection missing key %q: %#v", key, got)
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
		"reasoning_effort": "high", "service_tier": "priority",
		"approval_policy": "never", "sandbox_mode": "workspace-write",
	} {
		field := got[key].(map[string]any)
		if field["state"] != "observed" || field["value"] != want {
			t.Fatalf("%s projection = %#v", key, field)
		}
	}
	workspace := got["workspace_write"].(map[string]any)
	roots := workspace["writable_roots"].(map[string]any)
	if roots["state"] != "observed" || roots["count"] != float64(2) || !digestPattern.MatchString(roots["digest"].(string)) {
		t.Fatalf("writable_roots projection = %#v", roots)
	}
	features := got["features"].(map[string]any)
	if len(features) != 6 || features["plugins"].(map[string]any)["value"] != true {
		t.Fatalf("known feature projection = %#v", features)
	}
	if _, ok := features["private_feature_name"]; ok {
		t.Fatalf("arbitrary feature name projected: %#v", features)
	}
}

func equalCodexProjectionValues(got, want []any) bool {
	if len(got) != len(want) {
		return false
	}
	for i := range got {
		if got[i] != want[i] {
			return false
		}
	}
	return true
}

func TestCodexConfigurationProjectionDistinguishesMissingAndMalformed(t *testing.T) {
	t.Parallel()

	projection := projectCodexConfiguration(map[string]any{
		"model_provider":         " ",
		"model_reasoning_effort": "invented",
		"approval_policy":        7,
		"sandbox_workspace_write": map[string]any{
			"network_access": false,
			"writable_roots": []any{"relative", 9},
		},
		"features": "not-a-table",
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
	assertState(got["model"], "missing")
	assertState(got["model_provider"], "malformed")
	assertState(got["reasoning_effort"], "malformed")
	assertState(got["service_tier"], "missing")
	assertState(got["approval_policy"], "malformed")
	assertState(got["sandbox_mode"], "missing")
	workspace := got["workspace_write"].(map[string]any)
	assertState(workspace["network_access"], "observed")
	assertState(workspace["exclude_tmpdir_env_var"], "missing")
	assertState(workspace["writable_roots"], "malformed")
	features := got["features"].(map[string]any)
	for _, field := range features {
		assertState(field, "malformed")
	}

	nullProjection := projectCodexConfiguration(map[string]any{
		"model":                   nil,
		"sandbox_workspace_write": nil,
		"features":                nil,
	})
	nullRaw, err := json.Marshal(nullProjection)
	if err != nil {
		t.Fatal(err)
	}
	var nullGot map[string]any
	if err := json.Unmarshal(nullRaw, &nullGot); err != nil {
		t.Fatal(err)
	}
	assertState(nullGot["model"], "malformed")
	for _, field := range nullGot["workspace_write"].(map[string]any) {
		assertState(field, "malformed")
	}
	for _, field := range nullGot["features"].(map[string]any) {
		assertState(field, "malformed")
	}
}

func TestCodexConfigurationProjectionCanonicalizesWritableRoots(t *testing.T) {
	t.Parallel()

	rootA := filepath.Join(t.TempDir(), "alpha")
	rootB := filepath.Join(t.TempDir(), "beta")
	project := func(roots []any) codexConfigurationRoots {
		return projectCodexConfiguration(map[string]any{
			"sandbox_workspace_write": map[string]any{"writable_roots": roots},
		}).WorkspaceWrite.WritableRoots
	}
	first := project([]any{rootB, rootA, rootB})
	second := project([]any{rootA, rootB})
	if first.State != codexConfigObserved || second.State != codexConfigObserved ||
		first.Count == nil || *first.Count != 2 || second.Count == nil || *second.Count != 2 ||
		first.Digest != second.Digest {
		t.Fatalf("roots were not canonicalized: first=%#v second=%#v", first, second)
	}
	for _, roots := range [][]any{
		{filepath.Join(t.TempDir(), "bad\nroot")},
		{"/" + strings.Repeat("a", 4096)},
	} {
		if got := project(roots); got.State != codexConfigMalformed {
			t.Fatalf("invalid roots projected as %#v", got)
		}
	}
}

func TestCodexConfigurationProjectionValidatesIdentifiersBeforeDigest(t *testing.T) {
	t.Parallel()

	valid := projectCodexConfiguration(map[string]any{
		"model":          "openai/gpt-5.6_sol:v1",
		"model_provider": "synthetic.loopback-provider/v1",
	})
	if valid.Model.State != codexConfigObserved || valid.ModelProvider.State != codexConfigObserved {
		t.Fatalf("valid identifiers rejected: %#v", valid)
	}
	for _, value := range []any{
		" ", "https://provider.invalid/model", "user@provider", "模型",
		strings.Repeat("a", 257), "bad\nmodel", 42, nil,
	} {
		projection := projectCodexConfiguration(map[string]any{"model": value, "model_provider": value})
		if projection.Model.State != codexConfigMalformed || projection.ModelProvider.State != codexConfigMalformed {
			t.Fatalf("invalid identifier %#v projected: model=%#v provider=%#v", value, projection.Model, projection.ModelProvider)
		}
		if projection.Model.Digest != "" || projection.ModelProvider.Digest != "" {
			t.Fatalf("invalid identifier %#v was hashed", value)
		}
	}
}

func TestReadCodexConfigurationObservesBoundTaskCwdWithoutExtraRPC(t *testing.T) {
	t.Parallel()

	var logs bytes.Buffer
	client, stdin, _ := newTestCodexClient(t)
	client.cfg = Config{
		Logger:    slog.New(slog.NewJSONHandler(&logs, nil)),
		WorkDir:   filepath.Join(t.TempDir(), "task"),
		TaskID:    "task-123",
		RuntimeID: "runtime-456",
	}
	client.pid = 321
	client.attempt = 2
	wait := drainRPCScript(t, client, stdin, []rpcResponse{{
		method: "config/read",
		result: json.RawMessage(`{"config":{"model":"secret-model","approval_policy":"never","sandbox_mode":"read-only"}}`),
	}})
	defer wait()

	if _, err := readCodexConfiguration(context.Background(), client, client.cfg.WorkDir, codexConfigurationReadPurposeTaskEffective); err != nil {
		t.Fatal(err)
	}
	if calls := stdin.Lines(); len(calls) != 1 {
		t.Fatalf("config observation made %d RPCs, want exactly 1: %v", len(calls), calls)
	}
	entries := parseJSONLogEntries(t, logs.String())
	if len(entries) != 1 {
		t.Fatalf("observation logs = %v, want exactly one", entries)
	}
	entry := entries[0]
	if entry["msg"] != "codex configuration observed" || entry["task_id"] != "task-123" ||
		entry["runtime_id"] != "runtime-456" || entry["pid"] != float64(321) ||
		entry["attempt"] != float64(2) || entry["cwd"] != client.cfg.WorkDir {
		t.Fatalf("observation correlation fields = %#v", entry)
	}
	configuration := entry["configuration"].(map[string]any)
	if configuration["schema"] != "codex_configuration_projection/v1" {
		t.Fatalf("observation configuration = %#v", configuration)
	}
	if strings.Contains(logs.String(), "secret-model") {
		t.Fatalf("observation leaked model identifier: %s", logs.String())
	}
}

func TestReadCodexConfigurationNeutralPurposeDoesNotObserve(t *testing.T) {
	t.Parallel()

	var logs bytes.Buffer
	client, stdin, _ := newTestCodexClient(t)
	client.cfg = Config{
		Logger:    slog.New(slog.NewJSONHandler(&logs, nil)),
		WorkDir:   filepath.Join(t.TempDir(), "task"),
		TaskID:    "task-123",
		RuntimeID: "runtime-456",
	}
	client.pid = 321
	client.attempt = 2
	wait := drainRPCScript(t, client, stdin, []rpcResponse{{
		method: "config/read",
		result: json.RawMessage(`{"config":{"model":"secret-model"}}`),
	}})
	defer wait()

	if _, err := readCodexConfiguration(context.Background(), client, client.cfg.WorkDir, codexConfigurationReadPurposeNeutral); err != nil {
		t.Fatal(err)
	}
	if calls := stdin.Lines(); len(calls) != 1 {
		t.Fatalf("neutral config read made %d RPCs, want exactly 1: %v", len(calls), calls)
	}
	if logs.Len() != 0 {
		t.Fatalf("neutral config read emitted observation: %s", logs.String())
	}
}

func TestCodexConfigurationObservationRequiresExactBoundContext(t *testing.T) {
	t.Parallel()

	cases := []struct {
		name   string
		mutate func(*codexClient) (string, codexConfigurationReadPurpose)
	}{
		{name: "neutral purpose", mutate: func(c *codexClient) (string, codexConfigurationReadPurpose) {
			return c.cfg.WorkDir, codexConfigurationReadPurposeNeutral
		}},
		{name: "different cwd", mutate: func(c *codexClient) (string, codexConfigurationReadPurpose) {
			return "/neutral", codexConfigurationReadPurposeTaskEffective
		}},
		{name: "nil logger", mutate: func(c *codexClient) (string, codexConfigurationReadPurpose) {
			c.cfg.Logger = nil
			return c.cfg.WorkDir, codexConfigurationReadPurposeTaskEffective
		}},
		{name: "missing task", mutate: func(c *codexClient) (string, codexConfigurationReadPurpose) {
			c.cfg.TaskID = ""
			return c.cfg.WorkDir, codexConfigurationReadPurposeTaskEffective
		}},
		{name: "missing runtime", mutate: func(c *codexClient) (string, codexConfigurationReadPurpose) {
			c.cfg.RuntimeID = ""
			return c.cfg.WorkDir, codexConfigurationReadPurposeTaskEffective
		}},
		{name: "missing pid", mutate: func(c *codexClient) (string, codexConfigurationReadPurpose) {
			c.pid = 0
			return c.cfg.WorkDir, codexConfigurationReadPurposeTaskEffective
		}},
		{name: "missing attempt", mutate: func(c *codexClient) (string, codexConfigurationReadPurpose) {
			c.attempt = 0
			return c.cfg.WorkDir, codexConfigurationReadPurposeTaskEffective
		}},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			var logs bytes.Buffer
			client := &codexClient{
				cfg: Config{
					Logger:  slog.New(slog.NewJSONHandler(&logs, nil)),
					WorkDir: "/task", TaskID: "task", RuntimeID: "runtime",
				},
				pid: 10, attempt: 1,
			}
			cwd, purpose := tc.mutate(client)
			observeCodexConfiguration(client, cwd, purpose, map[string]any{"model": "secret"})
			if logs.Len() != 0 {
				t.Fatalf("unbound context logged: %s", logs.String())
			}
		})
	}
}
