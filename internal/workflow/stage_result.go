package workflow

import (
	"errors"
	"fmt"
	"strings"
	"time"
)

type StageResult struct {
	ID         string
	RunID      string
	Stage      Stage
	InputHash  string
	Status     StageStatus
	ResultJSON string
	ErrorJSON  string
	CreatedAt  time.Time
	UpdatedAt  time.Time
}

func (result StageResult) Validate() error {
	if strings.TrimSpace(result.ID) == "" {
		return errors.New("stage result id is empty")
	}
	if strings.TrimSpace(result.RunID) == "" {
		return errors.New("stage result run id is empty")
	}
	if !result.Stage.Valid() {
		return fmt.Errorf("invalid workflow stage %q", result.Stage)
	}
	if strings.TrimSpace(result.InputHash) == "" {
		return errors.New("stage result input hash is empty")
	}
	if !result.Status.Valid() {
		return fmt.Errorf("invalid workflow stage status %q", result.Status)
	}
	if result.CreatedAt.IsZero() {
		return errors.New("stage result created time is empty")
	}
	if result.UpdatedAt.IsZero() {
		return errors.New("stage result updated time is empty")
	}
	return nil
}
