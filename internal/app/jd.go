package app

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"log/slog"
	"strings"
	"unicode"
	"unicode/utf8"

	"github.com/ch1lam/autocv/internal/domain"
	"github.com/ch1lam/autocv/internal/ports"
	"github.com/google/uuid"
)

const (
	defaultJDTitle       = "待分析职位"
	shortJDWarningRunes  = 120
	maxDraftTitleRunes   = 80
	defaultJDAnalysisErr = "JD analysis failed"
)

var defaultJDID = uuid.NewSHA1(
	uuid.NameSpaceURL,
	[]byte("https://autocv.local/job-descriptions/default"),
).String()

type JDService struct {
	repository ports.JDRepository
	analyzer   ports.JDAnalyzer
	clock      ports.Clock
}

type JDWorkspace struct {
	ID             string             `json:"id"`
	Title          string             `json:"title"`
	Company        string             `json:"company"`
	RawText        string             `json:"rawText"`
	Language       string             `json:"language"`
	AnalysisStatus string             `json:"analysisStatus"`
	AnalysisError  string             `json:"analysisError"`
	UpdatedAt      string             `json:"updatedAt"`
	Warnings       []string           `json:"warnings"`
	Analysis       *JDAnalysisSummary `json:"analysis"`
}

type JDAnalysisSummary struct {
	Role                 string                 `json:"role"`
	Company              string                 `json:"company"`
	Level                string                 `json:"level"`
	Language             string                 `json:"language"`
	Responsibilities     []JDRequirementSummary `json:"responsibilities"`
	RequiredSkills       []JDRequirementSummary `json:"requiredSkills"`
	PreferredSkills      []JDRequirementSummary `json:"preferredSkills"`
	DomainSignals        []string               `json:"domainSignals"`
	ScreeningConstraints []string               `json:"screeningConstraints"`
	Ambiguities          []string               `json:"ambiguities"`
}

type JDRequirementSummary struct {
	ID             string `json:"id"`
	Text           string `json:"text"`
	Importance     int    `json:"importance"`
	HardConstraint bool   `json:"hardConstraint"`
}

func NewJDService(
	repository ports.JDRepository,
	analyzer ports.JDAnalyzer,
	clock ports.Clock,
) *JDService {
	return &JDService{
		repository: repository,
		analyzer:   analyzer,
		clock:      clock,
	}
}

func (service *JDService) GetWorkspace() (JDWorkspace, error) {
	return service.getWorkspace(context.Background())
}

func (service *JDService) SaveDraft(rawText string) (JDWorkspace, error) {
	if _, err := service.saveDraft(context.Background(), rawText); err != nil {
		return JDWorkspace{}, err
	}
	return service.getWorkspace(context.Background())
}

func (service *JDService) Analyze(rawText string) (JDWorkspace, error) {
	ctx := context.Background()
	draft, err := service.saveDraft(ctx, rawText)
	if err != nil {
		return JDWorkspace{}, err
	}

	analysis, err := service.analyzer.AnalyzeJD(
		ctx,
		ports.AnalyzeJDRequest{
			Text:         draft.RawText,
			LanguageHint: languageHint(draft.RawText),
		},
	)
	if err != nil {
		service.saveAnalysisFailure(ctx, draft, err)
		return JDWorkspace{}, fmt.Errorf("analyze JD: %w", err)
	}
	if err := analysis.Validate(); err != nil {
		service.saveAnalysisFailure(ctx, draft, err)
		return JDWorkspace{}, fmt.Errorf("validate JD analysis: %w", err)
	}
	contents, err := json.Marshal(analysis)
	if err != nil {
		service.saveAnalysisFailure(ctx, draft, err)
		return JDWorkspace{}, fmt.Errorf("encode JD analysis: %w", err)
	}

	company := ""
	if analysis.Company != nil {
		company = strings.TrimSpace(*analysis.Company)
	}
	now := service.clock.Now().UTC()
	if err := service.repository.UpdateAnalysis(
		ctx,
		ports.JDAnalysisUpdate{
			ID:           draft.ID,
			RawHash:      draft.RawHash,
			Title:        analysis.Role,
			Company:      company,
			Language:     string(analysis.Language),
			AnalysisJSON: string(contents),
			Status:       "succeeded",
			UpdatedAt:    now,
		},
	); err != nil {
		return JDWorkspace{}, err
	}

	slog.Info(
		"jd.analysis.succeeded",
		slog.String("jd_id", draft.ID),
		slog.String("raw_hash", draft.RawHash),
		slog.Int(
			"requirement_count",
			len(analysis.Responsibilities)+
				len(analysis.RequiredSkills)+
				len(analysis.PreferredSkills),
		),
	)
	return service.getWorkspace(ctx)
}

func (service *JDService) getWorkspace(
	ctx context.Context,
) (JDWorkspace, error) {
	item, found, err := service.repository.GetLatest(ctx)
	if err != nil {
		return JDWorkspace{}, err
	}
	if !found {
		return JDWorkspace{
			Title:          defaultJDTitle,
			AnalysisStatus: "empty",
			Warnings:       make([]string, 0),
		}, nil
	}
	return jdWorkspace(item)
}

