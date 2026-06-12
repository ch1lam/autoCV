package ports

import (
	"context"

	"github.com/ch1lam/autocv/internal/domain"
)

type MatchRepository interface {
	GetLatest(
		context.Context,
		string,
		string,
	) (domain.MatchAnalysis, bool, error)
	Save(context.Context, domain.MatchAnalysis) error
}
