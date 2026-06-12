package sqlite

import (
	"context"
	"path/filepath"
	"testing"
	"time"

	"github.com/ch1lam/autocv/internal/domain"
)

func TestProviderCallRepositoryStoresMetadataWithoutContent(t *testing.T) {
	ctx := context.Background()
	db := openTestDatabase(
		t,
		ctx,
		filepath.Join(t.TempDir(), "provider-calls.db"),
	)
	defer db.Close()

	call := domain.ProviderCall{
		ID:             "call-1",
		Provider:       domain.ProviderOpenAI,
		Model:          "gpt-5.5",
		Task:           "jd_analysis",
		PromptVersion:  "v1",
		InputHash:      "sha256-only",
		Status:         domain.ProviderCallStatusSucceeded,
		DurationMS:     320,
		InputTokens:    100,
		OutputTokens:   40,
		TotalTokens:    140,
		SchemaRepaired: true,
		CreatedAt: time.Date(
			2026,
			6,
			12,
			9,
			0,
			0,
			0,
			time.UTC,
		),
	}
	repository := NewProviderCallRepository(db)
	if err := repository.Record(ctx, call); err != nil {
		t.Fatalf("record Provider call: %v", err)
	}

	var stored domain.ProviderCall
	var createdAt string
	if err := db.QueryRowContext(
		ctx,
		`SELECT id, provider, model, task, prompt_version, input_hash,
		        status, duration_ms, input_tokens, output_tokens, total_tokens,
		        schema_repaired, error_kind, created_at
		   FROM provider_calls
		  WHERE id = ?`,
		call.ID,
	).Scan(
		&stored.ID,
		&stored.Provider,
		&stored.Model,
		&stored.Task,
		&stored.PromptVersion,
		&stored.InputHash,
		&stored.Status,
		&stored.DurationMS,
		&stored.InputTokens,
		&stored.OutputTokens,
		&stored.TotalTokens,
		&stored.SchemaRepaired,
		&stored.ErrorKind,
		&createdAt,
	); err != nil {
		t.Fatalf("read Provider call: %v", err)
	}
	if stored.InputHash != "sha256-only" ||
		stored.TotalTokens != 140 ||
		!stored.SchemaRepaired {
		t.Fatalf("unexpected Provider call %#v", stored)
	}

	columns, err := db.QueryContext(
		ctx,
		"PRAGMA table_info(provider_calls)",
	)
	if err != nil {
		t.Fatalf("read Provider call columns: %v", err)
	}
	defer columns.Close()
	for columns.Next() {
		var cid int
		var name string
		var columnType string
		var notNull int
		var defaultValue any
		var primaryKey int
		if err := columns.Scan(
			&cid,
			&name,
			&columnType,
			&notNull,
			&defaultValue,
			&primaryKey,
		); err != nil {
			t.Fatalf("scan Provider call column: %v", err)
		}
		switch name {
		case "prompt", "input", "output", "response", "content", "api_key":
			t.Fatalf("Provider call table must not contain %q", name)
		}
	}
}
