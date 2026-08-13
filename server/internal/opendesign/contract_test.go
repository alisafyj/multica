package opendesign

import (
	"strings"
	"testing"
)

func TestPinnedEngineIdentityMatchesValidatedRelease(t *testing.T) {
	identity := PinnedEngineIdentity()

	if identity.Schema != EngineIdentitySchema {
		t.Fatalf("schema = %q, want %q", identity.Schema, EngineIdentitySchema)
	}
	if identity.Release != "open-design-v0.16.1" {
		t.Fatalf("release = %q", identity.Release)
	}
	if identity.Commit != "276b4d8e970bc143d7ad060181a89a834e3d9caf" {
		t.Fatalf("commit = %q", identity.Commit)
	}
	if identity.LockfileSHA256 != "90bbe1375eb716240bbb79215c2a12a601abd977fe88587c6c6c6b4df31f6f23" {
		t.Fatalf("lockfile_sha256 = %q", identity.LockfileSHA256)
	}
	if identity.DistSHA256 != "bc0a56497d56f85f7c807fe742022077bc35b1360de39bf298f07b184db1e7de" {
		t.Fatalf("dist_sha256 = %q", identity.DistSHA256)
	}
	if err := identity.Validate(); err != nil {
		t.Fatalf("Validate: %v", err)
	}
}

func TestResolveAdapterUsesExplicitPinnedRegistryMapping(t *testing.T) {
	tests := []struct {
		provider string
		want     string
		ok       bool
	}{
		{provider: "opencode", want: "opencode", ok: true},
		{provider: "codex", want: "codex", ok: true},
		{provider: "cursor", want: "cursor-agent", ok: true},
		{provider: "openclaw", ok: false},
		{provider: "gemini", ok: false},
		{provider: "OpenCode", ok: false},
	}

	for _, tt := range tests {
		t.Run(tt.provider, func(t *testing.T) {
			got, ok := ResolveAdapter(tt.provider)
			if ok != tt.ok || got != tt.want {
				t.Fatalf("ResolveAdapter(%q) = (%q, %v), want (%q, %v)", tt.provider, got, ok, tt.want, tt.ok)
			}
		})
	}
}

func TestValidatePreflightRequiresObservedEngineAndAgentEvidence(t *testing.T) {
	expected := ExpectedPreflight{
		Engine:    PinnedEngineIdentity(),
		AdapterID: "opencode",
		Model:     "anthropic/claude-sonnet-4-5",
	}
	report := PreflightReport{
		Schema:    PreflightSchema,
		Engine:    PinnedEngineIdentity(),
		AdapterID: "opencode",
		Model:     "anthropic/claude-sonnet-4-5",
		Binary:    ProbeResult{Status: ProbePassed, Version: "1.0.0"},
		Auth:      ProbeResult{Status: ProbeUnknown, Required: false},
		ModelProbe: ProbeResult{
			Status: ProbePassed,
		},
		Plugins: PluginPreflight{Policy: PluginsDisabled},
	}

	if err := ValidatePreflight(expected, report); err != nil {
		t.Fatalf("ValidatePreflight: %v", err)
	}

	t.Run("engine digest mismatch", func(t *testing.T) {
		mismatch := report
		mismatch.Engine.DistSHA256 = strings.Repeat("0", 64)
		if err := ValidatePreflight(expected, mismatch); err == nil || !strings.Contains(err.Error(), "dist_sha256") {
			t.Fatalf("ValidatePreflight error = %v, want dist_sha256 mismatch", err)
		}
	})

	t.Run("binary unavailable", func(t *testing.T) {
		unavailable := report
		unavailable.Binary.Status = ProbeFailed
		if err := ValidatePreflight(expected, unavailable); err == nil || !strings.Contains(err.Error(), "binary") {
			t.Fatalf("ValidatePreflight error = %v, want binary failure", err)
		}
	})

	t.Run("required auth unknown", func(t *testing.T) {
		missing := report
		missing.Auth.Required = true
		if err := ValidatePreflight(expected, missing); err == nil || !strings.Contains(err.Error(), "auth") {
			t.Fatalf("ValidatePreflight error = %v, want auth failure", err)
		}
	})

	t.Run("model mismatch", func(t *testing.T) {
		mismatch := report
		mismatch.Model = "openai/gpt-5"
		if err := ValidatePreflight(expected, mismatch); err == nil || !strings.Contains(err.Error(), "model") {
			t.Fatalf("ValidatePreflight error = %v, want model mismatch", err)
		}
	})
}
