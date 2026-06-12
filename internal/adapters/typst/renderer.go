package typst

import (
	"context"
	"embed"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"sort"
	"strings"
	"time"

	"github.com/ch1lam/autocv/internal/domain"
	"github.com/ch1lam/autocv/internal/ports"
)

const (
	dataFilename     = "resume.json"
	templateFilename = "resume.typ"
	outputFilename   = "resume.pdf"
	previewPattern   = "preview-{p}.png"
)

//go:embed templates/resume.typ
var templateFS embed.FS

type Renderer struct {
	binary  string
	timeout time.Duration
}

type ViewModel struct {
	Language   string        `json:"language"`
	TargetRole string        `json:"target_role"`
	Fonts      []string      `json:"fonts"`
	Sections   []ViewSection `json:"sections"`
}

type ViewSection struct {
	Heading string     `json:"heading"`
	Items   []ViewItem `json:"items"`
}

type ViewItem struct {
	Kind string `json:"kind"`
	Text string `json:"text"`
}

func NewRenderer(binary string, timeout time.Duration) *Renderer {
	if strings.TrimSpace(binary) == "" {
		binary = "typst"
	}
	if timeout <= 0 {
		timeout = 20 * time.Second
	}
	return &Renderer{binary: binary, timeout: timeout}
}

func (renderer *Renderer) Render(
	ctx context.Context,
	resume domain.Resume,
) (ports.RenderedResume, error) {
	view := NewViewModel(resume)
	data, err := json.MarshalIndent(view, "", "  ")
	if err != nil {
		return ports.RenderedResume{}, fmt.Errorf(
			"encode Typst resume data: %w",
			err,
		)
	}
	template, err := templateFS.ReadFile("templates/" + templateFilename)
	if err != nil {
		return ports.RenderedResume{}, fmt.Errorf(
			"read embedded Typst template: %w",
			err,
		)
	}

	directory, err := os.MkdirTemp("", "autocv-typst-*")
	if err != nil {
		return ports.RenderedResume{}, fmt.Errorf(
			"create Typst temporary directory: %w",
			err,
		)
	}
	defer os.RemoveAll(directory)

	for name, contents := range map[string][]byte{
		dataFilename:     data,
		templateFilename: template,
	} {
		if err := os.WriteFile(
			filepath.Join(directory, name),
			contents,
			0o600,
		); err != nil {
			return ports.RenderedResume{}, fmt.Errorf(
				"write Typst %s: %w",
				name,
				err,
			)
		}
	}

	renderContext, cancel := context.WithTimeout(ctx, renderer.timeout)
	defer cancel()
	if err := renderer.compile(
		renderContext,
		directory,
		outputFilename,
	); err != nil {
		return ports.RenderedResume{}, err
	}
	if err := renderer.compile(
		renderContext,
		directory,
		previewPattern,
		"--ppi",
		"144",
	); err != nil {
		return ports.RenderedResume{}, err
	}
	if renderContext.Err() != nil {
		return ports.RenderedResume{}, fmt.Errorf(
			"Typst rendering timed out after %s",
			renderer.timeout,
		)
	}

	pdf, err := os.ReadFile(filepath.Join(directory, outputFilename))
	if err != nil {
		return ports.RenderedResume{}, fmt.Errorf("read rendered PDF: %w", err)
	}
	if len(pdf) < 5 || string(pdf[:5]) != "%PDF-" {
		return ports.RenderedResume{}, errors.New("Typst output is not a PDF")
	}
	previewPaths, err := filepath.Glob(filepath.Join(directory, "preview-*.png"))
	if err != nil {
		return ports.RenderedResume{}, fmt.Errorf(
			"list rendered preview pages: %w",
			err,
		)
	}
	sort.Strings(previewPaths)
	if len(previewPaths) == 0 {
		return ports.RenderedResume{}, errors.New(
			"Typst did not render preview pages",
		)
	}
	previews := make([][]byte, 0, len(previewPaths))
	for _, path := range previewPaths {
		page, err := os.ReadFile(path)
		if err != nil {
			return ports.RenderedResume{}, fmt.Errorf(
				"read rendered preview page: %w",
				err,
			)
		}
		previews = append(previews, page)
	}
	return ports.RenderedResume{PDF: pdf, PreviewPages: previews}, nil
}

