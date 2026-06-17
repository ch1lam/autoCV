package fakeprovider

import (
	"context"
	"fmt"
	"strings"
	"testing"
	"time"

	"github.com/ch1lam/autocv/internal/domain"
	"github.com/ch1lam/autocv/internal/ports"
)

func TestDraftResumeUsesOnlyMatchedEvidenceAndPreservesNumbers(t *testing.T) {
	evidence := []domain.Evidence{
		{
			ID:      "evidence-1",
			Kind:    string(domain.EvidenceKindExperience),
			Title:   "Go 服务",
			Content: "负责 Go 服务开发，将接口延迟降低 35%。",
		},
		{
			ID:      "evidence-2",
			Kind:    string(domain.EvidenceKindProject),
			Title:   "无关项目",
			Content: "维护内部站点。",
		},
	}
	match := domain.MatchAnalysis{
		Requirements: []domain.MatchRequirement{{
			ID:         "required-go",
			Text:       "Go 服务开发",
			Importance: 5,
		}},
		Suggestions: []domain.MatchSuggestion{{
			RequirementID: "required-go",
			Strength:      domain.MatchStrengthStrong,
			EvidenceIDs:   []string{"evidence-1"},
			Explanation:   "有直接证据。",
		}},
	}

	draft, err := New().DraftResume(
		context.Background(),
		ports.DraftResumeRequest{
			Language:       domain.ResumeLanguageChinese,
			TargetRole:     "后端工程师",
			PackagingLevel: 0.5,
			Match:          match,
			Evidence:       evidence,
		},
	)
	if err != nil {
		t.Fatalf("draft resume: %v", err)
	}
	if len(draft.Blocks) != 2 {
		t.Fatalf("expected summary and one grounded block, got %d", len(draft.Blocks))
	}
	if draft.Blocks[1].Content != evidence[0].Content {
		t.Fatalf("expected exact source content, got %q", draft.Blocks[1].Content)
	}
	if draft.Blocks[1].SourceEvidenceIDs[0] != evidence[0].ID {
		t.Fatalf("expected matched evidence source")
	}
}

func TestDraftResumeUsesUserConfirmedClarifications(t *testing.T) {
	now := time.Date(2026, 6, 17, 10, 0, 0, 0, time.UTC)
	draft, err := New().DraftResume(
		context.Background(),
		ports.DraftResumeRequest{
			Language:       domain.ResumeLanguageChinese,
			TargetRole:     "后端工程师",
			PackagingLevel: 0.5,
			Match: domain.MatchAnalysis{
				Requirements: []domain.MatchRequirement{{
					ID:         "required-team",
					Text:       "团队协作",
					Importance: 4,
				}},
				Suggestions: []domain.MatchSuggestion{{
					RequirementID:       "required-team",
					Strength:            domain.MatchStrengthMissing,
					Explanation:         "资料中缺少团队规模。",
					ClarificationNeeded: true,
				}},
			},
			Confirmations: []domain.RunConfirmation{{
				ID:                      "confirmation-1",
				RunID:                   "run-1",
				ClarificationQuestionID: "question-1",
				RequirementID:           "required-team",
				Content:                 "负责 8 人后端团队和跨部门交付。",
				CreatedAt:               now,
				UpdatedAt:               now,
			}},
		},
	)
	if err != nil {
		t.Fatalf("draft resume from confirmations: %v", err)
	}
	if len(draft.Blocks) != 2 {
		t.Fatalf("expected summary and confirmation block, got %#v", draft.Blocks)
	}
	confirmed := draft.Blocks[1]
	if confirmed.GroundingLevel != domain.GroundingUserConfirmed ||
		confirmed.Content != "负责 8 人后端团队和跨部门交付。" ||
		len(confirmed.SourceEvidenceIDs) != 0 {
		t.Fatalf("unexpected confirmed block %#v", confirmed)
	}
}

