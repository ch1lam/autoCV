package app

import (
	"context"
	"crypto/sha256"
	"encoding/base64"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"log/slog"
	"path/filepath"
	"strings"
	"time"
	"unicode"

	"github.com/ch1lam/autocv/internal/domain"
	"github.com/ch1lam/autocv/internal/ports"
	"github.com/ch1lam/autocv/internal/workflow"
	"github.com/google/uuid"
)

type PDFService struct {
	resumes   *ResumeService
	artifacts ports.ArtifactRepository
	store     ports.ArtifactStore
	renderer  ports.ResumeRenderer
	picker    ports.ExportPicker
	clock     ports.Clock
	events    WorkflowEventSink
}

type PDFWorkspace struct {
	Status             string   `json:"status"`
	Message            string   `json:"message"`
	Warnings           []string `json:"warnings"`
	ExportIssues       []string `json:"exportIssues"`
	ArtifactID         string   `json:"artifactId"`
	ResumeID           string   `json:"resumeId"`
	Version            int      `json:"version"`
	Language           string   `json:"language"`
	TargetRole         string   `json:"targetRole"`
	RenderedAt         string   `json:"renderedAt"`
	ContentHash        string   `json:"contentHash"`
	PDFBase64          string   `json:"pdfBase64"`
	PreviewPagesBase64 []string `json:"previewPagesBase64"`
	CanExport          bool     `json:"canExport"`
}

type ExportResult struct {
	Cancelled bool   `json:"cancelled"`
	Kind      string `json:"kind"`
	Path      string `json:"path"`
}

func NewPDFService(
	resumes *ResumeService,
	artifacts ports.ArtifactRepository,
	store ports.ArtifactStore,
	renderer ports.ResumeRenderer,
	picker ports.ExportPicker,
	clock ports.Clock,
	workflowEvents ...WorkflowEventSink,
) *PDFService {
	return &PDFService{
		resumes:   resumes,
		artifacts: artifacts,
		store:     store,
		renderer:  renderer,
		picker:    picker,
		clock:     clock,
		events:    workflowEventSinkFrom(workflowEvents),
	}
}

func (service *PDFService) GetWorkspace() (PDFWorkspace, error) {
	ctx := context.Background()
	resumeWorkspace, err := service.resumes.GetWorkspace()
	if err != nil {
		return PDFWorkspace{}, err
	}
	if resumeWorkspace.RunID == "" {
		return PDFWorkspace{
			Status:       "blocked",
			Message:      resumeWorkspace.Message,
			Warnings:     pdfWorkspaceWarnings(0),
			ExportIssues: resumeWorkspace.ExportIssues,
			Version:      resumeWorkspace.Version,
			Language:     resumeWorkspace.Language,
			TargetRole:   resumeWorkspace.TargetRole,
		}, nil
	}

	artifact, found, err := service.artifacts.GetLatest(
		ctx,
		resumeWorkspace.RunID,
		domain.ArtifactKindPDF,
	)
	if err != nil {
		return PDFWorkspace{}, err
	}
	if !found {
		status := "pending"
		message := "当前简历尚未生成 PDF。"
		if resumeWorkspace.Status == "stale" {
			status = "blocked"
			message = resumeWorkspace.Message
		}
		return PDFWorkspace{
			Status:       status,
			Message:      message,
			Warnings:     pdfWorkspaceWarnings(0),
			ExportIssues: resumeWorkspace.ExportIssues,
			ResumeID:     resumeWorkspace.ResumeID,
			Version:      resumeWorkspace.Version,
			Language:     resumeWorkspace.Language,
			TargetRole:   resumeWorkspace.TargetRole,
		}, nil
	}

	pdf, err := service.store.ReadArtifact(artifact.Path)
	if err != nil {
		return PDFWorkspace{}, fmt.Errorf("read PDF artifact: %w", err)
	}
	previewPages := make([]string, 0, len(artifact.PreviewPaths))
	for _, previewPath := range artifact.PreviewPaths {
		page, err := service.store.ReadArtifact(previewPath)
		if err != nil {
			return PDFWorkspace{}, fmt.Errorf(
				"read PDF preview page: %w",
				err,
			)
		}
		previewPages = append(
			previewPages,
			base64.StdEncoding.EncodeToString(page),
		)
	}
	current := resumeWorkspace.Status == "ready" &&
		artifact.ResumeID == resumeWorkspace.ResumeID
	status := "ready"
	message := "PDF 已从当前 Resume 版本渲染并保存到本地。"
	if !current {
		status = "stale"
		message = "正在预览上一份成功 PDF；当前简历版本需要重新渲染。"
	} else if !resumeWorkspace.CanExport {
		message = "当前 PDF 可以预览，但存在未确认内容，暂不能导出。"
	}
	return PDFWorkspace{
		Status:             status,
		Message:            message,
		Warnings:           pdfWorkspaceWarnings(len(previewPages)),
		ExportIssues:       resumeWorkspace.ExportIssues,
		ArtifactID:         artifact.ID,
		ResumeID:           artifact.ResumeID,
		Version:            resumeWorkspace.Version,
		Language:           resumeWorkspace.Language,
		TargetRole:         resumeWorkspace.TargetRole,
		RenderedAt:         artifact.CreatedAt.UTC().Format(timeFormat),
		ContentHash:        artifact.ContentHash,
		PDFBase64:          base64.StdEncoding.EncodeToString(pdf),
		PreviewPagesBase64: previewPages,
		CanExport:          current && resumeWorkspace.CanExport,
	}, nil
}

