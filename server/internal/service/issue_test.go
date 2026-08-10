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
