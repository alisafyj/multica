package scheduler

import (
	"context"
	"time"
)

// JobNamePMOSyncDispatch is the canonical audit-row name of the PMO
// requirement sync dispatcher. Stable across releases — do not rename
// without a migration of sys_cron_executions rows.
const JobNamePMOSyncDispatch = "pmo_sync_dispatch"

// PMOSyncDispatcher claims due PMO configurations and enqueues their
// scheduled runs. PMOService implements it; the scheduler stays
// dependency-light by programming against this interface.
type PMOSyncDispatcher interface {
	DispatchDuePMORuns(ctx context.Context) (int, error)
}

// PMOSyncDispatchJob returns the JobSpec for the minute-cadence global
// scanner that dispatches due PMO sync configurations.
//
// The handler itself is a thin delegation: the claim semantics (single
// winner per config, missed intervals collapsing to the latest, the
// active-run guard, and the first-apply gate) live in
// PMOService.DispatchDuePMORuns, so a duplicate tick or a re-entrant
// stale-lease attempt is just another dispatcher invocation — the SQL
// claim decides.
//
// Spec settings: the dispatcher does real work (config lock + run +
// agent-task enqueue per config), so the run timeout is generously sized
// but bounded well under the scheduler's stale window. Latest-only
// catch-up plus the service-side collapse means a long gap produces at
// most one run per config, matching the fixed 30-minute design.
func PMOSyncDispatchJob(dispatcher PMOSyncDispatcher) JobSpec {
	return JobSpec{
		Name:              JobNamePMOSyncDispatch,
		Cadence:           time.Minute,
		CatchUpMode:       CatchUpLatestOnly,
		CatchUpWindow:     24 * time.Hour,
		RunTimeout:        50 * time.Second,
		StaleTimeout:      2 * time.Minute,
		HeartbeatInterval: 20 * time.Second,
		AllowStaleReentry: true,
		MaxAttempts:       3,
		RetryBackoff: []time.Duration{
			time.Minute,
			5 * time.Minute,
			15 * time.Minute,
		},
		Scopes: StaticScopes(ScopeGlobal),
		Handler: func(ctx context.Context, _ HandlerInput) (HandlerResult, error) {
			count, err := dispatcher.DispatchDuePMORuns(ctx)
			return HandlerResult{RowsAffected: int64(count)}, err
		},
	}
}
