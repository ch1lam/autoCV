package domain

import (
	"testing"
	"time"
)

func TestProviderCallValidate(t *testing.T) {
	call := ProviderCall{
		ID:            "call-1",
		Provider:      ProviderOpenAI,
		Model:         "gpt-5.5",
		Task:          "jd_analysis",
		PromptVersion: "v1",
		InputHash:     "hash",
		Status:        ProviderCallStatusSucceeded,
		DurationMS:    25,
		InputTokens:   10,
		OutputTokens:  5,
		TotalTokens:   15,
		CreatedAt:     time.Date(2026, 6, 12, 9, 0, 0, 0, time.UTC),
	}
	if err := call.Validate(); err != nil {
		t.Fatalf("validate Provider call: %v", err)
	}

	call.Status = "other"
	if err := call.Validate(); err == nil {
		t.Fatal("expected invalid status error")
	}
}
