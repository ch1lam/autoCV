package fakeprovider

import (
	"context"
	"errors"
	"fmt"
	"sort"
	"strings"

	"github.com/ch1lam/autocv/internal/domain"
	"github.com/ch1lam/autocv/internal/ports"
)

var semanticKeywordGroups = [][]string{
	{"go", "golang", "goroutine", "channel"},
	{"postgresql", "postgres", "sqlite", "sql", "关系数据库", "数据库", "索引"},
	{"distributed", "分布式", "reliability", "可靠", "稳定", "故障", "超时", "指标", "日志"},
	{"message queue", "消息队列", "事件驱动", "kafka"},
	{"service", "services", "backend", "后端", "服务"},
	{"performance", "性能", "优化", "延迟", "瓶颈"},
	{"high-concurrency", "high concurrency", "高并发", "transaction", "交易"},
	{"english", "英文"},
	{"senior", "staff", "lead", "高级", "资深", "负责"},
}

func (provider *Provider) SuggestMatches(
	ctx context.Context,
	request ports.SuggestMatchesRequest,
) ([]domain.MatchSuggestion, error) {
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	if len(request.Requirements) == 0 {
		return nil, errors.New("match requirements are empty")
	}
	if len(request.Evidence) == 0 {
		return nil, errors.New("match evidence is empty")
	}

	suggestions := make([]domain.MatchSuggestion, 0, len(request.Requirements))
	for _, requirement := range request.Requirements {
		suggestion := fakeMatchSuggestion(requirement, request.Evidence)
		suggestions = append(suggestions, suggestion)
	}
	if err := domain.ValidateMatchSuggestions(
		request.Requirements,
		request.Evidence,
		suggestions,
	); err != nil {
		return nil, fmt.Errorf("validate fake match suggestions: %w", err)
	}
	return suggestions, nil
}

type evidenceMatchScore struct {
	id    string
	title string
	score int
}

func fakeMatchSuggestion(
	requirement domain.MatchRequirement,
	evidence []domain.Evidence,
) domain.MatchSuggestion {
	requirementText := strings.ToLower(requirement.Text)
	activeGroups := make([][]string, 0)
	for _, group := range semanticKeywordGroups {
		if containsAny(requirementText, group...) {
			activeGroups = append(activeGroups, group)
		}
	}

	candidates := make([]evidenceMatchScore, 0)
	for _, item := range evidence {
		content := strings.ToLower(item.Title + "\n" + item.Content)
		score := 0
		for _, group := range activeGroups {
			for _, keyword := range group {
				if strings.Contains(content, keyword) {
					score++
				}
			}
		}
		if score > 0 {
			candidates = append(candidates, evidenceMatchScore{
				id:    item.ID,
				title: item.Title,
				score: score,
			})
		}
	}
	sort.SliceStable(candidates, func(left, right int) bool {
		return candidates[left].score > candidates[right].score
	})
	if len(candidates) > 3 {
		candidates = candidates[:3]
	}

	suggestion := domain.MatchSuggestion{
		RequirementID: requirement.ID,
		EvidenceIDs:   make([]string, 0, len(candidates)),
	}
	for _, candidate := range candidates {
		suggestion.EvidenceIDs = append(
			suggestion.EvidenceIDs,
			candidate.id,
		)
	}

	switch {
	case len(candidates) == 0 && len(activeGroups) == 0:
		suggestion.Strength = domain.MatchStrengthUnknown
		suggestion.ClarificationNeeded = true
		suggestion.Explanation = "当前 Fake Provider 无法从资料关键词判断该要求，需要用户确认。"
	case len(candidates) == 0:
		suggestion.Strength = domain.MatchStrengthMissing
		suggestion.ClarificationNeeded = true
		suggestion.Explanation = "当前资料中没有找到可定位到来源的直接证据。"
	case candidates[0].score >= 2 || len(candidates) >= 2:
		suggestion.Strength = domain.MatchStrengthStrong
		suggestion.Explanation = fmt.Sprintf(
			"资料中的“%s”等证据与该要求存在多处直接关联。",
			candidates[0].title,
		)
	default:
		suggestion.Strength = domain.MatchStrengthPartial
		suggestion.ClarificationNeeded = true
		suggestion.Explanation = fmt.Sprintf(
			"资料中的“%s”提供了相关线索，但覆盖深度仍需确认。",
			candidates[0].title,
		)
	}
	return suggestion
}

var _ ports.MatchSuggester = (*Provider)(nil)
