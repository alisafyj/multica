package daemon

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"sync"
	"testing"
	"time"
)

func minimalTaskRunEvidenceRequest() TaskRunEvidenceRequest {
	return TaskRunEvidenceRequest{
		SchemaVersion: taskRunEvidenceSchemaVersion,
		Attempt:       2, ClaimGeneration: 100, Revision: 1,
		ProviderReported: TaskRunEvidenceProviderModel{Source: "missing"},
		Usage:            TaskRunEvidenceUsage{Source: "missing"},
		ProviderCost:     TaskRunEvidenceProviderCost{Authority: "missing", Basis: "missing", Source: "missing"},
	}
}

func taskRunEvidenceTestTask() Task {
	return Task{ID: "task-1", RuntimeID: "runtime-1", ClaimAttempt: 2, ClaimGeneration: 100}
}

func TestReportTaskRunEvidencePostsBoundedVersionedDTO(t *testing.T) {
	var gotPath string
	var got TaskRunEvidenceRequest
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotPath = r.URL.Path
		if err := json.NewDecoder(r.Body).Decode(&got); err != nil {
			t.Errorf("decode body: %v", err)
		}
		w.WriteHeader(http.StatusOK)
	}))
	defer server.Close()

	client := NewClient(server.URL)
	if err := client.ReportTaskRunEvidence(context.Background(), "runtime-1", "task-1", minimalTaskRunEvidenceRequest()); err != nil {
		t.Fatalf("report evidence: %v", err)
	}
	if gotPath != "/api/daemon/runtimes/runtime-1/tasks/task-1/run-evidence" {
		t.Fatalf("path = %q", gotPath)
	}
	if got.SchemaVersion != taskRunEvidenceSchemaVersion || got.Attempt != 2 || got.ClaimGeneration != 100 || got.Revision != 1 {
		t.Fatalf("body = %+v", got)
	}
}

func TestTaskRunEvidenceRecorderDoesNotPublishWithoutClaimGeneration(t *testing.T) {
	client := &recordingTaskRunEvidenceClient{}
	task := taskRunEvidenceTestTask()
	task.ClaimGeneration = 0
	recorder := newTaskRunEvidenceRecorder(client, task, nil)
	recorder.beginPreparation(time.Unix(100, 0))
	recorder.finishFinalization(time.Unix(101, 0))

	if requests := client.snapshot(); len(requests) != 0 {
		t.Fatalf("requests = %d, want 0", len(requests))
	}
}

func TestReportTaskRunEvidenceDoesNotRetryOldServer404(t *testing.T) {
	attempts := 0
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		attempts++
		http.Error(w, "not found", http.StatusNotFound)
	}))
	defer server.Close()

	err := NewClient(server.URL).ReportTaskRunEvidence(context.Background(), "runtime-1", "task-1", minimalTaskRunEvidenceRequest())
	if err == nil {
		t.Fatal("404 report succeeded")
	}
	if attempts != 1 {
		t.Fatalf("attempts = %d, want 1", attempts)
	}
}

type recordingTaskRunEvidenceClient struct {
	mu       sync.Mutex
	requests []TaskRunEvidenceRequest
	err      error
}

func (c *recordingTaskRunEvidenceClient) ReportTaskRunEvidence(_ context.Context, _, _ string, request TaskRunEvidenceRequest) error {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.requests = append(c.requests, request)
	return c.err
}

func (c *recordingTaskRunEvidenceClient) snapshot() []TaskRunEvidenceRequest {
	c.mu.Lock()
	defer c.mu.Unlock()
	return append([]TaskRunEvidenceRequest(nil), c.requests...)
}

func TestTaskRunEvidenceRecorderPublishesMonotonicStageRevisions(t *testing.T) {
	client := &recordingTaskRunEvidenceClient{}
	recorder := newTaskRunEvidenceRecorder(client, taskRunEvidenceTestTask(), nil)
	start := time.Unix(100, 0)
	recorder.beginPreparation(start)
	recorder.beginExecution(start.Add(10 * time.Millisecond))
	recorder.firstTool(start.Add(15 * time.Millisecond))
	recorder.finishExecution(start.Add(30*time.Millisecond), nil)
	recorder.finishFinalization(start.Add(35 * time.Millisecond))

	requests := client.snapshot()
	if len(requests) == 0 {
		t.Fatal("no evidence requests observed")
	}
	for i := 1; i < len(requests); i++ {
		if requests[i].Revision <= requests[i-1].Revision {
			t.Fatalf("revisions are not monotonic: %+v", requests)
		}
	}
	final := requests[len(requests)-1]
	if final.Revision != 5 {
		t.Fatalf("final revision = %d, want 5", final.Revision)
	}
	if got := *final.Timings.Finalization.DurationMS; got != 5 {
		t.Fatalf("finalization duration = %dms, want 5ms", got)
	}
}

