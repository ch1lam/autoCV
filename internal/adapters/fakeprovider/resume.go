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

type rankedResumeEvidence struct {
	evidence domain.Evidence
	score    int
	ordinal  int
	reasons  []string
}

func (provider *Provider) DraftResume(
	ctx context.Context,
	request ports.DraftResumeRequest,
) (domain.ResumeDraft, error) {
	if err := ctx.Err(); err != nil {
		return domain.ResumeDraft{}, err
	}
	if strings.TrimSpace(request.TargetRole) == "" {
		return domain.ResumeDraft{}, errors.New("resume target role is empty")
	}
	if request.PackagingLevel < 0 || request.PackagingLevel > 1 {
		return domain.ResumeDraft{}, fmt.Errorf(
			"resume packaging level %.2f is outside 0..1",
			request.PackagingLevel,
		)
	}

	ranked := rankResumeEvidence(request.Match, request.Evidence)
	if len(ranked) == 0 {
		return domain.ResumeDraft{}, errors.New(
			"match analysis has no grounded evidence for resume drafting",
		)
	}
	limit := 4 + int(request.PackagingLevel*4)
	if limit > len(ranked) {
		limit = len(ranked)
	}
	ranked = ranked[:limit]

	sourceIDs := make([]string, 0, min(2, len(ranked)))
	titles := make([]string, 0, min(2, len(ranked)))
	for index := 0; index < len(ranked) && index < 2; index++ {
		sourceIDs = append(sourceIDs, ranked[index].evidence.ID)
		titles = append(titles, ranked[index].evidence.Title)
	}

	draft := domain.ResumeDraft{
		Language:   request.Language,
		TargetRole: strings.TrimSpace(request.TargetRole),
		Blocks: []domain.ResumeBlockDraft{{
			Kind:              domain.ResumeBlockSummary,
			Content:           fakeResumeSummary(request.Language, titles),
			SourceEvidenceIDs: sourceIDs,
			GroundingLevel:    domain.GroundingDerived,
			Optimization:      "用最高相关度的来源概括候选人与目标岗位的连接。",
		}},
		OptimizationNotes: []string{
			fmt.Sprintf(
				"按当前匹配结果选择 %d 条来源证据，并按岗位相关度排序。",
				len(ranked),
			),
			"缺失和未知要求未写入简历，具体数字仅保留来源中已有的内容。",
		},
	}
	for _, item := range ranked {
		draft.Blocks = append(draft.Blocks, domain.ResumeBlockDraft{
			Kind:              resumeBlockKindForEvidence(item.evidence.Kind),
			Content:           strings.TrimSpace(item.evidence.Content),
			SourceEvidenceIDs: []string{item.evidence.ID},
			GroundingLevel:    domain.GroundingSource,
			Optimization: fmt.Sprintf(
				"对应 %s，保留来源原意并提高展示顺序。",
				strings.Join(item.reasons, "、"),
			),
		})
	}
	if err := domain.ValidateResumeDraft(draft, request.Evidence); err != nil {
		return domain.ResumeDraft{}, fmt.Errorf(
			"validate fake resume draft: %w",
			err,
		)
	}
	return draft, nil
}

func rankResumeEvidence(
	match domain.MatchAnalysis,
	evidence []domain.Evidence,
) []rankedResumeEvidence {
	evidenceByID := make(map[string]domain.Evidence, len(evidence))
	for _, item := range evidence {
		evidenceByID[item.ID] = item
	}
	requirementByID := make(
		map[string]domain.MatchRequirement,
		len(match.Requirements),
	)
	for _, requirement := range match.Requirements {
		requirementByID[requirement.ID] = requirement
	}

	rankedByID := make(map[string]rankedResumeEvidence)
	nextOrdinal := 0
	for _, suggestion := range match.Suggestions {
		if suggestion.Strength != domain.MatchStrengthStrong &&
			suggestion.Strength != domain.MatchStrengthPartial {
			continue
		}
		requirement := requirementByID[suggestion.RequirementID]
		strengthScore := 50
		if suggestion.Strength == domain.MatchStrengthStrong {
			strengthScore = 100
		}
		for _, evidenceID := range suggestion.EvidenceIDs {
			item, exists := evidenceByID[evidenceID]
			if !exists {
				continue
			}
			ranked, exists := rankedByID[evidenceID]
			if !exists {
				ranked = rankedResumeEvidence{
					evidence: item,
					ordinal:  nextOrdinal,
					reasons:  make([]string, 0),
				}
				nextOrdinal++
			}
			ranked.score += strengthScore + requirement.Importance*5
			ranked.reasons = appendUnique(
				ranked.reasons,
				strings.TrimSpace(requirement.Text),
			)
			rankedByID[evidenceID] = ranked
		}
	}

	ranked := make([]rankedResumeEvidence, 0, len(rankedByID))
	for _, item := range rankedByID {
		ranked = append(ranked, item)
	}
	sort.SliceStable(ranked, func(left int, right int) bool {
		if ranked[left].score == ranked[right].score {
			return ranked[left].ordinal < ranked[right].ordinal
		}
		return ranked[left].score > ranked[right].score
	})
	return ranked
}

func fakeResumeSummary(
	language domain.ResumeLanguage,
	titles []string,
) string {
	normalized := make([]string, 0, len(titles))
	for _, title := range titles {
		normalized = append(
			normalized,
			strings.TrimRight(
				strings.TrimSpace(title),
				"。.!！?？;；",
			),
		)
	}
	joined := strings.Join(normalized, "、")
	if language == domain.ResumeLanguageEnglish {
		return fmt.Sprintf(
			"Relevant evidence for the target role includes %s.",
			joined,
		)
	}
	return fmt.Sprintf(
		"围绕目标岗位，重点呈现%s等相关经历。",
		joined,
	)
}

func resumeBlockKindForEvidence(kind string) domain.ResumeBlockKind {
	switch domain.EvidenceKind(kind) {
	case domain.EvidenceKindProject:
		return domain.ResumeBlockProject
	case domain.EvidenceKindSkill:
		return domain.ResumeBlockSkill
	case domain.EvidenceKindEducation:
		return domain.ResumeBlockEducation
	case domain.EvidenceKindCertification:
		return domain.ResumeBlockCertification
	default:
		return domain.ResumeBlockExperience
	}
}

func appendUnique(values []string, value string) []string {
	if value == "" {
		return values
	}
	for _, existing := range values {
		if existing == value {
			return values
		}
	}
	return append(values, value)
}

var _ ports.ResumeDrafter = (*Provider)(nil)
