package sqlite

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"strings"

	"github.com/ch1lam/autocv/internal/domain"
	"github.com/ch1lam/autocv/internal/ports"
)

type RunConfirmationRepository struct {
	db *sql.DB
}

type runConfirmationScanner interface {
	Scan(dest ...any) error
}

func NewRunConfirmationRepository(db *sql.DB) *RunConfirmationRepository {
	return &RunConfirmationRepository{db: db}
}

func (repository *RunConfirmationRepository) ListRunConfirmations(
	ctx context.Context,
	runID string,
) ([]domain.RunConfirmation, error) {
	runID = strings.TrimSpace(runID)
	if runID == "" {
		return nil, errors.New("run confirmation run id is empty")
	}
	rows, err := repository.db.QueryContext(
		ctx,
		`SELECT id, run_id, clarification_question_id, requirement_id,
		        content, created_at, updated_at
		   FROM run_confirmations
		  WHERE run_id = ?
		  ORDER BY created_at, id`,
		runID,
	)
	if err != nil {
		return nil, fmt.Errorf("list run confirmations: %w", err)
	}
	defer rows.Close()

	confirmations := make([]domain.RunConfirmation, 0)
	for rows.Next() {
		confirmation, err := scanRunConfirmation(rows)
		if err != nil {
			return nil, err
		}
		confirmations = append(confirmations, confirmation)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("iterate run confirmations: %w", err)
	}
	return confirmations, nil
}

func (repository *RunConfirmationRepository) SaveRunConfirmation(
	ctx context.Context,
	confirmation domain.RunConfirmation,
) error {
	confirmation.ID = strings.TrimSpace(confirmation.ID)
	confirmation.RunID = strings.TrimSpace(confirmation.RunID)
	confirmation.ClarificationQuestionID = strings.TrimSpace(
		confirmation.ClarificationQuestionID,
	)
	confirmation.RequirementID = strings.TrimSpace(confirmation.RequirementID)
	confirmation.Content = strings.TrimSpace(confirmation.Content)
	if err := confirmation.Validate(); err != nil {
		return err
	}

	if _, err := repository.db.ExecContext(
		ctx,
		`INSERT INTO run_confirmations(
			id, run_id, clarification_question_id, requirement_id,
			content, created_at, updated_at
		) VALUES (?, ?, ?, ?, ?, ?, ?)
		ON CONFLICT(run_id, clarification_question_id) DO UPDATE SET
			requirement_id = excluded.requirement_id,
			content = excluded.content,
			updated_at = excluded.updated_at`,
		confirmation.ID,
		confirmation.RunID,
		confirmation.ClarificationQuestionID,
		confirmation.RequirementID,
		confirmation.Content,
		formatTime(confirmation.CreatedAt),
		formatTime(confirmation.UpdatedAt),
	); err != nil {
		return fmt.Errorf("save run confirmation: %w", err)
	}
	return nil
}

func (repository *RunConfirmationRepository) DeleteRunConfirmation(
	ctx context.Context,
	runID string,
	clarificationQuestionID string,
) error {
	runID = strings.TrimSpace(runID)
	clarificationQuestionID = strings.TrimSpace(clarificationQuestionID)
	if runID == "" {
		return errors.New("run confirmation run id is empty")
	}
	if clarificationQuestionID == "" {
		return errors.New("run confirmation clarification question id is empty")
	}
	if _, err := repository.db.ExecContext(
		ctx,
		`DELETE FROM run_confirmations
		  WHERE run_id = ? AND clarification_question_id = ?`,
		runID,
		clarificationQuestionID,
	); err != nil {
		return fmt.Errorf("delete run confirmation: %w", err)
	}
	return nil
}

func scanRunConfirmation(
	scanner runConfirmationScanner,
) (domain.RunConfirmation, error) {
	var confirmation domain.RunConfirmation
	var createdAt string
	var updatedAt string
	if err := scanner.Scan(
		&confirmation.ID,
		&confirmation.RunID,
		&confirmation.ClarificationQuestionID,
		&confirmation.RequirementID,
		&confirmation.Content,
		&createdAt,
		&updatedAt,
	); err != nil {
		return domain.RunConfirmation{}, fmt.Errorf(
			"scan run confirmation: %w",
			err,
		)
	}
	parsedCreatedAt, err := parseTime(createdAt)
	if err != nil {
		return domain.RunConfirmation{}, err
	}
	parsedUpdatedAt, err := parseTime(updatedAt)
	if err != nil {
		return domain.RunConfirmation{}, err
	}
	confirmation.CreatedAt = parsedCreatedAt
	confirmation.UpdatedAt = parsedUpdatedAt
	return confirmation, nil
}

var _ ports.RunConfirmationRepository = (*RunConfirmationRepository)(nil)
