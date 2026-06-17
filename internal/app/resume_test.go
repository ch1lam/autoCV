package app

import (
	"context"
	"errors"
	"slices"
	"strings"
	"testing"
	"time"

	"github.com/ch1lam/autocv/internal/adapters/fakeprovider"
	sqliteadapter "github.com/ch1lam/autocv/internal/adapters/sqlite"
	"github.com/ch1lam/autocv/internal/domain"
	"github.com/ch1lam/autocv/internal/ports"
	"github.com/ch1lam/autocv/internal/workflow"
)

type failingResumeDrafter struct{}

func (failingResumeDrafter) DraftResume(
	context.Context,
	ports.DraftResumeRequest,
) (domain.ResumeDraft, error) {
	return domain.ResumeDraft{}, errors.New("resume provider unavailable")
}

type cancelledResumeDrafter struct{}

func (cancelledResumeDrafter) DraftResume(
	context.Context,
	ports.DraftResumeRequest,
) (domain.ResumeDraft, error) {
	return domain.ResumeDraft{}, context.Canceled
}

func TestResumeServiceGeneratesRestoresEditsAndPreservesLocks(t *testing.T) {
	matchFixture := newMatchServiceFixture(t, fakeprovider.New())
	matchFixture.importProfile(t)
	matchFixture.analyzeJD(t, matchFixture.jdText)
	if _, err := matchFixture.service.Analyze(); err != nil {
		t.Fatalf("analyze matches: %v", err)
	}
	repository := sqliteadapter.NewResumeRepository(
		matchFixture.db,
	)
	service := NewResumeService(
		repository,
		matchFixture.stageRepository,
		matchFixture.confirmationRepository,
		matchFixture.matchRepository,
		matchFixture.profileRepository,
		matchFixture.jdRepository,
		fakeprovider.New(),
		fixedClock{now: profileTestTime.Add(2 * time.Hour)},
	)

	generated, err := service.Generate("zh", 0.5)
	if err != nil {
		t.Fatalf("generate resume: %v", err)
	}
	if generated.Status != "ready" || generated.Version != 1 ||
		len(generated.Blocks) < 2 {
		t.Fatalf("unexpected generated workspace %#v", generated)
	}
	if generated.PackagingStrategy.ID != "balanced" ||
		generated.PackagingStrategy.Description == "" {
		t.Fatalf("expected balanced packaging strategy, got %#v", generated.PackagingStrategy)
	}
	stageResult, found, err := matchFixture.stageRepository.LatestStageResult(
		context.Background(),
		generated.RunID,
		workflow.StageDrafted,
	)
	if err != nil {
		t.Fatalf("read resume draft stage result: %v", err)
	}
	if !found ||
		stageResult.Status != workflow.StageStatusSucceeded ||
		!strings.Contains(stageResult.ResultJSON, `"resume_id"`) {
		t.Fatalf("unexpected resume draft stage result found=%v %#v", found, stageResult)
	}
	reusing := NewResumeService(
		repository,
		matchFixture.stageRepository,
		matchFixture.confirmationRepository,
		matchFixture.matchRepository,
		matchFixture.profileRepository,
		matchFixture.jdRepository,
		failingResumeDrafter{},
		fixedClock{now: profileTestTime.Add(2*time.Hour + time.Minute)},
	)
	reused, err := reusing.Generate("zh", 0.5)
	if err != nil {
		t.Fatalf("reuse successful resume draft stage: %v", err)
	}
	if reused.Version != generated.Version ||
		reused.ResumeID != generated.ResumeID {
		t.Fatalf("unexpected reused resume workspace %#v", reused)
	}
	if generated.Blocks[0].Evidence[0].Sources[0].DocumentName == "" {
		t.Fatal("expected traceable resume source")
	}

	lockedID := generated.Blocks[1].ID
	lockedContent := generated.Blocks[1].Content
	locked, err := service.SetBlockLocked(lockedID, true)
	if err != nil {
		t.Fatalf("lock resume block: %v", err)
	}
	if locked.Version != 2 || !locked.Blocks[1].Locked {
		t.Fatalf("expected appended lock version, got %#v", locked)
	}

	regenerated, err := service.Generate("zh", 0.5)
	if err != nil {
		t.Fatalf("regenerate resume: %v", err)
	}
	if regenerated.Version != 3 {
		t.Fatalf("expected third version, got %d", regenerated.Version)
	}
	var preserved bool
	for _, block := range regenerated.Blocks {
		if block.ID == lockedID && block.Locked &&
			block.Content == lockedContent {
			preserved = true
		}
	}
	if !preserved {
		t.Fatal("expected locked block to survive regeneration")
	}

	editable := regenerated.Blocks[0]
	editedMarkdown := strings.Replace(
		regenerated.Markdown,
		editable.Content,
		editable.Content+" 用户确认补充。",
		1,
	)
	edited, err := service.UpdateMarkdown(editedMarkdown)
	if err != nil {
		t.Fatalf("update resume Markdown: %v", err)
	}
	if edited.Version != 4 ||
		edited.Blocks[0].GroundingLevel != "user_confirmed" {
		t.Fatalf("expected user-confirmed fourth version, got %#v", edited)
	}

	restarted := NewResumeService(
		repository,
		matchFixture.stageRepository,
		matchFixture.confirmationRepository,
		matchFixture.matchRepository,
		matchFixture.profileRepository,
		matchFixture.jdRepository,
		fakeprovider.New(),
		fixedClock{now: profileTestTime.Add(3 * time.Hour)},
	)
	restored, err := restarted.GetWorkspace()
	if err != nil {
		t.Fatalf("restore resume workspace: %v", err)
	}
	if restored.Status != "ready" || restored.Version != 4 ||
		restored.Markdown != edited.Markdown {
		t.Fatalf("unexpected restored workspace %#v", restored)
	}
}

