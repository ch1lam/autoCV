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

type ResumeRepository struct {
	db *sql.DB
}

type resumeStructureSnapshot struct {
	SchemaVersion     int                    `json:"schema_version"`
	Language          domain.ResumeLanguage  `json:"language"`
	TargetRole        string                 `json:"target_role"`
	Header            domain.ResumeHeader    `json:"header"`
	Sections          []domain.ResumeSection `json:"sections"`
	Blocks            []domain.ResumeBlock   `json:"blocks"`
	OptimizationNotes []string               `json:"optimization_notes"`
}

func NewResumeRepository(db *sql.DB) *ResumeRepository {
	return &ResumeRepository{db: db}
}

func (repository *ResumeRepository) GetScope(
	ctx context.Context,
	profileID string,
	jdID string,
) (domain.ResumeRunScope, bool, error) {
	var scope domain.ResumeRunScope
	var updatedAt string
	err := repository.db.QueryRowContext(
		ctx,
		`SELECT profile_id, jd_id, mode, updated_at
		   FROM run_scopes
		  WHERE profile_id = ? AND jd_id = ?`,
		profileID,
		jdID,
	).Scan(
		&scope.ProfileID,
		&scope.JDID,
		&scope.Mode,
		&updatedAt,
	)
	if errors.Is(err, sql.ErrNoRows) {
		return domain.ResumeRunScope{}, false, nil
	}
	if err != nil {
		return domain.ResumeRunScope{}, false, fmt.Errorf(
			"get run scope: %w",
			err,
		)
	}
	scope.UpdatedAt, err = parseTime(updatedAt)
	if err != nil {
		return domain.ResumeRunScope{}, false, err
	}

	rows, err := repository.db.QueryContext(
		ctx,
		`SELECT document_id
		   FROM run_scope_documents
		  WHERE profile_id = ? AND jd_id = ?
		  ORDER BY ordinal`,
		profileID,
		jdID,
	)
	if err != nil {
		return domain.ResumeRunScope{}, false, fmt.Errorf(
			"list run scope documents: %w",
			err,
		)
	}
	defer rows.Close()
	scope.DocumentIDs = make([]string, 0)
	for rows.Next() {
		var documentID string
		if err := rows.Scan(&documentID); err != nil {
			return domain.ResumeRunScope{}, false, fmt.Errorf(
				"scan run scope document: %w",
				err,
			)
		}
		scope.DocumentIDs = append(scope.DocumentIDs, documentID)
	}
	if err := rows.Err(); err != nil {
		return domain.ResumeRunScope{}, false, fmt.Errorf(
			"iterate run scope documents: %w",
			err,
		)
	}
	return scope, true, nil
}

func (repository *ResumeRepository) SaveScope(
	ctx context.Context,
	scope domain.ResumeRunScope,
) error {
	tx, err := repository.db.BeginTx(ctx, nil)
	if err != nil {
		return fmt.Errorf("begin run scope transaction: %w", err)
	}
	defer tx.Rollback()

	if _, err := tx.ExecContext(
		ctx,
		`INSERT INTO run_scopes(profile_id, jd_id, mode, updated_at)
		 VALUES (?, ?, ?, ?)
		 ON CONFLICT(profile_id, jd_id) DO UPDATE SET
			mode = excluded.mode,
			updated_at = excluded.updated_at`,
		scope.ProfileID,
		scope.JDID,
		scope.Mode,
		formatTime(scope.UpdatedAt),
	); err != nil {
		return fmt.Errorf("save run scope: %w", err)
	}
	if _, err := tx.ExecContext(
		ctx,
		`DELETE FROM run_scope_documents
		  WHERE profile_id = ? AND jd_id = ?`,
		scope.ProfileID,
		scope.JDID,
	); err != nil {
		return fmt.Errorf("clear run scope documents: %w", err)
	}
	for ordinal, documentID := range scope.DocumentIDs {
		if _, err := tx.ExecContext(
			ctx,
			`INSERT INTO run_scope_documents(
				profile_id, jd_id, document_id, ordinal
			) VALUES (?, ?, ?, ?)`,
			scope.ProfileID,
			scope.JDID,
			documentID,
			ordinal,
		); err != nil {
			return fmt.Errorf("insert run scope document: %w", err)
		}
	}

	if err := tx.Commit(); err != nil {
		return fmt.Errorf("commit run scope: %w", err)
	}
	return nil
}

func (repository *ResumeRepository) GetLatest(
	ctx context.Context,
	profileID string,
	jdID string,
) (domain.ResumeRun, domain.Resume, bool, error) {
	run, err := scanResumeRun(repository.db.QueryRowContext(
		ctx,
		`SELECT id, profile_id, jd_id, status, stage, packaging_level,
		        language, created_at, updated_at
		   FROM resume_runs
		  WHERE profile_id = ? AND jd_id = ?
		  ORDER BY updated_at DESC, created_at DESC, id
		  LIMIT 1`,
		profileID,
		jdID,
	))
	if errors.Is(err, sql.ErrNoRows) {
		return domain.ResumeRun{}, domain.Resume{}, false, nil
	}
	if err != nil {
		return domain.ResumeRun{}, domain.Resume{}, false, fmt.Errorf(
			"get latest resume run: %w",
			err,
		)
	}
	resume, found, err := repository.getLatestResume(ctx, run.ID)
	if err != nil {
		return domain.ResumeRun{}, domain.Resume{}, false, err
	}
	return run, resume, found, nil
}

