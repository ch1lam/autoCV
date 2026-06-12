package sqlite

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"

	"github.com/ch1lam/autocv/internal/domain"
	"github.com/ch1lam/autocv/internal/ports"
)

type ArtifactRepository struct {
	db *sql.DB
}

func NewArtifactRepository(db *sql.DB) *ArtifactRepository {
	return &ArtifactRepository{db: db}
}

func (repository *ArtifactRepository) GetLatest(
	ctx context.Context,
	runID string,
	kind domain.ArtifactKind,
) (domain.Artifact, bool, error) {
	var artifact domain.Artifact
	var createdAt string
	var previewPathsJSON string
	err := repository.db.QueryRowContext(
		ctx,
		`SELECT id, run_id, resume_id, kind, path, preview_paths_json,
		        content_hash, created_at
		   FROM artifacts
		  WHERE run_id = ? AND kind = ?
		  ORDER BY created_at DESC, id DESC
		  LIMIT 1`,
		runID,
		kind,
	).Scan(
		&artifact.ID,
		&artifact.RunID,
		&artifact.ResumeID,
		&artifact.Kind,
		&artifact.Path,
		&previewPathsJSON,
		&artifact.ContentHash,
		&createdAt,
	)
	if errors.Is(err, sql.ErrNoRows) {
		return domain.Artifact{}, false, nil
	}
	if err != nil {
		return domain.Artifact{}, false, fmt.Errorf(
			"get latest artifact: %w",
			err,
		)
	}
	if err := json.Unmarshal(
		[]byte(previewPathsJSON),
		&artifact.PreviewPaths,
	); err != nil {
		return domain.Artifact{}, false, fmt.Errorf(
			"decode artifact preview paths: %w",
			err,
		)
	}
	artifact.CreatedAt, err = parseTime(createdAt)
	if err != nil {
		return domain.Artifact{}, false, err
	}
	return artifact, true, nil
}

func (repository *ArtifactRepository) Save(
	ctx context.Context,
	artifact domain.Artifact,
) error {
	previewPathsJSON, err := json.Marshal(artifact.PreviewPaths)
	if err != nil {
		return fmt.Errorf("encode artifact preview paths: %w", err)
	}
	if _, err := repository.db.ExecContext(
		ctx,
		`INSERT INTO artifacts(
			id, run_id, resume_id, kind, path, preview_paths_json,
			content_hash, created_at
		) VALUES (?, ?, ?, ?, ?, ?, ?, ?)`,
		artifact.ID,
		artifact.RunID,
		artifact.ResumeID,
		artifact.Kind,
		artifact.Path,
		string(previewPathsJSON),
		artifact.ContentHash,
		formatTime(artifact.CreatedAt),
	); err != nil {
		return fmt.Errorf("save artifact: %w", err)
	}
	return nil
}

var _ ports.ArtifactRepository = (*ArtifactRepository)(nil)
