package app

import (
	"context"
	"crypto/sha256"
	"encoding/base64"
	"encoding/hex"
	"errors"
	"fmt"
	"path/filepath"
	"strings"
	"unicode"

	"github.com/ch1lam/autocv/internal/domain"
	"github.com/ch1lam/autocv/internal/ports"
	"github.com/google/uuid"
)

type PDFService struct {
	resumes   *ResumeService
	artifacts ports.ArtifactRepository
	store     ports.ArtifactStore
	renderer  ports.ResumeRenderer
	picker    ports.ExportPicker
	clock     ports.Clock
}

type PDFWorkspace struct {
	Status             string   `json:"status"`
	Message            string   `json:"message"`
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
) *PDFService {
	return &PDFService{
		resumes:   resumes,
		artifacts: artifacts,
		store:     store,
		renderer:  renderer,
		picker:    picker,
		clock:     clock,
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

func (service *PDFService) Render() (PDFWorkspace, error) {
	ctx := context.Background()
	run, resume, err := service.resumes.currentReadyResume(ctx)
	if err != nil {
		return PDFWorkspace{}, err
	}
	rendered, err := service.renderer.Render(ctx, resume)
	if err != nil {
		return PDFWorkspace{}, err
	}
	if len(rendered.PreviewPages) == 0 {
		return PDFWorkspace{}, errors.New("rendered PDF has no preview pages")
	}

	artifactID := uuid.NewString()
	path, err := service.store.SaveArtifact(
		run.ID,
		artifactID,
		"pdf",
		rendered.PDF,
	)
	if err != nil {
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
		return PDFWorkspace{}, err
	}
	return service.GetWorkspace()
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
