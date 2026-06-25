package fakeprovider

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"html"
	"strings"

	"github.com/ch1lam/autocv/internal/domain"
	"github.com/ch1lam/autocv/internal/ports"
)

const fakeResumeHTMLComposerVersion = "fake-resume-html/v1"

func (provider *Provider) ComposeResumeHTML(
	ctx context.Context,
	request ports.ComposeResumeHTMLRequest,
) (ports.ComposedResumeHTML, error) {
	if err := ctx.Err(); err != nil {
		return ports.ComposedResumeHTML{}, err
	}
	resume := domain.NormalizeResume(request.Resume)
	if strings.TrimSpace(request.Template.HTML) == "" {
		return ports.ComposedResumeHTML{}, fmt.Errorf("resume HTML template is empty")
	}
	body := fakeResumeHTMLBody(resume)
	fullHTML, err := replaceTemplateBody(request.Template.HTML, body)
	if err != nil {
		return ports.ComposedResumeHTML{}, err
	}
	inputHash := hashString(
		resume.ID + "\n" +
			resume.InputHash + "\n" +
			request.Template.ID + "\n" +
			request.Template.Version,
	)
	return ports.ComposedResumeHTML{
		HTML:            fullHTML,
		TemplateID:      request.Template.ID,
		TemplateVersion: request.Template.Version,
		Composer:        "fakeprovider",
		ComposerVersion: fakeResumeHTMLComposerVersion,
		PromptVersion:   "none",
		InputHash:       inputHash,
		HTMLHash:        hashString(fullHTML),
	}, nil
}

func (provider *Provider) CacheKey() string {
	return fakeResumeHTMLComposerVersion
}

func fakeResumeHTMLBody(resume domain.Resume) string {
	var builder strings.Builder
	builder.WriteString(`<main class="sheet">`)
	builder.WriteString("\n  <header class=\"resume-header\">\n")
	if strings.TrimSpace(resume.Header.Name) != "" {
		builder.WriteString(`    <h1 class="name" data-autocv-field="header.name">`)
		builder.WriteString(html.EscapeString(resume.Header.Name))
		builder.WriteString("</h1>\n")
	}
	builder.WriteString(`    <p class="target" data-autocv-field="header.target_role">`)
	builder.WriteString(html.EscapeString(resume.Header.TargetRole))
	builder.WriteString("</p>\n")
	if len(resume.Header.Contacts) > 0 {
		builder.WriteString(`    <ul class="contacts">`)
		for _, contact := range resume.Header.Contacts {
			builder.WriteString("<li>")
			builder.WriteString(html.EscapeString(contact.Value))
			builder.WriteString("</li>")
		}
		builder.WriteString("</ul>\n")
	}
	builder.WriteString("  </header>\n")
	for _, section := range resume.Sections {
		builder.WriteString(`  <section class="section" data-autocv-id="`)
		builder.WriteString(html.EscapeString(section.ID))
		builder.WriteString("\">\n")
		builder.WriteString(`    <h2 class="section-title" data-autocv-field="section.title">`)
		builder.WriteString(html.EscapeString(section.Title))
		builder.WriteString("</h2>\n")
		for _, item := range section.Items {
			builder.WriteString(`    <article class="item" data-autocv-id="`)
			builder.WriteString(html.EscapeString(item.ID))
			builder.WriteString("\">\n")
			if strings.TrimSpace(item.Title) != "" {
				builder.WriteString(`      <h3 class="item-title">`)
				builder.WriteString(html.EscapeString(item.Title))
				builder.WriteString("</h3>\n")
			}
			if strings.TrimSpace(item.Subtitle) != "" {
				builder.WriteString(`      <p class="item-subtitle">`)
				builder.WriteString(html.EscapeString(item.Subtitle))
				builder.WriteString("</p>\n")
			}
			builder.WriteString(`      <p class="item-content" data-autocv-field="item.content">`)
			builder.WriteString(html.EscapeString(item.Content))
			builder.WriteString("</p>\n")
			builder.WriteString("    </article>\n")
		}
		builder.WriteString("  </section>\n")
	}
	builder.WriteString("</main>")
	return builder.String()
}

func replaceTemplateBody(template string, body string) (string, error) {
	start := strings.Index(template, "<body>")
	end := strings.Index(template, "</body>")
	if start < 0 || end < 0 || end < start {
		return "", fmt.Errorf("resume HTML template body is missing")
	}
	return template[:start+len("<body>")] + "\n" + body + "\n" + template[end:], nil
}

func hashString(value string) string {
	digest := sha256.Sum256([]byte(value))
	return hex.EncodeToString(digest[:])
}

var _ ports.ResumeHTMLComposer = (*Provider)(nil)
