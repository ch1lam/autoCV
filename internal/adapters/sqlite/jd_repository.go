package sqlite

import (
	"context"
	"database/sql"
	"errors"
	"fmt"

	"github.com/ch1lam/autocv/internal/domain"
	"github.com/ch1lam/autocv/internal/ports"
)

type JDRepository struct {
	db *sql.DB
}

func NewJDRepository(db *sql.DB) *JDRepository {
	return &JDRepository{db: db}
}

func (repository *JDRepository) GetLatest(
	ctx context.Context,
) (domain.JobDescription, bool, error) {
	item, err := scanJobDescription(repository.db.QueryRowContext(
		ctx,
		`SELECT id, title, company, raw_text, language, raw_hash,
		        analysis_json, analysis_status, analysis_error,
		        created_at, updated_at
		   FROM job_descriptions
		  ORDER BY updated_at DESC, created_at DESC, id
		  LIMIT 1`,
	))
	if errors.Is(err, sql.ErrNoRows) {
		return domain.JobDescription{}, false, nil
	}
	if err != nil {
		return domain.JobDescription{}, false, fmt.Errorf(
			"get latest job description: %w",
			err,
		)
	}
	return item, true, nil
}

func (repository *JDRepository) SaveDraft(
	ctx context.Context,
	item domain.JobDescription,
) error {
	if _, err := repository.db.ExecContext(
		ctx,
		`INSERT INTO job_descriptions(
			id, title, company, raw_text, language, analysis_json,
			created_at, updated_at, raw_hash, analysis_status, analysis_error
		) VALUES (?, ?, ?, ?, ?, NULL, ?, ?, ?, 'pending', '')
		ON CONFLICT(id) DO UPDATE SET
			title = excluded.title,
			company = excluded.company,
			raw_text = excluded.raw_text,
			language = excluded.language,
			analysis_json = NULL,
			updated_at = excluded.updated_at,
			raw_hash = excluded.raw_hash,
			analysis_status = 'pending',
			analysis_error = ''`,
		item.ID,
		item.Title,
		item.Company,
		item.RawText,
		item.Language,
		formatTime(item.CreatedAt),
		formatTime(item.UpdatedAt),
		item.RawHash,
	); err != nil {
		return fmt.Errorf("save job description draft: %w", err)
	}
	return nil
}

func (repository *JDRepository) UpdateAnalysis(
	ctx context.Context,
	update ports.JDAnalysisUpdate,
) error {
	result, err := repository.db.ExecContext(
		ctx,
		`UPDATE job_descriptions
		    SET title = ?,
		        company = ?,
		        language = ?,
		        analysis_json = NULLIF(?, ''),
		        analysis_status = ?,
		        analysis_error = ?,
		        updated_at = ?
		  WHERE id = ? AND raw_hash = ?`,
		update.Title,
		update.Company,
		update.Language,
		update.AnalysisJSON,
		update.Status,
		update.Error,
		formatTime(update.UpdatedAt),
		update.ID,
		update.RawHash,
	)
	if err != nil {
		return fmt.Errorf("update job description analysis: %w", err)
	}
	affected, err := result.RowsAffected()
	if err != nil {
		return fmt.Errorf("read updated job description rows: %w", err)
	}
	if affected != 1 {
		return errors.New("job description changed before analysis was saved")
	}
	return nil
}

func scanJobDescription(row scanner) (domain.JobDescription, error) {
	var item domain.JobDescription
	var analysisJSON sql.NullString
	var createdAt string
	var updatedAt string
	err := row.Scan(
		&item.ID,
		&item.Title,
		&item.Company,
		&item.RawText,
		&item.Language,
		&item.RawHash,
		&analysisJSON,
		&item.AnalysisStatus,
		&item.AnalysisError,
		&createdAt,
		&updatedAt,
	)
	if err != nil {
		return domain.JobDescription{}, err
	}
	item.AnalysisJSON = analysisJSON.String
	item.CreatedAt, err = parseTime(createdAt)
	if err != nil {
		return domain.JobDescription{}, err
	}
	item.UpdatedAt, err = parseTime(updatedAt)
	if err != nil {
		return domain.JobDescription{}, err
	}
	return item, nil
}

var _ ports.JDRepository = (*JDRepository)(nil)