func TestResumeServiceRecordsDraftStageFailures(t *testing.T) {
	tests := []struct {
		name       string
		drafter    ports.ResumeDrafter
		wantStatus workflow.StageStatus
		wantError  string
	}{
		{
			name:       "failed",
			drafter:    failingResumeDrafter{},
			wantStatus: workflow.StageStatusFailed,
			wantError:  "resume provider unavailable",
		},
		{
			name:       "cancelled",
			drafter:    cancelledResumeDrafter{},
			wantStatus: workflow.StageStatusCancelled,
			wantError:  "context canceled",
		},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			matchFixture := newMatchServiceFixture(t, fakeprovider.New())
			matchFixture.importProfile(t)
			matchFixture.analyzeJD(t, matchFixture.jdText)
			if _, err := matchFixture.service.Analyze(); err != nil {
				t.Fatalf("analyze matches: %v", err)
			}
			service := NewResumeService(
				sqliteadapter.NewResumeRepository(matchFixture.db),
				matchFixture.stageRepository,
				matchFixture.confirmationRepository,
				matchFixture.matchRepository,
				matchFixture.profileRepository,
				matchFixture.jdRepository,
				test.drafter,
				fixedClock{now: profileTestTime.Add(2 * time.Hour)},
			)

			if _, err := service.Generate("zh", 0.5); err == nil {
				t.Fatal("expected resume generation error")
			}
			var runID string
			if err := matchFixture.db.QueryRow(
				"SELECT id FROM resume_runs LIMIT 1",
			).Scan(&runID); err != nil {
				t.Fatalf("read resume run id: %v", err)
			}
			stageResult, found, err := matchFixture.stageRepository.LatestStageResult(
				context.Background(),
				runID,
				workflow.StageDrafted,
			)
			if err != nil {
				t.Fatalf("read resume draft stage result: %v", err)
			}
			if !found ||
				stageResult.Status != test.wantStatus ||
				!strings.Contains(stageResult.ErrorJSON, test.wantError) {
				t.Fatalf(
					"unexpected resume draft stage result found=%v %#v",
					found,
					stageResult,
				)
			}
		})
	}
}

