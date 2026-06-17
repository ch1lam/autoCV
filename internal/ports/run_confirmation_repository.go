package ports

import (
	"context"

	"github.com/ch1lam/autocv/internal/domain"
)

type RunConfirmationRepository interface {
	ListRunConfirmations(
		context.Context,
		string,
	) ([]domain.RunConfirmation, error)
	SaveRunConfirmation(context.Context, domain.RunConfirmation) error
	DeleteRunConfirmation(context.Context, string, string) error
}
