package ports

import (
	"context"

	"github.com/ch1lam/autocv/internal/domain"
)

type ProviderCallRecorder interface {
	Record(context.Context, domain.ProviderCall) error
}