func TestResumeServiceUsesClarificationConfirmations(t *testing.T) {
	matchFixture := newMatchServiceFixture(t, allClarificationSuggester{})
	matchFixture.importProfile(t)
	matchFixture.analyzeJD(t, matchFixture.jdText)
	review, err := matchFixture.service.Analyze()
	if err != nil {
		t.Fatalf("analyze matches: %v", err)
	}
	if len(review.Clarifications) < 2 {
		t.Fatalf("expected clarification questions, got %#v", review.Clarifications)
	}

	answer := "负责 8 人后端团队和跨部门交付。"
	if _, err := matchFixture.service.AnswerClarification(
		review.Clarifications[0].ID,
		answer,
	); err != nil {
		t.Fatalf("answer clarification: %v", err)
	}
	service := NewResumeService(
		sqliteadapter.NewResumeRepository(matchFixture.db),
		matchFixture.stageRepository,
		matchFixture.confirmationRepository,
		matchFixture.matchRepository,
		matchFixture.profileRepository,
		matchFixture.jdRepository,
		fakeprovider.New(),
		fixedClock{now: profileTestTime.Add(2 * time.Hour)},
	)
	generated, err := service.Generate("zh", 0.5)
	if err != nil {
		t.Fatalf("generate resume with confirmation: %v", err)
	}
	if !slices.ContainsFunc(
		generated.Blocks,
		func(block ResumeBlockSummary) bool {
			return block.GroundingLevel == "user_confirmed" &&
				block.Content == answer
		},
	) {
		t.Fatalf("expected user-confirmed block, got %#v", generated.Blocks)
	}

	if _, err := matchFixture.service.AnswerClarification(
		review.Clarifications[1].ID,
		"补充第二条用户确认。",
	); err != nil {
		t.Fatalf("answer second clarification: %v", err)
	}
	stale, err := service.GetWorkspace()
	if err != nil {
		t.Fatalf("get stale resume after confirmation change: %v", err)
	}
	if stale.Status != "stale" ||
		!strings.Contains(stale.Message, "追问确认") {
		t.Fatalf("expected confirmation change to stale resume, got %#v", stale)
	}
}

func TestResumeServiceRejectsStructuralMarkdownChanges(t *testing.T) {
	fixture := newResumeServiceFixture(t)
	generated, err := fixture.Generate("zh", 0.5)
	if err != nil {
		t.Fatalf("generate resume: %v", err)
	}
	changed := strings.Replace(generated.Markdown, "# ", "## ", 1)
	if _, err := fixture.UpdateMarkdown(changed); err == nil ||
		!strings.Contains(err.Error(), "structure changed") {
		t.Fatalf("expected structural edit error, got %v", err)
	}
}

