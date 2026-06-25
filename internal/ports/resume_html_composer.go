package ports

import (
	"context"

	"github.com/ch1lam/autocv/internal/domain"
)

type ResumeHTMLTemplate struct {
	ID        string
	Version   string
	Language  domain.ResumeLanguage
	HTML      string
	StyleHash string
}

type ComposeResumeHTMLRequest struct {
	Resume   domain.Resume
	Template ResumeHTMLTemplate
}

type ComposedResumeHTML struct {
	HTML            string
	TemplateID      string
	TemplateVersion string
	Composer        string
	ComposerVersion string
	PromptVersion   string
	InputHash       string
	HTMLHash        string
}

type ResumeHTMLComposer interface {
	ComposeResumeHTML(
		context.Context,
		ComposeResumeHTMLRequest,
	) (ComposedResumeHTML, error)
	CacheKey() string
}
