package app

import (
	"context"
	"errors"
	"fmt"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/ch1lam/autocv/internal/adapters/fakeprovider"
	"github.com/ch1lam/autocv/internal/adapters/filesystem"
	sqliteadapter "github.com/ch1lam/autocv/internal/adapters/sqlite"
	"github.com/ch1lam/autocv/internal/domain"
	"github.com/ch1lam/autocv/internal/ports"
	"github.com/ch1lam/autocv/internal/workflow"
)

type memoryArtifactRepository struct {
	artifact domain.Artifact
	found    bool
}

func (repository *memoryArtifactRepository) GetLatest(
	_ context.Context,
	runID string,
	kind domain.ArtifactKind,
) (domain.Artifact, bool, error) {
	if !repository.found ||
		repository.artifact.RunID != runID ||
		repository.artifact.Kind != kind {
		return domain.Artifact{}, false, nil
	}
	return repository.artifact, true, nil
}

func (repository *memoryArtifactRepository) Save(
	_ context.Context,
	artifact domain.Artifact,
) error {
	repository.artifact = artifact
	repository.found = true
	return nil
}

type sequentialRenderer struct {
	calls            int
	previewPageCount int
}

func (renderer *sequentialRenderer) Render(
	_ context.Context,
	_ domain.Resume,
) (ports.RenderedResume, error) {
	renderer.calls++
	if renderer.calls > 1 {
		return ports.RenderedResume{}, errors.New("synthetic Typst failure")
	}
	pageCount := renderer.previewPageCount
	if pageCount == 0 {
		pageCount = 1
	}
	previews := make([][]byte, 0, pageCount)
	for page := 1; page <= pageCount; page++ {
		previews = append(
			previews,
			[]byte(fmt.Sprintf("\x89PNG\r\nsynthetic-page-%d", page)),
		)
	}
	return ports.RenderedResume{
		PDF:          []byte("%PDF-1.7\nsynthetic"),
		PreviewPages: previews,
	}, nil
}

type fixedExportPicker struct {
	pdfPath      string
	markdownPath string
}

func (picker fixedExportPicker) PickPDF(string) (string, bool, error) {
	return picker.pdfPath, picker.pdfPath != "", nil
}

func (picker fixedExportPicker) PickMarkdown(string) (string, bool, error) {
	return picker.markdownPath, picker.markdownPath != "", nil
}

type ungroundedResumeDrafter struct{}

func (ungroundedResumeDrafter) DraftResume(
	_ context.Context,
	request ports.DraftResumeRequest,
) (domain.ResumeDraft, error) {
	return domain.ResumeDraft{
		Language:   request.Language,
		TargetRole: request.TargetRole,
		Blocks: []domain.ResumeBlockDraft{{
			Kind:           domain.ResumeBlockSummary,
			Content:        "适合承担目标岗位相关职责。",
			GroundingLevel: domain.GroundingDerived,
			Optimization:   "待用户确认的岗位定位。",
		}},
		OptimizationNotes: []string{"存在待确认内容。"},
	}, nil
}

