package ports

import (
	"context"

	"github.com/ch1lam/autocv/internal/domain"
)

type SuggestMatchesRequest struct {
	Requirements []domain.MatchRequirement
	Evidence     []domain.Evidence
}

type MatchSuggester interface {
	SuggestMatches(
		context.Context,
		SuggestMatchesRequest,
	) ([]domain.MatchSuggestion, error)
}
