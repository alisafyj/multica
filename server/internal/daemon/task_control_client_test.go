package daemon

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestTaskControlClientUsesOnlyClaimDaemonCredential(t *testing.T) {
	for _, token := range []string{"mdt_fixture_claim", ""} {
		t.Run(token, func(t *testing.T) {
			server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				want := ""
				if token != "" {
					want = "Bearer " + token
				}
				if r.Header.Get("Authorization") != want {
					t.Error("task control request used a credential other than its claim credential")
				}
				if r.Header.Get("X-Client-Version") != "fixture" || r.Header.Get("X-Client-Platform") != "daemon" {
					t.Error("task control request lost client identity")
				}
				w.WriteHeader(http.StatusNoContent)
			}))
			defer server.Close()
			parent := NewClient(server.URL)
			parent.token = "fixture-login-credential"
			parent.SetVersion("fixture")
			child := parent.withDaemonToken(token)
			if err := child.ReportTaskRunEvidence(context.Background(), "runtime", "task", TaskRunEvidenceRequest{}); err != nil {
				t.Fatal(err)
			}
			if parent.token != "fixture-login-credential" || child.client != parent.client {
				t.Fatal("task client changed login auth or did not preserve the pooled transport")
			}
		})
	}
}
