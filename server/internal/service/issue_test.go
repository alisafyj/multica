package service

import (
	"context"
	"fmt"
	"os"
	"sync"
	"testing"
	"time"

	"github.com/jackc/pgx/v5/pgtype"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/multica-ai/multica/server/internal/events"
	"github.com/multica-ai/multica/server/internal/util"
	db "github.com/multica-ai/multica/server/pkg/db/generated"
	"github.com/multica-ai/multica/server/pkg/protocol"
)

func testDatabaseURL() string {
	if dbURL := os.Getenv("DATABASE_URL"); dbURL != "" {
		return dbURL
	}
	return "postgres://multica:multica@localhost:5432/multica?sslmode=disable"
}

// issueServiceTestPool connects to the worktree test database. Tests skip
// cleanly when the database is unavailable (matches the handler suite).
func issueServiceTestPool(t *testing.T) *pgxpool.Pool {
	t.Helper()
	dbURL := testDatabaseURL()
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	pool, err := pgxpool.New(ctx, dbURL)
	if err != nil {
		t.Skipf("database unavailable: %v", err)
	}
	if err := pool.Ping(ctx); err != nil {
		pool.Close()
		t.Skipf("database unreachable: %v", err)
	}
	t.Cleanup(pool.Close)
	return pool
}

// issueCreateFixture seeds a workspace + member for IssueService.Create
// tests and cleans everything up.
type issueCreateFixture struct {
	workspaceID string
	userID      string
}

func newIssueCreateFixture(t *testing.T, ctx context.Context, pool *pgxpool.Pool) issueCreateFixture {
	t.Helper()
	suffix := time.Now().UnixNano()
	var userID string
	if err := pool.QueryRow(ctx, `
		INSERT INTO "user" (name, email) VALUES ('Issue Create Test', $1) RETURNING id
	`, fmt.Sprintf("issue-create-%d@example.test", suffix)).Scan(&userID); err != nil {
		t.Fatalf("create user: %v", err)
	}
	var workspaceID string
	if err := pool.QueryRow(ctx, `
		INSERT INTO workspace (name, slug, description, issue_prefix)
		VALUES ('Issue Create Test', $1, 'temp issue create test workspace', 'ICT')
		RETURNING id
	`, fmt.Sprintf("issue-create-%d", suffix)).Scan(&workspaceID); err != nil {
		t.Fatalf("create workspace: %v", err)
	}
	t.Cleanup(func() {
		cleanup := context.Background()
		_, _ = pool.Exec(cleanup, `DELETE FROM issue WHERE workspace_id = $1`, workspaceID)
		_, _ = pool.Exec(cleanup, `DELETE FROM project WHERE workspace_id = $1`, workspaceID)
		_, _ = pool.Exec(cleanup, `DELETE FROM member WHERE workspace_id = $1`, workspaceID)
		_, _ = pool.Exec(cleanup, `DELETE FROM workspace WHERE id = $1`, workspaceID)
		_, _ = pool.Exec(cleanup, `DELETE FROM "user" WHERE id = $1`, userID)
	})
	return issueCreateFixture{workspaceID: workspaceID, userID: userID}
}