func (repository *ResumeRepository) LatestRun(
	ctx context.Context,
) (domain.ResumeRun, bool, error) {
	run, err := scanResumeRun(repository.db.QueryRowContext(
		ctx,
		`SELECT id, profile_id, jd_id, status, stage, packaging_level,
		        language, created_at, updated_at
		   FROM resume_runs
		  ORDER BY updated_at DESC, created_at DESC, id
		  LIMIT 1`,
	))
	if errors.Is(err, sql.ErrNoRows) {
		return domain.ResumeRun{}, false, nil
	}
	if err != nil {
		return domain.ResumeRun{}, false, fmt.Errorf(
			"get latest resume run: %w",
			err,
		)
	}
	return run, true, nil
}

func (repository *ResumeRepository) SaveRun(
	ctx context.Context,
	run domain.ResumeRun,
) error {
	tx, err := repository.db.BeginTx(ctx, nil)
	if err != nil {
		return fmt.Errorf("begin resume run transaction: %w", err)
	}
	defer tx.Rollback()

	if err := saveResumeRun(ctx, tx, run); err != nil {
		return fmt.Errorf("save resume run: %w", err)
	}
	if err := tx.Commit(); err != nil {
		return fmt.Errorf("commit resume run: %w", err)
	}
	return nil
}

func (repository *ResumeRepository) SaveVersion(
	ctx context.Context,
	run domain.ResumeRun,
	resume domain.Resume,
) error {
	resume = domain.NormalizeResume(resume)
	structureJSON, err := json.Marshal(resumeStructureSnapshot{
		SchemaVersion:     resume.SchemaVersion,
		Language:          resume.Language,
		TargetRole:        resume.TargetRole,
		Header:            resume.Header,
		Sections:          resume.Sections,
		Blocks:            resume.Blocks,
		OptimizationNotes: resume.OptimizationNotes,
	})
	if err != nil {
		return fmt.Errorf("encode resume structure: %w", err)
	}

	tx, err := repository.db.BeginTx(ctx, nil)
	if err != nil {
		return fmt.Errorf("begin resume transaction: %w", err)
	}
	defer tx.Rollback()

	if err := saveResumeRun(ctx, tx, run); err != nil {
		return fmt.Errorf("save resume run: %w", err)
	}

	if _, err := tx.ExecContext(
		ctx,
		`INSERT INTO resumes(
			id, run_id, version, structure_json, markdown,
			created_at, input_hash
		) VALUES (?, ?, ?, ?, ?, ?, ?)`,
		resume.ID,
		resume.RunID,
		resume.Version,
		string(structureJSON),
		resume.Markdown,
		formatTime(resume.CreatedAt),
		resume.InputHash,
	); err != nil {
		return fmt.Errorf("insert resume version: %w", err)
	}

	for ordinal, block := range resume.Blocks {
		if _, err := tx.ExecContext(
			ctx,
			`INSERT INTO resume_blocks(
				resume_id, id, kind, ordinal, content, locked,
				grounding_level, optimization
			) VALUES (?, ?, ?, ?, ?, ?, ?, ?)`,
			resume.ID,
			block.ID,
			block.Kind,
			ordinal,
			block.Content,
			block.Locked,
			block.GroundingLevel,
			block.Optimization,
		); err != nil {
			return fmt.Errorf("insert resume block: %w", err)
		}
		for sourceOrdinal, evidenceID := range block.SourceEvidenceIDs {
			if _, err := tx.ExecContext(
				ctx,
				`INSERT INTO block_sources(
					resume_id, block_id, evidence_id, ordinal,
					relation, risk_level
				) VALUES (?, ?, ?, ?, 'supports', ?)`,
				resume.ID,
				block.ID,
				evidenceID,
				sourceOrdinal,
				resumeBlockRiskLevel(block),
			); err != nil {
				return fmt.Errorf("insert resume block source: %w", err)
			}
		}
	}

	if err := tx.Commit(); err != nil {
		return fmt.Errorf("commit resume version: %w", err)
	}
	return nil
}

func saveResumeRun(
	ctx context.Context,
	tx *sql.Tx,
	run domain.ResumeRun,
) error {
	_, err := tx.ExecContext(
		ctx,
		`INSERT INTO resume_runs(
			id, profile_id, jd_id, status, stage, packaging_level,
			language, created_at, updated_at
		) VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?)
		ON CONFLICT(id) DO UPDATE SET
			status = excluded.status,
			stage = excluded.stage,
			packaging_level = excluded.packaging_level,
			language = excluded.language,
			updated_at = excluded.updated_at`,
		run.ID,
		run.ProfileID,
		run.JDID,
		run.Status,
		run.Stage,
		run.PackagingLevel,
		run.Language,
		formatTime(run.CreatedAt),
		formatTime(run.UpdatedAt),
	)
	return err
}

