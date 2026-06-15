package sqlite

import (
	"context"
	"database/sql"
	"path/filepath"
	"testing"
)

func TestOpenAppliesMigrationOnce(t *testing.T) {
	ctx := context.Background()
	path := filepath.Join(t.TempDir(), "autocv.db")

	db := openTestDatabase(t, ctx, path)
	assertMigrationCount(t, db, 11)
	assertMinimumTables(t, db)
	db.Close()

	db = openTestDatabase(t, ctx, path)
	defer db.Close()
	assertMigrationCount(t, db, 11)
	assertMinimumTables(t, db)
}

func TestSQLiteBuildSupportsFTS5(t *testing.T) {
	ctx := context.Background()
	path := filepath.Join(t.TempDir(), "fts.db")
	db := openTestDatabase(t, ctx, path)
	defer db.Close()

	if _, err := db.ExecContext(
		ctx,
		"CREATE VIRTUAL TABLE source_search USING fts5(content)",
	); err != nil {
		t.Fatalf("create FTS5 table: %v", err)
	}
}

func openTestDatabase(t *testing.T, ctx context.Context, path string) *sql.DB {
	t.Helper()

	db, err := Open(ctx, path)
	if err != nil {
		t.Fatalf("open database: %v", err)
	}
	return db
}

func assertMigrationCount(t *testing.T, db *sql.DB, expected int) {
	t.Helper()

	var count int
	if err := db.QueryRow("SELECT COUNT(*) FROM schema_migrations").Scan(&count); err != nil {
		t.Fatalf("count migrations: %v", err)
	}
	if count != expected {
		t.Fatalf("expected %d migration, got %d", expected, count)
	}
}

func assertMinimumTables(t *testing.T, db *sql.DB) {
	t.Helper()

	expected := []string{
		"artifacts",
		"evidence",
		"evidence_sources",
		"job_descriptions",
		"profiles",
		"provider_calls",
		"provider_configs",
		"profile_search",
		"run_scope_documents",
		"run_scopes",
		"resume_blocks",
		"resume_runs",
		"resumes",
		"block_sources",
		"source_chunks",
		"source_documents",
		"stage_results",
	}
	for _, table := range expected {
		var count int
		if err := db.QueryRow(
			"SELECT COUNT(*) FROM sqlite_schema WHERE type = 'table' AND name = ?",
			table,
		).Scan(&count); err != nil {
			t.Fatalf("query table %q: %v", table, err)
		}
		if count != 1 {
			t.Fatalf("expected table %q to exist", table)
		}
	}
}
