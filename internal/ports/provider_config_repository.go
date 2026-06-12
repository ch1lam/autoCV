package ports

import (
	"context"

	"github.com/ch1lam/autocv/internal/domain"
)

type ProviderConfigRepository interface {
	GetActive(context.Context) (domain.ProviderConfig, bool, error)
	GetByProvider(
		context.Context,
		string,
	) (domain.ProviderConfig, bool, error)
	Save(context.Context, domain.ProviderConfig) error
}
