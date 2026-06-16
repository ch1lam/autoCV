package domain

import (
	"errors"
	"fmt"
	"strings"
	"time"
)

const (
	MaxClarificationRounds            = 2
	MaxClarificationQuestionsPerRound = 5
)

type ClarificationQuestionStatus string

const (
	ClarificationPending  ClarificationQuestionStatus = "pending"
	ClarificationAnswered ClarificationQuestionStatus = "answered"
	ClarificationSkipped  ClarificationQuestionStatus = "skipped"
)

type ClarificationQuestion struct {
	ID            string
	RunID         string
	RequirementID string
	Round         int
	Ordinal       int
	Question      string
	Reason        string
	Status        ClarificationQuestionStatus
	Answer        string
	CreatedAt     time.Time
	UpdatedAt     time.Time
}

func ValidateClarificationQuestions(
	round int,
	questions []ClarificationQuestion,
) error {
	if round < 1 || round > MaxClarificationRounds {
		return fmt.Errorf("clarification round %d is outside 1..2", round)
	}
	if len(questions) > MaxClarificationQuestionsPerRound {
		return fmt.Errorf(
			"clarification round has %d questions, max is %d",
			len(questions),
			MaxClarificationQuestionsPerRound,
		)
	}
	seenOrdinals := make(map[int]struct{}, len(questions))
	for index, question := range questions {
		if question.Round != round {
			return fmt.Errorf(
				"clarification questions[%d] round %d does not match %d",
				index,
				question.Round,
				round,
			)
		}
		if _, exists := seenOrdinals[question.Ordinal]; exists {
			return fmt.Errorf(
				"duplicate clarification ordinal %d",
				question.Ordinal,
			)
		}
		seenOrdinals[question.Ordinal] = struct{}{}
		if err := question.Validate(); err != nil {
			return fmt.Errorf("clarification questions[%d]: %w", index, err)
		}
	}
	return nil
}

func (question ClarificationQuestion) Validate() error {
	if strings.TrimSpace(question.ID) == "" {
		return errors.New("clarification question id is empty")
	}
	if strings.TrimSpace(question.RunID) == "" {
		return errors.New("clarification run id is empty")
	}
	if question.Round < 1 || question.Round > MaxClarificationRounds {
		return fmt.Errorf(
			"clarification round %d is outside 1..2",
			question.Round,
		)
	}
	if question.Ordinal < 0 ||
		question.Ordinal >= MaxClarificationQuestionsPerRound {
		return fmt.Errorf(
			"clarification ordinal %d is outside 0..4",
			question.Ordinal,
		)
	}
	if strings.TrimSpace(question.Question) == "" {
		return errors.New("clarification question is empty")
	}
	if strings.TrimSpace(question.Reason) == "" {
		return errors.New("clarification reason is empty")
	}
	if err := ValidateClarificationResponse(
		question.Status,
		question.Answer,
	); err != nil {
		return err
	}
	if question.CreatedAt.IsZero() {
		return errors.New("clarification created_at is empty")
	}
	if question.UpdatedAt.IsZero() {
		return errors.New("clarification updated_at is empty")
	}
	return nil
}

func ValidateClarificationResponse(
	status ClarificationQuestionStatus,
	answer string,
) error {
	answer = strings.TrimSpace(answer)
	switch status {
	case ClarificationPending:
		if answer != "" {
			return errors.New("pending clarification answer must be empty")
		}
	case ClarificationAnswered:
		if answer == "" {
			return errors.New("answered clarification answer is empty")
		}
	case ClarificationSkipped:
	default:
		return fmt.Errorf("invalid clarification status %q", status)
	}
	return nil
}
