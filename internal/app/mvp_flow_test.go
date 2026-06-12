package app

import (
	"context"
	"encoding/base64"
	"strings"
	"testing"
	"time"

	"github.com/ch1lam/autocv/internal/adapters/fakeprovider"
	markdownparser "github.com/ch1lam/autocv/internal/adapters/markdown"
	sqliteadapter "github.com/ch1lam/autocv/internal/adapters/sqlite"
	"github.com/ch1lam/autocv/internal/domain"
	"github.com/ch1lam/autocv/internal/ports"
)

type mvpFixtureRenderer struct{}

func (mvpFixtureRenderer) Render(
	context.Context,
	domain.Resume,
) (ports.RenderedResume, error) {
	return ports.RenderedResume{
		PDF:          []byte("%PDF-1.7\nAutoCV synthetic fixture"),
		PreviewPages: [][]byte{[]byte("\x89PNG\r\nAutoCV synthetic fixture")},
	}, nil
}

func TestM1FakeProviderVerticalFlowPersistsSyntheticScenarios(
	t *testing.T,
) {
	tests := []struct {
		name               string
		profileFile        string
		jdFile             string
		resumeLanguage     string
		expectedJDLanguage string
		expectedHardCap    bool
	}{
		{
			name:               "Chinese",
			profileFile:        "backend-profile-zh.md",
			jdFile:             "backend-engineer-zh.txt",
			resumeLanguage:     "zh",
			expectedJDLanguage: "mixed",
		},
		{
			name:               "English",
			profileFile:        "backend-profile-en.md",
			jdFile:             "backend-engineer-en.txt",
			resumeLanguage:     "en",
			expectedJDLanguage: "en",
		},
		{
			name:               "InsufficientEvidence",
			profileFile:        "insufficient-profile-en.md",
			jdFile:             "backend-engineer-en.txt",
			resumeLanguage:     "en",
			expectedJDLanguage: "en",
			expectedHardCap:    true,
		},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			testM1SyntheticScenario(t, test)
		})
	}
}

