package app

import (
	"context"
	"errors"
	"fmt"
	"slices"
	"strings"
	"time"

	"github.com/ch1lam/autocv/internal/domain"
	"github.com/ch1lam/autocv/internal/ports"
)

func resolveRunScope(
	ctx context.Context,
	scopeRepository ports.RunScopeRepository,
	profileRepository ports.ProfileRepository,
	profileID string,
	jdID string,
	now time.Time,
) (domain.ResumeRunScope, []domain.SourceDocument, error) {
	documents, err := profileRepository.ListDocuments(ctx, profileID)
	if err != nil {
		return domain.ResumeRunScope{}, nil, err
	}
	scope, found, err := scopeRepository.GetScope(ctx, profileID, jdID)
	if err != nil {
		return domain.ResumeRunScope{}, nil, err
	}
	if !found {
		scope = domain.ResumeRunScope{
			ProfileID:   profileID,
			JDID:        jdID,
			Mode:        domain.RunScopeAll,
			DocumentIDs: make([]string, 0),
			UpdatedAt:   now.UTC(),
		}
	}
	return scope, documents, nil
}

func normalizeRunScope(
	profileID string,
	jdID string,
	mode string,
	documentIDs []string,
	documents []domain.SourceDocument,
	now time.Time,
) (domain.ResumeRunScope, error) {
	scopeMode := domain.RunScopeMode(strings.TrimSpace(mode))
	switch scopeMode {
	case domain.RunScopeAll:
		documentIDs = make([]string, 0)
	case domain.RunScopeSelected:
		if len(documentIDs) == 0 {
			return domain.ResumeRunScope{}, errors.New(
				"selected run scope requires at least one document",
			)
		}
	default:
		return domain.ResumeRunScope{}, fmt.Errorf(
			"invalid run scope mode %q",
			mode,
		)
	}

	available := make(map[string]struct{}, len(documents))
	for _, document := range documents {
		available[document.ID] = struct{}{}
	}
	normalized := make([]string, 0, len(documentIDs))
	seen := make(map[string]struct{}, len(documentIDs))
	for _, documentID := range documentIDs {
		documentID = strings.TrimSpace(documentID)
		if documentID == "" {
			return domain.ResumeRunScope{}, errors.New(
				"run scope document id is empty",
			)
		}
		if _, exists := available[documentID]; !exists {
			return domain.ResumeRunScope{}, fmt.Errorf(
				"run scope document %q does not belong to the active profile",
				documentID,
			)
		}
		if _, exists := seen[documentID]; exists {
			continue
		}
		seen[documentID] = struct{}{}
		normalized = append(normalized, documentID)
	}
	return domain.ResumeRunScope{
		ProfileID:   profileID,
		JDID:        jdID,
		Mode:        scopeMode,
		DocumentIDs: normalized,
		UpdatedAt:   now.UTC(),
	}, nil
}

func applyRunScope(
	evidence []domain.Evidence,
	scope domain.ResumeRunScope,
) []domain.Evidence {
	if scope.Mode == domain.RunScopeAll {
		return evidence
	}
	selected := make(map[string]struct{}, len(scope.DocumentIDs))
	for _, documentID := range scope.DocumentIDs {
		selected[documentID] = struct{}{}
	}
	filtered := make([]domain.Evidence, 0, len(evidence))
	for _, item := range evidence {
		sources := make([]domain.EvidenceSource, 0, len(item.Sources))
		for _, source := range item.Sources {
			if _, exists := selected[source.DocumentID]; exists {
				sources = append(sources, source)
			}
		}
		if len(sources) == 0 {
			continue
		}
		item.Sources = sources
		filtered = append(filtered, item)
	}
	return filtered
}

func runScopeContainsDocument(
	scope domain.ResumeRunScope,
	documentID string,
) bool {
	return scope.Mode == domain.RunScopeAll ||
		slices.Contains(scope.DocumentIDs, documentID)
}
