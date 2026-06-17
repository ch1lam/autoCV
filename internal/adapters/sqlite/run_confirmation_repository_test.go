package sqlite

import (
	"context"
	"path/filepath"
	"testing"
	"time"

	"github.com/ch1lam/autocv/internal/domain"
)

func TestRunConfirmationRepositorySavesListsAndDeletes(t *testing.T) {
	ctx := context.Background()
	db := openTestDatabase(t, ctx, filepath.Join(t.TempDir(), "confirm.db"))
	defer db.Close()
	now := time.Date(2026, 6, 17, 9, 0, 0, 0, time.UTC)
	seedResumeDependencies(t, db, now)
	seedClarificationRun(t, db, now)
	clarificationRepository := NewClarificationRepository(db)
	question := clarificationQuestion("question-1", 0, now)
	if err := clarificationRepository.ReplaceRoundQuestions(
		ctx,
		"run-1",
		1,
		[]domain.ClarificationQuestion{question},
	); err != nil {
		t.Fatalf("seed clarification question: %v", err)
	}
	repository := NewRunConfirmationRepository(db)

	confirmation := domain.RunConfirmation{
		ID:                      "confirmation-1",
		RunID:                   "run-1",
		ClarificationQuestionID: "question-1",
		RequirementID:           "requirement-1",
		Content:                 "  管理过 8 人后端团队。 ",
		CreatedAt:               now,
		UpdatedAt:               now,
	}
	if err := repository.SaveRunConfirmation(ctx, confirmation); err != nil {
		t.Fatalf("save run confirmation: %v", err)
	}

	saved, err := repository.ListRunConfirmations(ctx, "run-1")
	if err != nil {
		t.Fatalf("list run confirmations: %v", err)
	}
	if len(saved) != 1 ||
		saved[0].Content != "管理过 8 人后端团队。" ||
		saved[0].ClarificationQuestionID != "question-1" {
		t.Fatalf("unexpected saved confirmations %#v", saved)
	}

	confirmation.Content = "负责 8 人团队和跨部门交付。"
	confirmation.UpdatedAt = now.Add(time.Minute)
	if err := repository.SaveRunConfirmation(ctx, confirmation); err != nil {
		t.Fatalf("upsert run confirmation: %v", err)
	}
	saved, err = repository.ListRunConfirmations(ctx, "run-1")
	if err != nil {
		t.Fatalf("list upserted run confirmations: %v", err)
	}
	if len(saved) != 1 ||
		saved[0].Content != "负责 8 人团队和跨部门交付。" {
		t.Fatalf("expected upserted confirmation, got %#v", saved)
	}

	if err := repository.DeleteRunConfirmation(
		ctx,
		"run-1",
		"question-1",
	); err != nil {
		t.Fatalf("delete run confirmation: %v", err)
	}
	saved, err = repository.ListRunConfirmations(ctx, "run-1")
	if err != nil {
		t.Fatalf("list deleted run confirmations: %v", err)
	}
	if len(saved) != 0 {
		t.Fatalf("expected no confirmations, got %#v", saved)
	}
}
