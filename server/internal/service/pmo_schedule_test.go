package service

import (
	"context"
	"fmt"
	"sync"
	"testing"
	"time"

	"github.com/jackc/pgx/v5/pgtype"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/multica-ai/multica/server/internal/util"
	db "github.com/multica-ai/multica/server/pkg/db/generated"
)

// pmoDispatchFixture seeds a workspace + agent + one PMO config for
// DispatchDuePMORuns tests. Everything fictional; cleaned up on exit.
type pmoDispatchFixture struct {
	pool        *pgxpool.Pool
	svc         *PMOService
	workspaceID pgtype.UUID
	config      db.PmoSyncConfig
}

func newPMODispatchFixture(t *testing.T) pmoDispatchFixture {
	t.Helper()
	ctx := context.Background()
	pool := issueServiceTestPool(t)
	suffix := time.Now().UnixNano()

	var ownerID string
	if err := pool.QueryRow(ctx, `INSERT INTO "user" (name, email) VALUES ('PMO Dispatch Owner', $1) RETURNING id`,
		fmt.Sprintf("pmo-dispatch-%d@example.test", suffix)).Scan(&ownerID); err != nil {
		t.Fatalf("create owner: %v", err)
	}
	var workspaceID string
	if err := pool.QueryRow(ctx, `INSERT INTO workspace (name, slug, description, issue_prefix) VALUES ('PMO Dispatch', $1, 'temp', 'PMD') RETURNING id`,
		fmt.Sprintf("pmo-dispatch-%d", suffix)).Scan(&workspaceID); err != nil {
		t.Fatalf("create workspace: %v", err)
	}
	if _, err := pool.Exec(ctx, `INSERT INTO member (workspace_id, user_id, role) VALUES ($1, $2, 'owner')`, workspaceID, ownerID); err != nil {
		t.Fatalf("create member: %v", err)
	}
	var runtimeID string
	if err := pool.QueryRow(ctx, `
		INSERT INTO agent_runtime (workspace_id, name, runtime_mode, provider, status, device_info, metadata, owner_id, last_seen_at)
		VALUES ($1, 'PMO Dispatch Runtime', 'cloud', 'pmo_dispatch_runtime', 'online', 'pmo dispatch runtime', '{}'::jsonb, $2, now())
		RETURNING id
	`, workspaceID, ownerID).Scan(&runtimeID); err != nil {
		t.Fatalf("create runtime: %v", err)
	}
	var agentID string
	if err := pool.QueryRow(ctx, `
		INSERT INTO agent (workspace_id, name, description, runtime_mode, runtime_config, runtime_id, visibility, permission_mode, max_concurrent_tasks, owner_id)
		VALUES ($1, 'PMO Dispatch Agent', '', 'cloud', '{}'::jsonb, $2, 'private', 'private', 1, $3)
		RETURNING id
	`, workspaceID, runtimeID, ownerID).Scan(&agentID); err != nil {
		t.Fatalf("create agent: %v", err)
	}

	queries := db.New(pool)
	wsUUID := mustParsePMOUUID(t, workspaceID)
	config, err := queries.CreatePMOSyncConfig(ctx, db.CreatePMOSyncConfigParams{
		WorkspaceID:     wsUUID,
		Name:            "PMO Dispatch Config",
		AgentID:         mustParsePMOUUID(t, agentID),
		RootExternalKey: fmt.Sprintf("EXT-P-%d", suffix),
		CreatedBy:       mustParsePMOUUID(t, ownerID),
	})
	if err != nil {
		t.Fatalf("create config: %v", err)
	}

	t.Cleanup(func() {
		bg := context.Background()
		_, _ = pool.Exec(bg, `DELETE FROM agent_task_queue WHERE runtime_id = $1`, runtimeID)
		_, _ = pool.Exec(bg, `DELETE FROM pmo_sync_run WHERE config_id = $1`, config.ID)
		_, _ = pool.Exec(bg, `DELETE FROM pmo_sync_link WHERE config_id = $1`, config.ID)
		_, _ = pool.Exec(bg, `DELETE FROM pmo_sync_config WHERE id = $1`, config.ID)
		_, _ = pool.Exec(bg, `DELETE FROM agent WHERE id = $1`, agentID)
		_, _ = pool.Exec(bg, `DELETE FROM agent_runtime WHERE id = $1`, runtimeID)
		_, _ = pool.Exec(bg, `DELETE FROM member WHERE workspace_id = $1`, wsUUID)
		_, _ = pool.Exec(bg, `DELETE FROM workspace WHERE id = $1`, wsUUID)
		_, _ = pool.Exec(bg, `DELETE FROM "user" WHERE id = $1`, ownerID)
	})

	return pmoDispatchFixture{
		pool:        pool,
		svc:         NewPMOService(queries, pool, nil),
		workspaceID: wsUUID,
		config:      config,
	}
}