func pdfWorkspaceWarnings(pageCount int) []string {
	if pageCount <= 2 {
		return make([]string, 0)
	}
	return []string{
		fmt.Sprintf(
			"PDF 当前为 %d 页，默认目标是两页以内；建议压缩内容或调整取舍，必要时仍可导出。",
			pageCount,
		),
	}
}

func (service *PDFService) Render() (PDFWorkspace, error) {
	return service.render(false)
}

func (service *PDFService) rerun() (PDFWorkspace, error) {
	return service.render(true)
}

func (service *PDFService) render(force bool) (PDFWorkspace, error) {
	ctx := context.Background()
	run, resume, err := service.resumes.currentReadyResume(ctx)
	if err != nil {
		return PDFWorkspace{}, err
	}
	inputHash := service.hashPDFRenderInput(resume)
	if !force {
		if workspace, reused, err := service.reuseSuccessfulPDFRenderStage(
			ctx,
			run.ID,
			resume,
			inputHash,
		); err != nil {
			return PDFWorkspace{}, err
		} else if reused {
			return workspace, nil
		}
	}
	now := service.clock.Now().UTC()
	service.savePDFRenderStageResult(
		ctx,
		run.ID,
		inputHash,
		workflow.StageStatusRunning,
		"",
		"",
		now,
	)
	rendered, err := service.renderer.Render(ctx, resume)
	if err != nil {
		status := workflow.StageStatusFailed
		if errors.Is(err, context.Canceled) {
			status = workflow.StageStatusCancelled
		}
		service.savePDFRenderStageResult(
			ctx,
			run.ID,
			inputHash,
			status,
			"",
			stageErrorJSON("PDF render failed", err),
			service.clock.Now().UTC(),
		)
		return PDFWorkspace{}, err
	}
	if len(rendered.PreviewPages) == 0 {
		err := errors.New("rendered PDF has no preview pages")
		service.savePDFRenderStageResult(
			ctx,
			run.ID,
			inputHash,
			workflow.StageStatusFailed,
			"",
			stageErrorJSON("PDF render failed", err),
			service.clock.Now().UTC(),
		)
		return PDFWorkspace{}, err
	}

	artifactID := uuid.NewString()
	path, err := service.store.SaveArtifact(
		run.ID,
		artifactID,
		"pdf",
		rendered.PDF,
	)
	if err != nil {
		service.savePDFRenderStageResult(
			ctx,
			run.ID,
			inputHash,
			workflow.StageStatusFailed,
			"",
			stageErrorJSON("PDF render failed", err),
			service.clock.Now().UTC(),
		)
		return PDFWorkspace{}, err
	}
	previewPaths := make([]string, 0, len(rendered.PreviewPages))
	for index, page := range rendered.PreviewPages {
		previewPath, err := service.store.SaveArtifact(
			run.ID,
			fmt.Sprintf("%s-page-%d", artifactID, index+1),
			"png",
			page,
		)
		if err != nil {
			service.savePDFRenderStageResult(
				ctx,
				run.ID,
				inputHash,
				workflow.StageStatusFailed,
				"",
				stageErrorJSON("PDF render failed", err),
				service.clock.Now().UTC(),
			)
			return PDFWorkspace{}, err
		}
		previewPaths = append(previewPaths, previewPath)
	}
	digest := sha256.Sum256(rendered.PDF)
	artifact := domain.Artifact{
		ID:           artifactID,
		RunID:        run.ID,
		ResumeID:     resume.ID,
		Kind:         domain.ArtifactKindPDF,
		Path:         path,
		PreviewPaths: previewPaths,
		ContentHash:  hex.EncodeToString(digest[:]),
		CreatedAt:    service.clock.Now().UTC(),
	}
	if err := service.artifacts.Save(ctx, artifact); err != nil {
		service.savePDFRenderStageResult(
			ctx,
			run.ID,
			inputHash,
			workflow.StageStatusFailed,
			"",
			stageErrorJSON("PDF render failed", err),
			service.clock.Now().UTC(),
		)
		return PDFWorkspace{}, err
	}
	run.Stage = string(workflow.StageRendered)
	run.UpdatedAt = artifact.CreatedAt
	if err := service.resumes.resumeRepository.SaveRun(ctx, run); err != nil {
		service.savePDFRenderStageResult(
			ctx,
			run.ID,
			inputHash,
			workflow.StageStatusFailed,
			"",
			stageErrorJSON("PDF render failed", err),
			service.clock.Now().UTC(),
		)
		return PDFWorkspace{}, err
	}
	service.savePDFRenderStageResult(
		ctx,
		run.ID,
		inputHash,
		workflow.StageStatusSucceeded,
		pdfRenderStageResultJSON(artifact, resume, rendered.Metadata, inputHash),
		"",
		artifact.CreatedAt,
	)
	return service.GetWorkspace()
}

