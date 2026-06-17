package ports

import (
	"context"

	"github.com/ch1lam/autocv/internal/domain"
)

type DraftResumeRequest struct {
	Language       domain.ResumeLanguage
	TargetRole     string
	PackagingLevel float64
	Match          domain.MatchAnalysis
	Evidence       []domain.Evidence
	Confirmations  []domain.RunConfirmation
}

type ResumeDrafter interface {
	DraftResume(context.Context, DraftResumeRequest) (domain.ResumeDraft, error)
}