func TestPDFServicePreservesLastArtifactWhenRenderingFails(t *testing.T) {
	resumes := newResumeServiceFixture(t)
	generated, err := resumes.Generate("zh", 0.5)
	if err != nil {
		t.Fatalf("generate resume: %v", err)
	}
	store, err := filesystem.NewManagedFiles(t.TempDir())
	if err != nil {
		t.Fatalf("create artifact store: %v", err)
	}
	artifacts := &memoryArtifactRepository{}
	renderer := &sequentialRenderer{}
	service := NewPDFService(
		resumes,
		artifacts,
		store,
		renderer,
		fixedExportPicker{},
		fixedClock{now: profileTestTime.Add(3 * time.Hour)},
	)

	rendered, err := service.Render()
	if err != nil {
		t.Fatalf("render PDF: %v", err)
	}
	if rendered.Status != "ready" || !rendered.CanExport ||
		rendered.ResumeID != generated.ResumeID {
		t.Fatalf("unexpected rendered workspace %#v", rendered)
	}
	stageResult, found, err := resumes.stageRepository.LatestStageResult(
		context.Background(),
		generated.RunID,
		workflow.StageRendered,
	)
	if err != nil {
		t.Fatalf("read PDF stage result: %v", err)
	}
	if !found ||
		stageResult.Status != workflow.StageStatusSucceeded ||
		!strings.Contains(stageResult.ResultJSON, `"artifact_id"`) {
		t.Fatalf("unexpected PDF stage result found=%v %#v", found, stageResult)
	}
	artifactID := rendered.ArtifactID

	reused, err := service.Render()
	if err != nil {
		t.Fatalf("reuse rendered PDF: %v", err)
	}
	if reused.ArtifactID != artifactID || renderer.calls != 1 {
		t.Fatalf(
			"expected PDF render reuse artifact=%q calls=%d workspace=%#v",
			artifactID,
			renderer.calls,
			reused,
		)
	}

	editedMarkdown := strings.Replace(
		generated.Markdown,
		generated.Blocks[0].Content,
		generated.Blocks[0].Content+" 用户确认补充。",
		1,
	)
	if _, err := resumes.UpdateMarkdown(editedMarkdown); err != nil {
		t.Fatalf("update resume before failed render: %v", err)
	}
	service.clock = fixedClock{now: profileTestTime.Add(4 * time.Hour)}
	if _, err := service.Render(); err == nil {
		t.Fatal("expected render after resume change to fail")
	}
	stageResult, found, err = resumes.stageRepository.LatestStageResult(
		context.Background(),
		generated.RunID,
		workflow.StageRendered,
	)
	if err != nil {
		t.Fatalf("read failed PDF stage result: %v", err)
	}
	if !found ||
		stageResult.Status != workflow.StageStatusFailed ||
		!strings.Contains(stageResult.ErrorJSON, "synthetic Typst failure") {
		t.Fatalf("unexpected failed PDF stage result found=%v %#v", found, stageResult)
	}
	restored, err := service.GetWorkspace()
	if err != nil {
		t.Fatalf("restore PDF workspace: %v", err)
	}
	if restored.ArtifactID != artifactID || restored.Status != "stale" {
		t.Fatalf("expected previous artifact to survive, got %#v", restored)
	}
}

func TestPDFServiceExportsCurrentArtifactAndMarkdown(t *testing.T) {
	resumes := newResumeServiceFixture(t)
	if _, err := resumes.Generate("en", 0.5); err != nil {
		t.Fatalf("generate resume: %v", err)
	}
	root := t.TempDir()
	store, err := filesystem.NewManagedFiles(root)
	if err != nil {
		t.Fatalf("create artifact store: %v", err)
	}
	pdfPath := filepath.Join(t.TempDir(), "resume")
	markdownPath := filepath.Join(t.TempDir(), "resume")
	service := NewPDFService(
		resumes,
		&memoryArtifactRepository{},
		store,
		&sequentialRenderer{},
		fixedExportPicker{
			pdfPath:      pdfPath,
			markdownPath: markdownPath,
		},
		fixedClock{now: time.Date(2026, 6, 12, 8, 0, 0, 0, time.UTC)},
	)
	if _, err := service.Render(); err != nil {
		t.Fatalf("render PDF: %v", err)
	}

	pdfResult, err := service.ExportPDF()
	if err != nil {
		t.Fatalf("export PDF: %v", err)
	}
	if pdfResult.Path != pdfPath+".pdf" {
		t.Fatalf("unexpected PDF export path %q", pdfResult.Path)
	}
	markdownResult, err := service.ExportMarkdown()
	if err != nil {
		t.Fatalf("export Markdown: %v", err)
	}
	if markdownResult.Path != markdownPath+".md" {
		t.Fatalf("unexpected Markdown export path %q", markdownResult.Path)
	}
}