// TestIssueServiceCreateBehaviourLock guards the behavior the Task 6
// createInTx/afterCreate extraction must preserve: workspace-scoped
// numbering, parent validation, and exactly one issue:created event per
// successful create. Must pass BEFORE and AFTER the refactor.
func TestIssueServiceCreateBehaviourLock(t *testing.T) {
	ctx := context.Background()
	pool := issueServiceTestPool(t)
	fixture := newIssueCreateFixture(t, ctx, pool)
	workspaceUUID, _ := util.ParseUUID(fixture.workspaceID)
	userUUID, _ := util.ParseUUID(fixture.userID)

	bus := events.New()
	var mu sync.Mutex
	var createdEvents []events.Event
	bus.Subscribe(protocol.EventIssueCreated, func(e events.Event) {
		mu.Lock()
		defer mu.Unlock()
		createdEvents = append(createdEvents, e)
	})
	svc := NewIssueService(db.New(pool), pool, bus, nil, nil)

	create := func(title string, parent pgtype.UUID) (IssueCreateResult, error) {
		return svc.Create(ctx, IssueCreateParams{
			WorkspaceID:   workspaceUUID,
			Title:         title,
			Status:        "todo",
			Priority:      "medium",
			CreatorType:   "member",
			CreatorID:     userUUID,
			ParentIssueID: parent,
		}, IssueCreateOpts{})
	}

	// Numbering increments monotonically within the workspace.
	first, err := create("Behaviour lock first", pgtype.UUID{})
	if err != nil {
		t.Fatalf("create first issue: %v", err)
	}
	second, err := create("Behaviour lock second", pgtype.UUID{})
	if err != nil {
		t.Fatalf("create second issue: %v", err)
	}
	if second.Issue.Number != first.Issue.Number+1 {
		t.Fatalf("issue numbers %d then %d, want consecutive", first.Issue.Number, second.Issue.Number)
	}

	// A forged parent id in the same workspace is rejected without creating.
	forgedParent, _ := util.ParseUUID("0f2b6f6e-0000-4000-8000-000000000001")
	if _, err = create("Behaviour lock forged parent", forgedParent); err != ErrParentIssueNotFound {
		t.Fatalf("forged parent: want ErrParentIssueNotFound, got %v", err)
	}

	mu.Lock()
	count := len(createdEvents)
	mu.Unlock()
	if count != 2 {
		t.Fatalf("issue:created events = %d, want exactly 2 (one per successful create)", count)
	}

	// A failed create must not publish.
	beforeFailedRow, err := countWorkspaceIssues(ctx, pool, fixture.workspaceID)
	if err != nil {
		t.Fatalf("count issues: %v", err)
	}
	if _, err = create("Behaviour lock forged parent repeat", forgedParent); err == nil {
		t.Fatal("expected forged-parent create to fail")
	}
	afterFailedRow, err := countWorkspaceIssues(ctx, pool, fixture.workspaceID)
	if err != nil {
		t.Fatalf("count issues: %v", err)
	}
	if afterFailedRow != beforeFailedRow {
		t.Fatalf("failed create wrote rows: %d -> %d", beforeFailedRow, afterFailedRow)
	}
	mu.Lock()
	defer mu.Unlock()
	if len(createdEvents) != count {
		t.Fatalf("failed create published event: %d -> %d", count, len(createdEvents))
	}
}

func countWorkspaceIssues(ctx context.Context, pool *pgxpool.Pool, workspaceID string) (int, error) {
	var n int
	err := pool.QueryRow(ctx, `SELECT count(*) FROM issue WHERE workspace_id = $1`, workspaceID).Scan(&n)
	return n, err
}

// readyAgentFixture is a workspace + agent bound to an online runtime, the
// minimum an agent assignee needs to pass AgentReadiness — readiness reads
// the runtime row, not a live daemon connection.
type readyAgentFixture struct {
	workspaceID string
	userID      string
	agentID     string
}

func newReadyAgentFixture(t *testing.T, ctx context.Context, pool *pgxpool.Pool) readyAgentFixture {
	t.Helper()
	suffix := time.Now().UnixNano()
	var userID string
	if err := pool.QueryRow(ctx, `
		INSERT INTO "user" (name, email) VALUES ('Ready Agent Test', $1) RETURNING id
	`, fmt.Sprintf("ready-agent-%d@example.test", suffix)).Scan(&userID); err != nil {
		t.Fatalf("create user: %v", err)
	}
	var workspaceID string
	if err := pool.QueryRow(ctx, `
		INSERT INTO workspace (name, slug, description, issue_prefix)
		VALUES ('Ready Agent Test', $1, '', 'RAT')
		RETURNING id
	`, fmt.Sprintf("ready-agent-%d", suffix)).Scan(&workspaceID); err != nil {
		t.Fatalf("create workspace: %v", err)
	}
	var runtimeID string
	if err := pool.QueryRow(ctx, `
		INSERT INTO agent_runtime (
			workspace_id, daemon_id, name, runtime_mode, provider,
			status, device_info, metadata, last_seen_at, visibility, owner_id
		)
		VALUES ($1, NULL, 'Ready Agent Runtime', 'cloud', 'ready_agent_test',
		        'online', 'test runtime', '{}'::jsonb, now(), 'private', $2)
		RETURNING id
	`, workspaceID, userID).Scan(&runtimeID); err != nil {
		t.Fatalf("create runtime: %v", err)
	}
	var agentID string
	if err := pool.QueryRow(ctx, `
		INSERT INTO agent (
			workspace_id, name, description, runtime_mode, runtime_config,
			runtime_id, visibility, max_concurrent_tasks, owner_id
		)
		VALUES ($1, 'Ready Agent', '', 'cloud', '{}'::jsonb, $2, 'private', 1, $3)
		RETURNING id
	`, workspaceID, runtimeID, userID).Scan(&agentID); err != nil {
		t.Fatalf("create agent: %v", err)
	}
	t.Cleanup(func() {
		cleanup := context.Background()
		_, _ = pool.Exec(cleanup, `DELETE FROM agent_task_queue WHERE agent_id = $1`, agentID)
		_, _ = pool.Exec(cleanup, `DELETE FROM issue WHERE workspace_id = $1`, workspaceID)
		_, _ = pool.Exec(cleanup, `DELETE FROM agent WHERE id = $1`, agentID)
		_, _ = pool.Exec(cleanup, `DELETE FROM agent_runtime WHERE id = $1`, runtimeID)
		_, _ = pool.Exec(cleanup, `DELETE FROM workspace WHERE id = $1`, workspaceID)
		_, _ = pool.Exec(cleanup, `DELETE FROM "user" WHERE id = $1`, userID)
	})
	return readyAgentFixture{workspaceID: workspaceID, userID: userID, agentID: agentID}
}

