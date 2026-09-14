package agentconfig

import (
	"encoding/json"
	"errors"
	"fmt"
	"reflect"
	"strings"
	"testing"
)

func TestRuntimeMCPSelectionParser(t *testing.T) {
	for _, raw := range []string{"", "null", "{}", `{"mcpServers":{},"extension":{"keep":true}}`, `{"_multica":{}}`, `{"_multica":{"runtimeMcp":{"mode":"inherit"}}}`} {
		got, err := ParseRuntimeMCPSelection(json.RawMessage(raw))
		if err != nil || got.Mode != "inherit" || len(got.Allow) != 0 {
			t.Fatalf("default selection: %v, %v", got, err)
		}
	}
	for _, names := range [][]string{{}, {"Docs", "docs", "plugin:source"}, {strings.Repeat("x", 128)}, {strings.Repeat("\u00e9", 64)}} {
		raw, _ := json.Marshal(map[string]any{"_multica": map[string]any{"runtimeMcp": map[string]any{"mode": "allowlist", "allow": names}}, "extension": "keep"})
		before := string(raw)
		got, err := ParseRuntimeMCPSelection(raw)
		if err != nil || got.Mode != "allowlist" || !reflect.DeepEqual(got.Allow, names) {
			t.Fatalf("exact allowlist not preserved: %v", err)
		}
		if string(raw) != before {
			t.Fatal("parser mutated config")
		}
	}
	names := make([]string, 64)
	for i := range names {
		names[i] = fmt.Sprintf("server-%d", i)
	}
	raw, _ := json.Marshal(map[string]any{"_multica": map[string]any{"runtimeMcp": map[string]any{"mode": "allowlist", "allow": names}}})
	if _, err := ParseRuntimeMCPSelection(raw); err != nil {
		t.Fatal(err)
	}
}

func TestRuntimeMCPSelectionParserRejectsAmbiguityAndSecrets(t *testing.T) {
	const secret = "synthetic-policy-secret"
	invalid := []string{
		`[]`, `"value"`, `{`, `{"_multica":null}`, `{"_multica":[]}`,
		`{"_multica":{"runtimeMcp":null}}`, `{"_multica":{"runtimeMcp":{}}}`,
		`{"_multica":{"runtimeMcp":{"Mode":"deny_all"}}}`,
		`{"_multica":{"runtimeMcp":{"mode":null}}}`,
		`{"_multica":{"runtimeMcp":{"mode":"deny-all"}}}`,
		`{"_multica":{"runtimeMcp":{"mode":"allowlist"}}}`,
		`{"_multica":{"runtimeMcp":{"mode":"allowlist","allow":null}}}`,
		`{"_multica":{"runtimeMcp":{"mode":"allowlist","allow":"docs"}}}`,
		`{"_multica":{"runtimeMcp":{"mode":"allowlist","allow":[null]}}}`,
		`{"_multica":{"runtimeMcp":{"mode":"inherit","allow":[]}}}`,
		`{"_multica":{"runtimeMcp":{"mode":"deny_all","allow":[]}}}`,
		`{"_multica":{"runtimeMcp":{"mode":"deny_all","url":"` + secret + `"}}}`,
		`{"_multica":{"future":true,"runtimeMcp":{"mode":"deny_all","Allow":[]}}}`,
		`{"_multica":{"future":"` + secret + `","future":null}}`,
		`{"_multica":{"future":true,"futur\u0065":false}}`,
		`{"_multica":{"runtimeMcp":{"mode":"deny_all","mode":"inherit"}}}`,
		`{"_multica":{"future":true,"runtimeMcp":{"mode":"allowlist","allow":[],"allow":["docs"]}}}`,
		`{"_multica":{"runtimeMcp":{"mode":"deny_all"}},"_multica":{}}`,
		`{"_multica":{"runtimeMcp":{"mode":"deny_all"},"runtimeMcp":{"mode":"inherit"}}}`,
	}
	names65 := make([]string, 65)
	for i := range names65 {
		names65[i] = fmt.Sprintf("server-%d", i)
	}
	for _, names := range [][]string{{""}, {" docs"}, {"docs "}, {"two words"}, {"line\nbreak"}, {"tab\tname"}, {"non\u00a0breaking"}, {"nul\x00byte"}, {"same", "same"}, {strings.Repeat("x", 129)}, {strings.Repeat("\u00e9", 65)}, names65} {
		raw, _ := json.Marshal(map[string]any{"_multica": map[string]any{"runtimeMcp": map[string]any{"mode": "allowlist", "allow": names}}})
		invalid = append(invalid, string(raw))
	}
	for i, raw := range invalid {
		_, err := ParseRuntimeMCPSelection(json.RawMessage(raw))
		if !errors.Is(err, ErrRuntimeMCPSelection) {
			t.Errorf("case %d not rejected with safe selection error: %v", i, err)
		}
		if err != nil && strings.Contains(err.Error(), secret) {
			t.Fatal("error leaked rejected input")
		}
	}
}

