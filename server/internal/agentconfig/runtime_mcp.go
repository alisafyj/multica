package agentconfig

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"strings"
	"unicode"
	"unicode/utf8"
)

var ErrRuntimeMCPSelection = errors.New("invalid runtime MCP selection")

// RuntimeMCPSelection contains names only; runtime connection details stay local.
type RuntimeMCPSelection struct {
	Mode  string
	Allow []string
}

func (s RuntimeMCPSelection) ValidateProvider(provider string) error {
	if s.Mode != "inherit" && provider != "codex" && provider != "claude" {
		return fmt.Errorf("%w: non-default selection supports only Codex and Claude", ErrRuntimeMCPSelection)
	}
	if provider == "codex" && s.Mode == "allowlist" {
		// Match the existing Codex MCP renderer's isCodexBareTomlKey contract.
		for _, name := range s.Allow {
			if name == "" || strings.ContainsFunc(name, func(r rune) bool {
				return !((r >= 'a' && r <= 'z') || (r >= 'A' && r <= 'Z') || (r >= '0' && r <= '9') || r == '_' || r == '-')
			}) {
				return fmt.Errorf("%w: Codex allowlist names must match [A-Za-z0-9_-]+", ErrRuntimeMCPSelection)
			}
		}
	}
	return nil
}

// ParseRuntimeMCPSelection validates runtimeMcp within extensible _multica
// metadata. Other metadata and top-level overlay fields remain untouched.
func ParseRuntimeMCPSelection(raw json.RawMessage) (RuntimeMCPSelection, error) {
	selection := RuntimeMCPSelection{Mode: "inherit"}
	trimmed := bytes.TrimSpace(raw)
	if len(trimmed) == 0 || bytes.Equal(trimmed, []byte("null")) {
		return selection, nil
	}
	document, err := runtimeMCPObject(trimmed)
	if err != nil {
		return selection, err
	}
	envelopeRaw, present := document["_multica"]
	if !present {
		return selection, nil
	}
	envelope, err := runtimeMCPObject(envelopeRaw)
	if err != nil {
		return selection, err
	}
	for key := range envelope {
		if key != "runtimeMcp" && strings.EqualFold(key, "runtimeMcp") {
			return selection, fmt.Errorf("%w: reserved runtimeMcp field must use exact casing", ErrRuntimeMCPSelection)
		}
	}
	policyRaw, present := envelope["runtimeMcp"]
	if !present {
		return selection, nil
	}
	policy, err := runtimeMCPObject(policyRaw)
	if err != nil {
		return selection, err
	}
	for key := range policy {
		if key != "mode" && key != "allow" {
			return selection, fmt.Errorf("%w: unsupported runtimeMcp field", ErrRuntimeMCPSelection)
		}
	}
	var mode string
	if err := json.Unmarshal(policy["mode"], &mode); err != nil {
		return selection, fmt.Errorf("%w: mode must be inherit, deny_all or allowlist", ErrRuntimeMCPSelection)
	}
	selection.Mode = mode
	allow, hasAllow := policy["allow"]
	switch selection.Mode {
	case "inherit", "deny_all":
		if hasAllow {
			return selection, fmt.Errorf("%w: allow is only valid in allowlist mode", ErrRuntimeMCPSelection)
		}
	case "allowlist":
		if !hasAllow || json.Unmarshal(allow, &selection.Allow) != nil || selection.Allow == nil || len(selection.Allow) > 64 {
			return selection, fmt.Errorf("%w: allow must be an array of at most 64 names", ErrRuntimeMCPSelection)
		}
		seen := make(map[string]bool, len(selection.Allow))
		for _, name := range selection.Allow {
			if name == "" || len(name) > 128 || !utf8.ValidString(name) || strings.ContainsFunc(name, func(r rune) bool { return unicode.IsSpace(r) || unicode.IsControl(r) || r == utf8.RuneError }) || seen[name] {
				return selection, fmt.Errorf("%w: names must be unique, nonempty, at most 128 bytes and contain no whitespace or control characters", ErrRuntimeMCPSelection)
			}
			seen[name] = true
		}
	default:
		return selection, fmt.Errorf("%w: mode must be inherit, deny_all or allowlist", ErrRuntimeMCPSelection)
	}
	return selection, nil
}

// JSON object decoding normally accepts duplicate keys and case-insensitive
// struct fields. Neither is safe for a selection policy that must fail closed.
func runtimeMCPObject(raw json.RawMessage) (map[string]json.RawMessage, error) {
	invalid := fmt.Errorf("%w: expected a JSON object with exact, unique fields", ErrRuntimeMCPSelection)
	if !utf8.Valid(raw) {
		return nil, invalid
	}
	decoder := json.NewDecoder(bytes.NewReader(raw))
	start, err := decoder.Token()
	if err != nil || start != json.Delim('{') {
		return nil, invalid
	}
	result := make(map[string]json.RawMessage)
	for decoder.More() {
		token, err := decoder.Token()
		key, ok := token.(string)
		if err != nil || !ok {
			return nil, invalid
		}
		if _, duplicate := result[key]; duplicate {
			return nil, invalid
		}
		var value json.RawMessage
		if err := decoder.Decode(&value); err != nil {
			return nil, invalid
		}
		result[key] = value
	}
	if end, err := decoder.Token(); err != nil || end != json.Delim('}') {
		return nil, invalid
	}
	if _, err := decoder.Token(); err != io.EOF {
		return nil, invalid
	}
	return result, nil
}