func TestPDFServiceWarnsWhenRenderedPDFExceedsTwoPages(t *testing.T) {
	resumes := newResumeServiceFixture(t)
	if _, err := resumes.Generate("zh", 0.5); err != nil {
		t.Fatalf("generate resume: %v", err)
	}
	store, err := filesystem.NewManagedFiles(t.TempDir())
	if err != nil {
		t.Fatalf("create artifact store: %v", err)
	}
	service := NewPDFService(
		resumes,
		&memoryArtifactRepository{},
		store,
		&sequentialRenderer{previewPageCount: 3},
		fixedExportPicker{},
		fixedClock{now: time.Date(2026, 6, 12, 9, 0, 0, 0, time.UTC)},
	)

	workspace, err := service.Render()
	if err != nil {
		t.Fatalf("render PDF: %v", err)
	}
	if !workspace.CanExport {
		t.Fatalf("page-count warning should not block export, got %#v", workspace)
	}
	if len(workspace.PreviewPagesBase64) != 3 {
		t.Fatalf("expected three preview pages, got %#v", workspace)
	}
	if len(workspace.Warnings) != 1 ||
		!strings.Contains(workspace.Warnings[0], "PDF 当前为 3 页") {
		t.Fatalf("expected two-page warning, got %#v", workspace.Warnings)
	}

	restored, err := service.GetWorkspace()
	if err != nil {
		t.Fatalf("restore PDF workspace: %v", err)
	}
	if len(restored.Warnings) != 1 ||
		restored.Warnings[0] != workspace.Warnings[0] {
		t.Fatalf("expected restored warning, got %#v", restored.Warnings)
	}
}

func TestPDFServiceBlocksExportForUnconfirmedContent(t *testing.T) {
	matchFixture := newMatchServiceFixture(t, fakeprovider.New())
	matchFixture.importProfile(t)
	matchFixture.analyzeJD(t, matchFixture.jdText)
	if _, err := matchFixture.service.Analyze(); err != nil {
		t.Fatalf("analyze matches: %v", err)
	}
	resumes := NewResumeService(
		sqliteadapter.NewResumeRepository(matchFixture.db),
		matchFixture.stageRepository,
		matchFixture.confirmationRepository,
		matchFixture.matchRepository,
		matchFixture.profileRepository,
		matchFixture.jdRepository,
		ungroundedResumeDrafter{},
		fixedClock{now: profileTestTime.Add(2 * time.Hour)},
	)
	generated, err := resumes.Generate("zh", 0.5)
	if err != nil {
		t.Fatalf("generate reviewable resume: %v", err)
	}
	if generated.CanExport || len(generated.ExportIssues) != 1 {
		t.Fatalf("expected export gate, got %#v", generated)
	}

	service := NewPDFService(
		resumes,
		&memoryArtifactRepository{},
		matchFixture.files,
		&sequentialRenderer{},
		fixedExportPicker{
			pdfPath:      filepath.Join(t.TempDir(), "resume.pdf"),
			markdownPath: filepath.Join(t.TempDir(), "resume.md"),
		},
		fixedClock{now: profileTestTime.Add(3 * time.Hour)},
	)
	workspace, err := service.Render()
	if err != nil {
		t.Fatalf("render review PDF: %v", err)
	}
	if workspace.CanExport || len(workspace.ExportIssues) != 1 {
		t.Fatalf("expected preview-only PDF workspace, got %#v", workspace)
	}
	if _, err := service.ExportPDF(); err == nil ||
		!strings.Contains(err.Error(), "export blocked") {
		t.Fatalf("expected PDF export block, got %v", err)
	}
	if _, err := service.ExportMarkdown(); err == nil ||
		!strings.Contains(err.Error(), "export blocked") {
		t.Fatalf("expected Markdown export block, got %v", err)
	}
}

var _ ports.ResumeDrafter = ungroundedResumeDrafter{}
