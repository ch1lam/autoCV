package app

import (
	"testing"
	"time"

	"github.com/ch1lam/autocv/internal/domain"
)

func TestReconcileImportedEvidenceMergesDuplicateSources(t *testing.T) {
	now := time.Date(2026, 6, 15, 8, 0, 0, 0, time.UTC)
	existing := []domain.Evidence{{
		ID:        "existing",
		ProfileID: "profile",
		Kind:      "skill",
		Title:     "Go",
		Content:   "使用 Go 开发后端服务。",
		CreatedAt: now,
		UpdatedAt: now,
	}}
	incoming := []domain.Evidence{{
		ID:        "incoming",
		ProfileID: "profile",
		Kind:      "skill",
		Title:     "Go 后端",
		Content:   "  使用 Go 开发后端服务。 ",
		CreatedAt: now,
		UpdatedAt: now,
	}}
	sources := []domain.EvidenceSource{{
		EvidenceID: "incoming",
		ChunkID:    "chunk-new",
	}}

	kept, mergedSources, mergedCount, conflictCount :=
		reconcileImportedEvidence(existing, incoming, sources)

	if len(kept) != 0 || mergedCount != 1 || conflictCount != 0 {
		t.Fatalf(
			"unexpected reconciliation result kept=%#v merged=%d conflicts=%d",
			kept,
			mergedCount,
			conflictCount,
		)
	}
	if len(mergedSources) != 1 ||
		mergedSources[0].EvidenceID != "existing" {
		t.Fatalf("expected source to target existing evidence, got %#v", mergedSources)
	}
}

func TestEvidenceRelationsPreserveConflictsUntilOneVersionIsConfirmed(
	t *testing.T,
) {
	items := []domain.Evidence{
		{
			ID:      "older",
			Kind:    "experience",
			Title:   "支付平台职责",
			Content: "负责支付平台接口开发。",
		},
		{
			ID:      "newer",
			Kind:    "experience",
			Title:   " 支付平台职责 ",
			Content: "负责支付平台架构设计。",
		},
		{
			ID:      "skill",
			Kind:    "skill",
			Title:   "Go",
			Content: "熟悉 Go。",
		},
	}

	relations := analyzeEvidenceRelations(items)
	if len(relations.conflictIDs["older"]) != 1 ||
		relations.conflictIDs["older"][0] != "newer" ||
		len(relations.conflictIDs["newer"]) != 1 ||
		len(relations.conflictIDs["skill"]) != 0 {
		t.Fatalf("unexpected conflict relations %#v", relations.conflictIDs)
	}

	selected := selectUsableEvidence(items)
	if len(selected) != 1 || selected[0].ID != "skill" {
		t.Fatalf("expected unresolved conflict to be excluded, got %#v", selected)
	}

	items[1].UserVerified = true
	selected = selectUsableEvidence(items)
	if len(selected) != 2 ||
		selected[0].ID != "newer" ||
		selected[1].ID != "skill" {
		t.Fatalf("expected sole confirmed version and skill, got %#v", selected)
	}

	items[0].UserVerified = true
	selected = selectUsableEvidence(items)
	if len(selected) != 1 || selected[0].ID != "skill" {
		t.Fatalf("expected multiple confirmed conflicts to remain excluded, got %#v", selected)
	}
}

func TestEvidenceSummariesExposeConflictIDs(t *testing.T) {
	items := []domain.Evidence{
		{
			ID:      "left",
			Kind:    "project",
			Title:   "订单系统",
			Content: "负责订单接口。",
		},
		{
			ID:      "right",
			Kind:    "project",
			Title:   "订单系统",
			Content: "负责订单数据库。",
		},
	}

	summaries := evidenceSummaries(items)
	if len(summaries) != 2 ||
		len(summaries[0].ConflictEvidenceIDs) != 1 ||
		summaries[0].ConflictEvidenceIDs[0] != "right" ||
		summaries[1].ConflictEvidenceIDs[0] != "left" {
		t.Fatalf("unexpected conflict summaries %#v", summaries)
	}
}
