package daemon

import (
	"reflect"
	"strings"
	"testing"
)

func TestTaskCodexResponseMetadata(t *testing.T) {
	const key = "MULTICA_CODEX_RESPONSE_METADATA"
	for _, tc := range []struct {
		name, provider, version, value  string
		builtin, present, want, wantErr bool
	}{
		{name: "unset", provider: "claude", version: "unknown"},
		{name: "codex", provider: "codex", version: "0.153.4", value: "codex_sse_v0_153_4", builtin: true, present: true, want: true},
		{name: "version prefix", provider: "codex", version: "codex-cli 0.153.4", value: "codex_sse_v0_153_4", builtin: true, present: true, want: true},
		{name: "unknown value", provider: "codex", version: "0.153.4", value: "secret-invalid-value", builtin: true, present: true, wantErr: true},
		{name: "empty value", provider: "codex", version: "0.153.4", builtin: true, present: true, wantErr: true},
		{name: "wrong provider", provider: "claude", version: "0.153.4", value: "codex_sse_v0_153_4", builtin: true, present: true, wantErr: true},
		{name: "custom runtime", provider: "codex", version: "0.153.4", value: "codex_sse_v0_153_4", present: true, wantErr: true},
		{name: "upgraded runtime", provider: "codex", version: "0.153.5", value: "codex_sse_v0_153_4", builtin: true, present: true, wantErr: true},
	} {
		t.Run(tc.name, func(t *testing.T) {
			data := &AgentData{CustomEnv: map[string]string{"ordinary": "preserved"}}
			if tc.present {
				data.CustomEnv[key] = tc.value
			}
			before := make(map[string]string, len(data.CustomEnv))
			for k, v := range data.CustomEnv {
				before[k] = v
			}
			got, err := taskCodexResponseMetadata(data, tc.provider, tc.version, tc.builtin)
			if got != tc.want || (err != nil) != tc.wantErr {
				t.Fatalf("got enabled=%v err=%v", got, err)
			}
			if err != nil && strings.Contains(err.Error(), "secret-invalid-value") {
				t.Fatal("raw control value leaked")
			}
			if !reflect.DeepEqual(before, data.CustomEnv) {
				t.Fatal("custom environment mutated")
			}
		})
	}
	if enabled, err := taskCodexResponseMetadata(nil, "codex", "0.153.4", true); enabled || err != nil {
		t.Fatal("nil agent must preserve default behavior")
	}
	if !isBlockedEnvKey(key) {
		t.Fatal("diagnostic control must stay blocked from child environment")
	}
	for _, name := range codexShellAuthorizedCustomEnvNames(map[string]string{key: "codex_sse_v0_153_4"}) {
		if name == key {
			t.Fatal("diagnostic control entered tool shell inheritance")
		}
	}
}
