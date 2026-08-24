package service

import (
	"context"
	"testing"

	"github.com/multica-ai/multica/server/internal/util"
	db "github.com/multica-ai/multica/server/pkg/db/generated"
)

// The companion card the design launcher opens is the design work itself, so
// its own run starting is exactly when it stops being 待办. A card the USER
// linked is a different thing — normally the implementation the design feeds
// into — and must not be moved, because moving it claims implementation began.
//
// The rule lives in the query's WHERE clause so that concurrent starts cannot
// both claim the transition and a card a human already moved is never dragged
// backwards.
func TestStartDesignDocumentCompanionIssue(t *testing.T) {
	ctx := context.Background()
	pool := newDesignDocumentFailPool(t)
	queries := db.New(pool)

	var workspaceID string
	if err := pool.QueryRow(ctx, `
		INSERT INTO workspace (name, slug) VALUES ('companion advance', 'companion-advance-' || gen_random_uuid())
		RETURNING id::text
	`).Scan(&workspaceID); err != nil {
		t.Fatalf("seed workspace: %v", err)
	}
	t.Cleanup(func() { _, _ = pool.Exec(ctx, `DELETE FROM workspace WHERE id = $1::uuid`, workspaceID) })

	seed := func(t *testing.T, origin string, status string) string {
		t.Helper()
		var id string
		// number is workspace-scoped, and this workspace is this test's own.
		if err := pool.QueryRow(ctx, `
			INSERT INTO issue (workspace_id, number, title, status, priority, creator_type, creator_id, origin_type)
			VALUES ($1::uuid, (SELECT coalesce(max(number), 0) + 1 FROM issue WHERE workspace_id = $1::uuid),
			        'card', $2, 'none', 'member', gen_random_uuid(), NULLIF($3, ''))
			RETURNING id::text
		`, workspaceID, status, origin).Scan(&id); err != nil {
			t.Fatalf("seed issue: %v", err)
		}
		return id
	}

	for name, tc := range map[string]struct {
		origin string
		status string
		want   string
	}{
		"launcher's companion advances":      {origin: "design_document", status: "todo", want: "in_progress"},
		"a task the user linked is left":     {origin: "", status: "todo", want: "todo"},
		"an unrelated origin is left":        {origin: "quick_create", status: "todo", want: "todo"},
		"a card a human moved is not undone": {origin: "design_document", status: "done", want: "done"},
	} {
		t.Run(name, func(t *testing.T) {
			issueID := seed(t, tc.origin, tc.status)
			parsed, err := util.ParseUUID(issueID)
			if err != nil {
				t.Fatalf("parse issue id: %v", err)
			}
			_, _ = queries.StartDesignDocumentCompanionIssue(ctx, parsed)

			var status string
			if err := pool.QueryRow(ctx, `SELECT status FROM issue WHERE id = $1::uuid`, issueID).Scan(&status); err != nil {
				t.Fatalf("read issue status: %v", err)
			}
			if status != tc.want {
				t.Fatalf("status = %q, want %q", status, tc.want)
			}
		})
	}
}
