package ports

import (
	"context"
	"time"

	"github.com/ch1lam/autocv/internal/domain"
)

type JDAnalysisUpdate struct {
	ID           string
	RawHash      string
	Title        string
	Company      string
	Language     string
	AnalysisJSON string
	Status       string
	Error        string
	UpdatedAt    time.Time
}

type JDRepository interface {
	GetLatest(context.Context) (domain.JobDescription, bool, error)
	SaveDraft(context.Context, domain.JobDescription) error
	UpdateAnalysis(context.Context, JDAnalysisUpdate) error
}
