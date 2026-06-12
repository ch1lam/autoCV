package openaiprovider

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"

	"github.com/ch1lam/autocv/internal/domain"
)

type profileExtractionResponse struct {
	Evidence []profileEvidence `json:"evidence"`
}

type profileEvidence struct {
	Kind           domain.EvidenceKind `json:"kind"`
	Title          string              `json:"title"`
	Content        string              `json:"content"`
	SourceChunkIDs []string            `json:"source_chunk_ids"`
	Confidence     float64             `json:"confidence"`
}

type matchSuggestionResponse struct {
	Suggestions []matchSuggestion `json:"suggestions"`
}

type matchSuggestion struct {
	RequirementID       string               `json:"requirement_id"`
	Strength            domain.MatchStrength `json:"strength"`
	EvidenceIDs         []string             `json:"evidence_ids"`
	Explanation         string               `json:"explanation"`
	ClarificationNeeded bool                 `json:"clarification_needed"`
}

type RepairFunc func(
	context.Context,
	[]byte,
	error,
) ([]byte, error)

func DecodeProfileEvidence(
	contents []byte,
	chunks []domain.SourceChunk,
) ([]domain.ExtractedEvidence, error) {
	var response profileExtractionResponse
	if err := decodeStrict(contents, &response); err != nil {
		return nil, fmt.Errorf("decode profile extraction: %w", err)
	}
	if response.Evidence == nil {
		return nil, errors.New("profile extraction evidence is missing")
	}

	validChunkIDs := make(map[string]struct{}, len(chunks))
	for _, chunk := range chunks {
		validChunkIDs[chunk.ID] = struct{}{}
	}
	evidence := make([]domain.ExtractedEvidence, 0, len(response.Evidence))
	for index, item := range response.Evidence {
		converted := domain.ExtractedEvidence{
			Kind:           item.Kind,
			Title:          item.Title,
			Content:        item.Content,
			SourceChunkIDs: item.SourceChunkIDs,
			Confidence:     item.Confidence,
		}
		if err := converted.Validate(); err != nil {
			return nil, fmt.Errorf("profile evidence[%d]: %w", index, err)
		}
		for _, chunkID := range converted.SourceChunkIDs {
			if _, exists := validChunkIDs[chunkID]; !exists {
				return nil, fmt.Errorf(
					"profile evidence[%d] references unknown chunk %q",
					index,
					chunkID,
				)
			}
		}
		evidence = append(evidence, converted)
	}
	return evidence, nil
}

func DecodeJDAnalysis(contents []byte) (domain.JDAnalysis, error) {
	return domain.DecodeJDAnalysis(contents)
}

func DecodeMatchSuggestions(
	contents []byte,
	requirements []domain.MatchRequirement,
	evidence []domain.Evidence,
) ([]domain.MatchSuggestion, error) {
	var response matchSuggestionResponse
	if err := decodeStrict(contents, &response); err != nil {
		return nil, fmt.Errorf("decode match suggestions: %w", err)
	}
	if response.Suggestions == nil {
		return nil, errors.New("match suggestions are missing")
	}
	suggestions := make(
		[]domain.MatchSuggestion,
		0,
		len(response.Suggestions),
	)
	for _, item := range response.Suggestions {
		suggestions = append(suggestions, domain.MatchSuggestion{
			RequirementID:       item.RequirementID,
			Strength:            item.Strength,
			EvidenceIDs:         item.EvidenceIDs,
			Explanation:         item.Explanation,
			ClarificationNeeded: item.ClarificationNeeded,
		})
	}
	if err := domain.ValidateMatchSuggestions(
		requirements,
		evidence,
		suggestions,
	); err != nil {
		return nil, err
	}
	return suggestions, nil
}

func DecodeResumeDraft(
	contents []byte,
	evidence []domain.Evidence,
) (domain.ResumeDraft, error) {
	draft, err := domain.DecodeResumeDraft(contents)
	if err != nil {
		return domain.ResumeDraft{}, err
	}
	if err := domain.ValidateResumeDraft(draft, evidence); err != nil {
		return domain.ResumeDraft{}, err
	}
	return draft, nil
}

func ValidateWithSingleRepair[T any](
	ctx context.Context,
	initial []byte,
	decode func([]byte) (T, error),
	repair RepairFunc,
) (T, bool, error) {
	result, err := decode(initial)
	if err == nil {
		return result, false, nil
	}
	if repair == nil {
		var zero T
		return zero, false, err
	}
	repaired, repairErr := repair(ctx, initial, err)
	if repairErr != nil {
		var zero T
		return zero, true, fmt.Errorf(
			"repair structured output: %w",
			repairErr,
		)
	}
	result, err = decode(repaired)
	if err != nil {
		var zero T
		return zero, true, fmt.Errorf(
			"validate repaired structured output: %w",
			err,
		)
	}
	return result, true, nil
}

func decodeStrict(contents []byte, target any) error {
	decoder := json.NewDecoder(bytes.NewReader(contents))
	decoder.DisallowUnknownFields()
	if err := decoder.Decode(target); err != nil {
		return err
	}
	if err := decoder.Decode(&struct{}{}); !errors.Is(err, io.EOF) {
		return errors.New("trailing content")
	}
	return nil
}
