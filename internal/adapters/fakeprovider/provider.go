package fakeprovider

import (
	"context"
	"embed"
	"errors"
	"fmt"
	"strings"

	"github.com/ch1lam/autocv/internal/domain"
	"github.com/ch1lam/autocv/internal/ports"
)

//go:embed fixtures/jd_analysis*.json
var fixtures embed.FS

type Provider struct{}

func New() *Provider {
	return &Provider{}
}

func (provider *Provider) AnalyzeJD(
	ctx context.Context,
	request ports.AnalyzeJDRequest,
) (domain.JDAnalysis, error) {
	if err := ctx.Err(); err != nil {
		return domain.JDAnalysis{}, err
	}
	if strings.TrimSpace(request.Text) == "" {
		return domain.JDAnalysis{}, errors.New("JD text is empty")
	}

	fixtureName := "fixtures/jd_analysis.json"
	switch request.LanguageHint {
	case domain.JDLanguageChinese:
		fixtureName = "fixtures/jd_analysis_zh.json"
	case domain.JDLanguageEnglish:
		fixtureName = "fixtures/jd_analysis_en.json"
	}
	contents, err := fixtures.ReadFile(fixtureName)
	if err != nil {
		return domain.JDAnalysis{}, fmt.Errorf("read fake JD analysis: %w", err)
	}
	analysis, err := domain.DecodeJDAnalysis(contents)
	if err != nil {
		return domain.JDAnalysis{}, fmt.Errorf("validate fake JD analysis: %w", err)
	}
	return analysis, nil
}

var _ ports.JDAnalyzer = (*Provider)(nil)
