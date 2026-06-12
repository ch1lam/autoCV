package sqlite

import (
	"context"
	"path/filepath"
	"testing"
	"time"

	"github.com/ch1lam/autocv/internal/domain"
)

func TestArtifactRepositoryReturnsLatestSuccessfulArtifact(t *testing.T) {
	ctx := context.Background()
	db := openTestDatabase(t, ctx, filepath.Join(t.TempDir(), "artifact.db"))
	defer db.Close()

	now := time.Date(2026, 6, 12, 5, 0, 0, 0, time.UTC)
	seedResumeDependencies(t, db, now)
	resumeRepository := NewResumeRepository(db)
	run := domain.ResumeRun{
		ID:             "run-1",
		ProfileID:      "profile-1",
		JDID:           "jd-1",
		Status:         "active",
		Stage:          "drafted",
		PackagingLevel: 0.5,
		Language:       domain.ResumeLanguageChinese,
		CreatedAt:      now,
		UpdatedAt:      now,
	}
	firstResume := resumeRepositoryFixture(now)
	if err := resumeRepository.SaveVersion(ctx, run, firstResume); err != nil {
		t.Fatalf("save first resume: %v", err)
	}

	secondResume := firstResume
	secondResume.ID = "resume-2"
	secondResume.Version = 2
	secondResume.CreatedAt = now.Add(time.Minute)
	run.UpdatedAt = secondResume.CreatedAt
	if err := resumeRepository.SaveVersion(ctx, run, secondResume); err != nil {
		t.Fatalf("save second resume: %v", err)
	}

	repository := NewArtifactRepository(db)
	for _, artifact := range []domain.Artifact{
		{
			ID:       "artifact-1",
			RunID:    run.ID,
			ResumeID: firstResume.ID,
			Kind:     domain.ArtifactKindPDF,
			Path:     "runs/run-1/artifacts/artifact-1.pdf",
			PreviewPaths: []string{
				"runs/run-1/artifacts/artifact-1-page-1.png",
			},
			ContentHash: "hash-1",
			CreatedAt:   now,
		},
		{
			ID:       "artifact-2",
			RunID:    run.ID,
			ResumeID: secondResume.ID,
			Kind:     domain.ArtifactKindPDF,
			Path:     "runs/run-1/artifacts/artifact-2.pdf",
			PreviewPaths: []string{
				"runs/run-1/artifacts/artifact-2-page-1.png",
				"runs/run-1/artifacts/artifact-2-page-2.png",
			},
			ContentHash: "hash-2",
			CreatedAt:   now.Add(2 * time.Minute),
		},
	} {
		if err := repository.Save(ctx, artifact); err != nil {
			t.Fatalf("save artifact: %v", err)
		}
	}

	latest, found, err := repository.GetLatest(
		ctx,
		run.ID,
		domain.ArtifactKindPDF,
	)
	if err != nil {
		t.Fatalf("get latest artifact: %v", err)
	}
	if !found || latest.ID != "artifact-2" ||
		latest.ResumeID != secondResume.ID ||
		len(latest.PreviewPaths) != 2 {
		t.Fatalf("unexpected latest artifact %#v", latest)
	}
}
