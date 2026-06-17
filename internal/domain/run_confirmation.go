package domain

import (
	"errors"
	"strings"
	"time"
)

type RunConfirmation struct {
	ID                      string
	RunID                   string
	ClarificationQuestionID string
	RequirementID           string
	Content                 string
	CreatedAt               time.Time
	UpdatedAt               time.Time
}

func (confirmation RunConfirmation) Validate() error {
	if strings.TrimSpace(confirmation.ID) == "" {
		return errors.New("run confirmation id is empty")
	}
	if strings.TrimSpace(confirmation.RunID) == "" {
		return errors.New("run confirmation run id is empty")
	}
	if strings.TrimSpace(confirmation.ClarificationQuestionID) == "" {
		return errors.New("run confirmation clarification question id is empty")
	}
	if strings.TrimSpace(confirmation.RequirementID) == "" {
		return errors.New("run confirmation requirement id is empty")
	}
	if strings.TrimSpace(confirmation.Content) == "" {
		return errors.New("run confirmation content is empty")
	}
	if confirmation.CreatedAt.IsZero() {
		return errors.New("run confirmation created_at is empty")
	}
	if confirmation.UpdatedAt.IsZero() {
		return errors.New("run confirmation updated_at is empty")
	}
	return nil
}