func TestTaskRunEvidenceRecorderStopsAfterOldServer404(t *testing.T) {
	client := &recordingTaskRunEvidenceClient{err: &requestError{StatusCode: http.StatusNotFound}}
	recorder := newTaskRunEvidenceRecorder(client, taskRunEvidenceTestTask(), nil)
	start := time.Unix(100, 0)
	recorder.beginPreparation(start)
	recorder.beginExecution(start.Add(time.Millisecond))
	recorder.finishRunTask(start.Add(2 * time.Millisecond))
	recorder.finishFinalization(start.Add(3 * time.Millisecond))

	if got := len(client.snapshot()); got != 1 {
		t.Fatalf("requests = %d, want one diagnostic probe", got)
	}
}

func TestTaskRunEvidenceRecorderKeepsTaskFlowIndependentWhenServerIsOffline(t *testing.T) {
	client := &recordingTaskRunEvidenceClient{err: errors.New("server offline")}
	recorder := newTaskRunEvidenceRecorder(client, taskRunEvidenceTestTask(), nil)
	start := time.Unix(100, 0)
	recorder.beginPreparation(start)
	recorder.beginExecution(start.Add(time.Millisecond))
	recorder.finishRunTask(start.Add(2 * time.Millisecond))
	recorder.finishFinalization(start.Add(3 * time.Millisecond))

	requests := client.snapshot()
	if len(requests) == 0 || requests[len(requests)-1].Revision != 4 {
		t.Fatalf("final request = %+v, want revision 4", requests)
	}
	select {
	case <-recorder.done:
	default:
		t.Fatal("worker still running after offline terminal upload")
	}
}

func TestCloneTaskRunEvidenceRequestDoesNotSharePointers(t *testing.T) {
	value := int64(7)
	model := "provider-model"
	request := minimalTaskRunEvidenceRequest()
	request.Timings.Execution = TaskRunEvidenceTiming{Known: true, DurationMS: &value}
	request.ProviderReported.Model = &model
	request.Usage.InputUncachedTokens = &value

	clone := cloneTaskRunEvidenceRequest(request)
	value = 99
	model = "mutated"

	if *clone.Timings.Execution.DurationMS != 7 || *clone.Usage.InputUncachedTokens != 7 || *clone.ProviderReported.Model != "provider-model" {
		t.Fatalf("clone changed after source pointer mutation: %+v", clone)
	}
}

type blockingTaskRunEvidenceClient struct {
	started chan struct{}
	release chan struct{}
	once    sync.Once
	mu      sync.Mutex
	seen    []TaskRunEvidenceRequest
}

type supersededTaskRunEvidenceClient struct {
	started chan struct{}
	once    sync.Once
	mu      sync.Mutex
	seen    []TaskRunEvidenceRequest
}

func (c *supersededTaskRunEvidenceClient) ReportTaskRunEvidence(ctx context.Context, _, _ string, request TaskRunEvidenceRequest) error {
	first := false
	c.once.Do(func() {
		first = true
		close(c.started)
	})
	if first {
		<-ctx.Done()
		return ctx.Err()
	}
	c.mu.Lock()
	c.seen = append(c.seen, request)
	c.mu.Unlock()
	return nil
}

func (c *blockingTaskRunEvidenceClient) ReportTaskRunEvidence(ctx context.Context, _, _ string, request TaskRunEvidenceRequest) error {
	c.once.Do(func() { close(c.started) })
	select {
	case <-c.release:
	case <-ctx.Done():
		return ctx.Err()
	}
	c.mu.Lock()
	c.seen = append(c.seen, request)
	c.mu.Unlock()
	return nil
}

