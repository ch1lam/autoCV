package app

import (
	"context"
	"database/sql"
	"errors"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/ch1lam/autocv/internal/adapters/fakeprovider"
	"github.com/ch1lam/autocv/internal/adapters/filesystem"
	markdownparser "github.com/ch1lam/autocv/internal/adapters/markdown"
	sqliteadapter "github.com/ch1lam/autocv/internal/adapters/sqlite"
	"github.com/ch1lam/autocv/internal/domain"
	"github.com/ch1lam/autocv/internal/ports"
)

type invalidMatchSuggester struct{}

func (invalidMatchSuggester) SuggestMatches(
	context.Context,
	ports.SuggestMatchesRequest,
) ([]domain.MatchSuggestion, error) {
	return []domain.MatchSuggestion{{
		RequirementID: "unknown-requirement",
		Strength:      domain.MatchStrengthStrong,
		EvidenceIDs:   []string{"unknown-evidence"},
		Explanation:   "invalid fixture",
	}}, nil
}

type failingMatchSuggester struct{}

func (failingMatchSuggester) SuggestMatches(
	context.Context,
	ports.SuggestMatchesRequest,
) ([]domain.MatchSuggestion, error) {
	return nil, errors.New("provider unavailable")
}

func TestMatchServiceAnalyzesScoresAndRestoresReview(t *testing.T) {
	fixture := newMatchServiceFixture(t, fakeprovider.New())
	fixture.importProfile(t)
	fixture.analyzeJD(t, fixture.jdText)

	review, err := fixture.service.Analyze()
	if err != nil {
		t.Fatalf("analyze matches: %v", err)
	}
	if review.Status != "ready" || len(review.Requirements) != 10 {
		t.Fatalf("unexpected match review %#v", review)
	}
	if !review.HardCapApplied || review.TotalScore > 69 {
		t.Fatalf("expected missing hard constraint cap, got %#v", review)
	}
	if review.Counts.Strong == 0 || review.Counts.Missing == 0 {
		t.Fatalf("expected mixed match strengths, got %#v", review.Counts)
	}

	var foundTraceableEvidence bool
	for _, requirement := range review.Requirements {
		for _, evidence := range requirement.Evidence {
			if len(evidence.Sources) > 0 &&
				evidence.Sources[0].DocumentName == "backend-profile.md" {
				foundTraceableEvidence = true
			}
		}
	}
	if !foundTraceableEvidence {
		t.Fatal("expected match evidence to retain source document")
	}

	restarted := NewMatchService(
		fixture.matchRepository,
		fixture.scopeRepository,
		fixture.profileRepository,
		fixture.jdRepository,
		fakeprovider.New(),
		fixedClock{now: profileTestTime.Add(time.Hour)},
	)
	restored, err := restarted.GetReview()
	if err != nil {
		t.Fatalf("restore match review: %v", err)
	}
	if restored.Status != "ready" ||
		restored.TotalScore != review.TotalScore ||
		len(restored.Requirements) != len(review.Requirements) {
		t.Fatalf("unexpected restored review %#v", restored)
	}
}

func TestMatchServiceInvalidatesReviewWhenJDChanges(t *testing.T) {
	fixture := newMatchServiceFixture(t, fakeprovider.New())
	fixture.importProfile(t)
	fixture.analyzeJD(t, fixture.jdText)
	if _, err := fixture.service.Analyze(); err != nil {
		t.Fatalf("analyze matches: %v", err)
	}

	fixture.analyzeJD(t, fixture.jdText+"\nExperience with incident response.")
	review, err := fixture.service.GetReview()
	if err != nil {
		t.Fatalf("get stale review: %v", err)
	}
	if review.Status != "stale" {
		t.Fatalf("expected stale review, got %#v", review)
	}
}

func TestMatchServicePersistsProviderAndValidationFailures(t *testing.T) {
	for name, suggester := range map[string]ports.MatchSuggester{
		"provider":   failingMatchSuggester{},
		"validation": invalidMatchSuggester{},
	} {
		t.Run(name, func(t *testing.T) {
			fixture := newMatchServiceFixture(t, suggester)
			fixture.importProfile(t)
			fixture.analyzeJD(t, fixture.jdText)

			if _, err := fixture.service.Analyze(); err == nil {
				t.Fatal("expected match analysis error")
			}
			review, err := fixture.service.GetReview()
			if err != nil {
				t.Fatalf("get failed review: %v", err)
			}
			if review.Status != "failed" || review.Error == "" {
				t.Fatalf("expected persisted failure, got %#v", review)
			}
		})
	}
}

