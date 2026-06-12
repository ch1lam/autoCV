package domain

import (
	"errors"
	"fmt"
	"math"
	"strings"
	"time"
)

type MatchStrength string

const (
	MatchStrengthStrong  MatchStrength = "strong"
	MatchStrengthPartial MatchStrength = "partial"
	MatchStrengthMissing MatchStrength = "missing"
	MatchStrengthUnknown MatchStrength = "unknown"
)

type RequirementCategory string

const (
	RequirementCategoryRequired       RequirementCategory = "required"
	RequirementCategoryResponsibility RequirementCategory = "responsibility"
	RequirementCategoryLevel          RequirementCategory = "level"
	RequirementCategoryDomain         RequirementCategory = "domain"
	RequirementCategoryPreferred      RequirementCategory = "preferred"
)

type MatchRequirement struct {
	ID             string
	Category       RequirementCategory
	Text           string
	Importance     int
	HardConstraint bool
	Ordinal        int
}

type MatchSuggestion struct {
	RequirementID       string
	Strength            MatchStrength
	EvidenceIDs         []string
	Explanation         string
	ClarificationNeeded bool
}

type MatchAnalysis struct {
	ID           string
	ProfileID    string
	JDID         string
	InputHash    string
	Status       string
	Error        string
	CreatedAt    time.Time
	UpdatedAt    time.Time
	Requirements []MatchRequirement
	Suggestions  []MatchSuggestion
}

type MatchDimensionScore struct {
	Category         RequirementCategory
	Weight           int
	Earned           float64
	RequirementCount int
}

type MatchScore struct {
	Total          int
	HardCapApplied bool
	Dimensions     []MatchDimensionScore
}

var matchCategoryWeights = []struct {
	category RequirementCategory
	weight   int
}{
	{RequirementCategoryRequired, 40},
	{RequirementCategoryResponsibility, 30},
	{RequirementCategoryLevel, 15},
	{RequirementCategoryDomain, 10},
	{RequirementCategoryPreferred, 5},
}

func ValidateMatchSuggestions(
	requirements []MatchRequirement,
	evidence []Evidence,
	suggestions []MatchSuggestion,
) error {
	if len(requirements) == 0 {
		return errors.New("match requirements are empty")
	}

	requirementIDs := make(map[string]struct{}, len(requirements))
	for _, requirement := range requirements {
		if err := validateMatchRequirement(requirement); err != nil {
			return err
		}
		if _, exists := requirementIDs[requirement.ID]; exists {
			return fmt.Errorf("duplicate match requirement id %q", requirement.ID)
		}
		requirementIDs[requirement.ID] = struct{}{}
	}

	evidenceIDs := make(map[string]struct{}, len(evidence))
	for _, item := range evidence {
		if strings.TrimSpace(item.ID) == "" {
			return errors.New("match evidence id is empty")
		}
		evidenceIDs[item.ID] = struct{}{}
	}

	seenRequirements := make(map[string]struct{}, len(suggestions))
	for _, suggestion := range suggestions {
		if _, exists := requirementIDs[suggestion.RequirementID]; !exists {
			return fmt.Errorf(
				"match suggestion references unknown requirement %q",
				suggestion.RequirementID,
			)
		}
		if _, exists := seenRequirements[suggestion.RequirementID]; exists {
			return fmt.Errorf(
				"duplicate match suggestion for requirement %q",
				suggestion.RequirementID,
			)
		}
		seenRequirements[suggestion.RequirementID] = struct{}{}

		switch suggestion.Strength {
		case MatchStrengthStrong, MatchStrengthPartial:
			if len(suggestion.EvidenceIDs) == 0 {
				return fmt.Errorf(
					"%s match %q has no evidence",
					suggestion.Strength,
					suggestion.RequirementID,
				)
			}
		case MatchStrengthMissing, MatchStrengthUnknown:
			if len(suggestion.EvidenceIDs) != 0 {
				return fmt.Errorf(
					"%s match %q must not contain evidence",
					suggestion.Strength,
					suggestion.RequirementID,
				)
			}
		default:
			return fmt.Errorf(
				"invalid match strength %q",
				suggestion.Strength,
			)
		}
		if strings.TrimSpace(suggestion.Explanation) == "" {
			return fmt.Errorf(
				"match suggestion %q explanation is empty",
				suggestion.RequirementID,
			)
		}

		seenEvidence := make(map[string]struct{}, len(suggestion.EvidenceIDs))
		for _, evidenceID := range suggestion.EvidenceIDs {
			if _, exists := evidenceIDs[evidenceID]; !exists {
				return fmt.Errorf(
					"match suggestion %q references unknown evidence %q",
					suggestion.RequirementID,
					evidenceID,
				)
			}
			if _, exists := seenEvidence[evidenceID]; exists {
				return fmt.Errorf(
					"match suggestion %q repeats evidence %q",
					suggestion.RequirementID,
					evidenceID,
				)
			}
			seenEvidence[evidenceID] = struct{}{}
		}
	}

	if len(seenRequirements) != len(requirements) {
		for _, requirement := range requirements {
			if _, exists := seenRequirements[requirement.ID]; !exists {
				return fmt.Errorf(
					"match suggestion is missing requirement %q",
					requirement.ID,
				)
			}
		}
	}
	return nil
}

