package handler

import (
	"context"
	"encoding/base64"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/multica-ai/multica/server/internal/testutil"
)

// The two pieces of live test-host state the server keeps only in memory:
// the daemon's device hub summary and the last frame of a running case.

func TestDeviceHubReportIsServedToTheRuntimeOwnerWithPairing(t *testing.T) {
	runtimeID := dbfx.Runtime(t, "hub-runtime", testutil.Cols{"daemon_id": "daemon-hub-report"})

	report := newDaemonTokenRequest("POST", "/api/daemon/runtimes/"+runtimeID+"/capabilities", map[string]any{
		"capabilities": []any{},
		"device_hub": map[string]any{
			"reachable":    true,
			"url":          "http://127.0.0.1:18801",
			"version":      "0.1.0",
			"adb":          true,
			"devices":      2,
			"phones":       1,
			"leases":       0,
			"pairing_url":  "ws://10.0.0.5:18800/phone?code=ABCD2345",
			"pairing_code": "ABCD2345",
		},
	}, testWorkspaceID, "daemon-hub-report")
	w := httptest.NewRecorder()
	testHandler.ReportRuntimeCapabilities(w, withURLParam(report, "runtimeId", runtimeID))
	if w.Code != http.StatusOK {
		t.Fatalf("report: got %d: %s", w.Code, w.Body.String())
	}

	w = httptest.NewRecorder()
	testHandler.GetRuntimeDeviceHub(w, withURLParam(newRequest("GET", "/api/runtimes/"+runtimeID+"/device-hub", nil), "id", runtimeID))
	if w.Code != http.StatusOK {
		t.Fatalf("get device hub: got %d: %s", w.Code, w.Body.String())
	}
	var resp runtimeDeviceHubResponse
	if err := json.NewDecoder(w.Body).Decode(&resp); err != nil {
		t.Fatal(err)
	}
	if !resp.Reachable || resp.Phones != 1 || resp.Devices != 2 || resp.Version != "0.1.0" {
		t.Errorf("device hub response = %+v, want the reported summary", resp)
	}
	// The test user owns the runtime, so the pairing code is included.
	if resp.PairingCode == nil || *resp.PairingCode != "ABCD2345" || resp.PairingURL == nil {
		t.Errorf("pairing for the owner = (%v, %v), want the reported code and URL", resp.PairingURL, resp.PairingCode)
	}
	if resp.ReportedAt == nil {
		t.Error("reported_at missing")
	}
}

func TestInMemoryDeviceHubStoreForgetsStaleReports(t *testing.T) {
	store := NewInMemoryDeviceHubStore()
	ctx := context.Background()
	if err := store.Set(ctx, "d1", RuntimeDeviceHubReport{Reachable: true, ReportedAt: time.Now().Add(-deviceHubReportRetention - time.Minute)}); err != nil {
		t.Fatal(err)
	}
	if got, _ := store.Get(ctx, "d1"); got != nil {
		t.Errorf("a report older than the retention must read as unknown, got %+v", got)
	}
	if err := store.Set(ctx, "d1", RuntimeDeviceHubReport{Reachable: true, ReportedAt: time.Now()}); err != nil {
		t.Fatal(err)
	}
	if got, _ := store.Get(ctx, "d1"); got == nil || !got.Reachable {
		t.Errorf("a fresh report must be served, got %+v", got)
	}
}

func TestLiveFrameIsRelayedByTheDaemonAndServedWithAnETag(t *testing.T) {
	projectID := newTestRunProject(t)
	tc := createTestCaseForRun(t, projectID)
	run := createTestRunFromCases(t, "Live frame run", []string{tc.ID})
	runtimeID := dbfx.Runtime(t, "frame-runtime", testutil.Cols{"daemon_id": "daemon-frame"})
	cases := runCasesInOrder(t, run.ID)
	runCaseID := cases[0].id

	// Nothing relayed yet.
	w := httptest.NewRecorder()
	testHandler.GetTestRunCaseFrame(w, withURLParam(newRequest("GET", "/api/test-run-cases/"+runCaseID+"/frame", nil), "id", runCaseID))
	if w.Code != http.StatusNotFound {
		t.Fatalf("frame before any relay: got %d, want 404", w.Code)
	}

	jpeg := []byte{0xFF, 0xD8, 0xFF, 0xE0, 0x00, 0x10, 'J', 'F', 'I', 'F', 0xFF, 0xD9}
	relay := newDaemonTokenRequest("POST", "/api/daemon/runtimes/"+runtimeID+"/test-run-cases/"+runCaseID+"/frame", map[string]any{
		"jpeg_base64": base64.StdEncoding.EncodeToString(jpeg),
		"hash":        "abcd1234",
		"captured_at": time.Now().UnixMilli(),
		"lease_id":    "lease-1",
	}, testWorkspaceID, "daemon-frame")
	w = httptest.NewRecorder()
	testHandler.ReportTestRunCaseFrame(w, withURLParams(relay, "runtimeId", runtimeID, "runCaseId", runCaseID))
	if w.Code != http.StatusNoContent {
		t.Fatalf("relay: got %d: %s", w.Code, w.Body.String())
	}

	w = httptest.NewRecorder()
	testHandler.GetTestRunCaseFrame(w, withURLParam(newRequest("GET", "/api/test-run-cases/"+runCaseID+"/frame", nil), "id", runCaseID))
	if w.Code != http.StatusOK {
		t.Fatalf("frame: got %d: %s", w.Code, w.Body.String())
	}
	if ct := w.Header().Get("Content-Type"); ct != "image/jpeg" {
		t.Errorf("content-type = %q, want image/jpeg", ct)
	}
	if etag := w.Header().Get("ETag"); etag != `"abcd1234"` {
		t.Errorf("etag = %q, want the hub's frame hash", etag)
	}
	if w.Body.Len() != len(jpeg) {
		t.Errorf("body = %d bytes, want %d", w.Body.Len(), len(jpeg))
	}

	// A poller that already has this frame pays nothing.
	req := withURLParam(newRequest("GET", "/api/test-run-cases/"+runCaseID+"/frame", nil), "id", runCaseID)
	req.Header.Set("If-None-Match", `"abcd1234"`)
	w = httptest.NewRecorder()
	testHandler.GetTestRunCaseFrame(w, req)
	if w.Code != http.StatusNotModified {
		t.Errorf("frame with matching If-None-Match: got %d, want 304", w.Code)
	}

	// A daemon of another workspace cannot plant a frame on this case.
	foreign := newDaemonTokenRequest("POST", "/api/daemon/runtimes/"+runtimeID+"/test-run-cases/"+runCaseID+"/frame", map[string]any{
		"jpeg_base64": base64.StdEncoding.EncodeToString(jpeg),
		"hash":        "evil",
	}, "00000000-0000-0000-0000-000000000000", "daemon-elsewhere")
	w = httptest.NewRecorder()
	testHandler.ReportTestRunCaseFrame(w, withURLParams(foreign, "runtimeId", runtimeID, "runCaseId", runCaseID))
	if w.Code != http.StatusNotFound {
		t.Errorf("cross-workspace relay: got %d, want 404", w.Code)
	}
}
