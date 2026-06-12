package typst

import (
	"bytes"
	"context"
	"os/exec"
	"strings"
	"testing"
	"time"

	"github.com/ch1lam/autocv/internal/domain"
)

func TestNewViewModelUsesLanguageSpecificHeadingsAndItems(t *testing.T) {
	resume := renderFixture(domain.ResumeLanguageChinese)
	resume.Blocks[1].Content = "- Go 服务\n- 可观测性治理"
	resume.Blocks = append(resume.Blocks, domain.ResumeBlock{
		Kind:    domain.ResumeBlockExperience,
		Content: "负责数据库性能优化。",
	})

	view := NewViewModel(resume)
	if view.TargetRole != "后端工程师" {
		t.Fatalf("unexpected target role %q", view.TargetRole)
	}
	if view.Sections[0].Heading != "职业概述" ||
		view.Sections[1].Heading != "工作经历" {
		t.Fatalf("unexpected section headings %#v", view.Sections)
	}
	if view.Sections[1].Items[0].Kind != "bullet" ||
		view.Sections[1].Items[0].Text != "Go 服务" {
		t.Fatalf("unexpected list item %#v", view.Sections[1].Items[0])
	}
	if len(view.Sections) != 2 || len(view.Sections[1].Items) != 3 {
		t.Fatalf("expected repeated block kinds to share a section: %#v", view)
	}
	if !containsString(view.Fonts, "Noto Sans CJK SC") {
		t.Fatalf("expected portable Chinese font fallback: %#v", view.Fonts)
	}
}

func TestNewViewModelUsesPortableEnglishFontFallback(t *testing.T) {
	view := NewViewModel(renderFixture(domain.ResumeLanguageEnglish))
	if !containsString(view.Fonts, "Liberation Sans") {
		t.Fatalf("expected portable English font fallback: %#v", view.Fonts)
	}
}

func TestRendererCompilesChineseAndEnglishTextPDFs(t *testing.T) {
	if _, err := exec.LookPath("typst"); err != nil {
		t.Skip("typst CLI is not installed")
	}
	renderer := NewRenderer("typst", 20*time.Second)

	for _, language := range []domain.ResumeLanguage{
		domain.ResumeLanguageChinese,
		domain.ResumeLanguageEnglish,
	} {
		t.Run(string(language), func(t *testing.T) {
			rendered, err := renderer.Render(
				context.Background(),
				renderFixture(language),
			)
			if err != nil {
				t.Fatalf("render PDF: %v", err)
			}
			if !bytes.HasPrefix(rendered.PDF, []byte("%PDF-")) {
				t.Fatal("expected PDF header")
			}
			if !bytes.Contains(rendered.PDF, []byte("/ToUnicode")) {
				t.Fatal("expected PDF text mapping for selectable text")
			}
			if len(rendered.PreviewPages) == 0 ||
				!bytes.HasPrefix(rendered.PreviewPages[0], []byte("\x89PNG")) {
				t.Fatal("expected PNG preview pages")
			}
		})
	}
}

func TestRendererReportsMissingBinary(t *testing.T) {
	renderer := NewRenderer("autocv-missing-typst", time.Second)
	_, err := renderer.Render(
		context.Background(),
		renderFixture(domain.ResumeLanguageEnglish),
	)
	if err == nil || !strings.Contains(err.Error(), "Typst rendering failed") {
		t.Fatalf("expected Typst execution error, got %v", err)
	}
}

func renderFixture(language domain.ResumeLanguage) domain.Resume {
	role := "后端工程师"
	summary := "围绕目标岗位，重点呈现 Go 服务与稳定性治理。"
	experience := "负责支付平台服务开发，将接口延迟降低 35%。"
	if language == domain.ResumeLanguageEnglish {
		role = "Backend Engineer"
		summary = "Backend engineer focused on reliable Go services."
		experience = "Built payment services and reduced API latency by 35%."
	}
	return domain.Resume{
		ID:         "resume-1",
		RunID:      "run-1",
		InputHash:  "input-1",
		Version:    1,
		Language:   language,
		TargetRole: role,
		Blocks: []domain.ResumeBlock{
			{Kind: domain.ResumeBlockSummary, Content: summary},
			{Kind: domain.ResumeBlockExperience, Content: experience},
		},
	}
}

func containsString(values []string, expected string) bool {
	for _, value := range values {
		if value == expected {
			return true
		}
	}
	return false
}
