package sqlite

import (
	"context"
	"database/sql"
	"fmt"

	"github.com/ch1lam/autocv/internal/domain"
	"github.com/ch1lam/autocv/internal/ports"
)

type ProviderCallRepository struct {
	db *sql.DB
}

func NewProviderCallRepository(db *sql.DB) *ProviderCallRepository {
	return &ProviderCallRepository{db: db}
}

func (repository *ProviderCallRepository) Record(
	ctx context.Context,
	call domain.ProviderCall,
) error {
	if err := call.Validate(); err != nil {
		return fmt.Errorf("validate Provider call: %w", err)
	}
	if _, err := repository.db.ExecContext(
		ctx,
		`INSERT INTO provider_calls(
			id, provider, model, task, prompt_version, input_hash,
			status, duration_ms, input_tokens, output_tokens, total_tokens,
			schema_repaired, error_kind, created_at
		) VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)`,
		call.ID,
		call.Provider,
		call.Model,
		call.Task,
		call.PromptVersion,
		call.InputHash,
		call.Status,
		call.DurationMS,
		call.InputTokens,
		call.OutputTokens,
		call.TotalTokens,
		call.SchemaRepaired,
		call.ErrorKind,
		formatTime(call.CreatedAt),
	); err != nil {
		return fmt.Errorf("record Provider call: %w", err)
	}
	return nil
}

var _ ports.ProviderCallRecorder = (*ProviderCallRepository)(nil)
