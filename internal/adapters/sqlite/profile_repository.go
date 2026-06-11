package sqlite

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"time"

	"github.com/ch1lam/autocv/internal/domain"
	"github.com/ch1lam/autocv/internal/ports"
	"github.com/google/uuid"
)

var defaultProfileID = uuid.NewSHA1(
	uuid.NameSpaceURL,
	[]byte("https://autocv.local/profiles/default"),
).String()

type ProfileRepository struct {
	db *sql.DB
}

func NewProfileRepository(db *sql.DB) *ProfileRepository {
	return &ProfileRepository{db: db}
}

func (repository *ProfileRepository) EnsureDefaultProfile(
	ctx context.Context,
	name string,
	language string,
	now time.Time,
) (domain.Profile, error) {
	timestamp := now.UTC().Format(time.RFC3339Nano)
	if _, err := repository.db.ExecContext(
		ctx,
		`INSERT OR IGNORE INTO profiles(
			id, name, default_language, created_at, updated_at
		) VALUES (?, ?, ?, ?, ?)`,
		defaultProfileID,
		name,
		language,
		timestamp,
		timestamp,
	); err != nil {
		return domain.Profile{}, fmt.Errorf("ensure default profile: %w", err)
	}
	return repository.profileByID(ctx, defaultProfileID)
}

func (repository *ProfileRepository) FindDocumentByHash(
	ctx context.Context,
	profileID string,
	contentHash string,
) (domain.SourceDocument, bool, error) {
	document, err := scanSourceDocument(repository.db.QueryRowContext(
		ctx,
		`SELECT id, profile_id, kind, original_name, managed_path,
		        content_hash, parse_status, created_at, updated_at
		   FROM source_documents
		  WHERE profile_id = ? AND content_hash = ?`,
		profileID,
		contentHash,
	))
	if errors.Is(err, sql.ErrNoRows) {
		return domain.SourceDocument{}, false, nil
	}
	if err != nil {
		return domain.SourceDocument{}, false, fmt.Errorf(
			"find source document by hash: %w",
			err,
		)
	}
	return document, true, nil
}

func (repository *ProfileRepository) SaveImportedDocument(
	ctx context.Context,
	imported ports.ImportedDocument,
) error {
	tx, err := repository.db.BeginTx(ctx, nil)
	if err != nil {
		return fmt.Errorf("begin imported document transaction: %w", err)
	}
	defer tx.Rollback()

	document := imported.Document
	if _, err := tx.ExecContext(
		ctx,
		`INSERT INTO source_documents(
			id, profile_id, kind, original_name, managed_path,
			content_hash, parse_status, created_at, updated_at
		) VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?)`,
		document.ID,
		document.ProfileID,
		document.Kind,
		document.OriginalName,
		document.ManagedPath,
		document.ContentHash,
		document.ParseStatus,
		formatTime(document.CreatedAt),
		formatTime(document.UpdatedAt),
	); err != nil {
		return fmt.Errorf("insert source document: %w", err)
	}

	for _, chunk := range imported.Chunks {
		if _, err := tx.ExecContext(
			ctx,
			`INSERT INTO source_chunks(
				id, document_id, ordinal, text, locator_json
			) VALUES (?, ?, ?, ?, ?)`,
			chunk.ID,
			chunk.DocumentID,
			chunk.Ordinal,
			chunk.Text,
			chunk.LocatorJSON,
		); err != nil {
			return fmt.Errorf("insert source chunk: %w", err)
		}
	}

	for _, item := range imported.Evidence {
		if _, err := tx.ExecContext(
			ctx,
			`INSERT INTO evidence(
				id, profile_id, kind, title, content, confidence,
				user_verified, created_at, updated_at
			) VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?)`,
			item.ID,
			item.ProfileID,
			item.Kind,
			item.Title,
			item.Content,
			item.Confidence,
			item.UserVerified,
			formatTime(item.CreatedAt),
			formatTime(item.UpdatedAt),
		); err != nil {
			return fmt.Errorf("insert evidence: %w", err)
		}
	}

	for _, source := range imported.EvidenceSources {
		if _, err := tx.ExecContext(
			ctx,
			`INSERT INTO evidence_sources(
				evidence_id, chunk_id, quote_start, quote_end
			) VALUES (?, ?, ?, ?)`,
			source.EvidenceID,
			source.ChunkID,
			source.QuoteStart,
			source.QuoteEnd,
		); err != nil {
			return fmt.Errorf("insert evidence source: %w", err)
		}
	}

	if err := tx.Commit(); err != nil {
		return fmt.Errorf("commit imported document: %w", err)
	}
	return nil
}

