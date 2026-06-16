package ports

import (
	"context"

	"github.com/ch1lam/autocv/internal/domain"
)

type ResumeRepository interface {
	RunScopeRepository
	ResumeRunRepository
	GetLatest(
		context.Context,
		string,
		string,
	) (domain.ResumeRun, domain.Resume, bool, error)
	SaveVersion(context.Context, domain.ResumeRun, domain.Resume) error
}

type ResumeRunRepository interface {
	SaveRun(context.Context, domain.ResumeRun) error
}

type RunScopeRepository interface {
	GetScope(
		context.Context,
		string,
		string,
	) (domain.ResumeRunScope, bool, error)
	SaveScope(context.Context, domain.ResumeRunScope) error
}