func TestMatchServiceReportsMissingPrerequisites(t *testing.T) {
	fixture := newMatchServiceFixture(t, fakeprovider.New())

	review, err := fixture.service.GetReview()
	if err != nil {
		t.Fatalf("get blocked review: %v", err)
	}
	if review.Status != "blocked" {
		t.Fatalf("expected blocked review, got %#v", review)
	}
}

func TestMatchServiceUsesActiveProfile(t *testing.T) {
	fixture := newMatchServiceFixture(t, fakeprovider.New())
	fixture.importProfile(t)
	fixture.analyzeJD(t, fixture.jdText)
	if _, err := fixture.service.Analyze(); err != nil {
		t.Fatalf("analyze default profile: %v", err)
	}
	mainProfile, err := fixture.profileService.GetOverview()
	if err != nil {
		t.Fatalf("get default profile: %v", err)
	}

	if _, err := fixture.profileService.CreateProfile(
		"Empty profile",
		"en",
	); err != nil {
		t.Fatalf("create profile: %v", err)
	}
	blocked, err := fixture.service.GetReview()
	if err != nil {
		t.Fatalf("get review for empty profile: %v", err)
	}
	if blocked.Status != "blocked" {
		t.Fatalf("expected active empty profile to block matching, got %#v", blocked)
	}

	if _, err := fixture.profileService.SelectProfile(
		mainProfile.ProfileID,
	); err != nil {
		t.Fatalf("restore default profile: %v", err)
	}
	restored, err := fixture.service.GetReview()
	if err != nil {
		t.Fatalf("restore default profile review: %v", err)
	}
	if restored.Status != "ready" {
		t.Fatalf("expected saved default profile review, got %#v", restored)
	}
}

func TestMatchServiceAppliesSelectedRunDocuments(t *testing.T) {
	fixture := newMatchServiceFixture(t, fakeprovider.New())
	fixture.importProfile(t)

	secondContents, err := os.ReadFile(filepath.Join(
		"..",
		"..",
		"testdata",
		"synthetic",
		"profile",
		"backend-profile-en.md",
	))
	if err != nil {
		t.Fatalf("read second profile fixture: %v", err)
	}
	fixture.profileService.picker = fakeMarkdownPicker{
		selected: ports.SelectedMarkdown{
			OriginalName: "backend-profile-en.md",
			Contents:     secondContents,
		},
		accepted: true,
	}
	if _, err := fixture.profileService.ImportMarkdown(); err != nil {
		t.Fatalf("import second profile document: %v", err)
	}
	fixture.analyzeJD(t, fixture.jdText)

	overview, err := fixture.profileService.GetOverview()
	if err != nil {
		t.Fatalf("get profile overview: %v", err)
	}
	if len(overview.Documents) != 2 {
		t.Fatalf("expected two profile documents, got %#v", overview.Documents)
	}
	var selectedID string
	var otherID string
	for _, document := range overview.Documents {
		if document.OriginalName == "backend-profile-en.md" {
			selectedID = document.ID
		} else {
			otherID = document.ID
		}
	}
	if selectedID == "" || otherID == "" {
		t.Fatalf("expected both imported documents, got %#v", overview.Documents)
	}
	pending, err := fixture.service.SaveScope(
		string(domain.RunScopeSelected),
		[]string{selectedID},
	)
	if err != nil {
		t.Fatalf("save selected run scope: %v", err)
	}
	if pending.Status != "pending" ||
		pending.Scope.SelectedCount != 1 ||
		pending.Scope.SelectedDocumentIDs[0] != selectedID {
		t.Fatalf("unexpected pending scoped review %#v", pending)
	}

	review, err := fixture.service.Analyze()
	if err != nil {
		t.Fatalf("analyze scoped match: %v", err)
	}
	if review.Status != "ready" || review.Scope.SelectedCount != 1 {
		t.Fatalf("unexpected scoped review %#v", review)
	}
	var evidenceCount int
	for _, requirement := range review.Requirements {
		for _, evidence := range requirement.Evidence {
			evidenceCount++
			for _, source := range evidence.Sources {
				if source.DocumentID != selectedID {
					t.Fatalf(
						"expected only selected document sources, got %#v",
						source,
					)
				}
			}
		}
	}
	if evidenceCount == 0 {
		t.Fatal("expected scoped match to retain relevant evidence")
	}
	resumeService := NewResumeService(
		fixture.scopeRepository,
		fixture.matchRepository,
		fixture.profileRepository,
		fixture.jdRepository,
		fakeprovider.New(),
		fixedClock{now: profileTestTime.Add(2 * time.Hour)},
	)
	resume, err := resumeService.Generate("en", 0.5)
	if err != nil {
		t.Fatalf("generate scoped resume: %v", err)
	}
	for _, block := range resume.Blocks {
		for _, evidence := range block.Evidence {
			for _, source := range evidence.Sources {
				if source.DocumentID != selectedID {
					t.Fatalf(
						"expected scoped resume sources, got %#v",
						source,
					)
				}
			}
		}
	}

	stale, err := fixture.service.SaveScope(
		string(domain.RunScopeSelected),
		[]string{otherID},
	)
	if err != nil {
		t.Fatalf("replace selected run scope: %v", err)
	}
	if stale.Status != "stale" {
		t.Fatalf("expected scope change to invalidate match, got %#v", stale)
	}
}

