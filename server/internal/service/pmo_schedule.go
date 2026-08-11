package service

import (
	"context"
	"errors"
	"fmt"
	"log/slog"

	"github.com/jackc/pgx/v5"
	"github.com/multica-ai/multica/server/internal/util"
)

// DispatchDuePMORuns claims every due scheduled configuration and starts
// at most one queued scheduled run per claim. The fixed v1 cadence is 30
// minutes.
//
// Claim semantics (all enforced by ClaimDuePMOSyncConfig):
//   - eligibility: schedule_enabled AND last_applied_at set AND
//     next_run_at <= now() AND no active (queued/running) run;
//   - single winner: FOR UPDATE ... SKIP LOCKED, so concurrent dispatcher
//     ticks can never claim the same config;
//   - missed-interval collapse: next_run_at advances to now() + 30m,
//     never relative to the stale stored value, so downtime produces one
//     run per config at most.
//
// StartRun enforces the active-run guard again (defense in depth against
// a run appearing between claim and start) and stamps trigger_owner
// attribution for the scheduled trigger. ErrPMOActiveRun is skipped
// silently — the claim already advanced next_run_at. Returns the number
// of runs dispatched; a non-skip error aborts the tick (the scheduler
// retries per the job's backoff).
func (s *PMOService) DispatchDuePMORuns(ctx context.Context) (int, error) {
	dispatched := 0
	for {
		config, err := s.Queries.ClaimDuePMOSyncConfig(ctx)
		if err != nil {
			if errors.Is(err, pgx.ErrNoRows) {
				return dispatched, nil
			}
			return dispatched, fmt.Errorf("pmo dispatch: claim due config: %w", err)
		}

		if _, err := s.StartRun(ctx, config.WorkspaceID, config.ID, config.CreatedBy, "scheduled"); err != nil {
			if errors.Is(err, ErrPMOActiveRun) {
				// Another run appeared between the claim and the start;
				// next_run_at already advanced, so the next interval picks
				// the config up again. Not an error.
				continue
			}
			if errors.Is(err, ErrPMOAgentUnavailable) {
				// The selected agent is archived or lost its runtime. The
				// config stays claimable; nothing to escalate here.
				slog.Warn("pmo dispatch: agent unavailable, skipping config run this cycle",
					"config_id", util.UUIDToString(config.ID))
				continue
			}
			return dispatched, fmt.Errorf("pmo dispatch: start run: %w", err)
		}
		dispatched++
	}
}