func (service *JDService) saveDraft(
	ctx context.Context,
	rawText string,
) (domain.JobDescription, error) {
	rawText = strings.TrimSpace(rawText)
	if rawText == "" {
		return domain.JobDescription{}, errors.New("JD text is empty")
	}
	rawHash := hashJDText(rawText)
	existing, found, err := service.repository.GetLatest(ctx)
	if err != nil {
		return domain.JobDescription{}, err
	}
	if found && existing.RawHash == rawHash {
		return existing, nil
	}

	now := service.clock.Now().UTC()
	item := domain.JobDescription{
		ID:             defaultJDID,
		Title:          draftTitle(rawText),
		RawText:        rawText,
		Language:       string(languageHint(rawText)),
		RawHash:        rawHash,
		AnalysisStatus: "pending",
		CreatedAt:      now,
		UpdatedAt:      now,
	}
	if found {
		item.ID = existing.ID
		item.CreatedAt = existing.CreatedAt
	}
	if err := service.repository.SaveDraft(ctx, item); err != nil {
		return domain.JobDescription{}, err
	}
	slog.Info(
		"jd.draft.saved",
		slog.String("jd_id", item.ID),
		slog.String("raw_hash", item.RawHash),
	)
	return item, nil
}

func (service *JDService) saveAnalysisFailure(
	ctx context.Context,
	draft domain.JobDescription,
	analysisErr error,
) {
	message := defaultJDAnalysisErr
	if analysisErr != nil {
		message = analysisErr.Error()
	}
	err := service.repository.UpdateAnalysis(
		ctx,
		ports.JDAnalysisUpdate{
			ID:        draft.ID,
			RawHash:   draft.RawHash,
			Title:     draft.Title,
			Company:   draft.Company,
			Language:  draft.Language,
			Status:    "failed",
			Error:     message,
			UpdatedAt: service.clock.Now().UTC(),
		},
	)
	if err != nil {
		slog.Error(
			"jd.analysis.failure.persist.failed",
			slog.String("jd_id", draft.ID),
			slog.Any("error", err),
		)
	}
}

func jdWorkspace(item domain.JobDescription) (JDWorkspace, error) {
	workspace := JDWorkspace{
		ID:             item.ID,
		Title:          item.Title,
		Company:        item.Company,
		RawText:        item.RawText,
		Language:       item.Language,
		AnalysisStatus: item.AnalysisStatus,
		AnalysisError:  item.AnalysisError,
		UpdatedAt:      item.UpdatedAt.UTC().Format(timeFormat),
		Warnings:       jdWarnings(item.RawText),
	}
	if item.AnalysisJSON == "" {
		return workspace, nil
	}
	analysis, err := domain.DecodeJDAnalysis([]byte(item.AnalysisJSON))
	if err != nil {
		return JDWorkspace{}, fmt.Errorf("decode stored JD analysis: %w", err)
	}
	summary := jdAnalysisSummary(analysis)
	workspace.Analysis = &summary
	return workspace, nil
}

const timeFormat = "2006-01-02T15:04:05Z07:00"

func jdAnalysisSummary(analysis domain.JDAnalysis) JDAnalysisSummary {
	summary := JDAnalysisSummary{
		Role:                 analysis.Role,
		Language:             string(analysis.Language),
		Responsibilities:     requirementSummaries(analysis.Responsibilities),
		RequiredSkills:       requirementSummaries(analysis.RequiredSkills),
		PreferredSkills:      requirementSummaries(analysis.PreferredSkills),
		DomainSignals:        append([]string(nil), analysis.DomainSignals...),
		ScreeningConstraints: append([]string(nil), analysis.ScreeningConstraints...),
		Ambiguities:          append([]string(nil), analysis.Ambiguities...),
	}
	if analysis.Company != nil {
		summary.Company = *analysis.Company
	}
	if analysis.Level != nil {
		summary.Level = *analysis.Level
	}
	return summary
}

func requirementSummaries(
	requirements []domain.Requirement,
) []JDRequirementSummary {
	result := make([]JDRequirementSummary, 0, len(requirements))
	for _, requirement := range requirements {
		result = append(result, JDRequirementSummary{
			ID:             requirement.ID,
			Text:           requirement.Text,
			Importance:     requirement.Importance,
			HardConstraint: requirement.HardConstraint,
		})
	}
	return result
}

func jdWarnings(rawText string) []string {
	warnings := make([]string, 0, 1)
	if utf8.RuneCountInString(rawText) < shortJDWarningRunes {
		warnings = append(
			warnings,
			"JD 内容较短，分析结果可能缺少职责或筛选条件。",
		)
	}
	return warnings
}

func hashJDText(rawText string) string {
	digest := sha256.Sum256([]byte(rawText))
	return hex.EncodeToString(digest[:])
}

func draftTitle(rawText string) string {
	for _, line := range strings.Split(rawText, "\n") {
		line = strings.TrimSpace(line)
		if line == "" {
			continue
		}
		runes := []rune(line)
		if len(runes) > maxDraftTitleRunes {
			return string(runes[:maxDraftTitleRunes]) + "..."
		}
		return line
	}
	return defaultJDTitle
}

func languageHint(rawText string) domain.JDLanguage {
	var hasHan bool
	var hasLatin bool
	for _, value := range rawText {
		switch {
		case unicode.Is(unicode.Han, value):
			hasHan = true
		case unicode.Is(unicode.Latin, value):
			hasLatin = true
		}
	}
	switch {
	case hasHan && hasLatin:
		return domain.JDLanguageMixed
	case hasHan:
		return domain.JDLanguageChinese
	default:
		return domain.JDLanguageEnglish
	}
}