func TestResumeServiceWarnsWhenUpstreamChangesWithLockedBlocks(t *testing.T) {
	matchFixture := newMatchServiceFixture(t, fakeprovider.New())
	matchFixture.importProfile(t)
	matchFixture.analyzeJD(t, matchFixture.jdText)
	if _, err := matchFixture.service.Analyze(); err != nil {
		t.Fatalf("analyze matches: %v", err)
	}
	service := NewResumeService(
		sqliteadapter.NewResumeRepository(matchFixture.db),
		matchFixture.stageRepository,
		matchFixture.confirmationRepository,
		matchFixture.matchRepository,
		matchFixture.profileRepository,
		matchFixture.jdRepository,
		fakeprovider.New(),
		fixedClock{now: profileTestTime.Add(2 * time.Hour)},
	)
	generated, err := service.Generate("zh", 0.5)
	if err != nil {
		t.Fatalf("generate resume: %v", err)
	}
	locked, err := service.SetBlockLocked(generated.Blocks[0].ID, true)
	if err != nil {
		t.Fatalf("lock resume block: %v", err)
	}
	lockedContent := locked.Blocks[0].Content

	matchFixture.analyzeJD(
		t,
		matchFixture.jdText+"\nOwn incident response and reliability reviews.",
	)
	if _, err := matchFixture.service.Analyze(); err != nil {
		t.Fatalf("reanalyze matches: %v", err)
	}
	stale, err := service.GetWorkspace()
	if err != nil {
		t.Fatalf("get stale resume: %v", err)
	}
	if stale.Status != "stale" ||
		!strings.Contains(stale.Message, "1 个锁定 Block") {
		t.Fatalf("expected locked conflict warning, got %#v", stale)
	}

	regenerated, err := service.Generate("zh", 0.5)
	if err != nil {
		t.Fatalf("regenerate resume: %v", err)
	}
	if regenerated.Blocks[0].Content != lockedContent ||
		!regenerated.Blocks[0].Locked {
		t.Fatalf("expected locked content to remain unchanged, got %#v", regenerated.Blocks[0])
	}
	if !slices.ContainsFunc(
		regenerated.OptimizationNotes,
		func(note string) bool {
			return strings.Contains(note, "上游资料或 JD 已变化")
		},
	) {
		t.Fatalf(
			"expected upstream lock warning in optimization notes, got %#v",
			regenerated.OptimizationNotes,
		)
	}
}

func TestResumeServiceUsesActiveProfile(t *testing.T) {
	matchFixture := newMatchServiceFixture(t, fakeprovider.New())
	matchFixture.importProfile(t)
	matchFixture.analyzeJD(t, matchFixture.jdText)
	if _, err := matchFixture.service.Analyze(); err != nil {
		t.Fatalf("analyze default profile: %v", err)
	}
	mainProfile, err := matchFixture.profileService.GetOverview()
	if err != nil {
		t.Fatalf("get default profile: %v", err)
	}
	service := NewResumeService(
		sqliteadapter.NewResumeRepository(matchFixture.db),
		matchFixture.stageRepository,
		matchFixture.confirmationRepository,
		matchFixture.matchRepository,
		matchFixture.profileRepository,
		matchFixture.jdRepository,
		fakeprovider.New(),
		fixedClock{now: profileTestTime.Add(2 * time.Hour)},
	)
	generated, err := service.Generate("zh", 0.5)
	if err != nil {
		t.Fatalf("generate default profile resume: %v", err)
	}

	if _, err := matchFixture.profileService.CreateProfile(
		"Empty profile",
		"en",
	); err != nil {
		t.Fatalf("create profile: %v", err)
	}
	blocked, err := service.GetWorkspace()
	if err != nil {
		t.Fatalf("get workspace for empty profile: %v", err)
	}
	if blocked.Status != "blocked" {
		t.Fatalf("expected active empty profile to block resume, got %#v", blocked)
	}

	if _, err := matchFixture.profileService.SelectProfile(
		mainProfile.ProfileID,
	); err != nil {
		t.Fatalf("restore default profile: %v", err)
	}
	restored, err := service.GetWorkspace()
	if err != nil {
		t.Fatalf("restore default profile resume: %v", err)
	}
	if restored.Status != "ready" || restored.ResumeID != generated.ResumeID {
		t.Fatalf("expected saved default profile resume, got %#v", restored)
	}
}

func newResumeServiceFixture(t *testing.T) *ResumeService {
	t.Helper()
	matchFixture := newMatchServiceFixture(t, fakeprovider.New())
	matchFixture.importProfile(t)
	matchFixture.analyzeJD(t, matchFixture.jdText)
	if _, err := matchFixture.service.Analyze(); err != nil {
		t.Fatalf("analyze matches: %v", err)
	}
	return NewResumeService(
		sqliteadapter.NewResumeRepository(matchFixture.db),
		matchFixture.stageRepository,
		matchFixture.confirmationRepository,
		matchFixture.matchRepository,
		matchFixture.profileRepository,
		matchFixture.jdRepository,
		fakeprovider.New(),
		fixedClock{now: profileTestTime.Add(2 * time.Hour)},
	)
}
