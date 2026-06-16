package sqlite

import (
	"context"
	"database/sql"
	"path/filepath"
	"testing"
	"time"

	"github.com/ch1lam/autocv/internal/domain"
)

func TestClarificationRepositoryReplacesRoundAndUpdatesStatus(t *testing.T) {
	ctx := context.Background()
	db := openTestDatabase(t, ctx, filepath.Join(t.TempDir(), "clarify.db"))
	defer db.Close()
	now := time.Date(2026, 6, 16, 9, 0, 0, 0, time.UTC)
	seedResumeDependencies(t, db, now)
	seedClarificationRun(t, db, now)
	repository := NewClarificationRepository(db)

	questions := []domain.ClarificationQuestion{
		clarificationQuestion("question-1", 0, now),
		clarificationQuestion("question-2", 1, now),
	}
	if err := repository.ReplaceRoundQuestions(
		ctx,
		"run-1",
		1,
		questions,
	); err != nil {
		t.Fatalf("replace clarification round: %v", err)
	}

	saved, err := repository.ListQuestions(ctx, "run-1")
	if err != nil {
		t.Fatalf("list clarification questions: %v", err)
	}
	if len(saved) != 2 ||
		saved[0].ID != "question-1" ||
		saved[1].ID != "question-2" {
		t.Fatalf("unexpected saved questions %#v", saved)
	}

	answered, err := repository.UpdateQuestionStatus(
		ctx,
		"question-1",
		domain.ClarificationAnswered,
		"管理 8 人后端团队。",
		now.Add(time.Minute),
	)
	if err != nil {
		t.Fatalf("answer clarification question: %v", err)
	}
	if answered.Status != domain.ClarificationAnswered ||
		answered.Answer != "管理 8 人后端团队。" {
		t.Fatalf("unexpected answered question %#v", answered)
	}

	replacement := []domain.ClarificationQuestion{
		clarificationQuestion("question-3", 0, now.Add(2*time.Minute)),
	}
	if err := repository.ReplaceRoundQuestions(
		ctx,
		"run-1",
		1,
		replacement,
	); err != nil {
		t.Fatalf("replace existing clarification round: %v", err)
	}
	saved, err = repository.ListQuestions(ctx, "run-1")
	if err != nil {
		t.Fatalf("list replaced clarification questions: %v", err)
	}
	if len(saved) != 1 || saved[0].ID != "question-3" {
		t.Fatalf("expected round replacement, got %#v", saved)
	}
}

func seedClarificationRun(
	t *testing.T,
	db *sql.DB,
	now time.Time,
) {
	t.Helper()
	if _, err := db.Exec(
		`INSERT INTO resume_runs(
			id, profile_id, jd_id, status, stage, packaging_level,
			language, created_at, updated_at
		) VALUES (
			'run-1', 'profile-1', 'jd-1', 'active', 'match', 0.5,
			'zh', ?, ?
		)`,
		formatTime(now),
		formatTime(now),
	); err != nil {
		t.Fatalf("seed resume run: %v", err)
	}
}

func clarificationQuestion(
	id string,
	ordinal int,
	now time.Time,
) domain.ClarificationQuestion {
	return domain.ClarificationQuestion{
		ID:            id,
		RunID:         "run-1",
		RequirementID: "requirement-1",
		Round:         1,
		Ordinal:       ordinal,
		Question:      "是否负责过团队规模或协作范围？",
		Reason:        "JD 要求负责人经验，但 Profile 中没有团队规模。",
		Status:        domain.ClarificationPending,
		CreatedAt:     now,
		UpdatedAt:     now,
	}
}