func (service *PDFService) reuseSuccessfulPDFRenderStage(
	ctx context.Context,
	runID string,
	resume domain.Resume,
	inputHash string,
) (PDFWorkspace, bool, error) {
	if service.resumes.stageRepository == nil {
		return PDFWorkspace{}, false, nil
	}
	stageResult, found, err := service.resumes.stageRepository.SucceededStageResult(
		ctx,
		runID,
		workflow.StageRendered,
		inputHash,
	)
	if err != nil {
		return PDFWorkspace{}, false, err
	}
	if !found {
		return PDFWorkspace{}, false, nil
	}
	var payload struct {
		ArtifactID string `json:"artifact_id"`
		ResumeID   string `json:"resume_id"`
	}
	if err := json.Unmarshal([]byte(stageResult.ResultJSON), &payload); err != nil {
		return PDFWorkspace{}, false, nil
	}
	if payload.ResumeID != resume.ID {
		return PDFWorkspace{}, false, nil
	}
	artifact, found, err := service.artifacts.GetLatest(
		ctx,
		runID,
		domain.ArtifactKindPDF,
	)
	if err != nil {
		return PDFWorkspace{}, false, err
	}
	if !found || artifact.ID != payload.ArtifactID ||
		artifact.ResumeID != resume.ID {
		return PDFWorkspace{}, false, nil
	}
	workspace, err := service.GetWorkspace()
	if err != nil {
		return PDFWorkspace{}, false, err
	}
	return workspace, true, nil
}

func (service *PDFService) savePDFRenderStageResult(
	ctx context.Context,
	runID string,
	inputHash string,
	status workflow.StageStatus,
	resultJSON string,
	errorJSON string,
	now time.Time,
) {
	if service.resumes.stageRepository == nil {
		return
	}
	result := workflow.StageResult{
		ID:         stageResultID(runID, workflow.StageRendered, inputHash),
		RunID:      runID,
		Stage:      workflow.StageRendered,
		InputHash:  inputHash,
		Status:     status,
		ResultJSON: resultJSON,
		ErrorJSON:  errorJSON,
		CreatedAt:  now,
		UpdatedAt:  now,
	}
	if err := service.resumes.stageRepository.SaveStageResult(ctx, result); err != nil {
		slog.Error(
			"pdf.stage_result.persist.failed",
			slog.String("run_id", runID),
			slog.String("stage", string(workflow.StageRendered)),
			slog.String("status", string(status)),
			slog.Any("error", err),
		)
		return
	}
	emitWorkflowStageEvent(
		service.events,
		runID,
		workflow.StageRendered,
		status,
		errorJSON,
		now,
	)
}

func (service *PDFService) hashPDFRenderInput(resume domain.Resume) string {
	return hashPDFRenderInput(resume, rendererCacheKey(service.renderer))
}

func hashPDFRenderInput(resume domain.Resume, rendererCacheKey string) string {
	digest := sha256.Sum256([]byte(fmt.Sprintf(
		"%s\n%d\n%s\n%s",
		resume.ID,
		resume.Version,
		resume.InputHash,
		rendererCacheKey,
	)))
	return hex.EncodeToString(digest[:])
}

func rendererCacheKey(renderer ports.ResumeRenderer) string {
	if versioned, ok := renderer.(ports.VersionedResumeRenderer); ok {
		return versioned.CacheKey()
	}
	return "renderer=legacy"
}

