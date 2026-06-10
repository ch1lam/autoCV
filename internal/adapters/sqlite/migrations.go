package sqlite

import (
	"context"
	"database/sql"
	"fmt"
	"io/fs"
	"sort"
	"strconv"
	"strings"
	"time"

	"github.com/ch1lam/autocv/migrations"
)

const createMigrationTable = `
CREATE TABLE IF NOT EXISTS schema_migrations (
    version INTEGER PRIMARY KEY,
    name TEXT NOT NULL,
    applied_at TEXT NOT NULL
);`

type migration struct {
	version int
	name    string
	sql     string
}

func ApplyMigrations(ctx context.Context, db *sql.DB) error {
	if _, err := db.ExecContext(ctx, createMigrationTable); err != nil {
		return fmt.Errorf("create schema migrations table: %w", err)
	}

	available, err := loadMigrations()
	if err != nil {
		return err
	}

	applied, err := appliedVersions(ctx, db)
	if err != nil {
		return err
	}

	for _, item := range available {
		if applied[item.version] {
			continue
		}
		if err := applyMigration(ctx, db, item); err != nil {
			return err
		}
	}
	return nil
}

func loadMigrations() ([]migration, error) {
	entries, err := fs.ReadDir(migrations.FS, ".")
	if err != nil {
		return nil, fmt.Errorf("read embedded migrations: %w", err)
	}

	items := make([]migration, 0, len(entries))
	for _, entry := range entries {
		if entry.IsDir() || !strings.HasSuffix(entry.Name(), ".sql") {
			continue
		}

		versionText, _, found := strings.Cut(entry.Name(), "_")
		if !found {
			return nil, fmt.Errorf("migration %q has no numeric prefix", entry.Name())
		}
		version, err := strconv.Atoi(versionText)
		if err != nil {
			return nil, fmt.Errorf("parse migration version %q: %w", entry.Name(), err)
		}
		contents, err := migrations.FS.ReadFile(entry.Name())
		if err != nil {
			return nil, fmt.Errorf("read migration %q: %w", entry.Name(), err)
		}
		items = append(items, migration{
			version: version,
			name:    entry.Name(),
			sql:     string(contents),
		})
	}

	sort.Slice(items, func(left, right int) bool {
		return items[left].version < items[right].version
	})
	for index := 1; index < len(items); index++ {
		if items[index-1].version == items[index].version {
			return nil, fmt.Errorf("duplicate migration version %d", items[index].version)
		}
	}
	return items, nil
}

func appliedVersions(ctx context.Context, db *sql.DB) (map[int]bool, error) {
	rows, err := db.QueryContext(ctx, "SELECT version FROM schema_migrations")
	if err != nil {
		return nil, fmt.Errorf("query applied migrations: %w", err)
	}
	defer rows.Close()

	applied := make(map[int]bool)
	for rows.Next() {
		var version int
		if err := rows.Scan(&version); err != nil {
			return nil, fmt.Errorf("scan applied migration: %w", err)
		}
		applied[version] = true
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("iterate applied migrations: %w", err)
	}
	return applied, nil
}

func applyMigration(ctx context.Context, db *sql.DB, item migration) error {
	tx, err := db.BeginTx(ctx, nil)
	if err != nil {
		return fmt.Errorf("begin migration %q: %w", item.name, err)
	}
	defer tx.Rollback()

	if _, err := tx.ExecContext(ctx, item.sql); err != nil {
		return fmt.Errorf("apply migration %q: %w", item.name, err)
	}
	if _, err := tx.ExecContext(
		ctx,
		"INSERT INTO schema_migrations(version, name, applied_at) VALUES (?, ?, ?)",
		item.version,
		item.name,
		time.Now().UTC().Format(time.RFC3339Nano),
	); err != nil {
		return fmt.Errorf("record migration %q: %w", item.name, err)
	}
	if err := tx.Commit(); err != nil {
		return fmt.Errorf("commit migration %q: %w", item.name, err)
	}
	return nil
}
