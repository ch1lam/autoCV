package fakeprovider

import (
	"context"
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

func TestFakeResumeSummaryTrimsSourcePunctuation(t *testing.T) {
	summary := fakeResumeSummary(
		domain.ResumeLanguageChinese,
		[]string{"后端服务。", "稳定性治理。"},
	)
	if summary != "围绕目标岗位，重点呈现后端服务、稳定性治理等相关经历。" {
		t.Fatalf("unexpected summary %q", summary)
	}
}