func TestDraftResumeUsesFixedPackagingSamples(t *testing.T) {
	evidence, match := fixedPackagingSample()
	tests := []struct {
		level     float64
		wantID    string
		wantCount int
	}{
		{level: 0, wantID: "conservative", wantCount: 4},
		{level: 0.5, wantID: "balanced", wantCount: 6},
		{level: 1, wantID: "amplified", wantCount: 8},
	}
	for _, test := range tests {
		t.Run(test.wantID, func(t *testing.T) {
			strategy, found := domain.ResumePackagingStrategyForLevel(
				test.level,
			)
			if !found {
				t.Fatalf("expected strategy for %.1f", test.level)
			}
			draft, err := New().DraftResume(
				context.Background(),
				ports.DraftResumeRequest{
					Language:          domain.ResumeLanguageChinese,
					TargetRole:        "后端工程师",
					PackagingLevel:    test.level,
					PackagingStrategy: strategy,
					Match:             match,
					Evidence:          evidence,
				},
			)
			if err != nil {
				t.Fatalf("draft resume: %v", err)
			}
			if got := len(draft.Blocks) - 1; got != test.wantCount {
				t.Fatalf(
					"expected %d grounded blocks, got %d",
					test.wantCount,
					got,
				)
			}
			notes := strings.Join(draft.OptimizationNotes, "\n")
			if !strings.Contains(notes, "包装档位："+strategy.Label) {
				t.Fatalf("expected strategy note in %#v", draft.OptimizationNotes)
			}
			for index := 0; index < test.wantCount; index++ {
				block := draft.Blocks[index+1]
				if len(block.SourceEvidenceIDs) != 1 ||
					block.SourceEvidenceIDs[0] != evidence[index].ID ||
					block.Content != evidence[index].Content {
					t.Fatalf(
						"unexpected grounded block at %d: %#v",
						index,
						block,
					)
				}
			}
			allContent := strings.Join(resumeBlockContents(draft.Blocks), "\n")
			if strings.Contains(allContent, "Kubernetes") ||
				strings.Contains(allContent, "99%") {
				t.Fatalf("draft invented unsupported content: %s", allContent)
			}
		})
	}
}

func TestFakeResumeSummaryTrimsSourcePunctuation(t *testing.T) {
	summary := fakeResumeSummary(
		domain.ResumeLanguageChinese,
		[]string{"后端服务。", "稳定性治理。"},
	)
	if summary != "围绕目标岗位，重点呈现后端服务、稳定性治理等相关经历。" {
		t.Fatalf("unexpected summary %q", summary)
	}
}

func fixedPackagingSample() ([]domain.Evidence, domain.MatchAnalysis) {
	evidence := make([]domain.Evidence, 0, 8)
	requirements := make([]domain.MatchRequirement, 0, 8)
	suggestions := make([]domain.MatchSuggestion, 0, 8)
	for index := 0; index < 8; index++ {
		ordinal := index + 1
		evidenceID := fmt.Sprintf("evidence-%02d", ordinal)
		requirementID := fmt.Sprintf("requirement-%02d", ordinal)
		evidence = append(evidence, domain.Evidence{
			ID:      evidenceID,
			Kind:    string(domain.EvidenceKindExperience),
			Title:   fmt.Sprintf("服务治理 %02d", ordinal),
			Content: fmt.Sprintf("负责服务治理 %02d 的接口交付和故障复盘。", ordinal),
		})
		requirements = append(requirements, domain.MatchRequirement{
			ID:         requirementID,
			Text:       fmt.Sprintf("后端服务能力 %02d", ordinal),
			Importance: 5,
		})
		suggestions = append(suggestions, domain.MatchSuggestion{
			RequirementID: requirementID,
			Strength:      domain.MatchStrengthStrong,
			EvidenceIDs:   []string{evidenceID},
			Explanation:   "固定样本直接匹配。",
		})
	}
	return evidence, domain.MatchAnalysis{
		Requirements: requirements,
		Suggestions:  suggestions,
	}
}

func resumeBlockContents(blocks []domain.ResumeBlockDraft) []string {
	contents := make([]string, 0, len(blocks))
	for _, block := range blocks {
		contents = append(contents, block.Content)
	}
	return contents
}