type matchServiceFixture struct {
	db                *sql.DB
	files             *filesystem.ManagedFiles
	service           *MatchService
	profileRepository *sqliteadapter.ProfileRepository
	jdRepository      *sqliteadapter.JDRepository
	matchRepository   *sqliteadapter.MatchRepository
	scopeRepository   *sqliteadapter.ResumeRepository
	profileService    *ProfileService
	jdService         *JDService
	jdText            string
}

func newMatchServiceFixture(
	t *testing.T,
	suggester ports.MatchSuggester,
) matchServiceFixture {
	return newMatchServiceFixtureFromFiles(
		t,
		suggester,
		"backend-profile.md",
		"backend-engineer.txt",
	)
}

func newMatchServiceFixtureFromFiles(
	t *testing.T,
	suggester ports.MatchSuggester,
	profileFile string,
	jdFile string,
) matchServiceFixture {
	t.Helper()

	root := t.TempDir()
	db, err := sqliteadapter.Open(
		context.Background(),
		filepath.Join(root, "autocv.db"),
	)
	if err != nil {
		t.Fatalf("open database: %v", err)
	}
	t.Cleanup(func() {
		db.Close()
	})
	profileContents, err := os.ReadFile(filepath.Join(
		"..",
		"..",
		"testdata",
		"synthetic",
		"profile",
		profileFile,
	))
	if err != nil {
		t.Fatalf("read profile fixture: %v", err)
	}
	jdContents, err := os.ReadFile(filepath.Join(
		"..",
		"..",
		"testdata",
		"synthetic",
		"jd",
		jdFile,
	))
	if err != nil {
		t.Fatalf("read JD fixture: %v", err)
	}
	files, err := filesystem.NewManagedFiles(root)
	if err != nil {
		t.Fatalf("create managed files: %v", err)
	}
	profileRepository := sqliteadapter.NewProfileRepository(db)
	jdRepository := sqliteadapter.NewJDRepository(db)
	matchRepository := sqliteadapter.NewMatchRepository(db)
	scopeRepository := sqliteadapter.NewResumeRepository(db)
	provider := fakeprovider.New()
	profileService := NewProfileService(
		profileRepository,
		sqliteadapter.NewProfileSearch(db),
		markdownparser.New(),
		provider,
		files,
		fakeMarkdownPicker{
			selected: ports.SelectedMarkdown{
				OriginalName: profileFile,
				Contents:     profileContents,
			},
			accepted: true,
		},
		fixedClock{now: profileTestTime},
	)
	jdService := NewJDService(
		jdRepository,
		provider,
		fixedClock{now: profileTestTime},
	)
	return matchServiceFixture{
		db:    db,
		files: files,
		service: NewMatchService(
			matchRepository,
			scopeRepository,
			profileRepository,
			jdRepository,
			suggester,
			fixedClock{now: profileTestTime},
		),
		profileRepository: profileRepository,
		jdRepository:      jdRepository,
		matchRepository:   matchRepository,
		scopeRepository:   scopeRepository,
		profileService:    profileService,
		jdService:         jdService,
		jdText:            string(jdContents),
	}
}

func (fixture matchServiceFixture) importProfile(t *testing.T) {
	t.Helper()
	if _, err := fixture.profileService.ImportMarkdown(); err != nil {
		t.Fatalf("import profile: %v", err)
	}
}

func (fixture matchServiceFixture) analyzeJD(t *testing.T, rawText string) {
	t.Helper()
	if _, err := fixture.jdService.Analyze(rawText); err != nil {
		t.Fatalf("analyze JD: %v", err)
	}
}

var _ ports.MatchSuggester = failingMatchSuggester{}
var _ ports.MatchSuggester = invalidMatchSuggester{}