func (repository *ProfileRepository) ListDocuments(
	ctx context.Context,
	profileID string,
) ([]domain.SourceDocument, error) {
	rows, err := repository.db.QueryContext(
		ctx,
		`SELECT id, profile_id, kind, original_name, managed_path,
		        content_hash, parse_status, created_at, updated_at
		   FROM source_documents
		  WHERE profile_id = ?
		  ORDER BY created_at, id`,
		profileID,
	)
	if err != nil {
		return nil, fmt.Errorf("list source documents: %w", err)
	}
	defer rows.Close()

	documents := make([]domain.SourceDocument, 0)
	for rows.Next() {
		document, err := scanSourceDocument(rows)
		if err != nil {
			return nil, fmt.Errorf("scan source document: %w", err)
		}
		documents = append(documents, document)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("iterate source documents: %w", err)
	}
	return documents, nil
}

func (repository *ProfileRepository) ListEvidence(
	ctx context.Context,
	profileID string,
) ([]domain.Evidence, error) {
	rows, err := repository.db.QueryContext(
		ctx,
		`SELECT id, profile_id, kind, title, content, confidence,
		        user_verified, created_at, updated_at
		   FROM evidence
		  WHERE profile_id = ?
		  ORDER BY created_at, id`,
		profileID,
	)
	if err != nil {
		return nil, fmt.Errorf("list evidence: %w", err)
	}
	defer rows.Close()

	items := make([]domain.Evidence, 0)
	for rows.Next() {
		item, err := scanEvidence(rows)
		if err != nil {
			return nil, fmt.Errorf("scan evidence: %w", err)
		}
		items = append(items, item)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("iterate evidence: %w", err)
	}
	return items, nil
}

func (repository *ProfileRepository) profileByID(
	ctx context.Context,
	id string,
) (domain.Profile, error) {
	var profile domain.Profile
	var createdAt string
	var updatedAt string
	err := repository.db.QueryRowContext(
		ctx,
		`SELECT id, name, default_language, created_at, updated_at
		   FROM profiles
		  WHERE id = ?`,
		id,
	).Scan(
		&profile.ID,
		&profile.Name,
		&profile.DefaultLanguage,
		&createdAt,
		&updatedAt,
	)
	if err != nil {
		return domain.Profile{}, fmt.Errorf("query profile: %w", err)
	}
	profile.CreatedAt, err = parseTime(createdAt)
	if err != nil {
		return domain.Profile{}, err
	}
	profile.UpdatedAt, err = parseTime(updatedAt)
	if err != nil {
		return domain.Profile{}, err
	}
	return profile, nil
}

type scanner interface {
	Scan(...any) error
}

func scanSourceDocument(row scanner) (domain.SourceDocument, error) {
	var document domain.SourceDocument
	var createdAt string
	var updatedAt string
	err := row.Scan(
		&document.ID,
		&document.ProfileID,
		&document.Kind,
		&document.OriginalName,
		&document.ManagedPath,
		&document.ContentHash,
		&document.ParseStatus,
		&createdAt,
		&updatedAt,
	)
	if err != nil {
		return domain.SourceDocument{}, err
	}
	document.CreatedAt, err = parseTime(createdAt)
	if err != nil {
		return domain.SourceDocument{}, err
	}
	document.UpdatedAt, err = parseTime(updatedAt)
	if err != nil {
		return domain.SourceDocument{}, err
	}
	return document, nil
}

func scanEvidence(row scanner) (domain.Evidence, error) {
	var item domain.Evidence
	var createdAt string
	var updatedAt string
	err := row.Scan(
		&item.ID,
		&item.ProfileID,
		&item.Kind,
		&item.Title,
		&item.Content,
		&item.Confidence,
		&item.UserVerified,
		&createdAt,
		&updatedAt,
	)
	if err != nil {
		return domain.Evidence{}, err
	}
	item.CreatedAt, err = parseTime(createdAt)
	if err != nil {
		return domain.Evidence{}, err
	}
	item.UpdatedAt, err = parseTime(updatedAt)
	if err != nil {
		return domain.Evidence{}, err
	}
	return item, nil
}

func formatTime(value time.Time) string {
	return value.UTC().Format(time.RFC3339Nano)
}

func parseTime(value string) (time.Time, error) {
	parsed, err := time.Parse(time.RFC3339Nano, value)
	if err != nil {
		return time.Time{}, fmt.Errorf("parse database timestamp: %w", err)
	}
	return parsed, nil
}

var _ ports.ProfileRepository = (*ProfileRepository)(nil)
