package daemon

import (
	"context"
	"encoding/json"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/multica-ai/multica/server/pkg/agent"
)

func TestTaskRunEvidenceModelUsageRequiresClaimCapability(t *testing.T) {
	for _, enabled := range []bool{false, true} {
		t.Run(map[bool]string{false: "old_server", true: "negotiated"}[enabled], func(t *testing.T) {
			task := taskRunEvidenceTestTask()
			claim, _ := json.Marshal(map[string]bool{"task_run_evidence_model_usage_v1": enabled})
			if err := json.Unmarshal(claim, &task); err != nil {
				t.Fatal(err)
			}
			recorder := newTaskRunEvidenceRecorder(nil, task, nil)
			before, err := json.Marshal(recorder.request)
			if err != nil {
				t.Fatal(err)
			}
			var body map[string]json.RawMessage
			if err := json.Unmarshal(before, &body); err != nil {
				t.Fatal(err)
			}
			_, present := body["model_usage"]
			if present != enabled {
				t.Fatalf("model_usage present=%v, negotiated=%v", present, enabled)
			}
			if enabled && string(body["model_usage"]) != `{"entries":[],"complete":false,"truncated":false,"source":"missing"}` {
				t.Fatalf("initial inventory = %s", body["model_usage"])
			}
			var evidence agent.ExecutionEvidence
			if err := json.Unmarshal([]byte(`{"ModelUsage":{"Entries":[{"Model":"model-a","Usage":{"InputUncachedTokens":7,"Source":"model_usage"},"ProviderCost":{"Source":"missing","Authority":"missing","Basis":"missing"}}],"Source":"model_usage"}}`), &evidence); err != nil {
				t.Fatal(err)
			}
			recorder.applyAgentEvidence(&evidence)
			after, err := json.Marshal(recorder.request)
			if err != nil {
				t.Fatal(err)
			}
			if err := json.Unmarshal(after, &body); err != nil {
				t.Fatal(err)
			}
			_, present = body["model_usage"]
			if present != enabled {
				t.Fatalf("final model_usage present=%v, negotiated=%v", present, enabled)
			}
			if enabled && !strings.Contains(string(body["model_usage"]), `"input_uncached_tokens":7`) {
				t.Fatalf("model usage was not projected: %s", body["model_usage"])
			}
		})
	}
}

func TestTaskRunEvidenceModelUsageCapabilityAdvertised(t *testing.T) {
	if !strings.Contains(","+daemonClientCapabilities()+",", ",task-run-evidence-model-usage-v1,") {
		t.Fatal("HTTP and WebSocket capability declaration is missing model usage support")
	}
}

func TestTaskRunEvidenceModelUsageDeepClone(t *testing.T) {
	one := int64(1)
	value := agent.ModelUsageInventoryEvidence{
		Entries: []agent.ModelUsageEvidence{{Model: "model-a",
			Usage: agent.UsageEvidence{InputUncachedTokens: &one, InputCacheReadTokens: &one,
				InputCacheWriteTokens: &one, OutputTokens: &one, Complete: true, Source: "model_usage"},
			ProviderCost: agent.ProviderCostEvidence{AmountUSDTicks: &one, Complete: true,
				Authority: "provider_reported", Basis: "provider_reported", Source: "model_usage"},
		}}, Complete: true, Source: "model_usage",
	}
	request := minimalTaskRunEvidenceRequest()
	request.ModelUsage = modelUsageFromAgent(value)
	one = 99
	if *request.ModelUsage.Entries[0].Usage.InputUncachedTokens != 1 {
		t.Fatal("projection aliases agent evidence")
	}
	clone := cloneTaskRunEvidenceRequest(request)
	entry := &request.ModelUsage.Entries[0]
	entry.Model = "changed"
	for _, number := range []*int64{entry.Usage.InputUncachedTokens, entry.Usage.InputCacheReadTokens,
		entry.Usage.InputCacheWriteTokens, entry.Usage.OutputTokens, entry.ProviderCost.AmountUSDTicks} {
		*number = 22
	}
	got := clone.ModelUsage.Entries[0]
	if got.Model != "model-a" {
		t.Fatal("snapshot aliases model entry")
	}
	for _, number := range []*int64{got.Usage.InputUncachedTokens, got.Usage.InputCacheReadTokens,
		got.Usage.InputCacheWriteTokens, got.Usage.OutputTokens, got.ProviderCost.AmountUSDTicks} {
		if *number != 1 {
			t.Fatal("snapshot aliases model numeric evidence")
		}
	}
	if cloneTaskRunEvidenceRequest(minimalTaskRunEvidenceRequest()).ModelUsage != nil {
		t.Fatal("clone enabled the unnegotiated extension")
	}
}

func TestTaskRunEvidenceModelUsageHTTPDeclaration(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if !strings.Contains(","+r.Header.Get("X-Client-Capabilities")+",", ",task-run-evidence-model-usage-v1,") {
			t.Error("HTTP request lost the capability declaration")
		}
		w.WriteHeader(http.StatusOK)
	}))
	defer server.Close()
	if err := NewClient(server.URL).ReportTaskRunEvidence(context.Background(), "runtime", "task", minimalTaskRunEvidenceRequest()); err != nil {
		t.Fatal(err)
	}
}

func TestTaskRunEvidenceModelUsageWebSocketDeclaration(t *testing.T) {
	seen := make(chan string, 1)
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		seen <- r.Header.Get("X-Client-Capabilities")
		http.Error(w, "test handshake rejected", http.StatusUpgradeRequired)
	}))
	defer server.Close()
	d := New(Config{ServerBaseURL: server.URL}, slog.Default())
	ctx, cancel := context.WithTimeout(context.Background(), time.Second)
	defer cancel()
	if _, err := d.runTaskWakeupConnection(ctx, nil, make(chan taskWakeup, 1), make(chan struct{})); err == nil {
		t.Fatal("rejected test handshake unexpectedly succeeded")
	}
	select {
	case capabilities := <-seen:
		if !strings.Contains(","+capabilities+",", ",task-run-evidence-model-usage-v1,") {
			t.Fatal("WebSocket handshake lost model usage declaration")
		}
	default:
		t.Fatal("test handshake was not observed")
	}
}