func CalculateMatchScore(
	requirements []MatchRequirement,
	suggestions []MatchSuggestion,
) (MatchScore, error) {
	if len(requirements) == 0 {
		return MatchScore{}, errors.New("match requirements are empty")
	}

	suggestionsByID := make(map[string]MatchSuggestion, len(suggestions))
	for _, suggestion := range suggestions {
		suggestionsByID[suggestion.RequirementID] = suggestion
	}

	score := MatchScore{
		Dimensions: make([]MatchDimensionScore, 0, len(matchCategoryWeights)),
	}
	total := 0.0
	for _, weightedCategory := range matchCategoryWeights {
		categoryRequirements := make([]MatchRequirement, 0)
		for _, requirement := range requirements {
			if requirement.Category == weightedCategory.category {
				categoryRequirements = append(
					categoryRequirements,
					requirement,
				)
			}
		}

		dimension := MatchDimensionScore{
			Category:         weightedCategory.category,
			Weight:           weightedCategory.weight,
			RequirementCount: len(categoryRequirements),
		}
		if len(categoryRequirements) == 0 {
			dimension.Earned = float64(weightedCategory.weight)
			total += dimension.Earned
			score.Dimensions = append(score.Dimensions, dimension)
			continue
		}

		var earnedImportance float64
		var totalImportance int
		for _, requirement := range categoryRequirements {
			suggestion, exists := suggestionsByID[requirement.ID]
			if !exists {
				return MatchScore{}, fmt.Errorf(
					"match score is missing requirement %q",
					requirement.ID,
				)
			}
			totalImportance += requirement.Importance
			earnedImportance += float64(requirement.Importance) *
				matchStrengthFactor(suggestion.Strength)
			if requirement.HardConstraint &&
				suggestion.Strength == MatchStrengthMissing {
				score.HardCapApplied = true
			}
		}
		if totalImportance == 0 {
			return MatchScore{}, fmt.Errorf(
				"match category %q has zero importance",
				weightedCategory.category,
			)
		}
		dimension.Earned = float64(weightedCategory.weight) *
			earnedImportance / float64(totalImportance)
		total += dimension.Earned
		score.Dimensions = append(score.Dimensions, dimension)
	}

	score.Total = int(math.Round(total))
	if score.HardCapApplied && score.Total > 69 {
		score.Total = 69
	}
	return score, nil
}

func validateMatchRequirement(requirement MatchRequirement) error {
	if strings.TrimSpace(requirement.ID) == "" {
		return errors.New("match requirement id is empty")
	}
	switch requirement.Category {
	case RequirementCategoryRequired,
		RequirementCategoryResponsibility,
		RequirementCategoryLevel,
		RequirementCategoryDomain,
		RequirementCategoryPreferred:
	default:
		return fmt.Errorf(
			"invalid match requirement category %q",
			requirement.Category,
		)
	}
	if strings.TrimSpace(requirement.Text) == "" {
		return fmt.Errorf(
			"match requirement %q text is empty",
			requirement.ID,
		)
	}
	if requirement.Importance < 1 || requirement.Importance > 5 {
		return fmt.Errorf(
			"match requirement %q importance %d is outside 1..5",
			requirement.ID,
			requirement.Importance,
		)
	}
	if requirement.Ordinal < 0 {
		return fmt.Errorf(
			"match requirement %q ordinal is negative",
			requirement.ID,
		)
	}
	return nil
}

func matchStrengthFactor(strength MatchStrength) float64 {
	switch strength {
	case MatchStrengthStrong:
		return 1
	case MatchStrengthPartial:
		return 0.5
	case MatchStrengthMissing, MatchStrengthUnknown:
		return 0
	default:
		return 0
	}
}
