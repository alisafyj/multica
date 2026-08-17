package migrations

import (
	"context"
	"os"
	"testing"

	"github.com/jackc/pgx/v5/pgxpool"
)

func TestProjectCreatorConstraintMigration(t *testing.T) {
	dbURL := os.Getenv("DATABASE_URL")
	if dbURL == "" {
		t.Skip("integration test requires Postgres at DATABASE_URL")
	}

	ctx := context.Background()
	pool, err := pgxpool.New(ctx, dbURL)
	if err != nil {
		t.Fatalf("connect to Postgres: %v", err)
	}
	defer pool.Close()

	conn, err := pool.Acquire(ctx)
	if err != nil {
		t.Fatalf("acquire Postgres connection: %v", err)
	}
	defer conn.Release()

	if _, err := conn.Exec(ctx, `
		CREATE TEMP TABLE project (
			id UUID PRIMARY KEY,
			created_by UUID
		)
	`); err != nil {
		t.Fatalf("create temporary project table: %v", err)
	}

	legacyID := "00000000-0000-0000-0000-000000000001"
	creatorID := "00000000-0000-0000-0000-000000000002"
	if _, err := conn.Exec(ctx, `INSERT INTO project (id, created_by) VALUES ($1, NULL)`, legacyID); err != nil {
		t.Fatalf("insert legacy project: %v", err)
	}

	applyMigrationFile(t, ctx, conn.Conn(), "891_project_created_by_required.up.sql")

	var constraintValidated bool
	if err := conn.QueryRow(ctx, `
		SELECT convalidated
		FROM pg_constraint
		WHERE conrelid = 'project'::regclass
		  AND conname = 'project_created_by_required'
	`).Scan(&constraintValidated); err != nil {
		t.Fatalf("read project creator constraint: %v", err)
	}
	if constraintValidated {
		t.Fatal("project creator constraint should remain NOT VALID")
	}

	var legacyCount int
	if err := conn.QueryRow(ctx, `SELECT count(*) FROM project WHERE id = $1 AND created_by IS NULL`, legacyID).Scan(&legacyCount); err != nil {
		t.Fatalf("read legacy project: %v", err)
	}
	if legacyCount != 1 {
		t.Fatalf("legacy project count = %d, want 1", legacyCount)
	}

	assertInsertCheckViolation(t, ctx, conn.Conn(), `
		INSERT INTO project (id, created_by)
		VALUES ('00000000-0000-0000-0000-000000000003', NULL)
	`)
	if _, err := conn.Exec(ctx, `
		INSERT INTO project (id, created_by)
		VALUES ('00000000-0000-0000-0000-000000000004', $1)
	`, creatorID); err != nil {
		t.Fatalf("insert attributed project: %v", err)
	}

	applyMigrationFile(t, ctx, conn.Conn(), "891_project_created_by_required.down.sql")
}
