package app

import (
	"slices"
	"strings"
	"testing"
	"time"

	"github.com/ch1lam/autocv/internal/adapters/fakeprovider"
	sqliteadapter "github.com/ch1lam/autocv/internal/adapters/sqlite"
)

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
		matchFixture.matchRepository,
		matchFixture.profileRepository,
		matchFixture.jdRepository,
		fakeprovider.New(),
		fixedClock{now: profileTestTime.Add(2 * time.Hour)},
	)
}