type resumeRunScanner interface {
	Scan(dest ...any) error
}

func scanResumeRun(scanner resumeRunScanner) (domain.ResumeRun, error) {
	var run domain.ResumeRun
	var createdAt string
	var updatedAt string
	if err := scanner.Scan(
		&run.ID,
		&run.ProfileID,
		&run.JDID,
		&run.Status,
		&run.Stage,
		&run.PackagingLevel,
		&run.Language,
		&createdAt,
		&updatedAt,
	); err != nil {
		return domain.ResumeRun{}, err
	}
	var err error
	run.CreatedAt, err = parseTime(createdAt)
	if err != nil {
		return domain.ResumeRun{}, err
	}
	run.UpdatedAt, err = parseTime(updatedAt)
	if err != nil {
		return domain.ResumeRun{}, err
	}
	return run, nil
}

func (repository *ResumeRepository) getLatestResume(
	ctx context.Context,
	runID string,
) (domain.Resume, bool, error) {
	var resume domain.Resume
	var structureJSON string
	var createdAt string
	err := repository.db.QueryRowContext(
		ctx,
		`SELECT id, run_id, input_hash, version, structure_json,
		        markdown, created_at
		   FROM resumes
		  WHERE run_id = ?
		  ORDER BY version DESC
		  LIMIT 1`,
		runID,
	).Scan(
		&resume.ID,
		&resume.RunID,
		&resume.InputHash,
		&resume.Version,
		&structureJSON,
		&resume.Markdown,
		&createdAt,
	)
	if errors.Is(err, sql.ErrNoRows) {
		return domain.Resume{}, false, nil
	}
	if err != nil {
		return domain.Resume{}, false, fmt.Errorf(
			"get latest resume version: %w",
			err,
		)
	}
	resume.CreatedAt, err = parseTime(createdAt)
	if err != nil {
		return domain.Resume{}, false, err
	}

	var snapshot resumeStructureSnapshot
	if err := json.Unmarshal([]byte(structureJSON), &snapshot); err != nil {
		return domain.Resume{}, false, fmt.Errorf(
			"decode resume structure: %w",
			err,
		)
	}
	resume.Language = snapshot.Language
	resume.TargetRole = snapshot.TargetRole
	resume.SchemaVersion = snapshot.SchemaVersion
	resume.Header = snapshot.Header
	resume.Sections = snapshot.Sections
	resume.OptimizationNotes = snapshot.OptimizationNotes
	resume.Blocks, err = repository.listResumeBlocks(ctx, resume.ID)
	if err != nil {
		return domain.Resume{}, false, err
	}
	resume = domain.NormalizeResume(resume)
	return resume, true, nil
}

func (repository *ResumeRepository) listResumeBlocks(
	ctx context.Context,
	resumeID string,
) ([]domain.ResumeBlock, error) {
	rows, err := repository.db.QueryContext(
		ctx,
		`SELECT rb.id, rb.kind, rb.content, rb.locked,
		        rb.grounding_level, rb.optimization, bs.evidence_id
		   FROM resume_blocks rb
		   LEFT JOIN block_sources bs
		     ON bs.resume_id = rb.resume_id
		    AND bs.block_id = rb.id
		  WHERE rb.resume_id = ?
		  ORDER BY rb.ordinal, bs.ordinal`,
		resumeID,
	)
	if err != nil {
		return nil, fmt.Errorf("list resume blocks: %w", err)
	}
	defer rows.Close()

	blocks := make([]domain.ResumeBlock, 0)
	indexByID := make(map[string]int)
	for rows.Next() {
		var block domain.ResumeBlock
		var evidenceID sql.NullString
		if err := rows.Scan(
			&block.ID,
			&block.Kind,
			&block.Content,
			&block.Locked,
			&block.GroundingLevel,
			&block.Optimization,
			&evidenceID,
		); err != nil {
			return nil, fmt.Errorf("scan resume block: %w", err)
		}
		index, exists := indexByID[block.ID]
		if !exists {
			block.SourceEvidenceIDs = make([]string, 0)
			blocks = append(blocks, block)
			index = len(blocks) - 1
			indexByID[block.ID] = index
		}
		if evidenceID.Valid {
			blocks[index].SourceEvidenceIDs = append(
				blocks[index].SourceEvidenceIDs,
				evidenceID.String,
			)
		}
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("iterate resume blocks: %w", err)
	}
	return blocks, nil
}

func resumeBlockRiskLevel(block domain.ResumeBlock) string {
	if block.GroundingLevel == domain.GroundingDerived ||
		block.Kind == domain.ResumeBlockSummary {
		return "medium"
	}
	return "low"
}

var _ ports.ResumeRepository = (*ResumeRepository)(nil)