// Ordinary behavior: assigning a ready agent at create time starts its run —
// this is the baseline SuppressAssigneeRun below is proven against.
func TestIssueServiceCreateWithAgentAssigneeEnqueuesARun(t *testing.T) {
	ctx := context.Background()
	pool := issueServiceTestPool(t)
	fixture := newReadyAgentFixture(t, ctx, pool)
	workspaceUUID, _ := util.ParseUUID(fixture.workspaceID)
	userUUID, _ := util.ParseUUID(fixture.userID)
	agentUUID, _ := util.ParseUUID(fixture.agentID)

	bus := events.New()
	svc := NewIssueService(db.New(pool), pool, bus, nil, NewTaskService(db.New(pool), pool, nil, bus))
	res, err := svc.Create(ctx, IssueCreateParams{
		WorkspaceID:  workspaceUUID,
		Title:        "Ordinary agent assignment",
		Status:       "todo",
		Priority:     "none",
		AssigneeType: pgtype.Text{String: "agent", Valid: true},
		AssigneeID:   agentUUID,
		CreatorType:  "member",
		CreatorID:    userUUID,
	}, IssueCreateOpts{})
	if err != nil {
		t.Fatalf("create: %v", err)
	}
	if !res.AssignedTaskID.Valid {
		t.Fatal("agent assignee at create time did not enqueue a run")
	}
}

// A caller that assigns an agent purely to record who owns the work — a
// companion issue traced from another run, not a dispatch instruction — must
// not also start an independent run. That second run is what wedged a design
// task behind an unrelated one racing it for the same local directory.
func TestIssueServiceCreateSuppressAssigneeRunSkipsTheEnqueue(t *testing.T) {
	ctx := context.Background()
	pool := issueServiceTestPool(t)
	fixture := newReadyAgentFixture(t, ctx, pool)
	workspaceUUID, _ := util.ParseUUID(fixture.workspaceID)
	userUUID, _ := util.ParseUUID(fixture.userID)
	agentUUID, _ := util.ParseUUID(fixture.agentID)

	bus := events.New()
	svc := NewIssueService(db.New(pool), pool, bus, nil, NewTaskService(db.New(pool), pool, nil, bus))
	res, err := svc.Create(ctx, IssueCreateParams{
		WorkspaceID:  workspaceUUID,
		Title:        "Suppressed agent assignment",
		Status:       "todo",
		Priority:     "none",
		AssigneeType: pgtype.Text{String: "agent", Valid: true},
		AssigneeID:   agentUUID,
		CreatorType:  "member",
		CreatorID:    userUUID,
	}, IssueCreateOpts{SuppressAssigneeRun: true})
	if err != nil {
		t.Fatalf("create: %v", err)
	}
	if res.AssignedTaskID.Valid {
		t.Fatalf("SuppressAssigneeRun did not stop the enqueue: task %v", res.AssignedTaskID)
	}
	var taskCount int
	if err := pool.QueryRow(ctx, `SELECT count(*) FROM agent_task_queue WHERE issue_id = $1`,
		res.Issue.ID).Scan(&taskCount); err != nil {
		t.Fatalf("count tasks for issue: %v", err)
	}
	if taskCount != 0 {
		t.Fatalf("issue has %d queued tasks, want none", taskCount)
	}
}
