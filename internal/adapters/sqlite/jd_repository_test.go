package sqlite

import (
	"context"
	"path/filepath"
	"testing"
	"time"

	"github.com/ch1lam/autocv/internal/domain"
	"github.com/ch1lam/autocv/internal/ports"
)

func TestJDRepositorySavesAnalysisAndInvalidatesItOnEdit(t *testing.T) {
	ctx := context.Background()
	db := openTestDatabase(t, ctx, filepath.Join(t.TempDir(), "jd.db"))
	defer db.Close()
	repository := NewJDRepository(db)
	now := time.Date(2026, 6, 11, 2, 0, 0, 0, time.UTC)

	draft := domain.JobDescription{
		ID:             "jd-1",
		Title:          "Backend Engineer",
		RawText:        "Backend Engineer\nGo required.",
		Language:       "en",
		RawHash:        "hash-1",
		AnalysisStatus: "pending",
		CreatedAt:      now,
		UpdatedAt:      now,
	}
	if err := repository.SaveDraft(ctx, draft); err != nil {
		t.Fatalf("save draft: %v", err)
	}
	if err := repository.UpdateAnalysis(ctx, ports.JDAnalysisUpdate{
		ID:           draft.ID,
		RawHash:      draft.RawHash,
		Title:        "Senior Backend Engineer",
		Company:      "Example Technology",
		Language:     "mixed",
		AnalysisJSON: `{"role":"Senior Backend Engineer"}`,
		Status:       "succeeded",
		UpdatedAt:    now.Add(time.Minute),
	}); err != nil {
		t.Fatalf("save analysis: %v", err)
	}

	saved, found, err := repository.GetLatest(ctx)
	if err != nil {
		t.Fatalf("get analyzed JD: %v", err)
	}
	if !found || saved.AnalysisStatus != "succeeded" ||
		saved.AnalysisJSON == "" {
		t.Fatalf("expected analyzed JD, got %#v", saved)
	}

	draft.RawText = "Backend Engineer\nGo and PostgreSQL required."
	draft.RawHash = "hash-2"
	draft.UpdatedAt = now.Add(2 * time.Minute)
	if err := repository.SaveDraft(ctx, draft); err != nil {
		t.Fatalf("save edited draft: %v", err)
	}
	edited, found, err := repository.GetLatest(ctx)
	if err != nil {
		t.Fatalf("get edited JD: %v", err)
	}
	if !found || edited.AnalysisStatus != "pending" ||
		edited.AnalysisJSON != "" {
		t.Fatalf("expected invalidated analysis, got %#v", edited)
	}
}

func TestJDRepositoryRejectsAnalysisForStaleDraft(t *testing.T) {
	ctx := context.Background()
	db := openTestDatabase(t, ctx, filepath.Join(t.TempDir(), "stale-jd.db"))
	defer db.Close()
	repository := NewJDRepository(db)
	now := time.Date(2026, 6, 11, 2, 0, 0, 0, time.UTC)
	draft := domain.JobDescription{
		ID:             "jd-1",
		Title:          "Backend Engineer",
		RawText:        "Go required.",
		Language:       "en",
		RawHash:        "current-hash",
		AnalysisStatus: "pending",
		CreatedAt:      now,
		UpdatedAt:      now,
	}
	if err := repository.SaveDraft(ctx, draft); err != nil {
		t.Fatalf("save draft: %v", err)
	}

	err := repository.UpdateAnalysis(ctx, ports.JDAnalysisUpdate{
		ID:        draft.ID,
		RawHash:   "stale-hash",
		Title:     draft.Title,
		Language:  draft.Language,
		Status:    "succeeded",
		UpdatedAt: now.Add(time.Minute),
	})
	if err == nil {
		t.Fatal("expected stale analysis error")
	}
}
