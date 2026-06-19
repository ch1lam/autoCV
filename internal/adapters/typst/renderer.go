package typst

import (
	"context"
	"embed"
	"encoding/json"
	"errors"
	"fmt"
	neturl "net/url"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"sort"
	"strings"
	"time"
	"unicode/utf8"

	"github.com/ch1lam/autocv/internal/domain"
	"github.com/ch1lam/autocv/internal/ports"
)

const (
	ExpectedTypstVersion = "typst 0.14.2"
	TemplateVersion      = "resume.typ/v1"
	dataFilename         = "resume.json"
	templateFilename     = "resume.typ"
	outputFilename       = "resume.pdf"
	previewPattern       = "preview-{p}.png"
)

//go:embed templates/resume.typ
var templateFS embed.FS

type Renderer struct {
	binary  string
	timeout time.Duration
}

type ViewModel struct {
	Language     string        `json:"language"`
	TargetRole   string        `json:"target_role"`
	BodyFonts    []string      `json:"body_fonts"`
	HeadingFonts []string      `json:"heading_fonts"`
	Sections     []ViewSection `json:"sections"`
}

type ViewSection struct {
	Heading string     `json:"heading"`
	Items   []ViewItem `json:"items"`
}

type ViewItem struct {
	Kind string    `json:"kind"`
	Text string    `json:"text"`
	Runs []TextRun `json:"runs"`
}

type TextRun struct {
	Kind string `json:"kind"`
	Text string `json:"text"`
	URL  string `json:"url,omitempty"`
}

var (
	markdownLinkPattern = regexp.MustCompile(`\[([^\[\]\n]+)\]\((https?://[^\s)]+)\)`)
	rawURLPattern       = regexp.MustCompile(`https?://[^\s<>()\[\]]+`)
)

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
	rendererVersion, err := renderer.version(renderContext)
	if err != nil {
		return ports.RenderedResume{}, err
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
	return ports.RenderedResume{
		PDF:          pdf,
		PreviewPages: previews,
		Metadata: ports.RenderMetadata{
			Renderer:                "typst",
			RendererVersion:         rendererVersion,
			ExpectedRendererVersion: ExpectedTypstVersion,
			TemplateVersion:         TemplateVersion,
		},
	}, nil
}

func (renderer *Renderer) version(ctx context.Context) (string, error) {
	command := exec.CommandContext(ctx, renderer.binary, "--version")
	diagnostics, err := command.CombinedOutput()
	if ctx.Err() != nil {
		return "", fmt.Errorf(
			"Typst version check timed out after %s",
			renderer.timeout,
		)
	}
	if err != nil {
		return "", fmt.Errorf(
			"Typst version check failed: %s",
			diagnosticSummary(diagnostics),
		)
	}
	version := strings.TrimSpace(string(diagnostics))
	if version == "" {
		return "", errors.New("Typst version check returned empty output")
	}
	return version, nil
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
		Language:     string(resume.Language),
		TargetRole:   strings.TrimSpace(resume.TargetRole),
		BodyFonts:    bodyFontsFor(resume.Language),
		HeadingFonts: headingFontsFor(resume.Language),
		Sections:     sections,
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
			items = append(items, ViewItem{
				Kind: kind,
				Text: line,
				Runs: textRuns(line),
			})
		}
	}
	return items
}

func textRuns(value string) []TextRun {
	runs := make([]TextRun, 0, 1)
	cursor := 0
	for _, match := range markdownLinkPattern.FindAllStringSubmatchIndex(value, -1) {
		if match[0] > cursor {
			runs = appendRawURLRuns(runs, value[cursor:match[0]])
		}
		label := value[match[2]:match[3]]
		destination := value[match[4]:match[5]]
		if isHTTPURL(destination) {
			runs = append(runs, TextRun{
				Kind: "link",
				Text: label,
				URL:  destination,
			})
		} else {
			runs = appendRawURLRuns(runs, value[match[0]:match[1]])
		}
		cursor = match[1]
	}
	if cursor < len(value) {
		runs = appendRawURLRuns(runs, value[cursor:])
	}
	if len(runs) == 0 {
		return []TextRun{{Kind: "text", Text: value}}
	}
	return runs
}

func appendRawURLRuns(runs []TextRun, value string) []TextRun {
	cursor := 0
	for _, match := range rawURLPattern.FindAllStringIndex(value, -1) {
		if match[0] > cursor {
			runs = appendTextRun(runs, value[cursor:match[0]])
		}
		destination, suffix := splitURLTrailingPunctuation(
			value[match[0]:match[1]],
		)
		if isHTTPURL(destination) {
			runs = append(runs, TextRun{
				Kind: "link",
				Text: destination,
				URL:  destination,
			})
			runs = appendTextRun(runs, suffix)
		} else {
			runs = appendTextRun(runs, value[match[0]:match[1]])
		}
		cursor = match[1]
	}
	if cursor < len(value) {
		runs = appendTextRun(runs, value[cursor:])
	}
	return runs
}

func appendTextRun(runs []TextRun, value string) []TextRun {
	if value == "" {
		return runs
	}
	if len(runs) > 0 && runs[len(runs)-1].Kind == "text" {
		runs[len(runs)-1].Text += value
		return runs
	}
	return append(runs, TextRun{Kind: "text", Text: value})
}

func splitURLTrailingPunctuation(value string) (string, string) {
	end := len(value)
	for end > 0 {
		character, size := utf8.DecodeLastRuneInString(value[:end])
		if !strings.ContainsRune(".,;:!?", character) {
			break
		}
		end -= size
	}
	return value[:end], value[end:]
}

func isHTTPURL(value string) bool {
	parsed, err := neturl.ParseRequestURI(value)
	if err != nil {
		return false
	}
	return (parsed.Scheme == "http" || parsed.Scheme == "https") &&
		parsed.Host != ""
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

func bodyFontsFor(language domain.ResumeLanguage) []string {
	if language == domain.ResumeLanguageChinese {
		return []string{
			"Charter",
			"Songti SC",
			"PingFang SC",
			"Hiragino Sans GB",
			"Arial Unicode MS",
			"Noto Sans CJK SC",
			"Liberation Sans",
		}
	}
	return []string{
		"Charter",
		"Libertinus Serif",
		"Georgia",
		"Times New Roman",
		"Liberation Serif",
		"DejaVu Serif",
	}
}

func headingFontsFor(language domain.ResumeLanguage) []string {
	if language == domain.ResumeLanguageChinese {
		return []string{
			"PingFang SC",
			"Hiragino Sans GB",
			"Songti SC",
			"Arial Unicode MS",
			"Noto Sans CJK SC",
			"Liberation Sans",
		}
	}
	return []string{
		"Avenir Next",
		"Helvetica Neue",
		"Arial",
		"Liberation Sans",
		"DejaVu Sans",
	}
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
