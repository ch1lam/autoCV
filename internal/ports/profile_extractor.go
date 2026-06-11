package ports

import (
	"context"

	"github.com/ch1lam/autocv/internal/domain"
)

type ExtractProfileRequest struct {
	Chunks []domain.SourceChunk
}

type ProfileExtractor interface {
	ExtractProfile(
		context.Context,
		ExtractProfileRequest,
	) ([]domain.ExtractedEvidence, error)
}
