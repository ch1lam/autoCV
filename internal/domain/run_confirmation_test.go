package domain

import (
	"strings"
	"testing"
	"time"
)

func TestRunConfirmationValidateRequiresGroundedContent(t *testing.T) {
	now := time.Date(2026, 6, 17, 9, 0, 0, 0, time.UTC)
	confirmation := RunConfirmation{
		ID:                      "confirmation-1",
		RunID:                   "run-1",
		ClarificationQuestionID: "question-1",
		RequirementID:           "requirement-1",
		Content:                 "  管理过 8 人后端团队。 ",
		CreatedAt:               now,
		UpdatedAt:               now,
	}

	if err := confirmation.Validate(); err != nil {
		t.Fatalf("validate run confirmation: %v", err)
	}

	confirmation.Content = " "
	err := confirmation.Validate()
	if err == nil || !strings.Contains(err.Error(), "content is empty") {
		t.Fatalf("expected empty content error, got %v", err)
	}
}