func (renderer *Renderer) compile(
	ctx context.Context,
	directory string,
	output string,
	extraArguments ...string,
) error {
	arguments := []string{
		"compile",
		"--root",
		directory,
	}
	arguments = append(arguments, extraArguments...)
	arguments = append(
		arguments,
		filepath.Join(directory, templateFilename),
		filepath.Join(directory, output),
	)
	command := exec.CommandContext(ctx, renderer.binary, arguments...)
	command.Dir = directory
	diagnostics, err := command.CombinedOutput()
	if ctx.Err() != nil {
		return fmt.Errorf(
			"Typst rendering timed out after %s",
			renderer.timeout,
		)
	}
	if err != nil {
		return fmt.Errorf(
			"Typst rendering failed: %s",
			diagnosticSummary(diagnostics),
		)
	}
	return nil
}

func NewViewModel(resume domain.Resume) ViewModel {
	sections := make([]ViewSection, 0, len(resume.Blocks))
	indexByKind := make(map[domain.ResumeBlockKind]int)
	for _, block := range resume.Blocks {
		items := viewItems(block.Content)
		if len(items) == 0 {
			continue
		}
		if index, exists := indexByKind[block.Kind]; exists {
			sections[index].Items = append(sections[index].Items, items...)
			continue
		}
		indexByKind[block.Kind] = len(sections)
		sections = append(sections, ViewSection{
			Heading: sectionHeading(resume.Language, block.Kind),
			Items:   items,
		})
	}
	return ViewModel{
		Language:   string(resume.Language),
		TargetRole: strings.TrimSpace(resume.TargetRole),
		Fonts:      fontsFor(resume.Language),
		Sections:   sections,
	}
}

func viewItems(contents string) []ViewItem {
	lines := strings.Split(strings.ReplaceAll(contents, "\r\n", "\n"), "\n")
	items := make([]ViewItem, 0, len(lines))
	for _, line := range lines {
		line = strings.TrimSpace(line)
		if line == "" {
			continue
		}
		kind := "paragraph"
		for _, prefix := range []string{"- ", "* ", "• "} {
			if strings.HasPrefix(line, prefix) {
				kind = "bullet"
				line = strings.TrimSpace(strings.TrimPrefix(line, prefix))
				break
			}
		}
		if line != "" {
			items = append(items, ViewItem{Kind: kind, Text: line})
		}
	}
	return items
}

func sectionHeading(
	language domain.ResumeLanguage,
	kind domain.ResumeBlockKind,
) string {
	if language == domain.ResumeLanguageEnglish {
		switch kind {
		case domain.ResumeBlockSummary:
			return "Profile"
		case domain.ResumeBlockExperience:
			return "Experience"
		case domain.ResumeBlockProject:
			return "Projects"
		case domain.ResumeBlockSkill:
			return "Skills"
		case domain.ResumeBlockEducation:
			return "Education"
		case domain.ResumeBlockCertification:
			return "Certifications"
		}
	}
	switch kind {
	case domain.ResumeBlockSummary:
		return "职业概述"
	case domain.ResumeBlockExperience:
		return "工作经历"
	case domain.ResumeBlockProject:
		return "项目经历"
	case domain.ResumeBlockSkill:
		return "技能"
	case domain.ResumeBlockEducation:
		return "教育经历"
	case domain.ResumeBlockCertification:
		return "认证"
	default:
		return string(kind)
	}
}

func fontsFor(language domain.ResumeLanguage) []string {
	if language == domain.ResumeLanguageChinese {
		return []string{"Charter", "Hiragino Sans GB", "Arial Unicode MS"}
	}
	return []string{"Charter", "Arial"}
}

func diagnosticSummary(output []byte) string {
	message := strings.TrimSpace(string(output))
	if message == "" {
		return "no diagnostic output"
	}
	const limit = 1200
	if len(message) > limit {
		return message[:limit] + "..."
	}
	return message
}

var _ ports.ResumeRenderer = (*Renderer)(nil)