func TestTaskRunEvidenceStageHooksDoNotBlockOnSlowServer(t *testing.T) {
	client := &blockingTaskRunEvidenceClient{started: make(chan struct{}), release: make(chan struct{})}
	recorder := newTaskRunEvidenceRecorder(client, taskRunEvidenceTestTask(), nil)
	start := time.Unix(100, 0)
	recorder.beginPreparation(start)
	select {
	case <-client.started:
	case <-time.After(time.Second):
		t.Fatal("worker did not start upload")
	}

	returned := make(chan struct{})
	go func() {
		recorder.beginExecution(start.Add(time.Millisecond))
		recorder.firstTool(start.Add(2 * time.Millisecond))
		close(returned)
	}()
	select {
	case <-returned:
	case <-time.After(100 * time.Millisecond):
		t.Fatal("stage hook blocked on slow evidence HTTP")
	}
	close(client.release)
	recorder.finishExecution(start.Add(3*time.Millisecond), nil)
	recorder.finishFinalization(start.Add(4 * time.Millisecond))
	client.mu.Lock()
	defer client.mu.Unlock()
	if len(client.seen) == 0 || client.seen[len(client.seen)-1].Revision != 5 {
		t.Fatalf("final slow-server snapshot = %+v, want revision 5", client.seen)
	}
}

func TestTaskRunEvidenceTerminalFlushObservesLatestSnapshotAndStopsWorker(t *testing.T) {
	client := &recordingTaskRunEvidenceClient{}
	recorder := newTaskRunEvidenceRecorder(client, taskRunEvidenceTestTask(), nil)
	start := time.Unix(100, 0)
	recorder.beginPreparation(start)
	recorder.beginExecution(start.Add(time.Millisecond))
	recorder.firstTool(start.Add(2 * time.Millisecond))
	recorder.finishExecution(start.Add(3*time.Millisecond), nil)
	recorder.finishFinalization(start.Add(4 * time.Millisecond))

	select {
	case <-recorder.done:
	default:
		t.Fatal("worker still running after terminal flush")
	}
	requests := client.snapshot()
	final := requests[len(requests)-1]
	if final.Revision != 5 || !final.Timings.Finalization.Known {
		t.Fatalf("final snapshot = %+v", final)
	}
}

func TestTaskRunEvidenceTerminalFlushCancelsSupersededUpload(t *testing.T) {
	client := &supersededTaskRunEvidenceClient{started: make(chan struct{})}
	recorder := newTaskRunEvidenceRecorder(client, taskRunEvidenceTestTask(), nil)
	start := time.Unix(100, 0)
	recorder.beginPreparation(start)
	select {
	case <-client.started:
	case <-time.After(time.Second):
		t.Fatal("worker did not start upload")
	}
	recorder.beginExecution(start.Add(time.Millisecond))
	recorder.finishRunTask(start.Add(2 * time.Millisecond))

	returned := make(chan struct{})
	go func() {
		recorder.finishFinalization(start.Add(3 * time.Millisecond))
		close(returned)
	}()
	select {
	case <-returned:
	case <-time.After(200 * time.Millisecond):
		t.Fatal("terminal flush waited for the superseded upload timeout")
	}
	client.mu.Lock()
	defer client.mu.Unlock()
	if len(client.seen) == 0 {
		t.Fatal("terminal snapshot was not uploaded")
	}
	final := client.seen[len(client.seen)-1]
	if final.Revision != 4 || !final.Timings.Finalization.Known {
		t.Fatalf("terminal snapshot = %+v", client.seen)
	}
}

func TestTaskRunEvidenceTerminalSnapshotCannotBeReplacedByLaterNonterminalProducer(t *testing.T) {
	client := &blockingTaskRunEvidenceClient{started: make(chan struct{}), release: make(chan struct{})}
	recorder := newTaskRunEvidenceRecorder(client, taskRunEvidenceTestTask(), nil)
	recorder.beginPreparation(time.Unix(100, 0))
	select {
	case <-client.started:
	case <-time.After(time.Second):
		t.Fatal("worker did not start upload")
	}

	recorder.mu.Lock()
	recorder.request.Timings.Finalization = knownEvidenceTiming(time.Millisecond)
	recorder.queueLatestLocked(true)
	terminalRevision := recorder.revision
	recorder.queueLatestLocked(false)
	if recorder.revision != terminalRevision {
		t.Fatalf("nonterminal producer advanced revision after terminal: got %d, want %d", recorder.revision, terminalRevision)
	}
	recorder.mu.Unlock()

	close(client.release)
	recorder.waitForWorker(time.Second)
	client.mu.Lock()
	defer client.mu.Unlock()
	final := client.seen[len(client.seen)-1]
	if final.Revision != terminalRevision || !final.Timings.Finalization.Known {
		t.Fatalf("terminal snapshot was replaced: %+v", final)
	}
}

