package ports

import (
	"context"

	"github.com/ch1lam/autocv/internal/domain"
)

type AnalyzeJDRequest struct {
	Text         string
	LanguageHint domain.JDLanguage
}

type JDAnalyzer interface {
	AnalyzeJD(context.Context, AnalyzeJDRequest) (domain.JDAnalysis, error)
}
