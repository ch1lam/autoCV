package ports

import (
	"context"

	"github.com/ch1lam/autocv/internal/domain"
)

type ArtifactRepository interface {
	GetLatest(
		context.Context,
		string,
		domain.ArtifactKind,
	) (domain.Artifact, bool, error)
	Save(context.Context, domain.Artifact) error
}
