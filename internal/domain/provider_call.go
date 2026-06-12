package domain

import (
	"errors"
	"fmt"
	"strings"
	"time"
)

const (
	ProviderCallStatusSucceeded = "succeeded"
	ProviderCallStatusFailed    = "failed"
	ProviderCallStatusCancelled = "cancelled"
)

type ProviderCall struct {
	ID             string
	Provider       string
	Model          string
	Task           string
	PromptVersion  string
	InputHash      string
	Status         string
	DurationMS     int64
	InputTokens    int
	OutputTokens   int
	TotalTokens    int
	SchemaRepaired bool
	ErrorKind      string
	CreatedAt      time.Time
}

func (call ProviderCall) Validate() error {
	for field, value := range map[string]string{
		"id":             call.ID,
		"provider":       call.Provider,
		"model":          call.Model,
		"task":           call.Task,
		"prompt version": call.PromptVersion,
		"input hash":     call.InputHash,
	} {
		if strings.TrimSpace(value) == "" {
			return fmt.Errorf("Provider call %s is empty", field)
		}
	}
	switch call.Status {
	case ProviderCallStatusSucceeded,
		ProviderCallStatusFailed,
		ProviderCallStatusCancelled:
	default:
		return fmt.Errorf("invalid Provider call status %q", call.Status)
	}
	if call.DurationMS < 0 {
		return errors.New("Provider call duration is negative")
	}
	if call.InputTokens < 0 ||
		call.OutputTokens < 0 ||
		call.TotalTokens < 0 {
		return errors.New("Provider call token usage is negative")
	}
	if call.TotalTokens > 0 &&
		call.TotalTokens < call.InputTokens+call.OutputTokens {
		return errors.New("Provider call total tokens are inconsistent")
	}
	if call.CreatedAt.IsZero() {
		return errors.New("Provider call created time is empty")
	}
	return nil
}
