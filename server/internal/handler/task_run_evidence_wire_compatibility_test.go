package handler

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/multica-ai/multica/server/internal/daemon"
)

func TestTaskRunEvidenceModelUsageDaemonWireCompatibility(t *testing.T) {
	for _, negotiated := range []bool{false, true} {
		t.Run(map[bool]string{false: "legacy", true: "negotiated"}[negotiated], func(t *testing.T) {
			source := validTaskRunEvidenceRequest()
			body, err := json.Marshal(source)
			if err != nil {
				t.Fatal(err)
			}
			var request daemon.TaskRunEvidenceRequest
			if err := json.Unmarshal(body, &request); err != nil {
				t.Fatal(err)
			}
			if negotiated {
				request.ModelUsage = &daemon.TaskRunEvidenceModelUsageInventory{
					Entries: []daemon.TaskRunEvidenceModelUsage{{Model: "model-a",
						Usage: request.Usage, ProviderCost: request.ProviderCost,
					}}, Complete: true, Source: "model_usage",
				}
			}
			body, err = json.Marshal(request)
			if err != nil {
				t.Fatal(err)
			}
			response := httptest.NewRecorder()
			decoded, ok := decodeTaskRunEvidenceRequest(response,
				httptest.NewRequest(http.MethodPost, "/", bytes.NewReader(body)))
			if !ok {
				t.Fatalf("daemon DTO rejected: %s", response.Body.String())
			}
			if negotiated {
				if decoded.ModelUsage == nil || len(decoded.ModelUsage.Entries) != 1 ||
					decoded.ModelUsage.Entries[0].Model != "model-a" || !decoded.ModelUsage.Complete {
					t.Fatal("per-model evidence lost at daemon/handler boundary")
				}
			} else {
				var fields map[string]json.RawMessage
				if err := json.Unmarshal(body, &fields); err != nil {
					t.Fatal(err)
				}
				if _, present := fields["model_usage"]; present || decoded.ModelUsage != nil {
					t.Fatal("old-claim evidence gained the extension")
				}
			}
		})
	}
}
