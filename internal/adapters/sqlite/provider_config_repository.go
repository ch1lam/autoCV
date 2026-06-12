package sqlite

import (
	"context"
	"database/sql"
	"errors"
	"fmt"

	"github.com/ch1lam/autocv/internal/domain"
	"github.com/ch1lam/autocv/internal/ports"
)

type ProviderConfigRepository struct {
	db *sql.DB
}

func NewProviderConfigRepository(db *sql.DB) *ProviderConfigRepository {
	return &ProviderConfigRepository{db: db}
}

func (repository *ProviderConfigRepository) GetActive(
	ctx context.Context,
) (domain.ProviderConfig, bool, error) {
	config, err := scanProviderConfig(repository.db.QueryRowContext(
		ctx,
		`SELECT id, provider, base_url, model, secret_ref, enabled,
		        created_at, updated_at
		   FROM provider_configs
		  WHERE enabled = 1
		  ORDER BY updated_at DESC, id
		  LIMIT 1`,
	))
	if errors.Is(err, sql.ErrNoRows) {
		return domain.ProviderConfig{}, false, nil
	}
	if err != nil {
		return domain.ProviderConfig{}, false, fmt.Errorf(
			"get active provider config: %w",
			err,
		)
	}
	return config, true, nil
}

func (repository *ProviderConfigRepository) GetByProvider(
	ctx context.Context,
	provider string,
) (domain.ProviderConfig, bool, error) {
	config, err := scanProviderConfig(repository.db.QueryRowContext(
		ctx,
		`SELECT id, provider, base_url, model, secret_ref, enabled,
		        created_at, updated_at
		   FROM provider_configs
		  WHERE provider = ?`,
		provider,
	))
	if errors.Is(err, sql.ErrNoRows) {
		return domain.ProviderConfig{}, false, nil
	}
	if err != nil {
		return domain.ProviderConfig{}, false, fmt.Errorf(
			"get provider config: %w",
			err,
		)
	}
	return config, true, nil
}

func (repository *ProviderConfigRepository) Save(
	ctx context.Context,
	config domain.ProviderConfig,
) error {
	if err := config.Validate(); err != nil {
		return fmt.Errorf("validate provider config: %w", err)
	}

	tx, err := repository.db.BeginTx(ctx, nil)
	if err != nil {
		return fmt.Errorf("begin provider config transaction: %w", err)
	}
	defer tx.Rollback()

	if config.Enabled {
		if _, err := tx.ExecContext(
			ctx,
			"UPDATE provider_configs SET enabled = 0 WHERE enabled = 1",
		); err != nil {
			return fmt.Errorf("disable provider configs: %w", err)
		}
	}
	if _, err := tx.ExecContext(
		ctx,
		`INSERT INTO provider_configs(
			id, provider, base_url, model, secret_ref, enabled,
			created_at, updated_at
		) VALUES (?, ?, ?, ?, ?, ?, ?, ?)
		ON CONFLICT(provider) DO UPDATE SET
			base_url = excluded.base_url,
			model = excluded.model,
			secret_ref = excluded.secret_ref,
			enabled = excluded.enabled,
			updated_at = excluded.updated_at`,
		config.ID,
		config.Provider,
		config.BaseURL,
		config.Model,
		config.SecretRef,
		config.Enabled,
		formatTime(config.CreatedAt),
		formatTime(config.UpdatedAt),
	); err != nil {
		return fmt.Errorf("save provider config: %w", err)
	}
	if err := tx.Commit(); err != nil {
		return fmt.Errorf("commit provider config: %w", err)
	}
	return nil
}

func scanProviderConfig(row scanner) (domain.ProviderConfig, error) {
	var config domain.ProviderConfig
	var createdAt string
	var updatedAt string
	err := row.Scan(
		&config.ID,
		&config.Provider,
		&config.BaseURL,
		&config.Model,
		&config.SecretRef,
		&config.Enabled,
		&createdAt,
		&updatedAt,
	)
	if err != nil {
		return domain.ProviderConfig{}, err
	}
	config.CreatedAt, err = parseTime(createdAt)
	if err != nil {
		return domain.ProviderConfig{}, err
	}
	config.UpdatedAt, err = parseTime(updatedAt)
	if err != nil {
		return domain.ProviderConfig{}, err
	}
	return config, nil
}

var _ ports.ProviderConfigRepository = (*ProviderConfigRepository)(nil)
