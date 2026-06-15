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
			id, name, default_language, is_active, created_at, updated_at
		) VALUES (
			?, ?, ?,
			CASE WHEN EXISTS(
				SELECT 1 FROM profiles WHERE is_active = 1
			) THEN 0 ELSE 1 END,
			?, ?
		)`,
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

func (repository *ProfileRepository) CreateProfile(
	ctx context.Context,
	profile domain.Profile,
) error {
	tx, err := repository.db.BeginTx(ctx, nil)
	if err != nil {
		return fmt.Errorf("begin create profile transaction: %w", err)
	}
	defer tx.Rollback()

	if profile.Active {
		if _, err := tx.ExecContext(
			ctx,
			"UPDATE profiles SET is_active = 0 WHERE is_active = 1",
		); err != nil {
			return fmt.Errorf("clear active profile: %w", err)
		}
	}
	if _, err := tx.ExecContext(
		ctx,
		`INSERT INTO profiles(
			id, name, default_language, is_active, created_at, updated_at
		) VALUES (?, ?, ?, ?, ?, ?)`,
		profile.ID,
		profile.Name,
		profile.DefaultLanguage,
		profile.Active,
		formatTime(profile.CreatedAt),
		formatTime(profile.UpdatedAt),
	); err != nil {
		return fmt.Errorf("insert profile: %w", err)
	}
	if err := tx.Commit(); err != nil {
		return fmt.Errorf("commit create profile: %w", err)
	}
	return nil
}

func (repository *ProfileRepository) ListProfiles(
	ctx context.Context,
) ([]domain.Profile, error) {
	rows, err := repository.db.QueryContext(
		ctx,
		`SELECT id, name, default_language, is_active, created_at, updated_at
		   FROM profiles
		  ORDER BY is_active DESC, created_at, id`,
	)
	if err != nil {
		return nil, fmt.Errorf("list profiles: %w", err)
	}
	defer rows.Close()

	profiles := make([]domain.Profile, 0)
	for rows.Next() {
		profile, err := scanProfile(rows)
		if err != nil {
			return nil, fmt.Errorf("scan profile: %w", err)
		}
		profiles = append(profiles, profile)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("iterate profiles: %w", err)
	}
	return profiles, nil
}

func (repository *ProfileRepository) GetActiveProfile(
	ctx context.Context,
) (domain.Profile, bool, error) {
	profile, err := scanProfile(repository.db.QueryRowContext(
		ctx,
		`SELECT id, name, default_language, is_active, created_at, updated_at
		   FROM profiles
		  WHERE is_active = 1`,
	))
	if errors.Is(err, sql.ErrNoRows) {
		return domain.Profile{}, false, nil
	}
	if err != nil {
		return domain.Profile{}, false, fmt.Errorf("query active profile: %w", err)
	}
	return profile, true, nil
}

func (repository *ProfileRepository) SetActiveProfile(
	ctx context.Context,
	profileID string,
) (domain.Profile, error) {
	tx, err := repository.db.BeginTx(ctx, nil)
	if err != nil {
		return domain.Profile{}, fmt.Errorf("begin select profile transaction: %w", err)
	}
	defer tx.Rollback()

	if _, err := tx.ExecContext(
		ctx,
		"UPDATE profiles SET is_active = 0 WHERE is_active = 1",
	); err != nil {
		return domain.Profile{}, fmt.Errorf("clear active profile: %w", err)
	}
	result, err := tx.ExecContext(
		ctx,
		"UPDATE profiles SET is_active = 1 WHERE id = ?",
		profileID,
	)
	if err != nil {
		return domain.Profile{}, fmt.Errorf("select profile: %w", err)
	}
	affected, err := result.RowsAffected()
	if err != nil {
		return domain.Profile{}, fmt.Errorf("read selected profile result: %w", err)
	}
	if affected == 0 {
		return domain.Profile{}, fmt.Errorf("profile %q not found", profileID)
	}
	if err := tx.Commit(); err != nil {
		return domain.Profile{}, fmt.Errorf("commit selected profile: %w", err)
	}
	return repository.profileByID(ctx, profileID)
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
		`SELECT e.id, e.profile_id, e.kind, e.title, e.content,
		        e.confidence, e.user_verified, e.created_at, e.updated_at,
		        es.chunk_id, es.quote_start, es.quote_end,
		        sc.document_id, sd.original_name, sc.text, sc.locator_json
		   FROM evidence e
		   LEFT JOIN evidence_sources es ON es.evidence_id = e.id
		   LEFT JOIN source_chunks sc ON sc.id = es.chunk_id
		   LEFT JOIN source_documents sd ON sd.id = sc.document_id
		  WHERE e.profile_id = ?
		  ORDER BY e.created_at, e.id, es.chunk_id`,
		profileID,
	)
	if err != nil {
		return nil, fmt.Errorf("list evidence: %w", err)
	}
	defer rows.Close()

	items := make([]domain.Evidence, 0)
	itemIndexes := make(map[string]int)
	for rows.Next() {
		var item domain.Evidence
		var createdAt string
		var updatedAt string
		var chunkID sql.NullString
		var quoteStart sql.NullInt64
		var quoteEnd sql.NullInt64
		var documentID sql.NullString
		var documentName sql.NullString
		var chunkText sql.NullString
		var locatorJSON sql.NullString
		err := rows.Scan(
			&item.ID,
			&item.ProfileID,
			&item.Kind,
			&item.Title,
			&item.Content,
			&item.Confidence,
			&item.UserVerified,
			&createdAt,
			&updatedAt,
			&chunkID,
			&quoteStart,
			&quoteEnd,
			&documentID,
			&documentName,
			&chunkText,
			&locatorJSON,
		)
		if err != nil {
			return nil, fmt.Errorf("scan evidence: %w", err)
		}
		index, exists := itemIndexes[item.ID]
		if !exists {
			item.CreatedAt, err = parseTime(createdAt)
			if err != nil {
				return nil, err
			}
			item.UpdatedAt, err = parseTime(updatedAt)
			if err != nil {
				return nil, err
			}
			item.Sources = make([]domain.EvidenceSource, 0)
			items = append(items, item)
			index = len(items) - 1
			itemIndexes[item.ID] = index
		}
		if chunkID.Valid {
			items[index].Sources = append(
				items[index].Sources,
				domain.EvidenceSource{
					EvidenceID:   item.ID,
					ChunkID:      chunkID.String,
					DocumentID:   documentID.String,
					DocumentName: documentName.String,
					ChunkText:    chunkText.String,
					LocatorJSON:  locatorJSON.String,
					QuoteStart:   int(quoteStart.Int64),
					QuoteEnd:     int(quoteEnd.Int64),
				},
			)
		}
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("iterate evidence: %w", err)
	}
	return items, nil
}

func (repository *ProfileRepository) UpdateEvidence(
	ctx context.Context,
	profileID string,
	evidenceID string,
	title string,
	content string,
	userVerified bool,
	updatedAt time.Time,
) error {
	result, err := repository.db.ExecContext(
		ctx,
		`UPDATE evidence
		    SET title = ?,
		        content = ?,
		        user_verified = ?,
		        updated_at = ?
		  WHERE id = ?
		    AND profile_id = ?`,
		title,
		content,
		userVerified,
		formatTime(updatedAt),
		evidenceID,
		profileID,
	)
	if err != nil {
		return fmt.Errorf("update evidence: %w", err)
	}
	affected, err := result.RowsAffected()
	if err != nil {
		return fmt.Errorf("read updated evidence result: %w", err)
	}
	if affected == 0 {
		return fmt.Errorf(
			"evidence %q not found in active profile",
			evidenceID,
		)
	}
	return nil
}

func (repository *ProfileRepository) profileByID(
	ctx context.Context,
	id string,
) (domain.Profile, error) {
	profile, err := scanProfile(repository.db.QueryRowContext(
		ctx,
		`SELECT id, name, default_language, is_active, created_at, updated_at
		   FROM profiles
		  WHERE id = ?`,
		id,
	))
	if err != nil {
		return domain.Profile{}, fmt.Errorf("query profile: %w", err)
	}
	return profile, nil
}

func scanProfile(row scanner) (domain.Profile, error) {
	var profile domain.Profile
	var createdAt string
	var updatedAt string
	err := row.Scan(
		&profile.ID,
		&profile.Name,
		&profile.DefaultLanguage,
		&profile.Active,
		&createdAt,
		&updatedAt,
	)
	if err != nil {
		return domain.Profile{}, err
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
