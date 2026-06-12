package fakeprovider

import (
	"context"
	"testing"

	"github.com/ch1lam/autocv/internal/domain"
	"github.com/ch1lam/autocv/internal/ports"
)

func TestSuggestMatchesReturnsTraceableSuggestions(t *testing.T) {
	provider := New()
	evidence := []domain.Evidence{
		{
			ID:      "evidence-go",
			Title:   "Go 服务",
			Content: "使用 Go、goroutine 和 channel 构建后端服务。",
		},
		{
			ID:      "evidence-sql",
			Title:   "PostgreSQL",
			Content: "使用 PostgreSQL 和索引优化订单查询。",
		},
	}
	requirements := []domain.MatchRequirement{
		{
			ID:         "required-go",
			Category:   domain.RequirementCategoryRequired,
			Text:       "熟练使用 Go 开发生产服务",
			Importance: 5,
		},
		{
			ID:             "screening-english",
			Category:       domain.RequirementCategoryRequired,
			Text:           "能够阅读英文技术文档",
			Importance:     5,
			HardConstraint: true,
			Ordinal:        1,
		},
		{
			ID:         "level",
			Category:   domain.RequirementCategoryLevel,
			Text:       "Senior",
			Importance: 3,
			Ordinal:    2,
		},
	}

	suggestions, err := provider.SuggestMatches(
		context.Background(),
		ports.SuggestMatchesRequest{
			Requirements: requirements,
			Evidence:     evidence,
		},
	)
	if err != nil {
		t.Fatalf("suggest matches: %v", err)
	}
	if len(suggestions) != 3 {
		t.Fatalf("expected 3 suggestions, got %d", len(suggestions))
	}
	if suggestions[0].Strength != domain.MatchStrengthStrong ||
		len(suggestions[0].EvidenceIDs) == 0 {
		t.Fatalf("unexpected Go suggestion %#v", suggestions[0])
	}
	if suggestions[1].Strength != domain.MatchStrengthMissing {
		t.Fatalf("unexpected English suggestion %#v", suggestions[1])
	}
	if suggestions[2].Strength != domain.MatchStrengthMissing {
		t.Fatalf("unexpected level suggestion %#v", suggestions[2])
	}
}