func mustParsePMOUUID(t *testing.T, s string) pgtype.UUID {
	t.Helper()
	u, err := util.ParseUUID(s)
	if err != nil {
		t.Fatalf("parse uuid %q: %v", s, err)
	}
	return u
}

// seedScheduleState stamps the schedule knobs on the fixture config.
func (f pmoDispatchFixture) seedScheduleState(t *testing.T, enabled bool, nextRun *time.Time, applied bool) {
	t.Helper()
	var nextAt, appliedAt *time.Time
	nextAt = nextRun
	if applied {
		now := time.Now()
		appliedAt = &now
	}
	if _, err := f.pool.Exec(context.Background(), `
		UPDATE pmo_sync_config
		SET schedule_enabled = $2, next_run_at = $3, last_applied_at = $4
		WHERE id = $1
	`, f.config.ID, enabled, nextAt, appliedAt); err != nil {
		t.Fatalf("seed schedule state: %v", err)
	}
}

func (f pmoDispatchFixture) configRow(t *testing.T) db.PmoSyncConfig {
	t.Helper()
	config, err := f.svc.Queries.GetPMOSyncConfig(context.Background(), db.GetPMOSyncConfigParams{
		ID: f.config.ID, WorkspaceID: f.workspaceID,
	})
	if err != nil {
		t.Fatalf("read config: %v", err)
	}
	return config
}

func (f pmoDispatchFixture) runCount(t *testing.T) int {
	t.Helper()
	var n int
	if err := f.pool.QueryRow(context.Background(),
		`SELECT count(*) FROM pmo_sync_run WHERE config_id = $1`, f.config.ID).Scan(&n); err != nil {
		t.Fatalf("count runs: %v", err)
	}
	return n
}

func TestDispatchDuePMORunsCreatesScheduledRun(t *testing.T) {
	f := newPMODispatchFixture(t)
	ctx := context.Background()
	due := time.Now().Add(-5 * time.Minute)
	f.seedScheduleState(t, true, &due, true)

	count, err := f.svc.DispatchDuePMORuns(ctx)
	if err != nil {
		t.Fatalf("dispatch: %v", err)
	}
	if count != 1 {
		t.Fatalf("dispatched = %d, want 1", count)
	}
	if f.runCount(t) != 1 {
		t.Fatalf("run rows = %d, want 1", f.runCount(t))
	}

	// next_run_at advanced to ~now()+30m, not stale time + 30m.
	config := f.configRow(t)
	if !config.NextRunAt.Valid {
		t.Fatal("next_run_at not set after dispatch")
	}
	want := time.Now().Add(30 * time.Minute)
	if config.NextRunAt.Time.Before(want.Add(-2*time.Minute)) || config.NextRunAt.Time.After(want.Add(2*time.Minute)) {
		t.Fatalf("next_run_at = %v, want near %v", config.NextRunAt.Time, want)
	}
	var trigger, status string
	if err := f.pool.QueryRow(ctx, `SELECT trigger, status FROM pmo_sync_run WHERE config_id = $1 LIMIT 1`, f.config.ID).Scan(&trigger, &status); err != nil {
		t.Fatalf("read run: %v", err)
	}
	if trigger != "scheduled" || status != "queued" {
		t.Fatalf("run trigger/status = %q/%q, want scheduled/queued", trigger, status)
	}
}

