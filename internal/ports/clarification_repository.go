package ports

import (
	"context"
	"time"

	"github.com/ch1lam/autocv/internal/domain"
)

type ClarificationRepository interface {
	ListQuestions(
		context.Context,
		string,
	) ([]domain.ClarificationQuestion, error)
	ReplaceRoundQuestions(
		context.Context,
		string,
		int,
		[]domain.ClarificationQuestion,
	) error
	UpdateQuestionStatus(
		context.Context,
		string,
		domain.ClarificationQuestionStatus,
		string,
		time.Time,
	) (domain.ClarificationQuestion, error)
}