func TestTaskRunEvidenceCoalescerDoesNotBlockWhenWakeupIsPending(t *testing.T) {
	client := &blockingTaskRunEvidenceClient{started: make(chan struct{}), release: make(chan struct{})}
	recorder := newTaskRunEvidenceRecorder(client, taskRunEvidenceTestTask(), nil)
	recorder.beginPreparation(time.Unix(100, 0))
	select {
	case <-client.started:
	case <-time.After(time.Second):
		t.Fatal("worker did not start upload")
	}

	queued := make(chan struct{})
	go func() {
		for range 1_000 {
			recorder.mu.Lock()
			recorder.queueLatestLocked(false)
			recorder.mu.Unlock()
		}
		close(queued)
	}()
	select {
	case <-queued:
	case <-time.After(100 * time.Millisecond):
		t.Fatal("coalescer blocked while its wakeup was already pending")
	}

	recorder.mu.Lock()
	recorder.queueLatestLocked(true)
	recorder.mu.Unlock()
	close(client.release)
	recorder.waitForWorker(time.Second)
}

func TestTaskRunEvidenceCloseCancelsAnInFlightUpload(t *testing.T) {
	client := &blockingTaskRunEvidenceClient{started: make(chan struct{}), release: make(chan struct{})}
	recorder := newTaskRunEvidenceRecorder(client, taskRunEvidenceTestTask(), nil)
	recorder.beginPreparation(time.Unix(100, 0))
	select {
	case <-client.started:
	case <-time.After(time.Second):
		t.Fatal("worker did not start upload")
	}

	returned := make(chan struct{})
	go func() {
		recorder.close()
		close(returned)
	}()
	select {
	case <-returned:
	case <-time.After(100 * time.Millisecond):
		t.Fatal("recorder close did not cancel the in-flight upload")
	}
	select {
	case <-recorder.done:
	default:
		t.Fatal("worker still running after recorder close")
	}
}

func TestVerifiedRuntimeIdentityHashesOnlyConfiguredAbsoluteExecutable(t *testing.T) {
	path := filepath.Join(t.TempDir(), "runtime")
	if err := os.WriteFile(path, []byte("runtime-binary"), 0o700); err != nil {
		t.Fatalf("write runtime: %v", err)
	}
	version, digest, ok := verifiedRuntimeIdentity("0.153.4", path)
	if !ok || version != "0.153.4" || digest != "sha256:ee8a920cfb4f37eaac14068653ef293301fd7f3334c15552afc662491218f5db" {
		t.Fatalf("identity = (%q, %q, %v)", version, digest, ok)
	}
	if _, _, ok := verifiedRuntimeIdentity("0.153.4", "runtime"); ok {
		t.Fatal("relative PATH lookup was accepted as verified runtime identity")
	}
}

func TestVerifiedRuntimeIdentityRejectsNonRegularAndOversizedFiles(t *testing.T) {
	if _, _, ok := verifiedRuntimeIdentity("0.153.4", t.TempDir()); ok {
		t.Fatal("directory was accepted as executable identity")
	}
	path := filepath.Join(t.TempDir(), "oversized")
	file, err := os.Create(path)
	if err != nil {
		t.Fatalf("create oversized file: %v", err)
	}
	if err := file.Truncate(maxRuntimeIdentityBytes + 1); err != nil {
		t.Fatalf("truncate oversized file: %v", err)
	}
	_ = file.Close()
	if _, _, ok := verifiedRuntimeIdentity("0.153.4", path); ok {
		t.Fatal("oversized executable was hashed")
	}
}

func TestVerifiedRuntimeIdentityInvalidatesCacheWhenExecutableChanges(t *testing.T) {
	path := filepath.Join(t.TempDir(), "runtime")
	if err := os.WriteFile(path, []byte("runtime-one"), 0o700); err != nil {
		t.Fatalf("write first runtime: %v", err)
	}
	_, firstDigest, firstOK := verifiedRuntimeIdentity("0.153.4", path)
	if !firstOK {
		t.Fatal("first runtime identity was not verified")
	}
	if err := os.WriteFile(path, []byte("runtime-two"), 0o700); err != nil {
		t.Fatalf("write second runtime: %v", err)
	}
	changedTime := time.Now().Add(time.Second)
	if err := os.Chtimes(path, changedTime, changedTime); err != nil {
		t.Fatalf("change runtime timestamp: %v", err)
	}
	_, secondDigest, secondOK := verifiedRuntimeIdentity("0.153.4", path)
	if !secondOK || secondDigest == firstDigest {
		t.Fatalf("changed identity = (%q, %v), first digest = %q", secondDigest, secondOK, firstDigest)
	}
}
