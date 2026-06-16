package domain

import (
	"strings"
	"testing"
	"time"
)

func TestValidateClarificationQuestionsLimitsRoundsAndCount(t *testing.T) {
	now := time.Date(2026, 6, 16, 8, 0, 0, 0, time.UTC)
	questions := make([]ClarificationQuestion, 0, 6)
	for index := 0; index < 6; index++ {
		questions = append(questions, clarificationQuestionFixture(
			"question-"+string(rune('a'+index)),
			1,
			index,
			now,
		))
	}

	err := ValidateClarificationQuestions(1, questions)
	if err == nil || !strings.Contains(err.Error(), "max is 5") {
		t.Fatalf("expected max question error, got %v", err)
	}

	questions = questions[:1]
	questions[0].Round = 3
	err = ValidateClarificationQuestions(3, questions)
	if err == nil || !strings.Contains(err.Error(), "outside 1..2") {
		t.Fatalf("expected round error, got %v", err)
	}
}

func TestValidateClarificationQuestionRequiresAnswerWhenAnswered(t *testing.T) {
	now := time.Date(2026, 6, 16, 8, 0, 0, 0, time.UTC)
	question := clarificationQuestionFixture("question-1", 1, 0, now)
	question.Status = ClarificationAnswered

	err := question.Validate()
	if err == nil || !strings.Contains(err.Error(), "answer is empty") {
		t.Fatalf("expected missing answer error, got %v", err)
	}

	question.Answer = "  有 8 人团队管理经验。 "
	if err := question.Validate(); err != nil {
		t.Fatalf("validate answered question: %v", err)
	}
}

func clarificationQuestionFixture(
	id string,
	round int,
	ordinal int,
	now time.Time,
) ClarificationQuestion {
	return ClarificationQuestion{
		ID:            id,
		RunID:         "run-1",
		RequirementID: "requirement-1",
		Round:         round,
		Ordinal:       ordinal,
		Question:      "是否有团队规模信息？",
		Reason:        "JD 要求负责人经验，但资料缺少团队规模。",
		Status:        ClarificationPending,
		CreatedAt:     now,
		UpdatedAt:     now,
	}
}
