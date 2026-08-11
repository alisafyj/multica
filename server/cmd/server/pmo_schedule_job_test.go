package main

import (
	"context"
	"testing"
	"time"

	"github.com/multica-ai/multica/server/internal/scheduler"
)

// TestPMOScheduleDispatchJobRegistration verifies the job spec registered
// with the server's scheduler manager: name, minute cadence, latest-only
// catch-up, and that its handler reaches the wired dispatcher.
func TestPMOScheduleDispatchJobRegistration(t *testing.T) {
	if testPool == nil {
		t.Skip("database not available")
	}

	dispatched := 0
	dispatcher := pmoDispatchFunc(func(_ context.Context) (int, error) {
		dispatched++
		return 7, nil
	})
	spec := scheduler.PMOSyncDispatchJob(dispatcher)

	if spec.Name != scheduler.JobNamePMOSyncDispatch {
		t.Fatalf("job name = %q, want %q", spec.Name, scheduler.JobNamePMOSyncDispatch)
	}
	if spec.Cadence != time.Minute {
		t.Fatalf("cadence = %s, want 1m", spec.Cadence)
	}
	if spec.CatchUpMode != scheduler.CatchUpLatestOnly {
		t.Fatalf("catch-up mode = %s, want latest_only", spec.CatchUpMode)
	}

	mgr := scheduler.NewManager(testPool, scheduler.Options{RunnerID: "pmo-registration-test"})
	if err := mgr.Register(spec); err != nil {
		t.Fatalf("register pmo_sync_dispatch: %v", err)
	}
	// Duplicate registration proves the name is stable + unique in the
	// registry.
	if err := mgr.Register(scheduler.PMOSyncDispatchJob(dispatcher)); err == nil {
		t.Fatal("second register of pmo_sync_dispatch must fail (duplicate name)")
	}

	res, err := spec.Handler(context.Background(), scheduler.HandlerInput{Job: &spec})
	if err != nil {
		t.Fatalf("handler: %v", err)
	}
	if dispatched != 1 || res.RowsAffected != 7 {
		t.Fatalf("handler delegation: dispatched=%d rows=%d, want 1/7", dispatched, res.RowsAffected)
	}
}

// pmoDispatchFunc adapts a plain function to scheduler.PMOSyncDispatcher.
type pmoDispatchFunc func(ctx context.Context) (int, error)

func (f pmoDispatchFunc) DispatchDuePMORuns(ctx context.Context) (int, error) {
	return f(ctx)
}
