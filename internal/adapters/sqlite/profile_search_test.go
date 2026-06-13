package sqlite

import (
	"context"
	"database/sql"
	"path/filepath"
	"testing"

	"github.com/ch1lam/autocv/internal/ports"
)

func TestProfileSearchFindsChunksAndEvidenceWithinProfile(t *testing.T) {
	ctx := context.Background()
	db := openTestDatabase(
		t,
		ctx,
		filepath.Join(t.TempDir(), "search.db"),
	)
	defer db.Close()
	insertSearchFixture(t, db)

	search := NewProfileSearch(db)
	results, err := search.Search(ctx, "profile-main", "Postgre", 20)
	if err != nil {
		t.Fatalf("search profile: %v", err)
	}
	if len(results) != 2 {
		t.Fatalf("expected chunk and evidence results, got %#v", results)
	}
	for _, result := range results {
		if result.DocumentName != "backend.md" ||
			result.SourceChunkID != "chunk-main" ||
			result.Snippet == "" {
			t.Fatalf("unexpected search result %#v", result)
		}
	}

	isolated, err := search.Search(ctx, "profile-other", "Postgre", 20)
	if err != nil {
		t.Fatalf("search isolated profile: %v", err)
	}
	if len(isolated) != 0 {
		t.Fatalf("expected profile isolation, got %#v", isolated)
	}
}

func TestProfileSearchMigrationBackfillsExistingContent(t *testing.T) {
	ctx := context.Background()
	db, err := sql.Open(
		"sqlite",
		"file:"+filepath.Join(t.TempDir(), "legacy-search.db"),
	)
	if err != nil {
		t.Fatalf("open legacy database: %v", err)
	}
	defer db.Close()
	if _, err := db.ExecContext(ctx, "PRAGMA foreign_keys = ON"); err != nil {
		t.Fatalf("enable foreign keys: %v", err)
	}
	if _, err := db.ExecContext(ctx, createMigrationTable); err != nil {
		t.Fatalf("create migration table: %v", err)
	}

	available, err := loadMigrations()
	if err != nil {
		t.Fatalf("load migrations: %v", err)
	}
	for _, item := range available {
		if item.version == 10 {
			break
		}
		if err := applyMigration(ctx, db, item); err != nil {
			t.Fatalf("apply legacy migration %d: %v", item.version, err)
		}
	}
	insertSearchFixture(t, db)

	if err := ApplyMigrations(ctx, db); err != nil {
		t.Fatalf("apply search migration: %v", err)
	}
	results, err := NewProfileSearch(db).Search(
		ctx,
		"profile-main",
		"Postgre",
		20,
	)
	if err != nil {
		t.Fatalf("search backfilled content: %v", err)
	}
	if len(results) != 2 {
		t.Fatalf("expected backfilled chunk and evidence, got %#v", results)
	}
	if err := ApplyMigrations(ctx, db); err != nil {
		t.Fatalf("reapply migrations: %v", err)
	}
}

func TestProfileSearchSupportsChineseAndShortQueries(t *testing.T) {
	ctx := context.Background()
	db := openTestDatabase(
		t,
		ctx,
		filepath.Join(t.TempDir(), "short-search.db"),
	)
	defer db.Close()
	insertSearchFixture(t, db)

	search := NewProfileSearch(db)
	chinese, err := search.Search(ctx, "profile-main", "保存订单", 20)
	if err != nil {
		t.Fatalf("search Chinese text: %v", err)
	}
	if len(chinese) == 0 {
		t.Fatal("expected Chinese substring result")
	}

	short, err := search.Search(ctx, "profile-main", "Go", 20)
	if err != nil {
		t.Fatalf("search short text: %v", err)
	}
	if len(short) == 0 {
		t.Fatal("expected short query result")
	}
}

func TestProfileSearchUpdatesWithSourceChanges(t *testing.T) {
	ctx := context.Background()
	db := openTestDatabase(
		t,
		ctx,
		filepath.Join(t.TempDir(), "updated-search.db"),
	)
	defer db.Close()
	insertSearchFixture(t, db)

	if _, err := db.ExecContext(
		ctx,
		"UPDATE source_chunks SET text = ? WHERE id = ?",
		"使用 Rust 重写任务调度器。",
		"chunk-main",
	); err != nil {
		t.Fatalf("update source chunk: %v", err)
	}

	search := NewProfileSearch(db)
	oldResults, err := search.Search(ctx, "profile-main", "Postgre", 20)
	if err != nil {
		t.Fatalf("search old content: %v", err)
	}
	if len(oldResults) != 1 || oldResults[0].EntityType != "evidence" {
		t.Fatalf("expected only evidence to retain old content, got %#v", oldResults)
	}
	newResults, err := search.Search(ctx, "profile-main", "Rust", 20)
	if err != nil {
		t.Fatalf("search updated content: %v", err)
	}
	if len(newResults) != 1 ||
		newResults[0].EntityType != "source_chunk" {
		t.Fatalf("expected updated source chunk, got %#v", newResults)
	}
}

func insertSearchFixture(t *testing.T, db *sql.DB) {
	t.Helper()

	statements := []string{
		`INSERT INTO profiles(
			id, name, default_language, is_active, created_at, updated_at
		) VALUES
			('profile-main', '主资料库', 'zh-CN', 1, '2026-06-13T00:00:00Z', '2026-06-13T00:00:00Z'),
			('profile-other', '其他资料库', 'zh-CN', 0, '2026-06-13T00:00:00Z', '2026-06-13T00:00:00Z')`,
		`INSERT INTO source_documents(
			id, profile_id, kind, original_name, managed_path,
			content_hash, parse_status, created_at, updated_at
		) VALUES (
			'document-main', 'profile-main', 'markdown', 'backend.md',
			'/tmp/backend.md', 'hash-main', 'succeeded',
			'2026-06-13T00:00:00Z', '2026-06-13T00:00:00Z'
		)`,
		`INSERT INTO source_chunks(
			id, document_id, ordinal, text, locator_json
		) VALUES (
			'chunk-main', 'document-main', 0,
			'使用 Go 和 PostgreSQL 保存订单，支持本地恢复。',
			'{"heading_path":["项目经历"]}'
		)`,
		`INSERT INTO evidence(
			id, profile_id, kind, title, content, confidence,
			user_verified, created_at, updated_at
		) VALUES (
			'evidence-main', 'profile-main', 'project', '订单服务',
			'使用 PostgreSQL 保存订单。', 0.9, 0,
			'2026-06-13T00:00:00Z', '2026-06-13T00:00:00Z'
		)`,
		`INSERT INTO evidence_sources(
			evidence_id, chunk_id, quote_start, quote_end
		) VALUES ('evidence-main', 'chunk-main', 0, 20)`,
	}
	for _, statement := range statements {
		if _, err := db.Exec(statement); err != nil {
			t.Fatalf("insert search fixture: %v", err)
		}
	}
}

var _ ports.ProfileSearch = (*ProfileSearch)(nil)