func pdfRenderStageResultJSON(
	artifact domain.Artifact,
	resume domain.Resume,
	metadata ports.RenderMetadata,
	inputHash string,
) string {
	return stageResultJSON(struct {
		ArtifactID              string `json:"artifact_id"`
		ResumeID                string `json:"resume_id"`
		InputHash               string `json:"input_hash"`
		Version                 int    `json:"version"`
		PageCount               int    `json:"page_count"`
		Renderer                string `json:"renderer,omitempty"`
		RendererVersion         string `json:"renderer_version,omitempty"`
		ExpectedRendererVersion string `json:"expected_renderer_version,omitempty"`
		TemplateVersion         string `json:"template_version,omitempty"`
		HTMLTemplateID          string `json:"html_template_id,omitempty"`
		HTMLHash                string `json:"html_hash,omitempty"`
		HTMLStyleHash           string `json:"html_style_hash,omitempty"`
		Composer                string `json:"composer,omitempty"`
		ComposerVersion         string `json:"composer_version,omitempty"`
		PromptVersion           string `json:"prompt_version,omitempty"`
	}{
		ArtifactID:              artifact.ID,
		ResumeID:                resume.ID,
		InputHash:               inputHash,
		Version:                 resume.Version,
		PageCount:               len(artifact.PreviewPaths),
		Renderer:                metadata.Renderer,
		RendererVersion:         metadata.RendererVersion,
		ExpectedRendererVersion: metadata.ExpectedRendererVersion,
		TemplateVersion:         metadata.TemplateVersion,
		HTMLTemplateID:          metadata.HTMLTemplateID,
		HTMLHash:                metadata.HTMLHash,
		HTMLStyleHash:           metadata.HTMLStyleHash,
		Composer:                metadata.Composer,
		ComposerVersion:         metadata.ComposerVersion,
		PromptVersion:           metadata.PromptVersion,
	})
}

func (service *PDFService) ExportPDF() (ExportResult, error) {
	ctx := context.Background()
	run, resume, err := service.resumes.currentReadyResume(ctx)
	if err != nil {
		return ExportResult{}, err
	}
	if err := domain.ValidateResumeForExport(resume); err != nil {
		return ExportResult{}, err
	}
	artifact, found, err := service.artifacts.GetLatest(
		ctx,
		run.ID,
		domain.ArtifactKindPDF,
	)
	if err != nil {
		return ExportResult{}, err
	}
	if !found || artifact.ResumeID != resume.ID {
		return ExportResult{}, errors.New(
			"current resume version has not been rendered",
		)
	}

	destination, selected, err := service.picker.PickPDF(
		exportFilename(resume, ".pdf"),
	)
	if err != nil {
		return ExportResult{}, err
	}
	if !selected {
		return ExportResult{Cancelled: true, Kind: "pdf"}, nil
	}
	destination = ensureExtension(destination, ".pdf")
	if err := service.store.ExportArtifact(
		artifact.Path,
		destination,
	); err != nil {
		return ExportResult{}, err
	}
	return ExportResult{Kind: "pdf", Path: destination}, nil
}

func (service *PDFService) ExportMarkdown() (ExportResult, error) {
	_, resume, err := service.resumes.currentReadyResume(context.Background())
	if err != nil {
		return ExportResult{}, err
	}
	if err := domain.ValidateResumeForExport(resume); err != nil {
		return ExportResult{}, err
	}
	destination, selected, err := service.picker.PickMarkdown(
		exportFilename(resume, ".md"),
	)
	if err != nil {
		return ExportResult{}, err
	}
	if !selected {
		return ExportResult{Cancelled: true, Kind: "markdown"}, nil
	}
	destination = ensureExtension(destination, ".md")
	if err := service.store.ExportContents(
		[]byte(resume.Markdown),
		destination,
	); err != nil {
		return ExportResult{}, err
	}
	return ExportResult{Kind: "markdown", Path: destination}, nil
}

func exportFilename(resume domain.Resume, extension string) string {
	var builder strings.Builder
	for _, character := range strings.TrimSpace(resume.TargetRole) {
		switch {
		case unicode.IsLetter(character), unicode.IsNumber(character):
			builder.WriteRune(character)
		case unicode.IsSpace(character), character == '-', character == '_':
			if builder.Len() > 0 {
				builder.WriteRune('-')
			}
		}
		if builder.Len() >= 48 {
			break
		}
	}
	role := strings.Trim(builder.String(), "-")
	if role == "" {
		role = "resume"
	}
	return fmt.Sprintf("AutoCV-%s-v%d%s", role, resume.Version, extension)
}

func ensureExtension(path string, extension string) string {
	if strings.EqualFold(filepath.Ext(path), extension) {
		return path
	}
	return path + extension
}
