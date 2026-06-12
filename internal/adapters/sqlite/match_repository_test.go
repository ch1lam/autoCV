package sqlite

import (
	"context"
	"database/sql"
	"path/filepath"
	"testing"
	"time"

	"github.com/ch1lam/autocv/internal/domain"
	"github.com/ch1lam/autocv/internal/ports"
)

func TestMatchRepositoryReplacesPersistedAnalysis(t *testing.T) {
	ctx := context.Background()
	db := openTestDatabase(
		t,
		ctx,
		filepath.Join(t.TempDir(), "autocv.db"),
	)
	defer db.Close()

	profileRepository := NewProfileRepository(db)
	profile, err := profileRepository.EnsureDefaultProfile(
		ctx,
		"Main",
		"en",
		time.Date(2026, 6, 11, 1, 0, 0, 0, time.UTC),
	)
	if err != nil {
		t.Fatalf("ensure profile: %v", err)
	}
	jdRepository := NewJDRepository(db)
	now := time.Date(2026, 6, 11, 1, 0, 0, 0, time.UTC)
	jd := domain.JobDescription{
		ID:             "jd-1",
		Title:          "Backend Engineer",
		RawText:        "Backend Engineer",
		Language:       "en",
		RawHash:        "jd-hash",
		AnalysisStatus: "pending",
		CreatedAt:      now,
		UpdatedAt:      now,
	}
	if err := jdRepository.SaveDraft(ctx, jd); err != nil {
		t.Fatalf("save JD: %v", err)
	}

	importEvidenceFixture(t, ctx, db, profile.ID, now)

	repository := NewMatchRepository(db)
	first := matchAnalysisFixture(profile.ID, jd.ID, "input-1", now)
	if err := repository.Save(ctx, first); err != nil {
		t.Fatalf("save first analysis: %v", err)
	}
	restored, found, err := repository.GetLatest(ctx, profile.ID, jd.ID)
	if err != nil {
		t.Fatalf("get first analysis: %v", err)
	}
	if !found || len(restored.Requirements) != 1 ||
		len(restored.Suggestions) != 1 ||
		len(restored.Suggestions[0].EvidenceIDs) != 1 {
		t.Fatalf("unexpected restored analysis %#v", restored)
	}

	second := matchAnalysisFixture(profile.ID, jd.ID, "input-2", now.Add(time.Hour))
	second.Requirements[0].Text = "Updated requirement"
	second.Suggestions[0].Strength = domain.MatchStrengthPartial
	if err := repository.Save(ctx, second); err != nil {
		t.Fatalf("save second analysis: %v", err)
	}
	restored, found, err = repository.GetLatest(ctx, profile.ID, jd.ID)
	if err != nil {
		t.Fatalf("get second analysis: %v", err)
	}
	if !found || restored.InputHash != "input-2" ||
		restored.Requirements[0].Text != "Updated requirement" ||
		restored.Suggestions[0].Strength != domain.MatchStrengthPartial {
		t.Fatalf("analysis was not replaced: %#v", restored)
	}
}

func matchAnalysisFixture(
	profileID string,
	jdID string,
	inputHash string,
	now time.Time,
) domain.MatchAnalysis {
	return domain.MatchAnalysis{
		ID:        "match-analysis-1",
		ProfileID: profileID,
		JDID:      jdID,
		InputHash: inputHash,
		Status:    "succeeded",
		CreatedAt: now,
		UpdatedAt: now,
		Requirements: []domain.MatchRequirement{{
			ID:         "required-go",
			Category:   domain.RequirementCategoryRequired,
			Text:       "Production Go experience",
			Importance: 5,
		}},
		Suggestions: []domain.MatchSuggestion{{
			RequirementID: "required-go",
			Strength:      domain.MatchStrengthStrong,
			EvidenceIDs:   []string{"evidence-1"},
			Explanation:   "Go experience is present.",
		}},
	}
}

func importEvidenceFixture(
	t *testing.T,
	ctx context.Context,
	db *sql.DB,
	profileID string,
	now time.Time,
) {
	t.Helper()
	repository := NewProfileRepository(db)
	err := repository.SaveImportedDocument(ctx, ports.ImportedDocument{
		Document: domain.SourceDocument{
			ID:           "document-1",
			ProfileID:    profileID,
			Kind:         "markdown",
			OriginalName: "profile.md",
			ManagedPath:  "profile.md",
			ContentHash:  "profile-hash",
			ParseStatus:  "succeeded",
			CreatedAt:    now,
			UpdatedAt:    now,
		},
		Chunks: []domain.SourceChunk{{
			ID:          "chunk-1",
			DocumentID:  "document-1",
			Text:        "Go services",
			LocatorJSON: "{}",
		}},
		Evidence: []domain.Evidence{{
			ID:         "evidence-1",
			ProfileID:  profileID,
			Kind:       "skill",
			Title:      "Go",
			Content:    "Go services",
			Confidence: 0.9,
			CreatedAt:  now,
			UpdatedAt:  now,
		}},
		EvidenceSources: []domain.EvidenceSource{{
			EvidenceID: "evidence-1",
			ChunkID:    "chunk-1",
			QuoteEnd:   11,
		}},
	})
	if err != nil {
		t.Fatalf("import evidence fixture: %v", err)
	}
}
