package opendesign

import (
	"context"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

func TestProbeClientBuildsPreflightFromPinnedWorkerAgentInventory(t *testing.T) {
	const token = "worker-test-token"
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/api/agents" {
			http.NotFound(w, r)
			return
		}
		if r.Header.Get("Authorization") != "Bearer "+token {
			http.Error(w, "unauthorized", http.StatusUnauthorized)
			return
		}
		w.Header().Set("Content-Type", "application/json")
		fmt.Fprint(w, `{
			"agents":[{
				"id":"opencode",
				"name":"OpenCode",
				"bin":"opencode-cli",
				"available":true,
				"path":"/usr/local/bin/opencode",
				"version":"1.0.0",
				"modelsSource":"live",
				"models":[{"id":"anthropic/claude-sonnet-4-5","label":"Claude Sonnet","enabled":true}]
			}]
		}`)
	}))
	defer server.Close()

	expected := ExpectedPreflight{
		Engine:    PinnedEngineIdentity(),
		AdapterID: "opencode",
		Model:     "anthropic/claude-sonnet-4-5",
	}
	client, err := NewProbeClient(server.URL, token, server.Client())
	if err != nil {
		t.Fatalf("NewProbeClient: %v", err)
	}
	report, err := client.Probe(context.Background(), expected, PluginsDisabled)
	if err != nil {
		t.Fatalf("Probe: %v", err)
	}
	if report.Binary.Status != ProbePassed || report.Binary.Version != "1.0.0" {
		t.Fatalf("binary probe = %+v", report.Binary)
	}
	if report.Auth.Status != ProbeUnknown || report.Auth.Required {
		t.Fatalf("auth probe = %+v, want unprobed unknown", report.Auth)
	}
	if report.ModelProbe.Status != ProbePassed {
		t.Fatalf("model probe = %+v", report.ModelProbe)
	}
	if err := ValidatePreflight(expected, report); err != nil {
		t.Fatalf("ValidatePreflight: %v", err)
	}
}

func TestProbeClientDoesNotTreatFallbackModelsAsObservedAvailability(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		fmt.Fprint(w, `{
			"agents":[{
				"id":"opencode",
				"available":true,
				"version":"1.0.0",
				"modelsSource":"fallback",
				"models":[{"id":"anthropic/claude-sonnet-4-5","label":"Claude Sonnet"}]
			}]
		}`)
	}))
	defer server.Close()

	expected := ExpectedPreflight{
		Engine:    PinnedEngineIdentity(),
		AdapterID: "opencode",
		Model:     "anthropic/claude-sonnet-4-5",
	}
	client, err := NewProbeClient(server.URL, "", server.Client())
	if err != nil {
		t.Fatalf("NewProbeClient: %v", err)
	}
	report, err := client.Probe(context.Background(), expected, PluginsDisabled)
	if err != nil {
		t.Fatalf("Probe: %v", err)
	}
	if report.ModelProbe.Status != ProbeFailed {
		t.Fatalf("model probe = %+v, want failed fallback evidence", report.ModelProbe)
	}
	if err := ValidatePreflight(expected, report); err == nil || !strings.Contains(err.Error(), "model preflight") {
		t.Fatalf("ValidatePreflight error = %v, want model preflight failure", err)
	}
}

func TestNewProbeClientRejectsNonLoopbackWorkerURL(t *testing.T) {
	if _, err := NewProbeClient("https://example.com", "secret", http.DefaultClient); err == nil {
		t.Fatal("NewProbeClient accepted a non-loopback worker URL")
	}
}
