package sqlite

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/ch1lam/autocv/internal/domain"
	"github.com/ch1lam/autocv/internal/ports"
)

type ClarificationRepository struct {
	db *sql.DB
}

type clarificationScanner interface {
	Scan(dest ...any) error
}

func NewClarificationRepository(db *sql.DB) *ClarificationRepository {
	return &ClarificationRepository{db: db}
}

func (repository *ClarificationRepository) ListQuestions(
	ctx context.Context,
	runID string,
) ([]domain.ClarificationQuestion, error) {
	rows, err := repository.db.QueryContext(
		ctx,
		`SELECT id, run_id, requirement_id, round, ordinal, question,
		        reason, status, answer, created_at, updated_at
		   FROM clarification_questions
		  WHERE run_id = ?
		  ORDER BY round, ordinal`,
		runID,
	)
	if err != nil {
		return nil, fmt.Errorf("list clarification questions: %w", err)
	}
	defer rows.Close()

	questions := make([]domain.ClarificationQuestion, 0)
	for rows.Next() {
		question, err := scanClarificationQuestion(rows)
		if err != nil {
			return nil, err
		}
		questions = append(questions, question)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("iterate clarification questions: %w", err)
	}
	return questions, nil
}

func (repository *ClarificationRepository) ReplaceRoundQuestions(
	ctx context.Context,
	runID string,
	round int,
	questions []domain.ClarificationQuestion,
) error {
	runID = strings.TrimSpace(runID)
	if runID == "" {
		return errors.New("clarification run id is empty")
	}
	if err := domain.ValidateClarificationQuestions(round, questions); err != nil {
		return err
	}
	for index, question := range questions {
		if question.RunID != runID {
			return fmt.Errorf(
				"clarification questions[%d] run id %q does not match %q",
				index,
				question.RunID,
				runID,
			)
		}
	}

	tx, err := repository.db.BeginTx(ctx, nil)
	if err != nil {
		return fmt.Errorf("begin clarification transaction: %w", err)
	}
	defer tx.Rollback()

	if _, err := tx.ExecContext(
		ctx,
		`DELETE FROM clarification_questions
		  WHERE run_id = ? AND round = ?`,
		runID,
		round,
	); err != nil {
		return fmt.Errorf("clear clarification round: %w", err)
	}
	for _, question := range questions {
		if _, err := tx.ExecContext(
			ctx,
			`INSERT INTO clarification_questions(
				id, run_id, requirement_id, round, ordinal, question,
				reason, status, answer, created_at, updated_at
			) VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)`,
			question.ID,
			question.RunID,
			question.RequirementID,
			question.Round,
			question.Ordinal,
			question.Question,
			question.Reason,
			question.Status,
			question.Answer,
			formatTime(question.CreatedAt),
			formatTime(question.UpdatedAt),
		); err != nil {
			return fmt.Errorf("insert clarification question: %w", err)
		}
	}
	if err := tx.Commit(); err != nil {
		return fmt.Errorf("commit clarification transaction: %w", err)
	}
	return nil
}

func (repository *ClarificationRepository) UpdateQuestionStatus(
	ctx context.Context,
	questionID string,
	status domain.ClarificationQuestionStatus,
	answer string,
	updatedAt time.Time,
) (domain.ClarificationQuestion, error) {
	questionID = strings.TrimSpace(questionID)
	if questionID == "" {
		return domain.ClarificationQuestion{}, errors.New(
			"clarification question id is empty",
		)
	}
	answer = strings.TrimSpace(answer)
	if err := domain.ValidateClarificationResponse(status, answer); err != nil {
		return domain.ClarificationQuestion{}, err
	}
	if updatedAt.IsZero() {
		return domain.ClarificationQuestion{}, errors.New(
			"clarification updated_at is empty",
		)
	}

	result, err := repository.db.ExecContext(
		ctx,
		`UPDATE clarification_questions
		    SET status = ?, answer = ?, updated_at = ?
		  WHERE id = ?`,
		status,
		answer,
		formatTime(updatedAt),
		questionID,
	)
	if err != nil {
		return domain.ClarificationQuestion{}, fmt.Errorf(
			"update clarification question: %w",
			err,
		)
	}
	affected, err := result.RowsAffected()
	if err != nil {
		return domain.ClarificationQuestion{}, fmt.Errorf(
			"read clarification update count: %w",
			err,
		)
	}
	if affected == 0 {
		return domain.ClarificationQuestion{}, fmt.Errorf(
			"clarification question %q not found",
			questionID,
		)
	}
	return repository.getQuestion(ctx, questionID)
}

func (repository *ClarificationRepository) getQuestion(
	ctx context.Context,
	questionID string,
) (domain.ClarificationQuestion, error) {
	question, err := scanClarificationQuestion(repository.db.QueryRowContext(
		ctx,
		`SELECT id, run_id, requirement_id, round, ordinal, question,
		        reason, status, answer, created_at, updated_at
		   FROM clarification_questions
		  WHERE id = ?`,
		questionID,
	))
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return domain.ClarificationQuestion{}, fmt.Errorf(
				"clarification question %q not found",
				questionID,
			)
		}
		return domain.ClarificationQuestion{}, err
	}
	return question, nil
}

func scanClarificationQuestion(
	scanner clarificationScanner,
) (domain.ClarificationQuestion, error) {
	var question domain.ClarificationQuestion
	var status string
	var createdAt string
	var updatedAt string
	if err := scanner.Scan(
		&question.ID,
		&question.RunID,
		&question.RequirementID,
		&question.Round,
		&question.Ordinal,
		&question.Question,
		&question.Reason,
		&status,
		&question.Answer,
		&createdAt,
		&updatedAt,
	); err != nil {
		return domain.ClarificationQuestion{}, fmt.Errorf(
			"scan clarification question: %w",
			err,
		)
	}
	parsedCreatedAt, err := parseTime(createdAt)
	if err != nil {
		return domain.ClarificationQuestion{}, err
	}
	parsedUpdatedAt, err := parseTime(updatedAt)
	if err != nil {
		return domain.ClarificationQuestion{}, err
	}
	question.Status = domain.ClarificationQuestionStatus(status)
	question.CreatedAt = parsedCreatedAt
	question.UpdatedAt = parsedUpdatedAt
	return question, nil
}

var _ ports.ClarificationRepository = (*ClarificationRepository)(nil)