func TestRuntimeMCPSelectionParserPreservesUnknownMetadata(t *testing.T) {
	for _, tc := range []struct {
		name, policy, mode string
		allow              []string
	}{
		{"missing", "", "inherit", nil},
		{"inherit", `,"runtimeMcp":{"mode":"inherit"}`, "inherit", nil},
		{"deny_all", `,"runtimeMcp":{"mode":"deny_all"}`, "deny_all", nil},
		{"allowlist", `,"runtimeMcp":{"mode":"allowlist","allow":["Docs","docs"]}`, "allowlist", []string{"Docs", "docs"}},
		{"empty_allowlist", `,"runtimeMcp":{"mode":"allowlist","allow":[]}`, "allowlist", []string{}},
	} {
		t.Run(tc.name, func(t *testing.T) {
			raw := json.RawMessage(`{"mcpServers":{},"extension":{"keep":true},"_multica":{"future":{"enabled":true,"nested":[null,42]},"headers":"synthetic-metadata-secret","futureOther":{"mode":"deny_all"}` + tc.policy + `}}`)
			before := string(raw)
			got, err := ParseRuntimeMCPSelection(raw)
			if err != nil || got.Mode != tc.mode || !reflect.DeepEqual(got.Allow, tc.allow) {
				t.Fatalf("unknown metadata changed selection: %v, %v", got, err)
			}
			if string(raw) != before {
				t.Fatal("parser mutated metadata or overlays")
			}
		})
	}
}

func TestRuntimeMCPSelectionParserRejectsReservedCaseCollisions(t *testing.T) {
	for _, spelling := range []string{"runtimeMCP", "RUNTIMEMCP"} {
		for _, tc := range []struct{ name, policy string }{
			{"missing_mode", `{}`},
			{"null", `null`},
			{"inherit", `{"mode":"inherit"}`},
			{"deny_all", `{"mode":"deny_all"}`},
			{"allowlist", `{"mode":"allowlist","allow":[]}`},
		} {
			for _, withCanonical := range []bool{false, true} {
				t.Run(fmt.Sprintf("%s/%s/canonical=%t", spelling, tc.name, withCanonical), func(t *testing.T) {
					metadata := map[string]any{spelling: json.RawMessage(tc.policy), "futureOther": "synthetic-case-secret"}
					if withCanonical {
						metadata["runtimeMcp"] = map[string]any{"mode": "inherit"}
					}
					raw, err := json.Marshal(map[string]any{"_multica": metadata})
					if err != nil {
						t.Fatal(err)
					}
					_, err = ParseRuntimeMCPSelection(raw)
					if !errors.Is(err, ErrRuntimeMCPSelection) {
						t.Fatalf("reserved metadata case collision accepted: %v", err)
					}
					if strings.Contains(err.Error(), "synthetic-case-secret") {
						t.Fatal("case collision error leaked metadata")
					}
				})
			}
		}
	}
}

func TestRuntimeMCPSelectionProviderBoundary(t *testing.T) {
	for _, provider := range []string{"claude", "codex", "unknown", "openclaw", "codebuddy", "cursor"} {
		for _, mode := range []string{"inherit", "deny_all", "allowlist"} {
			got := (RuntimeMCPSelection{Mode: mode}).ValidateProvider(provider)
			wantOK := mode == "inherit" || provider == "claude" || provider == "codex"
			if (got == nil) != wantOK {
				t.Errorf("provider=%s mode=%s: %v", provider, mode, got)
			}
		}
	}
}

func TestRuntimeMCPSelectionCodexNameContract(t *testing.T) {
	for i, tc := range []struct {
		name    string
		codexOK bool
	}{
		{"docs", true}, {"Docs_012-ABC", true}, {"_", true}, {"-", true},
		{strings.Repeat("x", 128), true},
		{"plugin:source", false}, {"docs.read", false}, {"path/name", false},
		{"quoted\"name", false}, {"[table]", false},
		{"\u00e9", false}, {"\u5de5\u5177", false}, {strings.Repeat("\u00e9", 64), false},
		{"synthetic-name-secret:unsupported", false},
	} {
		t.Run(fmt.Sprintf("case-%d", i), func(t *testing.T) {
			raw, err := json.Marshal(map[string]any{"_multica": map[string]any{"runtimeMcp": map[string]any{"mode": "allowlist", "allow": []string{tc.name}}}})
			if err != nil {
				t.Fatal(err)
			}
			selection, err := ParseRuntimeMCPSelection(raw)
			if err != nil {
				t.Fatalf("provider-neutral name rejected: %v", err)
			}
			err = selection.ValidateProvider("codex")
			if tc.codexOK {
				if err != nil {
					t.Fatalf("renderable Codex name rejected: %v", err)
				}
			} else if !errors.Is(err, ErrRuntimeMCPSelection) {
				t.Fatalf("unrenderable Codex name accepted: %v", err)
			}
			if err != nil && strings.Contains(err.Error(), tc.name) {
				t.Fatal("provider validation error leaked name")
			}
			if err := selection.ValidateProvider("claude"); err != nil {
				t.Fatalf("Claude name contract narrowed: %v", err)
			}
			if !reflect.DeepEqual(selection.Allow, []string{tc.name}) {
				t.Fatal("provider validation changed exact name")
			}
		})
	}
	if err := (RuntimeMCPSelection{Mode: "allowlist", Allow: []string{}}).ValidateProvider("codex"); err != nil {
		t.Fatalf("empty allowlist rejected: %v", err)
	}
}