func testM1SyntheticScenario(
	t *testing.T,
	test struct {
		name               string
		profileFile        string
		jdFile             string
		resumeLanguage     string
		expectedJDLanguage string
		expectedHardCap    bool
	},
) {
	t.Helper()
	fixture := newMatchServiceFixtureFromFiles(
		t,
		fakeprovider.New(),
		test.profileFile,
		test.jdFile,
	)
	imported, err := fixture.profileService.ImportMarkdown()
	if err != nil {
		t.Fatalf("import Markdown profile: %v", err)
	}
	if imported.ChunkCount == 0 || imported.EvidenceCount == 0 {
		t.Fatalf("expected traceable imported evidence, got %#v", imported)
	}

	jdWorkspace, err := fixture.jdService.Analyze(fixture.jdText)
	if err != nil {
		t.Fatalf("analyze JD: %v", err)
	}
	if jdWorkspace.AnalysisStatus != "succeeded" ||
		jdWorkspace.Analysis == nil {
		t.Fatalf("expected structured JD analysis, got %#v", jdWorkspace)
	}
	if jdWorkspace.Analysis.Language != test.expectedJDLanguage {
		t.Fatalf(
			"expected JD language %q, got %#v",
			test.expectedJDLanguage,
			jdWorkspace.Analysis,
		)
	}

	matchReview, err := fixture.service.Analyze()
	if err != nil {
		t.Fatalf("analyze match: %v", err)
	}
	if matchReview.Status != "ready" ||
		len(matchReview.Requirements) == 0 {
		t.Fatalf("expected scored match review, got %#v", matchReview)
	}
	if matchReview.HardCapApplied != test.expectedHardCap {
		t.Fatalf(
			"expected hard cap %t, got %#v",
			test.expectedHardCap,
			matchReview,
		)
	}
	if test.expectedHardCap && matchReview.TotalScore > 69 {
		t.Fatalf("hard-capped score exceeded 69: %#v", matchReview)
	}

	resumeRepository := sqliteadapter.NewResumeRepository(fixture.db)
	resumeService := NewResumeService(
		resumeRepository,
		fixture.matchRepository,
		fixture.profileRepository,
		fixture.jdRepository,
		fakeprovider.New(),
		fixedClock{now: profileTestTime.Add(2 * time.Hour)},
	)
	resumeWorkspace, err := resumeService.Generate(test.resumeLanguage, 0.5)
	if err != nil {
		t.Fatalf("generate resume: %v", err)
	}
	if resumeWorkspace.Status != "ready" ||
		!strings.HasPrefix(resumeWorkspace.Markdown, "# ") {
		t.Fatalf("expected generated Markdown resume, got %#v", resumeWorkspace)
	}

	artifactRepository := sqliteadapter.NewArtifactRepository(fixture.db)
	pdfService := NewPDFService(
		resumeService,
		artifactRepository,
		fixture.files,
		mvpFixtureRenderer{},
		fixedExportPicker{},
		fixedClock{now: profileTestTime.Add(3 * time.Hour)},
	)
	pdfWorkspace, err := pdfService.Render()
	if err != nil {
		t.Fatalf("render PDF artifact: %v", err)
	}
	if pdfWorkspace.Status != "ready" || !pdfWorkspace.CanExport ||
		len(pdfWorkspace.PreviewPagesBase64) != 1 {
		t.Fatalf("expected ready PDF workspace, got %#v", pdfWorkspace)
	}
	pdfBytes, err := base64.StdEncoding.DecodeString(pdfWorkspace.PDFBase64)
	if err != nil {
		t.Fatalf("decode PDF workspace: %v", err)
	}
	if !strings.HasPrefix(string(pdfBytes), "%PDF-") {
		t.Fatal("expected persisted PDF content")
	}

	restartedProfile := NewProfileService(
		fixture.profileRepository,
		markdownparser.New(),
		fakeprovider.New(),
		fixture.files,
		fakeMarkdownPicker{},
		fixedClock{now: profileTestTime.Add(4 * time.Hour)},
	)
	restartedResume := NewResumeService(
		resumeRepository,
		fixture.matchRepository,
		fixture.profileRepository,
		fixture.jdRepository,
		fakeprovider.New(),
		fixedClock{now: profileTestTime.Add(4 * time.Hour)},
	)
	restartedPDF := NewPDFService(
		restartedResume,
		artifactRepository,
		fixture.files,
		mvpFixtureRenderer{},
		fixedExportPicker{},
		fixedClock{now: profileTestTime.Add(4 * time.Hour)},
	)

	profileOverview, err := restartedProfile.GetOverview()
	if err != nil {
		t.Fatalf("restore profile: %v", err)
	}
	restoredJD, err := fixture.jdService.GetWorkspace()
	if err != nil {
		t.Fatalf("restore JD: %v", err)
	}
	restoredMatch, err := fixture.service.GetReview()
	if err != nil {
		t.Fatalf("restore match: %v", err)
	}
	restoredResume, err := restartedResume.GetWorkspace()
	if err != nil {
		t.Fatalf("restore resume: %v", err)
	}
	restoredPDF, err := restartedPDF.GetWorkspace()
	if err != nil {
		t.Fatalf("restore PDF: %v", err)
	}
	if len(profileOverview.Documents) != 1 ||
		restoredJD.AnalysisStatus != "succeeded" ||
		restoredMatch.Status != "ready" ||
		restoredResume.Status != "ready" ||
		restoredPDF.Status != "ready" ||
		restoredPDF.ArtifactID != pdfWorkspace.ArtifactID {
		t.Fatalf(
			"expected complete persisted M1 flow, got profile=%#v jd=%#v match=%#v resume=%#v pdf=%#v",
			profileOverview,
			restoredJD,
			restoredMatch,
			restoredResume,
			restoredPDF,
		)
	}
}

var _ ports.ResumeRenderer = mvpFixtureRenderer{}
