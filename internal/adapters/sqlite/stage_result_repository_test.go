package sqlite

import (
	"context"
	"path/filepath"
	"testing"
	"time"

	"github.com/ch1lam/autocv/internal/domain"
	"github.com/ch1lam/autocv/internal/workflow"
)

func TestStageResultRepositorySavesAndRestoresResults(t *testing.T) {
	ctx := context.Background()
	db := openTestDatabase(
		t,
		ctx,
		filepath.Join(t.TempDir(), "stage-results.db"),
	)
	defer db.Close()
	now := time.Date(2026, 6, 17, 13, 0, 0, 0, time.UTC)
	seedResumeDependencies(t, db, now)
	resumeRepository := NewResumeRepository(db)
	run := domain.ResumeRun{
		ID:             "run-1",
		ProfileID:      "profile-1",
		JDID:           "jd-1",
		Status:         "active",
		Stage:          string(workflow.StageMatched),
		PackagingLevel: 0.5,
		Language:       domain.ResumeLanguageChinese,
		CreatedAt:      now,
		UpdatedAt:      now,
	}
	if err := resumeRepository.SaveRun(ctx, run); err != nil {
		t.Fatalf("save resume run: %v", err)
	}

	repository := NewStageResultRepository(db)
	first := workflow.StageResult{
		ID:         "stage-result-1",
		RunID:      run.ID,
		Stage:      workflow.StageMatched,
		InputHash:  "hash-1",
		Status:     workflow.StageStatusSucceeded,
		ResultJSON: `{"score":82}`,
		CreatedAt:  now,
		UpdatedAt:  now,
	}
	if err := repository.SaveStageResult(ctx, first); err != nil {
		t.Fatalf("save first stage result: %v", err)
	}
	reusable, found, err := repository.SucceededStageResult(
		ctx,
		run.ID,
		workflow.StageMatched,
		"hash-1",
	)
	if err != nil {
		t.Fatalf("get reusable stage result: %v", err)
	}
	if !found || reusable.ResultJSON != first.ResultJSON {
		t.Fatalf("expected reusable result, got found=%v %#v", found, reusable)
	}

	second := workflow.StageResult{
		ID:        "stage-result-2",
		RunID:     run.ID,
		Stage:     workflow.StageMatched,
		InputHash: "hash-2",
		Status:    workflow.StageStatusRunning,
		CreatedAt: now.Add(time.Minute),
		UpdatedAt: now.Add(time.Minute),
	}
	if err := repository.SaveStageResult(ctx, second); err != nil {
		t.Fatalf("save running stage result: %v", err)
	}
	if _, found, err := repository.SucceededStageResult(
		ctx,
		run.ID,
		workflow.StageMatched,
		"hash-2",
	); err != nil {
		t.Fatalf("get non-reusable stage result: %v", err)
	} else if found {
		t.Fatal("running stage result should not be reusable")
	}

	second.ID = "stage-result-3"
	second.Status = workflow.StageStatusFailed
	second.ErrorJSON = `{"message":"provider failed"}`
	second.UpdatedAt = now.Add(2 * time.Minute)
	if err := repository.SaveStageResult(ctx, second); err != nil {
		t.Fatalf("update failed stage result: %v", err)
	}
	latest, found, err := repository.LatestStageResult(
		ctx,
		run.ID,
		workflow.StageMatched,
	)
	if err != nil {
		t.Fatalf("get latest stage result: %v", err)
	}
	if !found ||
		latest.ID != second.ID ||
		latest.InputHash != second.InputHash ||
		latest.Status != workflow.StageStatusFailed ||
		latest.ErrorJSON != second.ErrorJSON {
		t.Fatalf("unexpected latest stage result found=%v %#v", found, latest)
	}
}