func TestDispatchDuePMORunsSkipsIneligibleConfigs(t *testing.T) {
	f := newPMODispatchFixture(t)
	ctx := context.Background()
	due := time.Now().Add(-5 * time.Minute)

	// Disabled config.
	f.seedScheduleState(t, false, &due, true)
	count, err := f.svc.DispatchDuePMORuns(ctx)
	if err != nil {
		t.Fatalf("dispatch disabled: %v", err)
	}
	if count != 0 || f.runCount(t) != 0 {
		t.Fatalf("disabled config dispatched: count=%d runs=%d", count, f.runCount(t))
	}

	// First-apply guard: schedule enabled but never applied.
	f.seedScheduleState(t, true, &due, false)
	count, err = f.svc.DispatchDuePMORuns(ctx)
	if err != nil {
		t.Fatalf("dispatch unapplied: %v", err)
	}
	if count != 0 || f.runCount(t) != 0 {
		t.Fatalf("unapplied config dispatched: count=%d runs=%d", count, f.runCount(t))
	}

	// Not yet due.
	future := time.Now().Add(30 * time.Minute)
	f.seedScheduleState(t, true, &future, true)
	count, err = f.svc.DispatchDuePMORuns(ctx)
	if err != nil {
		t.Fatalf("dispatch future: %v", err)
	}
	if count != 0 || f.runCount(t) != 0 {
		t.Fatalf("future config dispatched: count=%d runs=%d", count, f.runCount(t))
	}
}

// TestDispatchDuePMORunsTwoRunnersSingleWinner proves the claim query's
// single-winner semantics against the real DB: two concurrent dispatchers
// against one due config produce exactly one run.
func TestDispatchDuePMORunsTwoRunnersSingleWinner(t *testing.T) {
	f := newPMODispatchFixture(t)
	due := time.Now().Add(-5 * time.Minute)
	f.seedScheduleState(t, true, &due, true)

	var wg sync.WaitGroup
	var mu sync.Mutex
	total := 0
	for i := 0; i < 2; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			count, err := f.svc.DispatchDuePMORuns(context.Background())
			mu.Lock()
			defer mu.Unlock()
			if err != nil {
				t.Errorf("dispatch: %v", err)
				return
			}
			total += count
		}()
	}
	wg.Wait()

	if total != 1 {
		t.Fatalf("total dispatched = %d, want exactly 1", total)
	}
	if f.runCount(t) != 1 {
		t.Fatalf("run rows = %d, want 1", f.runCount(t))
	}
}

func TestDispatchDuePMORunsSkipsActiveRun(t *testing.T) {
	f := newPMODispatchFixture(t)
	ctx := context.Background()
	due := time.Now().Add(-5 * time.Minute)
	f.seedScheduleState(t, true, &due, true)

	// Seed an active (queued) run via StartRun: dispatch must not create a
	// second one for the same config.
	if _, err := f.svc.StartRun(ctx, f.workspaceID, f.config.ID, f.config.CreatedBy, "manual"); err != nil {
		t.Fatalf("seed active run: %v", err)
	}
	count, err := f.svc.DispatchDuePMORuns(ctx)
	if err != nil {
		t.Fatalf("dispatch with active run: %v", err)
	}
	if count != 0 {
		t.Fatalf("dispatched = %d with an active run, want 0", count)
	}
	if f.runCount(t) != 1 {
		t.Fatalf("run rows = %d, want 1", f.runCount(t))
	}
}

func TestDispatchDuePMORunsCollapsesMissedIntervals(t *testing.T) {
	f := newPMODispatchFixture(t)
	due := time.Now().Add(-6 * time.Hour)
	f.seedScheduleState(t, true, &due, true)

	count, err := f.svc.DispatchDuePMORuns(context.Background())
	if err != nil {
		t.Fatalf("dispatch: %v", err)
	}
	if count != 1 {
		t.Fatalf("dispatched = %d, want 1 (missed intervals collapse)", count)
	}
	if f.runCount(t) != 1 {
		t.Fatalf("run rows = %d, want 1", f.runCount(t))
	}
	config := f.configRow(t)
	want := time.Now().Add(30 * time.Minute)
	if config.NextRunAt.Time.Before(want.Add(-2*time.Minute)) || config.NextRunAt.Time.After(want.Add(2*time.Minute)) {
		t.Fatalf("next_run_at = %v, want near %v", config.NextRunAt.Time, want)
	}
}
