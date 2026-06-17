package sqlite

import (
	"context"
	"database/sql"
	"fmt"

	"github.com/ch1lam/autocv/internal/ports"
	"github.com/ch1lam/autocv/internal/workflow"
)

type StageResultRepository struct {
	db *sql.DB
}

func NewStageResultRepository(db *sql.DB) *StageResultRepository {
	return &StageResultRepository{db: db}
}

func (repository *StageResultRepository) SaveStageResult(
	ctx context.Context,
	result workflow.StageResult,
) error {
	if err := result.Validate(); err != nil {
		return fmt.Errorf("validate stage result: %w", err)
	}
	if _, err := repository.db.ExecContext(
		ctx,
		`INSERT INTO stage_results(
			id, run_id, stage, input_hash, status, result_json, error_json,
			created_at, updated_at
		) VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?)
		ON CONFLICT(run_id, stage, input_hash) DO UPDATE SET
			id = excluded.id,
			status = excluded.status,
			result_json = excluded.result_json,
			error_json = excluded.error_json,
			updated_at = excluded.updated_at`,
		result.ID,
		result.RunID,
		string(result.Stage),
		result.InputHash,
		string(result.Status),
		nullableStageJSON(result.ResultJSON),
		nullableStageJSON(result.ErrorJSON),
		formatTime(result.CreatedAt),
		formatTime(result.UpdatedAt),
	); err != nil {
		return fmt.Errorf("save stage result: %w", err)
	}
	return nil
}

func (repository *StageResultRepository) LatestStageResult(
	ctx context.Context,
	runID string,
	stage workflow.Stage,
) (workflow.StageResult, bool, error) {
	row := repository.db.QueryRowContext(
		ctx,
		`SELECT id, run_id, stage, input_hash, status, result_json, error_json,
		        created_at, updated_at
		   FROM stage_results
		  WHERE run_id = ? AND stage = ?
		  ORDER BY updated_at DESC, created_at DESC
		  LIMIT 1`,
		runID,
		string(stage),
	)
	result, err := scanStageResult(row)
	if err == sql.ErrNoRows {
		return workflow.StageResult{}, false, nil
	}
	if err != nil {
		return workflow.StageResult{}, false, fmt.Errorf(
			"get latest stage result: %w",
			err,
		)
	}
	return result, true, nil
}

func (repository *StageResultRepository) SucceededStageResult(
	ctx context.Context,
	runID string,
	stage workflow.Stage,
	inputHash string,
) (workflow.StageResult, bool, error) {
	row := repository.db.QueryRowContext(
		ctx,
		`SELECT id, run_id, stage, input_hash, status, result_json, error_json,
		        created_at, updated_at
		   FROM stage_results
		  WHERE run_id = ? AND stage = ? AND input_hash = ? AND status = ?
		  LIMIT 1`,
		runID,
		string(stage),
		inputHash,
		string(workflow.StageStatusSucceeded),
	)
	result, err := scanStageResult(row)
	if err == sql.ErrNoRows {
		return workflow.StageResult{}, false, nil
	}
	if err != nil {
		return workflow.StageResult{}, false, fmt.Errorf(
			"get succeeded stage result: %w",
			err,
		)
	}
	return result, true, nil
}

type stageResultScanner interface {
	Scan(dest ...any) error
}

func scanStageResult(
	scanner stageResultScanner,
) (workflow.StageResult, error) {
	var result workflow.StageResult
	var stage string
	var status string
	var resultJSON sql.NullString
	var errorJSON sql.NullString
	var createdAt string
	var updatedAt string
	if err := scanner.Scan(
		&result.ID,
		&result.RunID,
		&stage,
		&result.InputHash,
		&status,
		&resultJSON,
		&errorJSON,
		&createdAt,
		&updatedAt,
	); err != nil {
		return workflow.StageResult{}, err
	}
	result.Stage = workflow.Stage(stage)
	result.Status = workflow.StageStatus(status)
	if resultJSON.Valid {
		result.ResultJSON = resultJSON.String
	}
	if errorJSON.Valid {
		result.ErrorJSON = errorJSON.String
	}
	var err error
	result.CreatedAt, err = parseTime(createdAt)
	if err != nil {
		return workflow.StageResult{}, err
	}
	result.UpdatedAt, err = parseTime(updatedAt)
	if err != nil {
		return workflow.StageResult{}, err
	}
	return result, nil
}

func nullableStageJSON(value string) any {
	if value == "" {
		return nil
	}
	return value
}

var _ ports.StageResultRepository = (*StageResultRepository)(nil)
