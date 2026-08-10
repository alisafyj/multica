package scheduler

import (
	"context"
	"testing"
	"time"
)

// fakePMODispatcher records calls and returns a canned dispatch count so
// the job spec shape + delegation can be verified without a database.
type fakePMODispatcher struct {
	calls int
	count int
	err   error
}

func (f *fakePMODispatcher) DispatchDuePMORuns(_ context.Context) (int, error) {
	f.calls++
	return f.count, f.err
}

func TestPMOScheduleDispatchJobSpecShape(t *testing.T) {
	dispatcher := &fakePMODispatcher{count: 3}
	spec := PMOSyncDispatchJob(dispatcher)

	if spec.Name != JobNamePMOSyncDispatch {
		t.Fatalf("name = %q, want %q", spec.Name, JobNamePMOSyncDispatch)
	}
	if spec.Cadence != time.Minute {
		t.Fatalf("cadence = %s, want 1m", spec.Cadence)
	}
	if spec.CatchUpMode != CatchUpLatestOnly {
		t.Fatalf("catch-up mode = %s, want latest_only", spec.CatchUpMode)
	}
	if err := spec.validate(); err != nil {
		t.Fatalf("spec invalid: %v", err)
	}
	scopes, err := spec.Scopes(context.Background(), time.Now())
	if err != nil || len(scopes) != 1 || scopes[0] != ScopeGlobal {
		t.Fatalf("scopes = %v, %v; want exactly [ScopeGlobal]", scopes, err)
	}

	// Handler delegates to DispatchDuePMORuns and reports the dispatched
	// count on the audit row.
	res, err := spec.Handler(context.Background(), HandlerInput{Job: &spec})
	if err != nil {
		t.Fatalf("handler: %v", err)
	}
	if dispatcher.calls != 1 {
		t.Fatalf("dispatcher calls = %d, want 1", dispatcher.calls)
	}
	if res.RowsAffected != 3 {
		t.Fatalf("rows_affected = %d, want 3", res.RowsAffected)
	}
}
