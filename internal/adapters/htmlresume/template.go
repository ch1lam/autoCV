package htmlresume

import (
	"crypto/sha256"
	"embed"
	"encoding/hex"
	"fmt"
	"strings"

	"github.com/ch1lam/autocv/internal/domain"
	"github.com/ch1lam/autocv/internal/ports"
)

const (
	TemplateVersion = "resume-html/v1"
	templateZH      = "templates/resume-zh.html"
	templateEN      = "templates/resume-en.html"
)

//go:embed templates/*.html
var templateFS embed.FS

func TemplateFor(language domain.ResumeLanguage) (ports.ResumeHTMLTemplate, error) {
	path := templateZH
	id := "kami-resume-zh"
	switch language {
	case domain.ResumeLanguageChinese:
	case domain.ResumeLanguageEnglish:
		path = templateEN
		id = "kami-resume-en"
	default:
		return ports.ResumeHTMLTemplate{}, fmt.Errorf(
			"unsupported resume HTML template language %q",
			language,
		)
	}
	contents, err := templateFS.ReadFile(path)
	if err != nil {
		return ports.ResumeHTMLTemplate{}, fmt.Errorf(
			"read resume HTML template: %w",
			err,
		)
	}
	html := string(contents)
	style, err := extractStyle(html)
	if err != nil {
		return ports.ResumeHTMLTemplate{}, err
	}
	digest := sha256.Sum256([]byte(style))
	return ports.ResumeHTMLTemplate{
		ID:        id,
		Version:   TemplateVersion,
		Language:  language,
		HTML:      html,
		StyleHash: hex.EncodeToString(digest[:]),
	}, nil
}

func extractStyle(html string) (string, error) {
	lower := strings.ToLower(html)
	start := strings.Index(lower, "<style>")
	end := strings.Index(lower, "</style>")
	if start < 0 || end < 0 || end < start {
		return "", fmt.Errorf("resume HTML template style block is missing")
	}
	start += len("<style>")
	return html[start:end], nil
}
