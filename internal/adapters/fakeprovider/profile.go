package fakeprovider

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"unicode/utf8"

	"github.com/ch1lam/autocv/internal/domain"
	"github.com/ch1lam/autocv/internal/ports"
)

const fakeEvidenceConfidence = 0.75

func (provider *Provider) ExtractProfile(
	ctx context.Context,
	request ports.ExtractProfileRequest,
) ([]domain.ExtractedEvidence, error) {
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	if len(request.Chunks) == 0 {
		return nil, errors.New("profile has no source chunks")
	}

	result := make([]domain.ExtractedEvidence, 0, len(request.Chunks))
	for _, chunk := range request.Chunks {
		if strings.TrimSpace(chunk.Text) == "" {
			continue
		}
		item := domain.ExtractedEvidence{
			Kind:           inferEvidenceKind(chunk),
			Title:          evidenceTitle(chunk.Text),
			Content:        chunk.Text,
			SourceChunkIDs: []string{chunk.ID},
			Confidence:     fakeEvidenceConfidence,
		}
		if err := item.Validate(); err != nil {
			return nil, fmt.Errorf("validate fake evidence: %w", err)
		}
		result = append(result, item)
	}
	if len(result) == 0 {
		return nil, errors.New("profile has no extractable content")
	}
	return result, nil
}

func inferEvidenceKind(chunk domain.SourceChunk) domain.EvidenceKind {
	context := strings.ToLower(chunk.LocatorJSON + " " + chunk.Text)
	switch {
	case containsAny(context, "项目", "project"):
		return domain.EvidenceKindProject
	case containsAny(context, "技能", "skill", "technology", "tech stack"):
		return domain.EvidenceKindSkill
	case containsAny(context, "教育", "education", "university", "大学"):
		return domain.EvidenceKindEducation
	case containsAny(context, "认证", "certification", "certificate"):
		return domain.EvidenceKindCertification
	case containsAny(context, "奖", "achievement", "award"):
		return domain.EvidenceKindAchievement
	default:
		return domain.EvidenceKindExperience
	}
}

func containsAny(value string, candidates ...string) bool {
	for _, candidate := range candidates {
		if strings.Contains(value, candidate) {
			return true
		}
	}
	return false
}

func evidenceTitle(content string) string {
	const maxTitleRunes = 36
	content = strings.Join(strings.Fields(content), " ")
	if utf8.RuneCountInString(content) <= maxTitleRunes {
		return content
	}
	runes := []rune(content)
	return strings.TrimSpace(string(runes[:maxTitleRunes])) + "..."
}

var _ ports.ProfileExtractor = (*Provider)(nil)
