package ports

import (
	"context"

	"github.com/ch1lam/autocv/internal/domain"
)

type ResumeRepository interface {
	GetLatest(
		context.Context,
		string,
		string,
	) (domain.ResumeRun, domain.Resume, bool, error)
	SaveVersion(context.Context, domain.ResumeRun, domain.Resume) error
}
