package openaiprovider

import (
	"context"
	"errors"
	"strings"
	"testing"

	"github.com/ch1lam/autocv/internal/domain"
)

func TestDecodeProfileEvidenceRejectsUnknownChunk(t *testing.T) {
	contents := []byte(`{
		"evidence": [{
			"kind": "experience",
			"title": "Built backend services",
			"content": "Built backend services in Go.",
			"source_chunk_ids": ["other-chunk"],
			"confidence": 0.8
		}]
	}`)
	if _, err := DecodeProfileEvidence(
		contents,
		[]domain.SourceChunk{{ID: "chunk-1", Text: "Built backend services in Go."}},
	); err == nil {
		t.Fatal("expected unknown chunk error")
	}
}

func TestDecodeMatchSuggestionsRejectsProviderScore(t *testing.T) {
	contents := []byte(`{
		"suggestions": [{
			"requirement_id": "required-go",
			"strength": "strong",
			"evidence_ids": ["evidence-go"],
			"explanation": "Direct Go evidence.",
			"clarification_needed": false,
			"score": 100
		}]
	}`)
	_, err := DecodeMatchSuggestions(
		contents,
		[]domain.MatchRequirement{{
			ID:         "required-go",
			Category:   domain.RequirementCategoryRequired,
			Text:       "Go",
			Importance: 5,
		}},
		[]domain.Evidence{{
			ID:      "evidence-go",
			Kind:    string(domain.EvidenceKindExperience),
			Title:   "Go service",
			Content: "Built Go services.",
		}},
	)
	if err == nil || !strings.Contains(err.Error(), "unknown field") {
		t.Fatalf("expected unknown score field error, got %v", err)
	}
}

func TestValidateWithSingleRepairRepairsOnce(t *testing.T) {
	calls := 0
	decode := func(contents []byte) (string, error) {
		if string(contents) != "valid" {
			return "", errors.New("invalid output")
		}
		return "accepted", nil
	}
	result, repaired, err := ValidateWithSingleRepair(
		context.Background(),
		[]byte("invalid"),
		decode,
		func(
			_ context.Context,
			_ []byte,
			_ error,
		) ([]byte, error) {
			calls++
			return []byte("valid"), nil
		},
	)
	if err != nil {
		t.Fatalf("repair output: %v", err)
	}
	if result != "accepted" || !repaired || calls != 1 {
		t.Fatalf(
			"unexpected repair result result=%q repaired=%v calls=%d",
			result,
			repaired,
			calls,
		)
	}
}

func TestValidateWithSingleRepairRejectsSecondInvalidOutput(t *testing.T) {
	calls := 0
	_, repaired, err := ValidateWithSingleRepair(
		context.Background(),
		[]byte("invalid"),
		func([]byte) (string, error) {
			return "", errors.New("invalid output")
		},
		func(
			_ context.Context,
			_ []byte,
			_ error,
		) ([]byte, error) {
			calls++
			return []byte("still invalid"), nil
		},
	)
	if err == nil || !repaired || calls != 1 {
		t.Fatalf(
			"expected one failed repair, repaired=%v calls=%d err=%v",
			repaired,
			calls,
			err,
		)
	}
}
