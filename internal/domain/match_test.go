package domain

import (
	"strings"
	"testing"
)

func TestCalculateMatchScoreUsesDeterministicWeights(t *testing.T) {
	requirements := []MatchRequirement{
		matchRequirement("required-1", RequirementCategoryRequired, 5, false),
		matchRequirement("responsibility-1", RequirementCategoryResponsibility, 5, false),
		matchRequirement("level-1", RequirementCategoryLevel, 5, false),
		matchRequirement("domain-1", RequirementCategoryDomain, 5, false),
		matchRequirement("preferred-1", RequirementCategoryPreferred, 5, false),
	}
	suggestions := []MatchSuggestion{
		matchSuggestion("required-1", MatchStrengthStrong),
		matchSuggestion("responsibility-1", MatchStrengthPartial),
		matchSuggestion("level-1", MatchStrengthUnknown),
		matchSuggestion("domain-1", MatchStrengthStrong),
		matchSuggestion("preferred-1", MatchStrengthMissing),
	}

	score, err := CalculateMatchScore(requirements, suggestions)
	if err != nil {
		t.Fatalf("calculate match score: %v", err)
	}
	if score.Total != 65 {
		t.Fatalf("expected score 65, got %d", score.Total)
	}
	if score.HardCapApplied {
		t.Fatal("did not expect hard cap")
	}
}

func TestCalculateMatchScoreCapsMissingHardConstraint(t *testing.T) {
	requirements := []MatchRequirement{
		matchRequirement("required-1", RequirementCategoryRequired, 1, true),
		matchRequirement("required-2", RequirementCategoryRequired, 4, false),
	}
	suggestions := []MatchSuggestion{
		matchSuggestion("required-1", MatchStrengthMissing),
		matchSuggestion("required-2", MatchStrengthStrong),
	}

	score, err := CalculateMatchScore(requirements, suggestions)
	if err != nil {
		t.Fatalf("calculate match score: %v", err)
	}
	if score.Total != 69 {
		t.Fatalf("expected hard-capped score 69, got %d", score.Total)
	}
	if !score.HardCapApplied {
		t.Fatal("expected hard cap to be applied")
	}
}

func TestValidateMatchSuggestionsRejectsProviderScoreAndUnknownEvidence(t *testing.T) {
	requirements := []MatchRequirement{
		matchRequirement("required-1", RequirementCategoryRequired, 5, false),
	}
	evidence := []Evidence{{ID: "evidence-1"}}

	err := ValidateMatchSuggestions(
		requirements,
		evidence,
		[]MatchSuggestion{{
			RequirementID: "required-1",
			Strength:      MatchStrengthStrong,
			EvidenceIDs:   []string{"missing-evidence"},
			Explanation:   "provider suggestion",
		}},
	)
	if err == nil || !strings.Contains(err.Error(), "unknown evidence") {
		t.Fatalf("expected unknown evidence error, got %v", err)
	}
}

func matchRequirement(
	id string,
	category RequirementCategory,
	importance int,
	hardConstraint bool,
) MatchRequirement {
	return MatchRequirement{
		ID:             id,
		Category:       category,
		Text:           id,
		Importance:     importance,
		HardConstraint: hardConstraint,
	}
}

func matchSuggestion(
	requirementID string,
	strength MatchStrength,
) MatchSuggestion {
	suggestion := MatchSuggestion{
		RequirementID: requirementID,
		Strength:      strength,
		Explanation:   "fixture explanation",
	}
	if strength == MatchStrengthStrong || strength == MatchStrengthPartial {
		suggestion.EvidenceIDs = []string{"evidence-" + requirementID}
	}
	return suggestion
}
