package app

import (
	"strings"

	"github.com/ch1lam/autocv/internal/domain"
)

type evidenceRelations struct {
	conflictIDs map[string][]string
}

func reconcileImportedEvidence(
	existing []domain.Evidence,
	incoming []domain.Evidence,
	sources []domain.EvidenceSource,
) ([]domain.Evidence, []domain.EvidenceSource, int, int) {
	canonicalByDuplicateKey := make(map[string]domain.Evidence)
	for _, item := range existing {
		key := evidenceDuplicateKey(item)
		canonical, found := canonicalByDuplicateKey[key]
		if !found || (!canonical.UserVerified && item.UserVerified) {
			canonicalByDuplicateKey[key] = item
		}
	}

	kept := make([]domain.Evidence, 0, len(incoming))
	evidenceIDs := make(map[string]string, len(incoming))
	mergedCount := 0
	for _, item := range incoming {
		key := evidenceDuplicateKey(item)
		if canonical, found := canonicalByDuplicateKey[key]; found {
			evidenceIDs[item.ID] = canonical.ID
			mergedCount++
			continue
		}
		canonicalByDuplicateKey[key] = item
		evidenceIDs[item.ID] = item.ID
		kept = append(kept, item)
	}

	keptSources := make([]domain.EvidenceSource, 0, len(sources))
	seenSources := make(map[string]struct{}, len(sources))
	for _, source := range sources {
		if evidenceID, found := evidenceIDs[source.EvidenceID]; found {
			source.EvidenceID = evidenceID
		}
		key := source.EvidenceID + "\x00" + source.ChunkID
		if _, found := seenSources[key]; found {
			continue
		}
		seenSources[key] = struct{}{}
		keptSources = append(keptSources, source)
	}

	all := make([]domain.Evidence, 0, len(existing)+len(kept))
	all = append(all, existing...)
	all = append(all, kept...)
	relations := analyzeEvidenceRelations(all)
	conflictCount := 0
	for _, item := range kept {
		if len(relations.conflictIDs[item.ID]) > 0 {
			conflictCount++
		}
	}
	return kept, keptSources, mergedCount, conflictCount
}

func analyzeEvidenceRelations(items []domain.Evidence) evidenceRelations {
	groups := make(map[string][]domain.Evidence)
	for _, item := range items {
		key := evidenceSubjectKey(item)
		groups[key] = append(groups[key], item)
	}

	conflictIDs := make(map[string][]string)
	for _, group := range groups {
		for left := range group {
			leftContent := normalizeEvidenceText(group[left].Content)
			for right := range group {
				if left == right ||
					leftContent == normalizeEvidenceText(group[right].Content) {
					continue
				}
				conflictIDs[group[left].ID] = append(
					conflictIDs[group[left].ID],
					group[right].ID,
				)
			}
		}
	}
	return evidenceRelations{conflictIDs: conflictIDs}
}

func selectUsableEvidence(items []domain.Evidence) []domain.Evidence {
	deduplicated := deduplicateStoredEvidence(items)
	relations := analyzeEvidenceRelations(deduplicated)
	verifiedBySubject := make(map[string]int)
	for _, item := range deduplicated {
		if len(relations.conflictIDs[item.ID]) > 0 && item.UserVerified {
			verifiedBySubject[evidenceSubjectKey(item)]++
		}
	}

	selected := make([]domain.Evidence, 0, len(deduplicated))
	for _, item := range deduplicated {
		if len(relations.conflictIDs[item.ID]) == 0 {
			selected = append(selected, item)
			continue
		}
		if item.UserVerified &&
			verifiedBySubject[evidenceSubjectKey(item)] == 1 {
			selected = append(selected, item)
		}
	}
	return selected
}

func deduplicateStoredEvidence(
	items []domain.Evidence,
) []domain.Evidence {
	result := make([]domain.Evidence, 0, len(items))
	indexes := make(map[string]int)
	for _, item := range items {
		key := evidenceDuplicateKey(item)
		index, found := indexes[key]
		if !found {
			item.Sources = append([]domain.EvidenceSource(nil), item.Sources...)
			indexes[key] = len(result)
			result = append(result, item)
			continue
		}

		current := result[index]
		sources := mergeEvidenceSources(current.Sources, item.Sources)
		if !current.UserVerified && item.UserVerified {
			item.Sources = sources
			result[index] = item
			continue
		}
		current.Sources = sources
		result[index] = current
	}
	return result
}

func mergeEvidenceSources(
	left []domain.EvidenceSource,
	right []domain.EvidenceSource,
) []domain.EvidenceSource {
	merged := append([]domain.EvidenceSource(nil), left...)
	seen := make(map[string]struct{}, len(left)+len(right))
	for _, source := range left {
		seen[source.ChunkID] = struct{}{}
	}
	for _, source := range right {
		if _, found := seen[source.ChunkID]; found {
			continue
		}
		seen[source.ChunkID] = struct{}{}
		merged = append(merged, source)
	}
	return merged
}

func evidenceDuplicateKey(item domain.Evidence) string {
	return normalizeEvidenceText(item.Kind) +
		"\x00" +
		normalizeEvidenceText(item.Content)
}

func evidenceSubjectKey(item domain.Evidence) string {
	return normalizeEvidenceText(item.Kind) +
		"\x00" +
		normalizeEvidenceText(item.Title)
}

func normalizeEvidenceText(value string) string {
	return strings.ToLower(strings.Join(strings.Fields(value), " "))
}
